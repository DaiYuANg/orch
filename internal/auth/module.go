package auth

import (
	"crypto/rsa"
	"log/slog"

	"go.uber.org/fx"
)

var Module = fx.Module("auth", fx.Provide(newRsaKey, newSigner), fx.Invoke(generateRootToken))

func newSigner(privateKey *rsa.PrivateKey, logger *slog.Logger) *Singer {
	return &Singer{
		privateKey: privateKey,
		logger:     logger,
	}
}
