"use client";

import * as React from "react";

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

const ROLE_STORAGE_KEY = "umbra.roles";
const USER_STORAGE_KEY = "umbra.user";
const TENANT_STORAGE_KEY = "umbra.tenant_id";
const DEFAULT_ROLES = ["developer", "tool_admin", "policy_admin", "auditor"];

const AuthContext = React.createContext<AuthContextValue | null>(null);

type SessionResponse = {
  user?: AuthUser | null;
  roles?: string[];
  tenant_id?: string;
};

function parseRoles(input?: string | null): string[] {
  if (!input) return [];
  return input
    .split(",")
    .map((role) => role.trim().toLowerCase())
    .filter(Boolean);
}

function resolveRoles(authEnabled: boolean): string[] {
  if (typeof window === "undefined") {
    return authEnabled ? [] : DEFAULT_ROLES;
  }
  const stored = parseRoles(localStorage.getItem(ROLE_STORAGE_KEY));
  if (stored.length > 0) return stored;
  const envRoles = parseRoles(process.env.NEXT_PUBLIC_DEV_ROLES);
  if (envRoles.length > 0) return envRoles;
  return authEnabled ? [] : DEFAULT_ROLES;
}

function resolveUser(authEnabled: boolean): AuthUser | null {
  if (typeof window === "undefined") {
    return authEnabled ? null : { id: "dev-user" };
  }
  const stored = localStorage.getItem(USER_STORAGE_KEY);
  if (stored && stored.trim().length > 0) {
    return { id: stored.trim() };
  }
  const envUser = process.env.NEXT_PUBLIC_DEV_USER;
  if (envUser && envUser.trim().length > 0) {
    return { id: envUser.trim() };
  }
  return authEnabled ? null : { id: "dev-user" };
}

function resolveTenant(): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const tenant = localStorage.getItem(TENANT_STORAGE_KEY);
  return tenant && tenant.trim().length > 0 ? tenant : undefined;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const enabled = process.env.NEXT_PUBLIC_AUTH_ENABLED === "true";
  const [roles, setRoles] = React.useState<string[]>(enabled ? [] : DEFAULT_ROLES);
  const [user, setUser] = React.useState<AuthUser | null>(enabled ? null : { id: "dev-user" });
  const [tenantId, setTenantId] = React.useState<string | undefined>(undefined);

  React.useEffect(() => {
    let cancelled = false;
    const refreshDev = () => {
      setRoles(resolveRoles(enabled));
      setUser(resolveUser(enabled));
      setTenantId(resolveTenant());
    };
    const refreshSession = async () => {
      try {
        const res = await fetch("/api/auth/session");
        if (!res.ok) return;
        const data = (await res.json()) as SessionResponse;
        if (cancelled) return;
        setRoles(data.roles ?? []);
        setUser(data.user ?? null);
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
      void refreshSession();
      const focusHandler = () => refreshSession();
      window.addEventListener("focus", focusHandler);
      return () => {
        cancelled = true;
        window.removeEventListener("focus", focusHandler);
      };
    }

    refreshDev();
    const storageHandler = () => refreshDev();
    const interval = window.setInterval(refreshDev, 1000);
    window.addEventListener("storage", storageHandler);
    window.addEventListener("focus", storageHandler);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener("storage", storageHandler);
      window.removeEventListener("focus", storageHandler);
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
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
