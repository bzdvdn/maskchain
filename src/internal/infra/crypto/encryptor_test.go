package crypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func testKeyBase64(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// @sk-test conversation-logging#T1.2: TestEncryptor_RoundTrip (AC-004)
func TestEncryptor_RoundTrip(t *testing.T) {
	enc, err := New(testKeyBase64(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	plaintext := []byte("{\"messages\":[{\"role\":\"user\",\"content\":\"Привет мир\"}]}")

	ct, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if bytes.Contains(ct, plaintext) {
		t.Error("ciphertext must not contain plaintext")
	}

	decrypted, err := enc.Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("round trip mismatch:\n got  %q\n want %q", decrypted, plaintext)
	}
}

// @sk-test conversation-logging#T1.2: TestEncryptor_WrongKey (AC-004)
func TestEncryptor_WrongKey(t *testing.T) {
	enc, err := New(testKeyBase64(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct, err := enc.Encrypt([]byte("sensitive request payload"))
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	other := make([]byte, 32)
	other[0] = 99
	otherEnc, err := New(base64.StdEncoding.EncodeToString(other))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := otherEnc.Decrypt(ct); err == nil {
		t.Fatal("expected authentication error for wrong key, got nil")
	}
}

// @sk-test conversation-logging#T1.2: TestEncryptor_NoPlaintextInCiphertext (AC-004)
func TestEncryptor_NoPlaintextInCiphertext(t *testing.T) {
	enc, err := New(testKeyBase64(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	plaintext := []byte("very-secret-conversation-content")
	ct, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if bytes.Contains(ct, plaintext) {
		t.Error("ciphertext contains plaintext fragment")
	}
	if len(ct) != len(plaintext)+nonceSize+16 {
		t.Errorf("unexpected ciphertext length %d (want %d)", len(ct), len(plaintext)+nonceSize+16)
	}
}

// @sk-test conversation-logging#T1.2: TestEncryptor_NewFailClosed (AC-008)
func TestEncryptor_NewFailClosed(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "not base64", key: "!!!not-base64!!!"},
		{name: "wrong length", key: base64.StdEncoding.EncodeToString([]byte("short"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.key); err == nil {
				t.Fatalf("expected error for %s key", tt.name)
			}
		})
	}
}

// @sk-test conversation-logging#T1.2: TestEncryptor_DecryptShortInput (AC-004)
func TestEncryptor_DecryptShortInput(t *testing.T) {
	enc, err := New(testKeyBase64(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := enc.Decrypt([]byte("short")); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("expected 'too short' error, got %v", err)
	}
}
