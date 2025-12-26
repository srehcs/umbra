package decision

import "strings"

type Input struct {
	TenantID   string
	ActorRoles []string
	Method     string
	Path       string
}

type Result struct {
	Decision      string // allow|deny
	PolicyHash    string
	PolicyVersion int
	Reason        string
}

type Policy struct {
	Version int    `json:"version"`
	Mode    string `json:"mode"`
	Rules   []Rule `json:"rules"`
	Default string `json:"default"` // deny|allow
}

type Rule struct {
	Effect     string   `json:"effect"` // allow|deny
	RolesAny   []string `json:"roles_any"`
	MethodsAny []string `json:"methods_any"`
	PathPrefix string   `json:"path_prefix"`
}

type Evaluator interface {
	Evaluate(in Input, pol Policy, policyHash string) Result
}

type ABACV0 struct{}

func (e ABACV0) Evaluate(in Input, pol Policy, policyHash string) Result {
	// Default deny unless rule matches and effect=allow.
	// If a deny rule matches, deny wins (explicit deny).
	roles := make(map[string]struct{}, len(in.ActorRoles))
	for _, r := range in.ActorRoles {
		roles[strings.ToLower(r)] = struct{}{}
	}

	method := strings.ToUpper(in.Method)
	path := in.Path

	matchedAllow := false

	for _, rule := range pol.Rules {
		if rule.PathPrefix != "" && !strings.HasPrefix(path, rule.PathPrefix) {
			continue
		}
		if len(rule.MethodsAny) > 0 {
			ok := false
			for _, m := range rule.MethodsAny {
				if strings.ToUpper(m) == method {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(rule.RolesAny) > 0 {
			ok := false
			for _, rr := range rule.RolesAny {
				if _, exists := roles[strings.ToLower(rr)]; exists {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		// rule matches
		if strings.ToLower(rule.Effect) == "deny" {
			return Result{Decision: "deny", PolicyHash: policyHash, PolicyVersion: pol.Version, Reason: "explicit deny rule matched"}
		}
		if strings.ToLower(rule.Effect) == "allow" {
			matchedAllow = true
		}
	}

	if matchedAllow {
		return Result{Decision: "allow", PolicyHash: policyHash, PolicyVersion: pol.Version, Reason: "allow rule matched"}
	}

	// fallback default
	if strings.ToLower(pol.Default) == "allow" {
		return Result{Decision: "allow", PolicyHash: policyHash, PolicyVersion: pol.Version, Reason: "default allow"}
	}
	return Result{Decision: "deny", PolicyHash: policyHash, PolicyVersion: pol.Version, Reason: "default deny (no matching rule)"}
}
