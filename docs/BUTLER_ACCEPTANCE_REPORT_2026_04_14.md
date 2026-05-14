# Butler Acceptance Report

- Date: 2026-04-14T11:12:27+05:30
- Source DB: `/Users/rahulsinha/Desktop/House_of_Projects/ph_holdings/ph_holdings.db`
- Working copy: `/var/folders/q3/gmsc2s_j6vx3frjg1zxptw3m0000gn/T/TestManualButlerBusinessAcceptance482357640/001/butler_acceptance.db`

## Results

- Overall result: 10/10 prompts passed through the grounded Butler pipeline against a temp copy of the live `ph_holdings.db`.
- Important nuance: several prompts returned factual "no records found" answers for the current live data snapshot, which is a correct grounded outcome and not a Butler failure.
- Positive live-data hits in this pass included Peri notifications, Endress+Hauser supplier payment history, Endress+Hauser purchase history, and revenue projection.

### peri_tasks

- Prompt: How many tasks are assigned to Peri right now?
- Status: PASS
- Response excerpt: Peri currently has 0 active task(s), 0 blocked task(s), and 0 overdue task(s). I also found 1 unread notification(s).

### peri_notifications

- Prompt: What notifications does Peri have?
- Status: PASS
- Response excerpt: Peri currently has 0 active task(s), 0 blocked task(s), and 0 overdue task(s). I also found 1 unread notification(s).  Recent notifications: - Task reassigned | unread | 2026-04-08

### bapco_notes

- Prompt: What notes do we have for BAPCO?
- Status: PASS
- Response excerpt: I checked notes for BAPCO group and found no recorded customer notes.

### bapco_invoices_quarter

- Prompt: Show me BAPCO invoices this quarter
- Status: PASS
- Response excerpt: I checked Q2 2026 invoices for BAPCO group and found no recorded invoices in Q2 2026.  Account handlers on record: none found in current records.

### bapco_line_items

- Prompt: What have we sold to BAPCO?
- Status: PASS
- Response excerpt: I checked sold line items for BAPCO group but found no invoice line-item records in current data.

### bapco_offers_year

- Prompt: Show me BAPCO offers this year
- Status: PASS
- Response excerpt: I checked offers for BAPCO group in 2026 and found no recorded offers.  Account handlers on record: none found in current records.

### endress_payment_history

- Prompt: Tell me about Endress+Hauser payment history
- Status: PASS
- Response excerpt: Here is the supplier payment history for Endress+Hauser.  Recorded supplier invoices: 76 totaling 450195.883 BHD Pending supplier invoices: 22 totaling 207249.692 BHD Recent supplier payments totaled 192735.780 BHD in th...

### endress_purchase_history

- Prompt: What did we buy from Endress+Hauser?
- Status: PASS
- Response excerpt: Here is what we have bought from Endress+Hauser.  Recorded supplier invoices: 12 totaling 60633.978 BHD  Recent supplier invoices: - 6017272999 | 2024-12-11 | 10919.784 BHD | Paid - 163-22-DUP-3 | 2024-11-12 | 61.050 BHD...

### endress_issue_history

- Prompt: Are there any active supplier issues for Endress+Hauser?
- Status: PASS
- Response excerpt: I checked supplier issues for Endress+Hauser and found no active issue records.

### revenue_projection

- Prompt: Give me the revenue projection
- Status: PASS
- Response excerpt: Latest revenue projection (grounded from current records):  - Last full year (2025) revenue: 760154.809 BHD - Current year (2026) YTD billed: 0.000 BHD - Active offer pipeline: 1569374.338 BHD  Projection scenarios: - Ru...
