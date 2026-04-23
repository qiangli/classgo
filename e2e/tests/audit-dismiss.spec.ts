/**
 * E2E tests for audit flag dismiss and audit listing endpoints.
 *
 * Verifies that admins can dismiss audit flags, and that
 * non-admin users are rejected.
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

test.describe('Audit flag dismiss', () => {
  test('admin can dismiss an audit flag', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // First, create a checkin to potentially generate audit records
    // Then check if there are any flags
    const flagsRes = await fetch(`${BASE_URL}/api/v1/audit/flags`, {
      headers: { Cookie: cookie },
    });
    expect(flagsRes.status).toBe(200);
    const flags = await flagsRes.json();
    expect(Array.isArray(flags)).toBe(true);

    // If there are flags, dismiss the first one
    if (flags.length > 0) {
      const flagId = flags[0].id;
      const dismissRes = await fetch(`${BASE_URL}/api/v1/audit/dismiss`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Cookie: cookie },
        body: JSON.stringify({ id: flagId }),
      });
      expect(dismissRes.status).toBe(200);
      const data = await dismissRes.json();
      expect(data.ok).toBe(true);
    }
  });

  test('dismiss API rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/audit/dismiss`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: 1 }),
    });
    expect(res.status).toBe(401);
  });

  test('dismiss API rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/audit/dismiss`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ id: 1 }),
    });
    expect(res.status).toBe(403);
  });

  test('audit flags list is accessible to admin', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/audit/flags`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('audit devices list is accessible to admin', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/audit/devices`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });
});
