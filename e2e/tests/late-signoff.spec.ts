/**
 * E2E tests for the late signoff endpoint.
 *
 * Verifies that teachers and admins can record late signoffs for
 * tracker items on past dates, and that students cannot.
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

test.describe('Late signoff', () => {
  // Test data from csv.example:
  // T01 (Sarah Smith) teaches S001 (Alice Wang), S002 (Bob Wilson)
  // T02 (James Johnson) teaches S003 (Carlos Garcia), S004 (Diana Chen)

  test('teacher can record late signoff for own student', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // First create a global signoff task as admin
    const itemRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        name: `Late Signoff Test ${Date.now()}`,
        priority: 'medium',
        recurrence: 'daily',
        requires_signoff: true,
        active: true,
      }),
    });
    const item = await itemRes.json();
    expect(item.ok).toBe(true);

    // Login as teacher T01
    const teacherCookie = await userLogin('T01', PASSWORD);
    expect(teacherCookie).toBeTruthy();

    // Record late signoff for yesterday
    const yesterday = new Date(Date.now() - 86400000).toISOString().split('T')[0];
    const signoffRes = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: teacherCookie! },
      body: JSON.stringify({
        student_id: 'S001',
        item_id: item.id,
        due_date: yesterday,
        status: 'done',
        notes: 'Late completion recorded by teacher',
      }),
    });
    const signoffData = await signoffRes.json();
    expect(signoffData.ok).toBe(true);

    // Cleanup: delete the item
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ id: item.id }),
    });
  });

  test('admin can record late signoff', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Create a global signoff task
    const itemRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        name: `Admin Late Signoff ${Date.now()}`,
        priority: 'high',
        recurrence: 'daily',
        requires_signoff: true,
        active: true,
      }),
    });
    const item = await itemRes.json();
    expect(item.ok).toBe(true);

    // Record late signoff as admin
    const yesterday = new Date(Date.now() - 86400000).toISOString().split('T')[0];
    const res = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        student_id: 'S002',
        item_id: item.id,
        due_date: yesterday,
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);

    // Cleanup
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ id: item.id }),
    });
  });

  test('late signoff rejects missing required fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Missing student_id
    const res1 = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ item_id: 1, due_date: '2025-01-01' }),
    });
    expect(res1.status).toBe(400);

    // Missing item_id
    const res2 = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ student_id: 'S001', due_date: '2025-01-01' }),
    });
    expect(res2.status).toBe(400);

    // Missing due_date
    const res3 = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ student_id: 'S001', item_id: 1 }),
    });
    expect(res3.status).toBe(400);
  });

  test('student cannot record late signoff', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student_id: 'S001',
        item_id: 1,
        due_date: '2025-01-01',
      }),
    });
    expect(res.status).toBe(403);
  });

  test('late signoff is reflected in tracker responses', async ({ adminPage }) => {
    const adminCookie = await getAdminCookie(adminPage);

    // Create task
    const itemRes = await fetch(`${BASE_URL}/api/v1/tracker/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        name: `Response Check ${Date.now()}`,
        priority: 'medium',
        recurrence: 'daily',
        requires_signoff: true,
        active: true,
      }),
    });
    const item = await itemRes.json();

    // Record late signoff for a past due_date
    const pastDate = new Date(Date.now() - 2 * 86400000).toISOString().split('T')[0];
    await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({
        student_id: 'S001',
        item_id: item.id,
        due_date: pastDate,
        notes: 'verified later',
      }),
    });

    // The response_date defaults to today (when the signoff was recorded),
    // so query by today's date to find the late signoff record.
    const today = new Date().toISOString().split('T')[0];
    const respRes = await fetch(
      `${BASE_URL}/api/v1/tracker/responses?student_id=S001&date=${today}`,
      { headers: { Cookie: adminCookie } },
    );
    expect(respRes.status).toBe(200);
    const responses = await respRes.json();
    expect(Array.isArray(responses)).toBe(true);
    const found = responses.find((r: any) => r.item_id === item.id);
    expect(found).toBeTruthy();

    // Cleanup
    await fetch(`${BASE_URL}/api/v1/tracker/items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
      body: JSON.stringify({ id: item.id }),
    });
  });

  test('unauthenticated user cannot access late signoff', async () => {
    const res = await fetch(`${BASE_URL}/api/tracker/late-signoff`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        student_id: 'S001',
        item_id: 1,
        due_date: '2025-01-01',
      }),
      redirect: 'manual',
    });
    // RequireAuth redirects (302) or returns 401
    expect([302, 401]).toContain(res.status);
  });
});
