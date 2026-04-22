# Test Coverage Audit

## Scope, Method, and Constraints Compliance
- Audit method: static inspection only.
- No runtime execution performed (no tests, scripts, containers, servers, package install).
- Inspected scope only:
  - Route definitions: `internal/app/router.go`, `internal/modules/*/handler/routes.go`
  - Tests: `API_tests/*`, `unit_tests/*`, `frontend_tests/*`, `e2e_tests/tests/*`
  - Docs/scripts: `README.md`, `run_tests.sh`, `docker-compose.e2e.yml`

## Project Type Detection
- README declares: `**Type:** fullstack` (`repo/README.md:3`).
- Inference cross-check: backend routes + frontend assets + browser E2E suite present.
- Final project type: **fullstack**.

## Backend Endpoint Inventory
Route evidence:
- `repo/internal/app/router.go:93-269`
- `repo/internal/modules/auth/handler/routes.go:7-10`
- `repo/internal/modules/admissions/handler/routes.go:10-11`
- `repo/internal/modules/exams/handler/routes.go:12-31`
- `repo/internal/modules/finance/handler/routes.go:9-32`
- `repo/internal/modules/config_versions/handler/routes.go:11-15`
- `repo/internal/modules/diagnostics/handler/routes.go:8-10`
- `repo/internal/modules/reports/handler/routes.go:8`

Total resolved endpoints: **64**

1. `GET /healthz`
2. `GET /readyz`
3. `GET /`
4. `GET /login`
5. `POST /login`
6. `POST /logout`
7. `GET /dashboard`
8. `GET /users`
9. `GET /beds`
10. `GET /beds/:id`
11. `GET /admissions`
12. `GET /admissions/:id`
13. `GET /occupancy`
14. `GET /service-delivery`
15. `GET /service-delivery/orders/:id`
16. `GET /service-delivery/alerts/new`
17. `POST /service-delivery/alerts`
18. `GET /service-delivery/checkpoints/new`
19. `POST /service-delivery/checkpoints`
20. `GET /exercise-library`
21. `GET /exercise-library/:id`
22. `POST /exercise-library/:id/favorite`
23. `GET /media/:filename`
24. `GET /exams`
25. `GET /exams/scheduler`
26. `GET /exams/templates/new`
27. `POST /exams/templates`
28. `GET /exams/templates/:id`
29. `POST /exams/templates/:id`
30. `GET /exams/sessions/new`
31. `POST /exams/sessions`
32. `GET /exams/sessions/:id`
33. `POST /exams/sessions/:id/block`
34. `POST /exams/sessions/:id/publish`
35. `POST /exams/sessions/:id/candidates/add`
36. `POST /exams/sessions/:id/candidates/remove`
37. `POST /exams/conflicts/:id/resolve`
38. `GET /finance`
39. `GET /finance/payments/new`
40. `POST /finance/payments`
41. `GET /finance/payments/:id`
42. `GET /finance/payments/:id/refund`
43. `POST /finance/payments/:id/refund`
44. `GET /finance/shifts`
45. `POST /finance/shifts`
46. `GET /finance/shifts/:id`
47. `POST /finance/shifts/:id/close`
48. `GET /finance/batches/new`
49. `POST /finance/batches`
50. `GET /finance/batches/:id`
51. `GET /finance/exports/new`
52. `POST /finance/exports`
53. `GET /finance/exports/download`
54. `GET /reports`
55. `GET /audit`
56. `GET /jobs`
57. `GET /config-versions`
58. `POST /config-versions/snapshot`
59. `POST /config-versions/rollback`
60. `GET /config-versions/:id/edit`
61. `POST /config-versions/:id`
62. `GET /diagnostics`
63. `POST /diagnostics/exports`
64. `GET /diagnostics/exports/download`

## API Test Mapping Table
Legend:
- `covered` means exact method+path request is sent and route handler is reached.
- `test type` values: `true no-mock HTTP`, `HTTP with mocking`, `unit-only/indirect`.

| Endpoint | Covered | Test Type | Test Files | Evidence (file + test/function) |
|---|---|---|---|---|
| GET /healthz | yes | true no-mock HTTP | API_tests | `health_test.go:TestHealthz` |
| GET /readyz | yes | true no-mock HTTP | API_tests | `health_test.go:TestReadyz` |
| GET / | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestRoot_RedirectsToDashboard` |
| GET /login | yes | true no-mock HTTP | API_tests | `auth_test.go:TestLogin_GetRendersForm` |
| POST /login | yes | true no-mock HTTP | API_tests | `auth_test.go:TestLogin_ValidCredentialsRedirectsToDashboard` |
| POST /logout | yes | true no-mock HTTP | API_tests | `security_test.go:TestAuth_LogoutClearsCookie` |
| GET /dashboard | yes | true no-mock HTTP | API_tests | `e2e_test.go:TestE2E_LoginJourney` |
| GET /users | yes | true no-mock HTTP | API_tests | `protected_test.go:TestProtectedRoutes_AuthenticatedReturns200` |
| GET /beds | yes | true no-mock HTTP | API_tests | `operations_test.go:TestBeds_IndexReturns200` |
| GET /beds/:id | yes | true no-mock HTTP | API_tests | `operations_test.go:TestBeds_Detail` |
| GET /admissions | yes | true no-mock HTTP | API_tests | `operations_test.go:TestAdmissions_IndexReturns200` |
| GET /admissions/:id | yes | true no-mock HTTP | API_tests | `operations_test.go:TestAdmissions_ResidentDetail` |
| GET /occupancy | yes | true no-mock HTTP | API_tests | `operations_test.go:TestOccupancy_Returns200` |
| GET /service-delivery | yes | true no-mock HTTP | API_tests | `operations_test.go:TestServiceDelivery_IndexReturns200` |
| GET /service-delivery/orders/:id | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestServiceDelivery_OrderDetail_Returns200` |
| GET /service-delivery/alerts/new | yes | true no-mock HTTP | API_tests | `operations_test.go:TestAlertCreate_FormRenders` |
| POST /service-delivery/alerts | yes | true no-mock HTTP | API_tests | `operations_test.go:TestAlertCreate_ValidDataRedirects` |
| GET /service-delivery/checkpoints/new | yes | true no-mock HTTP | API_tests | `operations_test.go:TestCheckpointCreate_FormRenders` |
| POST /service-delivery/checkpoints | yes | true no-mock HTTP | API_tests | `operations_test.go:TestCheckpointCreate_ValidDataRedirects` |
| GET /exercise-library | yes | true no-mock HTTP | API_tests | `operations_test.go:TestExerciseLibrary_IndexReturns200` |
| GET /exercise-library/:id | yes | true no-mock HTTP | API_tests | `operations_test.go:TestExerciseLibrary_Detail` |
| POST /exercise-library/:id/favorite | yes | true no-mock HTTP | API_tests | `operations_test.go:TestExerciseLibrary_FavoriteToggle` |
| GET /media/:filename | yes | true no-mock HTTP | API_tests | `operations_test.go:TestMedia_PathTraversalRejected` |
| GET /exams | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_Index_Returns200_AsAdmin` |
| GET /exams/scheduler | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_Scheduler_Returns200` |
| GET /exams/templates/new | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_TemplateNew_Returns200` |
| POST /exams/templates | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_TemplateCreate_RedirectsOnSuccess` |
| GET /exams/templates/:id | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_TemplateDetail_ExistingTemplate` |
| POST /exams/templates/:id | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestExams_TemplateUpdate_RedirectsOnSuccess` |
| GET /exams/sessions/new | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_SessionNew_Returns200` |
| POST /exams/sessions | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_SessionCreate_RedirectsOnSuccess` |
| GET /exams/sessions/:id | yes | true no-mock HTTP | API_tests | `e2e_test.go:TestE2E_ExamSessionJourney` |
| POST /exams/sessions/:id/block | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_BlockUpdate_HTMX_Returns200` |
| POST /exams/sessions/:id/publish | yes | true no-mock HTTP | API_tests | `exam_scheduler_test.go:TestExams_Publish_SucceedsWithNoConflicts` |
| POST /exams/sessions/:id/candidates/add | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestExams_SessionAddCandidate_Redirects` |
| POST /exams/sessions/:id/candidates/remove | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestExams_SessionRemoveCandidate_Redirects` |
| POST /exams/conflicts/:id/resolve | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestExams_ConflictResolve_Redirects` |
| GET /finance | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_AdminGets200` |
| GET /finance/payments/new | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_PaymentFormGet200` |
| POST /finance/payments | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_RecordCashPayment` |
| GET /finance/payments/:id | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_PaymentDetail_Returns200` |
| GET /finance/payments/:id/refund | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_RefundForm_Returns200` |
| POST /finance/payments/:id/refund | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_RefundCreate_Redirects` |
| GET /finance/shifts | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_ShiftList200` |
| POST /finance/shifts | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_OpenShift` |
| GET /finance/shifts/:id | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_ShiftDetail_Returns200` |
| POST /finance/shifts/:id/close | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_ShiftClose_Redirects` |
| GET /finance/batches/new | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_BatchImportFormGet200` |
| POST /finance/batches | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_BatchImport_CSVSuccess` |
| GET /finance/batches/:id | yes | true no-mock HTTP | API_tests | `coverage_test.go:TestFinance_BatchDetail_Returns200` |
| GET /finance/exports/new | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_ExportFormGet200` |
| POST /finance/exports | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_ExportCreate_CSVRedirectsToDownload` |
| GET /finance/exports/download | yes | true no-mock HTTP | API_tests | `finance_test.go:TestFinance_ExportDownload_PathTraversalBlocked` |
| GET /reports | yes | true no-mock HTTP | API_tests | `protected_test.go:TestProtectedRoutes_AuthenticatedReturns200` |
| GET /audit | yes | true no-mock HTTP | API_tests | `audit_test.go:TestAudit_AdminGets200` |
| GET /jobs | yes | true no-mock HTTP | API_tests | `protected_test.go:TestProtectedRoutes_AuthenticatedReturns200` |
| GET /config-versions | yes | true no-mock HTTP | API_tests | `config_versions_test.go:TestConfigVersions_AdminGets200` |
| POST /config-versions/snapshot | yes | true no-mock HTTP | API_tests | `config_versions_test.go:TestConfigVersions_TakeSnapshot` |
| POST /config-versions/rollback | yes | true no-mock HTTP | API_tests | `config_versions_test.go:TestConfigVersions_SnapshotThenRollback` |
| GET /config-versions/:id/edit | yes | true no-mock HTTP | API_tests | `config_versions_test.go:TestConfigVersions_EditFormGet200` |
| POST /config-versions/:id | yes | true no-mock HTTP | API_tests | `config_versions_test.go:TestConfigVersions_UpdateRedirectsOnSuccess` |
| GET /diagnostics | yes | true no-mock HTTP | API_tests | `diagnostics_test.go:TestDiagnostics_AdminGets200` |
| POST /diagnostics/exports | yes | true no-mock HTTP | API_tests | `diagnostics_test.go:TestDiagnostics_ExportCreate_AdminRedirects` |
| GET /diagnostics/exports/download | yes | true no-mock HTTP | API_tests | `diagnostics_test.go:TestDiagnostics_GenerateAndDownload` |

## API Test Classification
1. **True No-Mock HTTP**
- All `repo/API_tests/*.go` tests bootstrap real app and DB (`health_test.go:buildTestApp`, lines 46-58, calling `app.Bootstrap` and `app.New`) and send real HTTP requests via `httptest.NewRequest` + `a.Test`.

2. **HTTP with Mocking**
- None found.

3. **Non-HTTP (unit/integration without HTTP)**
- `repo/unit_tests/*.go`
- `repo/frontend_tests/*.mjs`
- `repo/e2e_tests/tests/*.ts` are browser HTTP tests (fullstack E2E), but not API-test package route probes.

## Mock Detection
Pattern scan performed for: `jest.mock`, `vi.mock`, `sinon.stub`, gomock/testify mock, explicit mock() calls.
- Result: no mocking/stubbing detected in API execution path tests.
- Evidence:
  - No matches in `repo/API_tests`, `repo/unit_tests`, `repo/e2e_tests`.
  - API tests use real bootstrap and real request path (`repo/API_tests/health_test.go:46-58`).

## Coverage Summary
- Total endpoints: **64**
- Endpoints with HTTP tests: **64**
- Endpoints with TRUE no-mock tests: **64**
- HTTP coverage: **100%**
- True API coverage: **100%**

## Unit Test Summary

### Backend Unit Tests
Files:
- `repo/unit_tests/auth_policy_test.go`
- `repo/unit_tests/config_test.go`
- `repo/unit_tests/crypto_test.go`
- `repo/unit_tests/csvutil_test.go`
- `repo/unit_tests/domain_models_test.go`
- `repo/unit_tests/exam_scheduler_test.go`
- `repo/unit_tests/finance_test.go`
- `repo/unit_tests/idempotency_test.go`
- `repo/unit_tests/migrator_test.go`
- `repo/unit_tests/occupancy_test.go`
- `repo/unit_tests/optimistic_lock_test.go`
- `repo/unit_tests/password_test.go`
- `repo/unit_tests/session_domain_test.go`
- `repo/unit_tests/timeliness_test.go`
- `repo/unit_tests/validation_test.go`
- `repo/unit_tests/xlsxutil_test.go`
- `repo/unit_tests/ziputil_test.go`

Backend modules covered:
- Services/policies/domain logic: auth policy/session domain, finance domain, exams conflict logic, idempotency service.
- Shared/platform: crypto/passwords/validation, migrator/seeding, csv/xlsx/zip utilities, optimistic lock utilities, occupancy helper logic.

Important backend modules NOT directly unit-tested:
- Handler/controller layer across modules (`internal/modules/*/handler/*`)
- Many repository implementations (`internal/modules/*/repository/*`)
- Most service implementations (`internal/modules/*/service/*`) beyond domain-style logic
- Middleware units (`internal/platform/middleware/*`) not isolated in dedicated unit tests

### Frontend Unit Tests (STRICT REQUIREMENT)
Frontend test files found:
- `repo/frontend_tests/app.test.mjs`
- `repo/frontend_tests/device-cache.test.mjs`
- `repo/frontend_tests/form-validation.test.mjs`
- `repo/frontend_tests/htmx-ui.test.mjs`

Framework/tool evidence:
- Node built-in test framework: `node:test` imports in all four files.

Direct frontend-module import/render evidence:
- `app.test.mjs` reads and executes real `web/static/js/app.js` (`app.test.mjs:16`, `runInContext`).
- `device-cache.test.mjs` reads and executes real `web/static/js/device-cache.js` (`device-cache.test.mjs:17`, `runInContext`).

Components/modules covered:
- `web/static/js/app.js` (DOMContentLoaded behavior, HTMX response error handling)
- `web/static/js/device-cache.js` (cache CRUD, LRU behavior, degradation path)
- Additional JS behavior assertions in synthetic DOM contexts (`form-validation`, `htmx-ui` files)

Important frontend components/modules NOT tested deeply:
- Server-rendered template files under `web/templates/**` are not unit-tested directly.
- CSS behavior in `web/static/css/app.css` untested.

**Mandatory Verdict: Frontend unit tests: PRESENT**

Strict failure rule (fullstack + frontend missing/insufficient):
- Missing condition is not met (frontend unit tests exist with direct module evidence).
- No CRITICAL GAP on frontend unit-test presence.

### Cross-Layer Observation
- Backend route/API coverage is very strong and broad.
- Frontend has both unit tests and browser E2E now (`repo/e2e_tests/tests/*.ts`), improving balance versus prior backend-heavy pattern.

## API Observability Check
Strong:
- Method/path and request bodies are explicit in many tests (`coverage_test.go`, `finance_test.go`, `exam_scheduler_test.go`, `e2e_test.go`).
- Response semantics are asserted (status, redirects, body markers), including improved body checks in `protected_test.go` and `security_test.go`.

Remaining weak areas:
- Some authorization tests still primarily status-based without deep payload assertions (parts of `audit_test.go`, selected RBAC checks in `security_test.go`).

## Tests Check
- Success paths: covered across auth/operations/exams/finance/config/diagnostics.
- Failure and validation: covered (bad credentials, missing fields, forbidden access, stale row_version, traversal checks).
- Edge cases: idempotency duplicates, optimistic lock conflicts, HTMX-specific behavior.
- Auth/permissions: broad role matrix checks in API + browser E2E.
- Integration boundaries:
  - API integration via real app/db (`app.Bootstrap` + `a.Test`).
  - Browser FE↔BE via Playwright suite (`e2e_tests/tests/*.spec.ts`) with live server target (`docker-compose.e2e.yml`).

`run_tests.sh` check:
- Docker-based execution: **OK** (`run_tests.sh:45-60`, `run_tests.sh:62-83`).
- Local dependency requirement in README/run script: no host runtime install required by primary path.

## End-to-End Expectations (fullstack)
- Requirement: real FE↔BE tests should exist.
- Evidence now present:
  - `repo/e2e_tests/tests/login.spec.ts`
  - `repo/e2e_tests/tests/rbac.spec.ts`
  - `repo/e2e_tests/tests/finance.spec.ts`
  - `repo/e2e_tests/tests/exams.spec.ts`
  - `repo/e2e_tests/tests/config.spec.ts`
  - `repo/e2e_tests/tests/htmx.spec.ts`
- Conclusion: expectation satisfied.

## Test Coverage Score (0–100)
**93/100**

## Score Rationale
- + Full endpoint coverage and true no-mock API route testing (100/100 route coverage).
- + Added real-browser fullstack E2E closes previous critical FE↔BE gap.
- + Frontend unit tests include direct module execution evidence.
- - Backend unit coverage still light on handler/repository/service internals.
- - Some tests remain assertion-light (status-focused) in selected areas.

## Key Gaps
1. No dedicated unit tests for most handlers, repositories, and many service implementations.
2. Some authorization/filter tests verify status with limited payload-depth assertions.
3. Template-level frontend correctness still relies more on integration than direct component-level checks.

## Confidence & Assumptions
- Confidence: high.
- Assumptions:
  - Route registration files inspected are the authoritative runtime routes.
  - Static evidence is used only; no execution outcomes were assumed.

## Test Coverage Final Verdict
- **PASS**

---

# README Audit

## README Location
- Required: `repo/README.md`
- Found: yes.

## Hard Gates
### Formatting
- Pass. Markdown is structured, readable, and organized by domain and operations.

### Startup Instructions (Backend/Fullstack)
- Pass. Includes exact `docker-compose up` command (`README.md:141-143`).

### Access Method
- Pass. URL and port specified (`README.md:152`, `README.md:168`).

### Verification Method
- Pass. Includes concrete API/UI checks and expected outcomes (`README.md:404-417`).

### Environment Rules (STRICT)
- Pass.
- No `npm install`, `pip install`, `apt-get`, manual DB setup instruction in README.
- Primary startup and test flows are Docker-contained (`README.md:333-338`, `README.md:342-347`).

### Demo Credentials (Conditional on auth)
- Auth exists; credentials and role list provided (`README.md:191-200`).
- Pass.

## Engineering Quality
- Tech stack clarity: strong (`README.md:25-38`).
- Architecture explanation: strong (`README.md:41-58`).
- Testing instructions: strong, includes unit/API/frontend/E2E and dockerized E2E path (`README.md:330-359`).
- Security/roles: strong (`README.md:207-327`, `README.md:404+`).
- Workflow clarity: good startup/access/verify/stop sequence.
- Presentation quality: strong consistency and table-driven reference format.

## High Priority Issues
- None.

## Medium Priority Issues
1. README claims the Playwright suite uses image `mcr.microsoft.com/playwright:v1.44.0-jammy` (`README.md:346`), but `docker-compose.e2e.yml` builds `playwright` from local `e2e_tests/Dockerfile` (`docker-compose.e2e.yml:41-44`). This is a documentation/implementation mismatch.

## Low Priority Issues
1. Folder tree in README ends at `run_tests.sh` and does not mention new `e2e_tests/` and `docker-compose.e2e.yml`, reducing structural completeness.

## Hard Gate Failures
- None.

## README Verdict (PASS / PARTIAL PASS / FAIL)
- **PASS**

## README Final Verdict
- **PASS**

