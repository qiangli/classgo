import { test, expect } from '../fixtures/auth.js';
import { MobilePage } from '../pages/mobile.page.js';
import { hasAdminAuth } from '../fixtures/auth.js';

// Tracker tests require admin to create tasks
test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

test.describe('Checkout with Tracker', () => {

  test('checkout blocked when signoff task is pending', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create a requires_signoff task for Jack Brown (S010)
    const createRes = await fetch('http://localhost:9090/api/tracker/student-items', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student_id: 'S010',
        name: 'E2E Test Task',
        priority: 'high',
        recurrence: 'none',
        requires_signoff: true,
        active: true,
      }),
    });
    const createData = await createRes.json();
    expect(createData.ok).toBe(true);

    // Check in the student
    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Jack');
    await mobile.waitForConfirmation();

    // Attempt checkout — should show tracker overlay
    await mobile.checkoutBtn.click();

    await expect(mobile.trackerOverlay).toBeVisible();
    await expect(mobile.trackerOverlay).toContainText('E2E Test Task');
  });

  test('respond to tracker items and complete checkout', async ({ adminPage, page }) => {
    const cookie = await getAdminCookie(adminPage);

    // Use Jack Brown (S010) who already has a signoff item from test 1
    // Check in
    const mobile = new MobilePage(page);
    await mobile.goto();
    await mobile.checkin('Jack');
    await mobile.waitForConfirmation();

    // Attempt checkout — tracker overlay should appear (Jack has the E2E Test Task)
    await mobile.checkoutBtn.click();
    await expect(mobile.trackerOverlay).toBeVisible();
    await expect(mobile.trackerOverlay).toContainText('E2E Test Task');

    // Click "Done" on all tasks
    const doneButtons = mobile.trackerOverlay.locator('button:text("Done")');
    const count = await doneButtons.count();
    for (let i = 0; i < count; i++) {
      await doneButtons.nth(i).click();
    }

    // Submit tracker responses
    await page.locator('#tracker-submit-btn').click();

    // Checkout should complete
    await expect(mobile.trackerOverlay).toBeHidden();
    await expect(mobile.confirmedStatus).toHaveText('Checked out!');
  });
});
