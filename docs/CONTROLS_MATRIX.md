# Controls Matrix

Date: 2026-04-14
Status: Initial working matrix

This document is a practical control register for the repository and product.

| Control Area | Control | Current Implementation | Evidence Source | Owner | Cadence |
| --- | --- | --- | --- | --- | --- |
| Access control | Backend permission checks on sensitive actions | `requirePermission(...)` used on Wails-bound backend methods | `app.go`, service files, RBAC review notes | Engineering | Per release |
| Local data ownership | SQLite remains primary datastore | Local-first database design | `README.md`, `db_manager.go` | Engineering | Per architecture change |
| Optional sync | Remote sync is optional, not required for core operation | Supabase connection is conditional | `db_manager.go`, config review | Engineering | Per release |
| Backup and restore | App-controlled backup flow exists | Backup methods and retention logic implemented | `database.go`, drill notes, evidence log | Engineering / Ops | Monthly drill target |
| Role and admin controls | Settings and backup functions require privileged access | Settings and backup routes guarded | `database.go`, `db_manager.go`, `db_sync_service.go` | Engineering | Per release |
| Secure change discipline | AI-assisted changes require human review | Policy being formalized | `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md` | Engineering | Continuous |
| Build quality | Backend and frontend build checks should pass in CI | Quality workflow introduced | `.github/workflows/quality.yml` | Engineering | Every PR |
| Secret detection | Repo should be scanned for committed secrets | Security workflow introduced | `.github/workflows/security.yml`, scan logs | Engineering | Every PR |
| Vulnerability review | Go dependencies should be reviewed for known vulns | `govulncheck` planned in CI | `.github/workflows/security.yml` | Engineering | Every PR |
| Static security review | Go code should be scanned for common issues | `gosec` planned in CI | `.github/workflows/security.yml` | Engineering | Every PR |
| Threat awareness | High-risk workflows are explicitly enumerated | Initial threat model created | `docs/THREAT_MODEL.md` | Engineering | Per major change |
| Evidence discipline | Material scans and drills should be logged | Evidence log created | `docs/EVIDENCE_LOG.md` | Engineering / Ops | Ongoing |

## Notes

- This matrix is intentionally modest at the start.
- Controls should only be listed if they can be evidenced.
- As the evidence layer matures, this matrix should gain explicit owners, dated review entries, and links to artifacts.

