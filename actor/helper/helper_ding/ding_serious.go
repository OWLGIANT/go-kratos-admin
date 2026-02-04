package helper_ding

import (
	"fmt"
	"time"

	"github.com/goex-top/dingding"
)

// 定义dingding报警信息
var (
	dingSerious *dingding.Ding
)

const (
	dingKeywordSerious = "Quant"
)

// var defaultDingTokenSerious = "4b7b648722797366b7e440c680e628a925bf8ad94ed8ce2dbc62f8ec81fdf108"
var defaultDingTokenSerious string

func initDingding2() {
	dingSerious = dingding.NewDing(defaultDingTokenSerious)
}

func DingingSendSerious(msg string) {
	if defaultDingTokenSerious == "" {
		fmt.Sprintln("没设置 ding serious token，打印: " + msg)
		return
	}
	if dingSerious == nil {
		initDingding2()
	}
	go func() {
		nowUTC8 := time.Now().UTC().Add(time.Hour * 8).Format("021504") // 日时分
		dingSerious.SendMarkdown(dingding.Markdown{
			Content:  fmt.Sprintf("%s\n%s <br>u8time:%s", msg, AlertCarryInfo, nowUTC8),
			Title:    fmt.Sprintf("报警\t%s", dingKeywordSerious),
			AtPerson: nil,
			AtAll:    false,
		})
	}()
}
