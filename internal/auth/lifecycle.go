package auth

import (
	"log/slog"
	"os"
	"path/filepath"
)

func generateRootToken(signer *Singer, logger *slog.Logger) error {
	generatePath := filepath.Join(os.TempDir(), "warden.token")
	sign, err := signer.sign()
	if err != nil {
		return err
	}
	logger.Debug("generate root token", "path", generatePath)
	return os.WriteFile(generatePath, []byte(sign), 0644)
}
