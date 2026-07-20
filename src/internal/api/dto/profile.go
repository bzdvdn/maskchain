package dto

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

// @sk-task 40-profiles-api#T1.1: Create DTO types (AC-003, AC-011)
//
// CreateProfileRequest represents a domain entity or configuration.
type CreateProfileRequest struct {
	Slug          string                         `json:"slug" validate:"required,min=3"`
	Name          string                         `json:"name" validate:"required"`
	Description   *string                        `json:"description,omitempty"`
	Dictionaries  []DictionaryDTO                `json:"dictionaries,omitempty"`
	Preprocessors []preprocessor.PreprocessorDef `json:"preprocessors,omitempty"`
}

type UpdateProfileRequest struct {
	Name          *string                        `json:"name,omitempty" validate:"omitempty,required"`
	Description   *string                        `json:"description,omitempty"`
	Dictionaries  []DictionaryDTO                `json:"dictionaries,omitempty"`
	Preprocessors []preprocessor.PreprocessorDef `json:"preprocessors,omitempty"`
}

type ProfileResponse struct {
	ID            string                         `json:"id"`
	Slug          string                         `json:"slug"`
	Name          string                         `json:"name"`
	Description   *string                        `json:"description,omitempty"`
	Status        string                         `json:"status"`
	Dictionaries  []DictionaryDTO                `json:"dictionaries,omitempty"`
	Preprocessors []preprocessor.PreprocessorDef `json:"preprocessors,omitempty"`
	CreatedAt     string                         `json:"created_at"`
	UpdatedAt     string                         `json:"updated_at"`
}

type ProfileListItem struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type DictionaryDTO struct {
	Name      string               `json:"name" validate:"required"`
	Entries   []interface{}        `json:"entries" validate:"required"`
	MatchMode dictionary.MatchMode `json:"match_mode" validate:"required,oneof=exact contains regex fuzzy"`
}

type PatchDictionaryRequest struct {
	Action  string   `json:"action" validate:"required,oneof=add remove"`
	Name    string   `json:"name" validate:"required"`
	Entries []string `json:"entries" validate:"required,min=1"`
}

type ErrorResponse struct {
	Error   string             `json:"error"`
	Code    string             `json:"code"`
	Details []ValidationDetail `json:"details,omitempty"`
}

type ValidationDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
