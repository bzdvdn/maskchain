package provider

import "encoding/json"

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
