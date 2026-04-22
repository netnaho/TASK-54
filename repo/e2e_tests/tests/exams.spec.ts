/**
 * Competency Exams E2E:
 * - Admin creates an exam template.
 * - Admin creates a session from that template with a specific room name.
 * - Session detail page renders the template title and room.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('admin creates exam template then session', async ({ page }) => {
  await login(page, USERS.admin);

  // ── Create template ──────────────────────────────────────────────
  await page.goto('/exams/templates/new');
  await expect(page.locator('h1.page-title')).toHaveText('New Exam Template');

  await page.fill('#title', 'PW Competency Check');
  await page.fill('#role_scope', 'nurse');

  // Clear defaults and enter specific values
  await page.fill('#passing_score', '75');
  await page.fill('#duration_minutes', '45');
  await page.fill('#window_start', '08:00');
  await page.fill('#window_end', '17:00');

  await page.click('button:has-text("Create Template")');

  // Redirected to template detail
  await expect(page).toHaveURL(/\/exams\/templates\/[a-z0-9-]+/);
  await expect(page.locator('h1.page-title')).toContainText('PW Competency Check');

  // ── Create session from that template ────────────────────────────
  await page.goto('/exams/sessions/new');
  await expect(page.locator('h1.page-title')).toContainText('Generate Exam Session');

  // Select the template we just created (it will be in the dropdown)
  await page.locator('#template_id').selectOption({ label: /PW Competency Check/ });

  // Set a future date/time within the allowed window
  await page.fill('#planned_start', '2030-09-15T09:00');
  await page.fill('#room', 'PW Test Room 101');

  await page.click('button:has-text("Generate Draft Session")');

  // Redirected to session detail
  await expect(page).toHaveURL(/\/exams\/sessions\/[a-z0-9-]+/);

  // Session detail shows template title and room
  await expect(page.locator('h1.page-title')).toContainText('PW Competency Check');
  await expect(page.locator('body')).toContainText('PW Test Room 101');
});
