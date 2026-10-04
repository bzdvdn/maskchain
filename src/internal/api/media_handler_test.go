package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/routing"
	routingSvc "github.com/bzdvdn/maskchain/src/internal/domain/routing/service"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

type mediaMockClient struct {
	status   int
	body     []byte
	headers  map[string]string
	err      error
	captured *ports.ProviderRequest
}

func (m *mediaMockClient) Call(_ context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	m.captured = req
	if m.err != nil {
		return nil, m.err
	}
	return &ports.ProviderResponse{StatusCode: m.status, Body: m.body, Headers: m.headers}, nil
}

func (m *mediaMockClient) Stream(_ context.Context, _ *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	ch := make(chan ports.ProviderChunk, 1)
	ch <- ports.ProviderChunk{Done: true}
	close(ch)
	return ch, nil
}

func newMediaTestHandler(t *testing.T, model string, mock *mediaMockClient) *MediaHandler {
	t.Helper()
	reg, err := routingSvc.NewProviderRegistry(&routing.RoutingConfig{
		Providers: []routing.ProviderConfig{{Name: "openai", BaseURL: "http://openai"}},
		Rules: []routing.RuleConfig{{Tenant: routing.GlobalTenant, Routes: []routing.RouteConfig{
			{Model: model, Providers: []string{"openai"}},
		}}},
	})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	sel := routingSvc.NewRouteSelector(reg)
	fb := routingSvc.NewFallbackHandler(map[string]ports.ProviderClient{"openai": mock})
	return NewMediaHandler(sel, fb)
}

func doMedia(t *testing.T, h *MediaHandler, path, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	h.HandleMedia(c)
	return w
}

// @sk-test media-endpoints#T2.2: JSON generation is proxied unchanged (AC-001, AC-006)
func TestMediaHandlerImageGeneration(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK, body: []byte(`{"data":[{"url":"http://img"}]}`), headers: map[string]string{"Content-Type": "application/json"}}
	h := newMediaTestHandler(t, "img", mock)

	body := []byte(`{"model":"img","prompt":"a photo of alice@example.com"}`)
	w := doMedia(t, h, "/v1/images/generations", "application/json", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"data":[{"url":"http://img"}]}` {
		t.Errorf("body = %s", w.Body.String())
	}
	if w.Header().Get("X-Provider") != "openai" {
		t.Errorf("X-Provider = %q", w.Header().Get("X-Provider"))
	}
	if mock.captured == nil || mock.captured.URL != "/v1/images/generations" {
		t.Fatalf("upstream URL = %+v", mock.captured)
	}
	if !bytes.Equal(mock.captured.Body, body) {
		t.Errorf("prompt/body was modified before the provider call")
	}
	if mock.captured.ContentType != "application/json" {
		t.Errorf("ContentType = %q", mock.captured.ContentType)
	}
}

// @sk-test media-endpoints#T2.2: binary speech response is passed through (AC-003, AC-010)
func TestMediaHandlerSpeechBinary(t *testing.T) {
	audio := []byte{0x49, 0x44, 0x33, 0x04, 0x00}
	mock := &mediaMockClient{status: http.StatusOK, body: audio, headers: map[string]string{"Content-Type": "audio/mpeg"}}
	h := newMediaTestHandler(t, "tts", mock)

	w := doMedia(t, h, "/v1/audio/speech", "application/json", []byte(`{"model":"tts","input":"hi"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), audio) {
		t.Errorf("binary response altered: %v", w.Body.Bytes())
	}
	if ct := w.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("Content-Type = %q, want audio/mpeg", ct)
	}
}

// @sk-test media-endpoints#T2.2: missing model is rejected before the provider (AC-005)
func TestMediaHandlerMissingModel(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK}
	h := newMediaTestHandler(t, "img", mock)

	w := doMedia(t, h, "/v1/images/generations", "application/json", []byte(`{"prompt":"x"}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if mock.captured != nil {
		t.Error("provider must not be called without a model")
	}
}

// @sk-test media-endpoints#T2.2: unrouted model returns NO_ROUTE (AC-005)
func TestMediaHandlerNoRoute(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK}
	h := newMediaTestHandler(t, "img", mock)

	w := doMedia(t, h, "/v1/images/generations", "application/json", []byte(`{"model":"unknown"}`))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "NO_ROUTE") {
		t.Fatalf("expected NO_ROUTE 400, got %d: %s", w.Code, w.Body.String())
	}
}

// @sk-test media-endpoints#T2.2: unsupported shape returns UNSUPPORTED_ENDPOINT (AC-009)
func TestMediaHandlerUnsupported(t *testing.T) {
	mock := &mediaMockClient{err: ports.ErrUnsupportedEndpoint}
	h := newMediaTestHandler(t, "tts", mock)

	w := doMedia(t, h, "/v1/audio/speech", "application/json", []byte(`{"model":"tts"}`))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "UNSUPPORTED_ENDPOINT") {
		t.Fatalf("expected UNSUPPORTED_ENDPOINT 400, got %d: %s", w.Code, w.Body.String())
	}
}

// @sk-test media-endpoints#T3.2: multipart edits body is forwarded byte-for-byte (AC-002)
func TestMediaHandlerMultipartEdits(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK, body: []byte(`{"data":[]}`), headers: map[string]string{"Content-Type": "application/json"}}
	h := newMediaTestHandler(t, "img", mock)

	ct, body := multipartBody(t, "img", "image")
	w := doMedia(t, h, "/v1/images/edits", ct, body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.captured == nil || !bytes.Equal(mock.captured.Body, body) {
		t.Error("multipart body was re-encoded before the provider call")
	}
	if mock.captured.ContentType != ct {
		t.Errorf("Content-Type = %q, want %q", mock.captured.ContentType, ct)
	}
}

// @sk-test media-endpoints#T3.2: multipart transcription is forwarded byte-for-byte (AC-004)
func TestMediaHandlerMultipartTranscription(t *testing.T) {
	mock := &mediaMockClient{status: http.StatusOK, body: []byte(`{"text":"hi"}`), headers: map[string]string{"Content-Type": "application/json"}}
	h := newMediaTestHandler(t, "stt", mock)

	ct, body := multipartBody(t, "stt", "file")
	w := doMedia(t, h, "/v1/audio/transcriptions", ct, body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(mock.captured.Body, body) {
		t.Error("multipart body was re-encoded before the provider call")
	}
	if w.Body.String() != `{"text":"hi"}` {
		t.Errorf("transcript = %s", w.Body.String())
	}
}

func multipartBody(t *testing.T, model, fileField string) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", model)
	fw, _ := mw.CreateFormFile(fileField, "a.bin")
	_, _ = fw.Write([]byte("BINARYDATA"))
	_ = mw.Close()
	return mw.FormDataContentType(), buf.Bytes()
}
