package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// @sk-task media-endpoints#T2.1: model extraction for JSON and multipart (AC-005)
//
// ExtractModel returns the model from an OpenAI-compatible request body: a JSON
// "model" field, or the "model" form field of a multipart/form-data request. It
// only reads the body; callers always forward the original bytes.
func ExtractModel(contentType string, body []byte) (string, error) {
	mediaType := contentType
	if i := strings.Index(mediaType, ";"); i >= 0 {
		mediaType = mediaType[:i]
	}
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))

	if strings.HasPrefix(mediaType, "multipart/form-data") {
		return modelFromMultipart(contentType, body)
	}

	// JSON and unknown content types are read as JSON best-effort.
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", fmt.Errorf("invalid request body")
	}
	return req.Model, nil
}

func modelFromMultipart(contentType string, body []byte) (string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("invalid multipart content type")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", fmt.Errorf("missing multipart boundary")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return "", nil
		}
		if err != nil {
			return "", fmt.Errorf("invalid multipart body")
		}
		if part.FormName() == "model" {
			value, err := io.ReadAll(io.LimitReader(part, 1<<20))
			_ = part.Close()
			if err != nil {
				return "", fmt.Errorf("failed to read model field")
			}
			return strings.TrimSpace(string(value)), nil
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()
	}
}

// @sk-task media-endpoints#T2.3: media-aware model access (AC-005)
//
// MediaModelAccess enforces allowed/blocked model scopes for media endpoints,
// reading the model from JSON or a multipart form field. It restores the raw
// body so the request is forwarded unchanged.
func MediaModelAccess() gin.HandlerFunc {
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

		model, err := ExtractModel(c.GetHeader("Content-Type"), body)
		if err != nil || model == "" {
			AbortWithError(c, http.StatusBadRequest, ErrorCodeValidationError, "model is required")
			return
		}
		if !vk.AllowsModel(model) {
			AbortWithError(c, http.StatusForbidden, "MODEL_ACCESS_DENIED", "model not allowed for this key")
			return
		}
		c.Next()
	}
}
