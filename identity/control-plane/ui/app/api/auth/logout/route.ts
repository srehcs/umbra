import { NextResponse } from 'next/server';
import { isServerAuthEnabled } from '@/lib/auth';
import { clearAccessTokenCookie, getOIDCLogoutURL } from '@/lib/auth/server';

export async function GET(request: Request) {
  clearAccessTokenCookie();
  if (!isServerAuthEnabled()) {
    return NextResponse.redirect(new URL('/', request.url));
  }

  try {
    const logoutURL = await getOIDCLogoutURL();
    return NextResponse.redirect(logoutURL);
  } catch {
    return NextResponse.redirect(new URL('/', request.url));
  }
}
