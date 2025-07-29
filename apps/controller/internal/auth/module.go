package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
	"path/filepath"
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

func newSigner(privateKey *rsa.PrivateKey, logger *zap.SugaredLogger) *Singer {
	return &Singer{
		privateKey: privateKey,
		logger:     logger,
	}
}

func generateRootToken(signer *Singer, logger *zap.SugaredLogger) error {
	generatePath := filepath.Join(os.TempDir(), "warden.token")
	sign, err := signer.sign()
	if err != nil {
		return err
	}
	logger.Debugf("Generate Root token:%s", generatePath)
	return os.WriteFile(generatePath, []byte(sign), 0644)
}
