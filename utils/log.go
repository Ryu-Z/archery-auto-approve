package utils

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger = *zap.Logger

func NewLogger(level string) (*zap.Logger, error) {
	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(strings.ToLower(level))); err != nil {
		return nil, err
	}

	cfg := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	return cfg.Build()
}

func FieldString(key, value string) zap.Field {
	return zap.String(key, value)
}

func FieldInt(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func FieldBool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

func FieldTime(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

func FieldError(err error) zap.Field {
	return zap.Error(err)
}
