export const ROLE_STORAGE_KEY = 'umbra.roles';
export const USER_STORAGE_KEY = 'umbra.user';
export const TENANT_STORAGE_KEY = 'umbra.tenant_id';
export const AUTH_STATE_EVENT = 'umbra-auth-state-change';

export function parseRoles(input?: string | null): string[] {
  if (!input) return [];
  return input
    .split(',')
    .map((role) => role.trim().toLowerCase())
    .filter(Boolean);
}

export function getStoredTenant(): string | undefined {
  if (typeof window === 'undefined') return undefined;
  const tenant = window.localStorage.getItem(TENANT_STORAGE_KEY)?.trim();
  return tenant ? tenant : undefined;
}

export function notifyAuthStateChanged() {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(AUTH_STATE_EVENT));
}
