package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockProfileRepo struct {
	mu       sync.Mutex
	profiles map[string]*entity.Profile
}

func newMockProfileRepo() *mockProfileRepo {
	return &mockProfileRepo{profiles: make(map[string]*entity.Profile)}
}

func (r *mockProfileRepo) Save(ctx context.Context, p *entity.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[p.Slug().String()] = p
	return nil
}

func (r *mockProfileRepo) FindByID(ctx context.Context, id value.ProfileID) (*entity.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.profiles {
		if p.ID().String() == id.String() {
			return p, nil
		}
	}
	return nil, nil
}

func (r *mockProfileRepo) FindBySlug(ctx context.Context, tenantID value.TenantID, slug value.ProfileSlug) (*entity.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.profiles[slug.String()]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (r *mockProfileRepo) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*entity.Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		result = append(result, p)
	}
	return result, nil
}

func (r *mockProfileRepo) Delete(ctx context.Context, id value.ProfileID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for slug, p := range r.profiles {
		if p.ID().String() == id.String() {
			delete(r.profiles, slug)
			return nil
		}
	}
	return nil
}

var _ shield.ProfileRepository = (*mockProfileRepo)(nil)

// @sk-test 40-profiles-api#T4.2: TestCreateProfile (AC-001, AC-011)
func TestCreateProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	body := map[string]interface{}{
		"slug": "test-profile",
		"name": "Test Profile",
		"dictionaries": []map[string]interface{}{
			{
				"name":      "blocklist",
				"entries":   []string{"foo", "bar"},
				"match_mode": "exact",
			},
		},
		"preprocessors": []map[string]interface{}{
			{
				"name": "csv-pp",
				"type": "csv",
				"rules": []map[string]interface{}{
					{"columns": []string{"email"}, "mask": "full"},
				},
			},
		},
	}

	w := performPost(t, handler, "/api/v1/profiles", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.ProfileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %s", err)
	}

	if resp.Slug != "test-profile" {
		t.Errorf("expected slug test-profile, got %s", resp.Slug)
	}
	if resp.Name != "Test Profile" {
		t.Errorf("expected name Test Profile, got %s", resp.Name)
	}
	if resp.ID == "" {
		t.Error("expected non-empty id")
	}
	if resp.Status != "active" {
		t.Errorf("expected status active, got %s", resp.Status)
	}
	if len(resp.Dictionaries) != 1 {
		t.Fatalf("expected 1 dictionary, got %d", len(resp.Dictionaries))
	}
	if resp.Dictionaries[0].Name != "blocklist" {
		t.Errorf("expected dictionary name blocklist, got %s", resp.Dictionaries[0].Name)
	}
	if len(resp.Preprocessors) != 1 {
		t.Fatalf("expected 1 preprocessor, got %d", len(resp.Preprocessors))
	}
	if resp.CreatedAt == "" || resp.UpdatedAt == "" {
		t.Error("expected timestamps")
	}
}

// @sk-test 40-profiles-api#T4.2: TestCreateProfileDuplicateSlug (AC-002)
func TestCreateProfileDuplicateSlug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	// Create first profile
	body := map[string]interface{}{
		"slug": "my-profile",
		"name": "First",
	}
	w := performPost(t, handler, "/api/v1/profiles", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Create duplicate
	w = performPost(t, handler, "/api/v1/profiles", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var errResp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error: %s", err)
	}
	if errResp.Code != "SLUG_CONFLICT" {
		t.Errorf("expected SLUG_CONFLICT, got %s", errResp.Code)
	}
}

// @sk-test 40-profiles-api#T4.2: TestCreateProfileValidationError (AC-003)
func TestCreateProfileValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	body := map[string]interface{}{
		"slug": "",
		"name": "",
	}

	w := performPost(t, handler, "/api/v1/profiles", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error: %s", err)
	}
	if errResp.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", errResp.Code)
	}
	if len(errResp.Details) == 0 {
		t.Error("expected validation details")
	}
}

// @sk-test 40-profiles-api#T4.2: TestListProfiles (AC-004)
func TestListProfiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	// Create 3 profiles
	for i := 0; i < 3; i++ {
		body := map[string]interface{}{
			"slug": fmt.Sprintf("profile-%d", i),
			"name": fmt.Sprintf("Profile %d", i),
		}
		w := performPost(t, handler, "/api/v1/profiles", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create profile %d: %d", i, w.Code)
		}
	}

	w := performGet(t, handler, "/api/v1/profiles")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []dto.ProfileListItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Slug == "" || item.Name == "" || item.Status == "" {
			t.Errorf("expected non-empty fields, got %+v", item)
		}
	}
}

// @sk-test 40-profiles-api#T4.2: TestGetProfileBySlug (AC-005, AC-011)
func TestGetProfileBySlug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	createBody := map[string]interface{}{
		"slug": "my-profile",
		"name": "My Profile",
		"dictionaries": []map[string]interface{}{
			{
				"name":       "blocklist",
				"entries":    []string{"secret"},
				"match_mode": "exact",
			},
		},
		"preprocessors": []map[string]interface{}{
			{
				"name": "json-pp",
				"type": "json",
				"rules": []map[string]interface{}{
					{"path": "$.email", "mask": "full"},
				},
			},
		},
	}
	w := performPost(t, handler, "/api/v1/profiles", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	w = performGet(t, handler, "/api/v1/profiles/my-profile")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.ProfileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if resp.Slug != "my-profile" {
		t.Errorf("expected slug my-profile, got %s", resp.Slug)
	}
	if len(resp.Dictionaries) != 1 || resp.Dictionaries[0].Name != "blocklist" {
		t.Error("expected dictionary blocklist")
	}
	if len(resp.Preprocessors) != 1 || resp.Preprocessors[0].Name != "json-pp" {
		t.Error("expected preprocessor json-pp")
	}
}

// @sk-test 40-profiles-api#T4.2: TestGetProfileNotFound (AC-006)
func TestGetProfileNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	w := performGet(t, handler, "/api/v1/profiles/nonexistent")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error: %s", err)
	}
	if errResp.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", errResp.Code)
	}
}

// @sk-test 40-profiles-api#T4.2: TestUpdateProfile (AC-007, AC-011)
func TestUpdateProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	createBody := map[string]interface{}{
		"slug": "my-profile",
		"name": "Original",
		"dictionaries": []map[string]interface{}{
			{
				"name":       "dict1",
				"entries":    []string{"old"},
				"match_mode": "exact",
			},
		},
		"preprocessors": []map[string]interface{}{
			{
				"name": "csv-pp",
				"type": "csv",
				"rules": []map[string]interface{}{
					{"columns": []string{"email"}, "mask": "full"},
				},
			},
		},
	}
	w := performPost(t, handler, "/api/v1/profiles", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	updateBody := map[string]interface{}{
		"name":          "Updated",
		"dictionaries":  []map[string]interface{}{},
		"preprocessors": []map[string]interface{}{},
	}

	w = performPut(t, handler, "/api/v1/profiles/my-profile", updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.ProfileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if resp.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", resp.Name)
	}
	if len(resp.Dictionaries) != 0 {
		t.Errorf("expected 0 dictionaries, got %d", len(resp.Dictionaries))
	}
	if len(resp.Preprocessors) != 0 {
		t.Errorf("expected 0 preprocessors, got %d", len(resp.Preprocessors))
	}
}

// @sk-test 40-profiles-api#T4.2: TestDeleteProfile (AC-008)
func TestDeleteProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	createBody := map[string]interface{}{
		"slug": "my-profile",
		"name": "To Delete",
	}
	w := performPost(t, handler, "/api/v1/profiles", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	w = performDelete(t, handler, "/api/v1/profiles/my-profile")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = performGet(t, handler, "/api/v1/profiles/my-profile")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

// @sk-test 40-profiles-api#T4.2: TestPatchDictionaryAdd (AC-009)
func TestPatchDictionaryAdd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	createBody := map[string]interface{}{
		"slug": "my-profile",
		"name": "Test",
		"dictionaries": []map[string]interface{}{
			{
				"name":       "default",
				"entries":    []string{"foo"},
				"match_mode": "exact",
			},
		},
	}
	w := performPost(t, handler, "/api/v1/profiles", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	patchBody := map[string]interface{}{
		"action":  "add",
		"name":    "default",
		"entries": []string{"bar"},
	}

	w = performPatch(t, handler, "/api/v1/profiles/my-profile/dictionary", patchBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.ProfileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if len(resp.Dictionaries) != 1 {
		t.Fatalf("expected 1 dictionary, got %d", len(resp.Dictionaries))
	}
	entries := resp.Dictionaries[0].Entries
	if len(entries) != 2 || entries[0] != "foo" || entries[1] != "bar" {
		t.Errorf("expected [foo bar], got %v", entries)
	}
}

// @sk-test 40-profiles-api#T4.2: TestPatchDictionaryRemove (AC-010)
func TestPatchDictionaryRemove(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupTestHandler(t)

	createBody := map[string]interface{}{
		"slug": "my-profile",
		"name": "Test",
		"dictionaries": []map[string]interface{}{
			{
				"name":       "default",
				"entries":    []string{"foo", "bar"},
				"match_mode": "exact",
			},
		},
	}
	w := performPost(t, handler, "/api/v1/profiles", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	patchBody := map[string]interface{}{
		"action":  "remove",
		"name":    "default",
		"entries": []string{"bar"},
	}

	w = performPatch(t, handler, "/api/v1/profiles/my-profile/dictionary", patchBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.ProfileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if len(resp.Dictionaries) != 1 {
		t.Fatalf("expected 1 dictionary, got %d", len(resp.Dictionaries))
	}
	entries := resp.Dictionaries[0].Entries
	if len(entries) != 1 || entries[0] != "foo" {
		t.Errorf("expected [foo], got %v", entries)
	}
}

func setupTestHandler(t *testing.T) *ProfileHandler {
	t.Helper()
	return New(newMockProfileRepo())
}

func performPost(t *testing.T, h *ProfileHandler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, h, http.MethodPost, path, body)
}

func performGet(t *testing.T, h *ProfileHandler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, h, http.MethodGet, path, nil)
}

func performPut(t *testing.T, h *ProfileHandler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, h, http.MethodPut, path, body)
}

func performDelete(t *testing.T, h *ProfileHandler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, h, http.MethodDelete, path, nil)
}

func performPatch(t *testing.T, h *ProfileHandler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, h, http.MethodPatch, path, body)
}

func performRequest(t *testing.T, h *ProfileHandler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %s", err)
		}
		buf = *bytes.NewBuffer(b)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/api/v1/profiles", h.CreateProfile)
	r.GET("/api/v1/profiles", h.ListProfiles)
	r.GET("/api/v1/profiles/:slug", h.GetProfile)
	r.PUT("/api/v1/profiles/:slug", h.UpdateProfile)
	r.DELETE("/api/v1/profiles/:slug", h.DeleteProfile)
	r.PATCH("/api/v1/profiles/:slug/dictionary", h.PatchDictionary)

	r.ServeHTTP(w, req)
	return w
}
