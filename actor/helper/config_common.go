package helper

import (
	"sync/atomic"

	"actor/limit"
	bt "actor/tools"
)

var inited atomic.Bool

var InnerServerId int64

func init() {
	InitEnv()
}

func InitEnv() {
	if inited.Load() {
		return
	}
	ip := limit.GetMyIP()
	if len(ip) == 0 {
		panic("无法获取本机ip")
	}
	InnerServerId = bt.MustConvertIpToId(ip[0])
	inited.Store(true)
}
