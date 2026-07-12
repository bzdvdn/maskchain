package egress

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task 71-egress-streaming#T4.1: Implement SSE stream parser with graceful shutdown (AC-003)
func (c *Client) streamSSE(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.doWithRetry(ctx, req.Method, func() (*http.Response, error) {
		return c.tp.RoundTrip(httpReq)
	})
	if err != nil {
		return nil, err
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected content type: %s", ct)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	ch := make(chan ports.ProviderChunk, 10)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(ch)

		done := make(chan struct{})
		defer close(done)

		go func() {
			select {
			case <-ctx.Done():
				_ = resp.Body.Close()
			case <-done:
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					ch <- ports.ProviderChunk{Done: true}
					return
				}
				select {
				case ch <- ports.ProviderChunk{Data: []byte(data)}:
				case <-ctx.Done():
					return
				}
			} else if line == "data:" {
				select {
				case ch <- ports.ProviderChunk{Data: []byte{}}:
				case <-ctx.Done():
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- ports.ProviderChunk{Err: err}:
			case <-ctx.Done():
			}
			return
		}

		ch <- ports.ProviderChunk{Done: true}
	}()

	return ch, nil
}
