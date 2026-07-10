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
)

// @sk-task 40-profiles-api#T1.1: Implement error middleware and helpers (AC-011)
func AbortWithError(c *gin.Context, status int, code ErrorCode, message string, details ...dto.ValidationDetail) {
	resp := dto.ErrorResponse{
		Error: message,
		Code:  string(code),
	}
	if len(details) > 0 {
		resp.Details = details
	}
	c.AbortWithStatusJSON(status, resp)
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{
				Error: err.Error(),
				Code:  string(ErrorCodeInternal),
			})
		}
	}
}
