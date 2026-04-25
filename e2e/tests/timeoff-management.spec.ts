/**
 * E2E tests for the time-off management feature.
 *
 * Verifies that admins can create, list, update, and delete time-off records
 * for staff, and that authenticated users can view their own time-off via
 * the dashboard endpoint. Also tests access control enforcement.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PASSWORD = 'test1234';
const TEACHER_ID = 'T01'; // Sarah Smith

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

async function listTimeOff(cookie: string, params?: Record<string, string>) {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  const res = await fetch(`${BASE_URL}/api/v1/timeoff${qs}`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function saveTimeOff(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/timeoff/save`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function deleteTimeOff(cookie: string, id: number) {
  const res = await fetch(`${BASE_URL}/api/v1/timeoff/delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ id }),
  });
  return { status: res.status, data: await res.json() };
}

async function getMyTimeOff(cookie: string, params?: Record<string, string>) {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  const res = await fetch(`${BASE_URL}/api/dashboard/my-timeoff${qs}`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

// ==================== Admin Time-Off CRUD ====================

test.describe('Admin time-off management', () => {
  test('admin can create a time-off record for a teacher', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    const { status, data } = await saveTimeOff(cookie, {
      id: 0,
      user_id: TEACHER_ID,
      user_type: 'teacher',
      date: futureDate,
      type: 'personal',
      schedule_type: '',
      hours: 0,
      notes: 'E2E test time-off',
    });
    expect(status).toBe(200);
    expect(data.ok).toBe(true);
    expect(data.id).toBeTruthy();

    // Cleanup
    await deleteTimeOff(cookie, data.id);
  });

  test('admin can list time-off records', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    // Create a record first
    const { data: created } = await saveTimeOff(cookie, {
      id: 0,
      user_id: TEACHER_ID,
      user_type: 'teacher',
      date: futureDate,
      type: 'sick',
      hours: 4,
      notes: 'Half day sick',
    });

    // List all
    const { status, data } = await listTimeOff(cookie);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const found = data.find((r: any) => r.id === created.id);
    expect(found).toBeTruthy();
    expect(found.user_id).toBe(TEACHER_ID);
    expect(found.type).toBe('sick');
    expect(found.hours).toBe(4);

    // Cleanup
    await deleteTimeOff(cookie, created.id);
  });

  test('admin can filter time-off by user', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    // Create records for two different teachers
    const { data: rec1 } = await saveTimeOff(cookie, {
      id: 0, user_id: 'T01', user_type: 'teacher',
      date: futureDate, type: 'personal', hours: 0, notes: '',
    });
    const { data: rec2 } = await saveTimeOff(cookie, {
      id: 0, user_id: 'T02', user_type: 'teacher',
      date: futureDate, type: 'sick', hours: 0, notes: '',
    });

    // Filter by T01
    const { status, data } = await listTimeOff(cookie, { user_id: 'T01' });
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const ids = data.map((r: any) => r.user_id);
    expect(ids).toContain('T01');
    // T02 should not appear
    const hasT02 = data.some((r: any) => r.id === rec2.id);
    expect(hasT02).toBe(false);

    // Cleanup
    await deleteTimeOff(cookie, rec1.id);
    await deleteTimeOff(cookie, rec2.id);
  });

  test('admin can update an existing time-off record', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    // Create
    const { data: created } = await saveTimeOff(cookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher',
      date: futureDate, type: 'personal', hours: 0, notes: 'Original',
    });

    // Update (same endpoint, non-zero id)
    const { status, data } = await saveTimeOff(cookie, {
      id: created.id, user_id: TEACHER_ID, user_type: 'teacher',
      date: futureDate, type: 'sick', hours: 4, notes: 'Updated to sick',
    });
    expect(status).toBe(200);
    expect(data.ok).toBe(true);

    // Verify update
    const { data: list } = await listTimeOff(cookie, { user_id: TEACHER_ID });
    const updated = list.find((r: any) => r.id === created.id);
    expect(updated).toBeTruthy();
    expect(updated.type).toBe('sick');
    expect(updated.hours).toBe(4);
    expect(updated.notes).toBe('Updated to sick');

    // Cleanup
    await deleteTimeOff(cookie, created.id);
  });

  test('admin can delete a time-off record', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    // Create
    const { data: created } = await saveTimeOff(cookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher',
      date: futureDate, type: 'holiday', hours: 0, notes: '',
    });

    // Delete
    const { status, data } = await deleteTimeOff(cookie, created.id);
    expect(status).toBe(200);
    expect(data.ok).toBe(true);

    // Verify gone
    const { data: list } = await listTimeOff(cookie, { user_id: TEACHER_ID });
    const found = list.find((r: any) => r.id === created.id);
    expect(found).toBeUndefined();
  });
});

// ==================== Validation ====================

test.describe('Time-off validation', () => {
  test('save rejects missing required fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Missing user_id
    const res1 = await saveTimeOff(cookie, {
      id: 0, user_type: 'teacher', date: '2026-05-01', type: 'sick',
    });
    expect(res1.status).toBe(400);

    // Missing date
    const res2 = await saveTimeOff(cookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher', type: 'sick',
    });
    expect(res2.status).toBe(400);

    // Missing type
    const res3 = await saveTimeOff(cookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher', date: '2026-05-01',
    });
    expect(res3.status).toBe(400);
  });

  test('save rejects invalid type value', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await saveTimeOff(cookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher',
      date: '2026-05-01', type: 'vacation',
    });
    expect(status).toBe(400);
    expect(data.error).toContain('type must be');
  });
});

// ==================== User Own Time-Off ====================

test.describe('User own time-off view', () => {
  test('teacher can view own time-off via dashboard endpoint', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    const futureDate = new Date(Date.now() + 7 * 86400000).toISOString().split('T')[0];

    // Admin creates time-off for teacher
    const { data: created } = await saveTimeOff(adminCookie, {
      id: 0, user_id: TEACHER_ID, user_type: 'teacher',
      date: futureDate, type: 'personal', hours: 0, notes: 'Teacher day off',
    });

    // Teacher views own time-off
    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const { status, data } = await getMyTimeOff(teacherCookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const found = data.find((r: any) => r.id === created.id);
    expect(found).toBeTruthy();
    expect(found.type).toBe('personal');

    // Cleanup
    await deleteTimeOff(adminCookie, created.id);
  });

  test('student can view own time-off (empty)', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getMyTimeOff(cookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
  });

  test('parent can view own time-off (empty)', async () => {
    const cookie = await userLogin('P001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getMyTimeOff(cookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
  });
});

// ==================== Auth Protection ====================

test.describe('Time-off auth protection', () => {
  test('unauthenticated user gets 401 for admin time-off APIs', async () => {
    const endpoints = [
      '/api/v1/timeoff',
      '/api/v1/timeoff/save',
      '/api/v1/timeoff/delete',
    ];

    for (const endpoint of endpoints) {
      const res = await fetch(`${BASE_URL}${endpoint}`, { redirect: 'manual' });
      expect(res.status).toBe(401);
    }
  });

  test('non-admin user gets 403 for admin time-off APIs', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/timeoff`, {
      headers: { Cookie: cookie! },
    });
    expect(res.status).toBe(403);
  });

  test('unauthenticated user cannot access my-timeoff', async () => {
    const res = await fetch(`${BASE_URL}/api/dashboard/my-timeoff`, {
      redirect: 'manual',
    });
    expect([302, 401]).toContain(res.status);
  });
});
