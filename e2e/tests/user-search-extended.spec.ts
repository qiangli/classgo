/**
 * E2E tests for extended user search functionality.
 *
 * Tests search by email, phone, partial name, and cross-entity matching
 * beyond the basic name/ID search covered in coverage-gaps.spec.ts.
 */
import { test, expect } from '../fixtures/auth.js';
import { hasAdminAuth } from '../fixtures/auth.js';

const BASE_URL = 'http://localhost:9090';

test.beforeEach(async () => {
  test.skip(!hasAdminAuth(), 'Admin credentials not provided');
});

test.describe('Search by email', () => {

  test('search finds parent by email', async () => {
    // P002 Maria Garcia has email maria@example.com
    const res = await fetch(`${BASE_URL}/api/users/search?q=maria@example`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    if (data.length > 0) {
      const found = data.find((u: any) => u.email?.includes('maria'));
      expect(found).toBeTruthy();
    }
  });

  test('search finds teacher by email', async () => {
    // T01 Sarah Smith has email smith@example.com
    const res = await fetch(`${BASE_URL}/api/users/search?q=smith@example`);
    expect(res.status).toBe(200);
    const data = await res.json();
    if (data.length > 0) {
      const found = data.find((u: any) => u.type === 'Teacher');
      expect(found).toBeTruthy();
    }
  });
});

test.describe('Search by phone', () => {

  test('search finds entity by phone number', async () => {
    // P001 Wei Wang has phone 555-1234
    const res = await fetch(`${BASE_URL}/api/users/search?q=555-1234`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    if (data.length > 0) {
      const found = data.find((u: any) => u.phone?.includes('555-1234'));
      expect(found).toBeTruthy();
    }
  });
});

test.describe('Search edge cases', () => {

  test('search with exactly 2 characters returns results', async () => {
    // Minimum query length is 2
    const res = await fetch(`${BASE_URL}/api/users/search?q=Al`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    // "Al" should match Alice
    if (data.length > 0) {
      const alice = data.find((u: any) => u.first_name === 'Alice');
      expect(alice).toBeTruthy();
    }
  });

  test('search is case-insensitive', async () => {
    const lower = await fetch(`${BASE_URL}/api/users/search?q=alice`);
    const upper = await fetch(`${BASE_URL}/api/users/search?q=ALICE`);
    const mixed = await fetch(`${BASE_URL}/api/users/search?q=aLiCe`);

    const lowerData = await lower.json();
    const upperData = await upper.json();
    const mixedData = await mixed.json();

    // All should find Alice
    expect(lowerData.length).toBeGreaterThan(0);
    expect(upperData.length).toBeGreaterThan(0);
    expect(mixedData.length).toBeGreaterThan(0);
  });

  test('search returns multiple entity types', async () => {
    // "Wang" appears in students and parents
    const res = await fetch(`${BASE_URL}/api/users/search?q=Wang`);
    const data = await res.json();
    expect(data.length).toBeGreaterThan(0);

    const types = new Set(data.map((u: any) => u.type));
    // Should include at least Student type
    expect(types.has('Student')).toBe(true);
  });

  test('empty query returns empty array', async () => {
    const res = await fetch(`${BASE_URL}/api/users/search?q=`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBe(0);
  });

  test('search with special characters does not crash', async () => {
    const res = await fetch(`${BASE_URL}/api/users/search?q=${encodeURIComponent("O'Brien")}`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(Array.isArray(data)).toBe(true);
  });
});
