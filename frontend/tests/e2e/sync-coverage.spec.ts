import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('Sync coverage controls', () => {
  test('admin can inspect and repair remote schema coverage from settings', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Settings' }).click();
    await page.getByRole('button', { name: 'Remote Sync' }).click();

    await expect(page.getByRole('heading', { name: 'Remote PostgreSQL Sync' })).toBeVisible();
    await expect(page.getByText('Schema Coverage')).toBeVisible();
    await expect(page.getByText('Canonical Tables: 104')).toBeVisible();
    await expect(page.getByText('Remote Missing: 0')).toBeVisible();
    await expect(page.getByText('Local-only tables: ocr_documents')).toBeVisible();

    await page.getByRole('button', { name: 'Repair Remote Schema' }).click();
    await expect(page.getByText('Remote sync schema repaired')).toBeVisible();
  });
});
