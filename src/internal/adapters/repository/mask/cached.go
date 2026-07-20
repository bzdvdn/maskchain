package maskrepo

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 22-shield-mask-storage#T3.3: Implement CachedMaskRepo (AC-010)
//
// CachedMaskRepo represents a domain entity or configuration.
type CachedMaskRepo struct {
	primary   mask.MaskStorage
	secondary mask.MaskStorage
}

func NewCachedMaskRepo(primary, secondary mask.MaskStorage) *CachedMaskRepo {
	return &CachedMaskRepo{
		primary:   primary,
		secondary: secondary,
	}
}

func (r *CachedMaskRepo) Save(ctx context.Context, entry *mask.MaskEntry) error {
	if err := r.primary.Save(ctx, entry); err != nil {
		return err
	}
	_ = r.secondary.Save(ctx, entry)
	return nil
}

func (r *CachedMaskRepo) Get(ctx context.Context, maskID string) (*mask.MaskEntry, error) {
	entry, err := r.secondary.Get(ctx, maskID)
	if err == nil {
		return entry, nil
	}

	entry, err = r.primary.Get(ctx, maskID)
	if err != nil {
		return nil, err
	}

	_ = r.secondary.Save(ctx, entry)
	return entry, nil
}

func (r *CachedMaskRepo) Delete(ctx context.Context, maskID string) error {
	if err := r.primary.Delete(ctx, maskID); err != nil {
		return err
	}
	_ = r.secondary.Delete(ctx, maskID)
	return nil
}
