package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
)

// MessageEncryptor provides AES-256-GCM encryption for SECS-II message payloads.
// Based on the Secured SECS/GEM approach (IJACSA 2021).
// Operates as middleware: encrypt before send, decrypt after receive.
type MessageEncryptor struct {
	mu   sync.RWMutex
	aead cipher.AEAD
	keyID string // Current key identifier for rotation tracking
}

// NewMessageEncryptor creates an encryptor with a 256-bit AES key.
// Key must be exactly 32 bytes.
func NewMessageEncryptor(key []byte, keyID string) (*MessageEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("security: AES-256 requires 32-byte key, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("security: create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("security: create GCM: %w", err)
	}

	return &MessageEncryptor{
		aead:  aead,
		keyID: keyID,
	}, nil
}

// Encrypt encrypts a SECS-II payload using AES-256-GCM.
// Returns: nonce (12 bytes) + ciphertext + GCM tag (16 bytes).
func (e *MessageEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("security: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts an AES-256-GCM encrypted payload.
// Input format: nonce (12 bytes) + ciphertext + GCM tag (16 bytes).
func (e *MessageEncryptor) Decrypt(data []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nonceSize := e.aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("security: ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("security: decrypt: %w", err)
	}

	return plaintext, nil
}

// RotateKey replaces the encryption key. Existing encrypted data must be
// decrypted with the old key before rotation.
func (e *MessageEncryptor) RotateKey(newKey []byte, newKeyID string) error {
	if len(newKey) != 32 {
		return fmt.Errorf("security: AES-256 requires 32-byte key, got %d", len(newKey))
	}

	block, err := aes.NewCipher(newKey)
	if err != nil {
		return fmt.Errorf("security: create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("security: create GCM: %w", err)
	}

	e.mu.Lock()
	e.aead = aead
	e.keyID = newKeyID
	e.mu.Unlock()
	return nil
}

// KeyID returns the current key identifier.
func (e *MessageEncryptor) KeyID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.keyID
}

// GenerateKey generates a cryptographically secure 256-bit key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("security: generate key: %w", err)
	}
	return key, nil
}
