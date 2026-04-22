# CareOps — Clinic Administration Suite

**Type:** fullstack

Production-grade local-network-only administration system for a rehab and long-term care facility.
Full-stack Go + HTMX with real auth, real migrations, real CRUD, settlement shifts, card-batch imports, exam scheduling, config versioning, and diagnostics exports.

---

## Project Overview

CareOps covers the core operational domains of a long-term care facility:

- **Admissions & Beds** — resident intake, bed assignment, discharge
- **Service Delivery** — therapy orders (PT/OT/ST), session execution records, care alerts, quality checkpoints
- **Exercise Library** — clinical exercise catalog with tags and clinician favorites
- **Competency Exams** — staff certification templates, session scheduling, conflict detection
- **Finance** — payment recording, settlement shifts, card batch CSV imports, refunds, CSV/XLSX exports
- **Reports** — scheduled and on-demand report definitions and run history
- **Audit Log** — immutable system-wide action log with entity/date filters
- **System** — users/roles, config versioning with snapshots/rollback, diagnostics ZIP exports, background job history

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| Web framework | [Fiber v2](https://github.com/gofiber/fiber) |
| Templates | `html/template` via `gofiber/template/html/v2` |
| UI enhancement | [HTMX 1.9.12](https://htmx.org) (vendored at Docker build time) |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| Passwords | `golang.org/x/crypto/bcrypt` (cost 10) |
| Logging | `log/slog` (structured JSON) |
| Encryption | AES-256-GCM for sensitive payment references (opt-in via `FIELD_ENCRYPT_KEY`) |
| Container | Docker multi-stage build; runtime `alpine:3.20` |

---

## Architecture Summary

```
Browser → Fiber (Go) → SQLite
            ↓
      html/template (server-rendered)
      HTMX (fragment swaps on HX-Request)
```

- Single binary; no external services; no internet dependency at runtime.
- Bootstrap order: load config → open SQLite → run migrations → run seeds → start HTTP server.
- All DB migrations tracked via `schema_migrations`; seeds via `schema_seeds` (both idempotent).
- Session tokens stored as random 32-byte hex values; cookie is HTTP-only.
- Each module is layered: `domain → repository → service → handler → policy`.
- Idempotency keys prevent double-POSTs on payment creation, refunds, shift open/close, card-batch import, alert/checkpoint creation, and export generation.
- Optimistic concurrency control via `row_version` on write-likely entities (payments, service orders, exam sessions, config entries, report definitions).

Full component diagram: [`docs/architecture.md`](docs/architecture.md)

---

## Folder Structure

```
.
├── cmd/server/main.go                  Entrypoint
├── internal/
│   ├── app/                            Bootstrap + Fiber app + router
│   ├── config/                         Env-driven configuration
│   ├── platform/
│   │   ├── database/                   SQLite open, migrator, seeder
│   │   ├── logger/                     slog factory
│   │   ├── template/                   html/v2 engine + template funcs
│   │   └── middleware/                 request_id, access_log, recovery
│   ├── shared/
│   │   ├── httpx/                      PageData, RenderPage, CurrentUser
│   │   ├── passwords/                  bcrypt Hash/Verify
│   │   ├── idgen/                      UUIDv4 generator
│   │   ├── timeutil/                   Clock interface, NowISO()
│   │   ├── crypto/                     AES-256-GCM encrypt/decrypt
│   │   ├── csvutil/                    CSV file writer
│   │   ├── xlsxutil/                   Pure-Go XLSX writer (archive/zip + OOXML)
│   │   └── ziputil/                    ZIP archive builder for diagnostics
│   └── modules/
│       ├── auth/                       Login, logout, session resolution, RBAC
│       ├── users/                      Staff user and role management
│       ├── beds/                       Bed registry and occupancy status
│       ├── admissions/                 Resident intake and discharge
│       ├── occupancy/                  Daily occupancy snapshots
│       ├── service_delivery/           Service orders, execution records, alerts, checkpoints
│       ├── exercise_library/           Clinical exercise catalog
│       ├── exams/                      Competency exam templates, sessions, conflicts
│       ├── finance/                    Payments, shifts, refunds, batches, exports
│       ├── reports/                    Scheduled reports and run history
│       ├── audit/                      Immutable audit log
│       ├── jobs/                       Background job history
│       ├── config_versions/            Versioned facility configuration with snapshots
│       └── diagnostics/               DB stats and ZIP export packages
├── migrations/
│   ├── 0001_initial_schema.sql         Core tables: users, sessions, residents, beds, admissions
│   ├── 0002_indexes.sql                Non-PK performance indexes
│   ├── 0003_security_foundation.sql    Idempotency keys, audit logs, config versions
│   ├── 0004_operations.sql             Service orders, alerts, checkpoints, exercise library
│   ├── 0005_exam_scheduler.sql         Competency exam templates, sessions, candidates, conflicts
│   └── 0006_finance_phase4.sql         Payments, refunds, settlement shifts, card batches
├── seeds/
│   ├── 0001_roles.sql                  8 system roles
│   ├── 0002_users.sql                  8 demo staff users (bcrypt hashed)
│   ├── 0003_user_roles.sql             Role assignments
│   ├── 0004_beds.sql                   18 beds (3 wings)
│   ├── 0005_residents_admissions.sql   10 residents, admissions, care alerts
│   ├── 0006_exercise_library.sql       8 exercises, 10 tags
│   ├── 0007_exam_templates.sql         3 competency templates, 2 exam sessions
│   ├── 0008_config_finance.sql         Config entries, payment records, service orders, audit
│   ├── 0009_operations_phase2.sql      Service execution records, checkpoints
│   ├── 0010_exam_scheduler_phase3.sql  Exam candidates, conflicts
│   └── 0011_finance_phase4.sql         Settlement shifts, card batches, refunds
├── web/
│   ├── templates/                      Server-side HTML templates (one dir per module)
│   └── static/                         CSS, JS, vendored HTMX
├── unit_tests/                         Domain logic and utility tests
├── API_tests/                          Route-level integration tests via Fiber.Test()
├── docs/
│   ├── requirements_traceability.md    Requirement → implementation mapping
│   └── architecture.md                 Component diagram + design decisions
├── Dockerfile
├── docker-compose.yml
└── run_tests.sh
```

---

## Startup Instructions

### Prerequisites

- Docker Engine 24+ and Docker Compose v2.

### Start

```sh
docker-compose up
```

> **Development mode by default.** The supplied `docker-compose.yml` sets
> `APP_ENV=development` so the server starts without a `FIELD_ENCRYPT_KEY`.
> For a production deployment, set `APP_ENV=production` **and** supply
> `FIELD_ENCRYPT_KEY` (a 64-char hex string); startup hard-fails if the key is
> absent in production mode.  See the [Configuration Truth Table](#configuration-truth-table) below.

On first run:
1. Docker builds the image (downloads Go deps + HTMX at build time — internet required at build time only).
2. The container starts, runs all migrations, then runs all seeds.
3. The server begins serving on port **8080**.

### Access

Open `http://localhost:8080` in any browser on the local network.

### Stop

```sh
docker-compose down
```

Data persists in named Docker volumes and survives restarts.

---

## Exposed Ports

| Port | Protocol | Purpose |
|---|---|---|
| `8080` | HTTP | CareOps web application |

> No TLS — this system is designed for an isolated local facility network.
> Add a reverse proxy (nginx/Caddy with a self-signed cert) for TLS before deploying outside a trusted LAN.

---

## Volume & Storage Paths

| Docker Volume | Container Path | Contents |
|---|---|---|
| `careops_db` | `/data/db/` | SQLite database file (`careops.db`) |
| `careops_media` | `/data/media/` | Uploaded media files |
| `careops_exports` | `/data/exports/` | Generated CSV/XLSX report files |
| `careops_diagnostics` | `/data/diagnostics/` | Diagnostic ZIP export bundles |
| `careops_config_snapshots` | `/data/config_snapshots/` | Config version JSON snapshots |
| `careops_logs` | `/data/logs/` | Runtime application log (`app.log`); last 5 000 bytes bundled into diagnostic ZIPs |

---

## Seeded Demo Users & Passwords

All demo users are seeded automatically on first startup.

| Email | Password | Role |
|---|---|---|
| `admin@careops.local` | `Admin!234567` | admin |
| `auditor@careops.local` | `Auditor!2345` | auditor |
| `nurse@careops.local` | `Nurse!234567` | nurse |
| `front.desk@careops.local` | `FrontDesk!2345` | front_desk |
| `therapist@careops.local` | `Therapist!2345` | therapist |
| `aide@careops.local` | `Aide!2345678` | aide |
| `training@careops.local` | `Training!2345` | training_coordinator |
| `finance@careops.local` | `Finance!2345` | finance_clerk |

Passwords are bcrypt cost-10 hashes stored in `seeds/0002_users.sql`.
**Change all passwords before deploying to a real facility.**

---

## Route Reference

### Auth

| Route | Description |
|---|---|
| `GET /login` | Login form |
| `POST /login` | Authenticate; sets `careops_session` cookie |
| `POST /logout` | Invalidate session |

### Core

| Route | Roles | Description |
|---|---|---|
| `GET /` | all | Root redirect → `/dashboard` |
| `GET /dashboard` | all | At-a-glance counters from DB |
| `GET /healthz` | — | Liveness probe |
| `GET /readyz` | — | Readiness probe (DB reachable) |

### Users

| Route | Roles | Description |
|---|---|---|
| `GET /users` | admin | Staff user list with roles |
| `GET /users/new` | admin | New user creation form |
| `POST /users` | admin | Create staff user (password policy enforced) |
| `GET /users/:id/reset-password` | admin | Password reset form |
| `POST /users/:id/reset-password` | admin | Reset user password (policy enforced) |

### Beds & Admissions

| Route | Roles | Description |
|---|---|---|
| `GET /beds` | admin, nurse, front_desk | Bed registry |
| `GET /beds/:id` | admin, nurse, front_desk | Bed detail |
| `GET /admissions` | admin, nurse, front_desk | Resident list |
| `GET /admissions/:id` | admin, nurse, front_desk | Resident detail |
| `GET /occupancy` | admin, nurse, front_desk | Occupancy snapshot |

### Service Delivery

| Route | Roles | Description |
|---|---|---|
| `GET /service-delivery` | admin, nurse, therapist, aide, front_desk | Dashboard: orders, alerts, metrics |
| `GET /service-delivery/orders/:id` | admin, nurse, therapist, aide, front_desk | Service order detail |
| `GET /service-delivery/alerts/new` | admin, nurse, therapist, aide, front_desk | Alert create form |
| `POST /service-delivery/alerts` | admin, nurse, therapist, aide, front_desk | Create alert (idempotency-keyed) |
| `GET /service-delivery/checkpoints/new` | admin, nurse, therapist, aide, front_desk | Checkpoint create form |
| `POST /service-delivery/checkpoints` | admin, nurse, therapist, aide, front_desk | Create checkpoint |
| `GET /media/:filename` | authenticated | Serve local media (path-traversal protected) |

### Exercise Library

| Route | Roles | Description |
|---|---|---|
| `GET /exercise-library` | admin, therapist, training_coordinator, nurse, aide | Exercise grid with filters (category, difficulty, body part, tags, text search) |
| `GET /exercise-library/:id` | admin, therapist, training_coordinator, nurse, aide | Exercise detail with device-cache JS |
| `POST /exercise-library/:id/favorite` | admin, therapist, training_coordinator, nurse, aide | Toggle favorite (HTMX fragment) |

### Competency Exams (Phase 3)

| Route | Roles | Description |
|---|---|---|
| `GET /exams` | training_coordinator, admin | Exam list |
| `GET /exams/scheduler` | training_coordinator, admin | Scheduler view |
| `GET /exams/templates/new` | training_coordinator, admin | New template form |
| `POST /exams/templates` | training_coordinator, admin | Create template |
| `GET /exams/templates/:id` | training_coordinator, admin | Template detail |
| `POST /exams/templates/:id` | training_coordinator, admin | Update template |
| `GET /exams/sessions/new` | training_coordinator, admin | New session form |
| `POST /exams/sessions` | training_coordinator, admin | Create session |
| `GET /exams/sessions/:id` | training_coordinator, admin | Session detail |
| `POST /exams/sessions/:id/block` | training_coordinator, admin | Block/unblock session |
| `POST /exams/sessions/:id/publish` | training_coordinator, admin | Publish session |
| `POST /exams/sessions/:id/candidates/add` | training_coordinator, admin | Add candidate |
| `POST /exams/sessions/:id/candidates/remove` | training_coordinator, admin | Remove candidate |
| `POST /exams/conflicts/:id/resolve` | training_coordinator, admin | Resolve conflict |

### Finance (Phase 4)

| Route | Roles | Description |
|---|---|---|
| `GET /finance` | finance_clerk, admin | Finance dashboard |
| `GET /finance/payments/new` | finance_clerk, admin | New payment form |
| `POST /finance/payments` | finance_clerk, admin | Record payment (idempotency-keyed; encrypts sensitive refs) |
| `GET /finance/payments/:id` | finance_clerk, admin | Payment detail |
| `GET /finance/payments/:id/refund` | finance_clerk, admin | Issue refund form |
| `POST /finance/payments/:id/refund` | finance_clerk, admin | Process refund |
| `GET /finance/shifts` | finance_clerk, admin | Settlement shift list + open-shift form |
| `POST /finance/shifts` | finance_clerk, admin | Open shift (idempotency-keyed) |
| `GET /finance/shifts/:id` | finance_clerk, admin | Shift detail + close form |
| `POST /finance/shifts/:id/close` | finance_clerk, admin | Close shift (idempotency-keyed) |
| `GET /finance/batches/new` | finance_clerk, admin | Card batch import form |
| `POST /finance/batches` | finance_clerk, admin | Upload and process CSV batch (idempotency-keyed) |
| `GET /finance/batches/:id` | finance_clerk, admin | Batch detail |
| `GET /finance/exports/new` | finance_clerk, admin | Export form |
| `POST /finance/exports` | finance_clerk, admin | Generate CSV or XLSX export |
| `GET /finance/exports/download` | finance_clerk, admin | Download export file (path-traversal protected) |

### Config Versioning (Phase 4)

| Route | Roles | Description |
|---|---|---|
| `GET /config-versions` | admin | Config entries + snapshot list |
| `POST /config-versions/snapshot` | admin | Take JSON snapshot |
| `POST /config-versions/rollback` | admin | Rollback to snapshot |
| `GET /config-versions/:id/edit` | admin | Edit config entry |
| `POST /config-versions/:id` | admin | Update config entry (OCC via row_version) |

### Diagnostics (Phase 4)

| Route | Roles | Description |
|---|---|---|
| `GET /diagnostics` | admin | DB stats + export history |
| `POST /diagnostics/exports` | admin | Generate diagnostic ZIP package (README.txt, db_stats.json, recent_jobs.json, config.json, audit_logs.json, config_snapshots/, app_logs.txt when LOG_PATH is set — defaults to `./data/logs/app.log`) |
| `GET /diagnostics/exports/download` | admin | Download ZIP (path-traversal protected) |

### Reports (Phase 5)

| Route | Roles | Description |
|---|---|---|
| `GET /reports` | admin, auditor | Report definitions and run history |
| `GET /reports/new` | admin, auditor | New report definition form |
| `POST /reports` | admin, auditor | Create report definition (idempotent) |
| `GET /reports/:id/edit` | admin, auditor | Edit report definition form |
| `POST /reports/:id/update` | admin, auditor | Update definition (OCC via row_version) |
| `POST /reports/:id/run` | admin, auditor | Trigger report run (idempotent) |
| `POST /reports/:id/toggle` | admin, auditor | Toggle definition active/inactive |
| `GET /reports/runs/download` | admin, auditor | Download run output file (path-traversal protected) |

Report types: `occupancy`, `service_delivery`, `finance`, `audit`. Output formats: `csv`, `xlsx`.

The optional `parameters` JSON field accepts filter keys applied as WHERE clauses when the report runs: `date_from`, `date_to` (YYYY-MM-DD), `resident_id`, `user_id`, `entity_type`. Example: `{"resident_id":"res-001","date_from":"2025-01-01"}`.

When a `schedule_cron` is set, the in-process scheduler runs the report automatically. Scheduler runs are attributed to the `scheduler` system user.

### System

| Route | Roles | Description |
|---|---|---|
| `GET /audit` | admin, auditor | Audit log with entity/date/resident filters |
| `GET /jobs` | admin | Background job history |

---

## Test Instructions

Tests run against a temporary SQLite file path (not the production database volume).
**Only Docker is required** — no host Go, Node.js, or npm installation needed.

```sh
# Run all suites (Docker-contained)
./run_tests.sh
```

Three suites run in sequence:

| Suite | Docker image | Method |
|---|---|---|
| Unit & API (Go) | `golang:1.22-alpine` | Fiber `app.Test()` in-process — no network socket |
| Frontend unit (JS) | `node:18-alpine` | `node --test` built-in runner, no npm |
| Browser E2E (Playwright) | `mcr.microsoft.com/playwright:v1.44.0-jammy` | Real Chromium browser against a live server |

The runner exits non-zero if any suite fails.

#### Running only the E2E suite

```sh
# Build and start the app (fresh temp DB) + run Playwright, then tear down
docker compose -f docker-compose.e2e.yml run --rm playwright
docker compose -f docker-compose.e2e.yml down -v --remove-orphans
```

The E2E server (`careops-e2e`) uses `DB_PATH=/tmp/careops_e2e.db` — discarded on container exit.

### Test coverage summary

| Suite | File | Covers |
|---|---|---|
| unit | `config_test.go` | Env loading + defaults |
| unit | `password_test.go` | bcrypt Hash/Verify |
| unit | `crypto_test.go` | AES-GCM encrypt/decrypt |
| unit | `migrator_test.go` | Migration application + idempotency |
| unit | `finance_test.go` | Domain validation, parseDollars, SensitiveMethods |
| unit | `idempotency_test.go` | Idempotency key store |
| unit | `optimistic_lock_test.go` | row_version OCC enforcement |
| unit | `timeliness_test.go` | 15-min threshold rule |
| unit | `occupancy_test.go` | Occupancy snapshot helpers |
| unit | `exam_scheduler_test.go` | Conflict detection |
| unit | `validation_test.go` | CreatePayment, CreateRefund, ExportParams |
| unit | `csvutil_test.go` | CSV file creation |
| unit | `xlsxutil_test.go` | XLSX file creation |
| unit | `ziputil_test.go` | ZIP archive entries |
| API | `health_test.go` | `/healthz`, `/readyz` |
| API | `auth_test.go` | Login success + failure, logout |
| API | `protected_test.go` | Unauthenticated redirect; authenticated 200 |
| API | `security_test.go` | SQL injection, XSS, CSRF edge cases |
| API | `operations_test.go` | Service delivery RBAC + create flows |
| API | `exam_scheduler_test.go` | Exam template/session lifecycle |
| API | `finance_test.go` | Payment, shift, batch, export RBAC + flows |
| API | `config_versions_test.go` | Config edit, snapshot, rollback lifecycle |
| API | `diagnostics_test.go` | Diagnostics RBAC + generate + download + ZIP content verification |
| API | `reports_test.go` | Reports RBAC, create/edit/run/toggle/download, idempotency, all report types |
| API | `audit_test.go` | Audit filter, post-action audit entries |
| API | `audit_resident_test.go` | `resident_id` filter on audit log — positive match, exclusion, combined with date/entity_type, page renders input + badge |
| API | `idempotency_test.go` | Duplicate POST detection |
| API | `optimistic_lock_test.go` | Stale row_version → 400 |
| API | `finance_shift_idempotency_test.go` | Shift-open / shift-close / batch-import idempotency; OCC double-close → 400 |
| API | `report_filters_test.go` | Report filter params (resident_id, date range, entity_type) applied to all 4 report types |
| API | `snapshots_test.go` | Before/after snapshots on finance (shift open/close/refund/batch/export), exam scheduler (template create/update/publish/resolve/block/generate), service-delivery alert and checkpoint, reports (toggle/run), diagnostics export, config snapshot and rollback |
| API | `diagnostics_log_test.go` | app_logs.txt in ZIP with LOG_PATH set or via new default; absent when explicitly empty; bcrypt + hex-key redaction; README describes file |
| API | `docs_consistency_test.go` | Reports UI scheduler help text accuracy; README idempotency/OCC statement completeness |
| API | `coverage_test.go` | GET /, SD order detail, exam template update, candidates add/remove, conflict resolve, payment/refund/shift/batch detail pages |
| JS  | `frontend_tests/app.test.mjs` | Flash dismiss timer, HTMX error handler |
| JS  | `frontend_tests/device-cache.test.mjs` | cacheExercise/getExercise round-trip, LRU eviction, clearAll, getStats, IndexedDB degradation |
| JS  | `frontend_tests/form-validation.test.mjs` | Flash schedule/remove, HTMX error handler, submit-once guard, email normalization, dollar/cents parsing |
| JS  | `frontend_tests/htmx-ui.test.mjs` | HX-Request detection, cookie parsing, badge classes, favorite toggle rollback, nav active state |
| E2E | `e2e_tests/tests/login.spec.ts` | Admin login → dashboard heading + nav items; bad-password stays on /login |
| E2E | `e2e_tests/tests/rbac.spec.ts` | Nurse 403 page content on /finance; nurse allowed on /service-delivery; unauthed → /login |
| E2E | `e2e_tests/tests/finance.spec.ts` | finance_clerk creates cash payment; detail page shows amount + method |
| E2E | `e2e_tests/tests/exams.spec.ts` | Admin creates template then session; session detail shows room name |
| E2E | `e2e_tests/tests/config.spec.ts` | Admin takes snapshot; filename appears in listing; nurse blocked |
| E2E | `e2e_tests/tests/htmx.spec.ts` | Exercise filter HTMX partial swap (no full reload); favorite toggle outerHTML swap |

---

## Verification Steps

After `docker-compose up` completes (watch for `server listening on :8080` in logs):

| Step | Command / Action | Expected outcome |
|---|---|---|
| Liveness | `curl -s http://localhost:8080/healthz` | `{"status":"ok"}` |
| Readiness | `curl -s http://localhost:8080/readyz` | `{"status":"ready"}` |
| Login redirect | `curl -si http://localhost:8080/dashboard` | HTTP 302 → `/login` (no cookie) |
| Admin login | Browse to `http://localhost:8080/login`, enter `admin@careops.local` / `Admin!234567` | Redirected to `/dashboard`; counts show seeded data |
| Dashboard counts | Dashboard page | TotalBeds ≥ 18, TotalResidents ≥ 10, TotalUsers = 8 |
| Module page | Browse to `/beds` as admin | Table lists seeded beds; no 500 error |
| Role gate | Log in as `nurse@careops.local`; browse to `/finance` | HTTP 403 Forbidden |
| Test suite | `./run_tests.sh` (Docker must be running) | `All test suites passed.` printed; exit code 0 |

---

## Health Endpoints

| Endpoint | Description |
|---|---|
| `GET /healthz` | Always returns `{"status":"ok"}` if the process is alive |
| `GET /readyz` | Returns `{"status":"ready"}` when the database is reachable |

---

## Security Summary

- **Authentication**: bcrypt cost-10 password hashing; random 32-byte session tokens stored server-side.
- **Session lifetime**: 24-hour absolute TTL; 15-minute inactivity timeout (configurable via `SESSION_TTL_HOURS` / `INACTIVITY_TIMEOUT_MINUTES`). Inactivity clock resets on every authenticated request.
- **Authorization**: Role-based access control enforced per route via explicit Fiber middleware. The full role matrix is statically reviewable in `internal/app/router.go`. Finance routes require `finance_clerk` or `admin`. Admin-only routes require `admin`. Unauthorized requests receive HTTP 403.
- **Security response headers**: Every response carries `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Cache-Control: no-store`.
- **Sensitive payment references** (card numbers, check numbers): AES-256-GCM encrypted at rest when `FIELD_ENCRYPT_KEY` is set. Masked to last-4 in all views; finance views decrypt on demand. Raw reference numbers never appear in logs.
- **Idempotency keys**: Payment creation, refunds, shift open/close, card-batch import, alert/checkpoint creation, and export generation accept an `X-Idempotency-Key` header or `_idempotency_key` form field; duplicate requests within the 24-hour TTL window return the original response without re-executing.
- **Optimistic concurrency control**: `row_version` columns on write-likely entities (payments, service orders, exam sessions, config entries, report definitions); stale updates return HTTP 400/409.
- **Path traversal protection**: Export, batch, and media download handlers use `filepath.Base` + `filepath.Rel` to confine file access to the configured data directory.
- **Audit log**: Login, logout, and every privileged write action append an immutable record to `audit_logs` containing actor ID, IP address, entity type/ID, action, and outcome. No update or delete path exists for audit records. Privileged mutations (shift open/close, refunds, batch import, exam template/session lifecycle, service-delivery alerts) include a JSON `before_snapshot` and/or `after_snapshot` of the affected entity. The audit page supports filtering by entity type, entity ID, user, date range, and `resident_id` (matches entries linked to a resident's payments, service orders, admissions, and alert events).

---

## Configuration

All configuration is environment-driven. Defaults are suitable for local development.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | `/data/db/careops.db` | SQLite file path |
| `MEDIA_PATH` | `/data/media` | Uploaded media root |
| `EXPORTS_PATH` | `/data/exports` | Finance export output directory |
| `DIAGNOSTICS_PATH` | `/data/diagnostics` | Diagnostic ZIP output directory |
| `CONFIG_SNAP_PATH` | `/data/config_snapshots` | Config snapshot JSON directory |
| `SESSION_TTL_HOURS` | `24` | Absolute session lifetime in hours |
| `INACTIVITY_TIMEOUT_MINUTES` | `15` | Session inactivity timeout in minutes |
| `FACILITY_TIMEZONE` | `America/New_York` | IANA timezone for exam windows and display |
| `LOG_LEVEL` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | `json` | Log format (`json` or `text`) |
| `LOG_PATH` | `./data/logs/app.log` | File path for structured log output; logs are always written to stdout and additionally to this file when set. The last 5 000 bytes are bundled (redacted) into diagnostic ZIP exports. |
| `FIELD_ENCRYPT_KEY` | _(unset)_ | 64-char hex (32 bytes) AES-256-GCM key for payment reference encryption. **Required in production** — startup fails if unset when `APP_ENV=production`. |

---

## Configuration Truth Table

Behaviour of `FIELD_ENCRYPT_KEY` enforcement depends on `APP_ENV`:

| `APP_ENV` | `FIELD_ENCRYPT_KEY` | Startup result | Payment references |
|---|---|---|---|
| `development` | unset | ✅ starts | stored unencrypted (warning logged) |
| `development` | valid 64-char hex | ✅ starts | AES-256-GCM encrypted |
| `development` | invalid value | ✅ starts | unencrypted (warning logged) |
| `production` | unset | ❌ **hard-fails** | — |
| `production` | invalid value | ❌ **hard-fails** | — |
| `production` | valid 64-char hex | ✅ starts | AES-256-GCM encrypted |

Generate a key with: `openssl rand -hex 32`

---

## Device Cache (Exercise Library)

`DeviceCache` is a client-side LRU module loaded on exercise detail pages. It caps at 200 items / 2 GB (localStorage for exercise JSON metadata; IndexedDB for media blobs). Staff must click **"Clear Device Cache"** before leaving a shared workstation — signing out of CareOps does **not** clear the cache.

---

## Assumptions & Constraints

- **No TLS** — local network deployment only. Add a reverse proxy for production TLS.
- **Single instance** — SQLite does not support horizontal scaling. Suitable for a single facility node.
- **No email/notification system** — alert events are stored in DB and surfaced in the UI; external push notifications are not implemented.
- **Report scheduler** — An in-process goroutine evaluates active report definitions against their `schedule_cron` expressions every minute and runs any that are due. Deduplication via `HasRunForWindow` prevents double-runs across restarts. Scheduler runs are attributed to the `scheduler` system user in `report_runs.triggered_by`.
- **Card batch CSV format** — expected columns: `resident_mrn,amount_dollars,description,reference_number`. Rows with unrecognized MRNs are skipped and counted in `error_count`.

See [`docs/requirements_traceability.md`](docs/requirements_traceability.md) for the full traceability matrix.
