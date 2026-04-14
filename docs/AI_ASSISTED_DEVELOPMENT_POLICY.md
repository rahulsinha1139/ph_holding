# AI-Assisted Development Policy

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This policy defines how AI assistance is used in development without weakening accountability, review discipline, or production safety.

## Policy Statement

AI may be used to accelerate drafting, refactoring, exploration, and analysis.

AI output is not treated as trusted production output by default.

All AI-assisted changes are treated as if they were contributed by an untrusted junior engineer:

- they require human judgment
- they may contain logical defects
- they may contain unsafe assumptions
- they may contain security issues
- they must be reviewed in the context of the actual product

## Allowed Uses

AI assistance may be used for:

- code drafting
- refactoring suggestions
- documentation drafting
- test scaffolding
- workflow exploration
- impact analysis support
- repetitive transformation work

## Higher-Scrutiny Areas

The following areas require extra care and should not be accepted on AI output alone:

- authentication and session handling
- authorization and RBAC
- financial calculations and state transitions
- supplier and customer payment flows
- bank reconciliation logic
- backup and restore logic
- sync behavior and remote connectivity
- secret handling
- encryption or key-management logic
- document integrity, approvals, and audit-sensitive paths

## Review Requirements

Before merge or release, AI-assisted changes should be reviewed for:

- correctness in the actual business context
- security and permission impact
- hidden data-flow side effects
- regression risk
- misleading or incomplete tests
- unsafe hardcoded assumptions

## Verification Expectations

AI-assisted changes should be validated using one or more of the following:

- focused tests
- build verification
- targeted security review
- impact analysis
- manual scenario validation
- dependency and secret scanning where applicable

## Documentation Expectations

For trust-sensitive work, the team should prefer explicit documentation over implied memory.

Examples:

- control changes are reflected in the controls matrix
- notable risk decisions are reflected in the threat model
- important drills or fixes are reflected in the evidence log

## External Positioning

Recommended phrasing:

- `AI-assisted development with human review and automated validation`
- `AI output is treated as draft code and reviewed before production use`

Avoid phrasing that implies:

- AI-generated code is inherently safe
- AI can replace human review
- formal certification exists where it does not

