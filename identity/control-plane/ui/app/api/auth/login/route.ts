import { NextResponse } from 'next/server';
import { isServerAuthEnabled } from '@/lib/auth';
import { createOIDCRedirectURL } from '@/lib/auth/server';

export async function GET() {
  if (!isServerAuthEnabled()) {
    return NextResponse.json({ error: 'auth disabled' }, { status: 404 });
  }

  try {
    const redirectURL = await createOIDCRedirectURL();
    return NextResponse.redirect(redirectURL);
  } catch {
    return NextResponse.json({ error: 'auth unavailable' }, { status: 503 });
  }
}
