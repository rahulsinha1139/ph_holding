# Release Checklist

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This checklist is intentionally short enough to use.

## Pre-Release

- confirm the target branch and release intent
- review `git status` and ensure no accidental secrets or stray files are included
- confirm any trust-sensitive changes have corresponding docs or evidence updates

## Build And Verification

- backend build passes
- frontend build passes
- targeted tests for changed trust-sensitive areas pass
- any required scans are run or scheduled

## Security And Operations

- confirm no new credentials or tokens were committed
- confirm deployment mode assumptions are still accurate
- confirm backup and recovery implications of the release are understood
- confirm auth, settlement, or finalization behavior was not weakened by the change

## Data And Workflow Safety

- confirm migrations or schema-affecting changes are understood
- confirm any new lifecycle transitions are intentional
- confirm any admin-only or settings-level actions are still permission-gated

## Evidence

- add a concise evidence-log entry for major hardening, scan, drill, or control changes
- capture any follow-up items that should not be forgotten after release

## Sign-Off

- release owner reviewed
- known risks are documented
- unresolved issues are either accepted explicitly or deferred intentionally
