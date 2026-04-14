# Security Policy

This repository contains the AsymmFlow desktop ERP application used for customer, supplier, finance, operations, and reporting workflows.

## Current Security Posture

This project is not currently presented as SOC 2 certified or ISO 27001 certified.

The current position is:

- AI-assisted development is used, but production changes are expected to go through human review and validation.
- SQLite is the primary data store and supports offline operation.
- Supabase sync is optional and treated as an additional deployment mode, not a requirement for basic operation.
- role-based access control is enforced in backend application methods through permission checks
- backup and restore features exist in the application and are part of the operational control story
- security documentation and automated checks are being formalized in-repo

## Reporting A Vulnerability

Until a dedicated security mailbox is established, please report security issues privately to the product operator or maintainer and do not open a public issue containing exploit details.

When reporting, include:

- affected feature or workflow
- steps to reproduce
- impact
- any proof-of-concept data needed to confirm the issue
- whether customer or financial data may be exposed or altered

Please avoid:

- public disclosure before a fix is available
- including live credentials or sensitive customer data in the report

## Response Goals

Target operating goals for internal handling:

- acknowledge receipt of a credible report within 3 business days
- triage severity and affected workflows
- create a fix plan with evidence
- document remediation in the internal evidence log

## Supported Deployment Modes

This application may be deployed in more than one mode:

- local-only / offline-first with SQLite as the primary store
- local-first with optional Supabase background sync

Any security review or buyer-facing conversation should specify which deployment mode is being discussed.

## Security Work In Progress

The trust-signal workstream currently focuses on:

- buyer-facing security documentation
- AI-assisted development policy
- controls matrix and threat model
- baseline CI checks for quality and security scanning
- evidence logging for drills, scans, and hardening work

## Scope Notes

This policy describes the repository posture and reporting path. It does not claim external certification.

