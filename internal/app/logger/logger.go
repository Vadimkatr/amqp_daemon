package logger

type Log interface {
	Error(error)
	Errorf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Warnf(string, ...interface{})
	With(string) Log
}
