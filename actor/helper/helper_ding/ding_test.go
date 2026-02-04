package helper_ding

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDingingSendWarning(t *testing.T) {
	DingingSendWarning("测试测试123123")
}

func TestDingdingSendSerious(t *testing.T) {
	DingingSendSerious(fmt.Sprintf("utf test 测试通知\n\n%s\n\n%s/%s",
		"22@1.1.1.1", "binance_usdt_swap", strings.ToLower("btc_usdt")))
	time.Sleep(time.Second)
}
