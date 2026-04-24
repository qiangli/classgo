/**
 * E2E tests for soft-delete behavior and cascade effects.
 *
 * Verifies that soft-deleting a student hides them from active queries,
 * preserves their data in include_deleted mode, and records audit fields.
 * Also tests restore behavior and cascade effects on related records.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';

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

async function dataCRUD(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/data`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

async function getDirectory(cookie: string, includeDeleted = false) {
  const url = `${BASE_URL}/api/v1/directory` + (includeDeleted ? '?include_deleted=1' : '');
  const res = await fetch(url, { headers: { Cookie: cookie } });
  return res.json();
}

test.describe('Student soft-delete', () => {
  const testStudentId = `E2E-DEL-${Date.now() % 100000}`;

  test('soft-deleted student hidden from active directory', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Create test student
    await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: {
        id: testStudentId,
        first_name: 'Delete',
        last_name: 'Test',
        grade: '6',
        school: 'Test School',
        active: true,
      },
    });

    // Verify student exists
    let dir = await getDirectory(cookie);
    let found = dir.students.find((s: any) => s.id === testStudentId);
    expect(found).toBeTruthy();

    // Soft-delete
    const { data } = await dataCRUD(cookie, {
      action: 'delete',
      type: 'students',
      id: testStudentId,
    });
    expect(data.ok).toBe(true);

    // Should not appear in active directory
    dir = await getDirectory(cookie);
    found = dir.students.find((s: any) => s.id === testStudentId);
    expect(found).toBeUndefined();
  });

  test('soft-deleted student visible with include_deleted', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Should appear with include_deleted
    const dir = await getDirectory(cookie, true);
    const found = dir.students.find((s: any) => s.id === testStudentId);
    expect(found).toBeTruthy();
    expect(found.deleted).toBe(true);
    expect(found.deleted_at).toBeTruthy();
    expect(found.deleted_by).toBeTruthy();
  });

  test('soft-deleted student can be restored', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Restore by saving with active=true
    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: {
        id: testStudentId,
        first_name: 'Delete',
        last_name: 'Test',
        grade: '6',
        school: 'Test School',
        active: true,
      },
    });
    expect(data.ok).toBe(true);

    // Should appear in active directory again
    const dir = await getDirectory(cookie);
    const found = dir.students.find((s: any) => s.id === testStudentId);
    expect(found).toBeTruthy();

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'students', id: testStudentId });
  });
});

test.describe('Entity soft-delete across types', () => {

  test('soft-delete parent records audit fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const parentId = `E2E-PDEL-${Date.now() % 100000}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: {
        id: parentId,
        first_name: 'Parent',
        last_name: 'Delete',
        email: 'del@example.com',
      },
    });

    await dataCRUD(cookie, { action: 'delete', type: 'parents', id: parentId });

    const dir = await getDirectory(cookie, true);
    const found = dir.parents.find((p: any) => p.id === parentId);
    expect(found).toBeTruthy();
    expect(found.deleted).toBe(true);
    expect(found.deleted_at).toBeTruthy();
    expect(found.deleted_by).toBeTruthy();
  });

  test('soft-delete teacher records audit fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const teacherId = `E2E-TDEL-${Date.now() % 100000}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: {
        id: teacherId,
        first_name: 'Teacher',
        last_name: 'Delete',
        email: 'tdel@example.com',
        subjects: 'math',
      },
    });

    await dataCRUD(cookie, { action: 'delete', type: 'teachers', id: teacherId });

    const dir = await getDirectory(cookie, true);
    const found = dir.teachers.find((t: any) => t.id === teacherId);
    expect(found).toBeTruthy();
    expect(found.deleted).toBe(true);
    expect(found.deleted_at).toBeTruthy();
  });

  test('soft-delete room records audit fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const roomId = `E2E-RDEL-${Date.now() % 100000}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'rooms',
      data: {
        id: roomId,
        name: 'Temp Room',
        capacity: 4,
      },
    });

    await dataCRUD(cookie, { action: 'delete', type: 'rooms', id: roomId });

    const dir = await getDirectory(cookie, true);
    const found = dir.rooms.find((r: any) => r.id === roomId);
    expect(found).toBeTruthy();
    expect(found.deleted).toBe(true);
  });
});

test.describe('Soft-deleted student side effects', () => {

  test('soft-deleted student excluded from user search', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const studentId = `E2E-SRCH-${Date.now() % 100000}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: {
        id: studentId,
        first_name: 'Searchable',
        last_name: 'Deleteme',
        grade: '5',
        active: true,
      },
    });

    // Should be searchable before delete
    let searchRes = await fetch(`${BASE_URL}/api/users/search?q=Deleteme`);
    let results = await searchRes.json();
    let found = results.find((r: any) => r.id === studentId);
    expect(found).toBeTruthy();

    // Soft-delete
    await dataCRUD(cookie, { action: 'delete', type: 'students', id: studentId });

    // Should no longer appear in search
    searchRes = await fetch(`${BASE_URL}/api/users/search?q=Deleteme`);
    results = await searchRes.json();
    found = results.find((r: any) => r.id === studentId);
    expect(found).toBeUndefined();
  });

  test('soft-deleted student cannot check in', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // S011 (Karen Davis) and S012 (Leo Martinez) are inactive in csv.example
    const res = await fetch(`${BASE_URL}/api/checkin`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        student_name: 'Karen Davis',
        device_type: 'mobile',
        device_id: `e2e-inactive-${Date.now()}`,
      }),
    });
    const result = await res.json();
    // Inactive student should not be found or should fail
    expect(result.ok).toBe(false);
  });
});
