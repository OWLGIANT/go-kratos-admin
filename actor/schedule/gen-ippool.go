package schedule

import (
	"actor/helper"
	"actor/third/log"
)

// GenIpPoolTask 定时生成IP池任务
func GenIpPoolTask() {
	log.Info("开始执行 IP池生成任务")
	if err := helper.GenIpPool(); err != nil {
		log.Errorf("IP池生成任务失败: %s", err.Error())
	} else {
		log.Info("IP池生成任务完成")
	}
}
