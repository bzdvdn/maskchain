package dto

import "time"

// @sk-task conversation-logging#T3.1: Conversation list item DTO, metadata only (AC-006)
// @sk-task conversation-logging#T5.1: Expose MaskID in list item (AC-005)
//
// ConversationListItem represents a domain entity or configuration.
type ConversationListItem struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Model     string    `json:"model"`
	Status    string    `json:"status"`
	Masked    bool      `json:"masked"`
	Streamed  bool      `json:"streamed"`
	MaskID    string    `json:"mask_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// @sk-task conversation-logging#T3.1: Conversation list response DTO (AC-006)
//
// ConversationListResponse represents a domain entity or configuration.
type ConversationListResponse struct {
	Items []ConversationListItem `json:"items"`
}

// @sk-task conversation-logging#T3.1: Conversation payload DTO with base64 content (AC-005)
//
// ConversationPayload represents a domain entity or configuration.
type ConversationPayload struct {
	Request  string  `json:"request"`
	Response *string `json:"response"`
	Masking  *string `json:"masking"`
}

// @sk-task conversation-logging#T3.1: Conversation detail DTO with payload (AC-005)
// @sk-task conversation-logging#T5.1: Expose MaskID in detail (AC-005)
//
// ConversationDetail represents a domain entity or configuration.
type ConversationDetail struct {
	ID        string              `json:"id"`
	TenantID  string              `json:"tenant_id"`
	Model     string              `json:"model"`
	Status    string              `json:"status"`
	Masked    bool                `json:"masked"`
	Streamed  bool                `json:"streamed"`
	MaskID    string              `json:"mask_id,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	Payload   ConversationPayload `json:"payload"`
}
