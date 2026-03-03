package auth

import (
	"crypto/rsa"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Singer struct {
	privateKey *rsa.PrivateKey
	logger     *slog.Logger
}

func (s *Singer) sign() (string, error) {
	claims := jwt.MapClaims{
		"name":  "root",
		"admin": true,
		"exp":   time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t, err := token.SignedString(s.privateKey)
	if err != nil {
		s.logger.Error("token signed string error", "error", err)
		return "", err
	}
	return t, nil
}
