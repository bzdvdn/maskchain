package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

func newTestAdminServer(t *testing.T) *AdminServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	srv := NewAdminServer(&config.ServerConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test-admin", nil)
	if err := srv.RegisterSwaggerUI(); err != nil {
		t.Fatalf("RegisterSwaggerUI: %v", err)
	}
	// A minimal SPA filesystem so the NoRoute fallback is active.
	fsys := fstest.MapFS{"dist/index.html": &fstest.MapFile{Data: []byte("<html>spa</html>")}}
	if err := srv.RegisterStaticFiles(fsys); err != nil {
		t.Fatalf("RegisterStaticFiles: %v", err)
	}
	return srv
}

// @sk-test ui-swagger-assets: TestSwaggerUIAssetsServed — docs page, assets, redirect and spec (AC-008)
func TestSwaggerUIAssetsServed(t *testing.T) {
	srv := newTestAdminServer(t)

	cases := []struct {
		name        string
		path        string
		wantStatus  int
		wantBodyHas string
	}{
		{"index", "/api/v1/docs/", http.StatusOK, "swagger-ui"},
		{"index no slash", "/api/v1/docs", http.StatusOK, `<base href="/api/v1/docs/">`},
		{"initializer", "/api/v1/docs/swagger-initializer.js", http.StatusOK, "SwaggerUIBundle"},
		{"css", "/api/v1/docs/swagger-ui.css", http.StatusOK, "swagger-ui"},
		{"spec", "/api/v1/openapi.yaml", http.StatusOK, "openapi:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			srv.engine.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("%s: expected %d, got %d", tc.path, tc.wantStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tc.wantBodyHas) {
				t.Errorf("%s: body does not contain %q", tc.path, tc.wantBodyHas)
			}
		})
	}
}

// @sk-test ui-swagger-assets: TestSwaggerUIDocsNoRedirect — /docs is served without a redirect (AC-008)
func TestSwaggerUIDocsNoRedirect(t *testing.T) {
	srv := newTestAdminServer(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("expected no redirect, got Location %q", loc)
	}
}

// @sk-test ui-swagger-assets: TestUnknownAPIPathReturnsNotFound — /api/... 404 is not mislabeled (AC-004)
func TestUnknownAPIPathReturnsNotFound(t *testing.T) {
	srv := newTestAdminServer(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	srv.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "NOT_FOUND") || strings.Contains(body, "INTERNAL_ERROR") {
		t.Errorf("expected NOT_FOUND envelope, got %s", body)
	}
}
