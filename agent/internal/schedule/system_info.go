package schedule

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"
	"time"
)

func catMem(scheduler gocron.Scheduler, log *zap.SugaredLogger) error {
	_, err := scheduler.NewJob(
		gocron.DurationJob(
			1*time.Second,
		),
		gocron.NewTask(
			func() {
				v, _ := mem.VirtualMemory()
				log.Debugf("Total: %v, Free:%v, UsedPercent:%f%%", v.Total, v.Free, v.UsedPercent)
			},
		),
	)
	if err != nil {
		return err
	}
	return nil
}
