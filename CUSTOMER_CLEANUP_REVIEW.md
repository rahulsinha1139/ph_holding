# AsymmFlow — Customer Database Cleanup Review

**Date:** 2026-03-01
**For:** PH Trading Team
**Action Required:** Review and mark items for deletion/merge

---

## Summary

| Category | Count |
|----------|-------|
| Active customers (with invoices) | 49 |
| Pipeline (orders/offers, no invoices) | 14 |
| Prospects (contacts only) | 174 |
| Empty records (no data at all) | 49 |
| **Total active records** | **286** |

We already merged 62 obvious duplicates (e.g., "Alba" + "Aluminium Bahrain B.S.C" + "ALUMINIUM BAHRAIN B.S.C.(C)" → single record). Below are items that need your confirmation.

---

## SECTION A: Empty Records — Safe to Delete? (49 records)

These customers have **zero invoices, zero orders, zero offers, and zero contacts**. They appear to be leftover entries from imports or tests.

**Please mark each: DELETE or KEEP**

| # | Customer Name | Action |
|---|--------------|--------|
| 1 | AL HILLO TRADING CO. W.L.L | |
| 2 | AL HOSN TRADING | |
| 3 | AL THAWADI | |
| 4 | ALMOAYYED INDUSTRIAL CONTRACTING & TRADING B.S.C. CLOSED | |
| 5 | Al Alawi | |
| 6 | Al Bakali Geneal Trading B.S.C. | |
| 7 | Al Hilal Group | |
| 8 | Al Hoty Analytical Services | |
| 9 | Al Manhal Water Factory | |
| 10 | Al Mufeed Electrical Services W.L.L | |
| 11 | Al Rawaby trading CO | |
| 12 | Al Sane & Partners Soft drinks | |
| 13 | Al Zamih trading & Industrial Supplies | |
| 14 | Almoayyed Air Conditioning W.L.L | |
| 15 | Arab. Sugar Co. B.S.C | |
| 16 | Awal Dairy - Tetrapack | |
| 17 | BAHRAIN STEEL BSC C (EC) | |
| 18 | BAPCO GAS COMPANY B.S.C CLOSED | |
| 19 | BAPCO GAS EXPANSION COMPANY | |
| 20 | BAPCO REFINING B.S.C CLOSED | |
| 21 | BASF PLASTIC ADDITIVES MIDDLE EAST | |
| 22 | Bahrain District Cooling Company B.S.C (CLOSED) | |
| 23 | Bahrain District Cooling Company W.L.L | |
| 24 | Bahrain LNG W.L.L | |
| 25 | Bahrain National Gas Expansion Company W.L.L | |
| 26 | Benchmark Test Customer | |
| 27 | CPIC fiber glass | |
| 28 | CROWN PLAZA | |
| 29 | DELTA ELECTRICAL ITEMS WLL | |
| 30 | Delta Bahrain | |
| 31 | GULF ALUMINIUM ROLLING MILL | |
| 32 | GULF SNIPE SERVICES W.L.L | |
| 33 | Green star AC contracting | |
| 34 | Gulf Biotech | |
| 35 | Gulf Market International | |
| 36 | Gulf Temperature Sensors WLL | |
| 37 | Jalal Ionics W.L.L. - Aqua Cool | |
| 38 | Nass Electrical Contracting | |
| 39 | Nass Landscape | |
| 40 | Nass Mechnical Contr. Co. | |
| 41 | SMART WATER PROJECTS WLL | |
| 42 | SUEZ WATER TECHNOLOGIES & SOLUTIONS BAHRAIN W.L.L | |
| 43 | The CocaCola Bottling Company | |
| 44 | The Future Shop WLL | |
| 45 | The Gulf Projects and Tenders | |
| 46 | Veolia Energy Gulf WLL | |
| 47 | Worley Parsons | |
| 48 | Yateem Oxygen | |
| 49 | al eradah chemical bahrain | |

---

## SECTION B: Possible Duplicates — Same Company or Different? (8 groups)

These look like they might be the same company but we weren't sure. Please confirm.

**Mark each: MERGE (combine into one) or KEEP SEPARATE**

### B1: BAPCO Group (6 records)
Are these all the same entity or separate BAPCO subsidiaries?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| BAPCO UPSTREAM W.L.L | 25 | 14 | 0 | 14 |
| The Bahrain Petroleum Co. (Bapco) | 0 | 0 | 0 | 52 |
| Bapco Energies | 0 | 2 | 6 | 0 |
| BAPCO GAS COMPANY B.S.C CLOSED *(empty)* | 0 | 0 | 0 | 0 |
| BAPCO GAS EXPANSION COMPANY *(empty)* | 0 | 0 | 0 | 0 |
| BAPCO REFINING B.S.C CLOSED *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B2: Nass Group (4 records)
Same parent company or separate entities?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| Nass Corporation | 0 | 0 | 0 | 5 |
| Nass Electrical Contracting *(empty)* | 0 | 0 | 0 | 0 |
| Nass Landscape *(empty)* | 0 | 0 | 0 | 0 |
| Nass Mechnical Contr. Co. *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B3: CPIC / Mannai (4 records)
Same entity or different?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| CPIC Abahsain Fiberglass WLL | 0 | 0 | 0 | 2 |
| CPIC/Mannai Technical Services | 0 | 0 | 0 | 4 |
| Mannai Technical Services/CPIC | 0 | 0 | 0 | 2 |
| CPIC fiber glass *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B4: Worley (2 records)
Same company (renamed)?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| WORLEY SOLUTIONS BAHRAIN W.L.L | 3 | 0 | 0 | 3 |
| Worley Parsons *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B5: Awal Dairy (2 records)

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| AWAL DAIRY COMPANY W.L.L. | 2 | 2 | 0 | 1 |
| Awal Dairy - Tetrapack *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B6: Yateem (2 records)
Same company or parent vs subsidiary?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| Yateem Group | 0 | 1 | 4 | 1 |
| Yateem Oxygen *(empty)* | 0 | 0 | 0 | 0 |

**Decision:** ______________________

### B7: MCSC / MCSIX (2 records)
Same entity?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| MCSC | 0 | 0 | 0 | 3 |
| MCSIX | 0 | 0 | 0 | 1 |

**Decision:** ______________________

### B8: Al Faris (2 records)
Same company or different business lines?

| Customer Name | Invoices | Orders | Offers | Contacts |
|--------------|----------|--------|--------|----------|
| AL-FARIS Industrial Supplies W.L.L | 0 | 0 | 0 | 1 |
| Al Faris Equipment Rental | 0 | 0 | 1 | 0 |

**Decision:** ______________________

---

## SECTION C: Name Corrections Needed

Some active customer names have typos or informal names. Please confirm the correct legal names:

| Current Name | Suggested Correction |
|-------------|---------------------|
| AHS TRADING W.L.L CUSTOMER | AHS TRADING W.L.L |
| Accelerated Services for Instrument and Automation | *(confirm full legal name)* |
| al eradah chemical bahrain | *(capitalize? full name?)* |
| Techno Checm Trading | Techno Chem Trading? |
| Nass Mechnical Contr. Co. | Nass Mechanical Contr. Co.? |
| National Concrete Complay WLL | National Concrete Company WLL? |
| Al Bakali Geneal Trading B.S.C. | Al Bakali General Trading B.S.C.? |
| InternationalAgencies Co. Ltd | International Agencies Co. Ltd? |
| Crown Elctro-Mechanical Services | Crown Electro-Mechanical Services? |
| Elec mechanical equipments sys co. | *(confirm full legal name)* |
| P H Trading W.L.L | *(this is you — keep as client record?)* |

---

## How to Return This

1. Fill in the "Action" column for Section A (DELETE or KEEP)
2. Fill in decisions for Section B (MERGE or KEEP SEPARATE)
3. Confirm corrections in Section C
4. Send back to Asymmetrica — we'll apply all changes in one batch

---

*Generated by AsymmFlow Database Maintenance*
