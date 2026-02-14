# Schema.go Quick Reference Card

**File**: `cmd/rebuild_2025/schema.go`
**Author**: Agent 1 (Database Schema)
**Status**: ✅ COMPLETE

---

## Functions You Can Call

### 1. Drop All Tables (FK-Safe)
```go
dropAllTables(db *gorm.DB)
```
- Drops 70 tables in 5-tier FK-safe order
- **Preserves**: devices, device_users, roles
- No error return (logs warnings internally)
- Idempotent (safe to run multiple times)

### 2. Seed Lookup Tables
```go
err := seedLookupTables(db *gorm.DB)
```
- Seeds 5 roles + 1 warehouse + 18 chart of accounts
- **Returns error** (critical operation - check and abort if fails)
- Not idempotent (will fail if records already exist)

### 3. Convert Database URL
```go
dsn := convertDatabaseURL(databaseURL string)
```
- Converts `postgres://` URL to GORM DSN format
- Handles Render.com, Heroku, Railway formats
- Returns input if already DSN format

---

## Usage in main.go

```go
package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    // 1. Load DATABASE_URL
    godotenv.Load("../../.env")
    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL == "" {
        log.Fatal("DATABASE_URL not set")
    }

    // 2. Convert to DSN
    dsn := convertDatabaseURL(databaseURL)

    // 3. Connect
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("Connection failed:", err)
    }

    // 4. Drop tables (preserves devices/device_users/roles)
    dropAllTables(db)

    // 5. Run GORM AutoMigrate (Agent 3)
    // ...

    // 6. Seed lookup tables
    if err := seedLookupTables(db); err != nil {
        log.Fatal("Lookup seeding failed:", err)
    }

    // 7-9. Import data (Agents 4-6)
    // ...
}
```

---

## What Gets Seeded

### Roles (5)
- `admin` - Full access
- `manager` - Manager + Finance
- `sales` - Sales only
- `operations` - Ops only
- `staff` - Read-only

### Warehouse (1)
- `WH-SITRA-01` - Main Warehouse - Sitra

### Chart of Accounts (18)
- 1000-1300: Assets (Bank, AR, Inventory)
- 2000-2200: Liabilities (AP, VAT)
- 3000-3100: Equity (Retained Earnings)
- 4000-4100: Revenue (Sales)
- 5000-5200: COGS (Purchases, Freight)
- 6000-6300: OpEx (Salaries, Rent, Marketing)

---

## What Gets Preserved

- ✅ `devices` table
- ✅ `device_users` table
- ✅ `roles` table

**Why**: Device registration state and user accounts must survive rebuilds.

---

## What Gets Dropped (70 Tables)

**Tier 1 (Leaf)**: chat_messages, grn_items, invoice_items, payment_prediction_accuracy, grade_changes, costing_additional_costs, costing_items, delivery_note_items, purchase_order_items, supplier_invoice_items, bank_statement_lines, journal_lines, offer_items, order_items

**Tier 2 (Middle)**: payments, supplier_payments, offer_follow_ups, bank_statements, goods_received_notes, delivery_notes, supplier_invoices, journal_entries

**Tier 3 (Parent)**: invoices, orders, offers, purchase_orders, opportunities, costing_sheets, conversations, bank_accounts

**Tier 4 (Master)**: customers, suppliers, products, warehouses, chart_of_accounts

**Tier 5 (Supporting)**: 35 tables including tally imports, sync records, notes, contacts, alerts, predictions, etc.

---

## Error Handling

| Function | Error Handling | Action |
|----------|----------------|--------|
| `dropAllTables()` | Logs warnings | Continue anyway |
| `seedLookupTables()` | Returns error | **Abort** if fails |

---

## Integration Checklist

- [ ] Import `cmd/rebuild_2025` functions
- [ ] Load DATABASE_URL from .env
- [ ] Call `convertDatabaseURL()`
- [ ] Connect to PostgreSQL
- [ ] Call `dropAllTables(db)`
- [ ] Run GORM AutoMigrate (Agent 3)
- [ ] Call `seedLookupTables(db)` and check error
- [ ] Import customers (Agent 4)
- [ ] Import opportunities (Agent 5)
- [ ] Import invoices/payments (Agent 6)

---

## Files Created by Agent 1

1. **schema.go** (12,976 bytes) - Main implementation
2. **SCHEMA_IMPLEMENTATION.md** - Detailed guide
3. **VISUAL_SUMMARY.txt** - ASCII architecture map
4. **AGENT_1_COMPLETION_REPORT.md** - Full completion report
5. **QUICK_REF.md** - This file

---

**Ready for integration!** 🚀

Om Lokah Samastah Sukhino Bhavantu 🙏
