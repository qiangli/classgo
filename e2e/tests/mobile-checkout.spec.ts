import { test, expect } from '@playwright/test';
import { MobilePage } from '../pages/mobile.page.js';
import { forceCheckoutViaAPI } from '../helpers/api.js';

test.describe('Mobile Checkout', () => {
  test('checkout after check-in succeeds', async ({ page }) => {
    await forceCheckoutViaAPI('Bob Wang').catch(() => {});

    const mobile = new MobilePage(page);
    await mobile.goto();

    await mobile.checkin('Bob');
    await mobile.waitForConfirmation();
    await expect(mobile.confirmedName).toContainText('Bob Wang');

    // Click checkout
    await mobile.checkoutBtn.click();

    // Wait for either: confirmed status changes to "Checked out!" OR tracker overlay appears
    const result = await Promise.race([
      mobile.trackerOverlay.waitFor({ state: 'visible', timeout: 5000 })
        .then(() => 'tracker' as const),
      mobile.confirmedStatus.filter({ hasText: 'Checked out!' }).waitFor({ timeout: 5000 })
        .then(() => 'done' as const),
    ]);

    if (result === 'tracker') {
      // Respond to all tracker items with "Done"
      const doneButtons = mobile.trackerOverlay.locator('button:has-text("Done")');
      const count = await doneButtons.count();
      for (let i = 0; i < count; i++) {
        await doneButtons.nth(i).click();
      }
      // Submit tracker responses to complete checkout
      await page.locator('#tracker-submit-btn').click();
    }

    await expect(mobile.confirmedStatus).toHaveText('Checked out!');
  });
});
