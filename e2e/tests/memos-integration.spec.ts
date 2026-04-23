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

// ==================== Memos Sync API ====================

test.describe('Memos sync API', () => {

  test('admin can trigger full memos sync', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/v1/memos/sync`, {
      headers: { Cookie: cookie },
    });
    // May return 200 with ok:true if Memos is configured, or 503 if not
    expect([200, 503]).toContain(res.status);
    const data = await res.json();
    if (res.status === 200) {
      expect(data.ok).toBe(true);
    }
  });

  test('admin can trigger attendance sync', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/v1/memos/sync`, {
      method: 'POST',
      headers: { Cookie: cookie },
    });
    expect([200, 503]).toContain(res.status);
  });

  test('memos sync rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/memos/sync`);
    expect(res.status).toBe(401);
  });

  test('memos sync rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();
    const res = await fetch(`${BASE_URL}/api/v1/memos/sync`, {
      headers: { Cookie: studentCookie! },
    });
    expect(res.status).toBe(403);
  });
});

// ==================== Memos Proxy Access ====================

test.describe('Memos proxy access', () => {

  test('authenticated user can access memos page', async ({ adminPage }) => {
    // Navigate to /memos/ — should not get a login redirect
    const response = await adminPage.goto(`${BASE_URL}/memos/`);
    // Should load successfully (200) or be proxied to Memos SPA
    expect(response).toBeTruthy();
    expect(response!.status()).toBeLessThan(400);
  });

  test('unauthenticated user cannot access memos', async ({ page }) => {
    const response = await page.goto(`${BASE_URL}/memos/`);
    // Should redirect to login (302) or the page URL should contain login
    const url = page.url();
    // Either redirected to login or got 401
    expect(url.includes('login') || url.includes('entry') || response?.status() === 401 || response?.status() === 302).toBeTruthy();
  });
});
