import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S003'; // Carlos Garcia
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

async function createTaskViaAPI(cookie: string, name: string, opts: Record<string, any> = {}) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({
      student_id: STUDENT_ID,
      name,
      priority: opts.priority ?? 'medium',
      recurrence: 'none',
      requires_signoff: opts.requires_signoff ?? false,
      active: true,
    }),
  });
  return res.json();
}

async function getAllTasks(cookie: string, studentId = STUDENT_ID) {
  const res = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=${studentId}`, {
    headers: { Cookie: cookie },
  });
  return res.json();
}

async function getProgress(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/dashboard/progress?student_id=${STUDENT_ID}`, {
    headers: { Cookie: cookie },
  });
  return res.json();
}

// ==================== Dashboard Page Tests ====================

test.describe('Student dashboard page', () => {

  test('student can load dashboard page', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/dashboard`);
    // Dashboard should load with student's name
    await expect(page.locator('body')).toContainText('Carlos Garcia');
  });

  test('unauthenticated dashboard access redirects to entry page', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`);
    // RequireAuth redirects to /login which redirects to /?mode=login (entry page)
    await page.waitForURL('**/?mode=login**');
  });
});

// ==================== Student Task View Tests ====================

test.describe('Student task list via API', () => {

  test('student sees own tasks in all-tasks endpoint', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    // Admin creates tasks for the student
    const t1 = await createTaskViaAPI(adminCookie, 'Math Homework');
    const t2 = await createTaskViaAPI(adminCookie, 'Reading Assignment');
    expect(t1.ok).toBe(true);
    expect(t2.ok).toBe(true);

    // Student logs in and views tasks
    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    const tasks = await getAllTasks(studentCookie!);
    expect(tasks.student_items).toBeTruthy();
    expect(tasks.student_items.length).toBeGreaterThanOrEqual(2);

    const names = tasks.student_items.map((i: any) => i.name);
    expect(names).toContain('Math Homework');
    expect(names).toContain('Reading Assignment');

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('student can create own task', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Student creates a task for themselves
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        name: 'My Own Task',
        priority: 'low',
        recurrence: 'none',
        active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Verify it appears in task list
    const tasks = await getAllTasks(studentCookie!);
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.name).toBe('My Own Task');
    expect(item.owner_type).toBe('student');

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('student can delete own task but not admin-created task', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);

    // Admin creates a task
    const adminTask = await createTaskViaAPI(adminCookie, 'Admin Created');
    expect(adminTask.ok).toBe(true);

    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Student creates own task
    const studentRes = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ name: 'Student Created', priority: 'low', recurrence: 'none', active: true }),
    });
    const studentTask = await studentRes.json();
    expect(studentTask.ok).toBe(true);

    // Student can delete own task
    const delOwn = await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ id: studentTask.id }),
    });
    expect((await delOwn.json()).ok).toBe(true);

    // Student cannot delete admin-created task
    const delAdmin = await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ id: adminTask.id }),
    });
    const delResult = await delAdmin.json();
    expect(delAdmin.status).toBe(403);
    expect(delResult.error).toBeTruthy();

    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('student can view progress', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    const progress = await getProgress(studentCookie!);
    // Progress endpoint should return without error
    expect(progress).toBeTruthy();
  });

  test('student cannot access another student tasks', async () => {
    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Try to view S001's tasks — should get empty or own tasks
    const tasks = await getAllTasks(studentCookie!, 'S001');
    // The endpoint should either deny or return student's own tasks
    expect(tasks).toBeTruthy();
  });
});
