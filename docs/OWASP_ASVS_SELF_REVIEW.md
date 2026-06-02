# OWASP ASVS Self Review

Date: 2026-04-14
Basis: lightweight repository-level self-review inspired by OWASP ASVS
Status: initial pass

## Purpose

This is not a formal ASVS certification artifact.

It is a practical self-review that helps us answer a buyer or operator question with evidence instead of confidence language.

## Status Meanings

- `Implemented`: current repo evidence supports the control direction
- `Partial`: some control exists, but evidence or coverage is incomplete
- `Needs verification`: likely true or intended, but should be tested or reviewed directly
- `Gap`: not yet established well enough to claim

## Review Table

| Area | Check | Status | Evidence / Notes |
| --- | --- | --- | --- |
| Architecture | Trust boundaries are identified for local runtime, backend enforcement, and optional remote sync | Implemented | `docs/THREAT_MODEL.md`, `docs/ARCHITECTURE_AND_DATA_FLOW.md` |
| Access Control | Sensitive backend actions are permission-gated server-side | Implemented | `requirePermission(...)` usage across `app.go` and service files |
| Access Control | Frontend visibility is not treated as sufficient authorization | Implemented | frontend permission filtering exists, backend checks still enforced |
| Access Control | Privileged operations such as backup/settings access are restricted | Implemented | `database.go`, `app.go` |
| Business Logic | Finance and settlement workflows use controlled backend state transitions | Partial | major normalization work completed, but should keep being regression-tested |
| Input Validation | High-risk input paths use backend validation rather than UI-only assumptions | Partial | repo has many validations, but not yet documented exhaustively |
| Data Protection | SQLite is primary and deployment modes are explicitly separated | Implemented | `config.go`, `db_manager.go`, deployment docs |
| Data Protection | Backup creation and integrity validation exist | Implemented | `database.go` |
| Data Protection | Restore process is formally drilled and evidenced | Gap | backup exists, drill evidence still needs to be accumulated |
| Secrets | Baseline secret scanning workflow exists | Implemented | `.github/workflows/security.yml` |
| Secrets | Secrets handling is documented well enough for operators | Partial | posture improving, deployment guidance still maturing |
| Logging | Sensitive values should not be exposed in logs or errors | Partial | prior hardening exists, but should remain an explicit review area |
| Configuration | Cloud sync is disabled by default unless explicitly enabled | Implemented | `config.go` |
| Configuration | Remote configuration is safe and reviewed before deployment | Needs verification | policy/docs exist, evidence cadence needs buildup |
| File And Document Handling | Export and document flows are backend-mediated | Implemented | backend export and report generation paths |
| Error Handling | Security-sensitive failures should fail closed where practical | Partial | many guarded paths exist, but not yet reviewed systematically |
| Verification | Baseline build and security checks exist in CI config | Implemented | `.github/workflows/quality.yml`, `.github/workflows/security.yml` |
| Verification | Scan and drill evidence is being accumulated | Partial | evidence log exists but needs more entries |

## Priority Follow-Up

The highest-value follow-up checks are:

- run and log baseline scans
- perform and log a backup restore drill
- review auth and privileged operations for any remaining unguarded backend paths
- keep settlement and reconciliation flows under targeted regression tests
