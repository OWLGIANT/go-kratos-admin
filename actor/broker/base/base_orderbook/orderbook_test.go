package base_orderbook

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"actor/helper"
	"actor/third/log"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"
)

func TestOrderbook(t *testing.T) {

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
	{ // 上穿cut cross
		orderbookTmp := *orderbook
		bp0 := orderbookTmp.bids.header.next().key
		ap0 := orderbookTmp.asks.header.next().key
		ap1 := orderbookTmp.asks.header.next().next().key
		s := &Slot{}
		s.ExLastId = orderbookTmp.updateId + 1
		s.PriceItems = [][2]float64{
			{(ap0 + ap1) / 2, 0.045000}, // ap
		}
		s.SortIdx = time.Now().UnixNano()
		s.AskStartIdx = len(s.PriceItems)
		log.Debugf("gonna insert slot. ExLastId: %v, updateId: %v", s.ExLastId, orderbookTmp.updateId)
		orderbookTmp.InsertSlot(s)
		builder := strings.Builder{}
		orderbookTmp.Output(&builder)
		fmt.Println(builder.String())

		bpNew := orderbookTmp.bids.header.next().key
		apNew := orderbookTmp.asks.header.next().key
		log.Debugf("bp: %v, ap: %v, bpNew: %v, apNew: %v", bp0, ap0, bpNew, apNew)
		require.Equal(t, (ap0+ap1)/2, bpNew)
		require.Equal(t, ap1, apNew)
	}
	{ // 下穿cut cross
		orderbook := NewOrderbook("test", "test", firstMatchJudger, connectionJudger, getOrderbookSnap, SnapFetchType_WsConnectWithSeq, onExit, &tradeMsg.Depth)
		orderbook.SetDepthSubLevel(50)

		replayBybit(orderbook)
		orderbookTmp := *orderbook
		bp0 := orderbookTmp.bids.header.next().key
		bp1 := orderbookTmp.bids.header.next().next().key
		ap0 := orderbookTmp.asks.header.next().key
		// ap1 := orderbookTmp.asks.header.next().next().key
		s := &Slot{}
		s.ExLastId = orderbookTmp.updateId + 1
		s.PriceItems = [][2]float64{
			{(bp0 + bp1) / 2, 0.045000}, // ap
		}
		s.SortIdx = time.Now().UnixNano()
		s.AskStartIdx = 0
		log.Debugf("gonna insert slot. ExLastId: %v, updateId: %v", s.ExLastId, orderbookTmp.updateId)
		orderbookTmp.InsertSlot(s)
		builder := strings.Builder{}
		orderbookTmp.Output(&builder)
		fmt.Println(builder.String())

		bpNew := orderbookTmp.bids.header.next().key
		apNew := orderbookTmp.asks.header.next().key
		log.Debugf("bp: %v, ap: %v, bpNew: %v, apNew: %v", bp0, ap0, bpNew, apNew)
		require.Equal(t, (bp0+bp1)/2, apNew)
		require.Equal(t, bp1, bpNew)
	}
}

// bybit乱序接收
func replayBybit(orderbook *Orderbook) {
	// 打开文件
	// file, err := os.Open("bybit_ob_record.txt")
	file, err := os.Open("bybit_ob_record_lite.txt")
	if err != nil {
		fmt.Println("无法打开文件:", err)
		return
	}
	defer file.Close()

	// 创建一个 Scanner 来读取文件内容
	scanner := bufio.NewScanner(file)

	// 逐行读取文件内容
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		var p fastjson.Parser
		value, err := p.Parse(line)
		if err != nil {
			panic(err)
		}
		data := value.Get("data")
		lastId := helper.MustGetInt64(data, "u")
		type_ := helper.MustGetShadowStringFromBytes(value, "type")
		asks := data.GetArray("a")
		bids := data.GetArray("b")
		bidLen := len(bids)
		slot := orderbook.GetFreeSlot(len(asks)+bidLen, lastId) // bybit 乱序发送，但u有序
		if strings.HasPrefix(type_, "s") {                      // snapshot delta
			slot.IsSnap = true
		}
		// You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier.
		slot.ExSeq = helper.MustGetInt64(data, "seq")
		slot.ExLastId = lastId
		for _, v := range bids {
			s := v.GetArray()
			p := helper.MustGetFloat64FromBytes(s[0])
			a := helper.MustGetFloat64FromBytes(s[1])
			slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
		}
		slot.AskStartIdx = bidLen
		for _, v := range asks {
			s := v.GetArray()
			p := helper.MustGetFloat64FromBytes(s[0])
			a := helper.MustGetFloat64FromBytes(s[1])
			slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
		}

		if orderbook.InsertSlot(slot) {
		}
	}

}

func getOrderbookSnap() (*Slot, error) {
	return nil, errors.New("ws not connected")
}

func onExit(msg string) {
	fmt.Println("onexit " + msg)
}
