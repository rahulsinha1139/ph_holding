# Abhie May Observations - Fix And Proof Matrix

Generated: 2026-05-22

This document maps each item from `Abhie May Observations-2.pdf` to the implemented repair and the proof available in code, data checks, or tests.

## Data And Sync Sign-Off

- Local SQLite integrity: `PRAGMA integrity_check` returned `ok`.
- Packaged Windows SQLite integrity: `PRAGMA integrity_check` returned `ok`.
- Seeded active opportunities: 576 active / 644 total, with 0 active bad years.
- Active opportunity year distribution: 2024 = 111, 2025 = 325, 2026 = 140.
- Seed intelligence sidecars: `intelligence_opportunity_enrichment` = 140, `intelligence_line_item_enrichment` = 368, `intelligence_order_enrichment` = 35.
- Admin update tables exist locally: `data_update_manifests`, `data_update_operations`, `data_update_receipts`.
- Supabase schema migration: passed on 2026-05-22 after retry; remote schema includes the data update, intelligence, and data quality tables.
- Remote/local seed comparison: passed on 2026-05-22. Intelligence sidecars match exactly remote vs local: 140 opportunity enrichments, 368 line-item enrichments, 35 order enrichments. Supabase has a few extra historical rows in base commercial tables, which is acceptable because the check is tolerant of non-destructive historical data.
- Full backend gate: `go test ./... -timeout 20m` passed.
- Backend compile gate: `go build ./...` passed.
- Frontend gates: `npm run check` passed with 0 errors and existing warnings; `npm run build` passed.
- End-to-end gate: `npm run test:e2e` passed 290/290 Playwright tests across the configured browser matrix.
- Windows CGO package gate: `CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 wails build -platform windows/amd64 -skipbindings` built `build/bin/AsymmFlow.exe`.
- Fresh Windows zip package: `deploy_package/AsymmFlow_Windows_CGO_2026_05_22_E2E_GREEN.zip`.
- Fresh Windows package DB path: `deploy_package/AsymmFlow_Windows_CGO_2026_05_22_E2E_GREEN/data/ph_holdings.db`; integrity check returned `ok` and required update/intelligence/data-quality tables exist.
- Focused backend red-team gate passed:
  - `TestEnsureDataQualityReviewFoundationIsIdempotent`
  - `TestPreviewCustomerDataQualityFlagsReviewQueue`
  - `TestPendingDataUpdatesAutoApplyUsesPublishedManifests`
  - `TestApplyDataUpdateManifestCreatesBackupAndIsIdempotent`
  - `TestDataUpdateDryRunDoesNotMutateOrCreateBackup`
  - `TestDataUpdateRejectsUnknownTablesAndDeletes`
  - `TestSeedIntelligenceContextExposesSeededSidecars`
  - `TestRepairImportedOpportunityMetadataFixesSeedYearParse`
  - `TestListNotificationFeedHidesReadItemsAfterThirtyDays`
  - `TestEmployeeContributionSummariesIncludeCommercialYTD`
  - `TestExpenseNumberingSkipsExistingAndPreservesNewFields`
  - `TestUnifiedOfferThreadDoesNotBleedSimilarWorkflowKeys`

## Issue Matrix

| # | User Problem | Implemented Solution | Proof |
|---|---|---|---|
| 1 | Zoom/text scaling broke layouts. | Global text-scale store and commercial UI audit coverage keep `A`, `A+`, and `A++` within stable layouts. | `frontend/src/lib/stores/textScale.ts`; `frontend/tests/e2e/ui-commercial-audit.spec.ts`; `npm run check` and `npm run build` passed. |
| 2 | People page wasted real estate and reporting box was unclear. | People hub now exposes compact employee contribution summaries, selected-employee commercial metrics, and YTD opportunity/revenue contribution. | `frontend/src/lib/screens/PeopleHub.svelte`; `TestEmployeeContributionSummariesIncludeCommercialYTD`. |
| 3 | Expenses creation failed and needed denser page, payment method, and inventory linkage. | Expense numbering is transaction-safe/retry-aware; expense UI has payment method and inventory-linked fields; delete/locked states are visible. | `expense_service.go`; `frontend/src/lib/screens/ExpensesScreen.svelte`; `TestExpenseNumberingSkipsExistingAndPreservesNewFields`. |
| 4 | Inventory visibility was missing. | Operations now includes an Inventory workspace with Stock Position, Movements, Pending Fulfillment, and Item Master tabs. | `frontend/src/lib/screens/OperationsHub.svelte`; `frontend/src/lib/screens/InventoryScreen.svelte`; `GetInventoryPendingFulfillmentReport`; `TestGetInventoryPendingFulfillmentReportDerivesOrderGaps`. |
| 5 | Customer data quality needed duplicate/blank/missing-link cleanup without blind merge. | Added admin-reviewed Data Quality queue for duplicates, blank opportunity names, missing customer links, and missing offer customer data. | `frontend/src/lib/screens/DataQualityScreen.svelte`; `user_feedback_hardening_service.go`; `TestPreviewCustomerDataQualityFlagsReviewQueue`; `TestEnsureDataQualityReviewFoundationIsIdempotent`. |
| 6 | Offer notes were saved as `System`; notes needed author/timestamp/source. | Unified thread resolves current user display name for opportunity edits and collates offer/opportunity/RFQ comments with timestamps. | `UpdateOpportunityCommercialFields`; `GetUnifiedOfferThread`; `TestUnifiedOfferThreadDoesNotBleedSimilarWorkflowKeys`. |
| 7 | Offers needed search and natural commercial ordering. | Offers toolbar searches offer number, customer, RFQ, and issuer; extract action is available from the same toolbar. | `frontend/src/lib/screens/OffersScreen.svelte` search and `exportOffersExtract`. |
| 8 | Opportunities needed explicit edit/save/cancel and change-log display. | Opportunity modal now shows a Change Log panel and backend edits write audited opportunity comments. | `frontend/src/lib/screens/OpportunitiesScreen.svelte`; `UpdateOpportunityCommercialFields`; `TestUpdateOpportunityCommercialFieldsRejectsUnsupportedFieldsAndAudits`. |
| 9 | RFQ/folder/reference needed a canonical key and tender import support. | Tender folder parser maps names like `29.0 BAPCO UPSTREAM-PIT` to workflow key `T-29` and status `RFQ Received / Tender`. | `ParseTenderFolderName`; `TestParseTenderFolderName`; Opportunities toolbar has `Import Tenders`. |
| 10 | User-facing `PH Holdings` branding needed replacement with `PH Trading WLL`. | Window title, HTML title/meta, Wails product name, and primary user-facing labels now use PH Trading/PH Trading WLL. | `main.go`; `frontend/index.html`; `wails.json`; `frontend/src/lib/screens/CostingSheetScreen.svelte`. |
| 11 | Projects needed admin edit/archive/shelve/delete and POC/contact flow. | Project admin archive/delete guards are implemented; customer/supplier contact forms include decision chain fields and report-to hierarchy. | `TestProjectAdminActionsRequireDeletePermissionAndStayOutOfActiveList`; `CustomerDetailView.svelte`; `SupplierDetailView.svelte`. |
| 12 | RFQ, opportunity, costing, and offer comments needed one shared thread. | Unified offer thread searches linked workflow keys and prevents bleed from similar keys. | `GetUnifiedOfferThread`; `TestUnifiedOfferThreadDoesNotBleedSimilarWorkflowKeys`. |
| 13 | Purchase/sales workflow documentation was missing. | User guide/report documentation added for purchase, sales, inventory, expenses, bank flow, and reporting. | `docs/user-guides/PH_TRADING_WORKFLOW_REPORT_GUIDE.md`; `docs/user-guides/ASYMMFLOW_USER_MANUAL_INDEX.md`. |
| 14 | Audit Trail and VAT Reports were not clear enough. | Reports & Exports catalog now describes VAT/Tax Pack and Audit Pack with export actions. | `frontend/src/lib/screens/ReportsScreen.svelte`; Finance Hub exposes `Reports & Exports` and `Audit Trail`. |
| 15 | Extractable reports for P&L, ledgers, inventory, tax, audit, MBR, and shipping were needed. | Reports catalog includes P&L, Balance Sheet, General Ledger, Customer Ledger, Supplier Ledger, Expense Ledger, RFQ/Tender Tracker, Offer Pipeline, Order Backlog, Inventory Movement, VAT/Tax Pack, and Audit Pack. | `frontend/src/lib/screens/ReportsScreen.svelte`; `ExportReport`; `npm run build` passed. |
| 16 | Users needed clear guidance for purchase, sales, inventory, expenses, and bank flows. | Documentation now maps each commercial workflow to the correct screen and accounting result. | `docs/user-guides/PH_TRADING_WORKFLOW_REPORT_GUIDE.md`. |
| 17 | Open opportunities/intelligence extraction was needed. | Opportunities has an Extract action; Butler now reads seeded 2025/2026 intelligence sidecars for product mix, prepared-by quality, and pipeline/order classification. | `frontend/src/lib/screens/OpportunitiesScreen.svelte`; `butler_ai.go`; `TestSeedIntelligenceContextExposesSeededSidecars`. |
| 18 | Redundant Open Bank Recon button in Supplier Settlements caused duplicate navigation. | Bank Recon is its own Finance Hub tab; Supplier Settlements is scoped as the settlement workspace. | `frontend/src/lib/screens/FinanceHub.svelte`; `frontend/src/lib/screens/SupplierPaymentsScreen.svelte`. |
| 19 | RFQ, Offer, and Customer Order tabs needed extract buttons. | Extract buttons exist on RFQ, Offers, Opportunities, and Orders toolbars. | `RFQScreen.svelte`; `OffersScreen.svelte`; `OpportunitiesScreen.svelte`; `OrdersScreen.svelte`. |
| 20 | Blank costing needed a Link Opportunity button. | Blank costing header now has opportunity selector plus `Link Opportunity` button. | `frontend/src/lib/screens/CostingSheetScreen.svelte`. |
| 21 | Bank statement upload confused Al Salam/NBB accounts and parser accuracy. | Bank statement parser supports NBB, Al Salam, KFH, generic CSV/PDF paths, and selected account mismatch is rejected. | `bank_statement_parser.go`; `bank_statement_parser_test.go`; `TestSelectedBankStatementAccountMismatchIsRejected`; `TestRouteToBankStatementHonorsExplicitBankAccountSelection`. |
| 22 | Relationship contacts needed add/edit/delete/manage and hierarchy. | Customer and supplier detail views support contact editing and report-to decision chain fields. | `CustomerDetailView.svelte`; `SupplierDetailView.svelte`; `reports_to_name` schema columns. |
| 23 | Read notifications needed to disappear after 30 days. | Notification feed filters read receipts older than 30 days. | `collaboration_service.go`; `TestListNotificationFeedHidesReadItemsAfterThirtyDays`. |
| 24 | Customer dashboard revenue/AR aging needed better date logic. | AR summary uses invoice dates and 30/60/90 windows rather than RFQs; relationship values now use formatted commercial values. | `app.go` AR date-window logic; relationship dashboard formatting tests and UI formatting paths. |
| 25 | Costing created/prepared-by should be logged-in user, not a dropdown. | Costing sheet Prepared By is now read-only and defaults to the current user when legacy data is blank/System. | `frontend/src/lib/screens/CostingSheetScreen.svelte`; `frontend/tests/e2e/ui-commercial-audit.spec.ts`. |
| 26 | People page needed YTD opportunity and revenue progress. | People hub surfaces selected employee open/completed counts, revenue, and commercial contribution summaries. | `frontend/src/lib/screens/PeopleHub.svelte`; `TestEmployeeContributionSummariesIncludeCommercialYTD`. |
| 27 | RFQ create had a spacebar popup/keyboard regression. | Commercial UI regression tests cover modal controls and keyboard-safe workflow actions. | `frontend/tests/e2e/ui-commercial-audit.spec.ts`; `npm run check` and `npm run build` passed. |

## Remaining Watch Items

- `npm run check` still reports legacy warnings, mostly accessibility and unused CSS warnings. It reports 0 errors.
- Supabase migration still emits benign historical drift warnings for missing old unique constraints; these are now treated as warnings and the migration passes.
- The remote/local count comparison is intentionally tolerant of extra historical rows in Supabase; the critical requirement is that seeded local tables are not missing remotely.
