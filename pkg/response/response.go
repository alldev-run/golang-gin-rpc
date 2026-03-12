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

var (
	SuccessCode = status_code.Success
	ErrorCode   = status_code.BadRequest
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(int(SuccessCode), Response{
		Code: int(status_code.Success),
		Msg:  status_code.Success.Message(),
		Data: data,
	})
}

func Error(c *gin.Context, msg string, data interface{}) {
	c.JSON(int(ErrorCode), Response{
		Code: int(status_code.BadRequest),
		Msg:  msg,
		Data: data,
	})
}
