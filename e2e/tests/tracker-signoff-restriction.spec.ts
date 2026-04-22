import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S003'; // Carlos Garcia
const PARENT_ID = 'P002';  // Maria Garcia (parent of S003)
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

async function createTaskAs(cookie: string, studentId: string, name: string) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({
      student_id: studentId,
      name,
      priority: 'medium',
      recurrence: 'none',
      requires_signoff: true, // explicitly request signoff
      active: true,
    }),
  });
  return res.json();
}

async function getTaskItem(cookie: string, itemId: number) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items?student_id=${STUDENT_ID}`, {
    headers: { Cookie: cookie },
  });
  const items = await res.json();
  return Array.isArray(items) ? items.find((it: any) => it.id === itemId) : null;
}

test.describe('Signoff restriction by role', () => {

  test('student-created task is always type=task even if requires_signoff=true', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    const studentCookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Student creates a task requesting signoff
    const result = await createTaskAs(studentCookie!, STUDENT_ID, 'Student Signoff Test');
    expect(result.ok).toBe(true);

    // Verify the created item has type=task (not todo)
    const item = await getTaskItem(adminCookie, result.id);
    expect(item).toBeTruthy();
    expect(item.type).toBe('task');
    expect(item.requires_signoff).toBeFalsy();

    // Clean up
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('parent-created task is always type=task even if requires_signoff=true', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);
    const parentCookie = await userLogin(PARENT_ID, PASSWORD);
    expect(parentCookie).toBeTruthy();

    // Parent creates a task for their child requesting signoff
    const result = await createTaskAs(parentCookie!, STUDENT_ID, 'Parent Signoff Test');
    expect(result.ok).toBe(true);

    // Verify the created item has type=task (not todo)
    const item = await getTaskItem(adminCookie, result.id);
    expect(item).toBeTruthy();
    expect(item.type).toBe('task');
    expect(item.requires_signoff).toBeFalsy();

    // Clean up
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });

  test('admin-created task respects requires_signoff=true', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Admin creates a task requesting signoff
    const result = await createTaskAs(adminCookie, STUDENT_ID, 'Admin Signoff Test');
    expect(result.ok).toBe(true);

    // Verify the created item has type=todo (signoff required)
    const item = await getTaskItem(adminCookie, result.id);
    expect(item).toBeTruthy();
    expect(item.type).toBe('todo');
    expect(item.requires_signoff).toBe(true);

    // Clean up
    await clearStudentTrackerItemsViaAPI(adminCookie, STUDENT_ID);
  });
});
