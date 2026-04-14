# AsymmFlow Deployment Sign-Off Checklist

Date: 2026-04-08  
Product: AsymmFlow / PH Trading ERP  
Purpose: Final pre-deployment checklist for release sign-off, packaged-app validation, workflow verification, and rollback readiness.

## Release Rule

- [ ] Do not approve deployment if any blocking item below is incomplete.
- [ ] Do not approve deployment if a UI flow appears successful but does not persist correctly in the database.
- [ ] Do not approve deployment if a record persists in the database but is unusable or misleading in the UI.
- [ ] Do not approve deployment if packaged-app startup creates a fresh unintended database or ignores the intended runtime database path.

## Sign-Off Summary

- Release candidate commit:
- Installed app path:
- Runtime database path:
- Reviewer:
- Date:
- Final recommendation: Ready to deploy / Deploy with caution / Not ready to deploy

## 1. Automated Release Gates

- [ ] `go test ./...` passes on the current branch.
- [ ] `npm run build` passes on the current branch.
- [ ] `wails build` passes on the current branch.
- [ ] Build artifacts are generated from the same commit that is being signed off.
- [ ] No unexpected generated files are pending for commit other than local artifacts, reports, backups, and caches.

Evidence:

- Go test result:
- Frontend build result:
- Wails build result:

## 2. Database and Startup Integrity

- [ ] Packaged `.env` contains `DATABASE_PATH=data/ph_holdings.db`.
- [ ] The app binary/bundle is kept beside the packaged `.env` and `data/ph_holdings.db`.
- [ ] Runtime DB path resolves to `~/.local/share/AsymmFlow/ph_holdings.db`.
- [ ] App startup uses the runtime DB instead of the packaged DB copy.
- [ ] No unintended new DB file is created on first launch.
- [ ] App relaunch after force quit reuses the same runtime DB cleanly.
- [ ] Packaged DB is sanitized for license activation but structurally matches the verified runtime/repo schema.
- [ ] Startup migration/foundation pass materializes:
  - [ ] Work / People / Notifications tables
  - [ ] Expenses tables
  - [ ] Payroll tables
  - [ ] Phase 7 rollout support
- [ ] Deployment audit shows no missing critical tables.

Evidence:

- Packaged `.env` path pin:
- Runtime DB path observed:
- Packaged DB path observed:
- Startup log notes:

## 3. License, Device, and Session Behavior

- [ ] Installed app launches on the same Mac without an unexpected license prompt.
- [ ] Existing activated device remains recognized after rebuild/reinstall.
- [ ] Active role and sidebar permissions render correctly after login/startup.
- [ ] Current employee context resolves correctly from user or license.

Evidence:

- Active license key or role:
- Employee context resolution:

## 4. Data Quality and Master Data Readiness

- [ ] Customer duplicate review has been completed.
- [ ] Supplier duplicate review has been completed.
- [ ] No active customer uses a UUID-looking `customer_id`.
- [ ] No visible hollow records are presented as operational records.
- [ ] Legacy quoted/RFQ offer shells are hidden from the default live operational list.
- [ ] Remaining duplicates, if any, are documented and explicitly accepted.

Known review items to confirm before final sign-off:

- [ ] Confirm post-cleanup customer master is acceptable.
- [ ] Confirm supplier duplicate pairs affecting reconciliation are acceptable.
- [ ] Confirm any retained legacy records are audit-only and not surfaced as live operational records.

Evidence:

- Customer cleanup report:
- Supplier cleanup notes:
- Accepted residual data issues:

## 5. Sales and Commercial Workflow Verification

- [ ] Opportunities load correctly and match expected pipeline counts.
- [ ] Costing sheets create, save, reload, and export correctly.
- [ ] Offers create, edit, and list correctly.
- [ ] Won offers convert or remain operationally usable with valid child records.
- [ ] Customer Orders create, edit, and load with valid order items.
- [ ] Upstream edits do not break downstream offer/order/invoice linkages.

Adversarial checks:

- [ ] Repeated click on Save / Convert / Create does not create duplicates.
- [ ] Partial data entry fails clearly instead of silently.
- [ ] Duplicate-looking data is not mistaken for live transactional data.

## 6. Operations and Fulfillment Verification

- [ ] Purchase Orders load and remain navigable.
- [ ] GRN / delivery-related views still function.
- [ ] Operations views do not have dead links or hollow drilldowns.
- [ ] Serial / fulfillment workflows still work or are explicitly deprecated and hidden.

Adversarial checks:

- [ ] Downstream documents remain valid after upstream edits.
- [ ] Modal close and reopen does not lose unsaved or recently-saved state unexpectedly.

## 7. Finance Workflow Verification

- [ ] Customer Invoices create and persist with valid `invoice_items`.
- [ ] Payments Received can be viewed and recorded correctly.
- [ ] Payments Made can be viewed and recorded correctly.
- [ ] Expenses draft, submit, approve, post, and pay flows persist correctly.
- [ ] Payroll period, run, approve, post, and payout flows persist correctly.
- [ ] Cash flow and finance dashboards load without schema gaps.

Blocking finance data checks:

- [ ] Active invoices with no `invoice_items` = 0
- [ ] Active orders with no `order_items` = 0
- [ ] Active operational offers with no `offer_items` = 0
- [ ] Active zero-total operational orders = 0

## 8. Work / People / Notifications Verification

- [ ] Team Board loads on first open without needing another Work sub-page to initialize it.
- [ ] Task assignment persists and reloads correctly.
- [ ] Assigned user displays correctly on cards and in detail modal.
- [ ] Task comments and activity history load correctly.
- [ ] People roster loads correctly.
- [ ] Notifications render and reflect real records.
- [ ] No dangling assignee IDs remain in active tasks.

RBAC checks:

- [ ] Assigned users can open task detail for their tasks.
- [ ] Team-wide task visibility policy is explicitly accepted.
- [ ] No role can access disallowed screens or actions.

## 9. Relationships / CRM Detail Verification

- [ ] Customer detail pages load without broken metrics or dead tabs.
- [ ] Supplier detail pages load without broken metrics or dead tabs.
- [ ] Dashboard tiles do not show misleading placeholder values such as unrated `0/5` badges.
- [ ] Contacts, notes, and related transactional summaries align with DB state.

## 10. Bank Reconciliation and OCR Verification

- [ ] Bank statement import works from the bank reconciliation page.
- [ ] Baseline import works with AI assist disabled.
- [ ] AI-assisted import works with feature flag enabled and bounded timeout.
- [ ] Statement rows preserve debit/credit polarity correctly.
- [ ] Charges and fees are not merged incorrectly.
- [ ] Statement totals and balances reconcile before persistence.
- [ ] Manual matching covers realistic operational cases.
- [ ] Split allocation or multi-candidate matching behavior is sufficient for real statements.

Feature flag validation:

- [ ] `ENABLE_AIML_BANK_STATEMENT_ASSIST` tested
- [ ] `BANK_STATEMENT_AI_TIMEOUT_MS` tuned to acceptable latency
- [ ] Import remains usable when AI assist times out or is unavailable

## 11. Deployment Workspace Verification

- [ ] Deployment workspace loads without errors.
- [ ] Deployment audit renders current missing-table and data-issue state.
- [ ] Rollout checklist is editable and persists.
- [ ] Support export / deployment support actions remain functional.

## 12. Packaged-App Manual Sign-Off

- [ ] Fully quit the app from the Dock.
- [ ] Launch the installed app from `/Applications/AsymmFlow.app`.
- [ ] Launch from the copied deployment folder, not from a detached binary.
- [ ] Verify startup completes without hang.
- [ ] Verify relaunch after force quit completes without hang.
- [ ] Verify first launch from a clean copied package uses the bundled DB path without creating an unintended sibling DB elsewhere.
- [ ] Open each required screen at least once:
  - [ ] Opportunities
  - [ ] Costing
  - [ ] Offers
  - [ ] Customer Orders
  - [ ] Operations
  - [ ] Customer Invoices
  - [ ] Payments Received
  - [ ] Payments Made
  - [ ] Expenses
  - [ ] Payroll
  - [ ] Work
  - [ ] People
  - [ ] Notifications
  - [ ] Relationships / CRM details
  - [ ] Deployment

## 13. Rollback and Recovery Preparedness

- [ ] Verified runtime DB backup exists before release.
- [ ] Verified repo DB / packaged DB provenance is documented.
- [ ] Rollback owner is named.
- [ ] Rollback command or restore method is documented.
- [ ] Support contact and escalation path are documented.

Rollback owner:

- Name:
- Contact:
- Restore source:

## 14. Final Decision

- [ ] All blocking items passed.
- [ ] Any partial items are documented with owner and deadline.
- [ ] Residual risks are accepted explicitly.
- [ ] Release recommendation is recorded.

Residual risks:

- 

Decision notes:

- 
