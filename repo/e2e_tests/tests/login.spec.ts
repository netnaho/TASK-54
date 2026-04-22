/**
 * Login flow E2E:
 * - Visit /login, fill credentials, assert redirect to /dashboard.
 * - Assert dashboard heading and sidebar nav items are visible.
 * - Assert bad credentials stay on /login with error message.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('login as admin reaches dashboard with nav', async ({ page }) => {
  await page.goto('/login');

  // Page must show the login form
  await expect(page.locator('h1.auth-title')).toHaveText('Sign In');

  await page.fill('#email', USERS.admin.email);
  await page.fill('#password', USERS.admin.password);
  await page.click('button[type="submit"]');

  // Redirected to dashboard
  await expect(page).toHaveURL('/dashboard');

  // Dashboard heading
  await expect(page.locator('h1.page-title')).toHaveText('Operations Dashboard');

  // Key nav items are rendered
  await expect(page.locator('nav.sidebar')).toBeVisible();
  await expect(page.locator('nav a[href="/beds"]')).toBeVisible();
  await expect(page.locator('nav a[href="/finance"]')).toBeVisible();
  await expect(page.locator('nav a[href="/admissions"]')).toBeVisible();

  // Admin display name visible in sidebar footer
  await expect(page.locator('.sidebar-user-name')).toHaveText('Alex Administrator');
});

test('bad password stays on login with error', async ({ page }) => {
  await page.goto('/login');
  await page.fill('#email', USERS.admin.email);
  await page.fill('#password', 'WrongPassword!');
  await page.click('button[type="submit"]');

  // Must stay on /login
  await expect(page).toHaveURL('/login');

  // Error alert visible
  await expect(page.locator('.alert-danger')).toBeVisible();

  // No session cookie
  const cookies = await page.context().cookies();
  const session = cookies.find(c => c.name === 'careops_session');
  expect(session).toBeUndefined();
});

test('/ redirects to /dashboard when logged in', async ({ page }) => {
  await login(page, USERS.admin);
  await page.goto('/');
  await expect(page).toHaveURL('/dashboard');
});
