/**
 * Unit tests for web/static/js/app.js
 * Uses Node.js built-in test runner (node:test) — no npm required.
 * Runs in a vm context with mocked browser globals; no real DOM needed.
 *
 * Run: node --test frontend_tests/app.test.mjs
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createContext, runInContext } from 'node:vm';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const __dir = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(join(__dir, '../web/static/js/app.js'), 'utf8');

/**
 * Build a lightweight browser-like context and load app.js into it.
 * Returns an object with:
 *   - listeners: Map<eventName, Function[]>  (captured via document.addEventListener)
 *   - timeouts:  Array<{delay, fn}>          (captured via setTimeout)
 */
function loadApp() {
  const listeners = new Map();
  const timeouts  = [];

  const mockDocument = {
    addEventListener(eventName, fn) {
      if (!listeners.has(eventName)) listeners.set(eventName, []);
      listeners.get(eventName).push(fn);
    },
    querySelector(selector) { return null; },
  };

  const ctx = createContext({
    document:  mockDocument,
    console:   { error() {}, warn() {}, log() {} },
    setTimeout: (fn, delay) => { timeouts.push({ fn, delay }); },
  });

  runInContext(appSource, ctx);
  return { listeners, timeouts };
}

// ── DOMContentLoaded / flash dismiss ─────────────────────────────────────────

test('app.js registers a DOMContentLoaded listener', () => {
  const { listeners } = loadApp();
  assert.ok(listeners.has('DOMContentLoaded'), 'should have DOMContentLoaded listener');
});

test('flash dismiss: no-op when querySelector returns null', () => {
  const { listeners } = loadApp();
  const [callback] = listeners.get('DOMContentLoaded');
  // If document.querySelector returns null, callback must not throw.
  assert.doesNotThrow(() => callback());
});

test('flash dismiss: schedules outer setTimeout(6000) when flash element exists', () => {
  const listeners = new Map();
  const timeouts  = [];

  let removed = false;
  const flashEl = {
    style: {},
    remove: () => { removed = true; },
  };

  const ctx = createContext({
    document: {
      addEventListener(name, fn) {
        if (!listeners.has(name)) listeners.set(name, []);
        listeners.get(name).push(fn);
      },
      querySelector: (sel) => sel === '.flash' ? flashEl : null,
    },
    console:   { error() {}, warn() {}, log() {} },
    setTimeout: (fn, delay) => { timeouts.push({ fn, delay }); return timeouts.length - 1; },
  });

  runInContext(appSource, ctx);
  const [callback] = listeners.get('DOMContentLoaded');
  callback(); // trigger the listener

  const outer = timeouts.find(t => t.delay === 6000);
  assert.ok(outer, 'should schedule a 6000 ms timeout');

  // Fire the outer timeout — should set opacity and schedule inner timeout.
  outer.fn();
  assert.equal(flashEl.style.opacity, '0');
  assert.equal(flashEl.style.transition, 'opacity .4s');

  const inner = timeouts.find(t => t.delay === 400);
  assert.ok(inner, 'should schedule a 400 ms remove timeout');

  // Fire the inner timeout — element should be removed.
  inner.fn();
  assert.equal(removed, true);
});

// ── htmx:responseError handler ───────────────────────────────────────────────

test('app.js registers an htmx:responseError listener', () => {
  const { listeners } = loadApp();
  assert.ok(listeners.has('htmx:responseError'), 'should have htmx:responseError listener');
});

test('htmx:responseError injects error markup into event.detail.target', () => {
  const { listeners } = loadApp();
  const [handler] = listeners.get('htmx:responseError');

  let capturedHTML = '';
  const fakeTarget = {
    set innerHTML(v) { capturedHTML = v; },
  };

  handler({ detail: { target: fakeTarget } });
  assert.match(capturedHTML, /alert-danger/);
  assert.match(capturedHTML, /Request failed/);
});

test('htmx:responseError: no-op when target is null', () => {
  const { listeners } = loadApp();
  const [handler] = listeners.get('htmx:responseError');
  assert.doesNotThrow(() => handler({ detail: { target: null } }));
});
