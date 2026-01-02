const { defineConfig } = require("@playwright/test");

const baseURL = process.env.E2E_BASE_URL || "http://localhost:3000";

module.exports = defineConfig({
  testDir: "./tests/e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  retries: 0,
  use: {
    baseURL,
    launchOptions: {
      headless: true,
      args: ["--headless=new"],
    },
    trace: "retain-on-failure",
  },
});
