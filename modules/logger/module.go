package logger

import (
	"github.com/samber/mo"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
)

var Module = fx.Module("logger_module", fx.Provide(newLogger, sugaredLogger), fx.Invoke(deferLogger))

type NewLoggerDependency struct {
	fx.In
	FilePath *string `name:"logPath" optional:"true"`
}

func newLogger(dep NewLoggerDependency) *zap.Logger {
	filePathOpt := mo.PointerToOption(dep.FilePath)

	// 如果没有值，则使用默认临时目录
	logFile := filePathOpt.OrElse(filepath.Join(os.TempDir(), "warden.log"))

	lumberJackLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     7,
		Compress:   true,
	}

	consoleEncoderCfg := zap.NewProductionEncoderConfig()
	consoleEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	consoleEncoderCfg.TimeKey = "T"
	consoleEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderCfg)

	fileEncoderCfg := zap.NewProductionEncoderConfig()
	fileEncoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	fileEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoderCfg.TimeKey = "timestamp"
	fileEncoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderCfg)

	consoleSyncer := zapcore.Lock(os.Stdout)
	fileSyncer := zapcore.AddSync(lumberJackLogger)

	consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, zapcore.DebugLevel)
	fileCore := zapcore.NewCore(fileEncoder, fileSyncer, zapcore.DebugLevel)

	core := zapcore.NewTee(consoleCore, fileCore)

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
	)

	return logger
}

func sugaredLogger(log *zap.Logger) *zap.SugaredLogger {
	return log.Sugar()
}
