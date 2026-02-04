package quant

import (
	"actor"
	"actor/broker/base"
	"actor/broker/ex/okx_usdt_swap"
	"actor/helper"
	"actor/robot/cat/config"
	"actor/third/log"
	"context"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"sync"
	"time"
)

// Quant 调度器定义
type Quant struct {
	/*
		--------------------------------------------------------------------
		系统变量
		--------------------------------------------------------------------
	*/
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       *config.Config
	buildTime string
	logger    log.Logger
	elog      *logrus.Entry
	/*
		--------------------------------------------------------------------
		 运行状态变量
		--------------------------------------------------------------------
	*/
	StatusLock     sync.Mutex          // 运行标识锁
	Status         helper.TaskStatus   // 运行标识
	Msg            string              // 状态信息
	GenSignalLock  sync.Mutex          // 触发交易信号锁
	GetPosErrorNum atomic.Uint32       // 获取仓位连续错误次数

	/*
		--------------------------------------------------------------------
		交易trador 对象
		--------------------------------------------------------------------
	*/
	tradeWs   base.Ws // 交易盘口ws实例
	tradeRest base.Rs // 交易盘口rs实例

	/*
		--------------------------------------------------------------------
		一般变量
		--------------------------------------------------------------------
	*/
	startTime    int64   // 启动时间
	printTime    int64   // 打印时间
	totalCash    float64 // 当前总资金
	startCash    float64 // 总资金
	maxTotalCash float64 // 最高总资金
	stoploss     float64 // 止损
	profitloss   float64 // 止盈

	lever             float64     // 杠杆倍数
	tradePair         helper.Pair // 交易对
	tradeExchangeName string      // 交易盘口名称
	tradeAcctName     string      // 账户名称

	tradeAcctMsg    helper.AcctMsg        // 账户信息
	tradePairInfo   helper.ExchangeInfo   // 交易盘口交易信息
	infos           []helper.ExchangeInfo // 缓存所有交易信息
	ExchangeInfoS2P map[string]helper.ExchangeInfo
	tradeUpdateTime atomic.Int64 // 交易盘口行情更新时间
	isTradeSpot     bool         // 交易盘口是否为现货

	// onReset OnExit 相关
	resetLock  sync.Mutex // 重置锁
	resetTime  int64      // 上一次重置时间
	resetNum   int64      // 重置次数
	onExitOnce sync.Once  // 停机仅允许执行一次

	// 交易参数
	maxHoldValue float64        // 最大可持仓
	targitValue  atomic.Float64 // 目标仓位金额
	signalTime   atomic.Int64   // 最近一次触发信号的时间

	// 交易信息
	tradeTradeMsg helper.TradeMsg
	tradeMsgLocal *TradeMsgLocal

	// 行情统计信息
	market *Market
}

type MarketCache struct {
	LongTrends  uint32
	ShortTrends uint32
	WinCount    uint32
	LoseCount   uint32
	LoseStat    time.Duration
	Start       *time.Time
}

func (q *Quant) ResetCash() {
	if q.startCash == 0 {
		q.startCash = q.totalCash
	}
	if q.totalCash > q.maxTotalCash {
		q.maxTotalCash = q.totalCash
	}
}

func (q *Quant) SetAccountMode() (err error) {
	lever := q.cfg.StrategyParams.Lever
	q.lever = lever
	q.tradeRest.SetAccountMode(q.tradePair.String(), int(q.cfg.StrategyParams.Lever), helper.MarginMode_Cross, helper.PosModeOneway)
	log.Infof("交易盘口 %v 初始资金为 Cash: %.4f Coin: %.4f", q.tradeExchangeName, q.totalCash, q.tradeAcctMsg.Equity.Coin)
	return
}

func (q *Quant) ResetCandles() (klines []*helper.Kline) {
	q.market = &Market{
		stats:  &MarketStats{},
		klines: make([]*helper.Kline, 300),
	}
	trade, ok := q.tradeRest.(*okx_usdt_swap.OkxUsdtSwapRs)
	if ok {
		var err helper.ApiError
		klines, err = trade.GetCandles("1m")
		if err.NotNil() {
			q.OnExit(err.String())
		}
	} else {
		q.OnExit("ResetCandles 断言失败")
	}
	reverseSlice(klines)
	q.setKlines(klines)
	return
}

func (q *Quant) ClearPositions() {
	q.logger.Info("清仓 ClearPositions")
	pair, _ := helper.StringPairToPair(q.cfg.StrategyParams.Pair)
	rest, _ := actor.GetClient(
		actor.ClientTypeRs,
		q.tradeExchangeName,
		helper.BrokerConfig{
			Name:          q.tradeAcctName,
			AccessKey:     q.cfg.AccessKey,
			SecretKey:     q.cfg.SecretKey,
			PassKey:       q.cfg.PassKey,
			Pairs:         []helper.Pair{pair},
			ProxyURL:      q.cfg.Proxy,
			SymbolAll:     true,
			NeedAuth:      true,
			LocalAddr:     q.cfg.TradeIP,
			Logger:        q.logger,
			RootLogger:    log.RootLogger,
			RobotId:       q.cfg.TaskUid,
			OwnerDeerKeys: []string{},
		},
		&q.tradeTradeMsg,
		helper.CallbackFunc{
			OnTicker:        func(ts int64) {},
			OnDepth:         func(ts int64) {},
			OnPositionEvent: func(ts int64, event helper.PositionEvent) {},
			OnEquityEvent:   func(ts int64, event helper.EquityEvent) {},
			OnOrder:         func(ts int64, event helper.OrderEvent) {},
			OnExit:          func(msg string) {},
			OnReset:         func(msg string) {},
		},
	)
	rest.AfterTrade(helper.HandleModeCloseAll)
}

func reverseSlice[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
