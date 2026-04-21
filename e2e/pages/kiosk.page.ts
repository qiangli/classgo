import { Page, Locator } from '@playwright/test';

export class KioskPage {
  readonly nameInput: Locator;
  readonly searchResults: Locator;
  readonly keypad: Locator;
  readonly display: Locator;
  readonly submitBtn: Locator;
  readonly successOverlay: Locator;
  readonly successName: Locator;
  readonly checkoutOverlay: Locator;
  readonly checkoutName: Locator;
  readonly trackerOverlay: Locator;

  constructor(private page: Page) {
    this.nameInput = page.locator('#name-input');
    this.searchResults = page.locator('#kiosk-search-results');
    this.keypad = page.locator('#keypad');
    this.display = page.locator('#display');
    this.submitBtn = page.locator('#submit-btn');
    this.successOverlay = page.locator('#success-overlay');
    this.successName = page.locator('#success-name');
    this.checkoutOverlay = page.locator('#checkout-overlay');
    this.checkoutName = page.locator('#checkout-name');
    this.trackerOverlay = page.locator('#tracker-overlay');
  }

  async goto() {
    await this.page.goto('/kiosk');
  }

  async searchStudent(name: string) {
    await this.nameInput.fill(name);
    await this.searchResults.waitFor({ state: 'visible' });
  }

  async selectFirstResult() {
    await this.searchResults.locator('li').first().click();
  }

  async searchAndSelect(name: string) {
    await this.searchStudent(name);
    await this.selectFirstResult();
  }

  async pressKey(digit: string) {
    await this.page.click(`.kbtn:has-text("${digit}")`);
  }

  async enterPin(pin: string) {
    for (const d of pin) {
      await this.pressKey(d);
    }
  }

  async submit() {
    await this.submitBtn.click();
  }

  async waitForSuccess() {
    await this.successOverlay.waitFor({ state: 'visible' });
  }

  async waitForCheckoutOverlay() {
    await this.checkoutOverlay.waitFor({ state: 'visible' });
  }
}
