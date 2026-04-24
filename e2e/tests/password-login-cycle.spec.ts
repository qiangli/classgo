/**
 * E2E tests for password reset → login cycle.
 *
 * Verifies the full cycle: admin resets a password, then the user
 * can log in with the new password and access protected endpoints.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';

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

test.describe('Password reset and login cycle', () => {

  test('admin resets student password and student can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `reset${Date.now() % 10000}`;

    // Admin resets password for S003 (Carlos Garcia)
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S003',
        password: newPassword,
      }),
    });
    const resetData = await resetRes.json();
    expect(resetData.ok).toBe(true);
    expect(resetData.message).toContain('Carlos');

    // Student can login with new password
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        entity_id: 'S003',
        password: newPassword,
        action: 'login',
      }),
      redirect: 'manual',
    });
    const loginData = await loginRes.json().catch(() => ({}));
    const loginCookie = loginRes.headers.get('set-cookie');
    // Either JSON response with ok=true or a redirect with cookie
    if (loginData.ok !== undefined) {
      expect(loginData.ok).toBe(true);
    }
    expect(loginCookie).toBeTruthy();

    // User can access authenticated endpoint
    const match = loginCookie!.match(/classgo_session=([^;]+)/);
    expect(match).toBeTruthy();
    const sessionCookie = `classgo_session=${match![1]}`;

    const profileRes = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      headers: { Cookie: sessionCookie },
    });
    expect(profileRes.status).toBe(200);
    const profile = await profileRes.json();
    expect(profile.ok).toBe(true);
    expect(profile.student.id).toBe('S003');
  });

  test('admin resets teacher password and teacher can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `treset${Date.now() % 10000}`;

    // Reset T02 (James Johnson) password
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'teachers',
        id: 'T02',
        password: newPassword,
      }),
    });
    const resetData = await resetRes.json();
    expect(resetData.ok).toBe(true);

    // Teacher can login
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        entity_id: 'T02',
        password: newPassword,
        action: 'login',
      }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
  });

  test('admin resets parent password and parent can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `preset${Date.now() % 10000}`;

    // Reset P002 (Maria Garcia) password
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'parents',
        id: 'P002',
        password: newPassword,
      }),
    });
    const resetData = await resetRes.json();
    expect(resetData.ok).toBe(true);

    // Parent can login
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        entity_id: 'P002',
        password: newPassword,
        action: 'login',
      }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
  });

  test('password reset rejects short password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S003',
        password: '12',
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
  });

  test('password reset rejects non-admin', async () => {
    // Login as student
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'login' }),
      redirect: 'manual',
    });
    const sc = loginRes.headers.get('set-cookie');
    const match = sc?.match(/classgo_session=([^;]+)/);
    const studentCookie = match ? `classgo_session=${match[1]}` : '';

    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S002',
        password: 'newpass123',
      }),
    });
    expect(res.status).toBe(403);
  });

  test('password reset rejects unauthenticated', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'students',
        id: 'S001',
        password: 'newpass123',
      }),
    });
    expect(res.status).toBe(401);
  });
});
