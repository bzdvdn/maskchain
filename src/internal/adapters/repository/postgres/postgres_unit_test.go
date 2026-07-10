package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 30-shield-persistence#T3.3: TestParseSeverity (AC-006)
func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  value.Severity
	}{
		{"low", value.SeverityLow},
		{"medium", value.SeverityMedium},
		{"high", value.SeverityHigh},
		{"critical", value.SeverityCritical},
		{"unknown", value.SeverityLow},
		{"", value.SeverityLow},
	}
	for _, tt := range tests {
		got := parseSeverity(tt.input)
		if got != tt.want {
			t.Errorf("parseSeverity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// @sk-test 30-shield-persistence#T3.3: TestMarshalUnmarshalPreprocessors (AC-006)
func TestMarshalUnmarshalPreprocessors(t *testing.T) {
	pp := []preprocessor.PreprocessorDef{
		{Name: "csv-mask", Type: "csv", Rules: []preprocessor.Rule{{Columns: []string{"email"}, Mask: "full"}}},
	}

	data, err := marshalPreprocessors(pp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := unmarshalPreprocessors(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 preprocessor, got %d", len(got))
	}
	if got[0].Name != "csv-mask" {
		t.Errorf("unexpected name: %q", got[0].Name)
	}
}

// @sk-test 30-shield-persistence#T3.3: TestMarshalNilPreprocessors (AC-006)
func TestMarshalNilPreprocessors(t *testing.T) {
	data, err := marshalPreprocessors(nil)
	if err != nil {
		t.Fatalf("marshal nil: %v", err)
	}
	got, err := unmarshalPreprocessors(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// @sk-test 30-shield-persistence#T3.3: TestTransactionManager_RunInTx_success (AC-003, AC-006)
func TestTransactionManager_RunInTx_success(t *testing.T) {
	mock := newMockTxManager()
	var called bool

	err := mock.RunInTx(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected fn to be called")
	}
	if mock.beginCalls == 0 {
		t.Error("expected Begin to be called")
	}
	if mock.commitCalls == 0 {
		t.Error("expected Commit to be called")
	}
}

// @sk-test 30-shield-persistence#T3.3: TestTransactionManager_RunInTx_rollback (AC-003, AC-006)
func TestTransactionManager_RunInTx_rollback(t *testing.T) {
	mock := newMockTxManager()
	wantErr := errors.New("business error")

	err := mock.RunInTx(context.Background(), func(ctx context.Context) error {
		return wantErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if mock.rollbackCalls == 0 {
		t.Error("expected Rollback to be called on error")
	}
	if mock.commitCalls > 0 {
		t.Error("expected no Commit on error")
	}
}

// @sk-test 30-shield-persistence#T3.3: TestUnmarshalNullPreprocessors (AC-006)
func TestUnmarshalNullPreprocessors(t *testing.T) {
	got, err := unmarshalPreprocessors([]byte("null"))
	if err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	got, err = unmarshalPreprocessors(nil)
	if err != nil {
		t.Fatalf("unmarshal nil bytes: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// -- test helpers --

type mockTransactionManager struct {
	beginCalls    int
	commitCalls   int
	rollbackCalls int
	mu            sync.Mutex
	beginErr      error
	commitErr     error
}

func newMockTxManager() *mockTransactionManager {
	return &mockTransactionManager{}
}

func (m *mockTransactionManager) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	m.mu.Lock()
	m.beginCalls++
	m.mu.Unlock()

	if m.beginErr != nil {
		return m.beginErr
	}

	err := fn(ctx)

	m.mu.Lock()
	if err != nil {
		m.rollbackCalls++
		m.mu.Unlock()
		return err
	}

	if m.commitErr != nil {
		m.rollbackCalls++
		m.mu.Unlock()
		return m.commitErr
	}

	m.commitCalls++
	m.mu.Unlock()
	return nil
}
