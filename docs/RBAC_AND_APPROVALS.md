# RBAC And Approvals

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document explains how access control and approval-sensitive operations are intended to work in the current product.

## Core Access-Control Principle

The backend is the source of truth for authorization.

Frontend navigation and button visibility help the user experience, but sensitive operations are expected to be enforced in backend methods through permission checks.

## Current Access Model

The repository currently supports:

- user-based role and permission checks
- license-based role and permission checks
- merged permission awareness for the active session in some flows

This matters because the application is used as a desktop product and may operate in environments where license activation is part of the runtime identity model.

## Role And Permission Storage

At the data-model level:

- users exist as application records
- roles carry permission lists
- device-to-user relationships exist for device-linked operation

At the enforcement level:

- backend methods call `requirePermission(...)`
- wildcard permissions are supported for top-level admin-style roles
- permission aliases are normalized for some equivalent action names

## Important Trust Note

Frontend permission filtering is not a substitute for backend authorization.

The repo already reflects this principle:

- sidebar visibility is permission-filtered for UX
- backend methods still check permissions before performing sensitive actions

## Examples Of Privileged Areas

The current codebase uses backend permission checks in areas such as:

- settings and tool visibility
- backup creation and integrity verification
- customer and supplier create/update flows
- finance reporting and accounting actions
- inventory adjustments and stock movement
- procurement and operational transactions
- audit-log and administrative data access

## Approval-Sensitive Workflows

In addition to access control, some workflows rely on lifecycle controls so important records cannot be arbitrarily forced between states.

Important examples include:

- supplier invoice settlement and approval sequencing
- customer invoice settlement and receipt application
- bank reconciliation finalization and reopen behavior
- procurement and inventory status transitions
- journal posting and reversal-style finance actions

## Operating Rules

When reviewing or changing trust-sensitive code, the working rules should be:

- do not rely on frontend-only gating
- do not allow manual forcing of ledger-derived settlement states
- do not allow edits to finalized workflows without an explicit reopen or corrective path
- keep permission checks near the backend operation boundary
- document exceptions and temporary bypasses explicitly

## What Buyers Should Understand

The clearest buyer-facing message is:

`The application enforces role-based access in backend operations, not just in the UI, and approval-sensitive finance workflows use controlled lifecycle rules.`

That statement is credible, specific, and consistent with the current repo direction.

## Evidence Anchors

Current evidence sources for this area include:

- `app.go`
- `database.go`
- frontend auth-context and sidebar filtering code
- finance, payment, procurement, and reconciliation service files

## Next Maturity Steps

- define a lightweight recurring access review rhythm
- capture one real access review using the template in this repo
- identify the smallest set of admin-only operations that should be called out in buyer conversations
