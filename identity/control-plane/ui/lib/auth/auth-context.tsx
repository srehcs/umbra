'use client';

import * as React from 'react';
import { z } from 'zod';
import { isClientAuthEnabled } from './config';
import {
  AUTH_STATE_EVENT,
  getStoredTenant,
  parseRoles,
  ROLE_STORAGE_KEY,
  USER_STORAGE_KEY,
} from './storage';

type AuthUser = {
  id: string;
  name?: string;
};

type AuthContextValue = {
  enabled: boolean;
  user: AuthUser | null;
  roles: string[];
  tenantId?: string;
  tenantIds: string[];
  hasRole: (...roles: string[]) => boolean;
};

const DEFAULT_ROLES = ['developer', 'tool_admin', 'policy_admin', 'auditor'];

const AuthContext = React.createContext<AuthContextValue | null>(null);

const SessionResponseSchema = z.object({
  user: z
    .object({ id: z.string(), name: z.string().optional() })
    .nullable()
    .optional(),
  roles: z.array(z.string()).optional(),
  tenant_id: z.string().optional(),
});
type SessionResponse = z.infer<typeof SessionResponseSchema>;

function resolveRoles(authEnabled: boolean): string[] {
  if (typeof window === 'undefined') {
    return authEnabled ? [] : DEFAULT_ROLES;
  }
  const stored = parseRoles(localStorage.getItem(ROLE_STORAGE_KEY));
  if (stored.length > 0) return stored;
  const envRoles = parseRoles(process.env.NEXT_PUBLIC_DEV_ROLES);
  if (envRoles.length > 0) return envRoles;
  return authEnabled ? [] : DEFAULT_ROLES;
}

function resolveUser(authEnabled: boolean): AuthUser | null {
  if (typeof window === 'undefined') {
    return authEnabled ? null : { id: 'dev-user' };
  }
  const stored = localStorage.getItem(USER_STORAGE_KEY);
  if (stored && stored.trim().length > 0) {
    return { id: stored.trim() };
  }
  const envUser = process.env.NEXT_PUBLIC_DEV_USER;
  if (envUser && envUser.trim().length > 0) {
    return { id: envUser.trim() };
  }
  return authEnabled ? null : { id: 'dev-user' };
}

function resolveTenant(): string | undefined {
  return getStoredTenant();
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const enabled = isClientAuthEnabled();
  const [roles, setRoles] = React.useState<string[]>(
    enabled ? [] : DEFAULT_ROLES,
  );
  const [user, setUser] = React.useState<AuthUser | null>(
    enabled ? null : { id: 'dev-user' },
  );
  const [tenantId, setTenantId] = React.useState<string | undefined>(undefined);

  React.useEffect(() => {
    let cancelled = false;
    const refreshDev = () => {
      setRoles(resolveRoles(enabled));
      setUser(resolveUser(enabled));
      setTenantId(resolveTenant());
    };
    const refreshSession = async (signal?: AbortSignal) => {
      try {
        const res = await fetch('/api/auth/session', signal ? { signal } : undefined);
        if (!res.ok) {
          if (!cancelled) {
            setRoles([]);
            setUser(null);
            setTenantId(undefined);
          }
          return;
        }
        const json = await res.json();
        const parsed = SessionResponseSchema.safeParse(json);
        if (!parsed.success) {
          if (!cancelled) {
            setRoles([]);
            setUser(null);
            setTenantId(undefined);
          }
          return;
        }
        const data: SessionResponse = parsed.data;
        if (cancelled) return;
        setRoles(data.roles ?? []);
        const normalizedUser = data.user
          ? {
              id: data.user.id,
              ...(data.user.name ? { name: data.user.name } : {}),
            }
          : null;
        setUser(normalizedUser);
        setTenantId(data.tenant_id);
      } catch {
        if (!cancelled) {
          setRoles([]);
          setUser(null);
          setTenantId(undefined);
        }
      }
    };

    if (enabled) {
      const controller = new AbortController();
      void refreshSession(controller.signal);
      const focusHandler = () => refreshSession(controller.signal);
      const authStateHandler = () => refreshSession(controller.signal);
      window.addEventListener('focus', focusHandler);
      window.addEventListener(AUTH_STATE_EVENT, authStateHandler);
      window.addEventListener('storage', authStateHandler);
      return () => {
        cancelled = true;
        controller.abort();
        window.removeEventListener('focus', focusHandler);
        window.removeEventListener(AUTH_STATE_EVENT, authStateHandler);
        window.removeEventListener('storage', authStateHandler);
      };
    }

    refreshDev();
    const storageHandler = () => refreshDev();
    const interval = window.setInterval(refreshDev, 1000);
    window.addEventListener('storage', storageHandler);
    window.addEventListener('focus', storageHandler);
    window.addEventListener(AUTH_STATE_EVENT, storageHandler);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener('storage', storageHandler);
      window.removeEventListener('focus', storageHandler);
      window.removeEventListener(AUTH_STATE_EVENT, storageHandler);
    };
  }, [enabled]);

  const hasRole = React.useCallback(
    (...wanted: string[]) => {
      if (wanted.length === 0) return false;
      const set = new Set(roles.map((role) => role.toLowerCase()));
      return wanted.some((role) => set.has(role.toLowerCase()));
    },
    [roles],
  );

  const value = React.useMemo<AuthContextValue>(() => {
    const base = {
      enabled,
      user,
      roles,
      tenantIds: tenantId ? [tenantId] : [],
      hasRole,
    };
    return tenantId ? { ...base, tenantId } : base;
  }, [enabled, hasRole, roles, tenantId, user]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
