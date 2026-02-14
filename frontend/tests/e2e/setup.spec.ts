import { test, expect } from '@playwright/test';

test.describe('Arrival Ceremony (Setup)', () => {
  test('Complete conversational setup', async ({ page }) => {
    // Navigate to the app (should redirect to setup if not done)
    await page.goto('http://localhost:5173');
    
    // Check if we are in the setup
    await expect(page.locator('text=Welcome, Commander')).toBeVisible({ timeout: 10000 });

    // Wait for the Architect to finish initial scan
    await expect(page.locator('text=I\'ll prepare your command center')).toBeVisible();
    
    // The setup asks for company name
    // We wait until the input is enabled
    const input = page.locator('input[placeholder="Speak your command..."]');
    await expect(input).toBeEnabled({ timeout: 15000 });
    
    // Type company name
    await input.fill('PH Trading');
    await page.keyboard.press('Enter');
    
    // Verify response
    await expect(page.locator('text=PH Trading')).toBeVisible();
    
    // Continue through the flow... 
    // This is a complex flow so we'll just verify the start for now
  });
});
