package auth

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

func generateRootToken(signer *Singer, logger *zap.SugaredLogger) error {
	generatePath := filepath.Join(os.TempDir(), "warden.token")
	sign, err := signer.sign()
	if err != nil {
		return err
	}
	logger.Debugf("Generate Root token:%s", generatePath)
	return os.WriteFile(generatePath, []byte(sign), 0644)
}
