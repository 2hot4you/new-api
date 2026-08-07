package secure

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func encodedKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, MasterKeySize))
}

func TestKeyringEncryptDecrypt(t *testing.T) {
	keyring, err := NewKeyring(encodedKey(0x42), 7)
	require.NoError(t, err)
	keyring.random = bytes.NewReader(bytes.Repeat([]byte{0x11}, NonceSize))

	sealed, err := keyring.Encrypt("env-1", []byte("secret-token"))
	require.NoError(t, err)
	require.Equal(t, uint32(7), sealed.KeyVersion)
	require.Equal(t, bytes.Repeat([]byte{0x11}, NonceSize), sealed.Nonce)
	require.NotContains(t, string(sealed.Ciphertext), "secret-token")

	plaintext, err := keyring.Decrypt("env-1", sealed)
	require.NoError(t, err)
	require.Equal(t, []byte("secret-token"), plaintext)
}

func TestKeyringUsesUniqueRandomNonces(t *testing.T) {
	keyring, err := NewKeyring(encodedKey(0x21), 1)
	require.NoError(t, err)
	keyring.random = bytes.NewReader(append(bytes.Repeat([]byte{1}, NonceSize), bytes.Repeat([]byte{2}, NonceSize)...))

	first, err := keyring.Encrypt("env", []byte("same"))
	require.NoError(t, err)
	second, err := keyring.Encrypt("env", []byte("same"))
	require.NoError(t, err)
	require.NotEqual(t, first.Nonce, second.Nonce)
	require.NotEqual(t, first.Ciphertext, second.Ciphertext)
}

func TestKeyringBindsEnvironmentAndVersion(t *testing.T) {
	keyring, err := NewKeyringFromMap(map[uint32]string{1: encodedKey(1), 2: encodedKey(2)}, 2)
	require.NoError(t, err)
	keyring.random = bytes.NewReader(bytes.Repeat([]byte{3}, NonceSize))
	sealed, err := keyring.Encrypt("env-a", []byte("token"))
	require.NoError(t, err)

	_, err = keyring.Decrypt("env-b", sealed)
	require.ErrorIs(t, err, ErrInvalidEnvelope)

	sealed.KeyVersion = 1
	_, err = keyring.Decrypt("env-a", sealed)
	require.ErrorIs(t, err, ErrInvalidEnvelope)
}

func TestKeyringRejectsInvalidInputs(t *testing.T) {
	_, err := NewKeyring("not-base64", 1)
	require.ErrorIs(t, err, ErrInvalidMasterKey)
	_, err = NewKeyring(base64.StdEncoding.EncodeToString([]byte("short")), 1)
	require.ErrorIs(t, err, ErrInvalidMasterKey)
	_, err = NewKeyringFromMap(map[uint32]string{1: encodedKey(1)}, 2)
	require.ErrorIs(t, err, ErrUnknownKey)

	keyring, err := NewKeyring(encodedKey(1), 1)
	require.NoError(t, err)
	keyring.random = bytes.NewReader(nil)
	_, err = keyring.Encrypt("env", []byte("token"))
	require.Error(t, err)
	_, err = keyring.Decrypt("", SealedValue{})
	require.ErrorIs(t, err, ErrInvalidEnvelope)
}
