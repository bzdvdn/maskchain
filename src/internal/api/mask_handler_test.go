package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

type mockDetector struct {
	lastText string
}

func (m *mockDetector) Scan(_ context.Context, text string) ([]detector.DetectorResult, error) {
	m.lastText = text
	return nil, nil
}

type mockStorage struct {
	mu      sync.Mutex
	entries map[string]*mask.MaskEntry
	getErr  error
	saveErr error
}

func (m *mockStorage) Save(_ context.Context, entry *mask.MaskEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.entries == nil {
		m.entries = make(map[string]*mask.MaskEntry)
	}
	m.entries[entry.MaskID] = entry
	return nil
}

func (m *mockStorage) Get(_ context.Context, id string) (*mask.MaskEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.entries == nil {
		return nil, mask.ErrMaskNotFound
	}
	entry, ok := m.entries[id]
	if !ok {
		return nil, mask.ErrMaskNotFound
	}
	return entry, nil
}

func (m *mockStorage) Delete(_ context.Context, _ string) error { return nil }

// @sk-test 25-shield-preprocessors#T4.1: TestPreprocessorPipeline (AC-008)
func TestPreprocessorPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mock detector
	md := &mockDetector{}
	reg := detector.NewDetectorRegistry()
	if err := reg.Register(entity.DetectorType("test"), md); err != nil {
		t.Fatal(err)
	}

	// Setup use case
	uc := mask.NewMaskUseCase(reg, &mockStorage{})

	// Setup preprocessor
	csvPP, err := preprocessor.NewPreprocessor(preprocessor.PreprocessorDef{
		Name: "csv-mask",
		Type: "csv",
		Rules: []preprocessor.Rule{
			{Columns: []string{"email"}, Mask: preprocessor.MaskModeFull},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Setup handler
	handler := NewMaskHandler(uc, reg)
	handler.WithPreprocessors([]preprocessor.Processor{csvPP})

	// Setup server
	engine := gin.New()
	engine.POST("/mask", handler.HandleMask)

	// Send request with CSV data
	csvData := "name,email\nJohn,john@test.com\nJane,jane@test.com"
	body := bytes.NewBufferString(csvData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/mask", body)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify mock detector received masked text
	if md.lastText == "" {
		t.Fatal("detector was not called")
	}
	if strings.Contains(md.lastText, "john@test.com") {
		t.Error("detector received unmasked email: john@test.com")
	}
	if strings.Contains(md.lastText, "jane@test.com") {
		t.Error("detector received unmasked email: jane@test.com")
	}
	if !strings.Contains(md.lastText, "[MASK_csv.") {
		t.Error("detector should receive text with CSV placeholders")
	}
}

// @sk-test 25-shield-preprocessors#T4.1: TestNoPreprocessorPassthrough (AC-008)
func TestNoPreprocessorPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	md := &mockDetector{}
	reg := detector.NewDetectorRegistry()
	if err := reg.Register(entity.DetectorType("test"), md); err != nil {
		t.Fatal(err)
	}
	uc := mask.NewMaskUseCase(reg, &mockStorage{})

	handler := NewMaskHandler(uc, reg)
	engine := gin.New()
	engine.POST("/mask", handler.HandleMask)

	body := bytes.NewBufferString("plain text")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/mask", body)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if md.lastText != "plain text" {
		t.Errorf("expected detector to receive original text, got %q", md.lastText)
	}
}

// @sk-test 117-critical-test-coverage#T2.2: TestHandleUnmask (AC-007)
func TestHandleUnmask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		ms := &mockStorage{}
		reg := detector.NewDetectorRegistry()
		uc := mask.NewMaskUseCase(reg, ms)
		handler := NewMaskHandler(uc, reg)

		origText := "hello world"
		maskEntry := &mask.MaskEntry{
			MaskID:       "test-mask-1",
			Replacements: map[string]string{"[MASK_test.1]": "world"},
		}
		if err := ms.Save(nil, maskEntry); err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shield/unmask?mask_ids=test-mask-1",
			strings.NewReader("hello [MASK_test.1]"))
		c.Request.Header.Set("Content-Type", "text/plain")
		handler.HandleUnmask(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != origText {
			t.Errorf("expected %q, got %q", origText, w.Body.String())
		}
	})

	t.Run("not_found", func(t *testing.T) {
		ms := &mockStorage{}
		reg := detector.NewDetectorRegistry()
		uc := mask.NewMaskUseCase(reg, ms)
		handler := NewMaskHandler(uc, reg)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shield/unmask?mask_ids=nonexistent",
			strings.NewReader("some text"))
		handler.HandleUnmask(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("storage_error", func(t *testing.T) {
		ms := &mockStorage{getErr: fmt.Errorf("connection refused")}
		reg := detector.NewDetectorRegistry()
		uc := mask.NewMaskUseCase(reg, ms)
		handler := NewMaskHandler(uc, reg)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shield/unmask?mask_ids=any",
			strings.NewReader("some text"))
		handler.HandleUnmask(c)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}

type cycleDetector struct{}

func (d *cycleDetector) Scan(_ context.Context, text string) ([]detector.DetectorResult, error) {
	idx := strings.Index(text, "test@example.com")
	if idx < 0 {
		return nil, nil
	}
	return []detector.DetectorResult{
		{
			DetectorType: "email",
			Fragment:     "test@example.com",
			StartPos:     idx,
			EndPos:       idx + len("test@example.com"),
			Confidence:   1.0,
		},
	}, nil
}

// @sk-test 117-critical-test-coverage#T2.3: TestMaskUnmaskCycle (AC-002)
func TestMaskUnmaskCycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	md := &cycleDetector{}
	reg := detector.NewDetectorRegistry()
	if err := reg.Register(entity.DetectorType("test"), md); err != nil {
		t.Fatal(err)
	}
	ms := &mockStorage{}
	uc := mask.NewMaskUseCase(reg, ms)
	handler := NewMaskHandler(uc, reg)

	engine := gin.New()
	engine.POST("/api/v1/shield/mask", handler.HandleMask)
	engine.POST("/api/v1/shield/unmask", handler.HandleUnmask)

	origBody := "my email is test@example.com"
	body := bytes.NewBufferString(origBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shield/mask", body)
	engine.ServeHTTP(w, req)

	maskedText := w.Body.String()
	if maskedText == origBody {
		t.Fatal("masked text should differ from original")
	}
	if !strings.Contains(maskedText, "[MASK_") {
		t.Fatal("masked text should contain placeholders")
	}

	maskID := w.Header().Get("mask-id")
	docMaskID := w.Header().Get("data_mask_id")
	if maskID == "" {
		maskID = docMaskID
	}
	if maskID == "" {
		t.Fatal("expected mask-id or data_mask_id header")
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/shield/unmask?mask_ids="+maskID,
		strings.NewReader(maskedText))
	engine.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("unmask expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	if w2.Body.String() != origBody {
		t.Errorf("unmasked text should match original\n  got:  %q\n  want: %q", w2.Body.String(), origBody)
	}
}

// @sk-test 117-critical-test-coverage#T3.3: TestHandleMaskStorageError (AC-008)
func TestHandleMaskStorageError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ms := &mockStorage{saveErr: fmt.Errorf("disk full")}
	reg := detector.NewDetectorRegistry()
	uc := mask.NewMaskUseCase(reg, ms)
	handler := NewMaskHandler(uc, reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shield/mask",
		strings.NewReader("some text"))
	c.Request.Header.Set("Content-Type", "text/plain")
	handler.HandleMask(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
