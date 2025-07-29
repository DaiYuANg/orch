package auth

import (
	"crypto/rsa"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"time"
)

type Singer struct {
	privateKey *rsa.PrivateKey
	logger     *zap.SugaredLogger
}

func (s *Singer) sign() (string, error) {
	claims := jwt.MapClaims{
		"name":  "John Doe",
		"admin": true,
		"exp":   time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t, err := token.SignedString(s.privateKey)
	if err != nil {
		s.logger.Errorf("token.SignedString: %v", err)
		return "", err
	}
	return t, nil
}
