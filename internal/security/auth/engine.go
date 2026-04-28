package auth

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/authx"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/oopsx"
)

// NewEngine builds the authx engine and registers the JWT authentication provider when Auth.Enabled is true.
// When auth is disabled it returns nil, nil.
func NewEngine(cfg config.Config, logger *slog.Logger, jwt authx.AuthenticationProvider) (*authx.Engine, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if jwt == nil {
		return nil, oopsx.B("auth").Errorf("JWT authentication provider is required when auth is enabled")
	}

	engine := authx.NewEngine(
		authx.WithLogger(logger),
		authx.WithAuthorizer(authx.AuthorizerFunc(func(_ context.Context, input authx.AuthorizationModel) (authx.Decision, error) {
			if input.Principal == nil {
				return authx.Decision{
					Allowed: false,
					Reason:  "anonymous is not allowed",
				}, nil
			}
			return authx.Decision{
				Allowed: true,
				Reason:  "authenticated",
			}, nil
		})),
	)

	if err := authx.RegisterProvider(engine, jwt); err != nil {
		return nil, err
	}
	return engine, nil
}
