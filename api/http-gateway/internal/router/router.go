package router

import (
	"alldev-gin-rpc/api/http-gateway/internal/handler"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {

	api := r.Group("/api")

	{
		api.GET("/user", handler.GetUser)
	}
}