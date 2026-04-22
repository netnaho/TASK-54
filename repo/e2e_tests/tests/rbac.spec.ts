/**
 * Role-based access denial E2E:
 * - Nurse hitting /finance sees the 403 Access Denied page content.
 * - Nurse can reach /service-delivery (permitted role).
 * - Unauthenticated access to /dashboard redirects to /login.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('nurse sees 403 Access Denied page on /finance', async ({ page }) => {
  await login(page, USERS.nurse);
  await page.goto('/finance');

  // HTTP 403 — page must show the denied content, not a redirect
  await expect(page.locator('.error-code')).toHaveText('403');
  await expect(page.locator('.error-title')).toHaveText('Access Denied');
  await expect(page.locator('.error-message')).toContainText('do not have permission');

  // URL stays at /finance (no redirect to /login)
  await expect(page).toHaveURL('/finance');
});

test('nurse can access /service-delivery', async ({ page }) => {
  await login(page, USERS.nurse);
  await page.goto('/service-delivery');

  await expect(page.locator('h1.page-title')).toContainText('Service Delivery');
});

test('unauthenticated /dashboard redirects to /login', async ({ page }) => {
  // Navigate directly without logging in
  await page.goto('/dashboard');
  await expect(page).toHaveURL('/login');
});
