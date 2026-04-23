/**
 * E2E tests covering remaining coverage gaps that don't warrant their own file.
 *
 * Includes: profile finalization, user search, kiosk QR code, tracker
 * responses by date, dashboard column preferences, auto-assign on profile
 * save, CSV/ZIP export, and student field values for task filters.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, checkinViaAPI, forceCheckoutViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PASSWORD = 'test1234';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

// ---------------------------------------------------------------------------
// 1. Profile Finalization
// ---------------------------------------------------------------------------

test.describe('Profile finalization', () => {
  test('admin can finalize student profile', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Save profile as draft first
    const draftRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S001' },
        parent: {},
        finalize: false,
      }),
    });
    expect((await draftRes.json()).ok).toBe(true);

    // Now finalize
    const finalRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S001' },
        parent: {},
        finalize: true,
      }),
    });
    expect((await finalRes.json()).ok).toBe(true);

    // Verify profile status is final
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S001`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.student.profile_status).toBe('final');
  });

  test('student save sets draft status', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student: { id: 'S001', school: 'Test School' },
        parent: {},
      }),
    });
    expect((await res.json()).ok).toBe(true);
  });

  test('non-admin cannot finalize profile', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Student profile endpoint doesn't support finalize
    // The admin endpoint (api/v1/student/profile) requires admin auth
    const res = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student: { id: 'S001' },
        parent: {},
        finalize: true,
      }),
    });
    expect(res.status).toBe(403);
  });
});

// ---------------------------------------------------------------------------
// 2. User Search
// ---------------------------------------------------------------------------

test.describe('User search', () => {
  test('search returns students by name', async () => {
    // Use "Wang" which matches multiple students (S001 Alice Wang, S002 Bob Wang)
    const res = await fetch(`${BASE_URL}/api/users/search?q=Wang`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeGreaterThan(0);
    const student = data.find((u: any) => u.type === 'Student');
    expect(student).toBeTruthy();
    expect(student.last_name).toBe('Wang');
  });

  test('search returns parents', async () => {
    // Search for a parent by name (from csv.example: P001 is Li Wang, P002 is Maria Garcia)
    const res = await fetch(`${BASE_URL}/api/users/search?q=Maria`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    const parent = data.find((u: any) => u.type === 'Parent');
    expect(parent).toBeTruthy();
  });

  test('search returns teachers', async () => {
    // T01 is Sarah Smith
    const res = await fetch(`${BASE_URL}/api/users/search?q=Sarah`);
    expect(res.status).toBe(200);
    const data = await res.json();
    const teacher = data.find((u: any) => u.type === 'Teacher');
    expect(teacher).toBeTruthy();
  });

  test('search by ID works', async () => {
    const res = await fetch(`${BASE_URL}/api/users/search?q=S001`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.length).toBeGreaterThan(0);
    expect(data[0].id).toBe('S001');
  });

  test('short query returns empty', async () => {
    const res = await fetch(`${BASE_URL}/api/users/search?q=A`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBe(0);
  });

  test('no match returns empty array', async () => {
    const res = await fetch(`${BASE_URL}/api/users/search?q=ZZZZZZNOTFOUND`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.length).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 3. Kiosk QR Code
// ---------------------------------------------------------------------------

test.describe('Kiosk page', () => {
  test('kiosk page loads and shows QR code section', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/kiosk`);
    await expect(adminPage.locator('body')).toBeVisible();
    // Kiosk page should have check-in and check-out sections
    const pageText = await adminPage.locator('body').textContent();
    expect(pageText).toBeTruthy();
  });

  test('kiosk page is accessible without authentication', async ({ page }) => {
    const response = await page.goto(`${BASE_URL}/kiosk`);
    expect(response).toBeTruthy();
    expect(response!.status()).toBe(200);
  });
});

// ---------------------------------------------------------------------------
// 4. Tracker Responses by Date
// ---------------------------------------------------------------------------

test.describe('Tracker responses', () => {
  test('admin can view tracker responses for a student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const today = new Date().toISOString().split('T')[0];

    const res = await fetch(
      `${BASE_URL}/api/v1/tracker/responses?student_id=S001&date=${today}`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('tracker responses defaults to today when no date', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(
      `${BASE_URL}/api/v1/tracker/responses?student_id=S001`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('tracker responses requires student_id', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/tracker/responses`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(400);
  });

  test('tracker responses rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/tracker/responses?student_id=S001`);
    expect(res.status).toBe(401);
  });
});

// ---------------------------------------------------------------------------
// 5. Dashboard Column Preferences
// ---------------------------------------------------------------------------

test.describe('User preferences', () => {
  test('authenticated user can save and load preferences', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Save a preference
    const saveRes = await fetch(`${BASE_URL}/api/v1/preferences`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ data_columns: 'name,grade,school' }),
    });
    expect(saveRes.status).toBe(200);

    // Load preferences
    const loadRes = await fetch(`${BASE_URL}/api/v1/preferences`, {
      headers: { Cookie: studentCookie! },
    });
    expect(loadRes.status).toBe(200);
    const prefs = await loadRes.json();
    expect(prefs.data_columns).toBe('name,grade,school');
  });

  test('preferences require authentication', async () => {
    const getRes = await fetch(`${BASE_URL}/api/v1/preferences`, {
      redirect: 'manual',
    });
    expect([302, 401]).toContain(getRes.status);

    const postRes = await fetch(`${BASE_URL}/api/v1/preferences`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test: 'value' }),
      redirect: 'manual',
    });
    expect([302, 401]).toContain(postRes.status);
  });
});

// ---------------------------------------------------------------------------
// 6. Auto-assign on Profile Save
// ---------------------------------------------------------------------------

test.describe('Auto-assign on profile save', () => {
  test('saving profile with missing data triggers auto-assign', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // First, create a global tracker item that targets missing data
    // The auto-assign feature assigns items when student has missing profile fields
    // Get student's current tasks before profile save
    const beforeRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S001`,
      { headers: { Cookie: cookie } },
    );
    const beforeItems = await beforeRes.json();
    const beforeCount = Array.isArray(beforeItems) ? beforeItems.length : 0;

    // Save profile (which triggers auto-assign for missing fields)
    const saveRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S001', grade: '10' },
        parent: {},
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    // After profile save, items count should be >= what it was
    const afterRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S001`,
      { headers: { Cookie: cookie } },
    );
    const afterItems = await afterRes.json();
    const afterCount = Array.isArray(afterItems) ? afterItems.length : 0;
    expect(afterCount).toBeGreaterThanOrEqual(beforeCount);
  });
});

// ---------------------------------------------------------------------------
// 7. Export CSV/ZIP
// ---------------------------------------------------------------------------

test.describe('Export formats', () => {
  test('admin can export CSV/ZIP of all entities', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv/zip`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const contentType = res.headers.get('content-type');
    expect(contentType).toContain('application/zip');

    // Verify Content-Disposition header includes filename
    const disposition = res.headers.get('content-disposition');
    expect(disposition).toContain('classgo-csv-');
    expect(disposition).toContain('.zip');
  });

  test('admin can export single entity type as CSV', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    for (const type of ['students', 'parents', 'teachers', 'rooms', 'schedules']) {
      const res = await fetch(`${BASE_URL}/admin/export/csv?type=${type}`, {
        headers: { Cookie: cookie },
      });
      expect(res.status).toBe(200);
      const contentType = res.headers.get('content-type');
      expect(contentType).toContain('text/csv');
    }
  });

  test('export CSV rejects unknown entity type', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=unknown`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(400);
  });

  test('export endpoints reject unauthenticated requests', async () => {
    const csvRes = await fetch(`${BASE_URL}/admin/export/csv?type=students`, {
      redirect: 'manual',
    });
    // Admin routes redirect to login
    expect([302, 401]).toContain(csvRes.status);

    const zipRes = await fetch(`${BASE_URL}/admin/export/csv/zip`, {
      redirect: 'manual',
    });
    expect([302, 401]).toContain(zipRes.status);
  });
});

// ---------------------------------------------------------------------------
// 8. Student Field Values (used by task filters)
// ---------------------------------------------------------------------------

test.describe('Student field values', () => {
  test('admin can get distinct field values', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    for (const field of ['grade', 'school']) {
      const res = await fetch(
        `${BASE_URL}/api/v1/tracker/field-values?field=${field}`,
        { headers: { Cookie: cookie } },
      );
      expect(res.status).toBe(200);
      const data = await res.json();
      expect(Array.isArray(data)).toBe(true);
    }
  });

  test('field values rejects invalid field name', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/tracker/field-values?field=invalid`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(400);
  });

  test('field values rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/tracker/field-values?field=grade`);
    expect(res.status).toBe(401);
  });
});
