package security

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	enc, err := NewMessageEncryptor(key, "key-1")
	if err != nil {
		t.Fatalf("NewMessageEncryptor: %v", err)
	}

	plaintext := []byte("S6F11 event report data with sensitive process parameters")

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	// Ciphertext should be longer (nonce + tag overhead)
	if len(ciphertext) <= len(plaintext) {
		t.Errorf("ciphertext len = %d, should be > %d", len(ciphertext), len(plaintext))
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptEmpty(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key, "key-1")

	ct, err := enc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	pt, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(pt) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(pt))
	}
}

func TestDecryptTampered(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key, "key-1")

	ct, _ := enc.Encrypt([]byte("secret data"))

	// Tamper with ciphertext
	ct[len(ct)-1] ^= 0xFF

	_, err := enc.Decrypt(ct)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	enc1, _ := NewMessageEncryptor(key1, "key-1")
	enc2, _ := NewMessageEncryptor(key2, "key-2")

	ct, _ := enc1.Encrypt([]byte("secret"))
	_, err := enc2.Decrypt(ct)
	if err == nil {
		t.Error("expected error for wrong key")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key, "key-1")

	_, err := enc.Decrypt([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestKeyRotation(t *testing.T) {
	key1, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key1, "key-1")

	if enc.KeyID() != "key-1" {
		t.Errorf("KeyID = %s, want key-1", enc.KeyID())
	}

	// Encrypt with key 1
	ct, _ := enc.Encrypt([]byte("data"))

	// Rotate to key 2
	key2, _ := GenerateKey()
	enc.RotateKey(key2, "key-2")

	if enc.KeyID() != "key-2" {
		t.Errorf("KeyID = %s, want key-2", enc.KeyID())
	}

	// Old ciphertext should fail with new key
	_, err := enc.Decrypt(ct)
	if err == nil {
		t.Error("expected error: old ciphertext with new key")
	}
}

func TestInvalidKeySize(t *testing.T) {
	_, err := NewMessageEncryptor([]byte("short"), "bad")
	if err == nil {
		t.Error("expected error for short key")
	}

	key, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key, "ok")
	if err := enc.RotateKey([]byte("short"), "bad"); err == nil {
		t.Error("expected error for short rotation key")
	}
}

func TestGenerateKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	if len(key1) != 32 {
		t.Errorf("key length = %d, want 32", len(key1))
	}
	if bytes.Equal(key1, key2) {
		t.Error("two generated keys should differ")
	}
}

func TestUniqueNonces(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewMessageEncryptor(key, "key-1")

	ct1, _ := enc.Encrypt([]byte("same data"))
	ct2, _ := enc.Encrypt([]byte("same data"))

	// Same plaintext should produce different ciphertext (different nonce)
	if bytes.Equal(ct1, ct2) {
		t.Error("encryptions of same data should produce different ciphertext")
	}
}
