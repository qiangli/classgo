/**
 * E2E tests for export content validation.
 *
 * Verifies that CSV exports contain correct headers and data rows,
 * XLSX export returns valid content, and ZIP exports contain expected files.
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

test.describe('CSV export content', () => {

  test('students CSV has correct headers', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=students`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const csv = await res.text();

    // First line should be headers
    const headerLine = csv.split('\n')[0];
    expect(headerLine).toContain('id');
    expect(headerLine).toContain('first_name');
    expect(headerLine).toContain('last_name');
  });

  test('students CSV contains known students', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=students`, {
      headers: { Cookie: cookie },
    });
    const csv = await res.text();

    // Should contain known student IDs from example data
    expect(csv).toContain('S001');
    expect(csv).toContain('S002');
    expect(csv).toContain('S003');
    // Verify CSV has multiple rows (header + data)
    const lines = csv.trim().split('\n');
    expect(lines.length).toBeGreaterThan(5); // At least 5 students
  });

  test('parents CSV has correct headers', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=parents`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);
    const csv = await res.text();

    const headerLine = csv.split('\n')[0];
    expect(headerLine).toContain('id');
    expect(headerLine).toContain('first_name');
    expect(headerLine).toContain('last_name');
    expect(headerLine).toContain('email');
  });

  test('teachers CSV contains known teachers', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=teachers`, {
      headers: { Cookie: cookie },
    });
    const csv = await res.text();

    expect(csv).toContain('T01');
    expect(csv).toContain('Sarah');
    expect(csv).toContain('Smith');
  });

  test('rooms CSV contains known rooms', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=rooms`, {
      headers: { Cookie: cookie },
    });
    const csv = await res.text();

    expect(csv).toContain('R01');
    expect(csv).toContain('Main Room');
  });

  test('schedules CSV contains known schedules', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=schedules`, {
      headers: { Cookie: cookie },
    });
    const csv = await res.text();

    expect(csv).toContain('SCH001');
    expect(csv).toContain('Monday');
  });

  test('CSV has multiple data rows', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv?type=students`, {
      headers: { Cookie: cookie },
    });
    const csv = await res.text();
    const lines = csv.trim().split('\n');
    // Should have header + at least some data rows
    expect(lines.length).toBeGreaterThan(1);
  });
});

test.describe('XLSX export', () => {

  test('XLSX export returns valid file', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/xlsx`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const contentType = res.headers.get('content-type');
    expect(contentType).toContain('spreadsheet');

    const disposition = res.headers.get('content-disposition');
    expect(disposition).toContain('.xlsx');

    // Should have non-trivial content
    const body = await res.arrayBuffer();
    expect(body.byteLength).toBeGreaterThan(100);
  });
});

test.describe('ZIP export', () => {

  test('ZIP export has correct content type and size', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    const res = await fetch(`${BASE_URL}/admin/export/csv/zip`, {
      headers: { Cookie: cookie },
    });
    expect(res.status).toBe(200);

    const contentType = res.headers.get('content-type');
    expect(contentType).toContain('application/zip');

    const body = await res.arrayBuffer();
    // ZIP files start with PK magic bytes (0x50, 0x4B)
    const bytes = new Uint8Array(body);
    expect(bytes[0]).toBe(0x50); // P
    expect(bytes[1]).toBe(0x4b); // K

    // Should contain multiple CSV files worth of data
    expect(body.byteLength).toBeGreaterThan(500);
  });
});
