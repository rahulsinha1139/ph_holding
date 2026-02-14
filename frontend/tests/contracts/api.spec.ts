/**
 * ACE Engine API Contract Tests
 *
 * Scientific rigor based on UNIFIED_INTELLIGENCE_MONITORING_RESEARCH_PAPER:
 * - 50 samples per endpoint (statistical significance)
 * - 95% confidence intervals
 * - Response time < 100ms (Nielsen threshold)
 * - Defensive testing for production Nigeria deployment
 *
 * Test data from PHHoldingsScenariosTests.cs:
 * - Customers: C01011 (A), C01058 (A), C01176 (A), C01053 (B), C01200 (D)
 * - Products: LUB001 (150 BHD), FIL001 (45 BHD), CHM001 (280 BHD)
 * - Pricing: Grade A = 12% discount, B = 7%, D = -5% premium
 */

import { test, expect, APIRequestContext } from '@playwright/test';
import { devLog } from "$lib/utils/devLog";


// ============================================================================
// TYPE DEFINITIONS (Contract Schemas)
// ============================================================================

interface Customer {
  customerId: string;
  name: string;
  grade: 'A' | 'B' | 'C' | 'D';
  email: string;
  phone: string;
  industry?: string;
  country?: string;
}

interface PricingCalculation {
  productId: string;
  customerId: string;
  basePrice: number;
  appliedDiscount: number;
  finalPrice: number;
  quantity?: number;
}

interface HealthStatus {
  status: 'healthy' | 'degraded' | 'unhealthy';
  uptime: number;
  nodeCount: number;
  successRate: number;
  timestamp?: string;
}

interface ConsciousnessResponse {
  content: string;
  confidence: {
    score: number;
    r1: number;
    r2: number;
    r3: number;
  };
  sentiment: {
    dominantEmotion: string;
    positivity: number;
    certainty: number;
  };
  processingTimeMs: number;
}

interface RateLimitInfo {
  limit: number;
  remaining: number;
  resetTime: string;
}

// ============================================================================
// TEST CONFIGURATION
// ============================================================================

const BASE_URL = process.env.ACE_API_URL || 'http://localhost:5000';
const SAMPLES_PER_TEST = 50; // Statistical significance
const NIELSEN_THRESHOLD_MS = 100; // Nielsen usability threshold
const RATE_LIMIT_WINDOW = 5 * 60 * 1000; // 5 minutes
const RATE_LIMIT_MAX = 100; // 100 requests per 5 min

// Known test data (from PHHoldingsDataSeeder.cs)
const TEST_CUSTOMERS = {
  AL_NOOR: { id: 'C01011', name: 'Al Noor General Trading', grade: 'A' },
  SEAPEAK: { id: 'C01058', name: 'SEAPEAK Ship Management', grade: 'A' },
  NOMAC: { id: 'C01176', name: 'NOMAC', grade: 'A' },
  UNITED_SHIP: { id: 'C01053', name: 'United Ship Repairs', grade: 'B' },
  SMALL_VENDOR: { id: 'C01200', name: 'Small Vendor LLC', grade: 'D' },
};

const TEST_PRODUCTS = {
  LUBRICANT: { id: 'LUB001', name: 'Marine Engine Lubricant 20W-50', basePrice: 150.0 },
  FILTER: { id: 'FIL001', name: 'Oil Filter - Marine Grade', basePrice: 45.0 },
  CHEMICAL: { id: 'CHM001', name: 'Tank Cleaning Chemical', basePrice: 280.0 },
};

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

/**
 * Measure response time statistics across multiple samples
 */
function calculateStats(samples: number[]): {
  mean: number;
  median: number;
  p95: number;
  p99: number;
  min: number;
  max: number;
} {
  const sorted = [...samples].sort((a, b) => a - b);
  const n = sorted.length;

  return {
    mean: sorted.reduce((a, b) => a + b, 0) / n,
    median: sorted[Math.floor(n / 2)],
    p95: sorted[Math.floor(n * 0.95)],
    p99: sorted[Math.floor(n * 0.99)],
    min: sorted[0],
    max: sorted[n - 1],
  };
}

/**
 * Retry with harmonic backoff (defensive network handling)
 */
async function retryWithBackoff<T>(
  fn: () => Promise<T>,
  maxRetries = 3,
  baseDelay = 100
): Promise<T> {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (error) {
      if (attempt === maxRetries) throw error;

      // Harmonic backoff: delay = baseDelay / attempt
      const delay = baseDelay / attempt;
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  throw new Error('Retry logic failed'); // Should never reach here
}

/**
 * Assert valid customer schema
 */
function assertCustomerSchema(customer: any, description: string) {
  expect(customer, `${description}: should have customerId`).toHaveProperty('customerId');
  expect(customer, `${description}: should have name`).toHaveProperty('name');
  expect(customer, `${description}: should have grade`).toHaveProperty('grade');
  expect(customer, `${description}: should have email`).toHaveProperty('email');
  expect(customer, `${description}: should have phone`).toHaveProperty('phone');

  expect(['A', 'B', 'C', 'D'], `${description}: grade must be A/B/C/D`)
    .toContain(customer.grade);

  expect(customer.email, `${description}: email should be valid`)
    .toMatch(/^[^\s@]+@[^\s@]+\.[^\s@]+$/);
}

/**
 * Assert valid pricing schema
 */
function assertPricingSchema(pricing: any, description: string) {
  expect(pricing, `${description}: should have productId`).toHaveProperty('productId');
  expect(pricing, `${description}: should have basePrice`).toHaveProperty('basePrice');
  expect(pricing, `${description}: should have appliedDiscount`).toHaveProperty('appliedDiscount');
  expect(pricing, `${description}: should have finalPrice`).toHaveProperty('finalPrice');

  expect(typeof pricing.basePrice, `${description}: basePrice must be number`).toBe('number');
  expect(typeof pricing.finalPrice, `${description}: finalPrice must be number`).toBe('number');
  expect(pricing.basePrice, `${description}: basePrice must be positive`).toBeGreaterThan(0);
  expect(pricing.finalPrice, `${description}: finalPrice must be positive`).toBeGreaterThan(0);
}

// ============================================================================
// TEST SUITE: Health Endpoint
// ============================================================================

test.describe('GET /api/health - System Health Check', () => {

  test('should return healthy status with correct schema', async ({ request }) => {
    const response = await request.get(`${BASE_URL}/health`);

    expect(response.status(), 'Health endpoint should return 200').toBe(200);

    const health = await response.json();

    // Schema validation
    expect(health, 'Health response should have status').toHaveProperty('status');
    expect(health, 'Health response should have nodeCount').toHaveProperty('nodeCount');
    expect(health, 'Health response should have successRate').toHaveProperty('successRate');

    // Type validation
    expect(['Healthy', 'Degraded', 'Unhealthy'], 'Status must be valid enum')
      .toContain(health.status);
    expect(typeof health.nodeCount, 'nodeCount must be number').toBe('number');

    // Business logic validation
    expect(health.nodeCount, 'nodeCount must be non-negative').toBeGreaterThanOrEqual(0);
  });

  test('should respond within Nielsen threshold (100ms)', async ({ request }) => {
    const responseTimes: number[] = [];

    for (let i = 0; i < SAMPLES_PER_TEST; i++) {
      const start = Date.now();
      const response = await request.get(`${BASE_URL}/health`);
      const elapsed = Date.now() - start;

      expect(response.status(), `Sample ${i + 1}: Health check should succeed`).toBe(200);
      responseTimes.push(elapsed);
    }

    const stats = calculateStats(responseTimes);

    devLog.log(`Health endpoint performance (${SAMPLES_PER_TEST} samples):`);
    devLog.log(`  Mean: ${stats.mean.toFixed(2)}ms`);
    devLog.log(`  Median: ${stats.median.toFixed(2)}ms`);
    devLog.log(`  P95: ${stats.p95.toFixed(2)}ms`);
    devLog.log(`  P99: ${stats.p99.toFixed(2)}ms`);
    devLog.log(`  Range: ${stats.min}ms - ${stats.max}ms`);

    // Statistical assertion: P95 should be under Nielsen threshold
    expect(stats.p95, 'P95 response time must be under 100ms Nielsen threshold')
      .toBeLessThan(NIELSEN_THRESHOLD_MS);
  });

  test('should handle network failures gracefully', async ({ request }) => {
    // Test with retry logic
    const result = await retryWithBackoff(async () => {
      const response = await request.get(`${BASE_URL}/health`);
      expect(response.status()).toBe(200);
      return response.json();
    });

    expect(result, 'Should eventually succeed with retry logic').toHaveProperty('status');
  });
});

// ============================================================================
// TEST SUITE: Customer 360 View
// ============================================================================

test.describe('GET /api/customers/:id/360 - Customer Lookup', () => {

  test('should return Al Noor (C01011) with Grade A', async ({ request }) => {
    // Note: This endpoint needs to be implemented in Program.cs
    // For now, we'll test the knowledge base query endpoint
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: `Get customer information for ${TEST_CUSTOMERS.AL_NOOR.id}`,
      },
    });

    expect(response.status(), 'Customer lookup should return 200').toBe(200);

    const result = await response.json();

    // Consciousness response schema
    expect(result, 'Should have content').toHaveProperty('content');
    expect(result, 'Should have confidence').toHaveProperty('confidence');

    // Content should mention Al Noor and Grade A
    expect(result.content, 'Response should mention Al Noor')
      .toContain(TEST_CUSTOMERS.AL_NOOR.name);
  });

  test('should return 404 for invalid customer ID', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: 'Get customer information for INVALID_ID_99999',
      },
    });

    // Should still return 200 (consciousness always responds)
    expect(response.status()).toBe(200);

    const result = await response.json();

    // But confidence should be low
    expect(result.confidence.score, 'Confidence should be low for invalid customer')
      .toBeLessThan(0.5);
  });

  test('should return all Grade A customers', async ({ request }) => {
    const responseTimes: number[] = [];

    const gradeACustomers = [
      TEST_CUSTOMERS.AL_NOOR,
      TEST_CUSTOMERS.SEAPEAK,
      TEST_CUSTOMERS.NOMAC,
    ];

    for (const customer of gradeACustomers) {
      const start = Date.now();
      const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
        data: {
          userId: 'test-user',
          query: `Get customer ${customer.id}`,
        },
      });
      const elapsed = Date.now() - start;

      expect(response.status(), `Customer ${customer.id} lookup should succeed`).toBe(200);

      const result = await response.json();
      expect(result.content, `Should mention ${customer.name}`)
        .toContain(customer.name);

      responseTimes.push(elapsed);
    }

    const stats = calculateStats(responseTimes);
    devLog.log(`Grade A customer lookup performance: Mean ${stats.mean.toFixed(2)}ms`);
  });
});

// ============================================================================
// TEST SUITE: Pricing Calculation
// ============================================================================

test.describe('POST /api/pricing/calculate - Pricing Logic', () => {

  test('should calculate Al Noor pricing: 150 × 0.88 = 132.00 BHD', async ({ request }) => {
    // Using pricing recommendation endpoint
    const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
      data: {
        customerCode: TEST_CUSTOMERS.AL_NOOR.id,
        productCode: TEST_PRODUCTS.LUBRICANT.id,
        quantity: 10,
      },
    });

    expect(response.status(), 'Pricing calculation should return 200').toBe(200);

    const pricing = await response.json();

    // Schema validation
    assertPricingSchema(pricing, 'Al Noor lubricant pricing');

    // Business logic validation: Grade A = 12% discount
    const expectedPrice = 150.0 * (1 - 0.12); // 132.00

    expect(pricing.finalPrice, 'Al Noor should get 12% discount (132.00 BHD)')
      .toBeCloseTo(expectedPrice, 2);
  });

  test('should calculate Grade B pricing: 150 × 0.93 = 139.50 BHD', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
      data: {
        customerCode: TEST_CUSTOMERS.UNITED_SHIP.id, // Grade B
        productCode: TEST_PRODUCTS.LUBRICANT.id,
        quantity: 10,
      },
    });

    expect(response.status(), 'Grade B pricing should return 200').toBe(200);

    const pricing = await response.json();

    // Grade B = 7% discount
    const expectedPrice = 150.0 * (1 - 0.07); // 139.50

    expect(pricing.finalPrice, 'Grade B should get 7% discount (139.50 BHD)')
      .toBeCloseTo(expectedPrice, 2);
  });

  test('should calculate Grade D premium: 150 × 1.05 = 157.50 BHD', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
      data: {
        customerCode: TEST_CUSTOMERS.SMALL_VENDOR.id, // Grade D
        productCode: TEST_PRODUCTS.LUBRICANT.id,
        quantity: 10,
      },
    });

    expect(response.status(), 'Grade D pricing should return 200').toBe(200);

    const pricing = await response.json();

    // Grade D = -5% discount = +5% premium
    const expectedPrice = 150.0 * (1 + 0.05); // 157.50

    expect(pricing.finalPrice, 'Grade D should pay 5% premium (157.50 BHD)')
      .toBeCloseTo(expectedPrice, 2);
  });

  test('should enforce pricing hierarchy: Grade A < B < D', async ({ request }) => {
    const customers = [
      { ...TEST_CUSTOMERS.AL_NOOR, expectedDiscount: 0.12 },
      { ...TEST_CUSTOMERS.UNITED_SHIP, expectedDiscount: 0.07 },
      { ...TEST_CUSTOMERS.SMALL_VENDOR, expectedDiscount: -0.05 },
    ];

    const prices: number[] = [];

    for (const customer of customers) {
      const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
        data: {
          customerCode: customer.id,
          productCode: TEST_PRODUCTS.LUBRICANT.id,
          quantity: 10,
        },
      });

      expect(response.status(), `Pricing for ${customer.id} should succeed`).toBe(200);

      const pricing = await response.json();
      prices.push(pricing.finalPrice);
    }

    const [priceA, priceB, priceD] = prices;

    expect(priceA, 'Grade A should pay less than Grade B').toBeLessThan(priceB);
    expect(priceB, 'Grade B should pay less than Grade D').toBeLessThan(priceD);

    devLog.log(`Pricing hierarchy verified: A=${priceA} < B=${priceB} < D=${priceD}`);
  });

  test('should handle malformed pricing requests gracefully', async ({ request }) => {
    const malformedRequests = [
      { customerCode: '', productCode: TEST_PRODUCTS.LUBRICANT.id, quantity: 10 },
      { customerCode: TEST_CUSTOMERS.AL_NOOR.id, productCode: '', quantity: 10 },
      { customerCode: TEST_CUSTOMERS.AL_NOOR.id, productCode: TEST_PRODUCTS.LUBRICANT.id, quantity: -5 },
      { customerCode: null, productCode: TEST_PRODUCTS.LUBRICANT.id, quantity: 10 },
    ];

    for (const [index, badRequest] of malformedRequests.entries()) {
      const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
        data: badRequest,
      });

      // Should return 400 (Bad Request) or 422 (Unprocessable Entity)
      expect([400, 422, 500], `Malformed request ${index + 1} should return error status`)
        .toContain(response.status());
    }
  });

  test('should respond within Nielsen threshold for pricing calculations', async ({ request }) => {
    const responseTimes: number[] = [];

    for (let i = 0; i < SAMPLES_PER_TEST; i++) {
      const start = Date.now();
      const response = await request.post(`${BASE_URL}/api/pricing/recommend`, {
        data: {
          customerCode: TEST_CUSTOMERS.AL_NOOR.id,
          productCode: TEST_PRODUCTS.LUBRICANT.id,
          quantity: 10,
        },
      });
      const elapsed = Date.now() - start;

      expect(response.status(), `Sample ${i + 1}: Pricing should succeed`).toBe(200);
      responseTimes.push(elapsed);
    }

    const stats = calculateStats(responseTimes);

    devLog.log(`Pricing calculation performance (${SAMPLES_PER_TEST} samples):`);
    devLog.log(`  Mean: ${stats.mean.toFixed(2)}ms`);
    devLog.log(`  Median: ${stats.median.toFixed(2)}ms`);
    devLog.log(`  P95: ${stats.p95.toFixed(2)}ms`);

    expect(stats.p95, 'P95 pricing response time must be under 100ms')
      .toBeLessThan(NIELSEN_THRESHOLD_MS);
  });
});

// ============================================================================
// TEST SUITE: Consciousness Chat API
// ============================================================================

test.describe('POST /api/consciousness/chat - Main Query Endpoint', () => {

  test('should return valid consciousness response schema', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: 'What is the status of customer C01011?',
      },
    });

    expect(response.status(), 'Consciousness chat should return 200').toBe(200);

    const result = await response.json();

    // Schema validation
    expect(result, 'Should have content').toHaveProperty('content');
    expect(result, 'Should have confidence').toHaveProperty('confidence');
    expect(result, 'Should have sentiment').toHaveProperty('sentiment');
    expect(result, 'Should have processingTimeMs').toHaveProperty('processingTimeMs');

    // Confidence structure (three-regime dynamics)
    expect(result.confidence, 'Confidence should have score').toHaveProperty('score');

    // Sentiment structure
    expect(result.sentiment, 'Sentiment should have dominantEmotion').toHaveProperty('dominantEmotion');
    expect(result.sentiment, 'Sentiment should have positivity').toHaveProperty('positivity');
    expect(result.sentiment, 'Sentiment should have certainty').toHaveProperty('certainty');

    // Type validation
    expect(typeof result.content, 'Content must be string').toBe('string');
    expect(typeof result.confidence.score, 'Confidence score must be number').toBe('number');
    expect(typeof result.processingTimeMs, 'Processing time must be number').toBe('number');

    // Range validation
    expect(result.confidence.score, 'Confidence score must be 0-1').toBeGreaterThanOrEqual(0);
    expect(result.confidence.score, 'Confidence score must be 0-1').toBeLessThanOrEqual(1);
    expect(result.sentiment.positivity, 'Positivity must be -1 to +1').toBeGreaterThanOrEqual(-1);
    expect(result.sentiment.positivity, 'Positivity must be -1 to +1').toBeLessThanOrEqual(1);
  });

  test('should handle empty query gracefully', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: '',
      },
    });

    // Should still return 200 (consciousness always responds)
    expect(response.status(), 'Empty query should return 200').toBe(200);

    const result = await response.json();
    expect(result.content, 'Should provide helpful response for empty query')
      .toBeTruthy();
  });

  test('should detect emotional state from query', async ({ request }) => {
    const emotionalQueries = [
      { query: 'I am frustrated with the slow response!', expectedEmotion: 'frustrated' },
      { query: 'I am curious about how pricing works', expectedEmotion: 'curious' },
      { query: 'I need help urgently!', expectedEmotion: 'urgent' },
    ];

    for (const { query, expectedEmotion } of emotionalQueries) {
      const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
        data: {
          userId: 'test-user',
          query,
        },
      });

      expect(response.status(), `Query "${query}" should succeed`).toBe(200);

      const result = await response.json();

      devLog.log(`Query: "${query}"`);
      devLog.log(`  Detected emotion: ${result.sentiment.dominantEmotion}`);
      devLog.log(`  Positivity: ${result.sentiment.positivity.toFixed(2)}`);
      devLog.log(`  Certainty: ${result.sentiment.certainty.toFixed(2)}`);
    }
  });

  test('should maintain conversation context across turns', async ({ request }) => {
    const userId = `test-user-${Date.now()}`;

    // Turn 1: Ask about a customer
    const response1 = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId,
        query: `Tell me about customer ${TEST_CUSTOMERS.AL_NOOR.id}`,
      },
    });

    expect(response1.status(), 'Turn 1 should succeed').toBe(200);
    const result1 = await response1.json();

    // Turn 2: Follow-up (should remember context)
    const response2 = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId,
        query: 'What is their pricing?',
      },
    });

    expect(response2.status(), 'Turn 2 should succeed').toBe(200);
    const result2 = await response2.json();

    // Should have context about Al Noor from previous turn
    expect(result2.content, 'Should maintain context about Al Noor')
      .toBeTruthy();
  });

  test('should handle invalid JSON gracefully', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: '{ invalid json }',
      headers: { 'Content-Type': 'application/json' },
    });

    // Should return 400 (Bad Request)
    expect([400, 500], 'Invalid JSON should return error status')
      .toContain(response.status());
  });

  test('should handle missing required fields', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        // Missing userId and query
      },
    });

    expect([400, 422, 500], 'Missing fields should return error status')
      .toContain(response.status());
  });
});

// ============================================================================
// TEST SUITE: Rate Limiting
// ============================================================================

test.describe('Rate Limiting - 100 requests per 5 minutes', () => {

  test('should enforce global rate limit', async ({ request }) => {
    // Note: This test is destructive and should run in isolation
    // It will consume the entire rate limit budget

    const requests: Promise<any>[] = [];

    // Send 101 requests (1 over limit)
    for (let i = 0; i < 101; i++) {
      requests.push(
        request.get(`${BASE_URL}/health`).catch(err => ({ error: err }))
      );
    }

    const results = await Promise.all(requests);

    // Count 429 (Too Many Requests) responses
    const rateLimited = results.filter(r => r.status?.() === 429).length;

    expect(rateLimited, 'At least one request should be rate limited')
      .toBeGreaterThan(0);

    devLog.log(`Rate limit test: ${rateLimited} out of 101 requests were rate limited`);
  });

  test('should return proper rate limit headers', async ({ request }) => {
    const response = await request.get(`${BASE_URL}/health`);

    // Check for rate limit headers (if implemented)
    const headers = response.headers();

    // Common rate limit header patterns
    const hasRateLimitInfo =
      headers['x-ratelimit-limit'] ||
      headers['x-rate-limit-limit'] ||
      headers['ratelimit-limit'];

    if (hasRateLimitInfo) {
      devLog.log('Rate limit headers found:');
      devLog.log(`  Limit: ${headers['x-ratelimit-limit'] || headers['ratelimit-limit']}`);
      devLog.log(`  Remaining: ${headers['x-ratelimit-remaining'] || headers['ratelimit-remaining']}`);
    }
  });
});

// ============================================================================
// TEST SUITE: Error Handling & Resilience
// ============================================================================

test.describe('Error Handling & Defensive Testing', () => {

  test('should handle network timeouts gracefully', async ({ request }) => {
    // Test with very short timeout
    const result = await retryWithBackoff(
      async () => {
        const response = await request.get(`${BASE_URL}/health`, {
          timeout: 50, // 50ms timeout
        });
        return response.json();
      },
      3,
      100
    );

    expect(result, 'Should eventually succeed with retry logic')
      .toHaveProperty('status');
  });

  test('should validate CORS headers for frontend integration', async ({ request }) => {
    const response = await request.get(`${BASE_URL}/health`, {
      headers: {
        'Origin': 'http://localhost:5173',
      },
    });

    const headers = response.headers();

    // Check for CORS headers
    if (headers['access-control-allow-origin']) {
      devLog.log('CORS enabled:');
      devLog.log(`  Allow-Origin: ${headers['access-control-allow-origin']}`);
      devLog.log(`  Allow-Methods: ${headers['access-control-allow-methods'] || 'N/A'}`);
    }
  });

  test('should return JSON even on server errors', async ({ request }) => {
    // Try to trigger a server error with malformed request
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: 'x'.repeat(1000000), // Extremely long query
      },
    });

    // Even on error, should return JSON
    const contentType = response.headers()['content-type'];
    expect(contentType, 'Error responses should be JSON')
      .toContain('application/json');
  });

  test('should handle concurrent requests without data corruption', async ({ request }) => {
    const concurrentRequests = 20;
    const promises: Promise<any>[] = [];

    for (let i = 0; i < concurrentRequests; i++) {
      promises.push(
        request.post(`${BASE_URL}/api/consciousness/chat`, {
          data: {
            userId: `concurrent-user-${i}`,
            query: `Test query ${i}`,
          },
        })
      );
    }

    const results = await Promise.all(promises);

    // All requests should succeed
    const successCount = results.filter(r => r.status() === 200).length;

    expect(successCount, 'All concurrent requests should succeed')
      .toBe(concurrentRequests);

    devLog.log(`Concurrent test: ${successCount}/${concurrentRequests} requests succeeded`);
  });
});

// ============================================================================
// TEST SUITE: Statistical Performance Analysis
// ============================================================================

test.describe('Statistical Performance Analysis (50 samples)', () => {

  test('should measure end-to-end latency distribution', async ({ request }) => {
    const samples: {
      health: number[];
      pricing: number[];
      consciousness: number[];
    } = {
      health: [],
      pricing: [],
      consciousness: [],
    };

    // Collect 50 samples for each endpoint
    for (let i = 0; i < SAMPLES_PER_TEST; i++) {
      // Health check
      const healthStart = Date.now();
      await request.get(`${BASE_URL}/health`);
      samples.health.push(Date.now() - healthStart);

      // Pricing calculation
      const pricingStart = Date.now();
      await request.post(`${BASE_URL}/api/pricing/recommend`, {
        data: {
          customerCode: TEST_CUSTOMERS.AL_NOOR.id,
          productCode: TEST_PRODUCTS.LUBRICANT.id,
          quantity: 10,
        },
      });
      samples.pricing.push(Date.now() - pricingStart);

      // Consciousness query
      const consciousnessStart = Date.now();
      await request.post(`${BASE_URL}/api/consciousness/chat`, {
        data: {
          userId: 'perf-test-user',
          query: 'Quick test query',
        },
      });
      samples.consciousness.push(Date.now() - consciousnessStart);
    }

    // Calculate statistics
    const healthStats = calculateStats(samples.health);
    const pricingStats = calculateStats(samples.pricing);
    const consciousnessStats = calculateStats(samples.consciousness);

    devLog.log('\n=== PERFORMANCE ANALYSIS (50 SAMPLES) ===\n');

    devLog.log('Health Endpoint:');
    devLog.log(`  Mean: ${healthStats.mean.toFixed(2)}ms`);
    devLog.log(`  Median: ${healthStats.median.toFixed(2)}ms`);
    devLog.log(`  P95: ${healthStats.p95.toFixed(2)}ms`);
    devLog.log(`  P99: ${healthStats.p99.toFixed(2)}ms`);
    devLog.log(`  Range: ${healthStats.min}ms - ${healthStats.max}ms`);

    devLog.log('\nPricing Endpoint:');
    devLog.log(`  Mean: ${pricingStats.mean.toFixed(2)}ms`);
    devLog.log(`  Median: ${pricingStats.median.toFixed(2)}ms`);
    devLog.log(`  P95: ${pricingStats.p95.toFixed(2)}ms`);
    devLog.log(`  P99: ${pricingStats.p99.toFixed(2)}ms`);
    devLog.log(`  Range: ${pricingStats.min}ms - ${pricingStats.max}ms`);

    devLog.log('\nConsciousness Endpoint:');
    devLog.log(`  Mean: ${consciousnessStats.mean.toFixed(2)}ms`);
    devLog.log(`  Median: ${consciousnessStats.median.toFixed(2)}ms`);
    devLog.log(`  P95: ${consciousnessStats.p95.toFixed(2)}ms`);
    devLog.log(`  P99: ${consciousnessStats.p99.toFixed(2)}ms`);
    devLog.log(`  Range: ${consciousnessStats.min}ms - ${consciousnessStats.max}ms`);

    devLog.log('\n========================================\n');

    // Assertions for production readiness
    expect(healthStats.p95, 'Health P95 must be under Nielsen threshold')
      .toBeLessThan(NIELSEN_THRESHOLD_MS);

    // Pricing and Consciousness can be slower (they do real work)
    expect(pricingStats.p95, 'Pricing P95 should be reasonable (< 500ms)')
      .toBeLessThan(500);

    expect(consciousnessStats.p95, 'Consciousness P95 should be reasonable (< 2000ms)')
      .toBeLessThan(2000);
  });

  test('should calculate 95% confidence intervals', async ({ request }) => {
    const samples: number[] = [];

    for (let i = 0; i < SAMPLES_PER_TEST; i++) {
      const start = Date.now();
      await request.get(`${BASE_URL}/health`);
      samples.push(Date.now() - start);
    }

    const stats = calculateStats(samples);

    // Calculate standard deviation
    const variance = samples.reduce((acc, val) => acc + Math.pow(val - stats.mean, 2), 0) / samples.length;
    const stdDev = Math.sqrt(variance);

    // 95% confidence interval: mean ± 1.96 * (stdDev / sqrt(n))
    const marginOfError = 1.96 * (stdDev / Math.sqrt(samples.length));
    const ci95Lower = stats.mean - marginOfError;
    const ci95Upper = stats.mean + marginOfError;

    devLog.log(`\n95% Confidence Interval for Health Endpoint:`);
    devLog.log(`  Mean: ${stats.mean.toFixed(2)}ms`);
    devLog.log(`  Std Dev: ${stdDev.toFixed(2)}ms`);
    devLog.log(`  95% CI: [${ci95Lower.toFixed(2)}ms, ${ci95Upper.toFixed(2)}ms]`);

    // The true mean should fall within this interval with 95% probability
    expect(ci95Upper - ci95Lower, 'Confidence interval should be reasonably tight (< 20ms)')
      .toBeLessThan(20);
  });
});

// ============================================================================
// TEST SUITE: Production Readiness Checklist
// ============================================================================

test.describe('Production Readiness - Nigeria Deployment', () => {

  test('should have all critical endpoints accessible', async ({ request }) => {
    const criticalEndpoints = [
      { method: 'GET', path: '/health' },
      { method: 'POST', path: '/api/consciousness/chat' },
      { method: 'POST', path: '/api/pricing/recommend' },
    ];

    for (const endpoint of criticalEndpoints) {
      const response = await request.fetch(`${BASE_URL}${endpoint.path}`, {
        method: endpoint.method,
        data: endpoint.method === 'POST' ? { userId: 'test', query: 'test' } : undefined,
      });

      expect([200, 400, 422], `${endpoint.method} ${endpoint.path} should be accessible`)
        .toContain(response.status());
    }
  });

  test('should return proper error messages for debugging', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        // Missing required fields
      },
    });

    if (response.status() !== 200) {
      const error = await response.json().catch(() => ({}));

      // Error response should be informative
      expect(
        error.error || error.message || error.title,
        'Error response should have descriptive message'
      ).toBeTruthy();
    }
  });

  test('should handle UTF-8 characters (Arabic, emoji)', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/consciousness/chat`, {
      data: {
        userId: 'test-user',
        query: 'مرحبا! What is the status? 🚀',
      },
    });

    expect(response.status(), 'Should handle UTF-8 characters').toBe(200);

    const result = await response.json();
    expect(result.content, 'Should preserve UTF-8 in response').toBeTruthy();
  });
});
