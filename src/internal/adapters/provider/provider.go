package provider

import (
	"encoding/base64"
	"encoding/json"
)

// @sk-task 110-provider-adapters#T2.1: Create ProviderError type and ParseProviderError (AC-005)
type ProviderError struct {
	StatusCode int    `json:"status_code"`
	Type       string `json:"type,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (e *ProviderError) Error() string {
	return e.Message
}

// @sk-task 110-provider-adapters#T2.1: Create ProviderError type and ParseProviderError (AC-005)
func ParseProviderError(statusCode int, body []byte, apiType string) *ProviderError {
	var parsed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return &ProviderError{
			StatusCode: statusCode,
			Message:    string(body),
		}
	}

	switch apiType {
	case "anthropic":
		if parsed.Type != "" || parsed.Message != "" {
			return &ProviderError{
				StatusCode: statusCode,
				Type:       parsed.Type,
				Message:    parsed.Message,
			}
		}
		fallthrough
	default:
		if parsed.Error.Type != "" || parsed.Error.Message != "" {
			return &ProviderError{
				StatusCode: statusCode,
				Type:       parsed.Error.Type,
				Message:    parsed.Error.Message,
			}
		}
	}

	return &ProviderError{
		StatusCode: statusCode,
		Message:    string(body),
	}
}

// @sk-task 111-provider-auth-and-config#T3.1: Build auth headers from ProviderConfig (AC-004, AC-007)
func buildAuthHeader(scheme, headerName, key string) (string, string) {
	switch scheme {
	case "bearer":
		return headerName, "Bearer " + key
	case "api-key":
		return headerName, key
	case "basic":
		encoded := base64.StdEncoding.EncodeToString([]byte(":" + key))
		return "Authorization", "Basic " + encoded
	default:
		return headerName, key
	}
}

// @sk-task 111-provider-auth-and-config#T3.1: Merge headers with auth priority (AC-004, AC-007)
func mergeHeaders(authKey, authValue string, additional map[string]string) map[string]string {
	h := make(map[string]string, len(additional)+1)
	for k, v := range additional {
		h[k] = v
	}
	// Auth header has priority — always set after additional so it wins
	h[authKey] = authValue
	return h
}
