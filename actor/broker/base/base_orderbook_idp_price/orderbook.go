package base_orderbook_idp_price

// price level独立更新的orderbook 版本，目前2023-12-30只有dydx一个所有这个问题，日后看情况跟normal 版本orderbook合并，idp_price版本已经尽量兼容normal版本

import (
	"math"
	"sync"
	"time"

	"actor/helper"
	"actor/third/log"
)

type Orderbook struct {
	pool     sync.Pool
	skipList []any
	dualLink *SortedDualLink
	lock     sync.Mutex

	firstMatchJudger func(snapSlot *Slot, s *Slot) bool
	connectionJudger func(firstMatched bool, seq int64, s *Slot) bool
	snapFetcher      func() (*Slot, error)
	snapFetchType    int
	onExit           func(string)
	depth            *helper.Depth
	depthSubLevel    int // 订单薄档数，默认返回全部

	asks                 *SkipList
	bids                 *SkipList
	seq                  int64
	wrongTimeMs          int64 // 出现错误时间，等待几秒后协程还没补上，认为真正错误
	rebuilding           bool
	isOutOfOrderReceived bool
}

const _WRONG_TIMEOUT_MS = 3000 // 超过这个秒数没连接正确，认为错误，需要重建

const (
	SnapFetchType_Rs               = 1 + iota
	SnapFetchType_WsConnectWithSeq // 通过ws req获取snap，并且由seq保证连续
	SnapFetchType_WsConnectWithTs
)

const epsilon = 1e-10

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}
func newAsks(isOutOfOrderReceived bool) *SkipList {
	return NewCustomMap(isOutOfOrderReceived, func(l, r float64) bool {
		return l < r
	}, almostEqual)
}
func newBids(isOutOfOrderReceived bool) *SkipList {
	return NewCustomMap(isOutOfOrderReceived, func(l, r float64) bool {
		return l > r
	}, almostEqual)
}

/*
@params connectionJudger: seq, order_book当前的序列号; s 将要插入的slot
*/
func NewOrderbook(isOutOfOrderReceived bool, firstMatchJudger func(snapSlot *Slot, s *Slot) bool, connectionJudger func(firstMatched bool, seq int64, s *Slot) bool, snapFetcher func() (*Slot, error), snapFetchType int, //
	onExit func(string), depth *helper.Depth) *Orderbook {
	return &Orderbook{
		dualLink:             NewSortedDualLink(),
		firstMatchJudger:     firstMatchJudger,
		connectionJudger:     connectionJudger,
		snapFetcher:          snapFetcher,
		snapFetchType:        snapFetchType,
		onExit:               onExit,
		depth:                depth,
		asks:                 newAsks(isOutOfOrderReceived),
		bids:                 newBids(isOutOfOrderReceived),
		isOutOfOrderReceived: isOutOfOrderReceived,
	}
}
func (o *Orderbook) SetDepthSubLevel(l int) {
	o.depthSubLevel = l
}

func (o *Orderbook) GetFreeSlot(size int, receivedTsNs int64, priceLevelWithSeq ...bool) *Slot {
	s := o.dualLink.GetFreeSlot()
	if len(priceLevelWithSeq) > 0 && priceLevelWithSeq[0] {
		if cap(s.PriceItemsWithSeq) < size {
			s.PriceItemsWithSeq = make([]ListItemWithSeq, 0, size)
		}
		s.PriceLevelWithSeq = true
	} else {
		if cap(s.PriceItems) < size {
			s.PriceItems = make([][2]float64, 0, size)
		}
	}
	s.ReceivedTsNs = receivedTsNs
	return s
}

func (o *Orderbook) rebuild() {
	tried := 0
	for ; tried <= 5; tried++ {
		snapSlot, err := o.snapFetcher()
		if err != nil {
			log.Warnf("failed to fetch snap, %v", err)
			time.Sleep(time.Second)
			continue
		}
		if snapSlot == nil { // ws req类则返回nil
			break
		}
		//log.Debugf("partial. got snap to rebuild, seq:%v", snapSlot.ExPrevLastId)
		o.lock.Lock()
		defer o.lock.Unlock()
		o.asks = newAsks(o.isOutOfOrderReceived)
		o.bids = newBids(o.isOutOfOrderReceived)
		o.handleSlot(snapSlot)
		o.seq = snapSlot.ExPrevLastId // works for bn swap, but not okx
		// 遍历dual link 更新
		matched := false
		for {
			slot := o.dualLink.GetFirst()
			if slot == nil {
				break
			}
			// todo 抽象通用？
			if slot.ExLastId < snapSlot.ExPrevLastId {
				o.dualLink.RemoveFirst()
				continue
			}
			// if n.ExFirstId <= slot.ExPrevLastId && n.ExLastId >= slot.ExPrevLastId {
			if !matched && o.firstMatchJudger(snapSlot, slot) {
				//log.Debugf("partial. first matched")
				o.handleSlot(slot)
				o.dualLink.RemoveFirst()
				matched = true
				o.wrongTimeMs = 0
			} else {
				// if !matched || slot.ExPrevLastId != o.seq {
				if !matched || !o.connectionJudger(matched, o.seq, slot) {
					// 链接错误，或者go协程调度的原因还没插上来
					//log.Debugf("partial. connect wrong, %v, %v, %v", matched, o.connectionJudger(matched, o.seq, slot), o.wrongTimeMs)
					if o.wrongTimeMs == 0 {
						o.wrongTimeMs = time.Now().UnixMilli()
						// 不应删除链头
					}
					break
				} else {
					// 正常更新
					//log.Debugf("partial. update normal")
					o.handleSlot(slot)
					o.dualLink.RemoveFirst()
					matched = true

					if o.wrongTimeMs > 0 {
						o.wrongTimeMs = 0
					}

				}
			}
		}
		break
	}
	if tried >= 5 {
		helper.LogErrorThenCall("failed to fetch snap after 5 times", o.onExit)
	}
	if o.snapFetchType == SnapFetchType_Rs { // ws类型还在异步等待snap回报
		o.rebuilding = false
	}
}

func (o *Orderbook) InsertSlot(s *Slot) (isDepthUpdated bool) {
	o.lock.Lock()
	defer o.lock.Unlock()
	//log.Debugf("partial. gonna insert slot, %s", s.String())
	o.dualLink.Push(s)

	//log.Debugf("partial. ob status, rebuilding %v, sft %v, wrongTimeMs %v", o.rebuilding, o.snapFetchType, o.wrongTimeMs)
	if o.rebuilding {
		if o.snapFetchType == SnapFetchType_Rs {
			return false
		}
		// Ws类型snap，从右向左遍历链表
		// todo 找到snap slot后，对于左边那些slot，有些ws需要删除，有些则需要判断使用，以下先简单处理为都删除 kc@2023-09-08
		snapSlot := o.dualLink.RFindSnap()
		if snapSlot == nil {
			return false
		}
		// log.Debugf("partial. found snap in queue")
		o.dualLink.RemoveLeftOf(snapSlot)
		o.asks = newAsks(o.isOutOfOrderReceived)
		o.bids = newBids(o.isOutOfOrderReceived)
		o.handleSlot(snapSlot)
		// o.seq = snapSlot.ExPrevLastId // not works for okx, but bn swap will not run here
		o.seq = snapSlot.ExLastId
		o.dualLink.RemoveFirst()
		o.wrongTimeMs = 0
		o.rebuilding = false
		return false
	}

	needRebuild := o.wrongTimeMs != 0 && o.wrongTimeMs+_WRONG_TIMEOUT_MS < time.Now().UnixMilli()
	if needRebuild {
		o.rebuilding = true
		//log.Debugf("partial. gonna rebuild")
		go o.rebuild()
		return false
	}

	shouldUpdateDepth := false
	for {
		slot := o.dualLink.GetFirst()
		if slot == nil {
			//log.Debugf("partial. slot nil")
			break
		}
		if slot.IsSnap {
			o.asks = newAsks(o.isOutOfOrderReceived)
			o.bids = newBids(o.isOutOfOrderReceived)
			o.handleSlot(slot)
			// o.seq = slot.ExPrevLastId // not works for okx, but bn swap will not run here
			o.seq = slot.ExLastId
			o.dualLink.RemoveFirst()
			if o.wrongTimeMs > 0 {
				o.wrongTimeMs = 0
			}
			shouldUpdateDepth = true
			continue
		}
		if !o.connectionJudger(true, o.seq, slot) {
			//log.Debugf("partial. not connected, %v, %v", o.seq, slot.String())
			// 链接错误，或者go协程调度的原因还没插上来
			if o.wrongTimeMs == 0 {
				//log.Debugf("partial. set wrongTimeMs")
				o.wrongTimeMs = time.Now().UnixMilli()
				// 不应删除链头
			}
			break
		} else {
			// 正常更新
			//log.Debugf("partial. update normal")
			shouldUpdateDepth = true
			o.handleSlot(slot)
			o.dualLink.RemoveFirst()

			if o.wrongTimeMs > 0 {
				o.wrongTimeMs = 0
			}

		}
	}
	if shouldUpdateDepth {
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
		cnt := 0
		for it.Next() {
			if almostEqual(it.Value().Value, 0) {
				continue
			}
			o.depth.Bids = append(o.depth.Bids, helper.DepthItem{Price: it.Key(), Amount: it.Value().Value})
			cnt++
			if cnt >= o.depthSubLevel {
				break
			}
		}
		it = o.asks.Iterator()
		cnt = 0
		for it.Next() {
			if almostEqual(it.Value().Value, 0) { // todo 如果0单量的订单已经存在很长时间，清理
				continue
			}
			o.depth.Asks = append(o.depth.Asks, helper.DepthItem{Price: it.Key(), Amount: it.Value().Value})
			cnt++
			if cnt >= o.depthSubLevel {
				break
			}
		}
		//log.Debugf("partial. bid price, %f, %f", o.depth.Bids[0].Price, o.depth.Bids[bidLevel-1].Price)
		//log.Debugf("partial. ask price, %f, %f", o.depth.Asks[0].Price, o.depth.Asks[askLevel-1].Price)
		o.depth.Lock.Unlock()
	}
	return shouldUpdateDepth

}
func (o *Orderbook) handleSlot(slot *Slot) {
	//log.Debugf("partial. gonna handleSlot %v", slot.ExLastId)
	if slot.PriceLevelWithSeq {
		for _, pa := range slot.PriceItemsWithSeq[0:slot.AskStartIdx] {
			o.bids.Update(pa.Key, pa.Value)
		}
		for _, pa := range slot.PriceItemsWithSeq[slot.AskStartIdx:] {
			o.asks.Update(pa.Key, pa.Value)
		}
	} else {
		for _, pa := range slot.PriceItems[0:slot.AskStartIdx] {
			o.bids.Update(pa[0], ValueType{Value: pa[1]})
		}
		for _, pa := range slot.PriceItems[slot.AskStartIdx:] {
			o.asks.Update(pa[0], ValueType{Value: pa[1]})
		}
	}

	//log.Debugf("partial.sl bid price, %f, %f", o.bids.header.key, o.bids.footer.key)
	//log.Debugf("partial.sl ask price, %f, %f", o.asks.header.key, o.asks.footer.key)
	o.seq = slot.ExLastId
}
func (o *Orderbook) GetPriceItems() [][2]float64 {
	return nil
}
