// Package reaction provides reaction strategies for Content Shield.
package reaction

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.1: Define ReactionExecutor interface (DEC-001)
type ReactionExecutor interface {
	Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error)
}
