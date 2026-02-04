package rollingma_v2

// RollingMa
//
//	capacity is the numbers of this queue's ability (capacity - 1)
type RollingMa struct {
	capacity int
	elements []float64
	first    int
	end      int
	len      int
	sum      float64
	avg      float64
}

// NewCircularQueue create new CircularQueue passing a integer as its size
// and return its pointer
func NewCircularQueue(size int) *RollingMa {
	cq := RollingMa{capacity: size + 1, first: 0, end: 0}
	cq.elements = make([]float64, cq.capacity)
	return &cq
}

// IsEmpty return if this queue is empty
func (c *RollingMa) IsEmpty() bool {
	return c.first == c.end
}

// IsFull return if this queue is full
func (c *RollingMa) IsFull() bool {
	return c.first == (c.end+1)%c.capacity
}

func (c *RollingMa) Update(e float64) float64 {
	c.len++
	if c.IsFull() {
		c.sum -= c.elements[c.first]
		c.first = (c.first + 1) % c.capacity
		c.len = c.capacity - 1
	}
	c.elements[c.end] = e
	c.end = (c.end + 1) % c.capacity
	c.sum += e
	c.avg = c.sum / float64(c.len)
	return c.avg
}

func (c *RollingMa) Len() int {
	return c.len
}

// Shift shift a element witch pushed earlist
// note: will return empty if this queue is empty
func (c *RollingMa) Shift() (e float64) {
	if c.IsEmpty() {
		c.len = 0
		var t float64
		return t
	}
	c.len--
	e = c.elements[c.first]
	c.first = (c.first + 1) % c.capacity
	return
}

func (c *RollingMa) GetElement(idx int) (e float64) {
	if idx >= c.len {
		var t float64
		return t
	}
	idx = (c.first + idx) % c.capacity
	return c.elements[idx]
}

func (c *RollingMa) ShiftAll() []float64 {
	res := make([]float64, c.len)
	l := c.len
	var t float64
	for i := 0; i < l; i++ {
		v := c.Shift()
		if v == t {
			break
		}
		res[i] = v
	}
	return res
}
