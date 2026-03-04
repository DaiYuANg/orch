package registry

import "go.uber.org/fx"

var Module = fx.Module(
	"registry",
	fx.Provide(
		NewServiceWithRaft,
	),
	fx.Invoke(
		lifecycle,
	),
)

func lifecycle(lc fx.Lifecycle, service *Service) {
	lc.Append(
		fx.StopHook(func() error {
			return service.Close()
		}),
	)
}
