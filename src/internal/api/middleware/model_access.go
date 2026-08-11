package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type modelAccessRequest struct {
	Model string `json:"model"`
}

// @sk-task 300-virtual-keys#T2.2: ModelAccess enforces allowed/blocked model scopes (AC-002)
//
// ModelAccess handles the operation.
func ModelAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		vk, ok := VirtualKeyFromContext(c)
		if !ok || vk == nil {
			c.Next()
			return
		}
		if len(vk.AllowedModels) == 0 && len(vk.BlockedModels) == 0 {
			c.Next()
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			AbortWithError(c, http.StatusBadRequest, ErrorCodeValidationError, "invalid request body")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		var req modelAccessRequest
		if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
			AbortWithError(c, http.StatusBadRequest, ErrorCodeValidationError, "model is required")
			return
		}

		if !vk.AllowsModel(req.Model) {
			AbortWithError(c, http.StatusForbidden, "MODEL_ACCESS_DENIED", "model not allowed for this key")
			return
		}
		c.Next()
	}
}
