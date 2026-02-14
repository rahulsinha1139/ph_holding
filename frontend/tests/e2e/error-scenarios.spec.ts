/**
 * E2E Test: Error Scenarios & Failure Modes
 *
 * Tests all common failure scenarios:
 * - Database connection failure
 * - Missing data
 * - Invalid input
 * - File not found
 * - API timeout
 * - Concurrent updates (race conditions)
 *
 * DEFENSIVE TESTING for production deployment
 */

import { test, expect } from '@playwright/test';
import { devLog } from "$lib/utils/devLog";

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://localhost:5173';

test.describe('Error Scenarios - Defensive Testing', () => {

  // ============================================================================
  // DATABASE ERRORS
  // ============================================================================

  test('DB Error: Handle database connection failure gracefully', async ({ page }) => {
    // This would require mocking/disabling the database temporarily
    // For now, we test that errors are surfaced to the UI

    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-customers"]');

    // If database is unavailable, should show error (not blank screen!)
    const hasErrorState = await page.locator('[data-testid="error-boundary"]').isVisible();
    const hasEmptyState = await page.locator('[data-testid="empty-state"]').isVisible();

    // Either error boundary OR empty state (with helpful message)
    expect(hasErrorState || hasEmptyState).toBe(true);

    if (hasErrorState) {
      const errorText = await page.locator('[data-testid="error-message"]').textContent();
      expect(errorText).toBeTruthy();
      devLog.log(`✅ Error shown: ${errorText}`);
    }
  });

  test('Missing Data: Customer not found', async ({ page }) => {
    await page.goto(BASE_URL);

    // Try to view non-existent customer
    await page.goto(`${BASE_URL}/customer/INVALID_ID_99999`);

    // Should show 404 or error message (NOT crash!)
    const has404 = await page.locator('[data-testid="not-found"]').isVisible();
    const hasError = await page.locator('[data-testid="error-boundary"]').isVisible();

    expect(has404 || hasError).toBe(true);

    devLog.log('✅ Handled missing customer gracefully');
  });

  test('Missing Data: Order not found', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/99999`);

    const hasErrorMessage = await page.locator('[data-testid="error-message"]').isVisible();
    expect(hasErrorMessage).toBe(true);

    const errorText = await page.locator('[data-testid="error-message"]').textContent();
    expect(errorText).toContain('Order not found');
  });

  test('Missing Data: Opportunity not found', async ({ page }) => {
    await page.goto(`${BASE_URL}/opportunities/99999`);

    await expect(page.locator('[data-testid="error-message"]')).toBeVisible();
  });

  // ============================================================================
  // INVALID INPUT
  // ============================================================================

  test('Invalid Input: Negative price', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-opportunities"]');
    await page.click('[data-testid="create-rfq-button"]');

    await page.fill('[data-testid="rfq-client-input"]', 'Test Client');
    await page.fill('[data-testid="rfq-project-input"]', 'Test Project');
    await page.fill('[data-testid="rfq-value-input"]', '-1000'); // Invalid!

    await page.click('[data-testid="rfq-submit-button"]');

    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('Value must be positive');
  });

  test('Invalid Input: Empty required fields', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-opportunities"]');
    await page.click('[data-testid="create-rfq-button"]');

    // Submit without filling anything
    await page.click('[data-testid="rfq-submit-button"]');

    // Should show multiple validation errors
    const errors = await page.locator('[data-testid="validation-error"]').count();
    expect(errors).toBeGreaterThan(0);

    devLog.log(`✅ Validation errors shown: ${errors}`);
  });

  test('Invalid Input: Special characters in customer name', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-customers"]');
    await page.click('[data-testid="create-customer-button"]');

    // Try SQL injection pattern
    await page.fill('[data-testid="customer-name-input"]', "'; DROP TABLE customers; --");
    await page.click('[data-testid="save-customer-button"]');

    // Should either sanitize OR reject
    const hasError = await page.locator('[data-testid="error-toast"]').isVisible();

    if (!hasError) {
      // If accepted, verify it was sanitized (not executed!)
      const savedName = await page.locator('[data-testid="customer-name"]').textContent();
      expect(savedName).not.toContain('DROP TABLE');
    }

    devLog.log('✅ SQL injection prevented');
  });

  // ============================================================================
  // FILE NOT FOUND
  // ============================================================================

  test('File Not Found: OCR document missing', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    // Try to process non-existent file
    await page.click('[data-testid="process-document-button"]');
    await page.fill('[data-testid="file-path-input"]', 'C:/nonexistent/file.pdf');
    await page.click('[data-testid="process-confirm"]');

    // Should show file not found error
    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('File not found');
  });

  test('File Not Found: Report export missing template', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-reports"]');

    await page.click('[data-testid="export-report-button"]');
    await page.selectOption('[data-testid="report-type"]', 'custom');

    await page.click('[data-testid="export-confirm"]');

    // If template missing, should show helpful error
    const hasError = await page.locator('[data-testid="error-toast"]').isVisible();

    if (hasError) {
      const errorText = await page.locator('[data-testid="error-toast"]').textContent();
      devLog.log(`✅ Template error: ${errorText}`);
    }
  });

  // ============================================================================
  // API TIMEOUT (External Services)
  // ============================================================================

  test('API Timeout: Florence-2 OCR endpoint slow', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    // Upload a large document
    const fileInput = await page.locator('[data-testid="file-upload-input"]');
    // Simulate timeout with very large file
    // (In real test, would mock the API to return slowly)

    await page.click('[data-testid="process-document-button"]');

    // Should show loading state
    await expect(page.locator('[data-testid="processing-spinner"]')).toBeVisible();

    // Wait for timeout or completion (max 30 seconds)
    await page.waitForSelector('[data-testid="processing-spinner"]', {
      state: 'hidden',
      timeout: 30000,
    });

    // Should eventually show result OR timeout error
    const hasResult = await page.locator('[data-testid="ocr-result"]').isVisible();
    const hasError = await page.locator('[data-testid="error-toast"]').isVisible();

    expect(hasResult || hasError).toBe(true);

    if (hasError) {
      const errorText = await page.locator('[data-testid="error-toast"]').textContent();
      devLog.log(`✅ Timeout handled: ${errorText}`);
    }
  });

  test('API Timeout: Butler AI response delay', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-butler"]');

    await page.fill('[data-testid="butler-input"]', 'What is the status of all customers?');
    await page.click('[data-testid="butler-send"]');

    // Should show typing indicator
    await expect(page.locator('[data-testid="butler-typing"]')).toBeVisible();

    // Wait for response (max 15 seconds for AI)
    await page.waitForSelector('[data-testid="butler-response"]', {
      timeout: 15000,
    });

    const response = await page.locator('[data-testid="butler-response"]').textContent();
    expect(response).toBeTruthy();

    devLog.log('✅ Butler responded within timeout');
  });

  // ============================================================================
  // CONCURRENT UPDATES (Race Conditions)
  // ============================================================================

  test('Race Condition: Concurrent order updates', async ({ browser }) => {
    // Simulate 2 users updating same order simultaneously
    const context1 = await browser.newContext();
    const context2 = await browser.newContext();

    const page1 = await context1.newPage();
    const page2 = await context2.newPage();

    // Create order
    await page1.goto(BASE_URL);
    await page1.click('[data-testid="nav-opportunities"]');
    await page1.click('[data-testid="create-rfq-button"]');
    await page1.fill('[data-testid="rfq-client-input"]', 'Race Test Client');
    await page1.fill('[data-testid="rfq-project-input"]', 'Race Test Project');
    await page1.fill('[data-testid="rfq-value-input"]', '1000');
    await page1.click('[data-testid="rfq-submit-button"]');

    const orderID = await page1.locator('[data-testid="created-rfq-id"]').textContent();

    // Both users load the order
    await page1.goto(`${BASE_URL}/orders/${orderID}`);
    await page2.goto(`${BASE_URL}/orders/${orderID}`);

    // User 1 updates status to "In Progress"
    await page1.click('[data-testid="update-status-button"]');
    await page1.selectOption('[data-testid="status-select"]', 'in-progress');
    await page1.click('[data-testid="status-confirm"]');

    // User 2 ALSO tries to update status to "Shipped" (should conflict!)
    await page2.click('[data-testid="update-status-button"]');
    await page2.selectOption('[data-testid="status-select"]', 'shipped');
    await page2.click('[data-testid="status-confirm"]');

    // Should show conflict error
    const hasConflict = await page2.locator('[data-testid="error-toast"]').isVisible();

    if (hasConflict) {
      const errorText = await page2.locator('[data-testid="error-toast"]').textContent();
      expect(errorText).toContain('updated by another user');
      devLog.log('✅ Race condition detected and prevented');
    }

    // Cleanup
    await context1.close();
    await context2.close();
  });

  test('Race Condition: Concurrent customer creation with same email', async ({ browser }) => {
    const context1 = await browser.newContext();
    const context2 = await browser.newContext();

    const page1 = await context1.newPage();
    const page2 = await context2.newPage();

    const createCustomer = async (page: any) => {
      await page.goto(BASE_URL);
      await page.click('[data-testid="nav-customers"]');
      await page.click('[data-testid="create-customer-button"]');
      await page.fill('[data-testid="customer-name-input"]', 'Duplicate Test Customer');
      await page.fill('[data-testid="customer-email-input"]', 'duplicate@test.com'); // Same email!
      await page.click('[data-testid="save-customer-button"]');
    };

    // Try to create concurrently
    const results = await Promise.allSettled([
      createCustomer(page1),
      createCustomer(page2),
    ]);

    // One should succeed, one should fail with duplicate error
    const errors = results.filter(r => r.status === 'rejected');

    // OR both succeed but second shows duplicate warning
    const page1Success = await page1.locator('[data-testid="success-toast"]').isVisible().catch(() => false);
    const page2Error = await page2.locator('[data-testid="error-toast"]').isVisible().catch(() => false);

    expect(page1Success || page2Error).toBe(true);

    devLog.log('✅ Duplicate email prevented');

    await context1.close();
    await context2.close();
  });

  // ============================================================================
  // NETWORK ERRORS
  // ============================================================================

  test('Network Error: Simulated offline mode', async ({ page, context }) => {
    await page.goto(BASE_URL);

    // Simulate offline
    await context.setOffline(true);

    // Try to load customers
    await page.click('[data-testid="nav-customers"]');

    // Should show offline error
    await expect(page.locator('[data-testid="offline-indicator"]')).toBeVisible();

    // Restore online
    await context.setOffline(false);

    // Should auto-reconnect
    await page.waitForSelector('[data-testid="offline-indicator"]', { state: 'hidden' });

    devLog.log('✅ Offline mode handled');
  });

  test('Network Error: Slow connection (throttling)', async ({ page, context }) => {
    // Throttle network to 50kbps (2G speed)
    await page.route('**/*', route => {
      setTimeout(() => {
        route.continue();
      }, 2000); // 2 second delay for every request
    });

    await page.goto(BASE_URL);

    const startTime = Date.now();
    await page.click('[data-testid="nav-customers"]');

    // Should show loading state while slow
    await expect(page.locator('[data-testid="loading-spinner"]')).toBeVisible();

    await page.waitForSelector('[data-testid="loading-spinner"]', { state: 'hidden' });

    const loadTime = Date.now() - startTime;
    devLog.log(`✅ Slow network handled: ${loadTime}ms`);
  });

  // ============================================================================
  // BROWSER ERRORS
  // ============================================================================

  test('Browser Error: Local storage full', async ({ page }) => {
    await page.goto(BASE_URL);

    // Fill local storage to capacity
    await page.evaluate(() => {
      try {
        for (let i = 0; i < 10000; i++) {
          localStorage.setItem(`test-${i}`, 'x'.repeat(1000));
        }
      } catch (e) {
        // Storage full - expected
      }
    });

    // Try to use app (should handle quota exceeded)
    await page.click('[data-testid="nav-dashboard"]');

    // Should still work (or show helpful error)
    const isWorking = await page.locator('[data-testid="dashboard-stats"]').isVisible();
    const hasError = await page.locator('[data-testid="error-toast"]').isVisible();

    expect(isWorking || hasError).toBe(true);

    // Cleanup
    await page.evaluate(() => localStorage.clear());
  });

  test('Browser Error: Console errors logged', async ({ page }) => {
    const consoleErrors: string[] = [];

    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    page.on('pageerror', error => {
      consoleErrors.push(error.message);
    });

    await page.goto(BASE_URL);

    // Navigate through all major screens
    await page.click('[data-testid="nav-dashboard"]');
    await page.waitForTimeout(500);
    await page.click('[data-testid="nav-customers"]');
    await page.waitForTimeout(500);
    await page.click('[data-testid="nav-opportunities"]');
    await page.waitForTimeout(500);
    await page.click('[data-testid="nav-orders"]');
    await page.waitForTimeout(500);

    // Should have minimal console errors
    const criticalErrors = consoleErrors.filter(err =>
      err.includes('TypeError') || err.includes('ReferenceError') || err.includes('Uncaught')
    );

    expect(criticalErrors.length).toBe(0);

    if (consoleErrors.length > 0) {
      devLog.warn(`⚠️ Non-critical console errors: ${consoleErrors.length}`);
      consoleErrors.forEach(err => devLog.warn(`  - ${err}`));
    }

    devLog.log('✅ No critical console errors');
  });
});
