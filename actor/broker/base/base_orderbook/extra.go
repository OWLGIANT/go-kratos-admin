package base_orderbook

import (
	"actor/broker/base/orderbook_mediator"
)

func extraInfoUpdate(extra *orderbook_mediator.Extra, delta float64, tsMs int64) {
	if delta > 0 {
		extra.ChangedAddCnt++
	} else if delta < 0 {
		extra.ChangedReduceCnt++
	} else {
		// 没有变化  一般不会发生  没变化不会有推送
		//panic("extraInfoUpdate called with zero delta")
	}
	extra.RecentChanges.PushKick(orderbook_mediator.Change{
		Tsms:  tsMs,
		Delta: delta,
	})
}
