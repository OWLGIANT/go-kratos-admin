//go:build !utf_log
// +build !utf_log

package log

import (
	"fmt"
	"runtime"
	"strings"
)

// 仅供utf模式下使用，放这里是编译通过
var PanicIfErrorUnderUTF = false
var ErrmsgOutputFile = ""

func getCallerInfo(skip int) (funcName, file string, lineNo int) {

	// pc, file, lineNo
	pc, file, lineNo, ok := runtime.Caller(skip)
	if !ok {
		panic("can't get caller info")
	}
	vals := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	funcName = vals[len(vals)-1]
	fs := strings.Split(file, "/")
	file = fs[len(fs)-2] + "/" + fs[len(fs)-1]
	return funcName, file, lineNo
	// return fmt.Sprintf("FuncName:%s, line:%d", funcName, lineNo)
	// fileName := path.Base(file) // Base函数返回路径的最后一个元素
	// return fmt.Sprintf("FuncName:%s, file:%s, line:%d ", funcName, fileName, lineNo)
}

func Error(args ...interface{}) {
	if RootLogger == nil {
		return
	}

	args2 := make([]any, 0, len(args)+1)
	args2 = append(args2, args...)
	args2 = append(args2, fmt.Sprintf("[%s]", GitCommitHash))
	RootLogger.Error(args2...)
	// fn, f, ln := getCallerInfo(2)
	// go func() {
	// helper_ding.DingingSendWarning(fmt.Sprintf("%v\n[%s:%s:%d,%s]\n", args, f, fn, ln, GitCommitHash))
	// }()
}

func Errorf(template string, args ...interface{}) {
	if RootLogger == nil {
		return
	}
	// RootLogger.Errorf(template+fmt.Sprintf("[%s]", GitCommitHash), args...)
	// RootLogger.Errorf("\033[31m "+template+" \033[0m"+fmt.Sprintf("[%s]", GitCommitHash), args...)
	RootLogger.Errorf(template, args...)

	// fn, f, ln := getCallerInfo(2)
	// args2 := make([]any, 0, len(args)+4)
	// args2 = append(args2, args...)
	// args2 = append(args2, f, fn, ln, GitCommitHash)
	// go func() {
	// helper_ding.DingingSendWarning(fmt.Sprintf(template+"\n[%s:%s:%d,%s]\n", args2...))
	// }()
}
