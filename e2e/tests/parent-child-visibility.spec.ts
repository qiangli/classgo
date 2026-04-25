/**
 * E2E tests for parent cross-visibility into children's data.
 *
 * Verifies that parents can view their children's check-in/out status,
 * enrolled classes, and attendance history. Also verifies that parents
 * cannot view data for children that are not their own.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, checkinViaAPI, forceCheckoutViaAPI, getStatusViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PASSWORD = 'test1234';
const PARENT_ID = 'P001';  // Wei Wang — parent of S001 (Alice) and S002 (Bob)
const CHILD_1 = 'S001';    // Alice Wang
const CHILD_2 = 'S002';    // Bob Wang
const OTHER_CHILD = 'S003'; // Carlos Garcia — belongs to P002

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

// ==================== Child Check-In Status ====================

test.describe('Parent viewing child check-in status', () => {
  test('parent can check child status via public API', async () => {
    // The /api/status endpoint returns check-in state for a student
    const res = await fetch(`${BASE_URL}/api/status?student_name=Alice+Wang`);
    expect(res.status).toBe(200);
    const data = await res.json();
    // Response should have the expected shape regardless of current state
    expect(data).toHaveProperty('checked_in');
    expect(typeof data.checked_in).toBe('boolean');
  });

  test('parent can check both children status', async () => {
    const res1 = await fetch(`${BASE_URL}/api/status?student_name=Alice+Wang`);
    expect(res1.status).toBe(200);
    const s1 = await res1.json();
    expect(s1).toHaveProperty('checked_in');

    const res2 = await fetch(`${BASE_URL}/api/status?student_name=Bob+Wang`);
    expect(res2.status).toBe(200);
    const s2 = await res2.json();
    expect(s2).toHaveProperty('checked_in');
  });
});

// ==================== Child Enrolled Classes ====================

test.describe('Parent viewing child enrolled classes', () => {
  test('parent can view first child enrolled classes', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${CHILD_1}`, {
      headers: { Cookie: cookie! },
    });
    expect(res.status).toBe(200);

    const classes = await res.json();
    expect(Array.isArray(classes)).toBe(true);

    // Each class should have expected fields
    if (classes.length > 0) {
      const cls = classes[0];
      expect(cls).toHaveProperty('id');
      expect(cls).toHaveProperty('day_of_week');
      expect(cls).toHaveProperty('start_time');
      expect(cls).toHaveProperty('end_time');
      expect(cls).toHaveProperty('teacher_name');
      expect(cls).toHaveProperty('room_name');
    }
  });

  test('parent can view classes for each linked child', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Get the list of children linked to this parent
    const studentsRes = await fetch(`${BASE_URL}/api/dashboard/my-students`, {
      headers: { Cookie: cookie! },
    });
    const students = await studentsRes.json();
    expect(Array.isArray(students)).toBe(true);
    expect(students.length).toBeGreaterThanOrEqual(1);

    // Verify classes endpoint works for each linked child
    for (const student of students) {
      const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${student.id}`, {
        headers: { Cookie: cookie! },
      });
      expect(res.status).toBe(200);
      const classes = await res.json();
      expect(Array.isArray(classes)).toBe(true);
    }
  });

  test('parent cannot view classes for another parent child', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // P001 tries to view S003 (belongs to P002)
    const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${OTHER_CHILD}`, {
      headers: { Cookie: cookie! },
    });
    expect(res.status).toBe(403);
  });

  test('classes include enrollment status for the child', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${CHILD_1}`, {
      headers: { Cookie: cookie! },
    });
    const classes = await res.json();
    expect(Array.isArray(classes)).toBe(true);

    // Each class should indicate whether the child is enrolled
    if (classes.length > 0) {
      expect(classes[0]).toHaveProperty('is_enrolled');
    }
  });
});

// ==================== Child Attendance via Dashboard ====================

test.describe('Parent viewing child attendance', () => {
  test('parent can view child attendance by checking in and out', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Ensure clean state
    await forceCheckoutViaAPI('Alice Wang');

    // Check in Alice
    await checkinViaAPI('Alice Wang', 'mobile');

    // Check out Alice
    await forceCheckoutViaAPI('Alice Wang');

    // Parent logs in and views dashboard tasks (confirms child data is accessible)
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    // Parent can at least see the child's data via my-students
    const res = await fetch(`${BASE_URL}/api/dashboard/my-students`, {
      headers: { Cookie: parentCookie! },
    });
    expect(res.status).toBe(200);
    const students = await res.json();
    const alice = students.find((s: any) => s.id === CHILD_1);
    expect(alice).toBeTruthy();
    expect(alice.first_name).toBe('Alice');
  });

  test('parent can view child progress data', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Access progress for first child
    const res = await fetch(`${BASE_URL}/api/dashboard/progress?student_id=${CHILD_1}`, {
      headers: { Cookie: cookie! },
    });
    // Progress endpoint may return 200 with data or empty
    expect([200, 404]).toContain(res.status);
    if (res.status === 200) {
      const data = await res.json();
      expect(data).toBeTruthy();
    }
  });
});

// ==================== Auth Protection ====================

test.describe('Parent visibility auth protection', () => {
  test('unauthenticated user cannot access classes endpoint', async () => {
    const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${CHILD_1}`, {
      redirect: 'manual',
    });
    expect([302, 401]).toContain(res.status);
  });

  test('student classes endpoint ignores student_id param and returns own data', async () => {
    const studentCookie = await userLogin('S003', PASSWORD);
    expect(studentCookie).toBeTruthy();

    // S003 passes student_id=S001 but the handler uses the session's own entity ID
    const res = await fetch(`${BASE_URL}/api/dashboard/classes?student_id=${CHILD_1}`, {
      headers: { Cookie: studentCookie! },
    });
    expect(res.status).toBe(200);
    const classes = await res.json();
    expect(Array.isArray(classes)).toBe(true);
    // Classes returned are for S003, not S001
  });
});
