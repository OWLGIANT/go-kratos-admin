package okx_spot

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	"actor/tools"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"go.uber.org/atomic"
)

type OkxSpotRs struct {
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyDoGetPriceLimit
	base.DummyDoForSpot
	base.DummyOrderAction
	priWs       *ws.WS         // 继承ws
	connectOnce sync.Once      // 连接一次
	id          atomic.Int64   // 所有发送的消息都要唯一id
	lk          sync.Mutex     //
	stopCPri    chan struct{}  // 接受停机信号
	colo        bool           // 是否colo
	priWsUrl    string         // 私有ws地址
	restUrl     string         // rest地址
	client      *OkxSpotClient // 用来查询各种信息
	logged      bool           // 是否已登录, 登录完成才可以交易
	isAlive     bool           // 是否已开启且尚未关闭, 避免重复关闭
	//
	takerFee              atomic.Float64 // taker费率
	makerFee              atomic.Float64 // maker费率
	exState               atomic.Int32
	latestCallBeforeTrade int64 // 最近调用BeforeTrade时间，清仓时会多次调用。这里用于处理登录逻辑
	assetStr              string
}

func (rs *OkxSpotRs) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
	code := helper.BytesToString(value.GetStringBytes("code"))
	if code == "0" {
		rs.ReqSucc(base.FailNumActionIdx_AllReq)
		return true
	} else {
		if code == "50026" || code == "50013" || code == "52912" {
			// https://web3.okx.com/build/docs/waas/dex-error-code
			// 50026	500	System error. Try again later
			// 50013	429	Systems are busy. Please try again later.
			// 52912	500	Server timeout
			rs.Cb.OnExchangeDown()
		}
		rs.Logger.Errorf("请求失败 req: %s %v. rsp: %s", url, params, string(value.String()))
		rs.ReqFail(base.FailNumActionIdx_AllReq)
		return false
	}
}

func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	coloFlag := checkColo(params)
	rs := &OkxSpotRs{
		colo:     coloFlag,
		priWsUrl: okxWsPriUrl,
		restUrl:  okxRestUrl,
	}
	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, info, cb)
	rs.assetStr = strings.Join(rs.AssetsUpper, ",")

	if !params.BanRsWs {
		rs.priWs = ws.NewWS(rs.priWsUrl, params.LocalAddr, params.ProxyURL, rs.priHandler, cb.OnExit, params.BrokerConfig)
		rs.priWs.SetPingFunc(rs.ping)
		rs.priWs.SetPingInterval(10)
		rs.priWs.SetSubscribe(rs.subPri)
	}

	rs.client = NewClient(params, cb)

	return rs
}

func (rs *OkxSpotRs) run() {
	rs.isAlive = true

	if rs.priWs != nil && !rs.priWs.IsConnected() {
		var err error
		rs.stopCPri, err = rs.priWs.Serve()
		if err != nil {
			rs.Cb.OnExit("private ws 连接失败")
		}
	}
}

func (rs *OkxSpotRs) stop() {
	if rs.isAlive {
		rs.isAlive = false
		helper.CloseSafe(rs.stopCPri)
	}
}

func (rs *OkxSpotRs) ping() []byte {
	return []byte("ping")
}

func (rs *OkxSpotRs) subPri() error {
	// login first
	rs.logged = false
	msg := getWsLogin(&rs.BrokerConfig)
	rs.priWs.SendMessage(msg)
	// 实际登录注册信息被放在后面了, 在 realSubscribe 里面
	go func(latestCallBeforeTrade int64) {
		for i := 0; i < 15; i++ {
			if rs.logged {
				time.Sleep(time.Second) // 交易所会反应慢
				// rs.realSubscribe() // okx限制一直channel 20 connc，下单ws不需要账户信息
				break
			}
			time.Sleep(time.Second)
			// okx不允许重复发送登录
			// if i%2 == 0 {
			// msg := getWsLogin(rs.params)
			// rs.priWs.SendMessage(msg)
			// }
		}
		if !rs.logged {
			// BeforeTrade会在清仓过程中多次被调用，这里只在最后真的失败时退出
			if latestCallBeforeTrade == rs.latestCallBeforeTrade {
				rs.Cb.OnExit("okx usdt swap 登录失败")
			}
			return
		}
	}(rs.latestCallBeforeTrade)
	return nil
}

func (rs *OkxSpotRs) realSubscribe() {
	// 用 pri ws推送，不用这里
	// p := map[string]interface{}{
	// 	"op": "subscribe",
	// 	"args": []interface{}{
	// 		map[string]interface{}{
	// 			"channel": "account",
	// 			"ccy":     strings.ToUpper(rs.pair.Quote),
	// 		},
	// 	},
	// }
	// msg, err := json.Marshal(p)
	// if err != nil {
	// 	rs.Logger.Errorf("[ws][%s] json encode error , %s", rs.ExchangeName.String(), err)
	// }
	// rs.priWs.SendMessage(msg)

}

var mutex sync.Mutex

// 处理私有ws的处理器
func (rs *OkxSpotRs) priHandler(msg []byte, ts int64) {
	if rs.exState.Load() == helper.ExStateAfterTrade {
		return
	}
	if helper.DEBUGMODE {
		rs.Logger.Debug(fmt.Sprintf("收到okx pri ws推送 %s", string(msg)))
	}
	// 解析
	p := wsPrivateHandyPool.Get()
	defer wsPrivateHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) == "pong" {
			return
		}
		rs.Logger.Errorf("ws解析msg出错 err:%v", err)
		return
	}
	if value.Exists("data") {
		if value.Exists("op") { // 收到ws主动请求的回报，需要处理
			op := helper.BytesToString(value.GetStringBytes("op"))
			switch op {
			case "order":
				for _, data := range value.GetArray("data") {
					var order helper.OrderEvent
					order.Type = helper.OrderEventTypeNEW
					order.OrderID = string(data.GetStringBytes("ordId"))
					order.ClientID = string(data.GetStringBytes("clOrdId"))
					underscoreIndex := strings.IndexByte(order.ClientID, helper.CID_BASE_COIN_SPLIT_OKX)
					if underscoreIndex == -1 {
						rs.Logger.Error("cid format error", order.ClientID)
					}
					baseCoin := order.ClientID[:underscoreIndex]
					order.Pair = helper.NewPair(baseCoin, rs.Pair.Quote, "")
					scode := helper.BytesToString(data.GetStringBytes("sCode"))
					switch scode {
					case "0":
						order.Type = helper.OrderEventTypeNEW
						rs.ReqSucc(base.FailNumActionIdx_Place)
					default:
						order.Type = helper.OrderEventTypeREMOVE // 这里可以用 OrderEventTypeREMOVE 因为是交易所返回的err 如果是本地产生的err 就用 OrderEventTypeERROR
						order.ErrorReason = string(data.GetStringBytes("sMsg"))
					}
					rs.Cb.OnOrder(ts, order)
				}
			case "cancel-order":
				// 让 sub pri ws 处理
				// code := helper.BytesToString(value.GetStringBytes("code"))
				// switch code {
				// case "0":
				// default:
				// datas := value.GetArray("data")
				// if len(datas) > 0 {
				// data := datas[0]
				// scode := helper.BytesToString(data.GetStringBytes("sCode"))
				// switch scode {
				// case "51410":
				// Cancellation failed as the order is already under cancelling status
				// rs.Logger.Debugf(fmt.Sprintf("rest撤单中 %v", value))
				// return
				// }
				// }
				// rs.Logger.Warnf(fmt.Sprintf("rest撤单失败 %v", value))
				// }
			case "amend-order":
				for _, data := range value.GetArray("data") {
					var order helper.OrderEvent
					order.OrderID = string(data.GetStringBytes("ordId"))
					order.ClientID = string(data.GetStringBytes("clOrdId"))
					underscoreIndex := strings.IndexByte(order.ClientID, helper.CID_BASE_COIN_SPLIT_OKX)
					if underscoreIndex == -1 {
						rs.Logger.Error("cid format error", order.ClientID)
					}
					base := order.ClientID[:underscoreIndex]
					order.Pair = helper.NewPair(base, rs.Pair.Quote, "")
					scode := helper.BytesToString(data.GetStringBytes("sCode"))
					switch scode {
					case "0":
						order.Type = helper.OrderEventTypeAmendSucc
					default:
						order.Type = helper.OrderEventTypeAmendFail
						order.ErrorReason = string(data.GetStringBytes("sMsg"))
						// case "51513":
						// Number of modification requests that are currently in progress for an order cannot exceed 3.
						// 短时间内太多改单命令，debug即可。也不能完全不重视，毕竟占用限频，能规避还是规避好
						// rs.Logger.Debugf(fmt.Sprintf("rest改单过频 %v", value))
						// 尝试撤单
						// return
						// case "51510":
						// Modification failed as the order has been completed.
						// 订单已经完成了，给一个info吧, 后续也可以处理处理，不过这里信息太少了，光知道完成，是成交还是撤单，成交价格数量都不知道
						// rs.Logger.Infof(fmt.Sprintf("rest改单失效 %v", value))
						// return
					}
					rs.Cb.OnOrder(ts, order)
				}
			}

		} else if value.Exists("arg") {
			ch := value.Get("arg")
			op := helper.BytesToString(ch.GetStringBytes("channel"))
			switch op {
			case "account":
				//multi := w.pairInfo.Multi.Float()
				//datas := value.GetArray("data")
				//for _, data := range datas {
				//	details := data.GetArray("details")
				//	for _, v := range details {
				//		if helper.BytesToString(v.GetStringBytes("ccy")) == strings.ToUpper(w.pair.Quote) {
				//			t := &w.tradeMsg.Equity
				//			t.Lock.Lock()
				//			t.Cash = fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("disEq")))
				//			t.CashFree = fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("availEq")))
				//			t.Lock.Unlock()
				//			w.Cb.OnEquity()
				//			break // 此时不需要再循环了
				//		}
				//	}
				//}
			}
		}
	} else if value.Exists("event") { // event常见于事件汇报，一般都可以忽略，只需要看看是否有error
		e := helper.BytesToString(value.GetStringBytes("event"))
		switch e {
		case "error":
			rs.Logger.Errorf("[rest pri ws][%s] error , %v", rs.ExchangeName.String(), value)
		case "login":
			rs.logged = true
			rs.Cb.OnWsReady(rs.ExchangeName)
		}
	}
}

func (rs *OkxSpotRs) cancelAllOrders(only bool) {
	if only {
		rs.DoCancelPendingOrders(rs.Symbol)
	} else {
		rs.DoCancelPendingOrders("")
	}
}

func (rs *OkxSpotRs) DoCancelPendingOrders(symbol string) (err helper.ApiError) {
	params := map[string]interface{}{"instType": "SPOT"}
	if symbol != "" {
		params["instId"] = symbol // 空就是全部撤单
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/trade/orders-pending", params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("获取pending订单失败 回报错误 %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, "/api/v5/trade/orders-pending") {
			return
		}
		var msg map[string]interface{}
		datas := value.GetArray("data")
		for _, data := range datas {
			msg = map[string]interface{}{
				"instId": string(data.GetStringBytes("instId")),
				"ordId":  string(data.GetStringBytes("ordId")),
			}
			_, err := rs.client.post("/api/v5/trade/cancel-order", msg, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
				p := handyPool.Get()
				defer handyPool.Put(p)
				value, handlerErr := p.ParseBytes(respBody)
				if handlerErr != nil {
					rs.Logger.Errorf("cancelAllOrders 撤单失败 %v", handlerErr)
					return
				}
				if !rs.isOkApiResponse(value, "/api/v5/trade/cancel-order") {
					return
				}
			})
			if err != nil {
				rs.Logger.Errorf("撤单网络错误 %v", err)
			}
			time.Sleep(time.Millisecond * 120)
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("获取pending订单 网络错误 %v", err)
	}
	return
}

func (rs *OkxSpotRs) wsAmendOrder(info *helper.ExchangeInfo, cid, oid string, price float64, size fixed.Fixed, t int64) { //
	params := map[string]interface{}{
		"instId":    info.Symbol,
		"ordId":     oid,
		"clOrdId":   cid,
		"newPx":     helper.FixPrice(price, info.TickSize).String(),
		"newSz":     size.String(),
		"cxlOnFail": true, // 改单失败自动撤单
	}

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	curId := rs.id.Add(1)
	id := fmt.Sprintf("%v%04d", oid, curId)
	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   id,
			"op":   "amend-order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error { // 目前改单失败，自动去尝试撤单
			go rs.wsCancelOrder(info, cid, oid, 0)
			return nil
		},
	})

}
func (rs *OkxSpotRs) rsPlaceOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64) {
	// 请求必备变量
	url := "/api/v5/trade/order"
	params := map[string]interface{}{
		"instId":  info.Symbol,
		"tdMode":  "cross",
		"clOrdId": cid,
		"sz":      size.String(),
		"tgtCcy":  "base_ccy",
	}

	switch side {
	case helper.OrderSideKD:
		params["side"] = "buy"
	case helper.OrderSidePK:
		params["side"] = "buy"
	case helper.OrderSidePD:
		params["side"] = "sell"
	case helper.OrderSideKK:
		params["side"] = "sell"
	}
	switch orderType {
	case helper.OrderTypeLimit:
		params["ordType"] = "limit"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	case helper.OrderTypeIoc:
		params["ordType"] = "ioc"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	case helper.OrderTypeMarket:
		params["ordType"] = "market"
	case helper.OrderTypePostOnly:
		params["ordType"] = "post_only"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	}
	//
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	var err error
	var value *fastjson.Value

	_, err = rs.client.post(url, params, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("failed to rsPlaceOrder. %v", err)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}
		if !rs.isOkApiResponse(value, url) {
			var event helper.OrderEvent
			event.Pair = info.Pair
			event.Type = helper.OrderEventTypeERROR
			event.ErrorReason = string(respBody)
			event.ClientID = cid
			rs.Cb.OnOrder(0, event)
			err = errors.New(string(respBody))
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}

		for _, data := range helper.MustGetArray(value, "data") {
			var event helper.OrderEvent
			event.Pair = info.Pair
			sCode := helper.MustGetShadowStringFromBytes(data, "sCode")
			if sCode == "0" {
				event.Type = helper.OrderEventTypeNEW
				rs.ReqSucc(base.FailNumActionIdx_Place)
			} else {
				event.Type = helper.OrderEventTypeERROR
				event.ErrorReason = helper.MustGetStringFromBytes(data, "sMsg")
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
			event.OrderID = helper.MustGetStringFromBytes(data, "ordId")
			event.ClientID = cid
			rs.Cb.OnOrder(0, event)
		}
	})
	return
}

func (rs *OkxSpotRs) wsPlaceOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64) {
	params := map[string]interface{}{
		"tdMode":  "cross",
		"instId":  info.Symbol,
		"clOrdId": cid,
		"sz":      size.String(),
		"tgtCcy":  "base_ccy",
	}
	switch side {
	case helper.OrderSideKD:
		params["side"] = "buy"
	case helper.OrderSidePD:
		params["side"] = "sell"
	case helper.OrderSideKK:
		params["side"] = "sell"
	case helper.OrderSidePK:
		params["side"] = "buy"
	}

	switch orderType {
	case helper.OrderTypeLimit:
		params["ordType"] = "limit"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	case helper.OrderTypeIoc:
		params["ordType"] = "ioc"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	case helper.OrderTypeMarket:
		params["ordType"] = "market"
	case helper.OrderTypePostOnly:
		params["ordType"] = "post_only"
		params["px"] = helper.FixPrice(price, info.TickSize).String()
	}
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	curId := rs.id.Add(1)
	id := fmt.Sprintf("%v%04d", cid, curId)
	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   id,
			"op":   "order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return nil
		},
	})
}

func (rs *OkxSpotRs) wsCancelOrder(info *helper.ExchangeInfo, cid, oid string, t int64) {
	params := map[string]interface{}{
		"instId":  info.Symbol,
		"ordId":   oid,
		"clOrdId": cid,
	}

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	curId := rs.id.Add(1)
	id := fmt.Sprintf("%v%04d", oid, curId)
	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   id,
			"op":   "cancel-order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error {
			return nil
		},
	})
}

func (rs *OkxSpotRs) checkOrder(info *helper.ExchangeInfo, oid, cid string) {
	msg := map[string]interface{}{
		"instId":  info.Symbol,
		"ordId":   oid,
		"clOrdId": cid,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err := rs.client.get("/api/v5/trade/order", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("查单失败 解析错误 %v", handlerErr)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code == "51603" {
			//  {\"code\":\"51603\",\"data\":[],\"msg\":\"Order does not exist\"}"}
			order := helper.OrderEvent{
				Pair:     info.Pair,
				Type:     helper.OrderEventTypeNotFound,
				ClientID: cid,
				OrderID:  oid,
			}
			rs.Cb.OnOrder(0, order)
			return
		}
		if !rs.isOkApiResponse(value, "/api/v5/trade/order") {
			return
		}
		for _, data := range value.GetArray("data") {
			order := helper.OrderEvent{Pair: info.Pair}
			order.ClientID = string(data.GetStringBytes("clOrdId"))
			order.OrderID = string(data.GetStringBytes("ordId"))
			state := helper.BytesToString(helper.MustGetStringBytes(data, "state"))
			switch state {
			case "canceled", "filled", "mmp_canceled":
				order.Type = helper.OrderEventTypeREMOVE
			case "partially_filled":
				order.Type = helper.OrderEventTypePARTIAL
			default:
				order.Type = helper.OrderEventTypeNEW
			}
			sz := helper.BytesToString(helper.MustGetStringBytes(data, "accFillSz"))
			if sz != "" && sz != "0" {
				order.Filled = fixed.NewS(sz)
				order.FilledPrice = helper.MustGetFloat64FromBytes(data, "avgPx")
				//order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))))
				order.CashFee = fixed.NewF(helper.MustGetFloat64FromBytes(data, "rebate")).Sub(fixed.NewF(helper.MustGetFloat64FromBytes(data, "fee")))
			}
			switch helper.MustGetShadowStringFromBytes(data, "side") {
			case "buy":
				order.OrderSide = helper.OrderSideKD
			case "sell":
				order.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(data, "side"))
				return
			}
			switch helper.MustGetShadowStringFromBytes(data, "ordType") {
			case "limit":
				order.OrderType = helper.OrderTypeLimit
			case "market":
				order.OrderType = helper.OrderTypeMarket
			case "post_only":
				order.OrderType = helper.OrderTypePostOnly
			case "ioc":
				order.OrderType = helper.OrderTypeIoc
			}

			rs.Cb.OnOrder(0, order)
		}
	})
	if err != nil {
		rs.Logger.Errorf("查单失败 网络错误 %v", err)
	}
}

func (rs *OkxSpotRs) getExchangeInfo() []helper.ExchangeInfo {
	fileName := base.GenExchangeInfoFileName("")
	if pairInfo, infos, ok := helper.TryGetExchangeInfosPtrFromFile(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}

	// 请求必备信息
	uri := "/api/v5/public/instruments"
	params := make(map[string]interface{})
	params["instType"] = "SPOT"
	// 待使用数据结构
	infos := make([]helper.ExchangeInfo, 0)
	// 发起请求
	_, err := rs.client.get(uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value ExchangeInfo
		handlerErr := jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s, %s", rs.ExchangeName, handlerErr.Error(), string(respBody)))
			return
		}
		if value.Code != "0" {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, helper.BytesToString(respBody)))
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)

		datas := value.Data
		for _, data := range datas {
			baseCoin := strings.ToLower(data.BaseCcy)
			quoteCoin := strings.ToLower(data.QuoteCcy)

			var tickSize, stepSize float64
			var maxQty4MarketOrder, minQty fixed.Fixed
			var minValue fixed.Fixed
			tickSize = fixed.NewS(data.TickSz).Float()
			stepSize = fixed.NewS(data.LotSz).Float()
			maxQty4MarketOrder = fixed.NewS(data.MaxMktSz)
			maxQty4LimitOrder := fixed.NewS(data.MaxLmtSz)
			minQty = fixed.NewS(data.MinSz)
			maxValue4LimitOrder := fixed.NewS(data.MaxLmtAmt)
			maxValue4MarketOrder := fixed.NewS(data.MaxMktAmt)

			if minValue.Equal(fixed.ZERO) {
				minValue = fixed.TEN
				// maxValue = fixed.NewF(200000)
			}

			symbol := data.InstID
			info := helper.ExchangeInfo{
				Pair:                helper.Pair{Base: baseCoin, Quote: quoteCoin},
				Symbol:              symbol,
				Status:              data.State == "live",
				TickSize:            tickSize,
				StepSize:            stepSize,
				MaxOrderAmount:      maxQty4MarketOrder, //Didnt fix, some coin max market qty < minQty, sats {"alias":"","baseCcy":"SATS","category":"1","ctMult":"","ctType":"","ctVal":"","ctValCcy":"","expTime":"","instFamily":"","instId":"SATS-USDT","instType":"SPOT","lever":"5","listTime":"1702866701000","lotSz":"1","maxIcebergSz":"9999999999999999.0000000000000000","maxLmtAmt":"20000000","maxLmtSz":"9999999999999999","maxMktAmt":"1000000","maxMktSz":"1000000","maxStopSz":"1000000","maxTriggerSz":"9999999999999999.0000000000000000","maxTwapSz":"9999999999999999.0000000000000000","minSz":"10000000","optType":"","quoteCcy":"USDT","ruleType":"normal","settleCcy":"","state":"live","stk":"","tickSz":"0.0000000001","uly":""},
				MaxOrderValue:       maxValue4MarketOrder,
				MaxLimitOrderAmount: maxQty4LimitOrder,
				MaxLimitOrderValue:  maxValue4LimitOrder,

				MinOrderAmount: minQty,
				MinOrderValue:  minValue,
				Multi:          fixed.ONE,
				// 最大持仓价值
				MaxPosValue: fixed.BIG,
				// 最大持仓数量
				MaxPosAmount: fixed.BIG,
			}
			if info.MaxPosAmount == fixed.NaN || info.MaxPosAmount.IsZero() {
				info.MaxPosAmount = fixed.BIG
			}
			if baseCoin == rs.Pair.Base && quoteCoin == rs.Pair.Quote {
				helper.CopySymbolInfo(rs.PairInfo, &info)
				rs.Symbol = symbol
			}
			infos = append(infos, info)
			rs.ExchangeInfoPtrS2P.Set(symbol, &info)
			rs.ExchangeInfoPtrP2S.Set(info.Pair.String(), &info)
		}
	})
	if err != nil {
		rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, err.Error()))
		return nil
	}
	// 写入文件
	jsonData, err := json.Marshal(infos)
	if err != nil {
		helper.LogErrorThenCall(fmt.Sprintf("failed to marshal %v", err), rs.Cb.OnExit)
	}
	os.WriteFile(fileName, jsonData, 0644)
	return infos
}

// getTicker 获取ticker行情 函数本身不需要返回值 通过callbackFunc传递出去
func (rs *OkxSpotRs) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	// 请求必备信息
	uri := "/api/v5/market/ticker?instId=" + symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	_, err.NetworkError = rs.client.get(uri, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, nil) {
			rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, string(respBody))
			return
		}
		data := value.GetArray("data")
		if len(data) == 0 {
			rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, string(respBody))
			return
		}
		d := data[0]
		ap := helper.MustGetFloat64FromBytes(d, "askPx")
		aq := helper.MustGetFloat64FromBytes(d, "askSz")
		bp := helper.MustGetFloat64FromBytes(d, "bidPx")
		bq := helper.MustGetFloat64FromBytes(d, "bidSz")
		ticker.Set(ap, aq, bp, bq)
		if symbol == rs.Symbol {
			rs.TradeMsg.Ticker.Set(ap, aq, bp, bq)
			rs.Cb.OnTicker(0)
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, err.Error())
	}
	return
}
func (rs *OkxSpotRs) GetEquity() (resp helper.Equity, err helper.ApiError) {
	var bal []helper.BalanceSum
	bal, err = rs.getEquity(true)
	for _, b := range bal {
		if b.Name == rs.Pair.Base {
			resp.IsSet = true
			resp.Coin = b.Amount
			resp.CoinFree = b.Avail
		} else if b.Name == rs.Pair.Quote {
			resp.IsSet = true
			resp.Cash = b.Amount
			resp.CashFree = b.Avail
		}
	}
	return
}
func (rs *OkxSpotRs) getEquity(only bool) (balanceSum []helper.BalanceSum, err helper.ApiError) { //获取资产
	url := "/api/v5/account/balance" // 此为交易账户
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	params := make(map[string]interface{})
	if only {
		params["ccy"] = rs.assetStr
	}

	_, err.NetworkError = rs.client.get(url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取账户资金失败 %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			rs.Logger.Errorf("[%s]获取账户资金失败 %s", rs.ExchangeName, string(respBody))
			return
		}
		data := helper.MustGetArray(value, "data")
		balanceSum = make([]helper.BalanceSum, 0, len(data))
		for _, v := range helper.MustGetArray(data[0], "details") {
			asset := helper.MustGetStringLowerFromBytes(v, "ccy")
			total := helper.MustGetFloat64FromBytes(v, "cashBal")
			avail := helper.MustGetFloat64FromBytes(v, "availBal")
			uTime := helper.MustGetInt64FromBytes(v, "uTime")

			fs := helper.FieldsSet_T(helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail)
			if e, ok := rs.EquityNewerAndStore(asset, uTime, time.Now().UnixNano(), fs); ok {
				e.TotalWithoutUpl = total
				e.Avail = avail
				rs.Cb.OnEquityEvent(0, *e)
			}

			balanceSum = append(balanceSum, helper.BalanceSum{
				Name:   asset,
				Amount: total,
				Avail:  avail,
				Price:  0,
			})
		}
	})
	if !err.Nil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("[%s]获取账户资金失败 %s", rs.ExchangeName, err.Error())
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	return
}
func (rs *OkxSpotRs) GetAllTickersKeyedSymbol() (ret map[string]helper.Ticker, err helper.ApiError) {
	ret = make(map[string]helper.Ticker)
	uri := "/api/v5/market/tickers?instType=SPOT"
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	_, err.NetworkError = rs.client.get(uri, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, nil) {
			rs.Logger.Errorf("[%s]获取ticker行情失败 %s", rs.ExchangeName, string(respBody))
			return
		}
		data := helper.MustGetArray(value, "data")
		for _, d := range data {
			t := helper.Ticker{}
			symbol := helper.MustGetStringFromBytes(d, "instId")
			ap := helper.MustGetFloat64FromBytes(d, "askPx")
			aq := helper.MustGetFloat64FromBytes(d, "askSz")
			bp := helper.MustGetFloat64FromBytes(d, "bidPx")
			bq := helper.MustGetFloat64FromBytes(d, "bidSz")
			t.Set(ap, aq, bp, bq)
			ret[symbol] = t
		}
	})
	return
}

func (rs *OkxSpotRs) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	rs.client.updateTime()
	rs.getExchangeInfo()
	rs.UpdateExchangeInfoSimp(rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	rs.GetTickerBySymbol(rs.Symbol)
	switch mode {
	case helper.HandleModePublic:
		return
	case helper.HandleModePrepare:
	case helper.HandleModeCloseOne:
		rs.cancelAllOrders(true)
		leakedPrev = rs.HasPosition(rs, true)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOrders(false)
		leakedPrev = rs.HasPosition(rs, false)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOrders(true)
	case helper.HandleModeCancelAll:
		rs.cancelAllOrders(false)
	}
	time.Sleep(1 * time.Second)
	rs.getEquity(true)
	rs.latestCallBeforeTrade = time.Now().UnixMilli()
	rs.run()
	rs.exState.Store(helper.ExStateRunning)
	rs.PrintAcctSumWhenBeforeTrade(rs)
	return
}

func (rs *OkxSpotRs) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	switch mode {
	case helper.HandleModePrepare:
		isLeft = false
	case helper.HandleModeCloseOne:
		rs.cancelAllOrders(true)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOrders(false)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOrders(true)
		isLeft = false
	case helper.HandleModeCancelAll:
		rs.cancelAllOrders(false)
		isLeft = false
	}
	rs.exState.Store(helper.ExStateAfterTrade)
	return
}

func (rs *OkxSpotRs) DoStop() {
	rs.stop()
}

func (rs *OkxSpotRs) GetExName() string {
	return helper.BrokernameOkxSpot.String()
}

func (rs *OkxSpotRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	balance, _ := rs.getEquity(false)
	for _, b := range balance {
		if strings.EqualFold(b.Name, rs.Pair.Quote) {
			continue
		}
		resp = append(resp, helper.PositionSum{
			Name:        helper.NewPair(b.Name, rs.Pair.Quote, "").ToString(),
			Amount:      b.Amount,
			AvailAmount: b.Avail,
			Side:        helper.PosSideLong,
		})
	}
	return resp, helper.ApiErrorNil
}

func (rs *OkxSpotRs) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("failed to get symbol info. %s", symbol)
		return false
	}

	cid := fmt.Sprintf("99%d", uint32(time.Now().UnixMilli()))

	price := ticker.Bp.Load() * 0.996
	rs.rsPlaceOrder(info, price, orderAmount, cid, orderSide, helper.OrderTypeLimit, 0)
	return true
}

func (rs *OkxSpotRs) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	return rs.GetOrderListInFather(rs, startTimeMs, endTimeMs, orderState)
}

func (rs *OkxSpotRs) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/api/v5/trade/orders-pending"
	params := make(map[string]interface{})
	params["instType"] = "SPOT"
	if only {
		params["instId"] = rs.Symbol
	}
	var err error

	_, err = rs.client.get(uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("cancelAllOpenOrders error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := helper.MustGetArray(value, "data")
		hasPendingOrderBefore = len(data) != 0
	})
	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("cancelAllOpenOrders error %v", err)
	}
	if hasPendingOrderBefore {
		rs.cancelAllOrders(only)
	}
	return
}
func (rs *OkxSpotRs) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	const _LEN = 100 //交易所对该字段最大约束为100
	uri := "/api/v5/trade/orders-history-archive"

	params := make(map[string]interface{})
	params["instType"] = "SPOT"
	params["begin"] = fmt.Sprintf("%d", startTimeMs)
	params["end"] = fmt.Sprintf("%d", endTimeMs)

	_, err.NetworkError = rs.client.get(uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handler error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("%s", string(respBody))
			rs.Logger.Errorf("handler error %v", value)
			return
		}

		orders := helper.MustGetArray(value, "data")
		resp.Orders = make([]helper.OrderForList, 0, len(orders))
		if len(orders) >= _LEN {
			resp.HasMore = true
		}

		for _, v := range orders {
			var oid string

			if helper.MustGetStringFromBytes(v, "clOrdId") != "" {
				oid = helper.MustGetStringFromBytes(v, "clOrdId")
			} else {
				oid = "clOrdId is null"
			}
			order := helper.OrderForList{
				OrderID:       helper.MustGetStringFromBytes(v, "ordId"),
				ClientID:      oid,
				Price:         helper.GetFloat64FromBytes(v, "avgPx"),
				Amount:        fixed.NewS(helper.MustGetShadowStringFromBytes(v, "sz")),
				CreatedTimeMs: helper.MustGetInt64FromBytes(v, "cTime"),
				UpdatedTimeMs: helper.MustGetInt64FromBytes(v, "uTime"),
				Filled:        fixed.NewS(helper.MustGetShadowStringFromBytes(v, "fillSz")),
				FilledPrice:   helper.GetFloat64FromBytes(v, "fillPx"),
			}
			status := helper.BytesToString(v.GetStringBytes("state"))

			switch status {
			case "new", "init":
				order.OrderState = helper.OrderStatePending
			case "filled", "canceled", "mmp_canceled":
				order.OrderState = helper.OrderStateFinished
				order.UpdatedTimeMs = helper.GetInt64(v, "uTime")
			}
			if orderState != helper.OrderStateAll && orderState != order.OrderState {
				continue
			}
			side := helper.MustGetShadowStringFromBytes(v, "side")
			switch side {
			case "buy":
				order.OrderSide = helper.OrderSideKD
			case "sell":
				order.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %s", side)
				err.HandlerError =
					fmt.Errorf("side error: %s", side)
				return
			}
			// if helper.MustGetBool(v, "reduceOnly") {
			// 	if order.OrderSide == helper.OrderSideKD {
			// 		order.OrderSide = helper.OrderSidePK
			// 	} else if order.OrderSide == helper.OrderSideKK {
			// 		order.OrderSide = helper.OrderSidePD
			// 	}
			// }
			switch helper.MustGetShadowStringFromBytes(v, "ordType") {
			case "limit":
				order.OrderType = helper.OrderTypeLimit
			case "market":
				order.OrderType = helper.OrderTypeMarket
			case "ioc":
				order.OrderType = helper.OrderTypeIoc
			case "post_only":
				order.OrderType = helper.OrderTypePostOnly
			}
			order.OrderType = helper.OrderTypeUnknown
			resp.Orders = append(resp.Orders, order)
		}
	})

	return
}

func (rs *OkxSpotRs) GetFee() (fee helper.Fee, err helper.ApiError) {
	uri := "/api/v5/account/trade-fee"
	params := make(map[string]interface{})
	params["instType"] = "SPOT"

	_, err.NetworkError = rs.client.get(uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getFee error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("handler error %v", value)
			return
		}

		data := value.GetArray("data")

		fee.Maker = helper.MustGetFloat64FromBytes(data[0], "maker")
		fee.Taker = helper.MustGetFloat64FromBytes(data[0], "taker")
	})
	return
	//return helper.Fee{Maker: b.makerFee.Load(), Taker: b.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (rs *OkxSpotRs) GetFundingRate() (helper.FundingRate, error) {
	return helper.FundingRate{}, nil
}
func (rs *OkxSpotRs) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	a.Balances, _ = rs.getEquity(false)
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
func (rs *OkxSpotRs) SendSignal(signals []helper.Signal) {
	for i, s := range signals {
		if rs.isAlive && rs.logged { // 可操作的状态
			if helper.DEBUGMODE {
				rs.Logger.Debugf("发送信号 no.%d %s", i, s.String())
			}
			switch s.Type {
			case helper.SignalTypeNewOrder:
				info, ok := rs.GetPairInfoByPair(&s.Pair)
				if !ok {
					rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
					continue
				}
				if s.SignalChannelType == helper.SignalChannelTypeRs || rs.BrokerConfig.BanRsWs {
					rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time)
				} else {
					rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time)
				}
			case helper.SignalTypeAmend:
				info, ok := rs.GetPairInfoByPair(&s.Pair)
				if !ok {
					rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
					continue
				}
				rs.wsAmendOrder(info, s.ClientID, s.OrderID, s.Price, s.Amount, s.Time)
			case helper.SignalTypeCancelOrder:
				info, ok := rs.GetPairInfoByPair(&s.Pair)
				if !ok {
					rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
					continue
				}
				rs.wsCancelOrder(info, s.ClientID, s.OrderID, s.Time)
			case helper.SignalTypeCheckOrder:
				info, ok := rs.GetPairInfoByPair(&s.Pair)
				if !ok {
					rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
					continue
				}
				go rs.checkOrder(info, s.OrderID, s.ClientID)
			// case helper.SignalTypeGetPos:
			// go w.getPosition()
			case helper.SignalTypeGetEquity:
				go rs.getEquity(true)
			case helper.SignalTypeCancelOne:
				rs.cancelAllOrders(true)
			case helper.SignalTypeCancelAll:
				rs.cancelAllOrders(false)
			}
		} else {
			rs.Logger.Errorf("send signal err! alive %v logged %v signal %v", rs.isAlive, rs.logged, s)
			// 对于下单，就构造一个下单失败立刻返回
			switch s.Type {
			case helper.SignalTypeNewOrder:
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = s.ClientID
				rs.Cb.OnOrder(0, order)
			}
		}
	}
}

func (rs *OkxSpotRs) Do(act string, params any) (rsp any, err error) {
	switch act {
	case "transfer":
	}
	return nil, nil
}

func (rs *OkxSpotRs) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltNil
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
func (rs *OkxSpotRs) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SPOT | base.AbltWsPriReqOrder | base.AbltOrderAmend | base.AbltOrderAmendByCid
}

func (rs *OkxSpotRs) WsLogged() bool {
	return rs.logged
}

func (rs *OkxSpotRs) GetExchangeInfos() []helper.ExchangeInfo {
	return rs.getExchangeInfo()
}

func (rs *OkxSpotRs) GetFeatures() base.Features {
	f := base.Features{
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		OrderPostonly:         true,
		UpdateWsTickerWithSeq: true,
		MultiSymbolOneAcct:    true,
		WsDepthLevel:          true,
		GetPriWs:              true,
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}
func (rs *OkxSpotRs) GetPriWs() *ws.WS {
	return rs.priWs
}
func (rs *OkxSpotRs) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	return rs.DoGetPendingOrders("")
}

func (rs *OkxSpotRs) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	params := map[string]interface{}{"instType": "SPOT"}
	if symbol != "" {
		params["instId"] = symbol // 空就是全部撤单
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/trade/orders-pending", params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("获取pending订单失败 回报错误 %v", handlerErr)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			datas := value.GetArray("data")
			results = make([]helper.OrderForList, 0)
			for _, data := range datas {
				o := helper.OrderForList{}
				o.Symbol = helper.MustGetStringFromBytes(data, "instId")
				o.OrderID = helper.MustGetStringFromBytes(data, "ordId")
				o.ClientID = helper.MustGetStringFromBytes(data, "clOrdId")
				o.Price = helper.MustGetFloat64FromBytes(data, "px")
				o.Amount = fixed.NewF(helper.MustGetFloat64FromBytes(data, "sz"))
				switch helper.MustGetShadowStringFromBytes(data, "side") {
				case "buy":
					o.OrderSide = helper.OrderSideKD
				case "sell":
					o.OrderSide = helper.OrderSideKK
				}
				o.OrderState = helper.OrderStatePending
				o.CreatedTimeMs = helper.MustGetInt64FromBytes(data, "cTime")
				o.UpdatedTimeMs = helper.MustGetInt64FromBytes(data, "uTime")
				o.Filled = fixed.NewF(helper.MustGetFloat64FromBytes(data, "fillSz"))
				o.FilledPrice = helper.MustGetFloat64FromBytes(data, "fillPx")
				switch helper.MustGetShadowStringFromBytes(data, "ordType") {
				case "limit":
					o.OrderType = helper.OrderTypeLimit
				case "market":
					o.OrderType = helper.OrderTypeMarket
				case "post_only":
					o.OrderType = helper.OrderTypePostOnly
				case "ioc":
					o.OrderType = helper.OrderTypeIoc
				}
				results = append(results, o)
			}
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("获取pending订单 网络错误 %v", err)
	}
	return
}
