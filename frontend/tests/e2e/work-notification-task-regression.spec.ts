import { expect, test } from '@playwright/test';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

test.describe('Work notification task handoff', () => {
  test('opens a task from notifications even when the first task-detail fetch misses', async ({ page }) => {
    await installMockWailsBridge(page);
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Notifications' }).click();
    await expect(page.getByRole('heading', { name: 'Notifications.' })).toBeVisible();
    await expect(page.getByText('Review salary expense forecast has been assigned to you.')).toBeVisible();

    await page.getByRole('button', { name: 'Open task' }).click();

    await expect(page.getByRole('heading', { name: 'Work.' })).toBeVisible();
    await expect(page.locator('#modal-title')).toHaveText('Review salary expense forecast');
    await expect(page.getByText('Task Details')).toBeVisible();
    await expect(page.getByText(/task not found/i)).toHaveCount(0);
  });

  test('navigates to work immediately even if marking the notification as read is slow', async ({ page }) => {
    await installMockWailsBridge(page, { notificationReadDelayMs: 2000 });
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Notifications' }).click();
    await expect(page.getByRole('heading', { name: 'Notifications.' })).toBeVisible();

    await page.getByRole('button', { name: 'Open task' }).click();

    await expect(page.getByRole('heading', { name: 'Work.' })).toBeVisible({ timeout: 1000 });
    await expect(page.getByRole('heading', { name: 'Notifications.' })).toHaveCount(0);
    await expect(page.locator('#modal-title')).toHaveText('Review salary expense forecast');
    await expect(page.getByText(/task not found/i)).toHaveCount(0);
  });

  test('opens a team-board task through the retry path instead of failing on the first detail miss', async ({ page }) => {
    await installMockWailsBridge(page, {
      collaborativeDataReadyAfterRefreshCount: 1,
      taskDetailsReadyAfterRefreshCount: 2,
    });
    await page.goto(BASE_URL);

    await page.getByRole('link', { name: 'Work' }).click();
    await expect(page.getByRole('heading', { name: 'Work.' })).toBeVisible();
    await page.getByRole('button', { name: 'Team Board' }).click();
    await expect(page.getByRole('heading', { name: 'Team Board' })).toBeVisible();
    await expect(page.getByText(/Review salary expense forecast/i)).toBeVisible({ timeout: 10000 });

    await page.getByText(/Review salary expense forecast/i).click();

    await expect(page.locator('#modal-title')).toHaveText('Review salary expense forecast');
    await expect(page.getByText(/task not found/i)).toHaveCount(0);
  });
});
