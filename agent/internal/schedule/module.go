package schedule

import (
	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("schedule", fx.Provide(newScheduler), fx.Invoke(catMem, lifecycle))

func newScheduler() (gocron.Scheduler, error) {
	return gocron.NewScheduler(gocron.WithLogger(gocron.NewLogger(gocron.LogLevelInfo)))
}

func lifecycle(scheduler gocron.Scheduler) {
	scheduler.Start()
}
