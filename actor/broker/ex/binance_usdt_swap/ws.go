package binance_usdt_swap

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	base_orderbook "actor/broker/base/base_orderbook"
	"actor/broker/base/orderbook_mediator"
	"actor/broker/brokerconfig"
	"actor/broker/client/ws"
	"actor/helper"
	"actor/third/fixed"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
	"go.uber.org/atomic"
)

var (
	BaseWsUrl = "wss://fstream.binance.com/ws/"
)

var (
	wsPublicHandyPool fastjson.ParserPool

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

type BinanceUsdtSwapWs struct {
	base.FatherWs
	// BrokerConfig           *helper.BrokerConfig // 配置文件
	pubWs            *ws.WS        // 继承ws
	priWs            *ws.WS        // 继承ws
	connectOnce      sync.Once     // 连接一次
	authOnce         sync.Once     // 鉴权一次
	id               int64         // 所有发送的消息都要唯一id
	stopCPri         chan struct{} // 接受停机信号
	colo             bool          // 是否colo
	symbol_upper     string        // 交易对大写
	baseWsUrl        string
	listenKey        atomic.String
	getListenKeyTime atomic.Int64
	rs               *BinanceUsdtSwapRs
	//
	takerFee atomic.Float64 // taker费率
	makerFee atomic.Float64 // maker费率
	//
	acctUpdateTs   int64      // 资金信息更新时间戳
	acctUpdateLock sync.Mutex // 资金更新锁
	reConnectTs    int64      // priWs重连时间戳
	reConnectLock  sync.Mutex // priWs重连锁
	//
	totalPosValueAndUnrealizedProfit _TotalPosValueAndUnrealizedProfit // 存放收到ws的account推送时，保存所有币对的仓位的价值与未实现盈亏，用于计算可用保证金。
	orderbook                        *base_orderbook.Orderbook
	stopCtx                          context.Context
	stopFunc                         context.CancelFunc
	// TODO 最近一次收到 onTrade 的时间
	lastTradeTsNs int64
	mmap          *helper.Mmap
}

func NewWs(params *helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	// 判断colo

	baseWsUrl := BaseWsUrl
	cfg := brokerconfig.BrokerSession()
	if !params.BanColo && cfg.BinanceUsdtSwapWsUrl != "" {
		baseWsUrl = cfg.BinanceUsdtSwapWsUrl
		params.Logger.Infof("binance_usdt_swap ws启用colo  %v", baseWsUrl)
	}
	//if cfg.BinanceUsdtSwapProxy != "" {
	//	proxies := strings.Split(cfg.BinanceUsdtSwapProxy, ",")
	//	rand.Seed(time.Now().Unix())
	//	i := rand.Int31n(int32(len(proxies)))
	//	proxy := proxies[i]
	//	params.ProxyURL = proxy
	//	w.Logger.Infof("binance usdt swap ws 启用代理: %v", params.ProxyURL)
	//}
	//
	w := &BinanceUsdtSwapWs{
		baseWsUrl: baseWsUrl,
	}
	base.InitFatherWs(msg, w, &w.FatherWs, params, info, cb)
	w.BrokerConfig = *params
	// if params.WsDepthLevel > 0 && params.BrokerConfig.NeedPartialDeprecated {
	// 	helper.LogErrorThenCall("bn swap不能同时订阅depth and partial", cb.OnExit)
	// 	return nil
	// }
	w.pubWs = ws.NewWS(w.baseWsUrl+"qqlh", params.LocalAddr, params.ProxyURL, w.pubHandler, cb.OnExit, params.BrokerConfig, params.ExcludeIpsAsPossible)
	if params.BrokerConfig.RawDataMmapCollectPath != "" {
		hookedHandler := func(msg []byte, ts int64) {
			w.mmap.Write(msg, ts)
			w.pubHandler(msg, ts)
		}
		w.pubWs = ws.NewWS(w.baseWsUrl+"qqlh", params.LocalAddr, params.ProxyURL, hookedHandler, cb.OnExit, params.BrokerConfig, params.ExcludeIpsAsPossible)
		w.mmap, _ = helper.NewMmap(params.BrokerConfig.RawDataMmapCollectPath, w.ExchangeName.String()+"_ws_"+w.Pair.String())
		if params.WsDepthLevel > 0 {
			go func() {
				for {
					_, e := w.rs.getOrderbookSnap()
					if e != nil {
						time.Sleep(time.Minute)
					} else {
						time.Sleep(time.Hour)
					}
				}
			}()
		}
	}
	w.stopCtx, w.stopFunc = context.WithCancel(context.Background())

	rs := NewRs(
		params,
		msg,
		info,
		helper.CallbackFunc{
			OnExit:  cb.OnExit,
			OnReset: cb.OnReset,
		})
	var ok bool
	w.rs, ok = rs.(*BinanceUsdtSwapRs)
	if !ok {
		w.Logger.Errorf(" rs 转换失败")
		w.Cb.OnExit(" rs 转换失败")
		return nil
	}
	w.rs.getExchangeInfo()
	w.UpdateExchangeInfo(w.rs.ExchangeInfoPtrP2S, w.rs.ExchangeInfoPtrS2P, w.Cb.OnExit)

	connectionJudger := func(firstMatched bool, seq int64, s *base_orderbook.Slot) bool {
		return seq == s.ExPrevLastId
	}
	firstMatchJudger := func(snapSlot *base_orderbook.Slot, slot *base_orderbook.Slot) bool {
		return slot.ExFirstId <= snapSlot.ExPrevLastId && slot.ExLastId >= snapSlot.ExPrevLastId
	}
	w.orderbook = base_orderbook.NewOrderbook(w.ExchangeName.String(), params.Pairs[0].String(), firstMatchJudger, connectionJudger, w.rs.getOrderbookSnap, base_orderbook.SnapFetchType_Rs, cb.OnExit, &w.TradeMsg.Depth)
	w.orderbook.SetDisableAutoUpdateDepth(params.DisableAutoUpdateDepth)
	mediator := orderbook_mediator.NewOrderbookMediator()
	w.orderbook.SetMediator(mediator)
	w.TradeMsg.Orderbook = mediator
	// 底层有自动ping pong 不需要专用ping pong函数
	//w.ws.SetPingFunc(w.ping)
	w.symbol_upper = strings.ToUpper(w.Symbol)
	w.totalPosValueAndUnrealizedProfit = _TotalPosValueAndUnrealizedProfit{
		data:  make(map[string]*_PosValueAndUnrealizedProfit, 0),
		mutex: sync.Mutex{},
	}
	w.pubWs.SetSubscribe(w.subPub)
	w.pubWs.SetPongFunc(pong)
	w.pubWs.SetPingInterval(90)
	if params.NeedAuth {
		w.priWs = ws.NewWS(w.baseWsUrl, params.LocalAddr, params.ProxyURL, w.priHandler, cb.OnExit, params.BrokerConfig)
		w.priWs.SetSubscribe(w.subPri)
		w.priWs.SetPongFunc(pong)
		w.priWs.SetPingInterval(90)
	}
	w.AddWsConnections(w.pubWs, w.priWs)
	return w
}

func pong(ping []byte) []byte {
	return ping
}

func (w *BinanceUsdtSwapWs) subPub() error {
	// 公有订阅
	// bbo 1档的orderbook
	if w.BrokerConfig.NeedTicker {
		w.id++
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []interface{}{fmt.Sprintf("%s@bookTicker", strings.ToLower(w.Symbol))},
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}

	// depth 0ms 可能将来会被移除 orderbook
	if w.BrokerConfig.WsDepthLevel > 0 {
		// 小档全量更快
		finalWsDepthLevel, ok := helper.SuitableLevel([]int{5, 10, 20}, w.BrokerConfig.WsDepthLevel)
		if ok {
			w.UsingDepthType = base.DepthTypeSnapshot
			w.id++
			p := map[string]interface{}{
				"method": "SUBSCRIBE",
				"params": []interface{}{fmt.Sprintf("%s@depth%d@0ms", strings.ToLower(w.Symbol), finalWsDepthLevel)},
				"id":     w.id,
			}
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName.String(), err)
			}
			w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
		} else {
			w.UsingDepthType = base.DepthTypePartial
			w.orderbook.SetDepthSubLevel(w.BrokerConfig.WsDepthLevel)
			w.id++
			p := map[string]interface{}{
				"method": "SUBSCRIBE",
				"params": []interface{}{fmt.Sprintf("%s@depth@100ms", strings.ToLower(w.Symbol))},
				"id":     w.id,
			}
			msg, err := json.Marshal(p)
			if err != nil {
				w.Logger.Errorf("[ws][%s] json encode error , %s", err)
			}
			w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
		}
	}
	if w.BrokerConfig.NeedTrade || w.BrokerConfig.NeedTicker {
		w.id++
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []interface{}{fmt.Sprintf("%s@aggTrade", strings.ToLower(w.Symbol))},
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}

	if w.BrokerConfig.Need1MinKline {
		w.id++
		subList := []interface{}{}
		for _, pair := range w.Pairs {
			symbol := w.PairToSymbol(&pair)
			subList = append(subList, fmt.Sprintf("%s@kline_1m", strings.ToLower(symbol)))
		}
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": subList,
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}

	if w.BrokerConfig.NeedIndex {
		w.id++
		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []interface{}{fmt.Sprintf("%s@markPrice", strings.ToLower(w.Symbol))},
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}
	if w.BrokerConfig.NeedMarketLiquidation {
		w.id++

		symbols := w.Symbols
		if w.SymbolMode == helper.SymbolMode_All {
			symbols = []string{"!forceOrder@arr"}
		} else {
			for i, _ := range symbols {
				symbols[i] = strings.ToLower(symbols[i]) + "@forceOrder"
			}
		}

		p := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": symbols,
			"id":     w.id,
		}
		msg, err := json.Marshal(p)
		if err != nil {
			w.Logger.Errorf("[ws][%s] json encode error , %s", w.ExchangeName, err)
		}
		w.pubWs.SubWithRetry(helper.Itoa(w.id), w.Cb.OnExit, func() []byte { return msg })
	}
	return nil
}

func (w *BinanceUsdtSwapWs) getListenKey() {
	// 获取 listenkey
	for i := 0; i < 5; i++ {
		key, err := w.rs.generateListenKey()
		if err != nil {
			w.Logger.Infof("获取到listenkey失败: %v", err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		w.Logger.Infof("获取到listenkey: %v", key)
		w.listenKey.Store(key)
		w.getListenKeyTime.Store(time.Now().Unix())
		w.priWs.SetWsUrl(fmt.Sprintf("%s%s", w.baseWsUrl, w.listenKey.Load()))
		break
	}
}

func (w *BinanceUsdtSwapWs) deleteListenKey() {
	// 获取 listenkey
	targitKey := w.listenKey.Load()
	if targitKey != "" {
		err := w.rs.deleteListenKey(targitKey)
		if err != nil {
			w.Logger.Infof("删除listenkey失败: %v", err)
			time.Sleep(time.Second)
		} else {
			w.Logger.Infof("关闭listenkey成功: %v", targitKey)
		}
	}
}

func (w *BinanceUsdtSwapWs) subPri() error {
	// 私有订阅
	if ws.AllWsSubsuccess(w.priWs, w.pubWs) {
		w.Cb.OnWsReady(w.ExchangeName)
	}
	return nil
}

func (w *BinanceUsdtSwapWs) GetBindIp() string {
	return w.pubWs.GetBindIp()
}

// Run 准备ws连接 仅第一次调用时连接ws
func (w *BinanceUsdtSwapWs) Run() {
	w.connectOnce.Do(func() {
		if w.BrokerConfig.NeedPubWs() {
			var err1 error
			w.StopCPub, err1 = w.pubWs.Serve()
			if err1 != nil {
				w.Cb.OnExit("binance usdt swap public ws 连接失败")
			}
		}
		if w.BrokerConfig.NeedAuth {
			var err2 error
			w.getListenKey()
			w.stopCPri, err2 = w.priWs.Serve()
			if err2 != nil {
				w.Cb.OnExit("binance usdt swap private ws 连接失败")
				return
			}
			go func() {
				// 随机打散, 避免同一台机器太多实例同时启动
				waitSec := rand.Intn(100)
				ticker := time.NewTicker(time.Second * time.Duration(300+waitSec))
				for {
					select {
					case <-w.stopCtx.Done():
						return
					case <-ticker.C:
						w.getListenKeyTime.Store(time.Now().Unix())
						w.Logger.Infof("触发listenkey续期操作")
						w.rs.keepListenKey(w.listenKey.Load())
					}
				}
			}()
		}

	})
}

func (w *BinanceUsdtSwapWs) DoStop() {
	if w.StopCPub != nil {
		helper.CloseSafe(w.StopCPub)
	}
	if w.BrokerConfig.NeedAuth {
		if w.stopCPri != nil {
			helper.CloseSafe(w.stopCPri)
		}
	}
	w.stopFunc()
	w.connectOnce = sync.Once{}
}

// func (w *BinanceUsdtSwapWs) restartWebSocket() {
// 	w.DoStop()
// 	w.orderbook.Reset()
// 	time.Sleep(time.Second)
// 	w.Run()
// 	w.Logger.Infof("WebSocket重启完成")
// }

/**
goos: darwin
goarch: amd64
pkg: go-test/bn
cpu: 12th Gen Intel(R) Core(TM) i7-12700F
BenchmarkParseByIndex-20        10468501               117.6 ns/op             0 B/op          0 allocs/op
BenchmarkParseBytes-20           1203034              1001 ns/op            3384 B/op         11 allocs/op
*/

/**
goos: linux
goarch: amd64
pkg: actor/bn
cpu: Intel Xeon Processor (Cascadelake)
BenchmarkParseByIndex-2   	 4633628	       235.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseBytes-2     	  531594	      2410 ns/op	    3384 B/op	      11 allocs/op
*/

func (w *BinanceUsdtSwapWs) OptimistParseBBO(msg []byte, ts int64) bool {
	const _BBO_MIN_LEN = 120
	if len(msg) < _BBO_MIN_LEN {
		return false
	}
	c := helper.BytesToString(msg)
	idxStart := strings.Index(c[19:22], "u") // u: 位置在19, 算上时间增长
	if idxStart < 0 {
		return false
	}
	idxStart += 19 + 1
	offset := strings.Index(c[idxStart:], ":")
	idxStart += offset + 1
	// idxEnd := strings.Index(c[idxStart:], ",") + idxStart // u当seq
	// seq, err := strconv.ParseInt(c[idxStart:idxEnd], 10, 64)
	// if err != nil {
	// 	return false
	// }
	// if !w.tradeMsg.Ticker.Seq.NewerAndStore(seq, ts) {
	// 	return true
	// }

	var err error
	idxStart = strings.Index(c[48:68], "b") // "s:" 位置在48, 只要symbol不太长， "b:" 位置在100以内
	idxStart += 48
	var vals [4]float64
	for i := 0; i < 4; i++ {
		idxColon := strings.Index(c[idxStart:], ":") + idxStart
		idxColon++
		idxComma := strings.Index(c[idxColon+1:], "\"") + idxColon + 1
		vals[i], err = strconv.ParseFloat(c[idxColon+1:idxComma], 64)
		if err != nil {
			return false
		}
		idxStart = idxComma + 1
	}

	offset = strings.Index(c[idxStart:], "T") // T当seq
	if offset < 0 {
		return false
	}
	idxStart += offset + 3
	// offset = strings.Index(c[idxStart:], ",")
	idxEnd := strings.Index(c[idxStart:], ",") + idxStart
	seq, err := strconv.ParseInt(c[idxStart:idxEnd], 10, 64)
	if err != nil {
		return false
	}

	offset = strings.Index(c[idxStart:], "E") // T当seq
	if offset < 0 {
		return false
	}
	idxStart += offset + 3
	seqE, err := strconv.ParseInt(c[idxStart:idxStart+13], 10, 64)
	if err != nil {
		return false
	}

	if !w.TradeMsg.Ticker.Seq.NewerAndStore(seq, ts) {
		return true
	}

	bp := vals[0]
	ap := vals[2]
	mp := (bp + ap) / 2

	// TODO 当有最新 onTrade 数据 并且 价格 偏离较大时 更信赖 onTrade 或者 如果价格偏离不大 则信赖 bbo
	// TODO 修改这里逻辑务必经过我通过 @thousandquant 禁止私自改动!!!!!!!!!!!!!!!!! 2025-02-13
	trustBBO := false
	if ts-w.lastTradeTsNs > 500 {
		trustBBO = true
	} else if math.Abs(w.TradeMsg.Ticker.Price()-mp)/mp < 1e-4 {
		trustBBO = true
	}
	if trustBBO {
		w.TradeMsg.Ticker.Bp.Store(bp)
		w.TradeMsg.Ticker.Bq.Store(vals[1])
		w.TradeMsg.Ticker.Ap.Store(ap)
		w.TradeMsg.Ticker.Aq.Store(vals[3])
		w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
		tsMs := time.Now().UnixMilli()
		w.TradeMsg.Ticker.Delay.Store(tsMs - seq)
		w.TradeMsg.Ticker.DelayE.Store(tsMs - seqE)
		w.Cb.OnTicker(ts)
	}
	return true
}

func (w *BinanceUsdtSwapWs) pubHandler(msg []byte, ts int64) {
	// 解析
	if helper.DEBUGMODE && helper.DEBUG_PRINT_MARKETDATA {
		w.Logger.Debugf("收到pub ws推送 %v", string(msg))
	}
	w.TotalPubWsMsgCnt++
	if !helper.DEBUGMODE || (base.IsUtf && !base.SkipOptimistParseBBO) {
		if w.OptimistParseBBO(msg, ts) {
			w.OptimistBBOCnt++
			// if helper.DEBUGMODE {
			// w.Logger.Debugf("tryParseBBO success")
			// }
			return
		}
	}

	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		w.Logger.Errorf("Binance usdt swap ws解析msg出错 err:%v", err)
		return
	}
	//更新本地数据 并触发相应回调
	if value.Exists("e") {
		e := helper.BytesToString(value.GetStringBytes("e"))
		switch e {
		case "bookTicker":
			//{
			//	"e":"bookTicker",     // 事件类型
			//	"u":400900217,        // 更新ID
			//	"E": 1568014460893,   // 事件推送时间
			//	"T": 1568014460891,   // 撮合时间
			//	"s":"BNBUSDT",        // 交易对
			//	"b":"25.35190000",    // 买单最优挂单价格
			//	"B":"31.21000000",    // 买单最优挂单数量
			//	"a":"25.36520000",    // 卖单最优挂单价格
			//	"A":"40.66000000"     // 卖单最优挂单数量
			//}
			seq := helper.MustGetInt64(value, "T")
			seqE := helper.MustGetInt64(value, "E")
			//w.Logger.Infof("收到bbo Id:%v", t)
			// if t > w.tradeMsg.Ticker.ID.Load() {
			if w.TradeMsg.Ticker.Seq.NewerAndStore(seq, ts) {
				ap := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("a")))
				bp := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("b")))
				mp := (bp + ap) / 2
				// TODO 当有最新 onTrade 数据 并且 价格 偏离较大时 更信赖 onTrade 如果价格偏离不大 则信赖 bbo
				// TODO 修改这里逻辑务必经过我通过 @thousandquant 禁止私自改动!!!!!!!!!!!!!!!!! 2025-02-13
				trustBBO := false
				if ts-w.lastTradeTsNs > 500 {
					trustBBO = true
				} else if math.Abs(w.TradeMsg.Ticker.Price()-mp)/mp < 1e-4 {
					trustBBO = true
				}
				if trustBBO {
					w.TradeMsg.Ticker.Ap.Store(ap)
					w.TradeMsg.Ticker.Aq.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("A"))))
					w.TradeMsg.Ticker.Bp.Store(bp)
					w.TradeMsg.Ticker.Bq.Store(fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("B"))))
					w.TradeMsg.Ticker.Mp.Store(w.TradeMsg.Ticker.Price())
					if helper.DEBUGMODE {
						helper.MustMillis(seq)
					}
					tsMs := time.Now().UnixMilli()
					w.TradeMsg.Ticker.Delay.Store(tsMs - seq)
					w.TradeMsg.Ticker.DelayE.Store(tsMs - seqE)
					w.Cb.OnTicker(ts)
					if helper.DEBUGMODE && base.IsUtf && base.CollectTCData {
						w.Cb.Collect(ts, string(msg), &w.TradeMsg.Ticker)
					}
				}
			}
		case "depthUpdate":

			if w.UsingDepthType == base.DepthTypePartial {
				bids0 := value.GetArray("b")
				asks0 := value.GetArray("a")
				bidLen := len(bids0)
				slot := w.orderbook.GetFreeSlot(bidLen+len(asks0), ts)
				slot.ExTsMs = helper.MustGetInt64(value, "T")
				slot.ExFirstId = helper.MustGetInt64(value, "U")
				slot.ExLastId = helper.MustGetInt64(value, "u")
				slot.ExPrevLastId = helper.MustGetInt64(value, "pu")
				for _, v := range bids0 {
					s := v.GetArray()
					p := helper.MustGetFloat64FromBytes(s[0])
					a := helper.MustGetFloat64FromBytes(s[1])
					slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
				}
				slot.AskStartIdx = bidLen
				for _, v := range asks0 {
					s := v.GetArray()
					p := helper.MustGetFloat64FromBytes(s[0])
					a := helper.MustGetFloat64FromBytes(s[1])
					slot.PriceItems = append(slot.PriceItems, [2]float64{p, a})
				}

				if w.orderbook.InsertSlot(slot) {
					seq := helper.MustGetInt64(value, "T")
					if w.BrokerConfig.NeedTicker && w.TradeMsg.Ticker.Seq.NewerAndStore(seq, ts) {
						_ = helper.DEBUGMODE && helper.MustMillis(seq)
						t := &w.TradeMsg.Ticker
						ask := w.TradeMsg.Depth.Ask()
						bid := w.TradeMsg.Depth.Bid()
						t.Set(ask.Price, ask.Amount, bid.Price, bid.Amount)
						w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - seq)
						w.Cb.OnTicker(ts)
					}
					w.TradeMsg.Depth.Seq.NewerAndStore(slot.ExTsMs, ts)
					w.Cb.OnDepth(ts)
				}
			} else if w.UsingDepthType == base.DepthTypeSnapshot {

				seq := helper.MustGetInt64(value, "T")
				bids := value.GetArray("b")
				asks := value.GetArray("a")

				w.TradeMsg.Depth.Lock.Lock()
				if !w.TradeMsg.Depth.Seq.NewerAndStore(seq, ts) {
					w.TradeMsg.Depth.Lock.Unlock()
					return
				}
				w.TradeMsg.Depth.Bids, w.TradeMsg.Depth.Asks = w.TradeMsg.Depth.Bids[:0], w.TradeMsg.Depth.Asks[:0]
				end := min(len(bids), len(asks), w.BrokerConfig.WsDepthLevel)
				for k := 0; k < end; k++ {
					_bids, _ := bids[k].Array()
					_bidPrice, _ := _bids[0].StringBytes()
					_bidQty, _ := _bids[1].StringBytes()
					w.TradeMsg.Depth.Bids = append(w.TradeMsg.Depth.Bids,
						helper.DepthItem{Price: fastfloat.ParseBestEffort(helper.BytesToString(_bidPrice)), Amount: fastfloat.ParseBestEffort(helper.BytesToString(_bidQty))})
				}

				for k := 0; k < end; k++ {
					_asks, _ := asks[k].Array()
					_askPrice, _ := _asks[0].StringBytes()
					_askQty, _ := _asks[1].StringBytes()
					w.TradeMsg.Depth.Asks = append(w.TradeMsg.Depth.Asks,
						helper.DepthItem{Price: fastfloat.ParseBestEffort(helper.BytesToString(_askPrice)), Amount: fastfloat.ParseBestEffort(helper.BytesToString(_askQty))})
				}
				w.TradeMsg.Depth.Lock.Unlock()
				if w.BrokerConfig.NeedTicker && w.TradeMsg.Ticker.Seq.NewerAndStore(seq, ts) {
					_ = helper.DEBUGMODE && helper.MustMillis(seq)
					t := &w.TradeMsg.Ticker
					ask := w.TradeMsg.Depth.Ask()
					bid := w.TradeMsg.Depth.Bid()
					t.Set(ask.Price, ask.Amount, bid.Price, bid.Amount)
					w.TradeMsg.Ticker.Delay.Store(time.Now().UnixMilli() - seq)
					w.Cb.OnTicker(ts)
				}
				w.Cb.OnDepth(ts)
			}

		case "aggTrade":

			price := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("p")))
			qty := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("q")))
			// id := helper.Itoa(helper.MustGetInt64(value, "a"))
			// tsMsEx := helper.MustGetInt64(value, "T")
			// https://binance-docs.github.io/apidocs/futures/en/#live-subscribing-unsubscribing-to-streams
			// Is the buyer the market maker?
			// TODO 跪求各位开发 千万别把这个改错了 非常容易出错 一定要看清楚 改之前一定要问我 @thousandquant
			if value.GetBool("m") {
				w.TradeMsg.Trade.Update(helper.TradeSideSell, qty, price)
				// TODO 用 trade 修正ticker 并且要记录 onTrade 时间戳 防止 滞后的 onTicker 修改价格变错误
				// TODO 卖出的时候 如果 bp 需要修正到 price  ap 如果大于 price 则不需要修正 如果 ap 小于 price 则需要修正
				// TODO 修改这里逻辑务必经过我通过 @thousandquant 禁止私自改动!!!!!!!!!!!!!!!!! 2025-02-13
				w.TradeMsg.Ticker.Mp.Store(price)
				w.TradeMsg.Ticker.Bp.Store(price)
				lastAp := w.TradeMsg.Ticker.Ap.Load()
				if lastAp > price {

				} else {
					w.TradeMsg.Ticker.Ap.Store(price)
				}
			} else {
				w.TradeMsg.Trade.Update(helper.TradeSideBuy, qty, price)
				// TODO 用 trade 修正ticker 并且要记录 onTrade 时间戳 防止 滞后的 onTicker 修改价格变错误
				// TODO 买入的时候 如果 ap 需要修正到 price  bp 如果小于 price 则不需要修正 如果 bp 大于 price 则需要修正
				// TODO 修改这里逻辑务必经过我通过 @thousandquant 禁止私自改动!!!!!!!!!!!!!!!!! 2025-02-13
				w.TradeMsg.Ticker.Mp.Store(price)
				w.TradeMsg.Ticker.Ap.Store(price)
				lastBp := w.TradeMsg.Ticker.Bp.Load()
				if lastBp < price {

				} else {
					w.TradeMsg.Ticker.Bp.Store(price)
				}
			}
			w.lastTradeTsNs = ts
			w.Cb.OnTrade(ts)
		// todo index
		/**
		case "xxxindex":
			fakePrice := 234.2348
			w.tradeMsg.Index.IndexPrice.Store(fakePrice)
			w.Cb.OnIndex(ts)
		*/
		case "markPriceUpdate":
			indexPrice := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("i")))
			markPrice := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("p")))
			//fundingRate := fastfloat.ParseBestEffort(helper.BytesToString(value.GetStringBytes("r")))
			// w.TradeMsg.Index.IndexPrice.Store(indexPrice)
			// w.TradeMsg.Index.MarkPrice.Store(markPrice)
			if w.Cb.OnIndex != nil {
				w.Cb.OnIndex(ts, helper.IndexEvent{IndexPrice: indexPrice})
			}
			if w.Cb.OnMark != nil {
				w.Cb.OnMark(ts, helper.MarkEvent{MarkPrice: markPrice})
			}
		case "forceOrder":
			// "e":"forceOrder",                   // Event Type
			// "E":1568014460893,                  // Event Time
			// "o":{
			// 	"s":"BTCUSDT",                   // Symbol
			// 	"S":"SELL",                      // Side
			// 	"o":"LIMIT",                     // Order Type
			// 	"f":"IOC",                       // Time in Force
			// 	"q":"0.014",                     // Original Quantity
			// 	"p":"9910",                      // Price
			// 	"ap":"9910",                     // Average Price
			// 	"X":"FILLED",                    // Order Status
			// 	"l":"0.014",                     // Order Last Filled Quantity
			// 	"z":"0.014",                     // Order Filled Accumulated Quantity
			// 	"T":1568014460893,          	 // Order Trade Time
			// }
			data := value.Get("o")
			// OrderStatus := helper.MustGetStringFromBytes(data, ("X"))
			Symbol := helper.MustGetStringFromBytes(data, "s")
			Side := helper.MustGetStringFromBytes(data, ("S"))
			OrderType := helper.MustGetStringFromBytes(data, ("o"))
			OriginalQty := helper.MustGetFloat64FromBytes(data, ("q"))
			Price := helper.MustGetFloat64FromBytes(data, ("p"))
			AveragePrice := helper.MustGetFloat64FromBytes(data, ("ap"))
			FilledAccumulatedQty := helper.MustGetFloat64FromBytes(data, ("z"))
			OrderTradeTime := helper.MustGetInt64(data, "T")
			EventTimeMs := helper.MustGetInt64(value, "E")

			event := helper.MarketLiquidationEvent{}
			event.Symbol = Symbol
			if Side == "SELL" {
				event.Side = helper.OrderSidePD
			} else {
				event.Side = helper.OrderSidePK
			}
			switch OrderType {
			case "LIMIT":
				event.OrderType = helper.OrderTypeLimit
			case "MARKET":
				event.OrderType = helper.OrderTypeMarket
			}
			event.OrderQuantity = fixed.NewF(OriginalQty)
			event.OrderPrice = fixed.NewF(Price)
			event.FilledPrice = AveragePrice
			event.FilledQuantity = FilledAccumulatedQty
			event.TradeTimeMs = OrderTradeTime
			event.EventTimeMs = EventTimeMs
			w.Cb.OnMarketLiquidationEvent(ts, event)
		case "kline":
			//  {"e":"kline","E":1736218212557,"s":"SOLUSDT","k":{"t":1736218200000, "T":1736218259999, "s":"SOLUSDT", "i":"1m", "f":1988667843, "L":1988667990, "o":"216.9100", "c":"216.9800", "h":"216.9900", "l":"216.8800", "v":"1382", "n":148, "x":false, "q":"299786.5100", "V":"1063", "Q":"230593.9100", "B":"0"}}
			// {
			// 	"e": "kline",     // Event type
			// 	"E": 1638747660000,   // Event time
			// 	"s": "BTCUSDT",    // Symbol
			// 	"k": {
			// 	  "t": 1638747660000, // Kline start time
			// 	  "T": 1638747719999, // Kline close time
			// 	  "s": "BTCUSDT",  // Symbol
			// 	  "i": "1m",      // Interval
			// 	  "f": 100,       // First trade ID
			// 	  "L": 200,       // Last trade ID
			// 	  "o": "0.0010",  // Open price
			// 	  "c": "0.0020",  // Close price
			// 	  "h": "0.0025",  // High price
			// 	  "l": "0.0015",  // Low price
			// 	  "v": "1000",    // Base asset volume
			// 	  "n": 100,       // Number of trades
			// 	  "x": false,     // Is this kline closed?
			// 	  "q": "1.0000",  // Quote asset volume
			// 	  "V": "500",     // Taker buy base asset volume
			// 	  "Q": "0.500",   // Taker buy quote asset volume
			// 	  "B": "123456"   // Ignore
			// 	}
			//   }

			// 每分钟K线闭合时回调
			kline := value.Get("k")
			closed := kline.GetBool("x")
			if !closed {
				return
			}
			symbol := helper.MustGetStringFromBytes(value, "s")
			tsEx := helper.MustGetInt64(value, "E")
			start := helper.MustGetInt64(kline, "t")
			end := helper.MustGetInt64(kline, "T")
			open := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("o")))
			close := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("c")))
			high := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("h")))
			low := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("l")))
			volume := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("v"))) //qty
			takerBuyVolume := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("V")))
			turnover := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("q"))) //volume
			takerBuyTurnover := fastfloat.ParseBestEffort(helper.BytesToString(kline.GetStringBytes("Q")))
			numberOfTrades := helper.MustGetInt(kline, "n")
			w.Cb.OnKline(ts, helper.NewKline(symbol, start, end, tsEx, open, close, high, low, float64(numberOfTrades), 0, takerBuyVolume, volume-takerBuyVolume, takerBuyTurnover, turnover-takerBuyTurnover, ts))
		}
	} else {
		if value.Exists("id") {
			id := helper.MustGetInt(value, "id")
			if value.Get("result").Type() == fastjson.TypeNull {
				w.pubWs.SetSubSuccess(helper.Itoa(id))
			}
			if ws.AllWsSubsuccess(w.priWs, w.pubWs) {
				w.Cb.OnWsReady(w.ExchangeName)
			}
		}
		w.Logger.Infof(fmt.Sprintf("收到binance public ws推送 %v", value))
	}
}

type _TotalPosValueAndUnrealizedProfit struct {
	data  map[string]*_PosValueAndUnrealizedProfit
	mutex sync.Mutex
}

type _PosValueAndUnrealizedProfit struct {
	posMargin        float64 // 仓位占用保证金
	unrealizedProfit float64
}

func (w *BinanceUsdtSwapWs) priHandler(msg []byte, ts int64) {
	if helper.DEBUGMODE {
		w.Logger.Debugf("[%p]收到 pri ws 推送 %s", w, string(msg))
	}
	// 解析
	p := wsPublicHandyPool.Get()
	defer wsPublicHandyPool.Put(p)
	value, err := p.ParseBytes(msg)
	if err != nil {
		w.Logger.Errorf("Binance usdt swap ws解析msg出错 err:%v", err)
		return
	}
	// log
	//w.Logger.Debug(fmt.Sprintf("收到binance private ws推送 %v", value))
	// 解析信息
	if value.Exists("e") {
		e := helper.BytesToString(value.GetStringBytes("e"))
		seq := helper.MustGetInt64(value, "T")
		switch e {
		case "ORDER_TRADE_UPDATE":
			//"e":"ORDER_TRADE_UPDATE",         // 事件类型
			//"E":1568879465651,                // 事件时间
			//"T":1568879465650,                // 撮合时间
			data := value.Get("o")
			symbol := helper.BytesToString(data.GetStringBytes("s"))
			info, ok := w.GetPairInfoBySymbol(symbol)
			if !ok {
				if helper.DEBUGMODE {
					w.Logger.Warnf("unwant symbol event. ", symbol)
				}
				return
			}
			status := helper.BytesToString(data.GetStringBytes("X"))
			statusThisTime := helper.BytesToString(data.GetStringBytes("x"))
			order := helper.OrderEvent{Pair: info.Pair}
			order.ID = seq
			if status == "NEW" {
				order.Type = helper.OrderEventTypeNEW
			} else if status == "CANCELED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else if status == "PARTIALLY_FILLED " {
				order.Type = helper.OrderEventTypePARTIAL
			} else if status == "FILLED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else if status == "EXPIRED" {
				order.Type = helper.OrderEventTypeREMOVE
			} else {
				break
			}
			if statusThisTime == "AMENDMENT" {
				order.Type = helper.OrderEventTypeNEW
			}
			filledPrice := fastfloat.ParseBestEffort(helper.BytesToString(data.GetStringBytes("ap")))
			filled := fixed.NewS(helper.BytesToString(data.GetStringBytes("z")))
			cid := string(data.GetStringBytes("c"))
			oid := fmt.Sprint(data.GetInt64("i"))
			order.Amount = fixed.NewS(helper.MustGetShadowStringFromBytes(data, "q"))
			order.Price = helper.GetFloat64FromBytes(data, "p")
			order.OrderID = oid
			order.ClientID = cid
			order.Filled = filled
			order.FilledPrice = filledPrice

			switch helper.MustGetShadowStringFromBytes(data, "S") {
			case "BUY":
				order.OrderSide = helper.OrderSideKD
			case "SELL":
				order.OrderSide = helper.OrderSideKK
			default:
				w.Logger.Errorf("side error: %s", helper.MustGetShadowStringFromBytes(data, "S"))
				return
			}
			if helper.MustGetBool(data, "R") {
				if order.OrderSide == helper.OrderSideKD {
					order.OrderSide = helper.OrderSidePK
				} else if order.OrderSide == helper.OrderSideKK {
					order.OrderSide = helper.OrderSidePD
				}
			}
			switch helper.MustGetShadowStringFromBytes(data, "ot") {
			case "LIMIT":
				order.OrderType = helper.OrderTypeLimit
			case "MARKET":
				order.OrderType = helper.OrderTypeMarket
			case "LIMIT_MAKER":
				order.OrderType = helper.OrderTypePostOnly
			}
			// 必须在type后面。忽略其他类型
			switch helper.MustGetShadowStringFromBytes(data, "f") {
			case "IOC":
				order.OrderType = helper.OrderTypeIoc
			case "GTX":
				order.OrderType = helper.OrderTypePostOnly
			}

			order.ReceivedTsNs = ts
			w.Cb.OnOrder(ts, order)
			// 默认都是 USDT费率
			fee := helper.MustGetFloat64FromBytes(data, "n")
			dealedValue := helper.MustGetFloat64FromBytes(data, "ap") * helper.MustGetFloat64FromBytes(data, "z")
			if dealedValue != 0 {
				w.Fee.Update(fee / dealedValue)
			}
			// }
		case "ACCOUNT_UPDATE":
			//"e": "ACCOUNT_UPDATE",                // 事件类型
			//"E": 1564745798939,                   // 事件时间
			//"T": 1564745798938 ,                  // 撮合时间
			// 更新仓位
			w.totalPosValueAndUnrealizedProfit.mutex.Lock()
			defer w.totalPosValueAndUnrealizedProfit.mutex.Unlock()
			P := value.Get("a").GetArray("P")
			timeInEx := helper.MustGetInt64(value, "T")
			for _, v := range P {
				symbol := helper.MustGetStringFromBytes(v, "s")
				ps := helper.MustGetShadowStringFromBytes(v, "ps")
				pa := fixed.NewS(helper.MustGetShadowStringFromBytes(v, "pa"))
				ep := helper.MustGetFloat64FromBytes(v, "ep")

				// 不是每次都推送全部symbol
				{
					val, ok := w.totalPosValueAndUnrealizedProfit.data[symbol]
					if !ok {
						val = &_PosValueAndUnrealizedProfit{}
						w.totalPosValueAndUnrealizedProfit.data[symbol] = val
					}
					// 存下其他仓位的价值和未实现盈亏，用于计算可用保证金。默认已经经过beforeTrade，初始仓位是0。
					lever := LEVER_RATE
					if l, ok := LeverMap[symbol]; ok {
						lever = l
					}
					val.posMargin = math.Abs(pa.Float()) * ep / float64(lever)
					val.unrealizedProfit = helper.MustGetFloat64FromBytes(v, "up")
				}
				if _, pos, ok := w.PosNewerAndStore(symbol, timeInEx, ts); ok {
					pos.Lock.Lock()
					pos.ResetLocked()
					if ps == "BOTH" {
						if pa.GreaterThan(fixed.ZERO) {
							pos.LongPos = pa
							pos.LongAvg = ep
						} else if pa.LessThan(fixed.ZERO) {
							pos.ShortPos = pa.Abs()
							pos.ShortAvg = ep
						}
					}
					event := pos.ToPositionEvent()

					pos.Lock.Unlock()
					w.Cb.OnPositionEvent(0, event)
				}
			}
			// 更新资金
			var totalUnrealizedProfit, totalPosMargin float64
			for _, posAndUp := range w.totalPosValueAndUnrealizedProfit.data {
				totalPosMargin += posAndUp.posMargin
				totalUnrealizedProfit += posAndUp.unrealizedProfit
			}
			B := value.Get("a").GetArray("B")
			for _, v := range B {
				a := helper.MustGetStringLowerFromBytes(v, "a")
				cash := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("wb")))
				//cashFree := fastfloat.ParseBestEffort(helper.BytesToString(v.GetStringBytes("cw"))) // 除去逐仓仓位保证金的钱包余额
				fieldsSet := (helper.EquityEventField_TotalWithUpl | helper.EquityEventField_TotalWithoutUpl | helper.EquityEventField_Avail | helper.EquityEventField_Upl)
				if e, ok := w.EquityNewerAndStore(a, timeInEx, ts, fieldsSet); ok {
					// 将未实现盈亏统计出来
					e.TotalWithUpl = cash + totalUnrealizedProfit
					e.TotalWithoutUpl = cash
					e.Upl = totalUnrealizedProfit
					e.Avail = cash + totalUnrealizedProfit - totalPosMargin
					w.Cb.OnEquityEvent(ts, *e)
				}
			}

		case "listenKeyExpired":
			// 这里要避免bn连续推送多次 导致重复创建priWs
			now := time.Now().Unix()
			w.reConnectLock.Lock()
			if now-w.reConnectTs > 10 { // 每10秒只允许进行一次reConn
				//
				w.Logger.Warnf("触发listenkey过期，产生新listenkey")
				// 停掉老的priWs
				if w.stopCPri != nil {
					helper.CloseSafe(w.stopCPri)
				}
				w.deleteListenKey()
				time.Sleep(time.Second * 10)
				// 重启priWs
				w.priWs = nil
				w.priWs = ws.NewWS(w.baseWsUrl, w.BrokerConfig.LocalAddr, w.BrokerConfig.ProxyURL, w.priHandler, w.Cb.OnExit, w.BrokerConfig.BrokerConfig)
				w.priWs.SetSubscribe(w.subPri)
				// 更新key
				w.getListenKey()
				var err error
				w.stopCPri, err = w.priWs.Serve()
				if err != nil {
					w.Cb.OnExit("binance usdt swap private ws 连接失败")
				}
				w.reConnectTs = now
			} else {
				w.Logger.Warnf("不允许短时间内重复尝试重连")
			}
			w.reConnectLock.Unlock()
		case "TRADE_LITE":
		default:
			w.Logger.Warnf("未知事件%s", helper.BytesToString(msg))
		}
	}
	// 判断是否需要续期
	now := time.Now().Unix()
	if now-w.getListenKeyTime.Load() > 900 { // 15min续期一次
		w.getListenKeyTime.Store(now)
		w.Logger.Infof("触发listenkey续期操作")
		go func() {
			w.rs.keepListenKey(w.listenKey.Load())
		}()
	}
}

// GetFee 获取费率
func (w *BinanceUsdtSwapWs) GetFee() (fee helper.Fee, err helper.ApiError) {
	return helper.Fee{Maker: w.Fee.Low, Taker: w.Fee.High}, helper.ApiErrorNil
}

func (w *BinanceUsdtSwapWs) WsLogged() bool {
	return w.listenKey.Load() != ""
}

func (w *BinanceUsdtSwapWs) GetPriWs() *ws.WS {
	return w.priWs
}
func (w *BinanceUsdtSwapWs) GetPubWs() *ws.WS {
	return w.pubWs
}
