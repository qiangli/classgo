/**
 * E2E tests for the scheduler (cron) UI feature.
 *
 * Verifies that superadmin users can see and interact with the
 * scheduler section in the admin dashboard, including viewing
 * jobs, checking WebSocket connectivity, and triggering manual runs.
 * Also verifies that scheduler API endpoints are protected.
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

// --- API helper functions ---

async function getSchedulerJobs(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getSchedulerStatus(cookie: string) {
  const res = await fetch(`${BASE_URL}/api/v1/scheduler/status`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function runSchedulerJob(cookie: string, jobId: string) {
  const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs/${jobId}/run`, {
    method: 'POST',
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

async function getSchedulerJob(cookie: string, jobId: string) {
  const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs/${jobId}`, {
    headers: { Cookie: cookie },
  });
  return { status: res.status, data: await res.json() };
}

// --- UI tests ---

test('scheduler nav item is visible for superadmin', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin`);
  const navItem = adminPage.locator('#nav-scheduler');
  await expect(navItem).toBeVisible();
});

test('can navigate to scheduler section', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin`);
  await adminPage.locator('#nav-scheduler').click();

  const section = adminPage.locator('#section-scheduler');
  await expect(section).toBeVisible();

  // URL should include #scheduler
  expect(adminPage.url()).toContain('#scheduler');
});

test('scheduler section shows connection status', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin#scheduler`);

  // Wait for WebSocket to connect
  const status = adminPage.locator('#scheduler-connection-status');
  await expect(status).toBeVisible();

  // Should show "Connected" after WebSocket connects
  await expect(status).toContainText('Connected', { timeout: 10_000 });
});

test('scheduler section shows job count', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin#scheduler`);

  // Trigger navigation to load the section
  await adminPage.locator('#nav-scheduler').click();

  const jobCount = adminPage.locator('#scheduler-job-count');
  await expect(jobCount).toBeVisible();

  // Should show at least 2 jobs (daily-backup + daily-attendance-export)
  await expect(async () => {
    const text = await jobCount.textContent();
    expect(Number(text)).toBeGreaterThanOrEqual(2);
  }).toPass({ timeout: 10_000 });
});

test('scheduler section displays daily-backup job card', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin`);
  await adminPage.locator('#nav-scheduler').click();

  const container = adminPage.locator('#scheduler-jobs-container');

  // Wait for job cards to render (WebSocket delivers data)
  await expect(async () => {
    const html = await container.innerHTML();
    expect(html).toContain('daily-backup');
  }).toPass({ timeout: 10_000 });

  // Verify job card shows schedule info
  const cardText = await container.textContent();
  expect(cardText).toContain('Next Run');
  expect(cardText).toContain('Last Run');
  expect(cardText).toContain('Scheduler');
});

test('scheduler job card has Run button', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin`);
  await adminPage.locator('#nav-scheduler').click();

  // Wait for job cards to load
  await expect(async () => {
    const html = await adminPage.locator('#scheduler-jobs-container').innerHTML();
    expect(html).toContain('daily-backup');
  }).toPass({ timeout: 10_000 });

  // Find the Run button
  const runButton = adminPage.locator('#scheduler-jobs-container button', { hasText: 'Run' });
  await expect(runButton.first()).toBeVisible();
});

// --- Run button confirmation UI tests ---

/** Navigate to scheduler and wait for job cards to load */
async function gotoScheduler(adminPage: import('@playwright/test').Page) {
  await adminPage.goto(`${BASE_URL}/admin`);
  await adminPage.locator('#nav-scheduler').click();
  await expect(async () => {
    const html = await adminPage.locator('#scheduler-jobs-container').innerHTML();
    expect(html).toContain('daily-backup');
  }).toPass({ timeout: 10_000 });
}

test('Run button shows confirmation dialog with Cancel and Confirm', async ({ adminPage }) => {
  await gotoScheduler(adminPage);

  // Click the first Run button
  const runButton = adminPage.locator('#scheduler-jobs-container button', { hasText: 'Run' }).first();
  await runButton.click();

  // Confirmation dialog should appear with job name and both buttons
  const dialogText = adminPage.getByText(/Run ".*" now\?/);
  await expect(dialogText).toBeVisible({ timeout: 3_000 });
  // The confirm.js dialog appends a backdrop div; locate buttons inside it
  const backdrop = adminPage.locator('div', { has: dialogText });
  await expect(backdrop.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
  await expect(backdrop.getByRole('button', { name: 'Confirm', exact: true })).toBeVisible();
});

test('Run button cancel dismisses dialog without running job', async ({ adminPage }) => {
  await gotoScheduler(adminPage);

  const runButton = adminPage.locator('#scheduler-jobs-container button', { hasText: 'Run' }).first();
  await runButton.click();

  const dialogText = adminPage.getByText(/Run ".*" now\?/);
  await expect(dialogText).toBeVisible({ timeout: 3_000 });

  // Click Cancel
  const backdrop = adminPage.locator('div', { has: dialogText });
  await backdrop.getByRole('button', { name: 'Cancel', exact: true }).click();

  // Dialog should disappear
  await expect(dialogText).toBeHidden({ timeout: 3_000 });

  // No success toast should appear
  await adminPage.waitForTimeout(500);
  const toastContainer = adminPage.locator('#toast-container');
  const toastCount = await toastContainer.count();
  if (toastCount > 0) {
    const toastText = await toastContainer.textContent();
    expect(toastText).not.toContain('executed successfully');
  }
});

test('Run button confirm executes job and shows success toast', async ({ adminPage }) => {
  await gotoScheduler(adminPage);

  const runButton = adminPage.locator('#scheduler-jobs-container button', { hasText: 'Run' }).first();
  await runButton.click();

  const dialogText = adminPage.getByText(/Run ".*" now\?/);
  await expect(dialogText).toBeVisible({ timeout: 3_000 });

  // Click Confirm
  const backdrop = adminPage.locator('div', { has: dialogText });
  await backdrop.getByRole('button', { name: 'Confirm', exact: true }).click();

  // Dialog should disappear
  await expect(dialogText).toBeHidden({ timeout: 3_000 });

  // Success toast should appear
  const toast = adminPage.locator('#toast-container');
  await expect(toast).toContainText('executed successfully', { timeout: 5_000 });
});

test('Run button Escape key cancels dialog', async ({ adminPage }) => {
  await gotoScheduler(adminPage);

  const runButton = adminPage.locator('#scheduler-jobs-container button', { hasText: 'Run' }).first();
  await runButton.click();

  const dialogText = adminPage.getByText(/Run ".*" now\?/);
  await expect(dialogText).toBeVisible({ timeout: 3_000 });

  // Press Escape
  await adminPage.keyboard.press('Escape');

  // Dialog should disappear
  await expect(dialogText).toBeHidden({ timeout: 3_000 });
});

// --- API tests ---

test('scheduler API returns job list with both jobs', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);
  const { status, data } = await getSchedulerJobs(cookie);

  expect(status).toBe(200);
  expect(Array.isArray(data)).toBe(true);
  expect(data.length).toBeGreaterThanOrEqual(2);

  // Find the daily-backup job
  const backup = data.find((j: any) => j.name === 'daily-backup');
  expect(backup).toBeTruthy();
  expect(backup.id).toBeTruthy();
  expect(backup.nextRun).toBeTruthy();
  expect(backup.schedulerName).toBe('Default');

  // Find the daily-attendance-export job
  const exportJob = data.find((j: any) => j.name === 'daily-attendance-export');
  expect(exportJob).toBeTruthy();
  expect(exportJob.id).toBeTruthy();
  expect(exportJob.nextRun).toBeTruthy();
  expect(exportJob.schedulerName).toBe('Default');
});

test('scheduler API returns status', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);
  const { status, data } = await getSchedulerStatus(cookie);

  expect(status).toBe(200);
  expect(data.schedulers).toBe(1);
  expect(data.totalJobs).toBeGreaterThanOrEqual(1);
});

test('scheduler API returns single job by ID', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);

  // Get job list first to find the ID
  const { data: jobs } = await getSchedulerJobs(cookie);
  const backup = jobs.find((j: any) => j.name === 'daily-backup');
  expect(backup).toBeTruthy();

  // Fetch single job
  const { status, data } = await getSchedulerJob(cookie, backup.id);
  expect(status).toBe(200);
  expect(data.name).toBe('daily-backup');
  expect(data.id).toBe(backup.id);
});

test('scheduler API can trigger manual job run', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);

  // Get the daily-backup job ID
  const { data: jobs } = await getSchedulerJobs(cookie);
  const backup = jobs.find((j: any) => j.name === 'daily-backup');
  expect(backup).toBeTruthy();

  // Run it
  const { status, data } = await runSchedulerJob(cookie, backup.id);
  expect(status).toBe(200);
  expect(data.message).toBe('Job executed');
});

test('scheduler section displays both job cards', async ({ adminPage }) => {
  await adminPage.goto(`${BASE_URL}/admin`);
  await adminPage.locator('#nav-scheduler').click();

  const container = adminPage.locator('#scheduler-jobs-container');

  // Wait for both job cards to render
  await expect(async () => {
    const html = await container.innerHTML();
    expect(html).toContain('daily-backup');
    expect(html).toContain('daily-attendance-export');
  }).toPass({ timeout: 10_000 });
});

test('can trigger daily-attendance-export via API', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);

  // Get the export job ID
  const { data: jobs } = await getSchedulerJobs(cookie);
  const exportJob = jobs.find((j: any) => j.name === 'daily-attendance-export');
  expect(exportJob).toBeTruthy();

  // Run it
  const { status, data } = await runSchedulerJob(cookie, exportJob.id);
  expect(status).toBe(200);
  expect(data.message).toBe('Job executed');
});

test('scheduler API rejects invalid job ID', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);

  const { status } = await getSchedulerJob(cookie, 'not-a-uuid');
  expect(status).toBe(400);
});

test('scheduler API returns 404 for non-existent job', async ({ adminPage }) => {
  const cookie = await getAdminCookie(adminPage);

  const { status } = await getSchedulerJob(cookie, '00000000-0000-0000-0000-000000000000');
  expect(status).toBe(404);
});

// --- Auth protection tests ---

test('scheduler API rejects unauthenticated requests', async () => {
  const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`);
  expect(res.status).toBe(401);
});

test('scheduler API rejects non-superadmin requests', async () => {
  // Login as a regular user (student) — should get 403
  const setupRes = await fetch(`${BASE_URL}/api/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'setup' }),
    redirect: 'manual',
  });
  const setCookie = setupRes.headers.get('set-cookie');
  if (!setCookie) {
    // Already set up, do regular login
    const loginRes = await fetch(`${BASE_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ entity_id: 'S001', password: 'test1234', action: 'login' }),
      redirect: 'manual',
    });
    const loginCookie = loginRes.headers.get('set-cookie');
    expect(loginCookie).toBeTruthy();
    const match = loginCookie!.match(/classgo_session=([^;]+)/);
    const userCookie = `classgo_session=${match![1]}`;

    const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: userCookie },
    });
    expect(res.status).toBe(403);
  } else {
    const match = setCookie.match(/classgo_session=([^;]+)/);
    const userCookie = `classgo_session=${match![1]}`;

    const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: userCookie },
    });
    expect(res.status).toBe(403);
  }
});
