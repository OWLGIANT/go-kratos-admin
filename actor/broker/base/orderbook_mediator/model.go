package orderbook_mediator

import "actor/third/gcircularqueue_generic"

type Change struct {
	Tsms  int64
	Delta float64 // 变化量, 正数表示增加，负数表示减少
}

type Extra struct {
	ChangedAddCnt    int64 // 本档位 量增加次数
	ChangedReduceCnt int64 // 本档位 量减少次数
	RecentChanges    *gcircularqueue_generic.CircularQueue[Change]
}

type Value struct {
	Amount float64 // 单量
	Extra  Extra
}
