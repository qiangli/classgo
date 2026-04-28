import { Page, Locator } from '@playwright/test';

export class MobilePage {
  readonly studentName: Locator;
  readonly pinField: Locator;
  readonly pinInput: Locator;
  readonly submitBtn: Locator;
  readonly searchResults: Locator;
  readonly confirmedCard: Locator;
  readonly confirmedName: Locator;
  readonly confirmedStatus: Locator;
  readonly checkoutBtn: Locator;
  readonly checkoutPinField: Locator;
  readonly checkoutPinInput: Locator;
  readonly checkinMessage: Locator;
  readonly trackerOverlay: Locator;

  constructor(private page: Page) {
    this.studentName = page.locator('#student-name');
    this.pinField = page.locator('#pin-field');
    this.pinInput = page.locator('#pin');
    this.submitBtn = page.locator('#checkin-submit-btn');
    this.searchResults = page.locator('#search-results');
    this.confirmedCard = page.locator('#confirmed-card');
    this.confirmedName = page.locator('#confirmed-name');
    this.confirmedStatus = page.locator('#confirmed-status');
    this.checkoutBtn = page.locator('#checkout-btn');
    this.checkoutPinField = page.locator('#checkout-pin-field');
    this.checkoutPinInput = page.locator('#checkout-pin');
    this.checkinMessage = page.locator('#checkin-message');
    this.trackerOverlay = page.locator('#tracker-overlay');
  }

  async goto() {
    await this.page.goto('/');
    await this.page.waitForLoadState('networkidle');
  }

  async switchToTab(tab: 'checkin' | 'signup' | 'login') {
    await this.page.click(`#tab-${tab}`);
  }

  async searchStudent(name: string) {
    await this.studentName.waitFor({ state: 'visible' });
    await this.studentName.click();
    await this.studentName.fill(name);
    await this.searchResults.locator('li').first().waitFor({ state: 'visible', timeout: 10000 });
  }

  async selectFirstResult() {
    await this.searchResults.locator('li').first().click();
  }

  async searchAndSelect(name: string) {
    await this.searchStudent(name);
    await this.selectFirstResult();
  }

  async fillPin(pin: string) {
    await this.pinInput.fill(pin);
  }

  async submitCheckin() {
    await this.submitBtn.click();
  }

  async checkin(name: string, pin?: string) {
    await this.searchAndSelect(name);
    if (pin) await this.fillPin(pin);
    await this.submitCheckin();
  }

  async checkout(pin?: string) {
    if (pin) {
      await this.checkoutPinInput.fill(pin);
    }
    await this.checkoutBtn.click();
  }

  async waitForConfirmation() {
    await this.confirmedCard.waitFor({ state: 'visible' });
  }
}
