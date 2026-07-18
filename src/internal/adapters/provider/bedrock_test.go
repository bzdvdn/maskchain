package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-test provider-adapters-expansion#T4.3: TestBedrockClient_Call — OpenAI→Bedrock conversion via InvokeModel (AC-005)
func TestBedrockClient_Call(t *testing.T) {
	mock := &mockBedrockRuntime{
		invokeFunc: func(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
			if params.ModelId == nil || *params.ModelId != "anthropic.claude-3-sonnet-20240229-v1:0" {
				t.Errorf("unexpected model: %v", params.ModelId)
			}

			var reqBody map[string]interface{}
			if err := json.Unmarshal(params.Body, &reqBody); err != nil {
				t.Fatalf("unmarshal bedrock request: %v", err)
			}

			if reqBody["anthropic_version"] != "bedrock-2023-05-31" {
				t.Errorf("expected anthropic_version=bedrock-2023-05-31, got %v", reqBody["anthropic_version"])
			}

			messages, ok := reqBody["messages"].([]interface{})
			if !ok || len(messages) != 1 {
				t.Fatalf("expected 1 message, got %+v", reqBody)
			}

			msg := messages[0].(map[string]interface{})
			if msg["role"] != "user" || msg["content"] != "Hello" {
				t.Errorf("unexpected message: %+v", msg)
			}

			if reqBody["system"] != "You are helpful" {
				t.Errorf("expected system=You are helpful, got %v", reqBody["system"])
			}

			return &bedrockruntime.InvokeModelOutput{
				Body: []byte(`{"content":[{"type":"text","text":"Hi from Bedrock"}],"stop_reason":"end_turn"}`),
			}, nil
		},
	}

	client := &BedrockClient{runtime: mock}
	resp, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"anthropic.claude-3-sonnet-20240229-v1:0","messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"Hello"}]}`),
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Body, &openAIResp); err != nil {
		t.Fatalf("unmarshal openai response: %v", err)
	}
	if len(openAIResp.Choices) != 1 || openAIResp.Choices[0].Message.Content != "Hi from Bedrock" {
		t.Errorf("expected content=Hi from Bedrock, got %+v", openAIResp)
	}
	if openAIResp.Choices[0].FinishReason == nil || *openAIResp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", openAIResp.Choices[0].FinishReason)
	}
}

// @sk-test provider-adapters-expansion#T4.3: TestBedrockClient_Stream — reading events from bedrockStreamOutput (AC-006)
func TestBedrockClient_Stream(t *testing.T) {
	mock := &mockBedrockRuntime{
		streamFunc: func(ctx context.Context, params *bedrockruntime.InvokeModelWithResponseStreamInput, optFns ...func(*bedrockruntime.Options)) (bedrockStreamOutput, error) {
			if params.ModelId == nil || *params.ModelId != "anthropic.claude-3-sonnet-20240229-v1:0" {
				t.Errorf("unexpected model: %v", params.ModelId)
			}

			return &mockStreamOutput{
				events: []types.ResponseStream{
					&types.ResponseStreamMemberChunk{
						Value: types.PayloadPart{
							Bytes: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`),
						},
					},
					&types.ResponseStreamMemberChunk{
						Value: types.PayloadPart{
							Bytes: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`),
						},
					},
					&types.ResponseStreamMemberChunk{
						Value: types.PayloadPart{
							Bytes: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
						},
					},
				},
			}, nil
		},
	}

	client := &BedrockClient{runtime: mock}
	ch, err := client.Stream(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"anthropic.claude-3-sonnet-20240229-v1:0","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var contents []string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
		if chunk.Done {
			break
		}
		var openAIChunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(chunk.Data, &openAIChunk); err != nil {
			t.Fatalf("unmarshal chunk: %v", err)
		}
		for _, c := range openAIChunk.Choices {
			contents = append(contents, c.Delta.Content)
		}
	}

	expected := []string{"Hello", " world"}
	if len(contents) < len(expected) {
		t.Errorf("expected at least %d text chunks, got %d: %v", len(expected), len(contents), contents)
	}
	for i := range expected {
		if i >= len(contents) {
			break
		}
		if contents[i] != expected[i] {
			t.Errorf("chunk %d: expected %q, got %q", i, expected[i], contents[i])
		}
	}
}

// @sk-test provider-adapters-expansion#T4.3: TestBedrockClient_Error — Bedrock API error (AC-005)
func TestBedrockClient_Error(t *testing.T) {
	mock := &mockBedrockRuntime{
		invokeFunc: func(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
			return nil, errors.New("bedrock: access denied")
		},
	}

	client := &BedrockClient{runtime: mock}
	_, err := client.Call(context.Background(), &ports.ProviderRequest{
		Body: []byte(`{"model":"anthropic.claude-3-sonnet-20240229-v1:0","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- mocks ---

type mockBedrockRuntime struct {
	invokeFunc func(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
	streamFunc func(ctx context.Context, params *bedrockruntime.InvokeModelWithResponseStreamInput, optFns ...func(*bedrockruntime.Options)) (bedrockStreamOutput, error)
}

func (m *mockBedrockRuntime) InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, params, optFns...)
	}
	return &bedrockruntime.InvokeModelOutput{}, nil
}

func (m *mockBedrockRuntime) InvokeModelWithResponseStream(ctx context.Context, params *bedrockruntime.InvokeModelWithResponseStreamInput, optFns ...func(*bedrockruntime.Options)) (bedrockStreamOutput, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, params, optFns...)
	}
	return &mockStreamOutput{}, nil
}

type mockStreamOutput struct {
	events []types.ResponseStream
	closed bool
	err    error
}

func (m *mockStreamOutput) Events() <-chan types.ResponseStream {
	ch := make(chan types.ResponseStream, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch
}

func (m *mockStreamOutput) Close() error {
	m.closed = true
	return nil
}

func (m *mockStreamOutput) Err() error {
	return m.err
}
