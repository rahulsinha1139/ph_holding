# Trust Signal Spec

Date: 2026-04-13
Project: AsymmFlow / `ph_holdings`
Status: Planning spec for implementation starting next session

## Purpose

This spec defines the lowest-cost, highest-credibility trust layer we can add to the product before paying for formal external audits.

The goal is not to claim certification we do not yet have. The goal is to make the product visibly governed, reviewable, and buyer-legible.

This spec is designed for:

- `ph_holdings`
- future ERP/CRM repos built with heavy AI assistance
- solo or very small teams who need credible diligence before they can afford SOC 2 / ISO work

## Core Positioning

We are not trying to prove that AI was never used.

We are trying to prove:

- AI-assisted code is treated as untrusted draft output
- production code is reviewed, tested, and scanned
- security-sensitive workflows are documented
- buyer-visible operational controls exist
- the product has a clear path toward formal audits later

Recommended external phrasing:

- `AI-assisted development with documented human review and automated security gates`
- `SOC 2-aligned controls roadmap`
- `Security and operations controls documented in-product and in-repo`

Do not claim:

- `SOC 2 certified`
- `ISO 27001 certified`
- `compliant` unless a formal basis exists

## Why This Matters

For ERP/CRM buyers, trust is not abstract. They care about:

- customer and financial data access
- approval and segregation-of-duties logic
- backup and recovery
- auditability
- data residency / deployment control
- vulnerability handling
- operational stability

Because the product touches finance, procurement, invoicing, supplier data, banking, and customer records, buyers will look for external markers of seriousness even if they do not understand the code.

This spec creates those markers through evidence, process, and repeatable artifacts.

## Deliverables

The work is split into five deliverable groups.

### 1. Public Trust Docs

These are buyer-facing or buyer-readable artifacts.

Files to create:

- `SECURITY.md`
- `docs/BUYER_SECURITY_OVERVIEW.md`
- `docs/ARCHITECTURE_AND_DATA_FLOW.md`
- `docs/DEPLOYMENT_MODES.md`
- `docs/BACKUP_AND_RECOVERY.md`
- `docs/RBAC_AND_APPROVALS.md`
- `docs/VULNERABILITY_DISCLOSURE.md`

Purpose:

- explain how the system is secured
- explain local-first and cloud-sync deployment modes
- explain who can access what
- explain backup, restore, and failure recovery
- explain how security issues are reported and handled

Acceptance criteria:

- a non-technical buyer can understand the product’s control posture
- a technical reviewer can see concrete architecture and security decisions
- no false certification claims appear

### 2. Internal Governance Docs

These are operator-facing artifacts that prove discipline.

Files to create:

- `docs/CONTROLS_MATRIX.md`
- `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md`
- `docs/SECURE_DEVELOPMENT_STANDARD.md`
- `docs/RELEASE_CHECKLIST.md`
- `docs/INCIDENT_RESPONSE_RUNBOOK.md`
- `docs/ACCESS_REVIEW_TEMPLATE.md`
- `docs/DEPENDENCY_REVIEW_POLICY.md`

Purpose:

- define what controls exist
- define how AI-generated code is handled
- define release and review standards
- define incident handling
- define access review and dependency hygiene expectations

Acceptance criteria:

- each control has an owner, evidence source, and review cadence
- AI policy explicitly says AI output is reviewed like untrusted input
- release checklist is short enough to be actually used

### 3. Threat and Verification Layer

These artifacts turn intuition into evidence.

Files to create:

- `docs/THREAT_MODEL.md`
- `docs/OWASP_ASVS_SELF_REVIEW.md`
- `docs/SECURITY_TEST_PLAN.md`
- `docs/EVIDENCE_LOG.md`

Purpose:

- identify major risks in this product specifically
- self-assess against an industry-recognized verification framework
- define how security testing is done
- log dated evidence of controls and drills

Priority threat areas for this repo:

- RBAC bypass
- unsafe financial state transitions
- supplier/customer settlement drift
- document and report integrity
- cloud sync misconfiguration
- secret exposure
- backup/restore failure
- local database mishandling on customer machines
- audit trail gaps

Acceptance criteria:

- threat model maps to real product surfaces
- ASVS review is pass/fail/evidence based
- evidence log entries are dated and reproducible

### 4. Automated Security and Quality Gates

These are the repo-level signals buyers and technical evaluators trust.

Initial automation targets:

- Go linting
- Go vulnerability scan
- Go security scan
- frontend dependency audit
- secret scanning
- basic PR or branch validation workflow

Implementation targets:

- `.github/workflows/security.yml`
- `.github/workflows/quality.yml`
- repo configuration for secret scanning and dependency review where available

Recommended tools:

- `golangci-lint`
- `govulncheck`
- `gosec`
- `gitleaks`
- `npm audit` or equivalent lightweight dependency check

Nice-to-have later:

- `semgrep`
- `trivy`
- DAST against a deployed test target

Acceptance criteria:

- scans can be run locally and in CI
- failures are understandable, not decorative
- workflow output can be used as buyer evidence

### 5. Operational Proof Artifacts

These are lightweight, repeatable pieces of proof.

Files or outputs to maintain:

- backup restore drill note
- access review log
- dependency scan snapshot
- hardening changelog
- release approval record
- incident drill note

Format can be markdown in `docs/` if needed.

Acceptance criteria:

- every artifact has a date
- every artifact has an owner
- every artifact references actual evidence, not just intention

## Scope For This Repo

This repo has unusually strong trust-signaling raw material already. The documentation layer should reflect that reality.

Signals already present in the system:

- offline-first SQLite primary
- optional Supabase sync
- RBAC throughout Wails-bound operations
- financial workflow states
- customer and supplier lifecycle handling
- backup and restore mechanisms
- audit-sensitive flows such as invoices, payments, PO, GRN, DN, and bank reconciliation
- explicit refactoring and hardening history

That means our task is not to invent trust. It is to package and evidence it.

## Sequence Of Work

### Phase 1. Buyer-Facing Security Narrative

Create:

- `SECURITY.md`
- `docs/BUYER_SECURITY_OVERVIEW.md`
- `docs/DEPLOYMENT_MODES.md`
- `docs/ARCHITECTURE_AND_DATA_FLOW.md`

Outcome:

- we can send a buyer a clean security packet
- we can explain local-only vs sync-enabled deployments
- we can explain data flow without hand-waving

### Phase 2. Governance Backbone

Create:

- `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md`
- `docs/CONTROLS_MATRIX.md`
- `docs/SECURE_DEVELOPMENT_STANDARD.md`
- `docs/RELEASE_CHECKLIST.md`

Outcome:

- internal operating method becomes explicit
- we can truthfully say AI output is reviewed and governed
- change control stops being implicit memory

### Phase 3. Threat And Verification

Create:

- `docs/THREAT_MODEL.md`
- `docs/OWASP_ASVS_SELF_REVIEW.md`
- `docs/SECURITY_TEST_PLAN.md`

Outcome:

- the security story becomes testable
- buyer confidence moves from marketing copy to verification

### Phase 4. Automation Layer

Create:

- `.github/workflows/security.yml`
- `.github/workflows/quality.yml`

Add runnable commands for:

- `golangci-lint`
- `govulncheck`
- `gosec`
- `gitleaks`
- frontend dependency audit

Outcome:

- every meaningful change can produce evidence
- diligence becomes visible and repeatable

### Phase 5. Evidence Discipline

Create:

- `docs/EVIDENCE_LOG.md`
- initial drill entries and scan notes

Outcome:

- we begin accumulating audit-grade habits before paying for audits

## Working Rules

The following rules should guide implementation.

### Rule 1. No fake confidence

We document what exists, what is partially implemented, and what is planned next.

### Rule 2. Evidence beats adjectives

Prefer:

- `daily backup job documented`
- `RBAC required on finance actions`
- `CI runs gosec and govulncheck`

Over:

- `enterprise-grade`
- `military-grade`
- `fully secure`

### Rule 3. Keep artifacts lightweight

If a control document becomes too long to maintain, it fails.

Each artifact should be concise, dated, and tied to evidence.

### Rule 4. Align with recognizable standards

Use the language of:

- SOC 2-aligned controls
- ISO 27001-style governance
- OWASP ASVS verification
- NIST-style risk thinking

Without overstating formal status.

### Rule 5. AI is part of the process, not hidden

The policy stance should be:

- AI can accelerate implementation
- AI does not replace review, testing, validation, or accountability

## Suggested Repo Structure

Recommended additions:

```text
SECURITY.md
docs/
  BUYER_SECURITY_OVERVIEW.md
  ARCHITECTURE_AND_DATA_FLOW.md
  DEPLOYMENT_MODES.md
  BACKUP_AND_RECOVERY.md
  RBAC_AND_APPROVALS.md
  VULNERABILITY_DISCLOSURE.md
  CONTROLS_MATRIX.md
  AI_ASSISTED_DEVELOPMENT_POLICY.md
  SECURE_DEVELOPMENT_STANDARD.md
  RELEASE_CHECKLIST.md
  INCIDENT_RESPONSE_RUNBOOK.md
  ACCESS_REVIEW_TEMPLATE.md
  DEPENDENCY_REVIEW_POLICY.md
  THREAT_MODEL.md
  OWASP_ASVS_SELF_REVIEW.md
  SECURITY_TEST_PLAN.md
  EVIDENCE_LOG.md
.github/
  workflows/
    security.yml
    quality.yml
```

## Tomorrow's Build Plan

If we implement this tomorrow, the recommended order is:

1. `SECURITY.md`
2. `docs/BUYER_SECURITY_OVERVIEW.md`
3. `docs/AI_ASSISTED_DEVELOPMENT_POLICY.md`
4. `docs/CONTROLS_MATRIX.md`
5. `docs/THREAT_MODEL.md`
6. `.github/workflows/security.yml`
7. `.github/workflows/quality.yml`
8. `docs/EVIDENCE_LOG.md`

This order creates the highest visible credibility first, then backs it with process and automation.

## Definition Of Done

This trust-signal layer is considered meaningfully established when:

- buyer-facing security docs exist and are readable
- AI-assisted development policy is written and accurate
- controls matrix exists with owners and evidence references
- at least one threat model and one ASVS-style self-review exist
- CI runs basic security and quality checks
- an evidence log begins accumulating dated proof
- the repo can truthfully say it has documented security and development controls even without formal certification

## Longer-Term Path

After this foundation exists and paying pilots are active, the next maturity path is:

1. tighten evidence collection and review cadence
2. formalize access reviews and incident drills
3. expand CI scanning and dependency control
4. align evidence to SOC 2 trust criteria
5. pursue external audit when budget allows

## Notes For Reuse Across Future Repos

This framework is intentionally reusable.

For any future repo:

- replace product-specific architecture and threat model sections
- keep the AI policy skeleton
- keep the controls matrix format
- keep the evidence log habit
- keep the CI security gate pattern

The methodology should scale even if the exact stack changes.

