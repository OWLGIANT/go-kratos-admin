package rolling_quantile

import (
	"container/list"
	"github.com/petar/GoLLRB/llrb"
	"math"
)

type Item struct {
	value float64
	id    int
}

func (a Item) Less(b llrb.Item) bool {
	other := b.(Item)
	if a.value == other.value {
		return a.id < other.id
	}
	return a.value < other.value
}

type RollingQuantile struct {
	tree   *llrb.LLRB
	window *list.List
	size   int
	id     int
}

// NewRollingQuantile 要特别注意 quantile 取 0.9 0.99 0.999 准确 但是取 0.001 不准确 可以计算 -value的 quantile 在 负号 间接获得 0.001
func NewRollingQuantile(size int) *RollingQuantile {
	return &RollingQuantile{
		tree:   llrb.New(),
		window: list.New(),
		size:   size,
		id:     0,
	}
}

func (rq *RollingQuantile) Update(value float64) {
	item := Item{value: value, id: rq.id}
	rq.id++
	rq.tree.InsertNoReplace(item)
	rq.window.PushBack(item)

	if rq.window.Len() > rq.size {
		oldest := rq.window.Remove(rq.window.Front()).(Item)
		rq.tree.Delete(oldest)
	}
}

// Quantile 计算当前窗口中的指定分位数（q 范围为 0 到 1）
func (rq *RollingQuantile) Quantile(q float64) float64 {
	if rq.tree.Len() == 0 {
		return 0
	}
	k := int(math.Ceil(q * float64(rq.tree.Len())))
	i := 0
	var result float64
	rq.tree.AscendGreaterOrEqual(rq.tree.Min(), func(it llrb.Item) bool {
		if i == k-1 {
			result = it.(Item).value
			return false
		}
		i++
		return true
	})
	return result
}
