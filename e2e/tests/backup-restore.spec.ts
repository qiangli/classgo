/**
 * E2E tests for backup/restore functionality.
 *
 * Verifies that backup and attendance export can be triggered via the
 * scheduler's manual run API, and that job metadata is correct.
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

test.describe('Backup via scheduler', () => {
  test('can trigger backup via scheduler API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    // Get jobs list
    const jobsRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    });
    expect(jobsRes.status).toBe(200);
    const jobs = await jobsRes.json();
    const backupJob = jobs.find((j: any) => j.name === 'daily-backup');
    expect(backupJob).toBeTruthy();

    // Trigger manual run
    const runRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs/${backupJob.id}/run`, {
      method: 'POST',
      headers: { Cookie: cookie },
    });
    expect(runRes.status).toBe(200);
    const result = await runRes.json();
    expect(result.message).toBe('Job executed');
  });

  test('can trigger attendance export via scheduler API', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const jobsRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    });
    const jobs = await jobsRes.json();
    const exportJob = jobs.find((j: any) => j.name === 'daily-attendance-export');
    expect(exportJob).toBeTruthy();

    const runRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs/${exportJob.id}/run`, {
      method: 'POST',
      headers: { Cookie: cookie },
    });
    expect(runRes.status).toBe(200);
    const result = await runRes.json();
    expect(result.message).toBe('Job executed');
  });

  test('backup job appears in job list with schedule info', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const { status, ...rest } = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    }).then(async r => ({ status: r.status, data: await r.json() }));
    expect(status).toBe(200);
    const backup = rest.data.find((j: any) => j.name === 'daily-backup');
    expect(backup).toBeTruthy();
    expect(backup.id).toBeTruthy();
    expect(backup.nextRun).toBeTruthy();
    expect(backup.schedulerName).toBe('Default');
  });

  test('scheduler status shows expected job count', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/v1/scheduler/status`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.schedulers).toBe(1);
    expect(data.totalJobs).toBeGreaterThanOrEqual(2);
  });

  test('backup and export jobs can both run successfully in sequence', async ({ adminPage }) => {
    // This verifies backup doesn't interfere with export and vice versa
    const cookie = await getAdminCookie(adminPage);
    const jobsRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    });
    const jobs = await jobsRes.json();

    for (const jobName of ['daily-backup', 'daily-attendance-export']) {
      const job = jobs.find((j: any) => j.name === jobName);
      expect(job).toBeTruthy();
      const runRes = await fetch(`${BASE_URL}/api/v1/scheduler/jobs/${job.id}/run`, {
        method: 'POST',
        headers: { Cookie: cookie },
      });
      expect(runRes.status).toBe(200);
      const result = await runRes.json();
      expect(result.message).toBe('Job executed');
    }
  });
});
