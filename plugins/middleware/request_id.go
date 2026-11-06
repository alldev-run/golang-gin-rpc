package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/alldev-run/golang-gin-rpc/pkg/requestid"
)

func RequestID() gin.HandlerFunc {

	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = requestid.MustNew()
		}

		c.Set("request_id", id)

		c.Writer.Header().Set("X-Request-ID", id)

		c.Next()
	}
}
