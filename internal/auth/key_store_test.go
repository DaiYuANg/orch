package auth

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePrivateKeyPath_FromEnv(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "custom.pem")
	t.Setenv(privateKeyPathEnv, customPath)

	assert.Equal(t, customPath, resolvePrivateKeyPath())
}

func TestLoadOrCreatePrivateKey_PersistentAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "warden", "auth", "jwt.pem")

	first, loaded, err := loadOrCreatePrivateKey(path)
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.FileExists(t, path)

	second, loaded, err := loadOrCreatePrivateKey(path)
	require.NoError(t, err)
	assert.True(t, loaded)

	firstBytes := x509.MarshalPKCS1PrivateKey(first)
	secondBytes := x509.MarshalPKCS1PrivateKey(second)
	assert.Equal(t, firstBytes, secondBytes)
}

func TestLoadOrCreatePrivateKey_InvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.pem")
	require.NoError(t, os.WriteFile(path, []byte("not-a-valid-pem"), 0o600))

	_, _, err := loadOrCreatePrivateKey(path)
	require.Error(t, err)
}
