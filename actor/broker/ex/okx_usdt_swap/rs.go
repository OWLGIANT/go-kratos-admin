// rest将其实大部分采用的是ws来下单, 只是名字是rest
// 由于ok会关掉长时间没有数据的链接，所以这里会去订阅一下 accounts, 维持一个低水平的订阅

package okx_usdt_swap

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	"actor/third/log"
	"actor/tools"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

var NETMODE = true // 判断是单向还是双向

func SetLongShortMode(flag bool) {
	NETMODE = !flag
}

type OkxUsdtSwapRs struct {
	base.DummyOrderAction
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyDoSetLeverage
	base.DummyDoSetMarginMode
	base.DummyDoSetPositionMode
	priWs       *ws.WS             // 继承ws
	connectOnce sync.Once          // 连接一次
	id          int64              // 所有发送的消息都要唯一id
	lk          sync.Mutex         //
	stopCPri    chan struct{}      // 接受停机信号
	colo        bool               // 是否colo
	priWsUrl    string             // 私有ws地址
	restUrl     string             // rest地址
	client      *OkxUsdtSwapClient // 用来查询各种信息
	logged      bool               // 是否已登录, 登录完成才可以交易
	isAlive     bool               // 是否已开启且尚未关闭, 避免重复关闭
	//
	takerFee atomic.Float64 // taker费率
	makerFee atomic.Float64 // maker费率
	exState  atomic.Int32

	latestCallBeforeTrade int64 // 最近调用BeforeTrade时间，清仓时会多次调用。这里用于处理登录逻辑

	marginMode string
}

func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	coloFlag := checkColo(params)
	rs := &OkxUsdtSwapRs{
		colo:     coloFlag,
		priWsUrl: okxWsPriUrl,
		restUrl:  okxRestUrl,
	}
	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, info, cb)

	if !params.BanRsWs {
		rs.priWs = ws.NewWS(rs.priWsUrl, params.LocalAddr, params.ProxyURL, rs.priHandler, cb.OnExit, params.BrokerConfig)
		rs.priWs.SetPingFunc(rs.ping)
		// fixme 如果有信息交互，其实不用固定ping
		rs.priWs.SetPingInterval(10)
		rs.priWs.SetSubscribe(rs.subPri)
	}
	rs.marginMode = "cross" //default

	rs.client = NewClient(params, cb)

	return rs
}

func (rs *OkxUsdtSwapRs) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
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
		rs.Logger.Errorf("请求失败 req: %s %v. rsp: %s", url, params, value.String())
		rs.ReqFail(base.FailNumActionIdx_AllReq)
		return false
	}
}

func (rs *OkxUsdtSwapRs) run() {
	rs.isAlive = true

	if rs.priWs != nil && !rs.priWs.IsConnected() {
		var err error
		rs.stopCPri, err = rs.priWs.Serve()
		if err != nil {
			rs.Cb.OnExit("okx usdt swap private ws 连接失败")
		}
	}
}

func (rs *OkxUsdtSwapRs) stop() {
	if rs.isAlive && rs.stopCPri != nil {
		rs.isAlive = false
		helper.CloseSafe(rs.stopCPri)
	}
	rs.logged = false
}

func (rs *OkxUsdtSwapRs) ping() []byte {
	return []byte("ping")
}
func (rs *OkxUsdtSwapRs) GetPriWs() *ws.WS {
	return rs.priWs
}
func (rs *OkxUsdtSwapRs) subPri() error {
	// login first
	rs.logged = false
	msg := getWsLogin(&rs.BrokerConfig)
	rs.priWs.SendMessage(msg)
	// 实际登录注册信息被放在后面了, 在 realSubscribe 里面
	rs.logged = true
	return nil
}

func (rs *OkxUsdtSwapRs) realSubPri() {
	p := map[string]interface{}{
		"op": "subscribe",
		"args": []interface{}{
			map[string]interface{}{
				"channel": "account",
				"ccy":     strings.ToUpper(rs.Pair.Quote),
			},
		},
	}
	msg, err := json.Marshal(p)
	if err != nil {
		rs.Logger.Errorf("[ws][%s] json encode error , %s", rs.ExchangeName.String(), err)
	}
	rs.priWs.SendMessage(msg)
}

// 处理私有ws的处理器
func (rs *OkxUsdtSwapRs) priHandler(msg []byte, ts int64) {
	if rs.exState.Load() == helper.ExStateAfterTrade {
		return
	}
	if helper.DEBUGMODE {
		rs.Logger.Debugf("收到 pri ws 消息 %v", helper.BytesToString(msg))
	}
	// 解析
	p := wsPrivateHandyPool.Get()
	defer wsPrivateHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if len(msg) == 4 && helper.BytesToString(msg) == "pong" {
			return
		} else {
			rs.Logger.Errorf("okx usdt swap ws解析msg出错 err:%v", err)
			return
		}
	}

	// {"id":"17383872133852282880001","op":"amend-order","code":"0","msg":"","data":[{"ordId":"1738387213385228288","clOrdId":"IRU761696101","sCode":"0",
	// "sMsg":"","reqId":"","ts":"1724310388337"}],"inTime":"1724310388337055","outTime":"1724310388339074"}
	if value.Exists("data") {
		op := helper.BytesToString(value.GetStringBytes("op"))
		if op != "" {
			switch op {
			case "order":
				code := helper.BytesToString(value.GetStringBytes("code"))
				switch code {
				case "0":
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
						order.Pair = helper.NewPair(baseCoin, "usdt", "")
						rs.Cb.OnOrder(ts, order)
						rs.ReqSucc(base.FailNumActionIdx_Place)
					}
				default:
					rs.Logger.Warnf(fmt.Sprintf("rest发送订单失败 %v", value))
					for _, data := range value.GetArray("data") {
						var order helper.OrderEvent
						order.Type = helper.OrderEventTypeREMOVE // 这里可以用 OrderEventTypeREMOVE 因为是交易所返回的err 如果是本地产生的err 就用 OrderEventTypeERROR
						order.ClientID = string(data.GetStringBytes("clOrdId"))
						order.OrderID = string(data.GetStringBytes("ordId"))
						order.ErrorReason = string(data.GetStringBytes("sMsg"))
						underscoreIndex := strings.IndexByte(order.ClientID, helper.CID_BASE_COIN_SPLIT_OKX)
						if underscoreIndex == -1 {
							rs.Logger.Error("cid format error", order.ClientID)
						}
						baseCoin := order.ClientID[:underscoreIndex]
						order.Pair = helper.NewPair(baseCoin, "usdt", "")
						rs.Cb.OnOrder(ts, order)
						rs.ReqFail(base.FailNumActionIdx_Place)
					}
				}
			case "cancel-order":
				code := helper.BytesToString(value.GetStringBytes("code"))
				switch code {
				case "0":
				default:
					if value.Exists("data") {
						datas := value.GetArray("data")
						if len(datas) > 0 {
							data := datas[0]
							scode := helper.BytesToString(data.GetStringBytes("sCode"))
							switch scode {
							case "51410":
								// Cancellation failed as the order is already under cancelling status
								rs.Logger.Debugf(fmt.Sprintf("rest撤单中 %v", value))
								return
							}
						}
					}
					rs.Logger.Warnf(fmt.Sprintf("rest撤单失败 %v", value))
				}
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
					order.Pair = helper.NewPair(base, "usdt", "")
					scode := helper.BytesToString(data.GetStringBytes("sCode"))
					switch scode {
					case "0":
						order.Type = helper.OrderEventTypeAmendSucc
					case "51513":
						// Number of modification requests that are currently in progress for an order cannot exceed 3.
						// 短时间内太多改单命令，debug即可。也不能完全不重视，毕竟占用限频，能规避还是规避好
						rs.Logger.Debugf(fmt.Sprintf("rest改单过频 %v", value))
						// 尝试撤单
						// 这里拿不到pair，直接传入空pair
						// todo event
						// go rs.wsCancelOrder(helper.Pair{}, string(data.GetStringBytes("ordId")), 0) // 尝试在此时撤单
						order.Type = helper.OrderEventTypeAmendFail
					case "51510":
						// Modification failed as the order has been completed.
						// 订单已经完成了，给一个info吧, 后续也可以处理处理，不过这里信息太少了，光知道完成，是成交还是撤单，成交价格数量都不知道
						// rs.Logger.Infof(fmt.Sprintf("rest改单失效 %v", value))
						order.Type = helper.OrderEventTypeAmendFail
						return
					}
					rs.Cb.OnOrder(0, order)
				}
				rs.Logger.Warnf(fmt.Sprintf("rest改单失败 %v", value))
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
			// 登录订阅放在这里
			rs.logged = true
			rs.Cb.OnWsReady(rs.ExchangeName)
			time.Sleep(time.Second) // 交易所会反应慢
			// rs.realSubPri()  // okx限制一直channel 20 connc，下单ws不需要账户信息
		}
	}
}

// func (w *OkxUsdtSwapRs) existPosition(only bool) bool {
// 	var params map[string]interface{}
// 	params = map[string]interface{}{"instType": "SWAP"}
// 	if only || (!w.params.NeedMultiSymbol) {
// 		params["instId"] = w.Symbol
// 	}
// 	num := 0
// 	_, err := w.client.get("/api/v5/account/positions", params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
// 		p := handyPool.Get()
// 		defer handyPool.Put(p)
// 		value, handlerErr := p.ParseBytes(respBody)
// 		if handlerErr != nil {
// 			rs.Logger.Errorf("获取仓位 回报错误 %v", handlerErr)
// 			return
// 		}
// 		if value.Exists("data") {
// 			for _, data := range value.GetArray("data") {
// 				pos := fixed.NewS(helper.BytesToString(data.GetStringBytes("pos"))).Int64()
// 				if pos != 0 {
// 					num += 1
// 				}
// 			}
// 		}
// 	})
// 	if err != nil {
// 		rs.Logger.Errorf("获取仓位 网络错误 %v", err)
// 		// 获取仓位错误也要返回 true 只有正确http获取到仓位并且没有仓位 这种情况能100%确定无仓位 才返回false 策略层面会收集多次清仓结果 直到确认100%无仓位才允许退出 否则丁丁报警人工处理
// 		return true
// 	}
// 	return num > 0

// }

// todo code here
func (rs *OkxUsdtSwapRs) DoGetPosition(only bool) (pos []helper.PositionSum, err helper.ApiError) {
	// params := map[string]interface{}{"instType": "SWAP"}
	// if only {
	// 	params["instId"] = w.symbol
	// }
	// _, err.NetworkError = w.client.get("/api/v5/account/positions", params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
	// 	p := handyPool.Get()
	// 	defer handyPool.Put(p)
	// 	var value *fastjson.Value
	// 	value, err.HandlerError = p.ParseBytes(respBody)
	// 	if err.HandlerError != nil {
	// 		rs.Logger.Errorf("获取仓位 回报错误 %v", err.HandlerError)
	// 		return
	// 	}
	// 	for _, data := range value.GetArray("data") {
	// 		amount := helper.MustGetInt64FromBytes(data, "pos")
	// 		availAmount := helper.MustGetInt64FromBytes(data, "availPos")
	// 		instId := helper.MustGetStringFromBytes(data, "instId")
	// 		posSide := helper.MustGetStringFromBytes(data, "posSide")
	// 		params = map[string]interface{}{
	// 			"instId":  string(data.GetStringBytes("instId")),
	// 			"posSide": string(data.GetStringBytes("posSide")),
	// 			"mgnMode": string(data.GetStringBytes("mgnMode")),
	// 			"autoCxl": "true",
	// 		}
	// 		// todo 这个地方看上去能平掉不同仓位模式下的仓位 不知道测试过没有 最好充分测试一下 手动下单让账号同时持有不同品种的单向、双向模式 or 不同模式的仓位
	// 		_, err := w.client.post("/api/v5/trade/close-position", params, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
	// 			p := handyPool.Get()
	// 			defer handyPool.Put(p)
	// 			value, handlerErr := p.ParseBytes(respBody)
	// 			if handlerErr != nil {
	// 				rs.Logger.Errorf("close-position 失败 %v", handlerErr)
	// 				return
	// 			}
	// 			rs.Logger.Infof("市价平仓 %v", value)
	// 		})
	// 		if err != nil {
	// 			rs.Logger.Errorf("close-position 网络错误 %v", err)
	// 		}

	// 	}
	// })
	// if !err.Nil() {
	// 	rs.Logger.Errorf("获取仓位 网络错误 %v", err)
	// }
	return
}

func (rs *OkxUsdtSwapRs) CleanAllPositions(only bool) bool {
	var params map[string]interface{}
	isLeft := false
	params = map[string]interface{}{"instType": "SWAP"}
	if only {
		params["instId"] = rs.Symbol
	}
	_, err := rs.client.get("/api/v5/account/positions", params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("获取仓位 回报错误 %v", handlerErr)
			return
		}
		for _, data := range value.GetArray("data") {
			pos := helper.MustGetFloat64FromBytes(data, "pos")
			if pos != 0.0 {
				isLeft = true
				params = map[string]interface{}{
					"instId":  helper.MustGetStringFromBytes(data, "instId"),
					"posSide": helper.MustGetStringFromBytes(data, "posSide"),
					"mgnMode": helper.MustGetStringFromBytes(data, "mgnMode"),
					"autoCxl": "true",
				}
				// todo 这个地方看上去能平掉不同仓位模式下的仓位 不知道测试过没有 最好充分测试一下 手动下单让账号同时持有不同品种的单向、双向模式 or 不同模式的仓位
				_, err := rs.client.post("/api/v5/trade/close-position", params, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
					p = handyPool.Get()
					defer handyPool.Put(p)
					value, handlerErr = p.ParseBytes(respBody)
					if handlerErr != nil {
						rs.Logger.Errorf("close-position 失败 %v", handlerErr)
						return
					}
					rs.Logger.Infof("市价平仓 %v", value)
				})
				if err != nil {
					rs.Logger.Errorf("close-position 网络错误 %v", err)
				}

			}
		}
	})
	if err != nil {
		rs.Logger.Errorf("获取仓位 网络错误 %v", err)
	}
	return isLeft
}
func (rs *OkxUsdtSwapRs) DoCancelPendingOrders(symbol string) (err helper.ApiError) { // 撤销全部订单
	params := map[string]interface{}{"instType": "SWAP"}
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
			var msg map[string]interface{}
			datas := value.GetArray("data")
			for _, data := range datas {
				msg = map[string]interface{}{
					"instId": string(data.GetStringBytes("instId")),
					"ordId":  string(data.GetStringBytes("ordId")),
				}
				_, err:= rs.client.post("/api/v5/trade/cancel-order", msg, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
					p := handyPool.Get()
					defer handyPool.Put(p)
					value, handlerErr := p.ParseBytes(respBody)
					if handlerErr != nil {
						rs.Logger.Errorf("cancelAllOrders 撤单失败 %v", handlerErr)
						return
					}
					rs.Logger.Infof("自动撤单 %v", value)
				})
				if err != nil {
					rs.Logger.Errorf("撤单网络错误 %v", err)
				}
				time.Sleep(time.Second)
			}
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("获取pending订单 网络错误 %v", err)
	}
	return
}

func (rs *OkxUsdtSwapRs) cancelAllOpenOrders(only bool) { // 撤销全部订单
	if only {
		rs.DoCancelPendingOrders(rs.Symbol)
	} else {
		rs.DoCancelPendingOrders("")
	}
}

func (rs *OkxUsdtSwapRs) setPositionModeLongShort() {
	msg := map[string]interface{}{
		"posMode": "long_short_mode",
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err := rs.client.post("/api/v5/account/set-position-mode", msg, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("设置双向持仓失败 回报错误 %v", handlerErr)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			rs.Logger.Infof("设置双向持仓成功 %v", value)
		case "59000":
			// 59000	200	设置失败，请在设置前关闭任何挂单或持仓
			// CloseOne会出现
		default:
			helper.LogErrorThenCall(fmt.Sprintf("设置双向持仓失败 %v", value), rs.Cb.OnExit)
		}
	})
	if err != nil {
		helper.LogErrorThenCall(fmt.Sprintf("设置持仓失败 %v", err), rs.Cb.OnExit)
	}
}

func (rs *OkxUsdtSwapRs) setPositionModeNet() {
	msg := map[string]interface{}{
		"posMode": "net_mode",
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err := rs.client.post("/api/v5/account/set-position-mode", msg, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("设置单向持仓失败 回报错误 %v", handlerErr)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			rs.Logger.Infof("设置单向持仓成功 %v", value)
		case "59000":
			// 59000	200	设置失败，请在设置前关闭任何挂单或持仓
			// CloseOne会出现
		default:
			helper.LogErrorThenCall(fmt.Sprintf("设置单向持仓失败 %v", value), rs.Cb.OnExit)
		}
	})
	if err != nil {
		helper.LogErrorThenCall(fmt.Sprintf("设置持仓失败 %v", err), rs.Cb.OnExit)
	}
}
func (rs *OkxUsdtSwapRs) DoSetAccountMode(pairInfo *helper.ExchangeInfo, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) (err helper.ApiError) {
	if marginMode == helper.MarginMode_Iso {
		rs.marginMode = "isolated"
	} else if marginMode == helper.MarginMode_Cross {
		rs.marginMode = "cross"
	}

	lever := fmt.Sprintf("%d", leverage)

	msg := map[string]interface{}{
		"instId":  pairInfo.Symbol,
		"lever":   lever,
		"mgnMode": rs.marginMode,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.post("/api/v5/account/set-leverage", msg, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("设置杠杆率失败 网络错误 %v", err.HandlerError)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			rs.Logger.Infof("设置杠杆率成功 %v %v", rs.Symbol, lever)
		default:
			rs.Logger.Warnf("设置杠杆率失败 %v", value)
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("设置杠杆率失败 网络错误 %v", err)
	}
	rs.getRiskLimitFromLeverage(*pairInfo, leverage)

	if positionMode == helper.PosModeOneway {
		rs.setPositionModeNet()
	} else {
		rs.setPositionModeLongShort()
	}
	return
}

func (rs *OkxUsdtSwapRs) DoGetAccountMode(pairInfo *helper.ExchangeInfo) (leverage int, marginMode helper.MarginMode, posMode helper.PosMode, err helper.ApiError) {
	if rs.marginMode == "cross" {
		marginMode = helper.MarginMode_Cross
	} else if rs.marginMode == "isolated" {
		marginMode = helper.MarginMode_Iso
	}
	if marginMode == helper.MarginMode_Nil {
		return 0, helper.MarginMode_Nil, helper.PosModeNil, helper.ApiError{NetworkError: errors.New("未设置marign mode")}
	}

	symbol := pairInfo.Symbol
	url := "/api/v5/account/config"
	params := map[string]interface{}{}
	_, err.NetworkError = rs.client.get(url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		p := handyPool.Get()
		defer handyPool.Put(p)

		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to parse position response for symbol %s: %v", symbol, err.HandlerError)
			err = helper.ApiError{NetworkError: err.HandlerError}
			return
		}

		code := helper.MustGetStringFromBytes(value, "code")
		if code != "0" {
			rs.Logger.Errorf("API returned error code for symbol %s: %s", symbol, code)
			err = helper.ApiError{NetworkError: fmt.Errorf("API error code %s", code)}
			return
		}

		mode := helper.MustGetShadowStringFromBytes(value, "data", "0", "posMode")

		switch mode {
		case "net_mode":
			posMode = helper.PosModeOneway
		case "long_short_mode":
			posMode = helper.PosModeHedge
		default:
			posMode = helper.PosModeNil
			err = helper.ApiError{NetworkError: fmt.Errorf("Err posMode %s", code)}
			return
		}
	})

	// 如果出现网络或其他错误，记录日志
	if !err.Nil() {
		rs.Logger.Errorf("failed to get account mode for symbol %s: %v", symbol, err)
		return
	}

	url = "/api/v5/account/leverage-info"
	params = map[string]interface{}{
		"instId":  symbol,
		"mgnMode": rs.marginMode,
	}

	_, err.NetworkError = rs.client.get(url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		p := handyPool.Get()
		defer handyPool.Put(p)

		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to parse position response for symbol %s: %v", symbol, err.HandlerError)
			err = helper.ApiError{NetworkError: err.HandlerError}
			return
		}

		code := helper.MustGetStringFromBytes(value, "code")
		if code != "0" {
			rs.Logger.Errorf("API returned error code for symbol %s: %s", symbol, code)
			err = helper.ApiError{NetworkError: fmt.Errorf("API error code %s", code)}
			return
		}
		leverage = int(helper.GetInt64FromBytes(value, "data", "0", "lever"))
	})

	// 如果出现网络或其他错误，记录日志
	if !err.Nil() {
		rs.Logger.Errorf("failed to get account mode for symbol %s: %v", symbol, err)
		return
	}
	return
}

func (rs *OkxUsdtSwapRs) getRiskLimitFromLeverage(pairInfo helper.ExchangeInfo, leverage int) (err helper.ApiError) {
	msg := map[string]interface{}{
		"instType":   "SWAP",
		"tdMode":     rs.marginMode,
		"instFamily": strings.Replace(pairInfo.Symbol, "-SWAP", "", 1),
		"instId":     pairInfo.Symbol,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/public/position-tiers", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Warnf("[%s]getRiskLimitFromLeverage. %s", rs.ExchangeName, handlerErr.Error())
			return
		}
		if !rs.isOkApiResponse(value, "/api/v5/public/position-tiers") {
			rs.Logger.Errorf("[%s]getRiskLimitFromLeverage err. %v", rs.ExchangeName, value)
			return
		}
		infos := helper.MustGetArray(value, "data")
		type Item struct {
			Lever float64
			Max   float64
		}
		items := slice.Map(infos, func(idx int, v *fastjson.Value) Item {
			return Item{
				Lever: helper.MustGetFloat64FromBytes(v, "maxLever"),
				Max:   helper.MustGetFloat64FromBytes(v, "maxSz") * pairInfo.Multi.Float(),
			}
		})
		sort.Slice(items, func(i, j int) bool {
			return items[i].Lever < items[j].Lever
		})
		foundItem := Item{}
		for _, item := range items {
			if float64(leverage) <= item.Lever {
				foundItem = item
				break
			}
		}
		if foundItem.Lever == 0 {
			err.HandlerError = fmt.Errorf("杠杆档位配置错误. 太高，没有适配限额. 杠杆:%d", leverage)
			return
		}
		rs.Logger.Infof("杠杆档位配置, %s setted lev %d, %+v", pairInfo.Symbol, leverage, foundItem)
		rs.ExchangeInfoLabilePtrP2S.Set(pairInfo.Pair.String(), &helper.LabileExchangeInfo{
			Pair:           pairInfo.Pair,
			Symbol:         pairInfo.Symbol,
			SettedLeverage: leverage,
			RiskLimit: helper.RiskLimit{
				Underlying: pairInfo.Pair.Base,
				Amount:     foundItem.Max,
			},
		})
	})
	if err.NotNil() {
		helper.LogErrorThenCall(fmt.Sprintf("[%s]failed getRiskLimitFromLeverage . %v", rs.ExchangeName, err), rs.Cb.OnExit)
		return
	}
	return helper.ApiErrorNil
}

func (rs *OkxUsdtSwapRs) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	rs.client.updateTime()
	rs.GetExchangeInfos()
	rs.UpdateExchangeInfo(rs.ExchangeInfoPtrP2S, rs.ExchangeInfoPtrS2P, rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	rs.getTicker(rs.Symbol)
	needWs := true
	switch mode {
	case helper.HandleModePublic:
		return
	case helper.HandleModePrepare:
		needWs = false
		rs.getPosition(rs.PairInfo, true, true)
	case helper.HandleModeCloseOne:
		rs.cancelAllOpenOrders(true)
		leakedPrev = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		leakedPrev = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOpenOrders(true)
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
	}
	rs.adjustAcct()
	rs.GetEquity()
	rs.PrintAcctSumWhenBeforeTrade(rs)
	rs.latestCallBeforeTrade = time.Now().UnixMilli()
	if needWs {
		rs.run()
	}
	rs.exState.Store(helper.ExStateRunning)
	return
}

func (rs *OkxUsdtSwapRs) adjustAcct() {
	if !rs.BrokerConfig.InitialAcctConfig.IsEmpty() {
		rs.SetAccountMode(rs.Pair.String(), rs.BrokerConfig.InitialAcctConfig.MaxLeverage, rs.BrokerConfig.InitialAcctConfig.MarginMode, rs.BrokerConfig.InitialAcctConfig.PosMode)
		return
	}
	rs.DoSetAccountMode(rs.PairInfo, rs.PairInfo.MaxLeverage, helper.MarginMode_Cross, helper.PosModeOneway)
}

func (rs *OkxUsdtSwapRs) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	// rs.stop()
	time.Sleep(1 * time.Second)
	//w.cancelAllOrders(only)
	//w.cleanAllPositions(only)
	//return w.existPosition(only)
	switch mode {
	case helper.HandleModePrepare:
		isLeft = false
	case helper.HandleModeCloseOne:
		rs.cancelAllOpenOrders(true)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
		// isLeft = rs.existPosition(true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
		// isLeft = rs.existPosition(false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOpenOrders(true)
		isLeft = false
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
		isLeft = false
	}
	rs.exState.Store(helper.ExStateAfterTrade)
	return
}
func (rs *OkxUsdtSwapRs) DoStop() {
	rs.stop()
}

func (rs *OkxUsdtSwapRs) GetExName() string {
	return helper.BrokernameOkxUsdtSwap.String()
}

func (rs *OkxUsdtSwapRs) GetPosition() (resp []helper.PositionSum, err helper.ApiError) {
	return rs.getPosition(rs.PairInfo, true, true)
}
func (rs *OkxUsdtSwapRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	return rs.getPosition(rs.PairInfo, false, false)
}
func (rs *OkxUsdtSwapRs) DoGetPriceLimit(symbol string) (pl helper.PriceLimit, err helper.ApiError) {
	msg := map[string]interface{}{
		"instId": symbol,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/public/price-limit", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf(" 解析错误 %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, "get price limit") {
			err.HandlerError = fmt.Errorf("获取 失败 %v", value)
			return
		}
		data0 := helper.MustGetArray(value, "data")[0]
		pl.BuyLimit = helper.MustGetFloat64FromBytes(data0, "buyLmt")
		pl.SellLimit = helper.MustGetFloat64FromBytes(data0, "sellLmt")
	})
	return
}

func (rs *OkxUsdtSwapRs) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	return rs.getTicker(symbol)
}

func (rs *OkxUsdtSwapRs) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	cid := fmt.Sprintf("99%d", time.Now().UnixMilli())

	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("failed to get exchangeinfo for symbol %s", symbol)
		return false
	}
	log.PanicIfErrorUnderUTF = false
	ok = rs.placeOrderForUpc("isolated", info, 0, orderAmount, cid, orderSide, helper.OrderTypeMarket, 0, false)
	log.PanicIfErrorUnderUTF = true
	if !ok {
		rs.placeOrderForUpc("cross", info, 0, orderAmount, cid, orderSide, helper.OrderTypeMarket, 0, false)
	}
	return true
}

func (rs *OkxUsdtSwapRs) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	return rs.GetOrderListInFather(rs, startTimeMs, endTimeMs, orderState)
}
func (rs *OkxUsdtSwapRs) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/api/v5/trade/orders-pending"
	params := map[string]interface{}{"instType": "SWAP"}
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
		rs.cancelAllOpenOrders(only)
	}
	return
}

func (rs *OkxUsdtSwapRs) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	const _LEN = 100 //交易所对该字段最大约束为100
	uri := "/api/v5/trade/orders-history-archive"

	params := make(map[string]interface{})
	params["instType"] = "SWAP"
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
			order := helper.OrderForList{
				OrderID:       helper.MustGetStringFromBytes(v, "ordId"),
				ClientID:      helper.MustGetStringFromBytes(v, "clOrdId"),
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

func (rs *OkxUsdtSwapRs) GetFee() (fee helper.Fee, err helper.ApiError) {
	uri := "/api/v5/account/trade-fee"
	params := make(map[string]interface{})
	params["instType"] = "SWAP"
	//params["instId"] = b.symbol

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

		fee.Maker = math.Abs(helper.MustGetFloat64FromBytes(data[0], "makerU"))
		fee.Taker = math.Abs(helper.MustGetFloat64FromBytes(data[0], "takerU"))
	})
	return
	//return helper.Fee{Maker: b.makerFee.Load(), Taker: b.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (rs *OkxUsdtSwapRs) GetFundingRate() (helper.FundingRate, error) {
	// 请求必备变量
	url := "/api/v5/public/funding-rate"
	msg := map[string]interface{}{
		"instId": rs.Symbol,
	}
	var err error
	var value *fastjson.Value

	var fr helper.FundingRate
	_, err = rs.client.get(url, msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("failed to get funding rate. %v", err)
			return
		}
		if !rs.isOkApiResponse(value, url) {
			err = errors.New(string(respBody))
			return
		}
		item := value.GetArray("data")[0]
		fr.UpdateTimeMS = time.Now().UnixMilli()
		//okx chg certain symbol to bybit funding method
		// https://www.okx.com/docs-v5/en/#public-data-rest-api-get-funding-rate
		fr.FundingTimeMS = helper.MustGetInt64FromBytes(item, "fundingTime")
		fr.Rate = helper.MustGetFloat64FromBytes(item, "fundingRate")
		fr.NextFundingTimeMS = helper.MustGetInt64FromBytes(item, "nextFundingTime")
		if helper.MustGetStringFromBytes(item, "method") == "next_period" {
			fr.NextRate = helper.MustGetFloat64FromBytes(item, "nextFundingRate")
		}
		fr.Pair = rs.Pair
		fr.IntervalHours = int((fr.NextFundingTimeMS - fr.FundingTimeMS) / (3600 * 1000))
	})
	return fr, err
}
func (rs *OkxUsdtSwapRs) GetFundingRates(pairs []string) (res map[string]helper.FundingRate, err helper.ApiError) {
	// 请求必备变量
	if len(pairs) == 0 {
		rs.Logger.Error("Get Funding Rates cannot get all, must give pairs")
		return
	}
	// rate limit 20 time / 2 seconds
	res = make(map[string]helper.FundingRate, len(pairs))
	url := "/api/v5/public/funding-rate"
	for _, pair := range pairs {
		symb, ok := rs.ExchangeInfoPtrP2S.Get(pair)
		if !ok {
			rs.Logger.Error("Cannot find symbol name for pair: ", pair)
			return
		}
		msg := map[string]interface{}{
			"instId": symb.Symbol,
		}
		var fr helper.FundingRate
		_, err.NetworkError = rs.client.get(url, msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			var value *fastjson.Value
			p := handyPool.Get()
			defer handyPool.Put(p)
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				rs.Logger.Errorf("failed to get funding rate. %v", err.HandlerError)
				return
			}
			if !rs.isOkApiResponse(value, url) {
				err.HandlerError = errors.New(string(respBody))
				return
			}
			item := value.GetArray("data")[0]
			fr.UpdateTimeMS = time.Now().UnixMilli()
			fr.FundingTimeMS = helper.MustGetInt64FromBytes(item, "fundingTime")
			fr.Rate = helper.MustGetFloat64FromBytes(item, "fundingRate")
			fr.NextFundingTimeMS = helper.MustGetInt64FromBytes(item, "nextFundingTime")
			if helper.MustGetStringFromBytes(item, "method") == "next_period" {
				fr.NextRate = helper.MustGetFloat64FromBytes(item, "nextFundingRate")
			}
			fr.Pair = symb.Pair
			fr.IntervalHours = int((fr.NextFundingTimeMS - fr.FundingTimeMS) / (3600 * 1000))
			res[pair] = fr
		})
		time.Sleep(200 * time.Millisecond)
	}
	if helper.DEBUGMODE {
		rs.Logger.Info("Fundingrates", res)
	}
	return
}
func (rs *OkxUsdtSwapRs) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	a.Balances, _, err = rs.getEquity(false, true)
	if !err.Nil() {
		return
	}
	if !err.Nil() {
		return
	}
	a.Positions, err = rs.getPosition(rs.PairInfo, false, true)
	return
}

func (rs *OkxUsdtSwapRs) increaseId() int64 {
	rs.lk.Lock()
	defer rs.lk.Unlock()
	rs.id = (rs.id + 1) % 10000
	return rs.id
}

func (rs *OkxUsdtSwapRs) wsAmendOrder(info *helper.ExchangeInfo, cid, oid string, price float64, size fixed.Fixed, t int64) { //

	params := map[string]interface{}{
		"instId":    info.Symbol,
		"ordId":     oid,
		"clOrdId":   cid,
		"newPx":     helper.FixPrice(price, info.TickSize).String(),
		"newSz":     helper.FixAmount(size.Div(info.Multi), info.ContractSize), // 传入amount单位统一为币 下单时转换为张，支持0.x张
		"cxlOnFail": true,                                                      // 改单失败自动撤单
	}

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   fmt.Sprintf("%v%04d", oid, rs.increaseId()),
			"op":   "amend-order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error { // 目前改单失败，自动去尝试撤单
			rs.Logger.Debugf("amend order err. %v", msg)
			go rs.wsCancelOrder(info, cid, oid, 0)
			return nil
		},
	})
}

func (rs *OkxUsdtSwapRs) rsPlaceOrder(pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, needConv bool) {
	// 请求必备变量
	url := "/api/v5/trade/order"
	sz := size.String()
	if needConv {
		sz = helper.FixAmount(size.Div(pairInfo.Multi), pairInfo.ContractSize).String() // 传入amount单位统一为币 下单时转换为张
	}
	params := map[string]interface{}{
		"instId":  pairInfo.Symbol,
		"tdMode":  rs.marginMode,
		"clOrdId": cid,
		"sz":      sz,
	}

	if NETMODE {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["reduceOnly"] = true
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
		}
	} else {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
			params["posSide"] = "long"
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["posSide"] = "long"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
			params["posSide"] = "short"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["posSide"] = "short"
			params["reduceOnly"] = true
		}
	}
	switch orderType {
	case helper.OrderTypeLimit:
		params["ordType"] = "limit"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeIoc:
		params["ordType"] = "ioc"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeMarket:
		params["ordType"] = "market"
	case helper.OrderTypePostOnly:
		params["ordType"] = "post_only"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
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
			rs.Logger.Errorf("failed to parse. %v", err)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}
		if !rs.isOkApiResponse(value, url) {
			var order helper.OrderEvent
			order.Pair = pairInfo.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			err = errors.New(string(respBody))
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}

		for _, data := range helper.MustGetArray(value, "data") {
			var event helper.OrderEvent
			event.Pair = pairInfo.Pair
			sCode := helper.MustGetShadowStringFromBytes(data, "sCode")
			if sCode == "0" {
				event.Type = helper.OrderEventTypeNEW
				rs.ReqSucc(base.FailNumActionIdx_Place)
			} else {
				rs.ReqFail(base.FailNumActionIdx_Place)
				event.Type = helper.OrderEventTypeERROR
				event.ErrorReason = helper.MustGetStringFromBytes(data, "sMsg")
			}
			event.OrderID = helper.MustGetStringFromBytes(data, "ordId")
			event.ClientID = cid
			rs.Cb.OnOrder(0, event)
		}

	})
	return
}

func (rs *OkxUsdtSwapRs) rsCancelOrder(symbol string, cid, oid string, t int64) {
	url := "/api/v5/trade/cancel-order"
	params := map[string]interface{}{
		"instId": symbol,
	}
	if oid != "" {
		params["ordId"] = oid
	} else {
		params["clOrdId"] = cid
	}
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)
	var err error
	var value *fastjson.Value
	var handlerErr error
	start := time.Now().UnixMicro()
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)
	_, err = rs.client.post(url, params, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = handyPool.Get().ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("[%s] Failed to parse response for cancel order %s: %v", rs.ExchangeName, oid, handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			rs.Logger.Errorf("[%s] Failed to cancel order %s: %s", rs.ExchangeName, oid, value.String())
			return
		}
	})
	if err != nil {
		//得检查是否有限频提示
	}
	if handlerErr != nil {
		//忽略
	}
	if err == nil && handlerErr == nil {
		rs.CancelOrderPass.Update(time.Now().UnixMicro(), start)
	}
	return
}

func (rs *OkxUsdtSwapRs) placeOrderForUpc(marginMode string, pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, needConv bool) (ok bool) {
	// 请求必备变量
	url := "/api/v5/trade/order"
	sz := size.String()
	if needConv {
		sz = helper.FixAmount(size.Div(pairInfo.Multi), pairInfo.ContractSize).String() // 传入amount单位统一为币 下单时转换为张
	}
	params := map[string]interface{}{
		"instId":  pairInfo.Symbol,
		"tdMode":  marginMode,
		"clOrdId": cid,
		"sz":      sz,
	}

	if NETMODE {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["reduceOnly"] = true
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
		}
	} else {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
			params["posSide"] = "long"
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["posSide"] = "long"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
			params["posSide"] = "short"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["posSide"] = "short"
			params["reduceOnly"] = true
		}
	}
	switch orderType {
	case helper.OrderTypeLimit:
		params["ordType"] = "limit"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeIoc:
		params["ordType"] = "ioc"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeMarket:
		params["ordType"] = "market"
	case helper.OrderTypePostOnly:
		params["ordType"] = "post_only"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
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
			rs.Logger.Errorf("failed to parse. %v", err)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}
		if !rs.isOkApiResponse(value, url) {
			var order helper.OrderEvent
			order.Pair = pairInfo.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			err = errors.New(string(respBody))
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}

		for _, data := range helper.MustGetArray(value, "data") {
			var event helper.OrderEvent
			event.Pair = pairInfo.Pair
			sCode := helper.MustGetShadowStringFromBytes(data, "sCode")
			if sCode == "0" {
				event.Type = helper.OrderEventTypeNEW
				rs.ReqSucc(base.FailNumActionIdx_Place)
				ok = true
			} else {
				rs.ReqFail(base.FailNumActionIdx_Place)
				event.Type = helper.OrderEventTypeERROR
				event.ErrorReason = helper.MustGetStringFromBytes(data, "sMsg")
			}
			event.OrderID = helper.MustGetStringFromBytes(data, "ordId")
			event.ClientID = cid
			rs.Cb.OnOrder(0, event)
		}

	})
	return
}
func (rs *OkxUsdtSwapRs) wsPlaceOrder(pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, needConv bool) {
	sz := size.String()
	if needConv {
		sz = helper.FixAmount(size.Div(pairInfo.Multi), pairInfo.ContractSize).String() // 传入amount单位统一为币 下单时转换为张
	}
	params := map[string]interface{}{
		"tdMode":  rs.marginMode,
		"instId":  pairInfo.Symbol,
		"clOrdId": cid,
		"sz":      sz,
	}
	if NETMODE {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["reduceOnly"] = true
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
		}
	} else {
		switch side {
		case helper.OrderSideKD:
			params["side"] = "buy"
			params["posSide"] = "long"
		case helper.OrderSidePD:
			params["side"] = "sell"
			params["posSide"] = "long"
			params["reduceOnly"] = true
		case helper.OrderSideKK:
			params["side"] = "sell"
			params["posSide"] = "short"
		case helper.OrderSidePK:
			params["side"] = "buy"
			params["posSide"] = "short"
			params["reduceOnly"] = true
		}
	}
	switch orderType {
	case helper.OrderTypeLimit:
		params["ordType"] = "limit"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeIoc:
		params["ordType"] = "ioc"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	case helper.OrderTypeMarket:
		params["ordType"] = "market"
	case helper.OrderTypePostOnly:
		params["ordType"] = "post_only"
		params["px"] = helper.FixPrice(price, pairInfo.TickSize).String()
	}
	//
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   fmt.Sprintf("%v%04d", cid, rs.increaseId()),
			"op":   "order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error {
			var order helper.OrderEvent
			order.Pair = pairInfo.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			return nil
		},
	})
}

func (rs *OkxUsdtSwapRs) wsCancelOrder(info *helper.ExchangeInfo, cid, oid string, t int64) {
	params := map[string]interface{}{
		"instId":  info.Symbol,
		"ordId":   oid,
		"clOrdId": cid,
	}

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":   fmt.Sprintf("%v%04d", oid, rs.increaseId()),
			"op":   "cancel-order",
			"args": []interface{}{params},
		},
		Cb: func(msg map[string]interface{}) error {
			return nil
		},
	})
}

func (rs *OkxUsdtSwapRs) checkOrder(info *helper.ExchangeInfo, oid, cid string) {
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
		switch code {
		case "0":
			for _, data := range value.GetArray("data") {
				// instId := helper.BytesToString(data.GetStringBytes("instId"))
				// info, ok := rs.GetPairInfoBySymbol(instId)
				// if !ok {
				// rs.Logger.Errorf("unknow symbol when parse order event. %s", instId)
				// continue
				// }
				order := helper.OrderEvent{Pair: info.Pair}
				order.ClientID = string(data.GetStringBytes("clOrdId"))
				order.OrderID = string(data.GetStringBytes("ordId"))
				state := helper.BytesToString(data.GetStringBytes("state"))
				switch state {
				case "canceled", "filled":
					order.Type = helper.OrderEventTypeREMOVE
				case "partially_filled":
					order.Type = helper.OrderEventTypePARTIAL
				default:
					order.Type = helper.OrderEventTypeNEW
				}
				sz := helper.BytesToString(data.GetStringBytes("accFillSz"))
				if sz != "" && sz != "0" {
					order.Filled = fixed.NewS(sz).Mul(info.Multi)
					order.FilledPrice = fixed.NewS(helper.BytesToString(data.GetStringBytes("avgPx"))).Float()
					//order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))))
					order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))))
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
				if helper.MustGetShadowStringFromBytes(data, "reduceOnly") == "true" {
					if order.OrderSide == helper.OrderSideKD {
						order.OrderSide = helper.OrderSidePK
					} else if order.OrderSide == helper.OrderSideKK {
						order.OrderSide = helper.OrderSidePD
					}
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
		case "51603":
			//  {\"code\":\"51603\",\"data\":[],\"msg\":\"Order does not exist\"}"}
			order := helper.OrderEvent{
				Type:     helper.OrderEventTypeNotFound,
				Pair:     info.Pair,
				ClientID: cid,
				OrderID:  oid,
			}
			rs.Cb.OnOrder(0, order)
		default:
			rs.Logger.Warnf("查单失败 %v", value)
		}
	})
	if err != nil {
		rs.Logger.Errorf("查单失败 网络错误 %v", err)
	}
}

// needMulti 是否需要处理合约乘数
func (rs *OkxUsdtSwapRs) getPosition(pairinfo *helper.ExchangeInfo, only bool, needMulti bool) (resp []helper.PositionSum, err helper.ApiError) {
	msg := map[string]interface{}{}
	if only {
		msg["instId"] = pairinfo.Symbol
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/account/positions", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("获取持仓 解析错误 %v", err.HandlerError)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			for _, data := range value.GetArray("data") {
				instId := helper.BytesToString(data.GetStringBytes("instId"))
				if only && instId != pairinfo.Symbol {
					continue
				}

				ps := helper.BytesToString(data.GetStringBytes("posSide"))
				px := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("avgPx")))
				sz := fixed.NewS(helper.MustGetShadowStringFromBytes(data, "pos"))
				upl := fixed.NewS(helper.MustGetShadowStringFromBytes(data, "upl"))
				side := helper.PosSideLong
				switch ps {
				case "short":
					side = helper.PosSideShort
				case "net":
					if sz.LessThan(fixed.ZERO) {
						side = helper.PosSideShort
					}
				}

				info, ok := rs.ExchangeInfoPtrS2P.Get(instId)
				if !ok {
					rs.Logger.Error("not found exchange info for symbol %s", instId)
					continue
				}
				timeInEx := helper.MustGetInt64FromBytes(data, "uTime")
				tsns := time.Now().UnixNano()
				szInResp := sz
				if needMulti {
					szInResp = sz.Mul(info.Multi)
				}
				resp = append(resp, helper.PositionSum{
					Name:        info.Pair.String(),
					Amount:      szInResp.Abs().Float(),
					AvailAmount: szInResp.Abs().Float(),
					Ave:         px,
					Side:        side,
					Mode:        helper.PosModeOneway,
					Seq:         helper.NewSeq(timeInEx, tsns),
					Pnl:         upl.Float(),
				})

				if pos, ok := rs.PosPureNewerAndStore(instId, timeInEx, tsns); ok {
					pos.Lock.Lock()
					sz = sz.Mul(info.Multi)
					switch ps {
					case "long":
						pos.LongPos = sz
						pos.LongAvg = px
					case "short":
						pos.ShortPos = sz.Abs()
						pos.ShortAvg = px
					case "net":
						pos.ResetLocked()
						if sz.GreaterThan(fixed.ZERO) {
							pos.LongPos = sz
							pos.LongAvg = px
						} else {
							pos.ShortPos = sz.Abs()
							pos.ShortAvg = px
						}
					}
					event := pos.ToPositionEvent()
					pos.Lock.Unlock()
					rs.Cb.OnPositionEvent(0, event)
				}
			}
		default:
			rs.Logger.Warnf("获取持仓失败 %v", value)
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("获取持仓失败 网络错误 %v", err)
	}
	return
}

func (rs *OkxUsdtSwapRs) getIndex() {
	msg := map[string]interface{}{
		"instId": strings.ToUpper(rs.Pair.Base) + "-" + strings.ToUpper(rs.Pair.Quote),
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err := rs.client.get("/api/v5/market/index-tickers", msg, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Warnf("[%s]get idx price failed. %s", rs.ExchangeName, handlerErr.Error())
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			data := value.GetArray("data")[0]
			indexPrice := helper.MustGetFloat64FromBytes(data, "idxPx")
			rs.Cb.OnIndex(0, helper.IndexEvent{IndexPrice: indexPrice})
		default:
			rs.Logger.Warnf("获取指数价格失败 %v", value)
		}
	})
	if err != nil {
		rs.Logger.Errorf("获取指数价格失败 网络错误 %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	_, err = rs.client.get("/api/v5/public/mark-price", msg, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Warnf("[%s]get idx price failed. %s", rs.ExchangeName, handlerErr.Error())
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			data := value.GetArray("data")[0]
			markPx := helper.MustGetFloat64FromBytes(data, "markPx")
			rs.Cb.OnMark(0, helper.MarkEvent{MarkPrice: markPx})
		default:
			rs.Logger.Warnf("获取指数价格失败 %v", value)
		}
	})
	if err != nil {
		rs.Logger.Errorf("获取指数价格失败 网络错误 %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
}
func (rs *OkxUsdtSwapRs) GetIndexs(pairs []string) (res map[string]float64, err helper.ApiError) {
	msg := map[string]interface{}{
		"quoteCcy": strings.ToUpper(rs.Pair.Quote),
	}
	p := handyPool.Get()
	var value *fastjson.Value
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/market/index-tickers", msg, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Warnf("[%s]get idx price failed. %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			datas := helper.MustGetArray(value, "data")
			if len(pairs) == 0 {
				res = make(map[string]float64, rs.ExchangeInfoPtrS2P.Count())
				for _, data := range datas {
					pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "instId") + "-SWAP")
					if ok {
						res[pair.Pair.String()] = helper.MustGetFloat64FromBytes(data, "idxPx")
					}
				}
			} else {
				res = make(map[string]float64, len(pairs))
				for _, p := range pairs {
					res[p] = 0
				}
				for _, data := range datas {
					pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "instId") + "-SWAP")
					if ok {
						if _, ok := res[pair.Pair.String()]; ok {
							res[pair.Pair.String()] = helper.MustGetFloat64FromBytes(data, "idxPx")
						}
					}
				}
			}
		default:
			rs.Logger.Warnf("获取指数价格失败 %v", value)
		}
	})
	if err.HandlerError != nil || err.NetworkError != nil {
		rs.Logger.Errorf("获取指数价格失败 网络错误 %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	if helper.DEBUGMODE {
		rs.Logger.Info("Indexs", res)
	}
	return
}
func (rs *OkxUsdtSwapRs) getEquity(only bool, wantSum bool) (sum []helper.BalanceSum, resp helper.Equity, err helper.ApiError) {
	base.EnsureIsRsExposerOuter(rs)
	msg := map[string]interface{}{}
	if only {
		msg["ccy"] = strings.ToUpper(rs.Pair.Quote)
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/account/balance", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("获取资金 解析错误 %v", err.HandlerError)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		switch code {
		case "0":
			for _, data := range value.GetArray("data") {
				details := data.GetArray("details")
				for _, v := range details {
					upnl := helper.GetFloat64FromBytes(v, "upl")
					total := helper.MustGetFloat64FromBytes(v, "cashBal") + upnl
					avail := helper.MustGetFloat64FromBytes(v, "availBal")
					asset := helper.MustGetStringLowerFromBytes(v, "ccy")
					seqEx := helper.MustGetInt64FromBytes(data, "uTime")

					if wantSum && total != 0 {
						price := helper.MustGetFloat64FromBytes(v, "eqUsd") / helper.MustGetFloat64FromBytes(v, "eq")
						if asset == "usdt" || asset == "busd" || asset == "usdc" || asset == "fdusd" {
							price = 1
						}
						sum = append(sum, helper.BalanceSum{
							Name:   asset,
							Price:  price,
							Amount: total,
							Avail:  avail,
						})
					}

					var fieldsSet helper.FieldsSet_T
					fieldsSet = (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)
					if e, ok := rs.EquityNewerAndStore(asset, seqEx, time.Now().UnixNano(), fieldsSet); ok {
						e.TotalWithUpl = total
						e.TotalWithoutUpl = total - upnl
						e.Avail = avail
						e.Upl = upnl

						rs.Cb.OnEquityEvent(0, *e)
					}
					resp = helper.Equity{
						Cash:     total,
						CashFree: avail,
						CashUpl:  upnl,
						IsSet:    true,
					}
				}
			}
		default:
			rs.Logger.Warnf("获取资金失败 %v", value)
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("获取资金失败 网络错误 %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	return
}
func (rs *OkxUsdtSwapRs) GetEquity() (resp helper.Equity, err helper.ApiError) {
	_, resp, err = rs.getEquity(false, false)
	return
}
func (rs *OkxUsdtSwapRs) getTicker(symbol string) (resp helper.Ticker, err helper.ApiError) {
	msg := map[string]interface{}{
		"instId": symbol,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/market/books", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("获取ticker 解析错误 %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, "get_ticker") {
			err.HandlerError = fmt.Errorf("获取ticker 失败 %v", value)
			return
		}
		// body: {"code":"0","msg":"","data":[{"asks":[["183.64","36.85","0","9"]],"bids": [["183.63","167.07","0","10"]],"ts": "1730883958551"}]}
		for _, data := range value.GetArray("data") {
			if ts, err := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); err == nil {
				t := &rs.TradeMsg.Ticker
				asks := data.GetArray("asks")
				bids := data.GetArray("bids")

				if len(asks) == 0 || len(bids) == 0 {
					rs.Logger.Warnf("获取ticker失败 %v", data)
					return
				}

				info, _ := rs.ExchangeInfoPtrS2P.Get(symbol)
				ap := helper.MustGetFloat64FromBytes(asks[0], "0")
				aq := helper.MustGetFloat64FromBytes(asks[0], "1") * info.Multi.Float()
				bp := helper.MustGetFloat64FromBytes(bids[0], "0")
				bq := helper.MustGetFloat64FromBytes(bids[0], "1") * info.Multi.Float()

				if symbol == rs.Symbol {
					if t.Seq.NewerAndStore(ts, time.Now().UnixNano()) {
						t.Set(ap, aq, bp, bq)
					}
					rs.Cb.OnTicker(0)
				}
				resp.Set(ap, aq, bp, bq)
				resp.Seq.Ex.Store(ts)
			}
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("获取ticker失败 网络错误 %v", err)
	}
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
func (rs *OkxUsdtSwapRs) SendSignal(signals []helper.Signal) {
	for i, s := range signals {
		// if rs.isAlive && rs.logged { // 可操作的状态
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
				rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true)
			} else {
				rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true)
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
			if s.SignalChannelType == helper.SignalChannelTypeRs || rs.BrokerConfig.BanRsWs {
				rs.rsCancelOrder(info.Symbol, s.ClientID, s.OrderID, s.Time)
			} else {
				rs.wsCancelOrder(info, s.ClientID, s.OrderID, s.Time)
			}
		case helper.SignalTypeCheckOrder:
			info, ok := rs.GetPairInfoByPair(&s.Pair)
			if !ok {
				rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
				continue
			}
			go rs.checkOrder(info, s.OrderID, s.ClientID)
		case helper.SignalTypeGetPos:
			info, ok := rs.GetPairInfoByPair(&s.Pair)
			if !ok {
				rs.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
				continue
			}
			go rs.getPosition(info, true, true)
		case helper.SignalTypeGetEquity:
			go rs.GetEquity()
		case helper.SignalTypeGetIndex:
			go rs.getIndex()
		case helper.SignalTypeGetTicker:
			go rs.getTicker(rs.Symbol)
		case helper.SignalTypeCancelOne:
			go rs.cancelAllOpenOrders(true)
		case helper.SignalTypeCancelAll:
			go rs.cancelAllOpenOrders(false)
			// }
			// } else {
			// rs.Logger.Errorf("send signal err! alive %v logged %v signal %v", rs.isAlive, rs.logged, s)
			// 对于下单，就构造一个下单失败立刻返回
			// switch s.Type {
			// case helper.SignalTypeNewOrder:
			// var order helper.OrderEvent
			// order.Type = helper.OrderEventTypeERROR
			// order.ClientID = s.ClientID
			// rs.Cb.OnOrder(0, order)
			// }
		}
	}
}

// Do 发起任意请求 一般用于非交易任务 对时间不敏感
func (rs *OkxUsdtSwapRs) Do(string, any) (any, error) {
	return nil, nil
}

func (rs *OkxUsdtSwapRs) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltNil | base.AbltWsPriEquityAvailReducedWhenHold
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
func (rs *OkxUsdtSwapRs) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SWAP | base.ABILITIES_SEQ | base.AbltRsPriGetPosWithSeq | base.AbltRsPriGetAllIndex | base.AbltOrderAmend | base.AbltOrderAmendByCid
}

func (rs *OkxUsdtSwapRs) WsLogged() bool {
	return rs.logged
}

// 获取全市场交易规则
func (rs *OkxUsdtSwapRs) GetExchangeInfos() []helper.ExchangeInfo {
	_, res := rs.client.getExchangeInfo(rs.PairInfo, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S)
	return res
}

func (rs *OkxUsdtSwapRs) GetFeatures() base.Features {
	f := base.Features{
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		UpdateWsTickerWithSeq: true,
		GetFee:                true,
		GetOrderList:          true,
		GetFundingRate:        true,
		GetTickerSignal:       true,
		MultiSymbolOneAcct:    true,
		OrderIOC:              true,
		OrderPostonly:         true,
		UnifiedPosClean:       true,
		Partial:               true,
		DelayInTicker:         true,
		UpdatePosWithSeq:      true,
		WsDepthLevel:          true,
		GetPriWs:              true,
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}
func (rs *OkxUsdtSwapRs) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	return rs.DoGetPendingOrders("")
}

func (rs *OkxUsdtSwapRs) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	params := map[string]interface{}{"instType": "SWAP"}
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
				reduce := data.GetBool("reduceOnly")
				switch helper.MustGetShadowStringFromBytes(data, "side") {
				case "buy":
					if reduce {
						o.OrderSide = helper.OrderSidePK
					} else {
						o.OrderSide = helper.OrderSideKD
					}
				case "sell":
					if reduce {
						o.OrderSide = helper.OrderSidePD
					} else {
						o.OrderSide = helper.OrderSideKK
					}
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

func (rs *OkxUsdtSwapRs) DoGetDepth(info *helper.ExchangeInfo) (respDepth helper.Depth, err helper.ApiError) {
	msg := map[string]interface{}{
		"instId": info.Symbol,
		"sz":     400,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/market/books", msg, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("获取ticker 解析错误 %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, "get_ticker") {
			err.HandlerError = fmt.Errorf("获取ticker 失败 %v", value)
			return
		}
		// body: {"code":"0","msg":"","data":[{"asks":[["183.64","36.85","0","9"]],"bids": [["183.63","167.07","0","10"]],"ts": "1730883958551"}]}
		for _, data := range value.GetArray("data") {
			a := data.GetArray("asks")
			b := data.GetArray("bids")

			if len(a) == 0 || len(b) == 0 {
				rs.Logger.Warnf("获取ticker失败 %v", data)
				return
			}

			if len(a) > 0 && len(b) > 0 {
				for _, bid := range b {
					_bidPrice := helper.MustGetFloat64FromBytes(bid, "0")
					_bidQty := helper.MustGetFloat64FromBytes(bid, "1") * info.Multi.Float()
					respDepth.Bids = append(respDepth.Bids,
						helper.DepthItem{Price: _bidPrice, Amount: _bidQty})
				}
				for _, ask := range a {
					_askPrice := helper.MustGetFloat64FromBytes(ask, "0")
					_askQty := helper.MustGetFloat64FromBytes(ask, "1") * info.Multi.Float()
					respDepth.Asks = append(respDepth.Asks,
						helper.DepthItem{Price: _askPrice, Amount: _askQty})
				}
			}
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("获取ticker失败 网络错误 %v", err)
	}
	return
}
func (rs *OkxUsdtSwapRs) DoGetOI(info *helper.ExchangeInfo) (oi float64, err helper.ApiError) {
	msg := map[string]interface{}{
		"instId": info.Symbol,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, err.NetworkError = rs.client.get("/api/v5/public/open-interest", msg, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("获取ticker 解析错误 %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, "get_ticker") {
			err.HandlerError = fmt.Errorf("获取ticker 失败 %v", value)
			return
		}
		for _, data := range value.GetArray("data") {
			oi = helper.MustGetFloat64FromBytes(data, "oiUsd")
			return
		}
	})
	if !err.Nil() {
		rs.Logger.Errorf("获取ticker失败 网络错误 %v", err)
	}
	return
}

func (rs *OkxUsdtSwapRs) GetPosHist(startTimeMs int64, endTimeMs int64) (resp []helper.Positionhistory, err helper.ApiError) {
	res, err := rs.DoGetPosHist(startTimeMs, endTimeMs)
	resp = res.Pos
	return
}

func (rs *OkxUsdtSwapRs) DoGetPosHist(startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
	if rs.SymbolMode == helper.SymbolMode_All {
		resp, err = rs.doGetPosHist("", startTimeMs, endTimeMs)
	} else {
		for _, p := range rs.BrokerConfig.Pairs {
			symbol := rs.PairToSymbol(&p)
			if symbol == "" {
				continue
			}
			o, _ := rs.doGetPosHist(symbol, startTimeMs, endTimeMs)
			resp.Pos = append(resp.Pos, o.Pos...)
		}
		sort.Slice(resp.Pos, func(i, j int) bool {
			return resp.Pos[i].CloseTime > resp.Pos[j].CloseTime
		})
	}
	resp.HasMore = false
	return
}

func (rs *OkxUsdtSwapRs) doGetPosHist(symbol string, startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
	uri := "/api/v5/account/positions-history"
	params := make(map[string]interface{})
	if symbol != "" {
		params["instId"] = symbol
	}
	params["instType"] = "SWAP"
	params["before"] = strconv.FormatInt(endTimeMs, 10)
	params["after"] = strconv.FormatInt(startTimeMs, 10)

	_, err.NetworkError = rs.client.get(uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handler error %v", err)
			return
		}
		// 检查响应码，0表示成功
		if value.GetInt("code") != 0 {
			err.HandlerError = fmt.Errorf("handler error code %v, msg: %s", value.GetInt("code"), string(value.GetStringBytes("msg")))
			return
		}
		// 本接口返回data为数组
		rowsVal := value.GetArray("data")
		resp.Pos = make([]helper.Positionhistory, 0)
		for _, v := range rowsVal {
			// 解析时间: cTime 为开仓时间, uTime 为平仓时间（均为字符串形式的毫秒时间戳）
			openTimeStr := string(v.GetStringBytes("cTime"))
			closeTimeStr := string(v.GetStringBytes("uTime"))
			openTimeMs, _ := strconv.ParseInt(openTimeStr, 10, 64)
			closeTimeMs, _ := strconv.ParseInt(closeTimeStr, 10, 64)
			// 解析价格
			openPrice := helper.GetFloat64FromBytes(v, "avgPx")
			closePrice := helper.GetFloat64FromBytes(v, "closePx")
			// 解析成交量
			closedSize := helper.GetFloat64FromBytes(v, "closeTotalPos")
			// 根据 posSide判断交易方向，long为正，short为负
			posSide := string(v.GetStringBytes("posSide"))
			multiplier := 1.0
			if posSide == "short" {
				multiplier = -1.0
			}
			// 解析盈亏及费用
			realizedPnl := helper.GetFloat64FromBytes(v, "realizedPnl")
			fee := helper.GetFloat64FromBytes(v, "fee")
			fundingFee := helper.GetFloat64FromBytes(v, "fundingFee")
			liqPenalty := helper.GetFloat64FromBytes(v, "liqPenalty")
			totalFee := fee + fundingFee + liqPenalty
			// 获取交易对信息
			instId := string(v.GetStringBytes("instId"))
			info, ok := rs.ExchangeInfoPtrS2P.Get(instId)
			if !ok {
				rs.Logger.Errorf("not found pair for symbol %s", instId)
				continue
			}
			deal := helper.Positionhistory{
				Symbol:        instId,
				Pair:          info.Pair.String(),
				OpenTime:      openTimeMs,
				CloseTime:     closeTimeMs,
				OpenPrice:     openPrice,
				CloseAvePrice: closePrice,
				OpenedAmount:  0, // 当前接口无相关数据
				ClosedAmount:  closedSize * multiplier,
				PnlAfterFees:  realizedPnl,
				Fee:           totalFee,
			}
			resp.Pos = append(resp.Pos, deal)
		}
	})
	return
}

func (rs *OkxUsdtSwapRs) GetMMR() (res helper.UMAcctInfo) {
	msg := map[string]interface{}{}
	msg["ccy"] = strings.ToUpper(rs.Pair.Quote)
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, e := rs.client.get("/api/v5/account/balance", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, e := p.ParseBytes(respBody)
		if e != nil {
			rs.Logger.Errorf("获取资金 解析错误 %v", e)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "0" {
			rs.Logger.Warnf("获取资金失败 %v", value)
			return
		}
		data := value.GetArray("data")[0]
		res.TsS = time.Now().Unix()
		res.TotalEquity = helper.MustGetFloat64FromBytes(data, "totalEq")
		res.DiscountedTotalEquity = helper.MustGetFloat64FromBytes(data, "adjEq")
		res.TotalMaintenanceMargin = helper.MustGetFloat64FromBytes(data, "mmr")
		res.MaintenanceMarginRate = res.TotalMaintenanceMargin / res.DiscountedTotalEquity
	})
	if e != nil {
		rs.Logger.Errorf("获取资金失败 网络错误 %v", e)
	}
	return
}
func (rs *OkxUsdtSwapRs) PlaceOrderSpotUsdtSize(pair helper.Pair, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64) {
}
func (rs *OkxUsdtSwapRs) CancelSpotPendingOrder(pair helper.Pair) (err helper.ApiError) {
	return
}
func (rs *OkxUsdtSwapRs) Spot7DayTradeHist() (resp []helper.DealForList, err helper.ApiError) {
	return
}
