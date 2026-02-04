package base

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"actor/broker/client/ws"
	"actor/helper"
	"actor/helper/helper_ding"
	"actor/helper/transfer"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/log"
	"github.com/goex-top/dingding"
)

type PriData interface {
	ObtainPosition() *helper.Pos
	ObtainPositionMap() *cmap.ConcurrentMap[string, *helper.Pos]       // rs/ws返回的是同一个PositionMap
	ObtainEquityMap() *cmap.ConcurrentMap[string, *helper.EquityEvent] // rs/ws返回的不是同一个EquityMap
}

var IsInUPC bool // 是否upc清仓过程中

// utf 控制 start
// utf 控制 end

type Ws interface {
	PriData
	Run()  // 运行ws回测
	Stop() // 关闭
	GetFee() (helper.Fee, helper.ApiError)
	GetPairInfo() *helper.ExchangeInfo // 返回第一个pair的exchangeinfo信息
	UpdateLatestTickerTs(tsns int64)
	OptimistParseBBO(msg []byte, ts int64) bool
}
type WsInner interface {
	DoStop()
}
type WsImitation interface {
	ItsWsImitation()
}
type WsNetinfoExposer interface {
	GetPubWsBindIp() string
}

type Rs interface {
	PriData
	// SendSignal 执行外部传入的交易指令 发送http请求 并根据返回结果更新数据和触发回调
	// 注意，里面的signal必须是同一个symbol
	SendSignal([]helper.Signal)
	// BeforeTrade 交易前调用 做好交易前准备工作
	// 返回开机时是否有漏仓、是否有错， 如果是交易对错误应该马上停机。现货要过滤碎仓
	BeforeTrade(mode helper.HandleMode) (leakedPrev bool, err helper.SystemError)
	// AfterTrade 交易结束时调用 做好交易后收尾工作
	AfterTrade(mode helper.HandleMode) (isLeft bool, err helper.SystemError)
	Stop() // 关闭
	// GetDelay 获取延迟信息 依次为 挂单延迟 撤单延迟 系统延迟 (单位都是微秒)
	GetDelay() (int64, int64, int64)
	// GetFee 获取费率情况 从order推送中计算真实费率 taker maker
	GetFee() (helper.Fee, helper.ApiError)
	// orderState表示查询状态, OrderStateAll则是查询所有.
	// 错误时返回err.Nil() != false
	GetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, err helper.ApiError)
	GetDealList(startTimeMs int64, endTimeMs int64) (resp []helper.DealForList, err helper.ApiError)
	GetPosHist(startTimeMs int64, endTimeMs int64) (resp []helper.Positionhistory, err helper.ApiError)
	// GetAcctSum 获取账户概览信息 包含持币和持仓信息 持仓要返回所有币对持仓
	// Position 负数和side表示方向（兼容旧逻辑）；乘上multi
	GetAcctSum() (helper.AcctSum, helper.ApiError) // todo 零仓位是否需要返回？ kc@2023年11月1日
	GetFundingRate() (helper.FundingRate, error)
	// Do 发起任意请求 一般用于非交易任务 对时间不敏感
	Do(actType string, params any) (resp any, err error)
	GetPairInfo() *helper.ExchangeInfo
	GetPairInfoByPair(*helper.Pair) (*helper.ExchangeInfo, bool)
	// todo refactor
	GetExchangeInfos() []helper.ExchangeInfo
	GetLabileExchangeInfos() cmap.ConcurrentMap[string, *helper.LabileExchangeInfo] // key 是pair.String()
	GetLastFewErrors() []string
	GetExName() string // 返回 binance_usdt_swap 类似
	// 设置交易对杠杆，pair为空表示主交易对
	CancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool)
	RsAbility
	RsGetTicker
	// SetLeverage(pair string, leverage int)
	// SetMarginMode(pair string, marginMode helper.MarginMode) helper.ApiError
	// SetPositionMode(pair string, positionMode helper.PosMode) helper.ApiError
	GetAccountMode(pair string) (int, helper.MarginMode, helper.PosMode, helper.ApiError)
	SetAccountMode(pair string, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) helper.ApiError
	SwitchLine(action ActionType, line SelectedLine) (err error, changed bool)
	GetSelectedLine(action ActionType) (line SelectedLine)
	AddRsStopHandler(func(rs Rs))
	GetFeatures() Features
	// 撤销全部挂单
	CancelPendingOrders(pair *helper.Pair) helper.ApiError
	GetPriceLimit(pair *helper.Pair) (helper.PriceLimit, helper.ApiError)
	GetPendingOrders(pair *helper.Pair) (resp []helper.OrderForList, err helper.ApiError)
	// 获取账户级别下同大类(计价币)的挂单
	GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError)
	GetDepthByPair(pair string) (respDepth helper.Depth, err helper.ApiError)
	GetOIByPair(pair string) (oi float64, err helper.ApiError)
	CleanPosInFather(maxValueClosePerTimes float64, rs Rs, rsInner RsInner, only bool) bool
	PairToSymbol(pair *helper.Pair) string
	PairStrToSymbol(pairStr string) (string, error)
	SymbolToPair(symbol string) (*helper.Pair, error)
	RsGetPriWs
	ReqSucc(FailNumActionIdx_Type)
	GetFacilities() int64

	RsMarket
}

type RsGetPriWs interface {
	GetPriWs() *ws.WS
}

// todo event 迁入 Rs interface
type RsDelayMonitor interface {
	GetDelayMonitor() *DelayMonitor
}
type RsGetAllIndexFundingRate interface {
	GetFundingRates(pairs []string) (res map[string]helper.FundingRate, err helper.ApiError)
	GetIndexs(pairs []string) (res map[string]float64, err helper.ApiError)
}
type RsPreSign interface {
	// 获取预签名信息
	GetPreSignInfo(helper.Signal) (map[string]interface{}, error)
	PlaceSignedOrder(map[string]interface{})
}
type RsExposerOuter interface {
	RsExposerOuterSpot
	// 只返回主交易对的仓位，不需要全部交易对。amount带有multi，与GetOrigPositions不同
	GetPosition() (resp []helper.PositionSum, err helper.ApiError)
}
type RsExposerOuterSpot interface {
	GetEquity() (resp helper.Equity, err helper.ApiError)
}

func EnsureIsRsExposer(rs RsExposer)                   {}
func EnsureIsRsExposerOuter(rs RsExposerOuter)         {}
func EnsureIsRsExposerOuterSpot(rs RsExposerOuterSpot) {}

// ！！！ 不可随意改名，有运行时反射依赖名字，无法检测
type RsOrderAction interface {
	DoPlaceOrderRsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoPlaceOrderRsColo(info *helper.ExchangeInfo, s helper.Signal)
	DoPlaceOrderWsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoPlaceOrderWsColo(info *helper.ExchangeInfo, s helper.Signal)

	DoCancelOrderRsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoCancelOrderRsColo(info *helper.ExchangeInfo, s helper.Signal)
	DoCancelOrderWsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoCancelOrderWsColo(info *helper.ExchangeInfo, s helper.Signal)

	DoAmendOrderRsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoAmendOrderRsColo(info *helper.ExchangeInfo, s helper.Signal)
	DoAmendOrderWsNor(info *helper.ExchangeInfo, s helper.Signal)
	DoAmendOrderWsColo(info *helper.ExchangeInfo, s helper.Signal)
}
type RsBatchOrderAction interface {
	DoPlaceBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoPlaceBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoPlaceBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoPlaceBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal)

	DoCancelBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoCancelBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoCancelBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoCancelBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal)

	DoAmendBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoAmendBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoAmendBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal)
	DoAmendBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal)
}

// bq 内部各模块之间用的接口，不对外使用
type RsInner interface {
	// 获取时间范围内的订单，当没有取完整的时候，resp.HasMore必须置为true，上层会递归获取
	DoGetOrderList(startTimeMs int64, endTimeMs int64, orderState helper.OrderState) (resp helper.OrderListResponse, err helper.ApiError)
	DoGetDealList(startTimeMs int64, endTimeMs int64) (resp helper.DealListResponse, err helper.ApiError)
	DoGetPosHist(startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError)
	//
	DoGetAcctSum() (acctSum helper.AcctSum, err helper.ApiError)
	DoSetLeverage(pairInfo helper.ExchangeInfo, leverage int) (err helper.ApiError)
	GetExchangeInfos() []helper.ExchangeInfo
	GetPairInfo() *helper.ExchangeInfo
	// 获取该账户全部仓位。 positionsum：side 表示方向；amount都是正，没有multi
	GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError)
	PlaceCloseOrder(symbol string, orderSide helper.OrderSide, orderAmount fixed.Fixed, posMode helper.PosMode, marginMode helper.MarginMode, ticker helper.Ticker) bool
	// SymbolMode()
	RsGetTicker
	DoGetAccountMode(pairInfo *helper.ExchangeInfo) (int, helper.MarginMode, helper.PosMode, helper.ApiError)
	DoSetMarginMode(symbol string, marginMode helper.MarginMode) helper.ApiError
	DoSetPositionMode(symbol string, positionMode helper.PosMode) helper.ApiError
	DoCancelOrdersIfPresent(bool) bool
	DoCreateReqWsNor() error
	DoCreateReqWsColo() error
	DoStop()
	RsOrderAction
	RsBatchOrderAction
	DoCancelPendingOrders(symbol string) helper.ApiError
	DoGetPriceLimit(symbol string) (pl helper.PriceLimit, err helper.ApiError)
	DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError)
	DoGetDepth(info *helper.ExchangeInfo) (depth helper.Depth, err helper.ApiError)
	DoGetOI(info *helper.ExchangeInfo) (oi float64, err helper.ApiError)
}

type SwapRsTransfer interface {
	// 子账户与子账户之间的合约划转
	DoTransferSubInner(params transfer.TransferParams) helper.ApiError
	// 主账户和子账户之间的合约转账
	DoTransferSub(params transfer.TransferParams) helper.ApiError
	// 账户内万向划转
	DoTransferAllDireaction(params transfer.TransferAllDirectionParams) helper.ApiError
	// 获取子账户合约的余额
	GetBalance(params transfer.GetBalanceParams) (float64, helper.ApiError)
	// 获取母账户合约余额
	GetMainBalance(params transfer.GetMainBalanceParams) (float64, helper.ApiError)
	//获取总资产
	DoGetAcctSum() (acctSum helper.AcctSum, err helper.ApiError)

	GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError)
}

type RsTransfer interface {
	// 获取子账户列表
	GetSubList() ([]transfer.AccountInfo, helper.ApiError)
	// 子账户与子账户之间的现货划转
	DoTransferSubInner(params transfer.TransferParams) helper.ApiError
	// 主账户与子账户之间的现货划转
	DoTransferSub(params transfer.TransferParams) helper.ApiError
	// 账户内万向划转
	DoTransferAllDireaction(params transfer.TransferAllDirectionParams) helper.ApiError
	// 主账户合约现货之间的划转
	DoTransferMain(params transfer.TransferParams) helper.ApiError
	// 创建子账户
	CreateSubAcct(params transfer.CreateSubAcctParams) ([]transfer.AccountInfo, helper.ApiError)
	// 创建子账户API Key
	CreateSubAPI(params transfer.APIOperateParams) (transfer.Api, helper.ApiError)
	// 修改子账户API Key
	ModifySubAPI(params transfer.APIOperateParams) (transfer.Api, helper.ApiError)
	// 获取子账户现货的余额
	GetBalance(params transfer.GetBalanceParams) (float64, helper.ApiError)
	// 获取母账户现货余额
	GetMainBalance(params transfer.GetMainBalanceParams) (float64, helper.ApiError)
	//获取总资产
	DoGetAcctSum() (acctSum helper.AcctSum, err helper.ApiError)
	GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError)
	//获取erc20 trc20 地址
	GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError)
	// 提币
	WithDraw(param transfer.WithDraw) (restlt string, err helper.ApiError)
	// 提币记录
	WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError)
}

type OrangeTransfer interface {
	TransferSpotToWallet(coin string, amt float64) (err helper.ApiError)
	GetWalletEquity() (balanceSum []helper.BalanceSum, err helper.ApiError)
}

type RsInnerSetAccountMode interface {
	DoSetAccountMode(pairInfo *helper.ExchangeInfo, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) helper.ApiError
}

type RsTransferMarginToPair interface {
	TransferMarginToPair(symbol string, margin float64) helper.ApiError
}
type ClientUTF interface {
	GetPosBySymbol(symbol string) (pos *helper.Pos, ok bool)
	GetDefaultPair() helper.Pair
}

type CautionIntf interface {
	GetCautions() (resp []helper.Caution)
}

type RsInnerOneClickCloser interface {
	ClosePosOneClick(symbol string, positionId string)
}

func EnsureIsRsInnerOneClickCloser(rs RsInnerOneClickCloser) {}

type RsMarket interface {
	GetCandles(bar string) (resp []*helper.Kline, err helper.ApiError)
}

type RsGetTicker interface {
	// 返回的量是交易所原始数据，通过exchange info的 mutli \ amount unit type信息计算
	GetTickerBySymbol(symbol string) (ticker helper.Ticker, err helper.ApiError)
	// todo 不应在ex层实现，转换逻辑放在father层
	GetTickerByPair(pair *helper.Pair) (ticker helper.Ticker, err helper.ApiError)
}
type RsDepth interface {
	GetDepthByPair(pair string) (respDepth helper.Depth)
}
type RsMidFreqUM interface {
	// Repay()
	PlaceOrderSpotUsdtSize(pair helper.Pair, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, t int64)
	CancelSpotPendingOrder(pair helper.Pair) (err helper.ApiError)
	GetMMR() helper.UMAcctInfo
	Spot7DayTradeHist() (resp []helper.DealForList, err helper.ApiError)
}
type RsMultiAssetCollateral interface {
	SetCollateral(coin string) bool
}

type RsTransferableAmount interface {
	GetTransferableAmount() (transferable float64, err helper.ApiError)
}

type RsGetAllTickers interface {
	// map的key是symbol
	GetAllTickersKeyedSymbol() (tickers map[string]helper.Ticker, err helper.ApiError)
}

// 增加一些接口，避免修改现有ex代码和方便统一测试框架使用，先独立。全部ex实现后再合并到Rs interface
type RsExposer interface {
	// GetTicker()
	RsGetTicker
	// GetTicker(symbol string) (ticker helper.Ticker, err helper.ApiError)
	PairToSymbol(pair *helper.Pair) string
	WsLogged() bool
	GetPairInfo() *helper.ExchangeInfo
	// GetFullSymbolsPositions([]helper.Pos, error)
	// GetFullSymbolsPendingOrders([]helper.Order, error)
}

type WsExposer interface {
	WsLogged() bool
}

type WsExposerReconnect interface {
	GetPriWs() *ws.WS
	GetPubWs() *ws.WS
}

var GitCommitHash string
var AppName string
var BuildTime string
var InBeastMarket string

// 交易所可能在全平仓后，资金余额没有及时计算，这里用2秒间隔判断
func CheckBalanceIsStable(rs Rs, ignoreAvail ...bool) bool {
	time.Sleep(time.Second * 2)
	lastAcct, err := rs.GetAcctSum()
	log.Infof("cur acct. bal: %v, pos: %v", lastAcct.Balances, lastAcct.Positions)
	ignoreAvail0 := false
	if len(ignoreAvail) > 0 {
		ignoreAvail0 = ignoreAvail[0]
	}
	for tried := 0; tried < 5; tried++ {
		time.Sleep(time.Second * 2)
		var curAcct helper.AcctSum
		curAcct, err = rs.GetAcctSum()
		log.Infof("cur acct. bal: %v, pos: %v", curAcct.Balances, curAcct.Positions)
		if len(curAcct.Balances) != len(lastAcct.Balances) {
			lastAcct = curAcct
			continue
		}
		allEqual := true
		// 考虑多币种
		for _, b0 := range lastAcct.Balances {
			equal := false
			for _, b1 := range curAcct.Balances {
				if b1.Name == b0.Name {
					// if math.Abs(b1.Amount-b0.Amount) < 0.0000001 && math.Abs(b1.Avail-b0.Avail) < 0.0000001 {
					if math.Abs(b1.Amount-b0.Amount) < 0.0000001 && (ignoreAvail0 || math.Abs(b1.Avail-b0.Avail) < 0.0000001) {
						equal = true
						break
					}
					// 避免除0
					if math.Abs(b1.Amount-b0.Amount) <= (b1.Amount+b0.Amount)*0.002 && (ignoreAvail0 || math.Abs(b1.Avail-b0.Avail) <= (b1.Avail+b0.Avail)*0.002) {
						equal = true
						break
					}
				}
			}
			if !equal {
				allEqual = false
				break
			}
		}
		if !allEqual {
			lastAcct = curAcct
			continue
		} else if err.Nil() {
			log.Infof("余额已经稳定. %v, %v", curAcct.Balances, curAcct.Positions)
			return true
		}
	}
	log.Errorf("余额一直无法稳定. %v, %v", lastAcct.Balances, lastAcct.Positions)
	helper_ding.DingingSendSerious(fmt.Sprintf("余额一直无法稳定. %v, %v", lastAcct.Balances, lastAcct.Positions))
	return false
}

func CheckPositionAndOrderClosed(rs Rs, only bool) (isLeft bool) {
	log.Infof("checking CheckPositionAndOrderClosed")
	var curAcct helper.AcctSum
	var err helper.ApiError
	for tried := 0; tried < 5; tried++ {
		time.Sleep(time.Second * 2)
		curAcct, err = rs.GetAcctSum()
		if !err.Nil() {
			continue
		}
		log.Infof("cur acct. bal: %v, pos: %v", curAcct.Balances, curAcct.Positions)

		allClosed := true
		for _, p := range curAcct.Positions {
			if p.Amount != 0 || p.AvailAmount != 0 {
				if only && p.Name != rs.GetPairInfo().Pair.String() {
					continue
				}
				if strings.Contains(rs.GetExName(), "bitget_usdt_swap") {
					pair := helper.NewPair(strings.Split(p.Name, "_")[0], strings.Split(p.Name, "_")[1], "")
					tick, _ := rs.GetTickerByPair(&pair)
					info, ok := rs.GetPairInfoByPair(&pair)
					if p.Amount*tick.Price() < 10 || (ok && (p.Amount*tick.Price() < info.MinOrderValue.Float() || p.Amount < info.MinOrderAmount.Float())) {
						continue
					}
				}
				allClosed = false
				log.Warnf("仓位未归零： %v", curAcct)
				break
			}
		}
		if rs.CancelOrdersIfPresent(only) {
			log.Warnf("挂单未归零")
			allClosed = false
		}
		if !allClosed {
			continue
		} else {
			log.Infof("仓位挂单已归零. %v %v", curAcct.Balances, curAcct.Positions)
			isLeft = false
			return
		}
	}
	log.Errorf("仓位或挂单一直无法归零. %v, %v, err: %v", curAcct.Balances, curAcct.Positions, err)
	helper_ding.DingingSendSerious(fmt.Sprintf("仓位或挂单一直无法归零. %v, %v", curAcct.Balances, curAcct.Positions))
	isLeft = true

	var leftVal float64
	for _, p := range curAcct.Positions {
		leftVal += math.Abs(p.Amount) * p.Ave
	}

	if leftVal >= 500 {
		d := dingding.NewDing("")
		title := "警报-严重 存在过大遗留仓位"
		msg := title + fmt.Sprintf("\n\n[Leftval:%f][%s][%s][%s]\n\n", leftVal, AppName, strings.Split(helper.RobotId, "-")[0], rs.GetExName())
		for _, p := range curAcct.Positions {
			msg += fmt.Sprintf("%s: %f@%f\n\n", p.Name, p.Amount, p.Ave)
		}
		d.SendMarkdown(dingding.Markdown{
			Content:  msg,
			Title:    title,
			AtPerson: nil,
			AtAll:    false,
		})
	}

	return
}

func PosComapreAndChanged(t *helper.Pos, longPos, shortPos fixed.Fixed, longAvg, shortAvg float64) bool {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	if t.LongPos.Equal(longPos) && t.ShortPos.Equal(shortPos) && t.LongAvg == longAvg && t.ShortAvg == shortAvg {
		return false
	}
	t.LongPos = longPos
	t.ShortPos = shortPos
	t.LongAvg = longAvg
	t.ShortAvg = shortAvg
	return true
}

// 按照梯队排序
func SortExNames(exNamesOri []string) []string {
	_TIER1_BROKERS := []string{
		"binance", "okx", "bybit",
	}
	_TIER2_BROKERS := []string{
		"bitget", "kucoin", "gate", "huobi", "coinex",
	}
	type T struct {
		name string
		val  int
	}
	exNames := make([]T, 0, len(exNamesOri)) // 排序

	for i, exName := range exNamesOri {
		if strings.Index(exName, "unknown") >= 0 {
			continue
		}
		val := len(exNames) + 1 - int(i)
		for _, broker := range _TIER1_BROKERS {
			if strings.Index(exName, broker) >= 0 {
				val += 200
				break
			}
		}
		for _, broker := range _TIER2_BROKERS {
			if strings.Index(exName, broker) >= 0 {
				val += 100
				break
			}
		}
		if strings.Index(exName, "swap") >= 0 {
			val *= 1000
		}
		t := T{name: exName, val: val}
		exNames = append(exNames, t)
	}
	// exNames 按val反向排序
	sort.Slice(exNames, func(i, j int) bool {
		return exNames[i].val > exNames[j].val
	})
	res :=
		make([]string, 0, len(exNames))
	for _, t := range exNames {
		res = append(res, t.name)
	}
	return res
}

type DepthType int

const (
	DepthTypeSnapshot DepthType = iota + 1 // 全量深度
	DepthTypePartial                       // 增量深度
)
