'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { isDevTokenModeEnabled, useAuth } from '@/lib/auth';
import { notifyAuthStateChanged, TENANT_STORAGE_KEY } from '@/lib/auth/storage';

function getInitialTenant() {
  if (typeof window === 'undefined') return '';
  const existing = window.localStorage.getItem(TENANT_STORAGE_KEY);
  if (existing) return existing;
  const seeded = process.env.NEXT_PUBLIC_TENANT_ID || '';
  if (seeded) {
    window.localStorage.setItem(TENANT_STORAGE_KEY, seeded);
    return seeded;
  }
  return '';
}

export default function TenantSwitcher({
  compact = false,
}: {
  compact?: boolean;
}) {
  const { enabled, tenantId } = useAuth();
  const devTokenModeEnabled = isDevTokenModeEnabled();
  const [tenant, setTenant] = React.useState<string>('');
  const [draft, setDraft] = React.useState<string>('');

  React.useEffect(() => {
    const t = getInitialTenant();
    setTenant(t);
    setDraft(t);
  }, []);

  const save = () => {
    const v = draft.trim();
    window.localStorage.setItem(TENANT_STORAGE_KEY, v);
    setTenant(v);
    notifyAuthStateChanged();
  };

  if (enabled) {
    return (
      <div className="flex flex-col gap-2">
        {!compact && (
          <div className="flex flex-col">
            <div className="text-xs text-muted-foreground">Tenant</div>
            <div className="flex items-center gap-2">
              <Badge
                variant={tenantId ? 'outline' : 'warning'}
                className="truncate max-w-[240px]"
              >
                {tenantId ? tenantId : 'missing token'}
              </Badge>
            </div>
          </div>
        )}
        <p className="text-xs text-muted-foreground">
          {tenantId
            ? 'Derived from bearer token claims.'
            : devTokenModeEnabled
              ? 'Set a bearer token to derive tenant claims.'
              : 'Derived tenant claims require the configured auth provider flow.'}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {!compact && (
        <div className="flex flex-col">
          <div className="text-xs text-muted-foreground">Tenant</div>
          <div className="flex items-center gap-2">
            <Badge
              variant={tenant ? 'outline' : 'warning'}
              className="truncate max-w-[240px]"
            >
              {tenant ? tenant : 'not set'}
            </Badge>
          </div>
        </div>
      )}

      <Dialog>
        <DialogTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className={compact ? 'h-8' : 'h-9'}
            data-testid="tenant-set-button"
          >
            {compact ? 'Tenant' : 'Set tenant'}
          </Button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Tenant context (dev mode)</DialogTitle>
            <DialogDescription>
              For V0, the UI uses{' '}
              <span className="code">x-umbra-tenant-id</span>. In production
              this will be derived from OIDC claims.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Input
              placeholder="Paste tenant UUID (from `make seed` output)"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              data-testid="tenant-input"
            />
            <p className="text-xs text-muted-foreground">
              Tip: run <span className="code">make seed</span> and copy TenantA
              or TenantB ID.
            </p>
          </div>

          <DialogFooter>
            <Button onClick={save}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
