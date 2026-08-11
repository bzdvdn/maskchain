package dto

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T3.1: Budget create/update DTOs (AC-006)
type CreateBudgetRequest struct {
	TenantID     string    `json:"tenant_id" binding:"required"`
	VirtualKeyID string    `json:"virtual_key_id"`
	Model        string    `json:"model"`
	Scope        string    `json:"scope" binding:"required"`
	Type         string    `json:"type" binding:"required"`
	CustomDays   int       `json:"custom_days"`
	SoftLimit    *float64  `json:"soft_limit"`
	HardLimit    *float64  `json:"hard_limit"`
	Currency     string    `json:"currency"`
	NotifyAt     []float64 `json:"notify_at"`
}

type UpdateBudgetRequest struct {
	VirtualKeyID *string   `json:"virtual_key_id"`
	Model        *string   `json:"model"`
	Scope        *string   `json:"scope"`
	Type         *string   `json:"type"`
	CustomDays   *int      `json:"custom_days"`
	SoftLimit    *float64  `json:"soft_limit"`
	HardLimit    *float64  `json:"hard_limit"`
	Currency     *string   `json:"currency"`
	NotifyAt     []float64 `json:"notify_at"`
	Enabled      *bool     `json:"enabled"`
}

type BudgetResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	VirtualKeyID string    `json:"virtual_key_id"`
	Model        string    `json:"model"`
	Scope        string    `json:"scope"`
	Type         string    `json:"type"`
	CustomDays   int       `json:"custom_days"`
	SoftLimit    *float64  `json:"soft_limit"`
	HardLimit    *float64  `json:"hard_limit"`
	Currency     string    `json:"currency"`
	NotifyAt     []float64 `json:"notify_at"`
	Enabled      bool      `json:"enabled"`
	Spent        float64   `json:"spent"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SpendEntryResponse struct {
	ID           string    `json:"id"`
	BudgetID     string    `json:"budget_id"`
	VirtualKeyID string    `json:"virtual_key_id"`
	TenantID     string    `json:"tenant_id"`
	Model        string    `json:"model"`
	Cost         float64   `json:"cost"`
	Tokens       int64     `json:"tokens"`
	CreatedAt    time.Time `json:"created_at"`
}

func BudgetToResponse(b *budget.Budget) BudgetResponse {
	return BudgetResponse{
		ID:           b.ID,
		TenantID:     b.TenantID,
		VirtualKeyID: b.VirtualKeyID,
		Model:        b.Model,
		Scope:        string(b.Scope),
		Type:         string(b.Type),
		CustomDays:   b.CustomDays,
		SoftLimit:    b.SoftLimit,
		HardLimit:    b.HardLimit,
		Currency:     b.Currency,
		NotifyAt:     b.NotifyAt,
		Enabled:      b.Enabled,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}

func SpendEntryToResponse(e budget.SpendEntry) SpendEntryResponse {
	return SpendEntryResponse{
		ID:           e.ID,
		BudgetID:     e.BudgetID,
		VirtualKeyID: e.VirtualKeyID,
		TenantID:     e.TenantID,
		Model:        e.Model,
		Cost:         e.Cost,
		Tokens:       e.Tokens,
		CreatedAt:    e.CreatedAt,
	}
}
