package base_orderbook

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"actor/helper"
	"actor/third/log"
	"github.com/stretchr/testify/require"
)

func TestOrderbookLevel3Extra(t *testing.T) {

	log.Init("/tmp/log.log", "debug",
		log.SetStdout(true),
		log.SetCaller(true),
		log.SetLogger(log.LoggerZaplog),
	)

	tradeMsg := helper.TradeMsg{}

	connectionJudger := func(firstMatched bool, updateId int64, s *Slot) bool {
		return updateId+1 == s.ExLastId
	}
	firstMatchJudger := func(snapSlot *Slot, slot *Slot) bool {
		return true
	}
	orderbook := NewOrderbook("test", "test", firstMatchJudger, connectionJudger, getOrderbookSnap, SnapFetchType_WsConnectWithSeq, onExit, &tradeMsg.Depth)
	orderbook.SetDepthSubLevel(50)

	replayBybit(orderbook)

	builder := strings.Builder{}
	orderbook.Output(&builder)
	fmt.Println(builder.String())
	{
		// extra更加合理
		orderbookTmp := *orderbook
		bid0 := orderbookTmp.bids.header.next()
		// ap0 := orderbookTmp.asks.header.next().key
		// ap1 := orderbookTmp.asks.header.next().next().key
		s := &Slot{}
		s.ExLastId = orderbookTmp.updateId + 1
		origAmount := bid0.value.Amount
		s.PriceItems = [][2]float64{
			{bid0.key, origAmount + 0.045},
		}
		s.SortIdx = time.Now().UnixNano()
		s.AskStartIdx = len(s.PriceItems) // 没有asks
		log.Debugf("gonna insert slot. ExLastId: %v, updateId: %v", s.ExLastId, orderbookTmp.updateId)
		orderbookTmp.InsertSlot(s)
		builder := strings.Builder{}
		orderbookTmp.Output(&builder)
		fmt.Println(builder.String())

		bidNew := orderbookTmp.bids.header.next()
		// apNew := orderbookTmp.asks.header.next().key
		log.Debugf("bp: %v, bpNew: %v", bid0, bidNew)
		require.Equal(t, bid0.key, bidNew.key)
		require.Equal(t, origAmount+0.045, bidNew.value.Amount)
		require.Equal(t, 1, bidNew.value.Extra.ChangedAddCnt)
		// require.Equal(t, 1, len(bidNew.value.extra.recentChanges.GetAllElements()))
		ele := bidNew.value.Extra.RecentChanges.GetElement(0)
		require.InDelta(t, 0.045, ele.Delta, 1e-9)
		require.LessOrEqual(t, math.Abs(float64(time.Now().UnixMilli()-ele.Tsms)), 1000.0)
		{
			// 第二次变更
			s := &Slot{}
			s.ExLastId = orderbookTmp.updateId + 1
			origAmount := bid0.value.Amount
			s.PriceItems = [][2]float64{
				{bid0.key, origAmount - 0.01},
			}
			s.SortIdx = time.Now().UnixNano()
			s.AskStartIdx = len(s.PriceItems) // 没有asks
			log.Debugf("gonna insert slot. ExLastId: %v, updateId: %v", s.ExLastId, orderbookTmp.updateId)
			orderbookTmp.InsertSlot(s)
			builder := strings.Builder{}
			orderbookTmp.Output(&builder)
			fmt.Println(builder.String())

			bidNew := orderbookTmp.bids.header.next()
			// apNew := orderbookTmp.asks.header.next().key
			log.Debugf("bp: %v, bpNew: %v", bid0, bidNew)
			require.Equal(t, bid0.key, bidNew.key)
			require.Equal(t, origAmount-0.01, bidNew.value.Amount)
			require.Equal(t, 1, bidNew.value.Extra.ChangedReduceCnt)
			require.Equal(t, 1, bidNew.value.Extra.ChangedAddCnt)
			ele := bidNew.value.Extra.RecentChanges.GetElement(1)
			require.InDelta(t, -0.01, ele.Delta, 1e-9)
			require.LessOrEqual(t, math.Abs(float64(time.Now().UnixMilli()-ele.Tsms)), 1000.0)
		}
		{
			// rebuildWithSlot 后会保留之前的变更记录
			bid0 := orderbookTmp.bids.header.next()
			ask0 := orderbookTmp.asks.header.next()
			// ap1 := orderbookTmp.asks.header.next().next().key
			s := &Slot{}
			s.ExLastId = orderbook.updateId + 1
			origAmount := bid0.value.Amount
			origAskAmount := ask0.value.Amount
			s.PriceItems = [][2]float64{
				{bid0.key, origAmount + 0.045},
				{ask0.key, origAskAmount + 0.045},
			}
			s.SortIdx = time.Now().UnixNano()
			s.AskStartIdx = 1
			log.Debugf("gonna insert slot. ExLastId: %v, updateId: %v", s.ExLastId, orderbookTmp.updateId)
			// orderbookTmp.InsertSlot(s)
			orderbookTmp.rebuildWithSlot(s, time.Now().UnixMilli())
			builder := strings.Builder{}
			orderbookTmp.Output(&builder)
			fmt.Println(builder.String())

			bidNew := orderbookTmp.bids.header.next()
			askNew := orderbookTmp.asks.header.next()
			log.Debugf("bp: %v, bpNew: %v", bid0, bidNew)
			require.Equal(t, bid0.key, bidNew.key)
			require.Equal(t, ask0.key, askNew.key)
			require.Equal(t, origAmount+0.045, bidNew.value.Amount)
			require.Equal(t, origAskAmount+0.045, askNew.value.Amount)
			require.Equal(t, 2, bidNew.value.Extra.ChangedAddCnt)
			require.Equal(t, 1, askNew.value.Extra.ChangedAddCnt)
			{
				ele := bidNew.value.Extra.RecentChanges.GetElement(2)
				require.InDelta(t, 0.045, ele.Delta, 1e-9)
				require.LessOrEqual(t, math.Abs(float64(time.Now().UnixMilli()-ele.Tsms)), 1000.0)
			}
			{
				ele := askNew.value.Extra.RecentChanges.GetElement(0)
				require.InDelta(t, 0.045, ele.Delta, 1e-9)
				require.LessOrEqual(t, math.Abs(float64(time.Now().UnixMilli()-ele.Tsms)), 1000.0)
			}
		}
	}
}
