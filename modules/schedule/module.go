package schedule

import (
	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("schedule", fx.Provide(newScheduler), fx.Invoke(lifecycle))

func newScheduler(logger *zap.SugaredLogger) (gocron.Scheduler, error) {
	slog := &scheduleZapLogger{logger: logger}
	return gocron.NewScheduler(gocron.WithLogger(slog))
}

func lifecycle(scheduler gocron.Scheduler) {
	scheduler.Start()
}
