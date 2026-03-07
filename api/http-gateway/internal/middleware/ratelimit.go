package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(10, 20)

func RateLimit() gin.HandlerFunc {

	return func(c *gin.Context) {

		if !limiter.Allow() {

			c.JSON(http.StatusTooManyRequests, gin.H{
				"msg": "too many requests",
			})

			c.Abort()
			return
		}

		c.Next()
	}
}
