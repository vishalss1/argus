package common

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var service string

// InitLogger initializes the global production logger for a service and redirects stdlog
func InitLogger(name string) {
	service = name
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	l, err := config.Build()
	if err != nil {
		panic(err)
	}
	Logger = l.With(zap.String("service", name))
	zap.RedirectStdLog(Logger)
}

// Ctx returns a contextual logger enriched with correlation_id if found
func Ctx(ctx context.Context) *zap.Logger {
	if Logger == nil {
		InitLogger("unknown")
	}
	l := Logger
	if corrID, ok := GetCorrelationID(ctx); ok {
		l = l.With(zap.String("correlation_id", corrID))
	}
	return l
}
