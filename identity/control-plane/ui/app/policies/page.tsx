"use client";

import * as React from "react";
import { api } from "@/lib/api";
import { simulateABACV0, PolicySchema } from "@/lib/policy";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Plus, CheckCircle2, FlaskConical, BadgeCheck, Pencil } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import PageHeader from "@/components/app/page-header";
import EmptyState from "@/components/app/empty-state";
import SectionHeader from "@/components/app/section-header";
import StatusBanner from "@/components/app/status-banner";
import { useAuth } from "@/lib/auth";

import type { PolicyRow } from "@/lib/types";

const starterPolicy = {
  version: 1,
  mode: "abac_v0",
  rules: [
    { effect: "allow", roles_any: ["admin", "developer"], methods_any: ["GET"], path_prefix: "/demo" },
  ],
  default: "deny",
};

export default function PoliciesPage() {
  const [items, setItems] = React.useState<PolicyRow[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const refreshControllerRef = React.useRef<AbortController | null>(null);
  const { hasRole } = useAuth();
  const canManagePolicies = hasRole("policy_admin");

  const [name, setName] = React.useState("default-policy");
  const [policy, setPolicy] = React.useState(JSON.stringify(starterPolicy, null, 2));

  const [validation, setValidation] = React.useState<string | null>(null);
  const [editOpen, setEditOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<PolicyRow | null>(null);
  const [editPolicy, setEditPolicy] = React.useState("");
  const [editValidation, setEditValidation] = React.useState<string | null>(null);

  // simulate modal state
  const [roles, setRoles] = React.useState("developer");
  const [method, setMethod] = React.useState("GET");
  const [path, setPath] = React.useState("/demo");
  const [simResult, setSimResult] = React.useState<{
    decision: string;
    reason: string;
    policy_hash?: string;
    policy_version?: number;
    rule_index?: number | null;
  } | null>(null);
  const [simMode, setSimMode] = React.useState<"local" | "server">("local");
  const [simError, setSimError] = React.useState<string | null>(null);

  const [activePolicy, setActivePolicy] = React.useState<{
    id: string;
    name: string;
    version: number;
    policy_hash: string;
    updated_at: string;
  } | null>(null);

  async function refresh(signal?: AbortSignal) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listPolicies(signal);
      setItems(data.items ?? []);
      try {
        const active = await api.getActivePolicy(signal);
        setActivePolicy(active);
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : "";
        if (msg.includes("404") || msg.includes("no active policy")) {
          setActivePolicy(null);
        } else {
          setActivePolicy(null);
        }
      }
    } catch (e: unknown) {
      if (signal?.aborted) return;
      setError(e instanceof Error ? e.message : "Failed to load policies");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }

  function handleRefresh() {
    refreshControllerRef.current?.abort();
    const controller = new AbortController();
    refreshControllerRef.current = controller;
    void refresh(controller.signal);
  }

  React.useEffect(() => {
    const controller = new AbortController();
    refresh(controller.signal);
    return () => {
      controller.abort();
      refreshControllerRef.current?.abort();
    };
  }, []);

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
    if (!canManagePolicies) {
      setError("Requires role: policy_admin");
      return;
    }
    const ok = validateOnly();
    if (!ok) return;

    const parsed = PolicySchema.parse(JSON.parse(policy));
    try {
      await api.createPolicy({ name, policy: parsed });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Create failed");
    }
    }

    async function activate(id: string) {
    setError(null);
    if (!canManagePolicies) {
      setError("Requires role: policy_admin");
      return;
    }
    if (!window.confirm("Activate this policy? This will deactivate any currently active policy.")) {
      return;
    }
    try {
      await api.activatePolicy(id);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Activate failed");
    }
    }

  async function simulate() {
    setSimResult(null);
    setSimError(null);
    let parsed: unknown;
    try { parsed = JSON.parse(policy); } catch { setValidation("Policy must be valid JSON."); return; }
    const res = PolicySchema.safeParse(parsed);
    if (!res.success) {
      setValidation("Schema validation failed: " + res.error.issues.map(i => `${i.path.join(".")}: ${i.message}`).join("; "));
      return;
    }
    const roleList = roles.split(",").map(s => s.trim()).filter(Boolean);
    if (simMode === "server") {
      try {
        const result = await api.simulatePolicyServer({
          actor_roles: roleList,
          method,
          path,
          policy: res.data,
        });
        setSimResult(result);
      } catch (e: unknown) {
        setSimError(e instanceof Error ? e.message : "Server simulation failed");
      }
      return;
    }
    const r = simulateABACV0(res.data, { roles: roleList, method, path });
    setSimResult({ decision: r.decision, reason: r.reason });
  }

  function openEdit(policyRow: PolicyRow) {
    setEditing(policyRow);
    setEditPolicy(JSON.stringify(policyRow.policy ?? {}, null, 2));
    setEditValidation(null);
    setEditOpen(true);
  }

  function validateEdit(): boolean {
    setEditValidation(null);
    let parsed: unknown;
    try { parsed = JSON.parse(editPolicy); } catch { setEditValidation("Policy must be valid JSON."); return false; }
    const res = PolicySchema.safeParse(parsed);
    if (!res.success) {
      setEditValidation("Schema validation failed: " + res.error.issues.map(i => `${i.path.join(".")}: ${i.message}`).join("; "));
      return false;
    }
    setEditValidation("OK: policy JSON is valid for ABAC V0.");
    return true;
  }

  async function updatePolicy() {
    if (!editing) return;
    setError(null);
    if (!canManagePolicies) {
      setError("Requires role: policy_admin");
      return;
    }
    const ok = validateEdit();
    if (!ok) return;
    const parsed = PolicySchema.parse(JSON.parse(editPolicy));
    try {
      await api.updatePolicy(editing.id, { policy: parsed });
      setEditOpen(false);
      setEditing(null);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Update failed");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Policies"
        subtitle="Author, validate, simulate, and activate policies (default deny)."
        actions={(
          canManagePolicies ? (
            <Dialog>
              <DialogTrigger asChild>
                <Button data-testid="policy-new"><Plus className="h-4 w-4 mr-2" /> New policy</Button>
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
                  <Input value={name} onChange={(e) => setName(e.target.value)} data-testid="policy-name" />
                </div>

                <Alert>
                  <AlertTitle>Schema hints</AlertTitle>
                  <AlertDescription>
                    Required fields: <span className="code">version</span>, <span className="code">mode</span>, <span className="code">rules</span>, <span className="code">default</span>.<br />
                    Rule fields: <span className="code">effect</span> (allow|deny), optional <span className="code">roles_any</span>, <span className="code">methods_any</span>, <span className="code">path_prefix</span>,
                    <span className="code">actor_types_any</span>, <span className="code">actor_ids_any</span>, <span className="code">mcp_servers_any</span>, <span className="code">mcp_tools_any</span>, <span className="code">mcp_methods_any</span>.
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
                        <DialogTitle>Simulate decision</DialogTitle>
                        <DialogDescription>
                          Compare local evaluation with server-side simulation (no DB write).
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

                        <div className="flex flex-wrap gap-2">
                          <Button
                            variant={simMode === "local" ? "default" : "outline"}
                            onClick={() => setSimMode("local")}
                          >
                            Local
                          </Button>
                          <Button
                            variant={simMode === "server" ? "default" : "outline"}
                            onClick={() => setSimMode("server")}
                          >
                            Server
                          </Button>
                          <Button onClick={simulate}>Run simulation</Button>
                        </div>

                        {simResult && (
                          <Alert className="mt-2">
                            <AlertTitle>Result: {simResult.decision.toUpperCase()}</AlertTitle>
                            <AlertDescription>
                              {simResult.reason}
                              {simResult.policy_hash && (
                                <div className="mt-2 text-xs text-muted-foreground">
                                  policy_hash <span className="code">{simResult.policy_hash.slice(0, 12)}</span>
                                </div>
                              )}
                            </AlertDescription>
                          </Alert>
                        )}
                        {simError && (
                          <StatusBanner
                            className="mt-2"
                            title="Simulation failed"
                            description={simError}
                            variant="destructive"
                          />
                        )}
                      </div>

                      <DialogFooter>
                        <Button
                          variant="secondary"
                          onClick={() => {
                            setSimResult(null);
                            setSimError(null);
                          }}
                        >
                          Close
                        </Button>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </div>

                {validation && <div className="text-xs text-muted-foreground">{validation}</div>}
                {error && <div className="text-sm text-red-700">{error}</div>}
              </div>

              <div className="space-y-2">
                <Label>Policy JSON</Label>
                <Textarea className="min-h-[360px]" value={policy} onChange={(e) => setPolicy(e.target.value)} data-testid="policy-json" />
              </div>
            </div>

              <DialogFooter>
                <Button variant="secondary" onClick={handleRefresh} disabled={loading}>Refresh</Button>
              <Button onClick={create} data-testid="policy-create">Create</Button>
              </DialogFooter>
              </DialogContent>
            </Dialog>
          ) : (
            <Button disabled title="Requires role: policy_admin" data-testid="policy-new">
              <Plus className="h-4 w-4 mr-2" /> New policy
            </Button>
          )
        )}
      />

      <StatusBanner
        title="Development mode"
        description="Policy management is tenant-scoped via header. Production will enforce RBAC via Keycloak claims."
      />
      {!canManagePolicies && (
        <StatusBanner
          title="Role required"
          description="Policy creation, editing, and activation require policy_admin."
          variant="destructive"
        />
      )}

      <Card data-testid="active-policy-card">
        <SectionHeader
          title="Active policy"
          description={activePolicy ? "Currently enforced policy for this tenant." : "No active policy yet."}
        />
        <CardContent>
          {activePolicy ? (
            <div className="flex flex-wrap items-center gap-3 text-sm" data-testid="active-policy">
              <Badge variant="success">active</Badge>
              <span className="font-medium">{activePolicy.name}</span>
              <span className="text-muted-foreground">v{activePolicy.version}</span>
              <span className="code text-xs text-muted-foreground">{activePolicy.policy_hash.slice(0, 12)}</span>
              <span className="text-xs text-muted-foreground">
                updated {new Date(activePolicy.updated_at).toLocaleString()}
              </span>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">Activate a policy to see it here.</div>
          )}
        </CardContent>
      </Card>

      <Card>
        <SectionHeader
          title="Policies"
          description={loading ? "Loading…" : `${items.length} policy/policies`}
        />
        <CardContent>
          {error && (
            <StatusBanner
              className="mb-3"
              title="Load failed"
              description={error}
              variant="destructive"
            />
          )}
          <Table data-testid="policies-table">
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead>Policy hash</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((p) => (
                <TableRow key={p.id} data-testid="policies-row">
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>{p.version}</TableCell>
                  <TableCell>
                    {p.active ? <Badge variant="success">active</Badge> : <Badge variant="outline">inactive</Badge>}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{new Date(p.updated_at).toLocaleString()}</TableCell>
                  <TableCell className="code text-xs text-muted-foreground">{p.policy_hash?.slice(0, 12)}</TableCell>
                  <TableCell className="text-right space-x-2">
                    <Button size="sm" variant="outline" onClick={() => openEdit(p)} disabled={!canManagePolicies} title={!canManagePolicies ? "Requires role: policy_admin" : undefined} data-testid={`policy-edit-${p.id}`}>
                      <Pencil className="h-4 w-4 mr-2" />
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant={p.active ? "secondary" : "default"}
                      onClick={() => activate(p.id)}
                      disabled={p.active || !canManagePolicies}
                      title={!canManagePolicies ? "Requires role: policy_admin" : undefined}
                      data-testid={`policy-activate-${p.id}`}
                    >
                      <CheckCircle2 className="h-4 w-4 mr-2" />
                      Activate
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {items.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={6}>
                    <EmptyState message="No policies yet." />
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Edit policy</DialogTitle>
            <DialogDescription>
              {editing ? `${editing.name} • v${editing.version}` : "Edit policy JSON"}
              {editing?.active && " • active"}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label>Policy JSON</Label>
            <Textarea className="min-h-[360px]" value={editPolicy} onChange={(e) => setEditPolicy(e.target.value)} />
            {editValidation && <div className="text-xs text-muted-foreground">{editValidation}</div>}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={validateEdit}><BadgeCheck className="h-4 w-4 mr-2" /> Validate</Button>
            <Button onClick={updatePolicy} disabled={editing?.active || !canManagePolicies} title={!canManagePolicies ? "Requires role: policy_admin" : undefined}>Update</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
