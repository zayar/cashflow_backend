package middlewares

import (
	"context"
	"net/http"

	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/gin-gonic/gin"
)

type authString string

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.Request.Header.Get("Authorization")

		if auth == "" {
			c.Next()
			return
		}

		bearer := "Bearer "
		auth = auth[len(bearer):]

		validate, err := utils.JwtValidate(auth)
		if err != nil || !validate.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		customClaim, _ := validate.Claims.(*utils.JwtCustomClaim)

		ctx := context.WithValue(c.Request.Context(), authString("auth"), customClaim)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func CtxValue(ctx context.Context) *utils.JwtCustomClaim {
	raw, _ := ctx.Value(authString("auth")).(*utils.JwtCustomClaim)
	return raw
}
