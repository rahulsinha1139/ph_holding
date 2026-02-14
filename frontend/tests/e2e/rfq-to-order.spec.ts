import { test, expect } from '@playwright/test';

test.describe('RFQ to Order Pipeline', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the app
    await page.goto('http://localhost:5173');
    
    // If the wizard/setup is visible, we might need to bypass it or handle it
    // For now, assume it lands on dashboard if database is already populated
  });

  test('Create RFQ for BAPCO and generate Quotation', async ({ page }) => {
    // 1. Go to Opportunities
    await page.click('[data-testid="nav-opportunities"]');
    await expect(page).toHaveURL(/.*#\/opportunities/);

    // 2. Click Create RFQ
    await page.click('[data-testid="create-rfq-button"]');
    
    // 3. Fill in details
    await page.fill('[data-testid="rfq-client-input"]', 'BAPCO');
    await page.fill('[data-testid="rfq-project-input"]', 'Refinery Expansion Phase 4');
    await page.fill('[data-testid="rfq-value-input"]', '25000');
    
    // 4. Submit
    await page.click('[data-testid="rfq-submit-button"]');
    
    // 5. Verify success and get ID
    await expect(page.locator('[data-testid="success-toast"]')).toBeVisible();
    
    // 6. Navigate to the new RFQ
    // (In a real app, we'd find it in the list)
    await page.click('text=Refinery Expansion Phase 4');
    
    // 7. Click Create Quotation
    await page.click('[data-testid="create-quotation-btn"]');
    
    // 8. Verify it opened the quotation screen
    await expect(page.locator('text=Quotation for BAPCO')).toBeVisible();
  });
});
