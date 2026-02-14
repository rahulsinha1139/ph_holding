# Orchestrator Quick Reference

**File**: `main.go` | **Status**: ✅ READY | **Size**: 18 MB | **Runtime**: ~3-5s

---

## Usage

```bash
cd cmd/rebuild_2025
./rebuild_2025.exe
```

**Requirements**:
- `../../.env` with `DATABASE_URL=postgres://...`
- `Data_for_database/Tally_data/invoices 2025.xlsx`
- `Data_for_database/Tally_data/purchase 2025.xlsx`

---

## What It Does

| Step | Function | Status |
|------|----------|--------|
| 1 | Load .env | ✅ Complete |
| 2 | Connect PostgreSQL | ✅ Complete |
| 3 | Drop tables (FK-safe) | ✅ Complete |
| 4 | AutoMigrate 70+ models | ✅ Complete |
| 5 | Seed lookups (roles/warehouse/accounts) | ✅ Complete |
| 6 | Import customers/suppliers | ✅ Complete |
| 7 | Import 2025 invoices | ⚠️ Placeholder |
| 8 | Import 2025 purchases | ⚠️ Placeholder |
| 9 | Import 2025 opportunities | ⚠️ Placeholder |
| 10 | Run validation | ⚠️ Placeholder |
| 11 | Print summary | ✅ Complete |

---

## Files

| File | Purpose |
|------|---------|
| `main.go` | Main orchestrator (270 LOC) |
| `models.go` | All 70+ GORM models (1,143 LOC) |
| `schema.go` | Table dropping + seeding (499 LOC) |
| `master_data.go` | Customer/supplier import (563 LOC) |
| `invoices.go` | ⚠️ NOT CREATED |
| `purchases.go` | ⚠️ NOT CREATED |
| `pipeline.go` | ⚠️ NOT CREATED |
| `validation.go` | ⚠️ NOT CREATED |

**Total**: 2,475 LOC (implemented files)

---

## Expected Output (Current State)

```
✅ Imported 87 customers, 18 suppliers

📋 MASTER DATA:
   Customers:                87
   Suppliers:                18
   Roles:                    5
   Warehouses:               1
   Chart of Accounts:        18

📦 TRANSACTIONS: All 0 (no invoices/purchases imported yet)
💰 FINANCIAL METRICS: All 0.000 (no data yet)
```

---

## Tables Preserved

- `devices` (device registration state)
- `device_users` (user accounts + bcrypt hashes)
- `roles` (re-seeded to ensure 5 roles exist)

---

## Lookup Data Seeded

**5 Roles**: admin, manager, sales, operations, staff
**1 Warehouse**: WH-SITRA-01 (Main Warehouse - Sitra)
**18 Accounts**: Asset, Liability, Equity, Revenue, COGS, OpEx hierarchies

---

## Error Handling

**Fatal Errors** (abort execution):
- DATABASE_URL not set
- Database connection failed
- AutoMigrate failed
- Lookup seeding failed
- Master data import failed

**Non-Fatal Errors** (log warning, continue):
- .env file not found (use system env vars)
- Excel file not found (log, skip)
- Individual record creation failed (log, skip record)

---

## Next Steps to Complete

1. **Create `invoices.go`**: Import 2025 invoices from Tally Excel
2. **Create `purchases.go`**: Import 2025 purchases from Tally Excel
3. **Create `pipeline.go`**: Import archaeology data (RFQ opportunities)
4. **Create `validation.go`**: Data integrity checks
5. **Re-run orchestrator**: All 11 steps will execute

---

## Key Functions for Next Agents

```go
// Expected in invoices.go
func import2025Invoices(db *gorm.DB, customerMap map[string]string) error

// Expected in purchases.go
func import2025Purchases(db *gorm.DB, supplierMap map[string]string) error

// Expected in pipeline.go
func import2025Opportunities(db *gorm.DB, customerMap map[string]string) error

// Expected in validation.go
func runValidation(db *gorm.DB) error
```

---

## Troubleshooting

| Error | Fix |
|-------|-----|
| "DATABASE_URL not set" | Create `../../.env` with valid DATABASE_URL |
| "Failed to connect" | Check Render.com dashboard, verify credentials |
| "AutoMigrate failed" | Check models.go syntax, ensure PostgreSQL 14+ |
| "Could not open invoices 2025.xlsx" | Verify file path relative to rebuild_2025 directory |
| "Failed to create customer X" | Check for duplicate names, review GORM error |

---

## Build Info

```bash
# Rebuild executable
go build -o rebuild_2025.exe

# Size: 18 MB (includes GORM + PostgreSQL + Excel parser)
# Go version: 1.21+
```

---

## Performance

- **Current runtime**: 3-5 seconds (Steps 1-6)
- **Estimated full runtime**: 30-60 seconds (with all imports, 2000+ invoices)

---

**Status**: ✅ Steps 1-6 production-ready | ⚠️ Steps 7-10 placeholder
**Quality**: Full State execution, zero compilation errors, comprehensive logging

**Om Lokah Samastah Sukhino Bhavantu** 🙏
