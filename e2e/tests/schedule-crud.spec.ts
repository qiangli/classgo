/**
 * E2E tests for schedule CRUD operations, validation, session
 * materialization, conflict detection, and effective date boundaries.
 *
 * Covers the gaps identified in schedule-management.spec.ts which only
 * tests read-only endpoints and auth protection.
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

// --- API helpers ---

async function dataCRUD(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/data`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function getDirectory(cookie: string, includeDeleted = false) {
  const url = `${BASE_URL}/api/v1/directory` + (includeDeleted ? '?include_deleted=1' : '');
  const res = await fetch(url, { headers: { Cookie: cookie } });
  return res.json();
}

async function getWeekSessions(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/schedule/week`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getTodaySessions(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/schedule/today`, {
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

/** Returns the full English day name for today. */
function todayDayName(): string {
  return ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'][new Date().getDay()];
}

/** Returns today's date as YYYY-MM-DD. */
function todayDateStr(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** Returns a date string N days from now. */
function daysFromNow(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// ==================== Schedule CRUD ====================

test.describe('Schedule CRUD', () => {

  test('admin can create a new schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SCH-${stamp}`;

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Monday',
        start_time: '14:00',
        end_time: '15:30',
        teacher_id: 'T01',
        room_id: 'R01',
        subject: 'e2e-test-math',
        student_ids: 'S001;S002',
        effective_from: '2026-01-01',
        effective_until: '',
      },
    });
    expect(data.ok).toBe(true);

    // Verify in directory
    const dir = await getDirectory(cookie);
    const found = dir.schedules.find((s: any) => s.id === schedId);
    expect(found).toBeTruthy();
    expect(found.day_of_week).toBe('Monday');
    expect(found.start_time).toBe('14:00');
    expect(found.end_time).toBe('15:30');
    expect(found.teacher_id).toBe('T01');
    expect(found.room_id).toBe('R01');
    expect(found.subject).toBe('e2e-test-math');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('admin can update an existing schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SCHU-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Tuesday',
        start_time: '10:00',
        end_time: '11:00',
        teacher_id: 'T01',
        room_id: 'R01',
        subject: 'before-update',
        student_ids: 'S001',
      },
    });

    // Update — change subject, time, teacher, room, and students
    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Wednesday',
        start_time: '13:00',
        end_time: '14:30',
        teacher_id: 'T02',
        room_id: 'R02',
        subject: 'after-update',
        student_ids: 'S002;S003',
        effective_from: '2026-03-01',
        effective_until: '2026-06-30',
      },
    });
    expect(data.ok).toBe(true);

    // Verify all fields changed
    const dir = await getDirectory(cookie);
    const found = dir.schedules.find((s: any) => s.id === schedId);
    expect(found).toBeTruthy();
    expect(found.day_of_week).toBe('Wednesday');
    expect(found.start_time).toBe('13:00');
    expect(found.end_time).toBe('14:30');
    expect(found.teacher_id).toBe('T02');
    expect(found.room_id).toBe('R02');
    expect(found.subject).toBe('after-update');
    expect(found.effective_from).toBe('2026-03-01');
    expect(found.effective_until).toBe('2026-06-30');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('admin can soft-delete a schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SCHD-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Friday',
        start_time: '09:00',
        end_time: '10:00',
        subject: 'to-delete',
      },
    });

    // Delete
    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    expect(data.ok).toBe(true);

    // Should not appear in active directory
    const dir = await getDirectory(cookie);
    const found = dir.schedules.find((s: any) => s.id === schedId);
    expect(found).toBeUndefined();

    // Should appear with include_deleted
    const dirAll = await getDirectory(cookie, true);
    const foundDeleted = dirAll.schedules.find((s: any) => s.id === schedId);
    expect(foundDeleted).toBeTruthy();
    expect(foundDeleted.deleted).toBe(true);
  });

  test('soft-deleted schedule has audit fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SCHA-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Thursday',
        start_time: '16:00',
        end_time: '17:00',
        subject: 'audit-test',
      },
    });

    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });

    const dir = await getDirectory(cookie, true);
    const found = dir.schedules.find((s: any) => s.id === schedId);
    expect(found).toBeTruthy();
    expect(found.deleted).toBe(true);
    expect(found.deleted_at).toBeTruthy();
    expect(found.deleted_by).toBeTruthy();
  });
});

// ==================== Schedule Validation ====================

test.describe('Schedule validation', () => {

  test('rejects missing day_of_week', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: 'E2E-BAD-1',
        day_of_week: '',
        start_time: '10:00',
        end_time: '11:00',
      },
    });
    expect(data.ok).toBe(false);
    expect(data.error).toBeTruthy();
  });

  test('rejects missing start_time', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: 'E2E-BAD-2',
        day_of_week: 'Monday',
        start_time: '',
        end_time: '11:00',
      },
    });
    expect(data.ok).toBe(false);
    expect(data.error).toBeTruthy();
  });

  test('rejects missing end_time', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: 'E2E-BAD-3',
        day_of_week: 'Monday',
        start_time: '10:00',
        end_time: '',
      },
    });
    expect(data.ok).toBe(false);
    expect(data.error).toBeTruthy();
  });

  test('rejects missing schedule ID', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        day_of_week: 'Monday',
        start_time: '10:00',
        end_time: '11:00',
      },
    });
    expect(data.ok).toBe(false);
  });

  test('delete rejects empty ID', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: '' });
    expect(data.ok).toBe(false);
  });
});

// ==================== Schedule Auth Protection ====================

test.describe('Schedule CRUD auth protection', () => {

  test('unauthenticated user cannot create schedule', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/data`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        action: 'save',
        type: 'schedules',
        data: {
          id: 'E2E-UNAUTH',
          day_of_week: 'Monday',
          start_time: '10:00',
          end_time: '11:00',
        },
      }),
    });
    expect(res.status).toBe(401);
  });

  test('non-admin user cannot create schedule', async () => {
    const userCookie = await userLogin('S001', 'test1234');
    expect(userCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/data`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: userCookie! },
      body: JSON.stringify({
        action: 'save',
        type: 'schedules',
        data: {
          id: 'E2E-NONADMIN',
          day_of_week: 'Monday',
          start_time: '10:00',
          end_time: '11:00',
        },
      }),
    });
    expect(res.status).toBe(403);
  });

  test('non-admin user cannot delete schedule', async () => {
    const userCookie = await userLogin('S001', 'test1234');
    expect(userCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/data`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: userCookie! },
      body: JSON.stringify({ action: 'delete', type: 'schedules', id: 'SCH001' }),
    });
    expect(res.status).toBe(403);
  });
});

// ==================== Session Materialization ====================

test.describe('Session materialization', () => {

  test('newly created schedule appears in today sessions when day matches', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-TODAY-${stamp}`;
    const today = todayDayName();

    // Create a schedule for today's day of week
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '08:00',
        end_time: '09:00',
        teacher_id: 'T01',
        room_id: 'R01',
        subject: 'e2e-today-test',
        student_ids: 'S001',
        effective_from: daysFromNow(-1), // effective from yesterday
      },
    });

    // Verify it appears in today's sessions
    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeTruthy();
    expect(found.subject).toBe('e2e-today-test');
    expect(found.date_str).toBe(todayDateStr());

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('schedule does not appear in today sessions when day does not match', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-NODAY-${stamp}`;

    // Pick a day that is NOT today
    const days = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
    const today = todayDayName();
    const otherDay = days.find(d => d !== today)!;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: otherDay,
        start_time: '08:00',
        end_time: '09:00',
        subject: 'wrong-day',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeUndefined();

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('schedule appears in weekly sessions on the correct day', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-WEEK-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '07:00',
        end_time: '08:00',
        subject: 'e2e-weekly-test',
        effective_from: daysFromNow(-7),
      },
    });

    const { data } = await getWeekSessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeTruthy();
    expect(found.day_of_week).toBe(today);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('deleted schedule does not materialize sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-DELSESS-${stamp}`;
    const today = todayDayName();

    // Create and then delete
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '06:00',
        end_time: '07:00',
        subject: 'deleted-schedule',
        effective_from: daysFromNow(-1),
      },
    });

    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeUndefined();
  });
});

// ==================== Effective Date Boundaries ====================

test.describe('Effective date boundaries', () => {

  test('schedule not yet effective does not appear in today sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-FUTURE-${stamp}`;
    const today = todayDayName();

    // effective_from is tomorrow — should not appear today
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '08:00',
        end_time: '09:00',
        subject: 'future-schedule',
        effective_from: daysFromNow(1),
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeUndefined();

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('expired schedule does not appear in today sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-EXPIRED-${stamp}`;
    const today = todayDayName();

    // effective_until was yesterday — should not appear today
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '08:00',
        end_time: '09:00',
        subject: 'expired-schedule',
        effective_from: daysFromNow(-30),
        effective_until: daysFromNow(-1),
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeUndefined();

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('schedule within effective range appears in today sessions', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-ACTIVE-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '08:00',
        end_time: '09:00',
        subject: 'active-bounded',
        effective_from: daysFromNow(-7),
        effective_until: daysFromNow(7),
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeTruthy();
    expect(found.subject).toBe('active-bounded');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('schedule with no effective dates always appears', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-NODATE-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '08:00',
        end_time: '09:00',
        subject: 'no-date-bounds',
        effective_from: '',
        effective_until: '',
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeTruthy();

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });
});

// ==================== Conflict Detection ====================

test.describe('Conflict detection', () => {

  test('overlapping schedules in same room produce room conflict', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId1 = `E2E-CONF1-${stamp}`;
    const schedId2 = `E2E-CONF2-${stamp}`;
    const today = todayDayName();

    // Two schedules: same day, same room, overlapping times
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId1,
        day_of_week: today,
        start_time: '10:00',
        end_time: '11:30',
        teacher_id: 'T01',
        room_id: 'E2E-ROOM',
        subject: 'conflict-a',
        effective_from: daysFromNow(-1),
      },
    });

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId2,
        day_of_week: today,
        start_time: '11:00',
        end_time: '12:00',
        teacher_id: 'T02',
        room_id: 'E2E-ROOM',
        subject: 'conflict-b',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getScheduleConflicts(cookie);
    const roomConflicts = data.filter(
      (c: any) => c.type === 'room' && c.detail.includes('E2E-ROOM')
    );
    expect(roomConflicts.length).toBeGreaterThan(0);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId1 });
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId2 });
  });

  test('overlapping schedules with same teacher produce teacher conflict', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId1 = `E2E-TCONF1-${stamp}`;
    const schedId2 = `E2E-TCONF2-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId1,
        day_of_week: today,
        start_time: '14:00',
        end_time: '15:30',
        teacher_id: 'E2E-TEACHER',
        room_id: 'R01',
        subject: 'teacher-conflict-a',
        effective_from: daysFromNow(-1),
      },
    });

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId2,
        day_of_week: today,
        start_time: '15:00',
        end_time: '16:00',
        teacher_id: 'E2E-TEACHER',
        room_id: 'R02',
        subject: 'teacher-conflict-b',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getScheduleConflicts(cookie);
    const teacherConflicts = data.filter(
      (c: any) => c.type === 'teacher' && c.detail.includes('E2E-TEACHER')
    );
    expect(teacherConflicts.length).toBeGreaterThan(0);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId1 });
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId2 });
  });

  test('overlapping schedules with same student produce student conflict', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId1 = `E2E-SCONF1-${stamp}`;
    const schedId2 = `E2E-SCONF2-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId1,
        day_of_week: today,
        start_time: '17:00',
        end_time: '18:00',
        teacher_id: 'T01',
        room_id: 'R01',
        subject: 'student-conflict-a',
        student_ids: 'S001;S002',
        effective_from: daysFromNow(-1),
      },
    });

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId2,
        day_of_week: today,
        start_time: '17:30',
        end_time: '18:30',
        teacher_id: 'T02',
        room_id: 'R02',
        subject: 'student-conflict-b',
        student_ids: 'S002;S003',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getScheduleConflicts(cookie);
    const studentConflicts = data.filter(
      (c: any) => c.type === 'student' && c.detail.includes('S002')
    );
    expect(studentConflicts.length).toBeGreaterThan(0);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId1 });
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId2 });
  });

  test('non-overlapping schedules produce no conflict', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId1 = `E2E-NOCONF1-${stamp}`;
    const schedId2 = `E2E-NOCONF2-${stamp}`;
    const today = todayDayName();

    // Adjacent times — should NOT conflict
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId1,
        day_of_week: today,
        start_time: '19:00',
        end_time: '20:00',
        teacher_id: 'E2E-T-NC',
        room_id: 'E2E-R-NC',
        subject: 'no-conflict-a',
        student_ids: 'S001',
        effective_from: daysFromNow(-1),
      },
    });

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId2,
        day_of_week: today,
        start_time: '20:00',
        end_time: '21:00',
        teacher_id: 'E2E-T-NC',
        room_id: 'E2E-R-NC',
        subject: 'no-conflict-b',
        student_ids: 'S001',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getScheduleConflicts(cookie);
    // These specific schedules should not conflict (adjacent, not overlapping)
    const badConflicts = data.filter(
      (c: any) =>
        (c.session1.schedule_id === schedId1 && c.session2.schedule_id === schedId2) ||
        (c.session1.schedule_id === schedId2 && c.session2.schedule_id === schedId1)
    );
    expect(badConflicts.length).toBe(0);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId1 });
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId2 });
  });
});

// ==================== Schedule with student_ids ====================

test.describe('Schedule student enrollment', () => {

  test('student_ids round-trips correctly through create and directory', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SIDS-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Monday',
        start_time: '10:00',
        end_time: '11:00',
        subject: 'enrollment-test',
        student_ids: 'S001;S002;S003',
      },
    });

    const dir = await getDirectory(cookie);
    const found = dir.schedules.find((s: any) => s.id === schedId);
    expect(found).toBeTruthy();

    // student_ids comes back as an array in JSON
    const ids = found.student_ids;
    expect(ids).toContain('S001');
    expect(ids).toContain('S002');
    expect(ids).toContain('S003');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('session materializes with correct student_ids', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-SESSIDS-${stamp}`;
    const today = todayDayName();

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: '07:30',
        end_time: '08:30',
        subject: 'session-enrollment',
        student_ids: 'S004;S005',
        effective_from: daysFromNow(-1),
      },
    });

    const { data } = await getTodaySessions(cookie);
    const found = data.find((s: any) => s.schedule_id === schedId);
    expect(found).toBeTruthy();
    expect(found.student_ids).toContain('S004');
    expect(found.student_ids).toContain('S005');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });
});

// ==================== CSV Export ====================

test.describe('Schedule export', () => {

  test('schedule export includes created schedule', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-EXPORT-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Friday',
        start_time: '15:00',
        end_time: '16:00',
        subject: 'export-test',
        teacher_id: 'T01',
        room_id: 'R01',
        student_ids: 'S001',
      },
    });

    // Export schedules CSV
    const res = await fetch(`${BASE_URL}/admin/export/csv?type=schedules`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const csv = await res.text();
    expect(csv).toContain(schedId);
    expect(csv).toContain('export-test');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });
});
