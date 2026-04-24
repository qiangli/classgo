/**
 * E2E tests for profile finalization lifecycle.
 *
 * Verifies the profile_status transitions (draft → final), that
 * student saves produce draft status, and that profile fields are
 * correctly preserved across updates.
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

test.describe('Profile status lifecycle', () => {

  test('student save always sets draft status', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const studentCookie = await userLogin('S002', PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Student saves their own profile
    const saveRes = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student: { id: 'S002', school: 'Updated School' },
        parent: {},
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    // Admin checks the profile status — should be draft
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S002`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.student.profile_status).toBe('draft');
  });

  test('admin can finalize a draft profile', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Save as draft first
    await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S002' },
        parent: {},
        finalize: false,
      }),
    });

    // Finalize
    const finalRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S002' },
        parent: {},
        finalize: true,
      }),
    });
    expect((await finalRes.json()).ok).toBe(true);

    // Verify final status
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S002`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.student.profile_status).toBe('final');
  });

  test('student save after finalization resets to draft', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const studentCookie = await userLogin('S002', PASSWORD);
    expect(studentCookie).toBeTruthy();

    // Ensure profile is final
    await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S002' },
        parent: {},
        finalize: true,
      }),
    });

    // Student saves again — status should revert to draft
    const saveRes = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({
        student: { id: 'S002', notes: 'Updated notes' },
        parent: {},
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    // Verify draft status
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S002`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.student.profile_status).toBe('draft');
  });
});

test.describe('Profile field preservation', () => {

  test('profile save preserves all student fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const studentData = {
      id: 'S004',
      grade: '5',
      school: 'Lincoln Elementary',
      birthplace: 'Test City',
      years_in_us: '5',
      first_language: 'English',
      previous_schools: 'Old School',
      courses_outside: 'Piano',
    };

    const saveRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: studentData,
        parent: {},
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    // Read back and verify
    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S004`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.student.grade).toBe('5');
    expect(profile.student.school).toBe('Lincoln Elementary');
    expect(profile.student.birthplace).toBe('Test City');
    expect(profile.student.years_in_us).toBe('5');
    expect(profile.student.first_language).toBe('English');
    expect(profile.student.previous_schools).toBe('Old School');
    expect(profile.student.courses_outside).toBe('Piano');
  });

  test('profile save preserves parent fields', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // S004 (Diana Chen) has parent_id P003 (David Chen)
    // Save with parent fields including secondary contact
    const saveRes = await fetch(`${BASE_URL}/api/v1/student/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({
        student: { id: 'S004', parent_id: 'P003' },
        parent: {
          id: 'P003',
          first_name: 'David',
          last_name: 'Chen',
          email: 'david@example.com',
          phone: '555-9012',
          email2: 'mei.chen@example.com',
          phone2: '555-9013',
          address: '123 Test Ave',
        },
      }),
    });
    expect((await saveRes.json()).ok).toBe(true);

    const profileRes = await fetch(`${BASE_URL}/api/v1/student/profile?id=S004`, {
      headers: { Cookie: cookie },
    });
    const profile = await profileRes.json();
    expect(profile.parent).toBeTruthy();
    expect(profile.parent.email).toBe('david@example.com');
    expect(profile.parent.phone).toBe('555-9012');
  });

  test('student can view own profile via user endpoint', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      headers: { Cookie: studentCookie! },
    });
    expect(res.status).toBe(200);
    const profile = await res.json();
    expect(profile.ok).toBe(true);
    expect(profile.student).toBeTruthy();
    expect(profile.student.id).toBe('S001');
    expect(profile.student.first_name).toBe('Alice');
  });

  test('teacher can view own profile', async () => {
    const teacherCookie = await userLogin('T01', PASSWORD);
    expect(teacherCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      headers: { Cookie: teacherCookie! },
    });
    expect(res.status).toBe(200);
    const profile = await res.json();
    expect(profile.ok).toBe(true);
    expect(profile.teacher).toBeTruthy();
    expect(profile.teacher.id).toBe('T01');
  });

  test('parent can view profile and sees children list', async () => {
    const parentCookie = await userLogin('P001', PASSWORD);
    expect(parentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
      headers: { Cookie: parentCookie! },
    });
    expect(res.status).toBe(200);
    const profile = await res.json();
    expect(profile.ok).toBe(true);
    // Parent should see children (S001 Alice, S002 Bob)
    expect(profile.children).toBeTruthy();
    expect(Array.isArray(profile.children)).toBe(true);
    expect(profile.children.length).toBeGreaterThanOrEqual(1);
  });
});
