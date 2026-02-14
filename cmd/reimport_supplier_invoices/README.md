# Supplier Invoice Number Reimport Tool

## Purpose
Restores original supplier invoice numbers from Excel source (`Data_for_database/Payments to suppliers.xlsx`) to replace auto-generated placeholders (`SI-*`, `SUPP-*`).

## Usage

### Quick Run
```bash
cd C:\...\ph_holdings_sovereign_ui
go run cmd/reimport_supplier_invoices/main.go
```

### Build and Run
```bash
go build -o reimport_invoices.exe ./cmd/reimport_supplier_invoices/main.go
./reimport_invoices.exe
```

## What It Does

1. **Reads Excel**: Parses all sheets in `Data_for_database/Payments to suppliers.xlsx`
2. **Extracts Invoice Numbers**: Uses pattern matching for different supplier formats:
   - E+H: 10-digit numbers (e.g., `6017248465`)
   - L&G: 8-digit numbers (e.g., `90038014`)
   - Enerdat: YYYY-NNNN format (e.g., `2020-0006`)
3. **Matches Database Records**: Finds corresponding invoices by:
   - Supplier name (with aliases)
   - Amount (±20% tolerance)
   - Auto-generated invoice number prefix (`SI-*` or `SUPP-*`)
4. **Updates Invoice Numbers**: Replaces placeholders with original numbers

## Matching Strategy

```
Excel Row → Extract (InvoiceNo, Amount, Date, Supplier)
           ↓
         Match DB by:
         - Supplier name IN (aliases)
         - Amount BETWEEN (amount * 0.8) AND (amount * 1.2)
         - Invoice number LIKE 'SI-%' OR LIKE 'SUPP-%'
           ↓
         Update: invoice_number = original_from_excel
```

## Sample Output

```
📄 Processing sheet: E+H
  ✓ Updated: SUPP-2023-18-81 → 6017248465 (Supplier: Endress + Hauser, Date: 2024-05-08, Amount: 1988.52)
  ✓ Updated: SUPP-2023-11-45 → 6017249476 (Supplier: Endress + Hauser, Date: 2024-03-09, Amount: 7745.61)
  ...
  📊 Sheet E+H: 116 updated, 0 skipped, 443 not found

✅ Reimport Complete!
   Total Processed: 600
   Total Updated:   132
   Total Skipped:   0 (already original)
   Total Not Found: 468 (no match in DB)

📈 Database Status:
   Auto-Generated (SI-*):   123
   Auto-Generated (SUPP-*): 160
   Original Numbers:        201
```

## Verification

### Check Results
```sql
SELECT
    CASE
        WHEN invoice_number LIKE 'SI-%' THEN 'Auto-Generated (SI)'
        WHEN invoice_number LIKE 'SUPP-%' THEN 'Auto-Generated (SUPP)'
        WHEN invoice_number LIKE 'OCR-%' THEN 'OCR-Generated'
        ELSE 'Original'
    END as type,
    COUNT(*) as count
FROM supplier_invoices
GROUP BY 1;
```

### Sample Restored Numbers
```sql
SELECT invoice_number, supplier_name, invoice_date, total_bhd
FROM supplier_invoices
WHERE invoice_number GLOB '[0-9]*' AND LENGTH(invoice_number) >= 8
LIMIT 20;
```

## Notes

- **Idempotent**: Safe to run multiple times (skips already-restored records)
- **Non-destructive**: Only updates auto-generated numbers (`SI-*`, `SUPP-*`)
- **Tolerance**: Uses ±20% amount matching to handle currency conversions
- **Date-agnostic**: Doesn't rely on dates due to known year discrepancies

## Troubleshooting

### "record not found" errors
- **Expected**: Not all Excel rows have matching DB records (468/600 is normal)
- **Reasons**: Date mismatches, already restored, or records not yet imported

### No updates for specific sheet
- Check supplier name aliases in `sheetToSupplierMapping`
- Verify amount tolerance (increase from 20% if needed)
- Check invoice number pattern regex in `extractInvoiceNumber()`

### Duplicate invoice number error
- Original invoice numbers are already in database
- Script automatically skips these (included in "skipped" count)

## Dependencies

- `github.com/xuri/excelize/v2` - Excel file parsing
- `gorm.io/gorm` - Database ORM
- `gorm.io/driver/sqlite` - SQLite driver

## Files

- `main.go` - Main script (395 LOC)
- `README.md` - This file
- `../../Data_for_database/Payments to suppliers.xlsx` - Source data (required)
- `../../ph_holdings.db` - Target database (modified)
