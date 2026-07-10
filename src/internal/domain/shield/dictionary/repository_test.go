package dictionary

import (
	"context"
	"sync"
)

// @sk-test 24-shield-dictionaries#T6.1: TestDictionaryValueObject (AC-001)
type inMemoryRepo struct {
	mu   sync.RWMutex
	data map[string]*Dictionary
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{data: make(map[string]*Dictionary)}
}

func (r *inMemoryRepo) Save(ctx context.Context, dict *Dictionary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[dict.ProfileSlug().String()] = dict
	return nil
}

func (r *inMemoryRepo) FindByProfileSlug(ctx context.Context, slug string) (*Dictionary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.data[slug]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (r *inMemoryRepo) Delete(ctx context.Context, slug string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, slug)
	return nil
}
