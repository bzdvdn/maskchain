package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/adapters/egress"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task provider-adapters-expansion#T3.1: Create GeminiClient with OpenAI↔Gemini format conversion (AC-001, AC-002, AC-003)
//
// GeminiClient represents a domain entity or configuration.
type GeminiClient struct {
	baseURL string
	apiKey  string
	ec      *egress.Client
}

func newGeminiClient(cfg *config.ProviderConfig, ec *egress.Client) *GeminiClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	var apiKey string
	if len(cfg.APIKeys) > 0 {
		apiKey = cfg.APIKeys[0]
	}
	return &GeminiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		ec:      ec,
	}
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason *string       `json:"finish_reason,omitempty"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason,omitempty"`
}

type openAIDelta struct {
	Content string `json:"content,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiPart     `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiStreamChunk struct {
	Candidates []geminiStreamCandidate `json:"candidates"`
}

type geminiStreamCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason *string       `json:"finishReason,omitempty"`
}

func (c *GeminiClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	openAIReq, err := parseOpenAIRequest(req.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: parse request: %w", err)
	}

	geminiReq := convertToGemini(openAIReq)
	body, _ := json.Marshal(geminiReq)

	geminiURL := fmt.Sprintf("%s/v1/models/%s:generateContent?key=%s", c.baseURL, openAIReq.Model, c.apiKey)
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    geminiURL,
		Body:   body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	resp, err := c.ec.Call(ctx, providerReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		perr := ParseProviderError(resp.StatusCode, resp.Body, "gemini")
		resp.Body, _ = json.Marshal(perr)
		return resp, nil
	}

	openAIResp, err := convertFromGemini(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: convert response: %w", err)
	}

	resp.Body = openAIResp
	return resp, nil
}

func (c *GeminiClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	openAIReq, err := parseOpenAIRequest(req.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: parse request: %w", err)
	}

	geminiReq := convertToGemini(openAIReq)
	body, _ := json.Marshal(geminiReq)

	geminiURL := fmt.Sprintf("%s/v1/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, openAIReq.Model, c.apiKey)
	providerReq := &ports.ProviderRequest{
		Method: "POST",
		URL:    geminiURL,
		Body:   body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	ch, err := c.ec.Stream(ctx, providerReq)
	if err != nil {
		return nil, fmt.Errorf("gemini stream: %w", err)
	}

	out := make(chan ports.ProviderChunk)
	go func() {
		defer close(out)
		for chunk := range ch {
			if chunk.Err != nil {
				out <- chunk
				return
			}
			if chunk.Done {
				out <- chunk
				return
			}
			converted, err := convertGeminiStreamChunk(chunk.Data)
			if err != nil {
				out <- ports.ProviderChunk{Err: fmt.Errorf("gemini: convert stream chunk: %w", err)}
				return
			}
			for _, c := range converted {
				out <- c
			}
		}
	}()
	return out, nil
}

func parseOpenAIRequest(body []byte) (*openAIRequest, error) {
	var req openAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func convertToGemini(openAIReq *openAIRequest) *geminiRequest {
	var system *geminiPart
	contents := make([]geminiContent, 0, len(openAIReq.Messages))

	for _, msg := range openAIReq.Messages {
		if msg.Role == "system" {
			system = &geminiPart{Text: msg.Content}
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	g := &geminiRequest{
		Contents: contents,
	}
	if system != nil {
		g.SystemInstruction = system
	}
	return g
}

func convertFromGemini(body []byte) ([]byte, error) {
	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, err
	}

	openAIResp := openAIResponse{
		Choices: make([]openAIChoice, 0, len(geminiResp.Candidates)),
	}
	for _, c := range geminiResp.Candidates {
		var text string
		for _, p := range c.Content.Parts {
			text += p.Text
		}
		finishReason := geminiFinishToOpenAI(c.FinishReason)
		openAIResp.Choices = append(openAIResp.Choices, openAIChoice{
			Message: openAIMessage{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: finishReason,
		})
	}

	return json.Marshal(openAIResp)
}

func convertGeminiStreamChunk(data []byte) ([]ports.ProviderChunk, error) {
	var geminiChunk geminiStreamChunk
	if err := json.Unmarshal(data, &geminiChunk); err != nil {
		return nil, err
	}

	chunks := make([]ports.ProviderChunk, 0, len(geminiChunk.Candidates))
	for _, c := range geminiChunk.Candidates {
		var text string
		for _, p := range c.Content.Parts {
			text += p.Text
		}
		chunkData, _ := json.Marshal(openAIStreamChunk{
			Choices: []openAIStreamChoice{
				{
					Delta:        openAIDelta{Content: text},
					FinishReason: geminiFinishToOpenAI(derefStr(c.FinishReason)),
				},
			},
		})
		chunks = append(chunks, ports.ProviderChunk{Data: chunkData})
	}

	// No chunks → passthrough (might be a keepalive)
	if len(chunks) == 0 {
		return []ports.ProviderChunk{{Data: data}}, nil
	}

	return chunks, nil
}

func geminiFinishToOpenAI(finish string) *string {
	switch finish {
	case "":
		return nil
	case "STOP":
		s := "stop"
		return &s
	default:
		s := strings.ToLower(finish)
		return &s
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Ensure io, bytes are used (for future streaming helpers)
var _ = io.Discard
var _ = bytes.MinRead
