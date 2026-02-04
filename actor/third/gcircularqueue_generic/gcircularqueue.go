package gcircularqueue_generic

// type ringable interface {
// string
// }
// CirCircularQueue
//
//	capacity is the numbers of this queue's ability (capacity - 1)
type CircularQueue[T comparable] struct {
	capacity int
	elements []T
	first    int
	end      int
	len      int
}

// NewCircularQueue create new CircularQueue passing a integer as its size
// and return its pointer
func NewCircularQueue[T comparable](size int) *CircularQueue[T] {
	cq := CircularQueue[T]{capacity: size + 1, first: 0, end: 0}
	cq.elements = make([]T, cq.capacity)
	return &cq
}

// IsEmpty return if this queue is empty
func (c *CircularQueue[T]) IsEmpty() bool {
	return c.first == c.end
}

// IsFull return if this queue is full
func (c *CircularQueue[T]) IsFull() bool {
	return c.first == (c.end+1)%c.capacity
}

// Push pushing a element to this queue
// note: if pushing into a full queue, it will panic
func (c *CircularQueue[T]) Push(e T) {
	if c.IsFull() {
		panic("Queue is full")
	}
	c.len++
	c.elements[c.end] = e
	c.end = (c.end + 1) % c.capacity
}

// Push pushing a element to this queue
// note: if pushing into a full queue, it will kick oldest
func (c *CircularQueue[T]) PushKick(e T) {
	c.len++
	if c.IsFull() {
		c.first = (c.first + 1) % c.capacity
		c.len = c.capacity - 1
	}
	c.elements[c.end] = e
	c.end = (c.end + 1) % c.capacity
}

func (c *CircularQueue[T]) Len() int {
	return c.len
}

// Shift shift a element witch pushed earlist
// note: if will return nil if this queue is empty
func (c *CircularQueue[T]) Shift() (e T) {
	if c.IsEmpty() {
		c.len = 0
		var empty T
		return empty
	}
	c.len--
	e = c.elements[c.first]
	c.first = (c.first + 1) % c.capacity
	return
}

// idx must < size, or default value will returned
func (c *CircularQueue[T]) GetElement(idx int) (e T) {
	if idx >= c.len {
		var empty T
		return empty
	}
	idx = (c.first + idx) % c.capacity
	return c.elements[idx]
}

func (c *CircularQueue[T]) ShiftAll() []T {
	res := make([]T, c.len)
	l := c.len
	for i := 0; i < l; i++ {
		v := c.Shift()
		var empty T
		if v == empty {
			break
		}
		res[i] = v
	}
	return res
}

func (c *CircularQueue[T]) GetAllElements() (e []T) {
	return c.elements
}
