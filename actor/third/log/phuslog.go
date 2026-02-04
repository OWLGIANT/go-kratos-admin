package log

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/phuslu/log"
)

type PhusLogger struct {
	*Configuration
	logger log.Logger
}

func (l *PhusLogger) Build() {
	var level log.Level
	switch l.Level {
	case DebugLevel:
		level = log.DebugLevel
	case InfoLevel:
		level = log.InfoLevel
	case WarnLevel:
		level = log.WarnLevel
	case ErrorLevel:
		level = log.ErrorLevel
	case PanicLevel:
		level = log.PanicLevel
	default:
		level = log.InfoLevel
	}
	// writer
	var writer log.Writer
	writer = &log.FileWriter{
		Filename:   l.Path,
		FileMode:   0600,
		MaxSize:    int64(l.MaxFileSize) * 1024 * 1024,
		MaxBackups: l.MaxBackups,
		LocalTime:  true,
		ProcessID:  false,
		Cleaner: func(filename string, maxBackups int, matches []os.FileInfo) {
			var dir = filepath.Dir(filename)
			filenames := make([]string, 0, len(matches))
			for _, fi := range matches {
				if fi.Mode()&os.ModeSymlink != 0 {
					continue
				}
				filenames = append(filenames, fi.Name())
			}
			// 将 filenames 逆序排序
			sort.Sort(sort.Reverse(sort.StringSlice(filenames)))

			for i, fi := range filenames[1:] {
				filename := filepath.Join(dir, fi)
				// 如果是软连接，跳过
				switch {
				case i >= maxBackups:
					os.Remove(filename)
					// 强制不压缩. 大后台有个功能可以查看/搜索历史所有的日志，如果压缩就用不了这个功能了
					// case !strings.HasSuffix(filename, ".gz"):
					// 	if l.Compress {
					// 		go exec.Command("nice", "-n", "10", "gzip", filename).Run() // 19最低优先级
					// 	}
				}
			}
		},
	}
	if l.Stdout {
		// writer = &log.MultiIOWriter{
		// writer.(*log.FileWriter),
		// os.Stdout,
		// }
		writer = &log.MultiEntryWriter{
			writer.(*log.FileWriter),
			&log.ConsoleWriter{
				ColorOutput: true,
			},
		}
	}

	// caller
	caller := max(3, l.CallerSkip)
	l.logger = log.Logger{
		Level:     level,
		Caller:    caller,
		TimeField: "time",
		// TimeFormat: log.TimeFormatUnixWithMs,
		TimeFormat: "2006-01-02T15:04:05.000000000Z",
	}
	l.logger.Writer = &log.AsyncWriter{
		ChannelSize: 100,
		Writer:      writer,
	}

}

func (l *PhusLogger) Info(args ...interface{}) {
	l.logger.Info().Msgs(args...)
}
func (l *PhusLogger) Warn(args ...interface{}) {
	l.logger.Warn().Msgs(args...)
}

func (l *PhusLogger) Error(args ...interface{}) {
	l.logger.Error().Msgs(args...)
}

func (l *PhusLogger) Panic(args ...interface{}) {
	l.logger.Panic().Msgs(args...)
}

func (l *PhusLogger) Fatal(args ...interface{}) {
	l.logger.Fatal().Msgs(args...)
}

func (l *PhusLogger) Infof(template string, args ...interface{}) {
	l.logger.Info().Msgf(template, args...)
}

func (l *PhusLogger) Warnf(template string, args ...interface{}) {
	l.logger.Warn().Msgf(template, args...)
}

func (l *PhusLogger) ErrorfWithStacktrace(template string, args ...interface{}) {
	l.logger.Error().Msgf(template, args...)
	PrintStacktrace(l)
}

func (l *PhusLogger) Panicf(template string, args ...interface{}) {
	l.logger.Panic().Msgf(template, args...)
}

func (l *PhusLogger) Fatalf(template string, args ...interface{}) {
	l.logger.Fatal().Msgf(template, args...)
}

func (l *PhusLogger) Sync() error {
	return errors.New("nothing can do")
}

func (l *PhusLogger) FlushAndClose() error {
	// 可能会使用channel异步化，所以程序退出时要flush
	if c, ok := l.logger.Writer.(io.Closer); !ok {
		return errors.New("not io.Closer")
	} else {
		c.Close()
		return nil
	}
}

func NewPhusLogger(c *Configuration) *PhusLogger {
	logger := &PhusLogger{Configuration: c}
	logger.Build()

	return logger
}
