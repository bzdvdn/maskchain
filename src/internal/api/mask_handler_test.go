package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

type mockStorage struct{}

func (m *mockStorage) Save(_ context.Context, _ *mask.MaskEntry) error { return nil }
func (m *mockStorage) Get(_ context.Context, _ string) (*mask.MaskEntry, error) { return nil, nil }
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
	if !strings.Contains(md.lastText, "{{csv.") {
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
