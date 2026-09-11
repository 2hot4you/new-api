package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeRedisTestCA(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Redis test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "redis-ca.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0o600))
	return path
}

func TestParseRedisOptionsLoadsConfiguredCAForTLS(t *testing.T) {
	options, err := parseRedisOptions(
		"rediss://:secret@10.100.10.14:6379/0",
		writeRedisTestCA(t),
	)

	require.NoError(t, err)
	require.NotNil(t, options.TLSConfig)
	require.Equal(t, "10.100.10.14", options.TLSConfig.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), options.TLSConfig.MinVersion)
	require.Len(t, options.TLSConfig.RootCAs.Subjects(), 1)
}

func TestParseRedisOptionsKeepsExistingNonTLSConfiguration(t *testing.T) {
	options, err := parseRedisOptions("redis://:secret@redis.example:6379/0", "")

	require.NoError(t, err)
	require.Nil(t, options.TLSConfig)
}

func TestParseRedisOptionsRejectsCAWithoutTLS(t *testing.T) {
	_, err := parseRedisOptions("redis://:secret@redis.example:6379/0", writeRedisTestCA(t))

	require.ErrorContains(t, err, "requires a rediss:// connection")
}

func TestParseRedisOptionsRejectsUnreadableOrInvalidCA(t *testing.T) {
	_, err := parseRedisOptions("rediss://:secret@redis.example:6379/0", filepath.Join(t.TempDir(), "missing.pem"))
	require.ErrorContains(t, err, "read Redis TLS CA")

	invalidPath := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not a certificate"), 0o600))
	_, err = parseRedisOptions("rediss://:secret@redis.example:6379/0", invalidPath)
	require.ErrorContains(t, err, "parse Redis TLS CA")
}
