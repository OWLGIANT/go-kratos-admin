//go:build !utf_log
// +build !utf_log

package log

func (w WrapLogger) Error(args ...interface{}) {
	if w.Logger == nil {
		return
	}

	w.Logger.Error(args...)

	// args2 := make([]any, 0, len(args)+1)
	// args2 = append(args2, args...)
	// args2 = append(args2, fmt.Sprintf("[%s]", GitCommitHash))
	// w.Logger.Error(args2...)
	// fn, f, ln := getCallerInfo(2)
	// go func() {
	// helper_ding.DingingSendWarning(fmt.Sprintf("%v\n[%s:%s:%d,%s]\n", args, f, fn, ln, GitCommitHash))
	// }()
}

func (w WrapLogger) Errorf(template string, args ...interface{}) {
	if w.Logger == nil {
		return
	}
	w.Logger.Errorf(template, args...)
	// w.Logger.Errorf(template+fmt.Sprintf("[%s]", GitCommitHash), args...)

	// fn, f, ln := getCallerInfo(2)
	// args2 := make([]any, 0, len(args)+4)
	// args2 = append(args2, args...)
	// args2 = append(args2, f, fn, ln, GitCommitHash)
	// go func() {
	// helper_ding.DingingSendWarning(fmt.Sprintf(template+"\n[%s:%s:%d,%s]\n", args2...))
	// }()
}

func (w WrapLogger) ErrorfWithStacktrace(template string, args ...interface{}) {
	if w.Logger == nil {
		return
	}
	w.Logger.Errorf(template, args...)
	PrintStacktrace(w)
}
