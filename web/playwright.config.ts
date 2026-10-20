import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  reporter: "list",
  use: { baseURL: "http://127.0.0.1:3016", trace: "retain-on-failure" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    {
      name: "mobile",
      use: { ...devices["iPhone 13"], defaultBrowserType: "chromium" },
    },
  ],
  webServer: {
    command: "pnpm dev --host 127.0.0.1 --port 3016",
    url: "http://127.0.0.1:3016",
    reuseExistingServer: !process.env.CI,
  },
});
