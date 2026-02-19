# CLAUDE.md - AsymmFlow (formerly PH Holdings Sovereign UI)

**Project**: AsymmFlow ERP System (Wails + Go + Svelte)
**Status**: DEPLOYED TO TEAM (Phase 18)
**Last Updated**: 2026-02-17

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

### Key Files
| Path | Purpose |
|------|---------|
| `app.go` | Main backend (11,000+ LOC) |
| `wails.json` | App config - name: "AsymmFlow", outputfilename: "AsymmFlow" |
| `license_service.go` | License key generation & validation (350+ LOC) |
| `payment_service.go` | Customer payment CRUD with invoice reconciliation |
| `supplier_payment_service.go` | Supplier payment CRUD with race condition protection |
| `financial_year_service.go` | Dynamic finance with verified FS2024 data |
| `integration/com_types.go` | Shared types extracted from Windows-only `com_automation.go` |
| `frontend/src/lib/screens/` | All UI screens |
| `frontend/src/lib/components/ui/EnterpriseSidebar.svelte` | Nav with RBAC permission filtering |
| `build/appicon.png` | Asymmetrica Logo (Dark Mode) - embedded in app |
| `build/bin/` | Compiled executables |
| `deploy_package/` | Client deployment package (AsymmFlow + keys + DB) |
| `.env` | Supabase credentials (project: xkyydcxxkzjbnsrmzkpp) |

---

## Current Features (Phase 14)

### Finance Hub
- **Dynamic Year Selector**: View FY2023, FY2024, FY2025
- **Audited Data**: 2023/2024 from Grant Thornton FS2024
- **Live Calculation**: 2025 from Tally imports with estimated ratios
- **Audit Badges**: Green (audited) / Orange (unaudited)

### CRM Hub
- Customer/Supplier dashboards with KPIs
- 360° profiles with orders, invoices, notes
- Automatic data year detection (uses latest invoice year)

### Operations Hub
- RFQ → Costing → Offer → Order pipeline
- Purchase Orders with supplier name enrichment
- GRN with partial receiving support

### Intelligence Hub
- Butler AI (Mistral) for natural language queries
- PDF report generation on company letterhead

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

- **SQLite**: Source of truth, works offline
- **Supabase**: Background sync every 10 minutes (Pro plan: $25/month)
- **Sync Indicator**: Green/Yellow/Gray in sidebar

### Key Tables
- `customers`, `suppliers`, `products` (Master Data)
- `orders`, `invoices`, `payments` (Transactions)
- `purchase_orders`, `supplier_invoices`, `goods_received_notes` (Operations)
- `devices`, `device_users`, `roles` (RBAC)

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

### STILL PENDING: P2 Fixes for Production Hardening

The following P2 (Medium priority) fixes were identified in Phase 17 but deferred in favor of deployment. Should be completed when time permits:

**Crypto & Config (4 fixes):**
1. **Deterministic PBKDF2 salt** - Salt derived from the secret itself in `field_crypto.go:70`. Generate a random salt on first run and store it separately.
2. **OAuth callback binds to all interfaces** - `auth_handler.go:447` listens on `:8080` instead of `127.0.0.1:8080`. Remote machines on the same network can reach it.
3. **Cloud sync defaults to enabled** - `config.go:532`. Default `ENABLE_CLOUD_SYNC` to `false` to prevent accidental production data corruption.
4. **`.env` file traversal** - `config.go:179` walks parent directories looking for `.env`. Restrict to app directory only.

**Business Logic (5 fixes):**
5. **Business invariants not enforced** - `business_invariants.go` defines rules (Grade D = 100% advance, 8% min margin) but they're never called in create/update paths. Wire `ValidateAll()` into mutation flows.
6. **FX rate manipulation** - `fx_revaluation_service.go:21`. No sanity check on exchange rates. Add bounds validation (+/- 20% of last rate).
7. **Bank reconciliation amount mismatch** - `bank_transaction_matcher.go:420`. Manual match doesn't validate amounts. A 100 BHD credit can be matched to a 50,000 BHD invoice.
8. **Supplier invoice approval lacks segregation of duties** - `supplier_invoice_service.go:307`. Uses `po:create` permission (not a dedicated approval perm), and creator can approve their own invoice.
9. **Negative inventory not prevented** - Delivery notes can be created with quantities exceeding available stock.

**Input Validation & API (5 fixes):**
10. **`CreateUser` skips validation** - `app.go:11248`. Doesn't call `ValidateUserInput()`. Accepts XSS in `fullName`, no length limits on `department`/`jobTitle`.
11. **Filesystem traversal in document classifier** - `document_classifier.go:706`. Accepts any `basePath`, can enumerate entire filesystem.
12. **`ProcessInboxDocument` no auth** - `runtime_handlers.go:37`. No `requirePermission()` call, no path validation.
13. **Batch operations unbounded** - `batch_operations.go:116`. No max size limit on batch update/delete. Can mass-delete all orders in one call.
14. **Butler AI prompt injection** - `butler_ai.go:232`. Raw customer names/data injected into Mistral prompts without sanitization.

**Frontend & Dependencies (3 fixes):**
15. **Outdated frontend deps** - Vite 3.x and Svelte 3.x have known CVEs (CVE-2023-34092, CVE-2024-23331). Upgrade to Vite 6.x and Svelte 4/5.x.
16. **Incomplete log sanitization** - `security_enhancements.go:329`. `SanitizeForLog` doesn't mask `api_key`, `token`, `secret` fields. Expand patterns.
17. **Log file world-readable** - `app.go:223`. Created with permission `0666`. Change to `0600`.

**Estimated P2 effort:** ~2-3 hours with parallel agents. After P2, the app will be production-ready from a security standpoint.

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
| State machine | Lost offers cannot be re-won; PO terminal states enforced |
| RBAC | `requirePermission()` on 33+ functions |
| XSS | `escapeHtml()` on all DataTable renders |
| SQL injection | Parameterized queries, wildcard escaping, `isValidSQLIdentifier()` |
| Path traversal | Sanitized filenames in PDF/reports, `filepath.Base()` |
| Command injection | `escapePowerShell()`, `isReadOnlySQLQuery()` |
| Credit limit | `IsCreditBlocked` enforced with row locking |
| PO field protection | Amount/total/rate locked on Approved/Sent/Received POs |
| Payment integrity | Amount > 0, invoice bounds check, balance recalculation on CRUD |
| Device approval | Pending devices cannot access app |
| Session timeout | 8 hours auto-logout |
| License binding | `crypto/rand` keys, one device per key, hashed tokens |

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
