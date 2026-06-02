# Dependency Review Policy

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This document defines how new and existing dependencies should be reviewed so the repo does not accumulate avoidable supply-chain risk.

## Core Rules

- prefer existing dependencies over adding new ones casually
- add a dependency only when it solves a real problem the current stack does not already solve
- prefer mature, maintained packages with clear ownership and adoption
- avoid dependencies that weaken the local-first or desktop trust posture without strong reason

## New Dependency Review

Before adding a new dependency, review:

- what problem it solves
- whether the standard library or existing repo dependencies already solve it
- maintenance health
- security history
- license compatibility
- whether it adds network, storage, or execution risk

## Special Attention Areas

Apply extra scrutiny to dependencies involved in:

- auth or session behavior
- encryption or key handling
- file parsing
- document generation
- database access
- sync or remote connectivity
- shell execution

## Periodic Review

At a practical cadence, review:

- Go module vulnerabilities
- npm audit output for production dependencies
- stale or abandoned packages
- packages that are no longer actually used

## Evidence

Useful review evidence includes:

- `govulncheck` output
- `npm audit` output
- lockfile diffs
- dependency removal notes
- release notes for major upgrades

## Current Tooling Direction

The repo is moving toward a lightweight default set of dependency checks:

- `govulncheck`
- `gosec`
- secret scanning
- frontend dependency audit where the results remain actionable
