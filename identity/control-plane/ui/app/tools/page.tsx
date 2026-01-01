"use client";

import * as React from "react";
import type { Tool } from "@/lib/types";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import PageHeader from "@/components/app/page-header";
import EmptyState from "@/components/app/empty-state";
import SectionHeader from "@/components/app/section-header";
import StatusBanner from "@/components/app/status-banner";
import { Plus } from "lucide-react";
import { useAuth } from "@/lib/auth";
export default function ToolsPage() {
  const [items, setItems] = React.useState<Tool[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const { hasRole } = useAuth();
  const canManageTools = hasRole("tool_admin");

  const [name, setName] = React.useState("sample-http-tool");
  const [kind, setKind] = React.useState("http");
  const [config, setConfig] = React.useState(JSON.stringify({ upstream: "http://upstream-sample:9000" }, null, 2));

  async function refresh(signal?: AbortSignal) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listTools(signal);
      setItems(data.items ?? []);
    } catch (e: unknown) {
      if (signal?.aborted) return;
      setError(e instanceof Error ? e.message : "Failed to load tools");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }

  function handleRefresh() {
    void refresh();
  }

  React.useEffect(() => {
    const controller = new AbortController();
    refresh(controller.signal);
    return () => controller.abort();
  }, []);

  async function create() {
    setError(null);
    if (!canManageTools) {
      setError("Requires role: tool_admin");
      return;
    }
    let parsed: Record<string, unknown> = {};
    try { parsed = JSON.parse(config); } catch { setError("Config must be valid JSON"); return; }
    try {
      await api.createTool({ name, kind, config: parsed });
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Create failed");
    }
  }

  return (
    <div className="space-y-6">
      {/* Development mode banner */}
      
      <PageHeader
        title="Tools"
        subtitle="Register tool surfaces and upstream config (tenant-scoped)."
        actions={(
          canManageTools ? (
            <Dialog>
              <DialogTrigger asChild>
                <Button><Plus className="h-4 w-4 mr-2" /> New tool</Button>
              </DialogTrigger>
              <DialogContent>
              <DialogHeader>
                <DialogTitle>Create tool</DialogTitle>
                <DialogDescription>V0 supports kinds: http, mcp, cli. Start with http.</DialogDescription>
              </DialogHeader>

              <div className="space-y-3">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label>Kind</Label>
                  <Input value={kind} onChange={(e) => setKind(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label>Config (JSON)</Label>
                  <Textarea value={config} onChange={(e) => setConfig(e.target.value)} />
                </div>
                {error && <div className="text-sm text-red-700">{error}</div>}
              </div>

              <DialogFooter>
                <Button variant="secondary" onClick={handleRefresh} disabled={loading}>Refresh</Button>
                <Button onClick={create}>Create</Button>
              </DialogFooter>
              </DialogContent>
            </Dialog>
          ) : (
            <Button disabled title="Requires role: tool_admin">
              <Plus className="h-4 w-4 mr-2" /> New tool
            </Button>
          )
        )}
      />

      <StatusBanner
        title="Development mode"
        description="Tools are tenant-scoped via header. Production will enforce tool admin roles via OIDC claims."
      />
      {!canManageTools && (
        <StatusBanner
          title="Role required"
          description="Tool creation is limited to tool_admin."
          variant="destructive"
        />
      )}

      <Card>
        <SectionHeader
          title="Registered tools"
          description={loading ? "Loading…" : `${items.length} tool(s)`}
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
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>ID</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((t: Tool) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell><Badge variant="outline">{t.kind}</Badge></TableCell>
                  <TableCell className="code text-xs text-muted-foreground">{t.id}</TableCell>
                </TableRow>
              ))}
              {items.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={3}>
                    <EmptyState message="No tools yet. Create one." />
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
