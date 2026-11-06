package httplog

import (
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

func TestLogNoPanic(t *testing.T) {
	logger.Init(logger.DefaultConfig())
	Log(Fields{
		Method:    "GET",
		Path:      "/",
		ClientIP:  "127.0.0.1",
		UserAgent: "test",
		Status:    200,
		Latency:   10 * time.Millisecond,
		RequestID: "rid",
	})
}
