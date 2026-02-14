# PH Holdings Demo Data Seeding

This tool generates realistic sample data for professional demos of PH Holdings Sovereign UI.

## What It Seeds

### 1. Master Data (`seed_master_data`)
Run this FIRST to create foundational data:
- **4 Suppliers**: E+H, LG, Servomex, ABB (competitor)
- **30 Customers**: 4 Grade A, 8 Grade B, 12 Grade C, 6 Grade D
  - Realistic Bahraini/GCC company names
  - Complete payment profiles and histories
- **12 Products**: Flow meters, analyzers, pressure transmitters, displays
  - Real E+H part numbers and pricing
  - Competitive info vs ABB

### 2. Transaction Data (`seed_demo_data`)
Run this SECOND to create operational data:
- **10 Offers/RFQs**: Complete sales pipeline
  - Mix of stages: RFQ, Quoted, Follow-up, Won, Lost
  - Realistic values (28K - 120K BHD)
  - Win probabilities and competition tracking
- **15 Invoices**: Financial activity
  - 5 Paid (good payment history)
  - 3 Pending (within terms)
  - 4 Overdue (need follow-up!)
  - 3 Recent (just sent)
- **10 Payment Predictions**: AI predictions for top customers
  - Grade-appropriate R1/R2/R3 regime values
  - Realistic confidence scores
  - Predicted payment days
- **5 Quick Captures**: Demo notes/tasks
  - Payment follow-ups
  - Sales opportunities
  - Customer meetings
  - Collection tasks

## Usage

### Option 1: Run Both (Recommended)
```bash
cd ph_holdings_sovereign_ui

# Clean slate (optional)
rm ph_holdings.db

# Run migration first
go run cmd/migrate_schema/main.go

# Seed master data
go run cmd/seed_master_data/main.go

# Seed demo/transaction data
go run cmd/seed_demo_data/main.go
```

### Option 2: Quick Combined Script
```bash
# On Windows (PowerShell)
cd ph_holdings_sovereign_ui
.\scripts\seed_all.ps1

# On Linux/Mac
cd ph_holdings_sovereign_ui
bash scripts/seed_all.sh
```

### Option 3: Individual Seeds
```bash
# Master data only (customers, products, suppliers)
go run cmd/seed_master_data/main.go

# Transaction data only (invoices, offers, predictions)
go run cmd/seed_demo_data/main.go
```

## What You'll See

After seeding, the app will have:

### Customer 360 View
- 30 customers across all payment grades
- Complete payment histories and predictions
- Mix of industries: Oil & Gas, EPC, Manufacturing, etc.

### Pricing Screen
- 12 products with realistic E+H pricing
- 50% margins typical
- ABB competitive equivalents tracked

### Inbox Screen (Offers/RFQs)
- Active pipeline with various stages
- Emergency orders flagged
- ABB competition tracked
- Win probabilities calculated

### Customer 360
- Real customers: BAPCO, ALBA, Petrofac, etc.
- Complete payment profiles
- Prediction grades with R1/R2/R3 regime data
- Outstanding invoices and aging

### Quick Captures
- Sample follow-up tasks
- Payment collection reminders
- Sales opportunities

## Database Location

By default: `./ph_holdings.db`

Environment variable override: `PH_DB_PATH` or `DATABASE_PATH`

## Resetting Data

To start fresh:
```bash
# Delete the database
rm ph_holdings.db

# Re-run migration + seeds
go run cmd/migrate_schema/main.go
go run cmd/seed_master_data/main.go
go run cmd/seed_demo_data/main.go
```

## Real vs Demo Data

**This is DEMO data** - realistic but fictional:
- Company names are real (BAPCO, ALBA, etc.)
- Amounts and dates are plausible
- Payment patterns match real-world grades
- BUT: This is NOT production data

To switch to real data:
1. Back up demo database: `cp ph_holdings.db ph_holdings_demo.db`
2. Delete demo data: `rm ph_holdings.db`
3. Run migration: `go run cmd/migrate_schema/main.go`
4. Import real data (from OCR, Excel, etc.)

## Sample Customers Created

### Grade A (Excellent Payers)
- **CO1**: Bahrain Petroleum Company (BAPCO) - 28.5 avg days
- **CO5**: National Oil & Gas Authority (NOGA) - 32.0 avg days
- **EC3**: Saudi Engineering Services - 42.0 avg days
- **EP1**: Petrofac Engineering - 58.0 avg days

### Grade B (Good Payers)
- **CO2**: Aluminum Bahrain (ALBA) - 52.0 avg days
- **CO8**: Gulf Petrochemical Industries - 48.0 avg days
- **EC12**: Bahrain Engineering Services - 65.0 avg days
- **EP4**: Tecnicas Reunidas Middle East - 68.0 avg days

### Grade C (Fair Payers)
- **CO11**: Bahrain Steel Company - 68.0 avg days
- **CO3**: Gulf Industrial Services - 58.0 avg days
- **CO17**: Oman Refineries Company - 52.0 avg days

### Grade D (Risky Payers)
- **CO13**: Bahrain Chemical Company - 88.0 avg days
- **EC10**: Al-Jazira Engineering - 95.0 avg days
- **EC16**: Desert Engineering LLC - 102.0 avg days

## Technical Details

### Dependencies
- GORM for database operations
- SQLite for storage
- Matches exact schema from `database.go`

### Data Integrity
- Foreign key constraints respected
- Realistic relationships (invoices → customers, items → products)
- Proper timestamps and audit fields
- Three-regime R1/R2/R3 values sum to ~1.0

### Randomization
- Amounts vary realistically by customer grade
- Dates spread over last 90 days
- Mix of outcomes (paid, pending, overdue)
- Seeded for reproducibility

## Support

Issues? Check:
1. Did you run `seed_master_data` first?
2. Does `ph_holdings.db` exist?
3. Did migration run successfully?
4. Check console output for errors

## Making It Yours

To customize the seed data:
1. Edit `cmd/seed_master_data/main.go` for customers/products
2. Edit `cmd/seed_demo_data/main.go` for invoices/offers
3. Adjust amounts, dates, names to match your demo needs
4. Re-run the scripts

---

**Built with MATHEMATICAL RIGOR × PRODUCTION ROBUSTNESS × DEMO REALISM**

*May this demo data benefit all entities presenting PH Holdings!* 🙏
