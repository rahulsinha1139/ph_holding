/**
 * E2E Test: Complete Order Flow
 * RFQ → Quote → Offer → Order → Ship
 *
 * Critical Path Testing for Nigeria Deployment
 */

import { test, expect } from '@playwright/test';
import { devLog } from "$lib/utils/devLog";

// Test configuration
const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://localhost:5173';

test.describe('Order Flow - End-to-End', () => {

  test('Happy Path: RFQ → Quote → Offer → Order → Ship', async ({ page }) => {
    await page.goto(BASE_URL);

    // Step 1: Create RFQ
    devLog.log('📝 Step 1: Creating RFQ...');
    await page.click('[data-testid="nav-opportunities"]');
    await page.click('[data-testid="create-rfq-button"]');

    await page.fill('[data-testid="rfq-client-input"]', 'Al Noor General Trading');
    await page.fill('[data-testid="rfq-project-input"]', 'Marine Engine Parts Q1 2025');
    await page.fill('[data-testid="rfq-value-input"]', '12500');
    await page.fill('[data-testid="rfq-notes-input"]', 'Urgent request - ALBA project');

    await page.click('[data-testid="rfq-submit-button"]');

    // Verify RFQ created
    await expect(page.locator('[data-testid="success-toast"]')).toBeVisible({ timeout: 5000 });
    const rfqID = await page.locator('[data-testid="created-rfq-id"]').textContent();
    devLog.log(`✅ RFQ Created: #${rfqID}`);

    // Step 2: Create Costing Sheet
    devLog.log('💰 Step 2: Creating costing sheet...');
    await page.click(`[data-testid="rfq-${rfqID}-create-costing"]`);

    // Add items
    await page.click('[data-testid="add-costing-item"]');
    await page.fill('[data-testid="item-0-description"]', 'Marine Engine Lubricant 20W-50');
    await page.fill('[data-testid="item-0-quantity"]', '100');
    await page.fill('[data-testid="item-0-unit-price"]', '150');
    await page.fill('[data-testid="item-0-margin"]', '12'); // Grade A customer

    await page.click('[data-testid="add-costing-item"]');
    await page.fill('[data-testid="item-1-description"]', 'Oil Filter - Marine Grade');
    await page.fill('[data-testid="item-1-quantity"]', '50');
    await page.fill('[data-testid="item-1-unit-price"]', '45');
    await page.fill('[data-testid="item-1-margin"]', '12');

    await page.click('[data-testid="save-costing-button"]');

    const costingID = await page.locator('[data-testid="created-costing-id"]').textContent();
    devLog.log(`✅ Costing Sheet Created: #${costingID}`);

    // Step 3: Approve Costing Sheet
    devLog.log('✅ Step 3: Approving costing sheet...');
    await page.click(`[data-testid="costing-${costingID}-approve"]`);
    await expect(page.locator('[data-testid="costing-status"]')).toHaveText('Approved');

    // Step 4: Create Offer
    devLog.log('📄 Step 4: Creating offer...');
    await page.click(`[data-testid="costing-${costingID}-create-offer"]`);

    const offerID = await page.locator('[data-testid="created-offer-id"]').textContent();
    devLog.log(`✅ Offer Created: #${offerID}`);

    // Step 5: Send Offer (simulate email)
    devLog.log('📧 Step 5: Sending offer to customer...');
    await page.click(`[data-testid="offer-${offerID}-send"]`);
    await page.fill('[data-testid="offer-email-to"]', 'procurement@alnoor.bh');
    await page.click('[data-testid="offer-send-confirm"]');

    await expect(page.locator('[data-testid="offer-status"]')).toHaveText('Sent');

    // Step 6: Convert to Order
    devLog.log('🛒 Step 6: Converting offer to order...');
    await page.click(`[data-testid="offer-${offerID}-convert-to-order"]`);

    const orderID = await page.locator('[data-testid="created-order-id"]').textContent();
    devLog.log(`✅ Order Created: #${orderID}`);

    // Step 7: Ship Order
    devLog.log('🚚 Step 7: Creating shipment...');
    await page.goto(`${BASE_URL}/orders/${orderID}`);
    await page.click('[data-testid="create-shipment-button"]');
    await page.fill('[data-testid="tracking-number-input"]', 'BH-SHIP-2025-001');
    await page.click('[data-testid="shipment-confirm"]');

    await expect(page.locator('[data-testid="order-stage"]')).toHaveText('Shipped');

    devLog.log('🎉 COMPLETE: Order flow finished successfully!');
  });

  test('Error Case: RFQ with missing required fields', async ({ page }) => {
    await page.goto(BASE_URL);

    await page.click('[data-testid="nav-opportunities"]');
    await page.click('[data-testid="create-rfq-button"]');

    // Leave client name empty
    await page.fill('[data-testid="rfq-project-input"]', 'Test Project');
    await page.click('[data-testid="rfq-submit-button"]');

    // Should show validation error
    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('Client name is required');
  });

  test('Error Case: Costing sheet with invalid margin', async ({ page }) => {
    await page.goto(BASE_URL);

    // Create RFQ first
    await page.click('[data-testid="nav-opportunities"]');
    await page.click('[data-testid="create-rfq-button"]');
    await page.fill('[data-testid="rfq-client-input"]', 'Test Customer');
    await page.fill('[data-testid="rfq-project-input"]', 'Test Project');
    await page.fill('[data-testid="rfq-value-input"]', '1000');
    await page.click('[data-testid="rfq-submit-button"]');

    const rfqID = await page.locator('[data-testid="created-rfq-id"]').textContent();

    // Create costing with negative margin
    await page.click(`[data-testid="rfq-${rfqID}-create-costing"]`);
    await page.click('[data-testid="add-costing-item"]');
    await page.fill('[data-testid="item-0-description"]', 'Test Item');
    await page.fill('[data-testid="item-0-quantity"]', '1');
    await page.fill('[data-testid="item-0-unit-price"]', '100');
    await page.fill('[data-testid="item-0-margin"]', '-50'); // Invalid!

    await page.click('[data-testid="save-costing-button"]');

    // Should show error
    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('Margin must be positive');
  });

  test('Race Condition: Concurrent RFQ creation', async ({ browser }) => {
    // Simulate 3 users creating RFQs simultaneously
    const contexts = await Promise.all([
      browser.newContext(),
      browser.newContext(),
      browser.newContext(),
    ]);

    const pages = await Promise.all(contexts.map(ctx => ctx.newPage()));

    const createRFQ = async (page: any, clientName: string) => {
      await page.goto(BASE_URL);
      await page.click('[data-testid="nav-opportunities"]');
      await page.click('[data-testid="create-rfq-button"]');
      await page.fill('[data-testid="rfq-client-input"]', clientName);
      await page.fill('[data-testid="rfq-project-input"]', 'Concurrent Test');
      await page.fill('[data-testid="rfq-value-input"]', '1000');
      await page.click('[data-testid="rfq-submit-button"]');

      await expect(page.locator('[data-testid="success-toast"]')).toBeVisible();
      return page.locator('[data-testid="created-rfq-id"]').textContent();
    };

    const [rfq1, rfq2, rfq3] = await Promise.all([
      createRFQ(pages[0], 'Client A'),
      createRFQ(pages[1], 'Client B'),
      createRFQ(pages[2], 'Client C'),
    ]);

    // All should succeed with different IDs
    expect(rfq1).not.toBe(rfq2);
    expect(rfq2).not.toBe(rfq3);
    expect(rfq1).not.toBe(rfq3);

    devLog.log(`✅ Concurrent creation: #${rfq1}, #${rfq2}, #${rfq3}`);

    // Cleanup
    await Promise.all(contexts.map(ctx => ctx.close()));
  });
});
