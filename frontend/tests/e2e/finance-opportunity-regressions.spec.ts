import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('Opportunity and finance regression coverage', () => {
  test.beforeEach(async ({ page }) => {
    await installMockWailsBridge(page);
  });

  test('opportunity modal shows extracted items and persists comments', async ({ page }) => {
    test.slow();
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Opportunities' }).click();
    await page.getByRole('button', { name: /ACTIUM PG/i }).first().click();

    await expect(page.locator('#opportunity-modal-title')).toHaveText(/ACTIUM PG \/ 2026-311/i);
    await expect(page.getByText('Line Items (2)')).toBeVisible();
    await expect(page.getByText('Gas analyzer transmitter')).toBeVisible();
    await expect(page.getByText('Sample conditioning panel')).toBeVisible();

    const commentBox = page.getByPlaceholder('Add a comment...');
    await expect(commentBox).toBeEditable({ timeout: 10000 });
    await commentBox.fill('Updated regression note for deployment readiness.');
    await page.getByRole('button', { name: 'Post' }).click();
    await expect(page.getByText('Comment added')).toBeVisible();

    await page.getByText('Close', { exact: true }).click();
    await page.getByRole('button', { name: /ACTIUM PG/i }).first().click();
    await expect(page.getByText('Updated regression note for deployment readiness.').first()).toBeVisible();
    await expect(page.getByLabel('Opportunity change log').getByText('Updated regression note for deployment readiness.')).toBeVisible();
  });

  test('supplier invoice edit flow can mark an invoice paid and request payment entry creation', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Operations' }).click();
    await page.getByRole('button', { name: /Supplier Invoices/i }).click();
    await page.getByRole('button', { name: 'Edit invoice' }).first().click();

    await expect(page.getByText('Edit Supplier Invoice')).toBeVisible();
    const modal = page.locator('.modal-overlay').filter({ hasText: 'Edit Supplier Invoice' }).last();
    await modal.locator('select').nth(1).selectOption('Pending');
    await modal.locator('select').nth(2).selectOption('Paid');
    await modal.locator('select').nth(3).selectOption('Bank Transfer');
    await modal.getByPlaceholder('Transfer / cheque reference').fill('TXN-1001');
    await modal.getByLabel(/Create supplier payment ledger entry/i).check();
    await modal.getByRole('button', { name: 'Save Changes' }).click();

    await expect(page.getByText('Supplier invoice updated and payment entry created')).toBeVisible();
    await expect(page.getByText('Settled').first()).toBeVisible();
  });

  test('opportunity detail can jump into costing while folder and subject stay user-entered', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Opportunities' }).click();
    await page.getByRole('button', { name: /ACTIUM PG/i }).first().click();
    await page.getByRole('button', { name: 'Create Costing Sheet' }).click();

    await expect(page.getByText('Linked Opportunity:')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('2026-311')).toBeVisible();
    await page.getByRole('button', { name: /Compliance, Certificates & VAT/i }).click();
    await expect(page.locator('input[placeholder="Subject line for customer PDF"]')).toHaveValue('Sub: Gas Analyzer Package');
    await expect(page.getByText('Quote Price / Unit (BHD)').first()).toBeVisible();
    await expect(page.getByText('Exch. Rate').first()).toBeVisible();
    await expect(page.getByText('Freight %').first()).toBeVisible();
  });

  test('receipts tab exposes add receipt with invoice and on-account modes', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Finance' }).click();
    await page.getByRole('button', { name: 'Receipts' }).click();

    await expect(page.getByRole('button', { name: '+ Add Receipt' })).toBeVisible();
    await page.getByRole('button', { name: '+ Add Receipt' }).click();

    await expect(page.getByText('Apply Customer Receipt')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Apply to Invoice' })).toBeVisible();
    await page.getByRole('button', { name: 'On Account' }).click();
    const modal = page.getByRole('dialog', { name: 'Apply Customer Receipt' });
    await expect(modal.getByText('Select Customer')).toBeVisible();
    await expect(modal.locator('select').first()).toContainText('BAPCO');
  });

  test('accounting workspace exposes operational ledgers and P&L', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Finance' }).click();
    await page.getByRole('button', { name: 'Accounting' }).click();

    await expect(page.getByRole('heading', { name: 'Accounting' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'General Ledger' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Customer Ledger' })).toBeVisible();
    await page.getByRole('button', { name: 'P&L' }).click();
    await expect(page.getByText('Cost of Goods Sold').first()).toBeVisible();
    await expect(page.getByText('Operating Expenses').first()).toBeVisible();
    await expect(page.getByText('Net Income').first()).toBeVisible();
  });

  test('bank recon can allocate a credit line to a customer receipt', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Finance' }).click();
    await page.getByRole('button', { name: 'Bank Recon' }).click();
    await page.getByText('NBB-2026-03').click();
    await page.getByText('Customer receipt - BAPCO').click();

    const modal = page.getByRole('dialog', { name: 'Match Transaction' });
    await expect(modal).toBeVisible();
    await modal.locator('select').first().selectOption('CUSTOMER_RECEIPT');
    await expect(modal.getByText('RCT-2026-0001')).toBeVisible();
    await modal.getByRole('button', { name: 'Add' }).first().click();
    await expect(modal.getByText(/Remaining 0\.000 BHD/)).toBeVisible();
    await modal.getByRole('button', { name: 'Match Allocations' }).click();
    await expect(page.getByText('Transaction allocated successfully')).toBeVisible();
  });

  test('expenses page shows deletable and locked expense states', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Finance' }).click();
    await page.getByRole('button', { name: 'Expenses' }).click();

    await expect(page.getByText('Office supplies regression expense')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Delete' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Locked' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Locked' }).first()).toHaveAttribute('title', 'Posted or paid expenses are locked for audit');
  });

  test('offer header edits persist after saving in the view modal', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Opportunities' }).click();
    await page.getByRole('tab', { name: /Offers/i }).click();
    await page.getByRole('button', { name: 'View' }).first().click();

    await expect(page.getByText('Offer Details')).toBeVisible();
    await page.getByPlaceholder('Folder #').fill('64-26');
    await page.getByPlaceholder('RFQ / enquiry ref').fill('EH-64-26-R0');
    await page.getByPlaceholder('e.g. Net 30').fill('30 days from Date of Delivery');
    await page.getByPlaceholder('e.g. DAP Bahrain').fill('Delivery Duty Paid');
    await page.getByRole('button', { name: 'Save Changes' }).click();

    await expect(page.getByText('Offer details saved')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Offer Details' })).not.toBeVisible();
    await page.getByLabel('View full details').first().click();

    await expect(page.getByPlaceholder('Folder #')).toHaveValue('64-26');
    await expect(page.getByPlaceholder('RFQ / enquiry ref')).toHaveValue('EH-64-26-R0');
    await expect(page.getByPlaceholder('e.g. Net 30')).toHaveValue('30 days from Date of Delivery');
    await expect(page.getByPlaceholder('e.g. DAP Bahrain')).toHaveValue('Delivery Duty Paid');
  });
});
