# Threat Model

Date: 2026-04-14
Status: Initial repository-level threat model

## Scope

This threat model focuses on the main trust boundaries and failure modes relevant to the current AsymmFlow product architecture.

## Primary Assets

- customer and supplier master data
- invoices, payments, and settlement state
- purchase orders, GRN, delivery, and inventory records
- bank reconciliation data
- user, role, and settings data
- local SQLite database files
- sync credentials and operational secrets

## Trust Boundaries

### 1. Desktop Client Boundary

The application runs as a desktop app through Wails. Frontend interactions should not be treated as trusted simply because they originate from the shipped UI.

Key concern:

- backend must remain the source of truth for authorization and business-rule enforcement

### 2. Local Database Boundary

SQLite is the primary operational datastore.

Key concerns:

- exposure of local database files
- accidental corruption or incomplete recovery
- unauthorized local host access

### 3. Optional Cloud Sync Boundary

Supabase sync is optional but increases the trust surface.

Key concerns:

- misconfiguration of sync credentials
- unintended data exposure through remote configuration
- drift between local expectations and sync behavior

### 4. Financial Workflow Boundary

Finance and procurement workflows carry high trust sensitivity.

Key concerns:

- unauthorized state transitions
- payment or settlement drift
- approval bypass
- invalid edits after finalization

## Representative Threat Scenarios

| Scenario | Risk | Existing / Intended Control |
| --- | --- | --- |
| Frontend-only restriction bypass | Sensitive action executed without proper permission | backend permission checks on app methods |
| Invalid payment or invoice state mutation | financial records become misleading or fraudulent | backend lifecycle enforcement, payment-policy seams |
| Backup is assumed but not proven | recovery fails during real incident | backup drill evidence and restore verification |
| Sync configured unsafely | remote data leakage or inconsistent sync | explicit deployment mode documentation and config review |
| Secret exposure in repo or logs | unauthorized access to remote services | secret scanning, sanitized logging, env hygiene |
| AI-generated logic introduces unsafe behavior | hidden regressions in sensitive flows | AI policy, human review, targeted tests, CI checks |
| Local machine compromise | SQLite data exposed or tampered with | deployment guidance, least-privilege host expectations, backup controls |

## Highest-Priority Areas For Ongoing Review

- RBAC and privileged operations
- finance status transitions
- supplier and customer payment flows
- bank reconciliation lifecycle
- backup and restore
- optional sync configuration
- secret handling and logging

## Immediate Gaps To Close

- formal buyer-facing security narrative is still being built
- evidence log is new and needs dated entries
- baseline security workflows exist on paper but need usage evidence
- deployment and recovery docs still need expansion

## Review Trigger

Update this document when:

- a major new deployment mode is introduced
- auth or permission architecture changes
- sync behavior changes materially
- banking, finance, or approval workflows change materially
- backup or recovery process changes

