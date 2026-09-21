// Minimal MaskChain caller using only the standard library.
//
//	go run main.go
//
// Env: MASKCHAIN_URL (default http://localhost:8080/api/v1)
//
//	MASKCHAIN_KEY (default sk-test-default)
//	MASKCHAIN_MODEL (default llama3.2)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	baseURL := getenv("MASKCHAIN_URL", "http://localhost:8080/api/v1")
	apiKey := getenv("MASKCHAIN_KEY", "sk-test-default")
	model := getenv("MASKCHAIN_MODEL", "llama3.2")

	payload, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{{
			Role:    "user",
			Content: "Summarize this note: contact Alice at alice@example.com or +1-555-0100.",
		}},
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "request failed: %d %s\n", res.StatusCode, body)
		os.Exit(1)
	}

	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		panic(err)
	}
	if len(out.Choices) == 0 {
		fmt.Println(string(body))
		return
	}
	fmt.Println(out.Choices[0].Message.Content)
}
