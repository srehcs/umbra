import { NextResponse } from 'next/server';
import { isServerAuthEnabled } from '@/lib/auth';
import { exchangeOIDCCode } from '@/lib/auth/server';

export async function GET(request: Request) {
  if (!isServerAuthEnabled()) {
    return NextResponse.json({ error: 'auth disabled' }, { status: 404 });
  }

  const url = new URL(request.url);
  try {
    await exchangeOIDCCode(
      url.searchParams.get('code') ?? '',
      url.searchParams.get('state'),
    );
    return NextResponse.redirect(new URL('/', request.url));
  } catch {
    return NextResponse.redirect(new URL('/?auth_error=callback', request.url));
  }
}
