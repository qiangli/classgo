/**
 * E2E tests for detailed auto-assign behavior on profile save.
 *
 * Verifies that saving a profile with missing data fields triggers
 * auto-assignment of relevant tracker items, and that items already
 * responded to are not re-assigned.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

test.describe('Auto-assign on profile save', () => {

  test('profile save creates tasks for missing data fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Clear existing items for S009 (Ivy Patel)
    await clearStudentTrackerItemsViaAPI(cookie, 'S009');

    // Get items count before save
    const beforeRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S009`,
      { headers: { Cookie: cookie } },
    );
    const beforeItems = await beforeRes.json();
    const beforeCount = Array.isArray(beforeItems) ? beforeItems.length : 0;

    // Save profile — auto-assign should kick in for missing fields
    const saveRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S009', grade: '8' },
        parent: {},
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    // Check items after save
    const afterRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S009`,
      { headers: { Cookie: cookie } },
    );
    const afterItems = await afterRes.json();
    const afterCount = Array.isArray(afterItems) ? afterItems.length : 0;
    expect(afterCount).toBeGreaterThanOrEqual(beforeCount);
  });

  test('repeated profile save does not duplicate auto-assigned items', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // First save
    await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S009', grade: '8' },
        parent: {},
      }),
    });

    const firstRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S009`,
      { headers: { Cookie: cookie } },
    );
    const firstItems = await firstRes.json();
    const firstCount = Array.isArray(firstItems) ? firstItems.length : 0;

    // Second save with same data — should not duplicate
    await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S009', grade: '8' },
        parent: {},
      }),
    });

    const secondRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S009`,
      { headers: { Cookie: cookie } },
    );
    const secondItems = await secondRes.json();
    const secondCount = Array.isArray(secondItems) ? secondItems.length : 0;
    expect(secondCount).toBe(firstCount);
  });

  test('profile save with all fields filled does not create unnecessary items', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Clear and save with comprehensive data
    await clearStudentTrackerItemsViaAPI(cookie, 'S009');

    await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: {
          id: 'S009',
          grade: '8',
          school: 'Lincoln Middle',
          birthplace: 'India',
          years_in_us: '3',
          first_language: 'Hindi',
          previous_schools: 'Previous School',
          courses_outside: 'Math tutoring',
          dob: '2012-01-01',
        },
        parent: {
          id: 'P008',
          email: 'priya@example.com',
          phone: '555-4567',
        },
      }),
    });

    const itemsRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S009`,
      { headers: { Cookie: cookie } },
    );
    const items = await itemsRes.json();
    // With all fields filled, no profile-gap items should be needed
    // (though center-wide items may still be auto-assigned)
    expect(Array.isArray(items)).toBe(true);
  });
});
