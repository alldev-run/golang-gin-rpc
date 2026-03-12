package jwtx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Middleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		token := c.GetHeader("Authorization")

		if token == "" {

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing token",
			})

			return
		}

		claims, err := ValidateAccessToken(token)

		if err != nil {

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})

			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
