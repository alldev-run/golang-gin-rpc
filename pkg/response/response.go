package response

import (
	"golang-gin-rpc/pkg/status_code"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code: int(status_code.Success),
		Msg:  status_code.Success.Message(),
		Data: data,
	})
}

func Error(c *gin.Context, msg string, data interface{}) {
	c.JSON(400, Response{
		Code: int(status_code.BadRequest),
		Msg:  msg,
		Data: data,
	})
}
