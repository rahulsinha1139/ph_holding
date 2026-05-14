import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import { installMockWailsBridge } from './helpers/mockWailsBridge';

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173';

const TOP_LEVEL_NAV = [
  'Dashboard',
  'Opportunities',
  'Operations',
  'Finance',
  'Work',
  'People',
  'Notifications',
  'Relationships',
  'Intelligence',
  'Settings',
];

const CANONICAL_PREPARED_BY = [
  'EBIN CHACKO',
  'Periyasamy Neelakandan',
  'V.M.Sundar',
  'RAMYA BASKARAN',
  'STANLEY VAZ',
  'Abhie Kore',
];

function escapeRegex(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

async function waitForScreenIdle(page: Page) {
  await page.waitForLoadState('domcontentloaded').catch(() => undefined);
  await page.waitForLoadState('networkidle', { timeout: 2_000 }).catch(() => undefined);
  await page.locator('main').first().waitFor({ state: 'visible', timeout: 10_000 }).catch(() => undefined);
  await page.locator('.inline-screen-loader').waitFor({ state: 'hidden', timeout: 10_000 }).catch(() => undefined);
  await page.waitForTimeout(150);
}

async function clickTopNav(page: Page, label: string) {
  await page.getByRole('link', { name: label }).click();
  await waitForScreenIdle(page);
  await expect(page.getByText('Screen did not finish loading')).toHaveCount(0);
  await expect(page.getByText(/Access denied/i)).toHaveCount(0);
}

async function clickButtonLike(page: Page, label: string) {
  const matcher = new RegExp(`^\\s*${escapeRegex(label)}(?:\\s+\\d+)?\\s*$`, 'i');
  const button = page.getByRole('button', { name: matcher }).first();
  if (await button.count()) {
    await button.click();
    await waitForScreenIdle(page);
    return;
  }
  await page.getByText(label, { exact: true }).first().click();
  await waitForScreenIdle(page);
}

async function collectVisibleControls(page: Page) {
  return page.evaluate(() => {
    const root = document.querySelector('.screen-content') || document.body;
    const selector = [
      'button',
      'a[href]',
      'input',
      'select',
      'textarea',
      '[role="button"]',
      '[role="tab"]',
      '[role="link"]',
    ].join(',');

    const isVisible = (element: Element) => {
      const rect = element.getBoundingClientRect();
      const style = window.getComputedStyle(element);
      return rect.width > 0
        && rect.height > 0
        && style.visibility !== 'hidden'
        && style.display !== 'none'
        && Number(style.opacity || '1') > 0;
    };

    const readableName = (element: Element) => {
      const htmlElement = element as HTMLElement;
      const input = element as HTMLInputElement;
      return (
        htmlElement.getAttribute('aria-label')
        || htmlElement.getAttribute('title')
        || input.placeholder
        || input.value
        || htmlElement.innerText
        || htmlElement.textContent
        || htmlElement.getAttribute('name')
        || htmlElement.id
        || element.tagName.toLowerCase()
      ).replace(/\s+/g, ' ').trim();
    };

    return Array.from(root.querySelectorAll(selector))
      .filter(isVisible)
      .map((element) => ({
        tag: element.tagName.toLowerCase(),
        role: element.getAttribute('role') || '',
        type: (element as HTMLInputElement).type || '',
        name: readableName(element),
        disabled: Boolean((element as HTMLButtonElement).disabled || element.getAttribute('aria-disabled') === 'true'),
      }))
      .filter((control) => control.name.length > 0);
  });
}

test.describe('Commercial UI control audit', () => {
  test.setTimeout(120_000);

  test.beforeEach(async ({ page }) => {
    await installMockWailsBridge(page);
  });

  test('inventories visible controls on every top-level page without load failures', async ({ page }, testInfo) => {
    const pageErrors: string[] = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));

    await page.goto(BASE_URL);
    await waitForScreenIdle(page);

    const audit: Record<string, Awaited<ReturnType<typeof collectVisibleControls>>> = {};
    for (const label of TOP_LEVEL_NAV) {
      await expect(page.getByRole('link', { name: label })).toBeVisible();
      await clickTopNav(page, label);
      const controls = await collectVisibleControls(page);
      audit[label] = controls;
      expect(controls.length, `${label} should expose visible interactive controls`).toBeGreaterThan(0);
    }

    const outputPath = testInfo.outputPath('ui-control-audit.json');
    await fs.mkdir(path.dirname(outputPath), { recursive: true });
    await fs.writeFile(outputPath, JSON.stringify(audit, null, 2));

    expect(pageErrors, 'No unhandled browser errors during top-level page audit').toEqual([]);
  });

  test('exercises hub tabs and representative modal/action buttons', async ({ page }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));

    await page.goto(BASE_URL);
    await waitForScreenIdle(page);

    await clickTopNav(page, 'Opportunities');
    for (const tab of ['RFQs', 'Costing', 'Offers', 'Customer Orders']) {
      await clickButtonLike(page, tab);
      expect((await collectVisibleControls(page)).length, `Opportunities/${tab} controls`).toBeGreaterThan(0);
    }

    await clickButtonLike(page, 'RFQs');
    await page.getByRole('button', { name: /ACTIUM PG/i }).first().click();
    await expect(page.locator('#opportunity-modal-title')).toHaveText(/ACTIUM PG/i);
    await page.getByText('Close', { exact: true }).click();

    await clickButtonLike(page, 'Offers');
    await page.getByRole('button', { name: 'View' }).first().click();
    await expect(page.getByRole('heading', { name: 'Offer Details' })).toBeVisible();
    await page.getByRole('button', { name: 'Close', exact: true }).click();

    await clickTopNav(page, 'Operations');
    for (const tab of ['Supplier POs', 'Supplier Invoices', 'Delivery Notes']) {
      await clickButtonLike(page, tab);
      expect((await collectVisibleControls(page)).length, `Operations/${tab} controls`).toBeGreaterThan(0);
    }
    await clickButtonLike(page, 'Supplier POs');
    await page.getByLabel('View PO details').first().click();
    await expect(page.getByLabel('Purchase Order Details')).toBeVisible();
    await page.keyboard.press('Escape');

    await clickTopNav(page, 'Finance');
    for (const tab of ['Dashboard', 'Customer Invoices', 'Receipts', 'Supplier Settlements', 'Expenses', 'Approvals', 'Payroll', 'Bank Recon']) {
      await clickButtonLike(page, tab);
      expect((await collectVisibleControls(page)).length, `Finance/${tab} controls`).toBeGreaterThan(0);
    }

    await clickTopNav(page, 'Work');
    for (const tab of ['My Work', 'Team Board', 'Projects']) {
      await clickButtonLike(page, tab);
      expect((await collectVisibleControls(page)).length, `Work/${tab} controls`).toBeGreaterThan(0);
    }

    await clickTopNav(page, 'Relationships');
    for (const tab of ['Customers', 'Suppliers']) {
      await clickButtonLike(page, tab);
      expect((await collectVisibleControls(page)).length, `Relationships/${tab} controls`).toBeGreaterThan(0);
    }

    expect(pageErrors, 'No unhandled browser errors during hub/action audit').toEqual([]);
  });

  test('costing prepared-by dropdown carries the exact document signature team', async ({ page }) => {
    await page.goto(BASE_URL);
    await waitForScreenIdle(page);

    await clickTopNav(page, 'Opportunities');
    await clickButtonLike(page, 'Costing');
    await page.getByRole('button', { name: 'Start Blank Costing' }).click();

    const preparedBySelect = page.locator('.field-group', { hasText: 'Prepared By' }).locator('select').first();
    await expect(preparedBySelect).toBeVisible();
    const options = await preparedBySelect.locator('option').evaluateAll((nodes) => (
      nodes.map((node) => (node.textContent || '').trim()).filter(Boolean)
    ));

    for (const name of CANONICAL_PREPARED_BY) {
      expect(options, `Prepared By options should include ${name}`).toContain(name);
    }
  });
});
