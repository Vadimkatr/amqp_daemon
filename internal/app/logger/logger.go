package logger

type Log interface {
	Error(interface{})
	Info(interface{})
	Warn(interface{})

	Errorf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Warnf(string, ...interface{})

	With(string) Log
}
