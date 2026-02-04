package schedule

import (
	"actor/third/log"

	"github.com/robfig/cron/v3"
)

// InitSchedule 初始化定时任务调度器
// 启动后会立即执行一次 IP池生成，然后每2小时执行一次
func InitSchedule() {
	c := cron.New()

	// 启动时立即执行一次
	go GenIpPoolTask()

	// 每2小时执行一次 IP池生成任务
	if _, err := c.AddFunc("@every 7200s", GenIpPoolTask); err != nil {
		log.Errorf("InitSchedule GenIpPoolTask err: %s", err.Error())
	}

	c.Start()
	log.Info("定时任务调度器已启动")
}
