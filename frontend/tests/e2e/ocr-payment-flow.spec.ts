import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('OCR workflow smoke', () => {
  test.beforeEach(async ({ page }) => {
    await installMockWailsBridge(page);
  });

  test('opens OCR capture from the dashboard and shows Butler analysis for a bank statement', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('button', { name: 'Browse' }).click();

    await expect(page.getByText('Document Type')).toBeVisible();
    await expect(page.getByText('AI Summary:')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Mock Butler analysis: one credit transaction and a clean balance trail.')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save as Bank Statement' })).toBeVisible();
  });

  test('lets the operator inspect extracted transactions and switch the target save type', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('button', { name: 'Browse' }).click();
    await page.getByRole('button', { name: /Transactions \(1\)/ }).click();

    await expect(page.locator('.items-table input[type="text"]').first()).toHaveValue('Deposit');

    await page.getByRole('button', { name: 'Report / Summary' }).click();
    await expect(page.getByRole('button', { name: 'Save as Report / Summary' })).toBeVisible();
  });
});
