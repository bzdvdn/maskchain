package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeyEnvVar is the environment variable holding the 32-byte base64-encoded
// symmetric key used by the conversation logging pipeline.
const KeyEnvVar = "MASKCHAIN_CONVERSATION_KEY"

const nonceSize = 12

var errInvalidKey = errors.New("crypto: invalid key: expected 32 base64-encoded bytes")

// @sk-task conversation-logging#T1.2: Implement Encryptor with New fail-closed constructor (AC-004, AC-008)
//
// Encryptor performs AEAD (AES-256-GCM) encryption using a fixed symmetric key.
type Encryptor struct {
	aead cipher.AEAD
}

// @sk-task conversation-logging#T1.2: New creates an Encryptor from a base64 key, fail-closed (AC-004, AC-008)
//
// New decodes keyB64 (32 bytes) and initializes AES-256-GCM. An empty or
// invalid key returns an error so callers fail closed rather than silently
// storing plaintext.
func New(keyB64 string) (*Encryptor, error) {
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidKey, err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", errInvalidKey, len(raw))
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidKey, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidKey, err)
	}
	return &Encryptor{aead: aead}, nil
}

// @sk-task conversation-logging#T1.2: Encrypt seals plaintext as nonce(12)||ciphertext (AC-004)
//
// Encrypt returns a random 12-byte nonce followed by the AES-GCM ciphertext.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}
	sealed := e.aead.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

// @sk-task conversation-logging#T1.2: Decrypt opens nonce(12)||ciphertext (AC-004)
//
// Decrypt authenticates and decrypts a value produced by Encrypt. Any
// tampering or use of a wrong key results in an authentication error.
func (e *Encryptor) Decrypt(data []byte) ([]byte, error) {
	if len(data) < nonceSize {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce := data[:nonceSize]
	ct := data[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plaintext, nil
}
