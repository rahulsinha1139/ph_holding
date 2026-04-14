# Evidence Log

Date Opened: 2026-04-14
Purpose: record dated proof for security, quality, and operational diligence work

## How To Use

Log entries should be factual and lightweight.

Each entry should include:

- date
- activity
- artifact or command
- result
- follow-up if needed

## Entries

| Date | Activity | Artifact / Command | Result | Follow-Up |
| --- | --- | --- | --- | --- |
| 2026-04-14 | Trust-signal foundation created | `SECURITY.md`, buyer overview, AI policy, controls matrix, threat model, evidence log, CI workflow files | Initial trust documentation and baseline workflow definitions added to repo | Populate with first scan and drill evidence |
| 2026-04-14 | Trust packet and governance bundle expanded | Deployment modes, architecture/data flow, backup/recovery, RBAC/approvals, vulnerability disclosure, secure development, release, incident, dependency, ASVS, and security test docs | Repository trust layer now covers buyer packet, internal governance, and first-pass verification structure | Run baseline scans and log the first non-documentary evidence |
| 2026-04-14 | Baseline local build verification | `go build ./...` and `npm run build` | Backend build completed successfully. Frontend production build completed successfully with existing Svelte accessibility, unused CSS, and large-bundle warnings but no build failure | Triage frontend warning clusters as a dedicated hardening pass |
| 2026-04-14 | Frontend production dependency audit | `npm audit --omit=dev --audit-level=high` | `found 0 vulnerabilities` | Add production npm audit to CI security workflow |
| 2026-04-14 | Go vulnerability review and toolchain response | `govulncheck ./...`, local `go version`, `go.mod`, CI workflow updates | `govulncheck` on local `go1.25.3` reported 10 reachable Go standard-library vulnerabilities with fixes in patched `go1.25.x` releases. Repo updated to target `toolchain go1.25.9`, and CI Go version was pinned to `1.25.9` | Re-run full `govulncheck ./...` and backend build under the patched toolchain and record the clean result |
| 2026-04-14 | Post-toolchain verification | local `go version`, `go build ./...`, `govulncheck ./...` | Local toolchain resolved to `go1.25.9`. Backend build completed successfully. `govulncheck` reported `0` reachable vulnerabilities in repo code after the toolchain update | Review the remaining non-reachable imported/module vulnerabilities with verbose output in a future dependency pass |
| 2026-04-14 | Non-production backup and restore drill | copied `ph_holdings.db` into `temp/backup_drill_2026_04_14`, ran `PRAGMA integrity_check`, `VACUUM INTO`, copied backup to restored DB, then re-ran integrity and sample table counts | Source-copy integrity was `ok`, restored-copy integrity was `ok`, backup file was created successfully, and sample counts matched for `customers`, `suppliers`, `invoices`, and `supplier_invoices` | Repeat the drill later through the productized app path and record an operator-friendly restore checklist |
| 2026-04-14 | Secret scanning baseline and repo hygiene response | local `gitleaks` report in `temp/gitleaks_report_2026_04_14.json`, repo doc/config updates | `gitleaks` found 16 leaks, dominated by historical committed secrets plus a currently tracked `.env` and literal password examples in repo docs/comments. Repo remediation started by redacting doc examples, changing CI to scan the current tree, and preparing `.env` to be kept local-only | Untrack `.env`, rotate any still-live exposed credentials, and decide whether history rewrite is warranted later |
| 2026-04-14 | Static security baseline | local `gosec ./...` | `gosec` reported a large baseline (`821` findings), dominated by low-signal `G104` unchecked-error noise and `G115` conversion warnings, with some higher-signal findings in OCR/artifact paths | Triage by rule and path, then attack the first bounded `G115` and trust-sensitive `G104` clusters instead of treating the raw count as equally actionable |
| 2026-04-14 | Repo-facing secret hygiene remediation | `.env` removed from git tracking, `.gitignore`, docs/comment redactions, `security.yml` gitleaks step updated | Live `.env` remains local on the workstation but is no longer tracked by git. Literal password examples in `AGENTS.md`, `CLAUDE.md`, and `supabase_migration.sql` were sanitized. CI gitleaks now scans the current tree rather than historical commits | Rotate any still-live credentials that were previously committed in history and decide later whether history rewrite is worth the cost |
| 2026-04-14 | Tracked snapshot secret scan verification | exported tracked snapshot plus local `gitleaks detect --no-git --source <snapshot>` | After the repo hygiene pass and test-fixture cleanup, the tracked snapshot scan reported `no leaks found` | Keep fixture strings and local-only env handling aligned so the scan remains actionable |
| 2026-04-14 | Focused frontend accessibility hardening | `npm run build` after targeted fixes in supplier/customer modal and control files | Five high-signal Svelte accessibility hotspots were corrected in the targeted frontend files. Frontend build still reports broader repo warnings, but the warning profile moved off the files touched in this pass | Continue with the next warning cluster in settings, opportunities, offers, and supplier detail screens |
| 2026-04-14 | Regression check for scan-fixture cleanup | `go test ./... -run \"TestSetAPIKeys_EncryptsToDatabase|TestReassignEmployeeLicenseAccess\"` | Targeted tests passed after converting remaining gitleaks fixture hits into safer non-secret literals | Expand the regression band if more fixture or scan-related test data is normalized later |
