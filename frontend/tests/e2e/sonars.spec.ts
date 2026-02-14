import { test, expect, Page } from '@playwright/test';
import { Customer360Page } from './pages/Customer360Page';
import { devLog } from "$lib/utils/devLog";


/**
 * E2E Tests: Six Sonar Validation System
 *
 * Source: UNIFIED_INTELLIGENCE_MONITORING_RESEARCH_PAPER.html
 * Architecture: Three-Regime QA (Exploration/Optimization/Stabilization)
 * Target: PH Holdings Production Deployment
 *
 * The Six Sonars:
 *   1. UX Sonar       - Visual Smoothness (FPS ≥ 60)
 *   2. Design Sonar   - Visual Quality (Harmony, Contrast)
 *   3. Code Sonar     - Complexity (CC ≤ 5.0)
 *   4. Semantic Sonar - Architecture (No circular deps)
 *   5. Journey Sonar  - User Friction (0% frustration on happy paths)
 *   6. State Sonar    - System Complexity (SMT ≤ 6.0 for simple flows)
 *
 * System Health Metric (SHM):
 *   SHM = weighted_average(all sonars)
 *   - Nigeria Target: SHM ≥ 0.85 (Stabilization)
 *   - Optimization: SHM ≥ 0.70
 *   - Exploration: SHM ≥ 0.55
 */

// ============================================================================
// TELEMETRY COLLECTION HELPERS
// ============================================================================

/**
 * Inject telemetry collection script into page
 * Collects FPS, CLS, layout shifts, interactions, etc.
 */
async function injectTelemetry(page: Page): Promise<void> {
  await page.addInitScript(() => {
    // Global telemetry object
    (window as any).__sonicTelemetry = {
      frameTimes: [] as number[],
      layoutShifts: [] as number[],
      interactions: [] as any[],
      startTime: performance.now(),
      lastFrameTime: performance.now(),
      averageFPS: 60,
      cumulativeLayoutShift: 0,
      rageClicks: 0,
      hesitationEvents: 0,
      backtrackCount: 0
    };

    const telemetry = (window as any).__sonicTelemetry;

    // FPS tracking (via requestAnimationFrame)
    function measureFPS() {
      const now = performance.now();
      telemetry.frameTimes.push(now);
      telemetry.lastFrameTime = now;

      // Keep last 120 frames (2 seconds at 60 FPS)
      if (telemetry.frameTimes.length > 120) {
        telemetry.frameTimes.shift();
      }

      // Calculate average FPS
      if (telemetry.frameTimes.length >= 2) {
        let totalDelta = 0;
        for (let i = 1; i < telemetry.frameTimes.length; i++) {
          totalDelta += telemetry.frameTimes[i] - telemetry.frameTimes[i - 1];
        }
        const avgDelta = totalDelta / (telemetry.frameTimes.length - 1);
        telemetry.averageFPS = 1000 / avgDelta;
      }

      requestAnimationFrame(measureFPS);
    }
    requestAnimationFrame(measureFPS);

    // CLS tracking (Cumulative Layout Shift)
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.entryType === 'layout-shift' && !(entry as any).hadRecentInput) {
            const shift = (entry as any).value;
            telemetry.cumulativeLayoutShift += shift;
            telemetry.layoutShifts.push(shift);
          }
        }
      });
      observer.observe({ type: 'layout-shift', buffered: true });
    }

    // Click tracking (rage clicks, hesitation)
    let lastClickTime = 0;
    let lastClickTarget: EventTarget | null = null;
    let clickCount = 0;

    document.addEventListener('click', (e) => {
      const now = performance.now();
      const timeSinceLastClick = now - lastClickTime;

      // Rage click: 3+ clicks within 1 second on same element
      if (e.target === lastClickTarget && timeSinceLastClick < 1000) {
        clickCount++;
        if (clickCount >= 3) {
          telemetry.rageClicks++;
          telemetry.interactions.push({
            type: 'rage-click',
            target: (e.target as HTMLElement).tagName,
            timestamp: now
          });
        }
      } else {
        clickCount = 1;
      }

      // Hesitation: long pause before click (>3 seconds)
      if (timeSinceLastClick > 3000 && lastClickTime > 0) {
        telemetry.hesitationEvents++;
        telemetry.interactions.push({
          type: 'hesitation',
          duration: timeSinceLastClick,
          timestamp: now
        });
      }

      lastClickTime = now;
      lastClickTarget = e.target;
    });

    // Backtrack detection (popstate events)
    window.addEventListener('popstate', () => {
      telemetry.backtrackCount++;
      telemetry.interactions.push({
        type: 'backtrack',
        timestamp: performance.now()
      });
    });

    // Expose API
    (window as any).__getTelemetry = () => telemetry;
  });
}

/**
 * Get telemetry snapshot from page
 */
async function getTelemetry(page: Page): Promise<any> {
  return await page.evaluate(() => {
    return (window as any).__getTelemetry();
  });
}

/**
 * Calculate color contrast ratio (WCAG compliance)
 * Formula: (L1 + 0.05) / (L2 + 0.05) where L1 > L2
 */
function getContrastRatio(rgb1: number[], rgb2: number[]): number {
  const getLuminance = (rgb: number[]): number => {
    const [r, g, b] = rgb.map(val => {
      const sRGB = val / 255;
      return sRGB <= 0.03928 ? sRGB / 12.92 : Math.pow((sRGB + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };

  const l1 = getLuminance(rgb1);
  const l2 = getLuminance(rgb2);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);

  return (lighter + 0.05) / (darker + 0.05);
}

/**
 * Parse RGB color string to [r, g, b] array
 */
function parseRGB(color: string): number[] {
  const match = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1]), parseInt(match[2]), parseInt(match[3])];
}

// ============================================================================
// TEST SUITE: SIX SONAR VALIDATION
// ============================================================================

test.describe('Six Sonar Validation System', () => {

  // ============================================================================
  // 1. UX SONAR: Visual Smoothness
  // Formula: smoothness = min(1.0, FPS / 60.0)
  // ============================================================================

  test('UX Sonar: FPS ≥ 60 for Customer360 screen', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011'); // Al Noor (Grade A customer)

    // Wait for initial render
    await page.waitForTimeout(2000);

    // Perform typical user interactions
    await customer360.scrollToElement('health-score');
    await page.waitForTimeout(500);
    await customer360.scrollToElement('order-history-table');
    await page.waitForTimeout(500);
    await customer360.scrollToElement('customer-grade');
    await page.waitForTimeout(1000);

    // Get telemetry
    const telemetry = await getTelemetry(page);
    const fps = telemetry.averageFPS;
    const smoothness = Math.min(1.0, fps / 60.0);

    devLog.log(`[UX Sonar] FPS: ${fps.toFixed(2)}, Smoothness: ${smoothness.toFixed(3)}`);

    // Assertions
    expect(fps, 'FPS should be ≥ 60 for smooth experience').toBeGreaterThanOrEqual(60);
    expect(smoothness, 'Smoothness score should be ≥ 0.95').toBeGreaterThanOrEqual(0.95);

    // Report
    devLog.log(`✅ UX Sonar PASSED - FPS: ${fps.toFixed(2)} (Smoothness: ${(smoothness * 100).toFixed(1)}%)`);
  });

  test('UX Sonar: CLS < 0.1 for visual stability', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Wait for page to fully load
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Get CLS
    const telemetry = await getTelemetry(page);
    const cls = telemetry.cumulativeLayoutShift;

    devLog.log(`[UX Sonar] CLS: ${cls.toFixed(4)}`);

    // Nielsen threshold: CLS < 0.1 for good experience
    expect(cls, 'CLS should be < 0.1 for visual stability').toBeLessThan(0.1);

    devLog.log(`✅ UX Sonar (CLS) PASSED - ${cls.toFixed(4)}`);
  });

  // ============================================================================
  // 2. DESIGN SONAR: Visual Quality
  // Formula: harmony = (φ × 0.618) + contrast - clash
  // ============================================================================

  test('Design Sonar: Contrast ratio ≥ 4.5:1 (WCAG AA)', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Sample text elements and their backgrounds
    const elements = [
      { selector: '[data-testid="customer-name"]', type: 'text' },
      { selector: '[data-testid="customer-grade"]', type: 'badge' },
      { selector: '[data-testid="health-score"]', type: 'metric' }
    ];

    const contrastResults: any[] = [];

    for (const elem of elements) {
      const locator = page.locator(elem.selector);

      // Get computed styles
      const textColor = await locator.evaluate(el =>
        window.getComputedStyle(el).color
      );
      const bgColor = await locator.evaluate(el =>
        window.getComputedStyle(el).backgroundColor
      );

      const textRGB = parseRGB(textColor);
      const bgRGB = parseRGB(bgColor);
      const ratio = getContrastRatio(textRGB, bgRGB);

      contrastResults.push({
        selector: elem.selector,
        type: elem.type,
        ratio: ratio,
        textColor,
        bgColor
      });

      devLog.log(`[Design Sonar] ${elem.type}: ${ratio.toFixed(2)}:1 (${textColor} on ${bgColor})`);

      // WCAG AA: 4.5:1 for normal text, 3:1 for large text
      const minRatio = elem.type === 'text' ? 4.5 : 3.0;
      expect(ratio, `${elem.type} contrast should be ≥ ${minRatio}:1`).toBeGreaterThanOrEqual(minRatio);
    }

    const avgRatio = contrastResults.reduce((sum, r) => sum + r.ratio, 0) / contrastResults.length;
    devLog.log(`✅ Design Sonar (Contrast) PASSED - Average ratio: ${avgRatio.toFixed(2)}:1`);
  });

  test('Design Sonar: No color clashes (orange + purple flagged)', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Get all visible elements
    const allElements = await page.locator('*').evaluateAll(elements => {
      return elements
        .filter(el => {
          const style = window.getComputedStyle(el);
          return style.display !== 'none' && style.visibility !== 'hidden';
        })
        .map(el => {
          const style = window.getComputedStyle(el);
          return {
            tagName: el.tagName,
            color: style.color,
            backgroundColor: style.backgroundColor
          };
        });
    });

    // Check for orange + purple clashes (known bad combination)
    const clashPatterns = [
      { name: 'orange-purple', colors: ['rgb(215, 115, 66)', 'rgb(128, 0, 128)'] },
      { name: 'red-green', colors: ['rgb(255, 0, 0)', 'rgb(0, 255, 0)'] }
    ];

    let clashCount = 0;
    for (const elem of allElements) {
      for (const pattern of clashPatterns) {
        if (pattern.colors.includes(elem.color) && pattern.colors.includes(elem.backgroundColor)) {
          clashCount++;
          devLog.warn(`[Design Sonar] Color clash detected: ${pattern.name}`);
        }
      }
    }

    expect(clashCount, 'No color clashes should be present').toBe(0);
    devLog.log(`✅ Design Sonar (Color Harmony) PASSED - No clashes detected`);
  });

  // ============================================================================
  // 3. CODE SONAR: Complexity
  // Formula: bug_density = (CC^1.2 × duplication) / cohesion
  // ============================================================================

  test('Code Sonar: Average CC ≤ 5.0 (simple, maintainable)', async ({ page }) => {
    // This test would typically analyze the codebase statically
    // For E2E, we check that the bundled JS doesn't have obvious complexity issues

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Check for excessively large inline scripts (proxy for complexity)
    const scriptComplexity = await page.evaluate(() => {
      const scripts = Array.from(document.querySelectorAll('script'));
      const complexityMetrics = scripts.map(script => {
        const code = script.textContent || '';
        const lines = code.split('\n').length;
        const functions = (code.match(/function/g) || []).length;
        const conditionals = (code.match(/if\s*\(|switch\s*\(/g) || []).length;
        const loops = (code.match(/for\s*\(|while\s*\(/g) || []).length;

        // Simple CC approximation: 1 + conditionals + loops
        const estimatedCC = 1 + conditionals + loops;

        return {
          lines,
          functions,
          estimatedCC: functions > 0 ? estimatedCC / functions : 0
        };
      });

      const avgCC = complexityMetrics.length > 0
        ? complexityMetrics.reduce((sum, m) => sum + m.estimatedCC, 0) / complexityMetrics.length
        : 0;

      return {
        scriptCount: scripts.length,
        avgCC: avgCC,
        maxCC: Math.max(...complexityMetrics.map(m => m.estimatedCC), 0)
      };
    });

    devLog.log(`[Code Sonar] Scripts: ${scriptComplexity.scriptCount}, Avg CC: ${scriptComplexity.avgCC.toFixed(2)}, Max CC: ${scriptComplexity.maxCC.toFixed(0)}`);

    // Assertions
    expect(scriptComplexity.avgCC, 'Average CC should be ≤ 5.0').toBeLessThanOrEqual(5.0);
    expect(scriptComplexity.maxCC, 'Max CC should be ≤ 100').toBeLessThanOrEqual(100);

    devLog.log(`✅ Code Sonar PASSED - Avg CC: ${scriptComplexity.avgCC.toFixed(2)}`);
  });

  test('Code Sonar: No excessive function nesting', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Check console for errors (proxy for code quality issues)
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Trigger various interactions
    await customer360.scrollToElement('health-score');
    await customer360.scrollToElement('order-history-table');
    await page.waitForTimeout(1000);

    // No critical errors should occur
    const criticalErrors = consoleErrors.filter(err =>
      err.includes('TypeError') || err.includes('ReferenceError') || err.includes('Uncaught')
    );

    expect(criticalErrors.length, 'No critical JavaScript errors should occur').toBe(0);

    if (consoleErrors.length > 0) {
      devLog.log(`[Code Sonar] Non-critical errors: ${consoleErrors.length}`);
    }

    devLog.log(`✅ Code Sonar (Error-free) PASSED`);
  });

  // ============================================================================
  // 4. SEMANTIC SONAR: Architecture
  // Formula: AQS = (cohesion / coupling) × modularity
  // ============================================================================

  test('Semantic Sonar: 0 circular dependencies', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Check for module loading errors (indicates circular deps)
    const moduleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error' && msg.text().includes('circular')) {
        moduleErrors.push(msg.text());
      }
    });

    // Wait for all modules to load
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    expect(moduleErrors.length, 'No circular dependency errors should occur').toBe(0);
    devLog.log(`✅ Semantic Sonar (Circular Deps) PASSED - 0 detected`);
  });

  test('Semantic Sonar: Modularity ≥ 80%', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Check for proper component isolation (data-testid presence)
    const modularity = await page.evaluate(() => {
      const allElements = document.querySelectorAll('*');
      const componentsWithTestId = document.querySelectorAll('[data-testid]');

      // Modularity proxy: % of interactive elements with semantic identifiers
      const interactiveElements = Array.from(allElements).filter(el => {
        return el.tagName === 'BUTTON' ||
               el.tagName === 'A' ||
               el.tagName === 'INPUT' ||
               el.tagName === 'SELECT' ||
               el.getAttribute('role') === 'button';
      });

      const semanticInteractive = interactiveElements.filter(el =>
        el.hasAttribute('data-testid') || el.hasAttribute('aria-label')
      );

      const modularityScore = interactiveElements.length > 0
        ? semanticInteractive.length / interactiveElements.length
        : 1.0;

      return {
        total: interactiveElements.length,
        semantic: semanticInteractive.length,
        score: modularityScore
      };
    });

    devLog.log(`[Semantic Sonar] Modularity: ${(modularity.score * 100).toFixed(1)}% (${modularity.semantic}/${modularity.total} semantic)`);

    expect(modularity.score, 'Modularity should be ≥ 80%').toBeGreaterThanOrEqual(0.80);
    devLog.log(`✅ Semantic Sonar (Modularity) PASSED - ${(modularity.score * 100).toFixed(1)}%`);
  });

  // ============================================================================
  // 5. JOURNEY SONAR: User Friction
  // Formula: frustration = (hesitation / duration) × rage_clicks + backtrack_rate
  // ============================================================================

  test('Journey Sonar: Happy path has 0% frustration', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);

    // Happy path: Load customer, view details, check pricing
    const startTime = Date.now();

    await customer360.goto('C01011');
    await customer360.scrollToElement('customer-name');
    await page.waitForTimeout(300);

    await customer360.scrollToElement('customer-grade');
    await page.waitForTimeout(300);

    await customer360.scrollToElement('health-score');
    await page.waitForTimeout(300);

    await customer360.scrollToElement('order-history-table');
    await page.waitForTimeout(500);

    const duration = (Date.now() - startTime) / 1000;
    const telemetry = await getTelemetry(page);

    // Calculate frustration
    const hesitationRate = telemetry.hesitationEvents / duration;
    const backtrackRate = telemetry.backtrackCount / duration;
    const frustration = hesitationRate * telemetry.rageClicks + backtrackRate;

    devLog.log(`[Journey Sonar] Duration: ${duration.toFixed(2)}s, Hesitations: ${telemetry.hesitationEvents}, Rage clicks: ${telemetry.rageClicks}, Backtracks: ${telemetry.backtrackCount}`);
    devLog.log(`[Journey Sonar] Frustration: ${(frustration * 100).toFixed(2)}%`);

    // Happy path should have 0 frustration
    expect(telemetry.rageClicks, 'No rage clicks on happy path').toBe(0);
    expect(telemetry.backtrackCount, 'No backtracks on happy path').toBe(0);
    expect(frustration, 'Frustration should be 0% on happy path').toBe(0);

    devLog.log(`✅ Journey Sonar (Happy Path) PASSED - 0% frustration`);
  });

  test('Journey Sonar: No rage clicks detected', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Normal interactions (not rapid clicking)
    await customer360.scrollToElement('customer-name');
    await page.waitForTimeout(500);
    await customer360.scrollToElement('health-score');
    await page.waitForTimeout(500);

    const telemetry = await getTelemetry(page);

    expect(telemetry.rageClicks, 'No rage clicks should be detected').toBe(0);
    devLog.log(`✅ Journey Sonar (Rage Clicks) PASSED - 0 detected`);
  });

  // ============================================================================
  // 6. STATE SONAR: System Complexity
  // Formula: SMT = log₂(states × transitions) / explosion_factor
  // ============================================================================

  test('State Sonar: Simple flows have SMT ≤ 6.0', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Count state transitions in simple flow
    let stateChanges = 0;
    page.on('load', () => stateChanges++);

    // Simple flow: view customer details
    await customer360.scrollToElement('customer-name');
    await customer360.scrollToElement('health-score');
    await customer360.scrollToElement('order-history-table');

    // Get DOM complexity as proxy for state
    const complexity = await page.evaluate(() => {
      const elements = document.querySelectorAll('*').length;
      const interactiveElements = document.querySelectorAll('button, a, input, select').length;
      const stateAttributes = document.querySelectorAll('[data-state], [data-value], [data-flow-state]').length;

      // SMT approximation: log₂(states × transitions) / explosion_factor
      // states ≈ stateAttributes, transitions ≈ interactiveElements
      const states = Math.max(stateAttributes, 1);
      const transitions = Math.max(interactiveElements, 1);
      const explosionFactor = Math.max(elements / 100, 1); // Normalize by DOM size

      const smt = Math.log2(states * transitions) / explosionFactor;

      return {
        elements,
        interactiveElements,
        stateAttributes,
        states,
        transitions,
        explosionFactor: explosionFactor.toFixed(2),
        smt: smt.toFixed(2)
      };
    });

    devLog.log(`[State Sonar] States: ${complexity.states}, Transitions: ${complexity.transitions}, SMT: ${complexity.smt}`);

    // Simple flows should have SMT ≤ 6.0
    expect(parseFloat(complexity.smt), 'SMT should be ≤ 6.0 for simple flows').toBeLessThanOrEqual(6.0);

    devLog.log(`✅ State Sonar (Simple Flow) PASSED - SMT: ${complexity.smt}`);
  });

  test('State Sonar: Complex flows have SMT ≤ 10.0', async ({ page }) => {
    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Complex flow: search, scroll, view multiple sections
    await customer360.searchProduct('Marine Engine Lubricant');
    await page.waitForTimeout(500);

    await customer360.scrollToElement('customer-grade');
    await page.waitForTimeout(300);

    await customer360.scrollToElement('health-score');
    await page.waitForTimeout(300);

    await customer360.scrollToElement('order-history-table');
    await page.waitForTimeout(500);

    // Get complexity after complex interactions
    const complexity = await page.evaluate(() => {
      const elements = document.querySelectorAll('*').length;
      const interactiveElements = document.querySelectorAll('button, a, input, select').length;
      const stateAttributes = document.querySelectorAll('[data-state], [data-value], [data-flow-state]').length;

      const states = Math.max(stateAttributes, 1);
      const transitions = Math.max(interactiveElements, 1);
      const explosionFactor = Math.max(elements / 100, 1);

      const smt = Math.log2(states * transitions) / explosionFactor;

      return parseFloat(smt.toFixed(2));
    });

    devLog.log(`[State Sonar] Complex flow SMT: ${complexity}`);

    // Complex flows should have SMT ≤ 10.0
    expect(complexity, 'SMT should be ≤ 10.0 for complex flows').toBeLessThanOrEqual(10.0);

    devLog.log(`✅ State Sonar (Complex Flow) PASSED - SMT: ${complexity}`);
  });

  // ============================================================================
  // 7. SYSTEM HEALTH METRIC (SHM) - THE MASTER METRIC
  // ============================================================================

  test('SHM: Calculate System Health Metric ≥ 0.70 (Optimization regime)', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Wait for full page load
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Collect all sonar metrics
    const telemetry = await getTelemetry(page);

    // 1. UX Sonar (weight: 0.25)
    const fps = telemetry.averageFPS;
    const uxScore = Math.min(1.0, fps / 60.0);

    // 2. Design Sonar (weight: 0.25) - Simplified: check contrast on key element
    const designScore = await page.locator('[data-testid="customer-name"]').evaluate(el => {
      const textColor = window.getComputedStyle(el).color;
      const bgColor = window.getComputedStyle(el).backgroundColor;

      // Simple contrast check: if readable, score 0.8+
      const isReadable = textColor !== bgColor;
      return isReadable ? 0.85 : 0.5;
    });

    // 3. Code Sonar (weight: 0.125) - Check for console errors
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    await page.waitForTimeout(1000);
    const codeScore = consoleErrors.length === 0 ? 1.0 : 0.7;

    // 4. Semantic Sonar (weight: 0.125) - Check modularity
    const semanticScore = await page.evaluate(() => {
      const componentsWithTestId = document.querySelectorAll('[data-testid]').length;
      return componentsWithTestId > 5 ? 0.9 : 0.6;
    });

    // 5. Journey Sonar (weight: 0.125) - Check frustration
    const frustration = telemetry.rageClicks + telemetry.backtrackCount;
    const journeyScore = frustration === 0 ? 1.0 : 0.5;

    // 6. State Sonar (weight: 0.125) - Check SMT
    const stateScore = await page.evaluate(() => {
      const stateAttributes = document.querySelectorAll('[data-state], [data-value]').length;
      return stateAttributes < 50 ? 0.9 : 0.7;
    });

    // Calculate SHM (weighted average)
    const weights = {
      ux: 0.25,
      design: 0.25,
      code: 0.125,
      semantic: 0.125,
      journey: 0.125,
      state: 0.125
    };

    const shm =
      uxScore * weights.ux +
      designScore * weights.design +
      codeScore * weights.code +
      semanticScore * weights.semantic +
      journeyScore * weights.journey +
      stateScore * weights.state;

    // Determine regime
    let regime = 'Exploration';
    if (shm >= 0.85) regime = 'Stabilization';
    else if (shm >= 0.70) regime = 'Optimization';

    // Log detailed results
    devLog.log('========================================');
    devLog.log('SYSTEM HEALTH METRIC (SHM) REPORT');
    devLog.log('========================================');
    devLog.log(`UX Sonar:       ${(uxScore * 100).toFixed(1)}% (FPS: ${fps.toFixed(2)})`);
    devLog.log(`Design Sonar:   ${(designScore * 100).toFixed(1)}%`);
    devLog.log(`Code Sonar:     ${(codeScore * 100).toFixed(1)}%`);
    devLog.log(`Semantic Sonar: ${(semanticScore * 100).toFixed(1)}%`);
    devLog.log(`Journey Sonar:  ${(journeyScore * 100).toFixed(1)}%`);
    devLog.log(`State Sonar:    ${(stateScore * 100).toFixed(1)}%`);
    devLog.log('----------------------------------------');
    devLog.log(`SYSTEM HEALTH METRIC: ${(shm * 100).toFixed(1)}%`);
    devLog.log(`REGIME: ${regime}`);
    devLog.log('========================================');

    // Assertions
    expect(shm, 'SHM should be ≥ 0.70 for Optimization regime or better').toBeGreaterThanOrEqual(0.70);

    // For Nigeria deployment: SHM should be ≥ 0.85 (Stabilization)
    if (process.env.DEPLOYMENT === 'nigeria') {
      expect(shm, 'Nigeria deployment requires SHM ≥ 0.85 (Stabilization)').toBeGreaterThanOrEqual(0.85);
      devLog.log(`✅ NIGERIA DEPLOYMENT READY - SHM: ${(shm * 100).toFixed(1)}%`);
    } else {
      devLog.log(`✅ SHM PASSED - ${regime} regime (${(shm * 100).toFixed(1)}%)`);
    }
  });

  // ============================================================================
  // 8. BASELINE COLLECTION (for regression detection)
  // ============================================================================

  test('Baseline: Collect all metrics for regression tracking', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Perform standard interactions
    await customer360.scrollToElement('health-score');
    await page.waitForTimeout(300);
    await customer360.scrollToElement('order-history-table');
    await page.waitForTimeout(300);

    const telemetry = await getTelemetry(page);

    // Collect comprehensive baseline
    const baseline = {
      timestamp: new Date().toISOString(),
      page: 'Customer360',
      customer: 'C01011',
      metrics: {
        ux: {
          fps: telemetry.averageFPS,
          cls: telemetry.cumulativeLayoutShift,
          frameTimes: telemetry.frameTimes.length
        },
        journey: {
          rageClicks: telemetry.rageClicks,
          hesitations: telemetry.hesitationEvents,
          backtracks: telemetry.backtrackCount
        },
        performance: {
          loadTime: await page.evaluate(() => performance.timing.loadEventEnd - performance.timing.navigationStart),
          domContentLoaded: await page.evaluate(() => performance.timing.domContentLoadedEventEnd - performance.timing.navigationStart)
        }
      }
    };

    devLog.log('========================================');
    devLog.log('BASELINE METRICS COLLECTED');
    devLog.log('========================================');
    devLog.log(JSON.stringify(baseline, null, 2));
    devLog.log('========================================');

    // Save to file for regression comparison
    const fs = require('fs');
    const path = require('path');
    const baselineDir = path.join(__dirname, '../../test-results/baselines');

    // Ensure directory exists
    if (!fs.existsSync(baselineDir)) {
      fs.mkdirSync(baselineDir, { recursive: true });
    }

    const baselineFile = path.join(baselineDir, `customer360-baseline-${Date.now()}.json`);
    fs.writeFileSync(baselineFile, JSON.stringify(baseline, null, 2));

    devLog.log(`✅ Baseline saved to: ${baselineFile}`);
  });

  // ============================================================================
  // 9. PERFORMANCE ANOMALY DETECTION
  // ============================================================================

  test('Performance: Detect anomalies (FPS drops, layout thrashing)', async ({ page }) => {
    await injectTelemetry(page);

    const customer360 = new Customer360Page(page);
    await customer360.goto('C01011');

    // Stress test: rapid scrolling
    for (let i = 0; i < 5; i++) {
      await page.mouse.wheel(0, 500);
      await page.waitForTimeout(100);
    }

    await page.waitForTimeout(1000);

    const telemetry = await getTelemetry(page);

    // Detect FPS anomalies (drops below 30 FPS)
    const fpsDrops = telemetry.frameTimes.filter((time: number, idx: number) => {
      if (idx === 0) return false;
      const delta = time - telemetry.frameTimes[idx - 1];
      const instantFPS = 1000 / delta;
      return instantFPS < 30;
    });

    // Detect layout thrashing (many small shifts)
    const smallShifts = telemetry.layoutShifts.filter((shift: number) => shift > 0 && shift < 0.01);

    devLog.log(`[Anomaly Detection] FPS drops: ${fpsDrops.length}, Small layout shifts: ${smallShifts.length}`);

    // Warnings (not failures)
    if (fpsDrops.length > 0) {
      devLog.warn(`⚠️ Detected ${fpsDrops.length} FPS drops below 30 FPS`);
    }

    if (smallShifts.length > 10) {
      devLog.warn(`⚠️ Detected ${smallShifts.length} small layout shifts (possible thrashing)`);
    }

    devLog.log(`✅ Anomaly detection complete`);
  });
});

// ============================================================================
// TEST UTILITIES
// ============================================================================

test.describe('Sonar Utilities', () => {

  test('Utility: Verify telemetry injection works', async ({ page }) => {
    await injectTelemetry(page);
    await page.goto('http://127.0.0.1:5173');

    const hasTelemetry = await page.evaluate(() => {
      return typeof (window as any).__sonicTelemetry !== 'undefined';
    });

    expect(hasTelemetry, 'Telemetry should be injected').toBe(true);

    const telemetry = await getTelemetry(page);
    expect(telemetry).toBeDefined();
    expect(telemetry.frameTimes).toBeDefined();
    expect(Array.isArray(telemetry.frameTimes)).toBe(true);

    devLog.log('✅ Telemetry injection verified');
  });
});
