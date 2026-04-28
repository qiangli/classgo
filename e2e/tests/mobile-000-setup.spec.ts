// Restart server with a fresh DB before mobile-chrome tests run.
// This file sorts first alphabetically (mobile-000-*), ensuring it executes
// before other mobile-*.spec.ts files, eliminating DB state pollution from
// the chromium project.
import { test } from '@playwright/test';
import { restartServer } from '../helpers/server.js';
import { adminLogin } from '../helpers/api.js';
import { writeFileSync } from 'fs';
import path from 'path';

test('restart server with fresh DB', async () => {
  test.setTimeout(30000);
  await restartServer();

  // Re-authenticate admin for subsequent tests
  const user = process.env.CLASSGO_TEST_ADMIN_USER;
  const pass = process.env.CLASSGO_TEST_ADMIN_PASS;
  if (user && pass) {
    const cookie = await adminLogin(user, pass);
    if (cookie) {
      const storageState = {
        cookies: [{
          name: 'classgo_session',
          value: cookie.split('=')[1],
          domain: 'localhost',
          path: '/',
          httpOnly: true,
          secure: false,
          sameSite: 'Lax' as const,
          expires: -1,
        }],
        origins: [],
      };
      writeFileSync(
        path.join(__dirname, '..', '.auth', 'admin-state.json'),
        JSON.stringify(storageState, null, 2)
      );
    }
  }
});
