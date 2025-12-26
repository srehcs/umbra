import { z } from "zod";

export const RuleSchema = z.object({
  effect: z.enum(["allow", "deny"]),
  roles_any: z.array(z.string()).optional().default([]),
  methods_any: z.array(z.string()).optional().default([]),
  path_prefix: z.string().optional().default(""),
});

export const PolicySchema = z.object({
  version: z.number().int().positive(),
  mode: z.string(),
  rules: z.array(RuleSchema),
  default: z.enum(["allow", "deny"]),
});

export type Rule = z.infer<typeof RuleSchema>;
export type Policy = z.infer<typeof PolicySchema>;

export type SimInput = {
  roles: string[];
  method: string;
  path: string;
};

export type SimResult = {
  decision: "allow" | "deny";
  reason: string;
  matchedRule?: Rule;
};

export function simulateABACV0(policy: Policy, input: SimInput): SimResult {
  const roles = new Set(input.roles.map((r) => r.toLowerCase()));
  const method = input.method.toUpperCase();
  const path = input.path;

  for (const rule of policy.rules) {
    if (rule.path_prefix && !path.startsWith(rule.path_prefix)) continue;

    if (rule.methods_any.length > 0) {
      const ok = rule.methods_any.some((m) => m.toUpperCase() === method);
      if (!ok) continue;
    }

    if (rule.roles_any.length > 0) {
      const ok = rule.roles_any.some((rr) => roles.has(rr.toLowerCase()));
      if (!ok) continue;
    }

    if (rule.effect === "deny") return { decision: "deny", reason: "explicit deny rule matched", matchedRule: rule };
    return { decision: "allow", reason: "allow rule matched", matchedRule: rule };
  }

  if (policy.default === "allow") return { decision: "allow", reason: "default allow" };
  return { decision: "deny", reason: "default deny (no matching rule)" };
}
