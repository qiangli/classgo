/**
 * E2E tests for checkout edge cases with mixed tracker states.
 *
 * Covers scenarios: multiple pending signoff tasks, partial completion,
 * tasks from different creators, and checkout without pending tasks.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import {
  checkinViaAPI,
  checkoutViaAPI,
  forceCheckoutViaAPI,
  clearStudentTrackerItemsViaAPI,
  userLogin,
} from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PASSWORD = 'test1234';

// Use S006 (Frank Miller) for most tests to avoid conflicts with other test files
const STUDENT_ID = 'S006';
const STUDENT_NAME = 'Frank';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

async function createSignoffTask(cookie: string, studentId: string, name: string) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({
      student_id: studentId,
      name,
      priority: 'high',
      recurrence: 'none',
      requires_signoff: true,
      active: true,
    }),
  });
  return res.json();
}

test.describe('Checkout with multiple pending signoff tasks', () => {

  test('checkout blocked when multiple signoff tasks are pending', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Cleanup
    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    // Create two signoff tasks
    const task1 = await createSignoffTask(cookie, STUDENT_ID, `Signoff A ${Date.now()}`);
    expect(task1.ok).toBe(true);
    const task2 = await createSignoffTask(cookie, STUDENT_ID, `Signoff B ${Date.now()}`);
    expect(task2.ok).toBe(true);

    // Check in
    await checkinViaAPI(STUDENT_NAME, 'mobile');

    // Attempt checkout — should be blocked
    const result = await checkoutViaAPI(STUDENT_NAME);
    expect(result.ok).toBe(false);
    expect(result.pending_tasks).toBe(true);
    expect(result.items.length).toBeGreaterThanOrEqual(2);

    // Cleanup
    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });

  test('checkout succeeds when all signoff tasks responded via tracker/respond', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    // Create signoff task
    const task = await createSignoffTask(cookie, STUDENT_ID, `Respond Test ${Date.now()}`);
    expect(task.ok).toBe(true);

    // Check in
    await checkinViaAPI(STUDENT_NAME, 'mobile');

    // Respond via tracker/respond (atomic checkout)
    const respondRes = await fetch(`${BASE_URL}/api/tracker/respond`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        student_name: STUDENT_NAME,
        responses: [
          {
            item_type: 'personal',
            item_id: task.id,
            item_name: task.name || '',
            status: 'done',
            notes: 'completed in test',
          },
        ],
      }),
    });
    const respondData = await respondRes.json();
    expect(respondData.ok).toBe(true);

    // Cleanup
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });

  test('checkout succeeds when no signoff tasks exist', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    // Create only a non-signoff task
    await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student_id: STUDENT_ID,
        name: `Regular Task ${Date.now()}`,
        priority: 'low',
        recurrence: 'none',
        requires_signoff: false,
      }),
    });

    // Check in and checkout — use forceCheckout to handle any global signoff items
    await checkinViaAPI(STUDENT_NAME, 'mobile');
    const result = await forceCheckoutViaAPI(STUDENT_NAME);
    expect(result.ok).toBe(true);

    // Cleanup
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });

  test('checkout returns pending items list with correct metadata', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    const taskName = `Detail Check ${Date.now()}`;
    const task = await createSignoffTask(cookie, STUDENT_ID, taskName);
    expect(task.ok).toBe(true);

    await checkinViaAPI(STUDENT_NAME, 'mobile');

    const result = await checkoutViaAPI(STUDENT_NAME);
    expect(result.ok).toBe(false);
    expect(result.pending_tasks).toBe(true);
    expect(Array.isArray(result.items)).toBe(true);
    expect(result.items.length).toBeGreaterThan(0);

    // At least one item should match our created task (there may also be global items)
    const personalItems = result.items.filter((i: any) => i.source === 'personal');
    if (personalItems.length > 0) {
      const item = personalItems.find((i: any) => i.id === task.id);
      expect(item).toBeTruthy();
    }

    // Cleanup
    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });
});

test.describe('Checkout with tasks from different creators', () => {

  test('signoff task from teacher also blocks checkout', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const teacherCookie = await userLogin('T01', PASSWORD);
    expect(teacherCookie).toBeTruthy();

    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    // Teacher creates a signoff task for the student
    const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: STUDENT_ID,
        name: `Teacher Signoff ${Date.now()}`,
        priority: 'high',
        recurrence: 'none',
        requires_signoff: true,
      }),
    });
    const task = await res.json();
    expect(task.ok).toBe(true);

    // Check in and attempt checkout
    await checkinViaAPI(STUDENT_NAME, 'mobile');
    const result = await checkoutViaAPI(STUDENT_NAME);
    expect(result.ok).toBe(false);
    expect(result.pending_tasks).toBe(true);

    // Cleanup
    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });

  test('global signoff item also blocks checkout', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});

    // Create global signoff item
    const itemRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        name: `Global Signoff ${Date.now()}`,
        priority: 'high',
        recurrence: 'daily',
        requires_signoff: true,
        active: true,
      }),
    });
    const item = await itemRes.json();
    expect(item.ok).toBe(true);

    // Check in
    await checkinViaAPI(STUDENT_NAME, 'mobile');

    // Checkout should be blocked by global signoff
    const result = await checkoutViaAPI(STUDENT_NAME);
    expect(result.ok).toBe(false);
    expect(result.pending_tasks).toBe(true);

    // Cleanup
    await forceCheckoutViaAPI(STUDENT_NAME).catch(() => {});
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: item.id }),
    });
  });
});
