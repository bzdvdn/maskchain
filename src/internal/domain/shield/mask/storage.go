package mask

import "context"

// @sk-task 22-shield-mask-storage#T1.1: Define MaskStorage interface (AC-001)
//
// MaskStorage defines the interface for domain operations.
type MaskStorage interface {
	Save(ctx context.Context, entry *MaskEntry) error
	Get(ctx context.Context, maskID string) (*MaskEntry, error)
	Delete(ctx context.Context, maskID string) error
}
