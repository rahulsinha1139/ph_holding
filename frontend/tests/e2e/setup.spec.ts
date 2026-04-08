import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('Application shell setup', () => {
  test.beforeEach(async ({ page }) => {
    await installMockWailsBridge(page);
  });

  test('loads the dashboard shell and primary navigation', async ({ page }) => {
    await page.goto(BASE_URL);

    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Opportunities' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Operations' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Intelligence' })).toBeVisible();

    await expect(page.getByText('Overview.')).toBeVisible();
    await expect(page.getByText('TOTAL REVENUE (YTD)')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Browse' })).toBeVisible();
    await expect(page.getByText('Open Opportunities')).toBeVisible();
  });
});
