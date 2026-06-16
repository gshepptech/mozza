import { test, expect } from "@playwright/test";

test.describe("Authentication", () => {
  test.describe("Login Page", () => {
    test.beforeEach(async ({ page }) => {
      await page.goto("/login");
    });

    test("renders login form", async ({ page }) => {
      await expect(page.getByText("Sign in")).toBeVisible();
      await expect(page.getByLabel("Email")).toBeVisible();
      await expect(page.getByLabel("Password")).toBeVisible();
      await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
    });

    test("has link to register page", async ({ page }) => {
      await page.getByRole("link", { name: /create one/i }).click();
      await expect(page).toHaveURL(/\/register/);
    });

    test("has link back to home", async ({ page }) => {
      await page.getByRole("link", { name: /back to home/i }).click();
      await expect(page).toHaveURL("/");
    });

    test("shows validation on empty submit", async ({ page }) => {
      // HTML5 validation should prevent submit with empty required fields
      const email = page.getByLabel("Email");
      await expect(email).toHaveAttribute("required", "");
    });

    test("shows error on invalid credentials", async ({ page }) => {
      // Mock a failed login response
      await page.route("**/api/v1/auth/login", (route) =>
        route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ error: "Invalid credentials" }),
        }),
      );

      await page.getByLabel("Email").fill("bad@example.com");
      await page.getByLabel("Password").fill("wrongpass");
      await page.getByRole("button", { name: /sign in/i }).click();

      await expect(page.getByText("Invalid credentials")).toBeVisible();
    });

    test("redirects to /app on successful login", async ({ page }) => {
      await page.route("**/api/v1/auth/login", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ id: "u1", email: "test@example.com", name: "Test User", role: "admin" }),
        }),
      );
      await page.route("**/api/v1/auth/me", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ id: "u1", email: "test@example.com", name: "Test User", role: "admin" }),
        }),
      );
      await page.route("**/api/v1/teams", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ teams: [] }),
        }),
      );

      await page.getByLabel("Email").fill("test@example.com");
      await page.getByLabel("Password").fill("password123");
      await page.getByRole("button", { name: /sign in/i }).click();

      await expect(page).toHaveURL(/\/app/);
    });
  });

  test.describe("Register Page", () => {
    test.beforeEach(async ({ page }) => {
      await page.goto("/register");
    });

    test("renders registration form", async ({ page }) => {
      await expect(page.getByText("Create account")).toBeVisible();
      await expect(page.getByLabel("Name")).toBeVisible();
      await expect(page.getByLabel("Email")).toBeVisible();
      await expect(page.getByLabel("Password")).toBeVisible();
      await expect(page.getByRole("button", { name: /create account/i })).toBeVisible();
    });

    test("has link to login page", async ({ page }) => {
      await page.getByRole("link", { name: /sign in/i }).click();
      await expect(page).toHaveURL(/\/login/);
    });

    test("shows error on registration failure", async ({ page }) => {
      await page.route("**/api/v1/auth/register", (route) =>
        route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({ error: "Email already in use" }),
        }),
      );

      await page.getByLabel("Name").fill("Test User");
      await page.getByLabel("Email").fill("existing@example.com");
      await page.getByLabel("Password").fill("password123");
      await page.getByRole("button", { name: /create account/i }).click();

      await expect(page.getByText("Email already in use")).toBeVisible();
    });

    test("redirects to /app on successful registration", async ({ page }) => {
      await page.route("**/api/v1/auth/register", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ id: "u1", email: "new@example.com", name: "New User", role: "user" }),
        }),
      );
      await page.route("**/api/v1/auth/me", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ id: "u1", email: "new@example.com", name: "New User", role: "user" }),
        }),
      );
      await page.route("**/api/v1/teams", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ teams: [] }),
        }),
      );

      await page.getByLabel("Name").fill("New User");
      await page.getByLabel("Email").fill("new@example.com");
      await page.getByLabel("Password").fill("password123");
      await page.getByRole("button", { name: /create account/i }).click();

      await expect(page).toHaveURL(/\/app/);
    });
  });

  test.describe("Protected Routes", () => {
    test("redirects unauthenticated users to /login", async ({ page }) => {
      await page.route("**/api/v1/auth/me", (route) =>
        route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: "Unauthorized" }) }),
      );

      await page.goto("/app");
      await expect(page).toHaveURL(/\/login/);
    });
  });
});
