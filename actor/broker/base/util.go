package base

import (
	"fmt"
	"runtime"
	"strings"

	"actor/helper"
	"actor/third/fixed"
	"github.com/duke-git/lancet/v2/slice"
)

func GetExPackageName(depth int) string {
	pc := make([]uintptr, 10)  // 设置足够大的栈帧数
	runtime.Callers(depth, pc) // 跳过当前函数和调用该函数的函数

	// 获取调用者的信息
	f := runtime.FuncForPC(pc[0])
	file, _ := f.FileLine(pc[0])
	// fmt.Println("file ", file)
	ff := strings.Split(file, "/")
	slice.Reverse[string](ff)
	fmt.Println("ff ", ff)
	var pkg string
	for _, s := range ff {
		if strings.Contains(s, "swap") || strings.Contains(s, "spot") {
			pkg = s
			break
		}
	}
	if pkg == "" {
		panic(fmt.Sprintf("failed to get ex pkg. caller file %s", file))
	}

	return pkg
}

// 根据调用ex生成文件名，获取路径上的spot/swap字段
func GenExchangeInfoFileName(pkg string) string {
	if pkg == "" {
		pkg = GetExPackageName(3)
	}
	dec := "dec_low"
	if fixed.IsHighDecimal() {
		dec = "dec_high"
	}
	return fmt.Sprintf("exchangeInfo@%s.%s.%s.json", pkg, helper.KeepOnlyNumbers(BuildTime), dec)
}
