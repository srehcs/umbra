package policy

import (
	"strings"
	"testing"
)

func FuzzEvaluateABACV0Deterministic(f *testing.F) {
	f.Add(
		"actor-1",
		"user",
		"policy_admin,tool_admin",
		"GET",
		"/v1/tools",
		"server-1",
		"tool-1",
		"call",
		"allow",
		"deny",
		"policy_admin",
		"auditor",
		"/v1",
		"/admin",
		"deny",
	)

	f.Fuzz(func(t *testing.T, actorID, actorType, actorRolesCSV, method, path, mcpServer, mcpTool, mcpMethod, rule1Effect, rule2Effect, rule1RolesCSV, rule2RolesCSV, rule1Prefix, rule2Prefix, defaultDecision string) {
		roles := splitLimited(actorRolesCSV, 6)
		pol := Policy{
			Version: 1,
			Mode:    "abac_v0",
			Default: defaultDecision,
			Rules: []Rule{
				{
					Effect:     rule1Effect,
					RolesAny:   splitLimited(rule1RolesCSV, 4),
					MethodsAny: splitLimited(method, 2),
					PathPrefix: limitString(rule1Prefix, 128),
					ActorIDsAny: []string{
						limitString(actorID, 64),
					},
				},
				{
					Effect:        rule2Effect,
					RolesAny:      splitLimited(rule2RolesCSV, 4),
					MethodsAny:    splitLimited(method, 2),
					PathPrefix:    limitString(rule2Prefix, 128),
					ActorTypesAny: splitLimited(actorType, 2),
					MCPServersAny: splitLimited(mcpServer, 2),
					MCPToolsAny:   splitLimited(mcpTool, 2),
					MCPMethodsAny: splitLimited(mcpMethod, 2),
				},
			},
		}

		ctx := RequestContext{
			ActorID:    limitString(actorID, 64),
			ActorType:  limitString(actorType, 64),
			ActorRoles: roles,
			Method:     limitString(method, 16),
			Path:       limitString(path, 256),
			MCPServer:  limitString(mcpServer, 64),
			MCPTool:    limitString(mcpTool, 64),
			MCPMethod:  limitString(mcpMethod, 16),
		}

		first := EvaluateABACV0(ctx, pol)
		second := EvaluateABACV0(ctx, pol)

		if first.Decision != second.Decision || first.Reason != second.Reason || !sameRuleIndex(first.RuleIndex, second.RuleIndex) {
			t.Fatalf("non-deterministic decision: %+v vs %+v", first, second)
		}
		if first.Decision != "allow" && first.Decision != "deny" {
			t.Fatalf("unexpected decision: %s", first.Decision)
		}
		if first.Reason == "" {
			t.Fatalf("decision reason is empty")
		}
		if first.RuleIndex != nil {
			if *first.RuleIndex < 0 || *first.RuleIndex >= len(pol.Rules) {
				t.Fatalf("rule index out of bounds: %d", *first.RuleIndex)
			}
			ruleEffect := strings.ToLower(pol.Rules[*first.RuleIndex].Effect)
			if first.Decision == "allow" && ruleEffect != "allow" {
				t.Fatalf("allow decision does not match rule effect: %s", ruleEffect)
			}
			if first.Decision == "deny" && ruleEffect != "deny" {
				t.Fatalf("deny decision does not match rule effect: %s", ruleEffect)
			}
		}
	})
}

func splitLimited(raw string, max int) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, max)
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if len(item) > 64 {
			item = item[:64]
		}
		out = append(out, item)
		if len(out) == max {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func limitString(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > max {
		return trimmed[:max]
	}
	return trimmed
}

func sameRuleIndex(left, right *int) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return *left == *right
}
