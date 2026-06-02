# Backup And Recovery

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document explains the current backup and recovery posture of the application in practical terms.

## Current Backup Model

The current implementation is application-managed and SQLite-focused.

The core backup path is implemented in the backend and creates a full local database backup using `VACUUM INTO`.

## What The Application Currently Does

Based on the repository implementation:

- backup files are created next to the active database in a `backups/` directory
- backup directory is created with restricted permissions where possible
- backup files are named with timestamps
- backup files are pruned so only the most recent seven are retained
- backup file permissions are reduced to owner read/write where possible
- the database integrity check path uses `PRAGMA integrity_check`
- manual backup and backup info access are permission-gated backend operations

## Permission Model

Current backup-related access:

- backup creation requires `settings:update`
- backup metadata and integrity checks require `settings:view`

This means backup and verification functions are intended to be privileged operations, not general user actions.

## Recovery Position

The repository clearly implements backup creation and integrity checking.

The restore path is currently an operator-guided process rather than a fully productized one-click workflow.

Operationally, the current safe assumption is:

- restore should be tested on a non-production copy first
- a current database snapshot should be preserved before replacement
- post-restore validation should include application startup and integrity verification

## Recommended Recovery Procedure

Until a more guided restore flow exists, the operator procedure should be:

1. stop the application cleanly
2. preserve the current database file before making changes
3. copy the selected backup into place for recovery testing
4. restart the application
5. run integrity validation and key workflow checks
6. document the recovery result in the evidence log

## What Buyers Should Be Told

Accurate current wording:

- the product has an application-managed SQLite backup path
- backup retention is enforced locally
- integrity checks exist
- restore should be treated as an operator-run drill that should be tested and evidenced

Avoid saying:

- fully automated disaster recovery
- continuously validated restore guarantees

unless those are actually added and logged.

## Evidence Expectations

Useful evidence for this area includes:

- screenshot or log of backup creation
- integrity-check result
- restore drill note
- backup directory listing showing retained snapshots
- operator notes on recovery duration and issues

## Next Maturity Steps

- perform and log a non-production restore drill
- document expected RTO and RPO targets per deployment mode
- add a formal restore checklist if operators beyond engineering will perform recovery
