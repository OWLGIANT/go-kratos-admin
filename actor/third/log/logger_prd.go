//go:build !utf && !debug && !utf_log
// +build !utf,!debug,!utf_log

package log

func (l *PhusLogger) Debug(args ...interface{}) {
}
func (l *PhusLogger) Debugf(template string, args ...interface{}) {
}

func (l *ZapLogger) Debug(args ...interface{}) {
}
func (l *ZapLogger) Debugf(template string, args ...interface{}) {
}

func (l WrapLogger) Debug(args ...interface{}) {
}

func (l WrapLogger) Debugf(template string, args ...interface{}) {
}

func (l *PhusLogger) Errorf(template string, args ...interface{}) {
	l.logger.Error().Msgf("\033[31m "+template+" \033[0m", args...)
}

func (l *ZapLogger) Errorf(template string, args ...interface{}) {
	l.sugar.Errorf("\033[31m "+template+" \033[0m", args...)
}
