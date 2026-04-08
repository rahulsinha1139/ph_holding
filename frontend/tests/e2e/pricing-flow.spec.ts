import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('Commercial pricing and quotation flow', () => {
  test.beforeEach(async ({ page }) => {
    await installMockWailsBridge(page);
    await page.goto(BASE_URL);
    await page.getByRole('link', { name: 'Opportunities' }).click();
  });

  test('blank costing can be priced and saved as a quoted offer', async ({ page }) => {
    await page.getByRole('tab', { name: /Costing/i }).click();
    await page.getByRole('button', { name: 'Start Blank Costing' }).click();

    await page.locator('select.input').first().selectOption('BAPCO');
    await page.locator('select.input-sm').first().selectOption({ index: 1 });
    await page.getByPlaceholder('Product name').fill('Marine engine lubricant');
    await page.locator('input.money.sell-price').first().fill('1450');

    await page.getByRole('button', { name: 'Save as Offer' }).click();

    await expect(page.getByText(/Offer PHO-2026-002 created!/)).toBeVisible();
    await expect(page.getByRole('cell', { name: 'PHO-2026-002' })).toBeVisible();
    await expect(page.getByRole('cell', { name: '1,595.000 BHD' })).toBeVisible();
  });

  test('quoted offers expose the current commercial actions', async ({ page }) => {
    await page.getByRole('tab', { name: /Offers/i }).click();

    await expect(page.getByRole('cell', { name: 'PHO-2026-001' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Edit offer' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Download PDF' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Mark as won' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Schedule follow-up' }).first()).toBeVisible();
  });

  test('operations purchase order draft keeps pricing totals visible before submit', async ({ page }) => {
    await page.getByRole('link', { name: 'Operations' }).click();
    await page.getByRole('button', { name: /\+ New Purchase Order/ }).click();

    await page.locator('.po-form select.select-input').first().selectOption({ label: 'Endress+Hauser' });
    await page.getByRole('button', { name: '+ Add Item' }).click();
    await page.getByPlaceholder('Product or service description').fill('Temperature transmitter');
    await page.locator('.items-list input[type="number"]').first().fill('3');
    await page.locator('.items-list input[type="number"]').nth(1).fill('850');

    await expect(page.getByText('2550.000 BHD').first()).toBeVisible();
    await expect(page.getByText('2805.000 BHD').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Create PO' })).toBeVisible();
  });
});
