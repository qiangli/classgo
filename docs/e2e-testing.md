# E2E Testing with Playwright

Browser-based end-to-end tests for ClassGo's interactive UI flows (typeahead search, dynamic PIN fields, tab switching, tracker overlays).

## Quick Start

```bash
# First-time setup
make test-e2e-setup

# Run tests (public flows only)
make test-e2e

# Run with admin tests (requires OS-level credentials)
CLASSGO_TEST_ADMIN_USER=<user> CLASSGO_TEST_ADMIN_PASS=<pass> make test-e2e

# Run in headed mode (visible browser)
make test-e2e-headed

# View HTML report
cd e2e && npx playwright show-report
```

## Architecture

### Directory Structure

```
e2e/
  package.json              # Isolated from memos/web/ — Playwright deps only
  playwright.config.ts
  tsconfig.json
  global-setup.ts           # Build Go binary, start test server, admin auth
  global-teardown.ts        # Stop server, remove temp DB
  helpers/
    server.ts               # Server lifecycle: build, start, stop, health poll
    api.ts                  # Fetch wrappers for test preconditions
  fixtures/
    auth.ts                 # Playwright test.extend with adminPage fixture
  pages/
    mobile.page.ts          # Page Object for / (entry.html)
    kiosk.page.ts           # Page Object for /kiosk
    admin.page.ts           # Page Object for /admin
  tests/
    mobile-checkin.spec.ts
    mobile-checkout.spec.ts
    kiosk.spec.ts
    admin-pin.spec.ts
    checkout-tracker.spec.ts
    pin-mid-session.spec.ts
```

### How It Works

1. **Global setup** builds the Go binary, creates an ephemeral SQLite DB via `mktemp`, and starts the server on port 9090 with `data/csv.example` test data.
2. **Health check** polls `GET /api/settings` until the server responds (10s timeout, 500ms interval).
3. **Admin auth** (optional): if `CLASSGO_TEST_ADMIN_USER` / `CLASSGO_TEST_ADMIN_PASS` env vars are set, authenticates via `POST /admin/api/login` and saves the session cookie as Playwright `storageState`.
4. Tests run sequentially (`workers: 1`) against the shared server and DB.
5. **Global teardown** sends SIGTERM and removes the temp DB.

### Key Design Decisions

- **Sequential execution**: all tests share one server + ephemeral DB; parallel execution would cause state conflicts.
- **Two browser projects**: Desktop Chrome (admin/kiosk) + Mobile Chrome viewport (entry.html).
- **Port 9090**: matches the existing validate skill pattern.
- **`e2e/package.json`**: isolated from the project root and `memos/web/` to avoid dependency conflicts.
- **Admin tests skip gracefully**: when env vars aren't set, admin-gated tests are skipped (not failed).

## Test Coverage

### P1: Core Flows (no auth required)

| Spec File | Tests | What it validates |
|-----------|-------|-------------------|
| `mobile-checkin.spec.ts` | 5 | Typeahead search, student selection, check-in confirmation, search by ID, inactive student filtering, unknown student rejection, duplicate detection |
| `mobile-checkout.spec.ts` | 1 | Checkout flow with automatic tracker overlay handling |
| `kiosk.spec.ts` | 3 | Kiosk check-in, checkout (with tracker handling), search results with student ID |

### P2–P3: Admin + PIN Flows (requires auth)

| Spec File | Tests | What it validates |
|-----------|-------|-------------------|
| `admin-pin.spec.ts` | 6 | Sidebar navigation, PIN mode read-only on audit page, PIN override management on checkin page, center PIN enforcement on mobile, wrong PIN rejection |

### P4: Checkout + Tracker (requires auth)

| Spec File | Tests | What it validates |
|-----------|-------|-------------------|
| `checkout-tracker.spec.ts` | 2 | Checkout blocked by pending signoff task, respond to tracker items to complete checkout |

### P5: PIN State Changes Mid-Session (requires auth)

| Spec File | Tests | What it validates |
|-----------|-------|-------------------|
| `pin-mid-session.spec.ts` | 6 | Center PIN toggled on/off between check-in and checkout (mobile + kiosk), per-student flag/unflag between check-in and checkout (mobile + kiosk) |

## Writing New Tests

### Page Objects

Use the page objects in `e2e/pages/` to interact with UI elements:

```typescript
import { MobilePage } from '../pages/mobile.page.js';

test('example', async ({ page }) => {
  const mobile = new MobilePage(page);
  await mobile.goto();
  await mobile.checkin('Alice');
  await mobile.waitForConfirmation();
});
```

### API Helpers

Use `e2e/helpers/api.ts` for test setup/teardown (not for driving the UI):

```typescript
import { checkinViaAPI, checkoutViaAPI, setPinModeViaAPI } from '../helpers/api.js';

// Check in a student before testing checkout UI
await checkinViaAPI('Alice Wang', 'mobile');

// Clean up after test
await checkoutViaAPI('Alice Wang');
```

### Admin-Gated Tests

Use the `adminPage` fixture from `e2e/fixtures/auth.ts`:

```typescript
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

test('admin test', async ({ adminPage, page }) => {
  // adminPage is pre-authenticated as admin
  // page is an unauthenticated browser page
});
```

### Handling Tracker Overlays

Students loaded from `data/csv.example` may have tracker tasks assigned. When testing checkout, handle the tracker overlay:

```typescript
const result = await Promise.race([
  kiosk.checkoutOverlay.waitFor({ state: 'visible', timeout: 10_000 })
    .then(() => 'done' as const).catch(() => never),
  kiosk.trackerOverlay.waitFor({ state: 'visible', timeout: 10_000 })
    .then(() => 'tracker' as const).catch(() => never),
]);

if (result === 'tracker') {
  // Click Done on all items, then submit
  const doneButtons = trackerOverlay.locator('button:has-text("Done")');
  for (let i = 0; i < await doneButtons.count(); i++) {
    await doneButtons.nth(i).click();
  }
  await page.locator('#tracker-submit-btn').click();
}
```

### Common Pitfalls

- **Typeahead debounce**: Student search has a 300ms debounce. Use `waitFor({ state: 'visible' })` on the search results, not fixed waits.
- **PIN field async**: The mobile page fetches `/api/pin/check` on load. Wait for the response before asserting PIN field visibility.
- **Test isolation**: Tests that change PIN mode or flag students must reset state in `afterAll()` or `finally` blocks.
- **Rate limiting**: The kiosk has a rate limiter on check-ins from the same device. Use `checkinViaAPI()` for setup steps to avoid triggering it.
- **Kiosk button selectors**: Use `#name-step button:has-text("Check In")` to avoid matching hidden tracker overlay buttons with similar text.

## Relationship to Other Test Layers

| Layer | Tool | Coverage |
|-------|------|----------|
| **API/Handler tests** | Go `httptest` (`*_test.go`) | All API endpoints, PIN validation, rate limiting, audit records |
| **Integration validation** | curl (`skills/validate.md`) | Server startup, API round-trips, auth flows |
| **Browser E2E** | Playwright (`e2e/`) | Interactive UI flows, typeahead, overlays, tab switching, PIN field visibility |
