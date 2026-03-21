package router

import (
	"github.com/gin-gonic/gin"

	"alldev-gin-rpc/api/http-gateway/internal/handler"
	"alldev-gin-rpc/api/http-gateway/internal/service"
)

type Router struct {
	hello *handler.HelloHandler
	debug *handler.DebugHandler
}

func NewRouter() *Router {
	helloSvc := service.NewHelloService()
	return &Router{
		hello: handler.NewHelloHandler(helloSvc),
		debug: handler.NewDebugHandler(),
	}
}

func (r *Router) Register(engine *gin.Engine) {
	engine.GET("/", r.hello.Root)

	debug := engine.Group("/debug")
	{
		debug.GET("/request-id", r.debug.RequestID)
		debug.GET("/ok", r.debug.OK)
	}
}
