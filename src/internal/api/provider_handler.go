package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// @sk-task 51-shield-gateway-integration#T1.2: Create proxy handler stub (AC-002, AC-007)
func ProxyChatCompletionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"choices": []gin.H{
			{
				"message": gin.H{
					"role":    "assistant",
					"content": "ok",
				},
			},
		},
	})
}

func ProxyCompletionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"choices": []gin.H{
			{
				"text": "ok",
			},
		},
	})
}
