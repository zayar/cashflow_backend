package middlewares

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("token")
		if token == "" {
			c.Next()
			return
		}
		username, exists, err := config.GetRedisValue("Token:" + token)
		if err != nil || !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), utils.ContextKeyToken, token)
		ctx = context.WithValue(ctx, utils.ContextKeyUsername, username)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
