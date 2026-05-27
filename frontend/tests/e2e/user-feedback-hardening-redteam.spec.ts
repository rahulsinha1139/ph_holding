import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

async function workflowState(page: import('@playwright/test').Page) {
  return page.evaluate(() => JSON.parse(window.localStorage.getItem('__ph_e2e_workflow_state__') || '{}'));
}

test.describe('User feedback hardening red-team gates', () => {
  test('new inventory and data-quality workspaces are reachable and populated', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Operations' }).click();
    await page.getByRole('button', { name: 'Inventory' }).click();
    await expect(page.getByRole('heading', { name: 'Inventory' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Stock Position' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Movements' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Pending Fulfillment' })).toBeVisible();
    await expect(page.getByText('FMR20')).toBeVisible();

    await page.getByRole('link', { name: 'Relationships' }).click();
    await page.getByRole('button', { name: 'Data Quality' }).click();
    await expect(page.getByRole('heading', { name: 'Data Quality' })).toBeVisible();
    await expect(page.getByText('Possible duplicate customer group')).toBeVisible();
    await expect(page.getByText('Opportunity is missing a customer link')).toBeVisible();
  });

  test('data-quality decisions are blocked for non-admin users', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'manager' });
    await page.goto(BASE_URL);

    const blocked = await page.evaluate(async () => {
      try {
        const [issue] = await (window as any).go.main.App.PreviewCustomerDataQuality(10);
        await (window as any).go.main.App.ReviewDataQualityIssue(issue, 'resolved', 'manager attempt');
        return '';
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    });
    expect(blocked).toContain('admin');
  });

  test('admin data-quality resolution suppresses the live issue and records history', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);
    await page.getByRole('link', { name: 'Relationships' }).click();
    await page.getByRole('button', { name: 'Data Quality' }).click();

    const duplicateRow = page.locator('.issue-row').filter({ hasText: 'Possible duplicate customer group' });
    await expect(duplicateRow).toBeVisible();
    await duplicateRow.getByPlaceholder('Admin review note').fill('confirmed already handled in cleanup');
    await duplicateRow.getByRole('button', { name: 'Resolve' }).click();

    await expect(duplicateRow).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Review History' })).toBeVisible();
    await expect(page.getByText('confirmed already handled in cleanup')).toBeVisible();
  });

  test('project destructive status changes are blocked for management users', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'manager' });
    await page.goto(BASE_URL);

    const errorText = await page.evaluate(async () => {
      try {
        await (window as any).go.main.App.UpdateCollaborativeProject('proj-1', { status: 'deleted' });
        return '';
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    });

    expect(errorText).toContain('projects:delete');
    const state = await workflowState(page);
    expect(state.collaborativeProjects.find((project: any) => project.id === 'proj-1')?.status).toBe('active');
  });

  test('admin project archive leaves the record but removes it from active project lists', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    await page.evaluate(async () => {
      await (window as any).go.main.App.RefreshCollaborativeWorkspace();
      await (window as any).go.main.App.RefreshCollaborativeWorkspace();
    });
    await page.evaluate(() => (window as any).go.main.App.ArchiveCollaborativeProject('proj-1', 'red-team archive'));
    const active = await page.evaluate(() => (window as any).go.main.App.ListCollaborativeProjects(true));
    const all = await page.evaluate(() => (window as any).go.main.App.ListCollaborativeProjects(false));

    expect(active.some((project: any) => project.id === 'proj-1')).toBe(false);
    expect(all.find((project: any) => project.id === 'proj-1')?.status).toBe('archived');
  });

  test('opportunity commercial edit rejects unauthorized users and ignores hostile fields', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'staff' });
    await page.goto(BASE_URL);

    const blocked = await page.evaluate(async () => {
      try {
        await (window as any).go.main.App.UpdateOpportunityCommercialFields('pipe-1', { title: 'Blocked' });
        return '';
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    });
    expect(blocked).toContain('offers:edit');
  });

  test('admin opportunity commercial edit audits changes without allowing identity override', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    const updated = await page.evaluate(() => (window as any).go.main.App.UpdateOpportunityCommercialFields('pipe-1', {
      title: '  Hardened Gas Analyzer Package  ',
      customer_name: '  BAPCO  ',
      id: 'attacker-id',
      created_by: 'attacker',
    }));

    expect(updated.id).toBe('pipe-1');
    expect(updated.title).toBe('Hardened Gas Analyzer Package');
    expect(updated.customer_name).toBe('BAPCO');
    expect((updated as any).created_by).not.toBe('attacker');

    const state = await workflowState(page);
    expect(state.pipeline.find((item: any) => item.id === 'pipe-1')?.id).toBe('pipe-1');
    expect(state.opportunityComments[0].comment).toContain('Updated fields');
    expect(state.opportunityComments[0].comment).not.toContain('created_by');
  });
});
