// @sk-task remove-audit-incidents#T2.1: Remove Incident from ReactionResult (AC-007)
// Package reaction provides reaction strategies for Content Shield.
package reaction

import (
	"context"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 23-shield-reactions#T2.1: Define ReactionExecutor interface (DEC-001)
//
// ReactionExecutor defines the interface for domain operations.
type ReactionExecutor interface {
	Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error)
}
