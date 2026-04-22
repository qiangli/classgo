import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { checkinViaAPI, checkoutViaAPI, clearStudentTrackerItemsViaAPI } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S010'; // Jack Brown

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

async function createTask(cookie: string, name: string) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({
      student_id: STUDENT_ID,
      name,
      priority: 'medium',
      recurrence: 'none',
      requires_signoff: true,
      active: true,
    }),
  });
  return res.json();
}

async function completeTask(cookie: string, id: number, complete: boolean) {
  const res = await fetch(`${BASE_URL}/api/tracker/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ id, complete }),
  });
  return res.json();
}

async function getProgress(cookie: string): Promise<{ done_count: number; total_items: number; completion: number }> {
  const today = new Date().toISOString().slice(0, 10);
  const res = await fetch(
    `${BASE_URL}/api/tracker/progress?student_id=${STUDENT_ID}&start_date=${today}&end_date=${today}`,
    { headers: { Cookie: cookie } },
  );
  const stats = await res.json();
  return Array.isArray(stats) && stats.length > 0
    ? stats[0]
    : { done_count: 0, total_items: 0, completion: 0 };
}

test.describe('Tracker progress after complete/uncomplete', () => {

  test('progress changes by exactly 1 when toggling task completion', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Clean up any existing items for this student
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);

    // Create two tasks
    const task1 = await createTask(cookie, 'Progress Test Task A');
    const task2 = await createTask(cookie, 'Progress Test Task B');
    expect(task1.ok).toBe(true);
    expect(task2.ok).toBe(true);

    // Baseline: 2 expected, 0 done
    const baseline = await getProgress(cookie);
    expect(baseline.total_items).toBeGreaterThanOrEqual(2);
    const baseTotal = baseline.total_items;
    const baseDone = baseline.done_count;

    // Complete task 1 — done should increase by 1, total stays the same
    await completeTask(cookie, task1.id, true);
    const afterComplete = await getProgress(cookie);
    expect(afterComplete.total_items).toBe(baseTotal);
    expect(afterComplete.done_count).toBe(baseDone + 1);

    // Uncomplete task 1 — done should decrease by 1, back to baseline
    await completeTask(cookie, task1.id, false);
    const afterUncomplete = await getProgress(cookie);
    expect(afterUncomplete.total_items).toBe(baseTotal);
    expect(afterUncomplete.done_count).toBe(baseDone);

    // Complete both tasks — done should increase by 2
    await completeTask(cookie, task1.id, true);
    await completeTask(cookie, task2.id, true);
    const afterBoth = await getProgress(cookie);
    expect(afterBoth.total_items).toBe(baseTotal);
    expect(afterBoth.done_count).toBe(baseDone + 2);

    // Clean up
    await clearStudentTrackerItemsViaAPI(cookie, STUDENT_ID);
  });
});
