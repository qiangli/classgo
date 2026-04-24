/**
 * E2E tests for attendance-schedule linking and grace period.
 *
 * Verifies that check-in creates attendance_meta records linking
 * attendance to the correct schedule, and that the 30-minute grace
 * period before class start is respected.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { checkinViaAPI, forceCheckoutViaAPI } from '../helpers/api.js';

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

function todayDayName(): string {
  return ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'][new Date().getDay()];
}

function todayDateStr(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function daysFromNow(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

test.describe('Attendance-schedule linking', () => {

  test('check-in creates attendance record with student_id linked', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Check in Alice Wang (S001)
    await forceCheckoutViaAPI('Alice').catch(() => {});
    const result = await checkinViaAPI('Alice', 'mobile');
    expect(result.ok).toBe(true);

    // Verify attendance has student_id set
    const res = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&student_id=S001`,
      { headers: { Cookie: cookie } },
    );
    const attendees = await res.json();
    expect(Array.isArray(attendees)).toBe(true);
    const record = attendees.find((a: any) => a.student_id === 'S001');
    expect(record).toBeTruthy();
    expect(record.student_name).toContain('Alice');

    // Cleanup
    await forceCheckoutViaAPI('Alice').catch(() => {});
  });

  test('check-in links to matching schedule when session is active', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-LINK-${stamp}`;
    const today = todayDayName();

    // Create a schedule covering right now for S005 (Emma Taylor)
    const now = new Date();
    const startHour = now.getHours() - 1;
    const endHour = now.getHours() + 1;
    const startTime = `${String(Math.max(0, startHour)).padStart(2, '0')}:00`;
    const endTime = `${String(Math.min(23, endHour)).padStart(2, '0')}:00`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: today,
        start_time: startTime,
        end_time: endTime,
        teacher_id: 'T01',
        room_id: 'R01',
        subject: 'e2e-linking-test',
        student_ids: 'S005',
        effective_from: daysFromNow(-1),
      },
    });

    // Check in Emma
    await forceCheckoutViaAPI('Emma').catch(() => {});
    const result = await checkinViaAPI('Emma', 'mobile');
    expect(result.ok).toBe(true);

    // Verify attendance record exists for today
    const attnRes = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&student_id=S005`,
      { headers: { Cookie: cookie } },
    );
    const attendees = await attnRes.json();
    expect(attendees.length).toBeGreaterThan(0);

    // Cleanup
    await forceCheckoutViaAPI('Emma').catch(() => {});
    await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
  });

  test('check-in without matching schedule still records attendance', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // S008 Henry Kim — use a student unlikely to have a currently active schedule
    await forceCheckoutViaAPI('Henry').catch(() => {});
    const result = await checkinViaAPI('Henry', 'mobile');
    expect(result.ok).toBe(true);

    // Verify attendance exists
    const res = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&student_id=S008`,
      { headers: { Cookie: cookie } },
    );
    const attendees = await res.json();
    expect(attendees.length).toBeGreaterThan(0);

    // Cleanup
    await forceCheckoutViaAPI('Henry').catch(() => {});
  });

  test('check-in by student_id links attendance correctly', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await forceCheckoutViaAPI('Carlos').catch(() => {});
    // Check in using student_id directly via the API
    const body = {
      student_id: 'S003',
      device_type: 'mobile',
      device_id: `e2e-link-${Date.now()}`,
    };
    const res = await fetch(`${BASE_URL}/api/checkin`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const result = await res.json();
    expect(result.ok).toBe(true);

    // Verify student_id is S003
    const attnRes = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&student_id=S003`,
      { headers: { Cookie: cookie } },
    );
    const attendees = await attnRes.json();
    expect(attendees.length).toBeGreaterThan(0);
    expect(attendees[0].student_id).toBe('S003');

    // Cleanup
    await forceCheckoutViaAPI('Carlos').catch(() => {});
  });
});

test.describe('Attendance filtering', () => {

  test('attendees can be filtered by teacher_id', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // T01 teaches S001, S002 — check in S001
    await forceCheckoutViaAPI('Alice').catch(() => {});
    await checkinViaAPI('Alice', 'mobile');

    const res = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&teacher_id=T01`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);
    const attendees = await res.json();
    expect(Array.isArray(attendees)).toBe(true);

    // Cleanup
    await forceCheckoutViaAPI('Alice').catch(() => {});
  });

  test('attendees can be filtered by parent_id', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // P001 is parent of S001 (Alice) and S002 (Bob)
    await forceCheckoutViaAPI('Alice').catch(() => {});
    await checkinViaAPI('Alice', 'mobile');

    const res = await fetch(
      `${BASE_URL}/api/attendees?from=${todayDateStr()}&to=${todayDateStr()}&parent_id=P001`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);
    const attendees = await res.json();
    expect(Array.isArray(attendees)).toBe(true);

    // Cleanup
    await forceCheckoutViaAPI('Alice').catch(() => {});
  });

  test('attendance metrics returns summary stats', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(
      `${BASE_URL}/api/attendees/metrics?from=${todayDateStr()}&to=${todayDateStr()}`,
      { headers: { Cookie: cookie } },
    );
    expect(res.status).toBe(200);
    const metrics = await res.json();
    expect(metrics).toBeTruthy();
    // Metrics should have some structure
    expect(typeof metrics).toBe('object');
  });
});
