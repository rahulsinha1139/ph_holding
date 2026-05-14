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

    await expect(page.getByRole('heading', { name: /Good (morning|afternoon|evening), Developer/ })).toBeVisible();
    await expect(page.getByText('Revenue YTD 2026', { exact: true })).toBeVisible();
    await expect(page.getByText('Operating Focus', { exact: true })).toBeVisible();
    await expect(page.getByText('Top Customer Revenue', { exact: true })).toBeVisible();
    await expect(page.getByText('YTD 2026', { exact: true })).toBeVisible();
    await expect(page.getByText('Pipeline by Stage', { exact: true })).toBeVisible();
    await expect(page.getByText('Receivables Aging', { exact: true })).toBeVisible();
    await expect(page.getByText(/YTD 2026 records/)).toBeVisible();
  });
});
