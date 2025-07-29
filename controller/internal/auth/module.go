package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"go.uber.org/fx"
)

var Module = fx.Module("auth", fx.Provide(newRsaKey))

func newRsaKey() (*rsa.PrivateKey, error) {
	var privateKey *rsa.PrivateKey
	rng := rand.Reader
	var err error
	privateKey, err = rsa.GenerateKey(rng, 8192)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
