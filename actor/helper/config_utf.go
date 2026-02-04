//go:build utf
// +build utf

package helper

/* ------------------------------------------------------------------------------------------------------------------ */

// 用于判断是否记录debug信息 utf时设置打开
const DEBUGMODE = true

var NEED_RESUB_IN_UTF = false                 // 是否重订阅, 测试模式才可用
var RESUB_CHAN_4_UTF = make(chan struct{}, 1) // 重订阅信号, 测试模式才可用
const DEBUG_PRINT_HEADER = true

var DEBUG_PRINT_MARKETDATA = true

var AppName string
var BuildTime string
var HttpProxy string
var GitCommitHash string
