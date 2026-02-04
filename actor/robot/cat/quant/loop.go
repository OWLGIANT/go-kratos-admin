package quant

import "actor/helper"

// loop 打印信息
func (q *Quant) loop() {
	q.StatusLock.Lock()
	status := q.Status
	q.StatusLock.Unlock()

	if status == helper.TaskStatusRunning || status == helper.TaskStatusReseting {
	} else if status == helper.TaskStatusStopped || status == helper.TaskStatusError {
		q.OnExit("服务器状态异常")
		return
	}

	if q.elog != nil {
		stats := q.getMarketStats()

		q.elog.Infof("挂单趋势|%s |最新币价:%.9f |\n长趋势%s |%.9f|%.9f|%.9f|%.9f |\n短趋势%s |%.9f|%.9f|%.9f|%.9f \n持仓均价: %.9f| 手续费:%.9f |浮盈浮亏: %.9f |总资产: %.9f",
			stats.Str,
			q.tradeTradeMsg.Ticker.Price(),
			GetTrad(stats.LTrend),
			stats.LSlope,
			stats.LUp,
			stats.LMid,
			stats.LDown,
			GetTrad(stats.STrend),
			stats.SSlope,
			stats.SUp,
			stats.SMid,
			stats.SDown,
			GetPositionAvg(q.tradeMsgLocal.Position),
			q.tradeMsgLocal.Fee,
			q.tradeMsgLocal.EquityEvent.Upl,
			q.tradeMsgLocal.EquityEvent.TotalWithUpl,
		)
	}
}
