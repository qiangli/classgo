import { test, expect } from '../fixtures/auth.js';
import { MobilePage } from '../pages/mobile.page.js';
import {
  checkinViaAPI,
  checkoutViaAPI,
  forceCheckoutViaAPI,
  setPinModeViaAPI,
  setPinViaAPI,
  setStudentPinRequireViaAPI,
  pinCheckViaAPI,
  clearStudentTrackerItemsViaAPI,
} from '../helpers/api.js';
import { hasAdminAuth } from '../fixtures/auth.js';

// All tests require admin to change PIN settings
test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

test.describe('PIN State Changes Mid-Session', () => {

  test('center PIN enabled after check-in — mobile checkout requires PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Clear seeded tracker items so tracker overlay doesn't block checkout
    await clearStudentTrackerItemsViaAPI(cookie, 'S001');

    // Ensure PIN mode is off, then check in
    await setPinModeViaAPI(cookie, 'off');

    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Alice');
    await mobile.waitForConfirmation();

    // Admin enables center PIN while student is checked in
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '4444');

    try {
      // Attempt checkout — should require PIN
      await mobile.checkout();

      // The checkout-pin-field should appear or an error should show
      await expect(mobile.checkoutPinField).toBeVisible();

      // Enter correct PIN and checkout
      await mobile.checkoutPinInput.fill('4444');
      await mobile.checkoutBtn.click();

      await expect(mobile.confirmedStatus).toHaveText('Checked out!');
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  test('center PIN disabled after check-in — mobile checkout without PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Clear seeded tracker items so tracker overlay doesn't block checkout
    await clearStudentTrackerItemsViaAPI(cookie, 'S002');

    // Start with center PIN on, check in with PIN
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '3333');

    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Bob', '3333');
    await mobile.waitForConfirmation();

    // Admin disables center PIN
    await setPinModeViaAPI(cookie, 'off');

    try {
      // Checkout should succeed without PIN
      await mobile.checkout();
      await expect(mobile.confirmedStatus).toHaveText('Checked out!');
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  test('center PIN enabled after check-in — checkout API enforces PIN', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    // Use S009 (Ivy Patel) — less contention from other tests
    const studentId = 'S009';
    const studentName = 'Ivy Patel';

    await clearStudentTrackerItemsViaAPI(cookie, studentId);
    await forceCheckoutViaAPI(studentName).catch(() => {});

    // PIN off, check in via API
    const offRes = await setPinModeViaAPI(cookie, 'off');
    expect(offRes.ok, 'setPinMode off failed').toBe(true);
    const checkin = await checkinViaAPI(studentName);
    expect(checkin.ok, `Checkin failed: ${JSON.stringify(checkin)}`).toBe(true);

    // Admin enables center PIN
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '7777');

    try {
      // Checkout without PIN should be rejected
      const noPin = await checkoutViaAPI(studentName);
      expect(noPin.ok).toBe(false);

      // Checkout with correct PIN should succeed
      const withPin = await checkoutViaAPI(studentName, '7777');
      if (!withPin.ok && withPin.pending_tasks) {
        await forceCheckoutViaAPI(studentName, '7777');
      } else {
        expect(withPin.ok).toBe(true);
      }
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  test('flag student after check-in — mobile checkout requires personal PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Ensure PIN mode off, unflag student, check in
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S006', false);

    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Frank');
    await mobile.waitForConfirmation();

    // Admin flags student mid-session — this returns the generated PIN
    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S006', true);
    const personalPin = flagRes.pin;

    try {
      // Attempt checkout — should require PIN
      await mobile.checkout();
      await expect(mobile.checkoutPinField).toBeVisible();

      // Enter the personal PIN
      await mobile.checkoutPinInput.fill(personalPin);
      await mobile.checkoutBtn.click();

      await expect(mobile.confirmedStatus).toHaveText('Checked out!');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S006', false);
    }
  });

  test('flag student after check-in — checkout API enforces personal PIN', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    // Use S010 (Jack Brown) — less contention from other tests
    const studentId = 'S010';
    const studentName = 'Jack Brown';

    await clearStudentTrackerItemsViaAPI(cookie, studentId);
    await forceCheckoutViaAPI(studentName).catch(() => {});

    // PIN off, unflag student, check in via API (use 'mobile' to avoid kiosk rate limiting)
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, studentId, false);
    const checkin = await checkinViaAPI(studentName, 'mobile');
    expect(checkin.ok).toBe(true);

    // Flag student mid-session
    const flagRes = await setStudentPinRequireViaAPI(cookie, studentId, true);
    const personalPin = flagRes.pin;

    try {
      // Checkout without PIN should be rejected
      const noPin = await checkoutViaAPI(studentName);
      expect(noPin.ok).toBe(false);

      // Checkout with correct personal PIN should succeed
      const withPin = await checkoutViaAPI(studentName, personalPin);
      if (!withPin.ok && withPin.pending_tasks) {
        await forceCheckoutViaAPI(studentName, personalPin);
      } else {
        expect(withPin.ok).toBe(true);
      }
    } finally {
      await setStudentPinRequireViaAPI(cookie, studentId, false);
    }
  });

  test('unflag student after check-in — checkout without PIN', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Flag student, check in with personal PIN
    await setPinModeViaAPI(cookie, 'off');
    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S008', true);
    const personalPin = flagRes.pin;

    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Henry', personalPin);
    await mobile.waitForConfirmation();

    // Admin unflags student mid-session
    await setStudentPinRequireViaAPI(cookie, 'S008', false);

    try {
      // Checkout should succeed without PIN
      await mobile.checkout();
      await expect(mobile.confirmedStatus).toHaveText('Checked out!');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S008', false);
    }
  });
});
