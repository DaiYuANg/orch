package common

import (
	"github.com/go-playground/validator/v10"
	"go.uber.org/fx"
)

var Module = fx.Module("common", fx.Provide(newValidator))

func newValidator() *validator.Validate {
	return validator.New()
}
