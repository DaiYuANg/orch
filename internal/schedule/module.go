package schedule

import (
	"log/slog"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("schedule", fx.Provide(newScheduler), fx.Invoke(lifecycle))

func newScheduler(logger *slog.Logger) (gocron.Scheduler, error) {
	slog := &scheduleSlogLogger{logger: logger}
	return gocron.NewScheduler(gocron.WithLogger(slog))
}

func lifecycle(scheduler gocron.Scheduler) {
	scheduler.Start()
}
