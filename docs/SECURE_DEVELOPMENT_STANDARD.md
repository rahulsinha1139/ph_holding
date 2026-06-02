# Secure Development Standard

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This standard defines the minimum engineering behavior expected for production-facing changes.

## Core Rules

### 1. Backend Is Authoritative

- frontend visibility is never enough for authorization
- trust-sensitive actions must be enforced in backend code
- lifecycle-sensitive state must be validated server-side

### 2. Business Rules Need One Home

- settlement rules should not drift across screens and services
- approval logic should not be duplicated casually
- identifier and canonicalization rules should be centralized where practical

### 3. AI Output Is Draft Code

- AI-assisted code is reviewed as untrusted draft output
- high-risk domains need extra scrutiny
- tests and control review are required before relying on AI-generated logic

### 4. Sensitive Changes Need Evidence

Changes touching the following areas need explicit validation:

- auth and RBAC
- invoices, payments, settlements
- bank reconciliation
- procurement and inventory finalization
- sync and backup logic
- secrets, logging, and configuration handling

### 5. No Hidden Permission Bypasses

- temporary bypasses must be narrow, documented, and time-bounded
- startup-only exceptions must not quietly become general runtime exceptions

### 6. Finalized States Need Guardrails

- paid, settled, verified, reconciled, posted, or approved states should not be manually forced unless a defined corrective workflow exists

## Minimum Review Expectations

For a normal production change:

- understand the actual execution path
- review auth and business-rule impact
- run targeted verification
- update docs if control behavior changed

For a high-risk change:

- review the blast radius
- validate permission and state-transition behavior explicitly
- add or update focused tests
- record evidence if the change materially affects trust posture

## Coding Expectations

- avoid frontend-only trust decisions
- avoid hardcoded secrets or credentials
- avoid silent fallback logic that changes security posture
- prefer narrow helpers or policy seams over repeated inline rules
- prefer explicit validation over implied UI behavior

## Documentation Expectations

Update the relevant repo artifacts when appropriate:

- `docs/CONTROLS_MATRIX.md`
- `docs/THREAT_MODEL.md`
- `docs/EVIDENCE_LOG.md`
- buyer-facing trust docs when deployment or control posture changes

## Release Gate Mindset

The question for release is not just:

- does it work

It is also:

- is the control behavior still true
- can we explain it clearly
- can we show evidence if asked
