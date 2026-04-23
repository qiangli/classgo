/**
 * E2E tests for cloud sync (rclone integration).
 *
 * Verifies that:
 * - When cloud_sync is disabled (default), no cloud-sync job is registered
 * - Scheduler API endpoints remain protected
 * - The cloud sync configuration model is properly integrated
 *
 * Note: The test server starts with cloud_sync.enabled=false (default),
 * so we verify the disabled path. Full sync testing requires a real
 * rclone binary and service account, which is out of scope for e2e.
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

test.describe('Cloud sync - disabled by default', () => {
  test('no cloud-sync job when cloud_sync.enabled is false', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const jobs = await res.json();

    const cloudSyncJob = jobs.find((j: any) => j.name === 'cloud-sync');
    expect(cloudSyncJob).toBeFalsy();
  });

  test('backup and export jobs still exist without cloud sync', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const jobs = await res.json();

    expect(jobs.find((j: any) => j.name === 'daily-backup')).toBeTruthy();
    expect(jobs.find((j: any) => j.name === 'daily-attendance-export')).toBeTruthy();
  });
});

test.describe('Cloud sync - API protection', () => {
  test('scheduler API requires auth', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/scheduler/jobs`);
    expect(res.status).toBe(401);
  });

  test('scheduler status requires auth', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/scheduler/status`);
    expect(res.status).toBe(401);
  });
});

test.describe('Cloud sync - settings API', () => {
  test('settings endpoint returns cloud_sync config', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const res = await fetch(`${BASE_URL}/api/settings`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    // The settings endpoint should reflect that cloud_sync is not enabled
    // (default config has no cloud_sync section, so enabled defaults to false)
    expect(data.cloud_sync_enabled).toBeFalsy();
  });
});
