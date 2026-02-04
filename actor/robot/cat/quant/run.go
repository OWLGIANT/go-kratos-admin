package quant

import (
	"actor/helper"
	"actor/third/log"
	"time"
)

// Run 一切准备就绪后 开始交易
func (q *Quant) Run() {
	go q.Trader()

	q.startTime = time.Now().UnixMilli()

	go q.checkReady()

	tick := time.NewTicker(time.Second * 8)
	defer tick.Stop()
	update := time.NewTicker(48 * time.Hour)

	for {
		select {
		case <-q.ctx.Done():
			q.OnExit(helper.NORMAL_EXIT_MSG)
			for i := 0; i < 30; i++ {
				time.Sleep(time.Second * 5)
				q.StatusLock.Lock()
				if q.Status == helper.TaskStatusStopped || q.Status == helper.TaskStatusError {
					q.StatusLock.Unlock()
					return
				}
				q.StatusLock.Unlock()
			}
		case <-tick.C:
			q.loop()
		case <-update.C:
			q.infos = q.tradeRest.GetExchangeInfos()
			q.ExchangeInfoS2P = make(map[string]helper.ExchangeInfo)
			for _, info := range q.infos {
				q.ExchangeInfoS2P[info.Symbol] = info
				log.Infof("symbol:%s", info.Symbol)
			}
		}
	}
}
