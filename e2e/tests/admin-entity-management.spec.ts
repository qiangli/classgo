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

// ==================== Admin Add Student ====================

test.describe('Admin add student', () => {

  test('admin can create a new student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const studentId = `E2E-S-${stamp}`;

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: {
        id: studentId,
        first_name: 'E2EStudent',
        last_name: stamp,
        grade: '8',
        school: 'Test School',
        active: true,
      },
    });
    expect(data.ok).toBe(true);

    // Verify in directory (JSON uses lowercase field names from Go json tags)
    const dir = await getDirectory(cookie);
    const found = dir.students.find((s: any) => s.id === studentId);
    expect(found).toBeTruthy();
    expect(found.first_name).toBe('E2EStudent');
    expect(found.grade).toBe('8');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'students', id: studentId });
  });

  test('admin student creation rejects missing name', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: { id: 'E2E-BAD', first_name: '', last_name: '' },
    });
    expect(data.ok).toBe(false);
    expect(data.error).toContain('required');
  });

  test('admin student creation rejects missing ID', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: { first_name: 'No', last_name: 'ID' },
    });
    expect(data.ok).toBe(false);
    expect(data.error).toContain('ID');
  });
});

// ==================== Admin Edit Student ====================

test.describe('Admin edit student', () => {

  test('admin can update student fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const studentId = `E2E-SEDIT-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: { id: studentId, first_name: 'EditMe', last_name: 'Student', grade: '7' },
    });

    // Update
    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: { id: studentId, first_name: 'Edited', last_name: 'Student', grade: '9', school: 'Updated School' },
    });
    expect(data.ok).toBe(true);

    // Verify
    const dir = await getDirectory(cookie);
    const found = dir.students.find((s: any) => s.id === studentId);
    expect(found).toBeTruthy();
    expect(found.first_name).toBe('Edited');
    expect(found.grade).toBe('9');
    expect(found.school).toBe('Updated School');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'students', id: studentId });
  });

  test('admin can update student via profile API with finalize', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Use existing student S005
    const res = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S005', first_name: 'Emma', last_name: 'Taylor', school: 'Finalized School', parent_id: '' },
        finalize: true,
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);

    // Verify status is final
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S005`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.ok).toBe(true);
    expect(profile.student.profile_status).toBe('final');
    expect(profile.student.school).toBe('Finalized School');
  });
});

// ==================== Admin Add Parent ====================

test.describe('Admin add parent', () => {

  test('admin can create a new parent', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const parentId = `E2E-P-${stamp}`;

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: {
        id: parentId,
        first_name: 'E2EParent',
        last_name: stamp,
        email: `parent-${stamp}@test.com`,
        phone: '555-0001',
      },
    });
    expect(data.ok).toBe(true);

    // Verify in directory
    const dir = await getDirectory(cookie);
    const found = dir.parents.find((p: any) => p.id === parentId);
    expect(found).toBeTruthy();
    expect(found.first_name).toBe('E2EParent');
    expect(found.email).toBe(`parent-${stamp}@test.com`);

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'parents', id: parentId });
  });

  test('admin parent creation rejects missing name', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: { id: 'E2E-BAD-P', first_name: '', last_name: '' },
    });
    expect(data.ok).toBe(false);
  });
});

// ==================== Admin Edit Parent ====================

test.describe('Admin edit parent', () => {

  test('admin can update parent fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const parentId = `E2E-PEDIT-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: { id: parentId, first_name: 'EditParent', last_name: 'Before', email: 'before@test.com' },
    });

    // Update
    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: {
        id: parentId,
        first_name: 'EditParent',
        last_name: 'After',
        email: 'after@test.com',
        phone: '555-9999',
        email2: 'secondary@test.com',
        phone2: '555-8888',
        address: '123 Updated St',
      },
    });
    expect(data.ok).toBe(true);

    // Verify
    const dir = await getDirectory(cookie);
    const found = dir.parents.find((p: any) => p.id === parentId);
    expect(found).toBeTruthy();
    expect(found.last_name).toBe('After');
    expect(found.email).toBe('after@test.com');
    expect(found.phone).toBe('555-9999');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'parents', id: parentId });
  });
});

// ==================== Admin Add Teacher ====================

test.describe('Admin add teacher', () => {

  test('admin can create a new teacher', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const teacherId = `E2E-T-${stamp}`;

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: {
        id: teacherId,
        first_name: 'E2ETeacher',
        last_name: stamp,
        email: `teacher-${stamp}@test.com`,
        phone: '555-0002',
        subjects: 'Math;Science',
        active: true,
      },
    });
    expect(data.ok).toBe(true);

    // Verify in directory (subjects split by semicolons into JSON array)
    const dir = await getDirectory(cookie);
    const found = dir.teachers.find((t: any) => t.id === teacherId);
    expect(found).toBeTruthy();
    expect(found.first_name).toBe('E2ETeacher');
    expect(found.subjects).toContain('Math');
    expect(found.subjects).toContain('Science');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'teachers', id: teacherId });
  });

  test('admin teacher creation rejects missing name', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: { id: 'E2E-BAD-T', first_name: '', last_name: '' },
    });
    expect(data.ok).toBe(false);
  });
});

// ==================== Admin Edit Teacher ====================

test.describe('Admin edit teacher', () => {

  test('admin can update teacher fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const teacherId = `E2E-TEDIT-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: { id: teacherId, first_name: 'EditTeacher', last_name: 'Before', subjects: 'Math' },
    });

    // Update
    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: {
        id: teacherId,
        first_name: 'EditTeacher',
        last_name: 'After',
        email: 'updated@test.com',
        phone: '555-7777',
        address: '456 Teacher Blvd',
        subjects: 'Math;English;History',
      },
    });
    expect(data.ok).toBe(true);

    // Verify
    const dir = await getDirectory(cookie);
    const found = dir.teachers.find((t: any) => t.id === teacherId);
    expect(found).toBeTruthy();
    expect(found.last_name).toBe('After');
    expect(found.email).toBe('updated@test.com');
    expect(found.subjects).toContain('Math');
    expect(found.subjects).toContain('English');
    expect(found.subjects).toContain('History');

    // Cleanup
    await dataCRUD(cookie, { action: 'delete', type: 'teachers', id: teacherId });
  });
});

// ==================== Admin Soft Delete ====================

test.describe('Admin soft delete', () => {

  test('admin can soft-delete a student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const studentId = `E2E-SDEL-${stamp}`;

    // Create
    await dataCRUD(cookie, {
      action: 'save',
      type: 'students',
      data: { id: studentId, first_name: 'DeleteMe', last_name: 'Student' },
    });

    // Delete
    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'students', id: studentId });
    expect(data.ok).toBe(true);

    // Should not appear in active directory
    const dir = await getDirectory(cookie);
    const found = dir.students.find((s: any) => s.id === studentId);
    expect(found).toBeUndefined();

    // Should appear in directory with include_deleted
    const dirAll = await getDirectory(cookie, true);
    const foundDeleted = dirAll.students.find((s: any) => s.id === studentId);
    expect(foundDeleted).toBeTruthy();
  });

  test('admin can soft-delete a parent', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const parentId = `E2E-PDEL-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'parents',
      data: { id: parentId, first_name: 'DeleteMe', last_name: 'Parent' },
    });

    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'parents', id: parentId });
    expect(data.ok).toBe(true);

    const dir = await getDirectory(cookie);
    const found = dir.parents.find((p: any) => p.id === parentId);
    expect(found).toBeUndefined();
  });

  test('admin can soft-delete a teacher', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const teacherId = `E2E-TDEL-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'teachers',
      data: { id: teacherId, first_name: 'DeleteMe', last_name: 'Teacher' },
    });

    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'teachers', id: teacherId });
    expect(data.ok).toBe(true);

    const dir = await getDirectory(cookie);
    const found = dir.teachers.find((t: any) => t.id === teacherId);
    expect(found).toBeUndefined();
  });

  test('delete rejects missing ID', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'students', id: '' });
    expect(data.ok).toBe(false);
  });

  test('delete rejects unknown entity type', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, { action: 'delete', type: 'widgets', id: 'W001' });
    expect(data.ok).toBe(false);
  });
});

// ==================== Admin Password Reset (extended) ====================

test.describe('Admin password reset for all entity types', () => {

  test('admin can reset parent password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Use P002 to avoid interfering with P001 login tests
    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ type: 'parents', id: 'P002', password: 'newpass1234' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.username).toBeTruthy();
  });

  test('admin can reset teacher password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Use T02 to avoid interfering with T01 login tests
    const res = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ type: 'teachers', id: 'T02', password: 'newpass1234' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.username).toBeTruthy();
  });

  test('password reset allows subsequent login with new password', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const newPass = `reset-${Date.now() % 10000}`;

    // Reset S006's password
    const resetRes = await fetch(`${BASE_URL}/api/v1/password-reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ type: 'students', id: 'S006', password: newPass }),
    });
    expect((await resetRes.json()).ok).toBe(true);

    // Login with new password
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S006', password: newPass, action: 'login' }),
      redirect: 'manual',
    });
    const loginData = await loginRes.json();
    expect(loginData.ok).toBe(true);
  });
});

// ==================== Admin Data CRUD Validation ====================

test.describe('Admin data CRUD validation', () => {

  test('save rejects unknown action', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, { action: 'unknown', type: 'students', data: {} });
    expect(data.ok).toBe(false);
  });

  test('save rejects unknown entity type', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const { data } = await dataCRUD(cookie, {
      action: 'save',
      type: 'unknown',
      data: { id: 'X001', first_name: 'Test', last_name: 'Test' },
    });
    expect(data.ok).toBe(false);
  });

  test('save rejects empty data', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/data`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ action: 'save', type: 'students' }),
    });
    const data = await res.json();
    expect(data.ok).toBe(false);
  });
});

// ==================== Admin Student Profile Page ====================

test.describe('Admin student profile page', () => {

  test('admin profile page loads for existing student', async ({ adminPage }) => {
    await adminPage.goto(`${BASE_URL}/admin/profile?id=S001`);
    await expect(adminPage.locator('body')).toBeVisible();
    // Page should not error
    await adminPage.waitForLoadState('networkidle');
  });

  test('admin profile API returns student and parent data', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/student/profile?id=S001`, {
      headers: { Cookie: cookie },
    });
    const data = await res.json();
    expect(data.ok).toBe(true);
    expect(data.student).toBeTruthy();
    expect(data.student.id).toBe('S001');
    expect(data.student.first_name).toBe('Alice');
    // S001 has parent P001
    expect(data.parent).toBeTruthy();
    expect(data.parent.id).toBe('P001');
  });

  test('admin profile API rejects missing student ID', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(400);
  });

  test('admin profile API returns 404 for unknown student', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/student/profile?id=S999`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(404);
  });

  test('admin can save student profile via admin API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `AdminSave-${Date.now() % 10000}`;

    const res = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S005', first_name: 'Emma', last_name: 'Taylor', notes: stamp, parent_id: '' },
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);

    // Verify
    const getRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S005`, {
      headers: { Cookie: cookie },
    });
    const profile = await getRes.json();
    expect(profile.student.notes).toBe(stamp);
  });

  test('admin can save parent info via admin profile API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `admin-parent-${Date.now() % 10000}`;

    // Must include parent_id in student data to keep the link
    const res = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S001', first_name: 'Alice', last_name: 'Wang', parent_id: 'P001' },
        parent: { id: 'P001', first_name: 'Wei', last_name: 'Wang', address: stamp },
      }),
    });
    const data = await res.json();
    expect(data.ok).toBe(true);

    // Verify parent was updated
    const getRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S001`, {
      headers: { Cookie: cookie },
    });
    const profile = await getRes.json();
    expect(profile.parent).toBeTruthy();
    expect(profile.parent.address).toBe(stamp);
  });
});
