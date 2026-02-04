package quant

import (
	"actor/helper"
	"actor/third/fixed"
	"actor/third/log"
	"math"
	"math/rand"
	"sync/atomic"
	"time"
)

type TradeMsgLocal struct {
	Position    *helper.PositionEvent
	EquityEvent *helper.EquityEvent

	MarketCache *MarketCache
	PosHoldTime *PosHoldIng

	TakerFee float64
	Fee      float64

	StopTrade bool

	LastOpenPos *HoldPosStatus
}

func NewTradeMsgLocal() *TradeMsgLocal {
	return &TradeMsgLocal{
		EquityEvent: &helper.EquityEvent{},
		MarketCache: &MarketCache{},
		LastOpenPos: &HoldPosStatus{
			OpenAt: time.Now().Add(-time.Hour),
			Win:    true,
		},
	}
}

type HoldPosStatus struct {
	OpenAt time.Time
	Win    bool
}

func GetPositionAvg(pos *helper.PositionEvent) (avg float64) {
	if pos == nil {
		return
	}
	if !pos.ShortPos.IsZero() {
		return pos.ShortAvg
	}
	if !pos.LongPos.IsZero() {
		return pos.LongAvg
	}
	return
}

func GetPositionPos(pos *helper.PositionEvent) (posMount float64) {
	if pos == nil {
		return
	}
	if !pos.ShortPos.IsZero() {
		return pos.ShortPos.Float()
	}
	if !pos.LongPos.IsZero() {
		return pos.LongPos.Float()
	}
	return
}

func IfPositionLong(pos *helper.PositionEvent) bool {
	if pos == nil {
		return false
	}
	return !pos.LongPos.IsZero()
}

type PosHoldIng struct {
	Start  time.Time
	MaxPnl float64
}

func (q *Quant) Trader() {
	_fee, _ := q.tradeRest.GetFee()
	q.tradeMsgLocal.TakerFee = _fee.Taker
	if q.tradeMsgLocal.TakerFee == 0 {
		q.OnExit("TakerFee 获取失败")
	}

	time.Sleep(time.Minute * 5)
	q.logger.Infof("策略下单开始启动")

	for {
		time.Sleep(time.Second)

		for q.tradeMsgLocal.Position != nil {
			if q.tradeMsgLocal.PosHoldTime == nil {
				q.tradeMsgLocal.Fee = calcFee(
					q.tradeMsgLocal.TakerFee,
					GetPositionPos(q.tradeMsgLocal.Position)*GetPositionAvg(q.tradeMsgLocal.Position)) * 2
				q.tradeMsgLocal.PosHoldTime = &PosHoldIng{
					Start:  time.Now(),
					MaxPnl: q.tradeMsgLocal.Fee,
				}
			}

			upl := q.tradeMsgLocal.EquityEvent.Upl
			q.totalCash = q.tradeMsgLocal.EquityEvent.TotalWithUpl

			if q.totalCash > q.maxTotalCash {
				q.maxTotalCash = q.totalCash
			}

			if upl > q.tradeMsgLocal.PosHoldTime.MaxPnl {
				q.tradeMsgLocal.PosHoldTime.MaxPnl = upl
			}

			var takeProfit = calculateTakeProfit(q.tradeMsgLocal.PosHoldTime)
			var closeAllPosition bool

			if upl < 0 {
				closeLine := q.tradeMsgLocal.Fee * q.cfg.StrategyParams.StopLoss
				if math.Abs(upl) > math.Abs(closeLine) {
					q.elog.Infof("单笔亏损:%.9f超过移动止损 止损%.9f 主动平仓", upl, closeLine)
					closeAllPosition = true
				}
			} else {
				var winless = q.tradeMsgLocal.Fee * q.cfg.StrategyParams.WinLess
				if upl > math.Abs(winless) && upl < takeProfit {
					closeAllPosition = true
					q.elog.Infof("单笔盈利:%.9f 手续费:%.9f winless:%.9f 移动盈利%.9f 主动平仓", upl, q.tradeMsgLocal.Fee*1.2, winless, takeProfit)
				}
			}

			if closeAllPosition {
				if upl > 0 {
					q.tradeMsgLocal.LastOpenPos.Win = true
					q.tradeMsgLocal.MarketCache.WinCount++
					q.tradeMsgLocal.MarketCache.LoseStat = 0
					q.tradeMsgLocal.MarketCache.Start = nil
				} else {
					q.tradeMsgLocal.LastOpenPos.Win = false
					q.tradeMsgLocal.MarketCache.LoseCount++
					q.tradeMsgLocal.MarketCache.LoseStat++

					randDuration := time.Duration(rand.Intn(30))
					nextTime := time.Now().Add(q.tradeMsgLocal.MarketCache.LoseStat * time.Minute * randDuration)
					q.tradeMsgLocal.MarketCache.Start = &nextTime
				}

				q.ClearPositions()

				if q.tradeMsgLocal.StopTrade {
					q.OnExit("止损停机")
					return
				}
			}

			time.Sleep(time.Second)
		}

		time.Sleep(time.Second)

		for q.tradeMsgLocal.Position == nil {
			q.TradeDelay()
			stats := q.getMarketStats()

			switch stats.Str {
			case TFSUP:
				switch stats.STrend {
				case 3:
					if stats.IsYangNow && atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.LongTrends, 1, 0) {
						q.plaseOrder(helper.OrderSideKD)
					}
				case 5:
					atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.LongTrends, 0, 1)
				}
			case TFSDown:
				switch stats.STrend {
				case 2:
					if !stats.IsYangNow && atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.ShortTrends, 1, 0) {
						q.plaseOrder(helper.OrderSideKK)
					}
				case 4:
					atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.ShortTrends, 0, 1)
				}
			case GTS:
				switch stats.MTrend {
				case 4:
					switch stats.STrend {
					case 2:
						if !stats.IsYangNow && atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.ShortTrends, 1, 0) {
							q.plaseOrder(helper.OrderSideKK)
						}
					case 4:
						atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.ShortTrends, 0, 1)
					}
				case 5:
					switch stats.STrend {
					case 3:
						if stats.IsYangNow && atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.LongTrends, 1, 0) {
							q.plaseOrder(helper.OrderSideKD)
						}
					case 5:
						atomic.CompareAndSwapUint32(&q.tradeMsgLocal.MarketCache.LongTrends, 0, 1)
					}
				}
			}
			time.Sleep(time.Second)
		}
	}
}

func (q *Quant) TradeDelay() {
	if q.tradeMsgLocal.LastOpenPos.Win {
		return
	}
	if q.tradeMsgLocal.MarketCache.Start == nil {
		return
	}
	tradeDelay := q.tradeMsgLocal.MarketCache.Start.Sub(time.Now())
	log.Infof("胜: %v 负: %v 连续负次数 %v 时间: %vMin 开始",
		q.tradeMsgLocal.MarketCache.WinCount,
		q.tradeMsgLocal.MarketCache.LoseCount,
		q.tradeMsgLocal.MarketCache.LoseStat,
		tradeDelay.Minutes())
	time.Sleep(tradeDelay)
	q.tradeMsgLocal.MarketCache.Start = nil
	log.Info("休眠结束")
}

func (q *Quant) plaseOrder(side helper.OrderSide) {
	signal := helper.Signal{
		Time:              time.Now().UnixNano(),
		Type:              helper.SignalTypeNewOrder,
		OrderType:         helper.OrderTypeMarket,
		Pair:              q.tradePair,
		Price:             q.tradeTradeMsg.Ticker.Price(),
		Amount:            fixed.NewF(q.calcOrderAmount(q.tradeMsgLocal.EquityEvent.Avail, q.cfg.StrategyParams.Lever)),
		OrderSide:         side,
		SignalChannelType: helper.SignalChannelTypeRs,
	}
	q.tradeMsgLocal.LastOpenPos.OpenAt = time.Now()
	q.tradeRest.SendSignal([]helper.Signal{signal})
	stats := q.getMarketStats()
	q.elog.Infof("下单方向:%s 挂单趋势:%s 下单价格:%v 短期趋势:%s|%.9f|%.9f|%.9f|%.9f",
		side.String(),
		stats.Str,
		q.tradeTradeMsg.Ticker.Price(),
		GetTrad(stats.STrend),
		stats.SSlope,
		stats.SUp,
		stats.SMid,
		stats.SDown,
	)
}

func (q *Quant) calcOrderAmount(totalCash float64, lever float64) float64 {
	if lever >= 2 {
		lever--
	}

	tradeValue := totalCash * lever
	amount := tradeValue / q.tradeTradeMsg.Ticker.Price()

	minAmountByValue := q.tradePairInfo.MinOrderValue.Float() / q.tradeTradeMsg.Ticker.Price()
	finalMinAmount := math.Max(q.tradePairInfo.MinOrderAmount.Float(), minAmountByValue)

	maxAmountByValue := q.tradePairInfo.MaxOrderValue.Float() / q.tradeTradeMsg.Ticker.Price()
	finalMaxAmount := math.Min(q.tradePairInfo.MaxOrderAmount.Float(), maxAmountByValue)

	if amount < finalMinAmount {
		amount = finalMinAmount
	}
	if amount > finalMaxAmount {
		amount = finalMaxAmount
	}

	if q.tradePairInfo.StepSize > 0 {
		amount = math.Floor(amount/q.tradePairInfo.StepSize) * q.tradePairInfo.StepSize
	}

	return amount
}

func calculateTakeProfit(posHoldIng *PosHoldIng) float64 {
	holdDuration := time.Since(posHoldIng.Start)
	var baseRatio float64

	switch {
	case holdDuration < 5*time.Minute:
		baseRatio = 0.60
	case holdDuration < 8*time.Minute:
		baseRatio = 0.65
	case holdDuration < 10*time.Minute:
		baseRatio = 0.70
	case holdDuration < 15*time.Minute:
		baseRatio = 0.75
	case holdDuration < 35*time.Minute:
		baseRatio = 0.70
	case holdDuration < 40*time.Minute:
		baseRatio = 0.75
	case holdDuration < 45*time.Minute:
		baseRatio = 0.78
	case holdDuration < 50*time.Minute:
		baseRatio = 0.80
	case holdDuration < 55*time.Minute:
		baseRatio = 0.85
	case holdDuration < time.Hour:
		baseRatio = 0.80
	default:
		baseRatio = 0.88
	}
	return baseRatio * posHoldIng.MaxPnl
}

func calcFee(feeRate float64, tradeValue float64) float64 {
	return feeRate * tradeValue
}
