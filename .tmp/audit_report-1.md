# CareOps Delivery Acceptance and Project Architecture Audit (Static-Only Recheck)

Date: 2026-04-21  
Scope basis: static repository audit only (no runtime execution)

## 1. Verdict
- Overall conclusion: **Partial Pass**

## 2. Scope and Static Verification Boundary
- What was reviewed:
  - Documentation and run/test/config instructions: `repo/README.md`, `repo/docs/requirements_traceability.md`, `repo/run_tests.sh`
  - App wiring and route registration: `repo/cmd/server/main.go`, `repo/internal/app/*.go`
  - Security and auth: `repo/internal/modules/auth/**`, `repo/internal/modules/users/**`
  - Core business modules: service delivery, exercise library, exams, finance, reports, audit, diagnostics, config versions
  - Schema/migrations/seeds and selected unit/API/frontend/e2e test files (statically)
- What was not reviewed:
  - Runtime behavior in browser/network/containerized environment
  - External operational environment configuration beyond repository defaults
- What was intentionally not executed:
  - Project startup, Docker, tests, E2E, external services
- Claims requiring manual verification:
  - End-to-end runtime behavior on target LAN hardware
  - Real scheduling execution across wall-clock boundaries in deployment
  - Visual quality/responsiveness across real devices and browsers

## 3. Repository / Requirement Mapping Summary
- Prompt core goal: local-network CareOps suite with HTMX server-rendered workflows across occupancy, service delivery KPIs (15-minute timeliness), exercise CMS + client cache policy, exam scheduling/conflict handling, offline finance operations/shift settlement, permission-based reports + scheduled generation + exports, immutable auditability, local auth/RBAC/session timeout, encryption-at-rest for sensitive fields, and diagnostics ZIP for offline troubleshooting.
- Main mapped implementation areas:
  - Broad module coverage exists and is product-shaped (`repo/internal/modules/*`, `repo/internal/app/router.go:91-279`).
  - Core security, OCC, idempotency, and scheduler/report plumbing are implemented.
  - Remaining material gaps are mostly policy-consistency and requirement-fit gaps (custom report filters, resident-centric audit search, uniform privileged-action snapshots, diagnostics log packaging guarantee, and uniform idempotency on key finance writes).

## 4. Section-by-section Review

### 1. Hard Gates
#### 1.1 Documentation and static verifiability
- Conclusion: **Pass**
- Rationale: Startup/test/config and route surface are statically discoverable and coherent.
- Evidence: `repo/README.md:133-160`, `repo/README.md:344-363`, `repo/run_tests.sh:45-83`, `repo/cmd/server/main.go:13-43`, `repo/internal/app/router.go:91-279`

#### 1.2 Material deviation from Prompt
- Conclusion: **Partial Pass**
- Rationale: Most core flows are implemented, but several prompt-critical semantics remain partially met (custom report filters not applied, audit search not resident-centric, non-uniform privileged-action before/after snapshots).
- Evidence: `repo/internal/modules/reports/service/report_service.go:269-281`, `repo/web/templates/reports/new.html:61-65`, `repo/internal/modules/audit/handler/handler.go:20-29`, `repo/internal/modules/exams/service/scheduler.go:89-96,217-223,310-316`

### 2. Delivery Completeness
#### 2.1 Core explicit requirements coverage
- Conclusion: **Partial Pass**
- Rationale: Most explicit core requirements are implemented (RBAC, OCC, idempotency service, report scheduling engine, diagnostics export, finance/exam/service-delivery/exercise flows), with notable requirement-fit gaps in custom report filtering and resident-oriented audit traceability.
- Evidence: `repo/internal/modules/reports/handler/routes.go:10-17`, `repo/internal/app/scheduler.go:17-29`, `repo/internal/modules/reports/scheduler/scheduler.go:51-78`, `repo/internal/modules/reports/service/report_service.go:269-320`, `repo/internal/modules/audit/repository/audit_repo.go:79-98`

#### 2.2 0→1 end-to-end deliverable vs partial/demo
- Conclusion: **Pass**
- Rationale: Repository contains full application structure, layered modules, migrations/seeds, and test suites; not a fragment/demo.
- Evidence: `repo/README.md:62-129`, `repo/internal/app/router.go:106-279`, `repo/migrations/0001_initial_schema.sql`, `repo/unit_tests/*`, `repo/API_tests/*`

### 3. Engineering and Architecture Quality
#### 3.1 Structure and module decomposition
- Conclusion: **Pass**
- Rationale: Clear decomposition across domain/repository/service/handler with centralized route wiring.
- Evidence: `repo/README.md:62-129`, `repo/internal/app/router.go:65-91`, `repo/internal/modules/*`

#### 3.2 Maintainability/extensibility
- Conclusion: **Partial Pass**
- Rationale: Architecture is maintainable, but required cross-cutting guarantees (uniform idempotency and uniform before/after audit capture on privileged writes) are still inconsistently applied by module.
- Evidence: `repo/internal/modules/finance/handler/handler.go:232-247,272-328`, `repo/internal/modules/exams/service/scheduler.go:227-261,320-342`, `repo/internal/modules/finance/service/finance_service.go:205-214,239-248,278-287,360-369`

### 4. Engineering Details and Professionalism
#### 4.1 Error handling/logging/validation/API design
- Conclusion: **Partial Pass**
- Rationale: Strong baseline exists (structured logging, request IDs, centralized error handling, validations), but requirement-level implementation details are still incomplete in some critical flows (custom report filtering, resident query semantics, complete audit snapshots).
- Evidence: `repo/internal/app/app.go:48-116`, `repo/internal/platform/middleware/request_id.go:10-22`, `repo/internal/platform/middleware/access_log.go:10-25`, `repo/internal/modules/reports/service/report_service.go:269-281`, `repo/internal/modules/audit/handler/handler.go:20-29`

#### 4.2 Product vs demo shape
- Conclusion: **Pass**
- Rationale: Product-like breadth with real persistence, auth/RBAC, scheduler, exports, and diagnostics package generation.
- Evidence: `repo/internal/modules/reports/scheduler/scheduler.go:12-79`, `repo/internal/modules/diagnostics/service/diagnostics_service.go:78-136,231-267`, `repo/internal/modules/finance/handler/handler.go:125-426`

### 5. Prompt Understanding and Requirement Fit
#### 5.1 Business goal, semantics, constraints fit
- Conclusion: **Partial Pass**
- Rationale: Business scenario is largely implemented, but not fully aligned on some semantics: custom report filters are stored but not executed, audit search does not directly support resident dimension, and privileged-action before/after capture is not uniform.
- Evidence: `repo/web/templates/reports/new.html:61-65`, `repo/internal/modules/reports/service/report_service.go:189-190,269-281`, `repo/internal/modules/audit/domain/audit.go:24-30`, `repo/internal/modules/audit/handler/handler.go:20-29`, `repo/internal/modules/service_delivery/handler/handler.go:217-223,288-294`

### 6. Aesthetics (frontend-only/full-stack)
#### 6.1 Visual and interaction quality fit
- Conclusion: **Cannot Confirm Statistically**
- Rationale: Templates and interaction scaffolding exist, but visual quality, responsive behavior, and interaction smoothness require runtime/manual verification.
- Evidence: `repo/web/templates/*`, `repo/web/static/js/*`
- Manual verification note: Browser walkthrough needed for responsive layout, interaction feedback, and cross-device consistency.

## 5. Issues / Suggestions (Severity-Rated)

### High
1. Severity: **High**  
- Title: Report “custom filters” are not functionally applied to generated data  
- Conclusion: **Fail**  
- Evidence: `repo/internal/modules/reports/service/report_service.go:189-190,269-281`, `repo/web/templates/reports/new.html:61-65`  
- Impact: Prompt-required permission-based reporting with custom filters is not met; users can define filters that do not affect output.  
- Minimum actionable fix: Parse `scheduled_reports.parameters` and apply validated filter predicates in each report query builder (`buildOccupancyData`, `buildServiceDeliveryData`, `buildFinanceData`, `buildAuditData`) with tests for filter correctness.

2. Severity: **High**  
- Title: Audit traceability query model does not support resident-oriented search as required  
- Conclusion: **Partial Fail**  
- Evidence: `repo/internal/modules/audit/handler/handler.go:20-29`, `repo/internal/modules/audit/domain/audit.go:24-30`, `repo/web/templates/audit/index.html:13-31`  
- Impact: Auditors/admins cannot directly query audit trail by resident dimension from the audit interface/query model as required by prompt semantics.  
- Minimum actionable fix: Add resident-aware filter input and repository query support (e.g., resident_id, or resident join mapping for entity types tied to residents).

3. Severity: **High**  
- Title: Before/after snapshots are not uniformly captured for privileged mutations  
- Conclusion: **Partial Fail**  
- Evidence: Snapshot-capable model exists (`repo/internal/modules/audit/domain/audit.go:15-16`), but many privileged writes emit only `Details` without snapshots (`repo/internal/modules/exams/service/scheduler.go:89-96,217-223,310-316`; `repo/internal/modules/service_delivery/handler/handler.go:217-223,288-294`; `repo/internal/modules/finance/service/finance_service.go:205-214,239-248,278-287,360-369`)  
- Impact: Forensic traceability is inconsistent and weaker than prompt requirement for before/after change details on privileged actions.  
- Minimum actionable fix: Enforce central audit contract for privileged mutations requiring structured before/after payloads where state changes occur.

4. Severity: **High**  
- Title: Diagnostics log inclusion is optional and disabled by default configuration  
- Conclusion: **Partial Fail**  
- Evidence: App logs added only when `logPath` set (`repo/internal/modules/diagnostics/service/diagnostics_service.go:259-261,454-456`); default `LOG_PATH` empty (`repo/internal/config/config.go:78`); compose does not set `LOG_PATH` (`repo/docker-compose.yml:12-33`)  
- Impact: Prompt requires diagnostics bundle to include logs; default deployments can produce ZIP without app runtime logs.  
- Minimum actionable fix: Ensure runtime logs are always persisted and bundled by default (set deterministic log path in production config and include it in diagnostics package).

### Medium
5. Severity: **Medium**  
- Title: Idempotency is not uniformly enforced on all key finance mutation routes  
- Conclusion: **Partial Fail**  
- Evidence: Idempotency used on payment/refund/export (`repo/internal/modules/finance/handler/handler.go:128,200,368`) but absent on shift open/close and batch import submit (`repo/internal/modules/finance/handler/handler.go:232-247,272-284,303-328`)  
- Impact: Duplicate submissions remain possible for some high-impact operational writes.  
- Minimum actionable fix: Add idempotency key extraction/check/store to shift open, shift close, and batch import endpoints.

6. Severity: **Medium**  
- Title: Reports create form text conflicts with implementation state  
- Conclusion: **Fail**  
- Evidence: UI states automated scheduling is not active (`repo/web/templates/reports/new.html:57`) while scheduler is wired and started (`repo/cmd/server/main.go:38`, `repo/internal/app/scheduler.go:17-29`)  
- Impact: Operator misunderstanding and acceptance confusion; may lead to incorrect audit conclusions and misuse.  
- Minimum actionable fix: Update form/help text to reflect active scheduler behavior and current cron semantics.

### Low
7. Severity: **Low**  
- Title: Security/traceability documentation has stale assertions versus current behavior  
- Conclusion: **Partial Fail**  
- Evidence: README still states stale OCC returns HTTP 400 (`repo/README.md:453`), and idempotency examples are narrower than current implementation scope (`repo/README.md:452`)  
- Impact: Documentation drift can mislead reviewers/operators.
- Minimum actionable fix: Align README security summary with current OCC (409 behavior where implemented) and full idempotency coverage matrix.

## 6. Security Review Summary
- Authentication entry points: **Pass**  
  - Evidence: `repo/internal/modules/auth/handler/routes.go`, `repo/internal/modules/auth/service/auth_service.go:48-78`, `repo/internal/modules/auth/repository/session_repo.go:24-54`
  - Reasoning: Local username/password auth, bcrypt verification, session token hashing, and session persistence are present.

- Route-level authorization: **Pass**  
  - Evidence: `repo/internal/app/router.go:128-279`, `repo/internal/modules/auth/handler/middleware.go:53-72`
  - Reasoning: Explicit per-route RBAC is consistently wired in router.

- Object-level authorization: **Cannot Confirm Statistically**  
  - Evidence: Direct ID-based detail access patterns (e.g., `repo/internal/modules/finance/handler/handler.go:149-175`) without explicit object ownership checks.
  - Reasoning: Single-facility model may make strict object ownership optional, but static code does not prove stronger object-level policy guarantees.

- Function-level authorization: **Partial Pass**  
  - Evidence: Middleware-centric authorization model (`repo/internal/modules/auth/handler/middleware.go:53-72`), limited in-handler defense-in-depth.
  - Reasoning: Effective if routes remain correctly wired; less robust against accidental future route miswiring.

- Tenant / user isolation: **Not Applicable**  
  - Evidence: Prompt and schema define single-facility local deployment (no tenant model).

- Admin / internal / debug endpoint protection: **Pass**  
  - Evidence: Admin-only gating for diagnostics/config/jobs (`repo/internal/app/router.go:257-279`)
  - Reasoning: Sensitive operational endpoints are role-restricted.

## 7. Tests and Logging Review
- Unit tests: **Pass (breadth), Partial (coverage fit to remaining risks)**  
  - Evidence: `repo/unit_tests/*.go`, e.g., timeliness/conflict/idempotency tests (`repo/unit_tests/timeliness_test.go`, `repo/unit_tests/exam_scheduler_test.go`, `repo/unit_tests/idempotency_test.go`)

- API / integration tests: **Partial Pass**  
  - Evidence: `repo/API_tests/*.go`, with broad auth/RBAC/OCC/idempotency/report/diagnostics coverage (`repo/API_tests/security_test.go`, `repo/API_tests/reports_test.go`, `repo/API_tests/diagnostics_test.go`, `repo/API_tests/optimistic_lock_test.go`)  
  - Gap: No meaningful tests that prove report parameter filters affect output; limited coverage for resident-oriented audit query behavior and uniform privileged-action snapshots.

- Logging categories / observability: **Pass**  
  - Evidence: request IDs and structured access logs (`repo/internal/platform/middleware/request_id.go:10-22`, `repo/internal/platform/middleware/access_log.go:10-25`), job history writer (`repo/internal/platform/jobs/writer.go:43-60`), diagnostics package structure (`repo/internal/modules/diagnostics/service/diagnostics_service.go:231-267`)

- Sensitive-data leakage risk in logs / responses: **Partial Pass**  
  - Evidence: Diagnostics log redaction for bcrypt/hash-like secrets (`repo/internal/modules/diagnostics/service/diagnostics_service.go:481-491`), tests for redaction (`repo/API_tests/diagnostics_log_test.go:142-203`)  
  - Remaining risk: App-log inclusion is optional by config, so diagnostics completeness for incident analysis is not guaranteed by default.

## 8. Test Coverage Assessment (Static Audit)

### 8.1 Test Overview
- Unit tests exist: **Yes** (`repo/unit_tests/*.go`)
- API/integration tests exist: **Yes** (`repo/API_tests/*.go`)
- Frontend tests exist: **Yes** (`repo/frontend_tests/*.mjs`)
- E2E tests exist: **Yes** (`repo/e2e_tests/tests/*.ts`)
- Frameworks/approach (static):
  - Go tests via `go test` for unit/API (`repo/run_tests.sh:46-52`)
  - Node built-in test runner for frontend unit tests (`repo/run_tests.sh:55-60`)
  - Playwright for browser E2E (`repo/run_tests.sh:62-83`)
- Test entry points and docs: documented in README (`repo/README.md:344-370`) and script (`repo/run_tests.sh:1-94`)

### 8.2 Coverage Mapping Table

| Requirement / Risk Point | Mapped Test Case(s) | Key Assertion / Fixture / Mock | Coverage Assessment | Gap | Minimum Test Addition |
|---|---|---|---|---|---|
| Auth login/logout/session basics | `repo/API_tests/security_test.go:56-133`, `repo/API_tests/auth_test.go` | Session cookie issued/cleared; bad credentials not granting session | sufficient | None material | Keep regression tests for session expiry edge cases |
| Unauthenticated 401 / redirect behavior | `repo/API_tests/security_test.go:139-167` | Redirect to `/login`, HTMX 401 + `HX-Redirect` | sufficient | None material | Add negative tests for stale cookie tampering |
| Route-level RBAC (403) | `repo/API_tests/security_test.go:173-260`, `repo/API_tests/finance_test.go:16-39`, `repo/API_tests/reports_test.go:24-38` | Role matrix checks across finance/reports/diagnostics/audit/users | sufficient | None material | Keep matrix in sync with router role table |
| Admin/internal protection | `repo/API_tests/security_test.go:204-230`, `repo/API_tests/diagnostics_test.go:15-29` | Non-admin blocked from diagnostics/config routes | sufficient | None material | Add jobs-route explicit admin-only case |
| OCC conflict behavior | `repo/API_tests/optimistic_lock_test.go:14-92`, `repo/API_tests/consistency_audit_test.go:250-356` | Stale `row_version` returns conflict path | basically covered | Some endpoints still allow 400/409 ambiguity | Add strict expected status assertions per endpoint |
| Idempotency dedupe core | `repo/API_tests/reports_test.go:184-343`, `repo/API_tests/consistency_audit_test.go:145-248` | Repeated key replays without duplicate operation | basically covered | Not all key finance writes covered | Add tests for `/finance/shifts`, `/finance/shifts/:id/close`, `/finance/batches` idempotency |
| Service-delivery timeliness 15 min rule | `repo/unit_tests/timeliness_test.go:10-53` | Boundary checks (exact 15 min, +1s late, early, zero scheduled) | sufficient | None material | Keep as invariant test |
| Exam conflict detection | `repo/unit_tests/exam_scheduler_test.go:111-323` | Room/proctor/candidate/window conflict scenarios | sufficient | None material | Add test for multi-conflict merge behavior |
| Diagnostics ZIP required artifacts | `repo/API_tests/diagnostics_test.go:161-223`, `repo/API_tests/diagnostics_log_test.go:21-140` | ZIP content includes required JSONs; app logs conditional on LOG_PATH | basically covered | Default-no-log behavior still passes without app logs | Add policy test requiring configured deployment default includes app logs |
| Report scheduler due-run + dedupe | `repo/API_tests/scheduler_test.go:22-121` | Due cron run and dedup within same window | basically covered | No explicit test validating parameterized filters during run | Add scheduler run test with parameterized report and asserted filtered output |
| Report custom filters semantic requirement | No direct test found | N/A | missing | Tests do not assert parameter-to-query effect | Add API tests that create report with parameters and verify output rows are filtered |
| Audit search by resident semantics | No resident filter test found; only entity/date filters (`repo/API_tests/audit_test.go:130-151`) | N/A | missing | Prompt-specific resident query path unverified | Add audit API/UI tests for resident-oriented filtering |

### 8.3 Security Coverage Audit
- Authentication: **Meaningfully covered** (login/logout/bad credentials, HTMX unauthenticated paths).  
  Evidence: `repo/API_tests/security_test.go:56-167`.
- Route authorization: **Meaningfully covered** (multiple role matrices and protected routes).  
  Evidence: `repo/API_tests/security_test.go:173-260`, `repo/API_tests/finance_test.go`, `repo/API_tests/reports_test.go`.
- Object-level authorization: **Not meaningfully covered** for strict ownership semantics.  
  Evidence: no targeted tests proving per-object ownership constraints.
- Tenant/data isolation: **Not applicable / not tested** in multi-tenant sense (single-facility model).
- Admin/internal protection: **Covered** for diagnostics/config; partial for full admin surface breadth.  
  Evidence: `repo/API_tests/diagnostics_test.go:15-29`, `repo/API_tests/security_test.go:204-230`.

### 8.4 Final Coverage Judgment
- **Partial Pass**
- Boundary explanation:
  - Major risks covered: auth, session protection, core RBAC, key OCC/idempotency paths, report scheduler basic behavior, diagnostics ZIP safety/content checks.
  - Major uncovered/undercovered risks: prompt-level custom report filter semantics, resident-oriented audit search behavior, uniform idempotency and before/after audit coverage across all privileged writes. These gaps mean tests could pass while important requirement-fit defects remain.

## 9. Final Notes
- Audit conclusions are static-only and evidence-bound; no runtime success is claimed.
- The codebase is substantially complete and product-shaped, but the remaining requirement-fit gaps are material enough to prevent a full pass.
