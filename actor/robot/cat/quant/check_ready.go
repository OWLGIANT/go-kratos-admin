package quant

import (
	"actor/helper"
	"actor/third/log"
	"time"
)

// checkReady 启动60秒后检查是否已经开始交易 否则停机
func (q *Quant) checkReady() {
	n := 0
	var mp float64
	for {
		n += 1
		log.Infof("预热中")
		if n > 6 {
			break
		}
		time.Sleep(time.Second * 10)
		mp = q.tradeTradeMsg.Ticker.Mp.Load()
		if mp > 0 {
			break
		}
	}

	mp = q.tradeTradeMsg.Ticker.Mp.Load()
	if mp == 0 {
		q.OnExit("trade 长时间没有获取到行情")
	}

	q.StatusLock.Lock()
	if q.Status == helper.TaskStatusStopping ||
		q.Status == helper.TaskStatusStopped ||
		q.Status == helper.TaskStatusError ||
		q.Status == helper.TaskStatusReseting {
	} else {
		q.Status = helper.TaskStatusRunning
	}
	q.StatusLock.Unlock()
	log.Infof("初始化资金 交易盘口%.2f ", q.totalCash)
}
