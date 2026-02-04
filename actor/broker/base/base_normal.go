//go:build !utf
// +build !utf

package base

const IsUtf bool = false

var SkipOptimistParseBBO bool = false // 是否跳过乐观解析BBO
var CollectTCData bool = false        // 是否收集tc数据
