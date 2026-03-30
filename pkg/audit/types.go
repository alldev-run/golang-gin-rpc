package audit

import "time"

// Action represents audited operation category.
type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionLogin  Action = "login"
	ActionCustom Action = "custom"
)

// Event is a normalized audit event payload.
type Event struct {
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	TenantID    string                 `json:"tenant_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	ClientIP    string                 `json:"client_ip,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Path        string                 `json:"path,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	Action      Action                 `json:"action"`
	Resource    string                 `json:"resource,omitempty"`
	Result      string                 `json:"result,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Sensitive   bool                   `json:"sensitive,omitempty"`
	DurationMS  int64                  `json:"duration_ms,omitempty"`
}
