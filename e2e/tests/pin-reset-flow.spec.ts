/**
 * E2E tests for per-student PIN reset and management flows.
 *
 * Covers: enabling per-student PIN, resetting PIN, verifying PIN
 * requirement at check-in, and disabling per-student PIN.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';
import {
  forceCheckoutViaAPI,
  setStudentPinRequireViaAPI,
  resetStudentPinViaAPI,
  pinCheckViaAPI,
  setPinModeViaAPI,
} from '../helpers/api.js';

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

test.describe('Per-student PIN management', () => {

  test('admin can enable per-student PIN requirement', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Enable personal PIN for S005 (Emma Taylor)
    const result = await setStudentPinRequireViaAPI(cookie, 'S005', true);
    expect(result.ok).toBe(true);
    expect(result.pin).toBeTruthy();
    // PIN should be 4 digits
    expect(result.pin).toMatch(/^\d{4}$/);

    // Cleanup
    await setStudentPinRequireViaAPI(cookie, 'S005', false);
  });

  test('admin can reset per-student PIN', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Enable PIN first
    const enableResult = await setStudentPinRequireViaAPI(cookie, 'S005', true);
    const originalPin = enableResult.pin;

    // Reset PIN — should get a new one
    const resetResult = await resetStudentPinViaAPI(cookie, 'S005');
    expect(resetResult.ok).toBe(true);
    expect(resetResult.pin).toBeTruthy();
    expect(resetResult.pin).toMatch(/^\d{4}$/);
    // New PIN may or may not differ (random), but the call should succeed

    // Cleanup
    await setStudentPinRequireViaAPI(cookie, 'S005', false);
  });

  test('PIN check returns needs_pin for student with PIN required', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    // Ensure center PIN is off, but per-student is on
    await setPinModeViaAPI(cookie, 'off');
    const enableResult = await setStudentPinRequireViaAPI(cookie, 'S005', true);
    expect(enableResult.ok).toBe(true);

    // Check PIN requirement
    const check = await pinCheckViaAPI('S005');
    expect(check.needs_pin).toBe(true);

    // Cleanup
    await setStudentPinRequireViaAPI(cookie, 'S005', false);
  });

  test('check-in requires PIN when per-student PIN is enabled', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await setPinModeViaAPI(cookie, 'off');
    const enableResult = await setStudentPinRequireViaAPI(cookie, 'S005', true);
    const pin = enableResult.pin;

    await forceCheckoutViaAPI('Emma').catch(() => {});

    // Try check-in without PIN — should fail
    const noPin = await fetch(`${BASE_URL}/api/checkin`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        student_name: 'Emma',
        device_type: 'mobile',
        device_id: `e2e-pin-${Date.now()}`,
      }),
    });
    const noPinResult = await noPin.json();
    expect(noPinResult.needs_pin).toBe(true);

    // Try with correct PIN — should succeed
    const withPin = await fetch(`${BASE_URL}/api/checkin`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        student_name: 'Emma',
        device_type: 'mobile',
        device_id: `e2e-pin2-${Date.now()}`,
        pin: pin,
      }),
    });
    const withPinResult = await withPin.json();
    expect(withPinResult.ok).toBe(true);

    // Cleanup
    await forceCheckoutViaAPI('Emma').catch(() => {});
    await setStudentPinRequireViaAPI(cookie, 'S005', false);
  });

  test('disabling per-student PIN removes requirement', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);

    await setPinModeViaAPI(cookie, 'off');
    await setStudentPinRequireViaAPI(cookie, 'S005', true);

    // Disable
    const disableResult = await setStudentPinRequireViaAPI(cookie, 'S005', false);
    expect(disableResult.ok).toBe(true);

    // PIN check should show no PIN needed
    const check = await pinCheckViaAPI('S005');
    expect(check.needs_pin).toBe(false);
  });

  test('PIN reset rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/student/pin/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ student_id: 'S001' }),
    });
    expect(res.status).toBe(401);
  });

  test('PIN require rejects unauthenticated requests', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/student/pin/require`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ student_id: 'S001', require_pin: true }),
    });
    expect(res.status).toBe(401);
  });
});
