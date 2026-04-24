package logger

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"vrec/internal/config"
)

func NewLogger(cfg *config.LoggerConfig) (*zap.Logger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var cores []zapcore.Core

	// stderr output
	cores = append(cores, zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stderr),
		zapcore.WarnLevel,
	))

	// file output with lumberjack rotation
	if cfg.Path != "" {
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.Path,
			MaxSize:    cfg.MaxSize, // MB
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge, // days
			Compress:   cfg.Compress,
		}

		level, _ := zapcore.ParseLevel(cfg.Level)
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(lumberjackLogger),
			level,
		))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core)

	return logger, nil
}

func NewLoggerWithWriter(cfg *config.LoggerConfig, w io.Writer) (*zap.Logger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(w),
		zapcore.InfoLevel,
	)

	logger := zap.New(core)
	return logger, nil
}
