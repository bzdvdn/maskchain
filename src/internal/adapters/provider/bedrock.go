package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/ports"
)

// @sk-task provider-adapters-expansion#T4.2: Create BedrockClient with SigV4 and InvokeModel/Stream (AC-004, AC-005, AC-006)
type bedrockStreamOutput interface {
	Events() <-chan types.ResponseStream
	Close() error
	Err() error
}

type bedrockRuntimeIface interface {
	InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
	InvokeModelWithResponseStream(ctx context.Context, params *bedrockruntime.InvokeModelWithResponseStreamInput, optFns ...func(*bedrockruntime.Options)) (bedrockStreamOutput, error)
}

// @sk-task provider-adapters-expansion#T4.1: Add AWS SDK v2 dependency for Bedrock SigV4 (AC-008)
type bedrockRuntimeClient struct {
	inner *bedrockruntime.Client
}

func (c *bedrockRuntimeClient) InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	return c.inner.InvokeModel(ctx, params, optFns...)
}

func (c *bedrockRuntimeClient) InvokeModelWithResponseStream(ctx context.Context, params *bedrockruntime.InvokeModelWithResponseStreamInput, optFns ...func(*bedrockruntime.Options)) (bedrockStreamOutput, error) {
	out, err := c.inner.InvokeModelWithResponseStream(ctx, params, optFns...)
	if err != nil {
		return nil, err
	}
	return out.GetStream(), nil
}

var _ bedrockRuntimeIface = (*bedrockRuntimeClient)(nil)

type BedrockClient struct {
	runtime bedrockRuntimeIface
}

func newBedrockClient(cfg *config.ProviderConfig) (*BedrockClient, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
	}
	if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}

	return &BedrockClient{
		runtime: &bedrockRuntimeClient{inner: bedrockruntime.NewFromConfig(awsCfg)},
	}, nil
}

type bedrockRequest struct {
	AnthropicVersion string            `json:"anthropic_version"`
	MaxTokens        int               `json:"max_tokens"`
	System           string            `json:"system,omitempty"`
	Messages         []bedrockMessage  `json:"messages"`
}

type bedrockMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type bedrockResponse struct {
	Content    []bedrockContentBlock `json:"content"`
	StopReason string                `json:"stop_reason"`
}

type bedrockContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type bedrockStreamChunk struct {
	Type         string                `json:"type"`
	Index        *int                  `json:"index,omitempty"`
	Delta        *bedrockStreamDelta   `json:"delta,omitempty"`
	MessageDelta *bedrockMessageDelta  `json:"message_delta,omitempty"`
}

type bedrockStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type bedrockMessageDelta struct {
	StopReason string `json:"stop_reason"`
}

func (c *BedrockClient) Call(ctx context.Context, req *ports.ProviderRequest) (*ports.ProviderResponse, error) {
	openAIReq, err := parseOpenAIRequest(req.Body)
	if err != nil {
		return nil, fmt.Errorf("bedrock: parse request: %w", err)
	}

	bedrockBody, err := json.Marshal(convertToBedrock(openAIReq))
	if err != nil {
		return nil, fmt.Errorf("bedrock: marshal request: %w", err)
	}

	modelID := openAIReq.Model
	if modelID == "" {
		return nil, fmt.Errorf("bedrock: model is required")
	}

	output, err := c.runtime.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        bedrockBody,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock: invoke model: %w", err)
	}

	openAIResp, err := convertFromBedrock(output.Body)
	if err != nil {
		return nil, fmt.Errorf("bedrock: convert response: %w", err)
	}

	return &ports.ProviderResponse{
		StatusCode: 200,
		Body:       openAIResp,
	}, nil
}

func (c *BedrockClient) Stream(ctx context.Context, req *ports.ProviderRequest) (<-chan ports.ProviderChunk, error) {
	openAIReq, err := parseOpenAIRequest(req.Body)
	if err != nil {
		return nil, fmt.Errorf("bedrock: parse request: %w", err)
	}

	bedrockBody, err := json.Marshal(convertToBedrock(openAIReq))
	if err != nil {
		return nil, fmt.Errorf("bedrock: marshal request: %w", err)
	}

	modelID := openAIReq.Model
	if modelID == "" {
		return nil, fmt.Errorf("bedrock: model is required")
	}

	stream, err := c.runtime.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelID),
		Body:        bedrockBody,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock: invoke stream: %w", err)
	}

	out := make(chan ports.ProviderChunk)

	go func() {
		defer close(out)
		defer stream.Close()

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				chunks := convertBedrockStreamChunk(v.Value.Bytes)
				for _, c := range chunks {
					out <- c
				}
			default:
				out <- ports.ProviderChunk{
					Err: fmt.Errorf("bedrock: unexpected stream event type %T", event),
				}
				return
			}
		}

		if err := stream.Err(); err != nil {
			out <- ports.ProviderChunk{Err: fmt.Errorf("bedrock: stream error: %w", err)}
		}
	}()

	return out, nil
}

func convertToBedrock(openAIReq *openAIRequest) *bedrockRequest {
	var system string
	var messages []bedrockMessage

	for _, msg := range openAIReq.Messages {
		if msg.Role == "system" {
			if system != "" {
				system += "\n" + msg.Content
			} else {
				system = msg.Content
			}
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, bedrockMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	return &bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
		System:           system,
		Messages:         messages,
	}
}

func convertFromBedrock(body []byte) ([]byte, error) {
	var resp bedrockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var text string
	for _, block := range resp.Content {
		text += block.Text
	}

	finishReason := bedrockFinishToOpenAI(resp.StopReason)

	openAIResp := openAIResponse{
		Choices: []openAIChoice{
			{
				Message: openAIMessage{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: finishReason,
			},
		},
	}

	return json.Marshal(openAIResp)
}

func convertBedrockStreamChunk(data []byte) []ports.ProviderChunk {
	var chunk bedrockStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return []ports.ProviderChunk{{Err: fmt.Errorf("bedrock: parse stream chunk: %w", err)}}
	}

	switch chunk.Type {
	case "content_block_delta":
		if chunk.Delta != nil && chunk.Delta.Type == "text_delta" && chunk.Delta.Text != "" {
			chunkData, _ := json.Marshal(openAIStreamChunk{
				Choices: []openAIStreamChoice{
					{
						Delta: openAIDelta{Content: chunk.Delta.Text},
					},
				},
			})
			return []ports.ProviderChunk{{Data: chunkData}}
		}
	case "message_delta":
		if chunk.MessageDelta != nil && chunk.MessageDelta.StopReason != "" {
			chunkData, _ := json.Marshal(openAIStreamChunk{
				Choices: []openAIStreamChoice{
					{
						Delta:        openAIDelta{},
						FinishReason: bedrockFinishToOpenAI(chunk.MessageDelta.StopReason),
					},
				},
			})
			return []ports.ProviderChunk{{Data: chunkData}}
		}
	}

	return nil
}

func bedrockFinishToOpenAI(finish string) *string {
	switch finish {
	case "":
		return nil
	case "end_turn", "stop_sequence":
		s := "stop"
		return &s
	case "max_tokens":
		s := "length"
		return &s
	default:
		s := strings.ToLower(finish)
		return &s
	}
}


