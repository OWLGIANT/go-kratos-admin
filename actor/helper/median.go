package helper

import "container/heap"

// 小顶堆（存储较大一半的元素）
type MinHeap []float64

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(float64))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// 大顶堆（存储较小一半的元素）
type MaxHeap []float64

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i] > h[j] } // 反向排序
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x interface{}) {
	*h = append(*h, x.(float64))
}

func (h *MaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// RollingMedian 结构体
type RollingMedian struct {
	minHeap MinHeap // 小顶堆
	maxHeap MaxHeap // 大顶堆
	window  []float64
	size    int
}

// NewRollingMedian 创建一个滚动窗口中位数计算器
func NewRollingMedian(size int) *RollingMedian {
	return &RollingMedian{
		minHeap: MinHeap{},
		maxHeap: MaxHeap{},
		window:  []float64{},
		size:    size,
	}
}

// Add 插入新元素，并保持堆的平衡
func (rm *RollingMedian) Update(num float64) float64 {
	// 插入新元素
	rm.window = append(rm.window, num)

	// 保持窗口大小
	if len(rm.window) > rm.size {
		rm.removeOldest()
	}

	// 插入到合适的堆
	if len(rm.maxHeap) == 0 || num <= rm.maxHeap[0] {
		heap.Push(&rm.maxHeap, num)
	} else {
		heap.Push(&rm.minHeap, num)
	}

	// 平衡两个堆的大小
	if len(rm.maxHeap) > len(rm.minHeap)+1 {
		heap.Push(&rm.minHeap, heap.Pop(&rm.maxHeap))
	} else if len(rm.minHeap) > len(rm.maxHeap) {
		heap.Push(&rm.maxHeap, heap.Pop(&rm.minHeap))
	}

	return rm.Median()
}

// removeOldest 移除窗口中最老的元素
func (rm *RollingMedian) removeOldest() {
	if len(rm.window) == 0 {
		return
	}
	oldest := rm.window[0]
	rm.window = rm.window[1:]

	// 从对应的堆中移除元素
	if oldest <= rm.maxHeap[0] {
		rm.removeFromHeap(&rm.maxHeap, oldest)
	} else {
		rm.removeFromHeap(&rm.minHeap, oldest)
	}

	// 平衡两个堆的大小
	if len(rm.maxHeap) > len(rm.minHeap)+1 {
		heap.Push(&rm.minHeap, heap.Pop(&rm.maxHeap))
	} else if len(rm.minHeap) > len(rm.maxHeap) {
		heap.Push(&rm.maxHeap, heap.Pop(&rm.minHeap))
	}
}

// removeFromHeap 从堆中移除指定元素
func (rm *RollingMedian) removeFromHeap(h heap.Interface, value float64) {
	switch h := h.(type) {
	case *MinHeap:
		for i, v := range *h {
			if v == value {
				heap.Remove(h, i)
				return
			}
		}
	case *MaxHeap:
		for i, v := range *h {
			if v == value {
				heap.Remove(h, i)
				return
			}
		}
	default:
		panic("unexpected heap type")
	}
}

// Median 计算当前窗口的中位数
func (rm *RollingMedian) Median() float64 {
	if len(rm.maxHeap) == len(rm.minHeap) {
		return (rm.maxHeap[0] + rm.minHeap[0]) / 2.0
	}
	return rm.maxHeap[0]
}
