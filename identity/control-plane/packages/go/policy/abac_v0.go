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
	Effect         string   `json:"effect"` // "allow" | "deny"
	RolesAny       []string `json:"roles_any,omitempty"`
	MethodsAny     []string `json:"methods_any,omitempty"`
	PathPrefix     string   `json:"path_prefix,omitempty"`
	ActorTypesAny  []string `json:"actor_types_any,omitempty"`
	ActorIDsAny    []string `json:"actor_ids_any,omitempty"`
	MCPServersAny  []string `json:"mcp_servers_any,omitempty"`
	MCPToolsAny    []string `json:"mcp_tools_any,omitempty"`
	MCPMethodsAny  []string `json:"mcp_methods_any,omitempty"`
}

type Decision struct {
	Decision  string `json:"decision"` // allow|deny
	Reason    string `json:"reason"`
	RuleIndex *int   `json:"rule_index,omitempty"`
}

type RequestContext struct {
	ActorID    string
	ActorType  string
	ActorRoles []string
	Method     string
	Path       string
	MCPServer  string
	MCPTool    string
	MCPMethod  string
}

func EvaluateABACV0(ctx RequestContext, pol Policy) Decision {
	// Default deny unless allow rule matches; deny rule wins.
	roles := make(map[string]struct{}, len(ctx.ActorRoles))
	for _, r := range ctx.ActorRoles {
		roles[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}

	m := strings.ToUpper(strings.TrimSpace(ctx.Method))
	p := ctx.Path
	actorType := strings.ToLower(strings.TrimSpace(ctx.ActorType))
	actorID := strings.TrimSpace(ctx.ActorID)
	mcpServer := strings.ToLower(strings.TrimSpace(ctx.MCPServer))
	mcpTool := strings.TrimSpace(ctx.MCPTool)
	mcpMethod := strings.ToLower(strings.TrimSpace(ctx.MCPMethod))
	matchedAllow := false
	matchedAllowIdx := -1

	for i, rule := range pol.Rules {
		if len(rule.ActorTypesAny) > 0 {
			ok := false
			for _, t := range rule.ActorTypesAny {
				if strings.ToLower(strings.TrimSpace(t)) == actorType && actorType != "" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(rule.ActorIDsAny) > 0 {
			ok := false
			for _, id := range rule.ActorIDsAny {
				if strings.TrimSpace(id) == actorID && actorID != "" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
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
		if len(rule.MCPServersAny) > 0 {
			ok := false
			for _, srv := range rule.MCPServersAny {
				if strings.ToLower(strings.TrimSpace(srv)) == mcpServer && mcpServer != "" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(rule.MCPToolsAny) > 0 {
			ok := false
			for _, tool := range rule.MCPToolsAny {
				if strings.TrimSpace(tool) == mcpTool && mcpTool != "" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(rule.MCPMethodsAny) > 0 {
			ok := false
			for _, meth := range rule.MCPMethodsAny {
				if strings.ToLower(strings.TrimSpace(meth)) == mcpMethod && mcpMethod != "" {
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
