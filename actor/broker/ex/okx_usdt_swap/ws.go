// 基于okx的v5版本开发

package okx_usdt_swap

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	base_orderbook "actor/broker/base/base_orderbook"
	"actor/broker/base/orderbook_mediator"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

type OkxUsdtSwapWs struct {
	base.FatherWs
	pubWs *ws.WS // 继承ws
	priWs *ws.WS // 继承ws
	busWs *ws.WS

	connectOnce sync.Once     // 连接一次
	id          int64         // 所有发送的消息都要唯一id
	stopCPri    chan struct{} // 接受停机信号
	stopCBus    chan struct{} // 接受停机信号
	// needAuth     bool                 // 是否需要私有连接
	// needTicker   bool                 // 是否需要bbo
	// needDepth    bool                 // 是否需要深度
	// needTrade    bool                 //是否需要公开成交
	// needPartial  bool                 // 是否需要增量深度
	colo     bool   // 是否colo
	pubWsUrl string // 公共ws地址
	priWsUrl string // 私有ws地址
	busWsUrl string // 业务ws地址
	restUrl  string // rest地址
	// client    *OkxUsdtSwapClient   // 用来查询各种信息
	logged     bool           // 是否登录
	takerFee   atomic.Float64 // taker费率
	makerFee   atomic.Float64 // maker费率
	indexPrice float64
	orderbook  *base_orderbook.Orderbook
	rs         *OkxUsdtSwapRs
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	coloFlag := checkColo(params)
	w := &OkxUsdtSwapWs{
		colo:     coloFlag,
		pubWsUrl: okxWsPubUrl,
		priWsUrl: okxWsPriUrl,
		busWsUrl: okxWsBusUrl,
		restUrl:  okxRestUrl,
	}

	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)
	rs0 := NewRs(params, msg, info, cb)
	var ok bool
	w.rs, ok = rs0.(*OkxUsdtSwapRs)
	if !ok || w.rs == nil {
		helper.LogErrorThenCall(fmt.Sprintf("failed to create rs."), cb.OnExit)
		return nil
	}

	w.rs.GetExchangeInfos()
	w.UpdateExchangeInfo(w.rs.ExchangeInfoPtrP2S, w.rs.ExchangeInfoPtrS2P, w.Cb.OnExit)

	w.pubWs = ws.NewWS(w.pubWsUrl, params.LocalAddr, params.ProxyURL, w.pubHandler, cb.OnExit, params.BrokerConfig)
	w.pubWs.SetPingFunc(w.ping)
	w.pubWs.SetPingInterval(10)
	w.pubWs.SetSubscribe(w.subPub)
	if w.BrokerConfig.NeedAuth {
		w.priWs = ws.NewWS(w.priWsUrl, params.LocalAddr, params.ProxyURL, w.priHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetPingFunc(w.ping)
		w.priWs.SetPingInterval(10)
		w.priWs.SetSubscribe(w.subPri)
	}
	if w.BrokerConfig.Need1MinKline {
		w.busWs = ws.NewWS(w.busWsUrl, params.LocalAddr, params.ProxyURL, w.busHandler, cb.OnExit, params.BrokerConfig)
		w.busWs.SetPingFunc(w.ping)
		w.busWs.SetPingInterval(10)
		w.busWs.SetSubscribe(w.subBus)
	}
	// w.client = NewClient(params, cb)

	connectionJudger := func(firstMatched bool, seq int64, s *base_orderbook.Slot) bool {
		return seq == s.ExPrevLastId
	}
	firstMatchJudger := func(snapSlot *base_orderbook.Slot, slot *base_orderbook.Slot) bool {
		return true
	}
	w.orderbook = base_orderbook.NewOrderbook(w.ExchangeName.String(), params.Pairs[0].String(), firstMatchJudger, connectionJudger, w.getOrderbookSnap, base_orderbook.SnapFetchType_WsConnectWithSeq, cb.OnExit, &w.TradeMsg.Depth)
	w.orderbook.SetDisableAutoUpdateDepth(params.DisableAutoUpdateDepth)
	mediator := orderbook_mediator.NewOrderbookMediator()
	w.orderbook.SetMediator(mediator)
	w.TradeMsg.Orderbook = mediator
	w.orderbook.SetDepthSubLevel(w.BrokerConfig.WsDepthLevel)
	w.AddWsConnections(w.pubWs, w.priWs, w.busWs)
	return w
}

func (w *OkxUsdtSwapWs) getOrderbookSnap() (*base_orderbook.Slot, error) {
	if !w.pubWs.IsConnected() {
		return nil, errors.New("ws not connected")
	}
	p := map[string]interface{}{
		"op": "subscribe",
		"args": []interface{}{
			map[string]interface{}{
				"instId": w.Symbol,
				// books-l2-tbt400档深度频道，只允许交易手续费等级VIP5及以上的API用户订阅。
				// books50-l2-tbt50档深度频道，只允许交易手续费等级VIP4及以上的API用户订阅.
				// books 首次推400档快照数据，以后增量推送，每100毫秒推送一次变化的数据
				"channel": "books",
			},
		},
	}
	msg, err := json.Marshal(p)
	if err != nil {
		w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
		return nil, err
	}
	w.pubWs.SendMessage(msg)
	return nil, nil
}

func (w *OkxUsdtSwapWs) ping() []byte {
	return []byte("ping")
}

func (w *OkxUsdtSwapWs) subPub() error {
	// bbo
	if w.BrokerConfig.NeedTicker {
		p := map[string]interface{}{
			"op": "subscribe",
			"args": []interface{}{
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "bbo-tbt",
				},
				// map[string]interface{}{
				// "instId":  w.symbol,
				// "channel": "tickers",
				// },
				map[string]interface{}{
					"instId":  strings.Replace(w.Symbol, "-SWAP", "", -1),
					"channel": "index-tickers",
				},
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "price-limit",
				},
			},
		}
		w.pubWs.SubWithRetry("bbo-tbt", w.Cb.OnExit, func() []byte {
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			return msg
		})
	}
	if w.BrokerConfig.WsDepthLevel > 0 {
		finalLevel, ok := helper.SuitableLevel([]int{5}, w.BrokerConfig.WsDepthLevel)
		if ok {
			p := map[string]interface{}{
				"op": "subscribe",
				"args": []interface{}{
					map[string]interface{}{
						"instId":  w.Symbol,
						"channel": fmt.Sprintf("books%d", finalLevel),
					},
				},
			}
			w.pubWs.SubWithRetry(fmt.Sprintf("books%d", finalLevel), w.Cb.OnExit, func() []byte {
				msg, err := json.Marshal(p)
				if err != nil {
					w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
				}
				return msg
			})
		} else {
			p := map[string]interface{}{
				"op": "subscribe",
				"args": []interface{}{
					map[string]interface{}{
						"instId": w.Symbol,
						// books-l2-tbt 400档深度频道，只允许交易手续费等级VIP5及以上的API用户订阅。
						// books50-l2-tbt 50档深度频道，只允许交易手续费等级VIP4及以上的API用户订阅.
						// books 首次推400档快照数据，以后增量推送，每100毫秒推送一次变化的数据
						"channel": "books",
					},
				},
			}
			w.pubWs.SubWithRetry("books", w.Cb.OnExit, func() []byte {
				msg, err := json.Marshal(p)
				if err != nil {
					w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
				}
				return msg
			})
		}
	}

	if w.BrokerConfig.NeedTrade {
		p := map[string]interface{}{
			"op": "subscribe",
			"args": []interface{}{
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "trades",
				},
			},
		}
		w.pubWs.SubWithRetry("trades", w.Cb.OnExit, func() []byte {
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			return msg
		})
	}
	if w.BrokerConfig.NeedIndex {
		p := map[string]interface{}{
			"op": "subscribe",
			"args": []interface{}{
				map[string]interface{}{
					"instId":  strings.ToUpper(w.Pair.Base) + "-" + strings.ToUpper(w.Pair.Quote),
					"channel": "index-tickers",
				},
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "mark-price",
				},
			},
		}
		w.pubWs.SubWithRetry("index-tickers", w.Cb.OnExit, func() []byte {
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			return msg
		})
	}
	return nil
}

func (w *OkxUsdtSwapWs) subBus() error {
	if w.BrokerConfig.Need1MinKline {
		p := map[string]interface{}{
			"op": "subscribe",
			"args": []interface{}{
				map[string]interface{}{
					"instId":  strings.ToUpper(w.Pair.Base) + "-" + strings.ToUpper(w.Pair.Quote) + "-SWAP",
					"channel": "candle1m",
				},
			},
		}
		w.busWs.SubWithRetry("candle1m", w.Cb.OnExit, func() []byte {
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			return msg
		})
	}
	return nil
}

func (w *OkxUsdtSwapWs) subPri() error {
	// login first
	w.logged = false
	msg := getWsLogin(&w.BrokerConfig)
	w.priWs.SendMessage(msg)

	time.Sleep(time.Second * 3)
	w.logged = true
	// go func() {
	// for i := 0; i < 15; i++ {
	// if ws.logged {
	// ws.realSubPri()
	// break
	// }
	// okx不允许重复发送登录
	// if i%2 == 0 {
	// msg := getWsLogin(ws.params)
	// ws.priWs.SendMessage(msg)
	// }
	// time.Sleep(time.Second)
	// }
	// if !ws.logged {
	// ws.Cb.OnExit("okx usdt swap 登录失败")
	// return
	// }
	// }()
	return nil
}

func (w *OkxUsdtSwapWs) realSubPri() {
	p := map[string]interface{}{
		"op": "subscribe",
		"args": []interface{}{
			map[string]interface{}{
				"channel": "account",
				"ccy":     strings.ToUpper(w.Pair.Quote),
			},
			map[string]interface{}{
				"channel":  "positions",
				"instType": "SWAP",
				//"instId":   w.symbol,
			},
			map[string]interface{}{
				"channel":  "orders",
				"instType": "SWAP",
				//"instId":   w.symbol,
			},
		},
	}
	w.priWs.SubWithRetry("priWs", w.Cb.OnExit, func() []byte {
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
		}
		return msg
	})
}

func (w *OkxUsdtSwapWs) Run() {
	// w.client.updateExchangeInfo(w.PairInfo)
	w.connectOnce.Do(func() {
		var err error
		if w.BrokerConfig.NeedPubWs() {
			w.StopCPub, err = w.pubWs.Serve()
			if err != nil {
				w.Cb.OnExit("okx usdt swap public ws 连接失败")
			}
		}

		if w.BrokerConfig.Need1MinKline {
			w.stopCBus, err = w.busWs.Serve()
			if err != nil {
				w.Cb.OnExit("okx usdt swap Bus ws 连接失败")
			}
		}

		w.logged = false
		if w.BrokerConfig.NeedAuth {
			w.stopCPri, err = w.priWs.Serve()
			if err != nil {
				w.Cb.OnExit("okx usdt swap private ws 连接失败")
			}
		}
	})
}

func (w *OkxUsdtSwapWs) DoStop() {
	if w.pubWs != nil {
		if w.StopCPub != nil {
			helper.CloseSafe(w.StopCPub)
		}
	}
	if w.priWs != nil {
		if w.stopCPri != nil {
			helper.CloseSafe(w.stopCPri)
		}
	}
	if w.busWs != nil {
		if w.stopCBus != nil {
			helper.CloseSafe(w.stopCBus)
		}
	}

	w.connectOnce = sync.Once{}
}

// 处理公有ws的处理器
func (w *OkxUsdtSwapWs) pubHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debugf("收到 pub ws 推送 %s", helper.BytesToString(msg))
	}
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) != "pong" {
			w.Logger.Errorf("okx usdt swap ws解析msg出错 msg: %v err: %v", value, err)
		}
		return
	}
	datas := value.Get("data")
	if datas == nil || datas.Type() == fastjson.TypeNull {
		w.Logger.Infof("收到 pub ws 推送 %s", helper.BytesToString(msg))
		switch helper.BytesToString(value.GetStringBytes("event")) {
		// event常见于事件汇报，一般都可以忽略，只需要看看是否有error
		case "error":
			w.Logger.Errorf("[pub ws][%s] error , %v", w.ExchangeName.String(), value)
		case "subscribe":
			ch := helper.BytesToString(value.GetStringBytes("arg", "channel"))
			w.pubWs.SetSubSuccess(ch)
			if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
		return
	}
	multi := w.PairInfo.Multi.Float()
	// 存在data的基本都是行情推送，优先级最高，立刻处理
	arg := value.Get("arg")
	// symbol := helper.BytesToString(ch.GetStringBytes("instId"))
	channel := helper.BytesToString(arg.GetStringBytes("channel"))
	data := datas.GetArray()[0]
	switch channel {
	case "bbo-tbt":
		if tsInEx, err := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); err == nil {
			t := &w.TradeMsg.Ticker
			if t.Seq.NewerAndStore(tsInEx, ts) {

				asks := data.GetArray("asks")
				bids := data.GetArray("bids")
				if len(asks) == 0 {
					w.Logger.Errorf("asks 长度为0 msg %v", value)
					return
				}
				if len(bids) == 0 {
					w.Logger.Errorf("bids 长度为0 msg %v", value)
					return
				}
				ask, _ := asks[0].Array()
				ap, _ := ask[0].StringBytes()
				aq, _ := ask[1].StringBytes()
				bid, _ := bids[0].Array()
				bp, _ := bid[0].StringBytes()
				bq, _ := bid[1].StringBytes()
				t.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(ap)))
				t.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(aq)) * multi)
				t.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(bp)))
				t.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(bq)) * multi)
				t.Mp.Store(t.Price())

				if helper.DEBUGMODE {
					helper.MustMillis(tsInEx)
				}
				w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - tsInEx)
				w.Cb.OnTicker(ts)
			}
		}
		// 来源不同，去除
	case "tickers":
		if tsInEx, terr := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); terr == nil {
			t := &w.TradeMsg.Ticker
			if t.Seq.NewerAndStore(tsInEx, ts) {

				t.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("askPx"))))
				t.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("askSz"))) * multi)
				t.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("bidPx"))))
				t.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("bidSz"))) * multi)
				t.Mp.Store(t.Price())
				if helper.DEBUGMODE {
					helper.MustMillis(tsInEx)
				}
				w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - tsInEx)
				w.Cb.OnTicker(ts)
			}
		}
	case "price-limit":
		buyLmt := helper.MustGetFloat64FromBytes(data, "buyLmt")
		sellLmt := helper.MustGetFloat64FromBytes(data, "sellLmt")
		// buyLmt比sellLmt高
		if w.indexPrice != 0 {
			if w.indexPrice*0.99 < sellLmt || w.indexPrice*1.01 > buyLmt {
				w.Logger.Warnf("标记价格异常 buyLmt:%f, sellLmt:%f, indexPrice:%f, mp:%f", buyLmt, sellLmt, w.indexPrice, w.TradeMsg.Ticker.Mp.Load())
				w.Cb.OnExit("okx合约即将触发限价 紧急停机")
			}
		}
	case "mark-price":
		markPrice := helper.MustGetFloat64FromBytes(data, "markPx") //200ms一次，假设即使并发两次数据也差不远
		if w.Cb.OnMark != nil {
			w.Cb.OnMark(ts, helper.MarkEvent{MarkPrice: markPrice})
		}
	case "index-tickers":
		indexPrice := helper.MustGetFloat64FromBytes(data, "idxPx") //200ms一次，假设即使并发两次数据也差不远
		if w.Cb.OnIndex != nil {
			w.Cb.OnIndex(ts, helper.IndexEvent{IndexPrice: indexPrice})
		}
	case "books":
		prevSeqId := helper.MustGetInt64(data, "prevSeqId")
		seqId := helper.MustGetInt64(data, "seqId")
		if prevSeqId == seqId {
			return
		}
		asks := data.GetArray("asks")
		bids := data.GetArray("bids")
		bidLen := len(bids)
		slot := w.orderbook.GetFreeSlot(len(asks)+bidLen, ts)
		if prevSeqId == -1 {
			slot.IsSnap = true
		}
		slot.ExPrevLastId = prevSeqId
		slot.ExLastId = seqId
		for _, v := range bids {
			s := v.GetArray()
			p := helper.MustGetFloat64FromBytes(s[0])
			a := helper.MustGetFloat64FromBytes(s[1]) * multi
			slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
		}
		slot.AskStartIdx = bidLen
		for _, v := range asks {
			s := v.GetArray()
			p := helper.MustGetFloat64FromBytes(s[0])
			a := helper.MustGetFloat64FromBytes(s[1]) * multi
			slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
		}

		if w.orderbook.InsertSlot(slot) {
			if !w.TradeMsg.Depth.Seq.NewerAndStore(seqId, ts) {
				w.Logger.Error("didnt newer and store but insert success")
			}
			w.Cb.OnDepth(ts)
		}
	case "books5":
		seqId := helper.MustGetInt64(data, "seqId")
		if !w.TradeMsg.Depth.Seq.NewerAndStore(seqId, ts) {
			return
		}
		t := &w.TradeMsg.Depth
		t.Lock.Lock()
		asks := data.GetArray("asks")
		bids := data.GetArray("bids")
		if len(asks) == 0 {
			w.Logger.Errorf("asks 长度为0 msg %v", value)
			t.Lock.Unlock()
			return
		}
		if len(bids) == 0 {
			w.Logger.Errorf("bids 长度为0 msg %v", value)
			t.Lock.Unlock()
			return
		}
		t.Asks = t.Asks[:0]
		t.Bids = t.Bids[:0]
		for i, ask := range asks {
			_ask, err := ask.Array()
			if err != nil {
				continue
			}
			price, _ := _ask[0].StringBytes()
			amount, _ := _ask[1].StringBytes()
			t.Asks = append(t.Asks, helper.DepthItem{
				Price:  fastfloat.ParseBestEffort(helper.BytesToString(price)),
				Amount: fastfloat.ParseBestEffort(helper.BytesToString(amount)) * multi,
			})
			if i >= w.BrokerConfig.WsDepthLevel-1 {
				break
			}
		}
		for i, bid := range bids {
			_bid, err := bid.Array()
			if err != nil {
				continue
			}
			price, _ := _bid[0].StringBytes()
			amount, _ := _bid[1].StringBytes()
			t.Bids = append(t.Bids, helper.DepthItem{
				Price:  fastfloat.ParseBestEffort(helper.BytesToString(price)),
				Amount: fastfloat.ParseBestEffort(helper.BytesToString(amount)) * multi,
			})
			if i >= w.BrokerConfig.WsDepthLevel-1 {
				break
			}
		}
		t.Lock.Unlock()
		w.Cb.OnDepth(ts)
	case "trades":
		t := &w.TradeMsg.Trade
		side := helper.BytesToString(data.GetStringBytes("side"))
		price := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("px")))
		amount := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("sz"))) * multi
		switch side {
		case "buy":
			t.Update(helper.TradeSideBuy, amount, price)
		case "sell":
			t.Update(helper.TradeSideSell, amount, price)
		}
		w.Cb.OnTrade(ts)
	}
}

// 处理私有ws的处理器
func (w *OkxUsdtSwapWs) priHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE {
		w.Logger.Debugf("收到 pri ws 消息 %v", helper.BytesToString(msg))
	}
	// 解析
	p := wsPrivateHandyPool.Get()
	defer wsPrivateHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) != "pong" {
			w.Logger.Errorf("okx usdt swap ws解析msg出错 err:%v", err)
		}
		return
	}

	if value.Exists("data") {
		// 存在data的基本都是行情推送，优先级最高，立刻处理
		ch := value.Get("arg")
		op := helper.BytesToString(ch.GetStringBytes("channel"))
		datas := value.GetArray("data")
		switch op {
		case "account":
			for _, data := range datas {
				details := data.GetArray("details")
				for _, v := range details {
					asset := helper.MustGetStringLowerFromBytes(v, "ccy")
					seqEx := helper.MustGetInt64FromBytes(data, "uTime")

					var fieldsSet helper.FieldsSet_T
					fieldsSet = (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)
					if e, ok := w.EquityNewerAndStore(asset, seqEx, ts, fieldsSet); ok {
						upnl := helper.GetFloat64FromBytes(v, "upl")
						// todo 有时包含有时不包含吗？
						if v.Exists("upl") {
							// upnl = helper.MustGetFloat64FromBytes(v, "upl")
						}
						balance := helper.MustGetFloat64FromBytes(v, "cashBal")
						e.TotalWithUpl = balance + upnl
						e.TotalWithoutUpl = balance
						e.Avail = helper.MustGetFloat64FromBytes(v, "availBal")
						e.Upl = upnl

						w.Cb.OnEquityEvent(ts, *e)
					}
				}
			}
		case "positions":
			for _, data := range datas {
				instId := helper.BytesToString(data.GetStringBytes("instId"))
				// info, ok := w.GetPairInfoBySymbol(instId)
				// if !ok {
				// continue
				// }
				timeInEx := helper.MustGetInt64FromBytes(data, "uTime")
				if info, pos, ok := w.PosNewerAndStore(instId, timeInEx, ts); ok {
					ps := helper.BytesToString(data.GetStringBytes("posSide"))
					px := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("avgPx")))
					// sz :=  fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("pos"))) * multi
					sz := fixed.NewS(helper.BytesToString(data.GetStringBytes("pos"))).Mul(info.Multi)
					//openTs := helper.MustGetInt64FromBytes(data, "cTime")
					pos.Lock.Lock()
					switch ps {
					case "long":
						pos.LongAvg = px
						pos.LongPos = sz
					case "short":
						pos.ShortAvg = px
						pos.ShortPos = sz.Abs()
					case "net":
						pos.ResetLocked()
						if sz.GreaterThan(fixed.ZERO) {
							pos.LongAvg = px
							pos.LongPos = sz
						} else {
							pos.ShortAvg = px
							pos.ShortPos = sz.Abs()
						}
					}
					event := pos.ToPositionEvent()
					pos.Lock.Unlock()
					w.Cb.OnPositionEvent(ts, event)
				}
			}
		case "orders":
			for _, data := range datas {
				instId := helper.BytesToString(data.GetStringBytes("instId"))
				info, ok := w.GetPairInfoBySymbol(instId)
				if !ok {
					continue
				}
				order := helper.OrderEvent{Pair: info.Pair}
				order.OrderID = string(data.GetStringBytes("ordId"))
				order.ClientID = string(data.GetStringBytes("clOrdId"))
				state := helper.BytesToString(data.GetStringBytes("state"))
				switch state {
				case "partially_filled":
					order.Type = helper.OrderEventTypePARTIAL
				case "canceled", "filled":
					order.Type = helper.OrderEventTypeREMOVE
				default:
					order.Type = helper.OrderEventTypeNEW
				}
				szStr := helper.BytesToString(data.GetStringBytes("accFillSz"))
				if szStr != "" && szStr != "0" {
					order.Filled = fixed.NewS(szStr).Mul(info.Multi)
					order.FilledPrice = fixed.NewS(helper.BytesToString(data.GetStringBytes("avgPx"))).Float()
					//order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))))
					order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))))
				}
				switch helper.MustGetShadowStringFromBytes(data, "side") {
				case "buy":
					order.OrderSide = helper.OrderSideKD
				case "sell":
					order.OrderSide = helper.OrderSideKK
				default:
					w.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(data, "side"))
					return
				}
				if helper.MustGetShadowStringFromBytes(data, "reduceOnly") == "true" {
					if order.OrderSide == helper.OrderSideKD {
						order.OrderSide = helper.OrderSidePK
					} else if order.OrderSide == helper.OrderSideKK {
						order.OrderSide = helper.OrderSidePD
					}
				}
				switch helper.MustGetShadowStringFromBytes(data, "ordType") {
				case "limit":
					order.OrderType = helper.OrderTypeLimit
				case "market":
					order.OrderType = helper.OrderTypeMarket
				case "post_only":
					order.OrderType = helper.OrderTypePostOnly
				case "ioc":
					order.OrderType = helper.OrderTypeIoc
				}
				order.ReceivedTsNs = ts
				w.Cb.OnOrder(ts, order)
			}

		}
	} else { // event常见于事件汇报，一般都可以忽略，只需要看看是否有error
		w.Logger.Infof("收到 pri ws 推送 %s", helper.BytesToString(msg))
		e := helper.BytesToString(value.GetStringBytes("event"))
		switch e {
		case "error":
			w.Logger.Errorf("[pri ws][%s] error , %v", w.ExchangeName.String(), value)
		case "login":
			// 登录订阅放在这里
			w.logged = true
			// todo 这里是不是做了2次 realSubscribe 的尝试？是为了冗余保护吗
			w.realSubPri()
		case "subscribe":
			w.priWs.SetSubSuccess("priWs")
			if ws.AllWsSubsuccess(w.pubWs, w.priWs, w.busWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
	}
}

// 处理Bus ws的处理器
func (w *OkxUsdtSwapWs) busHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debugf("收到 bus ws 推送 %s", helper.BytesToString(msg))
	}
	// 解析
	p := wsBusinessHandyPool.Get()
	defer wsBusinessHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) != "pong" {
			w.Logger.Errorf("okx usdt swap ws解析msg出错 msg: %v err: %v", value, err)
		}
		return
	}
	datas := value.Get("data")
	if datas == nil || datas.Type() == fastjson.TypeNull {
		w.Logger.Infof("收到 bus ws 推送 %s", helper.BytesToString(msg))
		switch helper.BytesToString(value.GetStringBytes("event")) {
		// event常见于事件汇报，一般都可以忽略，只需要看看是否有error
		case "error":
			w.Logger.Errorf("[pub ws][%s] error , %v", w.ExchangeName.String(), value)
		case "subscribe":
			ch := helper.BytesToString(value.GetStringBytes("arg", "channel"))
			w.busWs.SetSubSuccess(ch)
			if ws.AllWsSubsuccess(w.busWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
		return
	}
	//multi := w.PairInfo.Multi.Float()
	// 存在data的基本都是行情推送，优先级最高，立刻处理
	arg := value.Get("arg")
	// symbol := helper.BytesToString(ch.GetStringBytes("instId"))
	channel := helper.BytesToString(arg.GetStringBytes("channel"))
	data := datas.GetArray()[0]
	switch channel {
	case "candle1m":
		w.Cb.OnKline(ts, helper.Kline{
			Symbol:     helper.BytesToString(arg.GetStringBytes("channel")),
			OpenTimeMs: helper.GetInt64(data.GetArray()[0]),
			//CloseTimeMs:  helper.GetInt64(data.GetArray()[0]),
			//EventTimeMs:  helper.GetInt64(data.GetArray()[0]),
			LocalTimeMs: time.Now().UnixMilli(),
			Open:        helper.GetFloat64(data.GetArray()[1]),
			Close:       helper.GetFloat64(data.GetArray()[4]),
			High:        helper.GetFloat64(data.GetArray()[2]),
			Low:         helper.GetFloat64(data.GetArray()[3]),
			//BuyNotional:  helper.GetInt64(data.GetArray()[0]),
			BuyVolume: helper.GetFloat64(data.GetArray()[7]),
			//SellNotional: helper.GetInt64(data.GetArray()[0]),
			//SellVolume:   helper.GetInt64(data.GetArray()[0]),
			//BuyTradeNum:  helper.GetInt64(data.GetArray()[0]),
			//SellTradeNum: helper.GetInt64(data.GetArray()[0]),
		})
	}
}

// GetFee 获取费率
func (w *OkxUsdtSwapWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.makerFee.Load(), Taker: w.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (w *OkxUsdtSwapWs) WsLogged() bool {
	return w.logged
}

func (w *OkxUsdtSwapWs) GetPriWs() *ws.WS {
	return w.priWs
}
func (w *OkxUsdtSwapWs) GetPubWs() *ws.WS {
	return w.pubWs
}

func (w *OkxUsdtSwapWs) GetCautions() []helper.Caution {
	return []helper.Caution{
		{
			AddedDate: "2023-12-26",
			Msg:       "深度能订阅的档位根据vip等级变化",
		},
	}
}
