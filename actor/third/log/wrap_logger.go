package log

type WrapLogger struct {
	Logger Logger
}

// 为了适应caller要添加一个层

func (l WrapLogger) Info(args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Info(args...)
}

func (l WrapLogger) Infof(template string, args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Infof(template, args...)
}

func (l WrapLogger) Warn(args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Warn(args...)
}

func (l WrapLogger) Warnf(template string, args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Warnf(template, args...)
}

func (l WrapLogger) Panic(args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Panic(args...)
}

func (l WrapLogger) Panicf(template string, args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Panicf(template, args...)
}

func (l WrapLogger) Sync() error {
	if l.Logger != nil {
		l.Logger.Sync()
	}
	return nil
}
func (l WrapLogger) FlushAndClose() error {
	if l.Logger != nil {
		l.Logger.FlushAndClose()
	}
	return nil
}
