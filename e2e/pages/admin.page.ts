import { Page, Locator } from '@playwright/test';

export class AdminPage {
  readonly pageTitle: Locator;

  constructor(private page: Page) {
    this.pageTitle = page.locator('#page-title');
  }

  async goto() {
    await this.page.goto('/admin');
  }

  async navigateTo(section: string) {
    await this.page.click(`#nav-${section}`);
    await this.page.locator(`#section-${section}`).waitFor({ state: 'visible' });
  }

  section(name: string): Locator {
    return this.page.locator(`#section-${name}`);
  }

  // Check-in section elements
  get pinToggle(): Locator { return this.page.locator('#pin-toggle'); }
  get pinValue(): Locator { return this.page.locator('#pin-value'); }
  get pinOverrideSearch(): Locator { return this.page.locator('#pin-override-search'); }
  get pinOverrideResults(): Locator { return this.page.locator('#pin-override-results'); }
  get pinOverrideList(): Locator { return this.page.locator('#pin-override-list'); }
  get pinOverrideEmpty(): Locator { return this.page.locator('#pin-override-empty'); }

  // Audit section elements (read-only)
  get pinModeOff(): Locator { return this.page.locator('#pin-mode-off'); }
  get pinModeCenter(): Locator { return this.page.locator('#pin-mode-center'); }
  get pinOverrideListAudit(): Locator { return this.page.locator('#pin-override-list-audit'); }
  get pinOverrideEmptyAudit(): Locator { return this.page.locator('#pin-override-empty-audit'); }

  async searchPinOverride(name: string) {
    await this.pinOverrideSearch.fill(name);
    await this.pinOverrideResults.waitFor({ state: 'visible' });
  }

  async selectFirstPinOverrideResult() {
    await this.pinOverrideResults.locator('div').first().click();
  }
}
