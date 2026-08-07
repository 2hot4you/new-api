// Package secure provides authenticated encryption for persisted secrets.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	MasterKeySize = 32
	NonceSize     = 12
)

var (
	ErrInvalidMasterKey = errors.New("master key must be base64-encoded 32 bytes")
	ErrUnknownKey       = errors.New("unknown encryption key version")
	ErrInvalidEnvelope  = errors.New("invalid encrypted value")
)

// SealedValue is the database representation of an encrypted secret. Ciphertext
// contains the GCM authentication tag. None of its fields contains plaintext.
type SealedValue struct {
	Ciphertext []byte
	Nonce      []byte
	KeyVersion uint32
}

// Keyring encrypts with the current key while retaining older keys for rotation.
// It is safe for concurrent use.
type Keyring struct {
	mu             sync.RWMutex
	keys           map[uint32][MasterKeySize]byte
	currentVersion uint32
	random         io.Reader
}

// NewKeyring constructs a keyring from one standard or raw base64-encoded
// AES-256 key.
func NewKeyring(masterKeyBase64 string, keyVersion uint32) (*Keyring, error) {
	key, err := decodeMasterKey(masterKeyBase64)
	if err != nil {
		return nil, err
	}
	return &Keyring{
		keys:           map[uint32][MasterKeySize]byte{keyVersion: key},
		currentVersion: keyVersion,
		random:         rand.Reader,
	}, nil
}

// NewKeyringFromMap constructs a rotation-aware keyring. The map values are
// standard or raw base64-encoded AES-256 keys.
func NewKeyringFromMap(encodedKeys map[uint32]string, currentVersion uint32) (*Keyring, error) {
	if len(encodedKeys) == 0 {
		return nil, ErrInvalidMasterKey
	}
	keys := make(map[uint32][MasterKeySize]byte, len(encodedKeys))
	for version, encoded := range encodedKeys {
		key, err := decodeMasterKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode key version %d: %w", version, err)
		}
		keys[version] = key
	}
	if _, ok := keys[currentVersion]; !ok {
		return nil, fmt.Errorf("current version %d: %w", currentVersion, ErrUnknownKey)
	}
	return &Keyring{keys: keys, currentVersion: currentVersion, random: rand.Reader}, nil
}

// CurrentVersion returns the version used for new ciphertexts.
func (k *Keyring) CurrentVersion() uint32 {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.currentVersion
}

// Encrypt encrypts plaintext using AES-256-GCM and binds it to the environment
// ID and key version through additional authenticated data.
func (k *Keyring) Encrypt(environmentID string, plaintext []byte) (SealedValue, error) {
	if environmentID == "" {
		return SealedValue{}, fmt.Errorf("environment ID is required: %w", ErrInvalidEnvelope)
	}

	k.mu.RLock()
	version := k.currentVersion
	key, ok := k.keys[version]
	random := k.random
	k.mu.RUnlock()
	if !ok {
		return SealedValue{}, ErrUnknownKey
	}

	gcm, err := newGCM(key)
	if err != nil {
		return SealedValue{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(random, nonce); err != nil {
		return SealedValue{}, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, additionalData(environmentID, version))
	return SealedValue{Ciphertext: ciphertext, Nonce: nonce, KeyVersion: version}, nil
}

// Decrypt authenticates and decrypts a persisted value. Moving ciphertext to a
// different environment or changing its key version causes authentication to fail.
func (k *Keyring) Decrypt(environmentID string, value SealedValue) ([]byte, error) {
	if environmentID == "" || len(value.Nonce) != NonceSize || len(value.Ciphertext) < 16 {
		return nil, ErrInvalidEnvelope
	}
	k.mu.RLock()
	key, ok := k.keys[value.KeyVersion]
	k.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownKey
	}

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, value.Nonce, value.Ciphertext, additionalData(environmentID, value.KeyVersion))
	if err != nil {
		return nil, fmt.Errorf("authenticate ciphertext: %w", ErrInvalidEnvelope)
	}
	return plaintext, nil
}

func decodeMasterKey(encoded string) ([MasterKeySize]byte, error) {
	var result [MasterKeySize]byte
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil || len(decoded) != MasterKeySize {
		return result, ErrInvalidMasterKey
	}
	copy(result[:], decoded)
	return result, nil
}

func newGCM(key [MasterKeySize]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return gcm, nil
}

func additionalData(environmentID string, keyVersion uint32) []byte {
	// The fixed prefix and fixed-width version make the encoding unambiguous. The
	// prefix is a format discriminator, not a secret.
	aad := make([]byte, 0, len(environmentID)+18)
	aad = append(aad, "molii-env-key\x00"...)
	var version [4]byte
	binary.BigEndian.PutUint32(version[:], keyVersion)
	aad = append(aad, version[:]...)
	aad = append(aad, environmentID...)
	return aad
}
