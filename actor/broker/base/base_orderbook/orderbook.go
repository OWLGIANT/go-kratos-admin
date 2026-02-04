package base_orderbook

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"actor/broker/base/orderbook_mediator"
	"actor/helper"
	"actor/helper/helper_ding"
	"actor/third/log"
	"actor/tools"
)

type Orderbook struct {
	ex       string
	pair     string
	name     string
	pool     sync.Pool
	skipList []any
	dualLink *SortedDualLink
	lock     sync.Mutex

	firstMatchJudger       func(snapSlot *Slot, s *Slot) bool
	connectionJudger       func(firstMatched bool, seq int64, s *Slot) bool
	snapFetcher            func() (*Slot, error)
	snapFetchType          int
	onExit                 func(string)
	depth                  *helper.Depth
	depthSubLevel          int  // 订单薄档数，默认返回全部；<0 表示不需要复制，只维护订单簿就行
	disableAutoUpdateDepth bool // 是否禁用自动更新depth
	asks                   *SkipList
	bids                   *SkipList
	updateId               int64 // 更新的序列号
	seq                    int64 // 有些所可以用seq进行多档位 orderbook ticker对比, 例如bybit
	wrongTimeMs            int64 // 出现错误时间，等待几秒后协程还没补上，认为真正错误
	waitSnapCnt            int
	rebuilding             bool
	slotChan               chan *Slot // ws底层用了协程化，避免旧数据所在协程一直得不到调度，这里用slotChan及时将指针line up
	ReceivedTsNs           int64      // 收到最后一笔有效更新的时间
	eraseOnceSnapShow      bool       // 使用了单线的ws msg接收模式 + 交易所推送有序，在出现了snap时，抛弃所有orderbook和duallink中的数据
	lastCallFetcher        int64

	rebuildFailCount atomic.Int32
	onWSRestart      func() // WebSocket重启回调函数

	mediator *orderbook_mediator.OrderbookMediator
}

const _WRONG_TIMEOUT_MS = 10000 // 超过这个秒数没连接正确，认为错误，需要重建

const (
	SnapFetchType_Rs               = 1 + iota
	SnapFetchType_WsConnectWithSeq // 通过ws req获取snap，并且由seq保证连续
	SnapFetchType_WsConnectWithTs
)
const epsilon = 1e-10

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}
func newAsks() *SkipList {
	return NewCustomMap(func(l, r float64) bool {
		return l < r
	}, almostEqual)
}
func newBids() *SkipList {
	return NewCustomMap(func(l, r float64) bool {
		return l > r
	}, almostEqual)
}

func (o *Orderbook) SetWSRestartCallback(callback func()) {
	o.onWSRestart = callback
}

// Reset 重置订单簿状态，用于重新连接后清理内部状态
func (o *Orderbook) ResetForbid() {
	// newAsks()不允许独立调用，需要复制level3状态
	panic("ResetForbid is forbidden")

	o.lock.Lock()
	defer o.lock.Unlock()

	o.asks = newAsks()
	o.bids = newBids()
	o.dualLink.CleanAll()
	o.updateId = 0
	o.seq = 0
	o.wrongTimeMs = 0
	o.waitSnapCnt = 0
	o.rebuilding = false

	// 重置插入失败计数器
	o.rebuildFailCount.Store(0)

	if o.slotChan != nil {
		tools.DryChan[*Slot](o.slotChan)
	}

	// 重置订单簿深度数据
	if o.depthSubLevel >= 0 && o.depth != nil {
		o.depth.Lock.Lock()
		o.depth.Asks = o.depth.Asks[:0]
		o.depth.Bids = o.depth.Bids[:0]
		o.depth.Lock.Unlock()
	}
	log.Infof("%s orderbook reset complete", o.name)
}

/*
@params connectionJudger: seq, order_book当前的序列号; s 将要插入的slot
needSlotChan 有些所乱序发送ob，且频率超出系统响应，需要用channel排序
*/
func NewOrderbook(ex, pair string, firstMatchJudger func(snapSlot *Slot, s *Slot) bool, connectionJudger func(firstMatched bool, updateId int64, s *Slot) bool, snapFetcher func() (*Slot, error), snapFetchType int, //
	onExit func(string), depth *helper.Depth) *Orderbook {
	ob := &Orderbook{
		dualLink:         NewSortedDualLink(),
		firstMatchJudger: firstMatchJudger,
		connectionJudger: connectionJudger,
		snapFetcher:      snapFetcher,
		snapFetchType:    snapFetchType,
		onExit:           onExit,
		depth:            depth,
		asks:             newAsks(),
		bids:             newBids(),
		name:             fmt.Sprintf("[%s@%s]", pair, ex),
		ex:               ex,
	}
	if strings.HasPrefix(ex, "phemex") {
		ob.dualLink.queue.SetIgnoreDuplicate()
	}
	ob.rebuilding = false
	return ob
}

func (ob *Orderbook) GetBidsAsks() (*SkipList, *SkipList) {
	return ob.bids, ob.asks
}

func (ob *Orderbook) GetInnerDepth() *helper.Depth {
	return ob.depth
}

func (ob *Orderbook) SetDepth(depthBids, depthAsks *helper.Depth) {
	ob.depth.Lock.Lock()
	defer ob.depth.Lock.Unlock()
	if cap(ob.depth.Bids) >= len(depthBids.Bids) {
		ob.depth.Bids = ob.depth.Bids[:0]
	} else {
		ob.depth.Bids = make([]helper.DepthItem, 0, len(depthBids.Bids))
	}
	if cap(ob.depth.Asks) >= len(depthAsks.Asks) {
		ob.depth.Asks = ob.depth.Asks[:0]
	} else {
		ob.depth.Asks = make([]helper.DepthItem, 0, len(depthAsks.Asks))
	}
}

func (ob *Orderbook) SetDepthLockFree(depthBids, depthTmp *helper.Depth) {
	ob.depth.Bids = depthTmp.Bids
	ob.depth.Asks = depthTmp.Asks
}

func (ob *Orderbook) SetMediator(mediator *orderbook_mediator.OrderbookMediator, handlers ...func(ob *Orderbook, tsMsNow int64, bidLevel, askLevel int)) {
	ob.mediator = mediator
	// 更新depth 的统计信息逻辑放在这里
	handler := func(ob *Orderbook, tsMsNow int64, bidLevel, askLevel int) {
	}
	if len(handlers) >= 1 {
		handler = handlers[0]
	}
	mediator.SetUpdateDepth(func(tsMsNow int64) error {
		askLevel := ob.depthSubLevel
		bidLevel := ob.depthSubLevel
		if ob.depthSubLevel == 0 {
			askLevel = ob.asks.length
			bidLevel = ob.bids.length
		}
		ob.depth.Lock.Lock()
		defer ob.depth.Lock.Unlock()
		//
		if cap(ob.depth.Asks) >= askLevel {
			ob.depth.Asks = ob.depth.Asks[:0]
		} else {
			ob.depth.Asks = make([]helper.DepthItem, 0, askLevel)
		}
		if cap(ob.depth.Bids) >= bidLevel {
			ob.depth.Bids = ob.depth.Bids[:0]
		} else {
			ob.depth.Bids = make([]helper.DepthItem, 0, bidLevel)
		}
		handler(ob, tsMsNow, bidLevel, askLevel)
		return nil
		//log.Debugf("partial. bid price, %f, %f", o.depth.Bids[0].Price, o.depth.Bids[bidLevel-1].Price)
		//log.Debugf("partial. ask price, %f, %f", o.depth.Asks[0].Price, o.depth.Asks[askLevel-1].Price)
	})
	mediator.SetIterAsk(func(cb func(float64, orderbook_mediator.Value) bool) {
		ob.lock.Lock()
		defer ob.lock.Unlock()
		iter := ob.asks.Iterator()
		for iter.Next() {
			if !cb(iter.Key(), iter.Value()) {
				break
			}
		}
	})
	mediator.SetIterBid(func(cb func(float64, orderbook_mediator.Value) bool) {
		ob.lock.Lock()
		defer ob.lock.Unlock()
		iter := ob.bids.Iterator()
		for iter.Next() {
			if !cb(iter.Key(), iter.Value()) {
				break
			}
		}
	})
}
func (ob *Orderbook) Output(builder *strings.Builder) {
	// builder.WriteString(fmt.Sprintf("ob: %s, updateId: %d, seq: %d, wrongTimeMs: %d, waitSnapCnt: %d, rebuilding: %v, snapFetchType: %d, eraseOnceSnapShow: %v, lastSnapTsMs: %d, lastCallFetcher: %d, rebuildFailCount: %d, onWSRestart: %v, lastSnapTsMs: %d\n", ob.name, ob.updateId, ob.seq, ob.wrongTimeMs, ob.waitSnapCnt, ob.rebuilding, ob.snapFetchType, ob.eraseOnceSnapShow, ob.lastSnapTsMs, ob.lastCallFetcher, ob.rebuildFailCount.Load(), ob.onWSRestart, ob.lastSnapTsMs))
	builder.WriteString("===============================\n")
	iter := ob.asks.SeekToLast()
	builder.WriteString(fmt.Sprintf("%f, %f\n", iter.Key(), iter.Value().Amount))
	for iter.Previous() {
		builder.WriteString(fmt.Sprintf("%f, %f\n", iter.Key(), iter.Value().Amount))
	}
	builder.WriteString("--------------------------------\n")
	iter = ob.bids.SeekToFirst()
	builder.WriteString(fmt.Sprintf("%f, %f\n", iter.Key(), iter.Value().Amount))
	for iter.Next() {
		builder.WriteString(fmt.Sprintf("%f, %f\n", iter.Key(), iter.Value().Amount))
	}
	builder.WriteString("===============================\n")
}
func (ob *Orderbook) SetEraseOnceSnapShow() {
	ob.eraseOnceSnapShow = true
}

// 如果orderbook更新，回调这里。
// ts： 衔接的最后一笔slot的接收时间，可能不是最新时间
func (ob *Orderbook) SetSlotChanUpdateCb(depthUpdateCb func(ts int64)) {
	ob.slotChan = make(chan *Slot, 300)
	go func() {
		log.Infof("%s. slot arrage insertor start. ", ob.name)
		for {
			select {
			case s := <-ob.slotChan:
				if ob.InsertSlot(s) {
					depthUpdateCb(ob.ReceivedTsNs)
				}
				// todo 添加退出生命周期时
				// case :
			}
		}
	}()
}

func (o *Orderbook) SetDepthSubLevel(l int) {
	o.depthSubLevel = l
}
func (o *Orderbook) SetDisableAutoUpdateDepth(disable bool) {
	o.disableAutoUpdateDepth = disable
}

func (o *Orderbook) GetFreeSlot(size int, sortIdx int64, priceLevelWithSeq ...bool) *Slot {
	s := o.dualLink.GetFreeSlot()
	if len(priceLevelWithSeq) > 0 && priceLevelWithSeq[0] {
		if cap(s.PriceItemsWithSeq) < size {
			s.PriceItemsWithSeq = make([][3]float64, 0, size)
		}
		s.PriceLevelWithSeq = true
	} else {
		if cap(s.PriceItems) < size {
			s.PriceItems = make([][2]float64, 0, size)
		}
	}
	s.SortIdx = sortIdx
	return s
}

func (o *Orderbook) rebuild(tsMs int64) {
	tools.DryChan[*Slot](o.slotChan)

	// 添加基于文件的锁机制控制REST请求频率
	folder := "/tmp/bqrs.rebuild/"
	basePath := fmt.Sprintf("%s/%s", folder, o.ex)
	if o.pair != "" {
		basePath = fmt.Sprintf("%s/%s", folder, o.ex)
	}

	_, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			log.Errorf("%s.failed to create rebuild temp folder: [%v]", o.name, err)
			return
		}
	}

	// 检查是否有太多并发重建请求
	files, err := os.ReadDir(basePath)
	if err != nil {
		log.Errorf("%s.failed to read rebuild tmp folder: [%v]", o.name, err)
		return
	}

	activeCnt := 0
	nowMs := time.Now().UnixMilli()
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		tsMs, err := strconv.ParseInt(f.Name(), 10, 64)
		if err != nil {
			continue
		}
		// 只计算5秒内的活跃重建请求
		if tsMs+(5*1000) > nowMs {
			activeCnt++
		} else if tsMs+(60*1000) < nowMs {
			// 清理过期的临时文件
			os.Remove(filepath.Join(basePath, f.Name()))
		}
	}

	// 如果活跃请求太多，等待一会再尝试
	if activeCnt >= 3 {
		log.Warnf("%s.too many rebuild requests, waiting...", o.name)
		time.Sleep(5 * time.Second)
	}

	// 创建临时文件标记当前重建
	timestamp := time.Now().UnixMilli()
	filePath := fmt.Sprintf("%s/%d", basePath, timestamp)
	tmpFile, err := os.Create(filePath)
	if err != nil {
		log.Errorf("%s.failed to create rebuild tmp file: [%v]", o.name, err)
		return
	}
	defer os.Remove(filePath)
	tmpFile.Close()

	tried := 0
	for ; tried <= 5; tried++ {
		if time.Now().UnixMilli()-o.lastCallFetcher < 1000 {
			time.Sleep(2 * time.Second)
		}
		if o.snapFetcher == nil {
			break // 部分交易所ws第一个消息返回snap，后续返回增量
		}
		snapSlot, err := o.snapFetcher()
		o.lastCallFetcher = time.Now().UnixMilli()

		if err != nil {
			log.Warnf("%s.failed to fetch snap, %v", o.name, err)
			time.Sleep(time.Second)
			continue
		}
		if snapSlot == nil { // ws req类则返回nil
			break
		}
		// log.Debugf("partial. got snap to rebuild, seq:%v, snap:%s", snapSlot.ExPrevLastId, snapSlot.String())
		o.lock.Lock()
		// o.asks = newAsks()
		// o.bids = newBids()
		// o.handleSlot(snapSlot)
		o.rebuildWithSlot(snapSlot, tsMs)
		o.updateId = snapSlot.ExPrevLastId // works for bn swap, but not okx
		// 遍历dual link 更新
		matched := false
		for {
			slot := o.dualLink.GetFirst()
			if slot == nil {
				break
			}
			// log.Debugf("partial. slot: %s", slot.String())
			// todo 抽象通用？
			if slot.ExLastId < snapSlot.ExPrevLastId {
				o.dualLink.RemoveFirst()
				o.wrongTimeMs = 0 // 避免dualLink中新的数据还没插上来，频繁触发rebuild
				continue
			}
			// if n.ExFirstId <= slot.ExPrevLastId && n.ExLastId >= slot.ExPrevLastId {
			if !matched && o.firstMatchJudger(snapSlot, slot) {
				// log.Debugf("partial. first matched")
				o.handleSlot(slot, tsMs)
				o.dualLink.RemoveFirst()
				matched = true
				o.wrongTimeMs = 0
			} else {
				// if !matched || slot.ExPrevLastId != o.seq {
				if !matched || !o.connectionJudger(matched, o.updateId, slot) {
					// 链接错误，或者go协程调度的原因还没插上来
					// log.Debugf("partial. connect wrong, %v, %v, %v", matched, o.connectionJudger(matched, o.seq, slot), o.wrongTimeMs)
					if o.wrongTimeMs == 0 {
						o.wrongTimeMs = time.Now().UnixMilli()
						// 不应删除链头
					}
					break
				} else {
					// 正常更新
					// log.Debugf("partial. update normal")
					o.handleSlot(slot, tsMs)
					o.dualLink.RemoveFirst()
					matched = true

					if o.wrongTimeMs > 0 {
						o.wrongTimeMs = 0
					}
				}
			}
		}
		o.lock.Unlock()
		// 重建成功，重置失败计数器
		o.rebuildFailCount.Store(0)
		break
	}

	if tried >= 5 {
		log.Errorf("%s.failed to rebuild after 5 times", o.name)
		//o.handleRebuildFail()
		return
	}

	if o.snapFetchType == SnapFetchType_Rs { // ws类型还在异步等待snap回报
		o.rebuilding = false
	}
}

// func (o *Orderbook) handleRebuildFail() {
// 	failCount := o.rebuildFailCount.Add(1)
// 	log.Warnf("%s orderbook重建失败,当前连续失败次数: %d", o.name, failCount)

// 	// 如果连续重建失败超过3次，通过回调函数重启ws
// 	if failCount >= 3 && o.onWSRestart != nil {
// 		log.Errorf("%s orderbook连续重建失败%d次,触发WebSocket重启", o.name, failCount)
// 		go o.onWSRestart()
// 	}
// }

func (o *Orderbook) UpdateId() int64 {
	return o.updateId
}
func (o *Orderbook) Seq() int64 {
	return o.seq
}
func (o *Orderbook) InsertSlotToChan(s *Slot) {
	// log.Debugf("InsertSlotToChan %s", s.String())
	o.slotChan <- s
}
func (o *Orderbook) InsertSlot(s *Slot, tsMsList ...int64) bool {
	// log.Debugf("InsertSlot %s", s.String())
	// 高并发且交易所乱序的场景下，会出现已经接收到旧数据的协程一直抢不到锁，导致接不上订单簿。这时要用chan模式
	var tsMs int64
	if len(tsMsList) > 0 {
		tsMs = tsMsList[0]
	} else {
		tsMs = time.Now().UnixMilli()
	}
	o.lock.Lock()
	defer o.lock.Unlock()
	// log.Debugf("partial. gonna insert slot, %s", s.String())

	if s.IsSnap && o.eraseOnceSnapShow {
		o.dualLink.CleanAll()
	}

	if err := o.dualLink.Push(s); err != nil {
		helper.LogErrorThenCall(fmt.Sprintf("failed to insert slot. %s, slot:%s, err:%v", o.name, s.Summary(), err), helper_ding.DingingSendSerious)
		return false
	}

	// log.Debugf("partial. ob status, rebuilding %v, sft %v, wrongTimeMs %v", o.rebuilding, o.snapFetchType, o.wrongTimeMs)
	if o.rebuilding {
		if o.snapFetchType == SnapFetchType_Rs {
			return false
		}
		// Ws类型snap，从右向左遍历链表
		if !s.IsSnap {
			// 已经独占锁插入，如果新的slot不是snap，队列一定没有snap
			o.waitSnapCnt++
			if o.waitSnapCnt > 500 {
				log.Warnf("partial. too long no see snap, gonna rebuild. ", o.name)
				go o.rebuild(tsMs)
				o.waitSnapCnt = 0
			}
			return false
		}
		// todo 找到snap slot后，对于左边那些slot，有些ws需要删除，有些则需要判断使用，以下先简单处理为都删除 kc@2023-09-08
		// snapSlot := o.dualLink.RFindSnap()
		// if snapSlot == nil {
		// 	return false
		// }
		// 2024-1-9 不用找，一定是当前slot
		snapSlot := s
		log.Infof("partial. found snap in queue. ", o.name)
		o.dualLink.RemoveLeftOf(snapSlot)
		// o.asks = newAsks()
		// o.bids = newBids()
		// o.handleSlot(snapSlot)
		o.rebuildWithSlot(snapSlot, tsMs)
		o.dualLink.RemoveFirst()
		o.wrongTimeMs = 0
		o.rebuilding = false
		return false
	}

	needRebuild := o.wrongTimeMs != 0 && o.wrongTimeMs+_WRONG_TIMEOUT_MS < time.Now().UnixMilli()
	if needRebuild {
		o.rebuilding = true
		log.Warnf("partial. gonna rebuild. ", o.name)
		go o.rebuild(tsMs)
		return false
	}

	orderbookUpdated := false
	for {
		slot := o.dualLink.GetFirst()
		if slot == nil {
			// log.Debugf("partial. slot nil")
			break
		}
		if slot.IsSnap {
			// o.asks = newAsks()
			// o.bids = newBids()
			// o.handleSlot(slot)
			o.rebuildWithSlot(slot, tsMs)
			o.dualLink.RemoveFirst()
			if o.wrongTimeMs > 0 {
				o.wrongTimeMs = 0
			}
			orderbookUpdated = true
			continue
		}
		if slot.ExLastId < o.updateId {
			o.dualLink.RemoveFirst()
			o.wrongTimeMs = 0 // 避免dualLink中新的数据还没插上来，频繁触发rebuild
			// if o.wrongTimeMs == 0 {
			// o.wrongTimeMs = time.Now().UnixMilli() // 避免dualLink中新的数据还没插上来，频繁触发rebuild
			// }
			continue
		} else if !o.connectionJudger(true, o.updateId, slot) {
			// 链接错误，或者go协程调度的原因还没插上来
			// log.Debugf("partial. connect wrong, connected %v, updateId %v, slot.ExLastId %v, wrongTimeMs %v", o.connectionJudger(true, o.seq, slot), o.updateId, slot.ExLastId, o.wrongTimeMs)
			if o.wrongTimeMs == 0 {
				o.wrongTimeMs = time.Now().UnixMilli()
			}
			break
		} else {
			// 正常更新
			// log.Debugf("partial. update normal")
			orderbookUpdated = true
			o.handleSlot(slot, tsMs)
			o.dualLink.RemoveFirst()

			if o.wrongTimeMs > 0 {
				o.wrongTimeMs = 0
			}

		}
	}
	if !o.disableAutoUpdateDepth && orderbookUpdated && o.depthSubLevel >= 0 { // 改由应用层调用控制
		askLevel := o.depthSubLevel
		bidLevel := o.depthSubLevel
		if o.depthSubLevel == 0 {
			askLevel = o.asks.length
			bidLevel = o.bids.length
		}
		o.depth.Lock.Lock()
		//
		if cap(o.depth.Asks) >= askLevel {
			o.depth.Asks = o.depth.Asks[:0]
		} else {
			o.depth.Asks = make([]helper.DepthItem, 0, askLevel)
		}
		if cap(o.depth.Bids) >= bidLevel {
			o.depth.Bids = o.depth.Bids[:0]
		} else {
			o.depth.Bids = make([]helper.DepthItem, 0, bidLevel)
		}
		it := o.bids.Iterator()
		it.Next()
		for i := 0; i < bidLevel; i++ {
			o.depth.Bids = append(o.depth.Bids, helper.DepthItem{Price: it.Key(), Amount: it.Value().Amount})
			if !it.Next() {
				break
			}
		}
		it = o.asks.Iterator()
		it.Next()
		for i := 0; i < askLevel; i++ {
			o.depth.Asks = append(o.depth.Asks, helper.DepthItem{Price: it.Key(), Amount: it.Value().Amount})
			if !it.Next() {
				break
			}
		}
		//log.Debugf("partial. bid price, %f, %f", o.depth.Bids[0].Price, o.depth.Bids[bidLevel-1].Price)
		//log.Debugf("partial. ask price, %f, %f", o.depth.Asks[0].Price, o.depth.Asks[askLevel-1].Price)
		o.depth.Lock.Unlock()
		// log.Debugf("partial. orderbook updated info, update id %d, seq %d", o.updateId, o.seq)
	}
	return orderbookUpdated

}
func (o *Orderbook) rebuildWithSlot(slot *Slot, tsMs int64) {
	prevAsks := o.asks
	prevBids := o.bids
	o.asks = newAsks()
	o.bids = newBids()
	o.handleSlot(slot, tsMs)
	{
		it := o.bids.Iterator()
		for it.Next() {
			old, exist := prevBids.Get(it.Key())
			if !exist {
				continue
			}
			v := it.ValuePtr()
			v.Extra = old.Extra
			if !almostEqual(v.Amount, old.Amount) {
				extraInfoUpdate(&v.Extra, v.Amount-old.Amount, tsMs)
			}
		}
	}
	{
		it := o.asks.Iterator()
		for it.Next() {
			old, exist := prevAsks.Get(it.Key())
			if !exist {
				continue
			}
			v := it.ValuePtr()
			v.Extra = old.Extra
			if !almostEqual(v.Amount, old.Amount) {
				extraInfoUpdate(&v.Extra, v.Amount-old.Amount, tsMs)
			}
		}
	}
}

func (o *Orderbook) handleSlot(slot *Slot, tsMs int64) {
	//log.Debugf("partial. gonna handleSlot %v", slot.ExLastId)
	bidUpdated := false
	if slot.PriceLevelWithSeq {
		for _, pa := range slot.PriceItemsWithSeq[0:slot.AskStartIdx] {
			o.bids.Update(pa[0], pa[1], tsMs)
			bidUpdated = true
		}
		for _, pa := range slot.PriceItemsWithSeq[slot.AskStartIdx:] {
			o.asks.Update(pa[0], pa[1], tsMs)
		}
	} else {
		for _, pa := range slot.PriceItems[0:slot.AskStartIdx] {
			o.bids.Update(pa[0], pa[1], tsMs)
			bidUpdated = true
		}
		for _, pa := range slot.PriceItems[slot.AskStartIdx:] {
			o.asks.Update(pa[0], pa[1], tsMs)
		}
	}
	// cut cross
	bestAsk := o.asks.header.next()
	bestBid := o.bids.header.next()
	if bestAsk != nil && bestBid != nil && bestAsk.key <= bestBid.key {
		if !bidUpdated {
			o.bids.RemoveFirstLessThan(bestAsk.key - 1e-9)
			// } else if !askUpdated {
			// o.asks.RemoveFirstLessThan(bestBid.key + 1e-9)
		} else { // 如果两边都更新，使用 asks
			o.asks.RemoveFirstLessThan(bestBid.key + 1e-9)
		}
	}

	//log.Debugf("partial.sl bid price, %f, %f", o.bids.header.key, o.bids.footer.key)
	//log.Debugf("partial.sl ask price, %f, %f", o.asks.header.key, o.asks.footer.key)
	o.updateId = slot.ExLastId
	o.seq = slot.ExSeq
	o.ReceivedTsNs = slot.ReceivedTsNs
}
func (o *Orderbook) GetPriceItems() [][2]float64 {
	return nil
}
