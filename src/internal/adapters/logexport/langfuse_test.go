package logexport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
)

// @sk-test log-export#T4.3: Langfuse sink posts masked input/output (AC-007)
func TestLangfuseSinkPostsMasked(t *testing.T) {
	var gotBody []byte
	var user, pass, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		user, pass, _ = r.BasicAuth()
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewLangfuseSink("lf", srv.URL, "pk", "sk", 0)
	recs := []domainlogexport.Record{{
		EventID: "e1",
		Tenant:  "alpha",
		Model:   "m",
		Content: &domainlogexport.Content{MaskedRequest: "[MASK.0]", MaskedResponse: "[MASK.1]"},
	}}
	if err := sink.Send(context.Background(), recs); err != nil {
		t.Fatalf("send: %v", err)
	}

	if path != "/api/public/ingestion" {
		t.Errorf("path = %q, want /api/public/ingestion", path)
	}
	if user != "pk" || pass != "sk" {
		t.Errorf("basic auth = %q/%q, want pk/sk", user, pass)
	}
	if !bytes.Contains(gotBody, []byte("[MASK.0]")) || !bytes.Contains(gotBody, []byte("[MASK.1]")) {
		t.Errorf("masked input/output missing: %s", gotBody)
	}
	if !bytes.Contains(gotBody, []byte("trace-create")) || !bytes.Contains(gotBody, []byte("maskchain")) {
		t.Errorf("expected a trace-create event: %s", gotBody)
	}
}
