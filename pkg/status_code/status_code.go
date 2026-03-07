package status_code

type StatusCode int

const (
	Success    StatusCode = 200
	Internal   StatusCode = 500
	BadRequest StatusCode = 400
	NotFound   StatusCode = 404
	Fail       StatusCode = 499
	Error      StatusCode = -1
)

var statusText = map[StatusCode]string{
	Success:    "success",
	Internal:   "internal error",
	BadRequest: "bad request",
	NotFound:   "not found",
	Fail:       "fail",
	Error:      "error",
}

// Message 返回状态码对应的 message
func (c StatusCode) Message() string {
	if msg, ok := statusText[c]; ok {
		return msg
	}
	return "unknown error"
}
