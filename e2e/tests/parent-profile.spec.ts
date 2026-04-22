import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const PARENT_ID = 'P001';  // Wei Wang
const CHILD_1 = 'S001';    // Alice Wang
const CHILD_2 = 'S002';    // Bob Wang
const PASSWORD = 'test1234';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

async function getProfile(cookie: string, studentId = '') {
  const url = `${BASE_URL}/api/v1/user/profile` + (studentId ? `?student_id=${studentId}` : '');
  const res = await fetch(url, { headers: { Cookie: cookie } });
  return res.json();
}

async function saveProfile(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/user/profile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return res.json();
}

test.describe('Parent profile', () => {

  test('parent can load profile without specifying student_id', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Load without student_id — should resolve to first child
    const data = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.student).toBeTruthy();
    expect(data.student.id).toBeTruthy();
    expect(data.parent).toBeTruthy();
    expect(data.parent.id).toBe(PARENT_ID);
  });

  test('parent can access both children profiles', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const data1 = await getProfile(cookie!, CHILD_1);
    expect(data1.ok).toBe(true);
    expect(data1.student.id).toBe(CHILD_1);
    expect(data1.student.first_name).toBe('Alice');

    const data2 = await getProfile(cookie!, CHILD_2);
    expect(data2.ok).toBe(true);
    expect(data2.student.id).toBe(CHILD_2);
    expect(data2.student.first_name).toBe('Bob');
  });

  test('children list is returned for parent', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const data = await getProfile(cookie!);
    expect(data.ok).toBe(true);
    expect(data.children).toBeTruthy();
    expect(data.children.length).toBeGreaterThanOrEqual(2);

    const ids = data.children.map((c: any) => c.id);
    expect(ids).toContain(CHILD_1);
    expect(ids).toContain(CHILD_2);
  });

  test('parent info is shared across siblings', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Save parent phone via child 1's profile
    const stamp = `555-${Date.now() % 10000}`;
    const profile1 = await getProfile(cookie!, CHILD_1);
    const result = await saveProfile(cookie!, {
      student: { id: CHILD_1, parent_id: PARENT_ID },
      parent: { id: PARENT_ID, ...profile1.parent, phone: stamp },
      tracker_values: {},
    });
    expect(result.ok).toBe(true);

    // Load child 2's profile — parent phone should match
    const profile2 = await getProfile(cookie!, CHILD_2);
    expect(profile2.ok).toBe(true);
    expect(profile2.parent.phone).toBe(stamp);
  });

  test('parent cannot access unrelated student', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // S010 (Jack Brown) is not P001's child
    const data = await getProfile(cookie!, 'S010');
    expect(data.ok).toBe(false);
  });

  test('parent saves child info independently from parent info', async () => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Get current state
    const before = await getProfile(cookie!, CHILD_1);
    const origParentPhone = before.parent.phone;

    // Save only child data (empty parent object) — should not blank parent fields
    const result = await saveProfile(cookie!, {
      student: { id: CHILD_1, first_name: 'Alice', last_name: 'Wang', parent_id: PARENT_ID },
      parent: {},
      tracker_values: {},
    });
    expect(result.ok).toBe(true);

    // Verify parent phone is unchanged
    const after = await getProfile(cookie!, CHILD_1);
    expect(after.parent.phone).toBe(origParentPhone);
  });
});
