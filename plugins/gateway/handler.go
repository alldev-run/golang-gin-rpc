package gateway

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler builds a gin.Engine, registers gateway middlewares/routes via SetupRoutes,
// and returns it as a standard http.Handler.
func (g *Gateway) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	g.SetupRoutes(engine)
	return engine
}
