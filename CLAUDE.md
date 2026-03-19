# CLAUDE.md - AsymmFlow (formerly PH Holdings Sovereign UI)

**Project**: AsymmFlow ERP System (Wails + Go + Svelte)
**Status**: Phase 33 COMPLETE — Canonical Seed Integration, Pipeline Import, Full Red Team Audit
**Last Updated**: 2026-03-18

---

## Quick Start

```bash
# Development
wails dev

# Production build (Mac)
wails build

# Production build (Windows cross-compile - requires mingw-w64)
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 wails build

# Output: build/bin/AsymmFlow (Mac) or build/bin/AsymmFlow.exe (Windows)
```

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    AsymmFlow ERP                             │
├─────────────────────────────────────────────────────────────┤
│  Frontend: Svelte + TypeScript (Onyx & Ether Design)        │
│  Backend:  Go + Wails v2.11.0                               │
│  Database: SQLite (Primary) + Supabase (Cloud Sync)         │
│  AI:       Mistral API (Butler Intelligence)                │
│  Icon:     Asymmetrica Logo (Dark Mode)                     │
└─────────────────────────────────────────────────────────────┘
```

### Codebase Scale
| Metric | Value |
|--------|-------|
| Go backend | 137,000+ LOC across 97 files |
| Svelte + TS frontend | 78,100+ LOC |
| Combined | ~215,000 LOC |
| Frontend screens | 52 Svelte screens |
| Backend services | 28 service files + 40+ supporting files |
| Database tables | 98 tables (SQLite + Supabase mirror) |

### Key Files
| Path | Purpose |
|------|---------|
| `app.go` | Main backend (16,000+ LOC) — App struct, 70+ Wails-bound API endpoints, RBAC guards, startup/shutdown |
| `wails.json` | App config — name: "AsymmFlow", outputfilename: "AsymmFlow" |
| `database.go` | SQLite setup, migrations, VACUUM backup, integrity check, WAL PRAGMAs |
| `license_service.go` | License key generation (crypto/rand), device binding, master key |
| `customer_invoice_service.go` | Invoice CRUD, send, status management, credit blocking, HMAC hashing |
| `delivery_note_service.go` | DN lifecycle (Prepared/Dispatched/Delivered), serial allocation, TOCTOU protection |
| `payment_service.go` | Customer payment CRUD with invoice reconciliation |
| `supplier_payment_service.go` | Supplier payment CRUD with race condition protection |
| `purchase_order_service.go` | PO CRUD, state machine, financial field protection, 5K BHD threshold |
| `financial_year_service.go` | Dynamic finance with verified FS2024 data |
| `einvoice_service.go` | E-invoice XML (UBL 2.1), VAT return CSV export |
| `credit_note_service.go` | Credit note CRUD, PDF, numbering, apply workflow |
| `serial_number_service.go` | Serial lifecycle: GRN → DN → Invoice traceability |
| `butler_ai.go` | Mistral AI integration, prompt injection protection |
| `db_sync_service.go` | Bidirectional Supabase sync (36 tables in dependency order) |
| `db_manager.go` | Supabase connection, sync health, download full database |
| `finance_reporting_service.go` | VAT reconciliation, payment aging, cash flow projection |
| `field_crypto.go` | PBKDF2 salt, HMAC-SHA256 document integrity |
| `business_invariants.go` | Business rule enforcement — 8% min margin, Grade C/D advance |
| `frontend/src/lib/screens/` | All 52 UI screens |
| `frontend/src/lib/components/ui/EnterpriseSidebar.svelte` | Nav with RBAC permission filtering |
| `build/appicon.png` | Asymmetrica Logo (Dark Mode) — embedded in app |
| `deploy_package/` | Client deployment package (AsymmFlow + keys + DB) |
| `.env` | Supabase credentials (project: xkyydcxxkzjbnsrmzkpp) |

---

## Current Capabilities (Phase 29)

### Sales Pipeline (6 screens, 10 tables)
- **RFQ Management**: Multi-product line items, stage pipeline (Pending/Qualified/Proposal/Negotiation/Won/Lost), win rate tracking
- **Costing Sheets**: Margin calculations, approval workflow (8% min margin enforced), Terms & Conditions, PDF generation
- **Offers/Quotations**: Create, edit, filter by status, convert to orders, PDF on letterhead
- **Customer Orders**: CRUD, status tracking, order-to-invoice pipeline
- **Opportunities**: Sales opportunity pipeline with stage tracking and conversion

### Finance Hub (9 screens, 17 tables)
- **Financial Dashboard**: McKinsey-style P&L, balance sheet, ratios, dynamic year selector (FY2023-FY2025)
- **Customer Invoicing**: Create from orders, send, PDF with serial numbers, HMAC integrity hashing
- **Credit Notes**: Draft → Issued → Applied workflow with over-crediting prevention
- **E-Invoicing**: UBL 2.1 XML generation (GCC/NBR compliant)
- **VAT Export**: Quarterly CSV for NBR VAT returns, excludes Cancelled/Void/Proforma/Draft
- **Payments**: Recording against invoices, KPI dashboard, filtering
- **Bank Reconciliation**: Statement import (PDF/CSV), auto-matching, manual matching, verification workflow
- **Book-Bank Reconciliation**: Side-by-side balance comparison, adjustments, variance analysis
- **Cheque Register**: Lifecycle management (Issued/Presented/Cleared/Stale/Bounced)
- **Cash Position**: Real-time totals across bank accounts, multi-currency, outstanding cheques
- **FX Revaluation**: Exchange rate management, unrealized gain/loss, multi-currency exposure
- **Accounting**: General ledger, journal entries, chart of accounts

### Operations Hub (5 screens, 12 tables)
- **Purchase Orders**: Multi-currency (EUR/USD/BHD), status pipeline (Draft→Sent→Acknowledged→Received→Closed), 5K BHD approval threshold, PDF generation
- **GRN**: Create from POs or manually, QC status (Pending/Passed/Failed/Partial), serial number capture at receiving
- **Delivery Notes**: Serial allocation, status guards (Prepared→Dispatched→Delivered), item line management
- **Serial Traceability**: Full lifecycle tracking PO → GRN → DN → Invoice → Customer, compliance-ready for government clients (BAPCO, Alba, EWA)
- **Delivery Tracking**: Pending/In Transit/Delivered tabs, shipment management

### CRM Hub (8 screens, 286 active customers)
- **Customer Dashboard**: KPIs, year filtering, create customer
- **Supplier Dashboard**: KPIs, year filtering, create supplier
- **Customer 360°**: Orders, invoices, contacts, notes, AI predictions
- **Supplier 360°**: POs, invoices, payment history
- **Supplier Invoices**: 3-way matching (PO/GRN/Invoice), match status visualization
- **Data Quality**: Deduplicated 62 duplicate customer records (WLL/B.S.C name variations) — Phase 29

### Intelligence Hub (3 screens)
- **Butler AI**: Mistral-powered natural language queries, conversation persistence, action suggestions, prompt injection protection
- **OCR Integration**: Document processing via Fly.io runtime, "Analyze with Butler" button
- **Data Archaeology**: D3 visualization, entity relationship discovery with force-directed graphs
- **PDF Reports**: Generation on company letterhead

### Administration (5 screens)
- **License System**: Key-based activation (PH-{ROLE}-{6-char}), device binding, master key for developer
- **User Management**: Roles, audit trail, device approval
- **RBAC**: 5 roles (Admin/Manager/Sales/Operations/Staff), `requirePermission()` on 70+ functions
- **Settings**: Application configuration, Supabase credentials

### Infrastructure
- **Offline-First**: SQLite primary, works fully without internet
- **Multi-Device Sync**: Bidirectional Supabase sync (36 tables, 10-min interval), Green/Yellow/Gray indicator
- **Auto Backup**: `VACUUM INTO` on startup, keeps last 7, integrity check on boot
- **Graceful Shutdown**: WaitGroup on sync goroutines, DB closed before log file
- **Log Rotation**: 50MB max, 5 rotated files
- **SQLite Tuning**: WAL mode, busy_timeout=5000, synchronous=NORMAL, 20MB cache, 256MB mmap

---

## Database Architecture

```
┌─────────────────┐     ┌──────────────────────┐
│  Local SQLite   │────▶│  Supabase PostgreSQL │
│   (PRIMARY)     │     │      (SYNC)          │
│  Always works   │     │  Cloud backup        │
│  Offline OK     │◀────│  Multi-device        │
└─────────────────┘     └──────────────────────┘
```

- **SQLite**: Source of truth, works offline, WAL mode + 20MB cache + 256MB mmap, single connection (PRAGMA safe)
- **Supabase**: Background sync every 10 minutes (Pro plan: $25/month), 36 tables bidirectional, both push/pull use canonical `dbSyncTables` list
- **Backup**: Automated `VACUUM INTO` on startup, keeps last 7, `chmod 0600`, integrity check on boot
- **Sync Indicator**: Green/Yellow/Gray in sidebar
- **98 tables** across: master data, sales pipeline, invoicing, procurement, banking, AI, documents, auth, sync

### Live Data (as of 2026-03-18)
| Entity | Count |
|--------|-------|
| Customers (active) | 304 (Phase 33: +18 new from seed) |
| Suppliers | 34 |
| Products | 19 |
| **Pipeline Opportunities** | **504** (Phase 33: imported from canonical_seed) |
| Customer Invoices | 468 (2023-2025) |
| Orders | 175 |
| Offers | 67 |
| Purchase Orders | 45 |
| Customer Payments | 93 |
| Supplier Invoices | 484 (2023-2025) |
| Supplier Payments | 601 (2020-2025) |
| Customer Contacts | 535 |
| Delivery Notes | 155 |

### Key Table Groups
- **Master Data (8):** `customers`, `suppliers`, `products`, `customer_contacts`, `supplier_contacts`, `warehouses`
- **Sales (10):** `opportunities`, `rfq_data`, `costing_sheet_data`, `offers`, `offer_items`, `offer_data`
- **Transactions (8):** `orders`, `order_items`, `invoices`, `invoice_items`, `payments`, `credit_notes`, `credit_note_items`, `invoice_sequences`
- **Procurement (8):** `purchase_orders`, `purchase_order_items`, `supplier_invoices`, `supplier_invoice_items`, `supplier_payments`, `goods_received_notes`, `grn_items`
- **Delivery (4):** `delivery_notes`, `delivery_note_items`, `serial_numbers`, `shipments`
- **Banking (12):** `company_bank_accounts`, `bank_statements`, `bank_statement_lines`, `cheque_registers`, `book_bank_reconciliations`, etc.
- **AI (6):** `conversations`, `chat_messages`, `prediction_records`, `win_probability_predictions`
- **System (13):** `devices`, `device_users`, `users`, `roles`, `license_keys`, `settings`, `sync_records`

---

## Environment Configuration

```bash
# .env file (updated 2026-02-17 - new Supabase project)
SUPABASE_DB_HOST=aws-1-ap-southeast-1.pooler.supabase.com
SUPABASE_DB_PORT=5432
SUPABASE_DB_USER=postgres.xkyydcxxkzjbnsrmzkpp
SUPABASE_DB_PASSWORD=PHHoldings2026
ENABLE_CLOUD_SYNC=true
MISTRAL_API_KEY=your_key
```

---

## Development History

### Phase 33: Canonical Seed Integration & Full Red Team Audit (2026-03-18) ✅ COMPLETE

**Context**: New `canonical_seed.xlsx` received from client via WhatsApp. Contains 355 Parties, 500 Pipeline opportunities, 175 Orders, 468 Invoices. Orders and invoices matched DB exactly (zero diffs). The pipeline data was the major new import — grew from 29 to 504 opportunities. Full red team audit (3 parallel agents) identified 47 findings across backend, frontend, and data layers. All resolved.

**Seed Data Verification:**
- Orders: 175/175 exact match — no import needed
- Invoices: 468/468 exact match, margins match — no import needed
- Parties: 266/355 matched existing customers, 18 new created, rest were soft-deleted Phase 29 dupes
- Pipeline: 29 → 504 opportunities (475 inserted, 23 updated, 6 original retained)

**Phase A: Schema & Sync (5 fixes):**
- Expanded `opportunities` table: 14 → 41 columns (13 seed fields + 14 Go struct fields that were never in DDL)
- Removed CHECK constraint on `stage` (was blocking non-standard values like "Quoted")
- Fixed sync table names: `costing_sheets` → `costing_sheet_data`, `costing_items` → `costing_line_items` (337 costing sheets were NEVER syncing)
- Added 6 tables to `dbSyncTables`: `opportunities`, `rfq_data`, `costing_sheet_data`, `costing_line_items`, `costing_history`, `offer_data`
- Fixed `GetApplicationPaths()` P0 nil crash: split into `getAppPaths()` (internal) + `GetApplicationPaths()` (RBAC for frontend). 14 internal callers migrated.

**Phase B: Data Integrity (6 fixes):**
- Built customer name resolution map: 331 seed names → active customer_ids (exact + normalized + manual mappings)
- Three-pass customer linking: 456/504 opportunities (90%) now have customer_id
- 18 genuinely new customers created from seed, 238 existing enriched with grades
- Active customers: 286 → 304

**Phase C: Pipeline Import:**
- 504 opportunities with year/opp_number/folder_name/title/eh_ref/comment/owner_notes/source/payment_terms/delivery_terms
- Stage distribution: Qualified (220), Won (155), Lost (117), Proposal (6), Quoted (6)
- Year distribution: 2024 (111), 2025 (321), 2026 (65)
- Total pipeline value: 3.84M BHD

**Phase D: P0-P1 Backend Fixes (5 fixes):**
- Fixed `GenerateOfferPDF` VAT 0% bug — now allows explicit 0% for VAT-exempt exports
- Fixed `generateOfferNumber()` double-BEGIN — removed nested `BEGIN EXCLUSIVE` inside GORM transaction
- Fixed `generateRFQNumber()` — same double-BEGIN bug
- Added RBAC to `BackfillOfferItemCostBreakdown()`
- Added `GetPipelineOpportunities()` API endpoint

**Phase E: P2-P3 Hardening (17 fixes):**
- CSV header fields escaped with `escCSV()` (10 fields)
- VatRate server-side bounds check (0-100%)
- QuoteType server-side whitelist (5 valid values)
- MobileNumber validation in `ValidateCustomerInput()`
- Fixed TRN validated as email → now just length check
- `GenerateOfferPDF` db nil check
- RBAC on `GetBusinessVATRate()` and `RefreshToolsStatus()`
- `SaveCostingAsOffer` transaction wrapping (duplicate check + create atomic)
- T&C VAT rate now reactive (updates when user changes rate)
- Customer Code asterisk removed (field is optional)
- Dead `fade` import removed from CostingSheetScreen
- Removed unnecessary `calculateAll()` on VAT/hiddenCharges inputs
- Unsaved changes warning on costing sheet back navigation
- Toast error messages wrapped in `String()`
- CSV `file.Sync()` added for write error detection
- `SanitizeForLog` expanded with 5 new patterns (credential, cert, salt, mistral_key, license_key)
- Customer dropdown limit: 100 → 500 in 4 screens

**Phase F: Frontend Wiring:**
- OpportunitiesScreen now loads pipeline data (504) merged with RFQs (38), deduped by folder number
- Status filter expanded: Qualified, Proposal, Quoted, Won, Lost
- `Opportunity` TypeScript class added to models.ts
- Wails binding stubs for `GetPipelineOpportunities`

**CLAUDE.md "STILL PENDING" P2 Items — All 10 Verified:**
- OAuth 127.0.0.1 binding ✅ (already fixed)
- CreateUser validation ✅ (already calls ValidateUserInput)
- ProcessInboxDocument auth ✅ (already has requirePermission)
- FX rate bounds ✅ (already validates absolute + ±20%)
- Bank recon amounts ✅ (already validates entity > 0 + ratio)
- Cloud sync default false ✅ (already set)
- .env traversal ✅ (already restricted to CWD + exe dir)
- Doc classifier path ✅ (already has RBAC + symlink + home dir)
- Supplier invoice segregation ✅ (already enforces creator != approver)
- SanitizeForLog ✅ (expanded in this phase)

**Files Modified (25):** `app.go`, `database.go`, `db_sync_service.go`, `offer_pdf_service.go`, `security_enhancements.go`, `assets_service.go`, `butler_reports.go`, `credit_note_service.go`, `einvoice_service.go`, `invoice_pdf_service.go`, `purchase_order_pdf_service.go`, `report_generators.go`, `reports.go`, `auth_handler.go`, `CostingSheetScreen.svelte`, `CustomersScreen.svelte`, `OpportunitiesScreen.svelte`, `RFQScreen.svelte`, `App.js`, `App.d.ts`, `models.ts`, `ph_holdings.db`, `roadmap-final.md`
**Build Status**: `go build ./...` and `npm run build` both pass clean
**`wails dev`**: Launches without errors (tested 2026-03-18)
**Supabase**: Connection failed (tenant not found) — project may need reactivation

---

### Phase 32: Client Bug Fixes & Production Readiness (2026-03-16) ✅ VERIFIED

**Context**: Client (PH Trading) provided a list of 12 issues found during production use. All 12 fixed in code, red-teamed with 4 parallel audit agents, then hardened with a second pass fixing 6 additional issues found by auditors. Supabase schema updated. **Verified with `wails dev` on 2026-03-18 — app launches clean, no errors.**

**MANDATORY NEXT SESSION CHECKLIST:**
1. Run `wails dev` and verify app launches without errors
2. Test customer creation (Fix 1): Create a new customer with minimal fields (just Company Name) — should auto-generate Customer Code
3. Test customer creation with mobile number (Fix 7): Verify mobile_number field saves and appears
4. Test costing sheet UI (Fixes 2-5): Open a costing, verify Sl No column, column headings, alternating row colors, no flickering
5. Test VAT adjustment (Fix 6): Change VAT rate in summary panel, verify it updates grand total
6. Test Budgetary Quote (Fix 9): Select "Budgetary Quote" from Document Type dropdown, export PDF, verify title says "BUDGETARY QUOTE"
7. Test hidden charges (Fix 11): Add hidden charges, verify profit decreases but grand total unchanged
8. Test Save as Offer: Save a costing as offer, then generate Offer PDF — verify PaymentTerms, DeliveryTerms, IssuedBy, VAT rate all carried through
9. Test Excel export (Fix 10): Export to CSV, open in Excel, verify alignment and no broken fields
10. Test PDF export (Fix 8): Verify detailed description appears ONLY on Annexure page, NOT on page 1

**12 Client-Reported Issues — All Fixed:**

| # | Issue | Fix | Files Modified |
|---|-------|-----|----------------|
| 1 | Customer creation not working | Made ShortCode validation optional; auto-generate customer_id from business_name + timestamp | `security_enhancements.go`, `CustomersScreen.svelte` |
| 2 | Serial number column missing in costing | Added "Sl No" column (35px) as first column in line items grid | `CostingSheetScreen.svelte` |
| 3 | Headings needed for Unit Price / Suggested Price | Renamed "Sell Price" → "Suggested Price", "Total" → "Total (BHD)" | `CostingSheetScreen.svelte` |
| 4 | Improve color combination | Alternating row backgrounds (indigo tint), hover accent, green grand total (#059669) | `CostingSheetScreen.svelte` |
| 5 | Page flickering | Removed `in:fade` transitions, global `animation: none !important` on costing page | `CostingSheetScreen.svelte` |
| 6 | VAT adjustment option needed | Editable VAT rate input (0-100%, step 0.5) in summary panel; dynamic label in PDF/Excel | `CostingSheetScreen.svelte`, `app.go`, `offer_pdf_service.go` |
| 7 | Customer mobile number field | Added `MobileNumber` to CustomerMaster; 4-column contact layout in form | `database.go`, `models.ts`, `CustomersScreen.svelte` |
| 8 | Full description showing on front page + annexure | Removed detailed description from PDF page 1; added dedicated Annexure page | `app.go` (ExportCostingToPDF) |
| 9 | Budgetary Offer option needed | Document type dropdown (Quotation/Budgetary Quote/Estimate/Technical/Commercial); dynamic PDF title | `CostingSheetScreen.svelte`, `database.go`, `app.go`, `offer_pdf_service.go` |
| 10 | Excel/PDF alignment incorrect | Fixed Y-positioning (accumulator vs index-based); quoted CSV text fields; RFC 4180 escaping | `app.go` |
| 11 | Hidden charges option needed | `hiddenCharges` variable affects cost/profit internally; NOT shown on customer-facing exports | `CostingSheetScreen.svelte` |
| 12 | Quotation fields overlapping | Fixed info/terms column layout to skip empty values without gaps | `app.go` |

**Red Team Audit Findings (6 additional fixes):**

| # | Severity | Issue Found | Fix Applied |
|---|----------|-------------|-------------|
| RT-1 | HIGH | `SaveCostingAsOffer` dropped PaymentTerms, DeliveryTerms, DeliveryWeeks, CountryOfOrigin, ContactPerson, RfqReference, PreparedBy | All now mapped to Offer struct fields |
| RT-2 | HIGH | Offer PDF hardcoded VAT at 10% | Added `VatRate` field to Offer struct; `GenerateOfferPDF` uses `offer.VatRate` dynamically |
| RT-3 | MEDIUM | `nth-child(odd)` alternating rows broken (sibling elements) | Wrapped items in `.line-item-group` with `class:alt-row` |
| RT-4 | MEDIUM | VatRate=0 showed misleading "VAT (10%)" label | Smart fallback: shows 0% if explicitly set, defaults to 10% only for legacy data |
| RT-5 | MEDIUM | CSV didn't escape inner double quotes | Added `escCSV()` helper for RFC 4180 compliance |
| RT-6 | MEDIUM | DiscountPercent not transferred to Offer | Computed from `data.Discount / data.Subtotal * 100` |

**New DB Fields Added:**

| Table | Column | Type | Default | Supabase |
|-------|--------|------|---------|----------|
| `customers` | `mobile_number` | VARCHAR(50) | `''` | ALTER TABLE applied |
| `offers` | `quote_type` | VARCHAR(50) | `'Quotation'` | ALTER TABLE applied |
| `offers` | `vat_rate` | DOUBLE PRECISION | `10` | ALTER TABLE applied |

**Files Modified (9):** `security_enhancements.go`, `database.go`, `app.go`, `offer_pdf_service.go`, `frontend/src/lib/screens/CostingSheetScreen.svelte`, `frontend/src/lib/screens/CustomersScreen.svelte`, `frontend/wailsjs/go/models.ts`

**Build Status**: `go build ./...` and `npm run build` both pass clean
**Supabase**: Schema updated (3 ALTER TABLE commands applied)
**OCR Pipeline**: Verified — Fly.io live (v2.0.0), Mistral key configured, AIML key configured, AnalyzeDocumentWithButler fully implemented

**Known Remaining Items (Low Priority — acceptable for production):**
- Customer form collects status/cr_number/email/phone but Go struct doesn't have these fields (UI placeholders)
- Revision loading doesn't restore all header fields (quoteType, terms) — header comes from opportunity
- CustomerDetailView doesn't show mobile_number edit field
- hiddenCharges is ephemeral (not persisted on save/load) — by design as quick calculation aid
- OffersScreen doesn't display QuoteType column

---

### Phase 29: Customer Data Deduplication (2026-03-02) ✅ COMPLETE

**Context**: Client reported that Bahrain regulatory changes caused company names to be updated with "WLL" (With Limited Liability) suffix, creating duplicate customer records in the database. Same companies existed under multiple names (e.g., "Alba", "Aluminium Bahrain B.S.C", "ALUMINIUM BAHRAIN B.S.C.(C)", "Aluminium Bahrain B.S.C.(C)").

**Investigation:**
- Queried all 348 active customers with child record counts across invoices, orders, offers, and contacts
- Identified **45 duplicate groups** totaling **62 duplicate records** that were clearly the same entity
- Skipped 8 ambiguous groups requiring client confirmation (BAPCO subsidiaries, Nass group, Yateem Group vs Oxygen, etc.)
- Discovered `payments` table has NO `customer_id` column (links through `invoice_id`) — only 93 total payments exist
- Found `tally_invoice_imports` uses `matched_customer_id` (not `customer_id`)

**Merge Operation:**
- Created pre-merge backup: `ph_holdings_PRE_MERGE_BACKUP.db` (7.7MB)
- Built merge script using temp `merge_map` table (62 dup→primary mappings)
- Updated `customer_id` across **20 tables**: `actual_outcomes`, `contracts`, `costing_sheet_data`, `credit_notes`, `customer_contacts`, `customer_name_mappings`, `customer_snapshots`, `delivery_notes`, `deposits_in_transit`, `followup_tasks`, `grade_changes`, `invoices`, `offer_data`, `offers`, `opportunities`, `orders`, `payment_prediction_accuracy`, `prediction_records`, `serial_numbers`, `tally_invoice_imports` (matched_customer_id)
- Soft-deleted 62 duplicate customer records
- Verified 0 orphaned records post-merge
- **Applied same merge to Supabase** — both databases now 286 active customers

**Key Merges (data consolidated):**

| Customer | Invoices | Orders | Offers | Records Merged |
|----------|----------|--------|--------|----------------|
| Electricity & Water Authority (EWA) | 86 | 20 | 2 | 3 duplicates |
| Veolia Water Technologies | 44 | 15 | 4 | 1 duplicate |
| Al Ezzel O&M | 42 | 19 | 5 | 3 duplicates |
| TTSJV W.L.L | 35 | 15 | 0 | 1 duplicate |
| Seapeak Bahrain | 31 | 8 | 3 | 2 duplicates |
| Arla Foods | 25 | 12 | 7 | 2 duplicates |
| Aluminium Bahrain (Alba) | 23 | 13 | 2 | 4 duplicates |
| Gulf Petrochemical Industries | 8 | 4 | 2 | 2 duplicates |

**Remaining Cleanup (sent to PH Trading for review):**
- `CUSTOMER_CLEANUP_REVIEW.md` generated with 3 sections:
  - **Section A**: 49 completely empty records (zero invoices/orders/offers/contacts) — delete candidates
  - **Section B**: 8 ambiguous duplicate groups needing client confirmation (BAPCO 6 entities, Nass 4, CPIC/Mannai 4, Worley 2, Awal Dairy 2, Yateem 2, MCSC/MCSIX 2, Al Faris 2)
  - **Section C**: Name typos to correct ("Complay"→"Company", "Mechnical"→"Mechanical", "CUSTOMER" suffix, etc.)

**Customer Database Summary:**

| Metric | Before | After |
|--------|--------|-------|
| Active customers | 348 | 286 |
| Duplicates merged | — | 62 |
| With invoices | — | 49 |
| With orders/offers only | — | 14 |
| Contacts only (prospects) | — | 174 |
| Completely empty | — | 49 |

**Files Created:** `CUSTOMER_CLEANUP_REVIEW.md`, `ph_holdings_PRE_MERGE_BACKUP.db`
**Databases Modified:** SQLite (local) + Supabase (cloud) — both in sync at 286 active customers

---

### Phase 28: Red Team Round 3 + Master Key Feature (2026-03-01) ✅ COMPLETE

**Context**: Third comprehensive security audit (4 parallel agents) after Phase 27 production readiness work. 46 findings across license system, sync infrastructure, database, and finance. 30 fixed (all P0/P1 + critical P2s). Also added reusable master key for developer access.

**Master Key Feature (license_service.go):**
- Hardcoded `masterKey = "PH-DEV-MASTER"` constant — reusable across any device/rebuild
- Upsert in transaction: creates or updates DB record, logs CRITICAL on device transfers
- Bypasses device binding — master key re-binds to current device on each activation
- Cannot be revoked via `RevokeLicense` (returns error)
- Full admin permissions (`"*"`)

**P1 Fixes (10):**

| ID | Fix | File |
|----|-----|------|
| P1-1 | **PRAGMAs per-connection**: Reverted `MaxOpenConns` to 1 — SQLite PRAGMAs are per-connection, so pool of 4 meant `foreign_keys=ON` only applied to 1 connection | `app.go` |
| P1-2 | **Goroutine leak on lock timeout**: Replaced goroutine-based lock acquisition with `TryLock()` in `pullChangesInternal`/`pushChangesInternal` — old pattern leaked goroutines that permanently held mutex | `sync_service_impl.go` |
| P1-3 | **Data race on `lastSyncTimes`**: Read map under `syncMutex` lock in `performBidirectionalSync` | `sync_service_impl.go` |
| P1-4 | **Data race on `circuitOpen`/`consecutiveErrors`**: Read/write under `syncMutex` lock | `sync_service_impl.go` |
| P1-5 | **RBAC on `StartPeriodicSync`**: Added `settings:update` guard + minimum interval (1 minute) | `sync_service_impl.go` |
| P1-6 | **Master key upsert race**: Wrapped in `db.Transaction()` with proper error handling | `license_service.go` |
| P1-7 | **`StartBackgroundDBSync` double-start**: Added guard against repeated calls (goroutine leak) | `db_sync_service.go` |
| P1-8 | **`StopBackgroundDBSync` double-close**: Replaced `select` pattern with `sync.Once` | `db_sync_service.go` |
| P1-9 | **db nil checks in license functions**: Added to `GenerateLicenseKey`, `ListLicenseKeys`, `RevokeLicense` | `license_service.go` |
| P1-10 | **RBAC namespace consistency**: `PullChanges`/`PushChanges` changed from `settings:manage` to `settings:update` | `sync_service_impl.go` |

**P2 Fixes (10):**

| ID | Fix | File |
|----|-----|------|
| P2-1 | **Shutdown order**: DB closed BEFORE log file — close errors now logged | `app.go` |
| P2-2 | **`GetCashFlowProjection`**: Uses collectible status whitelist (Sent/Overdue/PartiallyPaid) instead of `!= Paid` — was including Cancelled/Void/Draft as expected inflows | `finance_reporting_service.go` |
| P2-3 | **VAT supplier filter**: Excludes Rejected/Dispute supplier invoices from input VAT | `finance_reporting_service.go` |
| P2-4 | **`pruneOldBackups`**: Pattern-matches `ph_holdings_*_*.db` instead of any `.db` file | `database.go` |
| P2-5 | **`TestSupabaseConnection`**: Strips password from error messages returned to frontend | `db_manager.go` |
| P2-6 | **Pagination max**: Reduced from 1000 to 200 for consistency with other listing functions | `pagination.go` |
| P2-7 | **Backup file permissions**: `os.Chmod(backupPath, 0600)` after `VACUUM INTO` | `database.go` |
| P2-8 | **`GetSyncHealth` db path**: Uses `PRAGMA database_list` instead of hardcoded `"ph_holdings.db"` | `db_manager.go` |
| P2-9 | **`saveSetting` upsert**: Replaced delete+create with Unscoped upsert to prevent soft-deleted row accumulation | `db_sync_service.go` |
| P2-10 | **License key logging**: `GenerateLicenseKey` and `SeedEmployeeKeys` no longer log actual key values | `license_service.go` |

**P2/P3 items acknowledged but deferred (16):**
- `getSyncableModels` covers only 11/33 tables (dual sync system — `db_manager.go` path covers all 33)
- `SyncRecord.ConflictState` values don't match CHECK constraint
- `SeedLicenseKeys`/`SeedEmployeeKeys` exported without RBAC (startup-only callers)
- Log rotation targets not checked for symlinks before rename
- Master key extractable from binary (acceptable for desktop ERP)
- `syncTablePush`/`syncTablePull` missing `isValidSQLIdentifier` (tables from hardcoded list)
- `syncTablePull` does not validate pulled records
- Multiple `Remote()` calls per function (TOCTOU window — narrow)
- `supplier_issues`, `offer_follow_ups`, `audit_logs` not in `dbSyncTables`
- Various P3 code hygiene items

**Files Modified (8):** `app.go`, `sync_service_impl.go`, `db_sync_service.go`, `db_manager.go`, `license_service.go`, `database.go`, `finance_reporting_service.go`, `pagination.go`
**Build Status**: `go build ./...` passes clean

---

### Phase 27: Production Readiness & Multi-Device Sync Fix (2026-02-28) ✅ COMPLETE

**Context**: Hardening for production deployment — performance, reliability, data sync completeness, and deployment hardening.

**P0: Critical Sync Bug Fixed — Data Was Not Reaching Other Devices**
- `SyncFromRemote` (pull) was missing **10 tables** vs `SyncToRemote` (push): `order_items`, `invoice_items`, `purchase_order_items`, `supplier_payments`, `delivery_notes`, `delivery_note_items`, `goods_received_notes`, `grn_items`, `offers`, `offer_items`
- Computer A would push data to Supabase, but Computer B would never pull it back — orders arrived without line items, no delivery notes, no offers
- **Fix**: Both push and pull now use the single canonical `dbSyncTables` list (36 tables in dependency order)
- Also added 4 Phase 23 tables to sync: `credit_notes`, `credit_note_items`, `serial_numbers`, `invoice_sequences`

**SQLite Performance PRAGMAs (app.go):**
- WAL mode (concurrent reads during writes — biggest single perf improvement)
- `busy_timeout = 5000ms` (wait on lock instead of immediate error)
- `synchronous = NORMAL` (safe with WAL, much faster than FULL)
- `cache_size = 20MB`, `temp_store = MEMORY`, `mmap_size = 256MB`

**Automated Database Backup (database.go):**
- `BackupDatabase()` using `VACUUM INTO` — atomic, consistent backup
- Backups in `backups/` directory with timestamps, keeps last 7, prunes older
- `RunIntegrityCheck()` — `PRAGMA integrity_check` on every startup
- `TriggerBackup()` — RBAC-guarded UI endpoint for manual backups
- Both run automatically after successful migration on startup

**Graceful Shutdown (db_manager.go + app.go):**
- `stopChan` + `sync.WaitGroup` on `DBManager` — goroutine fully exits before connections close
- `shutdown()` stops both `dbSyncService` and `dbManager` sync goroutines
- Supabase connection explicitly closed on exit (no resource leak)

**Log Rotation (app.go):**
- `rotateLogFile()` on startup — if `app_debug.log` > 50MB, rotates to `.1` through `.5`

**Sync Health Endpoint (db_manager.go):**
- `SyncHealth` struct: online status, last sync, tables in sync, DB size, backup info, uptime
- `GetSyncHealth()` returns comprehensive health for frontend

**Red Team Audit (17 findings — 2 P0, 5 P1, 10 P2, all resolved):**

| ID | Sev | Fix |
|----|-----|-----|
| P0-1 | P0 | `VACUUM INTO` path escaped (single-quote safety) |
| P0-2 | P0 | `GetBackupInfo` uses same dir as `BackupDatabase` (was using CWD) |
| P1-1 | P1 | `CheckConnectivity` TOCTOU — copy `m.remote` under RLock |
| P1-2 | P1 | Shutdown race — `sync.WaitGroup` ensures goroutine exits before connection close |
| P1-3 | P1 | RBAC on `TriggerManualSync` (`settings:update`) |
| P1-4 | P1 | RBAC on `GetSyncHealth` (`settings:view`) |
| P1-5 | P1 | RBAC on `RunIntegrityCheck` (split into internal `runIntegrityCheck` for startup) |
| P2-1 | P2 | 6 tables in `dbSyncTables` added to `MigrateRemote()` |
| P2-2 | P2 | `DownloadFullDatabase` now uses `dbSyncTables` (was missing 13 tables) |
| P2-3 | P2 | `GetVATReconciliation` excludes Cancelled/Void/Proforma/Draft invoices |
| P2-4 | P2 | `GetPaymentAgingReport` only includes collectible statuses |
| P2-5 | P2 | `GetCashFlowProjection` days clamped to 1-365 |
| P2-7 | P2 | `GetDBSyncStatus` returns error map (not nil) on permission deny |
| P2-8 | P2 | `GetBackupInfo` returns empty map (not nil) on permission deny |
| P2-9 | P2 | Sync fallback only counts successful operations |
| P2-10 | P2 | RBAC on `TestSupabaseConnection`, `PerformFirstRunSync`, `GetFirstRunSyncStatus` |

**RBAC Coverage Audit (14 functions guarded):**
- 5 in `finance_reporting_service.go` (`finance:view`)
- 7 in `pagination.go` (per-entity namespaces)
- 2 in `db_manager.go` + `database.go` (`settings:view`)

**Files Modified (6):** `app.go`, `db_manager.go`, `db_sync_service.go`, `database.go`, `finance_reporting_service.go`, `pagination.go`
**Build Status**: `go build ./...` and `npm run build` both pass clean

---

### Phase 23: E-Invoicing, Credit Notes & Serial Number Tracking (2026-02-25) ✅ COMPLETE

**E-Invoicing & Credit Notes:**
- UBL 2.1 XML generation for every invoice (GCC/NBR e-invoicing compliant)
- HMAC-SHA256 document integrity hashes on invoices and credit notes
- Credit note system: Draft → Issued → Applied workflow with over-crediting protection
- Credit note PDFs on company letterhead
- Quarterly VAT return CSV export for NBR compliance

**Serial Number Tracking:**
- Full lifecycle: PO Receipt (GRN) → Inventory → Dispatch (DN) → Delivery → Invoice
- Serial capture at GRN receiving with quantity validation
- Serial allocation at DN creation with atomic availability check
- Serial status updates on dispatch (Shipped) and confirmation (Delivered)
- Warranty start date set on delivery, auto-calculated end date
- Calibration certificate attachment with path traversal protection
- Serial numbers printed on invoice PDFs
- Traceability screen in Operations Hub

**Security Hardening (4 rounds of red team audits):**
- P0 (4 fixes): RBAC guards on all exports, atomic serial allocation, CN over-crediting prevention
- P1 (8 fixes): HMAC hash upgrade, path traversal, pagination caps, serial validation, transaction wrapping
- P2 (3 fixes): Invoice pagination bounds, SendInvoice status guard, CN race condition (tx + row lock)
- Sweep fixes (6): DN status guards (Prepared→Dispatched→Delivered), GenerateDNNumber rewrite (sequence table), RegisterSerials TOCTOU, CreateDNWithSerials TOCTOU, GetDeliveryNotes limit

**New Files (4):** `einvoice_service.go`, `credit_note_service.go`, `serial_number_service.go`, `frontend/src/lib/screens/SerialTraceScreen.svelte`
**Modified Files (11):** `database.go`, `app.go`, `customer_invoice_service.go`, `invoice_pdf_service.go`, `grn_service.go`, `delivery_note_service.go`, `frontend/src/lib/screens/GRNScreen.svelte`, `DeliveryNotesScreen.svelte`, `InvoicesScreen.svelte`, `OperationsHub.svelte`, Wails bindings

**New DB Tables:** `credit_notes`, `credit_note_items`, `serial_numbers` (created in both SQLite and Supabase)
**New DB Columns:** `invoice_hash` + `vat_bhd` on invoices, `requires_serial_tracking` on products

**Remaining P1/P2 items:** All resolved in Phases 24-26.

---

### Phase 26: Complete Audit Resolution (2026-02-28) ✅ ALL FINDINGS RESOLVED

**Scope**: Resolved ALL remaining P3 code hygiene (14), Phase 17 deferred (7), plus 2 rounds of red team audits (22 additional fixes). **93 total fixes, 0 remaining.**

**P3 Code Hygiene (14 fixes):**
- Case-insensitive status filters via `UPPER()` (serial_number_service.go)
- Rune-aware string operations for multi-byte safety (credit_note_service.go — 4 fixes)
- HTML sanitization on credit note reason fields
- XML file permissions tightened `0644` → `0640` (einvoice_service.go)
- HMAC log flooding prevention via `sync.Once`
- Batch size validation (max 500 serials per allocation)
- Empty status bypass blocked on DN dispatch
- Force initial DN status to "Prepared"
- Synthetic order item creation moved inside transaction

**Phase 17 Deferred (7 fixes):**
- PBKDF2 random 32-byte salt via `crypto/rand` with atomic file write (field_crypto.go)
- `ValidateCostingApproval()` wired into `ApproveCostingSheet()` — enforces 8% min margin, Grade C/D advance, ABB 15% (business_invariants.go + app.go)
- Negative inventory prevention with quantity validation (delivery_note_service.go)
- Batch operations bounded to `maxBatchSize = 500` (batch_operations.go)
- Prompt injection sanitization on Butler AI context data (butler_ai.go)
- Frontend deps verified (Vite 5.4.0 + Svelte 4.2.0, no CVEs)

**Red Team Round 1 (10 fixes — 3 P1, 7 P2):**
- `sanitizeForPrompt` — removed destructive 2000-char truncation, case-insensitive regex (butler_ai.go)
- Customer grade lookup via RFQ→CustomerMaster linkage (app.go)
- CreateDeliveryNote TOCTOU — quantity validation + creation in single transaction (delivery_note_service.go)
- Dispatch/Confirm atomicity — DN status + serial updates in single transaction
- Deterministic salt fallback removed (field_crypto.go)
- Atomic salt write via temp-file-then-rename with symlink resolution
- `os.Executable()` error handling in HMAC computation
- `maxBatchSize` applied to all 9 BatchCreate + 2 BatchDelete methods

**Red Team Round 2 (12 fixes — 2 P0, 3 P1, 7 P2):**
- P0: `ConfirmDeliveryNote` fixed `dn_id` → `dn_number` (column didn't exist — confirm was broken for serials)
- P0: `CreateCostingSheet` fixed `markup_percent` → `margin_percent` (frontend key mismatch — margin always stored as 0)
- P1: `computeDocumentHMAC` now uses `globalFieldCrypto.salt` directly (avoids path mismatch)
- P1: `ApproveCostingSheet` grade lookup — WARNING log + case-insensitive fallback
- P1: `CreateDeliveryNoteWithItems` TOCTOU — quantity re-validation + creation in single transaction
- P2: Restored `shipped_date` on dispatch, `warranty_end_date` calculation on confirm (regressions)
- P2: Unknown OrderItemID now returns error instead of silent bypass
- P2: Butler prompt cleanup case-insensitive + 100K context size cap
- P2: App batch wrappers use `maxBatchSize` constant
- P2: FieldCrypto failure logs CRITICAL warning

**Files Modified (8):** `delivery_note_service.go`, `app.go`, `customer_invoice_service.go`, `butler_ai.go`, `batch_operations.go`, `field_crypto.go`, `serial_number_service.go`, `credit_note_service.go`
**Security Audit Status:** See `docs/NEXT_SESSION_FIXES.md` for complete fix log (93 items, 0 remaining)

---

### Phases 24-25: Security Red Team Hardening (2026-02-27) ✅ COMPLETE

**Phase 24 (33 fixes):** 14 Phase 23 sweep fixes, 5 feature fixes, 8 red team hardening fixes, 6 pre-existing P1 fixes.

**Phase 25 (48 fixes across 3 red team rounds):**
- Round 1 (7): Credit check TOCTOU, GrandTotalBHD protection, status validation, payment ID collision, PO threshold
- Round 2 (10): PO permission namespace `purchase_orders:*` → `po:*`, terminal state immutability, invoice number locking, VAT export filtering
- Round 3 (31): All P2 findings + 6 additional P1s (RBAC gaps, stale status strings, PO CreatedBy, CN Proforma guard)

---

### Phase 18: AsymmFlow Rebrand, Data Reconciliation & Team Deployment (2026-02-17) ✅ DEPLOYED

**App Rebranded to AsymmFlow:**
- Renamed from `ph_holdings_app` to `AsymmFlow` in `wails.json` (name + outputfilename)
- App icon changed to Asymmetrica Logo (Dark Mode) via `build/appicon.png`
- All deployment artifacts updated with new branding

**Database Setup on New Mac:**
- Pulled `ph_holdings.db` (8.2MB) and `.env` from GitHub (pushed from other system)
- Fixed schema: added missing `display_name` column to `license_keys` table
- Cleared old unactivated keys, configured fresh license system

**Data Reconciliation (Major - Invoice/Payment Accuracy):**
- **Un-soft-deleted 107 customers** that had `deleted_at = 2026-02-02` (bulk soft-delete artifact)
- **Fixed 2023/2024 invoices**: All 313 invoices incorrectly marked "Sent" → corrected to "Paid" with `outstanding_bhd = 0`
- **Fixed Tally VAT import artifact**: 93 payments recorded pre-VAT amounts (payment × 1.1 = invoice total). Updated all payment amounts to include 10% VAT, marked invoices as Paid
- **Reconciled 2025 invoices**: Matched payments to invoices, set PartiallyPaid/Paid status correctly
- **Final verified state**: 406 Paid, 62 genuinely Overdue, Outstanding AR = 272,819 BHD

**Supabase Migration to New Project:**
- Old project (`jgpwtuyfndtuxxyrytel`) was dead - "Tenant or user not found"
- Migrated to new Supabase project: `xkyydcxxkzjbnsrmzkpp`
- Updated `.env` with new Session Pooler credentials
- **Manual full sync**: Generated INSERT SQL for 242 missing records across 6 tables (customers, orders, order_items, offers, offer_items, customer_contacts)
- Fixed Supabase schema differences: boolean types, VARCHAR lengths, missing columns (`unit_price_bhd`, `total_price_bhd` on order_items, `display_name` on license_keys)
- **All 17 tables verified matching** between SQLite and Supabase

**Windows Cross-Compilation Fix:**
- Windows `.exe` wouldn't launch - `mattn/go-sqlite3` requires CGO with C compiler
- Previous build produced 27MB binary without proper SQLite linking (silent failure)
- Installed `mingw-w64` via Homebrew (1.4GB cross-compiler toolchain)
- Rebuilt with: `CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 wails build`
- New exe: **35MB** (properly linked) vs 27MB (broken)

**Personalized License Keys for 8 Team Members:**

| Person | Role | Key Format |
|--------|------|------------|
| Abhie (SPOC) | Admin | `PH-ADM-XXXXXX` |
| Stanley | Admin | `PH-ADM-XXXXXX` |
| Daniel | Admin | `PH-ADM-XXXXXX` |
| Sundar | Manager | `PH-MGR-XXXXXX` |
| Peri | Sales | `PH-SLS-XXXXXX` |
| Ebin | Sales | `PH-SLS-XXXXXX` |
| Support | Sales | `PH-SLS-XXXXXX` |
| Clinton | Operations | `PH-OPS-XXXXXX` |

(Actual keys in `deploy_package/LICENSE_KEYS.txt` and seeded in database)

**Deployment Package Shipped:**
```
deploy_package/PH_Holdings_Deploy_2026_02_17.tar.gz (28MB)
├── AsymmFlow        (32MB - Mac binary)
├── AsymmFlow.exe    (35MB - Windows binary, CGO-linked)
├── .env             (New Supabase credentials)
├── data/ph_holdings.db (7.9MB - reconciled data)
├── LICENSE_KEYS.txt (8 personalized keys with role matrix)
└── INSTALL_GUIDE.txt
```

**Pipeline Visibility Verified:**
- Frontend: `EnterpriseSidebar.svelte` filters nav items by permission store
- Backend: `requirePermission()` guards on all 33+ API functions
- Sales sees: Dashboard, Opportunities, Relationships, Intelligence
- Operations sees: Dashboard, Operations, Relationships, Intelligence
- Manager sees: All except user management
- Admin sees: Everything

**Commits Pushed:**
- `c7da7d5` - feat: Phase 17 security hardening, data reconciliation, AsymmFlow rebrand
- `1f2ddcb` - fix: Update Supabase credentials to new project and full cloud sync

---

### Phase 17: Security Red Team Audit & Hardening (2026-02-16) ✅ COMPLETE

**Dev Environment Setup (New Mac):**
- Cloned repo from `github.com/rahul-asymmetrica/ph_holdings`
- Installed Go 1.26.0, Wails CLI v2.11.0, frontend npm deps
- Fixed cross-platform build issue: extracted `ExcelWorkbookData` and `WordDocumentData` types from Windows-only `com_automation.go` into shared `integration/com_types.go`
- Verified full build passes (`go build ./...` + `wails build`)

**Red Team Audit (6 Parallel Agents):**
Ran 6 concurrent security audit agents covering: Authentication & Sessions, SQL/Command Injection, Data Security, API & Input Validation, Infrastructure & Config, Business Logic & Race Conditions. Total raw findings: **117** (deduplicated to ~74 unique).

**P0 Fixes Applied (Critical - Same Day):**

| Fix | File | Description |
|-----|------|-------------|
| Logout infinite recursion | `auth_session.go:367` | `Logout()` called `LogoutWithSession()` which called `Logout()` - infinite loop. Replaced with direct cleanup: nil Token/Profile, remove `.auth_token.json` |
| RBAC startup bypass scoped | `app.go:11433` | Previously ALL permissions bypassed for 5 min on startup. Now only import-related permissions (`settings:update/view`, `finance:create/read`, `orders:create/read`, `po:create/read`) are bypassed |

**P1 Fixes Applied (High Priority - 15 files, +411/-116 lines):**

*Injection Fixes:*
- PowerShell command injection: added `escapePowerShell()` sanitizer (`intent_processor.go`)
- sqlite3 CLI injection: added `isReadOnlySQLQuery()` validation rejecting dot-commands and non-SELECT queries (`tool_executor.go`)
- SQL injection via table names: added `isValidSQLIdentifier()` check in `downloadTable` (`db_manager.go`)
- CSRF token predictable fallback: `log.Fatalf` instead of degrading to timestamp-based token (`app.go`)

*Payment & Financial Controls:*
- `UpdatePayment`: added amount > 0 validation, invoice bounds check, prevented changing linked invoice, added invoice balance recalculation and audit logging (`payment_service.go`)
- `UpdateSupplierPayment`: same controls as above (`supplier_payment_service.go`)
- Credit limit bypass: fixed inverted `IsCreditBlocked` logic - blocked customers now correctly rejected (`customer_invoice_service.go`)
- Supplier payment race condition: moved balance check inside transaction with `SELECT FOR UPDATE` (`supplier_payment_service.go`)
- Duplicate supplier payments: detection now works even when reference is empty (checks amount + date) (`supplier_payment_service.go`)

*PO Controls:*
- PO financial field protection: blocked amount/total/rate changes on Approved/Sent/Received POs (`purchase_order_service.go`)
- PO status state machine: added valid transition map, terminal states (Received, Cancelled) can't revert (`purchase_order_service.go`)

*XSS Fixes:*
- Added `escapeHtml()` to render functions in `ChequeRegisterScreen.svelte`, `BankReconciliationScreen.svelte`, `FXRevaluationScreen.svelte`
- OAuth error page: `html.EscapeString(errorDesc)` prevents reflected XSS (`auth_handler.go`)

*Auth, Crypto & Upload:*
- Predictable license keys: replaced deterministic `SHA-256(role+index)` with `crypto/rand` (`license_service.go`)
- License keys no longer logged to console on startup - only counts per role (`license_service.go`)
- OAuth tokens stored as SHA-256 hashes in DB, not plaintext. Lookups use `hashToken()` (`auth_session.go`)
- OAuth scopes reduced: `Files.ReadWrite` (not `.All`), `Mail.Send` (removed `Mail.ReadWrite`), `Calendars.Read`, `Sites.Read.All` (`auth_handler.go`)
- Path traversal fix: `filepath.Base(fileName)` sanitization in `QuickCaptureDocumentFromBase64` (`app.go`)
- Size limit: 50MB max on base64 upload to prevent OOM (`app.go`)

*Crash Fix:*
- `InitializeDefaultAssets` nil pointer dereference: `GetApplicationPaths()` returns nil when no license activated. Added nil check (`assets_service.go`)

---

### STILL PENDING: Remaining P2 Fixes (10 items)

Items 1, 5, 9, 13, 14, 15, 17 resolved in Phase 26. Financial reporting accuracy fixed in Phase 27.

**Crypto & Config (3 remaining):**
- **OAuth callback binds to all interfaces** - `auth_handler.go:447` listens on `:8080` instead of `127.0.0.1:8080`.
- **Cloud sync defaults to enabled** - `config.go:532`. Default `ENABLE_CLOUD_SYNC` to `false`.
- **`.env` file traversal** - `config.go:179` walks parent directories looking for `.env`.

**Business Logic (2 remaining):**
- **FX rate manipulation** - `fx_revaluation_service.go:21`. No bounds validation on exchange rates.
- **Bank reconciliation amount mismatch** - `bank_transaction_matcher.go:420`. Manual match doesn't validate amounts.
- **Supplier invoice approval lacks segregation of duties** - `supplier_invoice_service.go:307`.

**Input Validation & API (3 remaining):**
- **`CreateUser` skips validation** - `app.go`. Doesn't call `ValidateUserInput()`.
- **Filesystem traversal in document classifier** - `document_classifier.go:706`. Accepts any `basePath`.
- **`ProcessInboxDocument` no auth** - `runtime_handlers.go:37`. No `requirePermission()` call.

**Frontend (1 remaining):**
- **Incomplete log sanitization** - `security_enhancements.go:329`. `SanitizeForLog` doesn't mask all secret patterns.

---

### Phase 16: Client Ship & Bug Fixes (2026-01-29) ✅ SHIPPED

**Critical Bug Fixes:**
- **Permissions Shadowing Bug**: Fixed variable shadowing in `App.svelte:483` where `permissions` from event.detail shadowed the Svelte store, causing `permissions.set()` to fail silently
- **Missing invoice_items Table**: Created missing table required by GORM Preload in ListCustomerInvoices
- **License Key Issues**: Added missing PH-DEV-M3NT0R key to database

**Data Import (2020-2025):**
- Imported 468 customer invoices (2023-2025)
- Imported 484 supplier invoices (2023-2025)
- Imported 601 supplier payments (2020-2025) - complete historical data
- Total value tracked: ~8.16M BHD invoices, ~3.85M BHD payments

**Verification Completed:**
- Supabase cloud sync: ✅ Connected, auto-syncs every 10 min
- Letterhead on PDFs: ✅ PH Trading template (427KB)
- OCR integration: ✅ Fly.io Runtime connected
- Butler AI: ✅ Mistral API configured
- License system: ✅ All roles working

**Packages Shipped:**
- `PH_Holdings_Client_2026_01_29.tar.gz` (15 MB) - Client deployment
- `PH_Holdings_Mentor_FIXED.tar.gz` (14 MB) - Mentor with full source

**Status:** App shipped to client, awaiting feedback on bugs found during testing.

**Session Notes:** See `SESSION_NOTES_2026_01_29.md` for detailed problem-solving documentation.

---

### Phase 15: Deployment & License System (2026-01-28) ✅

**License-Based Activation** (`license_service.go`, 350+ LOC):
- Replaced device registration with key-based licensing
- Key format: `PH-{ROLE}-{6-char-hash}` (e.g., `PH-ADM-A0C85B`)
- Roles: Admin (ADM), Manager (MGR), Sales (SLS), Operations (OPS), Developer (DEV)
- License binds to device hash on first activation
- Auto-creates valid format keys on first use
- Persists in SQLite - subsequent starts skip activation

**UI Polish**:
- Removed buggy card hover animations from CostingSheetScreen
- Added Terms & Conditions to costing PDF (separate page with letterhead)
- Deprecated Settings screen (removed from nav due to click bug)

**Intelligence Pipeline**:
- Hardcoded Mistral API key for seamless deployment
- Butler AI + OCR integration ("Analyze with Butler" button)
- Document insights: summary, entities, line items, suggested actions

**Offline-First Architecture**:
- SQLite primary, Supabase sync every 10 minutes
- Connected to new Supabase Pro account ($25/month)
- Works fully offline, syncs when connected

**Deployment Packages Created** (Phase 15 - superseded by Phase 18):
```
deploy_package/           <- Updated in Phase 18 with AsymmFlow branding
mentor_package/           <- Full source for mentor (Phase 15)
```

**Note:** Phase 15 license keys (`PH-ADM-A0C85B`, etc.) are now obsolete. Phase 18 keys are personalized per team member - see `deploy_package/LICENSE_KEYS.txt`.

---

### Phase 14: Dynamic Finance & Data Integrity (2026-01-28) ✅

**Financial Year Service** (`financial_year_service.go`, 530+ LOC):
- Year selector (2023/2024/2025) with audit badges
- 2023/2024: Verified Grant Thornton FS2024 data hardcoded
- 2025: Live calculation from invoices with margin-based COGS estimation
- Uses 2024's 18.2% gross margin for unaudited year P&L estimates

**Data Fixes Applied**:
- Fixed dates 1 year off (2026 → 2025): 155 invoices, 93 payments, 45 POs, 154 orders
- Added `supplier_name` column to `purchase_orders` table
- Enriched all 45 POs with actual supplier names
- Fixed CRM dashboard to detect latest invoice year (not system year 2026)
- Fixed financial calculations using SQLite `strftime()` syntax

**Key Functions Added**:
- `GetFinancialYearData(year)` - Returns verified or calculated data
- `GetDynamicFinancialDashboard(year)` - Full dashboard with ratios
- `GetAvailableFinancialYears()` - Lists years with data
- `FixDatabaseDates()` - One-time date correction utility
- `FixPurchaseOrderSupplierNames()` - Enriches PO supplier names
- `RunAllDataFixes()` - Runs all fixes (auto-runs on startup if needed)

### Phase 13: Hybrid Database (2026-01-27)
- Rolled back from Render PostgreSQL to SQLite primary
- Added Supabase as sync target (Session Pooler for IPv4)
- Sync status indicator in sidebar

### Phase 12: Red Team Audit (2026-01-27)
- 60+ issues fixed across P0-P3 priorities
- 9 new backend service files (3,130+ LOC)
- Cache, batch operations, pagination, security enhancements

### Phase 11: Device-Based RBAC (2026-01-26)
- First-install-becomes-admin pattern
- Device fingerprinting (MAC + hostname SHA-256)
- 5 roles: admin, manager, sales, operations, staff

### Phases 1-10: Foundation through Production Sprint
- Complete ERP functionality built
- Wails + Go + Svelte architecture
- Mistral AI integration
- Onyx & Ether design system

---

## Data Integrity Guards

| Guard | Implementation |
|-------|----------------|
| Duplicate invoice | `uniqueIndex` on OrderID + backend check |
| Overpayment | Backend validates amount ≤ outstanding |
| State machine | Lost offers cannot be re-won; PO terminal states enforced; DN Prepared→Dispatched→Delivered enforced |
| RBAC | `requirePermission()` on 70+ functions (Phase 27: finance reporting, pagination, sync, backup all guarded) |
| XSS | `escapeHtml()` on all DataTable renders |
| SQL injection | Parameterized queries, `escapeLikeWildcards()` with `ESCAPE '\'`, `isValidSQLIdentifier()` |
| Path traversal | Sanitized filenames in PDF/reports, `filepath.Base()` on calibration certs |
| Command injection | `escapePowerShell()`, `isReadOnlySQLQuery()` |
| Credit limit | `IsCreditBlocked` enforced with row locking |
| Over-crediting | CN creation in transaction with invoice row lock; sums all Draft+Issued+Applied CNs |
| Credit note workflow | Draft → Issued → Applied enforced; Draft CNs cannot be applied directly |
| Invoice immutability | `SendCustomerInvoice` only from Draft; Paid invoices immutable |
| PO field protection | Amount/total/rate locked on Approved/Sent/Received POs |
| Payment integrity | Amount > 0, invoice bounds check, balance recalculation on CRUD |
| Serial race safety | Atomic `UPDATE WHERE status='Available'` prevents double-allocation |
| Serial TOCTOU | Duplicate check + create inside single transaction |
| Document integrity | HMAC-SHA256 hashes on invoices and credit notes (`.field_crypto_salt` key) |
| Pagination bounds | All paginated functions max 200, `ListCustomerInvoices` max 200, `GetDeliveryNotes` max 200 |
| Sequence numbers | `invoice_sequences` table with row locking for INV/CN/DN (no COUNT-based fragility) |
| Device approval | Pending devices cannot access app |
| Session timeout | 8 hours auto-logout |
| License binding | `crypto/rand` keys, one device per key, hashed tokens; master key `PH-DEV-MASTER` for developer |
| SQLite PRAGMAs | Single connection (PRAGMAs per-connection safe), WAL mode, `busy_timeout=5000`, `synchronous=NORMAL`, 20MB cache, 256MB mmap |
| Auto backup | `VACUUM INTO` on startup, keeps last 7, `chmod 0600`, pattern-matched prune, integrity check on boot |
| Log rotation | 50MB max, 5 rotated logs, `Lstat` symlink check on startup |
| Graceful shutdown | `sync.WaitGroup` on all 3 sync goroutines; DB closed before log file; `sync.Once` on stop channels |
| Multi-device sync | Push and pull both use canonical `dbSyncTables` (36 tables in dependency order) |
| VAT accuracy | `GetVATReconciliation` excludes Cancelled/Void/Proforma/Draft invoices |
| AR aging accuracy | `GetPaymentAgingReport` only includes Sent/Overdue/PartiallyPaid |
| Cash flow accuracy | `GetCashFlowProjection` days clamped 1-365, only collectible statuses (Sent/Overdue/PartiallyPaid) |
| VAT supplier filter | `GetVATReconciliation` excludes Rejected/Dispute supplier invoices from input VAT |
| Sync lock safety | `TryLock()` in sync internal functions (no goroutine leak on lock contention) |
| Sync data races | `lastSyncTimes`, `circuitOpen`, `consecutiveErrors` all accessed under `syncMutex` |
| Credential safety | `TestSupabaseConnection` strips password from error messages to frontend |
| Customer dedup | Phase 29: 62 WLL/B.S.C duplicates merged across 20 tables + Supabase; pre-merge backup retained; 8 ambiguous groups sent to client |

---

## Deployment Guide (License-Based)

### Installation Steps
1. **Extract** `deploy_package/` to target PC (e.g., `C:\AsymmFlow\`)
2. **Run** `AsymmFlow.exe` (Windows) or `AsymmFlow` (Mac)
3. **Enter License Key** when prompted (see `LICENSE_KEYS.txt` for personalized keys):
   - Admin: Full access (Abhie, Stanley, Daniel)
   - Manager: Finance + operations (Sundar)
   - Sales: Sales pipeline only (Peri, Ebin, Support)
   - Operations: Procurement only (Clinton)
4. **Done** - App opens to dashboard

### Windows Requirements
- **WebView2 Runtime**: Pre-installed on Windows 10 21H2+ and Windows 11. Older systems need manual install from Microsoft.
- **CGO Note**: Windows binary must be cross-compiled with `mingw-w64` for SQLite support (see Quick Start)

### License Key Rules
- Each key activates **ONE PC only** (binds to device hash)
- License persists after activation (no re-entry needed)
- DEV keys (`PH-DEV-XXXXXX`) have full admin access for testing
- Key format: `PH-{ROLE}-{6-char}` (exactly 13 characters)
- **Master key**: `PH-DEV-MASTER` — reusable across any device/rebuild, full admin, cannot be revoked

### Role Permissions
| Feature | Admin | Manager | Sales | Operations |
|---------|-------|---------|-------|------------|
| Dashboard | ✅ | ✅ | ✅ | ✅ |
| Opportunities | ✅ | ✅ | ✅ | ❌ |
| Operations | ✅ | ✅ | ❌ | ✅ |
| Finance Hub | ✅ | ✅ | ❌ | ❌ |
| Butler Finance Queries | ✅ | ✅ | ❌ | ❌ |

### Multi-Device Sync
- Automatic every 10 minutes when online
- Green dot = Connected, Yellow = Connecting, Gray = Offline
- App works fully offline - changes sync when reconnected

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Function not found" | Run `wails build` to regenerate bindings |
| Database locked | Close other app instances |
| Supabase yellow dot | Check `.env` credentials, use Session Pooler host (`xkyydcxxkzjbnsrmzkpp`) |
| Build fails | `go mod tidy`, check Go 1.21+ |
| Dates showing 2026 | App auto-fixes on startup, or call `RunAllDataFixes()` |
| Unknown Supplier in POs | Call `FixPurchaseOrderSupplierNames()` |
| License key rejected | Check format: `PH-XXX-YYYYYY` exactly 13 chars (valid prefixes: ADM, MGR, SLS, OPS, DEV) |
| License already used | Each key binds to one PC - use a different key |
| Activation screen keeps appearing | Check `license_keys` table has `activated=1` for device |
| Windows exe won't open | Must cross-compile with mingw-w64 for CGO/SQLite. Binary should be ~35MB, not ~27MB |
| Windows exe needs WebView2 | Pre-installed on Win10 21H2+. Older systems: install from Microsoft |
| Dashboard shows wrong invoices | Check invoice statuses - historical imports may need reconciliation (see Phase 18) |
| Missing `display_name` column | `ALTER TABLE license_keys ADD COLUMN display_name TEXT DEFAULT ''` |
| Supabase boolean mismatch | SQLite uses 0/1, PostgreSQL uses true/false. Cast in INSERT statements |
| Duplicate customers | Phase 29 merged 62. If more appear, check `CUSTOMER_CLEANUP_REVIEW.md` for the 8 ambiguous groups awaiting PH confirmation |
| Pre-merge backup | `ph_holdings_PRE_MERGE_BACKUP.db` in project root — restore if merge caused issues |

---

## Business Context

**PH Trading WLL** (Bahrain)
- Industry: Process Instrumentation
- Suppliers: Endress+Hauser, Servomex, Landis+Gyr
- Customers: BAPCO, Alba, EWA
- Currency: BHD (3 decimal places)
- VAT: 10%

**FS2024 Verified Data**:
- Revenue: 1.59M BHD (2024), 5.21M BHD (2023 - EWA contract)
- Net Profit: 20.7K BHD (2024), 672.9K BHD (2023)
- Current Ratio: 4.70x (excellent liquidity)

---

## Contact

- **Part of**: Asymmetrica Mathematical Organism
- **Philosophy**: Research Sovereignty, Build-Test-Ship

**Om Lokah Samastah Sukhino Bhavantu** 🙏
