import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { checkinViaAPI, checkoutViaAPI, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

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

// ==================== Admin Dashboard Page ====================

test.describe('Admin dashboard page', () => {

  test('admin page loads successfully', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin`);
    // Admin page should render without error
    await expect(adminPage.locator('body')).toBeVisible();
  });

  test('non-admin user is redirected from /admin', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin`);
    // Should redirect to admin login
    await page.waitForURL('**/admin/login**');
  });
});

// ==================== Admin Directory ====================

test.describe('Admin directory', () => {

  test('admin can view directory with students, parents, teachers', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/directory`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(data.students).toBeDefined();
    expect(data.parents).toBeDefined();
    expect(data.teachers).toBeDefined();
    expect(Array.isArray(data.students)).toBe(true);
    expect(Array.isArray(data.parents)).toBe(true);
    expect(Array.isArray(data.teachers)).toBe(true);

    // Should have sample data loaded
    expect(data.students.length).toBeGreaterThan(0);
  });

  test('non-admin gets 401 for directory API', async ({ page }) => {
    const res = await fetch(`${BASE_URL}/api/v1/directory`);
    expect(res.status).toBe(401);
  });

  test('admin directory page loads', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin/directory`);
    await expect(adminPage.locator('body')).toBeVisible();
  });
});

// ==================== Admin Attendance ====================

test.describe('Admin attendance', () => {

  test('admin can view today attendees', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/attendees`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can view attendees by date range', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const today = new Date().toISOString().split('T')[0];
    const res = await fetch(`${BASE_URL}/api/attendees?from=${today}&to=${today}`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can view attendance metrics', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/attendees/metrics`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(data).toBeTruthy();
  });

  test('non-admin gets 401 for attendees API', async () => {
    const res = await fetch(`${BASE_URL}/api/attendees`);
    expect(res.status).toBe(401);
  });
});

// ==================== Admin Tracker Library (Global Items) ====================

test.describe('Admin tracker library CRUD', () => {

  test('admin can list global tracker items', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can create a global tracker item', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const itemName = `E2E Test Item ${Date.now()}`;
    const res = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        name: itemName,
        priority: 'high',
        recurrence: 'daily',
        requires_signoff: true,
        active: true,
      }),
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.id).toBeGreaterThan(0);

    // Verify it appears in the list
    const listRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      headers: { Cookie: cookie },
    });
    const items = await listRes.json();
    const found = items.find((i: any) => i.id === data.id);
    expect(found).toBeTruthy();
    expect(found.name).toBe(itemName);

    // Cleanup: delete the item
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: data.id }),
    });
  });

  test('admin can delete a global tracker item', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create first
    const createRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        name: `Delete Me ${Date.now()}`,
        priority: 'low',
        recurrence: 'none',
        active: true,
      }),
    });
    const created = await createRes.json();
    expect(created.ok).toBe(true);

    // Delete
    const delRes = await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: created.id }),
    });
    expect((await delRes.json()).ok).toBe(true);

    // Verify it's gone (not in active list)
    const listRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      headers: { Cookie: cookie },
    });
    const items = await listRes.json();
    const found = items.find((i: any) => i.id === created.id);
    expect(found).toBeUndefined();
  });

  test('non-admin gets 401 for tracker items API', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/tracker/items`);
    expect(res.status).toBe(401);
  });
});

// ==================== Admin Progress Summary ====================

test.describe('Admin progress summary', () => {

  test('admin can view progress summary', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/admin/progress-summary`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can view progress summary with date range', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const today = new Date().toISOString().split('T')[0];
    const weekAgo = new Date(Date.now() - 7 * 86400000).toISOString().split('T')[0];
    const res = await fetch(
      `${BASE_URL}/api/v1/admin/progress-summary?start_date=${weekAgo}&end_date=${today}`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('non-admin gets 401 for progress summary', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/admin/progress-summary`);
    expect(res.status).toBe(401);
  });
});

// ==================== Admin Password Reset ====================

test.describe('Admin password reset', () => {

  test('admin can reset student password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S005',
        password: 'newpass1234',
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
  });

  test('admin password reset rejects short password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S005',
        password: '12',
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
  });

  test('admin password reset rejects invalid student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S999',
        password: 'newpass1234',
      }),
    });
    expect(res.status).toBe(404);
  });

  test('non-admin gets 401 for password reset', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'students', id: 'S001', password: 'test' }),
    });
    expect(res.status).toBe(401);
  });
});

// ==================== Admin Audit ====================

test.describe('Admin audit', () => {

  test('admin can view audit flags', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/audit/flags`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can view audit devices', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/audit/devices`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('non-admin gets 401 for audit APIs', async () => {
    const flags = await fetch(`${BASE_URL}/api/v1/audit/flags`);
    expect(flags.status).toBe(401);

    const devices = await fetch(`${BASE_URL}/api/v1/audit/devices`);
    expect(devices.status).toBe(401);
  });
});

// ==================== Admin Student Profile Management ====================

test.describe('Admin student profile', () => {

  test('admin can view student profile', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/student/profile?id=S001`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.student).toBeTruthy();
    expect(data.student.id).toBe('S001');
  });

  test('admin profile page loads', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin/profile?id=S001`);
    await expect(adminPage.locator('body')).toBeVisible();
  });
});

// ==================== Admin Export ====================

test.describe('Admin export', () => {

  test('admin can export attendance CSV via API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    // Should return CSV content
    const contentType = res.headers.get('content-type');
    expect(contentType).toContain('text/csv');
  });

  test('admin can export XLSX', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/xlsx`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
  });
});

// ==================== Admin Access Control ====================

test.describe('Admin access control', () => {

  test('admin APIs reject unauthenticated requests', async () => {
    const endpoints = [
      '/api/attendees',
      '/api/attendees/metrics',
      '/api/v1/directory',
      '/api/v1/tracker/items',
      '/api/v1/admin/progress-summary',
      '/api/v1/audit/flags',
      '/api/v1/audit/devices',
    ];

    for (const endpoint of endpoints) {
      const res = await fetch(`${BASE_URL}${endpoint}`);
      expect(res.status).toBe(401);
    }
  });
});
