import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { MobilePage } from '../pages/mobile.page.js';
import {
  setPinModeViaAPI,
  setPinViaAPI,
  setStudentPinRequireViaAPI,
  checkoutViaAPI,
  forceCheckoutViaAPI,
  clearStudentTrackerItemsViaAPI,
} from '../helpers/api.js';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

/** Handle mobile checkout: if PIN field appears, fill PIN and resubmit. Handle tracker overlay. */
async function doMobileCheckout(mobile: MobilePage, page: import('@playwright/test').Page, pin?: string) {
  await mobile.checkoutBtn.click();

  // If PIN is needed, the checkout-pin-field appears
  if (pin) {
    const needsPin = await mobile.checkoutPinField.isVisible().catch(() => false);
    if (needsPin) {
      await mobile.checkoutPinInput.fill(pin);
      await mobile.checkoutBtn.click();
    }
  }

  // Handle tracker overlay or wait for checkout
  const result = await Promise.race([
    mobile.trackerOverlay.waitFor({ state: 'visible', timeout: 3000 })
      .then(() => 'tracker' as const).catch(() => 'none' as const),
    mobile.confirmedStatus.filter({ hasText: 'Checked out!' }).waitFor({ timeout: 3000 })
      .then(() => 'done' as const).catch(() => 'none' as const),
  ]);
  if (result === 'tracker') {
    const doneButtons = mobile.trackerOverlay.locator('button:has-text("Done")');
    const count = await doneButtons.count();
    for (let i = 0; i < count; i++) await doneButtons.nth(i).click();
    await page.locator('#tracker-submit-btn').click();
  }
}

// ==================== Mobile: Full check-in → checkout with PIN OFF ====================

test.describe('Mobile check-in → checkout (PIN off)', () => {

  // Use S004 (Diana Chen) — dedicated to this test
  test('full flow: check-in and checkout with PIN off', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S004');
    await forceCheckoutViaAPI('Diana Chen').catch(() => {});

    const mobile = new MobilePage(page);
    await mobile.goto();

    // Check in
    await mobile.checkin('Diana');
    await mobile.waitForConfirmation();
    await expect(mobile.confirmedName).toContainText('Diana Chen');
    await expect(mobile.confirmedStatus).toContainText(/checked in/i);

    // PIN fields should NOT be visible
    await expect(mobile.pinField).toBeHidden();

    // Checkout
    await doMobileCheckout(mobile, page);
    await expect(mobile.confirmedStatus).toHaveText('Checked out!');
  });
});

// ==================== Mobile: Full check-in → checkout with Center PIN ON ====================

test.describe('Mobile check-in → checkout (center PIN on)', () => {

  // Use S005 (Emma Taylor) — dedicated to this test block
  // This test mirrors pin-mid-session.spec.ts pattern: enable center PIN AFTER check-in,
  // then verify checkout requires PIN entry
  test('check-in then enable center PIN — checkout requires PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(cookie, 'S005');
    await forceCheckoutViaAPI('Emma Taylor').catch(() => {});

    // Start with PIN off for check-in
    await setPinModeViaAPI(cookie, 'off');

    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Emma');
    await mobile.waitForConfirmation();
    await expect(mobile.confirmedName).toContainText('Emma Taylor');

    // NOW enable center PIN
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '1234');

    try {
      // Attempt checkout — should require PIN
      await mobile.checkout();
      await expect(mobile.checkoutPinField).toBeVisible();

      // Enter correct PIN and retry
      await mobile.checkoutPinInput.fill('1234');
      await mobile.checkoutBtn.click();

      await expect(mobile.confirmedStatus).toHaveText('Checked out!');
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  // Use S007 (Grace Lee) — dedicated to this test
  test('check-in rejected without PIN when center PIN is on', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await forceCheckoutViaAPI('Grace Lee').catch(() => {});
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '9999');

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      await mobile.searchAndSelect('Grace');

      // PIN field should be visible
      await expect(mobile.pinField).toBeVisible();

      // Don't fill PIN, just submit
      await mobile.submitCheckin();

      // Should show error or not show confirmation
      await expect(mobile.confirmedCard).toBeHidden();
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  // Use S003 (Carlos Garcia) — dedicated to this test
  test('checkout rejected with wrong center PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(cookie, 'S003');
    await forceCheckoutViaAPI('Carlos Garcia').catch(() => {});

    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '5555');

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      // Check in with correct PIN
      await mobile.checkin('Carlos', '5555');
      await mobile.waitForConfirmation();

      // Attempt checkout with wrong PIN
      await mobile.checkout();
      await expect(mobile.checkoutPinField).toBeVisible();
      await mobile.checkoutPinInput.fill('0000');
      await mobile.checkoutBtn.click();

      // Should NOT show "Checked out!" — should stay or show error
      await page.waitForTimeout(1000);
      const status = await mobile.confirmedStatus.textContent();
      expect(status).not.toBe('Checked out!');
    } finally {
      await setPinModeViaAPI(cookie, 'off');
      await forceCheckoutViaAPI('Carlos Garcia').catch(() => {});
    }
  });
});

// ==================== Mobile: Flagged student personal PIN ====================

test.describe('Mobile check-in → checkout (student personal PIN)', () => {

  // Use S008 (Henry Kim) — dedicated to flagged student tests
  test('flagged student requires personal PIN to check in', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await forceCheckoutViaAPI('Henry Kim').catch(() => {});

    // Flag student — generates personal PIN
    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S008', true);
    const personalPin = flagRes.pin;
    expect(personalPin).toBeTruthy();

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      await mobile.searchAndSelect('Henry');

      // PIN field should be visible (student is flagged)
      await expect(mobile.pinField).toBeVisible();

      // Check in with personal PIN
      await mobile.fillPin(personalPin);
      await mobile.submitCheckin();
      await mobile.waitForConfirmation();
      await expect(mobile.confirmedName).toContainText('Henry Kim');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S008', false);
      await forceCheckoutViaAPI('Henry Kim').catch(() => {});
    }
  });

  test('flagged student rejected with wrong PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await forceCheckoutViaAPI('Henry Kim').catch(() => {});

    await setStudentPinRequireViaAPI(cookie, 'S008', true);

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      await mobile.searchAndSelect('Henry');
      await expect(mobile.pinField).toBeVisible();

      // Wrong PIN
      await mobile.fillPin('0000');
      await mobile.submitCheckin();

      // Should show error, not confirmation
      await expect(mobile.checkinMessage).toBeVisible();
      await expect(mobile.confirmedCard).toBeHidden();
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S008', false);
    }
  });

  // Use S004 (Diana Chen) for unflagged test
  test('unflagged student does not need PIN even with other students flagged', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await forceCheckoutViaAPI('Diana Chen').catch(() => {});

    // Flag S008 (not Diana)
    await setStudentPinRequireViaAPI(cookie, 'S008', true);

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      // S004 (Diana) is NOT flagged — should not need PIN
      await mobile.searchAndSelect('Diana');

      // PIN field should NOT be visible for unflagged student
      await expect(mobile.pinField).toBeHidden();

      await mobile.submitCheckin();
      await mobile.waitForConfirmation();
      await expect(mobile.confirmedName).toContainText('Diana Chen');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S008', false);
      await forceCheckoutViaAPI('Diana Chen').catch(() => {});
    }
  });
});
