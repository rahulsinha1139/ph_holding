# AsymmFlow Client Data Reconciliation Pack

Prepared for PH Trading / AHS Trading data reconciliation and import.

This document is the client-facing fill guide for the reconciliation workbook. Please create one Excel workbook using the sheet names and column headers below. Do not rename sheets or headers. If a field is unknown, leave it blank rather than entering placeholder text.

The purpose of this pack is to let the AsymmFlow team validate client data and seed it into the operational database with minimal manual cleanup.

## Delivery Checklist

Please send:

- One Excel workbook named `AsymmFlow_Data_Reconciliation_YYYY_MM_DD.xlsx`.
- Original bank statement PDFs, CSVs, or Excel files referenced in the workbook.
- Any Tally exports used as source evidence.
- Any unmatched customer, supplier, invoice, PO, cheque, or bank reference notes in the `99_recon_questions` sheet.

## Global Rules

Use these formats everywhere:

| Data type | Format |
|---|---|
| Date | `YYYY-MM-DD` |
| Date and time | `YYYY-MM-DD HH:MM:SS` |
| Currency code | `BHD`, `USD`, `EUR`, `GBP`, `AED`, `SAR`, etc. |
| BHD amounts | Decimal with 3 places, no commas, for example `1250.750` |
| Foreign amounts | Decimal, no commas |
| Boolean | `TRUE` or `FALSE` |
| JSON list | `["INV-001","INV-002"]` |
| Blank optional field | Leave empty |

System-generated columns do not need to be filled by the client: `id`, `created_at`, `updated_at`, `deleted_at`, `version`, `created_by`, internal UUID links, journal IDs, audit logs, sync records, hashes, and application user/device/license records.

Where this template uses a `*_key` column, it is an import helper. The importer will map it to an internal database ID. Good keys are stable business values such as customer code, supplier code, invoice number, PO number, bank account ID, statement number, cheque number, or folder number.

## Core Import Order

The workbook should be imported in this order:

1. `00_manifest`
2. `01_company_bank_accounts`
3. `02_customers`
4. `03_customer_contacts`
5. `04_suppliers`
6. `05_supplier_contacts`
7. `06_products`
8. `07_opportunities`
9. `08_opportunity_line_items`
10. `09_orders`
11. `10_order_items`
12. `11_customer_invoices`
13. `12_customer_invoice_items`
14. `13_customer_payments`
15. `14_purchase_orders`
16. `15_purchase_order_items`
17. `16_goods_received_notes`
18. `17_grn_items`
19. `18_supplier_invoices`
20. `19_supplier_invoice_items`
21. `20_supplier_payments`
22. `21_delivery_notes`
23. `22_delivery_note_items`
24. `23_serial_numbers`
25. `24_bank_statements`
26. `25_bank_statement_lines`
27. `26_bank_line_allocations`
28. `27_deposits_in_transit`
29. `28_cheque_registers`
30. `29_outstanding_cheques`
31. `30_bank_expense_entries`
32. `31_book_bank_reconciliations`
33. `32_fx_rates`
34. `99_recon_questions`

Sheets that do not apply may be left empty, but the sheet name and header row should remain.

## Existing Bank Account IDs

Use these values in `bank_account_key` where applicable. If a bank account is missing or incorrect, fill `01_company_bank_accounts`.

| bank_account_key | Division | Bank | Account name | Currency |
|---|---|---|---|---|
| `bank-ahs-alsalam` | AHS Trading | Al Salam Bank | AHS TRADING W.L.L. | BHD |
| `bank-nbb` | PH Trading | National Bank of Bahrain BSC | P H TRADING WLL | BHD |
| `bank-ahli` | PH Trading | Ahli United Bank | P H TRADING WLL | BHD |
| `bank-nbb-euro` | PH Trading | National Bank of Bahrain BSC | P H TRADING WLL - Euro Call | EUR |
| `bank-alsalam` | PH Trading | Al Salam Bank | P H TRADING WLL | BHD |
| `bank-alsalam-ahs` | PH Trading | Al Salam Bank | AHS Account | BHD |
| `bank-bbk` | PH Trading | BBK | P H TRADING WLL | BHD |
| `bank-kfh` | PH Trading | Kuwait Finance House | P H TRADING WLL | BHD |

## Bank Reconciliation Match Opportunities

Every bank statement line should be categorized into one of these reconciliation paths.

| Money direction | transaction_type | Match target | Allocation type |
|---|---|---|---|
| Credit | `CUSTOMER_PAYMENT` | Customer invoice receipt | `CUSTOMER_INVOICE` |
| Credit | `CHEQUE_RECEIVED` | Customer invoice receipt by cheque | `CUSTOMER_INVOICE` |
| Credit | `INTERNAL_TRANSFER` | Transfer from another company bank account | Leave allocation blank unless matched manually |
| Credit | `OTHER` | Manual review | Leave allocation blank or explain in notes |
| Debit | `SUPPLIER_PAYMENT` | Supplier invoice payment | `SUPPLIER_INVOICE` |
| Debit | `CHEQUE_ISSUED` | Outstanding cheque / supplier invoice | `SUPPLIER_INVOICE` |
| Debit | `BANK_FEE` | Bank charge expense | `EXPENSE` |
| Debit | `SWIFT_CHARGE` | Bank charge expense | `EXPENSE` |
| Debit | `VAT_ON_FEE` | VAT on bank fees | `EXPENSE` |
| Debit | `BG_FEE` | Bank guarantee fee | `EXPENSE` |
| Debit | `CORRESPONDENT_CHARGE` | Correspondent bank charge | `EXPENSE` |
| Debit | `SALARY` | Payroll payout | Manual review |
| Debit | `BILL_PAYMENT` | Utility or vendor payment | `EXPENSE` or manual review |
| Debit or credit | `FX_CONVERSION` | Currency movement | Manual review |
| Debit or credit | `OTHER` | Manual review | Leave allocation blank or explain in notes |

Allowed bank line `match_type` values: `Auto`, `Manual`, `Split`, `Unmatched`.

Allowed allocation `status` values: `Allocated`, `Verified`, `Disputed`.

## Sheet Structures

### 00_manifest

Use this sheet to describe the workbook and data coverage.

```csv
submission_date,prepared_by,company,period_start,period_end,source_system,source_files,notes
```

| Column | Required | Notes |
|---|---|---|
| `submission_date` | Yes | Date the workbook is sent. |
| `prepared_by` | Yes | Person responsible for the data. |
| `company` | Yes | `PH Trading`, `AHS Trading`, or `Both`. |
| `period_start` | Yes | Earliest transaction date covered. |
| `period_end` | Yes | Latest transaction date covered. |
| `source_system` | Optional | Tally, Excel, bank portal, manual, etc. |
| `source_files` | Optional | File names separated by semicolon. |
| `notes` | Optional | Anything we should know before import. |

### 01_company_bank_accounts

Targets database table `company_bank_accounts`.

```csv
bank_account_key,division,bank_name,account_name,account_number,iban,swift_bic,currency,is_active,display_order,booking_rate,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `bank_account_key` | Yes | `id` | Stable ID such as `bank-nbb`. |
| `division` | Yes | `division` | `PH Trading` or `AHS Trading`. |
| `bank_name` | Yes | `bank_name` | Official bank name. |
| `account_name` | Yes | `account_name` | Name on bank account. |
| `account_number` | Yes | `account_number` | Account number exactly as bank shows it. |
| `iban` | Optional | `iban` | Required if available. |
| `swift_bic` | Optional | `swift_bic` | Required for transfers if available. |
| `currency` | Yes | `currency` | Usually `BHD`, sometimes `EUR` or `USD`. |
| `is_active` | Yes | `is_active` | `TRUE` for active accounts. |
| `display_order` | Optional | `display_order` | Numeric sort order. |
| `booking_rate` | Optional | `booking_rate` | Opening FX booking rate for foreign currency accounts. |
| `notes` | Optional | Import note | Not stored unless importer audit keeps it. |

### 02_customers

Targets database table `customers`.

```csv
customer_key,customer_id,customer_code,customer_type,business_name,short_code,trading_name,cr_number,status,primary_phone,mobile_number,primary_email,website,address_line1,city,country,trn,industry,relation_years,payment_grade,customer_grade,payment_terms_days,avg_payment_days,dispute_count,total_orders_value,total_orders_count,avg_order_value,last_order_date,ar_risk_tier,outstanding_bhd,overdue_days,credit_limit_bhd,is_credit_blocked,requires_prepayment,has_abb_competition,is_emergency_only,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `customer_key` | Yes | Import key | Use customer code if known; otherwise exact business name. |
| `customer_id` | Optional | `customer_id` | Existing AsymmFlow customer ID if known. |
| `customer_code` | Optional | `customer_code` | Alternative business code. |
| `customer_type` | Optional | `customer_type` | End Customer, Engineering Company, OEM, etc. |
| `business_name` | Yes | `business_name` | Legal or main display name. |
| `short_code` | Optional | `short_code` | ALBA, EWA, GPIC, etc. |
| `trading_name` | Optional | `trading_name` | Trade name if different. |
| `cr_number` | Optional | `cr_number` | Commercial registration number. |
| `status` | Yes | `status` | Usually `Active`. |
| `primary_phone` | Optional | `primary_phone` | Main office phone. |
| `mobile_number` | Optional | `mobile_number` | Main mobile or WhatsApp number. |
| `primary_email` | Optional | `primary_email` | Main email. |
| `website` | Optional | `website` | Website URL. |
| `address_line1` | Optional | `address_line1` | Full address is acceptable. |
| `city` | Optional | `city` | City. |
| `country` | Optional | `country` | Country. |
| `trn` | Optional | `trn` | VAT/TRN number. |
| `industry` | Optional | `industry` | Oil and Gas, Utilities, Manufacturing, etc. |
| `relation_years` | Optional | `relation_years` | Number. |
| `payment_grade` | Optional | `payment_grade` | `A`, `B`, `C`, or `D`. |
| `customer_grade` | Optional | `customer_grade` | Overall grade, `A`, `B`, `C`, or `D`. |
| `payment_terms_days` | Optional | `payment_terms_days` | Numeric days. |
| `avg_payment_days` | Optional | `avg_payment_days` | Numeric days. |
| `dispute_count` | Optional | `dispute_count` | Number of payment disputes. |
| `total_orders_value` | Optional | `total_orders_value` | Lifetime value if known. |
| `total_orders_count` | Optional | `total_orders_count` | Lifetime order count if known. |
| `avg_order_value` | Optional | `avg_order_value` | Average order value if known. |
| `last_order_date` | Optional | `last_order_date` | `YYYY-MM-DD`. |
| `ar_risk_tier` | Optional | `ar_risk_tier` | Low, Medium, High, Critical. |
| `outstanding_bhd` | Optional | `outstanding_bhd` | Current AR outstanding. |
| `overdue_days` | Optional | `overdue_days` | Oldest overdue days. |
| `credit_limit_bhd` | Optional | `credit_limit_bhd` | Credit limit in BHD. |
| `is_credit_blocked` | Optional | `is_credit_blocked` | `TRUE` or `FALSE`. |
| `requires_prepayment` | Optional | `requires_prepayment` | `TRUE` or `FALSE`. |
| `has_abb_competition` | Optional | `has_abb_competition` | `TRUE` or `FALSE`. |
| `is_emergency_only` | Optional | `is_emergency_only` | `TRUE` or `FALSE`. |
| `notes` | Optional | Import note | Use for name changes, WLL/BSC variants, merge hints. |

### 03_customer_contacts

Targets database table `customer_contacts`.

```csv
customer_key,contact_name,job_title,email,phone,address,is_primary_contact,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `customer_key` | Yes | `customer_id` via lookup | Must match `02_customers.customer_key`, `customer_id`, `customer_code`, or exact business name. |
| `contact_name` | Yes | `contact_name` | Contact person name. |
| `job_title` | Optional | `job_title` | Role or department. |
| `email` | Optional | `email` | Email. |
| `phone` | Optional | `phone` | Phone or mobile. |
| `address` | Optional | `address` | Contact-specific address. |
| `is_primary_contact` | Optional | `is_primary_contact` | `TRUE` if main contact. |
| `notes` | Optional | Import note | Not stored unless importer audit keeps it. |

### 04_suppliers

Targets database table `suppliers`.

```csv
supplier_key,supplier_code,supplier_name,country,lead_time_days,tax_id,supplier_type,brands_handled,product_types,primary_contact,email,phone,address,bank_name,account_number,iban,swift_code,payment_terms,rating,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `supplier_key` | Yes | Import key | Use supplier code if known, otherwise exact supplier name. |
| `supplier_code` | Optional | `supplier_code` | Stable supplier code. |
| `supplier_name` | Yes | `supplier_name` | Legal supplier name. |
| `country` | Optional | `country` | Country. |
| `lead_time_days` | Optional | `lead_time_days` | Numeric. |
| `tax_id` | Optional | `tax_id` | VAT/TRN/tax number. |
| `supplier_type` | Optional | `supplier_type` | Manufacturer, Distributor, Agent, Service Provider, etc. |
| `brands_handled` | Optional | `brands_handled` | JSON list or semicolon list. |
| `product_types` | Optional | `product_types` | JSON list or semicolon list. |
| `primary_contact` | Optional | `primary_contact` | Main contact name. |
| `email` | Optional | `email` | Main email. |
| `phone` | Optional | `phone` | Main phone. |
| `address` | Optional | `address` | Address. |
| `bank_name` | Optional | `bank_name` | Supplier bank. |
| `account_number` | Optional | `account_number` | Supplier account. |
| `iban` | Optional | `iban` | Supplier IBAN. |
| `swift_code` | Optional | `swift_code` | Supplier SWIFT/BIC. |
| `payment_terms` | Optional | `payment_terms` | Example `Net 30`. |
| `rating` | Optional | `rating` | 1 to 5. |
| `notes` | Optional | `notes` | Supplier notes. |

### 05_supplier_contacts

Targets database table `supplier_contacts`.

```csv
supplier_key,contact_name,job_title,email,phone,address,is_primary_contact,notes
```

Use the same rules as `03_customer_contacts`, with `supplier_key` matched to `04_suppliers`.

### 06_products

Targets database table `products`.

```csv
product_key,product_code,product_name,product_category,supplier_key,supplier_code,standard_cost_bhd,standard_price_bhd,description,is_active,stock_quantity,sku,part_number,hs_code,unit_of_measure,datasheet_url,specifications,requires_serial_tracking,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `product_key` | Yes | Import key | Use product code, model number, or stable item code. |
| `product_code` | Yes | `product_code` | Unique product/model code. |
| `product_name` | Yes | `product_name` | Product display name. |
| `product_category` | Optional | `product_category` | Flow, pressure, analytical, spare, service, etc. |
| `supplier_key` | Optional | `supplier_id` via lookup | Must match `04_suppliers`. |
| `supplier_code` | Optional | `supplier_code` | Supplier code if known. |
| `standard_cost_bhd` | Optional | `standard_cost_bhd` | Standard cost in BHD. |
| `standard_price_bhd` | Optional | `standard_price_bhd` | Standard sale price in BHD. |
| `description` | Optional | `description` | Product description. |
| `is_active` | Optional | `is_active` | `TRUE` or `FALSE`. |
| `stock_quantity` | Optional | `stock_quantity` | Whole number if known. |
| `sku` | Optional | `sku` | SKU. |
| `part_number` | Optional | `part_number` | Manufacturer part number. |
| `hs_code` | Optional | `hs_code` | HS code. |
| `unit_of_measure` | Optional | `unit_of_measure` | `EA`, `PCS`, `SET`, etc. |
| `datasheet_url` | Optional | `datasheet_url` | Link if available. |
| `specifications` | Optional | `specifications` | JSON object/text specs. |
| `requires_serial_tracking` | Optional | `requires_serial_tracking` | `TRUE` for serialized items. |
| `notes` | Optional | Import note | Not stored unless importer audit keeps it. |

### 07_opportunities

Targets database table `opportunities`.

```csv
opportunity_key,folder_number,year,opp_number,folder_name,title,eh_ref,source,customer_key,customer_name,customer_grade,salesperson,division,offer_date,expected_date,order_date,closed_date,delivery_terms,payment_terms,revenue_bhd,cost_bhd,profit_bhd,spoc_status,wip_status,stage,confidence,product_type,has_abb_competition,won_reason,lost_reason,comment,owner_notes,product_details_json,linked_offer_number,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `opportunity_key` | Yes | Import key | Prefer `folder_number`. |
| `folder_number` | Yes | `folder_number` | Unique physical/digital folder number. |
| `year` | Yes | `year` | Pipeline year, for example `2026`. |
| `opp_number` | Optional | `opp_number` | Sequence number within year. |
| `folder_name` | Optional | `folder_name` | Full folder name. |
| `title` | Yes | `title` | Opportunity/project title. |
| `eh_ref` | Optional | `eh_ref` | E+H reference if available. |
| `source` | Optional | `source` | `client_recon`, `old_db`, `2025_excel`, `2026_ocr`, etc. |
| `customer_key` | Optional | `customer_id` via lookup | Strongly recommended. |
| `customer_name` | Yes | `customer_name` | Customer name as shown in source. |
| `customer_grade` | Optional | `customer_grade` | A/B/C/D at opportunity time. |
| `salesperson` | Optional | `salesperson` | Owner or sales person. |
| `division` | Yes | `division` | `PH Trading` or `AHS Trading`. |
| `offer_date` | Optional | `offer_date` | Date offered/created. |
| `expected_date` | Optional | `expected_date` | Expected close date. |
| `order_date` | Optional | `order_date` | Date won/order received. |
| `closed_date` | Optional | `closed_date` | Actual close/loss date. |
| `delivery_terms` | Optional | `delivery_terms` | Delivery terms/weeks. |
| `payment_terms` | Optional | `payment_terms` | Commercial payment terms. |
| `revenue_bhd` | Optional | `revenue_bhd` | Expected or actual revenue. |
| `cost_bhd` | Optional | `cost_bhd` | Expected or actual cost. |
| `profit_bhd` | Optional | `profit_bhd` | Profit if known. |
| `spoc_status` | Optional | `spoc_status` | Internal SPOC status. |
| `wip_status` | Optional | `wip_status` | Internal work-in-progress status. |
| `stage` | Yes | `stage` | Suggested: `Qualified`, `Proposal`, `Quoted`, `Won`, `Lost`, `Lead`, `Negotiation`. |
| `confidence` | Optional | `confidence` | 0 to 1. |
| `product_type` | Optional | `product_type` | Flow, pressure, analyzer, spare, service, etc. |
| `has_abb_competition` | Optional | `has_abb_competition` | `TRUE` or `FALSE`. |
| `won_reason` | Optional | `won_reason` | Required for won opportunities if known. |
| `lost_reason` | Optional | `lost_reason` | Required for lost opportunities if known. |
| `comment` | Optional | `comment` | Latest salesperson comment. |
| `owner_notes` | Optional | `owner_notes` | Management notes. |
| `product_details_json` | Optional | `product_details` | JSON list. Use `08_opportunity_line_items` if easier. |
| `linked_offer_number` | Optional | `offer_id` via lookup | Offer/quote number linked to opportunity. |
| `notes` | Optional | Import note | Not stored unless importer audit keeps it. |

### 08_opportunity_line_items

This is a staging sheet. The importer can convert it into `opportunities.product_details` JSON or later costing line items.

```csv
opportunity_key,line_number,product_code,equipment,model,specification,detailed_description,supplier_key,quantity,currency,unit_cost_foreign,exchange_rate,total_cost_bhd,suggested_price_bhd,total_suggested_bhd,notes
```

| Column | Required | Notes |
|---|---|---|
| `opportunity_key` | Yes | Match to `07_opportunities`. |
| `line_number` | Yes | 1, 2, 3, etc. |
| `product_code` | Optional | Model or product code. |
| `equipment` | Optional | Equipment/product name. |
| `model` | Optional | Model number. |
| `specification` | Optional | Short spec. |
| `detailed_description` | Optional | Full description/spec. |
| `supplier_key` | Optional | Match to `04_suppliers`. |
| `quantity` | Yes | Numeric. |
| `currency` | Optional | Source currency. |
| `unit_cost_foreign` | Optional | Unit cost in source currency. |
| `exchange_rate` | Optional | Rate to BHD. |
| `total_cost_bhd` | Optional | Total landed or estimated cost. |
| `suggested_price_bhd` | Optional | Suggested unit selling price. |
| `total_suggested_bhd` | Optional | Suggested line total. |
| `notes` | Optional | Line notes. |

### 09_orders

Targets database table `orders`.

```csv
order_key,order_number,customer_po_number,customer_key,customer_name,order_date,required_date,total_value_bhd,grand_total_bhd,status,payment_terms,delivery_terms,offer_number,opportunity_key,customer_reference,attention_person,attention_company,attention_phone,attention_address,delivery_weeks,country_of_origin,issued_by,contact_phone,discount_percent,division,notes
```

Allowed `status` values are flexible in the database, but suggested values are `Processing`, `Confirmed`, `Partially Delivered`, `Delivered`, `Cancelled`, `Closed`.

### 10_order_items

Targets database table `order_items`.

```csv
order_key,line_number,product_key,product_code,description,quantity,unit_price_bhd,quantity_shipped,quantity_invoiced,equipment,model,specification,detailed_description,currency,fob,freight,total_cost,margin_percent,total_price,notes
```

### 11_customer_invoices

Targets database table `invoices`.

```csv
invoice_key,invoice_number,invoice_date,customer_key,customer_name,order_key,customer_po_number,subtotal_bhd,vat_bhd,vat_percent,grand_total_bhd,outstanding_bhd,status,due_date,offer_number,delivery_note_number,total_supplier_cost_bhd,gross_margin_bhd,gross_margin_percent,customer_reference,attention_person,attention_company,attention_phone,attention_address,delivery_weeks,country_of_origin,issued_by,contact_phone,discount_percent,payment_terms,delivery_terms,division,mode_of_payment,buyers_order_number,buyers_order_date,despatch_document_no,delivery_note_date,despatched_through,destination,place_of_supply,terms_of_delivery,notes
```

Allowed invoice `status` values: `Draft`, `Sent`, `Paid`, `PartiallyPaid`, `Overdue`, `Cancelled`, `Void`, `Proforma`.

### 12_customer_invoice_items

Targets database table `invoice_items`.

```csv
invoice_key,line_number,product_key,product_code,description,quantity,rate,total_bhd,equipment,model,specification,detailed_description,currency,fob,freight,total_cost,margin_percent,total_price,serial_numbers,notes
```

Use `serial_numbers` as a semicolon-separated list if the invoice line contains serialized products.

### 13_customer_payments

Targets database table `payments`.

```csv
payment_key,invoice_number,amount_bhd,payment_date,payment_method,bank_account_key,reference,division,notes
```

Allowed `payment_method` values: `Cash`, `Cheque`, `Bank Transfer`, `Credit Card`, `LC`, `PDC`, `Other`.

### 14_purchase_orders

Targets database table `purchase_orders`.

```csv
po_key,po_number,order_key,opportunity_key,supplier_key,supplier_name,po_date,expected_delivery,currency,exchange_rate,subtotal_foreign,subtotal_bhd,vat_amount,total_foreign,total_bhd,payment_terms,payment_due_date,status,approved_by,approved_at,division,notes
```

Allowed PO `status` values: `Draft`, `Pending Approval`, `Approved`, `Sent`, `Acknowledged`, `Partially Received`, `Received`, `Cancelled`, `Closed`.

### 15_purchase_order_items

Targets database table `purchase_order_items`.

```csv
po_key,line_number,order_key,order_line_number,product_key,product_code,supplier_part_number,description,quantity,unit_price_foreign,unit_price_bhd,total_foreign,total_bhd,quantity_received,notes
```

### 16_goods_received_notes

Targets database table `goods_received_notes`.

```csv
grn_key,grn_number,po_number,received_date,received_by,warehouse_code,supplier_dn_number,qc_status,qc_notes,qc_date,qc_by,notes
```

Allowed `qc_status` values: `Pending`, `Passed`, `Failed`, `Partial`.

### 17_grn_items

Targets database table `grn_items`.

```csv
grn_key,line_number,po_number,po_line_number,product_key,quantity_ordered,quantity_received,quantity_rejected,rejection_reason,serial_numbers,notes
```

`quantity_accepted` is calculated as `quantity_received - quantity_rejected`.

### 18_supplier_invoices

Targets database table `supplier_invoices`.

```csv
supplier_invoice_key,invoice_number,supplier_key,supplier_name,po_number,grn_number,order_key,invoice_date,due_date,currency,exchange_rate,subtotal_foreign,subtotal_bhd,vat_foreign,vat_bhd,total_foreign,total_bhd,match_status,po_match_ok,grn_match_ok,status,approved_by,approved_at,payment_status,payment_date,payment_ref,payment_method,ocr_confidence,division,discrepancy_reason,dispute_reason,notes
```

Allowed `match_status` values: `Pending`, `Matched`, `Discrepancy`, `Dispute`.

Allowed supplier invoice `status` values: `Pending`, `Approved`, `Rejected`, `Paid`, `Dispute`.

### 19_supplier_invoice_items

Targets database table `supplier_invoice_items`.

```csv
supplier_invoice_key,line_number,description,quantity,unit_price,total_price,currency,product_key,notes
```

### 20_supplier_payments

Targets database table `supplier_payments`.

```csv
supplier_payment_key,supplier_invoice_key,invoice_number,supplier_key,supplier_name,amount_foreign,currency,exchange_rate,amount_bhd,payment_date,payment_method,reference,bank_account_key,division,notes
```

Allowed `payment_method` values: `Bank Transfer`, `Cheque`, `LC`, `Cash`, `Wire Transfer`, `PDC`, `Other`.

### 21_delivery_notes

Targets database table `delivery_notes`.

```csv
delivery_note_key,dn_number,order_key,customer_key,delivery_date,delivery_address,contact_person,contact_phone,driver_name,vehicle_number,transport_method,status,signed_by,signed_at,is_partial_delivery,delivery_sequence,total_deliveries,notes
```

Allowed DN `status` values: `Prepared`, `Dispatched`, `InTransit`, `Delivered`, `Signed`, `Cancelled`.

### 22_delivery_note_items

Targets database table `delivery_note_items`.

```csv
delivery_note_key,line_number,order_key,order_line_number,product_key,product_code,description,quantity_ordered,quantity_delivered,quantity_remaining,serial_numbers,notes
```

### 23_serial_numbers

Targets database table `serial_numbers`.

```csv
serial_key,serial_no,product_key,product_code,lot_number,status,po_number,grn_number,dn_number,invoice_number,customer_key,customer_name,received_date,shipped_date,warranty_start_date,warranty_end_date,warranty_months,calibration_date,calibration_due_date,calibration_cert_path,notes
```

Allowed serial `status` values: `Available`, `Reserved`, `Shipped`, `Delivered`.

### 24_bank_statements

Targets database table `bank_statements`.

```csv
statement_key,bank_account_key,statement_number,statement_date,period_start,period_end,opening_balance,closing_balance,currency,total_debits,total_credits,debit_count,credit_count,status,imported_from,import_method,ocr_confidence,balance_verified,discrepancy_amount,division,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `statement_key` | Yes | Import key | Usually same as `statement_number`. |
| `bank_account_key` | Yes | `bank_account_id` via lookup | Use existing bank account ID or account number. |
| `statement_number` | Yes | `statement_number` | Must be unique. |
| `statement_date` | Yes | `statement_date` | Statement issue date. |
| `period_start` | Yes | `period_start` | First date covered. |
| `period_end` | Yes | `period_end` | Last date covered. |
| `opening_balance` | Yes | `opening_balance` | Opening balance. |
| `closing_balance` | Yes | `closing_balance` | Closing balance. |
| `currency` | Yes | `currency` | Statement currency. |
| `total_debits` | Optional | `total_debits` | Importer can calculate from lines. |
| `total_credits` | Optional | `total_credits` | Importer can calculate from lines. |
| `debit_count` | Optional | `debit_count` | Importer can calculate from lines. |
| `credit_count` | Optional | `credit_count` | Importer can calculate from lines. |
| `status` | Yes | `status` | `Imported`, `InProgress`, `Reconciled`, `Verified`, `Cancelled`. |
| `imported_from` | Optional | `imported_from` | Original file name/path. |
| `import_method` | Optional | `import_method` | `CSV`, `PDF_OCR`, `Manual`, `XLS`. |
| `ocr_confidence` | Optional | `ocr_confidence` | 0 to 1 if OCR used. |
| `balance_verified` | Optional | `balance_verified` | `TRUE` if opening + credits - debits equals closing. |
| `discrepancy_amount` | Optional | `discrepancy_amount` | Difference if any. |
| `division` | Yes | `division` | `PH Trading` or `AHS Trading`. |
| `notes` | Optional | `notes` | Statement-level notes. |

### 25_bank_statement_lines

Targets database table `bank_statement_lines`.

```csv
statement_key,line_number,transaction_date,value_date,description,reference,debit,credit,balance,transaction_type,category,sub_category,extracted_customer,extracted_supplier,extracted_invoices,extracted_po_numbers,is_matched,match_type,match_confidence,matched_invoice_numbers,matched_supplier_invoice_numbers,matched_payment_reference,matched_expense_reference,verified_by,verified_at,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `statement_key` | Yes | `bank_statement_id` via lookup | Match to `24_bank_statements`. |
| `line_number` | Yes | `line_number` | Statement row number. |
| `transaction_date` | Yes | `transaction_date` | Posting date. |
| `value_date` | Optional | `value_date` | Defaults to transaction date if blank. |
| `description` | Yes | `description` | Bank narration. |
| `reference` | Optional | `reference` | Bank ref, cheque no, transfer ref. |
| `debit` | Required if money out | `debit` | Non-negative. Only debit or credit should be filled. |
| `credit` | Required if money in | `credit` | Non-negative. Only debit or credit should be filled. |
| `balance` | Optional | `balance` | Running balance after transaction. |
| `transaction_type` | Optional | `transaction_type` | Use values in Bank Reconciliation Match Opportunities. |
| `category` | Optional | `category` | Customer Receipt, Supplier Payment, Bank Fee, Payroll, Transfer, etc. |
| `sub_category` | Optional | `sub_category` | More detail if useful. |
| `extracted_customer` | Optional | `extracted_customer` | Customer name detected in narration. |
| `extracted_supplier` | Optional | `extracted_supplier` | Supplier name detected in narration. |
| `extracted_invoices` | Optional | `extracted_invoices` | JSON list such as `["PH25/091"]`. |
| `extracted_po_numbers` | Optional | `extracted_po_numbers` | JSON list such as `["PO-2026-001"]`. |
| `is_matched` | Optional | `is_matched` | `TRUE` if the client knows the match. |
| `match_type` | Optional | `match_type` | `Manual`, `Split`, `Auto`, or `Unmatched`. |
| `match_confidence` | Optional | `match_confidence` | 0 to 1. |
| `matched_invoice_numbers` | Optional | `matched_invoice_ids` via lookup | Customer invoice numbers, semicolon separated. |
| `matched_supplier_invoice_numbers` | Optional | Allocation lookup | Supplier invoice numbers, semicolon separated. |
| `matched_payment_reference` | Optional | `matched_payment_id` via lookup | Payment reference if already recorded. |
| `matched_expense_reference` | Optional | `matched_expense_id` via lookup | Expense reference if already recorded. |
| `verified_by` | Optional | `verified_by` | Person who verified line. |
| `verified_at` | Optional | `verified_at` | Verification date/time. |
| `notes` | Optional | `notes` | Explain unmatched, split, duplicate, or uncertain items. |

### 26_bank_line_allocations

Targets database table `bank_line_payment_allocations`.

Use this sheet when one bank line pays multiple invoices, or one invoice is partially paid by multiple bank lines.

```csv
statement_key,line_number,allocation_type,entity_reference,allocated_amount,currency,status,notes
```

| Column | Required | Database target | Notes |
|---|---|---|---|
| `statement_key` | Yes | Lookup | Match to bank statement. |
| `line_number` | Yes | `bank_statement_line_id` via lookup | Match to line in `25_bank_statement_lines`. |
| `allocation_type` | Yes | `allocation_type` | `CUSTOMER_INVOICE`, `SUPPLIER_INVOICE`, or `EXPENSE`. |
| `entity_reference` | Yes | Invoice/expense lookup | Invoice number or expense reference. |
| `allocated_amount` | Yes | `allocated_amount` | Must sum to bank line amount for split matches. |
| `currency` | Yes | `currency` | Usually statement currency. |
| `status` | Optional | `status` | `Allocated`, `Verified`, or `Disputed`. |
| `notes` | Optional | Import note | Use for disputes or partial-payment explanation. |

### 27_deposits_in_transit

Targets database table `deposits_in_transit`.

```csv
deposit_key,bank_account_key,deposit_date,amount,currency,deposit_slip_no,description,source_type,customer_key,invoice_numbers,status,cleared_date,matched_statement_key,matched_line_number,notes
```

Allowed `source_type` values: `CUSTOMER_PAYMENT`, `CASH_SALE`, `OTHER`.

Allowed `status` values: `PENDING`, `CLEARED`, `RETURNED`.

### 28_cheque_registers

Targets database table `cheque_registers`.

```csv
cheque_book_key,bank_account_key,cheque_book_no,start_number,end_number,current_number,status,issued_date,exhausted_date,notes
```

Allowed `status` values: `ACTIVE`, `EXHAUSTED`, `CANCELLED`.

### 29_outstanding_cheques

Targets database table `outstanding_cheques`.

```csv
cheque_key,bank_account_key,cheque_number,amount,currency,issued_date,payee_name,payee_type,supplier_key,purpose,status,cleared_date,matched_statement_key,matched_line_number,is_stale,stale_date,reissued_as,notes
```

Allowed `payee_type` values: `SUPPLIER`, `EMPLOYEE`, `OTHER`.

Allowed cheque `status` values: `ISSUED`, `PRESENTED`, `CLEARED`, `STALE`, `CANCELLED`, `BOUNCED`.

### 30_bank_expense_entries

Targets database table `bank_expense_entries`.

```csv
expense_key,statement_key,line_number,division,expense_date,description,category,amount,currency,vat_amount,gl_account_code,is_posted,notes
```

Allowed `category` values: `BANK_FEE`, `SWIFT_CHARGE`, `VAT`, `BG_FEE`, `CORRESPONDENT_CHARGE`, `OTHER`.

### 31_book_bank_reconciliations

Targets database table `book_bank_reconciliations`.

```csv
reconciliation_key,bank_account_key,reconciliation_date,currency,bank_statement_balance,deposits_in_transit,outstanding_cheques,bank_errors,adjusted_bank_balance,book_balance,bank_charges_not_recorded,interest_not_recorded,nsf_cheques,book_errors,adjusted_book_balance,difference,is_reconciled,reconciled_by,reconciled_at,notes
```

Calculation checks:

- `adjusted_bank_balance = bank_statement_balance + deposits_in_transit - outstanding_cheques + bank_errors`
- `adjusted_book_balance = book_balance - bank_charges_not_recorded + interest_not_recorded - nsf_cheques + book_errors`
- `difference = adjusted_bank_balance - adjusted_book_balance`

If the client does not calculate adjusted values, leave them blank and the importer can calculate them.

### 32_fx_rates

Targets database table `fx_rates` or `currency_exchange_rates`, depending on import path.

```csv
rate_key,from_currency,to_currency,rate_date,rate,source,notes
```

| Column | Required | Notes |
|---|---|---|
| `rate_key` | Optional | Stable key if available. |
| `from_currency` | Yes | Example `EUR`. |
| `to_currency` | Yes | Usually `BHD`. |
| `rate_date` | Yes | Rate effective date. |
| `rate` | Yes | Example: if 1 EUR = 0.410 BHD, enter `0.410000`. |
| `source` | Optional | `CBB`, `MANUAL`, `API`, bank statement, etc. |
| `notes` | Optional | Any FX assumptions. |

### 99_recon_questions

Use this sheet for anything uncertain.

```csv
question_key,area,source_reference,issue_type,description,suggested_resolution,priority,answered_by,answer,notes
```

Suggested `area` values: `Customer`, `Supplier`, `Opportunity`, `Invoice`, `Payment`, `Bank Statement`, `Cheque`, `PO`, `GRN`, `Delivery`, `Serial`, `FX`.

Suggested `issue_type` values: `Duplicate`, `Missing`, `Name mismatch`, `Amount mismatch`, `Date mismatch`, `Unmatched bank line`, `Ambiguous allocation`, `Needs client decision`.

## Validation Checklist Before Sending

Please confirm:

- Every sheet has the exact header row from this document.
- Customer and supplier names are legal/current names where possible.
- All bank statement lines have either debit or credit, not both.
- Statement opening balance plus credits minus debits equals closing balance.
- Every payment has a payment date, amount, method, reference, and bank account when known.
- Every bank line that matches an invoice includes the invoice number in `extracted_invoices`, `matched_invoice_numbers`, or `26_bank_line_allocations`.
- Every cheque has a cheque number, amount, issued date, payee, and status.
- Every opportunity has a folder number, year, customer, title, stage, and division.
- Any uncertain item is listed in `99_recon_questions`.

## Tables Not Requested From Client

The following tables are system-owned or generated by AsymmFlow and should not be filled manually unless specifically requested:

- User, role, device, license, and session tables.
- Audit logs and bank reconciliation audit logs.
- Sync records and sync status.
- Statement hashes and file archive hashes.
- Journal entries and journal lines generated by accounting workflows.
- AI conversation, prediction, and Butler intelligence tables.
- Background job, notification, and application settings tables.

