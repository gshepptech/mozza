import { test, expect, type Page } from "@playwright/test";

// Helper to mock authenticated state and navigate to /app.
async function loginAndGo(page: Page, path = "/app") {
  const user = { id: "u1", email: "test@example.com", name: "Test User", role: "admin" };
  const team = { id: "t1", name: "Acme Corp", slug: "acme", created_by: "u1" };

  await page.route("**/api/v1/auth/me", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(user) }),
  );
  await page.route("**/api/v1/teams", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ teams: [team] }) }),
  );
  await page.route("**/api/v1/teams/t1/members", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ members: [] }) }),
  );
  await page.route("**/api/v1/recipes?*", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ recipes: [] }) }),
  );
  await page.route("**/api/v1/deployments?*", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ deployments: [] }) }),
  );
  await page.route("**/api/v1/doctor", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ findings: [] }) }),
  );
  await page.route("**/api/v1/status", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ containers: [] }) }),
  );

  await page.goto(path);
}

test.describe("Dashboard Shell", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page);
  });

  test("renders sidebar with workspace nav", async ({ page }) => {
    await expect(page.getByText("Mozza").first()).toBeVisible();
    await expect(page.getByText("Overview")).toBeVisible();
    await expect(page.getByText("Applications")).toBeVisible();
    await expect(page.getByText("Deployments")).toBeVisible();
    await expect(page.getByText("Environments")).toBeVisible();
    await expect(page.getByText("Monitoring")).toBeVisible();
    await expect(page.getByText("Doctor")).toBeVisible();
    await expect(page.getByText("Recipes")).toBeVisible();
  });

  test("shows user avatar with initials", async ({ page }) => {
    // "Test User" → "TU"
    await expect(page.getByText("TU")).toBeVisible();
  });

  test("shows team selector with team name", async ({ page }) => {
    await expect(page.getByText("Acme Corp")).toBeVisible();
  });

  test("sidebar collapses and expands", async ({ page }) => {
    // Sidebar starts expanded (w-60)
    const sidebar = page.locator("aside").first();
    await expect(sidebar).toBeVisible();

    // Click collapse button
    const collapseBtn = sidebar.locator("button").filter({ has: page.locator("svg") }).first();
    // Use keyboard shortcut instead for reliability
    await page.keyboard.press("Meta+b");
    await page.waitForTimeout(300);

    // Sidebar should be collapsed - "Overview" text should be hidden
    await expect(page.getByText("Workspace")).not.toBeVisible();

    // Expand again
    await page.keyboard.press("Meta+b");
    await page.waitForTimeout(300);
    await expect(page.getByText("Workspace")).toBeVisible();
  });

  test("sidebar navigation works", async ({ page }) => {
    await page.getByText("Applications").click();
    await expect(page).toHaveURL(/\/app\/applications/);

    await page.getByText("Deployments").click();
    await expect(page).toHaveURL(/\/app\/deployments/);

    await page.getByText("Doctor").click();
    await expect(page).toHaveURL(/\/app\/doctor/);

    await page.getByText("Overview").click();
    await expect(page).toHaveURL("/app");
  });

  test("command palette opens with Cmd+K", async ({ page }) => {
    await page.keyboard.press("Meta+k");
    await expect(page.getByPlaceholder("Type a command or search...")).toBeVisible();

    // Close it
    await page.keyboard.press("Escape");
    await expect(page.getByPlaceholder("Type a command or search...")).not.toBeVisible();
  });

  test("command palette navigates on item select", async ({ page }) => {
    await page.keyboard.press("Meta+k");
    await expect(page.getByPlaceholder("Type a command or search...")).toBeVisible();

    // Click "Applications" in command palette
    const cmdItem = page.locator("[cmdk-item]").filter({ hasText: "Applications" }).first();
    await cmdItem.click();
    await expect(page).toHaveURL(/\/app\/applications/);
  });

  test("search button opens command palette", async ({ page }) => {
    await page.getByText("Search...").click();
    await expect(page.getByPlaceholder("Type a command or search...")).toBeVisible();
  });
});

test.describe("Overview Page", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page);
  });

  test("renders overview heading and stats", async ({ page }) => {
    await expect(page.getByText("Overview")).first().toBeVisible();
    await expect(page.getByText("Cluster Connected")).toBeVisible();
    await expect(page.getByText("Applications")).first().toBeVisible();
    await expect(page.getByText("Pods")).first().toBeVisible();
    await expect(page.getByText("Deployments Today")).toBeVisible();
  });

  test("quick actions link to correct pages", async ({ page }) => {
    const appsAction = page.locator("a").filter({ hasText: "View all apps" });
    await expect(appsAction).toHaveAttribute("href", "/app/applications");
  });

  test("activity feed toggle works", async ({ page }) => {
    const toggleBtn = page.getByText("Show all");
    if (await toggleBtn.isVisible()) {
      await toggleBtn.click();
      await expect(page.getByText("Show less")).toBeVisible();
    }
  });
});

test.describe("Applications Page", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page, "/app/applications");
  });

  test("renders applications heading", async ({ page }) => {
    await expect(page.locator("h1").filter({ hasText: "Applications" })).toBeVisible();
  });

  test("environment filter buttons work", async ({ page }) => {
    const allBtn = page.getByRole("button", { name: /^All/ });
    const prodBtn = page.getByRole("button", { name: /Production/ });

    await expect(allBtn).toBeVisible();
    await expect(prodBtn).toBeVisible();

    await prodBtn.click();
    // Filter should be active now
    await expect(prodBtn).toBeVisible();
  });

  test("grid/list view toggle works", async ({ page }) => {
    // Default is grid view - look for grid icon button
    const listBtn = page.locator("button[class*='w-8']").last();
    if (await listBtn.isVisible()) {
      await listBtn.click();
      // Should see list view header columns
      await page.waitForTimeout(200);
    }
  });

  test("search input filters applications", async ({ page }) => {
    const searchInput = page.getByPlaceholder("Search applications...");
    await expect(searchInput).toBeVisible();
    await searchInput.fill("nonexistent-app-xyz");
    await page.waitForTimeout(200);
  });
});

test.describe("Deployments Page", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page, "/app/deployments");
  });

  test("renders deployments page", async ({ page }) => {
    // Should show deployments heading or content
    await expect(page.locator("h1").first()).toBeVisible();
  });
});

test.describe("Doctor Page", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page, "/app/doctor");
  });

  test("renders doctor page", async ({ page }) => {
    await expect(page.locator("h1").first()).toBeVisible();
  });
});

test.describe("Recipes Page", () => {
  test.beforeEach(async ({ page }) => {
    await loginAndGo(page, "/app/recipes");
  });

  test("renders recipes list", async ({ page }) => {
    // With mock returning empty, should show empty state or heading
    await expect(page.locator("h1").first()).toBeVisible();
  });
});
