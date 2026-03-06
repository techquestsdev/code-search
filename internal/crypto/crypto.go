// Package crypto provides encryption utilities for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// EncryptedPrefix is prepended to encrypted values to identify them.
	// This allows distinguishing encrypted from plaintext values.
	EncryptedPrefix = "enc:"

	// KeySize is the required size for AES-256 keys (32 bytes).
	KeySize = 32
)

var (
	// ErrNoKey is returned when encryption/decryption is attempted without a key.
	ErrNoKey = errors.New("encryption key not configured")

	// ErrInvalidKey is returned when the key is invalid.
	ErrInvalidKey = errors.New("invalid encryption key")

	// ErrDecryptionFailed is returned when decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrInvalidCiphertext is returned when the ciphertext format is invalid.
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
)

// TokenEncryptor handles encryption and decryption of sensitive tokens.
type TokenEncryptor struct {
	key    []byte
	gcm    cipher.AEAD
	active bool // true if encryption is enabled (key is set)
}

// NewTokenEncryptor creates a new TokenEncryptor.
// If key is empty, encryption is disabled (passthrough mode).
// The key can be any string - it will be hashed to create a 32-byte AES key.
func NewTokenEncryptor(key string) (*TokenEncryptor, error) {
	te := &TokenEncryptor{
		active: false,
	}

	if key == "" {
		// No key provided - passthrough mode
		return te, nil
	}

	// Hash the key to get exactly 32 bytes for AES-256
	hash := sha256.Sum256([]byte(key))
	te.key = hash[:]

	// Create AES cipher
	block, err := aes.NewCipher(te.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	te.gcm = gcm
	te.active = true

	return te, nil
}

// IsActive returns true if encryption is enabled.
func (te *TokenEncryptor) IsActive() bool {
	return te.active
}

// Encrypt encrypts a plaintext token and returns the ciphertext with prefix.
// If encryption is disabled or the value is empty, returns the original value.
// If the value is already encrypted (has prefix), returns it unchanged.
func (te *TokenEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Already encrypted - return as-is
	if strings.HasPrefix(plaintext, EncryptedPrefix) {
		return plaintext, nil
	}

	// Encryption disabled - passthrough
	if !te.active {
		return plaintext, nil
	}

	// Generate random nonce
	nonce := make([]byte, te.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := te.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 and add prefix
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return EncryptedPrefix + encoded, nil
}

// Decrypt decrypts a ciphertext token and returns the plaintext.
// If the value doesn't have the encrypted prefix, returns it unchanged (plaintext).
// If encryption is disabled but value has prefix, returns an error.
func (te *TokenEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Not encrypted - return as-is
	if !strings.HasPrefix(ciphertext, EncryptedPrefix) {
		return ciphertext, nil
	}

	// Encryption disabled but value is encrypted
	if !te.active {
		return "", ErrNoKey
	}

	// Remove prefix and decode
	encoded := strings.TrimPrefix(ciphertext, EncryptedPrefix)

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: base64 decode: %w", ErrInvalidCiphertext, err)
	}

	// Validate length
	nonceSize := te.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	// Extract nonce and ciphertext
	nonce := data[:nonceSize]
	encrypted := data[nonceSize:]

	// Decrypt
	plaintext, err := te.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// MustDecrypt is like Decrypt but returns the ciphertext on error.
// Useful for backwards compatibility when some tokens may not be encrypted.
func (te *TokenEncryptor) MustDecrypt(ciphertext string) string {
	plaintext, err := te.Decrypt(ciphertext)
	if err != nil {
		// Return original value on error (backwards compat with unencrypted tokens)
		return strings.TrimPrefix(ciphertext, EncryptedPrefix)
	}

	return plaintext
}

// IsEncrypted checks if a value appears to be encrypted.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, EncryptedPrefix)
}
