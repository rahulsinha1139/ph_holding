/**
 * E2E Test: Pricing Flow
 * View pricing → Simulate margin → Quote → Send offer
 *
 * Tests the critical pricing engine and customer discount calculations
 */

import { test, expect } from '@playwright/test';
import { devLog } from "$lib/utils/devLog";

const BASE_URL = process.env.VITE_DEV_SERVER_URL || 'http://localhost:5173';

// Test customers with known grades
const TEST_CUSTOMERS = {
  GRADE_A: { id: 'C01011', name: 'Al Noor', discount: 12 },
  GRADE_B: { id: 'C01053', name: 'United Ship Repairs', discount: 7 },
  GRADE_D: { id: 'C01200', name: 'Small Vendor LLC', markup: 5 },
};

test.describe('Pricing Flow - Customer Discount Validation', () => {

  test('Grade A: 12% discount calculation', async ({ page }) => {
    await page.goto(BASE_URL);

    // Navigate to Pricing screen
    await page.click('[data-testid="nav-pricing"]');

    // Select Grade A customer
    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_A.id);

    // Select product: Marine Engine Lubricant (150 BHD base)
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');

    // Set quantity
    await page.fill('[data-testid="quantity-input"]', '10');

    // Click "Calculate Price"
    await page.click('[data-testid="calculate-price-button"]');

    // Wait for calculation
    await page.waitForSelector('[data-testid="calculated-price"]');

    // Verify: 150 BHD × 0.88 = 132 BHD per unit
    const unitPrice = await page.locator('[data-testid="unit-price"]').textContent();
    expect(parseFloat(unitPrice!)).toBeCloseTo(132.0, 2);

    // Verify total: 132 × 10 = 1320 BHD
    const totalPrice = await page.locator('[data-testid="total-price"]').textContent();
    expect(parseFloat(totalPrice!)).toBeCloseTo(1320.0, 2);

    // Verify discount shown
    const discountLabel = await page.locator('[data-testid="discount-label"]').textContent();
    expect(discountLabel).toContain('12%');

    devLog.log('✅ Grade A pricing: 150 BHD → 132 BHD (12% discount)');
  });

  test('Grade B: 7% discount calculation', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_B.id);
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');
    await page.fill('[data-testid="quantity-input"]', '10');
    await page.click('[data-testid="calculate-price-button"]');

    const unitPrice = await page.locator('[data-testid="unit-price"]').textContent();
    expect(parseFloat(unitPrice!)).toBeCloseTo(139.5, 2); // 150 × 0.93

    devLog.log('✅ Grade B pricing: 150 BHD → 139.5 BHD (7% discount)');
  });

  test('Grade D: 5% markup penalty', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_D.id);
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');
    await page.fill('[data-testid="quantity-input"]', '10');
    await page.click('[data-testid="calculate-price-button"]');

    const unitPrice = await page.locator('[data-testid="unit-price"]').textContent();
    expect(parseFloat(unitPrice!)).toBeCloseTo(157.5, 2); // 150 × 1.05

    // Verify markup warning shown
    const markupWarning = await page.locator('[data-testid="markup-warning"]').textContent();
    expect(markupWarning).toContain('5% markup');

    devLog.log('✅ Grade D pricing: 150 BHD → 157.5 BHD (5% markup penalty)');
  });

  test('Margin Simulation: Adjust margin and recalculate', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_A.id);
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');
    await page.fill('[data-testid="quantity-input"]', '10');

    // Override margin to 20%
    await page.click('[data-testid="enable-margin-override"]');
    await page.fill('[data-testid="margin-override-input"]', '20');
    await page.click('[data-testid="calculate-price-button"]');

    // Verify new calculation: 150 BHD × 0.80 = 120 BHD
    const unitPrice = await page.locator('[data-testid="unit-price"]').textContent();
    expect(parseFloat(unitPrice!)).toBeCloseTo(120.0, 2);

    devLog.log('✅ Margin override: 20% margin → 120 BHD');
  });

  test('Quote Generation: Create and send quote', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_A.id);
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');
    await page.fill('[data-testid="quantity-input"]', '100');
    await page.click('[data-testid="calculate-price-button"]');

    // Generate quote
    await page.click('[data-testid="generate-quote-button"]');

    // Fill quote details
    await page.fill('[data-testid="quote-validity-days"]', '30');
    await page.fill('[data-testid="quote-notes"]', 'Special pricing for Q1 2025');

    await page.click('[data-testid="save-quote-button"]');

    // Verify quote created
    await expect(page.locator('[data-testid="success-toast"]')).toBeVisible();
    const quoteID = await page.locator('[data-testid="created-quote-id"]').textContent();

    devLog.log(`✅ Quote created: #${quoteID}`);

    // Send quote by email
    await page.click(`[data-testid="quote-${quoteID}-send"]`);
    await page.fill('[data-testid="email-to"]', 'procurement@alnoor.bh');
    await page.fill('[data-testid="email-subject"]', 'Quote #' + quoteID + ' - Marine Engine Lubricant');
    await page.click('[data-testid="email-send-confirm"]');

    await expect(page.locator('[data-testid="email-success-toast"]')).toBeVisible();

    devLog.log('✅ Quote sent via email');
  });

  test('Error Case: Invalid quantity (negative)', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_A.id);
    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');

    // Try negative quantity
    await page.fill('[data-testid="quantity-input"]', '-10');
    await page.click('[data-testid="calculate-price-button"]');

    // Should show error
    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('Quantity must be positive');
  });

  test('Error Case: No customer selected', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.fill('[data-testid="product-search"]', 'Marine Engine Lubricant');
    await page.click('[data-testid="product-LUB001"]');
    await page.fill('[data-testid="quantity-input"]', '10');
    await page.click('[data-testid="calculate-price-button"]');

    // Should show error
    await expect(page.locator('[data-testid="error-toast"]')).toBeVisible();
    const errorText = await page.locator('[data-testid="error-toast"]').textContent();
    expect(errorText).toContain('Please select a customer');
  });

  test('Performance: Bulk pricing calculation', async ({ page }) => {
    await page.goto(BASE_URL);
    await page.click('[data-testid="nav-pricing"]');

    await page.selectOption('[data-testid="customer-select"]', TEST_CUSTOMERS.GRADE_A.id);

    // Add 10 products
    const products = [
      'Marine Engine Lubricant',
      'Oil Filter',
      'Tank Cleaning Chemical',
      'Hydraulic Oil',
      'Grease',
      'Coolant',
      'Brake Fluid',
      'Transmission Fluid',
      'Fuel Additive',
      'Engine Cleaner',
    ];

    const startTime = Date.now();

    for (const product of products) {
      await page.fill('[data-testid="product-search"]', product);
      await page.click('[data-testid="add-to-quote"]');
    }

    await page.click('[data-testid="calculate-all-prices-button"]');
    await page.waitForSelector('[data-testid="total-quote-price"]');

    const duration = Date.now() - startTime;

    // Should complete within 5 seconds
    expect(duration).toBeLessThan(5000);

    devLog.log(`✅ Bulk pricing: 10 products calculated in ${duration}ms`);
  });
});
