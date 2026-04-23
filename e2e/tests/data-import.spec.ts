/**
 * E2E tests for the data import endpoint.
 *
 * Verifies that POST /api/v1/import triggers a reimport from the
 * spreadsheet source of truth, with proper admin-only access control.
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

test.describe('Data import', () => {

  test('admin can trigger data reimport', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.message).toContain('Imported');
  });

  test('reimport preserves entity counts', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Get directory before import
    const beforeRes = await fetch(`${BASE_URL}/api/v1/directory`, {
      headers: { Cookie: cookie },
    });
    const before = await beforeRes.json();
    const studentCountBefore = before.students.length;
    const parentCountBefore = before.parents.length;
    const teacherCountBefore = before.teachers.length;

    // Trigger reimport
    const importRes = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { Cookie: cookie },
    });
    expect(importRes.status).toBe(200);

    // Get directory after import
    const afterRes = await fetch(`${BASE_URL}/api/v1/directory`, {
      headers: { Cookie: cookie },
    });
    const after = await afterRes.json();

    // Counts should be at least what they were before (import doesn't delete)
    expect(after.students.length).toBeGreaterThanOrEqual(studentCountBefore);
    expect(after.parents.length).toBeGreaterThanOrEqual(parentCountBefore);
    expect(after.teachers.length).toBeGreaterThanOrEqual(teacherCountBefore);
  });

  test('import API rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
    });
    expect(res.status).toBe(401);
  });

  test('import API rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { Cookie: studentCookie! },
    });
    expect(res.status).toBe(403);
  });
});
