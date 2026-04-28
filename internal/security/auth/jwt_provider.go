package auth

import (
	"fmt"
	"strings"

	"github.com/arcgolabs/authx"
	authjwt "github.com/arcgolabs/authx/jwt"

	"github.com/daiyuang/orch/internal/config"
)

// NewJWTAuthenticationProvider builds the JWT authentication provider used by authx when Auth.Enabled is true.
// When auth is disabled it returns nil, nil.
func NewJWTAuthenticationProvider(cfg config.Config) (authx.AuthenticationProvider, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, fmt.Errorf("auth enabled but ORCH auth JWT secret is empty (set ORCH_AUTH__JWT_SECRET or equivalent)")
	}
	return authjwt.NewAuthenticationProvider(
		authjwt.WithHMACSecret([]byte(cfg.Auth.JWTSecret), "HS256"),
	), nil
}
