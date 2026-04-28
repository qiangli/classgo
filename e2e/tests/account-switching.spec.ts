import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, adminLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S001'; // Alice Wang
const TEACHER_ID = 'T01';  // Sarah Smith
const PASSWORD = 'test1234';

function getSessionCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

// ==================== Account List API ====================

test.describe('Account list API', () => {

  test('new session has exactly one identity', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.identities).toHaveLength(1);
    expect(data.active_index).toBe(0);
    expect(data.identities[0].role).toBe('user');
    expect(data.identities[0].user_type).toBe('student');
  });

  test('unauthenticated request returns 401', async () => {
    const res = await fetch(`${BASE_URL}/api/account/list`);
    expect(res.status).toBe(401);
  });
});

// ==================== Add Account ====================

test.describe('Add account', () => {

  test('can add a guest identity to existing session', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'guest' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.redirect).toBe('/kiosk');

    // Verify session now has two identities
    const listRes = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    const listData = await listRes.json();
    expect(listData.identities).toHaveLength(2);
    // Active should be guest (just added)
    expect(listData.identities[listData.active_index].role).toBe('guest');
  });

  test('can add a user identity to existing session', async ({ adminPage }) => {
    const adminCookie = await getSessionCookie(adminPage);
    // Ensure teacher account exists
    await userLogin(TEACHER_ID, PASSWORD);

    // Add teacher to admin session
    const res = await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ type: 'user', entity_id: TEACHER_ID, password: PASSWORD }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.redirect).toBe('/home');

    // Verify both identities exist
    const listRes = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: adminCookie },
    });
    const listData = await listRes.json();
    expect(listData.identities.length).toBeGreaterThanOrEqual(2);
    const roles = listData.identities.map((id: any) => id.role);
    expect(roles).toContain('admin');
    expect(roles).toContain('user');
  });

  test('adding duplicate identity replaces instead of duplicating', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Add guest
    await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'guest' }),
    });
    // Add guest again
    await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'guest' }),
    });

    const listRes = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    const listData = await listRes.json();
    // Should have 2 (user + guest), not 3
    const guestCount = listData.identities.filter((id: any) => id.role === 'guest').length;
    expect(guestCount).toBe(1);
  });

  test('add user rejects wrong password', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await userLogin(TEACHER_ID, PASSWORD); // ensure account exists

    const res = await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'user', entity_id: TEACHER_ID, password: 'wrongpass' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
    expect(data.error).toContain('Invalid credentials');
  });

  test('add account rejects invalid type', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'invalid' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
  });
});

// ==================== Switch Account ====================

test.describe('Switch account', () => {

  test('can switch between identities', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Add guest identity
    await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'guest' }),
    });

    // Switch back to student (index 0)
    const res = await fetch(`${BASE_URL}/api/account/switch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 0 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.active.role).toBe('user');
    expect(data.redirect).toBe('/home');
  });

  test('switch to admin identity returns admin redirect', async ({ adminPage }) => {
    const adminCookie = await getSessionCookie(adminPage);
    await userLogin(STUDENT_ID, PASSWORD);

    // Add student to admin session
    await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ type: 'user', entity_id: STUDENT_ID, password: PASSWORD }),
    });

    // Switch back to admin (index 0)
    const res = await fetch(`${BASE_URL}/api/account/switch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ index: 0 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.active.role).toBe('admin');
    expect(data.redirect).toBe('/admin');
  });

  test('switch rejects out-of-range index', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/switch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 99 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
    expect(data.error).toContain('out of range');
  });
});

// ==================== Remove Account ====================

test.describe('Remove account', () => {

  test('can remove a non-active identity', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Add guest (now at index 1, and becomes active)
    await fetch(`${BASE_URL}/api/account/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ type: 'guest' }),
    });

    // Switch back to student
    await fetch(`${BASE_URL}/api/account/switch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 0 }),
    });

    // Remove guest (index 1)
    const res = await fetch(`${BASE_URL}/api/account/remove`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 1 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.identities).toHaveLength(1);
    expect(data.identities[0].role).toBe('user');
  });

  test('removing last identity clears session', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/remove`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 0 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.redirect).toBe('/login');

    // Session should now be invalid
    const listRes = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    expect(listRes.status).toBe(401);
  });

  test('remove rejects out-of-range index', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/account/remove`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ index: 5 }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
  });
});

// ==================== Login Preserves Existing Session ====================

test.describe('Login preserves existing session', () => {

  test('admin login with existing user session adds identity', async ({ adminPage }) => {
    // Start with a user session
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Verify single identity
    const list1 = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    const data1 = await list1.json();
    expect(data1.identities).toHaveLength(1);

    // Get admin credentials from the admin page context
    const adminUser = process.env.CLASSGO_TEST_ADMIN_USER!;
    const adminPass = process.env.CLASSGO_TEST_ADMIN_PASS!;

    const adminRes = await fetch(`${BASE_URL}/admin/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie! },
      body: JSON.stringify({ username: adminUser, password: adminPass }),
      redirect: 'manual',
    });
    const adminData = await adminRes.json();
    expect(adminData.ok).toBe(true);

    // The cookie should be the same (identity added, not replaced)
    // Check that we now have two identities
    const list2 = await fetch(`${BASE_URL}/api/account/list`, {
      headers: { Cookie: cookie! },
    });
    const data2 = await list2.json();
    expect(data2.identities.length).toBeGreaterThanOrEqual(2);
    const roles = data2.identities.map((id: any) => id.role);
    expect(roles).toContain('user');
    expect(roles).toContain('admin');
  });
});

// ==================== Account Switcher UI ====================

test.describe('Account switcher UI', () => {

  test('account switcher dropdown is visible on home page', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    const switcher = page.locator('#account-switcher');
    await expect(switcher).toBeVisible();
  });

  test('account switcher dropdown is visible on admin page', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin`);
    const switcher = adminPage.locator('#account-switcher');
    await expect(switcher).toBeVisible();
  });

  test('clicking switcher shows dropdown with identities', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    // Click the switcher button
    await page.locator('#account-switcher button').first().click();
    // Dropdown should appear
    const dropdown = page.locator('#account-dropdown');
    await expect(dropdown).toBeVisible();
    // Should show "Add account" option
    await expect(dropdown.locator('text=Add account')).toBeVisible();
    // Should show "Sign out all" option
    await expect(dropdown.locator('text=Sign out all')).toBeVisible();
  });
});
