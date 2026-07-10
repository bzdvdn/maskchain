package mask

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
)

// @sk-task 22-shield-mask-storage#T2.1: Implement MaskUseCase (AC-002, AC-003, AC-004, AC-005)
type MaskUseCase struct {
	registry *detector.DetectorRegistry
	storage  MaskStorage
}

func NewMaskUseCase(registry *detector.DetectorRegistry, storage MaskStorage) *MaskUseCase {
	return &MaskUseCase{
		registry: registry,
		storage:  storage,
	}
}

func (uc *MaskUseCase) MaskText(ctx context.Context, text string, maskID string) (maskedText string, entry *MaskEntry, err error) {
	var allResults []detector.DetectorResult
	for _, typ := range uc.registry.Types() {
		d := uc.registry.Get(typ)
		if d == nil {
			continue
		}
		results, scanErr := d.Scan(ctx, text)
		if scanErr != nil {
			return "", nil, fmt.Errorf("detector %s: %w", typ, scanErr)
		}
		allResults = append(allResults, results...)
	}

	entry = &MaskEntry{
		MaskID:       maskID,
		Replacements: make(map[string]string),
		CreatedAt:    time.Now(),
	}

	if len(allResults) == 0 {
		if saveErr := uc.storage.Save(ctx, entry); saveErr != nil {
			return "", nil, fmt.Errorf("save mask entry: %w", saveErr)
		}
		return text, entry, nil
	}

	sort.Slice(allResults, func(i, j int) bool {
		lenI := allResults[i].EndPos - allResults[i].StartPos
		lenJ := allResults[j].EndPos - allResults[j].StartPos
		if lenI != lenJ {
			return lenI > lenJ
		}
		if allResults[i].StartPos != allResults[j].StartPos {
			return allResults[i].StartPos < allResults[j].StartPos
		}
		return allResults[i].EndPos > allResults[j].EndPos
	})

	var kept []detector.DetectorResult
	for _, r := range allResults {
		overlap := false
		for _, k := range kept {
			if r.StartPos < k.EndPos && r.EndPos > k.StartPos {
				overlap = true
				break
			}
		}
		if !overlap {
			kept = append(kept, r)
		}
	}

	sort.Slice(kept, func(i, j int) bool {
		return kept[i].StartPos > kept[j].StartPos
	})

	masked := []byte(text)
	counter := 1
	for _, r := range kept {
		placeholder := fmt.Sprintf("{{%s.%d}}", maskID, counter)
		entry.Replacements[placeholder] = r.Fragment

		before := string(masked[:r.StartPos])
		after := string(masked[r.EndPos:])
		masked = []byte(before + placeholder + after)

		counter++
	}

	if saveErr := uc.storage.Save(ctx, entry); saveErr != nil {
		return "", nil, fmt.Errorf("save mask entry: %w", saveErr)
	}

	return string(masked), entry, nil
}

// @sk-task 22-shield-mask-storage#T2.1: Implement UnmaskText (AC-003, AC-004)
func (uc *MaskUseCase) UnmaskText(ctx context.Context, maskedText string, maskIDs []string) (string, error) {
	merged := make(map[string]string)

	for _, id := range maskIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		entry, getErr := uc.storage.Get(ctx, id)
		if getErr != nil {
			return "", fmt.Errorf("get mask %s: %w", id, getErr)
		}
		for k, v := range entry.Replacements {
			merged[k] = v
		}
	}

	if len(merged) == 0 {
		return maskedText, nil
	}

	result := maskedText
	for placeholder, original := range merged {
		result = strings.ReplaceAll(result, placeholder, original)
	}

	return result, nil
}
