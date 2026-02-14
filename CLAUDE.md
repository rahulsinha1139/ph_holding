# CLAUDE.md - PH Holdings Sovereign UI

**Project**: PH Holdings ERP System (Wails + Go + Svelte)
**Status**: SHIPPED TO CLIENT (Phase 16 Complete)
**Last Updated**: 2026-01-29

---

## Quick Start

```bash
# Development
wails dev

# Production build
wails build

# Output: build/bin/ph_holdings_app.exe
```

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    PH Holdings ERP                          │
├─────────────────────────────────────────────────────────────┤
│  Frontend: Svelte + TypeScript (Onyx & Ether Design)        │
│  Backend:  Go + Wails v2.11.0                               │
│  Database: SQLite (Primary) + Supabase (Cloud Sync)         │
│  AI:       Mistral API (Butler Intelligence)                │
└─────────────────────────────────────────────────────────────┘
```

### Key Files
| Path | Purpose |
|------|---------|
| `app.go` | Main backend (11,000+ LOC) |
| `license_service.go` | License key generation & validation (350+ LOC) |
| `financial_year_service.go` | Dynamic finance with verified FS2024 data |
| `frontend/src/lib/screens/` | All UI screens |
| `frontend/src/lib/screens/LicenseActivationScreen.svelte` | License entry UI |
| `build/bin/` | Compiled executables |
| `deploy_package/` | Client deployment package |
| `mentor_package/` | Full source + exe for mentor |

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
# .env file
SUPABASE_DB_HOST=aws-1-ap-southeast-1.pooler.supabase.com
SUPABASE_DB_PORT=5432
SUPABASE_DB_USER=postgres.jgpwtuyfndtuxxyrytel
SUPABASE_DB_PASSWORD=PHHoldings2026
ENABLE_CLOUD_SYNC=true
MISTRAL_API_KEY=your_key
```

---

## Development History

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

**Deployment Packages Created**:
```
deploy_package/           <- For client (5-7 users)
├── ph_holdings_app.exe   (34 MB)
├── .env                  (Supabase credentials)
├── LICENSE_KEYS.txt      (10 keys: 1 Admin, 2 Manager, 5 Sales, 2 Ops)
├── INSTALL_GUIDE.txt
├── data/ph_holdings.db
└── CRITICAL_PH_CONTEXT_SSOT/

mentor_package/           <- Full source for mentor
├── ph_holdings_app.exe
├── *.go (all source)
├── frontend/ (Svelte)
├── LICENSE_KEYS.txt
└── ph_holdings.db
```

**Generated License Keys**:
| Role | Keys |
|------|------|
| Admin | `PH-ADM-A0C85B` |
| Manager | `PH-MGR-8F4B94`, `PH-MGR-8613CB` |
| Sales | `PH-SLS-AA8C4A`, `PH-SLS-BD453E`, `PH-SLS-E44617`, `PH-SLS-EFFCC6`, `PH-SLS-F7E6F0` |
| Operations | `PH-OPS-3ADB85`, `PH-OPS-32605D` |
| Developer | `PH-DEV-M3NT0R` (mentor test key) |

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
| State machine | Lost offers cannot be re-won |
| RBAC | `requirePermission()` on 33+ functions |
| XSS | `escapeHtml()` on all DataTable renders |
| SQL injection | Parameterized queries, wildcard escaping |
| Path traversal | Sanitized filenames in PDF/reports |
| Device approval | Pending devices cannot access app |
| Session timeout | 8 hours auto-logout |

---

## Deployment Guide (License-Based)

### Installation Steps
1. **Extract** `deploy_package/` to target PC (e.g., `C:\PH_Holdings\`)
2. **Run** `ph_holdings_app.exe`
3. **Enter License Key** when prompted:
   - Admin: `PH-ADM-A0C85B` (full access)
   - Manager: `PH-MGR-XXXXXX` (finance + operations)
   - Sales: `PH-SLS-XXXXXX` (sales pipeline only)
   - Operations: `PH-OPS-XXXXXX` (procurement only)
4. **Done** - App opens to dashboard

### License Key Rules
- Each key activates **ONE PC only**
- License persists after activation (no re-entry needed)
- DEV keys (`PH-DEV-XXXXXX`) have full admin access for testing

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
| Supabase yellow dot | Check `.env` credentials, use Session Pooler host |
| Build fails | `go mod tidy`, check Go 1.21+ |
| Dates showing 2026 | App auto-fixes on startup, or call `RunAllDataFixes()` |
| Unknown Supplier in POs | Call `FixPurchaseOrderSupplierNames()` |
| License key rejected | Check format: `PH-XXX-YYYYYY` (valid prefixes: ADM, MGR, SLS, OPS, DEV) |
| License already used | Each key binds to one PC - use a different key |
| Activation screen keeps appearing | Check `license_keys` table has `activated=1` for device |

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
