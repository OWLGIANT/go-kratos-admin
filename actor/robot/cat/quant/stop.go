package quant

import (
	"actor/helper"
	"actor/third/log"
	"strings"
	"time"
)

// OnExit 停机退出相关逻辑
func (q *Quant) OnExit(msg string) {
	q.onExitOnce.Do(func() {
		q.StatusLock.Lock()
		q.Status = helper.TaskStatusStopping
		q.Msg = msg
		q.StatusLock.Unlock()

		q.tradeRest.AfterTrade(helper.HandleModeCloseAll)

		if q.tradeWs != nil {
			q.tradeWs.Stop()
		}

		q.StatusLock.Lock()
		if strings.Contains(msg, helper.NORMAL_EXIT_MSG) {
			q.Status = helper.TaskStatusStopped
		} else {
			q.Status = helper.TaskStatusError
		}
		q.Msg = msg
		q.StatusLock.Unlock()

		if q.cancel != nil {
			q.cancel()
		}
	})
}

// onReset 重置交易相关逻辑
func (q *Quant) onReset(msg string) {
	go func() {
		q.resetLock.Lock()
		defer q.resetLock.Unlock()

		now := time.Now().UnixMilli()
		if now-q.resetTime > 1000*60*1 {
			q.StatusLock.Lock()
			q.Status = helper.TaskStatusReseting
			q.Msg = msg
			q.StatusLock.Unlock()

			log.Warnf("收到重置信号 原因: %s", msg)
			q.resetTime = now + 1000
			q.resetNum += 1

			if q.resetNum > 10 {
				q.OnExit("重置次数太多 需要停机和人工检查")
			} else {
				go func() {
					for i := 0; i < 10; i++ {
						time.Sleep(time.Second * 15)
						go q.tradeRest.AfterTrade(helper.HandleModeCloseOne)
					}
					time.Sleep(time.Second * 15)
					go q.tradeRest.BeforeTrade(helper.HandleModeCloseOne)
					time.Sleep(time.Second * 15)

					if q.Status == helper.TaskStatusReseting {
						q.StatusLock.Lock()
						q.Status = helper.TaskStatusRunning
						q.Msg = "上次重置原因:" + msg
						q.StatusLock.Unlock()
					}
				}()
			}
		}
	}()
}
