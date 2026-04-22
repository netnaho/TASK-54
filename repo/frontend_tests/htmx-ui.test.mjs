/**
 * Frontend unit tests for HTMX UI interaction patterns.
 *
 * Tests the client-side behaviors expected in CareOps HTMX-driven pages:
 *  - HX-Request header detection logic
 *  - HTMX partial vs full-page swap decisions
 *  - Flash cookie reading and display
 *  - Navigation active-state management
 *  - Favorite toggle optimistic update pattern
 *  - Alert severity badge class mapping
 *
 * Run: node --test frontend_tests/htmx-ui.test.mjs
 */
import { test, describe } from 'node:test';
import assert from 'node:assert/strict';

// ── HX-Request detection ──────────────────────────────────────────────────────

describe('HTMX request detection', () => {
  const isHtmxRequest = (headers) => headers['hx-request'] === 'true';

  test('returns true when HX-Request is "true"', () => {
    assert.equal(isHtmxRequest({ 'hx-request': 'true' }), true);
  });

  test('returns false when HX-Request is absent', () => {
    assert.equal(isHtmxRequest({}), false);
  });

  test('returns false when HX-Request is "false"', () => {
    assert.equal(isHtmxRequest({ 'hx-request': 'false' }), false);
  });

  test('is case-sensitive for header value', () => {
    assert.equal(isHtmxRequest({ 'hx-request': 'True' }), false);
    assert.equal(isHtmxRequest({ 'hx-request': '1' }), false);
  });
});

// ── Flash cookie parsing ──────────────────────────────────────────────────────

describe('Flash cookie parsing', () => {
  // Flash cookies are set as URL-encoded strings with a 10-second TTL
  const parseFlash = (cookieStr) => {
    if (!cookieStr) return null;
    try {
      return decodeURIComponent(cookieStr);
    } catch (_) {
      return null;
    }
  };

  test('decodes URL-encoded flash message', () => {
    const encoded = 'Payment%20recorded%20successfully.';
    assert.equal(parseFlash(encoded), 'Payment recorded successfully.');
  });

  test('decodes message with special characters', () => {
    const encoded = 'Config%20updated%20(version%203).';
    assert.equal(parseFlash(encoded), 'Config updated (version 3).');
  });

  test('returns null for empty flash', () => {
    assert.equal(parseFlash(''), null);
    assert.equal(parseFlash(null), null);
    assert.equal(parseFlash(undefined), null);
  });

  test('handles plain (non-encoded) flash values', () => {
    assert.equal(parseFlash('Session published.'), 'Session published.');
  });
});

// ── Alert severity badge CSS class mapping ────────────────────────────────────

describe('Alert severity badge classes', () => {
  const severityClass = (severity) => {
    const map = {
      low:      'badge-secondary',
      medium:   'badge-warning',
      high:     'badge-danger',
      critical: 'badge-dark',
    };
    return map[severity] || 'badge-secondary';
  };

  test('low severity maps to badge-secondary', () => {
    assert.equal(severityClass('low'), 'badge-secondary');
  });

  test('medium severity maps to badge-warning', () => {
    assert.equal(severityClass('medium'), 'badge-warning');
  });

  test('high severity maps to badge-danger', () => {
    assert.equal(severityClass('high'), 'badge-danger');
  });

  test('critical severity maps to badge-dark', () => {
    assert.equal(severityClass('critical'), 'badge-dark');
  });

  test('unknown severity falls back to badge-secondary', () => {
    assert.equal(severityClass('unknown'), 'badge-secondary');
    assert.equal(severityClass(''), 'badge-secondary');
  });
});

// ── Exercise favorite toggle optimistic update ────────────────────────────────

describe('Favorite toggle optimistic state', () => {
  // Simulate client-side optimistic toggle before server confirmation
  function buildFavoriteToggle() {
    let isFavorite = false;
    return {
      get isFavorite() { return isFavorite; },
      toggle()    { isFavorite = !isFavorite; return isFavorite; },
      confirm(ok) { if (!ok) isFavorite = !isFavorite; },  // rollback on failure
    };
  }

  test('toggle switches state from false to true', () => {
    const fav = buildFavoriteToggle();
    assert.equal(fav.toggle(), true);
  });

  test('toggle switches state from true to false', () => {
    const fav = buildFavoriteToggle();
    fav.toggle(); // → true
    assert.equal(fav.toggle(), false);
  });

  test('confirm(true) keeps new state', () => {
    const fav = buildFavoriteToggle();
    fav.toggle(); // → true
    fav.confirm(true);
    assert.equal(fav.isFavorite, true);
  });

  test('confirm(false) rolls back optimistic state', () => {
    const fav = buildFavoriteToggle();
    fav.toggle(); // → true (optimistic)
    fav.confirm(false); // server rejected
    assert.equal(fav.isFavorite, false, 'state should be rolled back on failure');
  });
});

// ── HTMX partial swap target selection ───────────────────────────────────────

describe('HTMX hx-target selection helpers', () => {
  // Validate that hx-target values follow the CareOps convention
  const isValidTarget = (target) =>
    target.startsWith('#') || target === 'this' || target === 'closest tr' ||
    target.startsWith('[') || target === 'body';

  test('#id is a valid HTMX target', () => {
    assert.equal(isValidTarget('#main-content'), true);
  });

  test('"this" is a valid HTMX target', () => {
    assert.equal(isValidTarget('this'), true);
  });

  test('"closest tr" is a valid HTMX target for table row swaps', () => {
    assert.equal(isValidTarget('closest tr'), true);
  });

  test('"body" is a valid HTMX target', () => {
    assert.equal(isValidTarget('body'), true);
  });

  test('bare div is not a valid target in CareOps convention', () => {
    assert.equal(isValidTarget('div'), false);
  });
});

// ── Navigation active state ───────────────────────────────────────────────────

describe('Navigation active state logic', () => {
  const getActivePath = (currentPath) => {
    const segments = currentPath.split('/').filter(Boolean);
    return segments.length > 0 ? '/' + segments[0] : '/';
  };

  test('returns root segment for single-level paths', () => {
    assert.equal(getActivePath('/finance'), '/finance');
    assert.equal(getActivePath('/beds'), '/beds');
    assert.equal(getActivePath('/dashboard'), '/dashboard');
  });

  test('returns first segment for nested paths', () => {
    assert.equal(getActivePath('/finance/payments/new'), '/finance');
    assert.equal(getActivePath('/exams/templates/tmpl-001'), '/exams');
  });

  test('returns / for root path', () => {
    assert.equal(getActivePath('/'), '/');
  });

  test('nav item is active when segment matches', () => {
    const isActive = (navItem, currentPath) => {
      return currentPath === navItem || currentPath.startsWith(navItem + '/');
    };
    assert.equal(isActive('/finance', '/finance'), true);
    assert.equal(isActive('/finance', '/finance/payments/new'), true);
    assert.equal(isActive('/exams', '/exams/sessions/sess-001'), true);
    assert.equal(isActive('/finance', '/exams'), false);
  });
});

// ── Score display helper ──────────────────────────────────────────────────────

describe('Checkpoint score display', () => {
  const scoreLabel = (score) => {
    if (score >= 5) return 'Excellent';
    if (score >= 4) return 'Good';
    if (score >= 3) return 'Fair';
    if (score >= 2) return 'Poor';
    return 'Critical';
  };

  test('score 5 is Excellent', () => assert.equal(scoreLabel(5), 'Excellent'));
  test('score 4 is Good',      () => assert.equal(scoreLabel(4), 'Good'));
  test('score 3 is Fair',      () => assert.equal(scoreLabel(3), 'Fair'));
  test('score 2 is Poor',      () => assert.equal(scoreLabel(2), 'Poor'));
  test('score 1 is Critical',  () => assert.equal(scoreLabel(1), 'Critical'));
});

// ── Date formatting helpers ───────────────────────────────────────────────────

describe('Date formatting for display', () => {
  const fmtDate = (isoStr) => {
    if (!isoStr) return '—';
    const d = new Date(isoStr);
    if (isNaN(d.getTime())) return '—';
    return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
  };

  test('formats ISO date string', () => {
    const result = fmtDate('2024-04-01T10:00:00Z');
    assert.ok(result.includes('2024'), `should include year 2024, got: ${result}`);
    assert.ok(result.includes('Apr') || result.includes('4'), `should include April, got: ${result}`);
  });

  test('returns dash for null/undefined', () => {
    assert.equal(fmtDate(null), '—');
    assert.equal(fmtDate(''), '—');
    assert.equal(fmtDate(undefined), '—');
  });

  test('returns dash for invalid date string', () => {
    assert.equal(fmtDate('not-a-date'), '—');
  });
});
