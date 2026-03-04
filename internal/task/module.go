package task

import "go.uber.org/fx"

var Module = fx.Module(
	"task",
	fx.Provide(
		newServiceWithRaft,
	),
	fx.Invoke(
		lifecycle,
	),
)

func lifecycle(lc fx.Lifecycle, service *Service) {
	lc.Append(fx.StartStopHook(
		service.Start,
		service.Stop,
	))
}
