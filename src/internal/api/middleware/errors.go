package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
)

type ErrorCode string

const (
	ErrorCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrorCodeSlugConflict    ErrorCode = "SLUG_CONFLICT"
	ErrorCodeValidationError ErrorCode = "VALIDATION_ERROR"
	ErrorCodeInternal        ErrorCode = "INTERNAL_ERROR"
	// @sk-task 80-tenant-isolation#T2.1: Add unauthorized error code (AC-001, AC-002, AC-003)
	ErrorCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	// @sk-task rate-limiting-budgets#T2.2: Add rate limit error codes (AC-001)
	ErrorCodeRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrorCodeTokenBudgetExceeded ErrorCode = "TOKEN_BUDGET_EXCEEDED"
)

// @sk-task 40-profiles-api#T1.1: Implement error middleware and helpers (AC-011)
// @sk-task 118-api-consistency#T3.2: Updated to ApiResponse envelope format (AC-004)
//
// AbortWithError handles the operation.
func AbortWithError(c *gin.Context, status int, code ErrorCode, message string, details ...dto.ValidationDetail) {
	c.Set(EnvelopedKey, true)
	resp := dto.NewErrorResponse(string(code), message, details...)
	c.AbortWithStatusJSON(status, resp)
}

// @sk-task 118-api-consistency#T3.2: ErrorHandler now uses ApiResponse envelope (AC-004)
//
// ErrorHandler handles the operation.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			resp := dto.NewErrorResponse(string(ErrorCodeInternal), err.Error())
			c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
		}
	}
}
