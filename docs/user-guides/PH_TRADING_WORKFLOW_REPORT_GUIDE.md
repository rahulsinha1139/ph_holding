# PH Trading WLL Workflow And Report Guide

This guide is the operator-facing map for where commercial work is entered, reviewed, exported, and reconciled in AsymmFlow.

## Sales Flow

1. **RFQ / Tender Received**
   - Use `Opportunities > RFQs` for direct RFQs.
   - Use `Opportunities > Import Tenders` for OneDrive tender folders.
   - Keep the folder/RFQ number as the canonical workflow key. It is the reference used across costing, offers, orders, invoices, delivery notes, and comments.

2. **Opportunity Review**
   - Open the opportunity popup to inspect the customer, reference, commercial status, payment terms, delivery terms, notes, and change log.
   - Use `Edit`, then `Save`, only when the commercial data needs to change. The bottom change log records the edited fields.

3. **Costing**
   - Use `Opportunities > Costing`.
   - Select an opportunity to prefill the costing sheet, or start a blank costing and use `Link Opportunity` later.
   - `Quote Price / Unit (BHD)` is the per-unit customer quote price. The line total remains quantity multiplied by the chosen unit quote price.
   - Attach datasheets from the `Technical Datasheets` section. Large attachments are stored locally under the customer/offer folder and appended during PDF generation.

4. **Offer**
   - Save the costing as an offer.
   - Use `Opportunities > Offers` to search by offer number, customer, RFQ reference, issuer, or status.
   - Use `Extract Offers` when the client needs an Excel-compatible offer register.
   - Use `PDF` to generate the techno-commercial offer with the configured prepared-by/signature details.

5. **Order**
   - Mark an offer as won with the customer PO number.
   - Review the created customer order under `Opportunities > Customer Orders` or `Operations`.
   - Use `Extract Orders` for an Excel-compatible order register.

6. **Delivery And Invoice**
   - Create delivery notes and invoices from the order detail modal.
   - Delivery notes should capture what is physically delivered; invoices should match the commercial order and delivery status.

## Purchase Flow

1. Create supplier POs from customer orders where required.
2. Record GRNs when goods are received.
3. Record supplier invoices against PO/GRN evidence.
4. Supplier settlements and payments are handled from Finance.

## Expense Flow

1. Enter expenses from `Finance > Expenses`.
2. Add payment method and inventory/project/order allocation where relevant.
3. Draft/submitted/approved-unposted unpaid expenses can be deleted by permitted finance users.
4. Posted or paid expenses stay locked and should be corrected through reversal workflows.

## Bank Flow

1. Select the exact bank account before importing a statement.
2. Import PDF or CSV from `Finance > Bank Recon`.
3. The selected account is authoritative. If the OCR/CSV statement identifies a different account, the import is rejected with a clear mismatch message.
4. Match credits to customer invoices, receipts, payments, or split allocations.
5. Match debits to supplier invoices, supplier payments, payroll payouts, or expenses.

## Reports And Extracts

- Screen-level extracts are the fastest operational reports:
  - `RFQs > Extract RFQs`
  - `Opportunities > Extract`
  - `Offers > Extract Offers`
  - `Customer Orders > Extract Orders`
- `Intelligence > Reports` exports Excel-compatible CSV files for finance and management analysis.
- Customer-facing PDFs should be generated from their source workflow, not from the generic Reports screen:
  - Offer PDFs from Offers/Costing
  - Invoice PDFs from Finance invoices
  - Delivery notes from Operations
  - Purchase orders from Operations

## Data Quality Rules

- Do not silently merge customers or suppliers. Use the Data Quality review queue for admin-reviewed cleanup.
- Keep folder/RFQ/reference numbers stable. They are the cross-page key.
- Do not delete posted/paid financial records. Use reversal/correction workflows so audit history remains intact.
