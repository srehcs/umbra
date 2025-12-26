"use client";

import * as React from "react";
import type { PolicyRow } from "@/lib/types";
import { api } from "@/lib/api";
import { simulateABACV0, PolicySchema } from "@/lib/policy";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Plus, CheckCircle2, FlaskConical, BadgeCheck } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

import type { Policy } from "@/lib/types";

const starterPolicy = {
  version: 1,
  mode: "abac_v0",
  rules: [
    { effect: "allow", roles_any: ["admin", "developer"], methods_any: ["GET"], path_prefix: "/demo" },
  ],
  default: "deny",
};

export default function PoliciesPage() {
  const [items, setItems] = React.useState<Policy[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [name, setName] = React.useState("default-policy");
  const [policy, setPolicy] = React.useState(JSON.stringify(starterPolicy, null, 2));

  const [validation, setValidation] = React.useState<string | null>(null);

  // simulate modal state
  const [roles, setRoles] = React.useState("developer");
  const [method, setMethod] = React.useState("GET");
  const [path, setPath] = React.useState("/demo");
  const [simResult, setSimResult] = React.useState<{ decision: string; reason: string } | null>(null);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listPolicies();
      setItems(data.items ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : \"Failed to load policies\");
    } finally {
      setLoading(false);
    }
  }

  React.useEffect(() => { refresh(); }, []);

  function validateOnly(): boolean {
    setValidation(null);
    let parsed: unknown;
    try { parsed = JSON.parse(policy); } catch { setValidation("Policy must be valid JSON."); return false; }
    const res = PolicySchema.safeParse(parsed);
    if (!res.success) {
      setValidation("Schema validation failed: " + res.error.issues.map(i => `${i.path.join(".")}: ${i.message}`).join("; "));
      return false;
    }
    if (res.data.mode !== "abac_v0") {
      setValidation("Warning: mode is not 'abac_v0'. PDP V0 expects ABAC JSON (ADR-0005 governs future engines).");
      return true;
    }
    setValidation("OK: policy JSON is valid for ABAC V0.");
    return true;
  }

  async function create() {
    setError(null);
    const ok = validateOnly();
    if (!ok) return;

    const parsed: unknown = JSON.parse(policy);
    try {
      await api.createPolicy({ name, policy: parsed as Record<string, unknown> });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : \"Create failed\");
    }
  }

  async function activate(id: string) {
    setError(null);
    try {
      await api.activatePolicy(id);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : \"Activate failed\");
    }
  }

  function simulate() {
    setSimResult(null);
    let parsed: unknown;
    try { parsed = JSON.parse(policy); } catch { setValidation("Policy must be valid JSON."); return; }
    const res = PolicySchema.safeParse(parsed);
    if (!res.success) {
      setValidation("Schema validation failed: " + res.error.issues.map(i => `${i.path.join(".")}: ${i.message}`).join("; "));
      return;
    }
    const r = simulateABACV0(res.data, {
      roles: roles.split(",").map(s => s.trim()).filter(Boolean),
      method,
      path,
    });
    setSimResult({ decision: r.decision, reason: r.reason });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Policies</h1>
          <p className="text-sm text-muted-foreground">Author, validate, simulate, and activate policies (default deny).</p>
        </div>

        <Dialog>
          <DialogTrigger asChild>
            <Button><Plus className="h-4 w-4 mr-2" /> New policy</Button>
          </DialogTrigger>
          <DialogContent className="max-w-3xl">
            <DialogHeader>
              <DialogTitle>Create policy</DialogTitle>
              <DialogDescription>
                V0 uses ABAC JSON. Use validation + simulation before persisting.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-3">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>

                <Alert>
                  <AlertTitle>Schema hints</AlertTitle>
                  <AlertDescription>
                    Required fields: <span className="code">version</span>, <span className="code">mode</span>, <span className="code">rules</span>, <span className="code">default</span>.<br />
                    Rule fields: <span className="code">effect</span> (allow|deny), optional <span className="code">roles_any</span>, <span className="code">methods_any</span>, <span className="code">path_prefix</span>.
                  </AlertDescription>
                </Alert>

                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" onClick={validateOnly}><BadgeCheck className="h-4 w-4 mr-2" /> Validate</Button>

                  <Dialog>
                    <DialogTrigger asChild>
                      <Button variant="secondary"><FlaskConical className="h-4 w-4 mr-2" /> Simulate</Button>
                    </DialogTrigger>
                    <DialogContent>
                      <DialogHeader>
                        <DialogTitle>Simulate decision (local)</DialogTitle>
                        <DialogDescription>
                          Simulates ABAC V0 behavior on your edited policy (no DB write). PDP will mirror this in V0.
                        </DialogDescription>
                      </DialogHeader>

                      <div className="space-y-3">
                        <div className="space-y-2">
                          <Label>Roles (comma-separated)</Label>
                          <Input value={roles} onChange={(e) => setRoles(e.target.value)} />
                        </div>
                        <div className="grid gap-3 md:grid-cols-2">
                          <div className="space-y-2">
                            <Label>Method</Label>
                            <Input value={method} onChange={(e) => setMethod(e.target.value)} />
                          </div>
                          <div className="space-y-2">
                            <Label>Path</Label>
                            <Input value={path} onChange={(e) => setPath(e.target.value)} />
                          </div>
                        </div>

                        <Button onClick={simulate}>Run simulation</Button>

                        {simResult && (
                          <Alert className="mt-2">
                            <AlertTitle>Result: {simResult.decision.toUpperCase()}</AlertTitle>
                            <AlertDescription>{simResult.reason}</AlertDescription>
                          </Alert>
                        )}
                      </div>

                      <DialogFooter>
                        <Button variant="secondary" onClick={() => setSimResult(null)}>Close</Button>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </div>

                {validation && <div className="text-xs text-muted-foreground">{validation}</div>}
                {error && <div className="text-sm text-red-700">{error}</div>}
              </div>

              <div className="space-y-2">
                <Label>Policy JSON</Label>
                <Textarea className="min-h-[360px]" value={policy} onChange={(e) => setPolicy(e.target.value)} />
              </div>
            </div>

            <DialogFooter>
              <Button variant="secondary" onClick={refresh} disabled={loading}>Refresh</Button>
              <Button onClick={create}>Create</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Alert>
        <AlertTitle>Development mode</AlertTitle>
        <AlertDescription>
          Policy management is tenant-scoped via header. Production will enforce RBAC via Keycloak claims.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>Policies</CardTitle>
          <CardDescription>{loading ? "Loading…" : `${items.length} policy/policies`}</CardDescription>
        </CardHeader>
        <CardContent>
          {error && <div className="mb-3 text-sm text-red-700">{error}</div>}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Policy hash</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>{p.version}</TableCell>
                  <TableCell>
                    {p.active ? <Badge variant="success">active</Badge> : <Badge variant="outline">inactive</Badge>}
                  </TableCell>
                  <TableCell className="code text-xs text-muted-foreground">{p.policy_hash}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      variant={p.active ? "secondary" : "default"}
                      onClick={() => activate(p.id)}
                      disabled={p.active}
                    >
                      <CheckCircle2 className="h-4 w-4 mr-2" />
                      Activate
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {items.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={5} className="text-sm text-muted-foreground">No policies yet.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
