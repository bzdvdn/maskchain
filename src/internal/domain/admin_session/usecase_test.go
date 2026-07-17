package admin_session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseCreate (AC-001)
func TestAdminSessionUseCaseCreate(t *testing.T) {
	store := &mockAdminSessionStore{}
	uc := NewAdminSessionUseCase(store)

	sess, token, err := uc.Create(context.Background(), "admin", 30*time.Minute)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.Username != "admin" {
		t.Errorf("expected username admin, got %s", sess.Username)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if store.saved == nil {
		t.Error("expected session to be saved")
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseCreateStoreError (AC-001)
func TestAdminSessionUseCaseCreateStoreError(t *testing.T) {
	store := &mockAdminSessionStore{saveErr: errors.New("store error")}
	uc := NewAdminSessionUseCase(store)

	_, _, err := uc.Create(context.Background(), "admin", 30*time.Minute)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseValidate (AC-001, AC-004)
func TestAdminSessionUseCaseValidate(t *testing.T) {
	store := &mockAdminSessionStore{}
	uc := NewAdminSessionUseCase(store)

	_, rawToken, err := uc.Create(context.Background(), "admin", 30*time.Minute)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	sess, err := uc.Validate(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sess.Username != "admin" {
		t.Errorf("expected username admin, got %s", sess.Username)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseValidateExpired (AC-004)
func TestAdminSessionUseCaseValidateExpired(t *testing.T) {
	store := &mockAdminSessionStore{}
	uc := NewAdminSessionUseCase(store)

	sess, _, err := uc.Create(context.Background(), "admin", -1*time.Minute)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	store.getByHashResult = sess

	_, err = uc.Validate(context.Background(), "some-token")
	if err != ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseValidateNotFound (AC-004)
func TestAdminSessionUseCaseValidateNotFound(t *testing.T) {
	store := &mockAdminSessionStore{getByHashErr: ErrSessionNotFound}
	uc := NewAdminSessionUseCase(store)

	_, err := uc.Validate(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseDelete (AC-001)
func TestAdminSessionUseCaseDelete(t *testing.T) {
	store := &mockAdminSessionStore{}
	uc := NewAdminSessionUseCase(store)

	uc.Delete(context.Background(), "session-id")
	if store.deletedID != "session-id" {
		t.Errorf("expected session-id, got %s", store.deletedID)
	}
}

// @sk-test admin-ui-design#T4.1: TestAdminSessionUseCaseDeleteExpired (AC-004)
func TestAdminSessionUseCaseDeleteExpired(t *testing.T) {
	store := &mockAdminSessionStore{deleteExpiredResult: 5}
	uc := NewAdminSessionUseCase(store)

	deleted, err := uc.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 5 {
		t.Errorf("expected 5, got %d", deleted)
	}
}

// -- mocks --

type mockAdminSessionStore struct {
	saved              *AdminSession
	saveErr            error
	getByHashResult    *AdminSession
	getByHashErr       error
	deletedID          string
	deleteExpiredResult int64
}

func (m *mockAdminSessionStore) Save(_ context.Context, s *AdminSession) error {
	m.saved = s
	return m.saveErr
}

func (m *mockAdminSessionStore) GetByTokenHash(_ context.Context, _ string) (*AdminSession, error) {
	if m.getByHashErr != nil {
		return nil, m.getByHashErr
	}
	if m.getByHashResult != nil {
		return m.getByHashResult, nil
	}
	return m.saved, nil
}

func (m *mockAdminSessionStore) Delete(_ context.Context, id string) error {
	m.deletedID = id
	return nil
}

func (m *mockAdminSessionStore) DeleteExpired(_ context.Context) (int64, error) {
	return m.deleteExpiredResult, nil
}
