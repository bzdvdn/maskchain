package middleware

import (
	"testing"
)

// @sk-test usage-accounting-integrity#T4.1: JSON usage extraction (AC-001)
func TestExtractJSONUsage(t *testing.T) {
	body := []byte(`{"id":"x","usage":{"prompt_tokens":100,"completion_tokens":50},"choices":[]}`)
	u, ok := extractJSONUsage(body)
	if !ok {
		t.Fatal("expected usage to be found")
	}
	if u.PromptTokens != 100 || u.CompletionTokens != 50 {
		t.Errorf("got %+v, want 100/50", u)
	}
}

// @sk-test usage-accounting-integrity#T4.1: JSON without usage is reported absent (AC-004)
func TestExtractJSONUsageAbsent(t *testing.T) {
	if _, ok := extractJSONUsage([]byte(`{"id":"x","choices":[]}`)); ok {
		t.Error("expected no usage")
	}
	if _, ok := extractJSONUsage([]byte(`not json`)); ok {
		t.Error("expected no usage for invalid JSON")
	}
}

// @sk-test usage-accounting-integrity#T4.1: OpenAI streaming usage from final chunk (AC-001, AC-003)
func TestSSEUsageParserOpenAI(t *testing.T) {
	p := newSSEUsageParser()
	p.feed([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	p.feed([]byte("data: {\"usage\":{\"prompt_tokens\":120,\"completion_tokens\":80},\"choices\":[]}\n\n"))
	p.feed([]byte("data: [DONE]\n\n"))

	u, ok := p.usage()
	if !ok {
		t.Fatal("expected usage from OpenAI stream")
	}
	if u.PromptTokens != 120 || u.CompletionTokens != 80 {
		t.Errorf("got %+v, want 120/80", u)
	}
}

// @sk-test usage-accounting-integrity#T4.1: Anthropic streaming usage spans message_start and message_delta (AC-001, AC-003)
func TestSSEUsageParserAnthropic(t *testing.T) {
	p := newSSEUsageParser()
	p.feed([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":200,\"output_tokens\":1}}}\n\n"))
	p.feed([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":75}}\n\n"))
	p.feed([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))

	u, ok := p.usage()
	if !ok {
		t.Fatal("expected usage from Anthropic stream")
	}
	if u.PromptTokens != 200 {
		t.Errorf("input tokens = %d, want 200", u.PromptTokens)
	}
	if u.CompletionTokens != 75 {
		t.Errorf("output tokens = %d, want 75", u.CompletionTokens)
	}
}

// @sk-test usage-accounting-integrity#T4.1: stream without usage is reported absent (AC-004)
func TestSSEUsageParserAbsent(t *testing.T) {
	p := newSSEUsageParser()
	p.feed([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"))
	if _, ok := p.usage(); ok {
		t.Error("expected no usage")
	}
}

// @sk-test usage-accounting-integrity#T4.1: partial SSE lines are buffered across writes (AC-001)
func TestSSEUsageParserPartialLines(t *testing.T) {
	p := newSSEUsageParser()
	p.feed([]byte("data: {\"usage\":{\"prompt_tokens\":10,"))
	p.feed([]byte("\"completion_tokens\":5}}\n\n"))
	u, ok := p.usage()
	if !ok {
		t.Fatal("expected usage across split writes")
	}
	if u.PromptTokens != 10 || u.CompletionTokens != 5 {
		t.Errorf("got %+v, want 10/5", u)
	}
}

// @sk-test responses-api#T3.4: Responses usage (input_tokens/output_tokens) is extracted (AC-007)
func TestExtractJSONUsageResponses(t *testing.T) {
	body := []byte(`{"id":"resp_1","object":"response","usage":{"input_tokens":42,"output_tokens":17,"total_tokens":59}}`)
	u, ok := extractJSONUsage(body)
	if !ok {
		t.Fatal("expected usage to be found")
	}
	if u.PromptTokens != 42 || u.CompletionTokens != 17 {
		t.Errorf("got %+v, want 42/17", u)
	}
}

// @sk-test responses-streaming#T3.4: Responses streaming usage is read from the nested completed event (AC-007)
func TestSSEUsageParserResponses(t *testing.T) {
	p := newSSEUsageParser()
	p.feed([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"))
	p.feed([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":30,\"output_tokens\":12}}}\n\n"))
	u, ok := p.usage()
	if !ok {
		t.Fatal("expected Responses streaming usage")
	}
	if u.PromptTokens != 30 || u.CompletionTokens != 12 {
		t.Errorf("got %+v, want 30/12", u)
	}
}
