package middleware

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

func mediaAccessRequest(key *virtualkey.VirtualKey, contentType string, body []byte, captured *[]byte) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	tcKey := key
	engine.Use(func(c *gin.Context) {
		if tcKey != nil {
			c.Set(virtualKeyContextKey, tcKey)
		}
		c.Next()
	})
	engine.Use(MediaModelAccess())
	engine.POST("/api/v1/images/edits", func(c *gin.Context) {
		if captured != nil {
			b, _ := io.ReadAll(c.Request.Body)
			*captured = b
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("POST", "/api/v1/images/edits", bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	engine.ServeHTTP(w, req)
	return w
}

func multipartModelBody(t *testing.T, model string) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if model != "" {
		_ = mw.WriteField("model", model)
	}
	fw, _ := mw.CreateFormFile("image", "a.png")
	_, _ = fw.Write([]byte("PNGDATA"))
	_ = mw.Close()
	return mw.FormDataContentType(), buf.Bytes()
}

// @sk-test media-endpoints#T2.4: JSON model access still enforced (AC-005)
func TestMediaModelAccessJSON(t *testing.T) {
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), "application/json", []byte(`{"model":"img"}`), nil); w.Code != http.StatusOK {
		t.Errorf("allowed JSON model: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), "application/json", []byte(`{"model":"other"}`), nil); w.Code != http.StatusForbidden {
		t.Errorf("disallowed JSON model: expected 403, got %d", w.Code)
	}
}

// @sk-test media-endpoints#T2.4: multipart model is extracted for access (AC-005)
func TestMediaModelAccessMultipart(t *testing.T) {
	ct, body := multipartModelBody(t, "img")
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), ct, body, nil); w.Code != http.StatusOK {
		t.Errorf("allowed multipart model: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), ct, body, nil); w.Code == http.StatusForbidden {
		// sanity: same body allowed above, not forbidden
		t.Errorf("multipart model should be allowed")
	}
	ctBad, bodyBad := multipartModelBody(t, "other")
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), ctBad, bodyBad, nil); w.Code != http.StatusForbidden {
		t.Errorf("disallowed multipart model: expected 403, got %d", w.Code)
	}

	ctMissing, bodyMissing := multipartModelBody(t, "")
	if w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), ctMissing, bodyMissing, nil); w.Code != http.StatusBadRequest {
		t.Errorf("missing multipart model: expected 400, got %d", w.Code)
	}
}

// @sk-test media-endpoints#T2.4: access check does not alter the body (AC-002)
func TestMediaModelAccessPreservesBody(t *testing.T) {
	ct, body := multipartModelBody(t, "img")
	var captured []byte
	w := mediaAccessRequest(fakeKey("", "alpha", []string{"img"}, nil), ct, body, &captured)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("body was altered: got %d bytes, want %d", len(captured), len(body))
	}
}
