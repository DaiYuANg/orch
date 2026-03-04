package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/samber/mo"
)

const (
	privateKeyBits        = 2048
	privateKeyPathEnv     = "WARDEN_AUTH_PRIVATE_KEY_FILE"
	defaultPrivateKeyFile = "jwt_private_key.pem"
	defaultPrivateKeyDir  = "auth"
	defaultPrivateKeyApp  = "warden"
	privateKeyFileMode    = 0o600
	privateKeyDirMode     = 0o700
)

func newRsaKey(logger *slog.Logger) (*rsa.PrivateKey, error) {
	path := resolvePrivateKeyPath()

	key, loaded, err := loadOrCreatePrivateKey(path)
	if err != nil {
		return nil, err
	}

	if loaded {
		logger.Debug("loaded jwt signing key", "path", path)
		return key, nil
	}

	logger.Info("generated jwt signing key", "path", path)
	return key, nil
}

func resolvePrivateKeyPath() string {
	defaultPath := filepath.Join(xdg.DataHome, defaultPrivateKeyApp, defaultPrivateKeyDir, defaultPrivateKeyFile)
	return optionalEnv(privateKeyPathEnv).OrElse(defaultPath)
}

func optionalEnv(key string) mo.Option[string] {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return mo.None[string]()
	}
	return mo.Some(value)
}

func loadOrCreatePrivateKey(path string) (*rsa.PrivateKey, bool, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		key, parseErr := parsePrivateKeyPEM(raw)
		if parseErr != nil {
			return nil, false, fmt.Errorf("parse private key %q: %w", path, parseErr)
		}
		return key, true, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read private key %q: %w", path, err)
	}

	if mkErr := os.MkdirAll(filepath.Dir(path), privateKeyDirMode); mkErr != nil {
		return nil, false, fmt.Errorf("mkdir private key dir for %q: %w", path, mkErr)
	}

	key, genErr := rsa.GenerateKey(rand.Reader, privateKeyBits)
	if genErr != nil {
		return nil, false, fmt.Errorf("generate rsa private key: %w", genErr)
	}

	data, marshalErr := marshalPrivateKeyPEM(key)
	if marshalErr != nil {
		return nil, false, fmt.Errorf("encode private key: %w", marshalErr)
	}
	if writeErr := os.WriteFile(path, data, privateKeyFileMode); writeErr != nil {
		return nil, false, fmt.Errorf("write private key %q: %w", path, writeErr)
	}

	return key, false, nil
}

func parsePrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid pem block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not rsa")
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}
}

func marshalPrivateKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("private key is nil")
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(block), nil
}
