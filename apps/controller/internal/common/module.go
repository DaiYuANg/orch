package common

import (
	"github.com/go-playground/validator/v10"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("common", fx.Provide(newValidator, newAnts))

func newValidator() *validator.Validate {
	return validator.New()
}

func newAnts() (*ants.Pool, error) {
	return ants.NewPool(10000)
}
