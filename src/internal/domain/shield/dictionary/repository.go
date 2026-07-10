package dictionary

import "context"

// @sk-task 24-shield-dictionaries#T2.1: Implement DictionaryRepository interface (AC-002)
type DictionaryRepository interface {
	Save(ctx context.Context, dict *Dictionary) error
	FindByProfileSlug(ctx context.Context, slug string) (*Dictionary, error)
	Delete(ctx context.Context, slug string) error
}
