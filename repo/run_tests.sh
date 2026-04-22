#!/usr/bin/env sh
# ============================================================
# run_tests.sh — CareOps test runner (Docker-contained)
#
# Prerequisites: Docker Engine 24+ with Docker Compose v2.
# No host Go, Node.js, or npm required.
#
# Suites:
#   1. Unit & API tests   — golang:1.22-alpine
#   2. Frontend unit tests — node:18-alpine  (no npm)
#   3. Browser E2E tests   — Playwright in official mcr.microsoft.com/playwright image
#                            against a live careops container (temp SQLite DB, discarded on exit)
#
# Usage:
#   ./run_tests.sh
# ============================================================
set -e

PASS=0
FAIL=0

# Resolve the absolute path of the repo root so volume mounts work
# regardless of where the script is invoked from.
REPO="$(cd "$(dirname "$0")" && pwd)"

run_suite() {
  SUITE="$1"
  shift
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  $SUITE"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if "$@"; then
    PASS=$((PASS + 1))
    echo "  ✓ $SUITE PASSED"
  else
    FAIL=$((FAIL + 1))
    echo "  ✗ $SUITE FAILED"
  fi
}

echo "CareOps — Test Suite Runner"
echo "$(date)"

# ── Go tests (unit + API integration) ────────────────────────────────────────
run_suite "Unit & API Tests (Go)" \
  docker run --rm \
    -v "${REPO}:/src" \
    -w /src \
    golang:1.22-alpine \
    go test ./unit_tests/... ./API_tests/... \
      -count=1 -timeout 120s

# ── Frontend unit tests (Node.js, no npm) ────────────────────────────────────
run_suite "Frontend Tests (Node.js)" \
  docker run --rm \
    -v "${REPO}:/src" \
    -w /src \
    node:18-alpine \
    node --test frontend_tests/*.mjs

# ── Browser E2E tests (Playwright) ───────────────────────────────────────────
# Starts a live careops container with a fresh temp DB (discarded on exit),
# then runs Playwright browsers against it.  No host npm required.
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Browser E2E Tests (Playwright)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
E2E_EXIT=0
docker compose -f "${REPO}/docker-compose.e2e.yml" \
  --project-directory "${REPO}" \
  run --rm playwright || E2E_EXIT=$?
# Always tear down the careops-e2e container and any anonymous volumes.
docker compose -f "${REPO}/docker-compose.e2e.yml" \
  --project-directory "${REPO}" \
  down -v --remove-orphans 2>/dev/null || true
if [ "$E2E_EXIT" -eq 0 ]; then
  PASS=$((PASS + 1))
  echo "  ✓ Browser E2E Tests PASSED"
else
  FAIL=$((FAIL + 1))
  echo "  ✗ Browser E2E Tests FAILED (exit $E2E_EXIT)"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Results: $PASS passed, $FAIL failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "All test suites passed."
