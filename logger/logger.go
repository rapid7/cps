package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/cps/version"
)

// BuildLogger builds a logger with config options
func BuildLogger(options ...ConfigOption) *zap.Logger {
	initialFields := map[string]interface{}{"commit": version.GitCommit}

	cfg := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		InitialFields:    initialFields,
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey: "message",

			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,

			TimeKey:    "time",
			EncodeTime: zapcore.ISO8601TimeEncoder,

			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	for _, opt := range options {
		opt(&cfg)
	}

	log, _ := cfg.Build()

	return log
}

// ConfigOption is a way of configuring a logger
type ConfigOption func(*zap.Config)

// ConfigWithLevel configures a logger's level
func ConfigWithLevel(l zapcore.Level) ConfigOption {
	return func(config *zap.Config) {
		config.Level = zap.NewAtomicLevelAt(l)
	}
}

// ConfigWithDevelopmentMode configures a logger for dev mode
func ConfigWithDevelopmentMode() ConfigOption {
	return func(config *zap.Config) {
		config.Development = true
		config.Encoding = "console"
		ConfigWithLevel(zapcore.DebugLevel)(config)
	}
}
