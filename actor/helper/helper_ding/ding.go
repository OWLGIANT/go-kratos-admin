package helper_ding

import (
	"fmt"
	"time"

	"github.com/goex-top/dingding"
)

// 定义dingding报警信息
var (
	ding        *dingding.Ding
	DingToken   string
	DingKeyword string
)

const (
	defaultDingKeyword = "Beast"
)

// var defaultDingToken = "e89aa50cb030ab39fafa8fcdfbf88c14e4ab9af9917903209f112f54bb79b191"
var defaultDingToken string

func initDingding() {
	if DingToken == "" {
		DingToken = defaultDingToken
	}
	if DingKeyword == "" {
		DingKeyword = defaultDingKeyword
	}
	ding = dingding.NewDing(defaultDingToken)
}

var AlertCarryInfo string

func DingingSendWarning(msg string) {
	if defaultDingToken == "" {
		fmt.Sprintln("没设置 ding token，打印: " + msg)
		return
	}
	if ding == nil {
		initDingding()
	}
	nowUTC8 := time.Now().UTC().Add(time.Hour * 8).Format("021504") // 日时分
	ding.SendMarkdown(dingding.Markdown{
		Content:  fmt.Sprintf("%s\n%s <br>u8time:%s", msg, AlertCarryInfo, nowUTC8),
		Title:    fmt.Sprintf("报警\t%s", DingKeyword),
		AtPerson: nil,
		AtAll:    false,
	})
}

func SetDingTokens(normal, serios string) {
	defaultDingToken = normal
	defaultDingTokenSerious = serios
}
