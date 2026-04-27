import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ShieldCheck, FileText, Wrench } from 'lucide-react';
import PageHeader from '@/components/app/page-header';
import StatusBanner from '@/components/app/status-banner';

export default function Home() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Welcome"
        subtitle="This console is optimized for a high-signal V0 demo: tools + policies + receipts, with traceability."
      />

      <StatusBanner
        title="Current auth posture"
        description={
          <>
            In dev mode, this console can use{' '}
            <span className="code">x-umbra-tenant-id</span> from the tenant
            switcher. In auth mode, tenant and roles are derived from verified
            token claims.
          </>
        }
      />

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Wrench className="h-4 w-4" /> Tools
            </CardTitle>
            <CardDescription>
              Register tool surfaces and upstream config.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Badge>CRUD + tenant scoping</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4" /> Policies
            </CardTitle>
            <CardDescription>
              Author and activate ABAC rules for V0.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">default deny</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <FileText className="h-4 w-4" /> Receipts
            </CardTitle>
            <CardDescription>
              Tamper-evident audit trail with correlation IDs.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Badge variant="success">hash-chained</Badge>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Demo checklist</CardTitle>
          <CardDescription>
            Run these steps to show end-to-end control plane behavior.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm">
          <ol className="list-decimal pl-5 space-y-1">
            <li>
              Run <span className="code">make dev</span> then{' '}
              <span className="code">make seed</span>.
            </li>
            <li>Set tenant ID from seed output (sidebar: Set tenant).</li>
            <li>
              Invoke PEP:{' '}
              <span className="code">
                curl -H x-umbra-tenant-id:... http://localhost:8082/tool/demo
              </span>
            </li>
            <li>Open Receipts to show decision + invocation events.</li>
            <li>Open Jaeger for end-to-end trace.</li>
          </ol>
        </CardContent>
      </Card>
    </div>
  );
}
