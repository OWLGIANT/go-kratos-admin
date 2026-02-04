package binance_usdt_swap

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"sync"

	"actor/broker/base"
	base_orderbook "actor/broker/base/base_orderbook"
	"actor/broker/brokerconfig"
	"actor/broker/client/rest"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/tools"
	jsoniter "github.com/json-iterator/go"

	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

// 注意事项：
// 1. 改单：一定要带上 oid 或 cid。如果两者都带，以 oid 为准；一定要side和amount

var (
	handyPool fastjson.ParserPool
)

type BinanceUsdtSwapRs struct {
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyDoGetPriceLimit
	failNum                      atomic.Int64 // 出错次数
	baseUrl                      string       // 基础host
	base.DummyDoAmendOrderWsColo              //只有 cancelandnew 没有amend
	base.DummyDoAmendOrderWsNor
	client     *rest.Client // 通用rest客户端
	clientColo *rest.Client // 通用rest客户端
	rsColoOn   bool
	// binance 限频规则
	request_weight_limit int
	order_limit_10s      int
	order_limit_60s      int
	//
	takerFee           atomic.Float64 // taker费率
	makerFee           atomic.Float64 // maker费率
	orderPlaceReqPool  OrderPlaceReqPool
	orderAmendReqPool  OrderAmendReqPool
	orderCancelReqPool OrderCancelReqPool
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
	mmap                *helper.Mmap
}

// 创建新的实例
func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}

	rs := &BinanceUsdtSwapRs{
		client:     rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		clientColo: rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		baseUrl:    RS_URL,
		// 限频规则
		request_weight_limit: 2350, // 阈值2400
		order_limit_10s:      290,  //  阈值300
		order_limit_60s:      1180, // 阈值1200
		wsReqId2ClientIdMap:  cmap.NewWithCustomShardingFunction[int64, helper.CidWithPair](func(key int64) uint32 { return uint32(key) }),
	}
	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, pairInfo, cb)

	// colo url 配置
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BinanceUsdtSwapRestUrl != "" {
		rs.rsColoOn = true
		RS_URL_COLO = cfg.BinanceUsdtSwapRestUrl
		params.Logger.Infof("binance_usdt_swap rs 启用colo  %v", RS_URL_COLO)
	}

	if !params.BanColo && cfg.BinanceUsdtSwapWsUrl != "" && false { //BinanceUsdtSwapWsUrl is not for rs
		rs.wsColoOn = true
		WS_URL_COLO = cfg.BinanceUsdtSwapWsUrl
		params.Logger.Infof("binance_usdt_swap rs'ws 启用colo  %v", RS_URL_COLO)
	}

	// WS
	rs.stopCtx, rs.stopFunc = context.WithCancel(context.Background())
	if !params.BanRsWs {
		rs.priWs = ws.NewWS("wss://ws-fapi.binance.com/ws-fapi/v1?returnRateLimits=false", params.LocalAddr, params.ProxyURL, rs.priHandler, cb.OnExit, params.BrokerConfig)
		rs.priWs.SetPongFunc(pong)
		rs.priWs.SetSubscribe(func() error {
			rs.Cb.OnWsReady(rs.ExchangeName)
			return nil
		})
		rs.priWs.SetPingInterval(90)
	}

	//Delay monitor
	if params.ActivateDelayMonitor {
		influxCfg := base.DefaultInfluxConfig(params)
		base.InitDelayMonitor(&rs.DelayMonitor, &rs.FatherRs, influxCfg, rs.GetExName(), rs.Pair.String(), rs.MonitorTrigger)
	}
	rs.SetSelectedLine(base.LinkType_Nor, base.ClientType_Rs)
	rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Cross})
	rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Iso})
	if rs.rsColoOn {
		rs.SetSelectedLine(base.LinkType_Colo, base.ClientType_Rs)
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Cross})
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Iso})
	}
	// WS
	if !params.BanRsWs {
		rs.ReqWsNorLogged.Store(true)
		rs.stopCtx, rs.stopFunc = context.WithCancel(context.Background())
		rs.DoCreateReqWsNor()
		rs.SetSelectedLine(base.LinkType_Nor, base.ClientType_Ws)
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Cross})
		rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Iso})

		if rs.wsColoOn {
			rs.ReqWsColoLogged.Store(true)
			rs.DoCreateReqWsColo()
			rs.SetSelectedLine(base.LinkType_Colo, base.ClientType_Ws)
			rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Cross})
			rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Colo, Client: base.ClientType_Ws, MarginMode: helper.MarginMode_Iso})
		}
	}
	rs.ChoiceBestLine()
	if params.Pairs[0].Quote == "usdc" {
		rs.SetDefaultPair("btc_usdc")
	}

	if params.BrokerConfig.RawDataMmapCollectPath != "" {
		var err error
		rs.mmap, err = helper.NewMmap(params.BrokerConfig.RawDataMmapCollectPath, rs.ExchangeName.String()+"_rs_"+rs.Pair.String())
		if err != nil {
			return nil
		}
	}
	return rs
}

func (rs *BinanceUsdtSwapRs) DoCreateReqWsNor() error {
	rs.priWs = ws.NewWS("wss://ws-fapi.binance.com/ws-fapi/v1?returnRateLimits=false", rs.BrokerConfig.LocalAddr, rs.BrokerConfig.ProxyURL, rs.priHandler, rs.Cb.OnExit, rs.BrokerConfig.BrokerConfig)
	rs.priWs.SetPongFunc(pong)
	return nil
}
func (rs *BinanceUsdtSwapRs) DoCreateReqWsColo() error {
	rs.priWsColo = ws.NewWS(WS_URL_COLO+"/ws-fapi/v1?returnRateLimits=false", rs.BrokerConfig.LocalAddr, rs.BrokerConfig.ProxyURL, rs.priHandler, rs.Cb.OnExit, rs.BrokerConfig.BrokerConfig)
	rs.priWsColo.SetPongFunc(pong)
	return nil
}

func (rs *BinanceUsdtSwapRs) DoAmendOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsAmendOrder(info, s.Price, s.Amount, s.ClientID, s.OrderID, s.OrderSide, s.Time, rs.rsColoOn)
}
func (rs *BinanceUsdtSwapRs) DoAmendOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsAmendOrder(info, s.Price, s.Amount, s.ClientID, s.OrderID, s.OrderSide, s.Time, false)
}
func (rs *BinanceUsdtSwapRs) DoPlaceOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, rs.rsColoOn)
}
func (rs *BinanceUsdtSwapRs) DoPlaceOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
}
func (rs *BinanceUsdtSwapRs) DoCancelOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsCancelOrder(info, s.OrderID, s.ClientID, s.Time, rs.rsColoOn)
}
func (rs *BinanceUsdtSwapRs) DoCancelOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.rsCancelOrder(info, s.OrderID, s.ClientID, s.Time, false)
}
func (rs *BinanceUsdtSwapRs) DoPlaceOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, rs.wsColoOn)
}
func (rs *BinanceUsdtSwapRs) DoPlaceOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsPlaceOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
}
func (rs *BinanceUsdtSwapRs) DoCancelOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsCancelOrder(info, s.OrderID, s.ClientID, s.Time, rs.wsColoOn)
}
func (rs *BinanceUsdtSwapRs) DoCancelOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.wsCancelOrder(info, s.OrderID, s.ClientID, s.Time, false)
}

func (rs *BinanceUsdtSwapRs) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
	code := value.GetInt("code")
	if code == 0 || code == 200 {
		rs.ReqSucc(base.FailNumActionIdx_AllReq)
		return true
	} else {
		if code == -2013 { // Order not exist, 会短暂查不到，先过滤
			rs.ReqSucc(base.FailNumActionIdx_AllReq)
			return true
		}
		if code == -4059 { // {"code":-4059,"msg":"No need to change position side."}
			rs.ReqSucc(base.FailNumActionIdx_AllReq)
			return true
		}
		if code == -1007 {
			// https://developers.binance.com/docs/derivatives/usds-margined-futures/error-code#10xx---general-server-or-network-issues
			// -1007 Timeout waiting for response from backend server. Send status unknown; execution status unknown.
			rs.Cb.OnExchangeDown()
		}
		rs.Logger.Errorf("请求失败 req: %s %v. rsp: %s", url, params, value.String())
		rs.ReqFail(base.FailNumActionIdx_AllReq)
		return false
	}
}

func (rs *BinanceUsdtSwapRs) getOrderbookSnap() (*base_orderbook.Slot, error) {
	uri := "/fapi/v1/depth"
	params := make(map[string]interface{})
	params["limit"] = 1000
	params["symbol"] = rs.Symbol

	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	// 待使用数据结构
	snapSlot := &base_orderbook.Slot{}
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

		if rs.BrokerConfig.RawDataMmapCollectPath != "" {
			rs.mmap.Write(respBody, time.Now().UnixNano())
			return
		}

		snapSlot.ExPrevLastId = helper.MustGetInt64(value, "lastUpdateId")
		snapSlot.ExTsMs = helper.MustGetInt64(value, "T")
		bids := helper.MustGetArray(value, "bids")
		asks := helper.MustGetArray(value, "asks")
		snapSlot.PriceItems = make([][2]float64, 0, len(bids)+len(asks))
		snapSlot.AskStartIdx = len(bids)
		for _, bid := range bids {
			pa := helper.MustGetArray(bid)
			snapSlot.PriceItems = append(snapSlot.PriceItems, [2]float64{helper.MustGetFloat64FromBytes(pa[0]), helper.MustGetFloat64FromBytes(pa[1])})
		}
		for _, ask := range asks {
			pa := helper.MustGetArray(ask)
			snapSlot.PriceItems = append(snapSlot.PriceItems, [2]float64{helper.MustGetFloat64FromBytes(pa[0]), helper.MustGetFloat64FromBytes(pa[1])})
		}
	})
	if err != nil {
		rs.Logger.Errorf("failed to get orderbook snap, %v", err)
		return nil, err
	}

	return snapSlot, nil
}

// 直接从服务器获取
func (rs *BinanceUsdtSwapRs) fetchExchangeInfo(fileName string) ([]helper.ExchangeInfo, error) {
	// 请求必备信息
	uri := "/fapi/v1/exchangeInfo"
	params := make(map[string]interface{})

	// p := handyPool.Get()
	// defer handyPool.Put(p)
	// var value *fastjson.Value
	// 待使用数据结构
	infos := make([]helper.ExchangeInfo, 0)
	// 发起请求
	err := rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		// value, handlerErr = p.ParseBytes(respBody)
		var value ExchangeInfo
		handlerErr = jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			helper.LogErrorThenCall(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, string(respBody)), rs.Cb.OnExit)
			return
		}
		if value.Code != 0 && value.Code != 200 {
			helper.LogErrorThenCall(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, string(respBody)), rs.Cb.OnExit)
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)

		datas := value.Symbols
		for _, data := range datas {
			contractType := data.ContractType
			if contractType != "PERPETUAL" {
				continue
			}
			baseCoin := strings.ToLower(data.BaseAsset)
			baseCoin = helper.Trim10Multiple(baseCoin)
			quoteCoin := strings.ToLower(data.QuoteAsset)
			filters := data.Filters
			var tickSize, stepSize float64
			var maxQty, minQty fixed.Fixed
			var minValue, maxValue fixed.Fixed
			for _, filter := range filters {
				filterType := filter.FilterType
				switch filterType {
				case "PRICE_FILTER":
					tickSize = fixed.NewS(filter.TickSize).Float()
				case "LOT_SIZE":
					stepSize = fixed.NewS(filter.StepSize).Float()
					maxQty0 := fixed.NewS(filter.MaxQty).Float()
					// maxQty = fixed.NewS(helper.BytesToString(helper.MustGetStringBytes(filter, "maxQty")))
					minQty = fixed.NewS(filter.MinQty)
					if maxQty0 >= fixed.BIG.Float() {
						maxQty = fixed.BIG // 溢出
					} else {
						maxQty = fixed.NewF(maxQty0)
					}
				case "MIN_NOTIONAL":
					minValue = fixed.NewS(filter.Notional)
				}
			}
			if minValue.IsZero() {
				minValue = fixed.TEN
			}
			maxValue = fixed.NewF(200000)
			symbol := data.Symbol

			info := helper.ExchangeInfo{
				Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
				Symbol:         symbol,
				Status:         data.Status == "TRADING",
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
				MaxLeverage:  helper.MAX_LEVERAGE,
			}
			if info.MaxPosAmount == fixed.NaN || info.MaxPosAmount.IsZero() {
				info.MaxPosAmount = fixed.BIG
			}
			infos = append(infos, info)
			rs.ExchangeInfoPtrS2P.Set(symbol, &info)
			rs.ExchangeInfoPtrP2S.Set(info.Pair.String(), &info)
		}
	})
	if err != nil {
		helper.LogErrorThenCall(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, err.Error()), rs.Cb.OnExit)
		return infos, err
	}
	return infos, nil
}

func (rs *BinanceUsdtSwapRs) getExchangeInfo() []helper.ExchangeInfo {
	fileName := base.GenExchangeInfoFileName("")
	if pairInfo, infos, ok := helper.TryGetExchangeInfosFromFileAndRedis(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S, rs.fetchExchangeInfo); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}

	// redis不可用降级，随机等待一小段时间，避免大量请求，再网络获取
	// 生成范围在 2 到 60 之间的随机数
	// randomNumber := rand.Intn(59) + 2
	// if d, err := time.ParseDuration(fmt.Sprintf("%ds", randomNumber)); err == nil {
	// time.Sleep(d)
	// }
	f := helper.GetFileSlotForReqExchangeInfo("binance_usdt_swap")
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
func (rs *BinanceUsdtSwapRs) getLiteTicker(symbol string) {
	// 请求必备信息
	uri := "/fapi/v2/ticker/price"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var err error
	var value *fastjson.Value
	var handlerErr error

	err = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("getTicker error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		if symbol == rs.Symbol {
			tsInEx := helper.MustGetInt64(value, "time")
			t := &rs.TradeMsg.Ticker
			if t.Seq.NewerAndStore(tsInEx, time.Now().UnixNano()) {
				price := helper.MustGetFloat64FromBytes(value, "price")
				if price > t.Ap.Load() {
					t.Ap.Store(price)
					t.Aq.Store(rs.PairInfo.MinOrderAmount.Float())
					rs.Cb.OnTicker(0)
				} else if price < t.Bp.Load() {
					t.Bp.Store(price)
					t.Bq.Store(rs.PairInfo.MinOrderAmount.Float())
					rs.Cb.OnTicker(0)
				}
			}
		}
	})
	if err != nil {
		rs.Logger.Errorf("getTicker error %v", err)
		//得检查是否有限频提示
	}
	return
}

func (rs *BinanceUsdtSwapRs) getLeverage() {
	// 请求必备信息
	url := "/fapi/v1/leverageBracket"
	// 请求必备变量
	p := handyPool.Get()
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	defer handyPool.Put(p)
	var err helper.ApiError
	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		brackets := helper.MustGetArray(value, "0", "brackets")
		ml := 1
		for _, b := range brackets {
			ml = max(ml, helper.MustGetInt(b, "initialLeverage"))
		}
		rs.PairInfo.MaxLeverage = min(ml, helper.MAX_LEVERAGE)
	})

	return
}

// getTicker 获取ticker行情 函数本身不需要返回值 通过callbackFunc传递出去
func (rs *BinanceUsdtSwapRs) getTicker(symbol string) (rspTicker helper.Ticker) {
	// 请求必备信息
	uri := "/fapi/v1/depth"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	params["limit"] = 5
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var err error
	var value *fastjson.Value
	var handlerErr error

	err = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("getTicker error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		// 可能会返回  {"code":-1121,"msg":"Invalid symbol."}
		asks := value.GetArray("asks")
		bids := value.GetArray("bids")
		if len(asks) > 0 && len(bids) > 0 {
			seqEx := helper.MustGetInt64(value, "T")
			ask := asks[0].GetArray()
			ap := helper.MustGetFloat64FromBytes(ask[0])
			aq := helper.MustGetFloat64FromBytes(ask[1])
			bid := bids[0].GetArray()
			bp := helper.MustGetFloat64FromBytes(bid[0])
			bq := helper.MustGetFloat64FromBytes(bid[1])
			if symbol == rs.Symbol {
				if rs.TradeMsg.Ticker.Seq.NewerAndStore(seqEx, time.Now().UnixNano()) {
					rs.TradeMsg.Ticker.Set(ap, aq, bp, bq)
				}
				rs.Cb.OnTicker(0)
			}
			rspTicker.Set(ap, aq, bp, bq)
			rspTicker.Seq.Ex.Store(seqEx)
		}
	})
	if err != nil {
		rs.Logger.Errorf("getTicker error %v", err)
		//得检查是否有限频提示
	}
	return
}

func (rs *BinanceUsdtSwapRs) GetEquity() (resp helper.Equity, err helper.ApiError) { //获取资产

	url := "/fapi/v2/account" // 权重: 5
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	params := make(map[string]interface{})

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getEquity error %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			return
		}
		data := value.GetArray("assets")
		ts := time.Now().UnixNano()
		for _, v := range data {
			a := helper.MustGetStringLowerFromBytes(v, "asset")
			timeInEx := helper.GetInt64(v, "updateTime")
			walletBalance := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("walletBalance")))
			unrealizedProfit := helper.MustGetFloat64FromBytes(v, "unrealizedProfit")
			avail := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("availableBalance")))

			fieldsSet := (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)
			if e, ok := rs.EquityNewerAndStore(a, timeInEx, ts, fieldsSet); ok {
				e.TotalWithUpl = walletBalance + unrealizedProfit
				e.TotalWithoutUpl = walletBalance
				e.Avail = avail
				e.Upl = unrealizedProfit
				// todo 没有seq，上层需要判断是否来自 rs or ws吗？需要什么信息判断？
				rs.Cb.OnEquityEvent(0, *e)
			}

			// if rs.tradeMsg.Equity.Seq.NewerAndStore(updateTime, ns) {
			// 	rs.tradeMsg.Equity.Lock.Lock()
			// 	rs.tradeMsg.Equity.Cash = walletBalance + unrealizedProfit
			// 	rs.tradeMsg.Equity.CashFree = avail
			// 	rs.tradeMsg.Equity.Lock.Unlock()
			// }

			// if rs.tradeMsg.RsEquity.Seq.NewerAndStore(updateTime, ns) {
			// 	rs.tradeMsg.RsEquity.Lock.Lock()
			if strings.EqualFold(a, rs.PairInfo.Pair.Quote) {
				resp.Cash = walletBalance + unrealizedProfit
				resp.CashFree = avail
				resp.CashUpl = unrealizedProfit
				resp.IsSet = true
			}
			// 	rs.tradeMsg.RsEquity.Lock.Unlock()
			// }
			// rs.Cb.OnEquity(0)

			// resp = rs.tradeMsg.Equity
		}
	})
	if !err.Nil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("getEquity error %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	base.EnsureIsRsExposerOuter(rs)
	return
}

func (rs *BinanceUsdtSwapRs) generateListenKey() (listenKey string, err error) {
	url := "/fapi/v1/listenKey"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	reqParams := make(map[string]interface{})

	err1 := rs.doRequest(http.MethodPost, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("GenerateListenKey error %v", handlerErr)
			return
		}
		listenKey = string(value.GetStringBytes("listenKey"))

	})
	if err1 != nil {
		rs.Logger.Errorf("GenerateListenKey error %v", err1)
		return "", err1
	}

	return listenKey, nil
}

func (rs *BinanceUsdtSwapRs) keepListenKey(listenKey string) (err error) {
	var handlerErr error
	var value *fastjson.Value

	p := handyPool.Get()
	defer handyPool.Put(p)
	url := "/fapi/v1/listenKey"
	reqParams := make(map[string]interface{})
	reqParams["listenKey"] = listenKey
	err1 := rs.doRequest(http.MethodPut, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("keepListenKey error %v", handlerErr)
			return
		}
		rs.Logger.Infof("listenkey 续期成功 %v ", value)
	})
	if err1 != nil {
		rs.Logger.Errorf("keepListenKey error %v", err1)
		return err1
	}

	return nil
}

func (rs *BinanceUsdtSwapRs) deleteListenKey(listenKey string) (err error) {
	var handlerErr error
	var value *fastjson.Value

	p := handyPool.Get()
	defer handyPool.Put(p)
	url := "/fapi/v1/listenKey"
	reqParams := make(map[string]interface{})
	reqParams["listenKey"] = listenKey
	err1 := rs.doRequest(http.MethodDelete, url, reqParams, nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("deleteListenKey error %v", handlerErr)
			return
		}
		rs.Logger.Infof("listenkey 删除成功 %v ", value)
	})
	if err1 != nil {
		rs.Logger.Errorf("deleteListenKey error %v", err1)
		return err1
	}

	return nil
}

func (rs *BinanceUsdtSwapRs) DoGetAccountMode(pairInfo *helper.ExchangeInfo) (leverage int, marginMode helper.MarginMode, posMode helper.PosMode, err helper.ApiError) {
	symbol := pairInfo.Symbol
	url := "/fapi/v1/symbolConfig"
	params := make(map[string]interface{})
	params["symbol"] = symbol

	// 调用 Binance API 获取账户信息
	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		var value *fastjson.Value
		p := handyPool.Get()
		defer handyPool.Put(p)

		// 解析返回的 JSON 数据
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("failed to parse account info response: %v", handlerErr)
			err = helper.ApiError{NetworkError: handlerErr}
			return
		}

		// 获取 positions 数组
		positions := helper.MustGetArray(value)
		for _, pos := range positions {
			posSymbol := helper.MustGetStringFromBytes(pos, "symbol")
			if posSymbol == symbol {
				// 解析杠杆率
				leverage = int(helper.MustGetFloat64(pos, "leverage"))

				// 解析保证金模式
				if helper.MustGetShadowStringFromBytes(pos, "marginType") == "CROSSED" {
					marginMode = helper.MarginMode_Cross
				} else {
					marginMode = helper.MarginMode_Iso
				}
				return
			}
		}

		// 如果没有找到对应的 symbol，返回错误
		err = helper.ApiErrorNil
	})

	url = "/fapi/v1/positionSide/dual"
	params = make(map[string]interface{})

	// 调用 Binance API 获取账户信息
	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		var value *fastjson.Value
		p := handyPool.Get()
		defer handyPool.Put(p)

		// 解析返回的 JSON 数据
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("failed to parse account info response: %v", handlerErr)
			err = helper.ApiError{NetworkError: handlerErr}
			return
		}

		// 获取 positions 数组
		isDual := helper.MustGetBool(value, "dualSidePosition")
		if isDual {
			posMode = helper.PosModeHedge
		} else {
			posMode = helper.PosModeOneway
		}

		// 如果没有找到对应的 symbol，返回错误
		err = helper.ApiErrorNil
	})

	if !err.Nil() {
		rs.Logger.Errorf("failed to get account mode for symbol %s: %v", symbol, err)
	}
	return
}

// setLeverRate 设置账户的杠杆参数 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BinanceUsdtSwapRs) DoSetLeverage(pairInfo helper.ExchangeInfo, leverage int) (err helper.ApiError) {
	// 基本请求信息
	uri := "/fapi/v1/leverage"

	params := make(map[string]interface{})

	params["symbol"] = pairInfo.Symbol
	params["leverage"] = leverage // 一般支持20x 部分busd品种最高仅支持15x 为了通用性 降低到15 遇到过更奇葩的情况限制不能超过8x

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodPost, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("setLeverRate error %v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		LeverMap[pairInfo.Symbol] = leverage
		maxNotional := helper.MustGetFloat64FromBytes(value, "maxNotionalValue")
		rs.ExchangeInfoLabilePtrP2S.Set(pairInfo.Pair.String(), &helper.LabileExchangeInfo{
			Pair:           pairInfo.Pair,
			Symbol:         pairInfo.Symbol,
			SettedLeverage: leverage,
			RiskLimit: helper.RiskLimit{
				Underlying: strings.ToUpper(pairInfo.Pair.Quote),
				Amount:     maxNotional,
			},
		})
	})
	if err.NotNil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("setLeverRate error %v", err)
		return err
	}

	return
}

// setMarginMode 设置账户的保证金模式 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BinanceUsdtSwapRs) DoSetMarginMode(symbol string, mm helper.MarginMode) (err helper.ApiError) {
	// 基本请求信息
	uri := "/fapi/v1/marginType" // 权重: 1
	var marginType string

	if mm == helper.MarginMode_Cross {
		marginType = "CROSSED"
	} else if mm == helper.MarginMode_Iso {
		marginType = "ISOLATED"
	}

	params := make(map[string]interface{})

	params["symbol"] = symbol
	params["marginType"] = marginType

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	noNeedChange := false

	e := rs.call(http.MethodPost, uri, nil, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("setMarginMode error %v", err.HandlerError)
			return
		}
		if value.GetInt("code") == -4046 {
			// {"code":-4046,"msg":"No need to change margin type."}
			noNeedChange = true
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
	})
	if noNeedChange {
		return
	} else if e != nil {
		err.HandlerError = e
	}
	if err.NotNil() {
		rs.Logger.Errorf("setMarginMode error %v", err)
	}
	return
}

// setPositionMode 设置账户的持仓模式 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BinanceUsdtSwapRs) DoSetPositionMode(symbol string, pm helper.PosMode) (err helper.ApiError) {
	// 基本请求信息
	uri := "/fapi/v1/positionSide/dual" // 权重: 1

	params := make(map[string]interface{})

	params["dualSidePosition"] = "false"

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	var noNeedChange bool

	e := rs.call(http.MethodPost, uri, nil, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("setPositionMode error %v", err.HandlerError)
			return
		}

		// if code == -4059 { // {"code":-4059,"msg":"No need to change position side."}
		code := value.GetInt("code")
		if code == -4059 {
			noNeedChange = true
		}

		if !rs.isOkApiResponse(value, uri) {
			rs.Logger.Errorf("setPositionMode error %v", string(respBody))
		}

	})
	if noNeedChange {
		return
	} else if e != nil {
		err.HandlerError = e
	}
	if err.NotNil() {
		rs.Logger.Errorf("setMarginMode error %v", err)
	}
	return
}

// 撤掉所有挂单 https://binance-docs.github.io/apidocs/futures/cn/#user_data-4
func (rs *BinanceUsdtSwapRs) cancelAllOpenOrders(only bool) {
	uri := "/fapi/v1/openOrders"

	params := make(map[string]interface{})
	if only {
		params["symbol"] = rs.Symbol // 请小心使用不带symbol参数的调用 权重: - 带symbol 1 - 不带 40
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
			rs.Logger.Errorf("cancelAllOpenOrders error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data, _ := value.Array()
		canceledSymbols := make(map[string]int)
		for _, v := range data {
			symbol := helper.BytesToString(v.GetStringBytes("symbol"))
			if _, ok := canceledSymbols[symbol]; !ok {
				canceledSymbols[symbol] = 1
				// 执行此交易对撤单
				rs.DoCancelPendingOrders(symbol)
				time.Sleep(time.Second)
			}
		}
	})
	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("cancelAllOpenOrders error %v", err)
	}
}

// 撤掉单交易对所有挂单
// https://binance-docs.github.io/apidocs/futures/cn/#trade-7
func (rs *BinanceUsdtSwapRs) DoCancelPendingOrders(symbol string) (err helper.ApiError) {

	url := "/fapi/v1/allOpenOrders"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["symbol"] = symbol

	err.NetworkError = rs.call(http.MethodDelete, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("DoCancelPendingOrders error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			return
		}
	})
	if err.NotNil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("DoCancelPendingOrders error %v", err)
	}
	return err
}

func (rs *BinanceUsdtSwapRs) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/fapi/v1/openOrders"
	params := make(map[string]interface{})
	if only {
		params["symbol"] = rs.Symbol // 请小心使用不带symbol参数的调用 权重: - 带symbol 1 - 不带 40
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
			rs.Logger.Errorf("cancelAllOpenOrders error %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := helper.MustGetArray(value)
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

// adjustAcct 开始交易前 把账户调整到合适的状态 包括调整杠杆 仓位模式 买入一定数量的平台币等等
func (rs *BinanceUsdtSwapRs) adjustAcct() {
	if !rs.BrokerConfig.InitialAcctConfig.IsEmpty() {
		rs.SetAccountMode(rs.Pair.String(), rs.BrokerConfig.InitialAcctConfig.MaxLeverage, rs.BrokerConfig.InitialAcctConfig.MarginMode, rs.BrokerConfig.InitialAcctConfig.PosMode)
		return
	}
	// step 1
	rs.getLeverage()
	rs.DoSetLeverage(*rs.PairInfo, rs.PairInfo.MaxLeverage)
	// step 2
	rs.DoSetMarginMode(rs.PairInfo.Symbol, helper.MarginMode_Cross)
	// step 3
	rs.DoSetPositionMode(rs.Symbol, helper.PosModeOneway)
}

// BeforeTrade 开始交易前需要做的所有工作 调整好杠杆
func (rs *BinanceUsdtSwapRs) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	// 获取交易规则
	rs.getExchangeInfo()
	rs.UpdateExchangeInfo(rs.ExchangeInfoPtrP2S, rs.ExchangeInfoPtrS2P, rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	needWs := true
	switch mode {
	case helper.HandleModePublic:
		rs.getTicker(rs.Symbol)
		needWs = false
		return
	case helper.HandleModePrepare:
		needWs = false
		rs.getPosition(&helper.Pair{}, true, false)
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
	rs.adjustAcct()
	// 获取账户资金
	rs.GetEquity()
	// 获取ticker
	rs.getTicker(rs.Symbol)
	rs.PrintAcctSumWhenBeforeTrade(rs)
	if needWs {
		rs.Run()
	}
	return
}

// AfterTrade 结束交易时需要做的所有工作  清空挂单和仓位
// 返回值：清仓前是否有遗漏仓位(会多次调用，使用清仓前漏仓合理)，默认true，100%确定没有才false。忽略碎仓。
func (rs *BinanceUsdtSwapRs) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	switch mode {
	case helper.HandleModePrepare:
		// 有些模式不需要重复清仓
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

func (rs *BinanceUsdtSwapRs) GetPosition() (resp []helper.PositionSum, err helper.ApiError) {
	return rs.getPosition(&helper.Pair{}, true, true)
}

// getPosition 获取仓位 获取之后触发回调
func (rs *BinanceUsdtSwapRs) getPosition(pair *helper.Pair, only bool, needResp bool) (resp []helper.PositionSum, err helper.ApiError) {
	uri := "/fapi/v2/positionRisk"

	params := make(map[string]interface{})
	symbolWanted := rs.Symbol
	if only {
		if pair.Quote != "" {
			symbolWanted = rs.PairToSymbol(pair)
		}
		params["symbol"] = symbolWanted
	}

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	// 发起请求
	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getPosition parse error: %s", err.HandlerError.Error())
			return
		}

		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("%s", string(respBody))
			return
		}

		datas, _ := value.Array()

		if needResp {
			resp = make([]helper.PositionSum, 0, len(datas))
		}

		for _, data := range datas {
			symbol := helper.BytesToString(data.GetStringBytes("symbol"))
			if only && symbol != symbolWanted {
				continue
			}
			longPos := fixed.ZERO
			longAvg := 0.0
			shortPos := fixed.ZERO
			shortAvg := 0.0
			positionAmt := fixed.NewS(helper.BytesToString(data.GetStringBytes("positionAmt")))
			price := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("entryPrice")))
			positionSide := helper.BytesToString(data.GetStringBytes("positionSide"))
			if positionSide == "BOTH" {
				if positionAmt.GreaterThan(fixed.ZERO) {
					longPos = positionAmt
					longAvg = price
				} else if positionAmt.LessThan(fixed.ZERO) {
					shortPos = positionAmt.Abs()
					shortAvg = price
				}
			}
			// 会返回期权，在这里过滤
			info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
			if !ok {
				continue
			}
			updateTime := helper.MustGetInt64(data, "updateTime")
			tsns := time.Now().UnixNano()
			if needResp && !positionAmt.Equal(fixed.ZERO) {
				side := helper.PosSideLong
				if positionAmt.LessThan(fixed.ZERO) {
					side = helper.PosSideShort
				}
				resp = append(resp, helper.PositionSum{
					Name:        info.Pair.String(),
					Amount:      positionAmt.Abs().Float(),
					AvailAmount: positionAmt.Abs().Float(),
					Ave:         price,
					Side:        side,
					Mode:        helper.PosModeOneway,
					Seq:         helper.NewSeq(updateTime, tsns),
				})
			}

			// rs rsp "updateTime" 与 ws rsp E/T字段不是同一个，多数时间相等，但他们服务器负载高会出现偏差。ref: ci报告[nightly] 20240914-183927
			if pos, ok := rs.PosChangedAndFaraway(symbol, longPos, shortPos); ok {
				pos.Lock.Lock()
				pos.LongPos = longPos
				pos.LongAvg = longAvg
				pos.ShortPos = shortPos
				pos.ShortAvg = shortAvg
				event := pos.ToPositionEvent()

				pos.Lock.Unlock()
				rs.Cb.OnPositionEvent(0, event)
			}

			// if symbol == rs.symbol {
			// 	if !rs.tradeMsg.Position.Seq.NewerAndStore(updateTime, time.Now().UnixNano()) {
			// 		continue
			// 	}
			// 	if positionAmt.IsZero() {
			// 		rs.tradeMsg.Position.Reset() // 双向持仓会有问题
			// 		rs.Cb.OnPosition(0)
			// 		continue
			// 	} else {
			// 		hasPos = true
			// 	}
			// }
			//switch positionSide {
			//case "LONG":
			//	longPos = positionAmt
			//	longAvg = price
			//case "SHORT":
			//	shortPos = positionAmt.Abs()
			//	shortAvg = price
			//case "BOTH":
			//	if positionAmt.GreaterThan(fixed.ZERO) {
			//		longPos = positionAmt
			//		longAvg = price
			//	} else if positionAmt.LessThan(fixed.ZERO) {
			//		shortPos = positionAmt.Abs()
			//		shortAvg = price
			//	}
			//}
		}
		// if hasPos {
		// rs.tradeMsg.Position.Reset()
		// rs.tradeMsg.Position.Lock.Lock()
		// rs.tradeMsg.Position.ShortPos = shortPos
		// rs.tradeMsg.Position.ShortAvg = shortAvg
		// rs.tradeMsg.Position.LongPos = longPos
		// rs.tradeMsg.Position.LongAvg = longAvg
		// rs.tradeMsg.Position.Time = time.Now().UnixMilli()
		// rs.tradeMsg.Position.Lock.Unlock()
		// rs.Cb.OnPosition(0)

		// }
	})

	if !err.Nil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("getPosition error:%s", err.Error())
	}
	return
}
func (rs *BinanceUsdtSwapRs) rsPlaceOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, colo bool) {
	req := rs.orderPlaceReqPool.Get(rs)
	defer rs.orderPlaceReqPool.Put(req)
	symbol := info.Symbol
	if err := req.ResetParams(rs, symbol, info, price, size, cid, side, orderType, colo); err != nil {
		var order helper.OrderEvent
		order.Pair = info.Pair
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
		return
	}
	var handlerErr error
	var err error
	var value *fastjson.Value

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	start := time.Now().UnixMicro()
	c := rs.client
	if colo {
		c = rs.clientColo
	}
	_, err = c.RequestPure(&req.Request, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		handledOk := false
		defer func() {
			if !handledOk {
				order := helper.OrderEvent{Type: helper.OrderEventTypeERROR, ClientID: cid}
				order.Pair = info.Pair
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
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, handlerErr = p.ParseBytes(respBody)
		failed := false
		if handlerErr != nil {
			// 出现错误的时候也要触发回调, 不是json格式
			var order helper.OrderEvent
			order.Pair = info.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
			rs.ReqFail(base.FailNumActionIdx_Place)
			failed = true
		} else if !rs.isOkApiResponse(value, "/fapi/v1/order", nil) {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				var order helper.OrderEvent
				order.Pair = info.Pair
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
			}
			rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, string(respBody))
			rs.ReqFail(base.FailNumActionIdx_Place)
			failed = true
		}
		respHeader.VisitAll(func(key, value []byte) {
			k := helper.BytesToString(key)
			switch k {
			case "X-MBX-USED-WEIGHT-1M":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.request_weight_limit {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "X-MBX-ORDER-COUNT-10S":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_10s {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "X-MBX-ORDER-COUNT-1M":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_60s {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			}
		})

		if !failed {
			var order helper.OrderEvent
			order.Pair = info.Pair

			// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
			order.Type = helper.OrderEventTypeNEW
			order.OrderID = strconv.FormatInt(helper.MustGetInt64(value, "orderId"), 10)
			order.ClientID = string(value.GetStringBytes("clientOrderId"))
			rs.Cb.OnOrder(0, order)
			rs.ReqSucc(base.FailNumActionIdx_Place)

			rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Place, Cid: order.ClientID, Oid: order.OrderID, DurationUs: end - start}
			if ok := rs.DelayMonitor.TryNext(rsp); ok {
				return
			}
		}
		handledOk = true
	})

	if err != nil {
		//得检查是否有限频提示
		if !rs.DelayMonitor.IsMonitorOrder(cid) {
			var order helper.OrderEvent
			order.Pair = info.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
		}
		rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

func (rs *BinanceUsdtSwapRs) wsPlaceOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, colo bool) {
	symbol := info.Symbol
	params := map[string]interface{}{}
	params["symbol"] = symbol
	switch side {
	case helper.OrderSideKD:
		params["side"] = "BUY"
	case helper.OrderSideKK:
		params["side"] = "SELL"
	case helper.OrderSidePD:
		params["side"] = "SELL"
		params["reduceOnly"] = "true"
	case helper.OrderSidePK:
		params["side"] = "BUY"
		params["reduceOnly"] = "true"
	}
	if orderType == helper.OrderTypeIoc {
		params["type"] = "LIMIT"
		params["timeInForce"] = "IOC"
	} else if orderType == helper.OrderTypePostOnly {
		params["type"] = "LIMIT"
		params["timeInForce"] = "GTX"
	} else if orderType == helper.OrderTypeMarket {
		params["type"] = "MARKET"
	} else if orderType == helper.OrderTypeLimit {
		params["type"] = "LIMIT"
		params["timeInForce"] = "GTC"
	} else {
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		order.Pair = info.Pair
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
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

	rs.SystemPass.UpdateSince(t / 1e3)
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
			order.Pair = info.Pair
			rs.Cb.OnOrder(0, order)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return nil
		},
	})
}
func (rs *BinanceUsdtSwapRs) signOrder(params map[string]interface{}) string {
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

func (rs *BinanceUsdtSwapRs) rsAmendOrder(info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid, oid string, side helper.OrderSide, t int64, colo bool) {
	req := rs.orderAmendReqPool.Get(rs)
	defer rs.orderAmendReqPool.Put(req)
	if err := req.ResetParams(rs, info, price, size, cid, oid, side, colo); err != nil {
		var order helper.OrderEvent
		order.Pair = info.Pair
		order.Type = helper.OrderEventTypeAmendFail
		order.ClientID = cid
		order.OrderID = oid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s改单失败 %s", rs.ExchangeName, cid, err.Error())
		return
	}
	var handlerErr error
	var err error
	var value *fastjson.Value

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	start := time.Now().UnixMicro()
	c := rs.client
	if colo {
		c = rs.clientColo
	}
	_, err = c.RequestPure(&req.Request, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, handlerErr = p.ParseBytes(respBody)
		failed := false
		if handlerErr != nil {
			// 出现错误的时候也要触发回调, 不是json格式
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeAmendFail
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s改单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
			failed = true
		} else if !rs.isOkApiResponse(value, "/fapi/v1/order", nil) {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				// 出现错误的时候也要触发回调, 不是json格式
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeAmendFail
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
			}
			rs.Logger.Errorf("[%s]%s改单失败 %s", rs.ExchangeName, cid, string(respBody))
			failed = true
		}
		if rs.BrokerConfig.ActivateDelayMonitor && !failed {
			rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Amend, AmendSucc: true, Cid: cid, Oid: oid, DurationUs: end - start}
			if ok := rs.DelayMonitor.TryNext(rsp); ok {
				return
			}
		}
		respHeader.VisitAll(func(key, value []byte) {
			k := helper.BytesToString(key)
			switch k {
			case "X-MBX-USED-WEIGHT-1M":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.request_weight_limit {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "X-MBX-ORDER-COUNT-10S":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_10s {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			case "X-MBX-ORDER-COUNT-1M":
				v, _ := strconv.Atoi(helper.BytesToString(value))
				if v > rs.order_limit_60s {
					rs.Cb.OnReset("即将触发限频 重置交易")
				}
			}
		})

		if !failed {
			var order helper.OrderEvent

			// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
			order.Type = helper.OrderEventTypeAmendSucc
			order.OrderID = strconv.FormatInt(helper.MustGetInt64(value, "orderId"), 10)
			order.ClientID = string(value.GetStringBytes("clientOrderId"))
			rs.Cb.OnOrder(0, order)
		}
	})

	if err != nil {
		//得检查是否有限频提示
		if !rs.DelayMonitor.IsMonitorOrder(cid) {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeAmendFail
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
		}
		rs.Logger.Errorf("[%s]%s改单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

func (rs *BinanceUsdtSwapRs) rsCancelOrder(info *helper.ExchangeInfo, oid, cid string, t int64, colo bool) {
	req := rs.orderCancelReqPool.Get(rs)
	defer rs.orderCancelReqPool.Put(req)
	if err := req.ResetParams(rs, info.Symbol, oid, cid, colo); err != nil {
		return
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	// var value *fastjson.Value

	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	start := time.Now().UnixMicro()
	c := rs.client
	if colo {
		c = rs.clientColo
	}
	_, err = c.RequestPure(&req.Request, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		_, handlerErr = p.ParseBytes(respBody)
		// if handlerErr != nil {
		// return
		// }
		// 撤单不需要触发回调 撤单动作高度时间敏感 依赖ws推送
		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Cancel, Cid: cid, Oid: oid, DurationUs: end - start}
		if ok := rs.DelayMonitor.TryNext(rsp); ok {
			return
		}
	})

	if err != nil {
		rs.Logger.Errorf("cancelOrder error: %s", err.Error())
		//得检查是否有限频提示
	}
	if handlerErr != nil {
		rs.Logger.Errorf("cancelOrder parse error: %s", handlerErr.Error())
	}
	if err == nil && handlerErr == nil {
		rs.CancelOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

func (rs *BinanceUsdtSwapRs) checkOrder(info *helper.ExchangeInfo, cid string, oid string) {
	uri := "/fapi/v1/order"

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
			rs.Logger.Errorf("checkOrder parse error: %s", handlerErr)
			return
		}
		if value.GetInt("code") == -2013 {
			order := helper.OrderEvent{
				Type:     helper.OrderEventTypeNotFound,
				Pair:     info.Pair,
				ClientID: cid,
				OrderID:  oid,
			}
			rs.Cb.OnOrder(0, order)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		if value.Type() == fastjson.TypeNull {
			// 查不到这个订单不做任何处理  等待策略层触发重置交易
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		status := helper.BytesToString(value.GetStringBytes("status"))

		// 查到订单
		var event helper.OrderEvent
		event.Pair = info.Pair
		event.Amount = fixed.NewS(helper.MustGetShadowStringFromBytes(value, "origQty"))
		event.Price = helper.GetFloat64FromBytes(value, "price")
		event.OrderID = strconv.FormatInt(helper.MustGetInt64(value, "orderId"), 10)
		event.ClientID = string(value.GetStringBytes("clientOrderId"))
		dealed := false
		switch status {
		case "NEW":
			event.Type = helper.OrderEventTypeNEW
		case "PARTIALLY_FILLED":
			event.Type = helper.OrderEventTypePARTIAL
			dealed = true
		case "CANCELED", "EXPIRED", "FILLED":
			event.Type = helper.OrderEventTypeREMOVE
			dealed = true
			// default:
			// return
		}
		if dealed {
			event.Filled = fixed.NewF(helper.MustGetFloat64FromBytes(value, "executedQty"))
			if event.Filled.GreaterThan(fixed.ZERO) {
				event.FilledPrice = helper.MustGetFloat64FromBytes(value, "cumQuote") / event.Filled.Float()
			}
		}
		switch helper.MustGetShadowStringFromBytes(value, "side") {
		case "BUY":
			event.OrderSide = helper.OrderSideKD
		case "SELL":
			event.OrderSide = helper.OrderSideKK
		default:
			rs.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(value, "side"))
			return
		}
		if helper.MustGetBool(value, "reduceOnly") {
			if event.OrderSide == helper.OrderSideKD {
				event.OrderSide = helper.OrderSidePK
			} else if event.OrderSide == helper.OrderSideKK {
				event.OrderSide = helper.OrderSidePD
			}
		}
		switch helper.MustGetShadowStringFromBytes(value, "type") {
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
		case "GTX":
			event.OrderType = helper.OrderTypePostOnly
		}
		rs.Cb.OnOrder(0, event)

	})

	if err != nil {
		rs.Logger.Errorf("checkOrder error: %s", err.Error())
	}
}
func (rs *BinanceUsdtSwapRs) wsCancelOrder(info *helper.ExchangeInfo, oid, cid string, t int64, colo bool) {
	params := map[string]interface{}{}
	params["symbol"] = info.Symbol
	if oid != "" {
		num, err := strconv.ParseInt(oid, 10, 64)
		if err != nil {
			rs.Logger.Errorf("oid to int conversioin:", err)
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

	rs.SystemPass.UpdateSince(t / 1e3)
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
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.Pair = info.Pair
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			return nil
		},
	})
}

func (rs *BinanceUsdtSwapRs) wsCheckOrder(info *helper.ExchangeInfo, cid string, oid string) {
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
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
			"id":     nil,
			"method": "order.status",
			"params": params,
		},
		Cb: func(msg map[string]interface{}) error {
			var order helper.OrderEvent
			order.Pair = info.Pair
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			return nil
		},
	})
}

func (rs *BinanceUsdtSwapRs) doRequest(reqMethod string, path string, reqParams map[string]interface{}, reqBody map[string]interface{}, requestHeaders map[string]string, respHandler rest.FastHttpRespHandler) error {
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

// call 专用于binance_usdt_swap的发起请求函数
func (rs *BinanceUsdtSwapRs) call(reqMethod string, reqUrl string, reqParams map[string]interface{}, reqBody map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler, apiErr ...*helper.ApiError) error {
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

	status, err := rs.client.Request(reqMethod, rs.baseUrl+reqUrl, helper.StringToBytes(bodyString.String()), reqHeaders, respHandler, apiErr...)
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
		if status == 429 {
			rs.Cb.OnReset("IP即将被封禁 紧急重置")
		}
		if status == 418 {
			rs.Cb.OnExit("IP被封禁 紧急停机")
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

func (rs *BinanceUsdtSwapRs) GetExName() string {
	return helper.BrokernameBinanceUsdtSwap.String()
}

func (rs *BinanceUsdtSwapRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	return rs.getPosition(&helper.Pair{}, false, true)
}

func (rs *BinanceUsdtSwapRs) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	_, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		err.NetworkError = fmt.Errorf("not found symbol %s", symbol)
		return
	}
	ticker = rs.getTicker(symbol)
	return
}
func (rs *BinanceUsdtSwapRs) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("not found symbol for pair %s", symbol)
		return false
	}
	cid := fmt.Sprintf("99%d", time.Now().UnixMilli())
	rs.rsPlaceOrder(info, 0, orderAmount, cid, orderSide, helper.OrderTypeMarket, 0, false)
	return true
}
func (rs *BinanceUsdtSwapRs) GetDealList(startTimeMs int64, endTimeMs int64) (resp []helper.DealForList, err helper.ApiError) {
	return rs.GetDealListInFather(rs, startTimeMs, endTimeMs)
}
func (rs *BinanceUsdtSwapRs) DoGetDealList(startTimeMs int64, endTimeMs int64) (resp helper.DealListResponse, err helper.ApiError) {
	for _, p := range rs.BrokerConfig.Pairs {
		symbol := rs.PairToSymbol(&p)
		o, _ := rs.doGetDealList(symbol, startTimeMs, endTimeMs)
		resp.Deals = append(resp.Deals, o.Deals...)
		resp.HasMore = resp.HasMore || o.HasMore
	}
	return
}

func (rs *BinanceUsdtSwapRs) doGetDealList(symbol string, startTimeMs int64, endTimeMs int64) (resp helper.DealListResponse, err helper.ApiError) {
	const DEAL_MAX_LEN = 1000
	uri := "/fapi/v1/userTrades"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	params["startTime"] = startTimeMs
	params["endTime"] = endTimeMs
	params["limit"] = DEAL_MAX_LEN

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handler error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("handler error %v", value)
			return
		}

		deals := helper.MustGetArray(value)
		resp.Deals = make([]helper.DealForList, 0)
		for _, v := range deals {
			// {
			//   "buyer": false,
			//   "commission": "-0.07819010",
			//   "commissionAsset": "USDT",
			//   "id": 698759,
			//   "maker": false,
			//   "orderId": 25851813,
			//   "price": "7819.01",
			//   "qty": "0.002",
			//   "quoteQty": "15.63802",
			//   "realizedPnl": "-0.91539999",
			//   "side": "SELL",
			//   "positionSide": "SHORT",
			//   "symbol": "BTCUSDT",
			//   "time": 1569514978020
			// }
			isBuy := helper.MustGetShadowStringFromBytes(v, "side") == "BUY"
			side := helper.OrderSideKD
			if isBuy {
				side = helper.OrderSideKD
			} else {
				side = helper.OrderSideKK
			}

			symbol := helper.MustGetStringFromBytes(v, "symbol")
			info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
			if !ok {
				rs.Logger.Errorf("not found symbol for pair %s", symbol)
				continue
			}
			deal := helper.DealForList{
				Symbol:          symbol,
				Pair:            info.Pair.String(),
				DealID:          helper.Itoa(helper.MustGetInt64(v, "id")),
				OrderID:         helper.Itoa(helper.MustGetInt64(v, "orderId")),
				ClientID:        "no-client-id",
				FilledThis:      fixed.NewF(helper.MustGetFloat64(v, "qty")),
				FilledPriceThis: helper.MustGetFloat64FromBytes(v, "price"),
				Fee:             helper.MustGetFloat64FromBytes(v, "commission"),
				IsTaker:         !helper.MustGetBool(v, "maker"),
				TradeTimeMs:     helper.MustGetInt64(v, "time"),
				OrderSide:       side,
			}
			resp.Deals = append(resp.Deals, deal)
		}
		resp.HasMore = len(value.GetArray()) == DEAL_MAX_LEN
	})
	return
}

func (rs *BinanceUsdtSwapRs) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	return rs.GetOrderListInFather(rs, startTimeMs, endTimeMs, orderState)
}

func (rs *BinanceUsdtSwapRs) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	for _, p := range rs.BrokerConfig.Pairs {
		symbol := rs.PairToSymbol(&p)
		o, _ := rs.doGetOrderList(symbol, startTimeMs, endTimeMs, orderState)
		resp.Orders = append(resp.Orders, o.Orders...)
		resp.HasMore = resp.HasMore || o.HasMore
	}
	return
}
func (rs *BinanceUsdtSwapRs) doGetOrderList(symbol string, startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	const _LEN = 1000
	uri := "/fapi/v1/allOrders"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	params["limit"] = _LEN
	params["startTime"] = startTimeMs
	params["endTime"] = endTimeMs

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handler error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("handler error %v", value)
			return
		}
		orders := helper.MustGetArray(value)
		resp.Orders = make([]helper.OrderForList, 0, len(orders))
		if len(orders) >= _LEN {
			resp.HasMore = true
		}

		for _, v := range orders {
			order := helper.OrderForList{
				Symbol:        helper.MustGetStringFromBytes(v, "symbol"),
				OrderID:       strconv.Itoa(helper.MustGetInt(v, "orderId")),
				ClientID:      helper.MustGetStringFromBytes(v, "clientOrderId"),
				Price:         helper.GetFloat64FromBytes(v, "price"),
				Amount:        fixed.NewS(helper.GetShadowStringFromBytes(v, "origQty")),
				CreatedTimeMs: helper.GetInt64(v, "time"),
				UpdatedTimeMs: helper.GetInt64(v, "updateTime"),
				Filled:        fixed.NewS(helper.GetShadowStringFromBytes(v, "executedQty")),
				FilledPrice:   helper.GetFloat64FromBytes(v, "avgPrice"),
			}
			status := helper.BytesToString(v.GetStringBytes("status"))

			switch status {
			case "NEW", "PARTIALLY_FILLED":
				order.OrderState = helper.OrderStatePending
			case "CANCELED", "EXPIRED", "FILLED":
				order.OrderState = helper.OrderStateFinished
				order.UpdatedTimeMs = helper.GetInt64(v, "updateTime")
			}
			if orderState != helper.OrderStateAll && orderState != order.OrderState {
				continue
			}
			side := helper.MustGetShadowStringFromBytes(v, "side")
			switch side {
			case "BUY":
				order.OrderSide = helper.OrderSideKD
			case "SELL":
				order.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %s", side)
				err.HandlerError =
					fmt.Errorf("side error: %s", side)
				return
			}
			if helper.MustGetBool(v, "reduceOnly") {
				if order.OrderSide == helper.OrderSideKD {
					order.OrderSide = helper.OrderSidePK
				} else if order.OrderSide == helper.OrderSideKK {
					order.OrderSide = helper.OrderSidePD
				}
			}
			switch helper.MustGetShadowStringFromBytes(v, "type") {
			case "LIMIT":
				order.OrderType = helper.OrderTypeLimit
			case "MARKET":
				order.OrderType = helper.OrderTypeMarket
			case "LIMIT_MAKER":
				order.OrderType = helper.OrderTypePostOnly
			}
			// 必须在type后面。忽略其他类型
			switch helper.MustGetShadowStringFromBytes(v, "timeInForce") {
			case "IOC":
				order.OrderType = helper.OrderTypeIoc
			case "GTX":
				order.OrderType = helper.OrderTypePostOnly
			}
			resp.Orders = append(resp.Orders, order)
		}
	}, &err)
	return
}

func (rs *BinanceUsdtSwapRs) GetFee() (fee helper.Fee, err helper.ApiError) {

	uri := "/fapi/v1/commissionRate"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getFundingRate error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("handler error %v", value)
			return
		}

		fee.Maker = helper.MustGetFloat64FromBytes(value, "makerCommissionRate")
		fee.Taker = helper.MustGetFloat64FromBytes(value, "takerCommissionRate")
	}, &err)
	return
}
func (rs *BinanceUsdtSwapRs) GetFundingRate() (helper.FundingRate, error) {
	uri := "/fapi/v1/premiumIndex"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var err error
	var value *fastjson.Value
	//没有就默认8小时 @Weisheng
	fr := helper.FundingRate{
		IntervalHours: 8,
	}

	err = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("getFundingRate error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		fr.UpdateTimeMS = helper.MustGetInt64(value, "time")
		fr.FundingTimeMS = helper.MustGetInt64(value, "nextFundingTime")
		fr.Rate = helper.MustGetFloat64FromBytes(value, "lastFundingRate")
		fr.Pair = rs.Pair

		indexPrice := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("indexPrice")))
		markPrice := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("markPrice")))
		if rs.Cb.OnIndex != nil {
			rs.Cb.OnIndex(0, helper.IndexEvent{IndexPrice: indexPrice})
		}
		if rs.Cb.OnMark != nil {
			rs.Cb.OnMark(0, helper.MarkEvent{MarkPrice: markPrice})
		}
	})
	uri = "/fapi/v1/fundingInfo"
	params = make(map[string]interface{})

	err = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("getFundingRate error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		for _, v := range value.GetArray() {
			if helper.MustGetShadowStringFromBytes(v, "symbol") == rs.Symbol {
				fr.IntervalHours = helper.MustGetInt(v, "fundingIntervalHours")
				break
			}
		}
	})
	return fr, err
}

func (rs *BinanceUsdtSwapRs) GetFundingRates(pairs []string) (res map[string]helper.FundingRate, err helper.ApiError) {
	uri := "/fapi/v1/premiumIndex"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	// {
	//     "symbol": "BTCUSDT",
	//     "markPrice": "11793.63104562",  // mark price
	//     "indexPrice": "11781.80495970", // index price
	//     "estimatedSettlePrice": "11781.16138815", // Estimated Settle Price, only useful in the last hour before the settlement starts.
	//     "lastFundingRate": "0.00038246",  // This is the lastest estimated funding rate
	//     "nextFundingTime": 1597392000000,
	//     "interestRate": "0.00010000",
	//     "time": 1597370495002
	// }

	err.NetworkError = rs.call(http.MethodGet, uri, nil, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getFundingRate error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, nil) {
			return
		}
		datas := helper.MustGetArray(value)

		if len(pairs) == 0 {
			res = make(map[string]helper.FundingRate, rs.ExchangeInfoPtrS2P.Count())
			for _, data := range datas {
				pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "symbol"))
				if ok {
					var fr = helper.FundingRate{Pair: pair.Pair, FundingTimeMS: helper.MustGetInt64(data, "nextFundingTime"), UpdateTimeMS: helper.MustGetInt64(data, "time"), Rate: helper.MustGetFloat64FromBytes(data, "lastFundingRate")}
					fr.IntervalHours = 8
					res[pair.Pair.String()] = fr
				}
			}
		} else {
			res = make(map[string]helper.FundingRate, len(pairs))
			for _, p := range pairs {
				res[p] = helper.FundingRate{}
			}
			for _, data := range datas {
				pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "symbol"))
				if ok {
					if _, ok := res[pair.Pair.String()]; ok {
						var fr = helper.FundingRate{Pair: pair.Pair, FundingTimeMS: helper.MustGetInt64(data, "nextFundingTime"), UpdateTimeMS: helper.MustGetInt64(data, "time"), Rate: helper.MustGetFloat64FromBytes(data, "lastFundingRate")}
						//默认8小时 @Weisheng
						fr.IntervalHours = 8
						res[pair.Pair.String()] = fr
					}
				}
			}
		}
	})
	uri = "/fapi/v1/fundingInfo"
	err.NetworkError = rs.call(http.MethodGet, uri, nil, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getFundingRate error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, nil) {
			return
		}
		datas := value.GetArray()
		for _, data := range datas {
			pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "symbol"))
			if ok {
				if fr, ok := res[pair.Pair.String()]; ok {
					fr.IntervalHours = helper.MustGetInt(data, "fundingIntervalHours")
					res[pair.Pair.String()] = fr
				}
			}
		}
	})

	if helper.DEBUGMODE {
		rs.Logger.Info("Fundingrate", res)
	}
	return
}
func (rs *BinanceUsdtSwapRs) GetIndexs(pairs []string) (res map[string]float64, err helper.ApiError) {
	uri := "/fapi/v1/premiumIndex"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	// {
	//     "symbol": "BTCUSDT",
	//     "markPrice": "11793.63104562",  // mark price
	//     "indexPrice": "11781.80495970", // index price
	//     "estimatedSettlePrice": "11781.16138815", // Estimated Settle Price, only useful in the last hour before the settlement starts.
	//     "lastFundingRate": "0.00038246",  // This is the lastest estimated funding rate
	//     "nextFundingTime": 1597392000000,
	//     "interestRate": "0.00010000",
	//     "time": 1597370495002
	// }

	err.NetworkError = rs.call(http.MethodGet, uri, nil, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("GetIndexs error %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri, nil) {
			return
		}
		datas := helper.MustGetArray(value)

		if len(pairs) == 0 {
			res = make(map[string]float64, rs.ExchangeInfoPtrS2P.Count())
			for _, data := range datas {
				pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "symbol"))
				if ok {
					res[pair.Pair.String()] = helper.MustGetFloat64FromBytes(data, "indexPrice")
				}
			}
		} else {
			res = make(map[string]float64, len(pairs))
			for _, p := range pairs {
				res[p] = 0
			}
			for _, data := range datas {
				pair, ok := rs.ExchangeInfoPtrS2P.Get(helper.MustGetShadowStringFromBytes(data, "symbol"))
				if ok {
					if _, ok := res[pair.Pair.String()]; ok {
						res[pair.Pair.String()] = helper.MustGetFloat64FromBytes(data, "indexPrice")
					}
				}
			}
		}
	})

	if helper.DEBUGMODE {
		rs.Logger.Info("Indexs", res)
	}
	return
}
func (rs *BinanceUsdtSwapRs) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	a.Balances, err = rs.getEquity()
	if !err.Nil() {
		return
	}
	a.Positions, err = rs.getPosition(&rs.Pair, false, true)
	return
}
func (rs *BinanceUsdtSwapRs) getEquity() (a []helper.BalanceSum, err helper.ApiError) {
	// 获取账户资产
	var url string
	var value *fastjson.Value
	var params map[string]interface{}

	url = "/fapi/v2/account"
	p := handyPool.Get()
	defer handyPool.Put(p)

	params = make(map[string]interface{})

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getAcctSum parse error: %s", err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = fmt.Errorf("API response not OK for URL: %s", url)
			return
		}
		data := value.GetArray("assets")
		if data == nil {
			err.HandlerError = fmt.Errorf("no assets data found in response")
			return
		}
		ts := time.Now().UnixNano()
		for _, v := range data {
			asset := strings.ToLower(helper.BytesToString(v.GetStringBytes("asset")))
			cash := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("walletBalance")))
			unrealizedProfit := helper.MustGetFloat64FromBytes(v, "unrealizedProfit")
			cashFree := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("availableBalance")))
			if cash != 0 || unrealizedProfit != 0 {
				var price float64
				if asset == "usdt" || asset == "busd" || asset == "usdc" || asset == "fdusd" {
					price = 1
				}
				a = append(a, helper.BalanceSum{
					Name:   asset,
					Price:  price,
					Amount: cash + unrealizedProfit,
					Avail:  cashFree,
				})
			}
			timeInEx := helper.GetInt64(v, "updateTime")
			fieldsSet := (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)
			if e, ok := rs.EquityNewerAndStore(asset, timeInEx, ts, fieldsSet); ok {
				e.TotalWithUpl = cash + unrealizedProfit
				e.TotalWithoutUpl = cash
				e.Avail = cashFree
				e.Upl = unrealizedProfit
				// todo 没有seq，上层需要判断是否来自 rs or ws吗？需要什么信息判断？
				rs.Cb.OnEquityEvent(0, *e)
			}
		}
	})
	if !err.Nil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("getEquity error %v", err)
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	base.EnsureIsRsExposerOuter(rs)
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
// binance 合约批量下单没优势 批量下单权重5 https://binance-docs.github.io/apidocs/futures/cn/#trade-5
func (rs *BinanceUsdtSwapRs) SendSignal(signals []helper.Signal) {
	for _, s := range signals {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送信号 %s", s.String())
		}
		switch s.Type {
		case helper.SignalTypeNewOrder:
			go rs.PlaceOrderSelect(s)
		case helper.SignalTypeAmend:
			go rs.AmendOrderSelect(s)
		case helper.SignalTypeCancelOrder:
			go rs.CancelOrderSelect(s)
		case helper.SignalTypeCheckOrder:
			info, ok := rs.GetPairInfoByPair(&s.Pair)
			if !ok {
				continue
			}
			if s.SignalChannelType == helper.SignalChannelTypeRs || rs.priWs == nil {
				go rs.checkOrder(info, s.ClientID, s.OrderID)
			} else {
				go rs.wsCheckOrder(info, s.ClientID, s.OrderID)
			}
		case helper.SignalTypeGetPos:
			go rs.getPosition(&s.Pair, true, false)
		case helper.SignalTypeGetEquity:
			go rs.getEquity()
		case helper.SignalTypeGetIndex:
			go rs.GetFundingRate() // 获取指数价格和获取资金费率是一个接口，在获取资费里一并实现
		case helper.SignalTypeGetTicker:
			go rs.getTicker(rs.Symbol)
		case helper.SignalTypeGetLiteTicker:
			go rs.getLiteTicker(rs.Symbol)
		case helper.SignalTypeCancelOne:
			go rs.cancelAllOpenOrders(true)
		case helper.SignalTypeCancelAll:
			go rs.cancelAllOpenOrders(false)
		}
	}
}

// Do 发起任意请求 一般用于非交易任务 对时间不敏感
func (rs *BinanceUsdtSwapRs) Do(doAct string, doParams any) (any, error) {
	switch doAct {
	case "getTicker":
		rs.getTicker(rs.Symbol)
		return nil, nil
	case "getConState":
		uri := "/fapi/v1/clientConState"
		params := make(map[string]interface{})

		// 必备变量
		p := handyPool.Get()
		defer handyPool.Put(p)

		rs.call(http.MethodGet, uri, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			fmt.Println("resp: ", string(respBody))
		})
		return nil, nil
	case "GetSpot":
		response, err := http.Get("https://api.binance.com/api/v3/avgPrice?symbol=" + doParams.(string))
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return 0, err
		}
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return 0, err
		}
		price, err := strconv.ParseFloat(result["price"].(string), 64)
		if err != nil {
			return 0, err
		}
		return price, nil
	}
	return nil, nil
}

func (rs *BinanceUsdtSwapRs) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltEquityAvailReducedWhenPendingOrder | //
		base.AbltRsPriGetPosWithSeq | // 2024-9-14 实锤 bn swap,  rs rsp "updateTime" 与 ws rsp E/T字段不是同一个，多数时间相等，但他们服务器负载高会出现偏差。ref: ci报告[nightly] 20240914-183927
		base.AbltWsPriEquityAvailReducedWhenHold // "ACCOUNT_UPDATE" 中 cw 余额字段只对逐仓有效
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
func (rs *BinanceUsdtSwapRs) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SWAP | base.AbltOrderAmend | base.AbltOrderAmendByCid | base.ABILITIES_SEQ |
		base.AbltWsPriPosFasterOrder | base.AbltWsPriOrderFee | base.AbltWsPriOrderEventHasSeqSameAsTicker | base.AbltWsPriPositionHasSeqSameAsTicker |
		base.AbltRsPriGetAllIndex | base.AbltRsPriGetAllFundingRate
}

func (rs *BinanceUsdtSwapRs) WsLogged() bool {
	return true
}

// 获取全市场交易规则
func (rs *BinanceUsdtSwapRs) GetExchangeInfos() []helper.ExchangeInfo {
	base.EnsureIsRsFeatures(rs)
	return rs.getExchangeInfo()
}

func (rs *BinanceUsdtSwapRs) GetFeatures() base.Features {
	f := base.Features{
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		UpdateWsTickerWithSeq: true,
		UpdateWsDepthWithSeq:  true,
		GetFee:                true,
		GetOrderList:          true,
		GetTickerSignal:       true,
		GetFundingRate:        true,
		MultiSymbolOneAcct:    true,
		OrderIOC:              true,
		OrderPostonly:         true,
		UnifiedPosClean:       true,
		Partial:               true,
		DelayInTicker:         true,
		GetLiteTickerSignal:   true,
		ExchangeInfo_1000XX:   true,
		Standby:               true,
		WsDepthLevel:          true,
		GetPriWs:              true,
		GetIndex:              true,
		AutoRefillMargin:      false, //统一账户下支持
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}

// Run 准备ws连接 仅第一次调用时连接ws
func (rs *BinanceUsdtSwapRs) Run() {
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

func (rs *BinanceUsdtSwapRs) DoStop() {
	if rs.stopCPri != nil {
		helper.CloseSafe(rs.stopCPri)
	}
	rs.connectOnce = sync.Once{}
	if rs.stopCPriColo != nil {
		rs.stopCPriColo <- struct{}{}
	}
	rs.stopFunc()
	rs.connectOnceColo = sync.Once{}
}

/*
2024-5-30 kc
发现个特点，通过ws req下单后， ws req event能收到订单回报;
但ws sub有时10秒都不会收到, 有时又会收到order event
*/
func (rs *BinanceUsdtSwapRs) priHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE {
		rs.Logger.Debugf("[%p]收到 rs's pri ws 推送 %s", rs, string(msg))
	}
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		rs.Logger.Errorf("Binance usdt swap rs's ws解析msg出错 err:%v", err)
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
			rs.ReqFail(base.FailNumActionIdx_Place)
			order.ClientID = cid.Cid
			order.Pair = cid.Pair
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 Binance usdt swap rs's ws status %s", rs.ExchangeName, cid, msg)
			return
		}
		rs.Logger.Errorf("[%s]%s下单失败 Binance usdt swap rs's ws status %s", rs.ExchangeName, id, msg)
		return
	}

	result := value.Get("result")
	status := helper.MustGetShadowStringFromBytes(result, "status")

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
	order.Amount = fixed.NewS(helper.MustGetShadowStringFromBytes(result, "origQty"))
	order.Price = helper.GetFloat64FromBytes(result, "price")
	symbol := helper.MustGetShadowStringFromBytes(result, "symbol")
	info, ok := rs.GetPairInfoBySymbol(symbol)
	if !ok {
		rs.Logger.Warnf("not found info for symbol. ", symbol)
	}
	order.Pair = info.Pair
	switch status {
	case "NEW":
		order.Type = helper.OrderEventTypeNEW
		rs.Cb.OnOrder(0, order)
		rs.ReqSucc(base.FailNumActionIdx_Place)
		return
	case "CANCELED", "EXPIRED", "FILLED":
		order.Type = helper.OrderEventTypeREMOVE
		order.Filled = fixed.NewF(helper.MustGetFloat64FromBytes(result, "executedQty"))
		if order.Filled.GreaterThan(fixed.ZERO) {
			order.FilledPrice = helper.MustGetFloat64FromBytes(result, "cumQuote") / order.Filled.Float()
		}
	default:
		rs.Logger.Error("Unknown rs's ws order status: ", status)
		return
	}

	switch helper.MustGetShadowStringFromBytes(result, "side") {
	case "BUY":
		order.OrderSide = helper.OrderSideKD
	case "SELL":
		order.OrderSide = helper.OrderSideKK
	default:
		rs.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(result, "side"))
		return
	}
	if helper.MustGetBool(result, "reduceOnly") {
		if order.OrderSide == helper.OrderSideKD {
			order.OrderSide = helper.OrderSidePK
		} else if order.OrderSide == helper.OrderSideKK {
			order.OrderSide = helper.OrderSidePD
		}
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
func (rs *BinanceUsdtSwapRs) CancelAllOpenOrders() {
	rs.cancelAllOpenOrders(false)
}

func (rs *BinanceUsdtSwapRs) DoGetOI(info *helper.ExchangeInfo) (oi float64, err helper.ApiError) {
	// 请求必备信息
	uri := "/fapi/v1/openInterest"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("failed to get depth. %v", handlerErr)
			return
		}
		var tick helper.Ticker
		tick, err = rs.GetTickerBySymbol(info.Symbol)
		if err.NotNil() {
			return
		}
		oi = helper.MustGetFloat64FromBytes(value, "openInterest") * tick.Price()
	})
	return
}
func (rs *BinanceUsdtSwapRs) DoGetDepth(info *helper.ExchangeInfo) (respDepth helper.Depth, err helper.ApiError) {
	// 请求必备信息
	uri := "/fapi/v1/depth"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["limit"] = 10
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr := p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("failed to get depth. %v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		asks := helper.MustGetArray(value, "asks")
		bids := helper.MustGetArray(value, "bids")
		if len(asks) > 0 && len(bids) > 0 {
			for _, bid := range bids {
				_bidPrice := helper.MustGetFloat64FromBytes(bid, "0")
				_bidQty := helper.MustGetFloat64FromBytes(bid, "1")
				respDepth.Bids = append(respDepth.Bids,
					helper.DepthItem{Price: _bidPrice, Amount: _bidQty})
			}
			for _, ask := range asks {
				_askPrice := helper.MustGetFloat64FromBytes(ask, "0")
				_askQty := helper.MustGetFloat64FromBytes(ask, "1")
				respDepth.Asks = append(respDepth.Asks,
					helper.DepthItem{Price: _askPrice, Amount: _askQty})
			}
		}
	})
	// if helper.DEBUGMODE {
	// 	rs.Logger.Info("depth", respDepth.String())
	// }
	return
}

func (rs *BinanceUsdtSwapRs) GetPriWs() *ws.WS {
	return rs.priWs
}
func (rs *BinanceUsdtSwapRs) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	return rs.DoGetPendingOrders("")
}

// symbol 空表示获取全部
func (rs *BinanceUsdtSwapRs) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	url := "/fapi/v1/openOrders"
	p := handyPool.Get()
	var value *fastjson.Value

	params := make(map[string]interface{})
	if symbol != "" {
		params["symbol"] = symbol
	}

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("parse response error: %s", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = errors.New(string(respBody))
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
			order.FilledPrice = helper.MustGetFloat64FromBytes(data, "avgPrice")

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
	})
	if err.NotNil() {
		rs.Logger.Errorf("failed to get pending orders error: %s", err.Error())
	}
	return
}

func (rs *BinanceUsdtSwapRs) SetCollateral(coin string) (res bool) {
	// 获取账户资产
	var url string
	var value *fastjson.Value
	var params map[string]interface{}

	url = "/fapi/v1/multiAssetsMargin"
	p := handyPool.Get()
	defer handyPool.Put(p)

	params = make(map[string]interface{})
	params["multiAssetsMargin"] = true
	var err helper.ApiError

	err.NetworkError = rs.call(http.MethodPost, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getAcctInfo parse error: %s", err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			if strings.Contains(helper.BytesToString(respBody), "repeat") {
				res = true
			}
			return
		}
		res = true
	})
	if !err.Nil() {
		//得检查是否有限频提示
		return
	}
	return
}

func (rs *BinanceUsdtSwapRs) GetMMR() (res helper.UMAcctInfo) {
	// 获取账户资产
	var url string
	var value *fastjson.Value
	var params map[string]interface{}

	url = "/fapi/v3/account"
	p := handyPool.Get()
	defer handyPool.Put(p)

	params = make(map[string]interface{})
	var err helper.ApiError

	err.NetworkError = rs.call(http.MethodGet, url, params, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("getAcctInfo parse error: %s", err.HandlerError.Error())
			return
		}

		if !rs.isOkApiResponse(value, url, params) {
			return
		}

		// {
		// 	"totalInitialMargin": "0.00000000",            // the sum of USD value of all cross positions/open order initial margin
		// 	"totalMaintMargin": "0.00000000",              // the sum of USD value of all cross positions maintenance margin
		// 	"totalWalletBalance": "126.72469206",          // total wallet balance in USD
		// 	"totalUnrealizedProfit": "0.00000000",         // total unrealized profit in USD
		// 	"totalMarginBalance": "126.72469206",          // total margin balance in USD
		// 	"totalPositionInitialMargin": "0.00000000",    // the sum of USD value of all cross positions initial margin
		// 	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price in USD
		// 	"totalCrossWalletBalance": "126.72469206",     // crossed wallet balance in USD
		// 	"totalCrossUnPnl": "0.00000000",               // unrealized profit of crossed positions in USD
		// 	"availableBalance": "126.72469206",            // available balance in USD
		// 	"maxWithdrawAmount": "126.72469206"            // maximum virtual amount for transfer out in USD

		res.TotalEquity = helper.MustGetFloat64FromBytes(value, "totalWalletBalance")
		res.DiscountedTotalEquity = helper.MustGetFloat64FromBytes(value, "totalMarginBalance")
		res.TotalMaintenanceMargin = helper.MustGetFloat64FromBytes(value, "totalMaintMargin")
		if res.DiscountedTotalEquity == 0 {
			res.MaintenanceMarginRate = 10000
		} else {
			res.MaintenanceMarginRate = res.TotalMaintenanceMargin / res.DiscountedTotalEquity
		}
		res.TsS = time.Now().Unix()
	})

	if !err.Nil() {
		//得检查是否有限频提示
		return
	}
	return
}
func (rs *BinanceUsdtSwapRs) PlaceOrderSpotUsdtSize(pair helper.Pair, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64) {
}
func (rs *BinanceUsdtSwapRs) CancelSpotPendingOrder(pair helper.Pair) (err helper.ApiError) {
	return
}
func (rs *BinanceUsdtSwapRs) Spot7DayTradeHist() (resp []helper.DealForList, err helper.ApiError) {
	return
}

// func (rs *BinanceUsdtSwapRs) GetPosHist(startTimeMs int64, endTimeMs int64) (resp []helper.Positionhistory, err helper.ApiError) {
// 	res, err := rs.DoGetPosHist(startTimeMs, endTimeMs)
// 	resp = res.Pos
// 	return
// }

// func (rs *BinanceUsdtSwapRs) DoGetPosHist(startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
// 	uri := "/futures/data/openInterestHist"
// 	params := make(map[string]interface{})
// 	// 当symbol不为空时，映射到token参数
// 	params["symbol"] = rs.Symbol
// 	diff := endTimeMs - startTimeMs
// 	durationMin := math.Ceil(float64(diff) / 60000.0)
// 	var period string
// 	switch {
// 	case durationMin <= 5:
// 		period = "5m"
// 	case durationMin <= 15:
// 		period = "15m"
// 	case durationMin <= 30:
// 		period = "30m"
// 	case durationMin <= 60:
// 		period = "1h"
// 	case durationMin <= 120:
// 		period = "2h"
// 	case durationMin <= 240:
// 		period = "4h"
// 	case durationMin <= 360:
// 		period = "6h"
// 	case durationMin <= 720:
// 		period = "12h"
// 	default:
// 		period = "1d"
// 	}
// 	params["period"] = period
// 	params["startTime"] = startTimeMs
// 	params["endTime"] = endTimeMs

// 	err.NetworkError = rs.call(http.MethodGet, uri, params, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
// 		p := handyPool.Get()
// 		defer handyPool.Put(p)
// 		var value *fastjson.Value
// 		value, err.HandlerError = p.ParseBytes(respBody)
// 		if err.HandlerError != nil {
// 			rs.Logger.Errorf("handler error %v", err)
// 			return
// 		}
// 		// 检查响应码是否成功，0表示成功
// 		if value.GetInt("code") != 0 {
// 			err.HandlerError = fmt.Errorf("handler error code %v, msg: %s", value.GetInt("code"), string(value.GetStringBytes("msg")))
// 			return
// 		}
// 		// 解析 data 对象
// 		data := value.GetArray()
// 		resp.Pos = make([]helper.Positionhistory, 0)
// 		for _, v := range data {
// 			openAmount := helper.MustGetFloat64(v, "sumOpenInterest")
// 			posValue := helper.MustGetFloat64(v, "sumOpenInterestValue")
// 			closeAvgPrice := posValue / openAmount
// 			// 解析时间
// 			openTime := helper.MustGetInt64(v, "timestamp")

// 			Symbol := helper.GetStringFromBytes(v, "symbol")
// 			info, ok := rs.ExchangeInfoPtrS2P.Get(Symbol)
// 			if !ok {
// 				rs.Logger.Errorf("not found pair for symbol %s", rs.Symbol)
// 				continue
// 			}
// 			deal := helper.Positionhistory{
// 				Symbol:        Symbol,
// 				Pair:          info.Pair.String(),
// 				OpenTime:      openTime,
// 				CloseTime:     0, // 无相关数据
// 				OpenPrice:     0, // 无相关数据
// 				CloseAvePrice: closeAvgPrice,
// 				OpenedAmount:  openAmount,
// 				ClosedAmount:  0, // 无相关数据
// 				PnlAfterFees:  0, // 无相关数据
// 				Fee:           0, // 无相关数据
// 			}
// 			resp.Pos = append(resp.Pos, deal)
// 		}
// 	})
// 	return
// }
