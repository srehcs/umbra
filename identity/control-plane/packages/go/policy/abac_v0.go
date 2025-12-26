package policy

import (
	"strings"
)

type Policy struct {
	Version int    `json:"version"`
	Mode    string `json:"mode"` // "abac_v0"
	Rules   []Rule `json:"rules"`
	Default string `json:"default"` // "deny" | "allow"
}

type Rule struct {
	Effect     string   `json:"effect"` // "allow" | "deny"
	RolesAny   []string `json:"roles_any,omitempty"`
	MethodsAny []string `json:"methods_any,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

type Decision struct {
	Decision  string `json:"decision"` // allow|deny
	Reason    string `json:"reason"`
	RuleIndex *int   `json:"rule_index,omitempty"`
}

func EvaluateABACV0(actorRoles []string, method string, path string, pol Policy) Decision {
	// Default deny unless allow rule matches; deny rule wins.
	roles := make(map[string]struct{}, len(actorRoles))
	for _, r := range actorRoles {
		roles[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}

	m := strings.ToUpper(strings.TrimSpace(method))
	p := path
	matchedAllow := false
	matchedAllowIdx := -1

	for i, rule := range pol.Rules {
		if rule.PathPrefix != "" && !strings.HasPrefix(p, rule.PathPrefix) {
			continue
		}
		if len(rule.MethodsAny) > 0 {
			ok := false
			for _, mm := range rule.MethodsAny {
				if strings.ToUpper(mm) == m {
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
		eff := strings.ToLower(rule.Effect)
		if eff == "deny" {
			idx := i
			return Decision{Decision: "deny", Reason: "explicit deny rule matched", RuleIndex: &idx}
		}
		if eff == "allow" {
			matchedAllow = true
			matchedAllowIdx = i
		}
	}

	if matchedAllow {
		idx := matchedAllowIdx
		return Decision{Decision: "allow", Reason: "allow rule matched", RuleIndex: &idx}
	}

	if strings.ToLower(pol.Default) == "allow" {
		return Decision{Decision: "allow", Reason: "default allow"}
	}
	return Decision{Decision: "deny", Reason: "default deny (no matching rule)"}
}
