//go:build linux
// +build linux

package helper

import (
	"os"
	"syscall"
	"time"

	"actor/third/log"
)

/*------------------------------------------------------------------------------------------------------------------*/

// syscall.Stat在mac下不支持，这里拆分开

// 获取文件访问时间 返回unix时间戳
func GetFileAccessTime(path string) time.Time {
	// 获取文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Error("获取文件信息出错:", err)
		return time.Time{}
	}

	// 获取底层的文件信息
	sys := fileInfo.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		// 将 st_atime 字段转换为时间
		atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
		return atime
	} else {
		log.Error("无法获取文件最后访问时间")
	}
	return time.Time{}
}
