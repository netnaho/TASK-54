/**
 * Config snapshot E2E:
 * - Admin visits /config-versions.
 * - Clicks "Take Snapshot".
 * - After redirect, the snapshot appears in the Snapshots table.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('admin takes config snapshot and it appears in listing', async ({ page }) => {
  await login(page, USERS.admin);

  await page.goto('/config-versions');
  await expect(page.locator('h1.page-title')).toHaveText('Configuration Versions');

  // Click the Take Snapshot button (it's a form submit)
  await page.click('button:has-text("Take Snapshot")');

  // Redirected back to config-versions
  await expect(page).toHaveURL('/config-versions');

  // Snapshot row appears in the Snapshots section
  // The filename always starts with "config_snapshot_"
  await expect(page.locator('.data-table td.td-mono').first()).toContainText('config_snapshot_');
});

test('non-admin (nurse) cannot access /config-versions', async ({ page }) => {
  await login(page, USERS.nurse);
  await page.goto('/config-versions');

  await expect(page.locator('.error-code')).toHaveText('403');
  await expect(page.locator('.error-title')).toHaveText('Access Denied');
});
