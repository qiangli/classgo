/**
 * E2E tests for the Classes & Enrollment feature.
 * Tests the new /api/dashboard/classes, enroll, unenroll, schedule, and rooms endpoints.
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

async function dataCRUD(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/data`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function dashboardFetch(cookie: string, path: string, body?: Record<string, any>) {
  const opts: any = { headers: { Cookie: cookie } };
  if (body) {
    opts.method = 'POST';
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${BASE_URL}${path}`, opts);
  return { status: res.status, data: await res.json() };
}

// ==================== Classes Listing ====================

test.describe('Classes API', () => {

  test('admin can list classes via /api/dashboard/classes', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    // Admin has teacher-like behavior but let's use the endpoint
    const { status, data } = await dashboardFetch(cookie, '/api/dashboard/classes');
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
  });

  test('student can list all classes with enrollment status', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-CLS-${stamp}`;

    // Create a schedule via admin
    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Monday', start_time: '10:00', end_time: '11:00',
        subject: 'enrollment-test', student_ids: 'S001' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');
      expect(studentCookie).toBeTruthy();

      const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/classes?student_id=S001');
      expect(status).toBe(200);
      expect(Array.isArray(data)).toBe(true);

      const found = data.find((c: any) => c.id === schedId);
      expect(found).toBeTruthy();
      expect(found.is_enrolled).toBe(true);
      expect(found.enrolled_count).toBe(1);
      expect(found.subject).toBe('enrollment-test');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('classes response includes teacher_name and room_name', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-CLSN-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Tuesday', start_time: '14:00', end_time: '15:00',
        subject: 'name-resolve', teacher_id: 'T01', room_id: 'R01' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');
      const { data } = await dashboardFetch(studentCookie!, '/api/dashboard/classes?student_id=S001');
      const found = data.find((c: any) => c.id === schedId);
      expect(found).toBeTruthy();
      // teacher_name and room_name should be resolved (non-empty if T01/R01 exist in sample data)
      expect(found.teacher_name).toBeDefined();
      expect(found.room_name).toBeDefined();
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });
});

// ==================== Enrollment ====================

test.describe('Enrollment', () => {

  test('student can enroll in a class', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-ENR-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Monday', start_time: '10:00', end_time: '11:00',
        subject: 'enroll-test', student_ids: '' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');
      expect(studentCookie).toBeTruthy();

      // Enroll
      const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/enroll',
        { schedule_id: schedId, student_id: 'S001' });
      expect(status).toBe(200);
      expect(data.ok).toBe(true);

      // Verify enrollment
      const { data: classes } = await dashboardFetch(studentCookie!, '/api/dashboard/classes?student_id=S001');
      const found = classes.find((c: any) => c.id === schedId);
      expect(found).toBeTruthy();
      expect(found.is_enrolled).toBe(true);
      expect(found.enrolled_count).toBe(1);
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('student can unenroll from a class', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-UNE-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Tuesday', start_time: '09:00', end_time: '10:00',
        subject: 'unenroll-test', student_ids: 'S001' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');

      // Unenroll
      const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/unenroll',
        { schedule_id: schedId, student_id: 'S001' });
      expect(status).toBe(200);
      expect(data.ok).toBe(true);

      // Verify
      const { data: classes } = await dashboardFetch(studentCookie!, '/api/dashboard/classes?student_id=S001');
      const found = classes.find((c: any) => c.id === schedId);
      expect(found).toBeTruthy();
      expect(found.is_enrolled).toBe(false);
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('duplicate enrollment returns conflict error', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-DUP-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Wednesday', start_time: '10:00', end_time: '11:00',
        subject: 'dup-test', student_ids: 'S001' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');

      const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/enroll',
        { schedule_id: schedId, student_id: 'S001' });
      expect(status).toBe(409);
      expect(data.error).toContain('Already enrolled');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('enrollment respects room capacity', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const roomId = `E2E-RM-${stamp}`;
    const schedId = `E2E-CAP-${stamp}`;

    // Create room with capacity 1
    await dataCRUD(cookie, {
      action: 'save', type: 'rooms',
      data: { id: roomId, name: 'Tiny Room', capacity: '1' },
    });

    // Create schedule in that room with S001 already enrolled
    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Thursday', start_time: '10:00', end_time: '11:00',
        subject: 'cap-test', room_id: roomId, student_ids: 'S001' },
    });

    try {
      const student2Cookie = await userLogin('S002', 'test1234');
      expect(student2Cookie).toBeTruthy();

      // Should fail — class is full
      const { status, data } = await dashboardFetch(student2Cookie!, '/api/dashboard/enroll',
        { schedule_id: schedId, student_id: 'S002' });
      expect(status).toBe(409);
      expect(data.error).toContain('full');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
      await dataCRUD(cookie, { action: 'delete', type: 'rooms', id: roomId });
    }
  });

  test('student cannot enroll as another student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-AUTH-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Friday', start_time: '10:00', end_time: '11:00',
        subject: 'auth-test' },
    });

    try {
      const studentCookie = await userLogin('S001', 'test1234');

      // S001 trying to enroll S002 — should be forbidden
      const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/enroll',
        { schedule_id: schedId, student_id: 'S002' });
      expect(status).toBe(403);
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('unauthenticated user cannot enroll', async () => {
    const res = await fetch(`${BASE_URL}/api/dashboard/enroll`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ schedule_id: 'SCH001', student_id: 'S001' }),
      redirect: 'manual',
    });
    // RequireAuth redirects to login page (302)
    expect(res.status).toBe(302);
  });
});

// ==================== Rooms ====================

test.describe('Rooms API', () => {

  test('authenticated user can list rooms', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, data } = await dashboardFetch(cookie, '/api/dashboard/rooms');
    expect(status).toBe(200);
    expect(Array.isArray(data)).toBe(true);
    // Should have at least one room from sample data
    if (data.length > 0) {
      expect(data[0]).toHaveProperty('id');
      expect(data[0]).toHaveProperty('name');
      expect(data[0]).toHaveProperty('capacity');
    }
  });
});

// ==================== Teacher Schedule Management ====================

test.describe('Teacher schedule management', () => {

  test('teacher can create a new schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    // First create a teacher schedule via admin, then test teacher API
    // We need a teacher login. T01 should exist in sample data.
    const teacherCookie = await userLogin('T01', 'test1234');
    if (!teacherCookie) { test.skip(true, 'Teacher login not available'); return; }

    const { status, data } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule', {
      day_of_week: 'Monday', start_time: '16:00', end_time: '17:00',
      subject: 'teacher-create-test', room_id: 'R01',
    });
    expect(status).toBe(200);
    expect(data.ok).toBe(true);
    expect(data.id).toBeTruthy();

    // Verify in classes list
    const { data: classes } = await dashboardFetch(teacherCookie, '/api/dashboard/classes');
    const found = classes.find((c: any) => c.id === data.id);
    expect(found).toBeTruthy();
    expect(found.subject).toBe('teacher-create-test');

    // Cleanup via admin
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: data.id });
  });

  test('teacher can update their own schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const teacherCookie = await userLogin('T01', 'test1234');
    if (!teacherCookie) { test.skip(true, 'Teacher login not available'); return; }

    // Create
    const { data: created } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule', {
      day_of_week: 'Tuesday', start_time: '10:00', end_time: '11:00', subject: 'before-update',
    });
    const schedId = created.id;

    try {
      // Update
      const { status, data } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule', {
        id: schedId, day_of_week: 'Wednesday', start_time: '14:00', end_time: '15:30',
        subject: 'after-update', room_id: 'R01',
      });
      expect(status).toBe(200);
      expect(data.ok).toBe(true);

      // Verify
      const { data: classes } = await dashboardFetch(teacherCookie, '/api/dashboard/classes');
      const found = classes.find((c: any) => c.id === schedId);
      expect(found).toBeTruthy();
      expect(found.subject).toBe('after-update');
      expect(found.day_of_week).toBe('Wednesday');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('teacher can delete their own schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const teacherCookie = await userLogin('T01', 'test1234');
    if (!teacherCookie) { test.skip(true, 'Teacher login not available'); return; }

    const { data: created } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule', {
      day_of_week: 'Friday', start_time: '09:00', end_time: '10:00', subject: 'to-delete',
    });

    const { status, data } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule/delete',
      { id: created.id });
    expect(status).toBe(200);
    expect(data.ok).toBe(true);

    // Should not appear in classes
    const { data: classes } = await dashboardFetch(teacherCookie, '/api/dashboard/classes');
    const found = classes.find((c: any) => c.id === created.id);
    expect(found).toBeUndefined();
  });

  test('teacher cannot delete another teacher schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-OTHR-${stamp}`;

    // Create schedule owned by T02
    await dataCRUD(cookie, {
      action: 'save', type: 'schedules',
      data: { id: schedId, day_of_week: 'Monday', start_time: '10:00', end_time: '11:00',
        teacher_id: 'T02', subject: 'other-teacher' },
    });

    try {
      const teacherCookie = await userLogin('T01', 'test1234');
      if (!teacherCookie) { test.skip(true, 'Teacher login not available'); return; }

      const { status, data } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule/delete',
        { id: schedId });
      expect(status).toBe(403);
      expect(data.error).toContain('Not your schedule');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('student cannot create schedules', async () => {
    const studentCookie = await userLogin('S001', 'test1234');
    expect(studentCookie).toBeTruthy();

    const { status, data } = await dashboardFetch(studentCookie!, '/api/dashboard/schedule', {
      day_of_week: 'Monday', start_time: '10:00', end_time: '11:00', subject: 'no-access',
    });
    expect(status).toBe(403);
  });

  test('schedule creation rejects missing required fields', async () => {
    const teacherCookie = await userLogin('T01', 'test1234');
    if (!teacherCookie) return;

    const { status, data } = await dashboardFetch(teacherCookie, '/api/dashboard/schedule', {
      day_of_week: '', start_time: '10:00', end_time: '11:00',
    });
    expect(status).toBe(400);
  });
});
