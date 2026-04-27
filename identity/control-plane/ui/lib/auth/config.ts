function envEnabled(raw: string | undefined): boolean {
  switch ((raw ?? '').trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
    case 'on':
      return true;
    default:
      return false;
  }
}

function firstDefined(
  ...values: Array<string | undefined>
): string | undefined {
  for (const value of values) {
    if (value !== undefined) {
      return value;
    }
  }
  return undefined;
}

export function isClientAuthEnabled(): boolean {
  return envEnabled(process.env.NEXT_PUBLIC_AUTH_ENABLED);
}

export function isServerAuthEnabled(): boolean {
  return envEnabled(
    firstDefined(process.env.AUTH_ENABLED, process.env.UMBRA_AUTH_ENABLED),
  );
}

export function isDevTokenModeEnabled(): boolean {
  return envEnabled(process.env.NEXT_PUBLIC_AUTH_DEV_TOKEN_ENABLED);
}
