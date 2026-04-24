/**
 * E2E tests for the schedule edit modal UI — chip-based student picker,
 * date pickers for effective dates, and time pickers for start/end time.
 *
 * These tests exercise the modal form controls directly via the browser,
 * complementing the API-based tests in schedule-crud.spec.ts.
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

async function dataCRUD(cookie: string, body: Record<string, any>) {
  const res = await fetch(`${BASE_URL}/api/v1/data`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify(body),
  });
  return { status: res.status, data: await res.json() };
}

/** Navigate to Admin > Data > Schedules tab and wait for data to load. */
async function goToSchedulesTab(page: import('@playwright/test').Page) {
  await page.goto('/admin');
  await page.click('#nav-data');
  await page.locator('#section-data').waitFor({ state: 'visible' });
  // Wait for the data to load (data-tbody is rendered by renderDataTab after fetch)
  await page.locator('#data-tbody tr').first().waitFor({ state: 'visible', timeout: 10_000 });
  await page.click('#dtab-schedules');
  // Wait for schedule rows to render (or empty state)
  await page.waitForFunction(() => {
    const tbody = document.getElementById('data-tbody');
    return tbody && tbody.children.length > 0;
  }, { timeout: 5_000 });
}

// ==================== Student Chip Picker ====================

test.describe('Schedule edit modal — student chip picker', () => {

  test('new schedule modal shows student search input', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    // Open "New schedule" modal
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    // Should have the search input, not a plain text input with datalist
    const searchInput = adminPage.locator('#student-search-input');
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toHaveAttribute('placeholder', /Search by name/);

    // Hidden input for student_ids should exist
    const hiddenInput = adminPage.locator('#student-ids-hidden');
    await expect(hiddenInput).toHaveCount(1);

    // Chips container should exist (empty for new schedule)
    const chips = adminPage.locator('#student-chips');
    await expect(chips).toBeVisible();
    await expect(chips.locator('[data-student-id]')).toHaveCount(0);

    // Close modal
    await adminPage.locator('#entity-modal button', { hasText: 'Cancel' }).click();
  });

  test('typeahead search shows matching students', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const searchInput = adminPage.locator('#student-search-input');
    const resultsDiv = adminPage.locator('#student-search-results');

    // Type a search query — use 'S' which should match student IDs
    await searchInput.fill('S0');
    await resultsDiv.waitFor({ state: 'visible' });

    // Should have at least one result
    const results = resultsDiv.locator('div');
    await expect(results.first()).toBeVisible();
  });

  test('clicking a search result adds a chip and clears search', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const searchInput = adminPage.locator('#student-search-input');
    const resultsDiv = adminPage.locator('#student-search-results');
    const chipsContainer = adminPage.locator('#student-chips');
    const hiddenInput = adminPage.locator('#student-ids-hidden');

    // Search and click first result
    await searchInput.fill('S0');
    await resultsDiv.waitFor({ state: 'visible' });
    await resultsDiv.locator('div').first().click();

    // Chip should appear
    await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(1);

    // Search input should be cleared
    await expect(searchInput).toHaveValue('');

    // Hidden input should have the student ID
    const hiddenVal = await hiddenInput.inputValue();
    expect(hiddenVal).toMatch(/^S\d+/);
  });

  test('can add multiple students and remove them', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const searchInput = adminPage.locator('#student-search-input');
    const resultsDiv = adminPage.locator('#student-search-results');
    const chipsContainer = adminPage.locator('#student-chips');
    const hiddenInput = adminPage.locator('#student-ids-hidden');

    // Add first student
    await searchInput.fill('S0');
    await resultsDiv.waitFor({ state: 'visible' });
    await resultsDiv.locator('div').first().click();
    await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(1);

    // Add second student
    await searchInput.fill('S0');
    await resultsDiv.waitFor({ state: 'visible' });
    await resultsDiv.locator('div').first().click();
    await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(2);

    // Hidden input should have both IDs separated by semicolons
    const hiddenVal = await hiddenInput.inputValue();
    expect(hiddenVal).toContain('; ');

    // Remove first chip by clicking its × button
    await chipsContainer.locator('[data-student-id]').first().locator('button').click();
    await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(1);
  });

  test('already-selected students are excluded from search results', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const searchInput = adminPage.locator('#student-search-input');
    const resultsDiv = adminPage.locator('#student-search-results');
    const chipsContainer = adminPage.locator('#student-chips');

    // Add a student
    await searchInput.fill('S001');
    await resultsDiv.waitFor({ state: 'visible' });
    const firstResultText = await resultsDiv.locator('div').first().textContent();
    await resultsDiv.locator('div').first().click();
    await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(1);
    const selectedId = await chipsContainer.locator('[data-student-id]').first().getAttribute('data-student-id');

    // Search again for the same student — they should NOT appear
    await searchInput.fill(selectedId || 'S001');
    // Wait for the debounce
    await adminPage.waitForTimeout(200);
    const resultsVisible = await resultsDiv.isVisible();
    if (resultsVisible) {
      const allResults = await resultsDiv.locator('div').allTextContents();
      // None of the results should contain the already-selected ID as the primary value
      for (const text of allResults) {
        if (text.includes('No matching')) continue;
        // The selected student should not appear in results
        expect(text).not.toContain(`(${selectedId})`);
      }
    }
  });

  test('editing existing schedule shows pre-populated chips', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-CHIP-${stamp}`;

    // Create a schedule with students via API
    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Monday',
        start_time: '10:00',
        end_time: '11:00',
        subject: 'chip-test',
        student_ids: 'S001;S002',
      },
    });

    try {
      await goToSchedulesTab(adminPage);

      // Find and click the Edit button for our schedule
      const row = adminPage.locator(`button:has-text("Edit")`);
      // Find the row containing our schedule ID
      const schedRow = adminPage.locator(`tr`, { hasText: schedId });
      await schedRow.locator('button:has-text("Edit")').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      // Should have 2 pre-populated chips
      const chipsContainer = adminPage.locator('#student-chips');
      await expect(chipsContainer.locator('[data-student-id]')).toHaveCount(2);

      // Chips should have the correct student IDs
      const chipIds = await chipsContainer.locator('[data-student-id]').evaluateAll(
        els => els.map(el => (el as HTMLElement).dataset.studentId)
      );
      expect(chipIds).toContain('S001');
      expect(chipIds).toContain('S002');

      // Hidden input should match
      const hiddenVal = await adminPage.locator('#student-ids-hidden').inputValue();
      expect(hiddenVal).toContain('S001');
      expect(hiddenVal).toContain('S002');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('save schedule with chips round-trips student_ids correctly', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-CHIPSAVE-${stamp}`;

    try {
      await goToSchedulesTab(adminPage);
      await adminPage.locator('#section-data button', { hasText: 'New' }).click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      // Fill required fields
      await adminPage.locator('input[name="id"]').fill(schedId);
      await adminPage.locator('select[name="day_of_week"]').selectOption('Monday');
      await adminPage.locator('input[name="start_time"]').fill('10:00');
      await adminPage.locator('input[name="end_time"]').fill('11:00');
      await adminPage.locator('input[name="subject"]').fill('chip-save-test');

      // Add students via chip picker
      const searchInput = adminPage.locator('#student-search-input');
      const resultsDiv = adminPage.locator('#student-search-results');

      await searchInput.fill('S001');
      await resultsDiv.waitFor({ state: 'visible' });
      await resultsDiv.locator('div').first().click();

      await searchInput.fill('S002');
      await resultsDiv.waitFor({ state: 'visible' });
      await resultsDiv.locator('div').first().click();

      // Save
      await adminPage.locator('#modal-save-btn').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'hidden' });

      // Verify via API that student_ids was saved correctly
      const res = await fetch(`${BASE_URL}/api/v1/directory`, { headers: { Cookie: cookie } });
      const dir = await res.json();
      const found = dir.schedules.find((s: any) => s.id === schedId);
      expect(found).toBeTruthy();
      expect(found.student_ids).toContain('S001');
      expect(found.student_ids).toContain('S002');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });
});

// ==================== Date Pickers ====================

test.describe('Schedule edit modal — date pickers', () => {

  test('effective_from and effective_until render as date inputs', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const fromInput = adminPage.locator('input[name="effective_from"]');
    const untilInput = adminPage.locator('input[name="effective_until"]');

    await expect(fromInput).toBeVisible();
    await expect(untilInput).toBeVisible();
    await expect(fromInput).toHaveAttribute('type', 'date');
    await expect(untilInput).toHaveAttribute('type', 'date');
  });

  test('date inputs accept and save date values', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-DATE-${stamp}`;

    try {
      await goToSchedulesTab(adminPage);
      await adminPage.locator('#section-data button', { hasText: 'New' }).click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      await adminPage.locator('input[name="id"]').fill(schedId);
      await adminPage.locator('select[name="day_of_week"]').selectOption('Tuesday');
      await adminPage.locator('input[name="start_time"]').fill('09:00');
      await adminPage.locator('input[name="end_time"]').fill('10:00');
      await adminPage.locator('input[name="subject"]').fill('date-test');
      await adminPage.locator('input[name="effective_from"]').fill('2026-03-01');
      await adminPage.locator('input[name="effective_until"]').fill('2026-06-30');

      await adminPage.locator('#modal-save-btn').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'hidden' });

      // Verify via API
      const res = await fetch(`${BASE_URL}/api/v1/directory`, { headers: { Cookie: cookie } });
      const dir = await res.json();
      const found = dir.schedules.find((s: any) => s.id === schedId);
      expect(found).toBeTruthy();
      expect(found.effective_from).toBe('2026-03-01');
      expect(found.effective_until).toBe('2026-06-30');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('existing schedule shows pre-filled dates', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-DATEPRE-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Wednesday',
        start_time: '14:00',
        end_time: '15:00',
        subject: 'date-prefill',
        effective_from: '2026-01-15',
        effective_until: '2026-12-31',
      },
    });

    try {
      await goToSchedulesTab(adminPage);
      const schedRow = adminPage.locator('tr', { hasText: schedId });
      await schedRow.locator('button:has-text("Edit")').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      await expect(adminPage.locator('input[name="effective_from"]')).toHaveValue('2026-01-15');
      await expect(adminPage.locator('input[name="effective_until"]')).toHaveValue('2026-12-31');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });
});

// ==================== Time Pickers ====================

test.describe('Schedule edit modal — time pickers', () => {

  test('start_time and end_time render as time inputs', async ({ adminPage }) => {
    await goToSchedulesTab(adminPage);
    await adminPage.locator('#section-data button', { hasText: 'New' }).click();
    await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

    const startInput = adminPage.locator('input[name="start_time"]');
    const endInput = adminPage.locator('input[name="end_time"]');

    await expect(startInput).toBeVisible();
    await expect(endInput).toBeVisible();
    await expect(startInput).toHaveAttribute('type', 'time');
    await expect(endInput).toHaveAttribute('type', 'time');
  });

  test('time inputs accept and save time values', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-TIME-${stamp}`;

    try {
      await goToSchedulesTab(adminPage);
      await adminPage.locator('#section-data button', { hasText: 'New' }).click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      await adminPage.locator('input[name="id"]').fill(schedId);
      await adminPage.locator('select[name="day_of_week"]').selectOption('Thursday');
      await adminPage.locator('input[name="start_time"]').fill('15:30');
      await adminPage.locator('input[name="end_time"]').fill('17:00');
      await adminPage.locator('input[name="subject"]').fill('time-test');

      await adminPage.locator('#modal-save-btn').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'hidden' });

      // Verify via API
      const res = await fetch(`${BASE_URL}/api/v1/directory`, { headers: { Cookie: cookie } });
      const dir = await res.json();
      const found = dir.schedules.find((s: any) => s.id === schedId);
      expect(found).toBeTruthy();
      expect(found.start_time).toBe('15:30');
      expect(found.end_time).toBe('17:00');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });

  test('existing schedule shows pre-filled times', async ({ adminPage }) => {
    const cookie = await getAdminCookie(adminPage);
    const stamp = `${Date.now() % 100000}`;
    const schedId = `E2E-TIMEPRE-${stamp}`;

    await dataCRUD(cookie, {
      action: 'save',
      type: 'schedules',
      data: {
        id: schedId,
        day_of_week: 'Friday',
        start_time: '08:30',
        end_time: '09:45',
        subject: 'time-prefill',
      },
    });

    try {
      await goToSchedulesTab(adminPage);
      const schedRow = adminPage.locator('tr', { hasText: schedId });
      await schedRow.locator('button:has-text("Edit")').click();
      await adminPage.locator('#entity-modal').waitFor({ state: 'visible' });

      await expect(adminPage.locator('input[name="start_time"]')).toHaveValue('08:30');
      await expect(adminPage.locator('input[name="end_time"]')).toHaveValue('09:45');
    } finally {
      await dataCRUD(cookie, { action: 'delete', type: 'schedules', id: schedId });
    }
  });
});
