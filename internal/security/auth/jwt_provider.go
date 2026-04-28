package auth

import (
	"strings"

	"github.com/arcgolabs/authx"
	authjwt "github.com/arcgolabs/authx/jwt"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/oopsx"
)

// NewJWTAuthenticationProvider builds the JWT authentication provider used by authx when Auth.Enabled is true.
// When auth is disabled it returns nil, nil.
func NewJWTAuthenticationProvider(cfg config.Config) (authx.AuthenticationProvider, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Auth.JWT.Secret) == "" {
		return nil, oopsx.B("auth").Errorf("auth enabled but ORCH auth JWT secret is empty (set ORCH_AUTH_JWT_SECRET or equivalent)")
	}
	return authjwt.NewAuthenticationProvider(
		authjwt.WithHMACSecret([]byte(cfg.Auth.JWT.Secret), "HS256"),
	), nil
}
