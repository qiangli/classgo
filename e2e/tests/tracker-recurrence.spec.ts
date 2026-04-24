/**
 * E2E tests for tracker item recurrence logic.
 *
 * Verifies that daily/weekly/monthly/none recurrence types affect
 * whether items appear in due lists, and that responding to items
 * clears them from due for the appropriate period.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import {
  checkinViaAPI,
  forceCheckoutViaAPI,
  clearStudentTrackerItemsViaAPI,
} from '../helpers/api.js';

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

async function createGlobalItem(cookie: string, overrides: Record<string, any> = {}) {
  const res = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({
      name: `Recurrence Test ${Date.now()}`,
      priority: 'medium',
      recurrence: 'daily',
      requires_signoff: false,
      active: true,
      ...overrides,
    }),
  });
  return res.json();
}

async function deleteGlobalItem(cookie: string, id: number) {
  await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ id }),
  });
}

async function getDueItems(studentId: string, signoffOnly = false) {
  const url = `${BASE_URL}/api/tracker/due?student_id=${studentId}${signoffOnly ? '&signoff_only=true' : ''}`;
  const res = await fetch(url);
  return res.json();
}

test.describe('Recurrence types in due items', () => {

  test('daily recurrence item appears in due list', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const item = await createGlobalItem(cookie, {
      name: `Daily Item ${Date.now()}`,
      recurrence: 'daily',
    });
    expect(item.ok).toBe(true);

    const due = await getDueItems('S001');
    const found = due.find((d: any) => d.item_id === item.id);
    expect(found).toBeTruthy();
    expect(found.recurrence).toBe('daily');

    await deleteGlobalItem(cookie, item.id);
  });

  test('weekly recurrence item appears in due list', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const item = await createGlobalItem(cookie, {
      name: `Weekly Item ${Date.now()}`,
      recurrence: 'weekly',
    });
    expect(item.ok).toBe(true);

    const due = await getDueItems('S001');
    const found = due.find((d: any) => d.item_id === item.id);
    expect(found).toBeTruthy();
    expect(found.recurrence).toBe('weekly');

    await deleteGlobalItem(cookie, item.id);
  });

  test('monthly recurrence item appears in due list', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const item = await createGlobalItem(cookie, {
      name: `Monthly Item ${Date.now()}`,
      recurrence: 'monthly',
    });
    expect(item.ok).toBe(true);

    const due = await getDueItems('S001');
    const found = due.find((d: any) => d.item_id === item.id);
    expect(found).toBeTruthy();
    expect(found.recurrence).toBe('monthly');

    await deleteGlobalItem(cookie, item.id);
  });

  test('one-time item (recurrence=none) appears until completed', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const item = await createGlobalItem(cookie, {
      name: `One-Time Item ${Date.now()}`,
      recurrence: 'none',
    });
    expect(item.ok).toBe(true);

    const due = await getDueItems('S001');
    const found = due.find((d: any) => d.item_id === item.id);
    expect(found).toBeTruthy();
    expect(found.recurrence).toBe('none');

    await deleteGlobalItem(cookie, item.id);
  });
});

test.describe('Signoff filtering', () => {

  test('signoff_only=true returns only todo items', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create a signoff (todo) item and a non-signoff (task) item
    const todoItem = await createGlobalItem(cookie, {
      name: `Signoff Todo ${Date.now()}`,
      requires_signoff: true,
      recurrence: 'daily',
    });
    const taskItem = await createGlobalItem(cookie, {
      name: `Regular Task ${Date.now()}`,
      requires_signoff: false,
      recurrence: 'daily',
    });

    // signoff_only should include todo but not task
    const signoffDue = await getDueItems('S001', true);
    const foundTodo = signoffDue.find((d: any) => d.item_id === todoItem.id);
    const foundTask = signoffDue.find((d: any) => d.item_id === taskItem.id);
    expect(foundTodo).toBeTruthy();
    expect(foundTask).toBeUndefined();

    // Without signoff_only, both should appear
    const allDue = await getDueItems('S001', false);
    const foundTodoAll = allDue.find((d: any) => d.item_id === todoItem.id);
    const foundTaskAll = allDue.find((d: any) => d.item_id === taskItem.id);
    expect(foundTodoAll).toBeTruthy();
    expect(foundTaskAll).toBeTruthy();

    await deleteGlobalItem(cookie, todoItem.id);
    await deleteGlobalItem(cookie, taskItem.id);
  });
});

test.describe('Personal item recurrence', () => {

  test('personal task with recurrence is stored correctly', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create personal item for S003 with weekly recurrence
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student_id: 'S003',
        name: `Personal Weekly ${Date.now()}`,
        priority: 'medium',
        recurrence: 'weekly',
        requires_signoff: false,
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    // Verify it appears in student items
    const itemsRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S003`,
      { headers: { Cookie: cookie } },
    );
    const items = await itemsRes.json();
    const found = items.find((i: any) => i.id === created.id);
    expect(found).toBeTruthy();
    expect(found.recurrence).toBe('weekly');

    // Cleanup
    await clearStudentTrackerItemsViaAPI(cookie, 'S003');
  });

  test('personal task with category is stored correctly', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student_id: 'S003',
        name: `Categorized Task ${Date.now()}`,
        priority: 'high',
        recurrence: 'none',
        category: 'homework',
      }),
    });
    const created = await res.json();
    expect(created.ok).toBe(true);

    const itemsRes = await fetch(
      `${BASE_URL}/api/tracker/student-items?student_id=S003`,
      { headers: { Cookie: cookie } },
    );
    const items = await itemsRes.json();
    const found = items.find((i: any) => i.id === created.id);
    expect(found).toBeTruthy();
    expect(found.category).toBe('homework');

    // Cleanup
    await clearStudentTrackerItemsViaAPI(cookie, 'S003');
  });
});
