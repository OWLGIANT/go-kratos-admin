package base_orderbook

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"

	"actor/helper/helper_ding"
	"actor/limit"
	"actor/third/log"
)

type Slot struct {
	SortIdx           int64 // 排序序列号，一般是内部系统的接收时间。如果交易所乱序发送，则需要用交易所提供的其他有序字段，否则无法保障订单簿正确性。
	ExTsMs            int64
	ReceivedTsNs      int64
	ExSeq             int64
	ExFirstId         int64
	ExLastId          int64
	ExPrevLastId      int64
	ExCurId           int64
	ExCheckSum        string
	prev              *Slot
	next              *Slot
	IsSnap            bool         // 是全量快照还是增量更新
	AskStartIdx       int          // 第一个ask的位置，0表示没有bid
	PriceItems        [][2]float64 // bid1, bid2 .... ask1, ask2
	PriceLevelWithSeq bool         // 每档价格带有seq序列号，例如dydx
	PriceItemsWithSeq [][3]float64 // bid1, bid2 .... ask1, ask2
}

func (s *Slot) reset() {
	s.AskStartIdx = 0
	s.PriceItems = s.PriceItems[:0]
	s.next = nil
	s.prev = nil

	s.SortIdx = 0
	s.ExTsMs = 0
	s.ExSeq = 0
	s.ExFirstId = 0
	s.ExLastId = 0
	s.ExPrevLastId = 0
	s.ExCurId = 0
	s.ExCheckSum = ""
	s.IsSnap = false
}
func (s *Slot) Equal(s1 *Slot) bool {
	t := s.SortIdx == s1.SortIdx &&
		s.ExSeq == s1.ExSeq &&
		s.ExFirstId == s1.ExFirstId &&
		s.ExLastId == s1.ExLastId &&
		s.ExPrevLastId == s1.ExPrevLastId &&
		s.ExCurId == s1.ExCurId &&
		s.ExCheckSum == s1.ExCheckSum &&
		s.IsSnap == s1.IsSnap &&
		s.AskStartIdx == s1.AskStartIdx &&
		s.PriceLevelWithSeq == s1.PriceLevelWithSeq
	if t && len(s.PriceItems) == len(s1.PriceItems) {
		for i := 0; i < len(s.PriceItems); i++ {
			if t {
				t = t && almostEqual(s.PriceItems[i][0], s1.PriceItems[i][0]) && almostEqual(s.PriceItems[i][1], s1.PriceItems[i][1])
			} else {
				return false
			}
		}
	}
	return t
}
func (s *Slot) Summary() string {
	str := bytes.Buffer{}
	str.WriteString("{")
	str.WriteString("ReceivedTsNs=")
	str.WriteString(strconv.Itoa(int(s.ReceivedTsNs)))
	str.WriteString(", SortIdx=")
	str.WriteString(strconv.Itoa(int(s.SortIdx)))
	str.WriteString(", ExTsMs=")
	str.WriteString(strconv.Itoa(int(s.ExTsMs)))
	str.WriteString(", ExSeq=")
	str.WriteString(strconv.Itoa(int(s.ExSeq)))
	str.WriteString(", ExFirstId=")
	str.WriteString(strconv.Itoa(int(s.ExFirstId)))
	str.WriteString(", ExLastId=")
	str.WriteString(strconv.Itoa(int(s.ExLastId)))
	str.WriteString(", ExPrevLastId=")
	str.WriteString(strconv.Itoa(int(s.ExPrevLastId)))
	str.WriteString(", ExCurId=")
	str.WriteString(strconv.Itoa(int(s.ExCurId)))
	str.WriteString(", ExCheckSum=")
	str.WriteString(s.ExCheckSum)
	str.WriteString(", IsSnap=")
	str.WriteString(strconv.FormatBool(s.IsSnap))
	str.WriteString(", AskStartIdx=")
	str.WriteString(strconv.Itoa(s.AskStartIdx))
	str.WriteString("}")
	return str.String()
}

func (s *Slot) String() string {
	str := bytes.Buffer{}
	str.WriteString("Slot{")
	str.WriteString("ReceivedTsNs=")
	str.WriteString(strconv.Itoa(int(s.ReceivedTsNs)))
	str.WriteString(", SortIdx=")
	str.WriteString(strconv.Itoa(int(s.SortIdx)))
	str.WriteString(", ExTsMs=")
	str.WriteString(strconv.Itoa(int(s.ExTsMs)))
	str.WriteString(", ExSeq=")
	str.WriteString(strconv.Itoa(int(s.ExSeq)))
	str.WriteString(", ExFirstId=")
	str.WriteString(strconv.Itoa(int(s.ExFirstId)))
	str.WriteString(", ExLastId=")
	str.WriteString(strconv.Itoa(int(s.ExLastId)))
	str.WriteString(", ExPrevLastId=")
	str.WriteString(strconv.Itoa(int(s.ExPrevLastId)))
	str.WriteString(", ExCurId=")
	str.WriteString(strconv.Itoa(int(s.ExCurId)))
	str.WriteString(", ExCheckSum=")
	str.WriteString(s.ExCheckSum)
	str.WriteString(", IsSnap=")
	str.WriteString(strconv.FormatBool(s.IsSnap))
	str.WriteString(", AskStartIdx=")
	str.WriteString(strconv.Itoa(s.AskStartIdx))
	str.WriteString(", PriceItems=[")
	for _, item := range s.PriceItems {
		str.WriteString("[")
		str.WriteString(strconv.FormatFloat(item[0], 'f', -1, 64))
		str.WriteString(",")
		str.WriteString(strconv.FormatFloat(item[1], 'f', -1, 64))
		str.WriteString("],")
	}
	str.WriteString("]")
	str.WriteString("}")

	return str.String()
}

type PriceItem struct {
	Price  float64
	Amount float64
}

type SortedDualLink struct {
	// Lock  sync.Mutex
	queue *DualList
	pool  sync.Pool
}

func NewSortedDualLink() *SortedDualLink {
	return &SortedDualLink{
		queue: NewDualList(),
	}
}

func (d *SortedDualLink) Push(s *Slot) error {
	// d.Lock.Lock()
	// defer d.Lock.Unlock()
	return d.queue.RPush(s)
}

// 移除s左边不包含s的元素
func (d *SortedDualLink) RemoveLeftOf(s *Slot) {
	sp := s.prev
	for sp != nil {
		p := sp.prev
		sp.reset()
		d.pool.Put(sp)
		d.queue.len--
		sp = p
	}
	d.queue.head = s
	d.queue.head.prev = nil
}

func (d *SortedDualLink) RFindSnap() *Slot {
	slot := d.queue.tail
	for slot != nil {
		if slot.IsSnap {
			return slot
		}
		slot = slot.prev
	}
	return slot
}

func (d *SortedDualLink) GetFreeSlot() *Slot {
	r := d.pool.Get()
	if r == nil {
		return &Slot{
			PriceItems: make([][2]float64, 0, 10),
		}
	}
	return r.(*Slot)
}

func (d *SortedDualLink) GetFirst() *Slot {
	return d.queue.head
}

func (d *SortedDualLink) CleanAll() {
	s := d.queue.LPop()
	for s != nil {
		s.reset()
		d.pool.Put(s)
		s = d.queue.LPop()
	}
	return
}
func (d *SortedDualLink) RemoveFirst() {
	s := d.queue.LPop()
	if s == nil {
		return
	}
	s.reset()
	d.pool.Put(s)
}

type DualList struct {
	head            *Slot
	tail            *Slot
	len             int
	ignoreDuplicate bool // 出现重复slot时，忽略。phemex seq会是这种情况，但它60秒发一次 snap，所以错误是短暂的
}

func NewDualList() (list *DualList) {
	list = &DualList{}
	return
}

func (l *DualList) SetIgnoreDuplicate() {
	l.ignoreDuplicate = true
}

// func (l *DualList) Head() *Slot {
// 	return l.head
// }

// func (l *DualList) Tail() *Slot {
// 	return l.tail
// }

// func (l *DualList) Len() int {
// 	return l.len
// }

// todo 这里性能显示，有时会用了很多时间，可以指标生产监控
func (l *DualList) RPush(node *Slot) error {
	if node.SortIdx == 0 {
		return fmt.Errorf("sortIdx is zero")
	}
	if l.len == 0 {
		l.head = node
		l.tail = node
	} else {
		// log.Debug("partial. seeking for ", node.SortIdx)
		if node.SortIdx > l.tail.SortIdx {
			// log.Debug("partial. instead oldtail ", node.SortIdx, " ", l.oldtail.SortIdx)
			oldtail := l.tail
			oldtail.next = node
			node.prev = oldtail
			l.tail = node
		} else if node.SortIdx < l.head.SortIdx {
			// log.Debug("partial. instead oldhead ", node.SortIdx, " ", l.oldhead.SortIdx)
			oldhead := l.head
			oldhead.prev = node
			node.next = oldhead
			l.head = node
		} else {
			// 从右往左找到正确位置插入
			start := l.tail
			for i := start; i != nil; i = i.prev {
				// log.Debug("partial. seek at ", i.SortIdx)
				if node.SortIdx == i.SortIdx {
					if l.ignoreDuplicate {
						return nil
					}
					e := node.Equal(i)
					err := fmt.Errorf("duplicated slot, equal:%v", e)
					if !e {
						log.Warnf("partial. node same SortIdx but not equal. pending node: %v, in list: %v", node.String(), i.String())
					}
					return err
				}

				// l.tryDeference(node, i, i.prev)

				if node.SortIdx < i.SortIdx && node.SortIdx > i.prev.SortIdx {
					// log.Debug("partial. inserted ", i.prev.SortIdx, " ", i.SortIdx, " ", node.SortIdx)
					node.next = i
					node.prev = i.prev
					i.prev.next = node
					i.prev = node
					break
				}
			}
		}
	}
	l.len++
	return nil
}

func (l *DualList) LPop() (node *Slot) {
	if l.len == 0 {
		return
	}

	node = l.head
	if node.next == nil {
		l.head = nil
		l.tail = nil
		l.len = 0
	} else {
		node.next.prev = nil
		l.head = node.next
		l.len--
	}
	return
}

func (l *DualList) tryDeference(pendingNode, cur, prev *Slot) {
	ok := false
	defer func() {
		if !ok {
			log.Warnf("utf tryDef error")
			log.Warnf("pendingNode %p, s %v", pendingNode, pendingNode.Summary())
			log.Warnf("iter cur %p, s %v", cur, cur.Summary())
			log.Warnf("prev %p, ", prev)
			if err := recover(); err != nil {
				l.dump()
				log.Error(err)
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				log.Errorf("==> %s\n", string(buf[:n]))
				helper_ding.DingingSendSerious(fmt.Sprintf("got fish. %v", limit.GetMyIP()[0]))
			}
		}
	}()
	if prev.SortIdx == 0 {
		fmt.Println("test")
	}
	ok = true
}

func (l *DualList) dump() {
	// log.Warnf("DualList dump. %v %v %v %v, %v", l.len, l.head.SortIdx, l.tail.SortIdx, node.Summary(), i.SortIdx)
	log.Warnf("DualList dump. %v %v %v ", l.len, l.head.SortIdx, l.tail.SortIdx)
	{
		s := l.head
		i := -1
		log.Warnf("from left:")
		for s != nil {
			i++
			log.Warnf("slot %d, %p. summry %v, prev %p, next %p", i, s, s.Summary(), s.prev, s.next)
			s = s.next
		}
	}
	{
		s := l.tail
		i := -1
		log.Warnf("from right:")
		for s != nil {
			i++
			log.Warnf("slot %d, %p. summry %v, prev %p, next %p", i, s, s.Summary(), s.prev, s.next)
			s = s.prev
		}
	}
}
