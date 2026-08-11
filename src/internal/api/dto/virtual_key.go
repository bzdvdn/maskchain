package dto

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/virtualkey"
)

// @sk-task 300-virtual-keys#T3.1: Virtual key create/update DTOs (AC-001)
type CreateVirtualKeyRequest struct {
	TenantID      string            `json:"tenant_id" binding:"required"`
	Label         string            `json:"label"`
	AllowedModels []string          `json:"allowed_models"`
	BlockedModels []string          `json:"blocked_models"`
	BudgetCap     *float64          `json:"budget_cap"`
	ExpiresAt     *time.Time        `json:"expires_at"`
	Metadata      map[string]string `json:"metadata"`
}

type UpdateVirtualKeyRequest struct {
	Label         string            `json:"label"`
	AllowedModels []string          `json:"allowed_models"`
	BlockedModels []string          `json:"blocked_models"`
	BudgetCap     *float64          `json:"budget_cap"`
	ExpiresAt     *time.Time        `json:"expires_at"`
	Enabled       *bool             `json:"enabled"`
	Metadata      map[string]string `json:"metadata"`
}

type VirtualKeyResponse struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	Label         string            `json:"label"`
	AllowedModels []string          `json:"allowed_models"`
	BlockedModels []string          `json:"blocked_models"`
	BudgetCap     *float64          `json:"budget_cap"`
	Spent         float64           `json:"spent"`
	ExpiresAt     *time.Time        `json:"expires_at"`
	Metadata      map[string]string `json:"metadata"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type CreateVirtualKeyResponse struct {
	VirtualKeyResponse
	// Key is the plaintext secret returned exactly once at creation.
	Key string `json:"key"`
}

func VirtualKeyToResponse(k *virtualkey.VirtualKey) VirtualKeyResponse {
	return VirtualKeyResponse{
		ID:            k.ID,
		TenantID:      k.TenantID,
		Label:         k.Label,
		AllowedModels: k.AllowedModels,
		BlockedModels: k.BlockedModels,
		BudgetCap:     k.BudgetCap,
		Spent:         k.Spent,
		ExpiresAt:     k.ExpiresAt,
		Metadata:      k.Metadata,
		Enabled:       k.Enabled,
		CreatedAt:     k.CreatedAt,
		UpdatedAt:     k.UpdatedAt,
	}
}
