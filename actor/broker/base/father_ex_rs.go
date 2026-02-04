package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.uber.org/atomic"

	jsoniter "github.com/json-iterator/go"

	"actor/broker/brokerconfig"
	"actor/broker/client/ws"
	"actor/config"
	"actor/helper"
	"actor/helper/helper_ding"
	"actor/push"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/gcircularqueue_generic"
	"actor/third/log"
	"actor/tools"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/go-redis/redis/v8"
)

// 相当于oop里面的抽象类，用于抽取公共逻辑，适用于低频、重复、严格的场景，例如清仓、exchangeInfo等。
type FatherRs struct {
	// 实现一个所时，注释这两个Dummy
	FatherCommon
	DummyCreateReqWs
	DummyDoGetPendingOrders
	DummyGetAllPendingOrders
	Features   Features
	upcLock    sync.Mutex
	subRsInner RsInner
	subRs      Rs
	// 微秒单位
	TakerOrderPass  helper.PassTime
	AmendOrderPass  helper.PassTime
	CancelOrderPass helper.PassTime
	SystemPass      helper.PassTime
	IsInBeforeTrade bool
	LastFewErrors   *gcircularqueue_generic.CircularQueueThreadSafe[string] // 最后几条错误提示，方便交易员定位问题
	//
	DelayMonitor DelayMonitor
	// SelectedLines cmap.ConcurrentMap[ActionType, *SelectedLine]
	SelectedLine_Place  SelectedLine
	SelectedLine_Amend  SelectedLine
	SelectedLine_Cancel SelectedLine
	ReqWsNor            *ws.WS // 继承ws
	ReqWsNorLogged      atomic.Bool
	ReqWsColo           *ws.WS // 继承ws
	ReqWsColoLogged     atomic.Bool

	stopHandlers []func(Rs)
	bfafCallCnt  atomic.Int32 // 计数器， BeforeTrade AfterTrade 要求调用顺序一一对应
}

func InitFatherRs(msg *helper.TradeMsg, subRs Rs, subRsInner RsInner, rs *FatherRs, params *helper.BrokerConfigExt, info *helper.ExchangeInfo, cb helper.CallbackFunc) {
	rs.subRs = subRs
	rs.subRsInner = subRsInner
	rs.IsRs = true
	rs.stopHandlers = make([]func(Rs), 0)
	// rs.SelectedLines = cmap.NewStringer[ActionType, *SelectedLine]()
	// for _, a := range []ActionType{ActionType_Place, ActionType_Cancel} {
	// rs.SelectedLines.Set(a, &SelectedLine{})
	// }

	rs.InitCommon(params, info, cb, msg)
	rs.LastFewErrors = gcircularqueue_generic.NewCircularQueueThreadSafe[string](5)
	rs.Features = subRs.GetFeatures()
}

// 所有其他逻辑依赖这个顺序调用逻辑，基础中的基础。
// 不符合就当严重错误，提前发现问题，不要重试
func (f *FatherRs) EnsureCanRun() helper.SystemError {
	_, fn, _ := helper.GetCallerInfo(2)
	if helper.DEBUGMODE {
		log.RootLogger.Debugf("in EnsureCanRun, caller is: %s", fn)
	}
	switch fn {
	case "BeforeTrade":
		if helper.DEBUGMODE {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			f.Logger.Debugf("==> BeforeTrade stacktrace. %s\n", string(buf[:n]))
		}
		r := f.bfafCallCnt.CompareAndSwap(0, 1)
		if !r {
			msg := "必须在AfterTrade之前调用BeforeTrade"
			helper.AlerterSystemOwner.Push("bfaf调用顺序错乱", fmt.Sprintf("func type. %s", fn))
			// push.PushAlert("bfaf调用顺序错乱", fmt.Sprintf("func type. %s", fn))
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			f.Logger.Errorf("==> BeforeTrade wrong stacktrace. %s\n", string(buf[:n]))
			helper.LogErrorThenCall(msg, f.Cb.OnExit)
			if helper.DEBUGMODE && IsUtf {
				panic(msg)
			}
			return helper.SystemErrorBtAfOrder
		}
	case "AfterTrade":
		r := f.bfafCallCnt.CompareAndSwap(1, 0)
		if helper.DEBUGMODE {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			f.Logger.Debugf("==> AfterTrade stacktrace. %s\n", string(buf[:n]))
		}
		if !r {
			msg := "必须在BeforeTrade之后调用AfterTrade"
			helper.AlerterSystemOwner.Push("bfaf调用顺序错乱", fmt.Sprintf("func type. %s", fn))
			// push.PushAlert("bfaf调用顺序错乱", fmt.Sprintf("func type. %s", fn))
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			f.Logger.Errorf("==> AfterTrade wrong stacktrace. %s\n", string(buf[:n]))
			helper.LogErrorThenCall(msg, f.Cb.OnExit)
			if helper.DEBUGMODE && IsUtf {
				panic(msg)
			}
			return helper.SystemErrorBtAfOrder
		}
	}
	return helper.SystemErrorNil
}

func (b *FatherRs) CancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	return b.subRsInner.DoCancelOrdersIfPresent(only)
}

//	func (b *FatherRs) GetPairInfoMap() map[string]helper.ExchangeInfo {
//		return b.ExchangeInfoS2P
//	}
// func (rs *FatherRs) GetOrigPositions() (resp []helper.PositionSum, err helper.ApiError) {
// err = helper.ApiErrorNotImplemented
// log.Errorf("not implemented GetOrigPosition")
// return
// }

// 用于计算路由权重, 匹配越多，权重越高
type LineFactors struct {
	Zone string
	Uid  string // 账户id
}

func (f *LineFactors) Set(key, val string) {
	switch strings.ToLower(key) {
	case "zone":
		f.Zone = val
	case "uid":
		f.Uid = val
	}
}

// 根据本机、账户信息计算匹配权重
func CalcWeight(f0 LineFactors) (w int) {
	b, err := json.Marshal(f0)
	if err != nil {
		log.Error(err)
		return
	}
	m := make(map[string]string)
	err = json.Unmarshal(b, &m)
	if err != nil {
		log.Error(err)
		return
	}

	machineInfo := brokerconfig.LoadMachineInfo()
	// kv like : zone:aws-xxx;uid:28347;
	for k, v := range m {
		factor := strings.ToLower(k)
		switch factor {
		case "x_weight":
			v0, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				log.Errorf("x_weight 格式错误. %v, %v", factor, v)
				return 0
			}
			w += v0
		case "zone":
			// 创建正则判断匹配. 正则匹配、前缀匹配、完全匹配都会成功
			matched, err := regexp.MatchString(v, machineInfo.Zone)
			if err != nil {
				log.Error(err)
				return 0
			}
			if matched { // 可能是正则匹配
				w++
				if v == machineInfo.Zone { // 完全匹配，再+1
					w++
				}
			} else {
				log.Debugf("factor not match, return 0 weight. %v, %v", factor, v)
				return 0
			}
		}
	}

	return
}
func (f *FatherRs) TryFetchFromRedis(pair string) (info helper.ExchangeInfo, ok bool) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.REDIS_ADDR,
		Password: config.REDIS_PWD,
		// 仅适用exchangeInfo场景，其他不一定适合
		PoolSize:     2,
		MinIdleConns: 1,
		DialTimeout:  time.Second * 2,
		ReadTimeout:  time.Second * 2,
		WriteTimeout: time.Second * 2,
	})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	contentKey := fmt.Sprintf("exchangeinfo_%s_%s", f.ExchangeName, pair)
	val, err := client.Get(ctx, contentKey).Result()
	if err != nil {
		f.Logger.Errorf("failed to get manual exchangeinfo from redis %s , err: %v", contentKey, err)
		return info, false
	}
	err = json.Unmarshal([]byte(val), &info)
	if err != nil {
		f.Logger.Errorf("failed to unmarshal exchangeinfo from redis %s , err: %v", contentKey, err)
		return info, false
	}
	f.Logger.Infof("got exchangeinfo from redis %s , info: %v", contentKey, info)
	f.PairInfo = &info
	f.Symbol = info.Symbol
	f.ExchangeInfoPtrP2S.Set(pair, &info)
	f.ExchangeInfoPtrS2P.Set(info.Symbol, &info)
	return info, true
}

// 根据全局设置选择最优链路，必须在ex初始化line之后调用。
func (f *FatherRs) ChoiceBestLine() {
	return
	if helper.DEBUGMODE && IsUtf {
		f.Logger.Infof("in utf, skip choice best line")
		return
	}

	if helper.IsOuterEnv() {
		f.Logger.Infof("in utf, skip choice best line")
		return
	}
	// 创建 Redis 客户端
	// client := redis.NewClusterClient(&redis.ClusterOptions{
	// Addrs:    []string{"ave.gvneuq.clustercfg.memorydb.ap-northeast-1.amazonaws.com:6379", "172.17.15.225:6379"},
	// Password: "",
	// })

	f.Logger.Infof("gonna query best line from redis")
	client := redis.NewClient(&redis.Options{
		Addr:         config.REDIS_ADDR,
		Password:     config.REDIS_PWD,
		PoolSize:     2,
		MinIdleConns: 1,
		DialTimeout:  time.Second * 2,
		ReadTimeout:  time.Second * 2,
		WriteTimeout: time.Second * 2,
	})

	defer client.Close()

	// mi := brokerconfig.LoadMachineInfo()
	// myLineFactors := LineFactors{Zone: mi.Zone}

	contentKey := fmt.Sprintf("bestline_%s", f.subRs.GetExName())

	for tried := 0; tried < 2; tried++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		/*
			field name 由 action + factor组成：action;Uid\Zone\等，like place;zone:aws-xxx;uid:28347.
			支持正则匹配、前缀匹配、完全匹配, 前两者权重低些;
			x_weight factor 是初始化权重，默认为0
			factor要全部包含，否则权重返回0
		*/
		/*
			val 由 line 组成 client, link, like: client:rs;link:colo
		*/

		vals, err := client.HGetAll(ctx, contentKey).Result()
		if err != nil {
			// 除了 key not exist 的其他错误
			log.Errorf("[utf_ign]redis 读取失败. %s, %v", contentKey, err)
			time.Sleep(1 * time.Second)
			continue
		} else {
			f.Logger.Infof("got bestline for %s, %v", contentKey, vals)
			maxWeightLine := make(map[ActionType]*SelectedLine)
			for filed, lineStr := range vals {
				factors := strings.Split(filed, ";")
				action := ActionType(factors[0])
				thisFactors := LineFactors{}
				if len(factors) > 1 {
					for _, f := range factors[1:] {
						v := strings.Split(f, ":")
						fname := strings.TrimSpace(v[0])
						fval := strings.TrimSpace(v[1])
						thisFactors.Set(fname, fval)
					}
				}
				weight := CalcWeight(thisFactors)
				l, ok := maxWeightLine[action]
				if !ok {
					line, err := NewSelectLineFromStr(lineStr)
					if err != nil {
						helper.LogErrorThenCall(fmt.Sprintf("failed to NewSelectLineFromStr, %v, %v", lineStr, err), push.PushAlertCommon)
						continue
					}
					l = &line
					l.Weight = weight
					maxWeightLine[action] = l
				}
				if weight > l.Weight { // 不可=，会非预期覆盖
					line, err := NewSelectLineFromStr(lineStr)
					if err != nil {
						helper.LogErrorThenCall(fmt.Sprintf("failed to NewSelectLineFromStr, %v, %v", lineStr, err), push.PushAlertCommon)
						continue
					}
					maxWeightLine[action] = &line
				}
			}
			if l, ok := maxWeightLine[ActionType_Place]; ok && l.Weight > f.SelectedLine_Place.Weight {
				f.Logger.Infof("use max weight select line for place. %v", l)
				f.SelectedLine_Place = *l
			}
			if l, ok := maxWeightLine[ActionType_Cancel]; ok && l.Weight > f.SelectedLine_Cancel.Weight {
				f.Logger.Infof("use max weight select line for cancel. %v", l)
				f.SelectedLine_Cancel = *l
			}
			if l, ok := maxWeightLine[ActionType_Amend]; ok && l.Weight > f.SelectedLine_Amend.Weight {
				f.Logger.Infof("use max weight select line for amend. %v", l)
				f.SelectedLine_Amend = *l
			}
			break
		}
	}
	// print
	f.Logger.Infof("use best line for place %v", f.SelectedLine_Place)
	f.Logger.Infof("use best line for cancel %v", f.SelectedLine_Cancel)
	f.Logger.Infof("use best line for amend %v", f.SelectedLine_Amend)
}

func (b *FatherRs) GetDelay() (int64, int64, int64) {
	return b.TakerOrderPass.GetDelay(), b.CancelOrderPass.GetDelay(), b.SystemPass.GetDelay()
}

func (rs *FatherRs) GetOIByPair(pair string) (oi float64, err helper.ApiError) {
	info, ok := rs.ExchangeInfoPtrP2S.Get(pair)
	if !ok {
		rs.Logger.Error("not found symbol for pair %s", pair)
		return 0, helper.ApiError{NetworkError: errors.New("not found symbol for pair")}
	}
	return rs.subRsInner.DoGetOI(info)
}
func (rs *FatherRs) GetDepthByPair(pair string) (depth helper.Depth, err helper.ApiError) {
	info, ok := rs.ExchangeInfoPtrP2S.Get(pair)
	if !ok {
		rs.Logger.Error("not found symbol for pair %s", pair)
		return helper.Depth{}, helper.ApiError{NetworkError: errors.New("not found symbol for pair")}
	}
	return rs.subRsInner.DoGetDepth(info)
}
func (rs *FatherRs) GetAcctSum() (acctSum helper.AcctSum, err helper.ApiError) {
	for i := 0; i < 3; i++ {
		acctSum, err = rs.subRsInner.DoGetAcctSum()
		if err.Nil() {
			break
		}
		time.Sleep(time.Second)
	}
	if !err.Nil() {
		log.Errorf("failed to get acct sum, err {}", err.Error())
	}
	return
}
func (b *FatherRs) GetLastFewErrors() []string {
	return b.LastFewErrors.GetAllElements()
}
func (b *FatherRs) PutOneError(msg string) {
	b.LastFewErrors.PushKick(msg)
}

const _MAX_RECURSIVE_LEVEL = 10 // 最大递归层数，防止交易所错误返回

func (f *FatherRs) GetDealList(startTimeMs int64, endTimeMs int64) (resp []helper.DealForList, err helper.ApiError) {
	f.Logger.Errorf("not implemented GetDealList")
	return
}

func (f *FatherRs) DoGetDealList(startTimeMs int64, endTimeMs int64) (resp helper.DealListResponse, err helper.ApiError) {
	f.Logger.Errorf("not implemented DoGetDealList")
	return
}

func (f *FatherRs) DoGetAccountMode(pairInfo *helper.ExchangeInfo) (leverage int, marginMode helper.MarginMode, poMode helper.PosMode, err helper.ApiError) {
	f.Logger.Errorf("not implemented DoGetAccountMode")
	return
}

// 如果有遗漏订单，查看交易所是否没正确设置HasMore=true
func (rs *FatherRs) GetDealListInFather(subRs RsInner, startTimeMs, endTimeMs int64) (resp []helper.DealForList, errRet helper.ApiError) {
	const HOURS = 24

	if startTimeMs+HOURS*3600*1000 < endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("时间范围不能跨越 %d 小时", HOURS)}
	}
	if startTimeMs >= endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("startTime不应大于endTime")}
	}
	orders, err := rs.recurGetDealList(1, subRs, startTimeMs, endTimeMs)
	if !err.Nil() {
		rs.Logger.Errorf("%v", err)
		return nil, err
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].TradeTimeMs < orders[j].TradeTimeMs
	})
	return orders, helper.ApiErrorNil
}

// 递归获取时间范围内的订单列表，直到该时间范围内没有更多订单
func (rs *FatherRs) recurGetDealList(recurLevel int, subRs RsInner, startTimeMs, endTimeMs int64) (deals []helper.DealForList, errRet helper.ApiError) {
	res, err := subRs.DoGetDealList(startTimeMs, endTimeMs)
	if !err.Nil() {
		return nil, err
	}

	oldestTime, latestTime := int64(math.MaxInt64), int64(0)
	oldestIdx, latestIdx := 0, 0
	for i := 0; i < len(res.Deals); i++ {
		t := res.Deals[i].TradeTimeMs
		if t < oldestTime {
			oldestTime = t
		}
		if t > latestTime {
			latestTime = t
		}
	}

	if oldestTime < startTimeMs-1000 {
		rs.Logger.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Deals[oldestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Orders[oldestIdx])}
	}
	if latestTime > endTimeMs {
		rs.Logger.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Deals[latestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Orders[latestIdx])}
	}
	deals = append(deals, res.Deals...)

	if !res.HasMore {
		return deals, helper.ApiErrorNil
	}

	if recurLevel >= _MAX_RECURSIVE_LEVEL {
		return deals, helper.ApiError{NetworkError: fmt.Errorf("递归层级过大，请检查代码，原因可能是：交易所返回的订单不按请求时间范围返回、该时间范围内订单太多")}
	}
	if startTimeMs < oldestTime {
		// 获取时间之外的订单
		deals1, err := rs.recurGetDealList(recurLevel+1, subRs, startTimeMs, oldestTime)
		if !err.Nil() {
			return nil, err
		}
		deals = append(deals, deals1...)
	}
	if latestTime < endTimeMs {
		deals2, err := rs.recurGetDealList(recurLevel+1, subRs, latestTime, endTimeMs)
		if !err.Nil() {
			return nil, err
		}
		deals = append(deals, deals2...)
	}
	return deals, helper.ApiErrorNil
}

// 如果有遗漏订单，查看交易所是否没正确设置HasMore=true
func (rs *FatherRs) GetOrderListInFather(subRs RsInner, startTimeMs, endTimeMs int64, orderState helper.OrderState) (resp []helper.OrderForList, errRet helper.ApiError) {
	const HOURS = 24

	if startTimeMs+HOURS*3600*1000 < endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("时间范围不能跨越 %d 小时", HOURS)}
	}
	if startTimeMs >= endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("startTime不应大于endTime")}
	}
	orders, err := rs.recurGetOrderList(1, subRs, startTimeMs, endTimeMs, orderState)
	if !err.Nil() {
		rs.Logger.Errorf("%v", err)
		return nil, err
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedTimeMs < orders[j].CreatedTimeMs
	})
	return orders, helper.ApiErrorNil
}

// 递归获取时间范围内的订单列表，直到该时间范围内没有更多订单
func (rs *FatherRs) recurGetOrderList(recurLevel int, subRs RsInner, startTimeMs, endTimeMs int64, orderState helper.OrderState) (orders []helper.OrderForList, errRet helper.ApiError) {
	res, err := subRs.DoGetOrderList(startTimeMs, endTimeMs, orderState)
	if !err.Nil() {
		return nil, err
	}

	oldestTime, latestTime := int64(math.MaxInt64), int64(0)
	oldestIdx, latestIdx := 0, 0
	for i := 0; i < len(res.Orders); i++ {
		t := res.Orders[i].CreatedTimeMs
		if t < oldestTime {
			oldestTime = t
		}
		if t > latestTime {
			latestTime = t
		}
	}

	if oldestTime < startTimeMs-1000 {
		rs.Logger.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Orders[oldestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Orders[oldestIdx])}
	}
	if latestTime > endTimeMs {
		rs.Logger.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Orders[latestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Orders[latestIdx])}
	}
	orders = append(orders, res.Orders...)

	if !res.HasMore {
		return orders, helper.ApiErrorNil
	}

	if recurLevel >= _MAX_RECURSIVE_LEVEL {
		return orders, helper.ApiError{NetworkError: fmt.Errorf("递归层级过大，请检查代码，原因可能是：交易所返回的订单不按请求时间范围返回、该时间范围内订单太多")}
	}
	// 获取时间之外的订单
	if startTimeMs < oldestTime {
		orders1, err := rs.recurGetOrderList(recurLevel+1, subRs, startTimeMs, oldestTime, orderState)
		if !err.Nil() {
			return nil, err
		}
		orders = append(orders, orders1...)
	}
	if latestTime < endTimeMs {
		orders2, err := rs.recurGetOrderList(recurLevel+1, subRs, latestTime, endTimeMs, orderState)
		if !err.Nil() {
			return nil, err
		}
		orders = append(orders, orders2...)
	}
	return orders, helper.ApiErrorNil
}

/*
upc实现注意事项 kc@2023-12-14

1. 现货和合约现在用同一个upc过程，日后如果碰到无法兼容再考虑拆分。
2. 合约的单位类型有基础币量、计价币量、张几个。历史原因，现在交易过程会转换这些单位，但在upc里面，优先使用交易所原始单位类型。
3. 单位类型的参考交易所：基础币量的，bn swap; 张，ok swap；计价币量，暂无。
3. 各所在填写GetPositions()返回的PositionSum时，要使用交易所的原始单位。各个rs.go现有功能里面填写TradeMsg.Position时，可能已经用multi转换了，要注意。
4. 各所在添加GetTicker()返回helper.Ticker时，也要用交易所原始单位。
5. 各所在ExchangeInfo 的 StepSize可能有转换，标准还不同。要在doCleanPos中添加交易所设置adjStepSize
6. 下平仓单时
	1）用交易所原始单位类型，注意rs.go的placeOrder可能要添加是否需要转换的标志
	2）合约交易所对于 PD PK类型订单要添加reduce only标志，这样碎仓才能得到清理


*/

func (frs *FatherRs) CleanPosInFather(maxValueClosePerTimes float64, rs Rs, rsInner RsInner, only bool) bool {
	IsInUPC = true
	defer func() {
		IsInUPC = false
	}()
	frs.Logger.Info("[upc]enter CleanPosInFather")
	defer frs.Logger.Info("[upc]exit CleanPosInFather")
	frs.upcLock.Lock()
	defer frs.upcLock.Unlock()

	exName := rs.GetExName()
	pair := rsInner.GetPairInfo().Pair
	frs.Logger.Infof("start upc. %s %s %v", exName, pair.String(), only)
	isLeft := true
	ntyChan := make(chan bool, 2)
	go func() {
		dur := 60
		ticker := time.NewTicker(time.Second * time.Duration(dur))
		i := 0
		notified := false
		startSec := time.Now().Unix()
		for {
			select {
			case <-ticker.C:
				i++
				notified = true
				helper.LogErrorThenCall(fmt.Sprintf("[utf_ign]清仓还没完成，已过 %d sec", dur*i), helper_ding.DingingSendSerious)
			case <-ntyChan:
				if notified {
					helper.LogErrorThenCall(fmt.Sprintf("[utf_ign]清仓已经完成，总计 %d sec", time.Now().Unix()-startSec), helper_ding.DingingSendSerious)
				}
				return
			}
		}
	}()
	mv := 500.0
	if maxValueClosePerTimes != 0 {
		mv = maxValueClosePerTimes
	}
	var sentSerious bool
	var cleanedTimes int // 清仓干净的次数，2次以上正确
	for tried := 0; tried < 10; tried++ {
		frs.Logger.Infof("start upc tried %d. %s %s %v", tried, exName, pair.String(), only)
		orders, err := rs.GetAllPendingOrders()
		if err.NotNil() {
			frs.Logger.Errorf("failed to GetAllPendingOrders %v", err)
		}
		if len(orders) > 0 {
			frs.Logger.Infof("all pending orders in upc:")
			for _, o := range orders {
				frs.Logger.Infof("--- %s", o.StringHeavy())
			}
		}
		isLeft, sentSerious = frs.doCleanPos(rs, rsInner, only, mv, orders)
		if isLeft {
			time.Sleep(time.Second * 3)
		} else {
			cleanedTimes++
			if cleanedTimes >= 2 {
				ntyChan <- true
				break
			}
		}
	}
	ntyChan <- true
	if isLeft {
		helper.LogErrorThenCall(fmt.Sprintf("有漏仓, cleanTimes: %d", cleanedTimes), helper_ding.DingingSendSerious)
	} else if cleanedTimes < 2 {
		helper.LogErrorThenCall(fmt.Sprintf("可能有漏仓, cleanTimes: %d", cleanedTimes), helper_ding.DingingSendSerious)
	} else {
		if sentSerious {
			helper.LogErrorThenCall("最后完成清仓，无漏仓", helper_ding.DingingSendSerious)
		}
		exName := rs.GetExName()
		var ignoreAvail bool
		if tools.Contains[string]([]string{
			// 这些所资金释放慢，可用余额不准
			helper.BrokernamePhemexUsdtSwap.String(),
			helper.BrokernamePhemexUsdSwap.String(),
			helper.BrokernameBybitUsdtSwap.String(),
		}, exName) {
			ignoreAvail = true
		}
		CheckBalanceIsStable(rs, ignoreAvail)
		if CheckPositionAndOrderClosed(rs, only) {
			isLeft = true
		}
	}
	if isLeft {
		helper.AlerterSystemOwner.Push("有漏仓", "bq: "+GitCommitHash)
	}
	return isLeft
}

// 合约碎仓清仓，有些所合约平仓也有单量限制，例如coinex swap
func (frs *FatherRs) cleanPiecePosForSwap(rs Rs, rsInner RsInner, ticker helper.Ticker, pos helper.PositionSum, pairInfo helper.ExchangeInfo) (isLeft bool) {
	// valTimes := 1.1
	// // switch rs.GetExName() {
	// // case helper.BrokernameBitgetSpot.String():
	// // 	// bitget spot 奇葩，得放大
	// // 	// {"clientOrderId":"981721618483119","force":"normal","orderType":"market","quantity":"5.29","side":"buy","symbol":"ETHUSDT_SPBL"}
	// // 	// {"code":"45110","msg":"less than the minimum amount 5 USDT","requestTime":1721618483224,"data":null}
	// // 	valTimes = 1.3
	// // }
	// a1 := helper.FixAmount(fixed.NewF(pairInfo.MinOrderValue.Float()*valTimes/ticker.Mp.Load()), pairInfo.StepSize)
	// if a1.LessThan(pairInfo.MinOrderAmount) {
	// 	a1 = pairInfo.MinOrderAmount
	// }
	// ss := fixed.NewF(pairInfo.StepSize)
	// for a1.Float()*ticker.Mp.Load() < pairInfo.MinOrderValue.Float()*valTimes {
	// 	a1 = a1.Add(ss)
	// }

	a1 := pairInfo.MinOrderAmount //暂时只有bit需要
	cid := fmt.Sprintf("98%d", time.Now().UnixMilli())
	if strings.HasPrefix(rs.GetExName(), "gate") {
		cid = "t-" + cid
	}
	var price float64
	var orderSide helper.OrderSide
	var orderType helper.OrderType

	if pos.Side == helper.PosSideLong {
		price = ticker.Ap.Load() * 1.1
		orderSide = helper.OrderSideKD
	} else {
		price = ticker.Bp.Load() * 0.99
		orderSide = helper.OrderSideKK
	}
	orderType = helper.OrderTypeLimit

	if frs.ExchangeName == helper.BrokernameBitUsdtSwap { // bit合约限价严格
		orderType = helper.OrderTypeMarket
		price = 0
	}
	// weiredMarketBuyExs := []string{helper.BrokernameGateSpot.String()}
	// if has, err := HasAbilities(rs, AbltOrderMarketBuy); err == nil && has && //
	// 	!slice.Contain[string](weiredMarketBuyExs, rs.GetExName()) { // 这些所量价转换会有偏差
	// 	price = 0
	// 	orderType = helper.OrderTypeMarket
	// }
	pair, _ := helper.StringPairToPair(pos.Name)
	sigs := []helper.Signal{
		{
			SignalChannelType: helper.SignalChannelTypeRs,
			Type:              helper.SignalTypeNewOrder,
			ClientID:          cid,
			Amount:            a1,
			Price:             price,
			OrderSide:         orderSide,
			OrderType:         orderType,
			Pair:              pair,
		},
	}
	log.Infof("gonna open more pos for piece. mp %f, sigs %v", ticker.Mp.Load(), sigs)
	rs.SendSignal(sigs)
	time.Sleep(time.Second) // 确保cid往前 + 限频
	// 开仓后，有些所如cx spot 不会完全成交，无法知道持仓量，所以不能马上平仓，等下一轮获取仓位
	/**
	 {"code":0,"data":{"amount":"908355.21643627","amount_asset":"PEPE","asset_fee":"0","avg_price":"0.0000121162","client_id":"981721565382148","create_time":1721565382,"deal_amount":"908355.21642924","deal_fee":"0.005502906736649978844",
	 "deal_money":"11.005813473299957688","fee_asset":null,"fee_discount":"1","finished_time":null,"id":123554842871,"left":"0.00000703","maker_fee_rate":"0","market":"PEPEUSDT","money_fee":"0.005502906736649978844","order_type":"market",
	 "price":"0","status":"done","stock_fee":"0","taker_fee_rate":"0.0005","type":"buy"},"message":"Success"}
	**/
	return true
	// time.Sleep(time.Second * 2)
	// amt := fixed.NewF(pos.Amount).Add(a1)
	// amt = helper.FixAmount(amt, pairInfo.StepSize)
	// return rsInner.PlaceCloseOrder(pairInfo.Symbol, helper.OrderSidePD, amt, helper.PosModeOneway, ticker)
}

// 现货碎仓清仓
func (frs *FatherRs) cleanPiecePosForSpot(rs Rs, rsInner RsInner, ticker helper.Ticker, pos helper.PositionSum, pairInfo helper.ExchangeInfo) (isLeft bool) {
	valTimes := 1.1
	switch rs.GetExName() {
	case helper.BrokernameBitgetSpot.String(), helper.BrokernameBitgetSpotV2.String():
		// bitget spot 奇葩，得放大
		// {"clientOrderId":"981721618483119","force":"normal","orderType":"market","quantity":"5.29","side":"buy","symbol":"ETHUSDT_SPBL"}
		// {"code":"45110","msg":"less than the minimum amount 5 USDT","requestTime":1721618483224,"data":null}
		valTimes = 1.3
	}
	a1 := helper.FixAmount(fixed.NewF(pairInfo.MinOrderValue.Float()*valTimes/ticker.Mp.Load()), pairInfo.StepSize)
	if a1.LessThan(pairInfo.MinOrderAmount) {
		a1 = pairInfo.MinOrderAmount
	}
	ss := fixed.NewF(pairInfo.StepSize)
	for a1.Float()*ticker.Mp.Load() < pairInfo.MinOrderValue.Float()*valTimes {
		a1 = a1.Add(ss)
	}
	cid := fmt.Sprintf("98%d", time.Now().UnixMilli())
	if strings.HasPrefix(rs.GetExName(), "gate") {
		cid = "t-" + cid
	}
	price := ticker.Ap.Load() * 1.1
	orderType := helper.OrderTypeLimit
	weiredMarketBuyExs := []string{helper.BrokernameGateSpot.String()}
	if has, err := HasAbilities(rs, AbltOrderMarketBuy); err == nil && has && //
		!slice.Contain[string](weiredMarketBuyExs, rs.GetExName()) { // 这些所量价转换会有偏差
		price = 0
		orderType = helper.OrderTypeMarket
	}
	pair, _ := helper.StringPairToPair(pos.Name)
	sigs := []helper.Signal{
		{
			SignalChannelType: helper.SignalChannelTypeRs,
			Type:              helper.SignalTypeNewOrder,
			ClientID:          cid,
			Amount:            a1,
			Price:             price,
			OrderSide:         helper.OrderSideKD,
			OrderType:         orderType,
			Pair:              pair,
		},
	}
	log.Infof("gonna open more pos for piece. mp %f, sigs %v", ticker.Mp.Load(), sigs)
	rs.SendSignal(sigs)
	time.Sleep(time.Second) // 确保cid往前 + 限频
	// 开仓后，有些所如cx spot 不会完全成交，无法知道持仓量，所以不能马上平仓，等下一轮获取仓位
	/**
	 {"code":0,"data":{"amount":"908355.21643627","amount_asset":"PEPE","asset_fee":"0","avg_price":"0.0000121162","client_id":"981721565382148","create_time":1721565382,"deal_amount":"908355.21642924","deal_fee":"0.005502906736649978844",
	 "deal_money":"11.005813473299957688","fee_asset":null,"fee_discount":"1","finished_time":null,"id":123554842871,"left":"0.00000703","maker_fee_rate":"0","market":"PEPEUSDT","money_fee":"0.005502906736649978844","order_type":"market",
	 "price":"0","status":"done","stock_fee":"0","taker_fee_rate":"0.0005","type":"buy"},"message":"Success"}
	**/
	return true
	// time.Sleep(time.Second * 2)
	// amt := fixed.NewF(pos.Amount).Add(a1)
	// amt = helper.FixAmount(amt, pairInfo.StepSize)
	// return rsInner.PlaceCloseOrder(pairInfo.Symbol, helper.OrderSidePD, amt, helper.PosModeOneway, ticker)
}

// @param maxValue 每单清仓最大价值
// @return isLeft
func (frs *FatherRs) doCleanPos(rs Rs, rsInner RsInner, only bool, maxValue float64, pendingOrders []helper.OrderForList) (isLeft bool, sentSerious bool) {
	exName := rs.GetExName()
	isLeft = true // 避免以下网络请求失败，默认true再 false
	infos := rsInner.GetExchangeInfos()
	pairInfo := rsInner.GetPairInfo()
	if pairInfo == nil {
		log.Errorf("failed to get exchange infos in upc")
		return true, false
	}
	exchangeInfoP2S := make(map[string]helper.ExchangeInfo, 0)
	rsInnerCloser, _ := rs.(RsInnerOneClickCloser)
	for _, info := range infos {
		exchangeInfoP2S[info.Pair.String()] = info
	}
	positions, err := rsInner.GetOrigPositions()
	frs.Logger.Infof("upc positions :%v", positions)
	if !err.Nil() {
		frs.Logger.Errorf("%s 获取持仓失败, err:%v", exName, err.String())
		return true, sentSerious
	}
	isLeft = false
	isSpot := helper.IsSpot(exName)
	rsGetAllTickers, ok := rs.(RsGetAllTickers)
	tickers := make(map[string]helper.Ticker)
	if ok {
		tickers, _ = rsGetAllTickers.GetAllTickersKeyedSymbol()
	}
	// 统计碎仓清仓次数，避免浮点精度等问题导致一直清仓
	pieceCleanTimes := make(map[string]int)
	for _, item := range positions {
		// 兼容负数
		item.AvailAmount = math.Abs(item.AvailAmount)
		item.Amount = math.Abs(item.Amount)
		if only && item.Name != pairInfo.Pair.String() {
			frs.Logger.Infof("不在only clean 范围内 %v. pair:%s", item, pairInfo.Pair.String())
			continue
		}
		if item.Amount == 0 && item.AvailAmount == 0 {
			continue
		}
		pair, err1 := helper.StringPairToPair(item.Name)
		if err1 != nil {
			frs.Logger.Error(err1)
			isLeft = true
			continue
		}
		// orders, err := rs.GetPendingOrders(&pair)
		// if err.NotNil() {
		// 	_, ok := exchangeInfoP2S[item.Name]
		// 	if !ok && isSpot {
		// 		frs.Logger.Errorf("[utf_ign] Skip GetPendingOrders spot delisted: %v", item.Name)
		// 	} else {
		// 		isLeft = true
		// 		frs.Logger.Errorf("[utf_ign]failed to GetPendingOrders %v", err) // 有些下架币
		// 		continue
		// 	}
		// }
		pairGotPendingOrder := false
		if len(pendingOrders) > 0 {
			s, got := rs.GetPairInfoByPair(&pair)
			if got {
				for _, o := range pendingOrders {
					if o.Symbol == s.Symbol {
						frs.Logger.Infof("symbol pending orders in upc:")
						frs.Logger.Infof("   %s", o.StringHeavy())
						pairGotPendingOrder = true
					}
				}
			} else {
				frs.Logger.Errorf("[upc]no pairInfo for %v in check pendingOrder", pair)
			}
		}
		if pairGotPendingOrder {
			rs.CancelPendingOrders(&pair)
		}
		// 冻仓判断
		if !tools.Contains[string]([]string{
			// 这些所可用仓位计算慢，忽略，最后无漏仓就行
			helper.BrokernameBitgetUsdtSwap.String(),
		}, exName) {
			if item.AvailAmount < item.Amount {
				isLeft = true
				sentSerious = true
				helper.LogErrorThenCall(fmt.Sprintf("%s 存在冻仓，%v", exName, item.String()), helper_ding.DingingSendSerious)
			}
		}

		if item.Amount == 0 || item.Amount < 1e-14 {
			continue
		}

		info, ok := exchangeInfoP2S[item.Name]
		if !ok {
			frs.Logger.Errorf("[upc]没有symbol信息 %v", item.Name)
			if !isSpot {
				// 合约下架时不会持有仓位
				isLeft = true
			}
			continue
		}

		var ticker helper.Ticker
		ticker, ok = tickers[info.Symbol] // 第一次使用全体tickers，避免现货碎仓太多，拖长清仓时间
		if !ok {
			ticker, err = rsInner.GetTickerBySymbol(info.Symbol)
			if !err.Nil() {
				if err == helper.ApiErrorDelisted {
					frs.Logger.Warnf("delisted 已下架 %s %s", exName, info.Symbol)
					return false, sentSerious
				}
				frs.Logger.Errorf("%s failed to get ticker, err:%v", exName, err.String())
				return true, sentSerious
			}
		}
		if ticker.Mp.Load() == 0 {
			frs.Logger.Warnf("清仓时 ticker mp = 0，可能导致清仓不完整: %s", item.Name)
			// helper_ding.DingingSendSerious(fmt.Sprintf("清仓时无法获取价格，可能导致清仓不完整: %s", item.Name))
			isLeft = true
			continue
		}

		if !isSpot && (ticker.Mp.Load() > 2*item.Ave || item.Ave > ticker.Mp.Load()*2) {
			frs.Logger.Errorf("mid price far away from entry price, maybe not right ticker. mp %f, ep %f", ticker.Mp.Load(), item.Ave)
		}

		// 如果现货、合约清仓差异太大，考虑分离。

		// 碎仓处理
		// 合约也有碎仓无法平仓
		if isSpot || exName == "bit_usdt_swap" {
			if item.Amount < info.StepSize {
				frs.Logger.Warnf("超碎仓，交易所持币精度故障，忽略，%v. %v", item.String(), info)
				continue
			} else if item.Amount < info.MinOrderAmount.Float() {
				// frs.Logger.Warnf("清仓时，数量小于最小下单数量，%v. %v", item.String(), info)
				if t, ok := pieceCleanTimes[item.Name]; ok {
					if t > 3 {
						frs.Logger.Warnf("碎仓已尝试3次清仓，当漏仓处理，%v. %v", item.String(), info)
						isLeft = true
						continue
					}
					pieceCleanTimes[item.Name] = t + 1
				} else {
					pieceCleanTimes[item.Name] = 1
				}
				frs.Logger.Warnf("碎仓清仓，%v. %v", item.String(), info)
				if isSpot {
					frs.cleanPiecePosForSpot(rs, rsInner, ticker, item, info)
				} else { // 合约也出现碎仓不能清，bit swap
					frs.cleanPiecePosForSwap(rs, rsInner, ticker, item, info)
				}
				isLeft = true
				// continue
			} else if item.Amount*ticker.Mp.Load() < info.MinOrderValue.Float()*1.1 {
				// frs.Logger.Warnf("清仓时，价值小于最小下单价值，%v. %v", item.String(), info)
				if t, ok := pieceCleanTimes[item.Name]; ok {
					if t > 3 {
						frs.Logger.Warnf("碎仓已尝试3次清仓，当漏仓处理，%v. %v", item.String(), info)
						isLeft = true
						continue
					}
					pieceCleanTimes[item.Name] = t + 1
				} else {
					pieceCleanTimes[item.Name] = 1
				}
				frs.Logger.Warnf("碎仓清仓，%v. %v", item.String(), info)
				if isSpot {
					frs.cleanPiecePosForSpot(rs, rsInner, ticker, item, info)
				} else {
					frs.cleanPiecePosForSwap(rs, rsInner, ticker, item, info)
				}
				continue
			}
		}
		adjStepSize := info.StepSize
		adjMinOrderAmount := info.MinOrderAmount
		// 注意！！ 有些交易所的exchangeinfo 这两个字段已经加上 multi，需要去除，日后GetExchangeInfo变更时，这里也需要调整 , helper.BrokernameBybitUsdtSwap.String()
		if tools.Contains[string]([]string{
			helper.BrokernameOkxUsdtSwap.String(),      //
			helper.BrokernameBitmartUsdtSwap.String(),  //
			helper.BrokernameHuobiUsdtSwap.String(),    //
			helper.BrokernameHuobiUsdtSwapIso.String(), //
			helper.BrokernameGateUsdtSwap.String(),     //
			helper.BrokernameKucoinUsdtSwap.String(),   //
		}, exName) {
			adjStepSize = fixed.NewF(info.StepSize).Div(info.Multi).Float()
			adjMinOrderAmount = info.MinOrderAmount.Div(info.Multi)
		}
		if isSpot { // 合约多小都应该能清仓
			if adjMinOrderAmount.Float()*info.Multi.Float()*ticker.Mp.Load() < info.MinOrderValue.Float()*1.1 {
				adjMinOrderAmount = helper.FixAmount(fixed.NewF(info.MinOrderValue.Float()*1.1/ticker.Mp.Load()), adjStepSize)
			}
		}

		if helper.DEBUGMODE {
			// 如果StepSize没有做转换，utf阶段报错
			// if adjStepSize == info.Multi.Float() && !info.Multi.Equal(fixed.ONE) {
			// frs.Logger.Errorf("adjStepSize should not equal multi again. %s, %f, %f", info.Symbol, adjStepSize, info.Multi)
			// }
		}

		isLeft = true
		// amountPerShare 不小于min order value， 不大于 max value
		var posVal float64
		switch info.AmountUnitType {
		case helper.AmountUnitType_Base:
			posVal = item.Amount * info.Multi.Float() * ticker.Mp.Load()
		case helper.AmountUnitType_Quote:
			posVal = item.Amount * info.Multi.Float()
			// case helper.AmountUnitType_Piece:
			// switch info.UnderlyingType {
			// case helper.UnderlyingBase:
			// posVal = item.Amount * info.Multi.Float() * ticker.Mp.Load()
			// case helper.UnderlyingQuote:
			// posVal = item.Amount * info.Multi.Float()
			// default:
			// helper_ding.DingingSendSerious(fmt.Sprintf("清仓时，没有找到正确的underlying type: %v", info))
			// }
		}
		leftPosVal := posVal
		leftPosAmount := fixed.NewF(item.Amount).Abs()
		if helper.DEBUGMODE {
			frs.Logger.Debugf("leftPosVal %v. pos: %v, info.Multi %v, mp %v, symbol:%v", leftPosVal, item, info.Multi, ticker.Mp.Load(), info.Symbol)
		}

		if strings.Contains(exName, "bitget_usdt_swap") && (leftPosVal < 10 || leftPosVal < info.MinOrderValue.Float() || leftPosAmount.LessThan(adjMinOrderAmount)) {
			frs.Logger.Warnf("bg swap leftPos Too small 清仓一次就无视 %v. %v", item.String(), info)
			var orderSide helper.OrderSide
			if item.Side == helper.PosSideLong {
				// 注意！！ 这里每个交易所GetTicker时如果已经乘上Multi，Aq 会错误
				orderSide = helper.OrderSidePD
			} else if item.Side == helper.PosSideShort {
				orderSide = helper.OrderSidePK
			} else {
				frs.Logger.Errorf("position side not set. %v", item)
				isLeft = true
				return
			}
			rsInner.PlaceCloseOrder(info.Symbol, orderSide, leftPosAmount, item.Mode, item.MarginMode, ticker)
			isLeft = false
			continue
		}

		var cleanTimes int
		// 没有错误就一直清
		for {
			bboVal := 0.0
			var orderSide helper.OrderSide
			price := 1.0
			if item.Side == helper.PosSideLong {
				// 注意！！ 这里每个交易所GetTicker时如果已经乘上Multi，Aq 会错误
				bboVal = ticker.Aq.Load() * info.Multi.Float() * ticker.Ap.Load() * 0.8
				if strings.HasSuffix(exName, "usd_swap") {
					bboVal = ticker.Aq.Load() * info.Multi.Float() * 0.8
					frs.Logger.Debugf("tickerap %v infomulti %v tickeraq", ticker.Aq.Load(), info.Multi.Float(), ticker.Aq.Load())
				}
				orderSide = helper.OrderSidePD
				price = ticker.Ap.Load()
			} else if item.Side == helper.PosSideShort {
				bboVal = ticker.Bq.Load() * info.Multi.Float() * ticker.Bp.Load() * 0.8
				if strings.HasSuffix(exName, "usd_swap") {
					bboVal = ticker.Bq.Load() * info.Multi.Float() * 0.8
					frs.Logger.Debugf("tickerap %v infomulti %v tickeraq", ticker.Aq.Load(), info.Multi.Float(), ticker.Aq.Load())
				}
				orderSide = helper.OrderSidePK
				price = ticker.Bp.Load()
			} else {
				frs.Logger.Errorf("position side not set. %v", item)
				isLeft = true
				return
			}
			if bboVal == 0 {
				helper.LogErrorThenCall(fmt.Sprintf("多轮清仓时，没有获取到价格，%v. %v. %v", item.String(), info, cleanTimes), helper_ding.DingingSendSerious)
				sentSerious = true
				isLeft = true
				break
			}
			if bboVal < 50 {
				frs.Logger.Warnf("bboVal too small %f, adjust to 50. %s", bboVal, item.String())
				bboVal = 50
			}
			valShare := min(leftPosVal, maxValue, info.MaxOrderValue.Float(), bboVal)
			if valShare <= 0 {
				break
			}
			if isSpot && valShare < info.MinOrderValue.Float()*1.1 {
				// frs.Logger.Warnf("清仓时，价值小于最小下单价值，%v. %v", item.String(), info)
				break
			}

			switch info.AmountUnitType {
			case helper.AmountUnitType_Base:
				// 默认为Base，已经使用ticker price设置
			case helper.AmountUnitType_Quote:
				price = 1.0
			}
			// valShare 转为 order amount
			orderAmount := helper.FixAmount(fixed.NewF(valShare/(price*info.Multi.Float())), adjStepSize)
			if orderAmount.LessThan(adjMinOrderAmount) {
				if isSpot {
					frs.Logger.Infof("[upc] min orderAmount . %v,%v", adjMinOrderAmount, leftPosAmount)
					orderAmount = fixed.Min(adjMinOrderAmount, leftPosAmount)
					// if orderSide == helper.OrderSidePK {
					orderSide = helper.OrderSidePD // 现货强制设为PD，先不考虑杠杆交易
					// }
				} else {
					orderAmount = fixed.Min(adjMinOrderAmount, leftPosAmount)
				}
			}

			cleanTimes++
			// 不支持碎仓清仓且提供一键全平的交易所，低于50U时调用
			if rsInnerCloser != nil && (leftPosVal <= 50 || leftPosAmount.LessThanOrEqual(adjMinOrderAmount.Mul(fixed.TWO))) {
				rsInnerCloser.ClosePosOneClick(info.Symbol, item.PositionId)
				// 不判断返回，用第二次获取仓位判断
				leftPosVal = 0
				break
			} else {
				// if orderAmount.LessThanOrEqual(fixed.ZERO) || (leftPosVal <= maxValue && !isSpot) {
				if orderAmount.LessThanOrEqual(fixed.ZERO) || (leftPosVal <= maxValue) { // 现货也适用
					orderAmount = leftPosAmount
				}
				if isSpot {
					if orderAmount.GreaterThan(leftPosAmount) {
						// 有些余额信息不按精度来
						frs.Logger.Infof("[upc] reduce orderAmount . %v,%v, %v", orderAmount, leftPosAmount, info.StepSize)
						orderAmount = helper.FixAmount(leftPosAmount, info.StepSize)
					} else if orderAmount.Float() > leftPosAmount.Float()*0.9 { // 现货碎仓在补齐 MinOrderAmount/MinOrderValue 再平仓过程中，会有上下波动。超过9成就全平
						frs.Logger.Infof("[upc] close all leftPosAmount. %v,%v, %v", orderAmount, leftPosAmount, info.StepSize)
						orderAmount = helper.FixAmount(leftPosAmount, info.StepSize)
					}
					if orderAmount.LessThan(adjMinOrderAmount) { // 低于最小单量，无法下单，等待下一轮判断
						break
					}
				}

				frs.Logger.Infof("times %d to upc clean. pos:%v, valShare: %v, leftPosVal: %v, leftPosAmount: %v,  maxValue: %v, orderAmount %v, adjMinOrderAmount %v, adjStepSize %v. info.Multi %v, info.StepSize %v", //
					cleanTimes, item, valShare, leftPosVal, leftPosAmount, maxValue, orderAmount, adjMinOrderAmount, adjStepSize, info.Multi, info.StepSize)
				if !rsInner.PlaceCloseOrder(info.Symbol, orderSide, orderAmount, item.Mode, item.MarginMode, ticker) {
					break
				}
				leftPosVal -= valShare
				leftPosAmount = leftPosAmount.Sub(orderAmount)
				if leftPosVal <= 0 || leftPosAmount.LessThanOrEqual(fixed.ZERO) {
					break
				}
			}

			time.Sleep(time.Second)
			ticker, err = rsInner.GetTickerBySymbol(info.Symbol)
			if !err.Nil() {
				return true, sentSerious
			}
			time.Sleep(time.Second)
		}
	}
	return isLeft, sentSerious
}

func (frs *FatherRs) PrintAcctSumWhenBeforeTrade(rs Rs) {
	acct, err := rs.GetAcctSum()
	if !err.Nil() {
		frs.Logger.Infof("before trade, acct sum: %v. err %v", acct.String(), err.Error())
	} else {
		frs.Logger.Infof("before trade, acct sum: %v", acct.String())
	}
}

func (frs *FatherRs) HasPosition(rs Rs, only bool) bool {
	acct, err := rs.GetAcctSum()
	if !err.Nil() {
		log.Errorf("[HasPosition] failed to GetAcctSum. %v", err.Error())
		return true
	}
	if frs.ExchangeInfoPtrP2S.Count() == 0 || rs.GetPairInfo().Pair.Quote == "" {
		rs.GetExchangeInfos()
	}
	if frs.ExchangeInfoPtrP2S.Count() == 0 {
		log.Error("[HasPosition] frs.ExchangeInfoP2S is empty during check HasPosition")
		return true
	}
	quote := rs.GetPairInfo().Pair.Quote
	if quote == "" {
		log.Error("[HasPosition] cannot GetPairInfo quote [%s] ", rs.GetPairInfo())
		return true
	}

	has := false
	if helper.IsSpot(rs.GetExName()) {
		rsGetAllTickers, ok := rs.(RsGetAllTickers)
		tickers := make(map[string]helper.Ticker)
		if ok {
			var err helper.ApiError
			tickers, err = rsGetAllTickers.GetAllTickersKeyedSymbol()
			if !err.Nil() {
				log.Warnf("failed to GetAllTickersKeyedSymbol.%v", err.Error())
			}
		} else {
			log.Warnf("not implement GetAllTickersKeyedSymbol")
		}
		// 没实现ticker 会在后面获取判断

		for _, v := range acct.Balances {
			if v.Amount == 0 && v.Avail == 0 {
				continue
			}
			if strings.EqualFold(v.Name, frs.Pair.Quote) || helper.IsQuoteCoin(v.Name) {
				continue
			}

			info, ok := frs.ExchangeInfoPtrP2S.Get(v.Name + "_" + quote)
			if !ok {
				log.Errorf("[HasPosition]没有symbol信息 %s, might be delisted", v.Name+"_"+quote)
				// 忽略下架币，不能当漏仓
				// return true
				continue
			}
			if !info.Status {
				log.Errorf("[HasPosition] 不可交易", v.Name+"_"+quote)
				continue
			}

			if only && v.Name != rs.GetPairInfo().Pair.Base {
				log.Warnf("[HasPosition] case only base:%s, 跳过%s", rs.GetPairInfo().Pair.Base, v.Name)
				continue
			}

			price := v.Price
			if price == 0 {
				if t, ok := tickers[info.Symbol]; ok && t.Price() > 0 {
					price = t.Price()
				} else if t, err := rs.GetTickerBySymbol(info.Symbol); err.Nil() && t.Price() > 0 {
					price = t.Price()
				} else {
					log.Errorf("failed to GetTickerBySymbol")
				}
			}
			if price <= 0 {
				log.Errorf("failed to get price. return as leaked. %s ", info.Symbol)
				return true
			}
			has = (v.Amount > info.MinOrderAmount.Float()*1.1) && (price*v.Amount > 10)
			if has {
				log.Warnf("[HasPosition]has position before trade: %v", v)
				break
			}
		}
	} else {
		for _, v := range acct.Positions {
			info, ok := frs.ExchangeInfoPtrP2S.Get(v.Name)
			if !ok {
				log.Errorf("[HasPosition]没有symbol信息 %s", v.Name+"_"+quote)
				return true
			}
			if only && v.Name != rs.GetPairInfo().Pair.String() {
				log.Warnf("[HasPosition] case only:%s, 跳过%s", rs.GetPairInfo().Pair.String(), v.Name)
				continue
			}
			if v.Amount > 0 {
				if strings.Contains(frs.ExchangeName.String(), "bitget_usdt_swap") {
					tick, _ := rs.GetTickerBySymbol(info.Symbol)
					if v.Amount*tick.Price() < 10 || v.Amount*tick.Price() < info.MinOrderValue.Float() || v.Amount < info.MinOrderAmount.Float() {
						continue
					}
				}
				log.Warnf("has position before trade: %v", v)
				has = true
				break
			}
		}
	}
	return has
}

func (rs *FatherRs) GetTickerByPair(pair *helper.Pair) (ticker helper.Ticker, err helper.ApiError) {
	var symbol string
	if pair == nil || pair.Base == "" {
		symbol = rs.Symbol
	} else if pair.ExchangeInfo != nil {
		symbol = pair.ExchangeInfo.Symbol
	} else {
		info, ok := rs.ExchangeInfoPtrP2S.Get(pair.String())
		if !ok {
			err.NetworkError = fmt.Errorf("not found symbol for pair %s", pair)
			return
		}
		symbol = info.Symbol
	}
	return rs.subRsInner.GetTickerBySymbol(symbol)
}

func (frs *FatherRs) IsSpot() bool {
	return helper.IsSpot(frs.subRs.GetExName())
}

func (frs *FatherRs) GetAccountMode(pair string) (int, helper.MarginMode, helper.PosMode, helper.ApiError) {
	if frs.IsSpot() {
		return 0, helper.MarginMode_Nil, helper.PosModeNil, helper.ApiErrorNil
	}

	info, ok := frs.ExchangeInfoPtrP2S.Get(pair)
	if !ok {
		log.Errorf("not found info for pair %s", pair)
	}

	leverage, marginMode, positionMode, err := frs.subRsInner.DoGetAccountMode(info)
	if !err.Nil() {
		return 0, helper.MarginMode_Nil, helper.PosModeNil, err
	}

	return leverage, marginMode, positionMode, helper.ApiErrorNil
}

func (frs *FatherRs) VerifyAccountMode(pair string, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) (lastError helper.ApiError) {
	// 验证设置是否成功
	currentLeverage, currentMarginMode, currentPositionMode, err := frs.GetAccountMode(pair)
	if !err.Nil() {
		log.Errorf("Position info not found info for pair %s", pair)
		lastError = err
		return
	}
	// 检查是否与期望值匹配
	mismatches := []struct {
		name     string
		expected interface{}
		actual   interface{}
		skip     bool // 是否跳过检查
	}{
		// 如果目标杠杆率或现杠杆率为0则跳过检查
		{"leverage", leverage, currentLeverage, strings.Contains(frs.ExchangeName.String(), "bitget") || frs.ExchangeName == helper.BrokernameCoinexUsdtSwap || frs.ExchangeName == helper.BrokernameHyperUsdcSwap},
		{"margin mode", marginMode, currentMarginMode, frs.ExchangeName == helper.BrokernameCoinexUsdtSwap || frs.ExchangeName == helper.BrokernameBitmartUsdtSwap || frs.ExchangeName == helper.BrokernameHyperUsdcSwap},
		{"position mode", positionMode, currentPositionMode, frs.ExchangeName == helper.BrokernameCoinexUsdtSwap},
	}

	for _, mismatch := range mismatches {
		if !mismatch.skip && mismatch.expected != nil && mismatch.expected != mismatch.actual {
			e := fmt.Errorf("%s mismatch for pair %s: expected %v, got %v", mismatch.name, pair, mismatch.expected, mismatch.actual)
			log.Error(e)
			lastError.HandlerError = e
			return
		}
	}
	return helper.ApiErrorNil
}

func (frs *FatherRs) SetAccountMode(pair string, leverage int, marginMode helper.MarginMode, positionMode helper.PosMode) (lastError helper.ApiError) {
	if positionMode == helper.PosModeHedge && frs.ExchangeName != helper.BrokernameBitmartUsdtSwap {
		return helper.ApiErrorWithHandlerError("Please use one way")
	}

	if frs.IsSpot() {
		return helper.ApiErrorNil
	}
	info, ok := frs.ExchangeInfoPtrP2S.Get(pair)

	if !ok {
		log.Errorf("not found info for pair %s", pair)
		return helper.ApiErrorWithHandlerError("not found pairinfo for pair. " + pair)
	}

	defer func() {
		time.Sleep(3 * time.Second)
		lastError = frs.VerifyAccountMode(pair, leverage, marginMode, positionMode)
	}()

	if innerRs, ok := frs.subRsInner.(RsInnerSetAccountMode); ok {
		return innerRs.DoSetAccountMode(info, leverage, marginMode, positionMode)
	}
	if marginMode != helper.MarginMode_Nil {
		lastError = frs.subRsInner.DoSetMarginMode(info.Symbol, marginMode)
		if !lastError.Nil() {
			return lastError
		}
	}
	if positionMode != helper.PosModeNil {
		lastError = frs.subRsInner.DoSetPositionMode(info.Symbol, positionMode)
		if !lastError.Nil() {
			return lastError
		}
	}

	if leverage != 0 {
		lastError = frs.subRsInner.DoSetLeverage(*info, leverage)
		if !lastError.Nil() {
			return lastError
		}
	}
	return helper.ApiErrorNil
}

// func (frs *FatherRs) SetLeverage(pair string, leverage int) {
// 	info, ok := frs.ExchangeInfoPtrP2S.Get(pair)
// 	if !ok {
// 		log.Errorf("not found info for pair %s", pair)
// 	}
// 	frs.subRsInner.DoSetLeverage(*info, leverage)
// }

// func (f *FatherRs) SetMarginMode(pair string, marginMode helper.MarginMode) helper.ApiError {
// 	info, ok := f.ExchangeInfoPtrP2S.Get(pair)
// 	if !ok {
// 		return helper.ApiErrorWithHandlerError("not found pairinfo for pair. " + pair)
// 	}
// 	return f.subRsInner.DoSetMarginMode(info.Symbol, marginMode)
// }

// func (f *FatherRs) SetPositionMode(pair string, positionMode helper.PosMode) helper.ApiError {
// 	info, ok := f.ExchangeInfoPtrP2S.Get(pair)
// 	if !ok {
// 		return helper.ApiErrorWithHandlerError("not found pairinfo for pair. " + pair)
// 	}
// 	return f.subRsInner.DoSetPositionMode(info.Symbol, positionMode)
// }

func capitalizeFirstLetter(s string) string {
	if s == "" {
		return s
	}

	firstLetter := []rune(s)[0]
	capitalized := string(unicode.ToUpper(firstLetter)) + s[1:]
	return capitalized
}

func (f *FatherRs) hasFeatureForLine(action ActionType, line SelectedLine) bool {
	actionStr := capitalizeFirstLetter(action.String())
	clientStr := capitalizeFirstLetter(string(line.Client))
	linkStr := capitalizeFirstLetter(string(line.Link))
	featureName := fmt.Sprintf("Do%sOrder%s%s", actionStr, clientStr, linkStr)

	if v := tools.GetStructFieldValue(f.Features, featureName); v != nil && v.(bool) {
		return true
	}
	return false
}

func (f *FatherRs) GetSelectedLine(action ActionType) (line SelectedLine) {
	switch action {
	case ActionType_Place:
		return f.SelectedLine_Place
	case ActionType_Cancel:
		return f.SelectedLine_Cancel
	case ActionType_Amend:
		return f.SelectedLine_Amend
	}
	return SelectedLine{}
}
func (f *FatherRs) SwitchLine(action ActionType, line SelectedLine) (err error, changed bool) {
	f.Logger.Infof("gonna switchline. ex %s, action %v, line: %v", f.subRs.GetExName(), action, line)
	var s *SelectedLine
	if !f.hasFeatureForLine(action, line) {
		err = fmt.Errorf("no feature %v, %v", action, line)
		return
	}
	if line.MarginMode != helper.MarginMode_Nil && !helper.IsSpot(f.subRs.GetExName()) {
		if err0 := f.subRsInner.DoSetMarginMode(f.Symbol, line.MarginMode); !err0.Nil() {
			f.Logger.Errorf("failed to switch line for margin mode. %v", err0)
			err = errors.New(err0.Error())
			return
		} else {
			f.Logger.Infof("succ set margin mode. %v", line.MarginMode)
		}
	}
	switch action {
	case ActionType_Place:
		s = &f.SelectedLine_Place
	case ActionType_Cancel:
		s = &f.SelectedLine_Cancel
	case ActionType_Amend:
		s = &f.SelectedLine_Amend
	}
	changed = s.CAS(&line)
	f.Logger.Infof("switchline, changed %v, %v", changed, s)
	return nil, changed
}

func (f *FatherRs) EnsureReqWsNorLogged(succHandler func(), failHandler func()) error {
	if f.ReqWsNorLogged.Load() {
		go succHandler()
		return nil
	}
	go func() {
		if f.ReqWsNor == nil {
			f.subRsInner.DoCreateReqWsNor()
		}
		tried := 0
		for ; tried < 5; tried++ {
			if !f.ReqWsNorLogged.Load() {
				time.Sleep(time.Second)
				continue
			} else {
				break
			}
		}
		if !f.ReqWsNorLogged.Load() {
			f.Logger.Errorf("[%s] [%p] [%p] failed to login req ws nor", f.ExchangeName, f, &f.ReqWsNorLogged)
			go failHandler()
			return
		}
		go succHandler()
	}()
	return nil
}

func (f *FatherRs) EnsureReqWsColoLogged(succHandler func(), failHandler func()) error {
	if f.ReqWsColoLogged.Load() {
		go succHandler()
		return nil
	}
	go func() {
		if f.ReqWsColo == nil {
			f.subRsInner.DoCreateReqWsColo()
		}
		tried := 0
		for ; tried < 5; tried++ {
			if !f.ReqWsColoLogged.Load() {
				time.Sleep(time.Second)
				continue
			} else {
				break
			}
		}
		if !f.ReqWsColoLogged.Load() {
			f.Logger.Errorf("[%s] failed to login req ws colo", f.ExchangeName)
			go failHandler()
			return
		}
		go succHandler()
	}()
	return nil
}

func (f *FatherRs) AddRsStopHandler(h func(rs Rs)) {
	f.stopHandlers = append(f.stopHandlers, h)
}

func (f *FatherRs) saveRsPerformanceMetrics() {
	tags := map[string]string{
		"robot_id": f.BrokerConfig.RobotId,
		"ex":       f.subRs.GetExName(),
	}
	fields := map[string]interface{}{
		"pair_created_times": f.pairCreatedTimes.Load(),
	}
	f.saveOnePoint("performance", "rs", tags, fields)
}

func (f *FatherRs) Stop() {
	for _, h := range f.stopHandlers {
		h(f.subRs)
	}
	f.subRsInner.DoStop()
	f.Logger.Infof("pairCreatedTimes %d", f.pairCreatedTimes.Load())
	if !IsUtf {
		f.saveRsPerformanceMetrics()
	}
}
func (rs *FatherRs) TryHandleMonitorOrderRsCancel(cid string, colo bool, durationUs int64) (handled bool) {
	if rs.DelayMonitor.IsActiveReq(cid) {
		handled = true
		price := rs.TradeMsg.Ticker.Bp.Load() * 0.96
		size := helper.FixAmount(fixed.NewF(rs.PairInfo.MinOrderValue.Float()*1.1/price), rs.PairInfo.StepSize)
		if size.LessThan(rs.PairInfo.MinOrderAmount) {
			size = rs.PairInfo.MinOrderAmount
		}
		s := helper.Signal{
			Type:      helper.SignalTypeNewOrder,
			Price:     price,
			Amount:    size,
			OrderSide: helper.OrderSideKD,
			OrderType: helper.OrderTypeLimit,
			Time:      0,
		}
		if colo {
			s.ClientID = rs.DelayMonitor.GenCid(EndpointKey_RsColoPlace, false)
			rs.subRsInner.DoPlaceOrderRsColo(rs.PairInfo, s)
		} else {
			s.ClientID = rs.DelayMonitor.GenCid(EndpointKey_RsNorPlace, false)
			rs.subRsInner.DoPlaceOrderRsNor(rs.PairInfo, s)
		}
	} else if rs.DelayMonitor.IsMonitorOrder(cid) {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsColoCancel, durationUs)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsNorCancel, durationUs)
		}
	}
	return
}
func (rs *FatherRs) TryHandleMonitorOrderWsPlace(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	if _, _, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsColoPlace, durationUs)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsNorPlace, durationUs)
		}
		// next action
		// 先amend
		if (colo && rs.Features.DoAmendOrderWsColo) || (!colo && rs.Features.DoAmendOrderWsNor) {
			if s.Price == 0 {
				s.Price = rs.TradeMsg.Ticker.Bp.Load() * 0.92 // 比下单时价格距离更低
			} else {
				s.Price -= 2 * rs.PairInfo.TickSize
			}
			if colo {
				rs.subRsInner.DoAmendOrderWsColo(rs.PairInfo, s)
			} else {
				rs.subRsInner.DoAmendOrderWsNor(rs.PairInfo, s)
			}
		} else {
			if colo {
				rs.subRsInner.DoCancelOrderWsColo(rs.PairInfo, s)
			} else {
				rs.subRsInner.DoCancelOrderWsNor(rs.PairInfo, s)
			}
		}
	}
	return
}
func (rs *FatherRs) TryHandleMonitorOrderWsAmend(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	if _, _, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsColoAmend, durationUs)
			rs.subRsInner.DoCancelOrderWsColo(rs.PairInfo, s)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsNorAmend, durationUs)
			rs.subRsInner.DoCancelOrderWsNor(rs.PairInfo, s)
		}
	}
	return
}
func (rs *FatherRs) TryHandleMonitorOrderWsCancel(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	if _, _, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsColoCancel, durationUs)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_WsNorCancel, durationUs)
		}
	}
	return
}
func (rs *FatherRs) TryHandleMonitorOrderRsPlace(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	// var key EndpointKey_T
	if _, _, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsColoPlace, durationUs)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsNorPlace, durationUs)
		}
		// next action
		if (colo && rs.Features.DoAmendOrderRsColo) || (!colo && rs.Features.DoAmendOrderRsNor) {
			if colo {
				if s.Price == 0 {
					s.Price = rs.TradeMsg.Ticker.Bp.Load() * 0.92 // 比下单时价格距离更低
				} else {
					s.Price -= 2 * rs.PairInfo.TickSize
				}
				rs.subRsInner.DoAmendOrderRsColo(rs.PairInfo, s)
			} else {
				s.Price -= 2 * rs.PairInfo.TickSize
				rs.subRsInner.DoAmendOrderRsNor(rs.PairInfo, s)
			}
		} else {
			if colo {
				rs.subRsInner.DoCancelOrderRsColo(rs.PairInfo, s)
			} else {
				rs.subRsInner.DoCancelOrderRsNor(rs.PairInfo, s)
			}
		}
	}
	return
}

func (rs *FatherRs) TryHandleMonitorOrderRsDeprecated(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	var active bool
	var key EndpointKey_T
	if active, key, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if active {
			rs.MonitorTrigger(key, false)
			if colo {
				rs.subRsInner.DoCancelOrderRsColo(rs.PairInfo, s)
			} else {
				rs.subRsInner.DoCancelOrderRsNor(rs.PairInfo, s)
			}
		} else {
			if colo {
				rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsColoPlace, durationUs)
				// 先amend
				// rs.amendOrder(pairInfo, price-2*pairInfo.TickSize, size, cid, oid, side, orderType, t, needConv, colo)
				s.Price -= 2 * rs.PairInfo.TickSize
				rs.subRsInner.DoAmendOrderRsColo(rs.PairInfo, s)
			} else {
				rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsNorPlace, durationUs)
				s.Price -= 2 * rs.PairInfo.TickSize
				rs.subRsInner.DoAmendOrderRsNor(rs.PairInfo, s)
			}
		}
	}
	return
}
func (rs *FatherRs) TryHandleMonitorOrderRsAmend(s helper.Signal, colo bool, durationUs int64) (isMonitorOrder bool) {
	if _, _, isMonitorOrder = rs.DelayMonitor.GetActiveAndKeyFromCid(s.ClientID); isMonitorOrder {
		if colo {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsColoAmend, durationUs)
			rs.subRsInner.DoCancelOrderRsColo(rs.PairInfo, s)
		} else {
			rs.DelayMonitor.UpdateDelayWithDuration(EndpointKey_RsNorAmend, durationUs)
			rs.subRsInner.DoCancelOrderRsNor(rs.PairInfo, s)
		}
	}
	return
}

// todo remove
func (f *FatherRs) MonitorTrigger(url EndpointKey_T, activeFirst bool) {
	price := f.TradeMsg.Ticker.Bp.Load() * 0.96
	size := helper.FixAmount(fixed.NewF(f.PairInfo.MinOrderValue.Float()*1.1/price), f.PairInfo.StepSize)
	if size.LessThan(f.PairInfo.MinOrderAmount) {
		size = f.PairInfo.MinOrderAmount
	}

	s := helper.Signal{
		Type:      helper.SignalTypeNewOrder,
		Price:     price,
		Amount:    size,
		OrderSide: helper.OrderSideKD,
		OrderType: helper.OrderTypeLimit,
		Time:      0,
	}
	switch url {
	case EndpointKey_RsNorPlace:
		// s.ClientID = rs.DelayMonitor.GenCid(url, activeFirst)
		// rs.placeOrder(rs.PairInfo, price, size, cid, helper.OrderSideKD, helper.OrderTypeLimit, 0, true, false)
		// rs.DoPlaceOrderRsNor(rs.PairInfo, s)
		s.ClientID = f.DelayMonitor.mockCidForActive
		f.subRsInner.DoCancelOrderRsNor(f.PairInfo, s)
	case EndpointKey_RsColoPlace:
		// s.ClientID = rs.DelayMonitor.GenCid(url, activeFirst)
		// rs.placeOrder(rs.PairInfo, price, size, cid, helper.OrderSideKD, helper.OrderTypeLimit, 0, true, true)
		// rs.DoPlaceOrderRsColo(rs.PairInfo, s)
		s.ClientID = f.DelayMonitor.mockCidForActive
		f.subRsInner.DoCancelOrderRsColo(f.PairInfo, s)
	case EndpointKey_WsNorPlace:
		if !f.ReqWsNorLogged.Load() {
			return
		}
		s.ClientID = f.DelayMonitor.GenCid(url, false)
		f.subRsInner.DoPlaceOrderWsNor(f.PairInfo, s)
		// rs.wsPlaceOrder(rs.PairInfo, s, false)
		// reqId := fmt.Sprintf("%d", time.Now().UnixNano())
		// rs.wsCancelOrder(monitorTestOidForCancel, false, reqId)
		//{"label":"INVALID_ARGUMENT","message":"Text content not starting with `t-`"}
		// this is ok, because we use reqId to identify the order
	case EndpointKey_WsColoPlace:
		if !f.ReqWsColoLogged.Load() {
			return
		}
		s.ClientID = f.DelayMonitor.GenCid(url, false)
		// rs.wsPlaceOrder(rs.PairInfo, s, true)
		f.subRsInner.DoPlaceOrderWsColo(f.PairInfo, s)
		// reqId := fmt.Sprintf("%d", time.Now().UnixNano())
		// rs.wsCancelOrder(monitorTestOidForCancel, true, reqId)
	}
}

func (f *FatherRs) setFeaturesFalseForDummy(val reflect.Value, feat *Features) {
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		fieldName := typ.Field(i).Name
		if strings.HasPrefix(fieldName, "Dummy") {
			// fmt.Printf("here. %s\n", fieldName)
			featName := strings.Replace(fieldName, "Dummy", "", 1)
			tools.SetField(feat, featName, false)
			// fmt.Println("here. kind", typ.Kind())
			if typ.Kind() == reflect.Struct {
				val1 := val.Field(i)
				f.setFeaturesFalseForDummy(val1, feat)
			}
		}
	}
}

// 如果一个交易所结构体包含了DummyXYZ，则会去除 XYZ 这个feature，以此为第一优先级。不管有没实现对应方法
func (f *FatherRs) FillOtherFeatures(exPtr interface{}, feat *Features) {
	// tools.GetAllFields(exPtr)
	for _, d := range []interface{}{ // OrderActions
		DummyDoPlaceOrderRsNor{}, DummyDoPlaceOrderRsColo{}, DummyDoPlaceOrderWsNor{}, DummyDoPlaceOrderWsColo{},
		DummyDoCancelOrderRsNor{}, DummyDoCancelOrderRsColo{}, DummyDoCancelOrderWsNor{}, DummyDoCancelOrderWsColo{},
		DummyDoAmendOrderRsNor{}, DummyDoAmendOrderRsColo{}, DummyDoAmendOrderWsNor{}, DummyDoAmendOrderWsColo{},
		// ----------------------
		DummyDoGetPriceLimit{},
	} {
		notAsDummy := !tools.HasField(exPtr, reflect.TypeOf(d))
		name := tools.GetStructName(d)
		fieldName := strings.Replace(name, "Dummy", "", 1)
		tools.SetField(feat, fieldName, notAsDummy)
	}
	val := reflect.ValueOf(exPtr).Elem()
	f.setFeaturesFalseForDummy(val, feat)
	return
}

func (f *FatherRs) CancelPendingOrders(pair *helper.Pair) helper.ApiError {
	if pair == nil {
		f.Logger.Errorf("pair should not nil")
		err := helper.ApiError{
			HandlerError: errors.New("pair should not nil"),
		}
		return err
	}
	info := pair.ExchangeInfo
	if info == nil {
		info, _ = f.ExchangeInfoPtrP2S.Get(pair.ToString())
	}
	if info == nil {
		f.Logger.Errorf("not found exchangeinfo for pair %s", pair.ToString())
		err := helper.ApiError{
			HandlerError: errors.New("not found exchangeinfo for pair"),
		}
		return err
	}
	return f.subRsInner.DoCancelPendingOrders(info.Symbol)
}
func (f *FatherRs) GetPriceLimit(pair *helper.Pair) (helper.PriceLimit, helper.ApiError) {
	if pair == nil {
		f.Logger.Errorf("pair should not nil")
		err := helper.ApiError{
			HandlerError: errors.New("pair should not nil"),
		}
		return helper.PriceLimit{}, err
	}
	info := pair.ExchangeInfo
	if info == nil {
		info, _ = f.ExchangeInfoPtrP2S.Get(pair.ToString())
	}
	if info == nil {
		f.Logger.Errorf("not found exchangeinfo for pair %s", pair.ToString())
		err := helper.ApiError{
			HandlerError: errors.New("not found exchangeinfo for pair"),
		}
		return helper.PriceLimit{}, err
	}
	pl, err := f.subRsInner.DoGetPriceLimit(info.Symbol)
	if err.NotNil() {
		f.Logger.Errorf("failed to get price limit. %s, %s", f.ExchangeName, err.Error())
	}
	return pl, err
}

var json_iter = jsoniter.ConfigCompatibleWithStandardLibrary

func (f *FatherRs) CheckAndSaveExInfo(fileName string, infos []helper.ExchangeInfo) {
	succ := false
	defer func() {
		if !succ {
			f.Logger.Error("CheckAndSaveExInfo failed")
			helper.AlerterSystem.Push("CheckAndSaveExInfo failed", f.BrokerConfig.RobotId)
			f.Cb.OnExit("CheckAndSaveExInfo failed")
		}
	}()

	if !f.checkExInfo(infos) {
		f.Logger.Errorf("invalid exchange info ")
		helper.AlerterSystem.Push("invalid exchange info ", f.BrokerConfig.RobotId)
		return
	}
	jsonData, err := json_iter.Marshal(infos)
	if err != nil {
		f.Logger.Error("序列化失败 %v", err)
		return
	}
	var afterUnmarshal []map[string]any
	err = json_iter.Unmarshal(jsonData, &afterUnmarshal)
	if err != nil {
		f.Logger.Error("反序列化检查验证失败 %v", err)
		return
	}
	if !f.checkExInfo2(afterUnmarshal) {
		f.Logger.Errorf("failed to check after unmarshal exchange info ")
		helper.AlerterSystem.Push("invalid exchange info ", "反序列化检查验证失败. "+f.BrokerConfig.RobotId)
		return
	}
	succ = true
	os.WriteFile(fileName, jsonData, 0644)
}

func (f *FatherRs) checkExInfo(infos []helper.ExchangeInfo) bool {
	for _, info := range infos {
		ok, msg := info.CheckReady()
		if !ok {
			msg := fmt.Sprintf("invalid exchange info %v@%v@%v", info.Symbol, f.subRs.GetExName(), msg)
			log.Errorf(msg)
			return false
		}
	}
	return true
}

func (f *FatherRs) checkExInfo2(infos []map[string]any) bool {
	for _, info := range infos {
		failedTrigger := false
		MaxOrderAmount, err := strconv.ParseFloat(info["MaxOrderAmount"].(string), 64)
		MinOrderAmount, err2 := strconv.ParseFloat(info["MinOrderAmount"].(string), 64)
		if err != nil || err2 != nil || MaxOrderAmount == 0 || MaxOrderAmount <= MinOrderAmount {
			failedTrigger = true
			msg := fmt.Sprintf("invalid MaxOrderAmount %v@%v@%v er:%v,%v", info["MaxOrderAmount"], info["Symbol"], f.subRs.GetExName(), err, err2)
			f.Logger.Errorf(msg)
		}
		MaxOrderValue, err := strconv.ParseFloat(info["MaxOrderValue"].(string), 64)
		MinOrderValue, err2 := strconv.ParseFloat(info["MinOrderValue"].(string), 64)
		if err != nil || err2 != nil || MaxOrderValue == 0 || MaxOrderValue <= MinOrderValue {
			failedTrigger = true
			msg := fmt.Sprintf("invalid MaxOrderValue %v@%v@%v er:%v,%v", info["MaxOrderValue"], info["Symbol"], f.subRs.GetExName(), err, err2)
			f.Logger.Errorf(msg)
		}
		MaxPosAmount, err := strconv.ParseFloat(info["MaxPosAmount"].(string), 64)
		if err != nil || MaxPosAmount == 0 {
			failedTrigger = true
			msg := fmt.Sprintf("invalid MaxPosAmount %v@%v@%v er:%v", info["MaxPosAmount"], info["Symbol"], f.subRs.GetExName(), err)
			f.Logger.Errorf(msg)
		}
		MaxPosValue, err := strconv.ParseFloat(info["MaxPosValue"].(string), 64)
		if err != nil || MaxPosValue == 0 {
			failedTrigger = true
			msg := fmt.Sprintf("invalid MaxPosValue %v@%v@%v er:%v", info["MaxPosValue"], info["Symbol"], f.subRs.GetExName(), err)
			f.Logger.Errorf(msg)
		}
		if failedTrigger {
			f.Logger.Errorf("invalid field in exchange info check after marshal")
			return false
		}
	}
	return true
}

func (f *FatherRs) GetLabileExchangeInfos() cmap.ConcurrentMap[string, *helper.LabileExchangeInfo] {
	return f.ExchangeInfoLabilePtrP2S
}
func (f *FatherRs) GetPendingOrders(pair *helper.Pair) (resp []helper.OrderForList, err helper.ApiError) {
	var symbol string
	if pair == nil || pair.Base == "" {
		symbol = f.Symbol
	} else if pair.ExchangeInfo != nil {
		symbol = pair.ExchangeInfo.Symbol
	} else {
		info, ok := f.ExchangeInfoPtrP2S.Get(pair.String())
		if !ok {
			err.NetworkError = fmt.Errorf("not found symbol for pair %s", pair)
			return
		}
		symbol = info.Symbol
	}
	return f.subRsInner.DoGetPendingOrders(symbol)
}

// 在 BeforeTrade 初始化 exchangeinfo后调用
func (f *FatherRs) CheckPairs() helper.SystemError {
	notFound := make([]string, 0)
	for _, pair := range f.Pairs {
		if _, ok := f.ExchangeInfoPtrP2S.Get(pair.ToString()); !ok {
			notFound = append(notFound, pair.ToString())
		}
	}
	if len(notFound) > 0 {
		return helper.SystemErrorWithClientError(fmt.Sprintf("not found pairs: %s", strings.Join(notFound, ",")))
	}
	return helper.SystemErrorNil
}

func (f *FatherRs) GetPosHist(startTimeMs int64, endTimeMs int64) (resp []helper.Positionhistory, err helper.ApiError) {
	f.Logger.Errorf("not implemented GetPostionHistory")
	return
}

func (f *FatherRs) DoGetPosHist(startTimeMs int64, endTimeMs int64) (resp helper.PosHistResponse, err helper.ApiError) {
	f.Logger.Errorf("not implemented DoGetPostionHistory")
	return
}

// 如果有遗漏订单，查看交易所是否没正确设置HasMore=true
func (rs *FatherRs) GetPosHistInFather(subRs RsInner, startTimeMs, endTimeMs int64) (resp []helper.Positionhistory, errRet helper.ApiError) {
	const HOURS = 24

	if startTimeMs+HOURS*3600*1000 < endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("时间范围不能跨越 %d 小时", HOURS)}
	}
	if startTimeMs >= endTimeMs {
		return nil, helper.ApiError{NetworkError: fmt.Errorf("startTime不应大于endTime")}
	}
	orders, err := rs.recurGetGetPosHist(1, subRs, startTimeMs, endTimeMs)
	if !err.Nil() {
		rs.Logger.Errorf("%v", err)
		return nil, err
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CloseTime < orders[j].CloseTime
	})
	return orders, helper.ApiErrorNil
}

// 递归获取时间范围内的订单列表，直到该时间范围内没有更多订单
func (rs *FatherRs) recurGetGetPosHist(recurLevel int, subRs RsInner, startTimeMs, endTimeMs int64) (deals []helper.Positionhistory, errRet helper.ApiError) {
	res, err := subRs.DoGetPosHist(startTimeMs, endTimeMs)
	if !err.Nil() {
		return nil, err
	}

	oldestTime, latestTime := int64(math.MaxInt64), int64(0)
	oldestIdx, latestIdx := 0, 0
	for i := 0; i < len(res.Pos); i++ {
		t := res.Pos[i].CloseTime
		if t < oldestTime {
			oldestTime = t
		}
		if t > latestTime {
			latestTime = t
		}
	}

	if oldestTime < startTimeMs-1000 {
		rs.Logger.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Pos[oldestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出起始时间的订单. recurLevel:%d, startTimeMs: %d, order:%v", recurLevel, startTimeMs, res.Orders[oldestIdx])}
	}
	if latestTime > endTimeMs {
		rs.Logger.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Pos[latestIdx])
		// return nil, helper.ApiError{HandlerError: fmt.Errorf("交易所返回了超出结束时间的订单. recurLevel:%d, endTimeMs: %d, order:%v", recurLevel, endTimeMs, res.Orders[latestIdx])}
	}
	deals = append(deals, res.Pos...)

	if !res.HasMore {
		return deals, helper.ApiErrorNil
	}

	if recurLevel >= _MAX_RECURSIVE_LEVEL {
		return deals, helper.ApiError{NetworkError: fmt.Errorf("递归层级过大，请检查代码，原因可能是：交易所返回的订单不按请求时间范围返回、该时间范围内订单太多")}
	}
	if startTimeMs < oldestTime {
		// 获取时间之外的订单
		deals1, err := rs.recurGetGetPosHist(recurLevel+1, subRs, startTimeMs, oldestTime)
		if !err.Nil() {
			return nil, err
		}
		deals = append(deals, deals1...)
	}
	if latestTime < endTimeMs {
		deals2, err := rs.recurGetGetPosHist(recurLevel+1, subRs, latestTime, endTimeMs)
		if !err.Nil() {
			return nil, err
		}
		deals = append(deals, deals2...)
	}
	return deals, helper.ApiErrorNil
}

func (f *FatherRs) GetFacilities() int64 {
	return 0
}
