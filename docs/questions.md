# Business Logic Questions Log

## 1) Settlement Shift Windows and Operational Flow
**Question**  
The prompt specifies shift-close times (7:00 AM / 3:00 PM / 11:00 PM), but does not clarify whether shifts are auto-created/auto-closed or manually controlled.

**My Understanding / Hypothesis**  
Shift boundaries are business-defined windows, but staff should still control opening/closing to support offline operations and reconciliation exceptions.

**Solution**  
Implemented manual shift lifecycle with named shifts (`morning`, `afternoon`, `night`) and explicit open/close actions; close computes and stores settlement totals, with audit + job history entries.

## 2) Timeliness Rule Boundary for Service Execution
**Question**  
“On-time if completed within 15 minutes of scheduled start” is explicit, but edge behavior (exactly 15 minutes, early completion) is not explicitly stated.

**My Understanding / Hypothesis**  
Exactly 15 minutes should count as on-time; early completions should also be on-time because they are within the threshold.

**Solution**  
Implemented timeliness logic as minute delta `<= 15` = on-time, `> 15` = late, and surfaced this in dashboard/order views and metrics aggregation.

## 3) Timeliness Denominator Definition
**Question**  
Prompt asks for execution rate and timeliness, but does not explicitly define whether timeliness rate denominator is only completed executions or all scheduled execution records.

**My Understanding / Hypothesis**  
Timeliness should reflect all scheduled execution outcomes (completed/refused/missed) to avoid inflated on-time rates.

**Solution**  
Implemented scheduled-count denominator from records with `scheduled_start` set; on-time/late are computed against scheduled records and displayed in service delivery analytics.

## 4) Exercise Device Cache Ownership on Shared Workstations
**Question**  
Prompt requires a “Clear Device Cache” action but does not specify whether cache is tied to user sessions or browser/device storage.

**My Understanding / Hypothesis**  
Cache should be device/browser-profile scoped (not account scoped), which matches shared workstation risk.

**Solution**  
Implemented browser-scoped cache with explicit “Clear Device Cache” control and guidance that sign-out does not clear local cache.

## 5) Device Cache Limits and Quota Failure Behavior
**Question**  
Prompt sets LRU limits (2 GB or 200 items) but does not define behavior if browser quota is lower than the configured cap.

**My Understanding / Hypothesis**  
Browser quota failures should degrade gracefully without breaking the exercise experience.

**Solution**  
Implemented LRU eviction up to configured limits plus graceful fallback on quota/IndexedDB failures (warn and skip caching item, app remains usable).

## 6) Exam Scheduling Time Semantics (Local Windows vs Stored UTC)
**Question**  
Prompt gives local time windows (e.g., 9:00 AM–12:00 PM) but does not define storage/compare timezone policy.

**My Understanding / Hypothesis**  
Input and display should be facility-local, while persistence/comparisons should remain UTC for consistency.

**Solution**  
Implemented facility-timezone parsing/formatting for forms and UI, conversion to UTC for storage and conflict calculations.

## 7) Exam Conflict Policy at Publish Time
**Question**  
Prompt requires conflict detection across rooms/proctors/candidates and visual adjustment before publishing, but does not explicitly say whether publish can proceed with unresolved conflicts.

**My Understanding / Hypothesis**  
Publish should be blocked while unresolved blocking conflicts exist.

**Solution**  
Implemented conflict detection and unresolved-blocking-conflict check in publish flow; publish returns conflict response until issues are resolved.

## 8) Offline Finance Gateways Mapping
**Question**  
Prompt lists offline gateways (cash, check, facility charge account, imported card-terminal batches) but does not define exact storage/processing model.

**My Understanding / Hypothesis**  
All gateway methods should normalize into the same payment ledger with method-specific reference handling.

**Solution**  
Implemented payment methods including cash/check/facility-charge-account/card, plus batch CSV import for card-terminal records into the same finance model.

## 9) Sensitive Reference Number Handling by View Context
**Question**  
Prompt requires encrypted-at-rest sensitive fields and masking in non-finance views, but does not define fallback behavior when encryption key is missing in development.

**My Understanding / Hypothesis**  
Production must hard-fail without encryption key for sensitive flows; development may allow startup with explicit warnings.

**Solution**  
Implemented production startup validation for encryption key, conditional encryption for sensitive payment references, and masking behavior in non-finance contexts.

## 10) Idempotency Key Transport and Scope
**Question**  
Prompt mandates idempotency on key create/update/approve/export actions but does not define transport format.

**My Understanding / Hypothesis**  
HTMX/server-rendered forms should support both header and hidden form field to cover browser and API-style submissions.

**Solution**  
Implemented canonical extraction from `X-Idempotency-Key` header or `_idempotency_key` form field with 24-hour TTL-backed dedup.

## 11) OCC Applicability Across Write Paths
**Question**  
Prompt requires optimistic locking on key write paths, but OCC is not meaningful for all operations (e.g., pure create).

**My Understanding / Hypothesis**  
OCC should be enforced where stale-read update risk exists (update/approve/block/publish/reset flows), while create flows rely on idempotency.

**Solution**  
Implemented `row_version` conflict handling for update-style paths (config/report/exam/session/user reset password, etc.) and idempotency for create-style duplicate prevention.

## 12) User-Facing Conflict Contract
**Question**  
Prompt says conflicts should return a “record changed” prompt that reloads latest state, but does not prescribe exact HTTP contract.

**My Understanding / Hypothesis**  
Conflict should consistently map to HTTP 409 with clear reload guidance.

**Solution**  
Implemented 409 conflict responses with user-facing “modified by another request/reload and try again” messaging in OCC-protected handlers.

## 13) Privileged-Action Audit Payload Completeness
**Question**  
Prompt requires immutable privileged audit logs with operator, IP, timestamp, and before/after details, but before/after may not always be available for every action type.

**My Understanding / Hypothesis**  
At minimum capture actor/IP/request/outcome/entity/action always; include before/after snapshots whenever applicable to the mutation type.

**Solution**  
Implemented immutable audit entries across privileged flows with actor, IP, request ID, outcome, and before/after snapshots where meaningful (finance, exams, config, reports, users, diagnostics).

## 14) Audit Traceability by Resident
**Question**  
Prompt requires audit search by resident, record type, and date range, but resident linkage is indirect for some entity types.

**My Understanding / Hypothesis**  
Resident filter should include direct resident_id fields and mapped entities (e.g., payments/service orders/admissions/alerts tied to resident).

**Solution**  
Implemented audit filtering with resident-aware query support so auditors can trace resident-related operational events through linked entities.

## 15) Report Scheduling Behavior and Ownership
**Question**  
Prompt requires local scheduled report generation to shared folders, but does not define execution actor and deduplication behavior.

**My Understanding / Hypothesis**  
Scheduler runs should be attributable and deduplicated per run window to avoid duplicate files after restart.

**Solution**  
Implemented in-process cron scheduler with run-history attribution to system scheduler actor and window-level dedup safeguards, writing outputs to local exports path.

## 16) Diagnostic Package Scope Boundaries
**Question**  
Prompt requires one-click diagnostic export (logs, recent job results, config snapshots), but does not define what must be excluded for security/privacy.

**My Understanding / Hypothesis**  
Include operational troubleshooting artifacts but exclude encryption keys, password hashes, and raw sensitive references.

**Solution**  
Implemented local ZIP diagnostic bundle including logs/job/config/audit snapshots with explicit exclusions and redaction behavior for sensitive material.
