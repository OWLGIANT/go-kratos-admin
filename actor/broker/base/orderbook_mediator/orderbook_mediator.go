package orderbook_mediator

type OrderbookMediator struct {
	updateDepth func(tsMsNow int64) error
	iterAsk     func(func(float64, Value) bool)
	iterBid     func(func(float64, Value) bool)
}

func NewOrderbookMediator() *OrderbookMediator {
	return &OrderbookMediator{}
}
func (om *OrderbookMediator) SetUpdateDepth(updateDepth func(tsMsNow int64) error) {
	om.updateDepth = updateDepth
}

func (om *OrderbookMediator) SetIterAsk(iterAsk func(func(float64, Value) bool)) {
	om.iterAsk = iterAsk
}
func (om *OrderbookMediator) SetIterBid(iterBid func(func(float64, Value) bool)) {
	om.iterBid = iterBid
}

// return false to stop iteration
func (om *OrderbookMediator) IterateAsk(cb func(float64, Value) bool) {
	om.iterAsk(cb)
}

// return false to stop iteration
func (om *OrderbookMediator) IterateBid(cb func(float64, Value) bool) {
	om.iterBid(cb)
}

func (om *OrderbookMediator) UpdateDepth(tsMsNow int64) error {
	return om.updateDepth(tsMsNow)
}
