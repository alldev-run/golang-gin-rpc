package middleware

import "github.com/gin-gonic/gin"
import "golang-gin-rpc/pkg/response"

func JWTAuth() gin.HandlerFunc {

	return func(c *gin.Context) {

		token := c.GetHeader("Authorization")

		if token == "" {
			response.Error(c, "no token", nil)
			c.Abort()
			return
		}

		// 解析token
		// TODO verify jwt

		c.Next()
	}
}
