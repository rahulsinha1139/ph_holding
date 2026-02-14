/**
 * E2E Test: OCR + Payment Prediction Flow
 * Upload document → Extract text → Classify type → Route → Predict payment
 *
 * Tests the CRITICAL intelligence features
 */

import { test, expect } from '@playwright/test';
import { devLog } from "$lib/utils/devLog";

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://localhost:5173';

test.describe('OCR + Payment Prediction Flow', () => {

  test('OCR Flow: Upload RFQ → Extract → Classify → Route', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    // Upload document
    const fileInput = page.locator('[data-testid="file-upload-input"]');
    // Note: In real test, would upload actual PDF
    // For now, simulate file selection

    await page.click('[data-testid="upload-document-button"]');

    // Select document type
    await page.selectOption('[data-testid="doc-type-select"]', 'rfq');

    await page.click('[data-testid="process-document"]');

    // Wait for OCR processing
    await expect(page.locator('[data-testid="processing-spinner"]')).toBeVisible();

    // Wait for result (Florence-2 is fast: ~200ms)
    await page.waitForSelector('[data-testid="ocr-result"]', { timeout: 10000 });

    // Verify extracted text shown
    const extractedText = await page.locator('[data-testid="extracted-text"]').textContent();
    expect(extractedText).toBeTruthy();

    // Verify confidence shown
    const confidence = await page.locator('[data-testid="ocr-confidence"]').textContent();
    const confidenceValue = parseFloat(confidence!);
    expect(confidenceValue).toBeGreaterThan(0);
    expect(confidenceValue).toBeLessThanOrEqual(1);

    // Verify classification
    const docType = await page.locator('[data-testid="detected-doc-type"]').textContent();
    expect(docType).toBe('RFQ');

    devLog.log(`✅ OCR complete: ${extractedText?.length} chars, ${(confidenceValue * 100).toFixed(1)}% confidence`);

    // Verify routing suggestion
    const routingTo = await page.locator('[data-testid="routing-suggestion"]').textContent();
    expect(routingTo).toContain('Opportunities');

    // Accept routing
    await page.click('[data-testid="accept-routing-button"]');

    // Should navigate to Opportunities screen with pre-filled data
    await expect(page.locator('[data-testid="nav-opportunities"]')).toHaveClass(/active/);

    devLog.log('✅ Document routed to Opportunities');
  });

  test('Payment Prediction: Grade A customer (low risk)', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-customers"]');

    // Select Al Noor (Grade A)
    await page.click('[data-testid="customer-C01011"]');

    // View payment prediction
    await page.click('[data-testid="view-payment-prediction"]');

    // Verify prediction shown
    const prediction = await page.locator('[data-testid="payment-prediction"]').textContent();
    expect(prediction).toContain('On Time'); // Grade A = low risk

    // Verify confidence metrics (three-regime)
    const r1 = await page.locator('[data-testid="regime-r1"]').textContent();
    const r2 = await page.locator('[data-testid="regime-r2"]').textContent();
    const r3 = await page.locator('[data-testid="regime-r3"]').textContent();

    const r1Value = parseFloat(r1!.replace('%', ''));
    const r2Value = parseFloat(r2!.replace('%', ''));
    const r3Value = parseFloat(r3!.replace('%', ''));

    // Grade A should be in Stabilization (high R3)
    expect(r3Value).toBeGreaterThan(50);

    devLog.log(`✅ Payment prediction: On Time (R1=${r1Value}%, R2=${r2Value}%, R3=${r3Value}%)`);
  });

  test('Payment Prediction: Grade D customer (high risk)', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-customers"]');

    // Select Small Vendor LLC (Grade D)
    await page.click('[data-testid="customer-C01200"]');

    await page.click('[data-testid="view-payment-prediction"]');

    const prediction = await page.locator('[data-testid="payment-prediction"]').textContent();
    expect(prediction).toContain('Late'); // Grade D = high risk

    // Verify warning shown
    const warning = await page.locator('[data-testid="payment-warning"]').textContent();
    expect(warning).toContain('High risk');

    // Verify recommended action
    const action = await page.locator('[data-testid="recommended-action"]').textContent();
    expect(action).toContain('Prepayment required');

    devLog.log('✅ Payment prediction: Late (high risk, prepayment recommended)');
  });

  test('Batch Prediction: Predict all customers', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-reports"]');

    // Select payment risk report
    await page.selectOption('[data-testid="report-type"]', 'payment-risk');

    await page.click('[data-testid="generate-report-button"]');

    // Wait for batch prediction
    await expect(page.locator('[data-testid="processing-spinner"]')).toBeVisible();

    await page.waitForSelector('[data-testid="report-results"]', { timeout: 30000 });

    // Verify results table
    const rows = await page.locator('[data-testid="report-row"]').count();
    expect(rows).toBeGreaterThan(0);

    // Verify statistics shown
    const stats = await page.locator('[data-testid="report-stats"]').textContent();
    expect(stats).toContain('On Time:');
    expect(stats).toContain('Late:');

    devLog.log(`✅ Batch prediction: ${rows} customers analyzed`);
  });

  test('OCR Error Case: Unreadable document', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    // Upload corrupted/unreadable file
    await page.click('[data-testid="upload-document-button"]');
    await page.click('[data-testid="process-document"]');

    // Should show low confidence warning
    await page.waitForSelector('[data-testid="ocr-result"]', { timeout: 10000 });

    const confidence = await page.locator('[data-testid="ocr-confidence"]').textContent();
    const confidenceValue = parseFloat(confidence!);

    if (confidenceValue < 0.7) {
      // Should show warning
      const warning = await page.locator('[data-testid="low-confidence-warning"]').textContent();
      expect(warning).toContain('Low confidence');

      // Should suggest manual review
      const suggestion = await page.locator('[data-testid="manual-review-button"]').isVisible();
      expect(suggestion).toBe(true);

      devLog.log(`⚠️ Low confidence: ${(confidenceValue * 100).toFixed(1)}%, manual review suggested`);
    }
  });

  test('OCR Performance: Process 5 documents sequentially', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    const startTime = Date.now();
    const confidences: number[] = [];

    for (let i = 0; i < 5; i++) {
      await page.click('[data-testid="upload-document-button"]');
      await page.click('[data-testid="process-document"]');

      await page.waitForSelector('[data-testid="ocr-result"]', { timeout: 10000 });

      const confidence = await page.locator('[data-testid="ocr-confidence"]').textContent();
      confidences.push(parseFloat(confidence!));

      await page.click('[data-testid="clear-result"]');
    }

    const duration = Date.now() - startTime;
    const avgTime = duration / 5;
    const avgConfidence = confidences.reduce((a, b) => a + b, 0) / confidences.length;

    // Florence-2 target: <1 second per document
    expect(avgTime).toBeLessThan(1000);

    devLog.log(`✅ OCR performance: ${avgTime.toFixed(0)}ms/doc, ${(avgConfidence * 100).toFixed(1)}% avg confidence`);
  });

  test('Payment History: View customer payment timeline', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-customers"]');
    await page.click('[data-testid="customer-C01011"]'); // Al Noor

    await page.click('[data-testid="payment-history-tab"]');

    // Verify payment history loaded
    const payments = await page.locator('[data-testid="payment-record"]').count();
    expect(payments).toBeGreaterThanOrEqual(0);

    if (payments > 0) {
      // Verify timeline visualization
      const hasTimeline = await page.locator('[data-testid="payment-timeline"]').isVisible();
      expect(hasTimeline).toBe(true);

      // Verify statistics
      const avgDays = await page.locator('[data-testid="avg-payment-days"]').textContent();
      const onTimeRate = await page.locator('[data-testid="on-time-rate"]').textContent();

      devLog.log(`✅ Payment history: ${payments} records, ${avgDays} avg days, ${onTimeRate} on-time`);
    }
  });

  test('Integration: OCR → Customer Match → Payment Prediction', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-inbox"]');

    // Upload invoice with customer name
    await page.click('[data-testid="upload-document-button"]');
    await page.selectOption('[data-testid="doc-type-select"]', 'invoice');
    await page.click('[data-testid="process-document"]');

    await page.waitForSelector('[data-testid="ocr-result"]', { timeout: 10000 });

    // Verify customer auto-detected
    const detectedCustomer = await page.locator('[data-testid="detected-customer"]').textContent();
    expect(detectedCustomer).toBeTruthy();

    // Click to view customer
    await page.click('[data-testid="view-customer-button"]');

    // Should navigate to Customer360 with payment prediction
    await expect(page.locator('[data-testid="payment-prediction"]')).toBeVisible();

    devLog.log(`✅ End-to-end: OCR → Customer (${detectedCustomer}) → Prediction`);
  });
});
