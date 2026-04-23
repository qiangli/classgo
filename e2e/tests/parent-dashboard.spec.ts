import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PARENT_ID = 'P001';  // Wei Wang
const CHILD_1 = 'S001';    // Alice Wang
const CHILD_2 = 'S002';    // Bob Wang
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

async function getAllTasks(cookie: string, studentId: string) {
  const res = await fetch(`${BASE_URL}/api/dashboard/all-tasks?student_id=${studentId}`, {
    headers: { Cookie: cookie },
  });
  return res.json();
}

async function getMyStudents(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/dashboard/my-students`, {
    headers: { Cookie: cookie },
  });
  return res.json();
}

// ==================== Parent Dashboard Page ====================

test.describe('Parent dashboard page', () => {

  test('parent can load dashboard page', async ({ page }) => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/dashboard`);
    await expect(page.locator('body')).toContainText('Wei Wang');
  });

  test('parent home page shows correct role badge', async ({ page }) => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('header')).toContainText('parent');
  });
});

// ==================== Parent Children Access ====================

test.describe('Parent children access via API', () => {

  test('parent can view list of children via my-students', async () => {
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const students = await getMyStudents(parentCookie!);
    expect(Array.isArray(students)).toBe(true);
    expect(students.length).toBeGreaterThanOrEqual(2);

    const ids = students.map((s: any) => s.id);
    expect(ids).toContain(CHILD_1);
    expect(ids).toContain(CHILD_2);
  });

  test('parent children include parent info', async () => {
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const students = await getMyStudents(parentCookie!);
    const alice = students.find((s: any) => s.id === CHILD_1);
    expect(alice).toBeTruthy();
    expect(alice.parent_id).toBe(PARENT_ID);
    expect(alice.parent).toBeTruthy();
  });
});

// ==================== Parent Task Viewing ====================

test.describe('Parent viewing children tasks', () => {

  test('parent can view first child tasks', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);

    // Admin creates a task for child 1
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        student_id: CHILD_1, name: 'Alice Homework', priority: 'high',
        recurrence: 'none', active: true,
      }),
    });
    expect((await res.json()).ok).toBe(true);

    // Parent views child 1 tasks
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const tasks = await getAllTasks(parentCookie!, CHILD_1);
    expect(tasks.student_items).toBeTruthy();
    const names = tasks.student_items.map((i: any) => i.name);
    expect(names).toContain('Alice Homework');

    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);
  });

  test('parent can view second child tasks', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_2);

    // Admin creates a task for child 2
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        student_id: CHILD_2, name: 'Bob Project', priority: 'medium',
        recurrence: 'none', active: true,
      }),
    });
    expect((await res.json()).ok).toBe(true);

    // Parent views child 2 tasks
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const tasks = await getAllTasks(parentCookie!, CHILD_2);
    expect(tasks.student_items).toBeTruthy();
    const names = tasks.student_items.map((i: any) => i.name);
    expect(names).toContain('Bob Project');

    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_2);
  });

  test('parent default all-tasks resolves to first child', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    // Call without student_id — should resolve to first child
    const res = await fetch(`${BASE_URL}/api/dashboard/all-tasks`, {
      headers: { Cookie: parentCookie! },
    });
    const tasks = await res.json();
    expect(tasks).toBeTruthy();
    // Should not error out
    expect(tasks.student_items).toBeDefined();
  });
});

// ==================== Parent Task Creation ====================

test.describe('Parent creating tasks for children', () => {

  test('parent can create task for own child', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);

    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: parentCookie! },
      body: JSON.stringify({
        student_id: CHILD_1, name: 'Parent Assigned Task',
        priority: 'medium', recurrence: 'none', active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Verify it shows up
    const tasks = await getAllTasks(parentCookie!, CHILD_1);
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.owner_type).toBe('parent');
    expect(item.type).toBe('task'); // parent-created is always task

    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);
  });

  test('parent-created task ignores requires_signoff flag', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);

    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: parentCookie! },
      body: JSON.stringify({
        student_id: CHILD_1, name: 'Signoff Attempt',
        priority: 'medium', recurrence: 'none',
        requires_signoff: true, active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Even with requires_signoff=true, parent task should be type=task
    const tasks = await getAllTasks(parentCookie!, CHILD_1);
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeTruthy();
    expect(item.type).toBe('task');

    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);
  });

  test('parent can delete own-created task', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    await clearStudentTrackerItemsViaAPI(adminCookie, CHILD_1);

    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    // Create task
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: parentCookie! },
      body: JSON.stringify({
        student_id: CHILD_1, name: 'Deletable Task',
        priority: 'low', recurrence: 'none', active: true,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Delete it
    const delRes = await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: parentCookie! },
      body: JSON.stringify({ id: created.id }),
    });
    expect((await delRes.json()).ok).toBe(true);

    // Verify it's gone
    const tasks = await getAllTasks(parentCookie!, CHILD_1);
    const item = tasks.student_items.find((i: any) => i.id === created.id);
    expect(item).toBeUndefined();
  });
});
