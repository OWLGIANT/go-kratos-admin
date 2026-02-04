package log

import "runtime"

const (
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
	PanicLevel = "panic"
)

var RootLogger Logger

type Logger interface {
	Debug(args ...interface{})

	Info(args ...interface{})

	Warn(args ...interface{})

	Error(args ...interface{})

	Panic(args ...interface{})

	Debugf(template string, args ...interface{})

	Infof(template string, args ...interface{})

	Warnf(template string, args ...interface{})

	Errorf(template string, args ...interface{})
	ErrorfWithStacktrace(template string, args ...interface{})

	Panicf(template string, args ...interface{})

	Sync() error
	FlushAndClose() error
}

// Init init rootLogger
func Init(path, level string, options ...Option) {
	c := &Configuration{
		Path:        path,
		Level:       level,
		MaxFileSize: 100, //MB
		MaxBackups:  10,
		MaxAge:      60,
		Compress:    true,
		Caller:      false,
	}

	for _, option := range options {
		option(c)
	}

	switch c.Logger {
	// case c.SLog:
	// rootLogger = NewSLogger(c)
	case LoggerPhuslog:
		RootLogger = NewPhusLogger(c)
	default:
		RootLogger = NewZapLogger(c)
	}
}

func InitWithLogger(path, level string, options ...Option) (logger Logger) {
	c := &Configuration{
		Path:        path,
		Level:       level,
		MaxFileSize: 100, //MB
		MaxBackups:  10,
		MaxAge:      60,
		Compress:    true,
		Caller:      false,
	}

	for _, option := range options {
		option(c)
	}

	switch c.Logger {
	// case c.SLog:
	// rootLogger = NewSLogger(c)
	case LoggerPhuslog:
		logger = NewPhusLogger(c)
	default:
		logger = NewZapLogger(c)
	}
	return
}

func Debug(args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Debugf(template, args...)
}

func Info(args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Infof(template, args...)
}

func Warn(args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Warnf(template, args...)
}

func Panic(args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	if RootLogger == nil {
		return
	}

	RootLogger.Panicf(template, args...)
}

// Sync flushes buffer, if any
func Sync() {
	if RootLogger == nil {
		return
	}

	RootLogger.Sync()
}

func FlushAndClose() {
	if RootLogger == nil {
		return
	}

	RootLogger.FlushAndClose()
}

var GitCommitHash string

func PrintStacktrace(logger Logger) {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	logger.Errorf("==> %s\n", string(buf[:n]))
}
