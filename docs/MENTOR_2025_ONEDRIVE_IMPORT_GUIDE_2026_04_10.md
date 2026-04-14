# Mentor 2025 OneDrive Import Guide

This guide is for staging and reviewing `Offers 2025` data before any import is committed.

## What You Need

- The AsymmFlow source snapshot included in the mentor package
- Go installed on your machine
- Your local `Offers 2025` OneDrive folder path

## Safe Workflow

Run these in order from the source snapshot root.

### 1. Generate a staging report only

This scans the 2025 folder, parses costing files, matches customers, and writes audit CSV/JSON files into `reports/`.

```bash
ONEDRIVE_IMPORT_ROOT="/path/to/Offers 2025" \
ONEDRIVE_IMPORT_YEAR=2025 \
go test . -run 'TestManualExportOneDriveSeed$' -v
```

Expected output:

- A folder like `reports/onedrive_seed_2025_YYYYMMDD_HHMMSS`
- `opportunities.csv`
- `costings.csv`
- `line_items.csv`
- `db_audit.csv`
- `summary.json`

### 2. Review the staged data first

Check:

- unresolved customer matches
- duplicate-looking matches
- folders with no commercial data
- obvious title/customer mismatches

Do not import yet if the staging report still looks noisy.

### 3. Commit the import only after review

```bash
ONEDRIVE_IMPORT_ROOT="/path/to/Offers 2025" \
ONEDRIVE_IMPORT_YEAR=2025 \
ONEDRIVE_IMPORT_COMMIT=1 \
go test . -run 'TestManualImportOneDrive$' -v
```

This writes imported opportunities with source `2025_onedrive`.

### 4. Audit what landed in the DB

Replace the report path below with the exact staging report directory produced in step 1.

```bash
ONEDRIVE_IMPORT_YEAR=2025 \
ONEDRIVE_AUDIT_SOURCE_DIR="reports/onedrive_seed_2025_YYYYMMDD_HHMMSS" \
go test . -run 'TestManualAuditOneDriveImport$' -v
```

Expected output:

- A folder like `reports/onedrive_post_import_audit_YYYYMMDD_HHMMSS`
- `anomalies.csv`
- `spot_check_samples.csv`
- `summary.json`

## Important Notes

- The import is now year-aware.
- 2025 imports are tagged as `2025_onedrive`.
- 2026 imports remain tagged as `2026_onedrive`.
- This flow is safer than importing directly because it gives you a staging report first.
- Do not run old legacy date-fix utilities against imported 2025/2026 data.

## Recommended Review Criteria

- Customer matching confidence
- Folder-number normalization
- Stage correctness: `Qualified`, `Quoted`, `Won`
- Grand totals versus parsed line-item totals
- Missing `offer_items` or hollow commercial records

## After This

Once the 2025 OneDrive pass is staged and reviewed, we can reconcile that against Tally imports to produce a cleaner commercial history across 2025 and 2026.
