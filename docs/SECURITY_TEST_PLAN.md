# Security Test Plan

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document defines the minimum repeatable testing approach for trust-sensitive areas of the product.

## Test Categories

### 1. Build And Basic Health

Minimum checks:

- `go build ./...`
- `npm run build` in `frontend`

Purpose:

- catch broken integration points
- ensure the repo remains releasable at a baseline level

### 2. Permission And Access Checks

Focus:

- backend methods for settings, backup, finance, customer, supplier, and inventory actions
- confirmation that frontend hiding is not the only gate

Examples:

- privileged action denied without required permission
- privileged action allowed with intended role

### 3. Financial Lifecycle Checks

Focus:

- customer invoice and receipt flows
- supplier invoice and payment flows
- bank reconciliation
- procurement and inventory finalization

Examples:

- partial settlement cannot be misrepresented as fully settled
- finalized records cannot be casually edited
- approval paths enforce intended prerequisites

### 4. Backup And Recovery Checks

Focus:

- backup creation
- backup visibility and metadata
- integrity-check path
- restore drill on a non-production copy

### 5. Sync And Configuration Checks

Focus:

- sync disabled by default
- sync only enabled with explicit config
- sync health behavior and failure handling

### 6. Secrets And Dependency Checks

Focus:

- secret scanning
- Go vulnerability review
- static security review
- frontend dependency review where actionable

## Suggested Local Command Set

Baseline local checks:

- `go build ./...`
- `go test ./...`
- `cd frontend && npm run build`

Security-oriented checks as available:

- `govulncheck ./...`
- `gosec ./...`
- `npm audit --omit=dev`

## Evidence Expectations

For major trust-sensitive changes, record:

- command run
- date
- result
- follow-up if any

The evidence log should capture enough detail that another reviewer can understand what was checked.

## Priority Regression Bands For This Repo

These areas deserve repeated targeted attention:

- RBAC and admin-only actions
- customer settlement and receipt application
- supplier settlement and invoice approval flows
- bank reconciliation finalization and matching
- backup and integrity operations
- sync configuration and health behavior

## Current Limit

This test plan defines the minimum working discipline.

It does not yet claim:

- full automated security coverage
- penetration testing
- formal ASVS verification
