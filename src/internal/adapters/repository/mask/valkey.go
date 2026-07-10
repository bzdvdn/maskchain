package maskrepo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 22-shield-mask-storage#T3.2: Implement ValkeyMaskRepo (AC-009, AC-012)
type ValkeyMaskRepo struct {
	client valkey.Client
	ttl    time.Duration
}

func NewValkeyMaskRepo(client valkey.Client, ttl time.Duration) *ValkeyMaskRepo {
	return &ValkeyMaskRepo{client: client, ttl: ttl}
}

func (r *ValkeyMaskRepo) key(maskID string) string {
	return "mask:" + maskID
}

func (r *ValkeyMaskRepo) Save(ctx context.Context, entry *mask.MaskEntry) error {
	if r.client == nil {
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return r.client.Do(ctx, r.client.B().Set().Key(r.key(entry.MaskID)).
		Value(string(data)).
		Ex(r.ttl).
		Build()).Error()
}

func (r *ValkeyMaskRepo) Get(ctx context.Context, maskID string) (*mask.MaskEntry, error) {
	if r.client == nil {
		return nil, mask.ErrMaskNotFound
	}
	data, err := r.client.Do(ctx, r.client.B().Get().Key(r.key(maskID)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, mask.ErrMaskNotFound
		}
		return nil, err
	}

	var entry mask.MaskEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *ValkeyMaskRepo) Delete(ctx context.Context, maskID string) error {
	if r.client == nil {
		return nil
	}
	return r.client.Do(ctx, r.client.B().Del().Key(r.key(maskID)).Build()).Error()
}
