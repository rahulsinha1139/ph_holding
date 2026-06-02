# Deployment Modes

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document explains the supported operating modes of the application so buyers and operators can understand what changes when cloud features are enabled.

## Core Principle

SQLite is the primary operational datastore.

Supabase is optional and extends the deployment model for sync and remote coordination. It is not required for core local operation.

## Mode 1. Local-Only / Offline-First

### Summary

The application runs on a local machine with SQLite as the only active datastore.

### What This Means

- operational workflows continue without internet access
- customer, supplier, finance, inventory, and reporting work stay on the local device
- trust scope is mainly the workstation, local database file, operator permissions, and backup discipline

### Main Benefits

- smallest trust surface
- strongest data-locality story
- easiest mode to explain for buyers who want local control

### Main Responsibilities

- secure the endpoint where the application runs
- protect the local database and backup files
- perform backup and restore drills
- manage local user and device access responsibly

## Mode 2. Local-First With Optional Supabase Sync

### Summary

The application still runs with SQLite as the primary local store, but background sync to Supabase is enabled.

### What This Means

- each device continues to work locally
- sync adds a remote trust boundary for replication and multi-device coordination
- configuration and credential handling become part of the operational security story

### Current Implementation Notes

- cloud sync is disabled by default unless `ENABLE_CLOUD_SYNC=true`
- credentials must also be present before sync is actually enabled
- the backend loads sync configuration from environment variables or `DATABASE_URL`
- push and pull both use the canonical `dbSyncTables` list so the sync scope is defined centrally

### Main Benefits

- multi-device coordination
- remote data durability beyond the local machine
- smoother handoff between operators or devices

### Additional Responsibilities

- protect Supabase credentials and deployment configuration
- review sync health and error conditions
- make sure buyers understand that remote infrastructure is now part of the trust boundary

## Mode 3. New-Device Bootstrap Under Sync

### Summary

When sync is enabled and a device is being provisioned, the remote database can be used to bootstrap a local device state.

### Notes

- this is an operational convenience path, not the primary source-of-truth model
- the local device should still be treated as the active operational node after bootstrap
- any customer-facing deployment conversation should still describe the system as SQLite-primary

## External Integrations

The product may also use external services for specific supporting capabilities such as:

- optional Supabase sync
- document or report storage paths
- AI-assisted workflows
- OCR-style or document-processing helpers

These integrations should be described as feature extensions, not as the core transaction store.

## Buyer Guidance

When discussing deployment with a buyer, state which of these is true:

- local-only
- local-first with optional sync
- local-first with sync plus specific supporting integrations

Do not describe the product as a generic cloud-only SaaS unless that is actually how the specific customer environment is being run.

## Security Positioning By Mode

| Mode | Primary Trust Boundary | Main Risk Focus |
| --- | --- | --- |
| Local-only | workstation and local database | local access, backups, restore readiness |
| Local-first with sync | workstation plus remote sync boundary | credential handling, sync correctness, remote access |
| Bootstrap under sync | provisioning path plus local state | safe initialization, config hygiene, post-bootstrap validation |

## Current Non-Claims

This document does not claim:

- continuous hosted monitoring
- managed DR automation
- formal certification
- cloud-only tenancy isolation guarantees

Those items should only be claimed if separately implemented and evidenced.
