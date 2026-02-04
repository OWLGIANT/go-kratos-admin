//go:build utf_log
// +build utf_log

package log

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func (w WrapLogger) Error(args ...interface{}) {
	if w.Logger == nil {
		return
	}

	w.Logger.Error(args...)
	w.Logger.Sync()
	// RootLogger.FlushAndClose() // 避免ws msg handler中recover panic时还需要打印
	errmsg := fmt.Sprintf("error utf: %v", args...)
	fmt.Printf(errmsg)
	if ErrmsgOutputFile != "" {
		err := ioutil.WriteFile(ErrmsgOutputFile, []byte(errmsg), 0644)
		if err != nil {
			fmt.Printf("failed to create errmsg file. ", err.Error())
		}
	}
	if PanicIfErrorUnderUTF {
		if len(args) > 0 {
			s, ok := args[0].(string)
			if ok && strings.HasPrefix(s, "[utf_ign]") {
				return
			}
		}
		panic(fmt.Sprintf("error utf: %v", args...))
	}
}

func (w WrapLogger) Errorf(template string, args ...interface{}) {
	if w.Logger == nil {
		return
	}

	w.Logger.Errorf(template, args...)
	w.Logger.Sync()
	// RootLogger.FlushAndClose() // 避免ws msg handler中recover panic时还需要打印
	errmsg := fmt.Sprintf("error utf: "+template, args...)
	if ErrmsgOutputFile != "" {
		err := ioutil.WriteFile(ErrmsgOutputFile, []byte(errmsg), 0644)
		if err != nil {
			fmt.Printf("failed to create errmsg file. ", err.Error())
		}
	}
	if PanicIfErrorUnderUTF && strings.Index(template, "utf_ign") < 0 {
		fmt.Printf(errmsg)
		panic(fmt.Sprintf("error utf: "+template, args...))
	}
}

func (w WrapLogger) ErrorfWithStacktrace(template string, args ...interface{}) {
	if w.Logger == nil {
		return
	}
	w.Errorf(template, args...)
	PrintStacktrace(w)
}
