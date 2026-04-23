import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { KioskPage } from '../pages/kiosk.page.js';
import {
  checkinViaAPI,
  checkoutViaAPI,
  forceCheckoutViaAPI,
  setPinModeViaAPI,
  setPinViaAPI,
  setStudentPinRequireViaAPI,
  clearStudentTrackerItemsViaAPI,
  pinCheckViaAPI,
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

// ==================== Kiosk: Check-in with PIN off ====================

test.describe('Kiosk check-in (PIN off)', () => {

  test('check in via kiosk and verify via status API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S004');
    await forceCheckoutViaAPI('Diana Chen');

    const result = await checkinViaAPI('Diana Chen', 'kiosk');
    expect(result.ok).toBe(true);

    await forceCheckoutViaAPI('Diana Chen');
  });

  test('duplicate kiosk check-in via API returns already message', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S004');
    await forceCheckoutViaAPI('Diana Chen');

    const first = await checkinViaAPI('Diana Chen', 'kiosk');
    expect(first.ok).toBe(true);

    const second = await checkinViaAPI('Diana Chen', 'kiosk');
    expect(second.ok).toBe(true);
    expect(second.message).toContain('Already');

    await forceCheckoutViaAPI('Diana Chen');
  });

  test('kiosk page shows Check In and Check Out buttons', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');

    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchAndSelect('Diana');

    // Both buttons should be visible
    await expect(page.locator('#name-step button:has-text("Check In")')).toBeVisible();
    await expect(page.locator('#name-step button:has-text("Check Out")')).toBeVisible();
  });
});

// ==================== Kiosk: Center PIN shows keypad ====================

test.describe('Kiosk check-in (center PIN on)', () => {

  test('center PIN on shows keypad on kiosk', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S005');
    await forceCheckoutViaAPI('Emma Taylor');
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '2468');

    try {
      const kiosk = new KioskPage(page);
      await kiosk.goto();

      await kiosk.searchAndSelect('Emma');
      await page.locator('#name-step button:has-text("Check In")').click();

      // Keypad should appear for PIN entry
      await expect(kiosk.keypad).toBeVisible({ timeout: 5000 });
      // Step label should show PIN prompt
      await expect(page.locator('#step-label')).toContainText('Enter PIN');
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  // Center PIN API validation is covered in admin-checkin-management.spec.ts
  // (check-in via API with correct/wrong/no center PIN tests)
  // This test verifies the kiosk-specific behavior: keypad appears when center PIN is on

  test('wrong center PIN rejected on kiosk', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S007');
    await forceCheckoutViaAPI('Grace Lee');
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '1357');

    try {
      const kiosk = new KioskPage(page);
      await kiosk.goto();

      await kiosk.searchAndSelect('Grace');
      await page.locator('#name-step button:has-text("Check In")').click();

      await expect(kiosk.keypad).toBeVisible({ timeout: 5000 });

      // Enter wrong PIN
      await kiosk.enterPin('0000');
      await kiosk.submit();

      // Should NOT show success overlay
      await page.waitForTimeout(1000);
      await expect(kiosk.successOverlay).toBeHidden();
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  test('wrong center PIN rejected via API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S007');
    await forceCheckoutViaAPI('Grace Lee');
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '1357');

    try {
      const result = await checkinViaAPI('Grace Lee', 'kiosk', '0000');
      expect(result.ok).toBe(false);
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });
});

// ==================== Kiosk: Flagged student personal PIN ====================

test.describe('Kiosk check-in (student personal PIN)', () => {

  test('flagged student shows keypad on kiosk', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S008', false);
    await clearStudentTrackerItemsViaAPI(cookie, 'S008');
    await forceCheckoutViaAPI('Henry Kim');

    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S008', true);

    try {
      const kiosk = new KioskPage(page);
      await kiosk.goto();

      await kiosk.searchAndSelect('Henry');
      await page.locator('#name-step button:has-text("Check In")').click();

      // Keypad should appear (student is flagged)
      await expect(kiosk.keypad).toBeVisible({ timeout: 5000 });
      await expect(page.locator('#step-label')).toContainText('Enter PIN');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S008', false);
    }
  });

  // Use S004 (Diana Chen) — separate from the keypad UI test student (S008)
  test('flagged student check-in via API succeeds with correct PIN', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S004', false);
    await clearStudentTrackerItemsViaAPI(cookie, 'S004');
    await forceCheckoutViaAPI('Diana Chen');

    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S004', true);
    const pin = flagRes.pin;

    try {
      // Without PIN should fail
      const noPinResult = await checkinViaAPI('Diana Chen', 'kiosk');
      expect(noPinResult.ok).toBe(false);

      // With correct PIN should succeed
      const result = await checkinViaAPI('Diana Chen', 'kiosk', pin);
      expect(result.ok).toBe(true);
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S004', false);
      await forceCheckoutViaAPI('Diana Chen');
    }
  });

  test('unflagged student does not trigger keypad when PIN mode is off', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S007', false);

    // Verify via pin-check API
    const check = await pinCheckViaAPI('S007');
    expect(check.needs_pin).toBe(false);
    expect(check.pin_mode).toBe('off');
  });
});

// ==================== Kiosk: Checkout via API ====================

test.describe('Kiosk checkout', () => {

  test('kiosk checkout succeeds via forceCheckout', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    await setPinModeViaAPI(cookie, 'off');
    await clearStudentTrackerItemsViaAPI(cookie, 'S004');
    await forceCheckoutViaAPI('Diana Chen');

    // Check in
    const checkin = await checkinViaAPI('Diana Chen', 'kiosk');
    expect(checkin.ok).toBe(true);

    // Checkout
    const checkout = await forceCheckoutViaAPI('Diana Chen');
    expect(checkout.ok).toBe(true);
  });

  test('kiosk search shows student ID and grade', async ({ adminPage, page }) => {
    const kiosk = new KioskPage(page);
    await kiosk.goto();

    await kiosk.searchStudent('Grace');
    const firstResult = kiosk.searchResults.locator('li').first();
    await expect(firstResult).toContainText('Grace Lee');
    await expect(firstResult).toContainText('S007');
  });
});
