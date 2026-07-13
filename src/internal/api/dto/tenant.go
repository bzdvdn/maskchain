package dto

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task tenant-profile-sync#T2.2: Tenant request/response DTOs (AC-001, AC-005, AC-008)

type DictionaryItem struct {
	Name      string        `json:"name"`
	MatchMode string        `json:"match_mode"`
	Entries   []interface{} `json:"entries"`
}

type CreateTenantRequest struct {
	Slug         string           `json:"slug" binding:"required"`
	Name         string           `json:"name" binding:"required"`
	AuthHeader   string           `json:"auth_header"`
	APIKeys      []string         `json:"api_keys" binding:"required"`
	Dictionaries []DictionaryItem `json:"dictionaries"`
}

type UpdateTenantRequest struct {
	Name         string           `json:"name" binding:"required"`
	AuthHeader   string           `json:"auth_header"`
	APIKeys      []string         `json:"api_keys" binding:"required"`
	Dictionaries []DictionaryItem `json:"dictionaries"`
}

type TenantResponse struct {
	Slug         string           `json:"slug"`
	Name         string           `json:"name"`
	AuthHeader   string           `json:"auth_header"`
	APIKeys      []string         `json:"api_keys"`
	Dictionaries []DictionaryItem `json:"dictionaries,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type DictionaryRequest struct {
	Dictionaries []DictionaryItem `json:"dictionaries" binding:"required"`
}

type DictionaryResponse struct {
	Dictionaries []DictionaryItem `json:"dictionaries"`
}

func TenantToResponse(t *entity.Tenant) TenantResponse {
	return TenantResponse{
		Slug:       t.Slug().String(),
		Name:       t.Name(),
		AuthHeader: t.AuthHeader(),
		APIKeys:    t.APIKeys(),
		CreatedAt:  t.CreatedAt(),
		UpdatedAt:  t.UpdatedAt(),
	}
}
