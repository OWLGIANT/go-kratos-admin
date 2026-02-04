package gcircularqueue

import (
	"sync"
)

// CircularQueueThreadSafe
// composing sync.RWMutex and pointer of CircularQueue
type CircularQueueThreadSafe struct {
	sync.RWMutex
	*CircularQueue
}

// NewCircularQueueThreadSafe return a new NewCircularQueueThreadSafe
func NewCircularQueueThreadSafe(size int) *CircularQueueThreadSafe {
	return &CircularQueueThreadSafe{CircularQueue: NewCircularQueue(size)}
}

// IsEmpty return cq.CircularQueue.IsEmpty() wrapped by RLock
func (cq *CircularQueueThreadSafe) IsEmpty() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.CircularQueue.IsEmpty()
}

// IsFull return cq.CircularQueue.IsFull() wrapped by RLock
func (cq *CircularQueueThreadSafe) IsFull() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.CircularQueue.IsFull()
}

// Push push a element into cq.CircularQueue wrapped by Lock
func (cq *CircularQueueThreadSafe) Push(element interface{}) {
	cq.Lock()
	defer cq.Unlock()
	cq.CircularQueue.Push(element)
}

// Push pushing a element to this queue
// note: if pushing into a full queue, it will kick oldest
func (cq *CircularQueueThreadSafe) PushKick(e interface{}) {
	cq.Lock()
	defer cq.Unlock()
	cq.CircularQueue.PushKick(e)
}

func (cq *CircularQueueThreadSafe) Len() int {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.len
}

// Shift shift a element from cq.CircularQueue wrapped by Lock
func (cq *CircularQueueThreadSafe) Shift() interface{} {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.Shift()
}

func (cq *CircularQueueThreadSafe) ShiftAll() []interface{} {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.ShiftAll()
}

func (cq *CircularQueueThreadSafe) GetElement(idx int) interface{} {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.GetElement(idx)
}

func (cq *CircularQueueThreadSafe) GetAllElements() (e []interface{}) {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.elements
}
