package okx_spot

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/base/base_orderbook"
	"actor/broker/base/orderbook_mediator"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

type OkxSpotWs struct {
	base.FatherWs
	pubWs *ws.WS // 继承ws
	priWs *ws.WS // 继承ws

	connectOnce sync.Once      // 连接一次
	id          int64          // 所有发送的消息都要唯一id
	stopCPri    chan struct{}  // 接受停机信号
	colo        bool           // 是否colo
	pubWsUrl    string         // 公共ws地址
	priWsUrl    string         // 私有ws地址
	restUrl     string         // rest地址
	client      *OkxSpotClient // 用来查询各种信息
	logged      bool           // 是否登录
	takerFee    atomic.Float64 // taker费率
	makerFee    atomic.Float64 // maker费率
	orderbook   *base_orderbook.Orderbook
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	if msg == nil {
		msg = &helper.TradeMsg{}
	}
	coloFlag := checkColo(params)
	w := &OkxSpotWs{
		colo:     coloFlag,
		pubWsUrl: okxWsPubUrl,
		priWsUrl: okxWsPriUrl,
		restUrl:  okxRestUrl,
	}
	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)
	rs0 := NewRs(params, msg, info, cb)
	rs, ok := rs0.(*OkxSpotRs)
	if !ok || rs == nil {
		helper.LogErrorThenCall("failed to create rs", cb.OnExit)
		return nil
	}
	rs.getExchangeInfo()
	w.UpdateExchangeInfo(rs.ExchangeInfoPtrP2S, rs.ExchangeInfoPtrS2P, w.Cb.OnExit)

	w.pubWs = ws.NewWS(w.pubWsUrl, params.LocalAddr, params.ProxyURL, w.pubHandler, cb.OnExit, params.BrokerConfig)
	w.pubWs.SetPingFunc(w.ping)
	w.pubWs.SetPingInterval(10)
	w.pubWs.SetSubscribe(w.subPub)

	if params.NeedAuth {
		w.priWs = ws.NewWS(w.priWsUrl, params.LocalAddr, params.ProxyURL, w.priHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetPingFunc(w.ping)
		w.priWs.SetSubscribe(w.subPri)
		w.priWs.SetPingInterval(10)
		w.priWs.AddReconnectPreHandler(w.priReconnectPreHandler)
	}

	w.client = NewClient(params, cb)
	connectionJudger := func(firstMatched bool, seq int64, s *base_orderbook.Slot) bool {
		return seq == s.ExPrevLastId
	}
	firstMatchJudger := func(snapSlot *base_orderbook.Slot, slot *base_orderbook.Slot) bool {
		return true
	}
	w.orderbook = base_orderbook.NewOrderbook(w.ExchangeName.String(), rs.Pair.String(), firstMatchJudger, connectionJudger, w.getOrderbookSnap, base_orderbook.SnapFetchType_WsConnectWithSeq, cb.OnExit, &w.TradeMsg.Depth)
	w.orderbook.SetDisableAutoUpdateDepth(params.DisableAutoUpdateDepth)
	mediator := orderbook_mediator.NewOrderbookMediator()
	w.orderbook.SetMediator(mediator)
	w.TradeMsg.Orderbook = mediator
	w.orderbook.SetDepthSubLevel(w.BrokerConfig.WsDepthLevel)

	w.AddWsConnections(w.pubWs, w.priWs)
	return w
}
func (w *OkxSpotWs) getOrderbookSnap() (*base_orderbook.Slot, error) {
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
func (w *OkxSpotWs) ping() []byte {
	return []byte("ping")
}

func (w *OkxSpotWs) subPub() error {
	// bbo
	if w.BrokerConfig.NeedTicker {
		p := map[string]interface{}{
			"op": "subscribe",
			"args": []interface{}{
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "bbo-tbt",
				},
				map[string]interface{}{
					"instId":  w.Symbol,
					"channel": "tickers",
				},
			},
		}
		w.pubWs.SubWithRetry("tickers", w.Cb.OnExit, func() []byte {
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			return msg
		})
	}
	// books5, 没有实现增量更新books，因为那个需要每次校对，校对失败重新连接ws, 以后彻底熟悉ws之后，会更新这部分，当前就books5先
	if w.BrokerConfig.WsDepthLevel > 0 {
		if w.BrokerConfig.WsDepthLevel > 5 {
			p := map[string]interface{}{
				"op": "subscribe",
				"args": []interface{}{
					map[string]interface{}{
						"instId":  w.Symbol,
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
		} else {
			p := map[string]interface{}{
				"op": "subscribe",
				"args": []interface{}{
					map[string]interface{}{
						"instId":  w.Symbol,
						"channel": "books5",
					},
				},
			}
			w.pubWs.SubWithRetry("books5", w.Cb.OnExit, func() []byte {
				msg, err := json.Marshal(p)
				if err != nil {
					w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
				}
				return msg
			})
		}
	}
	// partial depth https://binance-docs.github.io/apidocs/futures/cn/#6ae7c2b506
	//
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
	return nil
}
func (w *OkxSpotWs) priReconnectPreHandler() error {
	w.logged = false
	return nil
}

func (w *OkxSpotWs) subPri() error {
	// login first
	w.logged = false
	msg := getWsLogin(&w.BrokerConfig)
	w.priWs.SendMessage(msg)

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		if w.logged {
			break
		}
		// okx不允许重复发送登录
		// if i%2 == 0 {
		// msg := getWsLogin(w.params)
		// w.priWs.SendMessage(msg)
		// }
	}
	if !w.logged {
		w.Cb.OnExit("ws login failed")
		return errors.New("ws login failed")
	}
	w.realSubscribe()
	return nil
}

func (w *OkxSpotWs) realSubscribe() {
	args := make([]interface{}, 0)
	if w.BrokerConfig.SymbolAll {
		args = append(args,
			map[string]interface{}{
				"channel":  "orders",
				"instType": "SPOT",
			},
		)
		args = append(args,
			map[string]interface{}{
				"channel": "account",
			},
		)
	} else {
		for _, sym := range w.Symbols {
			args = append(args,
				map[string]interface{}{
					"channel":  "orders",
					"instType": "SPOT",
					"instId":   sym,
				},
			)
		}
		for _, a := range w.AssetsUpper {
			args = append(args,
				map[string]interface{}{
					"channel": "account",
					"ccy":     a,
				},
			)
		}
	}

	p := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	msg, err := json.Marshal(p)
	if err != nil {
		w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
	}
	w.priWs.SubWithRetry("priWs", w.Cb.OnExit, func() []byte { return msg })
}

// 处理公有ws的处理器
func (w *OkxSpotWs) pubHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debug(fmt.Sprintf("收到okx public ws推送 %s", msg))
	}

	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) != "pong" {
			w.Logger.Errorf("ws解析msg出错 msg: %v err: %v", value, err)
		}
		return
	}

	if value.Exists("data") {
		multi := w.PairInfo.Multi.Float()
		// 存在data的基本都是行情推送，优先级最高，立刻处理
		ch := value.Get("arg")
		symbol := helper.BytesToString(ch.GetStringBytes("instId"))
		if symbol == w.Symbol {
			op := helper.BytesToString(ch.GetStringBytes("channel"))
			data := value.GetArray("data")[0]
			switch op {
			case "bbo-tbt":
				if id, err := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); err == nil {
					t := &w.TradeMsg.Ticker
					if t.Seq.NewerAndStore(id, ts) {

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
					}
					w.Cb.OnTicker(ts)
				}
			case "tickers":
				if id, err := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); err == nil {
					t := &w.TradeMsg.Ticker
					if t.Seq.NewerAndStore(id, ts) {

						t.Ap.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("askPx"))))
						t.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("askSz"))) * multi)
						t.Bp.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("bidPx"))))
						t.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("bidSz"))) * multi)
						t.Mp.Store(t.Price())
					}
					w.Cb.OnTicker(ts)
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
				if id, err := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts"))); err == nil {
					{
						t := &w.TradeMsg.Ticker
						if t.Seq.NewerAndStore(id, ts) {

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
						}
						w.Cb.OnTicker(ts)
					}

					t := &w.TradeMsg.Depth
					if t.Seq.NewerAndStore(id, ts) {
						t.Lock.Lock()
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
					}
				}
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
	} else { // event常见于事件汇报，一般都可以忽略，只需要看看是否有error
		w.Logger.Infof(fmt.Sprintf("收到okx public ws推送 %v", value))
		e := helper.BytesToString(value.GetStringBytes("event"))
		if e == "error" {
			w.Logger.Errorf("[pub ws][%s] error , %v", w.ExchangeName.String(), value)
		} else if e == "subscribe" {
			ch := helper.BytesToString(value.GetStringBytes("arg", "channel"))
			w.pubWs.SetSubSuccess(ch)
			if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
	}
}

// 处理私有ws的处理器
func (w *OkxSpotWs) priHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE {
		w.Logger.Debugf("收到 pri ws 推送 %v", helper.BytesToString(msg))
	}
	// 解析
	p := wsPrivateHandyPool.Get()
	defer wsPrivateHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		if helper.BytesToString(msg) != "pong" {
			w.Logger.Errorf("ws解析msg出错 msg: %v err: %v", value, err)
		}
		return
	}

	if value.Exists("data") {
		ch := value.Get("arg")
		op := helper.BytesToString(ch.GetStringBytes("channel"))
		datas := value.GetArray("data")
		switch op {
		case "account":
			for _, data := range datas {
				details := data.GetArray("details")
				uTime := helper.MustGetInt64FromBytes(data, "uTime")
				for _, v := range details {
					ccy := helper.MustGetStringLowerFromBytes(v, "ccy")
					tot := helper.MustGetFloat64FromBytes(v, "cashBal")
					avail := helper.MustGetFloat64FromBytes(v, "availBal")
					fs := helper.FieldsSet_T(helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail)
					if e, ok := w.EquityNewerAndStore(ccy, uTime, ts, fs); ok {
						e.TotalWithoutUpl = tot
						e.Avail = avail
						w.Cb.OnEquityEvent(ts, *e)
					}
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
				case "canceled", "filled":
					order.Type = helper.OrderEventTypeREMOVE
					szStr := helper.BytesToString(data.GetStringBytes("accFillSz"))
					if szStr != "" && szStr != "0" {
						order.Filled = fixed.NewS(szStr).Mul(info.Multi)
						order.FilledPrice = fixed.NewS(helper.BytesToString(data.GetStringBytes("avgPx"))).Float()
						//order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))))
						order.CashFee = fixed.NewS(helper.BytesToString(data.GetStringBytes("rebate"))).Sub(fixed.NewS(helper.BytesToString(data.GetStringBytes("fee"))))
					}
				default:
					order.Type = helper.OrderEventTypeNEW
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
	} else {
		w.Logger.Infof("收到 pri ws 推送 %v", helper.BytesToString(msg))
		e := helper.BytesToString(value.GetStringBytes("event"))
		switch e {
		case "error":
			w.Logger.Errorf("[pri ws][%s] error , %v", w.ExchangeName.String(), value)
		case "subscribe":
			w.priWs.SetSubSuccess("priWs")
			if ws.AllWsSubsuccess(w.pubWs, w.priWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		case "login":
			// 登录订阅放在这里
			w.logged = true
		}
	}
}

func (w *OkxSpotWs) Run() {
	w.connectOnce.Do(func() {
		if w.BrokerConfig.NeedPubWs() {
			var err1 error
			w.StopCPub, err1 = w.pubWs.Serve()
			if err1 != nil {
				w.Cb.OnExit("public ws 连接失败")
			}
		}
		if w.BrokerConfig.NeedAuth {
			var err2 error
			w.stopCPri, err2 = w.priWs.Serve()
			if err2 != nil {
				w.Cb.OnExit("private ws 连接失败")
			}
		}
	})
}

func (w *OkxSpotWs) DoStop() {
	if w.StopCPub != nil {
		helper.CloseSafe(w.StopCPub)
	}
	if w.stopCPri != nil {
		helper.CloseSafe(w.stopCPri)
	}
	w.connectOnce = sync.Once{}
}

// GetFee 获取费率
func (w *OkxSpotWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.makerFee.Load(), Taker: w.takerFee.Load()}, helper.ApiErrorNotImplemented
}

func (w *OkxSpotWs) WsLogged() bool {
	return w.logged
}

func (w *OkxSpotWs) GetPriWs() *ws.WS {
	return w.priWs
}
func (w *OkxSpotWs) GetPubWs() *ws.WS {
	return w.pubWs
}
