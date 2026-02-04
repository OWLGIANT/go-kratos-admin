// GetMergeEquity 从  ws rs 中获取资产
package base

import (
	"actor/helper"
	"actor/third/cmap"
	"actor/third/log"
)

func GetMergeEquity(TradeAssetMapWs, TradeAssetMapRest *cmap.ConcurrentMap[string, *helper.EquityEvent], Pair *helper.Pair) (float64, float64) {
	var cashWithPnl, coinWithPnl float64
	var cashWithoutPnl, coinWithoutPnl float64
	var ok bool
	var cashStruct, coinStruct *helper.EquityEvent
	//
	if TradeAssetMapWs != nil {
		cashStruct, ok = TradeAssetMapWs.Get(Pair.Quote)
		if ok {
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				cashWithPnl = cashStruct.TotalWithUpl
			}
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		cashStruct2, ok2 := TradeAssetMapRest.Get(Pair.Quote)
		if ok && ok2 { // 当同时存在 rs ws 数据时候  取最新的数据
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) && cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				if cashStruct2.Seq.Inner.Load() > cashStruct.Seq.Inner.Load() {
					cashWithPnl = cashStruct2.TotalWithUpl
				}
			} else if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) && !cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				cashWithPnl = cashStruct2.TotalWithUpl
			}

			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				if cashStruct2.Seq.Inner.Load() > cashStruct.Seq.Inner.Load() {
					cashWithoutPnl = cashStruct2.TotalWithoutUpl
				}
			} else if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && !cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct2.TotalWithoutUpl
			}
		} else if !ok && ok2 { // 当只存在 rs 数据时候 取 rs 数据
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				cashWithPnl = cashStruct2.TotalWithUpl
			}
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct2.TotalWithoutUpl
			}
		}
	}
	//
	if TradeAssetMapWs != nil {
		coinStruct, ok = TradeAssetMapWs.Get(Pair.Base)
		if ok {
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				coinWithPnl = coinStruct.TotalWithUpl
			}
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		coinStruct2, ok2 := TradeAssetMapRest.Get(Pair.Base)
		if ok && ok2 {
			if coinStruct2.Seq.Inner.Load() > coinStruct.Seq.Inner.Load() {
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
					coinWithPnl = coinStruct2.TotalWithUpl
				}
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					coinWithoutPnl = coinStruct2.TotalWithoutUpl
				}
			}
		} else if !ok && ok2 {
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				coinWithPnl = coinStruct2.TotalWithUpl
			}
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct2.TotalWithoutUpl
			}
		}
	}
	// 优先使用 with pnl 的数据  实在不行再 依赖 without pnl 的数据
	var cash, coin float64
	if cashWithPnl > 0 {
		cash = cashWithPnl
	} else {
		cash = cashWithoutPnl
	}
	if coinWithPnl > 0 {
		coin = coinWithPnl
	} else {
		coin = coinWithoutPnl
	}
	if helper.DEBUGMODE {
		log.Debugf("[GetMergeEquity] cash:%v %v coin:%v %v cash:%v coin:%v", cashWithPnl, cashWithoutPnl, coinWithPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapRest is nil")
		}
	}
	// 检查异常数值
	if cash == 0 && coin == 0 {
		log.Warnf("[GetMergeEquity] cash coin 都为0 异常")
		log.Warnf("[GetMergeEquity] cash:%v %v coin:%v %v cash:%v coin:%v", cashWithPnl, cashWithoutPnl, coinWithPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapRest is nil")
		}

	}
	return cash, coin
}

func GetMergeEquityAvail(TradeAssetMapWs, TradeAssetMapRest *cmap.ConcurrentMap[string, *helper.EquityEvent], Pair *helper.Pair) (float64, float64) {
	var cashFree, coinFree float64
	var ok bool
	var cashStruct, coinStruct *helper.EquityEvent
	//
	if TradeAssetMapWs != nil {
		cashStruct, ok = TradeAssetMapWs.Get(Pair.Quote)
		if ok {
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
				cashFree = cashStruct.Avail
			}
		}
	}
	if TradeAssetMapRest != nil {
		cashStruct2, ok2 := TradeAssetMapRest.Get(Pair.Quote)
		if ok && ok2 { // 当同时存在 rs ws 数据时候  取最新的数据
			if cashStruct2.Seq.Inner.Load() > cashStruct.Seq.Inner.Load() {
				if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
					cashFree = cashStruct2.Avail
				}
			}
		} else if !ok && ok2 { // 当只存在 rs 数据时候 取 rs 数据
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
				cashFree = cashStruct2.Avail
			}
		}
	}
	//
	if TradeAssetMapWs != nil {
		coinStruct, ok = TradeAssetMapWs.Get(Pair.Base)
		if ok {
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
				coinFree = coinStruct.Avail
			}
		}
	}
	if TradeAssetMapRest != nil {
		coinStruct2, ok2 := TradeAssetMapRest.Get(Pair.Base)
		if ok && ok2 {
			if coinStruct2.Seq.Inner.Load() > coinStruct.Seq.Inner.Load() {
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
					coinFree = coinStruct2.Avail
				}
			}
		} else if !ok && ok2 {
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_Avail) {
				coinFree = coinStruct2.Avail
			}
		}
	}
	// 优先使用 with pnl 的数据  实在不行再 依赖 without pnl 的数据
	var cash, coin float64
	cash = cashFree
	coin = coinFree
	if helper.DEBUGMODE {
		log.Debugf("[GetMergeEquityAvail] cashFree:%v coinFree:%v cash:%v coin:%v", cashFree, coinFree, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Debugf("[GetMergeEquityAvail]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Debugf("[GetMergeEquityAvail]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Debugf("[GetMergeEquityAvail]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Debugf("[GetMergeEquityAvail]TradeAssetMapRest is nil")
		}
	}
	// 检查异常数值
	if cash == 0 && coin == 0 {
		log.Warnf("[GetMergeEquityAvail] cash coin 都为0 异常")
		log.Debugf("[GetMergeEquityAvail] cashFree:%v coinFree:%v cash:%v coin:%v", cashFree, coinFree, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Warnf("[GetMergeEquityAvail]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Warnf("[GetMergeEquityAvail]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Warnf("[GetMergeEquityAvail]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Warnf("[GetMergeEquityAvail]TradeAssetMapRest is nil")
		}

	}
	return cash, coin
}

func GetMergeEquityWithPnlEvenOld(TradeAssetMapWs, TradeAssetMapRest *cmap.ConcurrentMap[string, *helper.EquityEvent], Pair *helper.Pair) (float64, float64) {
	var cashWithPnl, coinWithPnl float64
	var cashWithoutPnl, coinWithoutPnl float64
	var ok bool
	var cashStruct, coinStruct *helper.EquityEvent
	//
	if TradeAssetMapWs != nil {
		cashStruct, ok = TradeAssetMapWs.Get(Pair.Quote)
		if ok {
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				cashWithPnl = cashStruct.TotalWithUpl
			}
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		cashStruct2, ok2 := TradeAssetMapRest.Get(Pair.Quote)
		if ok && ok2 { // 当同时存在 rs ws 数据时候  取最新的数据
			if cashStruct2.Seq.Inner.Load() > cashStruct.Seq.Inner.Load() {
				if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
					cashWithPnl = cashStruct2.TotalWithUpl
				}
				if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					cashWithoutPnl = cashStruct2.TotalWithoutUpl
				}
			} else {
				if !cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) && cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
					cashWithPnl = cashStruct2.TotalWithUpl
				}
				if !cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					cashWithoutPnl = cashStruct2.TotalWithoutUpl
				}
			}
		} else if !ok && ok2 { // 当只存在 rs 数据时候 取 rs 数据
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				cashWithPnl = cashStruct2.TotalWithUpl
			}
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct2.TotalWithoutUpl
			}
		}
	}
	//
	if TradeAssetMapWs != nil {
		coinStruct, ok = TradeAssetMapWs.Get(Pair.Base)
		if ok {
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				coinWithPnl = coinStruct.TotalWithUpl
			}
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		coinStruct2, ok2 := TradeAssetMapRest.Get(Pair.Base)
		if ok && ok2 {
			if coinStruct2.Seq.Inner.Load() > coinStruct.Seq.Inner.Load() {
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
					coinWithPnl = coinStruct2.TotalWithUpl
				}
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					coinWithoutPnl = coinStruct2.TotalWithoutUpl
				}
			} else {
				if !coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) && coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
					coinWithPnl = coinStruct2.TotalWithUpl
				}
				if !coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					coinWithoutPnl = coinStruct2.TotalWithoutUpl
				}
			}
		} else if !ok && ok2 {
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithUpl) {
				coinWithPnl = coinStruct2.TotalWithUpl
			}
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct2.TotalWithoutUpl
			}
		}
	}
	// 优先使用 with pnl 的数据  实在不行再 依赖 without pnl 的数据
	var cash, coin float64
	if cashWithPnl > 0 {
		cash = cashWithPnl
	} else {
		cash = cashWithoutPnl
	}
	if coinWithPnl > 0 {
		coin = coinWithPnl
	} else {
		coin = coinWithoutPnl
	}
	if helper.DEBUGMODE {
		log.Debugf("[GetMergeEquity] cash:%v %v coin:%v %v cash:%v coin:%v", cashWithPnl, cashWithoutPnl, coinWithPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapRest is nil")
		}
	}
	// 检查异常数值
	if cash == 0 && coin == 0 {
		log.Warnf("[GetMergeEquity] cash coin 都为0 异常")
		log.Warnf("[GetMergeEquity] cash:%v %v coin:%v %v cash:%v coin:%v", cashWithPnl, cashWithoutPnl, coinWithPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapRest is nil")
		}

	}
	return cash, coin
}

func GetMergeEquityAvailEvenOld(TradeAssetMapWs, TradeAssetMapRest *cmap.ConcurrentMap[string, *helper.EquityEvent], Pair *helper.Pair) (float64, float64) {
	var cashWithoutPnl, coinWithoutPnl float64
	var ok bool
	var cashStruct, coinStruct *helper.EquityEvent
	//
	if TradeAssetMapWs != nil {
		cashStruct, ok = TradeAssetMapWs.Get(Pair.Quote)
		if ok {
			if cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		cashStruct2, ok2 := TradeAssetMapRest.Get(Pair.Quote)
		if ok && ok2 { // 当同时存在 rs ws 数据时候  取最新的数据
			if cashStruct2.Seq.Inner.Load() > cashStruct.Seq.Inner.Load() {
				if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					cashWithoutPnl = cashStruct2.TotalWithoutUpl
				}
			} else {
				if !cashStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					cashWithoutPnl = cashStruct2.TotalWithoutUpl
				}
			}
		} else if !ok && ok2 { // 当只存在 rs 数据时候 取 rs 数据
			if cashStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				cashWithoutPnl = cashStruct2.TotalWithoutUpl
			}
		}
	}
	//
	if TradeAssetMapWs != nil {
		coinStruct, ok = TradeAssetMapWs.Get(Pair.Base)
		if ok {
			if coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct.TotalWithoutUpl
			}
		}
	}
	if TradeAssetMapRest != nil {
		coinStruct2, ok2 := TradeAssetMapRest.Get(Pair.Base)
		if ok && ok2 {
			if coinStruct2.Seq.Inner.Load() > coinStruct.Seq.Inner.Load() {
				if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					coinWithoutPnl = coinStruct2.TotalWithoutUpl
				}
			} else {
				if !coinStruct.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) && coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
					coinWithoutPnl = coinStruct2.TotalWithoutUpl
				}
			}
		} else if !ok && ok2 {
			if coinStruct2.FieldsSet.ContainsOne(helper.EquityEventField_TotalWithoutUpl) {
				coinWithoutPnl = coinStruct2.TotalWithoutUpl
			}
		}
	}
	// 优先使用 with pnl 的数据  实在不行再 依赖 without pnl 的数据
	var cash, coin float64

	cash = cashWithoutPnl
	coin = coinWithoutPnl

	if helper.DEBUGMODE {
		log.Warnf("[GetMergeEquity] cashWithoutPnl:%v coinWithoutPnl:%v cash:%v coin:%v", cashWithoutPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Debugf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Debugf("[GetMergeEquity]TradeAssetMapRest is nil")
		}
	}
	// 检查异常数值
	if cash == 0 && coin == 0 {
		log.Warnf("[GetMergeEquity] cash coin 都为0 异常")
		log.Warnf("[GetMergeEquity] cashWithoutPnl:%v coinWithoutPnl:%v cash:%v coin:%v", cashWithoutPnl, coinWithoutPnl, cash, coin)
		if TradeAssetMapWs != nil {
			data1, _ := TradeAssetMapWs.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapWs %v ", string(data1))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapWs is nil")
		}
		if TradeAssetMapRest != nil {
			data2, _ := TradeAssetMapRest.MarshalJSON()
			log.Warnf("[GetMergeEquity]TradeAssetMapRest %v ", string(data2))
		} else {
			log.Warnf("[GetMergeEquity]TradeAssetMapRest is nil")
		}

	}
	return cash, coin
}
