"use client";

import * as React from "react";
import { api } from "@/lib/api";
import type { Receipt, ReceiptKind } from "@/lib/types";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import ReceiptDetail from "@/components/app/receipt-detail";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import PageHeader from "@/components/app/page-header";
import EmptyState from "@/components/app/empty-state";
import StatusBanner from "@/components/app/status-banner";

function badgeForOutcome(r: Receipt) {
  if (r.kind === "decision") {
    return r.decision === "allow" ? <Badge variant="success">allow</Badge> : <Badge variant="danger">deny</Badge>;
  }
  if (r.kind === "invocation") {
    if (r.outcome === "success") return <Badge variant="success">success</Badge>;
    if (r.outcome === "denied") return <Badge variant="warning">denied</Badge>;
    return <Badge variant="danger">error</Badge>;
  }
  return <Badge variant="outline">unknown</Badge>;
}

export default function ReceiptsPage() {
  const [items, setItems] = React.useState<Receipt[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [q, setQ] = React.useState("");
  const [kind, setKind] = React.useState<"all" | ReceiptKind>("all");

  const [nextBefore, setNextBefore] = React.useState<string | undefined>(undefined);
  const [selected, setSelected] = React.useState<Receipt | null>(null);
  const [selectedJSON, setSelectedJSON] = React.useState<string>("");
  const [verifyResult, setVerifyResult] = React.useState<{
    ok: boolean;
    checked: number;
    kind: string;
    failure?: { receipt_id: string; code: string };
  } | null>(null);
  const [verifyError, setVerifyError] = React.useState<string | null>(null);
  const [verifyLoading, setVerifyLoading] = React.useState(false);
  const verifyEnabled = process.env.NEXT_PUBLIC_RECEIPTS_VERIFY_ENABLED === "true";
  const traceBaseUrl = process.env.NEXT_PUBLIC_JAEGER_BASE_URL || "";

  async function load(reset: boolean, signal?: AbortSignal) {
    setLoading(true);
    setError(null);
    try {
      const qValue = q.trim();
      const beforeValue = reset ? undefined : nextBefore;
      const kindValue = kind === "all" ? undefined : kind;
      const data = await api.listReceipts({
        limit: 50,
        ...(kindValue ? { kind: kindValue } : {}),
        ...(qValue ? { q: qValue } : {}),
        ...(beforeValue ? { before: beforeValue } : {}),
      }, signal);

      const newItems = data.items ?? [];
      setItems(reset ? newItems : [...items, ...newItems]);
      setNextBefore(data.next_before);
    } catch (e: unknown) {
      if (signal?.aborted) return;
      const msg = e instanceof Error ? e.message : "Failed to load receipts";
      setError(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }

  React.useEffect(() => {
    const controller = new AbortController();
    load(true, controller.signal);
    return () => controller.abort();
    // eslint-disable-next-line
  }, []);
  React.useEffect(() => {
    const controller = new AbortController();
    const t = setTimeout(() => load(true, controller.signal), 250);
    return () => {
      controller.abort();
      clearTimeout(t);
    };
    // eslint-disable-next-line
  }, [q, kind]);

  React.useEffect(() => {
    if (!selected) {
      setSelectedJSON("");
      return;
    }
    setSelectedJSON(JSON.stringify(selected, null, 2));
  }, [selected]);

  async function runVerify() {
    if (!verifyEnabled) return;
    setVerifyError(null);
    setVerifyResult(null);
    setVerifyLoading(true);
    try {
      const res = await api.verifyReceipts({ kind: "all", limit: 100 });
      setVerifyResult(res);
    } catch (e: unknown) {
      setVerifyError(e instanceof Error ? e.message : "Verification failed");
    } finally {
      setVerifyLoading(false);
    }
  }

  function applyDecisionFilter(id: string) {
    setQ(id);
    setKind("all");
  }

  function applyRequestFilter(id: string) {
    setQ(id);
    setKind("all");
  }

  function applyReceiptFilter(id: string) {
    setQ(id);
    setKind("all");
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Receipts"
        subtitle="Evidence trail for decisions and tool invocations (hash-chained, signing-ready)."
        actions={(
          <>
            <Dialog>
              <DialogTrigger asChild>
                <Button variant="outline">Export</Button>
              </DialogTrigger>
              <DialogContent className="max-w-2xl">
                <DialogHeader>
                  <DialogTitle>Export receipts</DialogTitle>
                  <DialogDescription>
                    Use the export endpoint to download JSON or CSV with basic filters.
                  </DialogDescription>
                </DialogHeader>
                <div className="text-sm text-muted-foreground">
                  <div className="mb-2">Example (JSON, last 1 hour):</div>
                  <pre className="code text-xs bg-muted p-3 rounded-md overflow-auto">
{`curl -sS -H "x-umbra-tenant-id: <tenant_id>" \\
  "http://localhost:8080/v1/receipts/export?format=json&from=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)"`}
                  </pre>
                  <div className="mt-3 mb-2">Example (CSV, denies only):</div>
                  <pre className="code text-xs bg-muted p-3 rounded-md overflow-auto">
{`curl -sS -H "x-umbra-tenant-id: <tenant_id>" \\
  "http://localhost:8080/v1/receipts/export?format=csv&decision=deny&limit=200"`}
                  </pre>
                </div>
                <DialogFooter>
                  <Button variant="secondary" onClick={() => navigator.clipboard.writeText("http://localhost:8080/v1/receipts/export")}>
                    Copy endpoint
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
            {verifyEnabled && (
              <Button variant="outline" onClick={runVerify} disabled={verifyLoading}>
                {verifyLoading ? "Verifying…" : "Verify integrity"}
              </Button>
            )}
            <Button variant="secondary" onClick={() => load(true)} disabled={loading}>Refresh</Button>
          </>
        )}
      />

      {verifyResult && (
        <StatusBanner
          title={verifyResult.ok ? "Integrity verified" : "Integrity check failed"}
          variant={verifyResult.ok ? "default" : "destructive"}
          description={
            <div className="space-y-2 text-sm">
              <div>
                Checked {verifyResult.checked} receipt(s) ({verifyResult.kind}).
              </div>
              {verifyResult.failure?.receipt_id && (
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs text-muted-foreground">Failure code</span>
                  <span className="code text-xs">{verifyResult.failure.code}</span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => applyReceiptFilter(verifyResult.failure.receipt_id)}
                  >
                    View failing receipt
                  </Button>
                </div>
              )}
            </div>
          }
        />
      )}
      {verifyError && (
        <StatusBanner
          title="Verification failed"
          variant="destructive"
          description={verifyError}
        />
      )}

      <StatusBanner
        title="Development mode"
        description={
          <>
            Tenant context is provided via <span className="code">x-umbra-tenant-id</span>. Production derives tenant and roles from OIDC claims.
          </>
        }
      />

      <Card>
        <CardHeader className="gap-2">
          <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
            <div>
              <CardTitle>Recent receipts</CardTitle>
              <CardDescription>{loading ? "Loading…" : `${items.length} loaded`}</CardDescription>
            </div>
            <div className="flex flex-col gap-2 md:flex-row md:items-center">
              <div className="w-44">
                <label className="text-xs text-muted-foreground">Kind</label>
                <Select
                  value={kind}
                  onValueChange={(value) => {
                    if (value === "decision" || value === "invocation" || value === "all") setKind(value);
                  }}
                >
                  <SelectTrigger className="mt-1 h-10">
                    <SelectValue placeholder="All" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All</SelectItem>
                    <SelectItem value="decision">Decision</SelectItem>
                    <SelectItem value="invocation">Invocation</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="w-72">
                <label className="text-xs text-muted-foreground">Filter</label>
                <Input className="mt-1" placeholder="tool, decision, request, hash, trace…" value={q} onChange={(e) => setQ(e.target.value)} />
              </div>
            </div>
          </div>
        </CardHeader>

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
                <TableHead>Kind</TableHead>
                <TableHead>Result</TableHead>
                <TableHead>When</TableHead>
                <TableHead>Decision</TableHead>
                <TableHead>Request</TableHead>
                <TableHead>Hash</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((r, idx) => (
                <TableRow key={`${r.kind}-${r.id ?? idx}`}>
                  <TableCell><Badge variant="outline">{r.kind}</Badge></TableCell>
                  <TableCell>{badgeForOutcome(r)}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{String(r.ts ?? "")}</TableCell>
                  <TableCell className="code text-xs text-muted-foreground truncate max-w-[240px]">{String(r.decision_id ?? "")}</TableCell>
                  <TableCell className="code text-xs text-muted-foreground truncate max-w-[240px]">{String(r.request_id ?? "")}</TableCell>
                  <TableCell className="code text-xs text-muted-foreground truncate max-w-[240px]">{String(r.hash ?? "")}</TableCell>
                  <TableCell className="text-right">
                    <Dialog>
                      <DialogTrigger asChild>
                        <Button size="sm" variant="outline" onClick={() => setSelected(r)}>View</Button>
                      </DialogTrigger>
                      <DialogContent className="max-w-4xl">
                        <DialogHeader>
                          <DialogTitle>Receipt detail</DialogTitle>
                          <DialogDescription>
                            Structured fields first, with raw JSON available below.
                          </DialogDescription>
                        </DialogHeader>

                        <ReceiptDetail
                          r={selected ?? r}
                          onFilterDecisionId={applyDecisionFilter}
                          onFilterRequestId={applyRequestFilter}
                          traceBaseUrl={traceBaseUrl || undefined}
                        />

                        <div className="mt-4">
                          <div className="mb-2 text-xs text-muted-foreground">Raw JSON</div>
                          <pre className="code text-xs bg-muted p-4 rounded-md overflow-auto max-h-[45vh]">
{selectedJSON || JSON.stringify(r, null, 2)}
                          </pre>
                        </div>

                        <DialogFooter>
                          <Button
                            variant="secondary"
                            onClick={() => navigator.clipboard.writeText(selectedJSON || JSON.stringify(r, null, 2))}
                          >
                            Copy JSON
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  </TableCell>
                </TableRow>
              ))}
              {items.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={7}>
                    <EmptyState message="No receipts yet. Run PEP requests to generate receipts." />
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>

          <div className="mt-4 flex items-center justify-between">
            <div className="text-xs text-muted-foreground">
              {nextBefore ? <>Next cursor: <span className="code">{nextBefore}</span></> : "No more pages (or not loaded yet)."}
            </div>
            <Button
              variant="outline"
              disabled={!nextBefore || loading}
              onClick={() => load(false)}
            >
              Load more
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
