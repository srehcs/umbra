import { NextResponse } from 'next/server';
import { isDevTokenModeEnabled, isServerAuthEnabled } from '@/lib/auth';
import {
  clearAccessTokenCookie,
  setAccessTokenCookie,
} from '@/lib/auth/server';

export async function POST(request: Request) {
  if (!isServerAuthEnabled() || !isDevTokenModeEnabled()) {
    return NextResponse.json({ error: 'not available' }, { status: 404 });
  }

  try {
    const body = (await request.json()) as { token?: string };
    const token = body.token?.trim() ?? '';
    if (!token) {
      return NextResponse.json({ error: 'token required' }, { status: 400 });
    }
    const principal = await setAccessTokenCookie(token);
    return NextResponse.json({
      roles: principal.roles,
      tenant_id: principal.tenantId,
      user: { id: principal.userId },
    });
  } catch {
    clearAccessTokenCookie();
    return NextResponse.json({ error: 'unauthorized' }, { status: 401 });
  }
}

export async function DELETE() {
  clearAccessTokenCookie();
  return NextResponse.json({ ok: true });
}
