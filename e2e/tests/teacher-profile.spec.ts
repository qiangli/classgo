import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const TEACHER_ID = 'T01';   // Sarah Smith
const TEACHER_2 = 'T02';    // James Johnson
const STUDENT_ID = 'S001';  // Alice Wang
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

test.describe('Teacher profile', () => {

  test('teacher can load own profile', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.teacher).toBeTruthy();
    expect(data.teacher.id).toBe(TEACHER_ID);
    expect(data.teacher.first_name).toBe('Sarah');
    expect(data.teacher.last_name).toBe('Smith');
    expect(data.teacher.email).toBeTruthy();
    expect(data.teacher.phone).toBeTruthy();
  });

  test('teacher profile returns student field alias', async () => {
    // The API returns teacher data as both "teacher" and "student" keys
    // so the frontend profile fields (s-first_name etc.) work without changes
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.student).toBeTruthy();
    expect(data.student.first_name).toBe('Sarah');
  });

  test('teacher can update own profile', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const stamp = `555-${Date.now() % 10000}`;
    const { data: saveResult } = await saveProfile(cookie!, {
      student: { id: TEACHER_ID, first_name: 'Sarah', last_name: 'Smith', email: 'smith@example.com', phone: stamp, address: '123 Main St' },
    });
    expect(saveResult.ok).toBe(true);

    // Verify the update persisted
    const { data: profile } = await getProfile(cookie!);
    expect(profile.ok).toBe(true);
    expect(profile.teacher.phone).toBe(stamp);
    expect(profile.teacher.address).toBe('123 Main St');
  });

  test('teacher profile does not return student-specific fields', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);

    // Teacher profile should not have tracker items, children, or empty-profile flag
    expect(data.tracker_items).toBeUndefined();
    expect(data.tracker_values).toBeUndefined();
    expect(data.children).toBeUndefined();
    expect(data.is_empty_profile).toBeUndefined();
  });

  test('teacher profile ignores student_id query param', async () => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Even if student_id is passed, teacher should get their own profile
    const { data } = await getProfile(cookie!, STUDENT_ID);
    expect(data.ok).toBe(true);
    expect(data.teacher.id).toBe(TEACHER_ID);
  });

  test('teacher profile page renders without errors', async ({ page }) => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await page.context().addCookies([{
      name: 'classgo_session',
      value: cookie!.replace('classgo_session=', ''),
      url: BASE_URL,
    }]);

    // Collect console errors
    const errors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') errors.push(msg.text());
    });

    // Track failed network requests to the profile API
    const failedRequests: string[] = [];
    page.on('response', res => {
      if (res.url().includes('/api/v1/user/profile') && res.status() >= 400) {
        failedRequests.push(`${res.status()} ${res.url()}`);
      }
    });

    await page.goto(`${BASE_URL}/profile`);
    await page.waitForLoadState('networkidle');

    // No 400 errors on the profile API
    expect(failedRequests).toHaveLength(0);

    // Profile page should show the teacher's name
    await expect(page.locator('text=Sarah Smith').first()).toBeVisible();

    // Student-only sections should not appear for teachers
    await expect(page.locator('text=High School Education')).not.toBeVisible();
    await expect(page.locator('text=Parent / Guardian Information')).not.toBeVisible();
  });

  test('second teacher can also load profile', async () => {
    const cookie = await userLogin(TEACHER_2, PASSWORD);
    expect(cookie).toBeTruthy();

    const { data } = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.teacher.id).toBe(TEACHER_2);
    expect(data.teacher.first_name).toBe('James');
    expect(data.teacher.last_name).toBe('Johnson');
  });
});
