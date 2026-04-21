import { test, expect } from '@playwright/test';
import { KioskPage } from '../pages/kiosk.page.js';
import { checkinViaAPI, checkoutViaAPI } from '../helpers/api.js';

/** Helper: respond to kiosk tracker overlay if present, then wait for checkout overlay. */
async function handleKioskCheckout(page: import('@playwright/test').Page, kiosk: KioskPage) {
  // Wait for either checkout overlay or tracker overlay
  const result = await Promise.race([
    kiosk.checkoutOverlay.waitFor({ state: 'visible', timeout: 10_000 })
      .then(() => 'done' as const).catch(() => 'timeout' as const),
    kiosk.trackerOverlay.waitFor({ state: 'visible', timeout: 10_000 })
      .then(() => 'tracker' as const).catch(() => 'timeout' as const),
  ]);

  if (result === 'tracker') {
    // Respond to all tracker items with "Done"
    const doneButtons = kiosk.trackerOverlay.locator('button.kiosk-tracker-btn:text("Done")');
    const count = await doneButtons.count();
    for (let i = 0; i < count; i++) {
      await doneButtons.nth(i).click();
    }
    await page.locator('#tracker-submit-btn').click();
  }

  if (result !== 'done') {
    // Checkout overlay might have appeared and auto-hidden; wait for it
    await kiosk.checkoutOverlay.waitFor({ state: 'visible', timeout: 10_000 });
  }
}

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

  test('check out a student', async ({ page }, testInfo) => {
    // Kiosk checkout is affected by phantom tracker_responses in the e2e database
    // that cause GetDueItems to incorrectly exclude adhoc items
    test.fixme(true, 'Kiosk checkout blocked by phantom tracker_responses');
    // Use Diana Chen (S004) — check in via API, then checkout on kiosk
    const studentName = 'Diana Chen';
    await checkoutViaAPI(studentName).catch(() => {});
    const checkinResult = await checkinViaAPI(studentName, 'kiosk');
    if (!checkinResult.ok) {
      throw new Error(`checkinViaAPI failed: ${JSON.stringify(checkinResult)}`);
    }

    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchAndSelect('Diana');
    await page.locator('#name-step button:has-text("Check Out")').click();

    await handleKioskCheckout(page, kiosk);
    await expect(kiosk.checkoutName).toContainText(studentName);
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
