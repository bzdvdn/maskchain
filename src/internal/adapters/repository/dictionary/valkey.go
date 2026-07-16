package dictionaryrepo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
)

type ValkeyDictionaryCache struct {
	client valkey.Client
	ttl    time.Duration
}

func NewValkeyDictionaryCache(client valkey.Client, ttl time.Duration) *ValkeyDictionaryCache {
	return &ValkeyDictionaryCache{client: client, ttl: ttl}
}

func (c *ValkeyDictionaryCache) key(slug string) string {
	return "dict:" + slug
}

func (c *ValkeyDictionaryCache) Get(ctx context.Context, slug string) ([]*dictionary.Dictionary, error) {
	if c.client == nil {
		return nil, nil
	}
	data, err := c.client.Do(ctx, c.client.B().Get().Key(c.key(slug)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	var dicts []*dictionary.Dictionary
	if err := json.Unmarshal([]byte(data), &dicts); err != nil {
		return nil, err
	}
	return dicts, nil
}

func (c *ValkeyDictionaryCache) Set(ctx context.Context, slug string, dicts []*dictionary.Dictionary) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(dicts)
	if err != nil {
		return err
	}
	return c.client.Do(ctx, c.client.B().Set().Key(c.key(slug)).
		Value(string(data)).
		Ex(c.ttl).
		Build()).Error()
}

func (c *ValkeyDictionaryCache) Delete(ctx context.Context, slug string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Do(ctx, c.client.B().Del().Key(c.key(slug)).Build()).Error()
}
