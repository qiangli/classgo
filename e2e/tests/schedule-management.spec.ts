/**
 * E2E tests for the class schedule management feature.
 *
 * Verifies that admin users can view today's sessions, weekly sessions,
 * and schedule conflicts via the schedule API endpoints. Also verifies
 * auth protection and the schedule admin UI navigation.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

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

// --- API helper functions ---

async function getTodaySessions(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/schedule/today`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getWeekSessions(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/schedule/week`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getScheduleConflicts(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/schedule/conflicts`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

// --- Schedule API tests ---

test.describe('Schedule API', () => {
  test('admin can view today sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, data } = await getTodaySessions(cookie);

    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
  });

  test('admin can view weekly sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, data } = await getWeekSessions(cookie);

    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
    // The test server loads csv.example data which has schedules
  });

  test('admin can check schedule conflicts', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, data } = await getScheduleConflicts(cookie);

    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
  });

  test('weekly sessions include schedule metadata', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, data } = await getWeekSessions(cookie);

    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    // If there are sessions, verify they have expected fields
    if (data.length > 0) {
      const session = data[0];
      expect(session).toHaveProperty('day_of_week');
      expect(session).toHaveProperty('start_time');
      expect(session).toHaveProperty('end_time');
      expect(session).toHaveProperty('teacher_id');
      expect(session).toHaveProperty('room_id');
      expect(session).toHaveProperty('subject');
      expect(session).toHaveProperty('student_ids');
    }
  });
});

// --- Auth protection tests ---

test.describe('Schedule auth protection', () => {
  test('unauthenticated user gets 401 for schedule APIs', async () => {
    const endpoints = [
      '/api/v1/schedule/today',
      '/api/v1/schedule/week',
      '/api/v1/schedule/conflicts',
    ];

    for (const endpoint of endpoints) {
      const res = await fetch(`${BASE_URL}${endpoint}`);
      expect(res.status).toBe(401);
    }
  });

  test('non-admin user gets 403 for schedule APIs', async () => {
    const userCookie = await userLogin('S001', 'test1234');
    expect(userCookie).toBeTruthy();

    const endpoints = [
      '/api/v1/schedule/today',
      '/api/v1/schedule/week',
      '/api/v1/schedule/conflicts',
    ];

    for (const endpoint of endpoints) {
      const res = await fetch(`${BASE_URL}${endpoint}`, {
        headers: { Cookie: userCookie! },
      });
      expect(res.status).toBe(403);
    }
  });
});

// --- Schedule admin UI tests ---

test.describe('Schedule admin UI', () => {
  test('admin can navigate to schedule section', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin`);

    const navItem = adminPage.locator('#nav-schedule');
    await expect(navItem).toBeVisible();
    await navItem.click();

    // URL should include #schedule
    expect(adminPage.url()).toContain('#schedule');
  });

  test('schedule page loads via direct URL', async ({ adminPage }) => {
    // Navigate to /admin/schedule (which redirects to /admin#schedule)
    await adminPage.goto(`${BASE_URL}/admin/schedule`);

    await expect(adminPage.locator('body')).toBeVisible();
  });
});
