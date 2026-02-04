package rollingma_v2

import (
	"sync"
)

// RollingMaThreadSafe
// composing sync.RWMutex and pointer of CircularQueue
type RollingMaThreadSafe struct {
	sync.RWMutex
	*RollingMa
}

// NewCircularQueueThreadSafe return a new NewCircularQueueThreadSafe
func NewCircularQueueThreadSafe(size int) *RollingMaThreadSafe {
	return &RollingMaThreadSafe{RollingMa: NewCircularQueue(size)}
}

// IsEmpty return cq.CircularQueue.IsEmpty() wrapped by RLock
func (cq *RollingMaThreadSafe) IsEmpty() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.RollingMa.IsEmpty()
}

// IsFull return cq.CircularQueue.IsFull() wrapped by RLock
func (cq *RollingMaThreadSafe) IsFull() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.RollingMa.IsFull()
}

func (cq *RollingMaThreadSafe) Update(e float64) float64 {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.Update(e)
}

func (cq *RollingMaThreadSafe) Len() int {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.len
}

// Shift shift a element from cq.CircularQueue wrapped by Lock
func (cq *RollingMaThreadSafe) Shift() float64 {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.Shift()
}

func (cq *RollingMaThreadSafe) ShiftAll() []float64 {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.ShiftAll()
}

func (cq *RollingMaThreadSafe) GetElement(idx int) interface{} {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.GetElement(idx)
}

func (cq *RollingMaThreadSafe) GetAllElements() (e []float64) {
	cq.Lock()
	defer cq.Unlock()
	return cq.RollingMa.elements
}
