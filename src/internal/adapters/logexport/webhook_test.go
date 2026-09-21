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

// @sk-test log-export#T4.3: webhook deliveries are signed (AC-005)
func TestWebhookSinkSignsAndDelivers(t *testing.T) {
	var gotBody []byte
	var gotSig, gotEvent, gotTS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get(SignatureHeader)
		gotEvent = r.Header.Get("X-MaskChain-Event")
		gotTS = r.Header.Get("X-MaskChain-Timestamp")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewWebhookSink("wh", srv.URL, "s3cr3t", 0)
	recs := []domainlogexport.Record{{
		EventID: "e1",
		Tenant:  "alpha",
		Model:   "m",
		Content: &domainlogexport.Content{MaskedRequest: "email [MASK.0]", MaskedResponse: "ok"},
	}}
	if err := sink.Send(context.Background(), recs); err != nil {
		t.Fatalf("send: %v", err)
	}

	if gotSig != SignPayload("s3cr3t", gotBody) {
		t.Errorf("signature mismatch: got %q", gotSig)
	}
	if gotEvent == "" || gotTS == "" {
		t.Errorf("expected event id and timestamp headers, got %q / %q", gotEvent, gotTS)
	}
	if !bytes.Contains(gotBody, []byte("[MASK.0]")) {
		t.Errorf("masked content missing from payload: %s", gotBody)
	}
	if bytes.Contains(gotBody, []byte("a@b.com")) {
		t.Error("original value leaked into payload")
	}
}

// @sk-test log-export#T4.3: non-2xx webhook responses are errors (AC-005)
func TestWebhookSinkErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink := NewWebhookSink("wh", srv.URL, "s", 0)
	if err := sink.Send(context.Background(), []domainlogexport.Record{{EventID: "e1"}}); err == nil {
		t.Error("expected an error for a 500 response")
	}
}
