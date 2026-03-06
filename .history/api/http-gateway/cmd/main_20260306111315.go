package main

import (
	"go-micro/api/http-gateway/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	router.InitRouter(r)

	r.Run(":8080")
}