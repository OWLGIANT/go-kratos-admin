package bitget_usdt_swap

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"actor/helper/transfer"
	"actor/tools"
	"github.com/duke-git/lancet/v2/slice"
	jsoniter "github.com/json-iterator/go"

	"actor/broker/base"
	"actor/broker/brokerconfig"
	"actor/broker/client/rest"
	"actor/helper"
	"actor/third/fixed"
	"actor/third/log"

	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

var (
	BaseUrl = "https://api.bitget.com"
)

var (
	handyPool fastjson.ParserPool
)

// 用于批量下单的订单信息结构体
type oneOrderData struct {
	side       helper.OrderSide
	orderType  helper.OrderType
	price      float64
	amount     fixed.Fixed
	cid        string
	time       int64
	symbol     string
	reduceonly bool
}

func (o *oneOrderData) String() string {
	return fmt.Sprintf("cid:%v side:%v p:%v a:%v type:%v", o.cid, o.side, o.price, o.amount, o.orderType)
}

type BitgetUsdtSwapRs struct {
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyGetPriWs
	base.DummyDoGetPriceLimit
	base.DummyDoSetLeverage
	base.DummyDoSetPositionMode
	client                      *rest.Client   // 通用rest客户端
	failNum                     atomic.Int64   // 出错次数
	takerFee                    atomic.Float64 // taker费率
	makerFee                    atomic.Float64 // maker费率
	base.DummyDoAmendOrderRsNor                //只支持双向持仓模式的时候 "修改 size 和 price 只允许双向持仓时使用，单项持仓不允许使用"
	base.DummyOrderActionRsColo
	base.DummyOrderActionWsColo
	base.DummyOrderActionWsNor
	isInSpec bool
}

// 创建新的实例
func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}

	rs := &BitgetUsdtSwapRs{
		client: rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
	}
	rs.client.SetUserAgentSlice(helper.UserAgentSlice)
	rs.isInSpec = params.IgnoreDuplicateClientOidError
	// isUsingColo := false
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BitgetUsdtSwapRestUrl != "" {
		// isUsingColo = true
		BaseUrl = cfg.BitgetUsdtSwapRestUrl
		params.Logger.Infof("bitget_usdt_swap rs 启用colo  %v", BaseUrl)
	}

	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, pairInfo, cb)
	//Delay monitor
	if params.ActivateDelayMonitor {
		influxCfg := base.DefaultInfluxConfig(params)
		base.InitDelayMonitor(&rs.DelayMonitor, &rs.FatherRs, influxCfg, rs.GetExName(), rs.Pair.String(), rs.MonitorTrigger)
	}

	// only one rs, colo not controlled by select line
	rs.SetSelectedLine(base.LinkType_Nor, base.ClientType_Rs)
	rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Cross})
	rs.DelayMonitor.AddLine(base.Line{Link: base.LinkType_Nor, Client: base.ClientType_Rs, MarginMode: helper.MarginMode_Iso})
	if !rs.isInSpec {
		rs.ChoiceBestLine()
	}
	return rs
}

func (rs *BitgetUsdtSwapRs) DoPlaceOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.placeOrder(info, helper.FixPrice(s.Price, info.TickSize), s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false, s.ForceReduceOnly)
}

func (rs *BitgetUsdtSwapRs) DoCancelOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	rs.cancelOrderID(info, s.ClientID, s.OrderID, s.Time)
}

func (rs *BitgetUsdtSwapRs) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
	code := helper.BytesToString(value.GetStringBytes("code"))
	if code == "00000" {
		rs.ReqSucc(base.FailNumActionIdx_AllReq)
		return true
	} else {
		if code == "22001" { // "code":"22001","msg":"No order to cancel"
			rs.ReqSucc(base.FailNumActionIdx_AllReq)
			return true
		}
		// https://www.bitget.com/api-doc/contract/intro
		// 40015	System is abnormal, please try again later
		// 50031	System error
		// 80002	system error
		if code == "40015" || code == "50031" || code == "80002" {
			rs.Cb.OnExchangeDown()
		}
		rs.Logger.Errorf("请求失败 req: %s %v. rsp: %s", url, params, value.String())
		rs.ReqFail(base.FailNumActionIdx_AllReq)
		return false
	}
}

func (rs *BitgetUsdtSwapRs) symbolToPair(symbol string) helper.Pair {
	if info, ok := rs.ExchangeInfoPtrS2P.Get(symbol); ok {
		return info.Pair
	}
	rs.Logger.Errorf("没找到 %v 的pair，需要停机", symbol)
	rs.Cb.OnExit(fmt.Sprintf("没找到 %v 的pair，需要停机", symbol))
	return helper.Pair{}
}

func (rs *BitgetUsdtSwapRs) getExchangeInfo() []helper.ExchangeInfo {
	fileName := base.GenExchangeInfoFileName("")
	if pairInfo, infos, ok := helper.TryGetExchangeInfosPtrFromFile(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}
	// 请求必备信息
	uri := "/api/mix/v1/market/contracts"
	params := make(map[string]interface{})
	params["productType"] = "umcbl"

	// 待使用数据结构
	infos := make([]helper.ExchangeInfo, 0)
	// 发起请求
	err := rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		var value ExchangeInfo
		handlerErr = jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, handlerErr.Error()))
			return
		}
		if value.Code != "00000" {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, helper.BytesToString(respBody)))
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)

		datas := value.Data
		// 该接口没提供最大持仓限制信息 kc@2023-12-27
		for _, data := range datas {
			baseCoin := strings.ToLower(data.BaseCoinDisplayName)
			baseCoin = helper.Trim10Multiple(baseCoin)
			quoteCoin := strings.ToLower(data.QuoteCoin)
			pricePlace := helper.MustGetIntFromString(data.PricePlace)
			priceEndStep := helper.MustGetIntFromString(data.PriceEndStep)
			// volumePlace := helper.MustGetIntFromString(data.VolumePlace)
			minTradeNum := fixed.NewS(data.MinTradeNum)
			symbol := data.Symbol
			// todo 这里可以获取到限价规则 按百分比上下浮动 还没想好怎么处理
			info := helper.ExchangeInfo{
				Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
				Symbol:         symbol,
				Status:         true, // 没有给出，直接true
				TickSize:       float64(priceEndStep) / math.Pow10(pricePlace),
				StepSize:       minTradeNum.Float(), // bg 会floor 我们的量到 minorderamount
				MaxOrderAmount: fixed.BIG,
				MinOrderAmount: minTradeNum,
				MaxOrderValue:  fixed.NewF(200000), // 限制bitget单手最大下单量 因为交易所有限制
				MinOrderValue:  fixed.TEN,
				Multi:          fixed.ONE,
				// 最大持仓价值
				MaxPosValue: fixed.BIG,
				// 最大持仓数量
				MaxPosAmount: fixed.BIG,
				// MaxLeverage:  helper.MAX_LEVERAGE,
				MaxLeverage: 10,                                                                                                             // 这个所特殊，默认最大拉到10倍
				Extra:       strings.Replace(strings.Replace(strings.Replace(symbol, "_UMCBL", "", 1), "$", "", -1), "NEWUSDT", "USDT", -1), //V2 symbol
			}

			if info.Pair.Base == "major" {
				info.Extra = "MAJORUSDT"
			} else if info.Pair.Base == "kaia" {
				info.Extra = "KAIAUSDT"
			} else if info.Pair.Base == "taiko" {
				info.Extra = "TAIKOUSDT"
			} else if info.Pair.Base == "zk" {
				info.Extra = "ZKUSDT"
			} else if info.Pair.Base == "neiroeth" {
				info.Extra = "NEIROETHUSDT"
			} else if info.Pair.Base == "velo" {
				info.Extra = "VELOUSDT"
			} else if info.Pair.Base == "the" {
				info.Extra = "THEUSDT"
			} else if info.Pair.Base == "pumpbtc" {
				info.Extra = "PUMPBTCUSDT"
			} else if info.Pair.Base == "luna" {
				info.Extra = "LUNAUSDT"
			}

			if info.MaxPosAmount == fixed.NaN || info.MaxPosAmount.IsZero() {
				info.MaxPosAmount = fixed.BIG
			}
			if ok, err := info.CheckReady(); !ok {
				rs.Logger.Warnf("exchangeinfo not ready. %s", err)
				continue
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

	if info, ok := rs.ExchangeInfoPtrP2S.Get(rs.Pair.String()); ok {
		helper.CopySymbolInfo(rs.PairInfo, info)
		rs.Symbol = info.Symbol
	} else {
		if info, ok := rs.TryFetchFromRedis(rs.Pair.String()); !ok {
			rs.Logger.Errorf("交易对信息 %s %s 不存在", rs.Pair, rs.ExchangeName)
			rs.Cb.OnExit(fmt.Sprintf("交易对信息 %s %s 不存在, 需要停机", rs.Pair, rs.ExchangeName))
			return nil
		} else {
			infos = append(infos, info)
		}
	}
	// 写入文件
	rs.CheckAndSaveExInfo(fileName, infos)
	return infos
}

// getIndex 获取Index 函数本身不需要返回值 通过callbackFunc传递出去
func (rs *BitgetUsdtSwapRs) GetIndex() {
	uri := "/api/mix/v1/market/index"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	//params["productType"] = "USDT-FUTURES" // for vs endpoint only
	err := rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("handlerErr:%v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := value.Get("data")
		index := helper.MustGetFloat64FromBytes(data, "index")
		if rs.Cb.OnIndex != nil {
			rs.Cb.OnIndex(0, helper.IndexEvent{IndexPrice: index})
		}
	})
	if err != nil {
		rs.Logger.Errorf("连接 Ticker 获取Index信息失败")
	}

	uri = "/api/mix/v1/market/mark-price"
	err = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("handlerErr:%v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := value.Get("data")
		mark := helper.MustGetFloat64FromBytes(data, "markPrice")
		if rs.Cb.OnMark != nil {
			rs.Cb.OnMark(0, helper.MarkEvent{MarkPrice: mark})
		}
	})
	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("连接 Ticker 获取Mark信息失败")
	}
}
func (rs *BitgetUsdtSwapRs) GetIndexs(pairs []string) (res map[string]float64, err helper.ApiError) {
	uri := "/api/mix/v1/market/tickers"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	params := make(map[string]interface{})
	params["productType"] = "umcbl "

	err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handlerErr:%v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}

		datas := helper.MustGetArray(value, "data")
		if len(pairs) == 0 {
			res = make(map[string]float64, (rs.ExchangeInfoPtrS2P.Count()))
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
	if err.HandlerError != nil || err.NetworkError != nil {
		rs.Logger.Errorf("连接 Ticker 获取Index信息失败")
	}
	if helper.DEBUGMODE {
		rs.Logger.Info("Indexs", res)
	}
	return
}

func (rs *BitgetUsdtSwapRs) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	// 请求必备信息
	uri := "/api/mix/v1/market/depth"
	params := make(map[string]interface{})
	params["symbol"] = symbol
	params["limit"] = 5
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取tick失败, %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := value.Get("data")
		asks := data.GetArray("asks")
		bids := data.GetArray("bids")
		tsMsInEx := helper.MustGetInt64(data, "timestamp")

		if len(asks) == 0 || len(bids) == 0 {
			return
		}
		ask, _ := asks[0].Array()
		bid, _ := bids[0].Array()
		ap := helper.MustGetFloat64FromBytes(ask[0])
		aq := helper.MustGetFloat64FromBytes(ask[1])
		bp := helper.MustGetFloat64FromBytes(bid[0])
		bq := helper.MustGetFloat64FromBytes(bid[1])

		if symbol == rs.Symbol && rs.TradeMsg.Ticker.Seq.NewerAndStore(tsMsInEx, 0) {
			rs.TradeMsg.Ticker.Set(ap, aq, bp, bq)
			rs.Cb.OnTicker(0)
		}
		ticker.Set(ap, aq, bp, bq)
		ticker.Seq.Ex.Store(tsMsInEx)
	})
	// if err != nil {
	//得检查是否有限频提示
	// }
	return
}

// GetEquity 获取账户资金 函数本身无返回值 仅更新本地资产并通过callbackfunc传递出去
func (rs *BitgetUsdtSwapRs) GetEquity() (resp helper.Equity, err helper.ApiError) { //获取资产
	uri := "/api/mix/v1/account/account"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	params["marginCoin"] = "USDT"

	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handlerErr:%v", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}

		data := value.Get("data")
		// upnl := 0.0
		// if data.Get("unrealizedPL").Type() != fastjson.TypeNull {
		// upnl = helper.MustGetFloat64FromBytes(data, "unrealizedPL")
		// }
		total := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("equity"))) // 按文档包含 upnl
		avail := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("maxTransferOut")))
		marginCoin := helper.MustGetStringLowerFromBytes(data, "marginCoin")
		// {"code":"00000","msg":"success","requestTime":1693558702289,"data":{"marginCoin":"USDT","locked":"14.62226798","available":"286.27224854","crossMaxAvailable":"271.64998055","fixedMaxAvailable":"271.64998055","maxTransferOut":"271.64998055","equity":"286.27224854","usdtEquity":"286.272248543947","btcEquity":"0.011021727244","crossRiskRate":"0.015592426017","crossMarginLeverage":20,"fixedLongLeverage":20,"fixedShortLeverage":20,"marginMode":"crossed","holdMode":"double_hold","unrealizedPL":"0","bonus":"0"}}
		upl := helper.GetFloat64FromBytes(data, "unrealizedPL")

		ts := time.Now().UnixNano()
		if e, ok := rs.EquityNewerAndStore(marginCoin, 0, ts, (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)); ok {
			e.TotalWithUpl = total
			e.TotalWithoutUpl = total - upl
			e.Avail = avail
			e.Upl = upl

			rs.Cb.OnEquityEvent(0, *e)
		}

		resp = helper.Equity{
			Cash:     total,
			CashFree: avail,
			CashUpl:  upl,
		}
		resp.IsSet = true
	})
	if !err.Nil() {
		//得检查是否有限频提示
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}

	base.EnsureIsRsExposerOuter(rs)
	return
}

func (rs *BitgetUsdtSwapRs) getLeverage() {
	// 请求必备信息
	url := "/api/mix/v1/market/symbol-leverage"
	// 请求必备变量
	p := handyPool.Get()
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	defer handyPool.Put(p)
	var err helper.ApiError
	err.NetworkError = rs.call(http.MethodGet, url, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if msg := helper.BytesToString(value.GetStringBytes("msg")); msg != "success" {
			err.HandlerError = errors.New(helper.BytesToString(respBody))
			return
		}
		data := value.Get("data")
		availableLevelRate := helper.MustGetIntFromBytes(data, "maxLeverage")
		if rs.PairInfo.MaxLeverage > availableLevelRate {
			rs.PairInfo.MaxLeverage = availableLevelRate
		}
	})

	return
}

func (rs *BitgetUsdtSwapRs) DoGetAccountMode(pairInfo *helper.ExchangeInfo) (leverage int, marginMode helper.MarginMode, posMode helper.PosMode, err helper.ApiError) {
	url := "/api/mix/v1/account/account"
	params := map[string]interface{}{
		"symbol":     pairInfo.Symbol,
		"marginCoin": "USDT",
	}

	// 调用 API
	err.NetworkError = rs.call(http.MethodGet, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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

		// 检查响应码
		code := helper.MustGetStringFromBytes(value, "code")
		if code != "00000" {
			return
		}

		// 解析数据部分
		data := value.Get("data")

		// 确定保证金模式
		marginModeStr := helper.MustGetStringFromBytes(data, "marginMode")
		switch marginModeStr {
		case "crossed":
			marginMode = helper.MarginMode_Cross
		case "fixed":
			marginMode = helper.MarginMode_Iso
		default:
			rs.Logger.Errorf("unknown margin mode: %s", marginModeStr)
			return
		}

		// 确定持仓模式
		holdMode := string(data.GetStringBytes("holdMode"))
		switch holdMode {
		case "single_hold":
			posMode = helper.PosModeOneway
		case "double_hold":
			posMode = helper.PosModeHedge
		default:
			rs.Logger.Errorf("unknown hold mode: %s", holdMode)
			return
		}

		// 确定杠杆率
		if marginMode == helper.MarginMode_Cross {
			leverage = int(helper.MustGetFloat64(data, "crossMarginLeverage"))
		} else if marginMode == helper.MarginMode_Iso {
			longLeverage := int(helper.MustGetFloat64(data, "fixedLongLeverage"))
			shortLeverage := int(helper.MustGetFloat64(data, "fixedShortLeverage"))
			if posMode == helper.PosModeHedge && longLeverage != shortLeverage {
				rs.Logger.Errorf("mismatched leverage in hedge mode: long=%d, short=%d", longLeverage, shortLeverage)
				return
			}
			leverage = longLeverage // 如果一致，则返回一致的杠杆率
		}
	})

	// 处理 API 调用错误
	if !err.Nil() {
		rs.Logger.Errorf("failed to get account mode: %v", err)
	}

	return
}

func (rs *BitgetUsdtSwapRs) DoSetAccountMode(pairinfo *helper.ExchangeInfo, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) (lastError helper.ApiError) {
	if err := rs.DoSetMarginMode(pairinfo.Symbol, marginMode); err.NotNil() {
		lastError = err
		return
	}

	// 设置持仓模式
	if err := rs.setPositionMode(pairinfo.Symbol, positionMode); err.NotNil() {
		lastError = err
		return
	}

	if err := rs.setLeverage(*rs.PairInfo, leverage); err.NotNil() {
		lastError = err
		return
	}

	if marginMode == helper.MarginMode_Iso {
		if err := rs.SetAutoMargin(*rs.PairInfo, helper.PosSideShort); err.NotNil() {
			lastError = err
			return
		}
		if err := rs.SetAutoMargin(*rs.PairInfo, helper.PosSideLong); err.NotNil() {
			lastError = err
			return
		}
	}
	return
}

// setLeverRate 设置账户的杠杆参数 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BitgetUsdtSwapRs) setLeverage(pairInfo helper.ExchangeInfo, leverage int) (err helper.ApiError) {
	// 基本请求信息
	uri := "/api/mix/v1/account/setLeverage"

	params := make(map[string]interface{})

	params["symbol"] = pairInfo.Symbol
	params["marginCoin"] = "USDT"
	params["leverage"] = fmt.Sprintf("%d", leverage)

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("setLeverRate error %v", err.HandlerError)
			return
		}

		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
	})
	if err.NotNil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("setLeverRate error %v", err)
		return err
	}

	{
		// 获取杠杆档位梯度配置
		uri = "/api/v2/mix/market/query-position-lever"
		params = make(map[string]interface{})
		params["symbol"] = pairInfo.Extra
		params["productType"] = "USDT-FUTURES"
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value

		err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				rs.Logger.Errorf("setLeverRate error %v", err.HandlerError)
				return
			}
			if !rs.isOkApiResponse(value, uri, params) {
				err.HandlerError = errors.New(string(respBody))
				return
			}
			type Item struct {
				Lever int
				Max   float64
			}
			items := slice.Map(helper.MustGetArray(value, "data"), func(idx int, v *fastjson.Value) Item {
				return Item{
					Lever: helper.MustGetIntFromBytes(v, "leverage"),
					Max:   helper.MustGetFloat64FromBytes(v, "endUnit"),
				}
			})
			sort.Slice(items, func(i, j int) bool {
				return items[i].Lever < items[j].Lever
			})
			foundItem := Item{}
			for _, item := range items {
				if leverage <= item.Lever {
					foundItem = item
					break
				}
			}
			if foundItem.Lever == 0 {
				err.HandlerError = fmt.Errorf("杠杆档位配置错误. 太高，没有适配限额. 杠杆:%d", leverage)
				return
			}
			rs.Logger.Infof("杠杆档位配置, setted lev %d, %+v", leverage, foundItem)
			rs.ExchangeInfoLabilePtrP2S.Set(pairInfo.Pair.String(), &helper.LabileExchangeInfo{
				Pair:           pairInfo.Pair,
				Symbol:         pairInfo.Symbol,
				SettedLeverage: leverage,
				RiskLimit: helper.RiskLimit{
					Underlying: "USDT",
					Amount:     foundItem.Max,
				},
			})
		})
		if err.NotNil() {
			//得检查是否有限频提示
			rs.Logger.Errorf("setLeverRate error %v", err)
			return err
		}
	}

	return
}

// DoSetMarginMode 设置账户的保证金模式 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BitgetUsdtSwapRs) DoSetMarginMode(symbol string, marginMode helper.MarginMode) (err helper.ApiError) {
	// 基本请求信息
	uri := "/api/mix/v1/account/setMarginMode"
	var marginType string
	if marginMode == helper.MarginMode_Cross {
		marginType = "crossed"
	} else if marginMode == helper.MarginMode_Iso {
		marginType = "fixed"
	}

	params := make(map[string]interface{})

	params["symbol"] = symbol
	params["marginCoin"] = "USDT"
	params["marginMode"] = marginType

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("%s", err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
	})
	if !err.Nil() {
		//得检查是否有限频提示
	}
	return
}

// setPositionMode 设置账户的持仓模式 不需要返回值 改成单向持仓 全仓 20x杠杆
func (rs *BitgetUsdtSwapRs) setPositionMode(symbol string, pm helper.PosMode) (err helper.ApiError) {
	// 基本请求信息
	uri := "/api/mix/v1/account/setPositionMode"
	var posMode string
	if pm == helper.PosModeOneway {
		posMode = "single_hold"
	} else if pm == helper.PosModeHedge {
		posMode = "double_hold"
	}

	params := make(map[string]interface{})

	params["productType"] = "umcbl"
	params["holdMode"] = posMode
	//params["holdMode"] = "double_hold"

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("%s", err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}

	})
	if !err.Nil() {
		//得检查是否有限频提示
		rs.Logger.Errorf("%v", err)
	}
	return
}

// 撤掉当前交易对挂单
// https://bitgetlimited.github.io/apidoc/zh/mix/#cee6a1fd93
func (rs *BitgetUsdtSwapRs) DoCancelPendingOrders(symbol string) (err helper.ApiError) {
	uri := "/api/mix/v1/order/current"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["symbol"] = symbol

	info, ok := rs.GetPairInfoBySymbol(symbol)
	if !ok {
		rs.Logger.Errorf("fail to cancel pending orders")
		return
	}

	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("parse DoCancelPendingOrders error:%v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		datas := value.GetArray("data")
		for _, v := range datas {
			oid := string(v.GetStringBytes("orderId"))
			rs.cancelOrderID(info, "", oid, 0)
		}
	})
	if err.Nil() {
		//得检查是否有限频提示
	}
	return
}

func (rs *BitgetUsdtSwapRs) doCancelSymbolOrders(symbol string) (has bool) {
	has = true
	uri := "/api/mix/v1/order/current"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["symbol"] = symbol

	info, ok := rs.GetPairInfoBySymbol(symbol)
	if !ok {
		rs.Logger.Errorf("fail to cancel pending orders")
		return
	}
	var err helper.ApiError

	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("parse DoCancelPendingOrders error:%v", handlerErr)
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		datas := value.GetArray("data")
		has = len(datas) > 0
		for _, v := range datas {
			oid := string(v.GetStringBytes("orderId"))
			rs.cancelOrderID(info, "", oid, 0)
		}
	})
	if err.Nil() {
		//得检查是否有限频提示
	}
	return
}
func (rs *BitgetUsdtSwapRs) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/api/mix/v1/order/cancel-all-orders"
	params := make(map[string]interface{})
	params["productType"] = "umcbl"
	params["marginCoin"] = "USDT"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value
	if only {
		return rs.doCancelSymbolOrders(rs.Symbol)
	}

	realFailed := true

	err = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("撤销所有挂单失败: %s", handlerErr.Error())
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code == "22001" { // statuscode=400, "code":"22001","msg":"No order to cancel"
			hasPendingOrderBefore = false
			realFailed = false
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		realFailed = false
	})
	if err != nil {
		//得检查是否有限频提示
		if realFailed { // statuscode=400, "code":"22001","msg":"No order to cancel"
			rs.Logger.Errorf("撤销所有挂单失败: %s", err.Error())
		}
	}
	return
}

// 撤掉所有挂单
func (rs *BitgetUsdtSwapRs) cancelAllOpenOrders() {
	uri := "/api/mix/v1/order/cancel-all-orders"

	params := make(map[string]interface{})

	params["productType"] = "umcbl"
	params["marginCoin"] = "USDT"

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	realFailed := true

	err = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("撤销所有挂单失败: %s", handlerErr.Error())
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code == "22001" { // statuscode=400, "code":"22001","msg":"No order to cancel"
			realFailed = false
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		realFailed = false
	})
	if err != nil {
		//得检查是否有限频提示
		if realFailed { // statuscode=400, "code":"22001","msg":"No order to cancel"
			rs.Logger.Errorf("撤销所有挂单失败: %s", err.Error())
		}
	}
}

// deprecated 清空所有持仓 返回是否有遗漏仓位bool
// func (rs *BitgetUsdtSwapRs) cleanPosition(only bool) bool {

// adjustAcct 开始交易前 把账户调整到合适的状态 包括调整杠杆 仓位模式 买入一定数量的平台币等等
func (rs *BitgetUsdtSwapRs) adjustAcct() {
	if !rs.BrokerConfig.InitialAcctConfig.IsEmpty() {
		rs.SetAccountMode(rs.Pair.String(), rs.BrokerConfig.InitialAcctConfig.MaxLeverage, rs.BrokerConfig.InitialAcctConfig.MarginMode, rs.BrokerConfig.InitialAcctConfig.PosMode)
		return
	}
	rs.getLeverage()
	rs.DoSetAccountMode(rs.PairInfo, rs.PairInfo.MaxLeverage, helper.MarginMode_Iso, helper.PosModeOneway)
}

// BeforeTrade 开始交易前需要做的所有工作 调整好杠杆
func (rs *BitgetUsdtSwapRs) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	rs.getExchangeInfo() // 必须先获取交易规则
	rs.UpdateExchangeInfo(rs.ExchangeInfoPtrP2S, rs.ExchangeInfoPtrS2P, rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	switch mode {
	case helper.HandleModePublic:
		rs.GetTickerBySymbol(rs.Symbol)
		return
	case helper.HandleModePrepare:
		rs.GetPosition()
	case helper.HandleModeCloseOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		leakedPrev = rs.HasPosition(rs, true)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders()
		leakedPrev = rs.HasPosition(rs, false)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.DoCancelPendingOrders(rs.Symbol)
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders()
	}
	// 调整账户
	rs.adjustAcct()
	// 获取账户资金
	rs.GetEquity()
	// 获取ticker
	rs.GetTickerBySymbol(rs.Symbol)
	rs.PrintAcctSumWhenBeforeTrade(rs)

	rs.DelayMonitor.Run()
	return
}
func (rs *BitgetUsdtSwapRs) DoStop() {}

// AfterTrade 结束交易时需要做的所有工作  清空挂单和仓位
// 如果有遗漏仓位 返回true  如果清仓干净了 返回false
func (rs *BitgetUsdtSwapRs) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	switch mode {
	case helper.HandleModePrepare:
		isLeft = false
	case helper.HandleModeCloseOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
		time.Sleep(time.Second)
		rs.GetEquity()
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders()
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
		time.Sleep(time.Second)
		rs.GetEquity()
	case helper.HandleModeCancelOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		isLeft = false
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders()
		isLeft = false
	}
	return
}

// GetPosition 获取仓位 获取之后触发回调
func (rs *BitgetUsdtSwapRs) GetPosition() (resp []helper.PositionSum, err helper.ApiError) {
	return rs.GetOrigPositions()
}

// 限价单价格超价于bbo时会失败
/**
委托价格高于最高买价	40815	400
委托价格低于于最低卖价	40816	400
*/
func (rs *BitgetUsdtSwapRs) placeOrder(info *helper.ExchangeInfo, price fixed.Fixed, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, isHedgePosMode bool, forceReduce bool) {
	url := "/api/mix/v1/order/placeOrder"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["marginCoin"] = "USDT"
	params["size"] = size.String()
	//params["price"] = helper.FixPrice(price, b.PairInfo.TickSize).String()
	params["price"] = price.String()
	params["amount"] = size.String()

	if orderType == helper.OrderTypeIoc {
		params["orderType"] = "limit"
		params["timeInForceValue"] = "ioc"
	} else if orderType == helper.OrderTypePostOnly {
		params["orderType"] = "limit"
		params["timeInForceValue"] = "post_only"
	} else if orderType == helper.OrderTypeMarket {
		params["orderType"] = "market"
	} else if orderType == helper.OrderTypeLimit {
		params["orderType"] = "limit"
	} else {
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		order.Pair = info.Pair
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
		return
	}

	_side := ""
	if !isHedgePosMode {
		switch side {
		case helper.OrderSideKD:
			_side = "buy_single"
		case helper.OrderSidePK:
			_side = "buy_single"
			if forceReduce {
				params["reduceOnly"] = true
			}
		case helper.OrderSideKK:
			_side = "sell_single"
		case helper.OrderSidePD:
			_side = "sell_single"
			if forceReduce {
				params["reduceOnly"] = true
			}
		}
	} else {
		switch side {
		case helper.OrderSideKD:
			_side = "open_long"
		case helper.OrderSideKK:
			_side = "open_short"
		case helper.OrderSidePD:
			_side = "close_long"
		case helper.OrderSidePK:
			_side = "close_short"
		}
	}
	params["side"] = _side

	if cid != "" {
		params["clientOid"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()

	if helper.DEBUGMODE {
		rs.Logger.Debugf("bg rest下单 %v", params)
	}
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	ignore := false
	err = rs.call(http.MethodPost, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		end := time.Now().UnixMicro()
		handledOk := false
		defer func() {
			if !handledOk && !rs.DelayMonitor.IsMonitorOrder(cid) {
				order := helper.OrderEvent{Type: helper.OrderEventTypeERROR, ClientID: cid, Pair: info.Pair}
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
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				// 出现错误的时候也要触发回调, 不是json格式
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				order.Pair = info.Pair
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
				return
			}
			rs.ReqFail(base.FailNumActionIdx_Place)
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				handlerErr = errors.New(helper.BytesToString(respBody))
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.Pair = info.Pair
				setOrderErrorType(code, &order)
				if (code == "43008" || code == "43009") && !base.IsInUPC {
					helper.LogErrorThenCall("下单失败限价触发43008", rs.Cb.OnExit)
				} else if code == "40786" && rs.BrokerConfig.IgnoreDuplicateClientOidError { // {"code":"40786","msg":"Duplicate clientOid","requestTime":1741747522148,"data":null}
					ignore = true
					handledOk = true
					return
				}
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, cid, handlerErr)
				rs.ReqFail(base.FailNumActionIdx_Place)
				return
			}
		}

		data := value.Get("data")
		oid := helper.MustGetStringFromBytes(data, "orderId")

		var order helper.OrderEvent
		order.Pair = info.Pair
		// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
		order.Type = helper.OrderEventTypeNEW
		order.OrderID = oid
		order.ClientID = string(data.GetStringBytes("clientOid"))
		//
		handledOk = true

		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Place, Cid: cid, Oid: order.OrderID, DurationUs: end - start}
		if ok := rs.DelayMonitor.TryNext(rsp); ok {
			return
		}

		rs.Cb.OnOrder(0, order)
		if rs.isInSpec {
			log.Infof("v1rs got-new-order %d %s", t, cid)
		}
		rs.ReqSucc(base.FailNumActionIdx_Place)
	})

	if !ignore && err != nil && !rs.DelayMonitor.IsMonitorOrder(cid) {
		//得检查是否有限频提示
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		order.Pair = info.Pair
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[utf_ign] [%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

func setOrderErrorType(code string, order *helper.OrderEvent) {
	if code == "45119" || code == "22029" || code == "40715" {
		// code\":40715,\"msg\":\"delegate count can not high max of open count\"
		// {\"code\":\"22029\",\"msg\":\"Risk control, you can currently open a maximum position of 0 DOG. The risk con
		order.ErrorType = helper.OrderErrorTypeNotAllowOpen
	} else if code == "22057" {
		// {\"code\":\"22057\",\"msg\":\"Market makers must be in one-way position holding mode and cannot place orders that only reduce positions\",\"requestTime\":1730288366718,\"data\":null}"}
		order.ErrorType = helper.OrderErrorTypeNotAllowReduceOnly
	}
}

func (rs *BitgetUsdtSwapRs) placeOrderForUPC(info *helper.ExchangeInfo, price fixed.Fixed, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64, isHedgePosMode bool, forceReduce bool) (reduceRejected bool) {
	url := "/api/mix/v1/order/placeOrder"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["marginCoin"] = "USDT"
	params["size"] = size.String()
	//params["price"] = helper.FixPrice(price, b.PairInfo.TickSize).String()
	params["price"] = price.String()
	params["amount"] = size.String()

	if orderType == helper.OrderTypeIoc {
		params["orderType"] = "limit"
		params["timeInForceValue"] = "ioc"
	} else if orderType == helper.OrderTypePostOnly {
		params["orderType"] = "limit"
		params["timeInForceValue"] = "post_only"
	} else if orderType == helper.OrderTypeMarket {
		params["orderType"] = "market"
	} else if orderType == helper.OrderTypeLimit {
		params["orderType"] = "limit"
	} else {
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		order.Pair = info.Pair
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
		return
	}

	_side := ""
	if !isHedgePosMode {
		switch side {
		case helper.OrderSideKD:
			_side = "buy_single"
		case helper.OrderSidePK:
			_side = "buy_single"
			if forceReduce {
				params["reduceOnly"] = true
			}
		case helper.OrderSideKK:
			_side = "sell_single"
		case helper.OrderSidePD:
			_side = "sell_single"
			if forceReduce {
				params["reduceOnly"] = true
			}
		}
	} else {
		switch side {
		case helper.OrderSideKD:
			_side = "open_long"
		case helper.OrderSideKK:
			_side = "open_short"
		case helper.OrderSidePD:
			_side = "close_long"
		case helper.OrderSidePK:
			_side = "close_short"
		}
	}
	params["side"] = _side

	if cid != "" {
		params["clientOid"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()

	if helper.DEBUGMODE {
		rs.Logger.Debugf("bg rest下单 %v", params)
	}
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)

	err = rs.call(http.MethodPost, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				// 出现错误的时候也要触发回调, 不是json格式
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = cid
				order.Pair = info.Pair
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
				return
			}
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			if !rs.DelayMonitor.IsMonitorOrder(cid) {
				handlerErr = errors.New(helper.BytesToString(respBody))
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.Pair = info.Pair
				setOrderErrorType(code, &order)
				if (code == "43008" || code == "43009") && !base.IsInUPC {
					helper.LogErrorThenCall("下单失败限价触发43008", rs.Cb.OnExit)
				} else if code == "22057" {
					// {\"code\":\"22057\",\"msg\":\"Market makers must be in one-way position holding mode and cannot place orders that only reduce positions\",\"requestTime\":1730288366718,\"data\":null}"
					reduceRejected = true
				}
				order.ClientID = cid
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, cid, handlerErr)
				return
			}
		}

		data := value.Get("data")
		oid := helper.MustGetStringFromBytes(data, "orderId")

		var order helper.OrderEvent
		order.Pair = info.Pair
		// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
		order.Type = helper.OrderEventTypeNEW
		order.OrderID = oid
		order.ClientID = string(data.GetStringBytes("clientOid"))
		rs.Cb.OnOrder(0, order)
	})

	if err != nil && !rs.DelayMonitor.IsMonitorOrder(cid) {
		//得检查是否有限频提示
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		order.Pair = info.Pair
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
	return
}

// placeOrders 批量下单
func (rs *BitgetUsdtSwapRs) placeOrders(orderList []helper.Signal) {
	url := "/api/mix/v1/order/batch-orders"

	info, ok := rs.GetPairInfoByPair(&orderList[0].Pair)
	if !ok {
		return
	}
	params := make(map[string]interface{})
	params["marginCoin"] = "USDT"
	orderDatas := make([]map[string]interface{}, 0, len(orderList))
	params["symbol"] = info.Symbol

	for _, v := range orderList {
		_side := ""
		/*switch v.side {
		case helper.OrderSideKD:
			_side = "open_long"
		case helper.OrderSideKK:
			_side = "open_short"
		case helper.OrderSidePD:
			_side = "close_long"
		case helper.OrderSidePK:
			_side = "close_short"
		}*/
		switch v.OrderSide {
		case helper.OrderSideKD:
			_side = "buy_single"
		case helper.OrderSidePK:
			_side = "buy_single"
			// params["reduceOnly"] = false // bg 特殊，平仓量计算错误。看placeOrder调用处注释。upc不能也不会调用这里
		case helper.OrderSideKK:
			_side = "sell_single"
		case helper.OrderSidePD:
			_side = "sell_single"
			// params["reduceOnly"] = v.reduceonly
		}
		oneOrderData := make(map[string]interface{})
		oneOrderData["clientOid"] = v.ClientID
		oneOrderData["size"] = v.Amount.String()
		oneOrderData["side"] = _side
		oneOrderData["price"] = helper.FixPrice(v.Price, info.TickSize).String()
		if v.OrderType == helper.OrderTypeIoc {
			oneOrderData["orderType"] = "limit"
			oneOrderData["timeInForceValue"] = "ioc"
		} else if v.OrderType == helper.OrderTypePostOnly {
			oneOrderData["orderType"] = "limit"
			oneOrderData["timeInForceValue"] = "post_only"
		} else if v.OrderType == helper.OrderTypeMarket {
			oneOrderData["orderType"] = "market"
		} else if v.OrderType == helper.OrderTypeLimit {
			oneOrderData["orderType"] = "limit"
		} else {
			continue
		}
		orderDatas = append(orderDatas, oneOrderData)
	}
	params["orderDataList"] = orderDatas
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()

	if helper.DEBUGMODE {
		rs.Logger.Debugf("bg rest 批量下单 %v", params)
	}

	for _, v := range orderList {
		rs.SystemPass.Update(start, v.Time/1e3)
	}

	err = rs.call(http.MethodPost, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		handledOk := false
		defer func() {
			if !handledOk {
				err := recover()
				var errStr string
				if err != nil {
					errStr = err.(error).Error()
				}
				for i := range orderList {
					order := helper.OrderEvent{Type: helper.OrderEventTypeERROR, ClientID: orderList[i].ClientID, ErrorReason: errStr, Pair: info.Pair}
					rs.Cb.OnOrder(0, order)
					rs.ReqFail(base.FailNumActionIdx_Place)
				}
				if err != nil {
					panic(err)
				}
			}
		}()
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			// 出现错误的时候也要触发回调, 不是json格式
			for _, v := range orderList {
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = v.ClientID
				order.Pair = info.Pair
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, v.ClientID, handlerErr.Error())
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
			handledOk = true
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			if (code == "43008" || code == "43009") && !base.IsInUPC {
				helper.LogErrorThenCall("下单失败限价触发43008", rs.Cb.OnExit)
			}
			for _, v := range orderList {
				var order helper.OrderEvent
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = v.ClientID
				order.Pair = info.Pair
				setOrderErrorType(code, &order)
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, v.ClientID, handlerErr)
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
			handledOk = true
			return
		}

		data := value.Get("data")
		orderInfo := data.GetArray("orderInfo")
		for _, v := range orderInfo {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeNEW
			order.Pair = info.Pair
			order.OrderID = string(v.GetStringBytes("orderId"))
			order.ClientID = string(v.GetStringBytes("clientOid"))
			//
			rs.Cb.OnOrder(0, order)
			if rs.isInSpec {
				log.Infof("v1rs got-new-order %d %s", &orderList[0].Time, order.ClientID)
			}
			rs.ReqSucc(base.FailNumActionIdx_Place)
		}
		failure := data.GetArray("failure")
		for _, v := range failure {
			if helper.BytesToString(v.GetStringBytes("errorCode")) == "40786" && rs.BrokerConfig.IgnoreDuplicateClientOidError {
				continue
			}
			var order helper.OrderEvent
			// 这里可以考虑用cid找pair 需要考虑加
			// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
			order.Type = helper.OrderEventTypeERROR
			order.OrderID = string(v.GetStringBytes("orderId"))
			order.ClientID = string(v.GetStringBytes("clientOid"))
			//
			rs.Cb.OnOrder(0, order)
			rs.ReqFail(base.FailNumActionIdx_Place)
		}
		handledOk = true
	})

	if err != nil {
		for _, v := range orderList {
			var order helper.OrderEvent
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = v.ClientID
			order.Pair = info.Pair
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, v.ClientID, handlerErr)
			rs.ReqFail(base.FailNumActionIdx_Place)
		}
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

// cancelOrderID 撤单 bitget 仅支持oid撤单
func (rs *BitgetUsdtSwapRs) cancelOrderID(info *helper.ExchangeInfo, cid, oid string, tsns int64) {
	uri := "/api/mix/v1/order/cancel-order"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["marginCoin"] = "USDT"
	if oid != "" {
		params["orderId"] = oid
	} else {
		params["clientOid"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, tsns/1e3)

	err = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		rsp := base.MonitorOrderActionRsp{Client: base.ClientType_Rs, Action: base.ActionType_Cancel, Cid: cid, Oid: oid, DurationUs: time.Now().UnixMicro() - start}
		if ok := rs.DelayMonitor.TryNext(rsp); ok {
			return
		}

		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code == "40768" || code == "22001" || code == "25204" {
			// v1 body: {"code":"22001","msg":"No order to cancel","requestTime":1752235936932,"data":null}
			rs.failNum.Store(0)
			return
		}

		// 撤单不需要触发回调 撤单动作高度时间敏感 依赖ws推送
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
}

// cancelOrderIDs
func (rs *BitgetUsdtSwapRs) cancelOrderIDs(cancelList []helper.Signal) {
	uri := "/api/mix/v1/order/cancel-batch-orders"

	info, ok := rs.GetPairInfoByPair(&cancelList[0].Pair)
	if !ok {
		return
	}

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["marginCoin"] = "USDT"

	oid := []string{}
	cid := []string{}

	for _, v := range cancelList {
		if v.OrderID != "" {
			oid = append(oid, v.OrderID)
		} else {
			cid = append(cid, v.ClientID)
		}
	}
	if len(oid) > 0 {
		params["orderIds"] = oid
	}
	if len(cid) > 0 {
		params["clientOids"] = cid
	}

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, cancelList[0].Time/1e3)

	err = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {

		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}

		// 撤单不需要触发回调 撤单动作高度时间敏感 依赖ws推送
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
}

// checkOrder 查单 bitget 仅支持oid查单
func (rs *BitgetUsdtSwapRs) checkOrderID(info *helper.ExchangeInfo, cid, oid string) {
	uri := "/api/mix/v1/order/detail"

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	if oid != "" {
		params["orderId"] = oid
	} else {
		params["clientOid"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			rs.Logger.Errorf("checkOrderID %s %s %s", cid, oid, handlerErr.Error())
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code == "40109" {
			// {"code":"40109","msg":"The data of the order cannot be found, please confirm the order number","requestTime":1696498901805,"data":null},
			// order1 := helper.OrderEvent{} // todo 没support
			// order1.Type = helper.OrderEventTypeNotFound

			order := helper.OrderEvent{
				Pair:     info.Pair,
				Type:     helper.OrderEventTypeNotFound,
				ClientID: cid,
				OrderID:  oid,
			}
			rs.Cb.OnOrder(0, order)
			return
		}
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			rs.Logger.Errorf("wrong rsp. %v", helper.BytesToString(respBody))
			return
		}
		data := value.Get("data")
		if data.Type() == fastjson.TypeNull {
			// 查不到这个订单不做任何处理  等待策略层触发重置交易
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}

		// 查到订单
		var event helper.OrderEvent
		event.Pair = info.Pair
		event.OrderID = string(data.GetStringBytes("orderId"))
		event.ClientID = string(data.GetStringBytes("clientOid"))
		status := helper.BytesToString(data.GetStringBytes("state"))
		switch status {
		case "init", "new":
			event.Type = helper.OrderEventTypeNEW
		case "partially_filled":
			event.Type = helper.OrderEventTypePARTIAL
		case "filled", "cancelled", "canceled":
			event.Type = helper.OrderEventTypeREMOVE
		}
		event.Filled = fixed.NewF(data.GetFloat64("filledQty"))
		if event.Filled.GreaterThan(fixed.ZERO) {
			event.FilledPrice = data.GetFloat64("priceAvg")
		}
		reducesign := helper.MustGetBool(data, "reduceOnly")
		side := helper.MustGetShadowStringFromBytes(data, "side")
		event.OrderSide = rs.convertOrderSide(side, reducesign)
		if event.OrderSide == helper.OrderSideUnKnown {
			return
		}
		orderType := helper.MustGetShadowStringFromBytes(data, "orderType")
		tif := helper.MustGetShadowStringFromBytes(data, "timeInForce")
		event.OrderType = rs.convertOrderType(orderType, tif)
		rs.Cb.OnOrder(0, event)

	})

	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("check order err, %v", err)
	}
}

func (rs *BitgetUsdtSwapRs) convertOrderType(orderTypeStr string, tif string) (orderType helper.OrderType) {
	switch orderTypeStr {
	case "limit":
		orderType = helper.OrderTypeLimit
	case "market":
		orderType = helper.OrderTypeMarket
	}
	switch tif {
	case "ioc":
		orderType = helper.OrderTypeIoc
	case "post_only":
		orderType = helper.OrderTypePostOnly
	}
	return
}

func (rs *BitgetUsdtSwapRs) convertOrderSide(side string, reducesign bool) (orderSide helper.OrderSide) {
	switch side {
	case "buy_single":
		if reducesign {
			orderSide = helper.OrderSidePK
		} else {
			orderSide = helper.OrderSideKD
		}
	case "sell_single":
		if reducesign {
			orderSide = helper.OrderSidePD
		} else {
			orderSide = helper.OrderSideKK
		}

	default:
		rs.Logger.Errorf("side error: %s", side)
	}
	/*
		case "open_long":
			event.OrderSide = helper.OrderSideKD
		case "open_short":
			event.OrderSide = helper.OrderSideKK
		case "close_short":
			event.OrderSide = helper.OrderSidePK
		case "close_long":
			event.OrderSide = helper.OrderSidePD
	*/
	return
}

func sign(method, requestPath, body, timesStamp, secKey string) string {
	var payload strings.Builder
	payload.WriteString(timesStamp)
	payload.WriteString(method)
	payload.WriteString(requestPath)
	if body != "" && body != "?" {
		payload.WriteString(body)
	}
	hash := hmac.New(sha256.New, []byte(secKey))
	hash.Write([]byte(payload.String()))
	result := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return result
}

// call 专用于bitget_usdt_swap的发起请求函数
func (rs *BitgetUsdtSwapRs) call(reqMethod string, reqUrl string, params map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler) error {
	reqHeaders := make(map[string]string, 0)
	if rs.BrokerConfig.ApiBrokerCode != "" {
		reqHeaders["X-CHANNEL-API-CODE"] = rs.BrokerConfig.ApiBrokerCode
	}
	encodes := ""
	encode := make([]string, 0)
	for key, param := range params {
		encode = append(encode, fmt.Sprintf("%s=%v", key, param))
	}
	var body []byte
	if needSign {
		if params == nil {
			params = make(map[string]interface{}, 0)
		}
		now := strconv.FormatInt(time.Now().Unix()*1000, 10)
		encodes = strings.Join(encode, "&")

		_sign := ""
		if reqMethod == http.MethodPost {
			body, _ = json.Marshal(params)
			_sign = sign(reqMethod, reqUrl, helper.BytesToString(body), now, rs.BrokerConfig.SecretKey)
		} else {
			_sign = sign(reqMethod, reqUrl, "?"+encodes, now, rs.BrokerConfig.SecretKey)
		}

		reqHeaders["ACCESS-TIMESTAMP"] = now
		reqHeaders["ACCESS-SIGN"] = _sign
		reqHeaders["ACCESS-KEY"] = rs.BrokerConfig.AccessKey
		reqHeaders["ACCESS-PASSPHRASE"] = rs.BrokerConfig.PassKey
	} else {
		encodes = strings.Join(encode, "&")
	}
	reqHeaders["Content-Type"] = "application/json"
	switch reqMethod {
	case http.MethodGet, http.MethodDelete:
		if len(encodes) > 0 {
			if strings.Contains(reqUrl, "?") {
				reqUrl += "&" + encodes
			} else {
				reqUrl += "?" + encodes
			}
		}
		//case http.MethodPost:

		//default:

	}

	status, err := rs.client.Request(reqMethod, BaseUrl+reqUrl, body, reqHeaders, respHandler)
	if err != nil {
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return err
	}
	if status != 200 {
		// 如果418 429 就触发onExit
		// 有些交易所如果收到 418 429 必须马上停机 有些可以不停机bitget
		// 这部分逻辑需要定制化处理
		// bitget没status反馈，在实际的resp body中的code
		rs.failNum.Add(1)
		if rs.failNum.Load() > 50 {
			rs.Cb.OnExit("连续请求出错 需要停机")
		}
		return fmt.Errorf("status:%d", status)
	}
	rs.failNum.Store(0)
	return nil
}

func (rs *BitgetUsdtSwapRs) GetExName() string {
	return helper.BrokernameBitgetUsdtSwap.String()
}

func (rs *BitgetUsdtSwapRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	uri := "/api/mix/v1/position/allPosition-v2"

	// RESP:[200] body: {"code":"00000","msg":"success","requestTime":1720839298576,"data":[{"marginCoin":"USDT","symbol":"ETHUSDT_UMCBL","holdSide":"long","openDelegateCount":"0","margin":"9.38121",
	// "available":"0.03","locked":"0","total":"0.03","leverage":10,"achievedProfits":"0","averageOpenPrice":"3127.07","marginMode":"crossed","holdMode":"single_hold","unrealizedPL":"-0.0018","liquid
	// ationPrice":"-13757.963204658667","keepMarginRate":"0.005","marketPrice":"3127.01","marginRatio":"0.001560337569","autoMargin":"off","cTime":"1720839285163","uTime":"1720839285163"},{"marginCoin":
	// "USDT","symbol":"BTCUSDT_UMCBL","holdSide":"long","openDelegateCount":"0","margin":"5.7882","available":"0.001","locked":"0","total":"0.001","leverage":10,"achievedProfits":"0","averageOpenPrice"
	// :"57882","marginMode":"crossed","holdMode":"single_hold","unrealizedPL":"0.0061","liquidationPrice":"-448661.09613976","keepMarginRate":"0.004","marketPrice":"57888.1","marginRatio":"0.001560337569",
	// "autoMargin":"off","cTime":"1720839285124","uTime":"1720839285124"}]}, header:HTTP/1.1 200 OK

	params := make(map[string]interface{})
	params["productType"] = "umcbl"
	params["marginCoin"] = "USDT"
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	// 发起请求
	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("清空所有持仓失败: %s", err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri) {
			err.HandlerError = fmt.Errorf("%s", string(respBody))
			return
		}
		datas := value.GetArray("data")
		// longPos := fixed.ZERO
		// longAvg := 0.0
		// shortPos := fixed.ZERO
		// shortAvg := 0.0
		// uTimeMs := int64(0)
		// hasPos := false
		gotPosition := make(map[string]bool)
		for _, data := range datas {
			size := helper.MustGetFloat64FromBytes(data, "total")
			if size != 0 {
				symbol := string(data.GetStringBytes("symbol"))
				info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
				if !ok {
					rs.Logger.Errorf("not found pairinfo. %s", symbol)
					continue
				}

				side := string(data.GetStringBytes("holdSide"))
				gotPosition[symbol] = true
				averageOpenPrice := helper.MustGetFloat64FromBytes(data, "averageOpenPrice")
				available := helper.MustGetFloat64FromBytes(data, "available")

				updateTime := helper.MustGetInt64FromBytes(data, "uTime")
				tsns := time.Now().UnixNano()
				pos := helper.PositionSum{Name: info.Pair.String(), Amount: size, AvailAmount: available, Ave: averageOpenPrice,
					Seq: helper.NewSeq(updateTime, tsns),
				}
				pos.Side = helper.PosSideLong
				if side == "short" {
					pos.Side = helper.PosSideShort
				}

				holdMode := helper.MustGetShadowStringFromBytes(data, "holdMode")
				if holdMode == "double_hold" {
					// helper.LogErrorThenCall(fmt.Sprintf("bitget存在双向持仓，upc忽略该仓位。%v", data.String()), helper_ding.DingingSendSerious)
					// continue
					pos.Mode = helper.PosModeHedge
				}
				resp = append(resp, pos)

				var longPos, shortPos fixed.Fixed
				var longAvg, shortAvg float64
				if pos.Side == helper.PosSideShort {
					shortPos = fixed.NewF(size)
					shortAvg = averageOpenPrice
				} else {
					longPos = fixed.NewF(size)
					longAvg = averageOpenPrice
				}
				if _, pos, ok := rs.PosNewerAndStore(symbol, updateTime, tsns); ok {
					pos.Lock.Lock()
					pos.ResetLocked()
					pos.LongPos = longPos
					pos.LongAvg = longAvg
					pos.ShortPos = shortPos
					pos.ShortAvg = shortAvg
					event := pos.ToPositionEvent()

					pos.Lock.Unlock()
					rs.Cb.OnPositionEvent(0, event)
				}
			}
		}
		// Empty when no pos
		rs.CleanOthersPos(gotPosition, rs.Cb.OnPositionEvent)
	})
	return
}
func (rs *BitgetUsdtSwapRs) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("not found symbol for pair %s", symbol)
		return false
	}
	cid := fmt.Sprintf("99%d", uint32(time.Now().UnixMilli()))
	cannotReduce := rs.placeOrderForUPC(info, fixed.ZERO, orderAmount, cid, orderSide, helper.OrderTypeMarket, 0, posMode == helper.PosModeHedge, true)
	if cannotReduce {
		rs.placeOrderForUPC(info, fixed.ZERO, orderAmount, cid, orderSide, helper.OrderTypeMarket, 0, posMode == helper.PosModeHedge, false)
	}
	return true
}

func (rs *BitgetUsdtSwapRs) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	return rs.GetOrderListInFather(rs, startTimeMs, endTimeMs, orderState)
}

func (rs *BitgetUsdtSwapRs) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	const _LEN = 100 //交易所对该字段最大约束为100
	uri := "/api/mix/v1/order/historyProductType"
	params := make(map[string]interface{})
	params["productType"] = "umcbl"
	params["pageSize"] = _LEN
	params["startTime"] = startTimeMs
	params["endTime"] = endTimeMs

	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
		dataTemp := value.Get("data")
		orders := helper.MustGetArray(dataTemp, "orderList")
		resp.Orders = make([]helper.OrderForList, 0, len(orders))
		if len(orders) >= _LEN {
			resp.HasMore = true
		}

		for _, v := range orders {
			order := helper.OrderForList{
				OrderID:  helper.MustGetStringFromBytes(v, "orderId"),
				ClientID: helper.MustGetStringFromBytes(v, "clientOid"),
				//Price:         helper.GetFloat64FromBytes(v, "priceAvg"),
				Amount:        fixed.NewF(helper.MustGetFloat64(v, "size")),
				CreatedTimeMs: helper.MustGetInt64FromBytes(v, "cTime"),
				UpdatedTimeMs: helper.MustGetInt64FromBytes(v, "uTime"),
				// FinishedTime: ,
				//Filled: fixed.NewS(helper.GetShadowStringFromBytes(v, "filledQty")),
			}
			status := helper.BytesToString(v.GetStringBytes("state"))

			switch status {
			case "new", "init":
				order.OrderState = helper.OrderStatePending

			case "filled", "canceled", "partially_filled":
				order.OrderState = helper.OrderStateFinished
				order.UpdatedTimeMs = helper.MustGetInt64FromBytes(v, "uTime")
			}
			if orderState != helper.OrderStateAll && orderState != order.OrderState {
				continue
			}
			side := helper.MustGetShadowStringFromBytes(v, "side")
			switch side {
			case "open_long", "close_long", "buy_single":
				order.OrderSide = helper.OrderSideKD

			case "open_short", "close_short", "sell_single":
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
			switch helper.MustGetShadowStringFromBytes(v, "orderType") {
			case "limit":
				order.OrderType = helper.OrderTypeLimit
				order.Price = helper.MustGetFloat64(v, "price")
				order.FilledPrice = helper.MustGetFloat64(v, "price")
			case "market":
				order.OrderType = helper.OrderTypeMarket
				order.Price = helper.MustGetFloat64(v, "priceAvg")
				order.FilledPrice = helper.MustGetFloat64(v, "priceAvg")
			}
			// 必须在type后面。忽略其他类型
			switch helper.MustGetShadowStringFromBytes(v, "timeInForce") {
			case "ioc":
				order.OrderType = helper.OrderTypeIoc
			case "post_only":
				order.OrderType = helper.OrderTypePostOnly
			}
			resp.Orders = append(resp.Orders, order)
		}
	})
	return
}
func (rs *BitgetUsdtSwapRs) GetFee() (fee helper.Fee, err helper.ApiError) {
	// uri := "/api/mix/v1/market/contracts"
	// params := make(map[string]interface{})
	// params["productType"] = "umcbl"
	uri := "/api/v2/common/trade-rate"
	params := map[string]interface{}{
		"symbol":       rs.PairInfo.Extra,
		"businessType": "mix",
	}
	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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

		// data := value.GetArray("data")
		// fee.Maker = helper.MustGetFloat64FromBytes(data[0], "makerFeeRate")
		// fee.Taker = helper.MustGetFloat64FromBytes(data[0], "takerFeeRate")
		data := value.Get("data")
		fee.Maker = helper.MustGetFloat64FromBytes(data, "makerFeeRate")
		fee.Taker = helper.MustGetFloat64FromBytes(data, "takerFeeRate")
	})
	return
}
func (rs *BitgetUsdtSwapRs) GetFundingRate() (helper.FundingRate, error) {

	var fr helper.FundingRate
	uri := "/api/mix/v1/market/current-fundRate"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("failed to get funding rate. %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri) {
			err = errors.New(string(respBody))
			return
		}
		fr.UpdateTimeMS = time.Now().UnixMilli()
		// fr.FundingTimeMS = helper.MustGetInt64FromBytes(item, "fundingTime")
		fr.Rate = helper.MustGetFloat64FromBytes(value, "data", "fundingRate")
		fr.Pair = rs.symbolToPair(rs.Symbol)
	})
	uri = "/api/mix/v1/market/funding-time"
	err = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		value, err = p.ParseBytes(respBody)
		if err != nil {
			rs.Logger.Errorf("failed to get funding rate. %v", err)
			return
		}
		if !rs.isOkApiResponse(value, uri) {
			err = errors.New(string(respBody))
			return
		}
		fr.IntervalHours = helper.MustGetIntFromBytes(value, "data", "ratePeriod")
		fr.FundingTimeMS = helper.MustGetInt64FromBytes(value, "data", "fundingTime")
	})
	return fr, err
}
func (rs *BitgetUsdtSwapRs) GetFundingRates(pairs []string) (res map[string]helper.FundingRate, err helper.ApiError) {
	if len(pairs) == 0 {
		rs.Logger.Error("Get Funding Rates cannot get all, must give pairs")
		return
	}
	res = make(map[string]helper.FundingRate, len(pairs))

	params := make(map[string]interface{})
	var fr helper.FundingRate
	for _, pair := range pairs {
		symb, ok := rs.ExchangeInfoPtrP2S.Get(pair)
		if !ok {
			rs.Logger.Error("Cannot find symbol name for pair: ", pair)
			return
		}
		params["symbol"] = symb.Symbol

		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value

		uri := "/api/mix/v1/market/current-fundRate"
		err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				rs.Logger.Errorf("failed to get funding rate. %v", err.HandlerError)
				return
			}
			if !rs.isOkApiResponse(value, uri) {
				err.HandlerError = errors.New(string(respBody))
				return
			}
			fr.UpdateTimeMS = time.Now().UnixMilli()
			fr.Rate = helper.MustGetFloat64FromBytes(value, "data", "fundingRate")
			fr.Pair = symb.Pair
		})
		uri = "/api/mix/v1/market/funding-time"
		err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				rs.Logger.Errorf("failed to get funding rate. %v", err.HandlerError)
				return
			}
			if !rs.isOkApiResponse(value, uri) {
				err.HandlerError = errors.New(string(respBody))
				return
			}
			fr.IntervalHours = helper.MustGetIntFromBytes(value, "data", "ratePeriod")
			fr.FundingTimeMS = helper.MustGetInt64FromBytes(value, "data", "fundingTime")
		})
		res[pair] = fr
	}
	if helper.DEBUGMODE {
		rs.Logger.Info("Fundingrates", res)
	}
	return
}
func (rs *BitgetUsdtSwapRs) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	if e, err := rs.GetEquity(); err.Nil() {
		balance := helper.BalanceSum{
			Name:   "usdt",
			Price:  1.0,
			Amount: e.Cash,
			Avail:  e.CashFree,
		}
		a.Balances = append(a.Balances, balance)
	}
	if !err.Nil() {
		return
	}
	a.Positions, err = rs.GetOrigPositions()
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
func (rs *BitgetUsdtSwapRs) SendSignal(signals []helper.Signal) {
	// 判断是否需要批量下单 批量撤单
	lenSigs := len(signals)
	var sigsNewOrder = make([]helper.Signal, 0, lenSigs)
	var sigsCancel = make([]helper.Signal, 0, lenSigs)
	for _, s := range signals {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送信号 %s", s.String())
		}
		switch s.Type {
		case helper.SignalTypeNewOrder:
			if s.OrderSide == helper.OrderSidePK || s.OrderSide == helper.OrderSidePD || s.ForceReduceOnly {
				if helper.DEBUGMODE {
					rs.Logger.Debugf("发送挂单信号 %s", s.String())
				}
				info, ok := rs.GetPairInfoByPair(&s.Pair)
				if !ok {
					return
				}
				go rs.placeOrder(info, helper.FixPrice(s.Price, info.TickSize), s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false, s.ForceReduceOnly)
			} else {
				sigsNewOrder = append(sigsNewOrder, s)
			}
		case helper.SignalTypeCancelOrder:
			sigsCancel = append(sigsCancel, s)
		case helper.SignalTypeCheckOrder:
			info, ok := rs.GetPairInfoByPair(&s.Pair)
			if !ok {
				rs.Logger.Warnf("fail to found pairinfo. %s", &s.Pair)
				continue
			}
			go rs.checkOrderID(info, s.ClientID, s.OrderID)
		case helper.SignalTypeGetPos:
			go rs.GetPosition()
		case helper.SignalTypeGetEquity:
			go rs.GetEquity()
		case helper.SignalTypeGetTicker:
			go rs.GetTickerBySymbol(rs.Symbol)
		case helper.SignalTypeGetIndex:
			go rs.GetIndex()
		case helper.SignalTypeCancelOne:
			go rs.DoCancelPendingOrders(rs.Symbol)
		case helper.SignalTypeCancelAll:
			go rs.cancelAllOpenOrders()
		}
	}
	// 执行信号
	if len(sigsCancel) > 1 {
		go rs.cancelOrderIDs(sigsCancel)
	} else if len(sigsCancel) == 1 {
		// go rs.cancelOrderID(info, sigsCancel[0].OrderID, sigsCancel[0].Time)
		rs.CancelOrderSelect(sigsCancel[0])
	}
	lenNew := len(sigsNewOrder)
	if lenNew > 1 {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送批量挂单信号 %v", sigsNewOrder)
		}
		times := lenNew / 10
		if times*10 < lenNew {
			times += 1
		}
		for i := 0; i < times; i++ {
			// 批量下单
			if len(sigsNewOrder[i*10:]) > 10 {
				go rs.placeOrders(sigsNewOrder[i*10 : (i+1)*10])
			} else {
				go rs.placeOrders(sigsNewOrder[i*10:])
			}
		}

	} else if lenNew > 0 {

		s := sigsNewOrder[0]
		rs.Logger.Debugf("发送挂单信号 %s", s.String())

		// 2024-9-11
		// reduceonly 在常规交易固定设置为false，因为 bg会将 反向开仓单 也当平仓单处理，扣减 可平仓仓位量，例如 hold 100@long, pending 100@kk, place 100@pd 会出错
		// go rs.placeOrder(info, helper.FixPrice(s.Price, info.TickSize), s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, false)
		rs.PlaceOrderSelect(s)
	}
}

func (rs *BitgetUsdtSwapRs) getPosPnl() []transfer.Pnl {
	uri := "/api/mix/v1/position/history-position"
	after := time.Now().UTC().UnixNano() / 1000000
	sevenDaysAgo := time.Now().UTC().AddDate(0, 0, -7)
	before := sevenDaysAgo.UnixNano() / 1000000
	params := make(map[string]interface{})
	params["productType"] = "umcbl"
	params["endTime"] = after
	params["startTime"] = before
	params["pageSize"] = 90 //
	// params["lastEndId"]="1082617418837794816"
	// params["lastEndId"]="1082586467629932552"

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value
	var result transfer.Pnl
	var results []transfer.Pnl

	err = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		fmt.Println(value)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		da := value.Get("data")
		next := da.Get("endId")
		fmt.Println(next)
		datas := da.GetArray("list")
		// fmt.Println(next)
		for _, data := range datas {
			symbol := helper.BytesToString(data.GetStringBytes("symbol"))
			result.Symbol = symbol
			holdSide := helper.BytesToString(data.GetStringBytes("holdSide"))
			result.HoldSide = holdSide
			closeTotalPos := helper.BytesToString(data.GetStringBytes("closeTotalPos"))
			result.CloseTotalPos = closeTotalPos
			openTotalPos := helper.BytesToString(data.GetStringBytes("openTotalPos"))
			result.OpenTotalPos = openTotalPos
			pnl := helper.BytesToString(data.GetStringBytes("pnl"))
			result.Pnl = pnl
			netProfit := helper.BytesToString(data.GetStringBytes("netProfit"))
			result.NetProfit = netProfit
			openFee := helper.BytesToString(data.GetStringBytes("openFee"))
			result.OpenFee = openFee
			closeFee := helper.BytesToString(data.GetStringBytes("closeFee"))
			result.CloseFee = closeFee
			utime := helper.BytesToString(data.GetStringBytes("utime"))
			result.Utime = utime
			ctime := helper.BytesToString(data.GetStringBytes("ctime"))
			result.Ctime = ctime
			results = append(results, result)
		}
	})
	if err != nil {
		rs.Logger.Errorf("查询bitget仓位历史失败,返回错误为：%s", err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("查询bitget仓位历史失败,返回错误为：%s", handlerErr.Error())
	}
	return results
}

// Do 发起任意请求 一般用于非交易任务 对时间不敏感
func (rs *BitgetUsdtSwapRs) Do(actType string, params any) (any, error) {
	switch actType {
	case "GetFee":
		rs.BeforeTrade(helper.HandleModePrepare)
		f, _ := rs.GetFee()
		fmt.Println("v1 normal fee", f)
		uri := "/api/v2/common/trade-rate"
		params := map[string]interface{}{
			"symbol":       "BTCUSDT",
			"businessType": "mix",
		}
		rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			fmt.Printf("GetFee 返回结果为：%s", string(respBody))
		})
		time.Sleep(time.Second * 2)
		return nil, nil
	case "GetOpenCount":
		// rs.BeforeTrade(helper.HandleModeCloseAll)
		uri := "/api/mix/v1/account/open-count"
		params := make(map[string]interface{})
		params["symbol"] = rs.Symbol
		params["marginCoin"] = "USDT"
		params["openPrice"] = rs.TradeMsg.Ticker.Mp.Load()
		params["leverage"] = 20
		params["openAmount"] = 1000
		var value *fastjson.Value
		err := rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			rs.Logger.Infof("GetOpenCount 返回结果为：%s", string(respBody))
			p := handyPool.Get()
			defer handyPool.Put(p)
			value, _ = p.ParseBytes(respBody)
		})
		time.Sleep(time.Second * 2)
		return value, err
	}

	t, err := strconv.Atoi(actType)
	if err == nil {
		// actGetList := msgInterface[0].(transfer.DoActRecordListType)
		actGetList := transfer.DoActRecordListType(t)
		switch actGetList {
		case transfer.DoActLGetPosition:
			return rs.getPosPnl(), nil
		}
	}
	return nil, nil
}

func (rs *BitgetUsdtSwapRs) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltEquityAvailReducedWhenPendingOrder | base.AbltWsPriEquityWithUpnl
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
func (rs *BitgetUsdtSwapRs) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SWAP | base.ABILITIES_SEQ_WS | base.AbltRsPriGetPosWithSeq | base.AbltRsPriGetAllIndex
}

func (rs *BitgetUsdtSwapRs) WsLogged() bool {
	return false
}

func (rs *BitgetUsdtSwapRs) GetExchangeInfos() []helper.ExchangeInfo {
	return rs.getExchangeInfo()
}

func (rs *BitgetUsdtSwapRs) GetFeatures() base.Features {
	f := base.Features{
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		UpdateWsTickerWithSeq: true,
		GetFee:                true,
		GetOrderList:          true,
		GetFundingRate:        true,
		GetIndex:              true,
		MultiSymbolOneAcct:    true,
		OrderIOC:              true,
		OrderPostonly:         true,
		GetTickerSignal:       true,
		DelayInTicker:         true,
		ExchangeInfo_1000XX:   true,
		UnifiedPosClean:       true,
		AutoRefillMargin:      true, //通过SetAutoMargin设置
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}

func (rs *BitgetUsdtSwapRs) GetAllPendingOrders() (results []helper.OrderForList, err helper.ApiError) {
	url := "/api/mix/v1/order/marginCoinCurrent"
	p := handyPool.Get()
	var value *fastjson.Value

	params := make(map[string]interface{})
	params["productType"] = "umcbl"
	params["marginCoin"] = "USDT"

	err.NetworkError = rs.call(http.MethodGet, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("parse response error: %s", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = errors.New(string(respBody))
			return
		}
		for _, data := range helper.MustGetArray(value, "data") {
			order := helper.OrderForList{
				Symbol:   helper.MustGetStringFromBytes(data, "symbol"),
				ClientID: helper.MustGetStringFromBytes(data, "clientOid"),
				OrderID:  helper.MustGetStringFromBytes(data, "orderId"),
				Price:    data.GetFloat64("price"),
				Amount:   fixed.NewF(helper.MustGetFloat64(data, "size")),
			}
			order.Filled = fixed.NewF(helper.MustGetFloat64(data, "filledQty"))
			order.FilledPrice = helper.GetFloat64(data, "priceAvg")

			side := helper.GetShadowStringFromBytes(data, "side")
			order.OrderSide = rs.convertOrderSide(side, helper.MustGetBool(data, "reduceOnly"))

			orderType := helper.MustGetShadowStringFromBytes(data, "orderType")
			switch orderType {
			case "limit":
				order.OrderType = helper.OrderTypeLimit
			case "market":
				order.OrderType = helper.OrderTypeMarket
			}
			order.CreatedTimeMs = helper.MustGetInt64FromBytes(data, "cTime") * 1000
			results = append(results, order)
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("failed to get pending orders error: %s", err.Error())
	}
	return
}

func (rs *BitgetUsdtSwapRs) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	url := "/api/mix/v1/order/current"
	p := handyPool.Get()
	var value *fastjson.Value

	params := make(map[string]interface{})
	if symbol == "" {
		err.NetworkError = fmt.Errorf("symbol should not empty")
		return
	}
	params["symbol"] = symbol

	err.NetworkError = rs.call(http.MethodGet, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("parse response error: %s", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = errors.New(string(respBody))
			return
		}
		for _, data := range helper.MustGetArray(value, "data") {
			order := helper.OrderForList{
				Symbol:   helper.MustGetStringFromBytes(data, "symbol"),
				ClientID: helper.MustGetStringFromBytes(data, "clientOid"),
				OrderID:  helper.MustGetStringFromBytes(data, "orderId"),
				Price:    helper.MustGetFloat64(data, "price"),
				Amount:   fixed.NewF(helper.MustGetFloat64(data, "size")),
			}
			order.Filled = fixed.NewF(helper.MustGetFloat64(data, "filledQty"))
			order.FilledPrice = helper.GetFloat64(data, "priceAvg")

			side := helper.GetShadowStringFromBytes(data, "side")
			order.OrderSide = rs.convertOrderSide(side, helper.MustGetBool(data, "reduceOnly"))

			orderType := helper.MustGetShadowStringFromBytes(data, "orderType")
			switch orderType {
			case "limit":
				order.OrderType = helper.OrderTypeLimit
			case "market":
				order.OrderType = helper.OrderTypeMarket
			}
			order.CreatedTimeMs = helper.MustGetInt64FromBytes(data, "cTime") * 1000
			results = append(results, order)
		}
	})
	if err.NotNil() {
		rs.Logger.Errorf("failed to get pending orders error: %s", err.Error())
	}
	return
}

func (rs *BitgetUsdtSwapRs) GetPosHist(startTimeMs int64, endTimeMs int64) (resp []helper.Positionhistory, err helper.ApiError) {
	res, err := rs.DoGetPosHist(startTimeMs, endTimeMs)
	resp = res.Pos
	return
}

func (rs *BitgetUsdtSwapRs) DoGetPosHist(startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
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

func (rs *BitgetUsdtSwapRs) doGetPosHist(symbol string, startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
	lastEndId := ""
	for {
		res := helper.PosHistResponse{}
		res, lastEndId, err = rs.doGetPosHist1(symbol, startTimeMs, endTimeMs, lastEndId)
		if !err.Nil() {
			return
		} else {
			resp.Pos = append(resp.Pos, res.Pos...)
		}
		if !res.HasMore {
			break
		}
	}
	resp.HasMore = false
	return
}

func (rs *BitgetUsdtSwapRs) doGetPosHist1(symbol string, startTimeMs int64, endTimeMs int64, lastEndId string) (resp helper.PosHistResponse, endId string, err helper.ApiError) {
	uri := "/api/mix/v1/position/history-position"
	params := make(map[string]interface{})
	if symbol != "" {
		params["symbol"] = symbol
	} else {
		params["productType"] = "umcbl"
	}
	params["startTime"] = startTimeMs
	params["endTime"] = endTimeMs
	params["pageSize"] = 100
	if lastEndId != "" {
		params["lastEndId"] = lastEndId
	}

	err.NetworkError = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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

		deals := helper.MustGetArray(value, "data", "list")
		resp.Pos = make([]helper.Positionhistory, 0)

		for _, v := range deals {
			// {
			// "symbol": "ETHUSDT_UMCBL",
			// "marginCoin": "USDT",
			// "holdSide": "short",
			// "openAvgPrice": "1206.7",
			// "closeAvgPrice": "1206.8",
			// "marginMode": "fixed",
			// "openTotalPos": "1.15",
			// "closeTotalPos": "1.15",
			// "pnl": "-0.11",
			// "netProfit": "-1.780315",
			// "totalFunding": "0",
			// "openFee": "-0.83",
			// "closeFee": "-0.83",
			// "ctime": "1689300233897",
			// "utime": "1689300238205"
			// }
			neg := 1.0
			if helper.MustGetShadowStringFromBytes(v, "holdSide") != "short" {
				neg = -1
			}
			symbol := helper.MustGetStringFromBytes(v, "symbol")
			info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
			if !ok {
				rs.Logger.Errorf("not found pair for symbol %s", symbol)
				continue
			}
			deal := helper.Positionhistory{
				Symbol:        symbol,
				Pair:          info.Pair.String(),
				OpenTime:      helper.MustGetInt64(v, "ctime"),
				CloseTime:     helper.MustGetInt64(v, "utime"),
				OpenPrice:     helper.MustGetFloat64FromBytes(v, "openAvgPrice"),
				CloseAvePrice: helper.MustGetFloat64FromBytes(v, "closeAvgPrice"),
				OpenedAmount:  helper.MustGetFloat64FromBytes(v, "openTotalPos") * neg,
				ClosedAmount:  helper.MustGetFloat64FromBytes(v, "closeTotalPos") * neg,
				PnlAfterFees:  helper.MustGetFloat64FromBytes(v, "netProfit"),
				Fee:           helper.MustGetFloat64FromBytes(v, "closeFee") + helper.MustGetFloat64FromBytes(v, "openFee"),
			}
			resp.Pos = append(resp.Pos, deal)
		}
		endId = helper.GetStringFromBytes(value, "data", "endId")
		resp.HasMore = endId != ""
	})

	return
}

func (rs *BitgetUsdtSwapRs) SetAutoMargin(pair helper.ExchangeInfo, holdSide helper.PosSide) (err helper.ApiError) {
	url := "/api/mix/v1/account/set-auto-margin"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	symbol := pair.Symbol

	params := make(map[string]interface{})
	if symbol == "" {
		err.NetworkError = fmt.Errorf("symbol should not empty")
		return
	}

	var holdSideStr string
	if holdSide == helper.PosSideLong {
		holdSideStr = "long"
	} else {
		holdSideStr = "short"
	}

	params["symbol"] = symbol
	params["marginCoin"] = "USDT"
	params["holdSide"] = holdSideStr
	params["autoMargin"] = "on"

	err.NetworkError = rs.call(http.MethodPost, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("parse response error: %s", err.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = errors.New(string(respBody))
			return
		}
		respCode := helper.MustGetStringFromBytes(value, "code")
		if respCode != "00000" {
			err.HandlerError = errors.New(string(respBody))
			return
		}
	})

	if err.NotNil() {
		rs.Logger.Errorf("failed to get pending orders error: %s", err.Error())
	}
	return
}

func (rs *BitgetUsdtSwapRs) DoGetDepth(info *helper.ExchangeInfo) (respDepth helper.Depth, err helper.ApiError) {
	// 请求必备信息
	uri := "/api/mix/v1/market/depth"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取tick失败, %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, uri, params) {
			return
		}
		data := value.Get("data")
		asks := data.GetArray("asks")
		bids := data.GetArray("bids")
		if len(asks) == 0 || len(bids) == 0 {
			return
		}
		for _, bid := range bids {
			_bid, err := bid.Array()
			if err != nil {
				continue
			}
			price, _ := _bid[0].StringBytes()
			amount, _ := _bid[1].StringBytes()
			respDepth.Bids = append(respDepth.Bids, helper.DepthItem{
				Price:  fastfloat.ParseBestEffort(helper.BytesToString(price)),
				Amount: fastfloat.ParseBestEffort(helper.BytesToString(amount)),
			})
		}
		for _, ask := range asks {
			_ask, err := ask.Array()
			if err != nil {
				continue
			}
			price, _ := _ask[0].StringBytes()
			amount, _ := _ask[1].StringBytes()
			respDepth.Asks = append(respDepth.Asks, helper.DepthItem{
				Price:  fastfloat.ParseBestEffort(helper.BytesToString(price)),
				Amount: fastfloat.ParseBestEffort(helper.BytesToString(amount)),
			})
		}
	})
	return
}
func (rs *BitgetUsdtSwapRs) DoGetOI(info *helper.ExchangeInfo) (oi float64, err helper.ApiError) {
	// 请求必备信息
	uri := "/api/v2/mix/market/open-interest"
	params := make(map[string]interface{})
	params["symbol"] = info.Extra
	params["productType"] = "USDT-FUTURES"
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		var p fastjson.Parser
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to get depth. %v", err.HandlerError)
			return
		}

		if !rs.isOkApiResponse(value, uri, params) {
			err.HandlerError = fmt.Errorf("handler error %v", value)
			rs.Logger.Error(err.HandlerError)
			return
		}

		for _, item := range value.GetArray("data", "openInterestList") {
			var tick helper.Ticker
			tick, err = rs.GetTickerBySymbol(info.Symbol)
			if err.NotNil() {
				return
			}
			oi = helper.MustGetFloat64(item, "size") * tick.Price()
			return
		}
	})
	return
}
