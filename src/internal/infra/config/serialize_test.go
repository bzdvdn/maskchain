package config

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestProviderConfigLogValue_MasksAPIKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := ProviderConfig{
		Name:       "test",
		APIKeys:    []string{"sk-secret-1", "sk-secret-2"},
		AuthScheme: "bearer",
		AuthHeader: "Authorization",
	}
	logger.LogAttrs(context.Background(), slog.LevelDebug, "config", slog.Any("provider", cfg))
	output := buf.String()

	if bytes.Contains(buf.Bytes(), []byte("sk-secret-1")) {
		t.Errorf("expected APIKey to be masked, found 'sk-secret-1' in output:\n%s", output)
	}
	if bytes.Contains(buf.Bytes(), []byte("sk-secret-2")) {
		t.Errorf("expected APIKey to be masked, found 'sk-secret-2' in output:\n%s", output)
	}
}

func TestProviderConfigLogValue_AWSFieldsIncluded(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := ProviderConfig{
		Name:               "bedrock",
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIA123",
		AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	logger.LogAttrs(context.Background(), slog.LevelDebug, "config", slog.Any("provider", cfg))
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("us-east-1")) {
		t.Errorf("expected aws_region in output:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("AKIA123")) {
		t.Errorf("expected aws_access_key_id in output:\n%s", output)
	}
	if bytes.Contains([]byte(output), []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")) {
		t.Errorf("expected AWS secret access key to be masked:\n%s", output)
	}
}

func TestProviderConfigLogValue_NoAWSFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := ProviderConfig{
		Name: "generic",
	}
	logger.LogAttrs(context.Background(), slog.LevelDebug, "config", slog.Any("provider", cfg))
	output := buf.String()

	if bytes.Contains([]byte(output), []byte("aws_region")) {
		t.Errorf("expected no aws_region in output:\n%s", output)
	}
}
