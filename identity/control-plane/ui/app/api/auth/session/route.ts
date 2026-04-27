import { NextResponse } from 'next/server';
import { isServerAuthEnabled } from '@/lib/auth';
import { authenticateRequest } from '@/lib/auth/server';

const authEnabled = isServerAuthEnabled();

export async function GET() {
  if (!authEnabled) {
    return NextResponse.json({ error: 'auth disabled' }, { status: 404 });
  }

  try {
    const principal = await authenticateRequest();

    return NextResponse.json({
      user: { id: principal.userId },
      roles: principal.roles,
      tenant_id: principal.tenantId,
    });
  } catch {
    return NextResponse.json({ error: 'unauthorized' }, { status: 401 });
  }
}
