# Codebase vs Backup Comparison - 2026-05-09

Current repo:
`/Users/rahulsinha/Desktop/House_of_Projects/ph_holdings`

Backup repo:
`/Users/rahulsinha/Desktop/House_of_Projects/AsymmFlow_Offline_Backup_20260504_110122/repository`

This is a read-only comparison report. No code was restored as part of this pass.

## Scope

I compared the source/config/doc code surface while excluding generated dependency/build/runtime noise such as `.git`, `node_modules`, `frontend/node_modules`, `build`, `frontend/dist`, `deploy_package`, `reports`, `outputs`, `backups`, runtime DBs, and other generated artifacts.

Within that code scope:

| Metric | Count |
|---|---:|
| Current code/config/doc files | 805 |
| Backup code/config/doc files | 750 |
| Files present in both | 750 |
| Changed common files | 142 |
| Unchanged common files | 608 |
| Current-only source/config/doc files | 55 |
| Backup-only source/config/doc files | 0 |

Important generated/runtime artifact note: outside the code scope, current has large deployment/build/report churn. The OCR/build agent separately observed current scoped runtime/generated files at 1615 vs backup 965, mostly from deployment packages, reports, and dist artifacts.

## Executive Conclusion

Current is not simply "ahead" or "behind." It has valuable newer work that should be preserved: bank reconciliation locking/reopen, payment-ledger settlement policies, PH offer numbering/signature fixes, dashboard chart work, serial/inventory validation, OCR zip safety, Rahul admin license recovery, and security documentation.

But the backup still contains important workflow hardening that current lost or weakened. The highest-risk regressions to merge back are:

1. App shell persistence/accessibility: lazy persistent screen mounting, text scale controls, activity monitoring.
2. Quick Capture bank/OCR flow: bank account selection, company/division capture, bank polarity repair, structured bank transaction preservation.
3. Finance pages: Accounting, Payments, Supplier Payments, Expenses, Notifications, Work Hub lost backend-backed edit/delete/review/cache flows.
4. Delivery Notes: backup had richer sorting/enrichment, PDF download action, and safer item fallback reconstruction.
5. Frontend API wrappers: backup had safer Wails model/date normalization; current often raw-casts payloads.
6. Supplier invoice OCR/create normalization: current has better payment policy, but backup handled currency/division/PO/exchange-rate/create fallback more completely.
7. Purchase order currency/division handling: current is stronger on product/supplier cost resolution but weaker on non-BHD/division paths.
8. Collaboration sync startup: current startup calls an RBAC-protected method, which can block the background collaboration loop before login.
9. `.gitignore`: current is missing backup deploy-package ignores, creating a real packaged `.env`/DB/app binary staging risk.
10. Test coverage: current added policy tests, but removed/narrowed OCR routing, pricing, RFQ/reference, edit-offer-to-costing, and costing persistence coverage.

## Page-By-Page Frontend Ledger

| Page | Current vs backup | Recommendation |
|---|---|---|
| `DashboardScreen.svelte` | Current adds executive revenue/pipeline/AR aging panels and one-line operating focus work. Risk: hardcoded 2026 assumptions. | Keep current, review year handling. |
| `AHSDashboard.svelte` | Current simplifies audited/source metrics into operational KPIs. Backup had richer audited financial presentation. | Mixed; decide per AHS requirement. |
| `AccountingScreen.svelte` | Current appears to revert account/voucher CRUD to local/mock-like arrays and drops backend account/journal APIs. | Restore/merge backup backend-backed accounting flows. |
| `ArchaeologistScreen.svelte` | Mostly presentation/control changes. | Low risk. |
| `ArrivalCeremony.svelte` | Mostly presentation changes. | Low risk. |
| `AuditTrailViewer.svelte` | Mostly presentation/control changes. | Low risk. |
| `BankReconciliationScreen.svelte` | Current is ahead: scoped bank accounts, wider statements panel, OCR notes, debit/credit flip, matching guards, finalized lock/reopen/stats. | Keep current, continue UI width polish. |
| `BookBankReconciliationScreen.svelte` | Current is smaller and appears to remove backup detail/polish. | Review against backup before keeping current. |
| `ButlerScreen.svelte` | Current simplifies markdown rendering and removes clarify/action-chip behavior. | Merge backup answer readability/actions if still desired. |
| `CRMCustomerDashboard.svelte` | Current improves labels/revenue wording but fixes years to 2023-2025. | Keep UI improvements, fix dynamic years. |
| `CRMSupplierDashboard.svelte` | Current improves modal/a11y but fixes years to 2023-2025 and uses hard defaults. | Keep UI improvements, fix dynamic years. |
| `CashPositionWidget.svelte` | Smaller display changes. | Low risk. |
| `ChequeRegisterScreen.svelte` | Mostly control/presentation changes. | Low risk. |
| `CostingSheetScreen.svelte` | Current keeps post-pull color/table/simplification work. Test coverage for pricing/edit-offer-to-costing was narrowed vs backup. | Keep current visuals, re-add backup workflow coverage and any missing edit hydration. |
| `CustomerDetailView.svelte` | Current changes detail presentation and now has comma formatting. Agent marked mixed/risky. | Keep comma formatting; UI-test customer 360 before restoring anything. |
| `CustomersScreen.svelte` | Current removes delete customer action and auto-generated customer code fallback. | Merge backup creation/delete safety if required. |
| `DeliveryNotesScreen.svelte` | Current restores create-with-items path but loses PDF action, enriched display, latest-first/year grouping, robust item fallbacks. | Restore/merge backup DN polish and PDF action. |
| `DeliveryTrackingScreen.svelte` | Mostly small presentation/control changes. | Low risk. |
| `ExpensesScreen.svelte` | Current removes company scoping, delete category/vendor/entry/recurring actions, and note previews. Data was cleaned, but UI workflows regressed. | Restore/merge backup expense management actions. |
| `FXRevaluationScreen.svelte` | Mostly minor display/control changes. | Low risk. |
| `FinanceHub.svelte` | Current has company tabs/navigation changes. | Mixed/minor. |
| `FinancialDashboard.svelte` | Current removes cash statement notices and divide-by-zero guards, changes KPI story. | Restore backup guards/notices. |
| `InboxScreen.svelte` | Mostly smaller flow changes. | Low/medium risk; smoke test OCR routing. |
| `InvoicesScreen.svelte` | Current is ahead on settlement language, credit-note tabs, ledger-driven balance/status, proforma filters. Risk: create permission check removed. | Keep current, review permission gate. |
| `LicenseActivationScreen.svelte` | Current aligns with Rahul/admin activation changes. | Keep. |
| `NotificationsScreen.svelte` | Current removes cached notifications, delete-approval review actions, pending-task handoff. | Restore/merge backup review/handoff flows. |
| `OffersScreen.svelte` | Current adds offer follow-ups and recent offer-number fixes; loses auto-open PDF, formatted BHD helpers, some quote metadata preservation. | Merge backup metadata/PDF conveniences without losing follow-ups/PH numbering. |
| `OperationsHub.svelte` | Mostly presentation/control differences. | Low risk. |
| `OpportunitiesScreen.svelte` | Current restores full stage list and `CreateRFQ`; loses `CreateRFQWithReference` and costing navigation payload dispatch. | Restore backup RFQ/reference/costing payload flow. |
| `OrdersScreen.svelte` | Current recently restores several order actions; agent still saw status/document-chain differences from backup. | Keep current fixes, compare document-chain/status normalization carefully. |
| `PaymentsScreen.svelte` | Current reframes as receipts but removes edit/delete, permission gates, reference validation, Wails update path. | Restore backup edit/delete/validation gates. |
| `PayrollScreen.svelte` | Current API wrapper lost some division/date model normalization. | Merge wrapper normalization. |
| `PeopleHub.svelte` | Current adds employee task history/license reassignment; drops snapshot caching/tabs. | Keep detail work, consider backup caching/state stability. |
| `PricingScreen.svelte` | Test coverage narrowed around blank numeric fields, unsaved restore, back-nav warning, edit-offer-to-costing. | Re-add coverage; verify actual UI. |
| `PurchaseOrdersScreen.svelte` | Mostly mixed UI/control changes; backend PO currency/division is the bigger risk. | Test non-BHD/division PO creation. |
| `QuotationScreen.svelte` | Mostly minor/presentation differences. | Low risk. |
| `ReportsScreen.svelte` | Mostly minor/presentation differences. | Low risk. |
| `SettingsScreen.svelte` | Current adds VPS/OCR settings but removes backup management UI and text-size controls. | Merge backup backup/readability controls. |
| `SetupWizard.svelte` | Mostly minor setup copy/control differences. | Low risk. |
| `ShowcaseScreen.svelte` | Mostly showcase-only changes. | Low risk. |
| `SupplierDetailView.svelte` | Current has comma formatting and presentation changes. | Keep formatting; smoke test supplier 360. |
| `SupplierInvoicesScreen.svelte` | Backend AP settlement is ahead; OCR/create normalization is risky. | Keep policy, restore creation/OCR normalization. |
| `SupplierPaymentsScreen.svelte` | Current adds open payable summary but removes edit/delete, permission gates, FX exchange-rate support. | Restore backup edit/delete/FX gates. |
| `SuppliersScreen.svelte` | Mostly presentation/control differences. | Low/medium risk; test create/edit. |
| `UserManagementScreen.svelte` | Current improves basic modal/role display but removes activity monitoring, opportunity conflict review, role permission API view. | Restore backup admin tooling. |
| `WorkHub.svelte` | Current simplifies remote refresh and reassignment confirmation but removes snapshot cache, pending task handoff, robust task opening/loading states. | Restore backup work/handoff resilience. |

## Shared Frontend Components And APIs

| Area | Current vs backup | Recommendation |
|---|---|---|
| `App.svelte` | Current eagerly imports screens and removes persistent lazy mounting, text-scale init, activity monitor hooks; adds useful manual document capture. | Merge backup persistence/accessibility/activity hooks while retaining manual capture. |
| `QuickCaptureModal.svelte` | High-risk regression: current loses bank-account picker, company/division capture, bank polarity repair, structured bank preservation. Current keeps manual type/a11y improvements. | Restore backup bank/OCR save routing and polarity logic. |
| `OpportunityDetail.svelte` | Current adds editable notes cards but removes versioned stage update, permission-gated offer actions, comment delete, legacy-note merge. | Merge backup safety gates/comment tooling. |
| `EnterpriseHeader.svelte` | Current removes text-size controls. | Restore if readability control remains a requirement. |
| `EnterpriseSidebar.svelte` | Current adds callback nav and better sync disabled/mobile handling. | Keep current. |
| `SyncProgress.svelte` | Current richer progress presentation. | Keep current. |
| `WabiModal.svelte` | Current improves modal/topbar/backdrop behavior. | Keep current. |
| `EChartsSurface.svelte`, `ExecutiveRevenueChart.svelte` | Current-only executive chart components. | Keep current. |
| `frontend/src/lib/api/collaboration.ts` | Current adds employee task/license APIs but removes model/date normalization, refresh throttling, timeout, delete approval review. | Merge backup normalization/review handling. |
| `frontend/src/lib/api/expenses.ts` | Current removes delete APIs, division fields, Wails input/date normalization. | Restore backup wrapper behavior. |
| `frontend/src/lib/api/payroll.ts` | Current removes division fields and date/model normalization. | Restore backup wrapper behavior. |
| `authContext.js` | Current removes permission-list normalization. | Restore defensive normalization. |
| `formatters.ts` | Current removes generic `formatNumber` and comma-safe parsing. | Restore safer formatter helpers. |

## Backend Business Engine Ledger

| File / engine | Current vs backup | Recommendation |
|---|---|---|
| `app.go` | Current adds PH offer-number repair, 2026 dashboard endpoints, AR/AP payment-state reports, signature PDF block, inventory valuation fixes. Risks: hardcoded 2026 YTD, customer profile no longer populates credit limit. | Keep current, fix hardcoded year and credit-limit regression. |
| `assets_service.go` | Current searches `deploy_package/data` for letterheads too. | Keep. |
| `bank_reconciliation_policy.go` | Current-only mutability/status policy. | Keep. |
| `bank_reconciliation_service.go` | Current ahead: statement mutability, normalized statuses, transactional finalize/reopen audit. | Keep. |
| `bank_integrity_service.go` | Current transactional force reimport/audit logging. | Keep. |
| `bank_transaction_matcher.go` | Current transactional manual matching and immutable statement guards. | Keep. |
| `collaboration_service.go` | Current adds prepared-by options and self-notification on reassignment. | Keep, watch self-notification noise. |
| `collaboration_sync.go` | Current removes `delete_approval_requests` and `opportunity_edit_conflicts` from fast collaboration sync and gates start/stop with RBAC. Startup still calls RBAC wrapper. | Fix startup to internal method and review table list. |
| `customer_invoice_payment_policy.go` | Current-only derived invoice settlement policy. | Keep. |
| `customer_invoice_service.go` | Current ahead on payment ledger/status/outstanding and proforma detail carry-over. Risk: delete approval guard removed. | Keep policy, review delete workflow. |
| `payment_service.go` | Current uses shared invoice payment policy. Risks: removed division assignment and delete approval guard. | Keep policy, restore division/delete guard if needed. |
| `finance_reporting_service.go` | Current reports use live outstanding/payment state. | Keep. |
| `customer_linkage_service.go` | Current uses derived invoice state and filters bad aliases. | Keep. |
| `customer_seed_policy.go`, `customer_write_policy.go` | Current-only customer seed/write helpers. | Keep, wire into normal CRUD where useful. |
| `database.go` | Current adds transient `SupplierInvoice.OutstandingBHD`. | Keep. |
| `db_sync_service.go` | Current stricter remote table availability checks and cached remote handle. | Keep, but Supabase schema must be complete. |
| `delivery_note_service.go` | Current serial validation before DN creation. | Keep. |
| `grn_service.go` | Current serial/inventory reconciliation and real discrepancy cost. Risk: delete approval guard removed. | Keep integrity, review delete guard. |
| `inventory_service.go`, `product_service.go`, product policy files | Current improves supplier/product linking, valuation, serialized product rules. | Keep. |
| `purchase_order_service.go` | Current improves supplier/product cost derivation but backup had stronger currency/exchange-rate/division normalization. | Merge backup currency/division handling. |
| `serial_number_service.go` | Current requires serial tracking config before serial registration. | Keep. |
| `supplier_invoice_payment_policy.go` | Current-only AP ledger-derived payment state. | Keep. |
| `supplier_invoice_service.go` | Current better AP payment state/approval rules, but lost backup currency normalization, division resolution, CreatedBy fallback, OCR exchange-rate calc, OCR PO lookup. | Merge backup OCR/create normalization. |
| `supplier_payment_service.go` | Current stronger AP payment recalculation but lost division assignment and delete approval guard. | Merge division/delete guard if required. |
| `supplier_seed_policy.go`, `supplier_write_policy.go`, `supplier_contact_policy.go` | Current-only canonical supplier helpers. | Keep; some are only partially wired. |
| `offer_signature_blocks.go` | Current-only PH prepared-by/contact signature blocks. | Keep. |
| `license_service.go` | Current adds Rahul admin rebind and RBAC-protected seed/flush wrappers; role permission sets changed. | Keep admin recovery, review role permission delta. |

## Sync, Config, OCR, Build, And Security

| Area | Finding | Action |
|---|---|---|
| Canonical sync table count | `dbSyncTables` is 66 in both current and backup; no additions/removals. | No table-list regression there. |
| Missing remote tables | Backup skipped all missing remote tables; current errors for non-optional tables and skips only optional user-activity tables. | Safer, but migrate Supabase before enabling sync. |
| User activity tables | Current correctly treats `user_activity_sessions`, `user_activity_events`, `user_activity_weekly_summaries` as optional remote tables. | Keep. |
| Collaboration sync startup | `app.go` calls `StartCollaborativeSyncLoop`, which now requires `settings:update`; before login this can no-op. | Fix to call internal startup method. |
| Config/app identity | Current app data dir is `AsymmFlow-PH-Trading`, app bundle `AsymmFlowPHTrading.app`, Wails name `AsymmFlowPHTrading`. | Good for branding, verify license/storage upgrade path. |
| Deployment DB reseed guard | New guard detects newer runtime collaborative state, but appears used only by tests. | Wire into actual reseed path or remove false confidence. |
| OCR engine | Only `pkg/ocr/engine.go` changed: checked ZIP uncompressed size conversion, overflow rejection. | Keep. |
| Butler OCR | Backup had a hardcoded fallback Mistral key. Current removes it and returns local fallback insight with `fallback_reason` when AI missing/fails. | Keep secret fix, expose degraded AI state in UI. |
| OCR accuracy | No core OCR extraction algorithm change found. Butler enrichment may silently degrade without Mistral. | Add UI warning/telemetry for fallback. |
| Frontend package | Current adds `echarts`. Go deps unchanged. | Keep if executive dashboard remains. |
| Vite build | Current removed manual Rollup chunks; dist becomes one JS/CSS pair instead of many chunks. | Verify packaged startup/performance. |
| `.gitignore` | Current misses backup deploy-package ignores for app binaries, packaged `.env`, packaged DBs. | Restore backup deploy-package ignore rules immediately. |
| Docs | Current adds security/compliance docs; backup-only QA matrix/signoff PDF absent. | Keep docs, recover QA artifacts if still needed. |

## Testing Delta

Current added useful Go policy tests for bank reconciliation, invoice/payment state, supplier payment state, product valuation/linking, deployment artifact ZIP size, config reseed, and collaboration notification behavior.

Current lost or narrowed meaningful frontend/regression coverage:

- OCR bank-statement save-routing assertion.
- Pricing blank numeric tolerance.
- Unsaved costing draft restore.
- Back-navigation warning.
- Edit-offer-to-costing hydration.
- RFQ reference and opportunity line-item normalization cases.
- Same-day validity and PDF subject/body export tests.
- FX-multiplied offer item repair tests.

Recommendation: before the next production package, restore backup tests for OCR routing, costing persistence/edit hydration, RFQ reference flow, and pricing edge cases.

## Changed Common Files

These 142 files exist in both repos and differ:

`AGENTS.md`, `CLAUDE.md`, `app.go`, `app_test.go`, `artifacts.go`, `assets_service.go`, `bank_integrity_service.go`, `bank_reconciliation_service.go`, `bank_transaction_matcher.go`, `butler_ai.go`, `collaboration_service.go`, `collaboration_sync.go`, `company_branding.go`, `config.go`, `customer_invoice_service.go`, `customer_invoice_service_test.go`, `customer_linkage_service.go`, `dashboard_stats_test.go`, `database.go`, `db_manager.go`, `db_sync_service.go`, `delivery_note_service.go`, `deployment_audit_test.go`, `finance_reporting_service.go`, `frontend/package-lock.json`, `frontend/package.json`, `frontend/playwright-report/index.html`, `frontend/src/App.svelte`, `frontend/src/app.css`, `frontend/src/assets/design-tokens.css`, `frontend/src/assets/theme.css`, `frontend/src/lib/api/collaboration.ts`, `frontend/src/lib/api/expenses.ts`, `frontend/src/lib/api/payroll.ts`, `frontend/src/lib/asyl/components/Modal.svelte`, `frontend/src/lib/components/AlchemyForm.svelte`, `frontend/src/lib/components/ContextTaskModal.svelte`, `frontend/src/lib/components/FlowState.svelte`, `frontend/src/lib/components/OfferCard.svelte`, `frontend/src/lib/components/OpportunityDetail.svelte`, `frontend/src/lib/components/ProductSearch.svelte`, `frontend/src/lib/components/QuickCaptureModal.svelte`, `frontend/src/lib/components/TimeMachine.svelte`, `frontend/src/lib/components/charts/RevenueChart.svelte`, `frontend/src/lib/components/consciousness/BlackSwanAlert.svelte`, `frontend/src/lib/components/ui/AlchemyForm.svelte`, `frontend/src/lib/components/ui/Badge.svelte`, `frontend/src/lib/components/ui/Card.svelte`, `frontend/src/lib/components/ui/DataTable.svelte`, `frontend/src/lib/components/ui/Dropdown.svelte`, `frontend/src/lib/components/ui/EnterpriseHeader.svelte`, `frontend/src/lib/components/ui/EnterpriseSidebar.svelte`, `frontend/src/lib/components/ui/Input.svelte`, `frontend/src/lib/components/ui/Select.svelte`, `frontend/src/lib/components/ui/SyncProgress.svelte`, `frontend/src/lib/components/ui/WabiModal.svelte`, `frontend/src/lib/screens/AHSDashboard.svelte`, `frontend/src/lib/screens/AccountingScreen.svelte`, `frontend/src/lib/screens/ArchaeologistScreen.svelte`, `frontend/src/lib/screens/ArrivalCeremony.svelte`, `frontend/src/lib/screens/AuditTrailViewer.svelte`, `frontend/src/lib/screens/BankReconciliationScreen.svelte`, `frontend/src/lib/screens/BookBankReconciliationScreen.svelte`, `frontend/src/lib/screens/ButlerScreen.svelte`, `frontend/src/lib/screens/CRMCustomerDashboard.svelte`, `frontend/src/lib/screens/CRMSupplierDashboard.svelte`, `frontend/src/lib/screens/CashPositionWidget.svelte`, `frontend/src/lib/screens/ChequeRegisterScreen.svelte`, `frontend/src/lib/screens/CostingSheetScreen.svelte`, `frontend/src/lib/screens/CustomerDetailView.svelte`, `frontend/src/lib/screens/CustomersScreen.svelte`, `frontend/src/lib/screens/DashboardScreen.svelte`, `frontend/src/lib/screens/DeliveryNotesScreen.svelte`, `frontend/src/lib/screens/DeliveryTrackingScreen.svelte`, `frontend/src/lib/screens/ExpensesScreen.svelte`, `frontend/src/lib/screens/FXRevaluationScreen.svelte`, `frontend/src/lib/screens/FinanceHub.svelte`, `frontend/src/lib/screens/FinancialDashboard.svelte`, `frontend/src/lib/screens/InboxScreen.svelte`, `frontend/src/lib/screens/InvoicesScreen.svelte`, `frontend/src/lib/screens/LicenseActivationScreen.svelte`, `frontend/src/lib/screens/NotificationsScreen.svelte`, `frontend/src/lib/screens/OffersScreen.svelte`, `frontend/src/lib/screens/OperationsHub.svelte`, `frontend/src/lib/screens/OpportunitiesScreen.svelte`, `frontend/src/lib/screens/OrdersScreen.svelte`, `frontend/src/lib/screens/PaymentsScreen.svelte`, `frontend/src/lib/screens/PayrollScreen.svelte`, `frontend/src/lib/screens/PeopleHub.svelte`, `frontend/src/lib/screens/PricingScreen.svelte`, `frontend/src/lib/screens/PurchaseOrdersScreen.svelte`, `frontend/src/lib/screens/QuotationScreen.svelte`, `frontend/src/lib/screens/ReportsScreen.svelte`, `frontend/src/lib/screens/SettingsScreen.svelte`, `frontend/src/lib/screens/SetupWizard.svelte`, `frontend/src/lib/screens/ShowcaseScreen.svelte`, `frontend/src/lib/screens/SupplierDetailView.svelte`, `frontend/src/lib/screens/SupplierInvoicesScreen.svelte`, `frontend/src/lib/screens/SupplierPaymentsScreen.svelte`, `frontend/src/lib/screens/SuppliersScreen.svelte`, `frontend/src/lib/screens/UserManagementScreen.svelte`, `frontend/src/lib/screens/WorkHub.svelte`, `frontend/src/lib/stores/authContext.js`, `frontend/src/lib/types.ts`, `frontend/src/lib/utils/formatters.ts`, `frontend/src/main.ts`, `frontend/test-results/results.json`, `frontend/tests/contracts/api.spec.ts`, `frontend/tests/e2e/butler-ocr-smoke.spec.ts`, `frontend/tests/e2e/error-scenarios.spec.ts`, `frontend/tests/e2e/finance-opportunity-regressions.spec.ts`, `frontend/tests/e2e/helpers/mockWailsBridge.ts`, `frontend/tests/e2e/ocr-payment-flow.spec.ts`, `frontend/tests/e2e/order-flow.spec.ts`, `frontend/tests/e2e/pipeline-commercial-smoke.spec.ts`, `frontend/tests/e2e/pricing-flow.spec.ts`, `frontend/tests/e2e/rfq-to-order.spec.ts`, `frontend/tests/e2e/sales-operations-regressions.spec.ts`, `frontend/tests/e2e/setup.spec.ts`, `frontend/tests/e2e/sonars.spec.ts`, `frontend/vite.config.ts`, `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/models.ts`, `grn_service.go`, `inventory_service.go`, `license_service.go`, `manual_deployment_package_test.go`, `payment_service.go`, `phase7_rollout.go`, `phase7_rollout_test.go`, `pkg/ocr/engine.go`, `product_service.go`, `purchase_order_service.go`, `serial_number_service.go`, `supplier_invoice_service.go`, `supplier_payment_service.go`, `sync_record_normalization_test.go`, `sync_service_impl.go`, `user_activity_monitoring_test.go`, `wails.json`, `workflow_regression_test.go`.

## Current-Only Code/Config/Doc Files

These 55 files are present in current and not in the backup code scope:

`.github/workflows/quality.yml`, `.github/workflows/security.yml`, `SECURITY.md`, `artifacts_test.go`, `bank_reconciliation_policy.go`, `bank_reconciliation_policy_test.go`, `collaboration_sync_test.go`, `config_reseed_test.go`, `customer_invoice_payment_policy.go`, `customer_invoice_payment_policy_test.go`, `customer_receipt_reconciliation_test.go`, `customer_seed_policy.go`, `customer_write_policy.go`, `docs/ACCESS_REVIEW_TEMPLATE.md`, `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md`, `docs/ARCHITECTURE_AND_DATA_FLOW.md`, `docs/AUTONOMOUS_STABILITY_SPRINT_2026_04_14.md`, `docs/BACKUP_AND_RECOVERY.md`, `docs/BUYER_SECURITY_OVERVIEW.md`, `docs/CONTROLS_MATRIX.md`, `docs/CUSTOMER_CREATION_SLICE_BRIEF.md`, `docs/DEPENDENCY_REVIEW_POLICY.md`, `docs/DEPLOYMENT_MODES.md`, `docs/EVIDENCE_LOG.md`, `docs/GITNEXUS_REFACTOR_FRAMEWORK.md`, `docs/INCIDENT_RESPONSE_RUNBOOK.md`, `docs/OWASP_ASVS_SELF_REVIEW.md`, `docs/RBAC_AND_APPROVALS.md`, `docs/RELEASE_CHECKLIST.md`, `docs/SECURE_DEVELOPMENT_STANDARD.md`, `docs/SECURITY_TEST_PLAN.md`, `docs/THREAT_MODEL.md`, `docs/TRUST_SIGNAL_SPEC_2026_04_13.md`, `docs/TRUST_SPRINT_PLAN_2026_04_14.md`, `docs/VULNERABILITY_DISCLOSURE.md`, `excel_write_guard.go`, `frontend/pnpm-lock.yaml`, `frontend/src/lib/components/charts/EChartsSurface.svelte`, `frontend/src/lib/components/charts/ExecutiveRevenueChart.svelte`, `offer_signature_blocks.go`, `pkg/ocr/engine_security_test.go`, `procurement_inventory_policy.go`, `procurement_inventory_reconciliation_test.go`, `product_operations_policy.go`, `product_operations_policy_test.go`, `product_supplier_link_policy.go`, `product_supplier_linkage_test.go`, `product_valuation_policy.go`, `product_valuation_policy_test.go`, `runtime_integration_settings.go`, `supplier_contact_policy.go`, `supplier_invoice_payment_policy.go`, `supplier_invoice_payment_policy_test.go`, `supplier_seed_policy.go`, `supplier_write_policy.go`.

## Suggested Repair Order

1. Restore `.gitignore` deploy-package ignores first to avoid accidental secret/DB/binary staging.
2. Fix collaboration sync startup and table list.
3. Restore Quick Capture bank/OCR routing and bank account/company/division/polarity handling.
4. Merge backup frontend API normalization and formatter helpers.
5. Page pass 1: Accounting, Payments, Supplier Payments, Expenses, Notifications, Work Hub.
6. Page pass 2: Delivery Notes, Opportunities/RFQ costing navigation, Offers metadata/PDF auto-open, Settings backup/text-size controls.
7. Backend pass: supplier invoice OCR/create normalization and purchase order currency/division handling.
8. Re-add removed regression tests for OCR routing, costing persistence/edit hydration, RFQ reference, pricing edge cases, and offer PDF/FX repair.
9. Run `npm run build`, `go test ./...`, and targeted Playwright flows around bank OCR, costing-to-offer edit, payments, work notifications, and delivery notes.
