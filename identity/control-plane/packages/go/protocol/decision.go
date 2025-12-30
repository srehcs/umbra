package protocol

// Decision protocol between PEP and PDP.
//
// Keep this minimal for V0. As the API stabilizes, generate types from OpenAPI.

type TenantContext struct {
	TenantID string `json:"tenant_id"`
}

type Actor struct {
	Type  string   `json:"type"`            // "user" | "agent" (V0)
	ID    string   `json:"id"`              // stable identifier
	Roles []string `json:"roles,omitempty"` // role names (V0)
}

type Tool struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

type TraceContext struct {
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
}

type DecisionRequest struct {
	Tenant TenantContext `json:"tenant"`
	Actor  Actor         `json:"actor"`
	Tool   Tool          `json:"tool"`
	Trace  *TraceContext `json:"trace,omitempty"`
}

type Obligation struct {
	Kind   string            `json:"kind"`
	Params map[string]string `json:"params,omitempty"`
}

type DecisionResponse struct {
	Decision      string       `json:"decision"`                 // "allow" | "deny"
	DecisionID    string       `json:"decision_id"`              // UUID
	PolicyVersion int          `json:"policy_version,omitempty"` // active policy version
	PolicyHash    string       `json:"policy_hash,omitempty"`
	RuleIndex     *int         `json:"rule_index,omitempty"`
	Obligations   []Obligation `json:"obligations,omitempty"`
	Reason        string       `json:"reason,omitempty"`
	RequestID     string       `json:"request_id,omitempty"`
	TraceID       string       `json:"trace_id,omitempty"`
	SpanID        string       `json:"span_id,omitempty"`
}
