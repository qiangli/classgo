import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S003'; // Carlos Garcia — use a less-used student to avoid test conflicts
const STUDENT_2 = 'S004';  // Diana Chen
const PARENT_ID = 'P001';  // Wei Wang (parent of S001, S002)
const PASSWORD = 'test1234';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

async function getProfile(cookie: string, studentId = '') {
  const url = `${BASE_URL}/api/v1/user/profile` + (studentId ? `?student_id=${studentId}` : '');
  const res = await fetch(url, { headers: { Cookie: cookie } });
  return { status: res.status, data: await res.json() };
}

async function saveProfile(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

// ==================== Student Profile View ====================

test.describe('Student profile view', () => {

  test('student can load own profile', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.student).toBeTruthy();
    expect(data.student.id).toBe(STUDENT_ID);
    expect(data.student.first_name).toBe('Carlos');
    expect(data.student.last_name).toBe('Garcia');
  });

  test('student profile includes parent data when linked', async () => {
    // S001 has parent P001 — but check that the student actually has a parent_id set
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    // Parent may be null if parent_id was cleared by another test
    if (data.student.parent_id) {
      expect(data.parent).toBeTruthy();
      expect(data.parent.id).toBe(data.student.parent_id);
    }
  });

  test('student profile includes tracker items and values', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.tracker_items).toBeDefined();
    expect(data.tracker_values).toBeDefined();
  });

  test('student profile includes is_empty_profile flag', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.is_empty_profile).toBeDefined();
  });

  test('student does not get children list', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    // children is only for parents
    expect(data.children).toBeNull();
  });
});

// ==================== Student Profile Edit ====================

test.describe('Student profile edit', () => {

  test('student can update own profile fields', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const stamp = `School-${Date.now() % 10000}`;
    const { data: saveResult } = await saveProfile(cookie!, {
      student: {
        id: STUDENT_ID,
        first_name: 'Carlos',
        last_name: 'Garcia',
        school: stamp,
        grade: '10',
      },
      tracker_values: {},
    });
    expect(saveResult.ok).toBe(true);

    // Verify persistence
    const { data: profile } = await getProfile(cookie!);
    expect(profile.ok).toBe(true);
    expect(profile.student.school).toBe(stamp);
    expect(profile.student.grade).toBe('10');
  });

  test('student can update personal info fields', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const stamp = `${Date.now() % 10000}`;
    const { data: saveResult } = await saveProfile(cookie!, {
      student: {
        id: STUDENT_ID,
        first_name: 'Carlos',
        last_name: 'Garcia',
        dob: '2010-05-15',
        birthplace: `TestCity-${stamp}`,
        first_language: 'Spanish',
        years_in_us: '5',
      },
      tracker_values: {},
    });
    expect(saveResult.ok).toBe(true);

    const { data: profile } = await getProfile(cookie!);
    expect(profile.ok).toBe(true);
    expect(profile.student.dob).toBe('2010-05-15');
    expect(profile.student.birthplace).toContain(stamp);
    expect(profile.student.first_language).toBe('Spanish');
    expect(profile.student.years_in_us).toBe('5');
  });

  test('student profile save sets draft status', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await saveProfile(cookie!, {
      student: { id: STUDENT_ID, first_name: 'Carlos', last_name: 'Garcia' },
      tracker_values: {},
    });

    const { data: profile } = await getProfile(cookie!);
    expect(profile.ok).toBe(true);
    expect(profile.student.profile_status).toBe('draft');
  });

  test('student can save parent info through profile', async () => {
    // S001 (Alice) has parent P001
    const cookie = await userLogin('S001', PASSWORD);
    expect(cookie).toBeTruthy();

    const { data: before } = await getProfile(cookie!);
    // Skip if no parent linked
    if (!before.parent || !before.parent.id) return;

    const stamp = `parent-${Date.now() % 10000}@test.com`;

    const { data: saveResult } = await saveProfile(cookie!, {
      student: { id: 'S001', first_name: 'Alice', last_name: 'Wang', parent_id: before.parent.id },
      parent: { id: before.parent.id, first_name: before.parent.first_name, last_name: before.parent.last_name, email: stamp },
      tracker_values: {},
    });
    expect(saveResult.ok).toBe(true);

    const { data: after } = await getProfile(cookie!);
    expect(after.parent.email).toBe(stamp);
  });

  test('student save rejects missing student ID', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await saveProfile(cookie!, {
      student: { first_name: 'Carlos', last_name: 'Garcia' },
      tracker_values: {},
    });
    expect(data.ok).toBe(false);
  });
});

// ==================== Student Profile Access Control ====================

test.describe('Student profile access control', () => {

  test('student cannot access another student profile', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Try to access S001 (Alice) — should be forbidden
    const { status, data } = await getProfile(cookie!, 'S001');
    // Student profile resolution ignores student_id param — always returns own
    // The resolveStudentID for students always returns sess.EntityID
    expect(data.ok).toBe(true);
    expect(data.student.id).toBe(STUDENT_ID); // Returns own profile, not S001
  });

  test('student cannot save to another student profile', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await saveProfile(cookie!, {
      student: { id: 'S001', first_name: 'Hacked', last_name: 'Name' },
      tracker_values: {},
    });
    // Should be forbidden — student can only save own profile
    expect(data.ok).toBe(false);
  });

  test('parent cannot access unrelated student profile', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // S003 (Carlos) is not P001's child
    const { data } = await getProfile(cookie!, STUDENT_ID);
    expect(data.ok).toBe(false);
  });

  test('parent cannot save to unrelated student profile', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await saveProfile(cookie!, {
      student: { id: STUDENT_ID, first_name: 'Hacked', last_name: 'Name' },
      tracker_values: {},
    });
    expect(data.ok).toBe(false);
  });
});

// ==================== Student Profile UI ====================

test.describe('Student profile UI', () => {

  test('profile page loads with student data', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);

    await expect(page.locator('h1')).toContainText('My Profile');
    await expect(page.locator('text=Personal Information')).toBeVisible();
  });

  test('profile page shows student name in header', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);
    await expect(page.locator('header')).toContainText('Carlos');
  });

  test('student profile page does not show child selector', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);
    await page.waitForLoadState('networkidle');

    // Child selector is only for parents
    await expect(page.locator('#profile-child-select')).toHaveCount(0);
  });
});
