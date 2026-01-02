const { test, expect } = require("@playwright/test");

const tenantId = process.env.E2E_TENANT_ID ?? "11111111-1111-1111-1111-111111111111";
const roles = process.env.E2E_ROLES ?? "policy_admin,tool_admin,auditor";

test.beforeEach(async ({ page }) => {
  await page.addInitScript(
    ([tenant, roleList]) => {
      localStorage.setItem("umbra.tenant_id", tenant);
      localStorage.setItem("umbra.roles", roleList);
    },
    [tenantId, roles],
  );
});

test("receipts load for seeded tenant", async ({ page }) => {
  await page.goto("/receipts");
  await expect(page.getByTestId("receipts-table")).toBeVisible();
  await expect(page.getByTestId("receipts-row").first()).toBeVisible();
});

test("create tool and see it in list", async ({ page }) => {
  const name = `e2e-tool-${Date.now()}`;
  await page.goto("/tools");
  await page.getByTestId("tool-new").click();
  await page.getByTestId("tool-name").fill(name);
  await page.getByTestId("tool-kind").fill("http");
  await page.getByTestId("tool-config").fill('{"upstream":"http://upstream-sample:9000"}');
  await page.getByTestId("tool-create").click();
  await expect(page.getByTestId("tools-table")).toContainText(name);
});

test("create and activate policy", async ({ page }) => {
  const name = `e2e-policy-${Date.now()}`;
  const policyJson = JSON.stringify({
    version: 1,
    mode: "abac_v0",
    rules: [
      { effect: "allow", roles_any: ["admin", "developer"], methods_any: ["GET"], path_prefix: "/demo" },
    ],
    default: "deny",
  });

  await page.goto("/policies");
  await page.getByTestId("policy-new").click();
  await page.getByTestId("policy-name").fill(name);
  await page.getByTestId("policy-json").fill(policyJson);
  await page.getByTestId("policy-create").click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  const row = page.locator("[data-testid='policies-row']").filter({ hasText: name });
  await expect(row).toBeVisible();
  const activateButton = row.locator("[data-testid^='policy-activate-']");
  await expect(activateButton).toBeVisible();
  await expect(activateButton).toBeEnabled();
  page.once("dialog", (dialog) => dialog.accept());
  await activateButton.click();
  await expect(page.getByTestId("active-policy")).toContainText(name);
});
