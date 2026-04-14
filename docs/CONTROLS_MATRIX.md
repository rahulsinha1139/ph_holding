# Controls Matrix

Date: 2026-04-14
Status: Initial working matrix

This document is a practical control register for the repository and product.

| Control Area | Control | Current Implementation | Evidence Source | Owner | Cadence |
| --- | --- | --- | --- | --- | --- |
| Access control | Backend permission checks on sensitive actions | `requirePermission(...)` used on Wails-bound backend methods | `app.go`, service files, RBAC review notes | Engineering | Per release |
| Access governance | Permission model and approval posture are documented for operators and buyers | RBAC and approval narrative maintained in repo docs | `docs/RBAC_AND_APPROVALS.md` | Engineering | Per material auth change |
| Local data ownership | SQLite remains primary datastore | Local-first database design | `README.md`, `db_manager.go` | Engineering | Per architecture change |
| Optional sync | Remote sync is optional, not required for core operation | Supabase connection is conditional | `db_manager.go`, config review | Engineering | Per release |
| Backup and restore | App-controlled backup flow exists | Backup methods and retention logic implemented | `database.go`, drill notes, evidence log | Engineering / Ops | Monthly drill target |
| Deployment clarity | Deployment modes are documented explicitly | Local-only and sync-enabled modes described separately | `docs/DEPLOYMENT_MODES.md` | Engineering | Per deployment change |
| Role and admin controls | Settings and backup functions require privileged access | Settings and backup routes guarded | `database.go`, `db_manager.go`, `db_sync_service.go` | Engineering | Per release |
| Secure change discipline | AI-assisted changes require human review | Policy being formalized | `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md` | Engineering | Continuous |
| Secure development | Production changes follow a documented secure-development baseline | Secure development standard added | `docs/SECURE_DEVELOPMENT_STANDARD.md` | Engineering | Per release |
| Build quality | Backend and frontend build checks should pass in CI | Quality workflow introduced and locally validated | `.github/workflows/quality.yml`, `docs/EVIDENCE_LOG.md` | Engineering | Every PR |
| Secret detection | Repo should be scanned for committed secrets | Security workflow introduced | `.github/workflows/security.yml`, scan logs | Engineering | Every PR |
| Vulnerability review | Go dependencies and toolchain should be reviewed for known vulns | `govulncheck` configured, locally run, and tied to toolchain patching | `.github/workflows/security.yml`, `docs/EVIDENCE_LOG.md` | Engineering | Every PR |
| Static security review | Go code should be scanned for common issues | `gosec` planned in CI | `.github/workflows/security.yml` | Engineering | Every PR |
| Frontend dependency review | Production npm dependencies should be reviewed for high-severity issues | `npm audit --omit=dev --audit-level=high` configured and locally validated | `.github/workflows/security.yml`, `docs/EVIDENCE_LOG.md` | Engineering | Every PR |
| Threat awareness | High-risk workflows are explicitly enumerated | Initial threat model created | `docs/THREAT_MODEL.md` | Engineering | Per major change |
| Security verification | Verification expectations are documented and reviewable | ASVS-style self-review and test plan added | `docs/OWASP_ASVS_SELF_REVIEW.md`, `docs/SECURITY_TEST_PLAN.md` | Engineering | Per major change |
| Evidence discipline | Material scans and drills should be logged | Evidence log created | `docs/EVIDENCE_LOG.md` | Engineering / Ops | Ongoing |

## Notes

- This matrix is intentionally modest at the start.
- Controls should only be listed if they can be evidenced.
- As the evidence layer matures, this matrix should gain explicit owners, dated review entries, and links to artifacts.
