'use client';

import * as React from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { isDevTokenModeEnabled, useAuth } from '@/lib/auth';
import {
  notifyAuthStateChanged,
  ROLE_STORAGE_KEY,
  USER_STORAGE_KEY,
} from '@/lib/auth/storage';

export default function AuthDevControls() {
  const { enabled, roles, user } = useAuth();
  const devTokenModeEnabled = isDevTokenModeEnabled();
  const [roleInput, setRoleInput] = React.useState(roles.join(', '));
  const [userInput, setUserInput] = React.useState(user?.id ?? '');
  const [tokenInput, setTokenInput] = React.useState('');

  React.useEffect(() => {
    if (enabled) {
      setTokenInput('');
      return;
    }
    setRoleInput(roles.join(', '));
    setUserInput(user?.id ?? '');
  }, [enabled, roles, user]);

  if (enabled) {
    if (!devTokenModeEnabled) {
      return (
        <div className="space-y-2 rounded-lg border border-border bg-muted/40 p-4 text-xs">
          <div className="text-xs font-semibold">Auth mode</div>
          <p className="text-xs text-muted-foreground">
            Dev bearer-token entry is disabled in this build. Use the configured
            auth provider flow or enable
            <span className="code">
              {' '}
              NEXT_PUBLIC_AUTH_DEV_TOKEN_ENABLED=true
            </span>{' '}
            for local-only testing.
          </p>
        </div>
      );
    }

    async function saveToken() {
      const nextToken = tokenInput.trim();
      if (!nextToken) {
        return;
      }
      await fetch('/api/auth/dev-session', {
        body: JSON.stringify({ token: nextToken }),
        headers: {
          'content-type': 'application/json',
        },
        method: 'POST',
      });
      notifyAuthStateChanged();
    }

    async function clearToken() {
      await fetch('/api/auth/dev-session', {
        method: 'DELETE',
      });
      setTokenInput('');
      notifyAuthStateChanged();
    }

    return (
      <div className="space-y-3 rounded-lg border border-border bg-muted/40 p-4 text-xs">
        <div className="text-xs font-semibold">Dev token</div>
        <div className="space-y-2">
          <Label className="text-xs">Bearer token</Label>
          <Textarea
            value={tokenInput}
            onChange={(e) => setTokenInput(e.target.value)}
            placeholder="Paste a dev HS256 JWT"
            className="min-h-[96px] text-xs"
          />
          <p className="text-xs text-muted-foreground">
            Local-development only. The UI sends this token to{' '}
            <span className="code">/api/auth/session</span> and the
            control-plane proxy.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="secondary" onClick={saveToken}>
            Save
          </Button>
          <Button size="sm" variant="outline" onClick={clearToken}>
            Clear
          </Button>
        </div>
      </div>
    );
  }

  function save() {
    const nextRoles = roleInput.trim();
    if (nextRoles) {
      localStorage.setItem(ROLE_STORAGE_KEY, nextRoles);
    } else {
      localStorage.removeItem(ROLE_STORAGE_KEY);
    }
    const nextUser = userInput.trim();
    if (nextUser) {
      localStorage.setItem(USER_STORAGE_KEY, nextUser);
    } else {
      localStorage.removeItem(USER_STORAGE_KEY);
    }
    notifyAuthStateChanged();
  }

  function clear() {
    localStorage.removeItem(ROLE_STORAGE_KEY);
    localStorage.removeItem(USER_STORAGE_KEY);
    setRoleInput('');
    setUserInput('');
    notifyAuthStateChanged();
  }

  return (
    <div className="space-y-3 rounded-lg border border-border bg-muted/40 p-4 text-xs">
      <div className="text-xs font-semibold">Dev auth</div>
      <div className="space-y-2">
        <Label className="text-xs">Roles (comma-separated)</Label>
        <Input
          value={roleInput}
          onChange={(e) => setRoleInput(e.target.value)}
          placeholder="policy_admin,tool_admin,auditor"
          className="h-8 text-xs"
        />
      </div>
      <div className="space-y-2">
        <Label className="text-xs">User id</Label>
        <Input
          value={userInput}
          onChange={(e) => setUserInput(e.target.value)}
          placeholder="dev-user"
          className="h-8 text-xs"
        />
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="secondary" onClick={save}>
          Save
        </Button>
        <Button size="sm" variant="outline" onClick={clear}>
          Clear
        </Button>
      </div>
    </div>
  );
}
