package binance_spot

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/base/base_orderbook"
	"actor/broker/base/orderbook_mediator"
	"actor/broker/brokerconfig"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

var (
	BaseWsUrl = "wss://stream.binance.com:9443/ws"
)

var (
	wsPublicHandyPool fastjson.ParserPool

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

type BinanceSpotWs struct {
	base.FatherWs
	pubWs            *ws.WS        // 继承ws
	priWs            *ws.WS        // 继承ws
	connectOnce      sync.Once     // 连接一次
	authOnce         sync.Once     // 鉴权一次
	id               int64         // 所有发送的消息都要唯一id
	stopCPri         chan struct{} // 接受停机信号
	colo             bool          // 是否colo
	baseWsUrl        string
	listenKey        atomic.String
	getListenKeyTime atomic.Int64
	rs               *BinanceSpotRs
	takerFee         atomic.Float64 // taker费率
	makerFee         atomic.Float64 // maker费率
	stopCtx          context.Context
	stopFunc         context.CancelFunc
	orderbook        *base_orderbook.Orderbook
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	// 判断colo
	baseWsUrl := BaseWsUrl
	cfg := brokerconfig.BrokerSession()
	if cfg.BinanceSpotWsUrl != "" {
		baseWsUrl = cfg.BinanceSpotWsUrl
		params.Logger.Infof("binance_spot ws启用colo  %v", baseWsUrl)
	}
	//if cfg.BinanceSpotProxy != "" {
	//	proxies := strings.Split(cfg.BinanceSpotProxy, ",")
	//	rand.Seed(time.Now().Unix())
	//	i := rand.Int31n(int32(len(proxies)))
	//	proxy := proxies[i]
	//	params.ProxyURL = proxy
	//	w.Logger.Infof("binance spot ws 启用代理: %v", params.ProxyURL)
	//}
	//
	w := &BinanceSpotWs{
		baseWsUrl: baseWsUrl,
	}
	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)
	rs := NewRs(
		params,
		msg,
		info,
		helper.CallbackFunc{
			OnExit: cb.OnExit,
		})
	// 底层有自动ping pong 不需要专用ping pong函数
	//w.ws.SetPingFunc(w.ping)
	var ok bool
	w.rs, ok = rs.(*BinanceSpotRs)
	if !ok {
		w.Logger.Errorf(" rs 转换失败")
		w.Cb.OnExit(" rs 转换失败")
		return nil
	}
	w.rs.getExchangeInfo()
	w.UpdateExchangeInfo(w.rs.ExchangeInfoPtrP2S, w.rs.ExchangeInfoPtrS2P, cb.OnExit)

	w.stopCtx, w.stopFunc = context.WithCancel(context.Background())
	w.pubWs = ws.NewWS(w.baseWsUrl, params.LocalAddr, params.ProxyURL, w.publicHandler, cb.OnExit, params.BrokerConfig)
	w.pubWs.SetSubscribe(w.SubscribePublic)
	w.pubWs.SetPongFunc(pong)
	w.pubWs.SetPingInterval(90)

	connectionJudger := func(firstMatched bool, seq int64, s *base_orderbook.Slot) bool {
		return seq == s.ExPrevLastId
	}
	firstMatchJudger := func(snapSlot *base_orderbook.Slot, slot *base_orderbook.Slot) bool {
		return slot.ExFirstId <= snapSlot.ExPrevLastId && slot.ExLastId >= snapSlot.ExPrevLastId
	}

	w.orderbook = base_orderbook.NewOrderbook(w.ExchangeName.String(), params.Pairs[0].String(), firstMatchJudger, connectionJudger, w.rs.getOrderbookSnap, base_orderbook.SnapFetchType_Rs, cb.OnExit, &w.TradeMsg.Depth)
	w.orderbook.SetDisableAutoUpdateDepth(params.DisableAutoUpdateDepth)
	mediator := orderbook_mediator.NewOrderbookMediator()
	w.orderbook.SetMediator(mediator)
	w.TradeMsg.Orderbook = mediator
	if params.NeedAuth {
		w.priWs = ws.NewWS(fmt.Sprintf("%s/%s", w.baseWsUrl, w.listenKey.Load()), params.LocalAddr, params.ProxyURL, w.privateHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetSubscribe(w.SubscribePrivate)
		w.priWs.SetPongFunc(pong)
		w.priWs.SetPingInterval(90)
	}

	w.AddWsConnections(w.pubWs, w.priWs)
	return w
}

func pong(ping []byte) []byte {
	return ping
}

func (w *BinanceSpotWs) SubscribePublic() error {
	// 公有订阅
	if w.BrokerConfig.NeedTicker {
		w.id++
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []interface{}{fmt.Sprintf("%s@bookTicker", strings.ToLower(w.Symbol))}, // bbo
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
		//
	}
	if w.BrokerConfig.WsDepthLevel > 0 {
		// 小档全量更快
		finalWsDepthLevel, ok := helper.SuitableLevel([]int{5, 10, 20}, w.BrokerConfig.WsDepthLevel)
		if ok {
			w.UsingDepthType = base.DepthTypeSnapshot
			w.id++
			p := map[string]interface{}{
				"method": "SUBSCRIBE",
				"params": []interface{}{fmt.Sprintf("%s@depth%d", strings.ToLower(w.Symbol), finalWsDepthLevel)},
				"id":     w.id,
			}
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
		} else {
			w.UsingDepthType = base.DepthTypePartial
			w.orderbook.SetDepthSubLevel(w.BrokerConfig.WsDepthLevel)
			w.pubWs.SetDontRoutine()
			w.id++
			p := map[string]interface{}{
				"method": "SUBSCRIBE",
				"params": []interface{}{fmt.Sprintf("%s@depth@100ms", strings.ToLower(w.Symbol))},
				"id":     w.id,
			}
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", err)
			}
			w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
		}
	}
	//
	if w.BrokerConfig.Need1MinKline {
		w.id++
		subList := []interface{}{}
		for _, pair := range w.Pairs {
			symbol := w.PairToSymbol(&pair)
			subList = append(subList, fmt.Sprintf("%s@kline_1m", strings.ToLower(symbol)))
		}
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": subList,
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}
	//
	if w.BrokerConfig.NeedTrade {
		w.id++
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []interface{}{fmt.Sprintf("%s@aggTrade", strings.ToLower(w.Symbol))},
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}
	return nil
}

func (w *BinanceSpotWs) getListenKey() {
	// 获取 listenkey
	for i := 0; i < 5; i++ {
		key, err := w.rs.GenerateListenKey()
		if err != nil {
			w.Logger.Infof("获取到listenkey失败: %v", err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		w.Logger.Infof("获取到listenkey: %v", key)
		w.listenKey.Store(key)
		w.getListenKeyTime.Store(time.Now().UnixMilli())
		w.priWs.SetWsUrl(fmt.Sprintf("%s/%s", w.baseWsUrl, w.listenKey.Load()))
		break
	}
}

func (w *BinanceSpotWs) deleteListenKey() {
	// 获取 listenkey
	for i := 0; i < 5; i++ {
		targitKey := w.listenKey.Load()
		if targitKey != "" {
			err := w.rs.DeleteListenKey(targitKey)
			if err != nil {
				w.Logger.Infof("删除listenkey失败: %v", err)
				time.Sleep(time.Millisecond * 500)
				continue
			} else {
				w.Logger.Infof("关闭listenkey成功: %v", targitKey)
				break
			}
		}

	}
}

// SubscribePrivate binance 通过listenkey连接 不需要订阅private
func (w *BinanceSpotWs) SubscribePrivate() error {
	if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
		w.Cb.OnWsReady(w.ExchangeName)
	}
	return nil
}

// Run 准备ws连接 仅第一次调用时连接ws
func (w *BinanceSpotWs) Run() {
	w.connectOnce.Do(func() {
		if w.BrokerConfig.NeedTicker || w.BrokerConfig.NeedTrade || w.BrokerConfig.WsDepthLevel > 0 {
			var err1 error
			w.StopCPub, err1 = w.pubWs.Serve()
			if err1 != nil {
				w.Cb.OnExit("binance spot public ws 连接失败")
			}
		}
		if w.BrokerConfig.NeedAuth {
			var err2 error
			w.getListenKey()
			w.stopCPri, err2 = w.priWs.Serve()
			if err2 != nil {
				w.Cb.OnExit("binance spot private ws 连接失败")
				return
			}
			go func() {
				// 随机打散, 避免同一台机器太多实例同时启动
				waitSec := rand.Intn(100)
				ticker := time.NewTicker(time.Second * time.Duration(waitSec+300))
				for {
					select {
					case <-w.stopCtx.Done():
						return
					case <-ticker.C:
						w.getListenKeyTime.Store(time.Now().Unix())
						w.Logger.Infof("触发listenkey续期操作")
						w.rs.KeepListenKey(w.listenKey.Load())
					}
				}
			}()
		}

	})
}

func (w *BinanceSpotWs) DoStop() {
	if w.BrokerConfig.NeedTicker || w.BrokerConfig.NeedTrade || w.BrokerConfig.WsDepthLevel > 0 {
		if w.StopCPub != nil {
			helper.CloseSafe(w.StopCPub)
		}
	}
	if w.BrokerConfig.NeedAuth {
		if w.stopCPri != nil {
			helper.CloseSafe(w.stopCPri)
		}
	}
	w.stopFunc()
	w.connectOnce = sync.Once{}
}

// 最重要的函数 处理交易所推送的信息
func (w *BinanceSpotWs) publicHandler(msg []byte, ts int64) {
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		w.Logger.Errorf("Binance Spot ws解析msg出错 %v err:%v", helper.BytesToString(msg), err)
		return
	}
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Info(fmt.Sprintf("收到binance ws推送 %v", value))
	}
	// 更新本地数据 并触发相应回调
	// bookticker 频道的 u 字段 和 depth 频道的 lastUpdateId 字段 是同一个字段
	if value.Exists("A") && value.Exists("B") {
		t := value.GetInt64("u")

		if w.TradeMsg.Ticker.Seq.NewerAndStore(t, ts) {
			//w.Logger.Errorf(fmt.Sprintf("收到binance spot ws bbo 推送 并触发onTicker %v", value))

			w.TradeMsg.Ticker.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("a"))))
			w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("A"))))
			w.TradeMsg.Ticker.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("b"))))
			w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("B"))))
			w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
			w.Cb.OnTicker(ts)
			if helper.DEBUGMODE && base.IsUtf && base.CollectTCData {
				w.Cb.Collect(ts, string(msg), &w.TradeMsg.Ticker)
			}
		}
	} else if value.Exists("e") {
		channel := helper.BytesToString(value.GetStringBytes("e"))
		switch channel {
		case "depthUpdate":
			// w.snapShotLock.Lock()
			// defer w.snapShotLock.Unlock()
			// While listening to the stream, each new event's U should be equal to the previous event's u+1.
			// "E": 123456789,     	// 事件时间
			// "U": 157,           	// 从上次推送至今新增的第一个 update Id
			// "u": 160,           	// 从上次推送至今新增的最后一个 update Id
			bids0 := value.GetArray("b")
			asks0 := value.GetArray("a")
			bidLen := len(bids0)
			slot := w.orderbook.GetFreeSlot(bidLen+len(asks0), ts)
			slot.ExTsMs = helper.MustGetInt64(value, "E")
			slot.ExFirstId = helper.MustGetInt64(value, "U")
			slot.ExLastId = helper.MustGetInt64(value, "u")
			slot.ExPrevLastId = slot.ExFirstId - 1
			for _, v := range bids0 {
				s := v.GetArray()
				p := helper.MustGetFloat64FromBytes(s[0])
				a := helper.MustGetFloat64FromBytes(s[1])
				slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
			}
			slot.AskStartIdx = bidLen
			for _, v := range asks0 {
				s := v.GetArray()
				p := helper.MustGetFloat64FromBytes(s[0])
				a := helper.MustGetFloat64FromBytes(s[1])
				slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
			}

			if w.orderbook.InsertSlot(slot) {
				w.Cb.OnDepth(ts)
			}

		case "aggTrade", "trade":
			price := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("p")))
			qty := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("q")))

			// https://binance-docs.github.io/apidocs/futures/en/#live-subscribing-unsubscribing-to-streams
			// Is the buyer the market maker?
			// TODO 跪求各位开发 千万别把这个改错了 非常容易出错 一定要看清楚 改之前一定要问我 @thousandquant
			if value.GetBool("m") {
				w.TradeMsg.Trade.Update(helper.TradeSideSell, qty, price)
			} else {
				w.TradeMsg.Trade.Update(helper.TradeSideBuy, qty, price)
			}
			w.Cb.OnTrade(ts)
		case "kline":
			// 每分钟K线闭合时回调
			kline := value.Get("k")
			closed := kline.GetBool("x")
			if !closed {
				return
			}
			symbol := helper.MustGetStringFromBytes(value, "s")
			tsEx := helper.MustGetInt64(value, "E")
			start := helper.MustGetInt64(kline, "t")
			end := helper.MustGetInt64(kline, "T")
			open := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("o")))
			close := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("c")))
			high := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("h")))
			low := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("l")))
			volume := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("v"))) //qty
			takerBuyVolume := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("V")))
			turnover := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("q"))) //volume
			takerBuyTurnover := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("Q")))
			numberOfTrades := helper.MustGetInt(kline, "n")
			w.Cb.OnKline(ts, helper.NewKline(symbol, start, end, tsEx, open, close, high, low, float64(numberOfTrades), 0, takerBuyVolume, volume-takerBuyVolume, takerBuyTurnover, turnover-takerBuyTurnover, ts))
		}
	} else if value.Exists("bids") {
		t := value.GetInt64("lastUpdateId")

		bids := value.GetArray("bids")
		asks := value.GetArray("asks")

		if len(bids) == 0 || len(asks) == 0 {
			return
		}

		if w.TradeMsg.Ticker.Seq.NewerAndStore(t, ts) {

			_bids, _ := bids[0].Array()
			if len(_bids) > 1 {
				_bidPrice, _ := _bids[0].StringBytes()
				_bidQty, _ := _bids[1].StringBytes()
				w.TradeMsg.Ticker.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidPrice)))
				w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_bidQty)))
			} else {
				return
			}

			_asks, _ := asks[0].Array()
			if len(_asks) > 1 {
				_askPrice, _ := _asks[0].StringBytes()
				_askQty, _ := _asks[1].StringBytes()
				w.TradeMsg.Ticker.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askPrice)))
				w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(_askQty)))
			} else {
				return
			}

			w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())

			// onticker
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
	} else {
		if value.Exists("id") {
			id := helper.MustGetInt(value, "id")
			if value.Get("result").Type() == fastjson.TypeNull {
				w.pubWs.SetSubSuccess(helper.Itoa(id))
			}
			if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
		//除了订阅不应该有东西来这里
		w.Logger.Infof(fmt.Sprintf("收到binance ws推送 %v", value))
	}
}

func (w *BinanceSpotWs) privateHandler(msg []byte, ts int64) {
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		w.Logger.Errorf("Binance spot ws解析msg出错 err:%v", err)
		return
	}
	// log
	if helper.DEBUGMODE {
		w.Logger.Debugf("收到binance pri ws推送 %v", value)
	}
	// 解析信息
	if value.Exists("e") {
		e := helper.BytesToString(value.GetStringBytes("e"))
		switch e {
		case "executionReport":
			symbol := helper.MustGetShadowStringFromBytes(value, "s")
			info, ok := w.GetPairInfoBySymbol(symbol)
			if !ok {
				break
			}
			status := helper.BytesToString(value.GetStringBytes("X"))
			order := helper.OrderEvent{Pair: info.Pair}
			if status == "NEW" {
				order.Type = helper.OrderEventTypeNEW
			} else if status == "CANCELED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else if status == "FILLED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else if status == "EXPIRED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else {
				break
			}
			filledPrice := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("L")))
			filled := fixed.NewS(helper.BytesToString(value.GetStringBytes("z")))
			var cid string
			if helper.BytesToString(value.GetStringBytes("C")) == "" {
				cid = string(value.GetStringBytes("c"))
			} else {
				cid = string(value.GetStringBytes("C"))
			}
			oid := fmt.Sprint(value.GetInt64("i"))
			order.OrderID = oid
			order.ClientID = cid
			order.Filled = filled
			order.FilledPrice = filledPrice
			if filled.GreaterThan(fixed.ZERO) {
				// 默认 pri order 推送比 pub推送快 因为没这个字段 所以采用bookticker的u字段自增1
				order.ID = w.TradeMsg.Ticker.Seq.Ex.Load() + 1
			}
			switch helper.MustGetShadowStringFromBytes(value, "S") {
			case "BUY":
				order.OrderSide = helper.OrderSideKD
			case "SELL":
				order.OrderSide = helper.OrderSideKK
			default:
				w.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(value, "S"))
				return
			}
			switch helper.MustGetShadowStringFromBytes(value, "o") {
			case "LIMIT":
				order.OrderType = helper.OrderTypeLimit
			case "MARKET":
				order.OrderType = helper.OrderTypeMarket
			case "LIMIT_MAKER":
				order.OrderType = helper.OrderTypePostOnly
			}
			// 必须在type后面。忽略其他类型
			switch helper.MustGetShadowStringFromBytes(value, "f") {
			case "IOC":
				order.OrderType = helper.OrderTypeIoc
			}
			order.ReceivedTsNs = ts
			w.Cb.OnOrder(ts, order)

		case "outboundAccountPosition":
			// 更新资金
			B := value.GetArray("B")
			sequence := helper.MustGetInt64(value, "u")
			for _, v := range B {
				a := helper.Trim10Multiple(helper.MustGetStringLowerFromBytes(v, "a"))
				var fieldsSet helper.FieldsSet_T
				fieldsSet = (helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail)
				if e, ok := w.EquityNewerAndStore(a, sequence, ts, fieldsSet); ok {
					e.Avail = fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("f")))
					e.TotalWithoutUpl = e.Avail + fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("l")))
					w.Cb.OnEquityEvent(ts, *e)
				}
			}
		case "listenKeyExpired", "expired":
			w.Logger.Warnf("触发listenkey过期，产生新listenkey")
			// 停掉老的priWs
			if w.stopCPri != nil {
				helper.CloseSafe(w.stopCPri)
			}
			w.deleteListenKey()
			time.Sleep(time.Second * 10)
			// 重启priWs
			w.priWs = nil
			w.priWs = ws.NewWS(w.baseWsUrl, w.BrokerConfig.LocalAddr, w.BrokerConfig.ProxyURL, w.privateHandler, w.Cb.OnExit, w.BrokerConfig.BrokerConfig)
			w.priWs.SetSubscribe(w.SubscribePrivate)
			// 更新key
			w.getListenKey()
			var err error
			w.stopCPri, err = w.priWs.Serve()
			if err != nil {
				w.Cb.OnExit("binance spot private ws 连接失败")
			}
		default:
			w.Logger.Warnf("未知事件%s", helper.BytesToString(msg))
		}
	}
	// 判断是否需要续期
	now := time.Now().UnixMilli()
	if now-w.getListenKeyTime.Load() > 1800000 {
		w.getListenKeyTime.Store(now)
		w.Logger.Debugf("触发listenkey续期操作")
		go func() {
			w.rs.KeepListenKey(w.listenKey.Load())
		}()
	}
}

// GetFee 获取费率
func (w *BinanceSpotWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.makerFee.Load(), Taker: w.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (rs *BinanceSpotWs) WsLogged() bool {
	return true
}

func (ws *BinanceSpotWs) GetPriWs() *ws.WS {
	return ws.priWs
}
func (ws *BinanceSpotWs) GetPubWs() *ws.WS {
	return ws.pubWs
}
