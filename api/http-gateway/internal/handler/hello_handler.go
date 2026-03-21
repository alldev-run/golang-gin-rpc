package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alldev-gin-rpc/api/http-gateway/internal/service"
)

type HelloHandler struct {
	svc *service.HelloService
}

func NewHelloHandler(svc *service.HelloService) *HelloHandler {
	return &HelloHandler{svc: svc}
}

func (h *HelloHandler) Root(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Hello())
}
