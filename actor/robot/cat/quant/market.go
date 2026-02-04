package quant

import (
	"actor/helper"
	"math"
	"sort"
	"sync"
)

type Market struct {
	lock   sync.RWMutex
	stats  *MarketStats
	klines []*helper.Kline
}

type MarketStats struct {
	High float64
	Low  float64
	Last float64

	Str       Str
	IsYangNow bool

	SUp    float64
	SMid   float64
	SDown  float64
	STrend uint
	SSlope float64
	SCUp   int64
	SCDown int64

	MUp    float64
	MMid   float64
	MDown  float64
	MTrend uint
	MSlope float64
	MCUp   int64
	MCDown int64

	LUp    float64
	LMid   float64
	LDown  float64
	LTrend uint
	LSlope float64
	LCUp   int64
	LCDown int64
}

func (q *Quant) getMarketStats() (status *MarketStats) {
	q.market.lock.RLock()
	defer q.market.lock.RUnlock()
	return q.market.stats
}

func (q *Quant) getKlines() (klines []*helper.Kline) {
	q.market.lock.RLock()
	defer q.market.lock.RUnlock()
	return q.market.klines
}

func (q *Quant) setKlines(klines []*helper.Kline) {
	stats := q.reloadStats(klines)
	q.market.lock.Lock()
	defer q.market.lock.Unlock()
	q.market.stats = stats
	q.market.klines = klines
}

func (q *Quant) reloadStats(klines []*helper.Kline) *MarketStats {
	stats := &MarketStats{}

	for k, item := range klines {
		if k == len(klines)-1 {
			stats.Last = item.Close
			stats.IsYangNow = item.Close > item.Open
		}

		if k == 0 {
			stats.High = item.High
			stats.Low = item.Low
		} else {
			if item.High > stats.High {
				stats.High = item.High
			}
			if item.Low < stats.Low {
				stats.Low = item.Low
			}
		}
	}

	// 短期趋势
	{
		var priceSLine []float64
		for _, item := range klines[len(klines)-q.cfg.StrategyParams.Windows:] {
			priceSLine = append(priceSLine, item.High, item.Low, item.Open, item.Close)
		}

		stats.SSlope = linearSlope(priceSLine)
		sort.Float64s(priceSLine)
		stats.SUp = valueByRatio(priceSLine, 0.85)
		stats.SMid = percentile(priceSLine, 0.5)
		stats.SDown = valueByRatio(priceSLine, 0.15)

		switch {
		case stats.Last == stats.SMid:
			stats.STrend = 1
		case stats.Last > stats.SUp:
			stats.STrend = 4
		case stats.Last > stats.SMid && stats.Last < stats.SUp:
			stats.STrend = 2
		case stats.Last < stats.SDown:
			stats.STrend = 5
		case stats.Last < stats.SMid && stats.Last > stats.SDown:
			stats.STrend = 3
		}
	}

	// 中期趋势
	{
		var priceMLine []float64
		for _, item := range klines[150:] {
			priceMLine = append(priceMLine, item.High, item.Low, item.Open, item.Close)
		}
		stats.MSlope = linearSlope(priceMLine)
		sort.Float64s(priceMLine)
		stats.MUp = valueByRatio(priceMLine, 0.80)
		stats.MMid = percentile(priceMLine, 0.5)
		stats.MDown = valueByRatio(priceMLine, 0.20)

		switch {
		case stats.Last == stats.MMid:
			stats.MTrend = 1
		case stats.Last > stats.MUp:
			stats.MTrend = 4
		case stats.Last > stats.MMid && stats.Last < stats.MUp:
			stats.MTrend = 2
		case stats.Last < stats.MDown:
			stats.MTrend = 5
		case stats.Last < stats.MMid && stats.Last > stats.MDown:
			stats.MTrend = 3
		}
	}

	// 长期趋势
	{
		var priceLLine []float64
		for _, item := range klines {
			priceLLine = append(priceLLine, item.High, item.Low, item.Open, item.Close)
		}
		stats.LSlope = linearSlope(priceLLine)
		sort.Float64s(priceLLine)
		stats.LUp = valueByRatio(priceLLine, 0.75)
		stats.LMid = percentile(priceLLine, 0.5)
		stats.LDown = valueByRatio(priceLLine, 0.25)

		switch {
		case stats.Last == stats.LMid:
			stats.LTrend = 1
		case stats.Last > stats.LUp:
			stats.LTrend = 4
		case stats.Last > stats.LMid && stats.Last < stats.LUp:
			stats.LTrend = 2
		case stats.Last < stats.LDown:
			stats.LTrend = 5
		case stats.Last < stats.LMid && stats.Last > stats.LDown:
			stats.LTrend = 3
		}
	}

	stats.Str = stats.ChooseStrategy()
	return stats
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	pos := p * float64(len(sorted)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if upper == lower {
		return sorted[lower]
	}
	weight := pos - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func valueByRatio(arr []float64, p float64) float64 {
	if len(arr) == 0 {
		return math.NaN()
	}

	minV, maxV := arr[0], arr[0]
	for _, v := range arr[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	if maxV <= minV {
		return minV
	}

	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	return minV + (maxV-minV)*p
}

func linearSlope(prices []float64) float64 {
	n := float64(len(prices))
	if n < 2 {
		return 0
	}
	firstPrice := prices[0]
	lastPrice := prices[len(prices)-1]
	if firstPrice == lastPrice {
		return 0
	}
	result := (lastPrice - firstPrice) / firstPrice
	return math.Round(result*100000) / 100000
}

type Str string

const (
	TFSDown Str = "TrendDown"
	TFSUP   Str = "TrendUp"
	GTS     Str = "Grid"
)

func (stats *MarketStats) ChooseStrategy() Str {
	if (stats.LDown >= stats.MDown && stats.MDown >= stats.SDown) && (stats.LMid >= stats.MMid && stats.MMid >= stats.SMid) {
		return TFSDown
	}

	if (stats.LUp <= stats.MUp && stats.MUp <= stats.SUp) && (stats.LMid <= stats.MMid && stats.MMid <= stats.SMid) {
		return TFSUP
	}

	return GTS
}

func GetTrad(trend uint) string {
	switch trend {
	case 1:
		return "中轴线"
	case 2:
		return "缓慢上升偏离"
	case 3:
		return "缓慢下降偏离"
	case 4:
		return "强势上升偏离"
	case 5:
		return "强势下降偏离"
	default:
		return "未知趋势"
	}
}
