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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

function badgeForOutcome(r: Receipt) {
  if (r.kind === "decision") {
    return r.decision === "allow" ? <Badge variant="success">allow</Badge> : <Badge variant="danger">deny</Badge>;
  }
  if (r.kind === "invocation") {
    if (r.outcome === "success") return <Badge variant="success">success</Badge>;
    if (r.outcome === "denied") return <Badge variant="warning">denied</Badge>;
    return <Badge variant="danger">error</Badge>;
  }
  return <Badge variant="outline">{r.kind}</Badge>;
}

export default function ReceiptsPage() {
  const [items, setItems] = React.useState<Receipt[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [q, setQ] = React.useState("");
  const [kind, setKind] = React.useState<"all" | ReceiptKind>("all");

  const [nextBefore, setNextBefore] = React.useState<string | undefined>(undefined);
  const [selected, setSelected] = React.useState<Receipt | null>(null);

  async function load(reset: boolean) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listReceipts({
        limit: 50,
        kind,
        q: q.trim() ? q.trim() : undefined,
        before: reset ? undefined : nextBefore,
      });

      const newItems = data.items ?? [];
      setItems(reset ? newItems : [...items, ...newItems]);
      setNextBefore(data.next_before);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Failed to load receipts";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  React.useEffect(() => { load(true); /* eslint-disable-next-line */ }, []);
  React.useEffect(() => {
    const t = setTimeout(() => load(true), 250);
    return () => clearTimeout(t);
    // eslint-disable-next-line
  }, [q, kind]);

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Receipts</h1>
          <p className="text-sm text-muted-foreground">
            Evidence trail for decisions and tool invocations (hash-chained, signing-ready).
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={() => load(true)} disabled={loading}>Refresh</Button>
        </div>
      </div>

      <Alert>
        <AlertTitle>Development mode</AlertTitle>
        <AlertDescription>
          Tenant context is provided via <span className="code">x-umbra-tenant-id</span>. Production derives tenant and roles from OIDC claims.
        </AlertDescription>
      </Alert>

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
                <select
                  className="mt-1 h-10 w-full rounded-md border border-border bg-white px-3 text-sm"
                  value={kind}
                  onChange={(e) => {
                    const v = e.target.value as string;
                    if (v === "decision" || v === "invocation" || v === "all") setKind(v);
                  }}
                >
                  <option value="all">All</option>
                  <option value="decision">Decision</option>
                  <option value="invocation">Invocation</option>
                </select>
              </div>
              <div className="w-72">
                <label className="text-xs text-muted-foreground">Filter</label>
                <Input className="mt-1" placeholder="tool, decision, request, hash, trace…" value={q} onChange={(e) => setQ(e.target.value)} />
              </div>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          {error && <div className="mb-3 text-sm text-red-700">{error}</div>}
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

                        <ReceiptDetail r={selected ?? r} />

                        <div className="mt-4">
                          <div className="mb-2 text-xs text-muted-foreground">Raw JSON</div>
                          <pre className="code text-xs bg-muted p-4 rounded-md overflow-auto max-h-[45vh]">
{JSON.stringify(selected ?? r, null, 2)}
                          </pre>
                        </div>

                        <DialogFooter>
                          <Button
                            variant="secondary"
                            onClick={() => navigator.clipboard.writeText(JSON.stringify(selected ?? r, null, 2))}
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
                  <TableCell colSpan={7} className="text-sm text-muted-foreground">No receipts yet. Run PEP requests to generate receipts.</TableCell>
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
