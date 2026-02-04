package quant

import (
	"actor/helper"
)

func (q *Quant) onTicker(ts int64) {
}

func (q *Quant) onDepth(ts int64) {
	q.logger.Infof("onDepth ts:%v", ts)
}

func (q *Quant) onTrade(ts int64) {
	q.logger.Infof("onTrade ts:%v", ts)
}

func (q *Quant) onEquity(ts int64, event helper.EquityEvent) {
	q.tradeMsgLocal.EquityEvent = &event
}

func (q *Quant) onOrder(ts int64, event helper.OrderEvent) {
}

func (q *Quant) onPosition(ts int64, event helper.PositionEvent) {
	q.tradeMsgLocal.Position = &event
	if q.tradeMsgLocal.Position.ShortPos.IsZero() && q.tradeMsgLocal.Position.LongPos.IsZero() {
		q.logger.Infof("仓位清除 onPosition ts:%v event:%s", ts, event.String())
		q.tradeMsgLocal.Position = nil
		q.tradeMsgLocal.PosHoldTime = nil
	}
}

func (q *Quant) OnKline(ts int64, kline helper.Kline) {
	orignKlines := q.getKlines()
	if len(orignKlines) > 0 {
		lastKline := orignKlines[len(orignKlines)-1]
		if lastKline.OpenTimeMs == kline.OpenTimeMs {
			orignKlines[len(orignKlines)-1] = &kline
		} else {
			orignKlines = append(orignKlines, &kline)
			orignKlines = orignKlines[1:]
		}
	}
	q.setKlines(orignKlines)
}
