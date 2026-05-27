import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

async function workflowState(page: import('@playwright/test').Page) {
  return page.evaluate(() => JSON.parse(window.localStorage.getItem('__ph_e2e_workflow_state__') || '{}'));
}

test.describe('Commercial deployment readiness gates', () => {
  test('management role keeps commercial and operations access but cannot see company financials', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'manager' });
    await page.goto(BASE_URL);

    await expect(page.getByRole('link', { name: 'Opportunities' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Operations' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'People' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Finance' })).toHaveCount(0);

    await page.evaluate(() => { window.location.hash = '#finance'; });
    await expect(page.getByText(/Access denied: You don't have permission to view finance/i)).toBeVisible();

    const cleanupError = await page.evaluate(async () => {
      try {
        await (window as any).go.main.App.PreviewCommercialDeploymentCleanup();
        return '';
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    });
    expect(cleanupError).toContain("permission '*'");
  });

  test('admin clean-deployment action removes supplier actuals and expenses while keeping commercial records', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    const before = await workflowState(page);
    expect(before.supplierInvoices.length).toBeGreaterThan(0);
    expect(before.purchaseOrders.length).toBeGreaterThan(0);
    expect(before.expenseEntries.length).toBeGreaterThan(0);
    expect(before.offers.length).toBeGreaterThan(0);
    expect(before.orders.length).toBeGreaterThan(0);
    expect(before.customerInvoices.length).toBeGreaterThan(0);

    const preview = await page.evaluate(() => (window as any).go.main.App.PreviewCommercialDeploymentCleanup());
    expect(preview.dry_run).toBe(true);
    expect(preview.confirmation_required).toBe('CONFIRM_PH_TRADING_CLEAN_DEPLOYMENT');
    expect(preview.local_total).toBeGreaterThan(0);

    const rejectedConfirmation = await page.evaluate(async () => {
      try {
        await (window as any).go.main.App.ApplyCommercialDeploymentCleanup('NOPE');
        return '';
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    });
    expect(rejectedConfirmation).toContain('confirmation');

    const applied = await page.evaluate(() => (
      (window as any).go.main.App.ApplyCommercialDeploymentCleanup('CONFIRM_PH_TRADING_CLEAN_DEPLOYMENT')
    ));
    expect(applied.dry_run).toBe(false);
    expect(applied.local_total).toBe(preview.local_total);

    const after = await workflowState(page);
    expect(after.supplierInvoices).toHaveLength(0);
    expect(after.supplierPayments).toHaveLength(0);
    expect(after.purchaseOrders).toHaveLength(0);
    expect(after.expenseEntries).toHaveLength(0);
    expect(after.recurringExpenses).toHaveLength(0);
    expect(after.offers.length).toBe(before.offers.length);
    expect(after.orders.length).toBe(before.orders.length);
    expect(after.customerInvoices.length).toBe(before.customerInvoices.length);

    await page.getByRole('link', { name: 'Operations' }).click();
    await page.getByRole('button', { name: /Supplier Invoices/i }).click();
    await expect(page.getByText('6017300440')).toHaveCount(0);

    await page.getByRole('link', { name: 'Finance' }).click();
    await page.getByRole('button', { name: 'Expenses' }).click();
    await expect(page.getByText('Office supplies regression expense')).toHaveCount(0);
  });
});
