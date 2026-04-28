import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import { userLogin } from '../helpers/api.js';

const BASE_URL = 'http://localhost:9090';
const STUDENT_ID = 'S001'; // Alice Wang
const PARENT_ID = 'P001';  // Wei Wang
const TEACHER_ID = 'T01';
const PASSWORD = 'test1234';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

// ==================== Helper functions ====================

async function getPreferences(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/preferences`, {
    headers: { Cookie: cookie },
  });
  return res.json();
}

async function savePreferences(cookie: string, prefs: Record<string, string>) {
  const res = await fetch(`${BASE_URL}/api/v1/preferences`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(prefs),
  });
  return res.json();
}

async function clearHomePrefs(cookie: string) {
  await savePreferences(cookie, {
    home_favorites: JSON.stringify(['tasks', 'memos']),
    home_recent: '[]',
    home_sections: JSON.stringify({ message: true, favorites: true, recent: true, all: true }),
    home_bg_enabled: 'false',
    home_message_dismissed: 'false',
  });
}

// ==================== Routing & Redirect Tests ====================

test.describe('Home page routing', () => {

  test('unauthenticated user sees login form at /home', async ({ page }) => {
    await page.goto(`${BASE_URL}/home`);
    // Should render the entry page with login form
    await expect(page.locator('text=Check In').first()).toBeVisible();
  });

  test('login redirects to /home', async ({ page }) => {
    // First ensure user account is set up via API
    await userLogin(STUDENT_ID, PASSWORD);

    // Verify password works via API (avoid flake if env has stale password)
    const verify = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: STUDENT_ID, password: PASSWORD, action: 'login' }),
    });
    const verifyData = await verify.json();
    test.skip(!verifyData.ok, 'Password state inconsistent');

    // Now do browser-based login
    await page.goto(`${BASE_URL}/?mode=login`);
    const searchInput = page.locator('#user-search');
    await searchInput.fill('Alice');
    await page.locator('#user-results li').first().waitFor({ state: 'visible' });
    await page.locator('#user-results li').first().click();
    await page.locator('#password-section').waitFor({ state: 'visible' });
    await page.locator('#user-password').fill(PASSWORD);
    await page.locator('#login-btn').click();

    // Should redirect to /home
    await page.waitForURL('**/home');
    expect(page.url()).toContain('/home');
  });

  test('authenticated user sees home page sections', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Set cookie in browser context
    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);

    // Check that key sections exist
    await expect(page.locator('#section-favorites')).toBeVisible();
    await expect(page.locator('#section-all')).toBeVisible();

    // Check user name is displayed
    await expect(page.locator('header')).toContainText('Alice Wang');
  });

  test('admin user is redirected to /admin from /home', async ({ adminPage }) => {
    // Admin session redirects /home → /admin (302)
    // Use API check to avoid 30s timeout on browser navigation
    const cookie = await adminPage.context().cookies().then(
      cs => cs.find(c => c.name === 'classgo_session')
    );
    test.skip(!cookie, 'No admin cookie available');
    const res = await fetch(`${BASE_URL}/home`, {
      headers: { Cookie: `classgo_session=${cookie!.value}` },
      redirect: 'manual',
    });
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toBe('/admin');
  });
});

// ==================== Home Page Content Tests ====================

test.describe('Home page content', () => {

  test('home page shows app tiles in All section', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);

    const allGrid = page.locator('#all-grid');
    await expect(allGrid).toBeVisible();

    // Should have 5 app tiles: Tasks, Classes, Reports, Memos, Profile
    await expect(allGrid.locator('.app-tile-wrap')).toHaveCount(5);
    await expect(allGrid).toContainText('Tasks');
    await expect(allGrid).toContainText('Classes');
    await expect(allGrid).toContainText('Reports');
    await expect(allGrid).toContainText('Memos');
    await expect(allGrid).toContainText('Profile');
  });

  test('message section is visible by default', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('#section-message')).toBeVisible();
    await expect(page.locator('#section-message')).toContainText('Welcome');
  });

  test('default favorites show Tasks and Memos', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);

    const favGrid = page.locator('#favorites-grid');
    await expect(favGrid).toBeVisible();
    await expect(favGrid.locator('.app-tile-wrap')).toHaveCount(2);
    await expect(favGrid).toContainText('Tasks');
    await expect(favGrid).toContainText('Memos');
  });
});

// ==================== Navigation Tests ====================

test.describe('Home page navigation', () => {

  test('clicking Tasks tile navigates to /dashboard', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    // Click the Tasks tile in All section
    await page.locator('#all-grid .app-tile-wrap').filter({ hasText: 'Tasks' }).locator('.app-tile').click();

    await page.waitForURL('**/dashboard');
    expect(page.url()).toContain('/dashboard');
  });

  test('clicking Profile button navigates to /profile', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    // Click the profile button in top-right header
    await page.locator('header a[href="/profile"]').click();

    await page.waitForURL('**/profile');
    expect(page.url()).toContain('/profile');
  });

  test('dashboard has Home button in sidebar that links back to /home', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/dashboard`);

    const homeLink = page.locator('nav a[href="/home"]');
    await expect(homeLink).toBeVisible();
    await expect(homeLink).toContainText('Home');
  });

  test('dashboard sidebar no longer has Profile or Memos buttons', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/dashboard`);

    // Profile and Memos nav items should not exist
    await expect(page.locator('#nav-profile')).toHaveCount(0);
    await expect(page.locator('#nav-memos')).toHaveCount(0);
  });
});

// ==================== Preferences Persistence Tests ====================

test.describe('Home page preferences', () => {

  test('dismissing message persists across page reload', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('#section-message')).toBeVisible();

    // Dismiss the message
    await page.locator('#section-message button').click();
    await expect(page.locator('#section-message')).toBeHidden();

    // Reload and verify it stays hidden
    await page.reload();
    await expect(page.locator('#section-all')).toBeVisible(); // page loaded
    await expect(page.locator('#section-message')).toBeHidden();

    // Verify preference was saved
    const prefs = await getPreferences(cookie!);
    expect(prefs.home_message_dismissed).toBe('true');
  });

  test('toggling favorites persists via API', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    // Save custom favorites
    await savePreferences(cookie!, {
      home_favorites: JSON.stringify(['tasks', 'profile']),
    });

    // Verify preference was saved
    const prefs = await getPreferences(cookie!);
    const favorites = JSON.parse(prefs.home_favorites);
    expect(favorites).toContain('tasks');
    expect(favorites).toContain('profile');
    expect(favorites).not.toContain('memos');
  });

  test('recent apps are tracked and persisted', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    // Simulate recent app visits
    const recent = [
      { id: 'tasks', ts: Date.now() - 1000 },
      { id: 'memos', ts: Date.now() },
    ];
    await savePreferences(cookie!, {
      home_recent: JSON.stringify(recent),
    });

    // Verify
    const prefs = await getPreferences(cookie!);
    const savedRecent = JSON.parse(prefs.home_recent);
    expect(savedRecent).toHaveLength(2);
    expect(savedRecent[0].id).toBe('tasks');
    expect(savedRecent[1].id).toBe('memos');
  });

  test('section visibility persists via API', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    // Hide favorites and recent
    const sections = { message: true, favorites: false, recent: false, all: true };
    await savePreferences(cookie!, {
      home_sections: JSON.stringify(sections),
    });

    const prefs = await getPreferences(cookie!);
    const saved = JSON.parse(prefs.home_sections);
    expect(saved.favorites).toBe(false);
    expect(saved.recent).toBe(false);
    expect(saved.all).toBe(true);
  });

  test('background image preference persists', async () => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    await savePreferences(cookie!, { home_bg_enabled: 'true' });

    const prefs = await getPreferences(cookie!);
    expect(prefs.home_bg_enabled).toBe('true');

    // Disable
    await savePreferences(cookie!, { home_bg_enabled: 'false' });

    const prefs2 = await getPreferences(cookie!);
    expect(prefs2.home_bg_enabled).toBe('false');
  });
});

// ==================== Standalone Profile Tests ====================

test.describe('Standalone profile page', () => {

  test('profile page loads with user data', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);

    // Should show profile page with header
    await expect(page.locator('h1')).toContainText('My Profile');
    await expect(page.locator('header')).toContainText('Alice Wang');

    // Should have a back-to-home link
    const homeLink = page.locator('header a[href="/home"]');
    await expect(homeLink).toBeVisible();

    // Should show Personal Information section
    await expect(page.locator('text=Personal Information')).toBeVisible();
  });

  test('profile page loads for parent with child selector', async ({ page }) => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);

    // Parent should see parent/guardian section
    await expect(page.locator('text=Parent / Guardian').first()).toBeVisible();

    // Should have child selector
    await expect(page.locator('#profile-child-select')).toBeVisible();
  });

  test('profile page has back-to-home navigation', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/profile`);

    // Click the Home link
    await page.locator('header a[href="/home"]').click();
    await page.waitForURL('**/home');
    expect(page.url()).toContain('/home');
  });

  test('unauthenticated profile access redirects to login', async ({ page }) => {
    // RequireAuth redirects to /login (302), verify via API to avoid timeout
    const res = await fetch(`${BASE_URL}/profile`, { redirect: 'manual' });
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });
});

// ==================== Role-based Home Page Tests ====================

test.describe('Home page for different roles', () => {

  test('student sees correct role badge', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('header')).toContainText('student');
  });

  test('parent sees correct role badge', async ({ page }) => {
    const cookie = await userLogin(PARENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('header')).toContainText('parent');
  });

  test('teacher sees correct role badge', async ({ page }) => {
    const cookie = await userLogin(TEACHER_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('header')).toContainText('teacher');
  });
});

// ==================== Edit Mode Tests ====================

test.describe('Home page edit mode', () => {

  test('edit panel opens and closes', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);

    // Edit panel should be hidden initially
    await expect(page.locator('#edit-panel')).toBeHidden();

    // Click edit button
    await page.locator('#edit-btn').click();
    await expect(page.locator('#edit-panel')).toBeVisible();

    // Should have section toggles
    await expect(page.locator('#toggle-message')).toBeVisible();
    await expect(page.locator('#toggle-favorites')).toBeVisible();
    await expect(page.locator('#toggle-recent')).toBeVisible();
    await expect(page.locator('#toggle-all')).toBeVisible();
    await expect(page.locator('#toggle-bg')).toBeVisible();

    // Close via Done button
    await page.locator('#edit-panel button:has-text("Done")').click();
    await expect(page.locator('#edit-panel')).toBeHidden();
  });

  test('hiding a section via edit mode persists', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('#section-favorites')).toBeVisible();

    // Open edit, uncheck favorites
    await page.locator('#edit-btn').click();
    await page.locator('#toggle-favorites').uncheck();
    await page.locator('#edit-panel button:has-text("Done")').click();

    await expect(page.locator('#section-favorites')).toBeHidden();

    // Reload — should still be hidden
    await page.reload();
    await expect(page.locator('#section-all')).toBeVisible(); // page loaded
    await expect(page.locator('#section-favorites')).toBeHidden();

    // Cleanup
    await clearHomePrefs(cookie!);
  });

  test('enabling background image via edit mode', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();
    await clearHomePrefs(cookie!);

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);
    await expect(page.locator('#bg-image')).toBeHidden();

    // Enable background
    await page.locator('#edit-btn').click();
    await page.locator('#toggle-bg').check();
    await page.locator('#edit-panel button:has-text("Done")').click();

    await expect(page.locator('#bg-image')).toBeVisible();

    // Cleanup
    await clearHomePrefs(cookie!);
  });
});

// ==================== Theme Toggle Test ====================

test.describe('Home page theme', () => {

  test('theme toggle button exists and cycles theme', async ({ page }) => {
    const cookie = await userLogin(STUDENT_ID, PASSWORD);
    expect(cookie).toBeTruthy();

    const sessionValue = cookie!.replace('classgo_session=', '');
    await page.context().addCookies([{
      name: 'classgo_session',
      value: sessionValue,
      url: BASE_URL,
    }]);

    await page.goto(`${BASE_URL}/home`);

    const themeBtn = page.locator('#theme-btn');
    await expect(themeBtn).toBeVisible();

    // Get initial theme label
    const initialLabel = await page.locator('#theme-label').textContent();

    // Click to cycle
    await themeBtn.click();
    const newLabel = await page.locator('#theme-label').textContent();
    expect(newLabel).not.toBe(initialLabel);
  });
});
