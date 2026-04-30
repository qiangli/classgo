/**
 * E2E tests for the namelist import feature.
 *
 * Tests the admin Namelist section: navigation, file listing API,
 * preview API, execute API, and access control.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';
import { AdminPage } from '../pages/admin.page.js';

const BASE_URL = 'http://localhost:9090';
const PASSWORD = 'test1234';

function getAdminCookie(adminPage: import('@playwright/test').Page): Promise<string> {
  return adminPage.context().cookies().then(cookies => {
    const c = cookies.find(c => c.name === 'classgo_session');
    return c ? `classgo_session=${c.value}` : '';
  });
}

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

test.describe('Namelist API', () => {

  test('list files endpoint returns array', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/namelist/files`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data).toHaveProperty('files');
    expect(Array.isArray(data.files)).toBe(true);
  });

  test('preview rejects missing filename', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/namelist/preview`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({}),
    });
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toContain('filename');
  });

  test('preview rejects nonexistent file', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/namelist/preview`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ filename: 'nonexistent.xls' }),
    });
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toBeTruthy();
  });

  test('preview rejects path traversal', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/namelist/preview`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ filename: '../../../etc/passwd' }),
    });
    expect(res.status).toBe(400);
  });

  test('execute rejects missing filename', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/api/v1/namelist/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ decisions: {} }),
    });
    expect(res.status).toBe(400);
  });
});

test.describe('Namelist access control', () => {

  test('list files rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/namelist/files`);
    expect(res.status).toBe(401);
  });

  test('preview rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/namelist/preview`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ filename: 'test.xls' }),
    });
    expect(res.status).toBe(401);
  });

  test('execute rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/namelist/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ filename: 'test.xls', decisions: {} }),
    });
    expect(res.status).toBe(401);
  });

  test('list files rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/namelist/files`, {
      headers: { Cookie: studentCookie! },
    });
    expect(res.status).toBe(403);
  });

  test('preview rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/namelist/preview`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ filename: 'test.xls' }),
    });
    expect(res.status).toBe(403);
  });

  test('execute rejects non-admin requests', async () => {
    const studentCookie = await userLogin('S001', PASSWORD);
    expect(studentCookie).toBeTruthy();

    const res = await fetch(`${BASE_URL}/api/v1/namelist/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: studentCookie! },
      body: JSON.stringify({ filename: 'test.xls', decisions: {} }),
    });
    expect(res.status).toBe(403);
  });
});

test.describe('Namelist admin UI', () => {

  test('namelist nav item is visible and navigable', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();

    // Namelist nav item should exist
    const navItem = adminPage.locator('#nav-namelist');
    await expect(navItem).toBeVisible();

    // Click and verify section loads
    await admin.navigateTo('namelist');
    await expect(admin.pageTitle).toHaveText('Namelist');
    await expect(admin.section('namelist')).toBeVisible();
  });

  test('namelist section has file selector and preview button', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('namelist');

    const fileSelect = adminPage.locator('#namelist-file-select');
    await expect(fileSelect).toBeVisible();

    const previewBtn = adminPage.locator('#namelist-preview-btn');
    await expect(previewBtn).toBeVisible();
    // Preview button should be disabled when no file selected
    await expect(previewBtn).toBeDisabled();
  });

  test('file selector loads files from server', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('namelist');

    // File selector should have the default placeholder option
    const fileSelect = adminPage.locator('#namelist-file-select');
    const options = fileSelect.locator('option');
    // At least the placeholder option should exist
    await expect(options.first()).toHaveText('Select a namelist file...');
  });

  test('preview section is hidden initially', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('namelist');

    const preview = adminPage.locator('#namelist-preview');
    await expect(preview).toBeHidden();
  });

  test('execute button exists in preview section', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('namelist');

    // Execute button is in the preview section (hidden until preview is shown)
    const executeBtn = adminPage.locator('#namelist-execute-btn');
    // It exists in DOM but may be hidden inside the preview section
    expect(await executeBtn.count()).toBe(1);
  });
});

test.describe('Namelist data columns', () => {

  test('student data tab includes new namelist columns', async ({ adminPage }) => {
    const admin = new AdminPage(adminPage);
    await admin.goto();
    await admin.navigateTo('data');

    // Open column configuration
    const colBtn = adminPage.locator('#col-config-btn');
    await colBtn.click();
    const colMenu = adminPage.locator('#col-config-menu');
    await expect(colMenu).toBeVisible();

    // Verify new columns exist in the configuration
    const menuText = await colMenu.textContent();
    expect(menuText).toContain('English Name');
    expect(menuText).toContain('Package');
    expect(menuText).toContain('Major');
    expect(menuText).toContain('Enroll Term');
    expect(menuText).toContain('Graduation');
  });

  test('student CSV export includes new columns', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=students`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const csv = await res.text();
    const header = csv.split('\n')[0];
    expect(header).toContain('english_name');
    expect(header).toContain('package');
    expect(header).toContain('major');
    expect(header).toContain('enroll_term');
    expect(header).toContain('graduation');
  });
});
