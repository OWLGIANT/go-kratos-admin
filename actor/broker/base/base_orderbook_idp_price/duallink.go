package base_orderbook_idp_price

import (
	"bytes"
	"strconv"
	"sync"
)

type ListItemWithSeq struct {
	Key   float64
	Value ValueType
}

type Slot struct {
	ReceivedTsNs      int64
	ExTsMs            int64
	ExSeq             int64
	ExFirstId         int64
	ExLastId          int64
	ExPrevLastId      int64
	ExCurId           int64
	ExCheckSum        string
	prev              *Slot
	next              *Slot
	IsSnap            bool              // 是全量快照还是增量更新
	AskStartIdx       int               // 第一个ask的位置，0表示没有bid
	PriceItems        [][2]float64      // bid1, bid2 .... ask1, ask2
	PriceLevelWithSeq bool              // 每档价格带有seq序列号，例如dydx
	PriceItemsWithSeq []ListItemWithSeq // bid1, bid2 .... ask1, ask2
}

func (s *Slot) reset() {
	s.AskStartIdx = 0
	s.PriceItems = s.PriceItems[:0]
	s.PriceItemsWithSeq = s.PriceItemsWithSeq[:0]
	s.next = nil
	s.prev = nil

	s.ReceivedTsNs = 0
	s.ExTsMs = 0
	s.ExSeq = 0
	s.ExFirstId = 0
	s.ExLastId = 0
	s.ExPrevLastId = 0
	s.ExCurId = 0
	s.ExCheckSum = ""
	s.IsSnap = false
}

func (s *Slot) String() string {
	str := bytes.Buffer{}
	str.WriteString("Slot{")
	str.WriteString("ReceivedTsNs=")
	str.WriteString(strconv.Itoa(int(s.ReceivedTsNs)))
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

func (d *SortedDualLink) Push(s *Slot) {
	// d.Lock.Lock()
	// defer d.Lock.Unlock()
	d.queue.RPush(s)
}

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

func (d *SortedDualLink) RemoveFirst() {
	s := d.queue.LPop()
	if s == nil {
		return
	}
	s.reset()
	d.pool.Put(s)
}

type DualList struct {
	head *Slot
	tail *Slot
	len  int
}

func NewDualList() (list *DualList) {
	list = &DualList{}
	return
}

func (l *DualList) Head() *Slot {
	return l.head
}

func (l *DualList) Tail() *Slot {
	return l.tail
}

func (l *DualList) Len() int {
	return l.len
}

func (l *DualList) RPush(node *Slot) {
	if l.Len() == 0 {
		l.head = node
		l.tail = node
	} else {

		if node.ReceivedTsNs > l.tail.ReceivedTsNs {
			tail := l.tail
			tail.next = node
			node.prev = tail
			l.tail = node
		} else if node.ReceivedTsNs < l.head.ReceivedTsNs {
			head := l.head
			head.prev = node
			node.next = head
			l.head = node
		} else {
			// 从右往左找到正确位置插入
			for i := l.tail; i != nil; i = i.prev {
				if node.ReceivedTsNs <= i.ReceivedTsNs && node.ReceivedTsNs >= i.prev.ReceivedTsNs {
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
}

func (l *DualList) LPop() (node *Slot) {
	if l.len == 0 {
		return
	}

	node = l.head
	if node.next == nil {
		l.head = nil
		l.tail = nil
	} else {
		l.head = node.next
	}
	l.len--
	return
}
