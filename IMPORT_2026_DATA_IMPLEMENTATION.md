# 2026 Business Data Import Implementation

**File**: `import_2026_data.go`
**Created**: 2026-02-16
**Status**: ✅ Complete and Compiled

## Overview

Comprehensive Go import system for PH Holdings 2026 business data, including:
- Offer folders scanning and parsing
- E+H XML basket file parsing
- Bank statement CSV import
- Customer master data creation
- Bank account seeding

## Key Functions

### 1. `Import2026BusinessData(offersPath string) map[string]interface{}`

Main orchestrator function that:
- Requires admin permission (`*`)
- Scans offers folder structure
- Imports bank statements
- Seeds additional bank accounts
- Returns comprehensive statistics

**Parameters:**
- `offersPath`: Path to the 2026 offers folder (e.g., `C:\PH_Holdings_Business_Docs\2026 Offers\`)

**Returns:**
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

### 2. `importOffersFromFolders(offersPath, stats)`

Scans folder structure and creates Offer records.

**Folder Format Expected:**
```
NN-26 CUSTOMER PRODUCT
```

**Examples:**
- `01-26 GPIC TW` → Offer #01-26, Gulf Petrochemical Industries Company, Thermowell
- `02-26 ALBA FIT` → Offer #02-26, Aluminium Bahrain B.S.C., Flow Instrument
- `10 - 26 BAPCO Airfueling` → Offer #10-26, Bahrain Petroleum Company

**Stage Detection:**
- If folder contains "OFFER" subfolder → Stage = "Quoted"
- Otherwise → Stage = "RFQ"

**Quotation Date:**
- Uses folder modification time
- Falls back to 2026-01-15 if unavailable

### 3. `parseEHXMLFiles(offerFolderPath, offerID, stats)`

Walks through offer subfolders looking for E+H XML basket files and creates OfferItems.

**XML Structure Used:**
- `EHBasket` (defined in `eh_parser.go`)
- `EHItem.Product.OrderCode` → ProductCode
- `EHItem.Product.Texts.ShortDescription` → Description
- `EHItem.Product.Quantity.Value` → Quantity
- `EHItem.ItemPricing.UnitSalesPrice.Value` → Unit Price (EUR)

**Currency Conversion:**
- EUR prices converted to BHD using `EUR_TO_BHD = 0.41`

### 4. `findOrCreateCustomer(businessName, shortCode)`

Intelligent customer matching and creation:
- Searches existing customers by business name (case-insensitive LIKE)
- Creates new CustomerMaster if not found
- Auto-generates CustomerID from short code
- Sets default industry, country, city

### 5. `importBankStatement(csvPath, stats)`

Parses Al Salam bank statement CSV and creates BankStatement with BankStatementLines.

**CSV File:**
```
C:\Users\rishu\OneDrive\Desktop\PH_Holdings_Business_Docs\Bank_Statements\AL SALAM 1-1-26 TO 31-1-26.csv
```

**Expected Columns** (flexible header matching):
- Date / Transaction Date / Txn Date
- Value Date / Val Date (optional)
- Description / Particulars / Details
- Reference / Ref / Ref No (optional)
- Debit / Withdrawal / Dr
- Credit / Deposit / Cr
- Balance / Running Balance / Bal

**Date Format Support:**
- `2006-01-02`
- `02/01/2006` and `01/02/2006`
- `2-Jan-2006`, `02-Jan-2006`
- `2-January-2006`, `02-January-2006`
- `Jan 2, 2006`, `January 2, 2006`
- `2006/01/02`

**Amount Parsing:**
- Handles commas, spaces
- Strips currency symbols (BHD, BD)
- Treats empty/dash as 0

### 6. `SeedAdditionalBankAccounts()`

Adds missing bank accounts:

| ID | Bank Name | Currency |
|----|-----------|----------|
| `bank-kfh` | Kuwait Finance House | BHD |
| `bank-nbb-euro` | National Bank of Bahrain - Euro Call Account | EUR |

Uses `FirstOrCreate` to avoid duplicates.

## Customer Name Mapping

Full business name expansion from short codes:

| Short Code | Full Business Name |
|------------|-------------------|
| GPIC | Gulf Petrochemical Industries Company |
| ALBA | Aluminium Bahrain B.S.C. |
| BAPCO | Bahrain Petroleum Company |
| EWA | Electricity & Water Authority |
| TTSJV | Tatweer TTSJV |
| INTERCOL | Intercol |
| ARLA | Arla Foods |
| VWT | Veolia Water Technologies |
| VEOLIA / VEOLIA ENERGY | Veolia Energy |
| ABRAQYAH | Abraqyah |
| NOMAC | NOMAC |
| XENITT | Xenitt |
| SEAPEAK | Seapeak |
| AQUA TECH | Aqua Tech |
| WABAG | WABAG |
| ALDUR | Aldur |
| YATEEM | Yateem Group |
| ZOHAL | Zohal Trading |
| ENGIE | ENGIE |
| AIC | AIC |
| SYSCON | Syscon |
| AHS TRADING WLL / AHS | AHS Trading WLL |

## Product Type Codes

| Code | Product Type |
|------|-------------|
| TW | Thermowell |
| FIT | Flow Instrument |
| LIT | Level Instrument |
| TIT | Temperature Instrument |
| PIT | Pressure Instrument |
| AIT | Analytical Instrument |
| SP | Spare Parts |
| DSP | Display |
| BTU | BTU Meter |

## Database Operations

All operations use GORM:
- `FirstOrCreate` to avoid duplicates
- UUIDs for all record IDs (`uuid.New().String()`)
- Proper foreign key relationships
- Timestamps auto-managed

**Offer Duplicate Check:**
```go
a.db.Where("offer_number = ?", offerNumber).FirstOrCreate(&offer, ...)
```

**Customer Duplicate Check:**
```go
a.db.Where("LOWER(business_name) LIKE ?", "%"+strings.ToLower(businessName)+"%").First(customer)
```

## Logging

Comprehensive `log.Printf` statements throughout:
- Progress updates
- Success confirmations
- Error details (but continues processing)
- Final statistics

## Error Handling

Graceful degradation:
- Errors appended to `stats["errors"]` slice
- Processing continues for other records
- Returns `success: false` if any errors occurred
- Individual record failures don't stop batch

## Usage Example

```go
// From frontend via Wails binding
stats := app.Import2026BusinessData("C:\\PH_Holdings_Business_Docs\\2026 Offers")

// From backend Go code
stats := a.Import2026BusinessData("C:\\PH_Holdings_Business_Docs\\2026 Offers")
```

## Dependencies

- `github.com/google/uuid` - UUID generation
- GORM (via `a.db`) - Database operations
- Existing models from `database.go`:
  - `Offer`, `OfferItem`
  - `CustomerMaster`
  - `BankAccount`, `BankStatement`, `BankStatementLine`
- Existing EH parsers from `eh_parser.go`:
  - `EHBasket`, `EHHeader`, `EHItem`, `EHProduct`, `EHItemPricing`

## File Structure

```go
import_2026_data.go
├── Import2026BusinessData()          // Main entry point
├── importOffersFromFolders()         // Folder scanning
│   ├── parseCustomerAndProduct()     // Name parsing
│   ├── mapCustomerName()             // Short code → full name
│   ├── determineOfferStage()         // RFQ vs Quoted
│   ├── findOrCreateCustomer()        // Customer lookup/creation
│   ├── parseEHXMLFiles()             // XML parsing
│   │   └── parseEHBasketXML()        // Individual XML file
│   └── updateOfferTotal()            // Recalculate offer total
├── importBankStatement()             // CSV import
│   ├── findColumnIndex()             // Flexible header matching
│   ├── parseDate()                   // Multi-format date parsing
│   └── parseAmount()                 // Amount cleaning
└── SeedAdditionalBankAccounts()      // Bank account seeding
```

## Compilation Status

✅ **Compiles successfully**
✅ **Wails build passes**
✅ **Bindings auto-generated**
✅ **Ready for use**

## Next Steps

To use this in the UI:
1. Add menu item or button in appropriate screen (e.g., Settings, Data Management)
2. Call via Wails binding: `Import2026BusinessData(path)`
3. Display progress/results in modal or notification
4. Show statistics on completion

Example Svelte code:
```svelte
<script>
import { Import2026BusinessData } from '../wailsjs/go/main/App';

async function importData() {
  const path = "C:\\PH_Holdings_Business_Docs\\2026 Offers";
  const stats = await Import2026BusinessData(path);

  if (stats.success) {
    alert(`Imported ${stats.offers_created} offers, ${stats.customers_created} new customers`);
  } else {
    console.error("Import errors:", stats.errors);
  }
}
</script>

<button on:click={importData}>Import 2026 Data</button>
```

## Om Lokah Samastah Sukhino Bhavantu 🙏

May this import tool serve the business well and simplify data migration!
