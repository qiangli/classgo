/**
 * E2E tests for superadmin-only tracker item deletion.
 *
 * Verifies that the global tracker item delete endpoint is protected
 * behind superadmin access, and that admin users (who created the item)
 * can delete them via the superadmin-gated endpoint.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

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

// ==================== SuperAdmin Tracker Item Deletion ====================

test.describe('Global tracker item deletion', () => {
  test('admin can create and delete a global tracker item', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create a global tracker item
    const createRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        name: `Delete Test ${Date.now()}`,
        priority: 'low',
        recurrence: 'daily',
        requires_signoff: false,
        active: true,
      }),
    });
    const created = await createRes.json();
    expect(created.ok).toBe(true);
    expect(created.id).toBeTruthy();

    // Delete it via the superadmin endpoint
    const delRes = await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: created.id }),
    });
    const delData = await delRes.json();
    expect(delRes.status).toBe(200);
    expect(delData.ok).toBe(true);

    // Verify item is gone
    const listRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      headers: { Cookie: cookie },
    });
    const items = await listRes.json();
    const found = items.find((i: any) => i.id === created.id);
    expect(found).toBeUndefined();
  });

  test('teacher cannot delete global tracker items', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Create an item as admin
    const createRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        name: `Teacher Delete Attempt ${Date.now()}`,
        priority: 'medium',
        recurrence: 'daily',
        requires_signoff: false,
        active: true,
      }),
    });
    const created = await createRes.json();
    expect(created.ok).toBe(true);

    // Teacher tries to delete
    const teacherCookie = await userLogin('T01', PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({ id: created.id }),
    });
    expect(res.status).toBe(403);

    // Cleanup
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ id: created.id }),
    });
  });

  test('student cannot delete global tracker items', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Create an item as admin
    const createRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        name: `Student Delete Attempt ${Date.now()}`,
        priority: 'medium',
        recurrence: 'daily',
        requires_signoff: false,
        active: true,
      }),
    });
    const created = await createRes.json();
    expect(created.ok).toBe(true);

    // Student tries to delete
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ id: created.id }),
    });
    expect(res.status).toBe(403);

    // Cleanup
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ id: created.id }),
    });
  });

  test('unauthenticated user cannot delete global tracker items', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: 1 }),
    });
    expect(res.status).toBe(401);
  });
});
