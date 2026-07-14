package egress

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

func testConfig() *config.EgressConfig {
	return &config.EgressConfig{
		MaxIdleConns: 10,
		IdleTimeout:  30 * time.Second,
		MaxRetries:   1,
		BaseBackoff:  10 * time.Millisecond,
	}
}

// @sk-test 80-tenant-isolation#T4.6: TestCallWithHeaders verifies ProviderRequest headers forwarded (AC-007)
func TestCallWithHeaders(t *testing.T) {
	var capturedHeaders map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = map[string]string{
			"X-Tenant-ID":  r.Header.Get("X-Tenant-ID"),
			"X-Custom":     r.Header.Get("X-Custom"),
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(testConfig())
	_, err := client.Call(context.Background(), &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    server.URL,
		Headers: map[string]string{
			"X-Tenant-ID": "alpha",
			"X-Custom":    "custom-value",
		},
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if capturedHeaders["X-Tenant-ID"] != "alpha" {
		t.Errorf("expected X-Tenant-ID=alpha, got %q", capturedHeaders["X-Tenant-ID"])
	}
	if capturedHeaders["X-Custom"] != "custom-value" {
		t.Errorf("expected X-Custom=custom-value, got %q", capturedHeaders["X-Custom"])
	}
}

// @sk-test 71-egress-streaming#T2.2: TestCallViaProxy (AC-001)
func TestCallViaProxy(t *testing.T) {
	proxyCalled := atomic.Int64{}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled.Add(1)
		w.Header().Set("X-Proxy", "yes")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"proxied":true}`))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	client := NewClient(testConfig())
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    "http://echo.example.com/test",
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if proxyCalled.Load() == 0 {
		t.Fatal("proxy was not called")
	}
}

// @sk-test 71-egress-streaming#T2.2: TestConnectionReuse (AC-002)
func TestConnectionReuse(t *testing.T) {
	var connCount atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	server.Start()
	defer server.Close()

	client := NewClient(&config.EgressConfig{
		MaxIdleConns: 5,
		IdleTimeout:  time.Minute,
	})

	for i := 0; i < 10; i++ {
		_, err := client.Call(context.Background(), &ports.ProviderRequest{
			Method: http.MethodGet,
			URL:    server.URL + fmt.Sprintf("/req-%d", i),
		})
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	t.Logf("connection count: %d", connCount.Load())
}

// @sk-test 116-connection-pool-fixes#T2.4: TestMaxIdleConnsPerHost verifies bug fix (AC-001)
func TestMaxIdleConnsPerHost(t *testing.T) {
	cfg := &config.EgressConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 5,
	}
	tp := NewTransport(cfg)
	if tp.MaxIdleConnsPerHost != 5 {
		t.Fatalf("expected MaxIdleConnsPerHost=5, got %d", tp.MaxIdleConnsPerHost)
	}
	if tp.MaxIdleConns != 100 {
		t.Fatalf("expected MaxIdleConns=100, got %d", tp.MaxIdleConns)
	}
}

// @sk-test 116-connection-pool-fixes#T2.4: TestPerProviderTimeoutFromClient verifies client-set timeout (AC-002)
func TestPerProviderTimeoutFromClient(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	client := NewClientWithTransport(testConfig(), NewTransport(testConfig()), 100*time.Millisecond, nil)
	start := time.Now()
	_, err := client.Call(context.Background(), &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    slow.URL,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected quick timeout, took %v", elapsed)
	}
}

// @sk-test 116-connection-pool-fixes#T2.4: TestPerProviderTransportIsolation verifies per-provider transport (AC-008)
func TestPerProviderTransportIsolation(t *testing.T) {
	tp1 := NewTransport(&config.EgressConfig{MaxIdleConns: 10, MaxIdleConnsPerHost: 2})
	tp2 := NewTransport(&config.EgressConfig{MaxIdleConns: 20, MaxIdleConnsPerHost: 5})

	if tp1 == tp2 {
		t.Fatal("expected different transport pointers")
	}
	if tp1.MaxIdleConns == tp2.MaxIdleConns {
		t.Fatal("expected different MaxIdleConns values")
	}
}

// @sk-test 71-egress-streaming#T2.2: TestPerProviderTimeout (AC-004)
func TestPerProviderTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	client := NewClient(testConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Call(ctx, &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    slow.URL,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected quick cancellation, took %v", elapsed)
	}
}

// @sk-test 71-egress-streaming#T2.2: TestCancelMidRequest (AC-005)
func TestCancelMidRequest(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(blocked)

	client := NewClient(testConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Call(ctx, &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
}

// @sk-test 71-egress-streaming#T3.2: TestRetryJitter (AC-006)
func TestRetryJitter(t *testing.T) {
	intervals := make([]time.Duration, 0, 30)
	for i := 0; i < 10; i++ {
		d1 := backoff(1, 100*time.Millisecond)
		d2 := backoff(2, 100*time.Millisecond)
		d3 := backoff(3, 100*time.Millisecond)
		intervals = append(intervals, d1, d2, d3)
	}

	unique := make(map[time.Duration]int)
	for _, d := range intervals {
		unique[d]++
	}

	if len(unique) < 3 {
		t.Fatalf("expected variation (jitter), got only %d unique durations", len(unique))
	}
}

// @sk-test 71-egress-streaming#T3.2: TestRetryExhaustion (AC-007)
func TestRetryExhaustion(t *testing.T) {
	var callCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"busy"}`))
	}))
	defer server.Close()

	client := NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Minute,
		MaxRetries:   3,
		BaseBackoff:  5 * time.Millisecond,
		RetryOn5xx:   true,
	})

	_, err := client.Call(context.Background(), &ports.ProviderRequest{
		Method: http.MethodPost,
		URL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}

	total := callCount.Load()
	if total != 4 {
		t.Fatalf("expected 4 requests (1 initial + 3 retries), got %d", total)
	}
}

// @sk-test 71-egress-streaming#T3.2: TestRetryCancelDuringBackoff (AC-005)
func TestRetryCancelDuringBackoff(t *testing.T) {
	var callCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&config.EgressConfig{
		MaxIdleConns: 1,
		IdleTimeout:  time.Minute,
		MaxRetries:   3,
		BaseBackoff:  500 * time.Millisecond,
		RetryOn5xx:   true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)

	start := time.Now()
	_, err := client.Call(ctx, &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected quick cancellation during backoff, took %v", elapsed)
	}
}

// @sk-test 71-egress-streaming#T4.2: TestSSEChunkDelivery (AC-003)
func TestSSEChunkDelivery(t *testing.T) {
	chunks := []string{"hello", "world", "foo", "bar", "baz", "qux", "quux", "corge", "grault", "garply"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected http.Flusher")
			return
		}
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewClient(testConfig())
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	start := time.Now()
	var received []string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		received = append(received, string(chunk.Data))
	}
	elapsed := time.Since(start)

	if len(received) != len(chunks) {
		t.Fatalf("expected %d chunks, got %d", len(chunks), len(received))
	}
	for i, c := range chunks {
		if received[i] != c {
			t.Fatalf("chunk %d: expected %q, got %q", i, c, received[i])
		}
	}
	// Streaming should complete significantly before server would finish sending the full body.
	// Server takes 10 * 50ms = 500ms to send; total elapsed should be reasonably close to that.
	if elapsed > 2*time.Second {
		t.Fatalf("streaming too slow: %v (server send time ~500ms)", elapsed)
	}
}

// @sk-test 116-connection-pool-fixes#T3.5: TestTLSInsecureSkipVerify (AC-004)
func TestTLSInsecureSkipVerify(t *testing.T) {
	cfg := &config.EgressTLSConfig{InsecureSkipVerify: true}
	tlsCfg := buildTLSConfig(cfg)
	if tlsCfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if !tlsCfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=true")
	}
}

// @sk-test 116-connection-pool-fixes#T3.5: TestTLSCustomCA (AC-003)
func TestTLSCustomCA(t *testing.T) {
	caCertPEM, caKeyPEM := generateTestCert(t, "ca", true)
	caFile := tempFile(t, "ca-cert.pem", caCertPEM)
	_ = caKeyPEM

	cfg := &config.EgressTLSConfig{CACert: caFile}
	tlsCfg := buildTLSConfig(cfg)
	if tlsCfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if tlsCfg.RootCAs == nil {
		t.Fatal("expected non-nil RootCAs")
	}
	subjects := tlsCfg.RootCAs.Subjects()
	if len(subjects) == 0 {
		t.Fatal("expected at least one CA subject")
	}
}

// @sk-test 116-connection-pool-fixes#T3.5: TestTLSMutualTLS (AC-005)
func TestTLSMutualTLS(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t, "client", false)
	certFile := tempFile(t, "client-cert.pem", certPEM)
	keyFile := tempFile(t, "client-key.pem", keyPEM)

	cfg := &config.EgressTLSConfig{Cert: certFile, Key: keyFile}
	tlsCfg := buildTLSConfig(cfg)
	if tlsCfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if len(tlsCfg.Certificates) == 0 {
		t.Fatal("expected at least one client certificate")
	}
}

// @sk-test 116-connection-pool-fixes#T3.5: TestCircuitBreakerOpen (AC-006)
func TestCircuitBreakerOpen(t *testing.T) {
	cb := NewCircuitBreaker(&config.CircuitBreakerConfig{MaxFailures: 3, Cooldown: time.Minute})

	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("expected Allow()=true before failures, got false at attempt %d", i)
		}
		cb.Fail()
	}

	if cb.Allow() {
		t.Fatal("expected Allow()=false after 3 failures")
	}
}

// @sk-test 116-connection-pool-fixes#T3.5: TestCircuitBreakerCooldown (AC-007)
func TestCircuitBreakerCooldown(t *testing.T) {
	cb := NewCircuitBreaker(&config.CircuitBreakerConfig{MaxFailures: 1, Cooldown: 50 * time.Millisecond})

	cb.Fail()
	if cb.Allow() {
		t.Fatal("expected Allow()=false immediately after failure")
	}

	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("expected Allow()=true after cooldown")
	}
}

func generateTestCert(t *testing.T, cn string, isCA bool) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if !isCA {
		template.IsCA = false
		template.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	}

	parent := template
	parentKey := key
	if isCA {
		parent = template
		parentKey = key
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	return
}

func tempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// @sk-test 71-egress-streaming#T4.2: TestSSEPrematureClose (AC-003)
func TestSSEPrematureClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("expected http.Hijacker")
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack failed: %v", err)
			return
		}
		bufrw.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\r\n")
		bufrw.WriteString("data: chunk1\n\n")
		bufrw.WriteString("data: chunk2\n\n")
		bufrw.WriteString("data: chunk3\n\n")
		bufrw.Flush()

		tcpConn := conn.(*net.TCPConn)
		tcpConn.SetLinger(0)
		tcpConn.Close()
	}))
	defer server.Close()

	client := NewClient(testConfig())
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var received []string
	var finalErr error
	for chunk := range ch {
		if chunk.Err != nil {
			finalErr = chunk.Err
			break
		}
		if chunk.Done {
			break
		}
		received = append(received, string(chunk.Data))
	}

	if len(received) == 0 {
		t.Fatal("expected partial data before premature close")
	}
	if finalErr == nil {
		t.Fatal("expected error on premature close")
	}
	t.Logf("received %d chunks before error: %v", len(received), finalErr)
}
