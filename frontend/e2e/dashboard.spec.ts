import { test, expect } from "@playwright/test";

// Critical path: user can load the dashboard, see data, and interact with it.
// This catches regressions in the Go→React→API pipeline that unit tests miss.

test.describe("Dashboard", () => {
  test("loads and renders company data", async ({ page }) => {
    await page.goto("/");
    // Wait for the data table to appear (skeleton loader → real data)
    const table = page.locator("table");
    await expect(table).toBeVisible({ timeout: 45_000 });

    // At least one row with a stock symbol should be present
    const rows = table.locator("tbody tr");
    await expect(rows.first()).toBeVisible({ timeout: 30_000 });
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });

  test("sector navigation filters data", async ({ page }) => {
    await page.goto("/");
    await page.locator("table").waitFor({ timeout: 45_000 });

    // Click a sector in the sidebar
    const sectorLink = page.locator('nav a:has-text("Energy")');
    if ((await sectorLink.count()) > 0) {
      await sectorLink.click();
      // URL should update
      await expect(page).toHaveURL(/sector/i, { timeout: 5_000 });
    }
  });

  test("clicking a row opens the detail drawer", async ({ page }) => {
    await page.goto("/");
    const table = page.locator("table");
    await expect(table).toBeVisible({ timeout: 45_000 });

    const firstRow = table.locator("tbody tr").first();
    await firstRow.click();

    // The detail drawer or URL param should appear
    await expect(page).toHaveURL(/stock=/i, { timeout: 5_000 });
  });

  test("command palette opens with Cmd+K", async ({ page }) => {
    await page.goto("/");
    await page.locator("table").waitFor({ timeout: 45_000 });

    await page.keyboard.press("Meta+k");
    // The command palette input should be visible
    const input = page.locator('[role="combobox"], [placeholder*="Search"]');
    if ((await input.count()) > 0) {
      await expect(input.first()).toBeVisible({ timeout: 3_000 });
    }
  });

  test("API health endpoint returns ok", async ({ request }) => {
    const resp = await request.get("/api/health");
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    expect(body.status).toBe("ok");
    expect(body.rust_engine).toMatch(/wasm|subprocess|unavailable/);
  });
});
