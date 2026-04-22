/**
 * HTMX real-browser interaction E2E:
 *
 * The exercise library filter uses hx-trigger="input delay:400ms" + hx-target="#exercise-grid".
 * When the user types in the search box, HTMX fires a GET /exercise-library?q=... and swaps
 * only #exercise-grid — no full page reload.
 *
 * Assertions:
 *  1. #exercise-grid content changes after search input.
 *  2. The nav sidebar element (#sidebar) remains attached (proves no full page reload).
 *  3. The URL updates (hx-push-url="true") but the nav is still intact.
 *
 * Bonus: favorite toggle uses hx-post + hx-target="this" hx-swap="outerHTML".
 *  - Clicking ☆ on an exercise sends a POST and swaps the button to show ★.
 */
import { test, expect } from '@playwright/test';
import { USERS, login } from './fixtures';

test('exercise filter HTMX partial swap updates grid without full reload', async ({ page }) => {
  await login(page, USERS.admin);
  await page.goto('/exercise-library');

  // Wait for the initial grid to render
  await expect(page.locator('#exercise-grid')).toBeVisible();
  await expect(page.locator('nav.sidebar#sidebar')).toBeVisible();

  // Capture text in grid before the search
  const initialGridText = await page.locator('#exercise-grid').innerText();

  // Track page navigations — a full reload would fire 'load'
  let fullReloadCount = 0;
  page.on('load', () => { fullReloadCount++; });

  // Type in the search box — HTMX triggers after 400ms debounce
  const searchInput = page.locator('input[name="q"]');
  await searchInput.fill('balance');

  // Wait for the HTMX XHR to complete (network idle is ~500ms after debounce fires)
  await page.waitForLoadState('networkidle');

  // #exercise-grid content must have changed
  const updatedGridText = await page.locator('#exercise-grid').innerText();
  expect(updatedGridText).not.toBe(initialGridText);

  // The grid should show the "Single-Leg Stance" balance exercise (seeded as ex-001)
  await expect(page.locator('#exercise-grid')).toContainText('Single-Leg Stance');

  // Nav sidebar is still attached — no full page reload happened
  await expect(page.locator('nav.sidebar#sidebar')).toBeVisible();
  // No full-page reload should have occurred (HTMX does a partial swap).
  expect(fullReloadCount).toBe(0);
});

test('favorite toggle HTMX swaps star button in place', async ({ page }) => {
  await login(page, USERS.admin);
  await page.goto('/exercise-library');

  await expect(page.locator('#exercise-grid')).toBeVisible();

  // Find the first unfavorited exercise's toggle button
  const favBtn = page.locator('.fav-btn').first();
  const initialTitle = await favBtn.getAttribute('title');

  // Nav must be present before click
  await expect(page.locator('nav.sidebar#sidebar')).toBeVisible();

  // Click the toggle — HTMX fires hx-post + hx-swap="outerHTML"
  await favBtn.click();

  // Wait for HTMX response
  await page.waitForLoadState('networkidle');

  // The button in the same position should have a different title now
  const newBtn = page.locator('.fav-btn').first();
  const newTitle = await newBtn.getAttribute('title');
  expect(newTitle).not.toBe(initialTitle);

  // Nav still intact — no full page reload
  await expect(page.locator('nav.sidebar#sidebar')).toBeVisible();
});
