package api

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task mcp-a2a-gateway#T2.2: agent upstream relay (AC-001, AC-005, AC-006, AC-007)
//
// AgentHandler forwards a JSON-RPC request to a configured MCP or A2A upstream
// and returns the response, unmasking placeholders in the reply. SSE responses
// are relayed with per-chunk unmasking.
type AgentHandler struct {
	client      *egress.Client
	upstreamURL string
	cardURL     string
	authHeader  string
	authValue   string
}

// NewAgentHandler builds a handler for one upstream. It returns (nil, nil) when
// no upstream URL is configured (the route is not mounted).
func NewAgentHandler(upstreamURL, apiKey, authHeader, authPrefix, cardURL, timeout string, egressCfg *config.EgressConfig) (*AgentHandler, error) {
	if upstreamURL == "" {
		return nil, nil
	}
	ec := egressCfg
	if ec == nil {
		ec = &config.EgressConfig{}
	}
	tp, err := egress.NewTransport(ec, "")
	if err != nil {
		return nil, err
	}
	d := parseAgentTimeout(timeout, ec.IdleTimeout)
	client := egress.NewClientWithTransport(ec, tp, d, nil)

	if authHeader == "" {
		authHeader = "Authorization"
	}
	return &AgentHandler{
		client:      client,
		upstreamURL: upstreamURL,
		cardURL:     cardURL,
		authHeader:  authHeader,
		authValue:   authPrefix + apiKey,
	}, nil
}

func parseAgentTimeout(s string, fallback time.Duration) time.Duration {
	if s == "" {
		if fallback <= 0 {
			return 60 * time.Second
		}
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 60 * time.Second
	}
	return d
}

func (h *AgentHandler) headers() map[string]string {
	headers := map[string]string{"Content-Type": "application/json"}
	if strings.TrimSpace(h.authValue) != "" {
		headers[h.authHeader] = h.authValue
	}
	return headers
}

// Handle forwards the (already masked) JSON-RPC request to the upstream.
func (h *AgentHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	resp, err := h.client.Call(c.Request.Context(), &ports.ProviderRequest{
		Method:  http.MethodPost,
		URL:     h.upstreamURL,
		Body:    body,
		Headers: h.headers(),
	})
	if err != nil {
		respondWithError(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}

	mapping := middleware.AgentMaskFromContext(c)
	ct := headerValue(resp.Headers, "content-type")

	if strings.HasPrefix(ct, "text/event-stream") {
		h.relaySSE(c, resp.Body, mapping)
		return
	}

	c.Set(middleware.SkipEnvelopeKey, true)
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, unmaskJSONRecursive(mapping, resp.Body))
}

// relaySSE relays a buffered SSE body with per-event unmasking.
func (h *AgentHandler) relaySSE(c *gin.Context, body []byte, mapping map[string]string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Set(middleware.SkipEnvelopeKey, true)

	um := newStreamUnmasker(mapping)
	c.Stream(func(w io.Writer) bool {
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				out := um.processData([]byte(payload))
				_, _ = io.WriteString(w, "data: "+string(out)+"\n")
				continue
			}
			_, _ = io.WriteString(w, line+"\n")
		}
		if tail := um.flush(); tail != "" {
			_, _ = io.WriteString(w, tail)
		}
		return false
	})
}

// HandleAgentCard proxies the configured A2A agent card.
func (h *AgentHandler) HandleAgentCard(c *gin.Context) {
	if h.cardURL == "" {
		respondWithError(c, http.StatusNotFound, "NOT_FOUND", "agent card not configured")
		return
	}
	resp, err := h.client.Call(c.Request.Context(), &ports.ProviderRequest{
		Method:  http.MethodGet,
		URL:     h.cardURL,
		Headers: h.headers(),
	})
	if err != nil {
		respondWithError(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}
	c.Set(middleware.SkipEnvelopeKey, true)
	ct := headerValue(resp.Headers, "content-type")
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, resp.Body)
}
