package helper

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// func TestAlert_Debug(t *testing.T) {
// alert := Alert{
// Channel:       ChannelHFT,
// DingtalkToken: "",
// RobotID:       "d836613e-02cd-412c-b15d-315723676c42",
// }
// alert.Info("info测试")
// alert.Error("error测试")
// alert.Warn("warn test")
// alert.Fatal("fatal test")
// alert.Debug("debug test")
// time.Sleep(5 * time.Second)
// }

func TestAlertDeer_Debug(t *testing.T) {
	go func() {
		for {
			// alert := Alert{
			// 	Channel:       ChannelHFT,
			// 	DingtalkToken: "",
			// 	RobotID:       "d836613e-02cd-412c-b15d-315723676c42",
			// }
			// alert.Info("info测试")
			// alert.Error("error测试")
			// alert.Warn("warn test")
			// alert.Fatal("fatal test")
			// alert.Debug("debug test")
			// PushAlert("测试", "测试")
			time.Sleep(100 * time.Millisecond)
			fmt.Println(123)
		}
	}()
	go func() {
		for {
			threshold := 10
			gNum := runtime.NumGoroutine()
			if gNum > threshold {
				buf := make([]byte, 1<<20) // 1 MB buffer
				stacklen := runtime.Stack(buf, true)
				aa := string(buf[:stacklen])
				// helper.PushAlert("运行线程数量异常", fmt.Sprintf("运行线程数量异常 %v", gNum))
				fmt.Println(fmt.Sprintf("[线程数量]%v", gNum))
				fmt.Println(fmt.Sprintf("[线程堆栈]%v", aa))
			}
			time.Sleep(2 * time.Second)
		}
	}()

	for {
	}
}
