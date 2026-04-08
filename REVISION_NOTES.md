# REVISION_NOTES.md - AsymmFlow ERP

## Phase 21: Delivery Fulfillment, Partial Invoicing & Inventory Removal (2026-02-22)

### Summary
Closed 4 gaps in the Order -> Deliver -> Invoice pipeline so partial deliveries work end-to-end. Removed all inventory/stock/warehouse code — PH Trading is buy-to-order (no stock kept).

### Changes Made

#### 1. OrderItem fulfillment tracking (`database.go`)
- Added `QuantityDelivered` field to `OrderItem` struct
- GORM AutoMigrate adds the column automatically on next startup

#### 2. Delivery note fulfillment updates (`delivery_note_service.go`)
- **CreateDNFromOrder**: Updates `OrderItem.QuantityShipped` atomically after DN creation
- **ConfirmDeliveryNote**: Updates `OrderItem.QuantityDelivered` atomically when DN status changes to "Delivered"

#### 3. DN-based partial invoicing (`customer_invoice_service.go`)
- **Relaxed duplicate check**: Multiple invoices per order allowed when delivery notes exist
- **DN-based item creation**: When `deliveryNoteID` provided, uses `dnItem.QuantityDelivered` as invoice line quantity
- **Invoiceable quantity validation**: When no DN specified but order has DNs, only invoices delivered-but-not-yet-invoiced quantities
- **Recalculated totals**: Partial invoices compute SubtotalBHD from actual items, VAT from invoice subtotal (not order total)
- **CreateInvoiceFromDN wrapper**: New function loads DN, resolves OrderID, calls CreateInvoiceWithOptions
- **QuantityInvoiced update**: Uses atomic SQL increment matching actual invoice line quantities

#### 4. Partial invoice awareness (`payment_service.go`)
- **ProgressOrderOnInvoice**: Checks if all items are fully invoiced; sets "PartiallyInvoiced" if not, "Invoiced" if complete

#### 5. Full inventory/stock/warehouse removal
PH Trading is a buy-to-order business — no stock is kept. Removed all inventory code:

**Backend removed:**
- `inventory_service.go` — deleted entirely (336 lines: alerts, reorder suggestions)
- `app.go` — removed 12 inventory/stock/warehouse API functions (~480 lines)
- `database.go` — removed `InventoryItem`, `StockMovement`, `StockAdjustment`, `Warehouse` structs
- `database.go` — removed FK constraint on `GoodsReceivedNote.WarehouseID`
- `reports.go` — removed `StockMovementSummary` struct, inventory report data fields, `getInventoryReport` function
- `report_generators.go` — removed `addInventoryReportToPDF`, `addInventoryReportToExcel`, inventory case branches
- `tally_importer.go` — replaced inventory asset calculation with `report.Inventory = 0`

**Frontend removed:**
- `wailsMock.ts` — removed 13 inventory/stock/warehouse mock functions
- `types.ts` / `types/index.ts` — removed `'inventory'` from `ReportCategory`, removed `InventoryReportData` interface
- `ReportsScreen.svelte` — removed inventory report category option
- `ShowcaseScreen.svelte` — removed inventory tab
- `DashboardScreen.svelte` — removed "Low Stock Alerts" action item
- `FinancialDashboard.svelte` — zeroed hardcoded inventory value

**Database tables left dormant** (empty, harmless):
- `inventory_items`, `stock_movements`, `stock_adjustments`, `warehouses`
- No migration needed — tables aren't referenced by any code

### Edge Cases Handled
- **Legacy orders (no DNs)**: Full order quantities used (backward compatible)
- **Atomic updates**: SQL `quantity_shipped = quantity_shipped + ?` prevents race conditions
- **Multiple invoices per order**: Allowed when DNs exist, blocked for legacy
- **Nothing to invoice**: Returns error when no delivered-but-uninvoiced quantities remain

---

## Pending Items (Medium/Low Priority)

### From Phase 17 P2 (Security)
- Deterministic PBKDF2 salt in `field_crypto.go`
- OAuth callback binds to all interfaces (should be `127.0.0.1:8080`)
- Cloud sync defaults to enabled (should default to false)
- `.env` file traversal (walks parent directories)
- Business invariants not enforced in mutation paths
- FX rate manipulation (no sanity check on exchange rates)
- Bank reconciliation amount mismatch on manual match
- Supplier invoice approval lacks segregation of duties
- `CreateUser` skips validation
- Filesystem traversal in document classifier
- `ProcessInboxDocument` no auth
- Batch operations unbounded
- Butler AI prompt injection
- Outdated frontend deps (Vite 3.x, Svelte 3.x)
- Incomplete log sanitization
- Log file world-readable (0666 -> 0600)

### Future Enhancements
- Backfill existing data: Set `QuantityDelivered` from historical DN items for orders already delivered
- DN-based invoice PDF: Show DN number and delivered quantities on invoice PDF
- Partial delivery dashboard: Show fulfillment progress per order
- Credit note support for returned/rejected deliveries
