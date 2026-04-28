package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arcgolabs/authx"
	authhttp "github.com/arcgolabs/authx/http"
	authjwt "github.com/arcgolabs/authx/jwt"

	"github.com/daiyuang/orch/internal/config"
)

func NewGuard(cfg config.Config, logger *slog.Logger) (*authhttp.Guard, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, fmt.Errorf("auth enabled but WARDEN_AUTH_JWT_SECRET is empty")
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

	provider := authjwt.NewAuthenticationProvider(
		authjwt.WithHMACSecret([]byte(cfg.Auth.JWTSecret), "HS256"),
	)
	if err := authx.RegisterProvider(engine, provider); err != nil {
		return nil, err
	}

	guard := authhttp.NewGuard(
		engine,
		authhttp.WithCredentialResolverFunc(func(_ context.Context, req authhttp.RequestInfo) (any, error) {
			raw := strings.TrimSpace(req.Header("Authorization"))
			if raw == "" {
				return nil, fmt.Errorf("authorization header is missing")
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(raw, prefix) {
				return nil, fmt.Errorf("authorization must use bearer token")
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

