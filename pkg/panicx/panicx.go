package panicx

import (
	"fmt"
	"runtime/debug"
)

func ErrorString(v any) string {
	return fmt.Sprintf("%v", v)
}

func Stack() string {
	return string(debug.Stack())
}
