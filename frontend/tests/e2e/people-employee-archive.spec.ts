import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('People employee archive workflow', () => {
  test('lets admin add and archive an employee without losing the directory record', async ({ page }) => {
    await installMockWailsBridge(page, { currentEmployeeRole: 'admin' });
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'People' }).click();
    await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();

    await page.getByPlaceholder('Full name').first().fill('Archive Demo');
    await page.getByPlaceholder('Department').first().fill('Operations');
    await page.getByPlaceholder('Job title').first().fill('Coordinator');
    await page.getByRole('button', { name: 'Create Employee' }).click();

    await expect(page.getByRole('button', { name: /Archive Demo/ }).first()).toBeVisible();
    await page.getByRole('button', { name: /Archive Demo/ }).first().click();
    await page.getByPlaceholder('Reason for archive').fill('Commercial handoff archive regression');
    await page.getByRole('button', { name: 'Archive Employee' }).click();

    await page.reload();
    await page.getByRole('link', { name: 'People' }).click();
    await page.getByRole('button', { name: 'Archive', exact: true }).click();
    await expect(page.getByRole('button', { name: /Archive Demo/ }).first()).toBeVisible();
    await expect(page.getByText('Archived Employee')).toBeVisible();
    await expect(page.getByText('Commercial handoff archive regression')).toBeVisible();
  });
});
