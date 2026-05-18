package logger

import "log"

type Logger struct{}

func New() *Logger {
	return &Logger{}
}

func (l *Logger) Printf(format string, v ...any) {
	log.Printf(format, v...)
}
