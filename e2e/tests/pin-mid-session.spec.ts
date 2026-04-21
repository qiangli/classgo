import { test, expect } from '../fixtures/auth.js';
import { MobilePage } from '../pages/mobile.page.js';
import { KioskPage } from '../pages/kiosk.page.js';
import {
  checkinViaAPI,
  setPinModeViaAPI,
  setPinViaAPI,
  setStudentPinRequireViaAPI,
  pinCheckViaAPI,
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

  test('center PIN enabled after check-in — kiosk checkout requires keypad', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // PIN off, check in via API
    await setPinModeViaAPI(cookie, 'off');
    await checkinViaAPI('Emma Taylor', 'kiosk');

    // Admin enables center PIN
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '7777');

    const kiosk = new KioskPage(page);
    await kiosk.goto();

    try {
      // Start checkout on kiosk
      await kiosk.searchAndSelect('Emma');
      await page.click('button:has-text("Check Out")');

      // Keypad should appear for PIN entry
      await expect(kiosk.keypad).toBeVisible();

      // Enter PIN via keypad
      await kiosk.enterPin('7777');
      await kiosk.submit();

      await kiosk.waitForCheckoutOverlay();
      await expect(kiosk.checkoutName).toContainText('Emma Taylor');
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

  test('flag student after check-in — kiosk checkout requires keypad', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // PIN off, unflag student, check in via API
    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S007', false);
    await checkinViaAPI('Grace Lee', 'kiosk');

    // Flag student mid-session
    const flagRes = await setStudentPinRequireViaAPI(cookie, 'S007', true);
    const personalPin = flagRes.pin;

    const kiosk = new KioskPage(page);
    await kiosk.goto();

    try {
      await kiosk.searchAndSelect('Grace');
      await page.click('button:has-text("Check Out")');

      // Keypad should appear
      await expect(kiosk.keypad).toBeVisible();

      // Enter personal PIN
      await kiosk.enterPin(personalPin);
      await kiosk.submit();

      await kiosk.waitForCheckoutOverlay();
      await expect(kiosk.checkoutName).toContainText('Grace Lee');
    } finally {
      await setStudentPinRequireViaAPI(cookie, 'S007', false);
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
