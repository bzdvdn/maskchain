package reaction

import (
	"context"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.3: Implement RedactReaction (AC-002)
type RedactReaction struct{}

func NewRedactReaction() *RedactReaction {
	return &RedactReaction{}
}

func (r *RedactReaction) Execute(_ context.Context, result *entity.ScanResult, text string) (string, error) {
	if result == nil {
		return text, nil
	}

	incidents := result.Incidents()
	if len(incidents) == 0 {
		return text, nil
	}

	out := text
	for _, inc := range incidents {
		fragment := inc.Fragment()
		if fragment == "" {
			continue
		}
		pos := strings.Index(out, fragment)
		if pos < 0 {
			continue
		}
		out = out[:pos] + strings.Repeat("*", len(fragment)) + out[pos+len(fragment):]
	}
	return out, nil
}
