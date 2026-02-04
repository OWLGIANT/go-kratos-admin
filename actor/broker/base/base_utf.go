//go:build utf
// +build utf

package base

const IsUtf bool = true

var SkipOptimistParseBBO bool // 是否跳过乐观解析BBO
var CollectTCData bool        // 是否收集tc数据
