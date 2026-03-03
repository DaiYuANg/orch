package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"log/slog"

	"go.uber.org/fx"
)

var Module = fx.Module("auth", fx.Provide(newRsaKey, newSigner), fx.Invoke(generateRootToken))

func newRsaKey() (*rsa.PrivateKey, error) {
	var privateKey *rsa.PrivateKey
	rng := rand.Reader
	var err error
	privateKey, err = rsa.GenerateKey(rng, 2048)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func newSigner(privateKey *rsa.PrivateKey, logger *slog.Logger) *Singer {
	return &Singer{
		privateKey: privateKey,
		logger:     logger,
	}
}
