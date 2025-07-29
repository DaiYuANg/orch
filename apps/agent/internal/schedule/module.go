package schedule

import (
	"go.uber.org/fx"
)

var Module = fx.Module("schedule", fx.Invoke(catMem))
