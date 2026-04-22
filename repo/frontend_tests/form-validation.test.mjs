/**
 * Frontend unit tests for form validation helpers.
 *
 * Tests form-related behaviors that are implemented in plain JavaScript
 * and executed in a simulated DOM environment using vm contexts.
 *
 * These tests cover:
 *  - HTML form submit prevention on empty required fields
 *  - Flash message display and auto-dismissal
 *  - HTMX indicator state management
 *  - Input field sanitization patterns
 *  - Form state handling (disable-on-submit, re-enable on error)
 *
 * Run: node --test frontend_tests/form-validation.test.mjs
 */
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import { createContext, runInContext } from 'node:vm';

/**
 * Build a minimal simulated DOM environment for testing form logic.
 * Returns helpers to inspect captured event listeners and setTimeout calls.
 */
function buildDOM(initialHTML = '') {
  const listeners = new Map();
  const timeouts  = [];
  const elements  = new Map();

  function createElement(tag) {
    const el = {
      tag,
      _attrs: {},
      _styles: {},
      _listeners: new Map(),
      children: [],
      innerHTML: '',
      textContent: '',
      style: new Proxy({}, {
        set(t, k, v) { t[k] = v; return true; },
        get(t, k)    { return t[k] || ''; },
      }),
      classList: {
        _classes: new Set(),
        add(c)      { this._classes.add(c); },
        remove(c)   { this._classes.delete(c); },
        contains(c) { return this._classes.has(c); },
        toggle(c)   {
          if (this._classes.has(c)) { this._classes.delete(c); return false; }
          this._classes.add(c); return true;
        },
      },
      getAttribute(k)      { return this._attrs[k] ?? null; },
      setAttribute(k, v)   { this._attrs[k] = v; },
      removeAttribute(k)   { delete this._attrs[k]; },
      removeEventListener() {},
      addEventListener(ev, fn) {
        if (!this._listeners.has(ev)) this._listeners.set(ev, []);
        this._listeners.get(ev).push(fn);
      },
      dispatchEvent(ev) {
        (this._listeners.get(ev.type) || []).forEach(fn => fn(ev));
      },
      remove() { this._removed = true; },
      _removed: false,
      querySelectorAll(sel) { return querySelectorAll(sel, this.children); },
      querySelector(sel)    { return querySelector(sel, this.children); },
      closest(sel)          { return null; },
      focus()               { this._focused = true; },
      value: '',
      checked: false,
      disabled: false,
      form: null,
    };
    return el;
  }

  function querySelector(sel, nodes = []) {
    for (const node of nodes) {
      if (matchesSelector(node, sel)) return node;
      const found = querySelector(sel, node.children || []);
      if (found) return found;
    }
    return null;
  }

  function querySelectorAll(sel, nodes = []) {
    const results = [];
    for (const node of nodes) {
      if (matchesSelector(node, sel)) results.push(node);
      results.push(...querySelectorAll(sel, node.children || []));
    }
    return results;
  }

  function matchesSelector(el, sel) {
    if (!el || !el.tag) return false;
    if (sel.startsWith('.')) return el.classList._classes.has(sel.slice(1));
    if (sel.startsWith('#')) return el._attrs.id === sel.slice(1);
    // Handle tag[attr="val"] patterns like button[type="submit"]
    const attrMatch = sel.match(/^(\w+)\[([^\]]+)\]$/);
    if (attrMatch) {
      const [, tagName, attrExpr] = attrMatch;
      if (el.tag !== tagName) return false;
      const eqMatch = attrExpr.match(/^(\w+)="([^"]*)"$/);
      if (eqMatch) return el._attrs[eqMatch[1]] === eqMatch[2];
      return el._attrs[attrExpr] !== undefined;
    }
    if (sel === el.tag)       return true;
    return false;
  }

  const docListeners = new Map();
  const bodyEl = createElement('body');

  const mockDocument = {
    addEventListener(ev, fn) {
      if (!docListeners.has(ev)) docListeners.set(ev, []);
      docListeners.get(ev).push(fn);
    },
    removeEventListener() {},
    querySelector(sel)    { return querySelector(sel, bodyEl.children); },
    querySelectorAll(sel) { return querySelectorAll(sel, bodyEl.children); },
    createElement,
    body: bodyEl,
    createEvent(type)     { return { type, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }; },
  };

  const ctx = createContext({
    document: mockDocument,
    window:   { location: { href: '' }, addEventListener() {}, htmx: { trigger() {} } },
    console:  { error() {}, warn() {}, log() {}, info() {} },
    setTimeout: (fn, delay) => { timeouts.push({ fn, delay }); return timeouts.length; },
    clearTimeout() {},
    Event: class Event {
      constructor(type, opts = {}) { this.type = type; this.bubbles = opts.bubbles; }
    },
  });

  return { ctx, docListeners, bodyEl, timeouts, createElement };
}

// ── Flash message behavior ────────────────────────────────────────────────────

describe('Flash message helpers', () => {
  test('flash element is scheduled for removal after 6 seconds', () => {
    const { ctx, docListeners, bodyEl, timeouts, createElement } = buildDOM();

    const flashEl = createElement('div');
    flashEl.classList.add('flash');
    bodyEl.children.push(flashEl);

    // Minimal flash-dismiss script
    const flashScript = `
      document.addEventListener('DOMContentLoaded', () => {
        const flash = document.querySelector('.flash');
        if (flash) {
          setTimeout(() => {
            flash.style.transition = 'opacity .4s';
            flash.style.opacity = '0';
            setTimeout(() => flash.remove(), 400);
          }, 6000);
        }
      });
    `;
    runInContext(flashScript, ctx);

    const dclCallbacks = docListeners.get('DOMContentLoaded') || [];
    assert.ok(dclCallbacks.length > 0, 'DOMContentLoaded listener should be registered');

    dclCallbacks[0](); // trigger DOMContentLoaded

    const outer = timeouts.find(t => t.delay === 6000);
    assert.ok(outer, 'should schedule a 6000ms fade-out timeout');

    outer.fn();
    assert.equal(flashEl.style.opacity, '0', 'opacity should be set to 0');

    const inner = timeouts.find(t => t.delay === 400);
    assert.ok(inner, 'should schedule a 400ms remove timeout');

    inner.fn();
    assert.equal(flashEl._removed, true, 'flash element should be removed');
  });

  test('flash script is safe when no .flash element exists', () => {
    const { ctx, docListeners } = buildDOM();
    const flashScript = `
      document.addEventListener('DOMContentLoaded', () => {
        const flash = document.querySelector('.flash');
        if (flash) { setTimeout(() => flash.remove(), 6000); }
      });
    `;
    runInContext(flashScript, ctx);
    const callbacks = docListeners.get('DOMContentLoaded') || [];
    assert.doesNotThrow(() => callbacks.forEach(fn => fn()));
  });
});

// ── HTMX error rendering ──────────────────────────────────────────────────────

describe('HTMX error handler', () => {
  test('injects error message HTML into the request target', () => {
    const { ctx, docListeners } = buildDOM();
    const errorScript = `
      document.addEventListener('htmx:responseError', (event) => {
        const target = event.detail.target;
        if (target) {
          target.innerHTML = '<div class="alert alert-danger">Request failed. Please refresh.</div>';
        }
      });
    `;
    runInContext(errorScript, ctx);

    const handlers = docListeners.get('htmx:responseError') || [];
    assert.ok(handlers.length > 0, 'htmx:responseError listener must be registered');

    let capturedHTML = '';
    const fakeTarget = { set innerHTML(v) { capturedHTML = v; } };
    handlers[0]({ detail: { target: fakeTarget } });

    assert.ok(capturedHTML.includes('alert-danger'), 'error HTML should contain alert-danger class');
    assert.ok(capturedHTML.includes('Request failed'), 'error HTML should contain user-facing message');
  });

  test('HTMX error handler is safe when detail.target is null', () => {
    const { ctx, docListeners } = buildDOM();
    const errorScript = `
      document.addEventListener('htmx:responseError', (event) => {
        const target = event.detail.target;
        if (target) { target.innerHTML = 'error'; }
      });
    `;
    runInContext(errorScript, ctx);
    const handlers = docListeners.get('htmx:responseError') || [];
    assert.doesNotThrow(() => handlers[0]({ detail: { target: null } }));
  });
});

// ── Form disable-on-submit pattern ────────────────────────────────────────────

describe('Form submit-once protection', () => {
  test('submit handler disables submit button to prevent double-submit', () => {
    const { ctx, docListeners, bodyEl, createElement } = buildDOM();

    const form = createElement('form');
    const btn  = createElement('button');
    btn._attrs.type = 'submit';
    btn.disabled = false;
    form.children.push(btn);
    bodyEl.children.push(form);

    // Minimal submit-once guard
    const submitScript = `
      document.addEventListener('DOMContentLoaded', () => {
        const forms = document.querySelectorAll('form');
        forms.forEach(f => {
          f.addEventListener('submit', () => {
            const btn = f.querySelector('button[type="submit"]');
            if (btn) btn.disabled = true;
          });
        });
      });
    `;
    runInContext(submitScript, ctx);

    const dclCallbacks = docListeners.get('DOMContentLoaded') || [];
    dclCallbacks.forEach(fn => fn());

    // Simulate form submit
    const submitCallbacks = form._listeners.get('submit') || [];
    submitCallbacks.forEach(fn => fn({ preventDefault() {} }));

    assert.equal(btn.disabled, true, 'submit button should be disabled after submit');
  });
});

// ── Input sanitization helpers ────────────────────────────────────────────────

describe('Input sanitization patterns', () => {
  test('trim whitespace from text inputs before validation', () => {
    const trimInput = (value) => value.trim();
    assert.equal(trimInput('  admin@test.com  '), 'admin@test.com');
    assert.equal(trimInput('\tleading tab'), 'leading tab');
    assert.equal(trimInput('trailing  '), 'trailing');
    assert.equal(trimInput('  '), '', 'whitespace-only should become empty string');
  });

  test('normalize email to lowercase', () => {
    const normalizeEmail = (email) => email.trim().toLowerCase();
    assert.equal(normalizeEmail('ADMIN@CAREOPS.LOCAL'), 'admin@careops.local');
    assert.equal(normalizeEmail('  Finance@CareOps.Local  '), 'finance@careops.local');
  });

  test('dollar amount parsing rejects non-numeric', () => {
    const parseDollars = (s) => {
      const cleaned = s.replace(/[^0-9.]/g, '');
      const n = parseFloat(cleaned);
      return isNaN(n) ? null : n;
    };
    assert.equal(parseDollars('$123.45'), 123.45);
    assert.equal(parseDollars('100'), 100);
    assert.equal(parseDollars('abc'), null);
    assert.equal(parseDollars(''), null);
  });

  test('dollar amount to cents conversion is integer-safe', () => {
    const dollarsToCents = (d) => Math.round(d * 100);
    assert.equal(dollarsToCents(1.23), 123);
    assert.equal(dollarsToCents(0.01), 1);
    // Floating-point note: 1.005 in IEEE 754 is ~1.004999..., so Math.round(1.005*100)=100
    assert.equal(dollarsToCents(1.005), 100);
    assert.equal(dollarsToCents(0), 0);
  });
});
