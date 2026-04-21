import { test, expect } from '@playwright/test';
import { MobilePage } from '../pages/mobile.page.js';
import { checkoutViaAPI } from '../helpers/api.js';

test.describe('Mobile Check-In', () => {
  test('check in a student and see confirmation', async ({ page }) => {
    const mobile = new MobilePage(page);
    await mobile.goto();

    await mobile.searchStudent('Ali');
    await expect(mobile.searchResults).toBeVisible();
    await mobile.selectFirstResult();

    await mobile.submitCheckin();
    await mobile.waitForConfirmation();

    await expect(mobile.confirmedName).toContainText('Alice Wang');
    // Could be "Checked in!" or "Already checked in!" depending on DB state
    await expect(mobile.confirmedStatus).toContainText(/checked in/i);

    // Cleanup
    await checkoutViaAPI('Alice Wang');
  });

  test('search by student ID', async ({ page }) => {
    const mobile = new MobilePage(page);
    await mobile.goto();

    await mobile.searchStudent('S003');
    await expect(mobile.searchResults.locator('li')).toHaveCount(1);
    await expect(mobile.searchResults.locator('li').first()).toContainText('Carlos Garcia');
  });

  test('inactive student not found', async ({ page }) => {
    const mobile = new MobilePage(page);
    await mobile.goto();

    await mobile.studentName.fill('Karen');
    await page.waitForTimeout(500);
    await expect(mobile.searchResults).toBeHidden();
  });

  test('unknown student shows error', async ({ page }) => {
    const mobile = new MobilePage(page);
    await mobile.goto();

    await mobile.studentName.fill('Nobody Special');
    await mobile.submitCheckin();

    await expect(mobile.checkinMessage).toBeVisible();
    await expect(mobile.checkinMessage).toContainText(/not found|required/i);
  });

  test('duplicate check-in shows already message', async ({ page }) => {
    const mobile = new MobilePage(page);
    await mobile.goto();

    // First check-in
    await mobile.checkin('Carlos');
    await mobile.waitForConfirmation();

    // Click "Check in again" to reveal the form
    await page.click('#twistie-btn');
    await mobile.studentName.waitFor({ state: 'visible' });

    // Second check-in for same student
    await mobile.checkin('Carlos');
    await mobile.waitForConfirmation();

    await expect(mobile.confirmedStatus).toContainText(/already/i);

    // Cleanup
    await checkoutViaAPI('Carlos Garcia');
  });
});
