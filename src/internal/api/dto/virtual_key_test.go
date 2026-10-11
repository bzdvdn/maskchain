package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-test key-secret-reveal#T4.2: key responses never leak the secret or ciphertext (AC-002)
func TestVirtualKeyResponseDoesNotLeakSecret(t *testing.T) {
	key := &virtualkey.VirtualKey{
		ID:              "k1",
		TenantID:        "acme",
		KeyHash:         "hash",
		SecretEncrypted: "ciphertext-blob",
		Label:           "prod",
		Metadata:        map[string]string{},
	}

	out := VirtualKeyToResponse(key)

	if _, ok := reflect.TypeOf(out).FieldByName("SecretEncrypted"); ok {
		t.Error("response type must not expose SecretEncrypted")
	}
	if _, ok := reflect.TypeOf(out).FieldByName("KeyHash"); ok {
		t.Error("response type must not expose KeyHash")
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	body := string(b)
	if strings.Contains(body, "ciphertext-blob") || strings.Contains(body, "hash") || strings.Contains(body, "secret") {
		t.Errorf("response leaked sensitive data: %s", body)
	}
}
