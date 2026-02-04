package binance_spot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"sync"

	"actor/broker/base"
	"actor/broker/base/base_orderbook"
	"actor/broker/brokerconfig"
	"actor/broker/client/rest"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/log"
	"actor/tools"
	jsoniter "github.com/json-iterator/go"

	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"go.uber.org/atomic"
)

const (
	BaseUrl    = "https://api.binance.com"
	WsOrderUrl = "wss://ws-api.binance.com:443/ws-api/v3?returnRateLimits=false"
)

var (
	BaseUrlColo    = ""
	WsOrderUrlColo = ""
)

var (
	handyPool fastjson.ParserPool
)

type BinanceSpotRs struct {
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyGetPriWs
	// base.DummyDoSetLeverage
	base.DummyDoGetPriceLimit
	base.DummyDoForSpot
	base.DummyDoAmendOrderRsColo
	base.DummyDoAmendOrderRsNor
	base.DummyDoAmendOrderWsColo
	base.DummyDoAmendOrderWsNor

	client     *rest.Client // 通用rest客户端
	clientColo *rest.Client // 通用rest客户端
	rsColoOn   bool
	failNum    atomic.Int64 // 出错次数
	baseUrl    string       // 基础host
	// 适用于pair的交易规则 避免每次使用都要查询
	// binance 限频规则
	request_weight_limit    int
	order_limit_10s         int
	order_limit_1day        int
	raw_requests_limit_5min int
	//
	takerFee atomic.Float64 // taker费率
	makerFee atomic.Float64 // maker费率
	// WS
	priWs       *ws.WS        // 继承ws
	connectOnce sync.Once     // 连接一次
	stopCPri    chan struct{} // 接受停机信号
	// WS Colo
	wsColoOn        bool
	priWsColo       *ws.WS        // 继承ws
	connectOnceColo sync.Once     // 连接一次
	stopCPriColo    chan struct{} // 接受停机信号
	//
	stopCtx  context.Context
	stopFunc context.CancelFunc
	//
	wsReqId2ClientIdMap cmap.ConcurrentMap[int64, helper.CidWithPair] // 暂时忽略有个别下单但交易所没有推送的本地清理工作，交易所更新后这个可能不需要用
}

// 创建新的实例
func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	baseUrl := BaseUrl
	rs := &BinanceSpotRs{
		client:     rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		clientColo: rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		baseUrl:    baseUrl,
		// 限频规则
		request_weight_limit:    1180,   // 阈值1200
		order_limit_10s:         48,     //  阈值50
		order_limit_1day:        158000, //  160000
		raw_requests_limit_5min: 6000,   // 阈值6100
		wsReqId2ClientIdMap:     cmap.NewWithCustomShardingFunction[int64, helper.CidWithPair](func(key int64) uint32 { return uint32(key) }),
	}
	rs.Cb.OnOrder = func(ts int64, event helper.OrderEvent) {
		if !rs.DelayMonitor.IsMonitorOrder(event.ClientID) {
			cb.OnOrder(ts, event)
		} else {
			rs.Logger.Errorf("monitor order triggered on order %v", event)
		}
	}

	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, pairInfo, cb)

	// colo url 配置
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BinanceUsdtSwapRestUrl != "" {
		rs.rsColoOn = true
		BaseUrlColo = cfg.BinanceUsdtSwapRestUrl
		params.Logger.Infof("binance_usdt_swap rs 启用colo  %v", BaseUrlColo)
	}

	if !params.BanColo && cfg.BinanceUsdtSwapWsUrl != "" && false { //BinanceUsdtSwapWsUrl is not for rs
		rs.wsColoOn = true
		WsOrderUrlColo = cfg.BinanceUsdtSwapWsUrl
		params.Logger.Infof("binance_usdt_swap rs'ws 启用colo  %v", WsOrderUrlColo)
	}
	// WS
	rs.stopCtx, rs.stopFunc = context.WithCancel(context.Background())

	//Delay monitor
	if params.ActivateDelayMonitor {
		influxCfg := base.DefaultInfluxConfig(params)
		base.InitDelayMonitor(&rs.DelayMonitor, &rs.FatherRs, influxCfg, rs.GetExName(), rs.Pair.String(), rs.MonitorTrigger)
	}
	rs.SetSelectedLine(base.LinkType_Nor, base.ClientType_Rs)
	rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Nil})
	if rs.rsColoOn {
		rs.SetSelectedLine(base.LinkType_Colo, base.ClientType_Rs)
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Nil})
	}
	// WS
	if !params.BanRsWs {
		rs.ReqWsNorLogged.Store(true)
		rs.stopCtx, rs.stopFunc = context.WithCancel(context.Background())
		rs.DoCreateReqWsNor()
		rs.SetSelectedLine(base.LinkType_Nor, base.ClientType_Ws)
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Nil})

		if rs.wsColoOn {
			rs.ReqWsColoLogged.Store(true)
			rs.DoCreateReqWsColo()
			rs.SetSelectedLine(base.LinkType_Colo, base.ClientType_Ws)
			rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Nil})
		}
	}
	rs.ChoiceBestLine()
	return rs
}

func (rs *BinanceSpotRs) getOrderbookSnap() (*base_orderbook.Slot, error) {
	uri := "/api/v3/depth"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	params["limit"] = 1000
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	// 待使用数据结构
	slot := &base_orderbook.Slot{}
	// 发起请求
	err := rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			helper.LogErrorThenCall(fmt.Sprintf("[%s]获取snap失败 需要停机. %s", rs.ExchangeName, handlerErr.Error()), rs.Cb.OnExit)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			helper.LogErrorThenCall(fmt.Sprintf("[%s]获取snap失败 需要停机. %s", rs.ExchangeName, string(respBody)), rs.Cb.OnExit)
			return
		}
		slot.ExTsMs = time.Now().UnixMilli() - 50
		slot.ExPrevLastId = helper.MustGetInt64(value, "lastUpdateId")
		bids := helper.MustGetArray(value, "bids")
		asks := helper.MustGetArray(value, "asks")
		slot.PriceItems = make([][2]float64, 0, len(bids)+len(asks))
		slot.AskStartIdx = len(bids)
		for _, bid := range bids {
			pa := helper.MustGetArray(bid)
			slot.PriceItems = append(slot.PriceItems, [2]float64{helper.MustGetFloat64FromBytes(pa[0]), helper.MustGetFloat64FromBytes(pa[1])})
		}
		for _, ask := range asks {
			pa := helper.MustGetArray(ask)
			slot.PriceItems = append(slot.PriceItems, [2]float64{helper.MustGetFloat64FromBytes(pa[0]), helper.MustGetFloat64FromBytes(pa[1])})
		}
	}, false)

	if err != nil {
		rs.Logger.Errorf("failed to get orderbook snap, %v", err)
		return nil, err
	}

	return slot, nil
}

func (rs *BinanceSpotRs) DoPlaceOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, rs.rsColoOn)
}
func (rs *BinanceSpotRs) DoPlaceOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
}
func (rs *BinanceSpotRs) DoCancelOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsCancelOrder(info, s.OrderID, s.ClientID, s.Time, rs.rsColoOn)
}
func (rs *BinanceSpotRs) DoCancelOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsCancelOrder(info, s.OrderID, s.ClientID, s.Time, false)
}
func (rs *BinanceSpotRs) DoPlaceOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, rs.wsColoOn)
}
func (rs *BinanceSpotRs) DoPlaceOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
}
func (rs *BinanceSpotRs) DoCancelOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsCancelOrder(info, s.OrderID, s.ClientID, s.Time, rs.wsColoOn)
}
func (rs *BinanceSpotRs) DoCancelOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsCancelOrder(info, s.OrderID, s.ClientID, s.Time, false)
}

func (rs *BinanceSpotRs) DoCreateReqWsNor() error {
	rs.priWs = ws.NewWS(WsOrderUrl, rs.BrokerConfig.LocalAddr, rs.BrokerConfig.ProxyURL, rs.priHandler, rs.Cb.OnExit, rs.BrokerConfig.BrokerConfig)
	rs.priWs.SetPongFunc(pong)
	return nil
}

func (rs *BinanceSpotRs) DoCreateReqWsColo() error {
	rs.priWs = ws.NewWS(WsOrderUrlColo+"?returnRateLimits=false", rs.BrokerConfig.LocalAddr, rs.BrokerConfig.ProxyURL, rs.priHandler, rs.Cb.OnExit, rs.BrokerConfig.BrokerConfig)
	rs.priWsColo.SetPongFunc(pong)
	return nil
}

// Run 准备ws连接 仅第一次调用时连接ws
func (rs *BinanceSpotRs) Run() {
	rs.connectOnce.Do(func() {
		if rs.priWs != nil {
			var err2 error
			rs.stopCPri, err2 = rs.priWs.Serve()
			if err2 != nil {
				rs.Cb.OnExit("binance usdt swap private ws 连接失败")
				return
			}
		}
	})
	rs.connectOnceColo.Do(func() {
		if rs.priWsColo != nil {
			var err2 error
			rs.stopCPriColo, err2 = rs.priWsColo.Serve()
			if err2 != nil {
				rs.Cb.OnExit("binance usdt swap private ws colo 连接失败")
				return
			}
		}
	})
	rs.DelayMonitor.Run()
}

func (rs *BinanceSpotRs) DoStop() {
	if rs.stopCPri != nil {
		rs.stopCPri <- struct{}{}
	}
	rs.connectOnce = sync.Once{}
	if rs.stopCPriColo != nil {
		rs.stopCPriColo <- struct{}{}
	}
	rs.stopFunc()
	rs.connectOnceColo = sync.Once{}
}

func (rs *BinanceSpotRs) wsPlaceOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, colo bool) {
	params := map[string]interface{}{}
	params["symbol"] = info.Symbol
	switch side {
	case helper.OrderSideKD:
		params["side"] = "BUY"
	case helper.OrderSideKK:
		params["side"] = "SELL"
	case helper.OrderSidePD:
		params["side"] = "SELL"
	case helper.OrderSidePK:
		params["side"] = "BUY"
	}
	if orderType == helper.OrderTypeIoc {
		params["type"] = "LIMIT"
		params["timeInForce"] = "IOC"
	} else if orderType == helper.OrderTypePostOnly {
		params["type"] = "LIMIT_MAKER"
	} else if orderType == helper.OrderTypeMarket {
		params["type"] = "MARKET"
	} else if orderType == helper.OrderTypeLimit {
		params["type"] = "LIMIT"
		params["timeInForce"] = "GTC"
	} else {
		if !rs.DelayMonitor.IsMonitorOrder(cid) {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
		}
		log.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
	}

	if orderType != helper.OrderTypeMarket {
		params["price"] = helper.FixPrice(price, info.TickSize)
	}
	params["quantity"] = size.String()
	params["newClientOrderId"] = cid

	params["apiKey"] = rs.BrokerConfig.AccessKey
	params["recvWindow"] = 5000
	params["timestamp"] = time.Now().UnixMilli()
	params["signature"] = rs.signOrder(params)

	w := rs.priWs
	if colo {
		w = rs.priWsColo
	}
	id := time.Now().UnixNano()
	rs.wsReqId2ClientIdMap.Set(id, helper.CidWithPair{Cid: cid, Pair: info.Pair})
	w.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":     id,
			"method": "order.place",
			"params": params,
		},
		Cb: func(msg map[string]interface{}) error {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			return nil
		},
	})
}
func (rs *BinanceSpotRs) signOrder(params map[string]interface{}) string {
	encode := make([]string, 0)
	for key, param := range params {
		encode = append(encode, fmt.Sprintf("%s=%v", key, param))
	}
	sort.Strings(encode)
	raw := ""
	if len(encode) > 0 {
		raw = strings.Join(encode, "&")
	}
	mac := hmac.New(sha256.New, helper.StringToBytes(rs.BrokerConfig.SecretKey))
	_, _ = mac.Write(helper.StringToBytes(raw))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
func (rs *BinanceSpotRs) wsCancelOrder(info *helper.ExchangeInfo, oid, cid string, t int64, colo bool) {
	params := map[string]interface{}{}
	params["symbol"] = info.Symbol
	if oid != "" {
		num, err := strconv.ParseInt(oid, 10, 64)
		if err != nil {
			log.Errorf("oid to int conversioin:", err)
			return
		}
		params["orderId"] = num
	} else if cid != "" {
		params["origClientOrderId"] = cid
	} else {
		return
	}
	params["apiKey"] = rs.BrokerConfig.AccessKey
	params["recvWindow"] = 5000
	params["timestamp"] = time.Now().UnixMilli()
	params["signature"] = rs.signOrder(params)

	w := rs.priWs
	if colo {
		w = rs.priWsColo
	}
	w.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":     time.Now().UnixNano(),
			"method": "order.cancel",
			"params": params,
		},
		Cb: func(msg map[string]interface{}) error {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
			}
			return nil
		},
	})
}

func (rs *BinanceSpotRs) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
	code := value.GetInt("code")
	if code == 0 {
		rs.ReqSucc(base.FailNumActionIdx_AllReq)
		return true
	} else {
		// {"code":-2011,"msg":"Unknown order sent."}
		if code == -2011 {
			rs.ReqSucc(base.FailNumActionIdx_AllReq)
			return true
		}
		if code == -1007 {
			// https://developers.binance.com/docs/derivatives/usds-margined-futures/error-code#10xx---general-server-or-network-issues
			// -1007 Timeout waiting for response from backend server. Send status unknown; execution status unknown.
			rs.Cb.OnExchangeDown()
		}
		rs.Logger.Errorf("请求失败 req: %s %v. rsp: %s", url, params, string(value.String()))
		rs.ReqFail(base.FailNumActionIdx_AllReq)
		return false
	}
}

func pairToSymbol(pair helper.Pair) string {
	return strings.ToUpper(pair.Base + pair.Quote)
}

func (rs *BinanceSpotRs) fetchExchangeInfo(fileName string) ([]helper.ExchangeInfo, error) {
	uri := "/api/v3/exchangeInfo"
	params := make(map[string]interface{})
	infos := make([]helper.ExchangeInfo, 0)
	err := rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		var value ExchangeInfo
		handlerErr = jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, handlerErr.Error()))
			return
		}
		code := value.Code
		if code != "" {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, helper.BytesToString(respBody)))
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)

		datas := value.Symbols
		for _, data := range datas {
			baseCoin := strings.ToLower(data.BaseAsset)
			baseCoin = helper.Trim10Multiple(baseCoin)
			quoteCoin := strings.ToLower(data.QuoteAsset)
			filters := data.Filters
			var tickSize, stepSize float64
			var maxQty, minQty fixed.Fixed
			var minValue, maxValue fixed.Fixed
			if len(filters) > 1 {
				tickSize = fixed.NewS(filters[0].TickSize).Float()
				stepSize = fixed.NewS(filters[1].StepSize).Float()
				maxQty = fixed.NewS(filters[1].MaxQty)
				minQty = fixed.NewS(filters[1].MinQty)
				minValue = fixed.NewS(filters[6].MinNotional)
				maxValue = fixed.NewS(filters[6].MaxNotional)
			}
			if minValue.Equal(fixed.ZERO) {
				minValue = fixed.TEN
				maxValue = fixed.NewF(200000.0)
			}
			symbol := data.Symbol
			info := helper.ExchangeInfo{
				Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
				Symbol:         symbol,
				Status:         data.IsSpotTradingAllowed,
				TickSize:       tickSize,
				StepSize:       stepSize,
				MaxOrderAmount: maxQty,
				MinOrderAmount: minQty,
				MaxOrderValue:  maxValue,
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
	}, false)
	if err != nil {
		rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, err.Error()))
		return nil, err
	}
	return infos, nil
}

func (rs *BinanceSpotRs) getExchangeInfo() []helper.ExchangeInfo {
	fileName := base.GenExchangeInfoFileName("")

	if pairInfo, infos, ok := helper.TryGetExchangeInfosFromFileAndRedis(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S, rs.fetchExchangeInfo); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}

	// randomNumber := rand.Intn(59) + 2
	// if d, err := time.ParseDuration(fmt.Sprintf("%ds", randomNumber)); err == nil {
	// time.Sleep(d)
	// }

	f := helper.GetFileSlotForReqExchangeInfo("binance_spot")
	if f == "" {
		rs.Logger.Error("finaly failed to request exchange info")
		return nil
	}
	defer os.Remove(f)

	if pairInfo, infos, ok := helper.TryGetExchangeInfosPtrFromFile(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}

	infos, err := rs.fetchExchangeInfo(fileName)
	if err == nil {
		helper.StoreExchangeInfos(fileName, infos)
		for _, pairInfo := range infos {
			if pairInfo.Pair.Equal(rs.Pair) {
				helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
				rs.Symbol = pairInfo.Symbol
				return infos
			}
		}
	}
	rs.Logger.Errorf("交易对信息 %s %s 不存在", rs.Pair, rs.ExchangeName)
	helper.LogErrorThenCall(fmt.Sprintf("交易对信息 %s %s 不存在, 需要停机", rs.Pair, rs.ExchangeName), rs.Cb.OnExit)
	return nil
}

func (rs *BinanceSpotRs) getLiteTicker() {
	uri := "/api/v3/ticker/price"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	p := handyPool.Get()
	defer handyPool.Put(p)
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("解析错误 %s", handlerErr.Error())
			return
		}

		if !rs.isOkApiResponse(value, uri) {
			return
		}

		price := helper.MustGetFloat64FromBytes(value, "price")
		t := &rs.TradeMsg.Ticker

		if price > t.Ap.Load() {
			t.Ap.Store(price)
			t.Aq.Store(rs.PairInfo.MinOrderAmount.Float())
			rs.Cb.OnTicker(0)
		} else if price < t.Bp.Load() {
			t.Bp.Store(price)
			t.Bq.Store(rs.PairInfo.MinOrderAmount.Float())
			rs.Cb.OnTicker(0)
		}
	}, false)
	if err != nil {
		rs.Logger.Errorf("请求错误 %s", err.Error())
	}
	return
}

func (rs *BinanceSpotRs) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	uri := "/api/v3/depth"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	params["limit"] = 5
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}

		if !rs.isOkApiResponse(value, uri) {
			err.HandlerError = fmt.Errorf("%s", helper.BytesToString(respBody))
			return
		}
		// 可能会返回  {"code":-1121,"msg":"Invalid symbol."}
		asks := value.GetArray("asks")
		bids := value.GetArray("bids")
		if len(asks) > 0 && len(bids) > 0 {
			ask, _ := asks[0].Array()
			bid, _ := bids[0].Array()
			// ap, _ := ask[0].StringBytes()
			// bp, _ := bid[0].StringBytes()

			t := &rs.TradeMsg.Ticker
			ap := helper.MustGetFloat64FromBytes(ask[0])
			aq := helper.MustGetFloat64FromBytes(ask[1])
			bp := helper.MustGetFloat64FromBytes(bid[0])
			bq := helper.MustGetFloat64FromBytes(bid[1])
			if symbol == rs.Symbol {
				// bookticker 频道的 u 字段 和 depth 频道的 lastUpdateId 字段 是同一个字段
				lastUpdateId := helper.MustGetInt64(value, "lastUpdateId")
				if t.Seq.NewerAndStore(lastUpdateId, time.Now().UnixNano()) {
					t.Set(ap, aq, bp, bq)
					rs.Cb.OnTicker(0)
				}
			}
			ticker.Set(ap, aq, bp, bq)
		}
	}, false)
	if !err.Nil() {
		rs.Logger.Errorf("getTicker error:%s", err.Error())
	}
	return
}
func (rs *BinanceSpotRs) GetAllTickersKeyedSymbol() (ret map[string]helper.Ticker, err helper.ApiError) {
	ret = make(map[string]helper.Ticker)
	url := "/api/v3/ticker/bookTicker"
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, url, nil, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to parse ticker. %v", err.HandlerError)
			return
		}

		datas := helper.MustGetArray(value)
		for _, data := range datas {
			t := helper.Ticker{}
			symbol := helper.MustGetStringFromBytes(data, "symbol")
			ap := helper.MustGetFloat64FromBytes(data, "askPrice")
			aq := helper.MustGetFloat64FromBytes(data, "askQty")
			bp := helper.MustGetFloat64FromBytes(data, "bidPrice")
			bq := helper.MustGetFloat64FromBytes(data, "bidQty")
			t.Set(ap, aq, bp, bq)
			ret[symbol] = t
		}
	}, false)
	return
}

func (rs *BinanceSpotRs) GetEquity() (resp helper.Equity, err helper.ApiError) { //获取资产
	equity, err0 := rs.getEquity(true)
	if !err0.Nil() {
		err = err0
		return
	}
	for _, e := range equity {
		if e.Name == rs.Pair.Base {
			resp.Coin = e.Amount
			resp.CoinFree = e.Avail
			resp.IsSet = true
		} else if e.Name == rs.Pair.Quote {
			resp.Cash = e.Amount
			resp.CashFree = e.Avail
			resp.IsSet = true
		}
	}
	return
}
func (rs *BinanceSpotRs) GetPosition() (resp []helper.PositionSum, err helper.ApiError) { //获取资产
	base.EnsureIsRsExposerOuter(rs)
	equity, err0 := rs.getEquity(true)
	if !err0.Nil() {
		err = err0
		return
	}
	for _, e := range equity {
		if e.Name == rs.Pair.Quote {
			continue
		}
		resp = append(resp, helper.PositionSum{
			Name:        helper.NewPair(e.Name, rs.Pair.Quote, "").ToString(),
			Amount:      e.Amount,
			AvailAmount: e.Avail,
			Side:        helper.PosSideLong,
		})
	}
	return
}

// getEquity 获取账户资金 函数本身无返回值 仅更新本地资产并通过callbackfunc传递出去
func (rs *BinanceSpotRs) getEquity(only bool) (resp []helper.BalanceSum, err helper.ApiError) { //获取资产
	priceMap, _ := rs.GetAllTickersKeyedSymbol()
	url := "/api/v3/account"
	p := handyPool.Get()
	defer handyPool.Put(p)

	params := make(map[string]interface{})

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to parse equity %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url) {
			err.HandlerError = fmt.Errorf("API response not OK for URL: %s", url)
			return
		}
		data := value.GetArray("balances")
		if data == nil {
			err.HandlerError = fmt.Errorf("no balances data in response")
			return
		}

		fieldsSet := helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail
		for _, v := range data {
			asset := helper.Trim10Multiple(helper.MustGetStringLowerFromBytes(v, "asset"))
			sequence := helper.GetInt64(v, "updateTime")
			free := helper.MustGetFloat64FromBytes(v, "free")
			locked := helper.MustGetFloat64FromBytes(v, "locked")
			totalWithoutUpl := locked + free
			avail := free

			if e, ok := rs.EquityNewerAndStore(asset, sequence, time.Now().UnixNano(), fieldsSet); ok {
				e.TotalWithoutUpl = totalWithoutUpl
				e.Avail = avail
				rs.Cb.OnEquityEvent(0, *e)
			}
			price := 0.0
			if strings.EqualFold(asset, rs.Pair.Quote) {
				price = 1
			} else if p, ok := priceMap[pairToSymbol(helper.NewPair(asset, rs.Pair.Quote, ""))]; ok {
				price = p.Price()
			}
			resp = append(resp, helper.BalanceSum{
				Name:   asset,
				Amount: totalWithoutUpl,
				Avail:  free,
				Price:  price,
			})
		}
	}, false)
	if !err.Nil() {
		rs.Logger.Errorf("failed to get equity %v", err.Error())
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	return
}

func (rs *BinanceSpotRs) GenerateListenKey() (listenKey string, err error) {
	url := "/api/v3/userDataStream"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	reqParams := make(map[string]interface{})

	err1 := rs.doRequest(http.MethodPost, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		listenKey = helper.MustGetStringFromBytes(value, "listenKey")
	})
	if err1 != nil {
		return "", err1
	}

	return listenKey, nil
}

func (rs *BinanceSpotRs) KeepListenKey(listenKey string) (err error) {
	var handlerErr error
	var value *fastjson.Value

	p := handyPool.Get()
	defer handyPool.Put(p)
	url := "/api/v3/userDataStream"
	reqParams := make(map[string]interface{})
	reqParams["listenKey"] = listenKey
	err1 := rs.doRequest(http.MethodPut, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		rs.Logger.Debugf("listenkey 续期成功 %v ", value)
	})
	if err1 != nil {
		return err1
	}

	return nil
}

func (rs *BinanceSpotRs) DeleteListenKey(listenKey string) (err error) {
	var handlerErr error
	var value *fastjson.Value

	p := handyPool.Get()
	defer handyPool.Put(p)
	url := "/api/v3/userDataStream"
	reqParams := make(map[string]interface{})
	reqParams["listenKey"] = listenKey
	err1 := rs.doRequest(http.MethodDelete, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		rs.Logger.Debugf("listenkey 关闭成功 %v ", value)
	})
	if err1 != nil {
		return err1
	}

	return nil
}

// 撤掉所有挂单
func (rs *BinanceSpotRs) cancelAllOpenOrders(only bool) {
	if only {
		rs.DoCancelPendingOrders(rs.Symbol)
	} else {
		uri := "/api/v3/openOrders"

		params := make(map[string]interface{})

		// 必备变量
		p := handyPool.Get()
		defer handyPool.Put(p)
		var handlerErr error
		var err error
		var value *fastjson.Value

		err = rs.call(http.MethodGet, uri, nil, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			value, handlerErr = p.ParseBytes(respBody)
			if handlerErr != nil {
				rs.Logger.Errorf("BinanceSpot cancelAllOpenOrders parse response error: %s", handlerErr.Error())
				return
			}
			if !rs.isOkApiResponse(value, uri, params) {
				return
			}
			data, _ := value.Array()
			symbols := make(map[string]int)
			for _, v := range data {
				symbol := helper.BytesToString(v.GetStringBytes("symbol"))
				if _, ok := symbols[symbol]; ok {

				} else {
					symbols[symbol] = 1
					// 执行此交易对撤单
					rs.DoCancelPendingOrders(symbol)
					time.Sleep(time.Second * 1)
				}
			}
		}, false)
		if err != nil {
			//得检查是否有限频提示
			rs.Logger.Errorf("BinanceSpot cancelAllOpenOrders error: %s", err.Error())
		}
		return
	}

}

// 撤掉所有挂单
func (rs *BinanceSpotRs) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/api/v3/openOrders"
	params := make(map[string]interface{})
	if only {
		params["symbol"] = rs.Symbol
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("BinanceSpot cancelAllOpenOrders parse response error: %s", handlerErr.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := helper.MustGetArray(value)
		hasPendingOrderBefore = len(data) != 0
	}, false)
	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("BinanceSpot cancelAllOpenOrders error: %s", err.Error())
	}

	if hasPendingOrderBefore {
		rs.cancelAllOpenOrders(only)
	}
	return
}

// 撤掉单交易对所有挂单
// https://binance-docs.github.io/apidocs/futures/cn/#trade-7
func (rs *BinanceSpotRs) DoCancelPendingOrders(symbol string) (err helper.ApiError) {

	url := "/api/v3/openOrders"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["symbol"] = symbol

	emptyOrders := false

	err.NetworkError = rs.call(http.MethodDelete, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("BinanceSpot DoCancelPendingOrders parse response error: %s", handlerErr.Error())
			return
		}
		//  RESP:[400] body: {"code":-2011,"msg":"Unknown order sent."}
		code := helper.GetInt64(value, "code")
		if code == -2011 {
			emptyOrders = true
		}
		if !rs.isOkApiResponse(value, url, params) {
			return
		}
	}, false)
	if err.NotNil() && !emptyOrders {
		//得检查是否有限频提示
		rs.Logger.Errorf("BinanceSpot DoCancelPendingOrders error: %s", err.Error())
	}
	return
}

// BeforeTrade 开始交易前需要做的所有工作 调整好杠杆
func (rs *BinanceSpotRs) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	rs.getExchangeInfo()
	rs.UpdateExchangeInfo(rs.ExchangeInfoPtrP2S, rs.ExchangeInfoPtrS2P, rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	switch mode {
	case helper.HandleModePublic:
		rs.GetTickerBySymbol(rs.Symbol)
		return
	case helper.HandleModePrepare:
	case helper.HandleModeCloseOne:
		rs.cancelAllOpenOrders(true)
		leakedPrev = rs.HasPosition(rs, true)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		leakedPrev = rs.HasPosition(rs, false)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOpenOrders(true)
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
	}
	// 调整账户
	//b.adjustAcct()
	// 获取账户资金
	rs.getEquity(true)
	// 获取ticker
	rs.GetTickerBySymbol(rs.Symbol)
	rs.PrintAcctSumWhenBeforeTrade(rs)
	rs.Run()
	return
}

// AfterTrade 结束交易时需要做的所有工作  清空挂单和仓位
// 如果有遗漏仓位 返回true  如果清仓干净了 返回false
func (rs *BinanceSpotRs) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	switch mode {
	case helper.HandleModePrepare:
		isLeft = false
	case helper.HandleModeCloseOne:
		rs.cancelAllOpenOrders(true)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.cancelAllOpenOrders(true)
		isLeft = false
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
		isLeft = false
	}
	return
}

func (rs *BinanceSpotRs) rsPlaceOrder(pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, colo bool) {
	url := "/api/v3/order"
	params := make(map[string]interface{})
	params["symbol"] = pairInfo.Symbol
	if orderType != helper.OrderTypeMarket {
		params["price"] = helper.FixPrice(price, pairInfo.TickSize).String()
	}
	params["quantity"] = size
	if orderType == helper.OrderTypeIoc {
		params["type"] = "LIMIT"
		params["timeInForce"] = "IOC"
	} else if orderType == helper.OrderTypePostOnly {
		params["type"] = "LIMIT_MAKER"
	} else if orderType == helper.OrderTypeMarket {
		params["type"] = "MARKET"
	} else if orderType == helper.OrderTypeLimit {
		params["type"] = "LIMIT"
		params["timeInForce"] = "GTC"
	} else {
		var order helper.OrderEvent
		order.Pair = pairInfo.Pair
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
		return
	}

	_side := ""
	switch side {
	case helper.OrderSideKD:
		_side = "BUY"
	case helper.OrderSideKK:
		_side = "SELL"
	case helper.OrderSidePD:
		_side = "SELL"
	case helper.OrderSidePK:
		_side = "BUY"
	}
	params["side"] = _side

	if cid != "" {
		params["newClientOrderId"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()

	if helper.DEBUGMODE {
		rs.Logger.Debugf("binance rest下单 %v", params)
	}

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)
	err = rs.call(http.MethodPost, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		handledOk := false
		defer func() {
			if !handledOk && !rs.DelayMonitor.IsMonitorOrder(cid) {
				order := helper.OrderEvent{Type: helper.OrderEventTypeERROR, ClientID: cid}
				order.Pair = pairInfo.Pair
				var err any
				err = recover()
				if err != nil {
					order.ErrorReason = err.(error).Error()
					rs.Cb.OnOrder(0, order)
					rs.ReqFail(base.FailNumActionIdx_Place)
					panic(err)
				}
				rs.Cb.OnOrder(0, order)
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
		}()

		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			// 出现错误的时候也要触发回调, 不是json格式
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				var order helper.OrderEvent
				order.Pair = pairInfo.Pair
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
			}
			rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
			handledOk = true
			return
		}
		respHeader.VisitAll(func(key, value []byte) {
			k := helper.BytesToString(key)
			switch k {
			case "x-mbx-used-weight":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.raw_requests_limit_5min {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "x-mbx-order-count-10s":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_10s {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "x-mbx-used-weight-1m":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.request_weight_limit {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "x-mbx-order-count-1d":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_1day-100 {
					rs.Cb.OnExit("即将触发限频 停机")
				}
			}
		})

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "" {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				handlerErr = errors.New(string(respBody))
				var order helper.OrderEvent
				order.Pair = pairInfo.Pair
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
			}
			rs.Logger.Errorf("[%s]%s下单失败 %s %s", rs.ExchangeName, cid, err.Error(), helper.BytesToString(respBody))
			handledOk = true
			return
		}

		var order helper.OrderEvent
		order.Pair = pairInfo.Pair

		// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
		order.Type = helper.OrderEventTypeNEW
		order.OrderID = fmt.Sprint(value.GetInt64("orderId"))
		order.ClientID = string(value.GetStringBytes("clientOrderId"))
		//
		handledOk = true
		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Place, Cid: order.ClientID, Oid: order.OrderID, DurationUs: end - start}
		if ok := rs.DelayMonitor.TryNext(rsp); ok {
			return
		}
		rs.Cb.OnOrder(0, order)
		rs.ReqSucc(base.FailNumActionIdx_Place)
	}, colo)

	if err != nil {
		//得检查是否有限频提示
		var order helper.OrderEvent
		order.Pair = pairInfo.Pair
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

// cancelClientID 撤单
func (rs *BinanceSpotRs) rsCancelOrder(info *helper.ExchangeInfo, oid, cid string, t int64, colo bool) {
	uri := "/api/v3/order"

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	if oid != "" {
		params["orderId"] = oid
	} else if cid != "" {
		params["origClientOrderId"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error

	start := time.Now().UnixMicro()

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	err = rs.call(http.MethodDelete, uri, nil, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		_, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		// 撤单不需要触发回调 撤单动作高度时间敏感 依赖ws推送
		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Cancel, Cid: cid, Oid: oid, DurationUs: end - start}
		if ok := rs.DelayMonitor.TryNext(rsp); ok {
			return
		}
	}, colo)

	if err != nil {
		//得检查是否有限频提示
	}
	if handlerErr != nil {
		//忽略
	}
	if err == nil && handlerErr == nil {
		rs.CancelOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

// checkOrder 查单
func (rs *BinanceSpotRs) checkOrderID(info *helper.ExchangeInfo, cid string, oid string) {
	uri := "/api/v3/order"

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	if oid != "" {
		params["orderId"] = oid
	} else if cid != "" {
		params["origClientOrderId"] = cid
	} else {
		return
	}

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("checkOrderID:%s", handlerErr.Error())
			return
		}
		if value.Type() == fastjson.TypeNull {
			// 查不到这个订单不做任何处理  等待策略层触发重置交易
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		status := string(value.GetStringBytes("status"))

		// 查到订单
		var event helper.OrderEvent
		event.Pair = info.Pair
		event.OrderID = strconv.Itoa(helper.MustGetInt(value, "orderId"))
		event.ClientID = helper.MustGetStringFromBytes(value, "clientOrderId")
		switch helper.MustGetShadowStringFromBytes(value, "side") {
		case "BUY":
			event.OrderSide = helper.OrderSideKD
		case "SELL":
			event.OrderSide = helper.OrderSideKK
		default:
			rs.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(value, "side"))
			return
		}
		orderType := helper.MustGetShadowStringFromBytes(value, "type")
		switch orderType {
		case "LIMIT":
			event.OrderType = helper.OrderTypeLimit
		case "MARKET":
			event.OrderType = helper.OrderTypeMarket
		case "LIMIT_MAKER":
			event.OrderType = helper.OrderTypePostOnly
		}
		// 必须在type后面。忽略其他类型
		switch helper.MustGetShadowStringFromBytes(value, "timeInForce") {
		case "IOC":
			event.OrderType = helper.OrderTypeIoc
		}

		switch status {
		case "NEW":
			event.Type = helper.OrderEventTypeNEW
		case "PARTIALLY_FILLED":
			event.Type = helper.OrderEventTypePARTIAL
		case "CANCELED", "EXPIRED", "FILLED":
			event.Type = helper.OrderEventTypeREMOVE
			event.Filled = fixed.NewS(helper.MustGetShadowStringFromBytes(value, "executedQty"))
			if event.Filled.GreaterThan(fixed.ZERO) {
				event.FilledPrice = helper.MustGetFloat64FromBytes(value, "cummulativeQuoteQty") / event.Filled.Float()
			}
		}
		rs.Cb.OnOrder(0, event)

	}, rs.rsColoOn)

	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("order query error %v", err)
	}
}
func (rs *BinanceSpotRs) doRequest(reqMethod string, path string, reqParams map[string]interface{}, reqBody map[string]interface{}, requestHeaders map[string]string, respHandler rest.FastHttpRespHandler) error {
	if requestHeaders == nil {
		requestHeaders = make(map[string]string, 0)
	}
	queryString := strings.Builder{}
	bodyString := strings.Builder{}
	_bodyString := ""
	if reqBody != nil {

		first := true
		for key, value := range reqBody {
			if first {
				first = false
				bodyString.WriteString(key)
				bodyString.WriteString("=")
				bodyString.WriteString(value.(string))
			} else {
				bodyString.WriteString("&")
				bodyString.WriteString(key)
				bodyString.WriteString("=")
				bodyString.WriteString(value.(string))
			}
		}
	}

	reqUrl := path
	if reqParams != nil {
		first := true
		for key, param := range reqParams {
			if first {
				first = false
				queryString.WriteString(key)
				queryString.WriteString("=")
				queryString.WriteString(fmt.Sprintf("%v", param))
			} else {
				queryString.WriteString("&")
				queryString.WriteString(key)
				queryString.WriteString("=")
				queryString.WriteString(fmt.Sprintf("%v", param))
			}
		}
	}

	if bodyString.Len() != 0 {
		requestHeaders["Content-Type"] = "application/x-www-form-urlencoded"
		_bodyString = strings.Replace(bodyString.String(), "%40", "@", -1)
		_bodyString = strings.Replace(_bodyString, "%2C", ",", -1)
	}
	requestHeaders["X-MBX-APIKEY"] = rs.BrokerConfig.AccessKey

	if queryString.Len() > 0 {
		if strings.Contains(reqUrl, "?") {
			reqUrl = fmt.Sprintf("%s&%s", reqUrl, queryString.String())
		} else {
			reqUrl = fmt.Sprintf("%s?%s", reqUrl, queryString.String())
		}
	}

	status, err := rs.client.Request(reqMethod, rs.baseUrl+reqUrl, helper.StringToBytes(bodyString.String()), requestHeaders, respHandler)
	if err != nil {
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return err
	}
	if status != 200 {
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return fmt.Errorf("status:%d", status)
	}
	rs.failNum.Store(0)

	return nil
}

// call 专用于bitget_usdt_swap的发起请求函数
func (rs *BinanceSpotRs) call(reqMethod string, reqUrl string, reqParams map[string]interface{}, reqBody map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler, colo bool) error {
	reqHeaders := make(map[string]string, 0)

	queryString := strings.Builder{}
	bodyString := strings.Builder{}
	_bodyString := ""
	if reqBody != nil {
		first := true
		for key, value := range reqBody {
			if first {
				first = false
				bodyString.WriteString(key)
				bodyString.WriteString("=")
				bodyString.WriteString(value.(string))
			} else {
				bodyString.WriteString("&")
				bodyString.WriteString(key)
				bodyString.WriteString("=")
				bodyString.WriteString(value.(string))
			}
		}
	}

	if reqParams != nil {
		first := true
		for key, param := range reqParams {
			if first {
				first = false
				queryString.WriteString(key)
				queryString.WriteString("=")
				queryString.WriteString(fmt.Sprintf("%v", param))
			} else {
				queryString.WriteString("&")
				queryString.WriteString(key)
				queryString.WriteString("=")
				queryString.WriteString(fmt.Sprintf("%v", param))
			}
		}
	}

	if bodyString.Len() != 0 {
		reqHeaders["Content-Type"] = "application/x-www-form-urlencoded"
		_bodyString = strings.Replace(bodyString.String(), "%40", "@", -1)
		_bodyString = strings.Replace(_bodyString, "%2C", ",", -1)
	}

	if needSign {
		reqHeaders["X-MBX-APIKEY"] = rs.BrokerConfig.AccessKey
		if queryString.Len() > 0 {
			queryString.WriteString("&")
		}
		queryString.WriteString("recvWindow")
		queryString.WriteString("=")
		queryString.WriteString("5000")

		queryString.WriteString("&")
		queryString.WriteString("timestamp")
		queryString.WriteString("=")
		queryString.WriteString(strconv.Itoa(int(time.Now().UnixMilli())))

		raw := fmt.Sprintf("%s%s", queryString.String(), _bodyString)
		mac := hmac.New(sha256.New, helper.StringToBytes(rs.BrokerConfig.SecretKey))

		_, _ = mac.Write(helper.StringToBytes(raw))

		if queryString.Len() == 0 {
			queryString.WriteString("signature")
			queryString.WriteString("=")
			queryString.WriteString(fmt.Sprintf("%x", mac.Sum(nil)))
		} else {
			queryString.WriteString("&")
			queryString.WriteString("signature")
			queryString.WriteString("=")
			queryString.WriteString(fmt.Sprintf("%x", mac.Sum(nil)))
		}
	}
	if queryString.Len() > 0 {
		if strings.Contains(reqUrl, "?") {
			reqUrl = fmt.Sprintf("%s&%s", reqUrl, queryString.String())
		} else {
			reqUrl = fmt.Sprintf("%s?%s", reqUrl, queryString.String())
		}
	}

	c := rs.client
	if colo {
		c = rs.clientColo
	}
	status, err := c.Request(reqMethod, BaseUrl+reqUrl, helper.StringToBytes(bodyString.String()), reqHeaders, respHandler)
	if err != nil {
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return err
	}
	if status != 200 {
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return fmt.Errorf("status:%d", status)
	}

	//_ = respHeader
	// parse header
	//p := restHandyPool.Get()
	//defer restHandyPool.Put(p)
	//
	//value, err1 := p.ParseBytes(header)
	//if err1 == nil {
	//	if value.Exists("ratelimit-remaining") {
	//		remain := value.GetInt("ratelimit-remaining")
	//		if b.rateLimitWarnCallback != nil {
	//			b.rateLimitWarnCallback(remain)
	//		}
	//	}
	//}
	//
	//fmt.Printf("resp header: %s", util.BytesToString(header))

	rs.failNum.Store(0)
	return nil
}

func (rs *BinanceSpotRs) GetExName() string {
	return helper.BrokernameBinanceSpot.String()
}

func (rs *BinanceSpotRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	equity, err0 := rs.getEquity(false)
	if !err0.Nil() {
		err = err0
		return
	}
	for _, e := range equity {
		if e.Name == rs.Pair.Quote {
			continue
		}
		resp = append(resp, helper.PositionSum{
			Name:        helper.NewPair(e.Name, rs.Pair.Quote, "").ToString(),
			Amount:      e.Amount,
			AvailAmount: e.Avail,
			Side:        helper.PosSideLong,
		})
	}
	return
}
func (rs *BinanceSpotRs) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("failed to get symbol info. %s", symbol)
		return false
	}

	cid := fmt.Sprintf("99%d", uint32(time.Now().UnixMilli()))

	price := ticker.Bp.Load() * 0.98
	rs.rsPlaceOrder(info, price, orderAmount, cid, orderSide, helper.OrderTypeLimit, 0, false)
	return true
}

func (rs *BinanceSpotRs) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	err = helper.ApiErrorNotImplemented
	return
}

func (rs *BinanceSpotRs) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	err = helper.ApiErrorNotImplemented
	return
}
func (rs *BinanceSpotRs) GetFee() (fee helper.Fee, err helper.ApiError) {
	uri := "/sapi/v1/asset/tradeFee"

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, nil, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Error(err.HandlerError)
			return
		}
		for _, item := range value.GetArray() {
			if strings.EqualFold(helper.MustGetShadowStringFromBytes(item, "symbol"), rs.Symbol) {
				fee.Taker = helper.MustGetFloat64FromBytes(item, "takerCommission")
				fee.Maker = helper.MustGetFloat64FromBytes(item, "makerCommission")
				break
			}
		}
	}, false)

	return
}

func (rs *BinanceSpotRs) GetFundingRate() (helper.FundingRate, error) {
	return helper.FundingRate{}, nil
}
func (rs *BinanceSpotRs) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	a.Balances, err = rs.getEquity(false)
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
// binance 现货没有 批量下单 和 按orderIds批量撤单
func (rs *BinanceSpotRs) SendSignal(signals []helper.Signal) {
	for _, s := range signals {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送信号 %s", s.String())
		}
		switch s.Type {
		case helper.SignalTypeNewOrder:
			rs.PlaceOrderSelect(s)
			// if pairInfo, ok := rs.GetPairInfoByPair(&s.Pair); ok {
			// 	go rs.wsPlaceOrder(pairInfo, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
			// } else {
			// 	rs.Logger.Errorf("wrong pair %v", s.Pair)
			// 	continue
			// }
		case helper.SignalTypeCancelOrder:
			rs.CancelOrderSelect(s)
			// if pairInfo, ok := rs.GetPairInfoByPair(&s.Pair); ok {
			// 	rs.wsCancelOrder(pairInfo, s.OrderID, s.ClientID, s.Time, false)
			// } else {
			// 	rs.Logger.Errorf("wrong pair %v", s.Pair)
			// 	continue
			// }
		case helper.SignalTypeCheckOrder:
			pairInfo, ok := rs.GetPairInfoByPair(&s.Pair)
			if !ok {
				rs.Logger.Errorf("wrong pair %v", s.Pair)
				continue
			}
			go rs.checkOrderID(pairInfo, s.ClientID, s.OrderID)

		// case helper.SignalTypeNewOrder:
		// 	rs.PlaceOrderSelect(s)
		// case helper.SignalTypeCancelOrder:
		// 	rs.CancelOrderSelect(s)
		// // case helper.SignalTypeAmend:
		// // 	rs.AmendOrderSelect(s)

		case helper.SignalTypeGetEquity:
			go rs.getEquity(true)
		case helper.SignalTypeGetLiteTicker:
			go rs.getLiteTicker()
		case helper.SignalTypeCancelOne:
			go rs.cancelAllOpenOrders(true)
		case helper.SignalTypeCancelAll:
			go rs.cancelAllOpenOrders(false)
		}
	}
}

// Do 发起任意请求 一般用于非交易任务 对时间不敏感
func (rs *BinanceSpotRs) Do(actType string, params any) (any, error) {
	if actType == "raw" {
		url := params.(string)
		p := handyPool.Get()
		defer handyPool.Put(p)
		// var handlerErr error
		// var err error
		// var value *fastjson.Value

		params := make(map[string]interface{})

		rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			fmt.Println(string(respBody))
		}, false)
	}
	return nil, nil
}

func (rs *BinanceSpotRs) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltNil | base.AbltOrderAmend
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
func (rs *BinanceSpotRs) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SPOT
}

func (rs *BinanceSpotRs) WsLogged() bool {
	return true
}

// 获取全市场交易规则
func (rs *BinanceSpotRs) GetExchangeInfos() []helper.ExchangeInfo {
	return rs.getExchangeInfo()
}

func (rs *BinanceSpotRs) GetFeatures() base.Features {
	f := base.Features{
		GetFee:                true,
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		UpdateWsTickerWithSeq: true,
		GetLiteTickerSignal:   true,
		Standby:               true,
		OrderPostonly:         true,
		MultiSymbolOneAcct:    true,
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}

func (rs *BinanceSpotRs) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	return rs.DoGetPendingOrders("")
}

// symbol 空表示获取全部
func (rs *BinanceSpotRs) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	url := "/api/v3/openOrders"
	p := handyPool.Get()
	var value *fastjson.Value

	params := make(map[string]interface{})
	if symbol != "" {
		params["symbol"] = symbol
	}

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("BinanceSpot parse response error: %s", err.HandlerError)
			return
		}
		//  RESP:[400] body: {"code":-2011,"msg":"Unknown order sent."}
		code := helper.GetInt64(value, "code")
		if code == -2011 {
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			return
		}
		for _, data := range helper.MustGetArray(value) {
			order := helper.OrderForList{
				Symbol:   helper.MustGetStringFromBytes(data, "symbol"),
				ClientID: helper.MustGetStringFromBytes(data, "clientOrderId"),
				OrderID:  helper.Itoa(helper.MustGetInt64(data, "orderId")),
				Price:    helper.MustGetFloat64FromBytes(data, "price"),
				Amount:   fixed.NewS(helper.MustGetShadowStringFromBytes(data, "origQty")),
			}
			order.Filled = fixed.NewS(helper.MustGetShadowStringFromBytes(data, "executedQty"))
			if order.Filled.GreaterThan(fixed.ZERO) {
				filledValue := fixed.NewS(helper.MustGetShadowStringFromBytes(data, "cummulativeQuoteQty"))
				order.FilledPrice = filledValue.Div(order.Filled).Float()
			}

			side := helper.GetShadowStringFromBytes(data, "side")
			switch side {
			case "BUY":
				order.OrderSide = helper.OrderSideKD
			case "SELL":
				order.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %v", side)
				return
			}
			orderType := helper.MustGetShadowStringFromBytes(data, "type")
			switch orderType {
			case "LIMIT":
				order.OrderType = helper.OrderTypeLimit
			case "MARKET":
				order.OrderType = helper.OrderTypeMarket
			case "LIMIT_MAKER":
				order.OrderType = helper.OrderTypePostOnly
			}
			tif := helper.MustGetShadowStringFromBytes(data, "timeInForce")
			switch tif {
			case "IOC":
				order.OrderType = helper.OrderTypeIoc
			}
			order.CreatedTimeMs = helper.MustGetInt64(data, "time")
			order.UpdatedTimeMs = helper.MustGetInt64(data, "updateTime")
			results = append(results, order)
		}
	}, false)
	if err.NotNil() {
		rs.Logger.Errorf("failed to get pending orders error: %s", err.Error())
	}
	return
}
func (rs *BinanceSpotRs) wsCheckOrder(info *helper.ExchangeInfo, cid string, oid string) {
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	if oid != "" {
		params["orderId"] = oid
	} else if cid != "" {
		params["origClientOrderId"] = cid
	} else {
		return
	}
	params["apiKey"] = rs.BrokerConfig.AccessKey
	params["recvWindow"] = 5000
	params["timestamp"] = time.Now().UnixMilli()
	params["signature"] = rs.signOrder(params)

	rs.priWs.SendMessage2(ws.WsMsg{
		Msg: map[string]interface{}{
			"id":     time.Now().UnixNano(),
			"method": "order.status",
			"params": params,
		},
		Cb: func(msg map[string]interface{}) error {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			return nil
		},
	})
}

func (rs *BinanceSpotRs) priHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE {
		log.Debugf("[%p]收到 rs's pri ws 推送 %s", rs, string(msg))
	}
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		log.Errorf("Binance usdt swap rs's ws解析msg出错 err:%v", err)
		return
	}
	code := helper.MustGetInt64(value, "status")
	id := value.Get("id")
	if code != 200 {
		if id.Type() != fastjson.TypeNull {
			requestId := helper.MustGetInt64(id)
			cid, ok := rs.wsReqId2ClientIdMap.Get(requestId)
			if !ok {
				rs.Logger.Errorf("not cid for reqId. %d ", requestId)
				return
			}
			rs.wsReqId2ClientIdMap.Remove(requestId)
			if rs.DelayMonitor.IsMonitorOrder(cid.Cid) {
				return
			}
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid.Cid
			order.Pair = cid.Pair
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 Binance usdt swap rs's ws status %s", rs.ExchangeName, cid, msg)
			return
		}
	}

	result := value.Get("result")
	status := helper.GetShadowStringFromBytes(result, "status")
	var order helper.OrderEvent
	order.OrderID = strconv.FormatInt(helper.MustGetInt64(result, "orderId"), 10)
	order.ClientID = helper.MustGetStringFromBytes(result, "clientOrderId")
	origCid := result.GetStringBytes("origClientOrderId")
	if origCid != nil {
		order.ClientID = helper.MustGetStringFromBytes(result, "origClientOrderId")
	}

	if id.Type() != fastjson.TypeNull {
		tsns := id.GetInt64()
		act := base.ActionType_Place
		if status == "CANCELED" {
			act = base.ActionType_Cancel
		}
		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Ws, Action: act, Cid: order.ClientID, Oid: order.OrderID, DurationUs: time.Now().UnixMicro() - tsns/1000}
		if rs.DelayMonitor.TryNext(rsp) {
			return
		}
	}

	symbol := helper.MustGetShadowStringFromBytes(result, "symbol")
	info, ok := rs.GetPairInfoBySymbol(symbol)
	if !ok {
		log.Errorf("wrong symbol %v", symbol)
		return
	}
	order.Pair = info.Pair

	switch status {
	case "NEW", "":
		order.Type = helper.OrderEventTypeNEW
		rs.Cb.OnOrder(0, order)
		return
	case "CANCELED", "EXPIRED", "FILLED":
		order.Amount = fixed.NewS(helper.MustGetShadowStringFromBytes(result, "origQty"))
		order.Price = helper.GetFloat64FromBytes(result, "price")
		order.Type = helper.OrderEventTypeREMOVE
		order.Filled = fixed.NewF(helper.MustGetFloat64FromBytes(result, "executedQty"))
		if order.Filled.GreaterThan(fixed.ZERO) {
			order.FilledPrice = helper.MustGetFloat64FromBytes(result, "cummulativeQuoteQty") / order.Filled.Float()
		}
	default:
		log.Error("Unknown rs's ws order status: ", status)
		return
	}
	switch helper.MustGetShadowStringFromBytes(result, "side") {
	case "BUY":
		order.OrderSide = helper.OrderSideKD
	case "SELL":
		order.OrderSide = helper.OrderSideKK
	default:
		log.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(result, "side"))
		return
	}

	switch helper.MustGetShadowStringFromBytes(result, "type") {
	case "LIMIT":
		order.OrderType = helper.OrderTypeLimit
	case "MARKET":
		order.OrderType = helper.OrderTypeMarket
	case "LIMIT_MAKER":
		order.OrderType = helper.OrderTypePostOnly
	}
	switch helper.MustGetShadowStringFromBytes(result, "timeInForce") {
	case "IOC":
		order.OrderType = helper.OrderTypeIoc
	case "GTX":
		order.OrderType = helper.OrderTypePostOnly
	}
	rs.Cb.OnOrder(0, order)
}
