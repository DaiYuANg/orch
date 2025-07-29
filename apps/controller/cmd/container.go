package cmd

//func container() *fx.App {
//	return fx.New(
//		config.Module,
//		auth.Module,
//		logger.Module,
//		mdns.Module,
//		raft.Module,
//		common.Module,
//		endpoint.Module,
//		http.Module,
//		dns.Module,
//		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
//			fxLogger := &fxevent.ZapLogger{Logger: log}
//			fxLogger.UseLogLevel(zapcore.DebugLevel)
//			return fxLogger
//		}),
//	)
//}
