//go:build debug || utf || utf_log
// +build debug utf utf_log

package log

func (l *PhusLogger) Debug(args ...interface{}) {
	l.logger.Debug().Msgs(args...)
}
func (l *PhusLogger) Debugf(template string, args ...interface{}) {
	l.logger.Debug().Msgf(template, args...)
}

func (l *ZapLogger) Debug(args ...interface{}) {
	l.sugar.Debug(args...)
}
func (l *ZapLogger) Debugf(template string, args ...interface{}) {
	l.sugar.Debugf(template, args...)
}
func (l *ZapLogger) Errorf(template string, args ...interface{}) {
	l.sugar.Errorf(template, args...) // 终端输出时，zap本身有颜色，不额外支持颜色打印
}

// func (l *ZapLogger) Errorf(template string, args ...interface{}) {
// l.sugar.Errorf(template, args...)
// }

func (l *PhusLogger) Errorf(template string, args ...interface{}) {
	l.logger.Error().Msgf(template, args...)
}

func (l WrapLogger) Debug(args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Debug(args...)
}

func (l WrapLogger) Debugf(template string, args ...interface{}) {
	if l.Logger == nil {
		return
	}

	l.Logger.Debugf(template, args...)
}
