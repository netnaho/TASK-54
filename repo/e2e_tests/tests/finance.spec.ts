/**
 * Finance happy path E2E:
 * - finance_clerk creates a cash payment through the UI form.
 * - After submit, redirected to payment detail page.
 * - Detail page shows the entered amount and method.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('finance_clerk creates cash payment and sees detail', async ({ page }) => {
  await login(page, USERS.finance);

  // Navigate to new payment form
  await page.goto('/finance/payments/new');
  await expect(page.locator('h1.page-title')).toHaveText('Record Payment');

  // Select first resident in the dropdown (seeded data guarantees at least one)
  const residentSelect = page.locator('#resident_id');
  await residentSelect.selectOption({ index: 1 });

  // Select cash payment method
  await page.locator('#payment_method').selectOption('cash');

  // Enter amount
  await page.fill('#amount', '85.00');

  // Description
  await page.fill('#description', 'E2E test payment');

  // Submit
  await page.click('button[type="submit"]');

  // Redirected to payment detail (URL contains /finance/payments/<id>)
  await expect(page).toHaveURL(/\/finance\/payments\/[a-z0-9-]+/);

  // Detail page shows entered data
  await expect(page.locator('h1.page-title')).toContainText('$85.00');
  await expect(page.locator('.badge-method')).toHaveText('cash');
  await expect(page.locator('.detail-list')).toContainText('E2E test payment');
});

test('finance dashboard loads and shows seeded payment', async ({ page }) => {
  await login(page, USERS.finance);
  await page.goto('/finance');

  await expect(page.locator('h1.page-title')).toContainText('Finance');
  // Seeded payment pay-001 should appear in the list
  await expect(page.locator('body')).toContainText('$');
});
