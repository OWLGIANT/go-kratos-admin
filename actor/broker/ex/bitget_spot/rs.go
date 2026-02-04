package bitget_spot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"reflect"

	"actor/helper/transfer"
	"actor/tools"
	"github.com/duke-git/lancet/v2/slice"
	jsoniter "github.com/json-iterator/go"

	"actor/broker/base"
	"actor/broker/brokerconfig"
	"actor/broker/client/rest"
	"actor/helper"
	"actor/third/fixed"

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

func (rs *BitgetSpot) isOkApiResponse(value *fastjson.Value, url string, params ...map[string]interface{}) bool {
	code := helper.BytesToString(value.GetStringBytes("code"))
	if code == "00000" {
		rs.ReqSucc(base.FailNumActionIdx_AllReq)
		return true
	} else {
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

// 用于批量下单的订单信息结构体
type oneOrderData struct {
	side      helper.OrderSide
	orderType helper.OrderType
	price     float64
	amount    fixed.Fixed
	cid       string
	time      int64
	pair      helper.Pair
}

func (o *oneOrderData) String() string {
	return fmt.Sprintf("cid:%v side:%v p:%v a:%v type:%v", o.cid, o.side, o.price, o.amount, o.orderType)
}

type BitgetSpot struct {
	base.DummyTransfer
	base.FatherRs
	base.DummyBatchOrderAction
	base.DummyGetPriWs
	base.DummyDoGetPriceLimit
	base.DummyDoForSpot
	base.DummyOrderAction
	client    *rest.Client   // 通用rest客户端
	failNum   atomic.Int64   // 出错次数
	takerFee  atomic.Float64 // taker费率
	makerFee  atomic.Float64 // maker费率
	lastTotal atomic.Float64 // 最新一次获取的总资产
}

// 创建新的实例
func NewRs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}

	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BitgetSpotRestUrl != "" {
		BaseUrl = cfg.BitgetSpotRestUrl
		params.Logger.Infof("bitget_spot rs 启用colo  %v", BaseUrl)
	}

	rs := &BitgetSpot{
		client: rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
	}
	rs.client.SetUserAgentSlice(helper.UserAgentSlice)
	base.InitFatherRs(msg, rs, rs, &rs.FatherRs, params, pairInfo, cb)
	rs.Symbol = rs.PairToSymbol(&rs.Pair)
	return rs
}

// 获取全市场交易规则
// https://bitgetlimited.github.io/apidoc/zh/spot/#d990beddee
// 限速规则：20次/1s
func (rs *BitgetSpot) getExchangeInfo() []helper.ExchangeInfo {
	// 尝试从文件中读取exchangeInfo
	fileName := base.GenExchangeInfoFileName("")
	if pairInfo, infos, ok := helper.TryGetExchangeInfosPtrFromFile(fileName, rs.Pair, rs.ExchangeInfoPtrS2P, rs.ExchangeInfoPtrP2S); ok {
		helper.CopySymbolInfo(rs.PairInfo, &pairInfo)
		rs.Symbol = pairInfo.Symbol
		return infos
	}

	tickers := make(map[string]float64, 1)
	{
		url := "/api/spot/v1/market/tickers"
		var handleErr error
		var value *fastjson.Value
		err := rs.call(http.MethodGet, url, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := fastjson.Parser{}
			value, handleErr = p.ParseBytes(respBody)
			if handleErr != nil {
				rs.Logger.Errorf("获取 tickers 失败 %s", handleErr.Error())
				rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, handleErr.Error()))
				return
			}
			if !rs.isOkApiResponse(value, url) {
				rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. ", rs.ExchangeName))
				return
			}
			for _, v := range value.GetArray("data") {
				symbol := string(v.GetStringBytes("symbol"))
				last := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("close")))
				tickers[symbol] = last
			}
		})
		if err != nil {
			rs.Logger.Errorf("获取 tickers 失败 %s", err.Error())
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", rs.ExchangeName, err.Error()))
			return nil
		}
	}

	uri := "/api/spot/v1/public/products"

	// 待使用数据结构
	infos := make([]helper.ExchangeInfo, 0)
	// 发起请求
	err := rs.call(http.MethodGet, uri, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var handlerErr error
		var value ExchangeInfo
		handlerErr = jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %v, %s", rs.ExchangeName, handlerErr, string(respBody)))
			return
		}
		if value.Code != "00000" {
			rs.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %v, %s", rs.ExchangeName, handlerErr, string(respBody)))
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)
		datas := value.Data
		for _, data := range datas {
			baseCoin := strings.ToLower(data.BaseCoinDisplayName)
			quoteCoin := strings.ToLower(data.QuoteCoin)
			symbol := data.Symbol         // BTCUSDT_SPBL
			symbolName := data.SymbolName // BTCUSDT
			priceScale := helper.MustGetInt64FromString(data.PriceScale)
			quantityScale := helper.MustGetInt64FromString(data.QuantityScale)
			info := helper.ExchangeInfo{
				Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
				Symbol:         symbol,
				Status:         data.Status == "online",
				TickSize:       helper.ConvIntToFixed(priceScale),
				StepSize:       helper.ConvIntToFixed(quantityScale),
				MaxOrderAmount: helper.MustGetFloat64FixedCappedFromString(data.MaxTradeAmount),
				MinOrderAmount: fixed.NewS(data.MinTradeAmount),
				MaxOrderValue:  fixed.NewF(200000),
				MinOrderValue:  fixed.NewS(data.MinTradeUSDT),
				Multi:          fixed.ONE,
				// 最大持仓价值
				MaxPosValue: fixed.BIG,
				// 最大持仓数量
				MaxPosAmount: fixed.BIG,
			}
			if info.MaxPosAmount == fixed.NaN || info.MaxPosAmount.IsZero() {
				info.MaxPosAmount = fixed.BIG
			}
			if info.MinOrderValue.LessThan(fixed.NewF(5)) {
				info.MinOrderValue = fixed.NewF(5)
				if strings.ToLower(info.Pair.Quote) != "usdt" && strings.ToLower(info.Pair.Quote) != "usdc" && strings.ToLower(info.Pair.Quote) != "eur" {
					px := tickers[strings.ToUpper(quoteCoin+"USDT")]
					if px > 0 {
						info.MinOrderValue = fixed.NewF(5 / px)
						info.MaxOrderValue = fixed.NewF(200000 / px)
					} else {
						rs.Logger.Warn("non usdt pair 无法获取quoteCoin_usdt价格" + strings.ToUpper(quoteCoin))
						continue
					}
				}
			}
			if info.MinOrderAmount.IsZero() {
				v, ok := tickers[symbolName]
				if !ok || v == 0.0 {
					rs.Logger.Errorf("[utf_ign]获取交易对 %s 最新价格失败, 可能已下架", symbolName) // 有些上线币种没有订单薄，也会返回PEPEGBP这些币对
					continue
					// b.Cb.OnExit(fmt.Sprintf("[%s]获取交易信息失败 需要停机. %s", b.ExchangeName, "获取交易对最新价格失败"))
					// return
				}
				info.MinOrderAmount = info.MinOrderValue.Div(fixed.NewF(v))
			}
			if info.MaxOrderAmount.IsZero() {
				info.MaxOrderAmount = fixed.BIG
			}
			if info.MinOrderAmount.GreaterThan(info.MaxOrderAmount) {
				rs.Logger.Warnf("交易对 %s 最小下单量大于最大下单量", symbol)
				continue
			}
			if ok, err := info.CheckReady(); !ok {
				rs.Logger.Warnf("exchangeinfo not ready. %s", err)
				continue
			}
			if baseCoin == rs.Pair.Base && quoteCoin == rs.Pair.Quote {
				helper.CopySymbolInfo(rs.PairInfo, &info)
				rs.Symbol = info.Symbol
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

	if rs.PairInfo.MinOrderAmount.IsZero() || rs.PairInfo.MinOrderValue.IsZero() {
		helper.LogErrorThenCall("无法获取正确交易规则，需要停机", rs.Cb.OnExit)
		return nil
	}

	// 写入文件
	rs.CheckAndSaveExInfo(fileName, infos)
	return infos
}

// getTicker 获取ticker行情
// https://bitgetlimited.github.io/apidoc/zh/spot/#ticker
// 限速规则：20次/1s
func (rs *BitgetSpot) GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError) {
	// 请求必备信息
	uri := "/api/spot/v1/market/ticker"
	params := make(map[string]interface{}, 1)
	params["symbol"] = symbol
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value

	err.NetworkError = rs.call(http.MethodGet, uri, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			err.HandlerError = errors.New(helper.BytesToString(respBody))
			return
		}
		data := value.Get("data")
		ap := helper.MustGetFloat64FromBytes(data, "sellOne")
		bp := helper.MustGetFloat64FromBytes(data, "buyOne")
		aq := helper.MustGetFloat64FromBytes(data, "askSz")
		bq := helper.MustGetFloat64FromBytes(data, "bidSz")
		if symbol == rs.Symbol {
			rs.TradeMsg.Ticker.Set(ap, aq, bp, bq)
			rs.Cb.OnTicker(0)
		}
		ticker.Set(ap, aq, bp, bq)
	})
	if !err.Nil() {
		rs.Logger.Errorf("[%s] get ticker error: %s", rs.ExchangeName, err.Error())
	}
	return
}

func (rs *BitgetSpot) GetAllTickersKeyedSymbol() (ret map[string]helper.Ticker, err helper.ApiError) {
	ret = make(map[string]helper.Ticker)
	url := "/api/spot/v1/market/tickers"
	// 请求必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)

	err.NetworkError = rs.call(http.MethodGet, url, nil, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("failed to parse ticker. %v", err.HandlerError)
			return
		}

		datas := helper.MustGetArray(value, "data")
		for _, data := range datas {
			t := helper.Ticker{}
			symbol := helper.MustGetStringFromBytes(data, "symbol") + "_SPBL"
			ap := helper.MustGetFloat64FromBytes(data, "sellOne")
			bp := helper.MustGetFloat64FromBytes(data, "buyOne")
			if ap == 0 || bp == 0 {
				continue
			}
			aq := helper.MustGetFloat64FromBytes(data, "askSz")
			bq := helper.MustGetFloat64FromBytes(data, "bidSz")
			t.Set(ap, aq, bp, bq)
			ret[symbol] = t
		}
	})
	return
}

// GetEquity 获取账户资金 函数本身无返回值 仅更新本地资产并通过callbackfunc传递出去
// https://bitgetlimited.github.io/apidoc/zh/margin/#bcf414608b
// 限速规则：10次/1s
func (rs *BitgetSpot) GetEquity() (resp helper.Equity, err helper.ApiError) { //获取资产
	var balances []helper.BalanceSum
	balances, err = rs.getEquity()
	if !err.Nil() {
		rs.Logger.Errorf("failed to get equity, %v", err.Error())
		return
	}
	//
	var coin, coinFree, cash, cashFree float64
	for _, ba := range balances {
		if ba.Name == rs.Pair.Base {
			coin = ba.Amount
			coinFree = ba.Avail
		} else if ba.Name == rs.Pair.Quote {
			cash = ba.Amount
			cashFree = ba.Avail
		}
	}
	mp := rs.TradeMsg.Ticker.Mp.Load()
	if mp > 0 {
		total := coin*mp + cash
		lastTotal := rs.lastTotal.Load()
		canUpdate := false
		if lastTotal == 0.0 {
			canUpdate = true
		} else if math.Abs(total-lastTotal)/total < 0.05 { // 过滤资金跳变
			canUpdate = true
		}
		if canUpdate {
			rs.lastTotal.Store(total)
		}
	}
	resp.Coin = coin
	resp.CoinFree = coinFree
	resp.Cash = cash
	resp.CashFree = cashFree
	resp.IsSet = true
	return
}

// 撤掉所有挂单
// https://bitgetlimited.github.io/apidoc/zh/spot/#19671a1099 [获取未成交列表] [限速规则 20次/1s]
// https://bitgetlimited.github.io/apidoc/zh/spot/#v2-5 [单币对批量撤单] [限速规则 10次/1s]
func (rs *BitgetSpot) cancelAllOpenOrders(only bool) {
	uri := "/api/spot/v1/trade/open-orders"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodPost, uri, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		data := value.GetArray("data")
		for _, item := range data {
			symbol := strings.ToUpper(string(item.GetStringBytes("symbol")))
			if only && symbol != rs.Symbol {
				continue
			}
			rs.DoCancelPendingOrders(symbol)
			time.Sleep(time.Millisecond * 1200 * 1 / 10)
		}
	})
	if err != nil {
		rs.Logger.Errorf("[%s] cancelAllOpenOrders request error: %s", rs.ExchangeName, err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("[%s] cancelAllOpenOrders handle error: %s", rs.ExchangeName, handlerErr.Error())
	}
}

// 撤掉单交易对所有挂单
// https://bitgetlimited.github.io/apidoc/zh/spot/#d086f19b09
// 限速规则 10次/1s
func (rs *BitgetSpot) DoCancelPendingOrders(symbol string) (err helper.ApiError) {
	uri := "/api/spot/v1/trade/cancel-symbol-order"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var value *fastjson.Value
	params := make(map[string]interface{}, 1)
	params["symbol"] = symbol
	code := ""
	err.NetworkError = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code = helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
	})

	if code == "43001" {
		return
	}

	if err.NotNil() {
		rs.Logger.Errorf("[%s] DoCancelPendingOrders request error: %s", rs.ExchangeName, err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("[%s] DoCancelPendingOrders handle error: %s", rs.ExchangeName, handlerErr.Error())
	}
	return
}

// BeforeTrade 开始交易前需要做的所有工作 调整好杠杆
func (rs *BitgetSpot) BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError) {
	err = rs.EnsureCanRun()
	if err.NotNil() {
		return
	}
	// 获取交易规则
	rs.getExchangeInfo()
	rs.UpdateExchangeInfoSimp(rs.Cb.OnExit)
	if err = rs.CheckPairs(); err.NotNil() {
		return
	}
	switch mode {
	case helper.HandleModePublic:
		rs.GetTickerBySymbol(rs.Symbol)
		return
	case helper.HandleModeCloseOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		leakedPrev = rs.HasPosition(rs, true)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		leakedPrev = rs.HasPosition(rs, false)
		rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.DoCancelPendingOrders(rs.Symbol)
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
	}
	// 获取ticker
	rs.GetTickerBySymbol(rs.Symbol)
	// 获取账户资金
	rs.GetEquity()
	rs.PrintAcctSumWhenBeforeTrade(rs)
	return
}

func (rs *BitgetSpot) DoStop() {}

// AfterTrade 结束交易时需要做的所有工作  清空挂单和仓位
// 如果有遗漏仓位 返回true  如果清仓干净了 返回false
func (rs *BitgetSpot) AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError) {
	isLeft = true
	err = rs.EnsureCanRun()
	switch mode {
	case helper.HandleModePrepare:
		isLeft = false
	case helper.HandleModeCloseOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, true)
	case helper.HandleModeCloseAll:
		rs.cancelAllOpenOrders(false)
		isLeft = rs.CleanPosInFather(rs.BrokerConfig.MaxValueClosePerTimes, rs, rs, false)
	case helper.HandleModeCancelOne:
		rs.DoCancelPendingOrders(rs.Symbol)
		isLeft = false
	case helper.HandleModeCancelAll:
		rs.cancelAllOpenOrders(false)
		isLeft = false
	}
	return
}

// placeOrder 下单
// https://bitgetlimited.github.io/apidoc/zh/spot/#fd6ce2a756
// 限速规则：10次/1s
func (rs *BitgetSpot) placeOrder(pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64) {
	url := "/api/spot/v1/trade/orders"
	params := make(map[string]interface{}, 7)
	params["symbol"] = pairInfo.Symbol
	if price > 0.0 {
		params["price"] = helper.FixPrice(price, pairInfo.TickSize).String()
	}
	if orderType == helper.OrderTypeMarket && (side == helper.OrderSideKD || side == helper.OrderSidePK) {
		ticker, err := rs.GetTickerBySymbol(pairInfo.Symbol)
		if !err.Nil() {
			rs.Logger.Errorf("failed to get ticker. %s %v", pairInfo.Symbol, err)
			return
		}
		params["quantity"] = helper.FixAmount(fixed.NewF(size.Float()*ticker.Ap.Load()), pairInfo.TickSize).String()
	} else {
		params["quantity"] = size.String()
	}

	if orderType == helper.OrderTypeIoc {
		params["orderType"] = "limit"
		params["force"] = "ioc"
	} else if orderType == helper.OrderTypePostOnly {
		params["orderType"] = "limit"
		params["force"] = "post_only"
	} else if orderType == helper.OrderTypeMarket {
		params["orderType"] = "market"
		params["force"] = "normal"
	} else if orderType == helper.OrderTypeLimit {
		params["orderType"] = "limit"
		params["force"] = "normal"
	} else {
		order := helper.OrderEvent{Pair: pairInfo.Pair}
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 下单类型不正确%v", rs.ExchangeName, cid, orderType)
		return
	}

	switch side {
	case helper.OrderSideKD, helper.OrderSidePK:
		params["side"] = "buy"
	case helper.OrderSidePD, helper.OrderSideKK:
		params["side"] = "sell"
	default:
		order := helper.OrderEvent{Pair: pairInfo.Pair}
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 错误的订单方向 %v", rs.ExchangeName, cid, side)
		return
	}

	if cid != "" {
		params["clientOrderId"] = cid
	}
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(time.Now().UnixMicro(), t/1e3)
	err = rs.call(http.MethodPost, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		handledOk := false
		defer func() {
			if !handledOk {
				order := helper.OrderEvent{Pair: pairInfo.Pair, Type: helper.OrderEventTypeERROR, ClientID: cid}
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
			order := helper.OrderEvent{Pair: pairInfo.Pair}
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, handlerErr.Error())
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			order := helper.OrderEvent{Pair: pairInfo.Pair}
			order.Type = helper.OrderEventTypeERROR
			if code == "45119" {
				order.ErrorType = helper.OrderErrorTypeNotAllowOpen
			} else if (code == "43008" || code == "43009") && !base.IsInUPC {
				helper.LogErrorThenCall("下单失败限价触发43008", rs.Cb.OnExit)
			}
			order.ClientID = cid
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, cid, handlerErr)
			rs.ReqFail(base.FailNumActionIdx_Place)
			return
		}

		data := value.Get("data")
		order := helper.OrderEvent{Pair: pairInfo.Pair}

		// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
		order.Type = helper.OrderEventTypeNEW
		order.OrderID = string(data.GetStringBytes("orderId"))
		order.ClientID = string(data.GetStringBytes("clientOrderId"))
		handledOk = true
		rs.Cb.OnOrder(0, order)
		rs.ReqSucc(base.FailNumActionIdx_Place)
	})

	if err != nil {
		//得检查是否有限频提示
		order := helper.OrderEvent{Pair: pairInfo.Pair}
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		rs.Cb.OnOrder(0, order)
		rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, cid, err.Error())
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

// placeOrders 批量下单
// https://bitgetlimited.github.io/apidoc/zh/spot/#de93fae07b
// 限速规则 5次/1s
func (rs *BitgetSpot) placeOrders(orderList []helper.Signal) {
	url := "/api/spot/v1/trade/batch-orders"
	params := make(map[string]interface{})
	info, ok := rs.GetPairInfoByPair(&orderList[0].Pair)
	if !ok {
		rs.Logger.Error("fail to place order")
		return
	}
	params["symbol"] = info.Symbol

	orderDatas := make([]map[string]interface{}, 0, len(orderList))
	for _, v := range orderList {
		oneOrderData := make(map[string]interface{})
		oneOrderData["clientOrderId"] = v.ClientID
		oneOrderData["quantity"] = v.Amount.String()
		oneOrderData["price"] = helper.FixPrice(v.Price, info.TickSize).String()
		if v.OrderType == helper.OrderTypeIoc {
			oneOrderData["orderType"] = "limit"
			oneOrderData["force"] = "ioc"
		} else if v.OrderType == helper.OrderTypePostOnly {
			oneOrderData["orderType"] = "limit"
			oneOrderData["force"] = "post_only"
		} else if v.OrderType == helper.OrderTypeMarket {
			oneOrderData["orderType"] = "market"
			oneOrderData["force"] = "normal"
		} else if v.OrderType == helper.OrderTypeLimit {
			oneOrderData["orderType"] = "limit"
			oneOrderData["force"] = "normal"
		} else {
			continue
		}
		switch v.OrderSide {
		case helper.OrderSideKD, helper.OrderSidePK:
			oneOrderData["side"] = "buy"
		case helper.OrderSidePD, helper.OrderSideKK:
			oneOrderData["side"] = "sell"
		default:
			continue
		}
		orderDatas = append(orderDatas, oneOrderData)
	}
	params["orderList"] = orderDatas
	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
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
					order := helper.OrderEvent{Pair: info.Pair,
						Type: helper.OrderEventTypeERROR, ClientID: orderList[i].ClientID, ErrorReason: errStr}
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
				order := helper.OrderEvent{Pair: info.Pair}
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = v.ClientID
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %s", rs.ExchangeName, v.ClientID, handlerErr.Error())
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			if (code == "43008" || code == "43009") && !base.IsInUPC {
				helper.LogErrorThenCall("下单失败限价触发43008"+string(respBody), rs.Cb.OnExit)
			}
			for _, v := range orderList {
				order := helper.OrderEvent{Pair: info.Pair}
				order.Type = helper.OrderEventTypeERROR
				order.ClientID = v.ClientID
				rs.Cb.OnOrder(0, order)
				rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, v.ClientID, handlerErr)
				rs.ReqFail(base.FailNumActionIdx_Place)
			}
			return
		}

		data := value.Get("data")
		orderInfo := data.GetArray("resultList")
		for _, v := range orderInfo {
			order := helper.OrderEvent{Pair: info.Pair}
			// 下单成功时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
			order.Type = helper.OrderEventTypeNEW
			// order.Pair = v.Pair
			order.OrderID = string(v.GetStringBytes("orderId"))
			order.ClientID = string(v.GetStringBytes("clientOrderId"))
			rs.Cb.OnOrder(0, order)
			rs.ReqSucc(base.FailNumActionIdx_Place)
		}
		failure := data.GetArray("failure")
		for _, v := range failure {
			order := helper.OrderEvent{Pair: info.Pair}
			// order.Pair = v.Pair
			// 下单失败时 只需要获取oid信息 抛出到策略层 将oid和本地cid匹配
			order.Type = helper.OrderEventTypeERROR
			order.OrderID = string(v.GetStringBytes("orderId"))
			order.ClientID = string(v.GetStringBytes("clientOrderId"))
			rs.Cb.OnOrder(0, order)
			rs.ReqFail(base.FailNumActionIdx_Place)
		}
		handledOk = true
	})

	if err != nil {
		for _, v := range orderList {
			order := helper.OrderEvent{Pair: info.Pair}
			order.Type = helper.OrderEventTypeERROR
			order.ClientID = v.ClientID
			rs.Cb.OnOrder(0, order)
			rs.Logger.Errorf("[%s]%s下单失败 %v", rs.ExchangeName, v.ClientID, handlerErr)
		}
	}

	if err == nil && handlerErr == nil {
		rs.TakerOrderPass.Update(time.Now().UnixMicro(), start)
	}
}

// cancelOrderID 通过 oid 撤单
// https://bitgetlimited.github.io/apidoc/zh/spot/#v2-4
// 限速规则 10次/1s
func (rs *BitgetSpot) cancelOrderID(info *helper.ExchangeInfo, oid string, tsns int64) {
	uri := "/api/spot/v1/trade/cancel-order-v2"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["orderId"] = oid

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, tsns/1e3)
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

// cancelClientID 通过 cid 撤单
// https://bitgetlimited.github.io/apidoc/zh/spot/#v2-4
// 限速规则 10次/1s
func (rs *BitgetSpot) cancelClientID(info *helper.ExchangeInfo, cid string, tsns int64) {
	uri := "/api/spot/v1/trade/cancel-order-v2"
	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["clientOid"] = cid

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, tsns/1e3)
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

// cancelOrderIDs 通过 cid 批量撤单
// https://bitgetlimited.github.io/apidoc/zh/spot/#v2-5
// 限速规则 10次/1s
func (rs *BitgetSpot) cancelOrderIDs(info *helper.ExchangeInfo, cancelList []string, tsns int64) {
	uri := "/api/spot/v1/trade/cancel-batch-orders-v2"

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["orderIds"] = cancelList

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, tsns/1e3)

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

// cancelClientIDs 通过 oid 批量撤单
// https://bitgetlimited.github.io/apidoc/zh/spot/#v2-5
// 限速规则 10次/1s
func (rs *BitgetSpot) cancelClientIDs(info *helper.ExchangeInfo, cids []string, tsns int64) {
	uri := "/api/spot/v1/trade/cancel-batch-orders-v2"

	params := make(map[string]interface{})
	params["symbol"] = info.Symbol
	params["clientOids"] = cids

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	start := time.Now().UnixMicro()
	rs.SystemPass.Update(start, tsns/1e3)

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

// checkOrder 通过 oid
// https://bitgetlimited.github.io/apidoc/zh/spot/#f876a4e1f9
// 限速规则 20次/1s
func (rs *BitgetSpot) checkOrder(pair helper.Pair, cid, oid string) {
	uri := "/api/spot/v1/trade/orderInfo"
	params := make(map[string]interface{})
	// opt 优化查找
	params["symbol"] = rs.PairToSymbol(&pair)
	if oid != "" {
		params["orderId"] = oid
	} else {
		params["clientOrderId"] = cid
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

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
		datas := value.GetArray("data")
		for _, data := range datas {
			// 查到订单
			event := helper.OrderEvent{Pair: pair}
			event.OrderID = string(data.GetStringBytes("orderId"))
			event.ClientID = string(data.GetStringBytes("clientOrderId"))
			status := helper.BytesToString(data.GetStringBytes("status"))
			switch status {
			case "init", "new", "partial_fill":
				event.Type = helper.OrderEventTypeNEW
			case "full_fill", "cancelled":
				event.Type = helper.OrderEventTypeREMOVE
				event.Filled = fixed.NewS(string(helper.MustGetStringBytes(data, "fillQuantity")))
				if event.Filled.GreaterThan(fixed.ZERO) {
					event.FilledPrice = helper.BytesToFloat64(helper.MustGetStringBytes(data, "fillPrice"))
				}
			}
			side := helper.MustGetShadowStringFromBytes(data, "side")
			switch side {
			case "buy":
				event.OrderSide = helper.OrderSideKD
			case "sell":
				event.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %s", side)
				return
			}
			orderType := helper.MustGetShadowStringFromBytes(data, "orderType")
			switch orderType {
			case "limit":
				event.OrderType = helper.OrderTypeLimit
			case "market":
				event.OrderType = helper.OrderTypeMarket
			}
			// 文档没有force
			// 必须在type后面。忽略其他类型
			// switch helper.MustGetShadowStringFromBytes(data, "timeInForce") {
			// case "IOC":
			// event.OrderType = helper.OrderTypeIoc
			// }
			rs.Cb.OnOrder(0, event)
		}
	})

	if err != nil {
		//得检查是否有限频提示
		rs.Logger.Errorf("checkOrderID err, %v", err)
	}
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
func (rs *BitgetSpot) call(reqMethod string, reqUrl string, params map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler) error {
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

func (rs *BitgetSpot) GetExName() string {
	return helper.BrokernameBitgetSpot.String()
}

func (rs *BitgetSpot) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
	acct, err := rs.DoGetAcctSum()
	if !err.Nil() {
		return nil, err
	}
	for _, b := range acct.Balances {
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

func (rs *BitgetSpot) PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool {
	info, ok := rs.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		rs.Logger.Errorf("failed to get symbol info. %s", symbol)
		return false
	}

	cid := fmt.Sprintf("99%d", uint32(time.Now().UnixMilli()))

	price := ticker.Bp.Load() * 0.99
	rs.placeOrder(info, price, orderAmount, cid, orderSide, helper.OrderTypeLimit, 0)
	return true
}

func (rs *BitgetSpot) GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError) {
	return rs.GetOrderListInFather(rs, startTimeMs, endTimeMs, orderState)
}

// todo 没有历史订单？
func (rs *BitgetSpot) DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError) {
	const _LEN = 100 //交易所对该字段最大约束为100
	uri := "/api/spot/v1/trade/open-orders"
	params := make(map[string]interface{})
	// params["productType"] = "umcbl"
	// params["pageSize"] = _LEN
	// params["startTime"] = startTimeMs
	// params["endTime"] = endTimeMs
	//params["symbol"] = b.Symbol

	err.NetworkError = rs.call(http.MethodPost, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("handler error %v", err)
			return
		}
		// if !rs.isOkApiResponse(value, uri, params) {
		// 	err.HandlerError = fmt.Errorf("handler error %v", value)
		// 	return
		// }
		//dataTemp := value.GetArray("data")
		orders := value.GetArray("data")
		resp.Orders = make([]helper.OrderForList, 0, len(orders))
		if len(orders) >= _LEN {
			resp.HasMore = true
		}

		for _, v := range orders {
			order := helper.OrderForList{
				OrderID:       fmt.Sprintf("%d", helper.MustGetInt64(v, "orderId")),
				ClientID:      helper.MustGetStringFromBytes(v, "clientOrderId"),
				Price:         helper.GetFloat64FromBytes(v, "price"),
				Amount:        fixed.NewS(helper.GetShadowStringFromBytes(v, "size")),
				CreatedTimeMs: helper.GetInt64(v, "time"),
				// FinishedTime: ,
				Filled:      fixed.NewS(helper.GetShadowStringFromBytes(v, "fillQuantity")),
				FilledPrice: helper.GetFloat64FromBytes(v, "fillPrice"),
			}
			status := helper.MustGetShadowStringFromBytes(v, "status")

			switch status {
			case "new", "init":
				order.OrderState = helper.OrderStatePending
			case "filled", "canceled", "partially_filled":
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
			case "market":
				order.OrderType = helper.OrderTypeMarket
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
func (rs *BitgetSpot) GetFee() (fee helper.Fee, err helper.ApiError) {
	uri := "/api/user/v1/fee/query"
	params := make(map[string]interface{})
	params["symbol"] = rs.Symbol
	params["business"] = "spot"

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

		data := value.Get("data")

		fee.Maker = helper.MustGetFloat64FromBytes(data, "makerRate")
		fee.Taker = helper.MustGetFloat64FromBytes(data, "takerRate")
	})
	return
}

func (rs *BitgetSpot) GetFundingRate() (helper.FundingRate, error) {
	return helper.FundingRate{}, nil
}
func (rs *BitgetSpot) DoGetAcctSum() (a helper.AcctSum, err helper.ApiError) {
	a.Lock.Lock()
	defer a.Lock.Unlock()
	a.Balances, err = rs.getEquity()
	return
}

func (rs *BitgetSpot) getEquity() (resp []helper.BalanceSum, err helper.ApiError) {
	priceMap, err := rs.GetAllTickersKeyedSymbol()
	var url string
	var value *fastjson.Value

	p := handyPool.Get()
	defer handyPool.Put(p)
	url = "/api/spot/v1/account/assets-lite"
	params := make(map[string]interface{})
	// params["coin"] = fmt.Sprintf("%s,%s", b.Pair.Base, b.Pair.Quote)
	if err.NetworkError != nil || err.HandlerError != nil {
		rs.Logger.Errorf("[%s]获取PriceMap %v", rs.ExchangeName, err)
		err.HandlerError = fmt.Errorf("[%s]获取PriceMap %v", rs.ExchangeName, err)
		return
	}
	err.NetworkError = rs.call(http.MethodGet, url, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(string(respBody))
		}
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			rs.Logger.Errorf("[%s]获取账户资产失败 %s", rs.ExchangeName, err.HandlerError.Error())
			return
		}
		if !rs.isOkApiResponse(value, url, params) {
			err.HandlerError = fmt.Errorf("API response not OK for URL: %s", url)
			return
		}
		datas := value.GetArray("data")
		if len(datas) == 0 {
			err.HandlerError = fmt.Errorf("no data found in API response")
			return
		}
		gotEquity := make(map[string]bool)
		for _, data := range datas {
			asset := helper.MustGetStringLowerFromBytes(data, "coinDisplayName")
			price := 0.0
			if strings.EqualFold(asset, rs.Pair.Quote) {
				price = 1
			} else if p, ok := priceMap[rs.PairToSymbol(&helper.Pair{Base: asset, Quote: rs.Pair.Quote})]; ok {
				price = p.Price()
			}
			gotEquity[asset] = true
			available := helper.BytesToFloat64(data.GetStringBytes("available"))
			lock := helper.BytesToFloat64(data.GetStringBytes("lock"))
			frozen := helper.BytesToFloat64(data.GetStringBytes("frozen"))
			// 1. onEquityEvent
			ts := time.Now().UnixNano()
			if e, ok := rs.EquityNewerAndStore(asset, 0, ts, (helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail)); ok {
				e.TotalWithoutUpl = available + lock + frozen
				e.Avail = available
				rs.Cb.OnEquityEvent(0, *e)
			}

			// 2. set tradeMsg
			resp = append(resp, helper.BalanceSum{
				Name:   asset,
				Amount: available + lock + frozen,
				Avail:  available,
				Price:  price,
			})
		}
		rs.CleanOtherEquity(gotEquity, (helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail), rs.Cb.OnEquityEvent)
	})
	if !err.Nil() {
		rs.Logger.Errorf("[%s]获取账户资产失败 %s", rs.ExchangeName, err.Error())
		if rs.Cb.OnDetail != nil {
			rs.Cb.OnDetail(err.Error())
		}
	}
	return
}

// SendSignal 发送信号 关键函数 必须要异步发单
func (rs *BitgetSpot) SendSignal(signals []helper.Signal) {
	lenSigs := len(signals)
	var sigsNewOrder = make([]helper.Signal, 0, lenSigs)
	var sigsCancelOid = make([]helper.Signal, 0, lenSigs)
	var sigsCancelCid = make([]helper.Signal, 0, lenSigs)
	for _, s := range signals {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送信号 %s", s.String())
		}
		if s.Pair.Quote == "" {
			s.Pair = rs.Pair
		}
		switch s.Type {
		case helper.SignalTypeNewOrder:
			sigsNewOrder = append(sigsNewOrder, s)
		case helper.SignalTypeCancelOrder:
			if s.OrderID != "" {
				sigsCancelOid = append(sigsCancelOid, s)
			} else {
				sigsCancelCid = append(sigsCancelCid, s)
			}
		case helper.SignalTypeCheckOrder:
			go rs.checkOrder(s.Pair, s.ClientID, s.OrderID)
		case helper.SignalTypeGetEquity:
			go rs.GetEquity()
		case helper.SignalTypeCancelOne:
			go rs.cancelAllOpenOrders(true)
		case helper.SignalTypeCancelAll:
			go rs.cancelAllOpenOrders(false)
		}
	}
	// 执行信号
	if len(sigsNewOrder) > 1 {
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送批量挂单信号 %v", sigsNewOrder)
		}

		lenNew := len(sigsNewOrder)
		times := lenNew / 10
		if times*10 < lenNew {
			times += 1
		}
		for i := 0; i < times; i++ {
			if len(sigsNewOrder[i*10:]) > 10 {
				go rs.placeOrders(sigsNewOrder[i*10 : (i+1)*10])
			} else {
				go rs.placeOrders(sigsNewOrder[i*10:])
			}
		}
	} else if len(sigsNewOrder) == 1 {
		s := sigsNewOrder[0]
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送挂单信号 %s", s.String())
		}
		info, ok := rs.GetPairInfoByPair(&s.Pair)
		if !ok {
			return
		}
		go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time)
	}

	if len(sigsCancelOid) > 1 {
		// 批量撤单
		inf, ok := rs.GetPairInfoByPair(&sigsCancelOid[0].Pair)
		if !ok {
			rs.Logger.Errorf("not found pairinfo")
			return
		}
		ids := slice.Map[helper.Signal](sigsCancelOid, func(idx int, item helper.Signal) string {
			return item.OrderID
		})
		go rs.cancelOrderIDs(inf, ids, sigsCancelOid[0].Time)
	} else if len(sigsCancelOid) == 1 {
		// 单次撤单
		s := sigsCancelOid[0]
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送撤单信号 %v", s)
		}
		info, ok := rs.GetPairInfoByPair(&s.Pair)
		if !ok {
			rs.Logger.Errorf("not found pairinfo")
			return
		}
		go rs.cancelOrderID(info, s.OrderID, sigsCancelOid[0].Time)
	}

	if len(sigsCancelCid) > 1 {
		// 批量撤单
		inf, ok := rs.GetPairInfoByPair(&sigsCancelCid[0].Pair)
		if !ok {
			rs.Logger.Errorf("not found pairinfo")
			return
		}
		ids := slice.Map[helper.Signal](sigsCancelCid, func(idx int, item helper.Signal) string {
			return item.ClientID
		})
		go rs.cancelClientIDs(inf, ids, sigsCancelCid[0].Time)
	} else if len(sigsCancelCid) == 1 {
		// 单次撤单
		s := sigsCancelCid[0]
		if helper.DEBUGMODE {
			rs.Logger.Debugf("发送撤单信号 %v", s)
		}
		info, ok := rs.GetPairInfoByPair(&s.Pair)
		if !ok {
			rs.Logger.Errorf("not found pairinfo")
			return
		}
		go rs.cancelClientID(info, s.ClientID, sigsCancelCid[0].Time)
	}

}

// beasttransfer withDrawRecord 返回

// 账户提币记录获取
func (rs *BitgetSpot) getWithdraw() transfer.WithDraw {
	uri := "/api/spot/v1/wallet/withdrawal-list"
	after := time.Now().UnixNano() / 1000000
	eightyDaysAgo := time.Now().AddDate(0, -2, -20)
	before := eightyDaysAgo.UnixNano() / 1000000
	params := make(map[string]interface{})
	params["endTime"] = after
	params["startTime"] = before

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value
	var result transfer.WithDraw

	err = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		fmt.Println(value)
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		datas := value.GetArray("data")
		fmt.Println(datas)
		for _, data := range datas {
			id := helper.BytesToString(data.GetStringBytes("id"))
			result.Id = id
			coin := helper.BytesToString(data.GetStringBytes("coin"))
			result.Coin = coin
			amount := helper.BytesToFloat64(data.GetStringBytes("amount"))
			result.Amount = amount
			fee := helper.BytesToString(data.GetStringBytes("fee"))
			result.Fee = fee
			chain := helper.BytesToString(data.GetStringBytes("chain"))
			result.Chain = chain
			toAddress := helper.BytesToString(data.GetStringBytes("toAddress"))
			result.ToAddress = toAddress
			createdTime := helper.BytesToString(data.GetStringBytes("cTime"))
			result.CreatedTime = createdTime
			withdraw := helper.BytesToString(data.GetStringBytes("type"))
			result.Withdraw = withdraw
		}
	})
	if err != nil {
		rs.Logger.Errorf("查询bitget提币失败,返回错误为：%s", err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("查询bitget提币失败,返回错误为：%s", handlerErr.Error())
	}
	return result
}

// 账户充币记录获取
func (rs *BitgetSpot) getDeposit() transfer.WithDraw {
	uri := "/api/spot/v1/wallet/deposit-list"
	after := time.Now().UnixNano() / 1000000
	eightyDaysAgo := time.Now().AddDate(0, -2, -20)
	before := eightyDaysAgo.UnixNano() / 1000000
	params := make(map[string]interface{})
	params["endTime"] = after
	params["startTime"] = before

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value
	var result transfer.WithDraw

	err = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}

		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		datas := value.GetArray("data")
		for _, data := range datas {
			id := helper.BytesToString(data.GetStringBytes("id"))
			result.Id = id
			coin := helper.BytesToString(data.GetStringBytes("coin"))
			result.Coin = coin
			amount := helper.BytesToFloat64(data.GetStringBytes("amount"))
			result.Amount = amount
			fee := helper.BytesToString(data.GetStringBytes("fee"))
			result.Fee = fee
			chain := helper.BytesToString(data.GetStringBytes("chain"))
			result.Chain = chain
			toAddress := helper.BytesToString(data.GetStringBytes("toAddress"))
			result.ToAddress = toAddress
			createdTime := helper.BytesToString(data.GetStringBytes("cTime"))
			result.CreatedTime = createdTime
			withdraw := helper.BytesToString(data.GetStringBytes("type"))
			result.Withdraw = withdraw
		}
	})
	if err != nil {
		rs.Logger.Errorf("查询bitget充币失败,返回错误为：%s", err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("查询bitget充币失败,返回错误为：%s", handlerErr.Error())
	}
	return result
}

// 账户现货划转记录获取
func (rs *BitgetSpot) getTransfer() transfer.WithDraw {
	uri := "/api/spot/v1/account/transferRecords"
	after := time.Now().UnixNano() / 1000000
	eightyDaysAgo := time.Now().AddDate(0, -2, -20)
	before := eightyDaysAgo.UnixNano() / 1000000
	params := make(map[string]interface{})
	params["endTime"] = after
	params["startTime"] = before
	params["coinId"] = 2            //USDT 1:BTC
	params["fromType"] = "exchange" //"usdt_mix" future

	// 必备变量
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value
	var result transfer.WithDraw

	err = rs.call(http.MethodGet, uri, params, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		datas := value.GetArray("data")
		fmt.Println(datas)
		for _, data := range datas {
			id := helper.BytesToString(data.GetStringBytes("transferId"))
			result.Id = id
			coin := helper.BytesToString(data.GetStringBytes("coinDisplayName"))
			result.Coin = coin
			amount := helper.BytesToFloat64(data.GetStringBytes("amount"))
			result.Amount = amount
			fromType := helper.BytesToString(data.GetStringBytes("fromType"))
			result.FromType = fromType
			toType := helper.BytesToString(data.GetStringBytes("toType"))
			result.ToType = toType
			fromSymbol := helper.BytesToString(data.GetStringBytes("fromSymbol"))
			result.FromSymbol = fromSymbol
		}
	})
	if err != nil {
		rs.Logger.Errorf("查询bitget划转coinID%s失败,返回错误为：%s", params["coinId"], err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("查询bitget划转coinID%s失败,返回错误为：%s", params["coinId"], handlerErr.Error())
	}
	return result
}

// Do 发起任意请求 一般用于非交易任务 对时间不敏感
func (rs *BitgetSpot) Do(actType string, params any) (any, error) {
	fmt.Printf("Do() with msg: %v", actType)
	// msgInterface := actType.([]interface{})
	//param := msgInterface[1].(transfer.Parameter)
	// _, ok := msgInterface[0].(transfer.DoActRecordListType)
	t, err := strconv.Atoi(actType)
	if err == nil {
		// actGetList := msgInterface[0].(transfer.DoActRecordListType)
		actGetList := transfer.DoActRecordListType(t)
		switch actGetList {
		case transfer.DoActLGetWithdrawRecord:
			return rs.getWithdraw(), nil
		case transfer.DoActLGetTransferRecord:
			return rs.getTransfer(), nil
		case transfer.DoActLGetDepositRecord:
			return rs.getDeposit(), nil
		}
	}
	return nil, nil
}

// 交易所不具备的能力, 优先级比Include高
func (rs *BitgetSpot) GetExcludeAbilities() base.TypeAbilitySet {
	return base.AbltWsPriReqOrder | base.AbltEquityAvailReducedWhenPendingOrder // 挂单时，ws不会推送余额变化
}

// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES_XXX
func (rs *BitgetSpot) GetIncludeAbilities() base.TypeAbilitySet {
	return base.DEFAULT_ABILITIES_SPOT
}

func (rs *BitgetSpot) GetExchangeInfos() []helper.ExchangeInfo {
	return rs.getExchangeInfo()
}

func (rs *BitgetSpot) WsLogged() bool {
	return false
}

func (rs *BitgetSpot) GetFeatures() base.Features {
	f := base.Features{
		GetTicker:             !tools.HasField(*rs, reflect.TypeOf(base.DummyGetTicker{})),
		UpdateWsTickerWithSeq: true,
		OrderPostonly:         true,
		MultiSymbolOneAcct:    true,
	}
	rs.FillOtherFeatures(rs, &f)
	return f
}
func (rs *BitgetSpot) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	hasPendingOrderBefore = true
	uri := "/api/spot/v1/trade/open-orders"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var handlerErr error
	var err error
	var value *fastjson.Value

	err = rs.call(http.MethodPost, uri, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, handlerErr = p.ParseBytes(respBody)
		if handlerErr != nil {
			return
		}
		code := helper.BytesToString(value.GetStringBytes("code"))
		if code != "00000" {
			handlerErr = errors.New(helper.BytesToString(respBody))
			return
		}
		data := value.GetArray("data")
		if len(data) == 0 {
			hasPendingOrderBefore = false
			return
		}
		for _, item := range data {
			symbol := strings.ToUpper(string(item.GetStringBytes("symbol")))
			if only && symbol != rs.Symbol {
				continue
			}
			rs.DoCancelPendingOrders(symbol)
			time.Sleep(time.Millisecond * 1200 * 1 / 10)
		}
	})
	if err != nil {
		rs.Logger.Errorf("[%s] cancelAllOpenOrders request error: %s", rs.ExchangeName, err.Error())
	}
	if handlerErr != nil {
		rs.Logger.Errorf("[%s] cancelAllOpenOrders handle error: %s", rs.ExchangeName, handlerErr.Error())
	}
	return
}

func (rs *BitgetSpot) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	return rs.DoGetPendingOrders("")
}

// symbol 空表示获取全部
func (rs *BitgetSpot) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	url := "/api/spot/v1/trade/open-orders"
	p := handyPool.Get()
	var value *fastjson.Value

	params := make(map[string]interface{})
	if symbol != "" {
		params["symbol"] = symbol
	}

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
		for _, data := range helper.MustGetArray(value, "data") {
			order := helper.OrderForList{
				Symbol:   helper.MustGetStringFromBytes(data, "symbol"),
				ClientID: helper.MustGetStringFromBytes(data, "clientOrderId"),
				OrderID:  helper.MustGetStringFromBytes(data, "orderId"),
				Price:    helper.MustGetFloat64FromBytes(data, "price"),
				Amount:   fixed.NewS(helper.MustGetShadowStringFromBytes(data, "quantity")),
			}
			order.Filled = fixed.NewS(helper.MustGetShadowStringFromBytes(data, "fillQuantity"))
			order.FilledPrice = helper.MustGetFloat64FromBytes(data, "fillPrice")

			side := helper.GetShadowStringFromBytes(data, "side")
			switch side {
			case "buy":
				order.OrderSide = helper.OrderSideKD
			case "sell":
				order.OrderSide = helper.OrderSideKK
			default:
				rs.Logger.Errorf("side error: %v", side)
				return
			}
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
