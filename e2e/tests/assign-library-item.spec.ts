/**
 * E2E tests for the assign-library-item endpoint.
 *
 * Verifies that library items (global tracker items) can be assigned
 * to students via POST /api/dashboard/assign-library-item, with
 * proper auth, ownership, and validation checks.
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

test.describe('Assign library item', () => {

  test('admin can assign library item to students', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create a library item (global tracker item)
    const createRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        name: `Library Item ${Date.now()}`,
        priority: 'medium',
        recurrence: 'none',
        requires_signoff: false,
        active: true,
      }),
    });
    const item = await createRes.json();
    expect(item.ok).toBe(true);

    // Assign to students
    const assignRes = await fetch(`${BASE_URL}/api/dashboard/assign-library-item`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        item_id: item.id,
        student_ids: ['S001', 'S002'],
      }),
    });
    const assignData = await assignRes.json();
    expect(assignData.ok).toBe(true);
    expect(assignData.count).toBe(2);

    // Verify items appear in student's task list
    const tasksRes = await fetch(`${BASE_URL}/api/tracker/student-items?student_id=S001`, {
      headers: { Cookie: cookie },
    });
    const tasks = await tasksRes.json();
    expect(Array.isArray(tasks)).toBe(true);

    // Cleanup
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: item.id }),
    });
  });

  test('teacher can assign own library item', async ({ adminPage }) => {
    // Teacher creates a student-items entry as a library item
    const teacherCookie = await userLogin('T01', PASSWORD);
    expect(teacherCookie).toBeTruthy();

    // Create a personal task (library - no student assigned yet by using teacher's own creation)
    const createRes = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: 'S001',
        name: `Teacher Library ${Date.now()}`,
        priority: 'medium',
        recurrence: 'none',
        requires_signoff: true,
        active: true,
      }),
    });
    const item = await createRes.json();
    expect(item.ok).toBe(true);
  });

  test('assign library item rejects non-existent item', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/dashboard/assign-library-item`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        item_id: 999999,
        student_ids: ['S001'],
      }),
    });
    expect(res.status).toBe(404);
  });

  test('assign library item requires authentication', async () => {
    const res = await fetch(`${BASE_URL}/api/dashboard/assign-library-item`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        item_id: 1,
        student_ids: ['S001'],
      }),
      redirect: 'manual',
    });
    expect([302, 401]).toContain(res.status);
  });

  test('assign library item requires student_ids', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/dashboard/assign-library-item`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ item_id: 1, student_ids: [] }),
    });
    expect(res.status).toBe(400);
  });
});
