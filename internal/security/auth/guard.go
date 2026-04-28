package auth

import (
	"context"
	"strings"

	"github.com/arcgolabs/authx"
	authhttp "github.com/arcgolabs/authx/http"
	authjwt "github.com/arcgolabs/authx/jwt"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// NewGuard wraps the HTTP guard around an authx Engine when Auth.Enabled is true.
// When auth is disabled it returns nil, nil.
func NewGuard(cfg config.Config, engine *authx.Engine) (*authhttp.Guard, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if engine == nil {
		return nil, oopsx.B("auth").Errorf("guard requires non-nil engine when auth is enabled")
	}

	guard := authhttp.NewGuard(
		engine,
		authhttp.WithCredentialResolverFunc(func(_ context.Context, req authhttp.RequestInfo) (any, error) {
			raw := strings.TrimSpace(req.Header("Authorization"))
			if raw == "" {
				return nil, oopsx.B("auth").Errorf("authorization header is missing")
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(raw, prefix) {
				return nil, oopsx.B("auth").Errorf("authorization must use bearer token")
			}
			token := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
			return authjwt.NewTokenCredential(token), nil
		}),
		authhttp.WithAuthorizationResolverFunc(func(_ context.Context, req authhttp.RequestInfo, principal any) (authx.AuthorizationModel, error) {
			return authx.AuthorizationModel{
				Principal: principal,
				Action:    req.Method,
				Resource:  req.Path,
			}, nil
		}),
	)
	return guard, nil
}
