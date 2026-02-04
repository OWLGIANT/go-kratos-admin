package bitget_spot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/brokerconfig"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var (
	WS_HOST = "wss://ws.bitget.com"
	TARGET  = "/spot/v1/stream"
)

var (
	wsPublicHandyPool fastjson.ParserPool
)

type BitgetSpotWs struct {
	base.FatherWs
	pubWs *ws.WS // 继承ws
	// 私有订阅合在 pubWs，暂时保留
	priWs         *ws.WS                 // 继承ws
	connectOnce   sync.Once              // 连接一次
	id            int64                  // 所有发送的消息都要唯一id
	stopCPri      chan struct{}          // 接受停机信号
	colo          bool                   // 是否colo
	mapSymbols    map[string]helper.Pair //
	onMsgInterval atomic.Int64           // 触发onMsg间隔
	takerFee      atomic.Float64         // taker费率
	makerFee      atomic.Float64         // maker费率
	lastTotal     atomic.Float64         // 最新精致
	rs            *BitgetSpot
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BitgetSpotWsUrl != "" {
		WS_HOST = cfg.BitgetSpotWsUrl
		params.Logger.Infof("bitget_spot ws 启用colo  %v", WS_HOST)
	}
	w := &BitgetSpotWs{}
	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)

	w.pubWs = ws.NewWS(WS_HOST+TARGET, params.LocalAddr, params.ProxyURL, w.pubHandler, cb.OnExit, params.BrokerConfig)
	w.pubWs.SetHeader(http.Header{
		"User-Agent": []string{helper.UserAgentSlice[rand.Intn(len(helper.UserAgentSlice))]},
	})
	w.pubWs.SetPingFunc(w.ping)
	w.pubWs.SetPingInterval(10)
	w.pubWs.SetSubscribe(w.subPub)
	if params.NeedAuth {
		w.priWs = ws.NewWS(WS_HOST+TARGET, params.LocalAddr, params.ProxyURL, w.priHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetHeader(http.Header{
			"User-Agent": []string{helper.UserAgentSlice[rand.Intn(len(helper.UserAgentSlice))]},
		})
		w.priWs.SetPingFunc(w.ping)
		w.priWs.SetPingInterval(10)
		w.priWs.SetAuth(w.auth)
		w.priWs.SetSubscribe(w.subPri)
	}
	rs0 := NewRs(&w.BrokerConfig, &helper.TradeMsg{}, &helper.ExchangeInfo{}, w.Cb)
	ok := false
	w.rs, ok = rs0.(*BitgetSpot)
	if !ok {
		w.Logger.Errorf(" rs 转换失败")
		w.Cb.OnExit(" rs 转换失败")
	}
	w.rs.getExchangeInfo()
	w.UpdateExchangeInfo(w.rs.ExchangeInfoPtrP2S, w.rs.ExchangeInfoPtrS2P, w.Cb.OnExit)
	w.Symbol = w.rs.Symbol

	w.ExchangeInfoPtrP2S = w.rs.ExchangeInfoPtrP2S
	w.ExchangeInfoPtrS2P = w.rs.ExchangeInfoPtrS2P

	w.AddWsConnections(w.pubWs, w.priWs)
	return w
}

func (w *BitgetSpotWs) ping() []byte {
	return []byte("ping")
}

func (w *BitgetSpotWs) auth() error {
	// 先进行鉴权
	now := time.Now().Unix()
	prehash := fmt.Sprint(now) + "GET" + "/user/verify"
	hm := hmac.New(sha256.New, helper.StringToBytes(w.BrokerConfig.SecretKey))
	hm.Write(helper.StringToBytes(prehash))
	sign := base64.StdEncoding.EncodeToString(hm.Sum(nil))
	e := []map[string]interface{}{
		{
			"apiKey":     w.BrokerConfig.AccessKey,
			"passphrase": w.BrokerConfig.PassKey,
			"timestamp":  now,
			"sign":       sign,
		}}
	p := map[string]interface{}{
		"op":   "login",
		"args": e,
	}
	msg, _ := json.Marshal(p)
	w.priWs.SendMessage(msg)
	return nil
}
func (w *BitgetSpotWs) subPri() error {
	e := []map[string]interface{}{
		// 订阅资产
		{
			"instType": "spbl",
			"channel":  "account",
			// "instId":   b.symbol, // 是的，没看错，bg symbol inst概念混乱
			"instId": "default",
		}}
	if w.BrokerConfig.SymbolAll {
		e = append(e, map[string]interface{}{
			"instType": "spbl",
			"channel":  "orders",
			"instId":   "default",
		})
	} else {
		for _, p := range w.BrokerConfig.Pairs {
			e = append(e, map[string]interface{}{
				"instType": "spbl",
				"channel":  "orders",
				"instId":   w.rs.PairToSymbol(&p),
			})
		}
	}

	p := map[string]interface{}{
		"op":   "subscribe",
		"args": e,
	}
	msg, _ := json.Marshal(p)
	w.priWs.SubWithRetry("subscribe", w.Cb.OnExit, func() []byte { return msg })

	return nil
}

// 订阅函数 注意订阅顺序和时间间隔
func (w *BitgetSpotWs) subPub() error {
	// step 1 公有订阅 ticker
	var p map[string]interface{}
	var e []map[string]interface{}
	var msg []byte
	s := strings.Split(w.Symbol, "_")[0]
	if w.BrokerConfig.NeedTicker {
		w.id++
		e = append(e, map[string]interface{}{
			"instType": "SP",
			"channel":  "books1",
			"instId":   s,
		})
	}
	// 订阅depth
	if w.BrokerConfig.WsDepthLevel > 0 {
		w.id++
		e = append(e, map[string]interface{}{
			"instType": "SP",
			"channel":  "books15",
			"instId":   s,
		})
	}
	if w.BrokerConfig.NeedTrade {
		w.id++
		e = append(e, map[string]interface{}{
			"instType": "SP",
			"channel":  "trade",
			"instId":   s,
		})
	}
	if len(e) > 0 {
		p = map[string]interface{}{
			"op":   "subscribe",
			"args": e,
		}
		msg, _ = json.Marshal(p)
		w.pubWs.SubWithRetry("subscribe", w.Cb.OnExit, func() []byte { return msg })
		w.Logger.Infof("[bitget_usdt_swap] 发送订阅公共行情")
	} else {
		w.Logger.Infof("[bitget_usdt_swap] 没订阅任何行情")
	}
	// 订阅 public 成交
	return nil
}

// Run 准备ws连接 仅第一次调用时连接ws
func (w *BitgetSpotWs) Run() {
	w.connectOnce.Do(func() {
		var err error
		w.StopC, err = w.pubWs.Serve()
		if err != nil {
			w.Logger.Errorf("websocket serve error: %v", err)
			w.Cb.OnExit(fmt.Sprintf("websocket serve error: %v", err))
		}
		if w.BrokerConfig.NeedAuth {
			w.stopCPri, err = w.priWs.Serve()
			if err != nil {
				panic(err)
			}
		}
	})
}

func (w *BitgetSpotWs) DoStop() {
	if w.StopC != nil {
		helper.CloseSafe(w.StopC)
	}
	if w.stopCPri != nil {
		helper.CloseSafe(w.stopCPri)
	}
	w.connectOnce = sync.Once{}
}

func (w *BitgetSpotWs) priHandler(msg []byte, ts int64) {
	// 解析
	if helper.DEBUGMODE {
		w.Logger.Debugf("收到 pri ws 推送 %s", helper.BytesToString(msg))
	}
	if len(msg) == 4 && helper.BytesToString(msg) == "pong" {
		return
	}

	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		msg_str := helper.BytesToString(msg)
		if msg_str == "pong" {

		} else {
			w.Logger.Errorf("[%s]ws 解析msg失败 %v err:%s", w.ExchangeName, msg_str, err.Error())
		}
		return
	}

	// 消费信息
	if value.Exists("data") {
		channel := helper.BytesToString(value.GetStringBytes("arg", "channel"))
		datas := value.GetArray("data")
		switch channel {
		case "account":
			// snapshot推送较慢，update推送变化的币种（挂单时只变化基础币）
			// {"action":"snapshot","arg":{"instType":"spbl","channel":"account","instId":"default"},"data":[{"coinId":"746","coinName":"ORDI","available":"0.00074745","frozen":"0","lock":"0","coinDisplayName":"ORDI"},{"coinId":"715","coinName":"PEPE","available":"0.17955","frozen":"0","lock":"0","coinDisplayName":"PEPE"},{"coinId":"3","coinName":"ETH","available":"0.039491985","frozen":"0","lock":"0","coinDisplayName":"ETH"},{"coinId":"203","coinName":"TONCOIN","available":"0.000434","frozen":"0","lock":"0","coinDisplayName":"TON"},{"coinId":"2","coinName":"USDT","available":"295.536987580919904","frozen":"99.999474208","lock":"0","coinDisplayName":"USDT"},{"coinId":"1467","coinName":"DOGS","available":"91630.274617","frozen":"0","lock":"0","coinDisplayName":"DOGS"},{"coinId":"122","coinName":"SOL","available":"0.00008202","frozen":"0","lock":"0","coinDisplayName":"SOL"},{"coinId":"1159","coinName":"BRETT","available":"0.006878","frozen":"0","lock":"0","coinDisplayName":"BRETT"},{"coinId":"1","coinName":"BTC","available":"0.00000071275","frozen":"0","lock":"0","coinDisplayName":"BTC"}],"ts":1725377056331}
			// {"action":"update","arg":{"instType":"spbl","channel":"account","instId":"default"},"data":[{"coinId":"2","coinName":"USDT","available":"395.536461788919904","frozen":"0","lock":"0","coinDisplayName":"USDT"}],"ts":1725377059670}
			// s := helper.MustGetShadowStringFromBytes(value, "action") == "snapshot"
			// assetMap := make(map[string]bool)
			seqInEx := helper.MustGetInt64(value, "ts")
			for _, data := range datas {
				coinName := helper.MustGetStringLowerFromBytes(data, "coinDisplayName")
				// assetMap[coinName] = true
				if e, ok := w.EquityNewerAndStore(coinName, seqInEx, ts, (helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail)); ok {
					avail := helper.MustGetFloat64FromBytes(data, "available")
					tot := avail + helper.MustGetFloat64FromBytes(data, "frozen") + helper.MustGetFloat64FromBytes(data, "lock")
					e.TotalWithoutUpl = tot
					e.Avail = avail
					w.Cb.OnEquityEvent(0, *e)
				}

			}
			// if s {
			// 	w.CleanOtherEquity(assetMap, w.Cb.OnEquityEvent)
			// }
		case "orders":
			if len(datas) < 1 {
				return
			}
			for _, data := range datas {
				instId := helper.BytesToString(data.GetStringBytes("instId"))
				info := w.PairInfo
				if len(w.BrokerConfig.Pairs) > 0 {
					var ok bool
					info, ok = w.ExchangeInfoPtrS2P.Get(instId)
					if !ok {
						w.Logger.Warnf("unknow pos of symbol, %s", instId)
						return
					}
				} else if instId != w.Symbol {
					continue
				}
				status := helper.BytesToString(data.GetStringBytes("status"))
				orderId := string(data.GetStringBytes("ordId"))
				clientId := string(data.GetStringBytes("clOrdId"))
				event := helper.OrderEvent{Pair: info.Pair}
				event.OrderID = orderId
				event.ClientID = clientId
				switch status {
				case "new", "init":
					event.Type = helper.OrderEventTypeNEW
				case "full-fill", "cancelled":
					event.Type = helper.OrderEventTypeREMOVE
				case "partial-fill":
					event.Type = helper.OrderEventTypePARTIAL
				default:
					w.Logger.Warnf("unknow order status. %s", status)
				}
				event.Filled = fixed.NewS(helper.BytesToString(data.GetStringBytes("accFillSz"))) // 累计成交数量
				event.FilledPrice = helper.GetFloat64FromBytes(data, "avgPx")
				// 如果有成交
				if event.Filled.GreaterThan(fixed.ZERO) {
					// 计算最新一笔maker的手续费 检查是否异常
					execType := helper.BytesToString(data.GetStringBytes("execType"))
					if execType == "M" {
						fillFeeCcy := helper.BytesToString(data.GetStringBytes("fillFeeCcy")) //最新一笔成交的手续费币种
						if fillFeeCcy == "USDT" {
							fillSz := helper.GetFloat64FromBytes(data, "fillSz")   // 最新成交数量
							fillFee := helper.GetFloat64FromBytes(data, "fillFee") // 最新成交费
							event.CashFee = fixed.NewF(-fillFee)                   // 负为rebate 正为扣
							makerFeeRate := fixed.NewS(fmt.Sprintf("%.2f", -fillFee/(fillSz*event.FilledPrice))).Float()
							w.makerFee.Store(makerFeeRate * 1e4)
							// vip0 maker手续费万2 vip1 maker手续费万0.6
							if makerFeeRate > 0.00008 {
								//b.Cb.OnExit(fmt.Sprintf("bitget合约maker手续费异常%.6f", makerFeeRate))
								now := time.Now().UnixMilli()
								if now-w.onMsgInterval.Load() > 900000 {
									w.onMsgInterval.Store(now)
									w.Cb.OnMsg(fmt.Sprintf("%v bitget合约maker手续费异常%.6f", w.BrokerConfig.Name, makerFeeRate))
								}
							}
						}
					}
				}
				switch helper.MustGetShadowStringFromBytes(data, "side") {
				case "buy":
					event.OrderSide = helper.OrderSideKD
				case "sell":
					event.OrderSide = helper.OrderSideKK
				default:
					w.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(data, "side"))
					return
				}
				switch helper.MustGetShadowStringFromBytes(data, "ordType") {
				case "limit":
					event.OrderType = helper.OrderTypeLimit
				case "market":
					event.OrderType = helper.OrderTypeMarket
				}
				// 必须在type后面。忽略其他类型
				switch helper.MustGetShadowStringFromBytes(data, "force") {
				case "ioc":
					event.OrderType = helper.OrderTypeIoc
				case "post_only":
					event.OrderType = helper.OrderTypePostOnly
				}
				// }
				event.ReceivedTsNs = ts
				w.Cb.OnOrder(ts, event)
			}
		}
	} else {
		event := helper.BytesToString(value.GetStringBytes("event"))
		switch event {
		case "login":
			if value.GetInt("code") == 0 {
				w.priWs.SetAuthSuccess()
			}
		case "error":
			w.Logger.Errorf("订阅失败 %s", string(msg))
			w.Cb.OnExit(fmt.Sprintf("订阅失败 %s", string(msg)))
		case "subscribe":
			if !w.priWs.IsAllSubSuccess() {
				w.priWs.SetSubSuccess("subscribe")
				if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
					w.Cb.OnWsReady(w.ExchangeName)
				}
			}
		}
		w.Logger.Infof("收到 pri ws 推送 %s", helper.BytesToString(msg))
	}

}

// 最重要的函数 处理交易所推送的信息
func (w *BitgetSpotWs) pubHandler(msg []byte, ts int64) {
	// 解析
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debugf("收到 pub ws 推送 %s", helper.BytesToString(msg))
	}
	if len(msg) == 4 && helper.BytesToString(msg) == "pong" {
		return
	}

	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		msg_str := helper.BytesToString(msg)
		if msg_str == "pong" {

		} else {
			w.Logger.Errorf("[%s]ws 解析msg失败 %v err:%s", w.ExchangeName, msg_str, err.Error())
		}
		return
	}

	// 消费信息
	if value.Exists("data") {
		channel := helper.BytesToString(value.GetStringBytes("arg", "channel"))
		datas := value.GetArray("data")
		switch channel {
		case "books1":
			for _, data := range datas {
				tsInEx, _ := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts")))
				asks := data.GetArray("asks")
				bids := data.GetArray("bids")

				if len(asks) == 0 || len(bids) == 0 {
					return
				}
				if w.TradeMsg.Ticker.Seq.NewerAndStore(tsInEx, ts) {

					_bids, _ := bids[0].Array()
					_bidPrice, _ := _bids[0].StringBytes()
					_bidQty, _ := _bids[1].StringBytes()

					_asks, _ := asks[0].Array()
					_askPrice, _ := _asks[0].StringBytes()
					_askQty, _ := _asks[1].StringBytes()
					w.TradeMsg.Ticker.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askPrice)))
					w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askQty)))
					w.TradeMsg.Ticker.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidPrice)))
					w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidQty)))
					w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
					w.Cb.OnTicker(ts)
				}
			}
		case "books15":
			for _, data := range datas {
				tsInEx := data.GetInt64("ts")
				asks := data.GetArray("asks")
				bids := data.GetArray("bids")

				if len(asks) == 0 || len(bids) == 0 {
					return
				}

				if w.TradeMsg.Ticker.Seq.NewerAndStore(tsInEx, ts) {

					_bids, _ := bids[0].Array()
					_bidPrice, _ := _bids[0].StringBytes()
					_bidQty, _ := _bids[1].StringBytes()

					_asks, _ := asks[0].Array()
					_askPrice, _ := _asks[0].StringBytes()
					_askQty, _ := _asks[1].StringBytes()

					w.TradeMsg.Ticker.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askPrice)))
					w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askQty)))
					w.TradeMsg.Ticker.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidPrice)))
					w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidQty)))
					w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())

					// onticker
					w.Cb.OnTicker(ts)
				}

				w.TradeMsg.Depth.Lock.Lock()
				w.TradeMsg.Depth.Bids, w.TradeMsg.Depth.Asks = w.TradeMsg.Depth.Bids[:0], w.TradeMsg.Depth.Asks[:0]
				for k := range bids {
					_bids, _ := bids[k].Array()
					_bidPrice, _ := _bids[0].StringBytes()
					_bidQty, _ := _bids[1].StringBytes()
					w.TradeMsg.Depth.Bids = append(w.TradeMsg.Depth.Bids,
						helper.DepthItem{Price: fastfloat.ParseBestEffort(helper.BytesToString(_bidPrice)), Amount: fastfloat.ParseBestEffort(helper.BytesToString(_bidQty))})
				}

				for k := range asks {
					_asks, _ := asks[k].Array()
					_askPrice, _ := _asks[0].StringBytes()
					_askQty, _ := _asks[1].StringBytes()
					w.TradeMsg.Depth.Asks = append(w.TradeMsg.Depth.Asks,
						helper.DepthItem{Price: fastfloat.ParseBestEffort(helper.BytesToString(_askPrice)), Amount: fastfloat.ParseBestEffort(helper.BytesToString(_askQty))})
				}
				w.TradeMsg.Depth.Lock.Unlock()
				// ondepth
				w.Cb.OnDepth(ts)
			}
		case "trade":
			if len(datas) < 1 {
				return
			}
			for _, v := range datas {
				ts, _ := fastfloat.ParseInt64(helper.BytesToString(v.GetStringBytes("0")))
				price := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("1")))
				amount := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("2")))
				side := helper.BytesToString(v.GetStringBytes("3"))
				if side == "buy" {
					w.TradeMsg.Trade.Update(helper.TradeSideBuy, amount, price)
				} else {
					w.TradeMsg.Trade.Update(helper.TradeSideSell, amount, price)
				}
				w.Cb.OnTrade(ts)
			}
		}
	} else {
		event := helper.BytesToString(value.GetStringBytes("event"))
		switch event {
		case "login":
		case "error":
			w.Logger.Errorf("订阅失败 %s", string(msg))
			w.Cb.OnExit(fmt.Sprintf("订阅失败 %s", string(msg)))
		case "subscribe":
			if !w.pubWs.IsAllSubSuccess() {
				w.pubWs.SetSubSuccess("subscribe")
				if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
					w.Cb.OnWsReady(w.ExchangeName)
				}
			}
		}
		w.Logger.Infof("收到 pub ws 推送 %s", helper.BytesToString(msg))
	}
}

// GetFee 获取费率
func (w *BitgetSpotWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.makerFee.Load(), Taker: w.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (w *BitgetSpotWs) WsLogged() bool {
	if w.priWs == nil {
		return false
	}
	return w.priWs.IsAuthSuccess()
}

func (w *BitgetSpotWs) GetPriWs() *ws.WS {
	return w.priWs
}
func (w *BitgetSpotWs) GetPubWs() *ws.WS {
	return w.pubWs
}
