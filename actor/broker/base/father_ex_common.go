package base

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"actor/helper"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/log"
	"github.com/duke-git/lancet/v2/slice"
	client "github.com/influxdata/influxdb1-client/v2"
)

type FatherCommon struct {
	Logger                   log.Logger
	BrokerConfig             helper.BrokerConfigExt
	ExchangeName             helper.BrokerName
	Cb                       helper.CallbackFunc // 所有的回调函数
	Fee                      helper.DualFeeRate
	ExchangeInfoPtrS2P       cmap.ConcurrentMap[string, *helper.ExchangeInfo] // 交易信息集合
	ExchangeInfoPtrP2S       cmap.ConcurrentMap[string, *helper.ExchangeInfo] // 交易信息集合
	ExchangeInfoLabilePtrP2S cmap.ConcurrentMap[string, *helper.LabileExchangeInfo]
	Symbol                   string      // 交易所 外部
	Pair                     helper.Pair // 交易对
	PairInfo                 *helper.ExchangeInfo
	TradeMsg                 *helper.TradeMsg // 交易常用数据结构
	// 应对event
	SymbolMode helper.SymbolMode
	Pairs      []helper.Pair
	Symbols    []string
	Position   *helper.Pos
	EquityMap  cmap.ConcurrentMap[string, *helper.EquityEvent] // key: asset name, 小写. rs/ws使用不同EquityMap
	IsRs       bool                                            // rs or ws
	// ShortestPrefixLen int // symbol最短可识别前缀，0表示只有main pair, -1 不用转换
	AssetsUpper []string // 该client实例关联的资产种类，spot会有多个，swap只有几个计价币
	AssetsLower []string // 该client实例关联的资产种类，spot会有多个，swap只有几个计价币

	// CanColoRs        bool                                    // rest可以用colo
	// CanColoWs        bool                                    // ws可以用colo
	PositionMap      cmap.ConcurrentMap[string, *helper.Pos] // key is symbol, not pair. rs/ws使用同一个PositionMap
	defaultPair      helper.Pair
	pairCreatedTimes atomic.Int32 // pair创建次数。应该很低，尽量复用

	ContinuousFailNums []atomic.Int32 // 全部请求, 连续下单出错, 撤单
}

type FailNumActionIdx_Type int

func (t FailNumActionIdx_Type) String() string {
	switch t {
	case FailNumActionIdx_AllReq:
		return "请求"
	case FailNumActionIdx_Place:
		return "下单"
	case FailNumActionIdx_Cancel:
		return "撤单"
	}
	return ""
}

const (
	FailNumActionIdx_AllReq FailNumActionIdx_Type = 0 + iota
	FailNumActionIdx_Place
	FailNumActionIdx_Cancel
	FailNumActionIdx_EndBarrier
)

func (f *FatherCommon) ReqFail(action FailNumActionIdx_Type) {
	if f.ContinuousFailNums[action].Add(1) > 50 { // 次数不是很精准，可能25次就触发
		helper.LogErrorThenCall(fmt.Sprintf("连续 %s 出错，需要停机", action), f.Cb.OnExit)
		return
	}
	if action != FailNumActionIdx_AllReq {
		action2 := FailNumActionIdx_AllReq
		if f.ContinuousFailNums[action2].Add(1) > 50 {
			helper.LogErrorThenCall(fmt.Sprintf("连续 %s 出错，需要停机", action2), f.Cb.OnExit)
			return
		}
	}
}
func (f *FatherCommon) ReqSucc(action FailNumActionIdx_Type) {
	f.ContinuousFailNums[action].Store(0)
	if action != FailNumActionIdx_AllReq {
		action2 := FailNumActionIdx_AllReq
		f.ContinuousFailNums[action2].Store(0)
	}
}
func (f *FatherCommon) ObtainPosition() *helper.Pos {
	return f.Position
}
func (f *FatherCommon) ObtainPositionMap() *cmap.ConcurrentMap[string, *helper.Pos] {
	return &f.PositionMap
}
func (f *FatherCommon) ObtainEquityMap() *cmap.ConcurrentMap[string, *helper.EquityEvent] {
	return &f.EquityMap
}
func (f *FatherCommon) IsSymbolOne() bool {
	return f.SymbolMode == helper.SymbolMode_One
}
func (f *FatherCommon) GetDefaultPair() helper.Pair {
	return f.defaultPair
}

func (f *FatherCommon) SymbolToPair(symbol string) (*helper.Pair, error) {
	info, ok := f.ExchangeInfoPtrS2P.Get(symbol)
	if !ok {
		return nil, fmt.Errorf("not found pair for symbol %s", symbol)
	}
	return &info.Pair, nil
}
func (f *FatherCommon) PairStrToSymbol(pairStr string) (string, error) {
	info, ok := f.ExchangeInfoPtrP2S.Get(pairStr)
	if !ok {
		return "", fmt.Errorf("not found symbol for pair %s", pairStr)
	}
	return info.Symbol, nil
}

func (f *FatherCommon) SetDefaultPair(pair string) (err error) {
	f.defaultPair, err = helper.StringPairToPair(pair)
	return err
}
func (f *FatherCommon) saveOnePoint(database, measurement string, tags map[string]string, fields map[string]any) error {
	return nil
	if helper.DEBUGMODE && IsUtf {
		f.Logger.Debugf("ignore write influx under utf")
		return nil
	}
	influxCfg := DefaultInfluxConfig(nil)
	influxDBConfig := client.HTTPConfig{
		Addr:     "http://" + influxCfg.Addr,
		Username: influxCfg.User,
		Password: influxCfg.Pw,
		Timeout:  time.Second * 10,
	}

	influxClient, err := client.NewHTTPClient(influxDBConfig)

	if err != nil {
		log.Errorf("inited influx client failed %s", err.Error())
		return err
	} else {
		log.Infof("inited influx client")
	}

	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database: database,
	})
	if err != nil {
		log.Errorf("init NewBatchPoints failed %s", err)
		return err
	}

	p, err := client.NewPoint(measurement,
		tags,
		fields,
	)
	if err != nil {
		log.Errorf("NewPoint failed %s", err.Error())
		return err
	}
	bp.AddPoint(p)

	err = influxClient.Write(bp)
	if err != nil {
		log.Errorf("write influx failed %s", err)
	} else {
		if helper.DEBUGMODE {
			log.Debugf("write influx succ %s,  %v", f.BrokerConfig.Name, bp)
		}
	}
	return nil
}
func (f *FatherCommon) InitCommon(params *helper.BrokerConfigExt, info *helper.ExchangeInfo, cb helper.CallbackFunc, msg *helper.TradeMsg) {
	f.TradeMsg = msg
	f.Logger = params.Logger
	f.ExchangeName = helper.StringToBrokerName(params.Name)
	if f.ExchangeName == helper.BrokernameUnknown {
		panic("BrokerConfig.Name 有错误. content :" + params.String())
	}
	f.Cb = cb
	if cb.OnWsReady == nil {
		f.Cb.OnWsReady = func(ex helper.BrokerName) {}
	}
	if f.defaultPair.Base == "" {
		f.defaultPair = helper.NewPair("btc", "usdt", "")
	}
	f.PairInfo = info
	f.BrokerConfig = *params
	f.ExchangeInfoPtrS2P = cmap.New[*helper.ExchangeInfo]()
	f.ExchangeInfoPtrP2S = cmap.New[*helper.ExchangeInfo]()
	f.ExchangeInfoLabilePtrP2S = cmap.New[*helper.LabileExchangeInfo]()
	f.ContinuousFailNums = make([]atomic.Int32, int(FailNumActionIdx_EndBarrier))
	f.Pairs = params.Pairs
	if len(f.Pairs) == 0 {
		f.Pairs = []helper.Pair{f.defaultPair}
	}
	if params.SymbolAll {
		f.SymbolMode = helper.SymbolMode_All
		f.Pair = f.Pairs[0]
	} else {
		if len(f.Pairs) == 1 {
			f.SymbolMode = helper.SymbolMode_One
			f.Pair = f.Pairs[0]
		} else if len(f.Pairs) >= 2 {
			log.Info("init FatherCommon, Pairs %v", params.Pairs)
			f.Pair = f.Pairs[0]
			f.SymbolMode = helper.SymbolMode_Multi
		}
	}
	f.PositionMap = params.PositionMap
	if f.IsRs {
		f.EquityMap = params.EquityMapRs
	} else {
		f.EquityMap = params.EquityMapWs
	}
	// f.EquityMap = cmap.New[*helper.EquityEvent]()
	f.Position = params.Position
	if f.SymbolMode != helper.SymbolMode_All {
		assets := slice.FlatMap[helper.Pair](f.Pairs, func(idx int, p helper.Pair) []string {
			return []string{strings.ToUpper(p.Base)}
		})
		assets = append(assets, strings.ToUpper(f.Pair.Quote))
		f.AssetsUpper = assets
		f.AssetsLower = slice.Map[string](assets, func(idx int, p string) string {
			return strings.ToLower(p)
		})
	}
}
func (w *FatherCommon) GetPosBySymbol(symbol string) (pos *helper.Pos, ok bool) {
	if w.SymbolMode == helper.SymbolMode_One {
		if symbol == w.Symbol {
			return w.Position, true
		}
		return nil, false
	} else if w.SymbolMode == helper.SymbolMode_Multi {
		pos, ok = w.PositionMap.Get(symbol)
		return pos, ok
	} else {
		// 如果不存在，动态创建
		pos, ok = w.PositionMap.Get(symbol)
		if !ok {
			info, ok2 := w.ExchangeInfoPtrS2P.Get(symbol)
			if !ok2 {
				log.Errorf("failed to dynamic get pos for symbol. %v", symbol)
				return nil, false
			}
			pos = &helper.Pos{Pair: info.Pair}
			ok = true
			s := helper.EnsureClone(symbol)
			w.PositionMap.Set(s, pos)
		}
		return pos, ok
	}
}

func (w *FatherCommon) UpdateExchangeInfoSimp(onExit func(string)) bool {
	return w.UpdateExchangeInfo(w.ExchangeInfoPtrP2S, w.ExchangeInfoPtrS2P, onExit)
}
func (w *FatherCommon) UpdateExchangeInfo(ExchangeInfoPtrP2S, ExchangeInfoPtrS2P cmap.ConcurrentMap[string, *helper.ExchangeInfo], onExit func(string)) bool {
	w.ExchangeInfoPtrP2S = ExchangeInfoPtrP2S
	w.ExchangeInfoPtrS2P = ExchangeInfoPtrS2P

	for _, v := range w.ExchangeInfoPtrP2S.Items() {
		v.Pair.ExchangeInfo = &(*v)
	}
	for _, v := range w.ExchangeInfoPtrS2P.Items() {
		v.Pair.ExchangeInfo = &(*v)
	}
	w.Symbols = make([]string, 0)
	if w.SymbolMode == helper.SymbolMode_Multi {
		for _, p := range w.Pairs {
			info, ok := w.ExchangeInfoPtrP2S.Get(p.String())
			if !ok {
				helper.LogErrorThenCall(fmt.Sprintf("not found info for pair %s", p), onExit)
				return false
			}
			w.Symbols = append(w.Symbols, info.Symbol)
		}
		// w.ShortestPrefixLen = helper.UniqueIdentifyShortest(symbols)
		// log.Infof("use ShortestPrefixLen %d, symbols %v", w.ShortestPrefixLen, symbols)
		// if w.ShortestPrefixLen == -1 {
		// helper.LogErrorThenCall(fmt.Sprintf("symbol/pair 有重叠前缀, %v", w.Pairs), onExit)
		// return false
		// }
		for _, s := range w.Symbols {
			// w.PositionMap[s[0:w.ShortestPrefixLen]] = &helper.Pos{}
			info, ok := w.ExchangeInfoPtrS2P.Get(s)
			if !ok {
				log.Errorf("failed to get pairinfo . %v", s)
				continue
			}
			w.PositionMap.Set(s, &helper.Pos{Pair: info.Pair})
		}
		w.Symbol = w.Symbols[0]
	} else { //  helper.SymbolMode_One SymbolAll都跑这里，All也必须指定一个pair
		for p, info := range w.ExchangeInfoPtrP2S.Items() {
			if p == w.Pair.Output {
				w.Symbols = append(w.Symbols, info.Symbol)
				w.Symbol = info.Symbol
				break
			}
		}
		if w.Symbol == "" {
			log.Errorf("not found symbol for main pair %v. maybe not call InitFatherRs/Ws first.", w.Pair)
			return false
		}
	}
	// 用rs.PairInfo替换，避免数据不一致
	s, ok := w.ExchangeInfoPtrS2P.Get(w.Symbol)
	if !ok {
		helper.LogErrorThenCall(fmt.Sprintf("failed to get symbolinfo, %s", w.Symbol), w.Cb.OnExit)
		return false
	}
	helper.CopySymbolInfo(w.PairInfo, s)
	w.ExchangeInfoPtrP2S.Set(w.Pair.String(), w.PairInfo)
	w.ExchangeInfoPtrS2P.Set(w.Symbol, w.PairInfo)
	w.PositionMap.Set(w.Symbol, w.Position)
	return true
}
func (rs *FatherCommon) GetPairInfo() *helper.ExchangeInfo {
	return rs.PairInfo
}
func (rs *FatherCommon) PairToSymbol(pair *helper.Pair) string {
	if pair.ExchangeInfo != nil {
		return pair.ExchangeInfo.Symbol
	}
	if info, ok := rs.ExchangeInfoPtrP2S.Get(pair.String()); ok {
		return info.Symbol
	}
	return ""
}
func (w *FatherCommon) GetPairInfoByPair(pair *helper.Pair) (*helper.ExchangeInfo, bool) {
	if pair.ExchangeInfo != nil {
		return pair.ExchangeInfo, true
	}
	if pair != nil && pair.Base != "" {
		if pair.Output == "" {
			_ = pair.String()
		}
		info, ok := w.ExchangeInfoPtrP2S.Get(pair.Output)
		if !ok {
			w.Logger.Errorf("not found pairinfo for pair %v", pair)
			return nil, false
		}
		pair.ExchangeInfo = info
		w.pairCreatedTimes.Add(1)
		return info, true
	}
	if helper.DEBUGMODE {
		w.Logger.Debugf("using main pair info. %v, pair %s", w.PairInfo, pair)
	}
	return w.PairInfo, true
}
func (w *FatherCommon) GetPairInfoBySymbol(symbol string) (*helper.ExchangeInfo, bool) {
	if w.SymbolMode == helper.SymbolMode_One {
		if symbol != w.Symbol {
			return nil, false
		}
		return w.PairInfo, true
	}
	info, ok := w.ExchangeInfoPtrS2P.Get(symbol)
	return info, ok
}

// todo rename
func (w *FatherCommon) EquityNewerAndStore(a string, sequence, tsns int64, fieldSet helper.FieldsSet_T) (*helper.EquityEvent, bool) {
	_ = helper.DEBUGMODE && helper.MustNanos(tsns)
	e, ok := w.EquityMap.Get(a)
	if ok {
		if e.Seq.NewerAndStore(sequence, tsns) {
			return e, true
		}
		return nil, false
	}
	// 避免引用，确保一定复制
	asset := helper.EnsureClone(a)
	e = &helper.EquityEvent{
		Name:      asset,
		FieldsSet: fieldSet,
	}
	if w.IsRs {
		e.EventWay = helper.EventWayRs
	} else {
		e.EventWay = helper.EventWayWs
	}
	w.EquityMap.Set(asset, e)
	e.Seq.Ex.Store(sequence)
	e.Seq.Inner.Store(tsns)
	return e, true
}

func (w *FatherCommon) CleanOtherEquity(assetMapGot map[string]bool, fieldsSet helper.FieldsSet_T, onEquityEvent func(ts int64, event helper.EquityEvent)) {
	for asset, e := range w.EquityMap.Items() {
		if assetMapGot != nil {
			if _, ok := assetMapGot[asset]; ok {
				continue
			}
		}
		tsns := time.Now().UnixNano()

		if helper.DEBUGMODE {
			log.Debugf("CleanOtherEquity %s %d", asset, tsns)
		}
		e.Avail = 0
		e.TotalWithUpl = 0
		e.TotalWithoutUpl = 0
		e.Upl = 0
		e.FieldsSet = fieldsSet
		e.Seq.Inner.Store(tsns)
		onEquityEvent(0, *e)
	}
}

// rs/ws没有一致seqEx时，rs里面调用。仓位不一致且已经5秒没变化，可以更新
func (w *FatherCommon) PosChangedAndFaraway(symbol string, longPos, shortPos fixed.Fixed) (*helper.Pos, bool) {
	pos, ok := w.GetPosBySymbol(symbol)
	if !ok {
		return nil, false
	}
	tsns := time.Now().UnixNano()

	if helper.DEBUGMODE {
		log.Debugf("PosChangedAndFaraway. pos %v, longPos %v, shortPos %v, tsns %d", pos, longPos, shortPos, tsns)
	}

	pos.Lock.Lock()
	defer pos.Lock.Unlock()
	if pos.LongPos.Equal(longPos) && pos.ShortPos.Equal(shortPos) {
		return nil, false
	}
	if pos.Seq.Inner.Load()+(time.Second*5).Nanoseconds() > tsns {
		return nil, false
	}
	pos.Seq.Inner.Store(tsns)
	pos.LongPos = longPos
	pos.ShortPos = shortPos

	return pos, true
}

// todo event rename
func (w *FatherCommon) PosPureNewerAndStore(symbol string, seqEx, ts int64) (*helper.Pos, bool) {
	pos, ok := w.GetPosBySymbol(symbol)
	if !ok {
		return nil, false
	}

	if pos.Seq.NewerAndStore(seqEx, ts) {
		return pos, true
	} else {
		if helper.DEBUGMODE {
			log.Debugf("found pos, but seq is fresh")
		}
		return nil, false
	}
}

// todo event rename, 返回 pairinfo，需要pairinfo.Multi
func (w *FatherCommon) PosNewerAndStore(symbol string, seqEx, tsns int64) (*helper.ExchangeInfo, *helper.Pos, bool) {
	_ = helper.DEBUGMODE && helper.MustNanos(tsns)
	pos, ok := w.GetPosBySymbol(symbol)
	if !ok {
		log.Debugf("not found pos for symbol, %s", symbol)
		return nil, nil, false
	}

	if pos.Seq.NewerAndStore(seqEx, tsns) {
		info, ok := w.ExchangeInfoPtrS2P.Get(symbol)
		if !ok {
			log.Warnf("unknow pairinfo of symbol, %s", symbol)
			return nil, nil, false
		}
		return info, pos, true
	} else {
		if helper.DEBUGMODE {
			log.Debugf("found pos, but seq is fresh")
		}
		return nil, nil, false
	}
}

func (w *FatherCommon) GetPairInfoAndPos(symbol string) (info *helper.ExchangeInfo, pos *helper.Pos, ok bool) {
	pos, ok = w.GetPosBySymbol(symbol)
	if !ok {
		return nil, nil, false
	}
	if w.SymbolMode == helper.SymbolMode_One {
		info = w.PairInfo
	} else {
		info, ok = w.ExchangeInfoPtrS2P.Get(symbol)
		if !ok {
			log.Warnf("unknow pos of symbol, %s", symbol)
			return nil, nil, false
		}
	}
	return info, pos, ok
}

// 会有性能损耗，只在0仓位时不推送的所使用，明确推送0仓的所不能用。
func (f *FatherCommon) CleanOthersPosImmediately(excludeKyes map[string]bool, ts int64, onPositionEvent func(ts int64, event helper.PositionEvent)) {
	for sym, pos := range f.PositionMap.Items() {
		if excludeKyes != nil {
			if _, ok := excludeKyes[sym]; ok {
				continue
			}
		}
		pos.Lock.Lock()
		pos.ResetLocked()
		pos.Seq.Inner.Store(ts)
		e := pos.ToPositionEvent()
		pos.Lock.Unlock()
		onPositionEvent(0, e)
	}
}

// 使用5秒准则更新其他仓位，只在rs里面用
func (f *FatherCommon) CleanOthersPos(excludeKyes map[string]bool, onPositionEvent func(ts int64, event helper.PositionEvent)) {
	for sym := range f.PositionMap.Items() {
		if excludeKyes != nil {
			if _, ok := excludeKyes[sym]; ok {
				continue
			}
		}
		if pos, ok := f.PosChangedAndFaraway(sym, fixed.ZERO, fixed.ZERO); ok {
			pos.Lock.Lock()
			pos.LongAvg = 0
			pos.ShortAvg = 0
			e := pos.ToPositionEvent()
			pos.Lock.Unlock()
			onPositionEvent(0, e)
		}
	}
}
