# Buyer Security Overview

Date: 2026-04-14
Product: AsymmFlow

## What This Document Is

This document is a buyer-facing summary of the product's current security and operational controls.

It is intended to answer the practical questions a customer, evaluator, or partner is likely to ask before deployment.

It does not claim formal certification. It documents the current control posture and the roadmap toward more formal assurance.

## Product Overview

AsymmFlow is a desktop ERP application built with:

- Go backend
- Wails desktop runtime
- Svelte frontend
- SQLite as the primary operational datastore
- optional Supabase sync for cloud-backed multi-device scenarios

Core product areas include:

- customer and supplier management
- procurement
- invoicing and payments
- banking and reconciliation
- reporting
- role-based administration

## Deployment Model

The platform is designed to work offline first.

Primary operating mode:

- customer data is stored locally in SQLite on the deployed machine
- the application continues functioning without internet connectivity

Optional extended mode:

- Supabase can be configured for sync and remote backup-style behavior
- sync is an extension of the core local deployment, not a hard dependency for core workflows

This allows customers to choose an operating posture that fits their environment.

## Control Themes

Current security and trust-relevant themes in the product include:

- local-first data ownership
- backend role-based access control
- workflow-level state enforcement across finance and operations flows
- backup and restore support
- optional sync rather than mandatory cloud dependency
- active hardening and refactoring process

## Access Control

The application uses backend permission checks on Wails-bound operations to control access to sensitive actions.

Examples of protected areas include:

- settings and sync administration
- finance views and mutations
- customer and supplier creation paths
- inventory actions
- backup functions

This is intended to reduce reliance on frontend-only enforcement.

## Data Handling Summary

Primary data categories handled by the application include:

- customer master data
- supplier master data
- invoices and payments
- purchase orders and goods receiving
- bank reconciliation and finance summaries
- user and role data

Recommended buyer conversation topics:

- whether the deployment is local-only or sync-enabled
- who controls the host environment
- backup retention expectations
- user and role administration process
- operational recovery expectations

## Backup And Recovery

The product includes application-level backup support for the SQLite primary datastore.

Current posture summary:

- SQLite remains the primary operational database
- backups are created through application-controlled logic
- integrity and retention behavior are part of the intended operational story

Operational proof still being formalized in-repo:

- dated backup drill entries
- restore verification notes
- evidence log references

## AI-Assisted Development Position

AI assistance is used as part of development, but it is not treated as an authority.

The intended operating standard is:

- AI output is treated as draft code
- changes are reviewed by a human
- automated checks are added where practical
- security-sensitive flows receive higher scrutiny

This is being formalized in the repository through policy and evidence documents.

## What We Do Not Claim Yet

At the current stage, we do not claim:

- SOC 2 certification
- ISO 27001 certification
- blanket compliance statements without scope

Recommended wording instead:

- `documented controls`
- `SOC 2-aligned roadmap`
- `security and operational controls are being formalized and evidenced`

## What Buyers Can Request

The following materials are appropriate to share once completed and current:

- security overview
- deployment modes overview
- controls matrix
- AI-assisted development policy
- threat model summary
- evidence log extracts

