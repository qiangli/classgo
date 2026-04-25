/**
 * E2E tests for the reports feature.
 *
 * Verifies that each user role (student, parent, teacher, admin) can access
 * the report catalog and retrieve report data for their permitted report types.
 * Also tests report subscriptions CRUD and role-based access enforcement.
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

// --- API helper functions ---

async function getCatalog(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/reports/catalog`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getReportData(cookie: string, type: string, from?: string, to?: string) {
  const params = new URLSearchParams({ type });
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await fetch(`${BASE_URL}/api/v1/reports/data?${params}`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getSubscriptions(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/reports/subscriptions`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function createSubscription(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/reports/subscriptions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function updateSubscription(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/reports/subscriptions`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function deleteSubscription(cookie: string, id: number) {
  const res = await fetch(`${BASE_URL}/api/v1/reports/subscriptions?id=${id}`, {
    method: 'DELETE',
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

// ==================== Student Reports ====================

test.describe('Student reports', () => {
  test('student can view report catalog with student reports', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getCatalog(cookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const types = data.map((r: any) => r.type);
    expect(types).toContain('student-weekly-summary');
    expect(types).toContain('student-monthly-progress');
    // Student should not see admin reports
    expect(types).not.toContain('admin-daily-attendance');
  });

  test('student can retrieve weekly summary report', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'student-weekly-summary');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('student can retrieve monthly progress report', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'student-monthly-progress');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('student cannot access teacher reports', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status } = await getReportData(cookie!, 'teacher-weekly-hours');
    expect(status).toBe(403);
  });

  test('student cannot access admin reports', async () => {
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status } = await getReportData(cookie!, 'admin-daily-attendance');
    expect(status).toBe(403);
  });
});

// ==================== Parent Reports ====================

test.describe('Parent reports', () => {
  test('parent can view report catalog with parent reports', async () => {
    const cookie = await userLogin('P001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getCatalog(cookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const types = data.map((r: any) => r.type);
    expect(types).toContain('parent-child-activity');
    // Parent should not see admin or teacher reports
    expect(types).not.toContain('admin-daily-attendance');
    expect(types).not.toContain('teacher-weekly-hours');
  });

  test('parent can retrieve child activity report', async () => {
    const cookie = await userLogin('P001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'parent-child-activity');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('parent cannot access admin reports', async () => {
    const cookie = await userLogin('P001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status } = await getReportData(cookie!, 'admin-daily-attendance');
    expect(status).toBe(403);
  });
});

// ==================== Teacher Reports ====================

test.describe('Teacher reports', () => {
  test('teacher can view report catalog with teacher reports', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getCatalog(cookie!);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const types = data.map((r: any) => r.type);
    expect(types).toContain('teacher-weekly-hours');
    expect(types).toContain('teacher-biweekly-summary');
    expect(types).toContain('teacher-monthly-summary');
    expect(types).toContain('teacher-timesheet');
    // Teacher should not see admin reports
    expect(types).not.toContain('admin-daily-attendance');
  });

  test('teacher can retrieve weekly hours report', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'teacher-weekly-hours');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('teacher can retrieve biweekly summary report', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'teacher-biweekly-summary');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('teacher can retrieve monthly summary report', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'teacher-monthly-summary');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('teacher can retrieve own timesheet', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await getReportData(cookie!, 'teacher-timesheet');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('teacher cannot access admin reports', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status } = await getReportData(cookie!, 'admin-daily-attendance');
    expect(status).toBe(403);
  });
});

// ==================== Admin Reports ====================

test.describe('Admin reports', () => {
  test('admin can view report catalog with all admin reports', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getCatalog(cookie);
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);

    const types = data.map((r: any) => r.type);
    expect(types).toContain('admin-daily-attendance');
    expect(types).toContain('admin-weekly-performance');
    expect(types).toContain('admin-teacher-workload');
    expect(types).toContain('admin-monthly-dashboard');
    expect(types).toContain('admin-engagement-scorecard');
    expect(types).toContain('admin-audit-compliance');
    expect(types).toContain('admin-staff-timesheet');
  });

  test('admin can retrieve daily attendance report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-daily-attendance');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve weekly performance report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-weekly-performance');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve teacher workload report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-teacher-workload');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve monthly dashboard report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-monthly-dashboard');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve engagement scorecard', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-engagement-scorecard');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve audit compliance log', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-audit-compliance');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve staff timesheet report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getReportData(cookie, 'admin-staff-timesheet');
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });

  test('admin can retrieve report with date range', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const today = new Date().toISOString().split('T')[0];
    const weekAgo = new Date(Date.now() - 7 * 86400000).toISOString().split('T')[0];

    const { status, data } = await getReportData(
      cookie, 'admin-daily-attendance', weekAgo, today,
    );
    expect(status).toBe(200);
    expect(data).toBeTruthy();
  });
});

// ==================== Report Data Validation ====================

test.describe('Report data validation', () => {
  test('report data rejects missing type parameter', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/reports/data`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(400);
  });

  test('report data rejects unknown report type', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Unknown type is not in any role's catalog, so role check returns 403
    const { status } = await getReportData(cookie, 'nonexistent-report');
    expect(status).toBe(403);
  });

  test('report catalog includes expected fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await getCatalog(cookie);
    expect(status).toBe(200);

    if (data.length > 0) {
      const report = data[0];
      expect(report).toHaveProperty('type');
      expect(report).toHaveProperty('name');
      expect(report).toHaveProperty('description');
      expect(report).toHaveProperty('roles');
      expect(Array.isArray(report.roles)).toBe(true);
    }
  });
});

// ==================== Report Subscriptions ====================

test.describe('Report subscriptions', () => {
  test('teacher can create, list, update, and delete a subscription', async () => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    // Create subscription
    const { status: createStatus, data: created } = await createSubscription(cookie!, {
      report_type: 'teacher-timesheet',
      frequency: 'weekly',
      day_of_week: 'friday',
      channel: 'email',
    });
    expect(createStatus).toBe(201);
    expect(created.id).toBeTruthy();

    // List subscriptions
    const { status: listStatus, data: subs } = await getSubscriptions(cookie!);
    expect(listStatus).toBe(200);
    expect(Array.isArray(subs)).toBe(true);
    const found = subs.find((s: any) => s.id === created.id);
    expect(found).toBeTruthy();
    expect(found.report_type).toBe('teacher-timesheet');
    expect(found.frequency).toBe('weekly');

    // Update subscription
    const { status: updateStatus } = await updateSubscription(cookie!, {
      id: created.id,
      frequency: 'biweekly',
      day_of_week: 'monday',
      channel: 'email',
      active: true,
    });
    expect(updateStatus).toBe(200);

    // Verify update
    const { data: subsAfter } = await getSubscriptions(cookie!);
    const updated = subsAfter.find((s: any) => s.id === created.id);
    expect(updated.frequency).toBe('biweekly');

    // Delete subscription
    const { status: delStatus } = await deleteSubscription(cookie!, created.id);
    expect(delStatus).toBe(200);

    // Verify deletion
    const { data: subsAfterDel } = await getSubscriptions(cookie!);
    const afterList = subsAfterDel || [];
    const deleted = afterList.find((s: any) => s.id === created.id);
    expect(deleted).toBeUndefined();
  });

  test('parent can subscribe to child activity report', async () => {
    const cookie = await userLogin('P001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { status, data } = await createSubscription(cookie!, {
      report_type: 'parent-child-activity',
      frequency: 'weekly',
      channel: 'email',
    });
    expect(status).toBe(201);
    expect(data.id).toBeTruthy();

    // Cleanup
    await deleteSubscription(cookie!, data.id);
  });

  test('admin can subscribe to staff timesheet report', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { status, data } = await createSubscription(cookie, {
      report_type: 'admin-staff-timesheet',
      frequency: 'weekly',
      channel: 'email',
    });
    expect(status).toBe(201);
    expect(data.id).toBeTruthy();

    // Cleanup
    await deleteSubscription(cookie, data.id);
  });
});

// ==================== Auth Protection ====================

test.describe('Reports auth protection', () => {
  test('unauthenticated user gets 401 for report APIs', async () => {
    const endpoints = [
      '/api/v1/reports/catalog',
      '/api/v1/reports/data?type=admin-daily-attendance',
      '/api/v1/reports/subscriptions',
    ];

    for (const endpoint of endpoints) {
      const res = await fetch(`${BASE_URL}${endpoint}`, { redirect: 'manual' });
      expect([302, 401]).toContain(res.status);
    }
  });

  test('reports page loads for authenticated user', async ({ page }) => {
    const cookie = await userLogin('T01', PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/reports`);
    await expect(page.locator('body')).toBeVisible();
  });
});
