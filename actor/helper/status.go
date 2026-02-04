package helper

const NORMAL_EXIT_MSG = "收到停机请求 正常停机"

type TaskStatus int

const (
	// [非最终状态] 未找到交易任务 正在运行中和历史缓存都没有
	TaskStatusNotFind TaskStatus = 0 + iota
	// [非最终状态] 交易任务正在初始化
	TaskStatusPrepare
	// [非最终状态] 交易任务正在运行
	TaskStatusRunning
	// [非最终状态] 交易任务正在停止过程中
	TaskStatusStopping
	// [非最终状态] 交易任务正在重置中
	TaskStatusReseting
	// [本状态为最终状态] 交易任务停止 正常停止 大概率无遗漏仓位 无需过多关注
	TaskStatusStopped
	// [本状态为最终状态] 交易任务停机 出现错误导致停止 大概率无遗漏仓位 需要关注异常原因
	TaskStatusError
	// [本状态为最终状态] 交易任务停机 且大概率存在遗漏仓位 必须高度关注立刻处理
	TaskStatusFatal
	// [本状态为最终状态] 交易任务停机 且大概率是oom 将来可能会存在遗漏仓位 必须高度关注立刻处理
	TaskStatusOOM
	// [1xx] 全栈组专用
	TaskStatusTransferring = 201 //套利策略使用状态 划转中 需要可以支持动态改惨
	// [2xx] 策略组可用于自定义 每个策略可以占用一段 比如200～210 A 策略占用 211～220 B策略占用
)

// 机器人特性
const (
	// ws订单操作
	TaskFacility_WsOrderOps int64 = 1 << iota
)
