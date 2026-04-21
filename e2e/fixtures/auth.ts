import { test as base, Page } from '@playwright/test';
import path from 'path';
import { existsSync } from 'fs';

const ADMIN_STATE = path.join(__dirname, '..', '.auth', 'admin-state.json');

export const test = base.extend<{ adminPage: Page }>({
  adminPage: async ({ browser }, use) => {
    if (!existsSync(ADMIN_STATE)) {
      test.skip(true, 'Admin credentials not provided');
      return;
    }
    const ctx = await browser.newContext({ storageState: ADMIN_STATE });
    const page = await ctx.newPage();
    await use(page);
    await ctx.close();
  },
});

export { expect } from '@playwright/test';

export function hasAdminAuth(): boolean {
  return existsSync(ADMIN_STATE);
}
