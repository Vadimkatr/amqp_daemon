package logger

import (
	"log"
	"os"
)

type CustomLogger struct {
	logger *log.Logger
}

func (cl *CustomLogger) Init() {
	cl.logger = log.New(os.Stdout, "",  log.Ldate|log.Ltime)
}

func (cl CustomLogger) Error(err interface{}) {
	cl.logger.Printf("ERROR: %v", err)
}

func (cl CustomLogger) Info(info interface{}) {
	cl.logger.Printf("INFO: %v", info)
}

func (cl CustomLogger) Warn(warn interface{}) {
	cl.logger.Printf("WARNING: %v", warn)
}

func (cl CustomLogger) Errorf(format string, a ...interface{}) {
	cl.logger.Printf("ERROR: " + format, a)
}

func (cl CustomLogger) Infof(format string, a ...interface{}) {
	cl.logger.Printf("INFO: " + format, a)
}

func (cl CustomLogger) Warnf(format string, a ...interface{}) {
	cl.logger.Printf("WARNING: " + format, a)
}

func (cl CustomLogger) With(prf string) Log {
	cl.logger.SetPrefix(prf + " ")
	return cl
}
