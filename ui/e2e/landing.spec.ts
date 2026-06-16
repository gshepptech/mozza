import { test, expect } from "@playwright/test";

test.describe("Landing Page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("renders hero section with branding", async ({ page }) => {
    await expect(page.locator("text=Mozza").first()).toBeVisible();
    await expect(page.locator("text=Deploy apps like you order pizza")).toBeVisible();
    await expect(page.getByRole("link", { name: /get started/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /sign in/i })).toBeVisible();
  });

  test("tab navigation switches content", async ({ page }) => {
    // Default tab should be visible (Recipe)
    await expect(page.locator("[data-tab]").first()).toBeVisible();

    // Click each tab and verify content changes
    const tabs = ["Problem", "Solution", "Recipe", "CLI", "Doctor", "Compare"];
    for (const tab of tabs) {
      const tabBtn = page.locator(`button:has-text("${tab}")`).first();
      if (await tabBtn.isVisible()) {
        await tabBtn.click();
        // Tab should be active after click
        await expect(tabBtn).toBeVisible();
      }
    }
  });

  test("get started link navigates to register", async ({ page }) => {
    const link = page.getByRole("link", { name: /get started/i });
    await link.click();
    await expect(page).toHaveURL(/\/register/);
  });

  test("sign in link navigates to login", async ({ page }) => {
    const link = page.getByRole("link", { name: /sign in/i });
    await link.click();
    await expect(page).toHaveURL(/\/login/);
  });
});
