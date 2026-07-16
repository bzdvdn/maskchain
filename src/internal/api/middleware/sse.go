package middleware

import (
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

// @sk-task 112-proxy-streaming-wiring#T2.2: Create WrapSSE middleware (AC-002)
func WrapSSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isStreamRequest(c) {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Transfer-Encoding", "chunked")
		}
		c.Next()
	}
}

func isStreamRequest(c *gin.Context) bool {
	if c.Request == nil || c.Request.Body == nil {
		return false
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
	return strings.Contains(string(body), `"stream": true`) ||
		strings.Contains(string(body), `"stream":true`)
}
