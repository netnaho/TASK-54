import { Page, expect } from '@playwright/test';

export const USERS = {
  admin:        { email: 'admin@careops.local',      password: 'Admin!2345' },
  nurse:        { email: 'nurse@careops.local',       password: 'Nurse!2345' },
  finance:      { email: 'finance@careops.local',     password: 'Finance!2345' },
  training:     { email: 'training@careops.local',    password: 'Training!2345' },
};

export async function login(page: Page, user: { email: string; password: string }) {
  await page.goto('/login');
  await page.fill('#email', user.email);
  await page.fill('#password', user.password);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL('/dashboard');
}
