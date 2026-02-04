//go:build darwin
// +build darwin

package helper

import (
	"time"
)

/*------------------------------------------------------------------------------------------------------------------*/

func GetFileAccessTime(path string) time.Time {
	panic("not support call GetFillAccessTime under darwin")
}
