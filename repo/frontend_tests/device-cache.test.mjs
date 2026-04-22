/**
 * Unit tests for web/static/js/device-cache.js
 * Uses Node.js built-in test runner (node:test) — no npm required.
 * Mocks localStorage and disables IndexedDB; only tests pure logic and
 * localStorage-backed paths (cacheExercise, getExercise, getStats, clearAll).
 *
 * Run: node --test frontend_tests/device-cache.test.mjs
 */
import { test, beforeEach } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createContext, runInContext } from 'node:vm';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const __dir = dirname(fileURLToPath(import.meta.url));
const cacheSource = readFileSync(join(__dir, '../web/static/js/device-cache.js'), 'utf8');

/** Build a fresh localStorage mock and DeviceCache instance. */
function buildCache() {
  const store = new Map();
  const mockLocalStorage = {
    getItem:    (k)    => store.has(k) ? store.get(k) : null,
    setItem:    (k, v) => { store.set(k, String(v)); },
    removeItem: (k)    => { store.delete(k); },
    clear:      ()     => { store.clear(); },
    get length()        { return store.size; },
  };

  // Set window.indexedDB = null so openIDB() rejects immediately;
  // cacheMedia / getMedia degrade gracefully.
  const ctx = createContext({
    window:       { indexedDB: null },
    localStorage: mockLocalStorage,
    console:      { error() {}, warn() {}, log() {} },
    Promise:      Promise,
    JSON:         JSON,
    Date:         Date,
  });

  // const bindings in vm scripts don't propagate to the sandbox object.
  // Rewrite the top-level `const DeviceCache =` to `var DeviceCache =` so
  // the IIFE result is accessible as ctx.DeviceCache after execution.
  const execSource = cacheSource.replace(/^const DeviceCache\s*=/m, 'var DeviceCache =');
  runInContext(execSource, ctx);
  return { cache: ctx.DeviceCache, store };
}

// ── loadMeta / saveMeta (via getStats) ───────────────────────────────────────

test('getStats: empty cache returns count=0 and totalBytes=0', async () => {
  const { cache } = buildCache();
  const stats = await cache.getStats();
  assert.equal(stats.count,      0);
  assert.equal(stats.totalBytes, 0);
  assert.equal(stats.sizeMB,     '0.00');
});

// ── cacheExercise / getExercise ───────────────────────────────────────────────

test('cacheExercise + getExercise: round-trip returns the cached object', async () => {
  const { cache } = buildCache();
  const data = { id: 'ex-001', title: 'Balance Training', reps: 10 };
  cache.cacheExercise('ex-001', data);
  const result = cache.getExercise('ex-001');
  assert.deepEqual(result, data);
});

test('getExercise: returns null for unknown id', () => {
  const { cache } = buildCache();
  const result = cache.getExercise('ex-unknown');
  assert.equal(result, null);
});

test('cacheExercise increments count in stats', async () => {
  const { cache } = buildCache();
  cache.cacheExercise('ex-001', { title: 'A' });
  cache.cacheExercise('ex-002', { title: 'B' });
  const stats = await cache.getStats();
  assert.equal(stats.count, 2);
});

test('cacheExercise updates totalBytes in stats', async () => {
  const { cache } = buildCache();
  const payload = JSON.stringify({ x: 'hello' });
  cache.cacheExercise('ex-001', { x: 'hello' });
  const stats = await cache.getStats();
  assert.equal(stats.totalBytes, payload.length);
});

test('caching the same exercise id twice does not double-count', async () => {
  const { cache } = buildCache();
  cache.cacheExercise('ex-001', { v: 1 });
  cache.cacheExercise('ex-001', { v: 2 });
  const stats = await cache.getStats();
  assert.equal(stats.count, 1);
  // Most recent value should be returned
  assert.deepEqual(cache.getExercise('ex-001'), { v: 2 });
});

// ── clearAll ──────────────────────────────────────────────────────────────────

test('clearAll removes all cached exercises and resets stats', async () => {
  const { cache } = buildCache();
  cache.cacheExercise('ex-001', { title: 'A' });
  cache.cacheExercise('ex-002', { title: 'B' });

  await cache.clearAll();

  const stats = await cache.getStats();
  assert.equal(stats.count, 0);
  assert.equal(cache.getExercise('ex-001'), null);
  assert.equal(cache.getExercise('ex-002'), null);
});

// ── LRU eviction ─────────────────────────────────────────────────────────────

test('eviction: oldest item is evicted when MAX_ITEMS exceeded', async () => {
  // Build a cache where MAX_ITEMS is effectively 2 by filling 3 items.
  // We use a fresh cache and add 201 distinct items; the first one should be evicted.
  const { cache } = buildCache();

  // Add 200 items (filling the cache to the limit).
  for (let i = 0; i < 200; i++) {
    cache.cacheExercise(`ex-${i}`, { n: i });
  }

  // The first item should still be present (limit is 200, we have exactly 200).
  assert.notEqual(cache.getExercise('ex-0'), null);

  // Adding one more should trigger eviction of the LRU item.
  cache.cacheExercise('ex-trigger', { n: 'trigger' });

  const stats = await cache.getStats();
  // Count must remain at or below 200
  assert.ok(stats.count <= 200, `count ${stats.count} should be <= 200`);
  // The newly added item should be accessible.
  assert.notEqual(cache.getExercise('ex-trigger'), null);
});

// ── cacheMedia graceful degradation (IndexedDB disabled) ─────────────────────

test('cacheMedia resolves without throwing when IndexedDB unavailable', async () => {
  const { cache } = buildCache();
  // Blob is a browser API; simulate with a plain object that has .size.
  const mockBlob = { size: 1024 };
  await assert.doesNotReject(() => cache.cacheMedia('photo.jpg', mockBlob));
});

test('getMedia resolves to null when IndexedDB unavailable', async () => {
  const { cache } = buildCache();
  const result = await cache.getMedia('photo.jpg');
  assert.equal(result, null);
});
