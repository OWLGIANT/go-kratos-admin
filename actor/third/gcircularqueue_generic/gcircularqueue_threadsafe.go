package gcircularqueue_generic

import (
	"sync"
)

// CircularQueueThreadSafe
// composing sync.RWMutex and pointer of CircularQueue
type CircularQueueThreadSafe[T comparable] struct {
	sync.RWMutex
	*CircularQueue[T]
}

// NewCircularQueueThreadSafe return a new NewCircularQueueThreadSafe
func NewCircularQueueThreadSafe[T comparable](size int) *CircularQueueThreadSafe[T] {
	return &CircularQueueThreadSafe[T]{CircularQueue: NewCircularQueue[T](size)}
}

// IsEmpty return cq.CircularQueue.IsEmpty() wrapped by RLock
func (cq *CircularQueueThreadSafe[T]) IsEmpty() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.CircularQueue.IsEmpty()
}

// IsFull return cq.CircularQueue.IsFull() wrapped by RLock
func (cq *CircularQueueThreadSafe[T]) IsFull() bool {
	cq.RLock()
	defer cq.RUnlock()
	return cq.CircularQueue.IsFull()
}

// Push push a element into cq.CircularQueue wrapped by Lock
func (cq *CircularQueueThreadSafe[T]) Push(element T) {
	cq.Lock()
	defer cq.Unlock()
	cq.CircularQueue.Push(element)
}

// Push pushing a element to this queue
// note: if pushing into a full queue, it will kick oldest
func (cq *CircularQueueThreadSafe[T]) PushKick(e T) {
	cq.Lock()
	defer cq.Unlock()
	cq.CircularQueue.PushKick(e)
}

func (cq *CircularQueueThreadSafe[T]) Len() int {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.len
}

// Shift shift a element from cq.CircularQueue wrapped by Lock
func (cq *CircularQueueThreadSafe[T]) Shift() T {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.Shift()
}

func (cq *CircularQueueThreadSafe[T]) ShiftAll() []T {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.ShiftAll()
}

func (cq *CircularQueueThreadSafe[T]) GetElement(idx int) T {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.GetElement(idx)
}

func (cq *CircularQueueThreadSafe[T]) GetAllElements() (e []T) {
	cq.Lock()
	defer cq.Unlock()
	return cq.CircularQueue.elements
}
