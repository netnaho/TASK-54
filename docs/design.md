# CareOps System Design

## 1. Purpose and Scope
CareOps is a single-node, LAN-first clinic administration suite implemented as a server-rendered Go/Fiber web application with SQLite persistence. The current implementation covers:

- admissions, beds, occupancy snapshots
- service delivery tracking (orders, execution metrics, alerts, checkpoints)
- exercise library with favorites and client-side device cache
- competency exam scheduling and conflict resolution
- finance workflows (payments, refunds, shifts, card batch import, exports)
- report definitions with on-demand and scheduled runs
- immutable audit log search
- configuration versioning with snapshot and rollback
- diagnostics package export and job history

The system is designed to run without internet dependency at runtime.

## 2. Architectural Style
The implementation follows a modular monolith architecture:

- single Go binary (`cmd/server/main.go`)
- single SQLite database file with migration/seed bootstrap
- server-rendered HTML templates with HTMX partial updates
- module-level layering: `domain -> repository -> service -> handler`
- centralized route wiring and role policy in `internal/app/router.go`

This gives predictable deployment, low ops overhead, and strong static traceability of business behavior.

## 3. Runtime and Boot Sequence
Application startup flow:

1. Load environment-driven config.
2. Initialize structured logger (stdout + optional file).
3. Open SQLite and apply migrations/seeds (idempotent tracking tables).
4. Validate production-only encryption requirements.
5. Start report scheduler goroutine.
6. Build Fiber app (middleware, routes, template engine).
7. Listen on configured port.

Design implications:

- no request handling starts before schema/data bootstrap completes
- production startup fails fast if field encryption key is missing/invalid
- report scheduling is in-process and lifecycle-bound to app context

## 4. Delivery Topology and Data Boundaries
### 4.1 Process topology
- Web/API process: Fiber app and template renderer
- Background worker behavior: in-process scheduler tick for reports
- Database: embedded SQLite (WAL mode, FK on, busy timeout)

### 4.2 Storage boundaries
- relational data: SQLite
- file outputs:
- report exports (`EXPORTS_PATH`)
- diagnostics ZIPs (`DIAGNOSTICS_PATH`)
- config snapshots (`CONFIG_SNAP_PATH`)
- uploaded and served media (`MEDIA_PATH`)
- application log file (`LOG_PATH`, optional but defaulted)

### 4.3 Time handling
- persisted timestamps: RFC3339 UTC
- facility-local display and exam-window interpretation: configured IANA timezone (`FACILITY_TIMEZONE`)

## 5. Request/Response Design
### 5.1 UI model
- pages are server-rendered via `html/template`
- HTMX requests receive fragment-friendly responses
- shared `PageData` contract used by handlers for consistent layout metadata

### 5.2 Error contract
Centralized Fiber error handler supports:

- `401`: redirect login (or HTMX JSON + `HX-Redirect`)
- `403`, `404`, `409`, `500`: explicit templates for full page; JSON for HTMX
- `409` conflict message aligns with optimistic concurrency behavior

### 5.3 Health and readiness
- `/healthz`: process liveness
- `/readyz`: DB ping readiness

## 6. Module Decomposition and Responsibilities
### 6.1 Auth and RBAC
- local email/password auth with bcrypt
- session token generation and hashed token persistence
- absolute session TTL + inactivity timeout enforcement
- route-level RBAC middleware (`RequireSession`, `RequireRole`)

### 6.2 Clinical operations
- Beds: inventory/status views
- Admissions: resident intake/detail/discharge-oriented views
- Occupancy: census-style summary views
- Service Delivery: order tracking plus execution/timeliness analytics, alerts, checkpoints

### 6.3 Exercise Library
- filtering by category/difficulty/body part/tags/search
- favorites toggle
- client-side LRU cache module (`device-cache.js`) with explicit clear action

### 6.4 Exams
- template lifecycle
- session generation and candidate assignment
- conflict detection across room/proctor/candidate/window rules
- block updates and publish gate with unresolved-conflict prevention

### 6.5 Finance
- payment capture across offline methods
- refund processing with over-refund protection
- shift open/close settlement workflow
- card-terminal batch CSV import pipeline
- CSV/XLSX exports and safe download endpoints

### 6.6 Reports
- report definition CRUD with schedule metadata
- on-demand run execution to CSV/XLSX
- in-process scheduled execution with minute-window dedup

### 6.7 Audit, Config, Diagnostics, Jobs
- Audit: immutable event storage + filtered search (including resident-centric filtering)
- Config Versions: value updates, snapshots, rollback
- Diagnostics: one-click ZIP bundle (stats/jobs/config/audit/snapshots/log excerpt)
- Jobs: operational history for scheduled/on-demand jobs

## 7. Data and Consistency Design
### 7.1 Relational schema strategy
Schema is expanded through ordered SQL migrations and seeded with deterministic starter data. Migration and seed application are each tracked by dedicated metadata tables.

### 7.2 Optimistic concurrency control (OCC)
`row_version` is used on update-sensitive entities. Update operations enforce compare-and-swap semantics and surface stale-write conflicts as user-visible `409` responses.

### 7.3 Idempotency
Critical write endpoints accept idempotency keys via:

- `X-Idempotency-Key` header
- `_idempotency_key` form field

Idempotency records persist for 24 hours and replay prior outcomes to prevent duplicate submissions.

## 8. Security Design
### 8.1 Authentication and session security
- local user store only
- bcrypt verification
- random 32-byte session token generation
- inactivity + absolute expiration checks on every protected request

### 8.2 Authorization
Route registration is the primary authorization map. Allowed roles are visible at route declaration time, making access policy statically auditable.

### 8.3 Auditability
Privileged actions append immutable audit records with actor, action, entity, outcome, IP, request ID, and snapshots where applicable. Audit write failures are logged but do not block business transactions.

### 8.4 Sensitive data protection
- AES-256-GCM field encryption for sensitive finance references when key is configured
- production startup enforces encryption-key presence
- masking helpers for non-finance exposure
- redaction helpers for logs

### 8.5 HTTP hardening
Global security headers include clickjacking, sniffing, referrer, and cache controls. Request IDs are propagated in both request context and response header.

## 9. Observability and Operations
### 9.1 Logging
- structured slog events
- per-request access logs with method/path/status/latency/request-id/IP
- startup/bootstrap/validation events

### 9.2 Job history
Significant operations (report runs, diagnostics exports, config actions, settlement actions) record durable `job_history` entries for operational traceability.

### 9.3 Diagnostics package
Diagnostics export assembles a local ZIP containing:

- DB stats
- recent jobs
- config snapshot of current values
- recent audit log rows
- recent snapshot files
- optional redacted log excerpt

## 10. Frontend Interaction Design
The UI is intentionally server-first:

- full page rendering for primary flows
- HTMX for targeted partial refreshes (filters, toggles, fragments)
- template helper functions for timezone-aware formatting
- progressive behavior with explicit fallback-friendly handlers

## 11. Configuration Model
All runtime behavior is environment-driven with safe defaults for local development and explicit production constraints.

Important dimensions:

- storage paths
- auth/session timeout
- timezone localization
- log format/level/path
- encryption key via env or key file
- environment mode (`development`/`production`)

## 12. Testing Strategy (as implemented)
The repository includes:

- domain/unit tests (validation, crypto, idempotency, OCC, scheduling)
- API tests via Fiber `app.Test()` for route behavior and integration logic
- frontend JS tests for client helpers and cache behavior
- Playwright E2E suite for major user journeys and RBAC

This multi-layer approach validates both core invariants and user-facing flows.

## 13. Known Constraints and Design Tradeoffs
- single-instance SQLite architecture (not horizontally scaled)
- no built-in external notification/email subsystem
- in-process scheduler (no separate job runner service)
- LAN-oriented deployment assumptions (TLS termination external if needed)
- file-based exports/snapshots/diagnostics rely on local filesystem integrity

## 14. Extension Points
Current structure supports low-friction extensions:

- add modules with existing handler/service/repository pattern
- add new report types by extending report service generators
- expand audit filters and diagnostics payload builders
- introduce externalized scheduler/queue later with minimal module contract changes

