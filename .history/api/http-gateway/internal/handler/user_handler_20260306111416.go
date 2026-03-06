package handler
package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetUser(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{
		"msg": "user ok",
	})
}