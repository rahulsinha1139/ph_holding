# 2026 Data Import - Quick Reference

## TL;DR

```go
// Import everything from 2026 offers folder
stats := app.Import2026BusinessData("C:\\PH_Holdings_Business_Docs\\2026 Offers")
```

## What It Does

✅ Scans offer folders → Creates Offer records
✅ Parses E+H XML files → Creates OfferItem records
✅ Matches/creates customers → Updates CustomerMaster
✅ Imports bank statement → Creates BankStatement + lines
✅ Adds KFH & NBB Euro accounts → Updates BankAccount

## Folder Format

```
01-26 GPIC TW          → Offer #01-26, GPIC, Thermowell
02-26 ALBA FIT         → Offer #02-26, ALBA, Flow Instrument
10 - 26 BAPCO Whatever → Offer #10-26, BAPCO
```

## Customer Codes

| Code | Expands To |
|------|------------|
| GPIC | Gulf Petrochemical Industries Company |
| ALBA | Aluminium Bahrain B.S.C. |
| BAPCO | Bahrain Petroleum Company |
| EWA | Electricity & Water Authority |
| VEOLIA | Veolia Energy |
| AHS | AHS Trading WLL |

See `IMPORT_2026_DATA_IMPLEMENTATION.md` for full list.

## Product Codes

| Code | Type |
|------|------|
| TW | Thermowell |
| FIT | Flow Instrument |
| LIT | Level Instrument |
| PIT | Pressure Instrument |

## Bank Statement

**File**: `C:\...\Bank_Statements\AL SALAM 1-1-26 TO 31-1-26.csv`

**Required Columns:**
- Date (any format: DD/MM/YYYY, DD-Mon-YYYY, etc.)
- Debit
- Credit
- Balance

**Optional:** Value Date, Description, Reference

## Output

```json
{
  "success": true,
  "offers_created": 15,
  "offer_items_created": 142,
  "customers_created": 3,
  "customers_matched": 12,
  "bank_statements": 1,
  "bank_transactions": 234,
  "bank_accounts_added": 2,
  "errors": []
}
```

## Key Features

- **Smart Matching**: Finds existing customers before creating new ones
- **Flexible Parsing**: Handles various date/amount formats
- **E+H Integration**: Parses XML basket files automatically
- **EUR → BHD**: Auto-converts E+H prices (EUR × 0.41)
- **Duplicate Safe**: Uses `FirstOrCreate` - won't create duplicates
- **Graceful Errors**: Logs errors but continues processing

## Permission Required

Admin only (`*` permission)

## Files

- `import_2026_data.go` - Main implementation (693 LOC)
- `IMPORT_2026_DATA_IMPLEMENTATION.md` - Full docs
- `IMPORT_2026_QUICKREF.md` - This file

## Build Status

✅ Compiles successfully
✅ Wails bindings generated
✅ Ready to use

## Usage in UI

```svelte
<script>
import { Import2026BusinessData } from '../wailsjs/go/main/App';

let loading = false;
let stats = null;

async function doImport() {
  if (loading) return;
  loading = true;
  try {
    stats = await Import2026BusinessData("C:\\PH_Holdings_Business_Docs\\2026 Offers");
  } finally {
    loading = false;
  }
}
</script>

<button on:click={doImport} disabled={loading}>
  {loading ? 'Importing...' : 'Import 2026 Data'}
</button>

{#if stats}
  <div>
    ✅ Created {stats.offers_created} offers
    ✅ Created {stats.customers_created} customers
    ✅ Imported {stats.bank_transactions} bank transactions
  </div>
{/if}
```

## Om Lokah Samastah Sukhino Bhavantu 🙏
