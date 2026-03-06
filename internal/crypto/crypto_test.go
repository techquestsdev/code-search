package crypto

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
)

func TestNewTokenEncryptor(t *testing.T) {
	t.Run("with empty key (passthrough mode)", func(t *testing.T) {
		enc, err := NewTokenEncryptor("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if enc.IsActive() {
			t.Error("expected encryptor to be inactive with empty key")
		}
	})

	t.Run("with valid key", func(t *testing.T) {
		enc, err := NewTokenEncryptor("my-secret-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !enc.IsActive() {
			t.Error("expected encryptor to be active with key")
		}
	})
}

func TestTokenEncryptor_Encrypt(t *testing.T) {
	enc, err := NewTokenEncryptor("test-key-123")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("encrypts plaintext", func(t *testing.T) {
		plaintext := "ghp_supersecrettoken123"

		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		if !strings.HasPrefix(ciphertext, EncryptedPrefix) {
			t.Errorf("expected prefix %q, got %q", EncryptedPrefix, ciphertext[:4])
		}

		if ciphertext == plaintext {
			t.Error("ciphertext should differ from plaintext")
		}
	})

	t.Run("returns empty string for empty input", func(t *testing.T) {
		result, err := enc.Encrypt("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("skips already encrypted values", func(t *testing.T) {
		alreadyEncrypted := EncryptedPrefix + "somebase64data=="

		result, err := enc.Encrypt(alreadyEncrypted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != alreadyEncrypted {
			t.Errorf("expected unchanged value, got %q", result)
		}
	})

	t.Run("each encryption produces different ciphertext (random nonce)", func(t *testing.T) {
		plaintext := "test-token"
		result1, _ := enc.Encrypt(plaintext)
		result2, _ := enc.Encrypt(plaintext)

		if result1 == result2 {
			t.Error("expected different ciphertext for same plaintext (nonce should differ)")
		}
	})
}

func TestTokenEncryptor_Decrypt(t *testing.T) {
	enc, err := NewTokenEncryptor("test-key-123")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("decrypts encrypted value", func(t *testing.T) {
		original := "ghp_supersecrettoken123"

		encrypted, err := enc.Encrypt(original)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		decrypted, err := enc.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if decrypted != original {
			t.Errorf("expected %q, got %q", original, decrypted)
		}
	})

	t.Run("returns plaintext unchanged", func(t *testing.T) {
		plaintext := "not-encrypted-token"

		result, err := enc.Decrypt(plaintext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != plaintext {
			t.Errorf("expected %q, got %q", plaintext, result)
		}
	})

	t.Run("returns empty string for empty input", func(t *testing.T) {
		result, err := enc.Decrypt("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("fails with invalid base64", func(t *testing.T) {
		_, err := enc.Decrypt(EncryptedPrefix + "not-valid-base64!!!")
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})

	t.Run("fails with wrong key", func(t *testing.T) {
		// Encrypt with one key
		enc1, _ := NewTokenEncryptor("key-one")
		encrypted, _ := enc1.Encrypt("secret-data")

		// Try to decrypt with different key
		enc2, _ := NewTokenEncryptor("key-two")

		_, err := enc2.Decrypt(encrypted)
		if !errors.Is(err, ErrDecryptionFailed) {
			t.Errorf("expected ErrDecryptionFailed, got %v", err)
		}
	})
}

func TestTokenEncryptor_MustDecrypt(t *testing.T) {
	enc, err := NewTokenEncryptor("test-key")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("returns decrypted value on success", func(t *testing.T) {
		original := "my-token"
		encrypted, _ := enc.Encrypt(original)

		result := enc.MustDecrypt(encrypted)
		if result != original {
			t.Errorf("expected %q, got %q", original, result)
		}
	})

	t.Run("returns original on error (backwards compat)", func(t *testing.T) {
		// Using a different key for encryption
		otherEnc, _ := NewTokenEncryptor("other-key")
		encrypted, _ := otherEnc.Encrypt("data")

		// MustDecrypt should return the value without prefix on error
		result := enc.MustDecrypt(encrypted)
		// Should strip the prefix and return the base64 part
		expected := strings.TrimPrefix(encrypted, EncryptedPrefix)
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("passes through plaintext", func(t *testing.T) {
		plaintext := "plain-token"

		result := enc.MustDecrypt(plaintext)
		if result != plaintext {
			t.Errorf("expected %q, got %q", plaintext, result)
		}
	})
}

func TestTokenEncryptor_PassthroughMode(t *testing.T) {
	// Create encryptor without key (passthrough mode)
	enc, err := NewTokenEncryptor("")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("encrypt returns value unchanged", func(t *testing.T) {
		token := "ghp_token123"

		result, err := enc.Encrypt(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != token {
			t.Errorf("expected %q, got %q", token, result)
		}
	})

	t.Run("decrypt returns value unchanged", func(t *testing.T) {
		token := "ghp_token123"

		result, err := enc.Decrypt(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != token {
			t.Errorf("expected %q, got %q", token, result)
		}
	})

	t.Run("decrypt fails for encrypted value without key", func(t *testing.T) {
		// Value was encrypted with a key, but we're in passthrough mode
		encryptedValue := EncryptedPrefix + "somebase64data=="

		_, err := enc.Decrypt(encryptedValue)
		if !errors.Is(err, ErrNoKey) {
			t.Errorf("expected ErrNoKey, got %v", err)
		}
	})
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{EncryptedPrefix + "data", true},
		{"enc:base64stuff", true},
		{"plaintext", false},
		{"", false},
		{"encrypted:", false}, // Wrong prefix
	}

	for _, tt := range tests {
		result := IsEncrypted(tt.value)
		if result != tt.expected {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	enc, _ := NewTokenEncryptor("round-trip-key")

	tokens := []string{
		"ghp_abcdefghijklmnop",
		"glpat-xxxxxxxxxxxx",
		"short",
		"a",
		"token with spaces",
		"token/with/slashes",
		"特殊字符",                    // Unicode
		strings.Repeat("x", 1000), // Long token
	}

	for _, token := range tokens {
		encrypted, err := enc.Encrypt(token)
		if err != nil {
			t.Errorf("encrypt(%q) failed: %v", token, err)
			continue
		}

		decrypted, err := enc.Decrypt(encrypted)
		if err != nil {
			t.Errorf("decrypt failed for %q: %v", token, err)
			continue
		}

		if decrypted != token {
			t.Errorf("round trip failed: got %q, want %q", decrypted, token)
		}
	}
}
