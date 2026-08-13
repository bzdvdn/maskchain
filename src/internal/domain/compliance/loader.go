package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry is a collection of loaded compliance packs indexed by key.
type Registry struct {
	packs map[string]*Pack
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{packs: make(map[string]*Pack)}
}

// Packs returns the registered packs by key.
func (r *Registry) Packs() map[string]*Pack {
	return r.packs
}

// Pack returns the pack for a key, or nil when absent.
func (r *Registry) Pack(key string) *Pack {
	return r.packs[key]
}

// Add registers a pack by its key.
func (r *Registry) Add(p *Pack) {
	if p != nil {
		r.packs[p.Key] = p
	}
}

// LoadPacksFromDir scans dir for *.yaml/*.yml preset files, parses and
// validates each against the catalog, and returns the packs registered by key
// plus per-file load errors. A file that fails to parse or validate does not
// stop other packs from loading — its error is returned in the errors slice.
// Presets are versioned in the repo: adding a YAML file is enough to expose a
// new pack without changing code.
func LoadPacksFromDir(dir string, catalog Catalog) (*Registry, []error) {
	reg := NewRegistry()
	var errs []error
	entries, err := os.ReadDir(dir)
	if err != nil {
		return reg, []error{fmt.Errorf("read compliance preset dir %q: %w", dir, err)}
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	for _, f := range files {
		pack, err := loadOne(f, catalog)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		reg.Add(pack)
	}
	return reg, errs
}

func loadOne(path string, catalog Catalog) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compliance preset %q: %w", path, err)
	}
	var p Pack
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse compliance preset %q: %w", path, err)
	}
	if err := p.Validate(catalog); err != nil {
		return nil, fmt.Errorf("compliance preset %q: %w", path, err)
	}
	return &p, nil
}
