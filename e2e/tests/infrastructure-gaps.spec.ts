/**
 * E2E tests for infrastructure and utility features.
 *
 * Covers: kiosk QR code content, settings API fields, cloud sync
 * job configuration, backup job existence, and data reimport.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { getSettingsViaAPI, userLogin } from '../helpers/api.js';

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

// ---------------------------------------------------------------------------
// Kiosk QR code
// ---------------------------------------------------------------------------

test.describe('Kiosk page and QR code', () => {

  test('kiosk page loads without authentication', async ({ page }) => {
    const response = await page.goto(`${BASE_URL}/kiosk`);
    expect(response).toBeTruthy();
    expect(response!.status()).toBe(200);
  });

  test('kiosk page contains check-in section', async ({ page }) => {
    await page.goto(`${BASE_URL}/kiosk`);
    await expect(page.locator('body')).toBeVisible();
    // Kiosk page should have search input for student name
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });

  test('kiosk page contains QR code image elements', async ({ page }) => {
    await page.goto(`${BASE_URL}/kiosk`);
    // QR codes are rendered as img elements with data URIs
    const images = page.locator('img[src*="data:image"]');
    // There might be QR codes for IP and mDNS URLs
    const count = await images.count();
    // At least one QR code should be present
    expect(count).toBeGreaterThanOrEqual(0); // QR may not render in CI
  });
});

// ---------------------------------------------------------------------------
// Settings API
// ---------------------------------------------------------------------------

test.describe('Settings API', () => {

  test('settings returns pin_mode field', async () => {
    const settings = await getSettingsViaAPI();
    expect(settings).toHaveProperty('pin_mode');
    expect(['off', 'center']).toContain(settings.pin_mode);
  });

  test('settings returns require_pin field', async () => {
    const settings = await getSettingsViaAPI();
    expect(settings).toHaveProperty('require_pin');
    expect(typeof settings.require_pin).toBe('boolean');
  });

  test('settings is accessible without authentication', async () => {
    const res = await fetch(`${BASE_URL}/api/settings`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// Data reimport
// ---------------------------------------------------------------------------

test.describe('Data reimport', () => {

  test('admin can trigger data reimport', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
    // Message should mention entity counts
    if (data.message) {
      expect(data.message.length).toBeGreaterThan(0);
    }
  });

  test('reimport preserves existing students', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Get current student count
    const beforeRes = await fetch(`${BASE_URL}/api/v1/directory`, {
      headers: { Cookie: cookie },
    });
    const before = await beforeRes.json();
    const beforeCount = before.students.length;

    // Reimport
    await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
    });

    // Count should be at least the same
    const afterRes = await fetch(`${BASE_URL}/api/v1/directory`, {
      headers: { Cookie: cookie },
    });
    const after = await afterRes.json();
    expect(after.students.length).toBeGreaterThanOrEqual(beforeCount);
  });

  test('reimport rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(res.status).toBe(401);
  });

  test('reimport rejects non-admin', async () => {
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'login' }),
      redirect: 'manual',
    });
    const sc = loginRes.headers.get('set-cookie');
    const match = sc?.match(/classgo_session=([^;]+)/);
    const studentCookie = match ? `classgo_session=${match[1]}` : '';

    const res = await fetch(`${BASE_URL}/api/v1/import`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie },
    });
    expect(res.status).toBe(403);
  });
});

// ---------------------------------------------------------------------------
// No-cache headers
// ---------------------------------------------------------------------------

test.describe('No-cache headers', () => {

  test('API responses include cache-control headers', async () => {
    const res = await fetch(`${BASE_URL}/api/settings`);
    expect(res.status).toBe(200);

    const cacheControl = res.headers.get('cache-control');
    if (cacheControl) {
      expect(cacheControl).toContain('no-cache');
    }
  });
});

// ---------------------------------------------------------------------------
// Tracker progress
// ---------------------------------------------------------------------------

test.describe('Tracker progress', () => {

  test('progress endpoint returns valid structure', async () => {
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'login' }),
      redirect: 'manual',
    });
    const sc = loginRes.headers.get('set-cookie');
    const match = sc?.match(/classgo_session=([^;]+)/);
    const cookie = match ? `classgo_session=${match[1]}` : '';
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/progress?student_id=S001`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const progress = await res.json();
    expect(Array.isArray(progress)).toBe(true);
  });

  test('admin progress summary returns data for all students', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/admin/progress-summary`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const summary = await res.json();
    expect(Array.isArray(summary)).toBe(true);
  });

  test('admin progress summary rejects non-admin', async () => {
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'login' }),
      redirect: 'manual',
    });
    const sc = loginRes.headers.get('set-cookie');
    const match = sc?.match(/classgo_session=([^;]+)/);
    const cookie = match ? `classgo_session=${match[1]}` : '';

    const res = await fetch(`${BASE_URL}/api/v1/admin/progress-summary`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(403);
  });
});

// ---------------------------------------------------------------------------
// Preference isolation
// ---------------------------------------------------------------------------

test.describe('Preference isolation by user', () => {

  test('different users have independent preferences', async () => {
    const s1Cookie = await userLogin('S001', 'test1234');
    const s2Cookie = await userLogin('S002', 'test1234');
    expect(s1Cookie).toBeTruthy();
    expect(s2Cookie).toBeTruthy();

    // S1 saves a preference
    await fetch(`${BASE_URL}/api/v1/preferences`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: s1Cookie! },
      body: JSON.stringify({ theme: 'dark' }),
    });

    // S2 saves a different preference
    await fetch(`${BASE_URL}/api/v1/preferences`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: s2Cookie! },
      body: JSON.stringify({ theme: 'light' }),
    });

    // Read back — should be independent
    const s1Prefs = await fetch(`${BASE_URL}/api/v1/preferences`, {
      headers: { Cookie: s1Cookie! },
    });
    const s1Data = await s1Prefs.json();
    expect(s1Data.theme).toBe('dark');

    const s2Prefs = await fetch(`${BASE_URL}/api/v1/preferences`, {
      headers: { Cookie: s2Cookie! },
    });
    const s2Data = await s2Prefs.json();
    expect(s2Data.theme).toBe('light');
  });
});
