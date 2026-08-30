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
//
// MaskUseCase represents a domain entity or configuration.
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

// @sk-task 23-shield-reactions#T1.2: Implement MaskFromResults (DEC-002)
// @sk-task mask-token-format#T1.3: Format-aware token builder (AC-001, AC-002, AC-007)
//
// MaskFromResults replaces detected fragments with format-specific tokens and
// stores a MaskEntry so the document can be restored via UnmaskText.
func (uc *MaskUseCase) MaskFromResults(ctx context.Context, text string, maskID string, documentMaskID string, results []detector.DetectorResult, format Format) (maskedText string, entry *MaskEntry, err error) {
	docID := documentMaskID
	if docID == "" {
		docID = maskID
	}
	entry = &MaskEntry{
		MaskID:         maskID,
		DocumentMaskID: docID,
		Replacements:   make(map[string]string),
		Reversible:     format != FormatRedact,
		CreatedAt:      time.Now(),
	}

	if len(results) == 0 {
		if saveErr := uc.storage.Save(ctx, entry); saveErr != nil {
			return "", nil, fmt.Errorf("save mask entry: %w", saveErr)
		}
		return text, entry, nil
	}

	kept := ResolveOverlaps(results)
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].StartPos > kept[j].StartPos
	})

	// Assign counters left-to-right, then replace right-to-left so earlier
	// byte offsets stay valid while the rendered tokens read in text order.
	tokens := make([]string, len(kept))
	for i := 0; i < len(kept); i++ {
		leftIdx := len(kept) - 1 - i
		var placeholder string
		switch format {
		case FormatID:
			placeholder = fmt.Sprintf("[MASK_%s.%d]", docID, i+1)
		case FormatRedact:
			placeholder = "[REDACTED]"
		default:
			placeholder = fmt.Sprintf("[MASK.%d]", i+1)
		}
		tokens[leftIdx] = placeholder
	}

	masked := []byte(text)
	for idx, r := range kept {
		placeholder := tokens[idx]

		if format != FormatRedact {
			entry.Replacements[placeholder] = r.Fragment
		}

		before := string(masked[:r.StartPos])
		after := string(masked[r.EndPos:])
		masked = []byte(before + placeholder + after)
	}

	if saveErr := uc.storage.Save(ctx, entry); saveErr != nil {
		return "", nil, fmt.Errorf("save mask entry: %w", saveErr)
	}

	return string(masked), entry, nil
}

// @sk-task 22-shield-mask-storage#T2.1: Implement UnmaskText (AC-003, AC-004)
// @sk-task mask-token-format#T1.3: Sequential per-document unmask + reversible check (AC-004, AC-005, AC-006, AC-007)
//
// UnmaskText restores masked text by applying each entry's replacements for the
// given ids in order; non-reversible (redact) entries are rejected.
func (uc *MaskUseCase) UnmaskText(ctx context.Context, maskedText string, maskIDs []string) (string, error) {
	result := maskedText

	for _, id := range maskIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		entry, getErr := uc.storage.Get(ctx, id)
		if getErr != nil {
			return "", fmt.Errorf("get mask %s: %w", id, getErr)
		}
		if !entry.Reversible {
			return "", fmt.Errorf("unmask mask %s: %w", id, ErrNotReversible)
		}
		for placeholder, original := range entry.Replacements {
			result = strings.ReplaceAll(result, placeholder, original)
		}
	}

	return result, nil
}

// @sk-task 23-shield-reactions#T1.2: Extract overlap resolution for reuse across mask and proxy paths
//
// ResolveOverlaps sorts results by length (longest first) and removes
// shorter results that overlap with longer ones — exactly like MaskFromResults.
// Exported for reuse in ShieldMiddleware so the proxy path resolves overlaps
// the same way as the /mask endpoint.
func ResolveOverlaps(results []detector.DetectorResult) []detector.DetectorResult {
	if len(results) == 0 {
		return results
	}
	sort.Slice(results, func(i, j int) bool {
		lenI := results[i].EndPos - results[i].StartPos
		lenJ := results[j].EndPos - results[j].StartPos
		if lenI != lenJ {
			return lenI > lenJ
		}
		if results[i].StartPos != results[j].StartPos {
			return results[i].StartPos < results[j].StartPos
		}
		return results[i].EndPos > results[j].EndPos
	})
	var kept []detector.DetectorResult
	for _, r := range results {
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
	return kept
}
