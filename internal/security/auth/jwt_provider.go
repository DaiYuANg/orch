package auth

import (
	"context"
	"reflect"
	"strings"

	"github.com/arcgolabs/authx"
	authjwt "github.com/arcgolabs/authx/jwt"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// jwtPlaceholderAuth is registered in the DI graph when auth is disabled; the engine registers
// no providers in that mode, so AuthenticateAny is never called.
type jwtPlaceholderAuth struct{}

func (jwtPlaceholderAuth) CredentialType() reflect.Type {
	return reflect.TypeFor[authjwt.TokenCredential]()
}

func (jwtPlaceholderAuth) AuthenticateAny(context.Context, any) (authx.AuthenticationResult, error) {
	return authx.AuthenticationResult{}, oopsx.B("auth").Errorf("JWT provider is disabled")
}

// NewJWTAuthenticationProvider builds the JWT authentication provider used by authx when Auth.Enabled is true.
// When auth is disabled it returns a non-registering placeholder provider.
func NewJWTAuthenticationProvider(cfg config.Config) (authx.AuthenticationProvider, error) {
	if !cfg.Auth.Enabled {
		return jwtPlaceholderAuth{}, nil
	}
	if strings.TrimSpace(cfg.Auth.JWT.Secret) == "" {
		return nil, oopsx.B("auth").Errorf("auth enabled but ORCH auth JWT secret is empty (set ORCH_AUTH_JWT_SECRET or equivalent)")
	}
	return authjwt.NewAuthenticationProvider(
		authjwt.WithHMACSecret([]byte(cfg.Auth.JWT.Secret), "HS256"),
	), nil
}
