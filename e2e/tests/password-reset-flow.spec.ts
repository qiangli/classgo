import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';

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

// ==================== Password Reset Full Flow ====================

test.describe('Password reset full flow', () => {

  test('admin resets student password and student can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `reset${Date.now()}`;

    // Reset password
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        type: 'students',
        id: 'S005',
        password: newPassword,
      }),
    });
    const resetData = await resetRes.json();
    expect(resetData.ok).toBe(true);

    // Student can login with new password
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S005', password: newPassword, action: 'login' }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
    expect(loginCookie).toContain('classgo_session');
  });

  test('admin resets parent password and parent can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `parentreset${Date.now()}`;

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

    // Parent can login with new password
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'P002', password: newPassword, action: 'login' }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
    expect(loginCookie).toContain('classgo_session');
  });

  test('admin resets teacher password and teacher can login', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `teacherreset${Date.now()}`;

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

    // Teacher can login with new password
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'T02', password: newPassword, action: 'login' }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
    expect(loginCookie).toContain('classgo_session');
  });

  test('old password no longer works after reset', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPassword = `changed${Date.now()}`;

    // First setup a known password
    await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ type: 'students', id: 'S005', password: 'knownpass123' }),
    });

    // Reset to new password
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ type: 'students', id: 'S005', password: newPassword }),
    });
    expect((await resetRes.json()).ok).toBe(true);

    // Old password should fail
    const oldLoginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S005', password: 'knownpass123', action: 'login' }),
      redirect: 'manual',
    });
    // Should not get a session cookie (login fails)
    const oldCookie = oldLoginRes.headers.get('set-cookie');
    // Either no cookie or the login returned an error
    if (oldCookie) {
      expect(oldCookie).not.toContain('classgo_session');
    }

    // New password works
    const newLoginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S005', password: newPassword, action: 'login' }),
      redirect: 'manual',
    });
    const newCookie = newLoginRes.headers.get('set-cookie');
    expect(newCookie).toBeTruthy();
    expect(newCookie).toContain('classgo_session');
  });
});
