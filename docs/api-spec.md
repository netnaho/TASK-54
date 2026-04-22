# CareOps API Specification (Implementation-Aligned)

## 1. Scope
This document describes the HTTP interface implemented in `repo/` as of the current codebase. The app is server-rendered (HTML + HTMX partials) with Fiber handlers and SQLite-backed modules.

## 2. Base Behavior
- Base URL: local deployment host (for example `http://<facility-host>:8080`)
- API style: REST-like route structure, but primarily HTML responses and redirects
- Authentication: cookie-based session (`careops_session` by default)
- Common request correlation header: `X-Request-ID` (generated if not supplied)
- HTMX detection: `HX-Request: true`
- Idempotency key transport:
- Header: `X-Idempotency-Key`
- Form field fallback: `_idempotency_key`

## 3. Security and Access Model
### 3.1 Public routes
- `GET /healthz`
- `GET /readyz`
- `GET /login`
- `POST /login`
- `POST /logout`

### 3.2 Protected route role matrix
- `/dashboard`: any authenticated user
- `/users*`: `admin`
- `/beds*`: `admin`, `nurse`, `front_desk`
- `/admissions*`: `admin`, `nurse`, `front_desk`
- `/occupancy*`: `admin`, `nurse`, `front_desk`
- `/service-delivery*`: `admin`, `nurse`, `therapist`, `aide`, `front_desk`
- `/exercise-library*`: `admin`, `therapist`, `training_coordinator`, `nurse`, `aide`
- `/media/:filename`: any authenticated user
- `/exams*`: `admin`, `training_coordinator`
- `/finance*`: `finance_clerk`, `admin`
- `/reports*`: `admin`, `auditor`
- `/audit`: `admin`, `auditor`
- `/jobs`: `admin`
- `/config-versions*`: `admin`
- `/diagnostics*`: `admin`

### 3.3 Auth/session behavior
- Missing/expired session:
- normal request: redirect to `/login`
- HTMX request: `401` JSON and `HX-Redirect: /login`
- Forbidden role:
- normal request: `403` page
- HTMX request: `403` JSON and `HX-Redirect: /dashboard`

## 4. Concurrency and Replay Controls
### 4.1 Idempotency
- Keys are stored and replayed for 24 hours
- Many write endpoints check/store idempotency keys (create/update/approve/export-like flows)

### 4.2 Optimistic concurrency (row version)
Implemented on update-sensitive flows where applicable, typically via `row_version`/`template_version` form fields. Stale writes return `409` conflict with reload guidance.

## 5. Error Contract
- Full-page requests: rendered error pages (`403`, `404`, `409`, `500`) or redirect for `401`
- HTMX requests: JSON error payloads for `401`/`403`/`404`/`409`/`500`
- Common conflict message semantics: record changed, reload latest state and retry

## 6. Endpoints

## 6.1 Core and auth
### `GET /healthz`
- Auth: none
- Response: JSON `{ "status": "ok" }`

### `GET /readyz`
- Auth: none
- Response:
- `200` JSON `{ "status": "ready" }` when DB reachable
- `503` JSON `{ "status": "db_unreachable" }` when DB ping fails

### `GET /`
- Auth: none
- Response: redirect to `/dashboard`

### `GET /login`
- Auth: none
- Response: login page

### `POST /login`
- Auth: none
- Form fields:
- `email`
- `password`
- Success: sets session cookie and redirects `/dashboard`
- Failure: re-renders login page with error

### `POST /logout`
- Auth: public endpoint, clears current cookie/session if present
- Response: redirect `/login`

### `GET /dashboard`
- Auth: any authenticated user
- Response: dashboard page with aggregate counts

## 6.2 Users (`admin`)
### `GET /users`
- Query:
- `flash` (optional)
- Response: user list page

### `GET /users/new`
- Response: create-user form

### `POST /users`
- Form fields:
- `email`
- `display_name`
- `password` (password policy enforced)
- `role`
- `_idempotency_key` (optional)
- Success: redirect `/users?flash=...`
- Validation failure: form re-render

### `GET /users/:id/reset-password`
- Response: reset-password form

### `POST /users/:id/reset-password`
- Form fields:
- `password`
- `row_version` (OCC)
- `_idempotency_key` (optional)
- Success: redirect `/users?flash=...`
- Conflict: `409`

## 6.3 Beds (`admin|nurse|front_desk`)
### `GET /beds`
- Query:
- `ward` (optional)
- `status` (`occupied|available|all`, optional)
- HTMX: returns table partial when `HX-Request: true`
- Otherwise: full page

### `GET /beds/:id`
- Response: bed detail page or `404`

## 6.4 Admissions (`admin|nurse|front_desk`)
### `GET /admissions`
- Query:
- `care_level` (optional)
- `status` (`admitted|all`, default `admitted`)
- HTMX: residents table partial
- Otherwise: full page

### `GET /admissions/:id`
- Response: resident detail page or `404`

## 6.5 Occupancy (`admin|nurse|front_desk`)
### `GET /occupancy`
- Response: occupancy overview page (live counts + snapshots)

## 6.6 Service Delivery (`admin|nurse|therapist|aide|front_desk`)
### `GET /service-delivery`
- Query:
- `date_from`
- `date_to`
- `resident_id`
- `status` (default `active`)
- `service_type`
- HTMX: content partial
- Otherwise: full page

### `GET /service-delivery/orders/:id`
- Response: order detail page

### `GET /service-delivery/alerts/new`
- Response: alert form

### `POST /service-delivery/alerts`
- Form fields:
- `resident_id`
- `alert_type`
- `severity`
- `message` (max 500)
- `_idempotency_key` (optional)
- Success: redirect `/service-delivery` with flash
- Validation failure: form re-render

### `GET /service-delivery/checkpoints/new`
- Response: checkpoint form

### `POST /service-delivery/checkpoints`
- Form fields:
- `resident_id`
- `checkpoint_type`
- `score` (1..5)
- `notes`
- `_idempotency_key` (optional)
- Success: redirect `/service-delivery` with flash
- Validation failure: form re-render

## 6.7 Exercise Library (`admin|therapist|training_coordinator|nurse|aide`)
### `GET /exercise-library`
- Query:
- `category`
- `difficulty`
- `tag`
- `body_part`
- `q`
- `favorites` (`1` for favorites-only)
- HTMX: grid partial
- Otherwise: full page

### `GET /exercise-library/:id`
- Response: exercise detail page or `404`

### `POST /exercise-library/:id/favorite`
- Action: toggle favorite for current user
- Response: favorite button partial (HTMX-friendly)

### `GET /media/:filename`
- Auth: any authenticated user
- Path safety validated before file serving
- Response: local media file

## 6.8 Exams (`admin|training_coordinator`)
### Overview
- `GET /exams`: templates + recent sessions
- `GET /exams/scheduler`: day-view scheduler
- Scheduler query:
- `date` (`YYYY-MM-DD`, optional)

### Template endpoints
- `GET /exams/templates/new`
- `POST /exams/templates`
- Form fields:
- `title`, `description`, `role_scope`, `passing_score`, `duration_minutes`, `window_start`, `window_end`
- `_idempotency_key` optional
- `GET /exams/templates/:id`
- `POST /exams/templates/:id`
- Form fields:
- `title`, `description`, `duration_minutes`, `window_start`, `window_end`, `passing_score`, `template_version` (OCC)
- `_idempotency_key` optional
- Conflict: `409`

### Session endpoints
- `GET /exams/sessions/new`
- `POST /exams/sessions`
- Form fields:
- `template_id`, `planned_start`, `room`, `proctor_id`, repeated `candidate_ids`
- `_idempotency_key` optional
- `GET /exams/sessions/:id`
- `POST /exams/sessions/:id/block`
- Form fields:
- `planned_start`, `room`, `proctor_id`, `row_version`
- `_idempotency_key` optional
- Conflict: `409`
- HTMX response: session block partial
- non-HTMX response: redirect with flash
- `POST /exams/sessions/:id/publish`
- Form fields:
- `row_version`
- `_idempotency_key` optional
- Conflict cases return `409` (blocking conflicts/version conflict)
- `POST /exams/sessions/:id/candidates/add`
- Form fields: `user_id`, `_idempotency_key` optional
- HTMX response: conflicts partial
- `POST /exams/sessions/:id/candidates/remove`
- Form fields: `user_id`, `_idempotency_key` optional
- HTMX response: conflicts partial

### Conflict resolution endpoint
- `POST /exams/conflicts/:id/resolve`
- Form fields:
- `resolution_notes` (optional)
- `session_id` (used for redirect)
- `_idempotency_key` optional
- HTMX response: conflict row partial
- non-HTMX response: redirect to `/exams/sessions/:session_id`

## 6.9 Finance (`finance_clerk|admin`)
### Overview
- `GET /finance`: finance dashboard

### Payments
- `GET /finance/payments/new`
- `POST /finance/payments`
- Form fields:
- `resident_id`, `shift_id`, `payment_method`, `amount`, `reference_number`, `description`
- `_idempotency_key` optional
- `GET /finance/payments/:id`
- `GET /finance/payments/:id/refund`
- `POST /finance/payments/:id/refund`
- Form fields:
- `amount`, `reason`, `_idempotency_key` optional

### Shifts
- `GET /finance/shifts`
- `POST /finance/shifts`
- Form fields:
- `shift_date` (optional; defaults to current UTC date)
- `shift_name`
- `_idempotency_key` optional
- `GET /finance/shifts/:id`
- `POST /finance/shifts/:id/close`
- Form fields:
- `reconciliation_notes`
- `_idempotency_key` optional

### Card-terminal batches
- `GET /finance/batches/new`
- `POST /finance/batches`
- Form fields:
- `shift_id`
- `batch_file` (required upload, max 10 MB)
- `_idempotency_key` optional
- `GET /finance/batches/:id`

### Exports
- `GET /finance/exports/new`
- `POST /finance/exports`
- Form fields:
- `report_type`, `date_from`, `date_to`, `shift_id`, `format` (defaults to `csv`)
- `_idempotency_key` optional
- Success: redirect to download route
- `GET /finance/exports/download?file=<name>`
- Safe file serving under configured exports path
- Response: file attachment (`csv` or `xlsx` content type)

## 6.10 Reports (`admin|auditor`)
### Definition and run management
- `GET /reports`
- Query: `flash` optional
- `GET /reports/new`
- `POST /reports`
- Form fields:
- `name`, `report_type`, `schedule_cron`, `output_format`, `parameters`
- `_idempotency_key` optional
- `GET /reports/:id/edit`
- `POST /reports/:id/update`
- Form fields:
- `row_version` (OCC)
- `name`, `report_type`, `schedule_cron`, `output_format`, `parameters`
- `_idempotency_key` optional
- Conflict: `409`
- `POST /reports/:id/run`
- `_idempotency_key` optional
- `POST /reports/:id/toggle`
- `_idempotency_key` optional

### Output download
- `GET /reports/runs/download?run=<run_id>`
- Response: run output attachment (`csv` or `xlsx`)

## 6.11 Audit (`admin|auditor`)
### `GET /audit`
- Query filters:
- `entity_type`
- `entity_id`
- `user_id`
- `resident_id`
- `date_from` (`YYYY-MM-DD`)
- `date_to` (`YYYY-MM-DD`)
- Response: audit log page (capped list)

## 6.12 Jobs (`admin`)
### `GET /jobs`
- Response: recent background job history page

## 6.13 Config Versions (`admin`)
### `GET /config-versions`
- Response: config list + snapshots

### `GET /config-versions/:id/edit`
- Response: config edit form

### `POST /config-versions/:id`
- Form fields:
- `config_value`
- `row_version` (OCC, required integer)
- `_idempotency_key` optional
- Conflict: `409`

### `POST /config-versions/snapshot`
- `_idempotency_key` optional
- Response: redirect with flash

### `POST /config-versions/rollback`
- Form fields:
- `snapshot_file` (required)
- `_idempotency_key` optional
- Response: redirect with flash

## 6.14 Diagnostics (`admin`)
### `GET /diagnostics`
- Query:
- `flash` (optional)
- Response: diagnostics page with DB stats + export history

### `POST /diagnostics/exports`
- `_idempotency_key` optional
- Response: redirect with flash

### `GET /diagnostics/exports/download?file=<name>`
- Safe file serving under diagnostics directory
- Response: ZIP (or existing export file) attachment

## 7. Notes for Integrators
- This interface is intentionally HTML-first. Most POST actions return redirects rather than JSON bodies.
- HTMX clients should send `HX-Request: true` and be prepared for partial HTML responses or JSON errors.
- For duplicate-submission safety, include `X-Idempotency-Key` on all user-triggered write actions.
- For OCC-protected updates, include the current `row_version`/`template_version` from the rendered form.
