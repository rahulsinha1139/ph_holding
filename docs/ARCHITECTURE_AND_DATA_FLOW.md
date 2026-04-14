# Architecture And Data Flow

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document gives a buyer-readable and operator-readable view of how the system is structured and where trust-sensitive control points live.

## High-Level Architecture

The product is a desktop ERP application built with:

- Svelte frontend
- Wails application bridge
- Go backend services
- SQLite as the primary local datastore
- optional Supabase sync for remote coordination

## Main Runtime Layers

### 1. Frontend

The Svelte frontend provides screens, workflows, and client-side guidance.

Important trust note:

- frontend visibility and navigation are helpful UX controls
- frontend visibility is not treated as the final authorization boundary

### 2. Wails Bridge

The frontend talks to the Go backend through Wails-bound application methods.

This layer is important because it is the boundary where user actions become backend operations.

### 3. Go Backend

The Go backend owns:

- authorization checks
- business rules
- finance and procurement lifecycle logic
- backup and integrity operations
- sync orchestration
- document generation and reporting

The backend is the main control plane for trust-sensitive behavior.

### 4. Local SQLite Database

SQLite is the primary operational data store.

It holds:

- customer and supplier master data
- transactional records such as orders, invoices, payments, supplier invoices, and supplier payments
- inventory and procurement state
- bank reconciliation and accounting records
- user, role, settings, and device-related state

### 5. Optional Remote Sync

When enabled, the backend uses Supabase-related configuration to synchronize selected tables between local and remote stores.

The remote side extends availability and device coordination, but the local database remains the primary operating state.

## Data Flow Patterns

### Interactive Workflow Path

Typical business action flow:

1. user takes action in the frontend
2. frontend calls a Wails-bound backend method
3. backend checks permissions and business rules
4. backend writes to SQLite
5. frontend receives the result and updates the UI

### Reporting And Document Path

Typical report or document flow:

1. frontend requests report or export
2. backend validates permissions and input
3. backend reads local data and generates output
4. output is written to a local path or configured document target

### Sync Path

Typical sync flow when enabled:

1. backend loads sync configuration
2. background sync loop waits for startup stabilization
3. backend pushes local changes to remote
4. backend pulls remote changes to local
5. status and timing data are recorded

## Key Control Points

### Authorization

Backend application methods use `requirePermission(...)` to enforce access control on sensitive actions.

This is important because:

- frontend-only hiding is not enough
- finance, settings, inventory, and other actions must be enforced on the backend

### Business Lifecycle Enforcement

Many high-risk workflows now have dedicated policy seams or backend lifecycle checks, especially in:

- customer and supplier settlement flows
- supplier invoice and payment lifecycle
- customer invoice and receipt lifecycle
- bank reconciliation
- procurement and inventory transitions

### Backup And Integrity

The application includes backend-managed backup and database integrity operations rather than relying only on external operator habits.

### Optional Sync Boundary

Sync enablement is configuration-driven and intentionally separate from local operation. This makes it easier to explain and review the local-only trust posture.

## Trust Boundaries

The highest-value trust boundaries are:

- frontend to backend boundary
- backend to local database boundary
- local runtime to backup files boundary
- optional local-to-remote sync boundary
- operator/admin actions affecting settings, data fixes, or recovery

## Architecture Positioning For Buyers

The most accurate short description is:

`Desktop ERP with a Go backend, SQLite-primary local data model, and optional remote sync.`

That description is stronger and more honest than calling it a generic cloud app.

## Evidence Anchors In The Repo

The current architecture story is grounded in:

- `app.go`
- `database.go`
- `db_manager.go`
- `db_sync_service.go`
- frontend permission context and navigation files
- domain service files for finance, operations, and CRM workflows

These files should be the primary evidence base when this document is updated.
