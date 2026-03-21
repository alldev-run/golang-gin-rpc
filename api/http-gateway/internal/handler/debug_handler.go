package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type DebugHandler struct{}

func NewDebugHandler() *DebugHandler {
	return &DebugHandler{}
}

func (h *DebugHandler) RequestID(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"request_id_ctx": c.GetString("request_id"),
		"request_id_hdr": c.GetHeader("X-Request-ID"),
		"request_id_resp": c.Writer.Header().Get("X-Request-ID"),
	})
}

func (h *DebugHandler) OK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
