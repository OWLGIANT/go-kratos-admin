//go:build !utf && !debug
// +build !utf,!debug

package helper

/* ------------------------------------------------------------------------------------------------------------------ */

// 用于判断是否记录debug信息 关闭可以提高性能 开启会在编译阶段绕过写日志分支
const DEBUGMODE = false

var RESUB_CHAN_4_UTF = make(chan struct{}, 1) // 重订阅信号, 测试模式才可用

const DEBUG_PRINT_HEADER = false

var DEBUG_PRINT_MARKETDATA = false

var AppName string
var BuildTime string
var HttpProxy string
var GitCommitHash string

/* ------------------------------------------------------------------------------------------------------------------ */
