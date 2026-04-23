import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const TEACHER_ID = 'T01';   // Sarah Smith
const TEACHER_2 = 'T02';    // James Johnson
const STUDENT_ID = 'S003';  // Carlos Garcia
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

// ==================== Teacher Dashboard Page ====================

test.describe('Teacher dashboard page', () => {

  test('teacher can load dashboard page', async ({ page }) => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/dashboard`);
    await expect(page.locator('body')).toContainText('Sarah Smith');
  });

  test('teacher home page navigates to dashboard', async ({ page }) => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    // Click Tasks tile to go to dashboard
    await page.locator('#all-grid .app-tile-wrap').filter({ hasText: 'Tasks' }).locator('.app-tile').click();
    await page.waitForURL('**/dashboard');
    expect(page.url()).toContain('/dashboard');
  });
});

// ==================== Teacher Classes ====================

test.describe('Teacher classes via API', () => {

  test('teacher can view own classes', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/my-classes`, {
      headers: { Cookie: cookie! },
    });
    const classes = await res.json();
    expect(Array.isArray(classes)).toBe(true);
    // Classes should have expected fields if any exist
    if (classes.length > 0) {
      expect(classes[0]).toHaveProperty('id');
      expect(classes[0]).toHaveProperty('day_of_week');
      expect(classes[0]).toHaveProperty('start_time');
      expect(classes[0]).toHaveProperty('end_time');
    }
  });

  test('non-teacher gets forbidden for my-classes', async () => {
    const cookie = await userLogin('S003', PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/my-classes`, {
      headers: { Cookie: cookie! },
    });
    expect(res.status).toBe(403);
  });
});

// ==================== Teacher Students ====================

test.describe('Teacher students via API', () => {

  test('teacher can view own students', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/my-students`, {
      headers: { Cookie: cookie! },
    });
    const students = await res.json();
    expect(Array.isArray(students)).toBe(true);
    // If teacher has students, verify structure
    if (students.length > 0) {
      expect(students[0]).toHaveProperty('id');
      expect(students[0]).toHaveProperty('first_name');
      expect(students[0]).toHaveProperty('last_name');
    }
  });
});

// ==================== Teacher Task Creation ====================

test.describe('Teacher task management', () => {

  test('teacher can create task for a student', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'Teacher Assigned Homework',
        priority: 'high', recurrence: 'none', active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Verify via admin
    const tasksRes = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=${STUDENT_ID}`, {
      headers: { Cookie: adminCookie },
    });
    const tasks = await tasksRes.json();
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.owner_type).toBe('teacher');

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('teacher can create signoff task (type=todo)', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'Needs Signoff',
        priority: 'high', recurrence: 'none',
        requires_signoff: true, active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Verify type is todo (signoff enabled)
    const tasksRes = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=${STUDENT_ID}`, {
      headers: { Cookie: adminCookie },
    });
    const tasks = await tasksRes.json();
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.type).toBe('todo');

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('teacher can delete own-created task', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    // Create task
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'To Be Deleted',
        priority: 'medium', recurrence: 'none', active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Delete it
    const delRes = await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({ id: created.id }),
    });
    expect((await delRes.json()).ok).toBe(true);

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('teacher cannot delete admin-created task', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    // Admin creates a task
    const adminRes = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'Admin Task',
        priority: 'high', recurrence: 'none', active: true,
      }),
    });
    const adminTask = await adminRes.json();
    expect(adminTask.ok).toBe(true);

    // Teacher tries to delete it
    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const delRes = await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({ id: adminTask.id }),
    });
    const delData = await delRes.json();
    expect(delRes.status).toBe(403);
    expect(delData.error).toBeTruthy();

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });
});

// ==================== Teacher Items View ====================

test.describe('Teacher items view', () => {

  test('teacher can view own created items via teacher-items endpoint', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    // Teacher creates a task
    const createRes = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'Teacher Item View Test',
        priority: 'medium', recurrence: 'none', active: true,
      }),
    });
    const created = await createRes.json();
    expect(created.ok).toBe(true);

    // View via teacher-items endpoint
    const res = await fetch(`${BASE_URL}/api/dashboard/teacher-items`, {
      headers: { Cookie: teacherCookie! },
    });
    const items = await res.json();
    expect(Array.isArray(items)).toBe(true);

    const item = items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.name).toBe('Teacher Item View Test');
    expect(item.student_name).toBeTruthy(); // Should include resolved student name

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('teacher-items only shows own items, not other teacher items', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    // Teacher 1 creates a task
    const t1Cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(t1Cookie).toBeTruthy();
    const t1Res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: t1Cookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID, name: 'T01 Item',
        priority: 'medium', recurrence: 'none', active: true,
      }),
    });
    expect((await t1Res.json()).ok).toBe(true);

    // Teacher 2 views their items — should not see T01's item
    const t2Cookie = await userLogin(TEACHER_2, PASSWORD);
    expect(t2Cookie).toBeTruthy();
    const res = await fetch(`${BASE_URL}/api/dashboard/teacher-items`, {
      headers: { Cookie: t2Cookie! },
    });
    const items = await res.json();
    const found = items.find((i: any) => i.name === 'T01 Item');
    expect(found).toBeUndefined();

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });
});

// ==================== Teacher Bulk Assign ====================

test.describe('Teacher bulk assign', () => {

  test('teacher can bulk assign tasks to multiple students', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, 'S001');
    await clearStudentTrackerItemsViaAPI(adminCookie, 'S002');

    const teacherCookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/bulk-assign`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_ids: ['S001', 'S002'],
        name: 'Group Assignment',
        priority: 'high',
        recurrence: 'none',
        active: true,
      }),
    });
    const result = await res.json();
    expect(result.ok).toBe(true);

    // Verify both students got the task
    const tasks1 = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=S001`, {
      headers: { Cookie: adminCookie },
    });
    const data1 = await tasks1.json();
    expect(data1.student_items.some((i: any) => i.name === 'Group Assignment')).toBe(true);

    const tasks2 = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=S002`, {
      headers: { Cookie: adminCookie },
    });
    const data2 = await tasks2.json();
    expect(data2.student_items.some((i: any) => i.name === 'Group Assignment')).toBe(true);

    await clearStudentTrackerItemsViaAPI(adminCookie, 'S001');
    await clearStudentTrackerItemsViaAPI(adminCookie, 'S002');
  });

  test('student cannot bulk assign tasks', async () => {
    const studentCookie = await userLogin('S003', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/dashboard/bulk-assign`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student_ids: ['S001'], name: 'Should Fail',
        priority: 'medium', recurrence: 'none', active: true,
      }),
    });
    const result = await res.json();
    expect(res.status).toBe(403);
    expect(result.error).toBeTruthy();
  });
});
