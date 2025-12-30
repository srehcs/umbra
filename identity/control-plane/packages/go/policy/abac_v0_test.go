package policy

import "testing"

func TestEvaluateABACV0_ActorTypeMatch(t *testing.T) {
	pol := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:        "allow",
				ActorTypesAny: []string{"agent"},
			},
		},
	}

	decision := EvaluateABACV0(RequestContext{
		ActorType: "agent",
	}, pol)

	if decision.Decision != "allow" {
		t.Fatalf("expected allow for actor type match, got %s", decision.Decision)
	}
}

func TestEvaluateABACV0_ActorIDMatch(t *testing.T) {
	pol := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:      "allow",
				ActorIDsAny: []string{"user-123"},
			},
		},
	}

	decision := EvaluateABACV0(RequestContext{
		ActorID: "user-123",
	}, pol)

	if decision.Decision != "allow" {
		t.Fatalf("expected allow for actor id match, got %s", decision.Decision)
	}
}
