import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: "http://localhost:8000",
    headless: true,
  },
  webServer: {
    command: "cd .. && ./bin/api",
    url: "http://localhost:8000/api/health",
    reuseExistingServer: true,
    timeout: 60_000,
  },
});
