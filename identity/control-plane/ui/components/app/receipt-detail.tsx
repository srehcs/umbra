import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Receipt } from "@/lib/types";

function Field({ label, value }: { label: string; value: unknown }) {
  const v = value === null || value === undefined || value === "" ? "—" : String(value);
  return (
    <div className="space-y-1">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="text-sm code break-all">{v}</div>
    </div>
  );
}

export default function ReceiptDetail({ r }: { r: Receipt }) {
  const kind = r.kind;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-sm text-muted-foreground">Timestamp</div>
        <div className="code text-xs">{String(r.ts ?? "")}</div>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Summary</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2">
              <Badge variant="outline">{kind}</Badge>
              {kind === "decision" && (
                r.decision === "allow" ? <Badge variant="success">allow</Badge> : <Badge variant="danger">deny</Badge>
              )}
              {kind === "invocation" && (
                r.outcome === "success" ? <Badge variant="success">success</Badge> :
                r.outcome === "denied" ? <Badge variant="warning">denied</Badge> :
                <Badge variant="danger">error</Badge>
              )}
            </div>
            <Field label="Decision ID" value={r.decision_id} />
            <Field label="Trace ID" value={r.trace_id} />
            <Field label="Span ID" value={r.span_id} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Integrity</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Hash" value={r.hash} />
            <Field label="Previous hash" value={r.prev_hash} />
            {kind === "decision" && <Field label="Policy hash" value={r.policy_hash} />}
          </CardContent>
        </Card>
      </div>

      {kind === "invocation" && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Invocation</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <Field label="Tool" value={r.tool_name} />
            <Field label="Method" value={r.method} />
            <Field label="Path" value={r.path} />
            <Field label="Outcome" value={r.outcome} />
            <Field label="Status" value={r.status_code} />
            <Field label="Latency (ms)" value={r.latency_ms} />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
