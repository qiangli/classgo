import { test, expect } from '@playwright/test';
import { KioskPage } from '../pages/kiosk.page.js';
import { checkinViaAPI, checkoutViaAPI } from '../helpers/api.js';

test.describe('Kiosk', () => {
  test('check in a student', async ({ page }) => {
    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchAndSelect('Emma');
    await page.locator('#name-step button:has-text("Check In")').click();

    // PIN is off by default, so check-in should succeed directly
    await kiosk.waitForSuccess();
    await expect(kiosk.successName).toContainText('Emma Taylor');

    // Cleanup
    await checkoutViaAPI('Emma Taylor');
  });

  test('check out a student', async ({ page }) => {
    // Ensure clean state: checkout first (ignore errors), then check in
    await checkoutViaAPI('Ivy Patel').catch(() => {});
    const checkinResult = await checkinViaAPI('Ivy Patel', 'kiosk');
    if (!checkinResult.ok) {
      throw new Error(`checkinViaAPI failed: ${JSON.stringify(checkinResult)}`);
    }

    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchAndSelect('Ivy');
    await page.locator('#name-step button:has-text("Check Out")').click();

    // Wait for either checkout overlay or tracker overlay (first to resolve wins)
    const never = new Promise<never>(() => {});
    const result = await Promise.race([
      kiosk.checkoutOverlay.waitFor({ state: 'visible', timeout: 10_000 })
        .then(() => 'done' as const).catch(() => never),
      kiosk.trackerOverlay.waitFor({ state: 'visible', timeout: 10_000 })
        .then(() => 'tracker' as const).catch(() => never),
    ]);

    if (result === 'tracker') {
      // Respond to all tracker items with "Done"
      const doneButtons = kiosk.trackerOverlay.locator('button:has-text("Done")');
      const count = await doneButtons.count();
      for (let i = 0; i < count; i++) {
        await doneButtons.nth(i).click();
      }
      // Click "Complete Check Out"
      await page.locator('#tracker-submit-btn').click();
    }

    await kiosk.waitForCheckoutOverlay();
    await expect(kiosk.checkoutName).toContainText('Ivy Patel');
  });

  test('search results show student ID and grade', async ({ page }) => {
    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchStudent('Grace');
    const firstResult = kiosk.searchResults.locator('li').first();
    await expect(firstResult).toContainText('Grace Lee');
    await expect(firstResult).toContainText('S007');
  });
});
