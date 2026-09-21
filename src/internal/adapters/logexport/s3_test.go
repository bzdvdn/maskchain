package logexport

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
)

type fakeS3 struct {
	keys   []string
	bodies map[string]string
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	key := ""
	if in.Key != nil {
		key = *in.Key
	}
	body, _ := io.ReadAll(in.Body)
	f.keys = append(f.keys, key)
	if f.bodies == nil {
		f.bodies = map[string]string{}
	}
	f.bodies[key] = string(body)
	return &s3.PutObjectOutput{}, nil
}

// @sk-test log-export#T4.3: S3 sink writes per-tenant, date-partitioned objects (AC-006)
func TestS3SinkWritesPerTenant(t *testing.T) {
	f := &fakeS3{}
	sink := NewS3Sink("s3", "bucket", "logs", f)

	recs := []domainlogexport.Record{
		{EventID: "e1", Tenant: "alpha", Content: &domainlogexport.Content{MaskedRequest: "[MASK.0]", MaskedResponse: "ok"}},
		{EventID: "e2", Tenant: "beta"},
	}
	if err := sink.Send(context.Background(), recs); err != nil {
		t.Fatalf("send: %v", err)
	}

	if len(f.keys) != 2 {
		t.Fatalf("expected 2 objects (one per tenant), got %v", f.keys)
	}
	var alphaKey string
	for _, k := range f.keys {
		if !strings.HasPrefix(k, "logs/") || !strings.HasSuffix(k, ".jsonl") {
			t.Errorf("unexpected key %q", k)
		}
		if strings.Contains(k, "/alpha/") {
			alphaKey = k
		}
	}
	if alphaKey == "" {
		t.Fatalf("no tenant-partitioned key for alpha: %v", f.keys)
	}
	// Key shape: logs/<tenant>/<YYYY>/<MM>/<DD>/<event>.jsonl
	if parts := strings.Split(alphaKey, "/"); len(parts) != 6 {
		t.Errorf("expected 6 path segments, got %d (%q)", len(parts), alphaKey)
	}
	if body := f.bodies[alphaKey]; !strings.Contains(body, "[MASK.0]") {
		t.Errorf("masked content missing from object: %s", body)
	}
}
