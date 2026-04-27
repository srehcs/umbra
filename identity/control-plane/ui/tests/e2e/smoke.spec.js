const crypto = require('node:crypto');
const { test, expect } = require('@playwright/test');

const tenantId =
  process.env.E2E_TENANT_ID ?? '11111111-1111-1111-1111-111111111111';
const roles = process.env.E2E_ROLES ?? 'policy_admin,tool_admin,auditor';
const authEnabled = process.env.E2E_AUTH_ENABLED === 'true';
const authSecret = process.env.E2E_AUTH_SECRET ?? '';
const authIssuer = process.env.E2E_AUTH_ISSUER ?? '';
const authAudience = process.env.E2E_AUTH_AUDIENCE ?? '';

test.beforeEach(async ({ page }) => {
  if (authEnabled) {
    const token = signHS256Token({
      sub: 'e2e-user',
      ...(authIssuer ? { iss: authIssuer } : {}),
      ...(authAudience ? { aud: authAudience } : {}),
      tenant_id: tenantId,
      roles: roles
        .split(',')
        .map((role) => role.trim())
        .filter(Boolean),
      exp: Math.floor(Date.now() / 1000) + 300,
    });
    await page.goto('/');
    const sessionResult = await page.evaluate(async (value) => {
      const response = await fetch('/api/auth/dev-session', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ token: value }),
      });
      return {
        body: await response.text(),
        ok: response.ok,
        status: response.status,
      };
    }, token);
    if (!sessionResult.ok) {
      throw new Error(
        `failed to establish auth dev session: ${sessionResult.status} ${sessionResult.body}`,
      );
    }
    await page.reload();
    return;
  }

  await page.addInitScript(
    ([tenant, roleList]) => {
      localStorage.setItem('umbra.tenant_id', tenant);
      localStorage.setItem('umbra.roles', roleList);
    },
    [tenantId, roles],
  );
});

function signHS256Token(claims) {
  if (!authSecret) {
    throw new Error('E2E_AUTH_SECRET required when E2E_AUTH_ENABLED=true');
  }
  const header = Buffer.from(
    JSON.stringify({ alg: 'HS256', typ: 'JWT' }),
  ).toString('base64url');
  const payload = Buffer.from(JSON.stringify(claims)).toString('base64url');
  const signed = `${header}.${payload}`;
  const signature = crypto
    .createHmac('sha256', authSecret)
    .update(signed)
    .digest('base64url');
  return `${signed}.${signature}`;
}

test('receipts load for seeded tenant', async ({ page }) => {
  await page.goto('/receipts');
  await expect(page.getByTestId('receipts-table')).toBeVisible();
  await expect(page.getByTestId('receipts-row').first()).toBeVisible();
});

test('create tool and see it in list', async ({ page }) => {
  const name = `e2e-tool-${Date.now()}`;
  await page.goto('/tools');
  await page.getByTestId('tool-new').click();
  await page.getByTestId('tool-name').fill(name);
  await page.getByTestId('tool-kind').fill('http');
  await page
    .getByTestId('tool-config')
    .fill('{"upstream":"http://upstream-sample:9000"}');
  await page.getByTestId('tool-create').click();
  await expect(page.getByTestId('tools-table')).toContainText(name);
});

test('create and activate policy', async ({ page }) => {
  const name = `e2e-policy-${Date.now()}`;
  const policyJson = JSON.stringify({
    version: 1,
    mode: 'abac_v0',
    rules: [
      {
        effect: 'allow',
        roles_any: ['admin', 'developer'],
        methods_any: ['GET'],
        path_prefix: '/demo',
      },
    ],
    default: 'deny',
  });

  await page.goto('/policies');
  await page.getByTestId('policy-new').click();
  await page.getByTestId('policy-name').fill(name);
  await page.getByTestId('policy-json').fill(policyJson);
  await page.getByTestId('policy-create').click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);

  const row = page
    .locator("[data-testid='policies-row']")
    .filter({ hasText: name });
  await expect(row).toBeVisible();
  const activateButton = row.locator("[data-testid^='policy-activate-']");
  await expect(activateButton).toBeVisible();
  await expect(activateButton).toBeEnabled();
  page.once('dialog', (dialog) => dialog.accept());
  await activateButton.click();
  await expect(page.getByTestId('active-policy')).toContainText(name);
});
