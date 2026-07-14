package middleware

import "github.com/gin-gonic/gin"

// @sk-task 112-proxy-streaming-wiring#T2.2: Create WrapSSE middleware (AC-002)
func WrapSSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Transfer-Encoding", "chunked")
		c.Next()
	}
}
