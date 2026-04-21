import { test, expect } from '../fixtures/auth.js';
import { AdminPage } from '../pages/admin.page.js';
import { MobilePage } from '../pages/mobile.page.js';
import { setPinModeViaAPI, setPinViaAPI, setStudentPinRequireViaAPI } from '../helpers/api.js';
import { hasAdminAuth } from '../fixtures/auth.js';

// All tests in this file require admin auth
test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

test.describe('Admin PIN Management', () => {
  test('sidebar navigation switches sections', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();

    // Navigate to checkin section
    await admin.navigateTo('checkin');
    await expect(admin.section('checkin')).toBeVisible();
    await expect(admin.section('audit')).toBeHidden();

    // Navigate to audit section
    await admin.navigateTo('audit');
    await expect(admin.section('audit')).toBeVisible();
    await expect(admin.section('checkin')).toBeHidden();
  });

  test('audit section shows PIN mode as read-only', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('audit');

    // PIN mode labels should be spans (not buttons)
    const offLabel = admin.pinModeOff;
    await expect(offLabel).toBeVisible();
    // Verify it's a span, not a button
    const tagName = await offLabel.evaluate(el => el.tagName.toLowerCase());
    expect(tagName).toBe('span');
  });

  test('audit section PIN override list is read-only (no action buttons)', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);

    // First flag a student so the list is non-empty
    const cookies = await adminPage.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'classgo_session');
    const cookie = `classgo_session=${sessionCookie!.value}`;
    await setStudentPinRequireViaAPI(cookie, 'S007', true);

    await admin.goto();
    await admin.navigateTo('audit');

    // Wait for the audit override list to load
    await adminPage.waitForTimeout(500);

    const auditList = admin.pinOverrideListAudit;
    await expect(auditList).toBeVisible();
    // Should show student name but no Regenerate/Remove buttons
    await expect(auditList).toContainText('Grace Lee');
    await expect(auditList.locator('button')).toHaveCount(0);

    // Cleanup
    await setStudentPinRequireViaAPI(cookie, 'S007', false);
  });

  test('checkin section allows managing PIN overrides', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('checkin');

    // Search for a student in the PIN override search
    await admin.searchPinOverride('Henry');
    await expect(admin.pinOverrideResults).toBeVisible();
    await admin.selectFirstPinOverrideResult();

    // Wait for the list to update
    await adminPage.waitForTimeout(500);

    // Student should appear in the override list with action buttons
    await expect(admin.pinOverrideList).toContainText('Henry Kim');
    await expect(admin.pinOverrideList.locator('button:has-text("Regenerate")')).toBeVisible();
    await expect(admin.pinOverrideList.locator('button:has-text("Remove")')).toBeVisible();

    // Cleanup: remove the override
    await admin.pinOverrideList.locator('button:has-text("Remove")').click();
    await adminPage.waitForTimeout(500);
  });

  test('center PIN mode makes mobile page show PIN field', async ({ adminPage, page }) => {
    const cookies = await adminPage.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'classgo_session');
    const cookie = `classgo_session=${sessionCookie!.value}`;

    // Enable center PIN
    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '5678');

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      await mobile.searchAndSelect('Frank');

      // PIN field should be visible
      await expect(mobile.pinField).toBeVisible();

      // Check in with correct PIN
      await mobile.fillPin('5678');
      await mobile.submitCheckin();
      await mobile.waitForConfirmation();
      await expect(mobile.confirmedName).toContainText('Frank Miller');
    } finally {
      // Cleanup: reset PIN mode
      await setPinModeViaAPI(cookie, 'off');
    }
  });

  test('wrong center PIN is rejected', async ({ adminPage, page }) => {
    const cookies = await adminPage.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'classgo_session');
    const cookie = `classgo_session=${sessionCookie!.value}`;

    await setPinModeViaAPI(cookie, 'center');
    await setPinViaAPI(cookie, '5678');

    try {
      const mobile = new MobilePage(page);
      await mobile.goto();

      await mobile.searchAndSelect('Ivy');
      await mobile.fillPin('0000');
      await mobile.submitCheckin();

      // Should show error, not confirmation
      await expect(mobile.checkinMessage).toBeVisible();
      await expect(mobile.confirmedCard).toBeHidden();
    } finally {
      await setPinModeViaAPI(cookie, 'off');
    }
  });
});
