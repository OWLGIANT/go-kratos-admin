package bitget_usdt_swap

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
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

const baseUrl = "/mix/v1/stream"

var (
	WS_HOST = "wss://ws.bitget.com"
)

var (
	wsPrivateHandyPool fastjson.ParserPool
	wsPublicHandyPool  fastjson.ParserPool
)

type BitgetUsdtSwapWs struct {
	base.FatherWs
	priWs          *ws.WS         // 继承ws
	pubWs          *ws.WS         // 继承ws
	connectOnce    sync.Once      // 连接一次
	colo           bool           // 是否colo
	symbolNoSuffix string         // 目标交易品种 交易所格式
	onMsgInterval  atomic.Int64   // 触发onMsg间隔
	takerFee       atomic.Float64 // taker费率
	makerFee       atomic.Float64 // maker费率
	rs             *BitgetUsdtSwapRs
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BitgetUsdtSwapWsUrl != "" {
		WS_HOST = cfg.BitgetUsdtSwapWsUrl
		params.Logger.Infof("bitget_usdt_swap ws 启用colo  %v", WS_HOST)
	}
	w := &BitgetUsdtSwapWs{}
	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)

	rs := NewRs(
		params,
		msg,
		info,
		helper.CallbackFunc{
			OnExit: cb.OnExit,
		})
	var ok bool
	w.rs, ok = rs.(*BitgetUsdtSwapRs)
	if !ok {
		w.Logger.Errorf(" rs 转换失败")
		w.Cb.OnExit(" rs 转换失败")
		return nil
	}
	w.rs.getExchangeInfo()
	w.UpdateExchangeInfo(w.rs.ExchangeInfoPtrP2S, w.rs.ExchangeInfoPtrS2P, cb.OnExit)

	w.symbolNoSuffix = strings.ReplaceAll(w.rs.Symbol, "_UMCBL", "")
	w.pubWs = ws.NewWS(WS_HOST+baseUrl, params.LocalAddr, params.ProxyURL, w.pubHandler, cb.OnExit, params.BrokerConfig)
	w.pubWs.SetHeader(http.Header{
		"User-Agent": []string{helper.UserAgentSlice[rand.Intn(len(helper.UserAgentSlice))]},
	})
	w.pubWs.SetSubscribe(w.subPub)
	w.pubWs.SetPingFunc(w.ping)
	w.pubWs.SetPingInterval(10)

	if w.BrokerConfig.NeedAuth {
		w.priWs = ws.NewWS(WS_HOST+baseUrl, params.LocalAddr, params.ProxyURL, w.priHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetHeader(http.Header{
			"User-Agent": []string{helper.UserAgentSlice[rand.Intn(len(helper.UserAgentSlice))]},
		})
		w.priWs.SetAuth(w.auth)
		w.priWs.SetSubscribe(w.subPri)
		w.priWs.SetPingFunc(w.ping)
		w.priWs.SetPingInterval(10)
	}

	w.AddWsConnections(w.priWs, w.pubWs)
	return w
}

// 专用ping方法
func (w *BitgetUsdtSwapWs) ping() []byte {
	return []byte("ping")
}

// 订阅函数 注意订阅顺序和时间间隔
func (w *BitgetUsdtSwapWs) subPub() error {
	// step 1 公有订阅 depth
	var p map[string]interface{}
	var e []map[string]interface{}
	var msg []byte
	//
	if w.BrokerConfig.NeedTicker {
		e = append(e, map[string]interface{}{
			"instType": "mc",
			"channel":  "books1",
			"instId":   w.symbolNoSuffix,
		})
	}
	// 订阅depth
	if w.BrokerConfig.WsDepthLevel > 0 {
		e = append(e, map[string]interface{}{
			"instType": "mc",
			"channel":  "books15",
			"instId":   w.symbolNoSuffix,
		})
	}
	//
	if w.BrokerConfig.NeedTrade {
		e = append(e, map[string]interface{}{
			"instType": "mc",
			"channel":  "trade",
			"instId":   w.symbolNoSuffix,
		})
	}
	//
	if w.BrokerConfig.NeedIndex {
		e = append(e, map[string]interface{}{
			"instType": "mc",
			"channel":  "ticker",
			"instId":   w.symbolNoSuffix,
		})
	}
	if len(e) > 0 {
		p = map[string]interface{}{
			"op":   "subscribe",
			"args": e,
		}
		msg, _ = json.Marshal(p)
		w.pubWs.SubWithRetry("subscribe", w.Cb.OnExit, func() []byte { return msg })
		w.Logger.Info("[bitget_usdt_swap] 发送订阅公共行情")
	} else {
		w.Logger.Warn("[bitget_usdt_swap] 没订阅任何行情")
	}
	return nil
}
func (w *BitgetUsdtSwapWs) subPri() error {
	if w.BrokerConfig.NeedAuth {
		e := []map[string]interface{}{
			// 订阅资产
			{
				"instType": "UMCBL",
				"channel":  "account",
				"instId":   "default",
			},
			// 订阅订单
			{
				"instType": "UMCBL",
				"channel":  "orders",
				"instId":   "default",
			},
			// 订阅仓位
			{
				"instType": "UMCBL",
				"channel":  "positions",
				"instId":   "default",
			}}
		p := map[string]interface{}{
			"op":   "subscribe",
			"args": e,
		}
		msg, _ := json.Marshal(p)
		w.priWs.SubWithRetry("subscribe", w.Cb.OnExit, func() []byte { return msg })
	}
	return nil
}
func (w *BitgetUsdtSwapWs) auth() error {
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
	auth, _ := json.Marshal(p)
	w.priWs.SendMessage(auth)
	return nil
}

// Run 准备ws连接 仅第一次调用时连接ws
func (w *BitgetUsdtSwapWs) Run() {
	w.connectOnce.Do(func() {
		var err error
		w.StopCPub, err = w.pubWs.Serve()
		if err != nil {
			w.Logger.Errorf("websocket serve error: %v", err)
			w.Cb.OnExit(fmt.Sprintf("websocket serve error: %v", err))
		}
		if w.BrokerConfig.NeedAuth {
			w.StopC, err = w.priWs.Serve()
			if err != nil {
				w.Logger.Errorf("websocket serve error: %v", err)
				w.Cb.OnExit(fmt.Sprintf("websocket serve error: %v", err))
			}
		}
	})
}

func (w *BitgetUsdtSwapWs) DoStop() {
	if w.StopCPub != nil {
		helper.CloseSafe(w.StopCPub)
	}
	if w.StopC != nil {
		helper.CloseSafe(w.StopC)
	}
	w.connectOnce = sync.Once{}
}

func (w *BitgetUsdtSwapWs) OptimistParseBBO(msg []byte, ts int64) bool {
	const _BBO_MIN_LEN = 120
	if len(msg) < _BBO_MIN_LEN {
		return false
	}

	// {"action":"snapshot","arg":{"instType":"mc","channel":"books1","instId":"XRPUSDT"},"data":[{"asks":[["2.4607","1768"]],"bids":[["2.4605","15854"]],"checksum":0,"ts":"1742796495288"}],"ts":1742796495290}

	c := helper.BytesToString(msg)
	idxStart := strings.Index(c[76:96], "as") //
	if idxStart < 0 {
		return false
	}
	idxStart = 76 + idxStart
	var res [4]float64

	offset := strings.Index(c[idxStart:], "[")
	idxStart += offset
	if c[idxStart+1] == ']' { // [] meanings no data
		return false
	}
	idxStart += 3 // [{" then data
	offset = strings.Index(c[idxStart:], "\"")
	res[0] = fastfloat.ParseBestEffort(c[idxStart : idxStart+offset])
	idxStart += offset + 3
	offset = strings.Index(c[idxStart:], "\"")
	res[1] = fastfloat.ParseBestEffort(c[idxStart : idxStart+offset])
	idxStart += offset

	offset = strings.Index(c[idxStart:], "b") //
	idxStart += offset
	if offset < 0 {
		return false
	}
	offset = strings.Index(c[idxStart:], "[")
	if c[idxStart+1] == ']' {
		return false
	}
	idxStart += offset + 3 // [{" then data
	offset = strings.Index(c[idxStart:], "\"")
	res[2] = fastfloat.ParseBestEffort(c[idxStart : idxStart+offset])
	idxStart += offset + 3 // ","xxxx"
	offset = strings.Index(c[idxStart:], "\"")
	res[3] = fastfloat.ParseBestEffort(c[idxStart : idxStart+offset])
	idxStart += offset

	offset = strings.Index(c[idxStart:], "t")
	if offset < 0 {
		return false
	}

	// ts":"xxxxxx"
	idxStart += offset + 5
	offset = strings.Index(c[idxStart:], "\"")
	tsEx, e := strconv.ParseInt(c[idxStart:idxStart+offset], 10, 64)
	if e != nil {
		return false
	}
	idxStart += offset
	offset = strings.Index(c[idxStart:], "t")
	if offset < 0 {
		return false
	}

	// ts":xxxxxx}
	// idxStart += offset + 4
	// offset = strings.Index(c[idxStart:], "}")
	// tsExOuter, e := strconv.ParseInt(c[idxStart:idxStart+offset], 10, 64) //pubHandler 用的data里面的ts
	// if e != nil {
	// 	return false
	// }

	if !w.TradeMsg.Ticker.Seq.NewerAndStore(tsEx, ts) {
		return true
	}
	w.TradeMsg.Ticker.Set(res[0], res[1], res[2], res[3])
	w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
	tsMs := time.Now().UnixMilli()
	// w.TradeMsg.Ticker.Delay.Store(tsMs - tsExOuter)
	w.TradeMsg.Ticker.DelayE.Store(tsMs - tsEx)
	w.Cb.OnTicker(ts)
	return true
}

// 最重要的函数 处理交易所推送的信息
func (w *BitgetUsdtSwapWs) pubHandler(msg []byte, ts int64) {
	// 解析
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debugf("收到 pub ws 消息:%s", string(msg))
	}

	if !helper.DEBUGMODE || (base.IsUtf && !base.SkipOptimistParseBBO) {
		if w.OptimistParseBBO(msg, ts) {
			w.OptimistBBOCnt++
			// if helper.DEBUGMODE {
			// w.Logger.Debugf("tryParseBBO success")
			// }
			return
		}
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
		channel := helper.BytesToString(helper.MustGetStringBytes(value, "arg", "channel"))
		datas := value.GetArray("data")
		switch channel {
		case "ticker":
			//限价信息 todo 感觉不需要
			if len(datas) < 1 {
				return
			}
			t := datas[0]
			markPrice := helper.MustGetFloat64FromBytes(t, "markPrice")
			indexPrice := helper.MustGetFloat64FromBytes(t, "indexPrice")
			if w.Cb.OnIndex != nil {
				w.Cb.OnIndex(ts, helper.IndexEvent{IndexPrice: indexPrice})
			}
			if w.Cb.OnMark != nil {
				w.Cb.OnMark(ts, helper.MarkEvent{MarkPrice: markPrice})
			}
		case "books15":
			for _, data := range datas {
				timeInEx := helper.MustGetInt64FromBytes(data, "ts")

				asks := data.GetArray("asks")
				bids := data.GetArray("bids")

				if len(asks) == 0 || len(bids) == 0 {
					return
				}

				if w.TradeMsg.Ticker.Seq.NewerAndStore(timeInEx, ts) {

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
					tsMsInEx := helper.MustGetInt64(data, "ts")
					if helper.DEBUGMODE {
						helper.MustMillis(tsMsInEx)
					}
					w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - tsMsInEx)
					w.Cb.OnTicker(ts)
					if helper.DEBUGMODE && base.IsUtf && base.CollectTCData {
						w.Cb.Collect(ts, string(msg), &w.TradeMsg.Ticker)
					}
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

		case "books1":
			if len(datas) < 1 {
				return
			}
			book := datas[0]
			timeInEx := helper.MustGetInt64FromBytes(book, "ts")
			if w.TradeMsg.Ticker.Seq.NewerAndStore(timeInEx, ts) {

				asks := book.GetArray("asks")
				bids := book.GetArray("bids")
				if len(asks) == 0 || len(bids) == 0 {
					return
				}
				// ap
				ask, _ := asks[0].Array()
				ap, _ := ask[0].StringBytes()
				aq, _ := ask[1].StringBytes()
				// bp
				bid, _ := bids[0].Array()
				bp, _ := bid[0].StringBytes()
				bq, _ := bid[1].StringBytes()

				_ap := fastfloat.ParseBestEffort(helper.BytesToString(ap))
				_bp := fastfloat.ParseBestEffort(helper.BytesToString(bp))
				w.TradeMsg.Ticker.Ap.Store(_ap)
				w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(aq)))
				w.TradeMsg.Ticker.Bp.Store(_bp)
				w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(bq)))
				w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
				// onticker
				if helper.DEBUGMODE {
					helper.MustMillis(timeInEx)
				}
				w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - timeInEx)
				w.Cb.OnTicker(ts)
				if helper.DEBUGMODE && base.IsUtf && base.CollectTCData {
					w.Cb.Collect(ts, string(msg), &w.TradeMsg.Ticker)
				}
			}

		case "trade":
			if len(datas) < 1 {
				return
			}
			for _, v := range datas {
				vals := v.GetArray()
				side := helper.BytesToString(vals[3].GetStringBytes())
				price := helper.MustGetFloat64FromBytes(vals[1])
				amount := helper.MustGetFloat64FromBytes(vals[2])
				if side == "buy" {
					w.TradeMsg.Trade.Update(helper.TradeSideBuy, amount, price)
				} else {
					w.TradeMsg.Trade.Update(helper.TradeSideSell, amount, price)
				}
			}
			w.Cb.OnTrade(ts)
		}
	} else {
		event := helper.BytesToString(value.GetStringBytes("event"))
		if event == "error" {
			w.Logger.Errorf("[%s]ws error event %s", w.ExchangeName, helper.BytesToString(msg))
			w.Cb.OnExit(fmt.Sprintf("订阅失败 %s", string(msg)))
		} else if event == "subscribe" {
			if !w.pubWs.IsAllSubSuccess() {
				w.pubWs.SetSubSuccess("subscribe")
				if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
					w.Cb.OnWsReady(w.ExchangeName)
				}
			}
		}
		w.Logger.Infof("收到 pub ws 推送 %s", helper.BytesToString(msg))
		return
	}
}

// 最重要的函数 处理交易所推送的信息
func (w *BitgetUsdtSwapWs) priHandler(msg []byte, ts int64) {
	// 解析
	if helper.DEBUGMODE {
		w.Logger.Debugf("收到 pri ws 消息:%s", string(msg))
	}
	p := wsPrivateHandyPool.Get()
	defer wsPrivateHandyPool.Put(p)
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
		channel := helper.BytesToString(helper.MustGetStringBytes(value, "arg", "channel"))
		datas := value.GetArray("data")
		switch channel {
		case "account":
			if helper.DEBUGMODE {
				w.Logger.Debugf("收到 ws account 推送 %s", helper.BytesToString(msg))
			}
			for _, data := range datas {
				marginCoin := helper.MustGetStringLowerFromBytes(data, "marginCoin")
				// if strings.EqualFold(marginCoin, ws.pair.Quote) {
				// {"action":"snapshot","arg":{"instType":"umcbl","channel":"account","instId":"default"},"data":[{"marginCoin":"USDT","locked":"14.61393666","available":"286.27224854","maxOpenPosAvailable":"271.65831188","maxTransferOut":"271.65831188","equity":"286.27224854","usdtEquity":"286.272248543947"}],"ts":1693558537275}
				// todo 代utf 测试验证

				// {"marginCoin":"USDT","locked":"0.00000000","available":"505.47209146","maxOpenPosAvailable":"498.73415146","maxTransferOut":"498.71345146",
				// "equity":"505.49279146","usdtEquity":"505.492791463760","coinDisplayName":"USDT"}],"ts":1722006550642}

				equity := helper.MustGetFloat64FromBytes(data, "equity") // 可能包含upnl，因为rs就是
				available := helper.MustGetFloat64FromBytes(data, "maxTransferOut")
				timeInEx := helper.MustGetInt64(value, "ts")

				e, ok := w.EquityNewerAndStore(marginCoin, timeInEx, ts, (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_Avail))
				if ok {
					e.Name = marginCoin
					e.TotalWithUpl = equity
					e.Avail = available

					w.Cb.OnEquityEvent(ts, *e)
				}
			}
		case "positions":
			if helper.DEBUGMODE {
				w.Logger.Debugf("收到 ws pos 推送 %s", helper.BytesToString(msg))
			}
			gotPosition := make(map[string]bool)

			var uTime int64
			for _, data := range datas {
				instId := helper.BytesToString(data.GetStringBytes("instId"))
				uTime = helper.MustGetInt64FromBytes(data, "uTime")
				price := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("averageOpenPrice")))
				size := fixed.NewS(helper.BytesToString(data.GetStringBytes("total")))
				side := helper.BytesToString(data.GetStringBytes("holdSide"))

				var longPos, shortPos fixed.Fixed
				var longAvg, shortAvg float64
				if side == "short" {
					shortPos = size
					shortAvg = price
				} else {
					longPos = size
					longAvg = price
				}
				if _, pos, ok := w.PosNewerAndStore(instId, uTime, time.Now().UnixNano()); ok {
					gotPosition[instId] = true
					pos.Lock.Lock()
					pos.ResetLocked()
					pos.LongPos = longPos
					pos.LongAvg = longAvg
					pos.ShortPos = shortPos
					pos.ShortAvg = shortAvg
					event := pos.ToPositionEvent()

					pos.Lock.Unlock()
					w.Cb.OnPositionEvent(0, event)
				}
				// 0仓会这样推送
				// {"action":"snapshot","arg":{"instType":"umcbl","channel":"positions","instId":"default"},"data":[{其他pair仓位...}],"ts":1705992442301}
			}
			// Empty when no pos
			w.CleanOthersPosImmediately(gotPosition, ts, w.Cb.OnPositionEvent)
		case "orders":
			// 生产环境可以注释以节省性能
			if helper.DEBUGMODE {
				w.Logger.Debugf("收到 ws order 推送 %s", helper.BytesToString(msg))
			}
			if len(datas) < 1 {
				return
			}
			// [{"accFillSz":"0","cTime":1722000134141,"clOrdId":"1722000135505","eps":"API","force":"normal","hM":"single_hold","instId":"BTCUSDT_UMCBL","lever":"10","lo
			// w":false,"notionalUsd":"60.5304","ordId":"1200721078620143620","ordType":"limit","orderFee":[{"feeCcy":"USDT","fee":"0"}],"posSide":"net","px":"60530.4",
			// "side":"buy","status":"new","sz":"0.001","tS":"buy_single","tdMode":"cross","tgtCcy":"USDT","uTime":1722000134141}],"ts":1722000134145}

			for _, data := range datas {
				instId := helper.BytesToString(helper.MustGetStringBytes(data, "instId"))
				status := helper.BytesToString(helper.MustGetStringBytes(data, "status"))
				orderId := string(helper.MustGetStringBytes(data, "ordId"))
				clientId := string(helper.MustGetStringBytes(data, "clOrdId"))

				info, ok := w.GetPairInfoBySymbol(instId)

				if !ok {
					w.Logger.Warnf("unknow pos of symbol, %s", instId)
					continue
				}

				event := helper.OrderEvent{Pair: info.Pair}
				event.OrderID = orderId
				event.ClientID = clientId
				switch status {
				case "new":
					event.Type = helper.OrderEventTypeNEW
				case "partial-fill":
					event.Type = helper.OrderEventTypePARTIAL
				case "full-fill", "cancelled", "canceled":
					event.Type = helper.OrderEventTypeREMOVE
				}
				event.Filled = fixed.NewS(helper.BytesToString(data.GetStringBytes("accFillSz"))) // 累计成交数量
				// 如果有成交
				if event.Filled.GreaterThan(fixed.ZERO) {
					// 获取成交价格 todo 什么情况下返回什么字段有规律吗？
					if data.Exists("fillPx") {
						event.FilledPrice = fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("fillPx"))) // 最新成交价格
					} else {
						event.FilledPrice = fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("px")))
					}
					// 计算最新一笔maker的手续费 检查是否异常 todo 暂时不需要此功能
					//execType := helper.BytesToString(data.GetStringBytes("execType"))
					//if execType == "M" {
					//	fillFeeCcy := helper.BytesToString(data.GetStringBytes("fillFeeCcy")) //最新一笔成交的手续费币种
					//	if fillFeeCcy == "USDT" {
					//		fillSz := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("fillSz")))   // 最新成交数量
					//		fillFee := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("fillFee"))) // 最新成交费
					//		event.CashFee = fixed.NewF(-fillFee)                                                       // 负为rebate 正为扣
					//		makerFeeRate := fixed.NewS(fmt.Sprintf("%.2f", -fillFee/(fillSz*event.FilledPrice))).Float()
					//		ws.makerFee.Store(makerFeeRate * 1e4)
					//		// vip0 maker手续费万2 vip1 maker手续费万0.6
					//		if makerFeeRate > 0.00008 {
					//			//b.Cb.OnExit(fmt.Sprintf("bitget合约maker手续费异常%.6f", makerFeeRate))
					//			now := time.Now().UnixMilli()
					//			if now-ws.onMsgInterval.Load() > 900000 {
					//				ws.onMsgInterval.Store(now)
					//				ws.Cb.OnMsg(fmt.Sprintf("%v bitget合约maker手续费异常%.6f", ws.BrokerConfig.Name, makerFeeRate))
					//			}
					//		}
					//	}
					//}
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
				switch helper.MustGetShadowStringFromBytes(data, "force") {
				case "ioc":
					event.OrderType = helper.OrderTypeIoc
				case "post_only":
					event.OrderType = helper.OrderTypePostOnly
				}
				event.ReceivedTsNs = ts
				w.Cb.OnOrder(ts, event)
			}
		}
	} else {
		defer func() {
			w.Logger.Infof("收到 pri ws 消息:%s", string(msg))
		}()
		event := helper.BytesToString(value.GetStringBytes("event"))
		if event == "login" && value.GetInt("code") == 0 {
			w.priWs.SetAuthSuccess()
		} else if event == "error" {
			w.Logger.Errorf("[%s]ws error event %s", w.ExchangeName, helper.BytesToString(msg))
			w.Cb.OnExit(fmt.Sprintf("订阅失败 %s", string(msg)))
			return
		} else if event == "subscribe" {
			if !w.priWs.IsAllSubSuccess() {
				w.priWs.SetSubSuccess("subscribe")
				if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
					w.Cb.OnWsReady(w.ExchangeName)
				}
			}
		}
		return
	}
}

// GetFee 获取费率
func (w *BitgetUsdtSwapWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.makerFee.Load(), Taker: w.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (w *BitgetUsdtSwapWs) WsLogged() bool {
	if w.priWs == nil {
		return false
	}
	return w.priWs.IsAuthSuccess()
}

func (w *BitgetUsdtSwapWs) GetPriWs() *ws.WS {
	return w.priWs
}
func (w *BitgetUsdtSwapWs) GetPubWs() *ws.WS {
	return w.pubWs
}
