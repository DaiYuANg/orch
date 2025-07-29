package cmd

import (
	"github.com/DaiYuANg/warden/controller/internal/auth"
	"github.com/DaiYuANg/warden/controller/internal/common"
	"github.com/DaiYuANg/warden/controller/internal/config"
	"github.com/DaiYuANg/warden/controller/internal/dns"
	"github.com/DaiYuANg/warden/controller/internal/endpoint"
	"github.com/DaiYuANg/warden/controller/internal/http"
	"github.com/DaiYuANg/warden/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func container() *fx.App {
	return fx.New(
		config.Module,
		auth.Module,
		logger.Module,
		//raft.Module,
		common.Module,
		endpoint.Module,
		http.Module,
		dns.Module,
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			fxLogger := &fxevent.ZapLogger{Logger: log}
			fxLogger.UseLogLevel(zapcore.DebugLevel)
			return fxLogger
		}),
	)
}
