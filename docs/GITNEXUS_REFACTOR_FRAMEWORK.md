# GitNexus Refactor Framework

**Project**: `ph_holdings`  
**Date**: 2026-04-13  
**Status**: Planning baseline for R&D refactor branch

---

## Purpose

This document defines a repeatable methodology for reducing AI-accelerated codebase bloat without losing behavior, business rules, or delivery momentum.

The goal is not "fewest lines of code." The goal is:

- minimum behavioral surface area
- minimum duplicate logic
- minimum number of write paths per business entity
- minimum blast radius for common changes
- maximum clarity of ownership for workflows, rules, and side effects

In short: **mathematical minimality of moving parts**, not cosmetic minimality.

---

## What We Are Optimizing For

For `ph_holdings`, an optimal state means:

- each business invariant has one authoritative home
- each workflow has one clear orchestrator
- the Wails `App` layer is a boundary, not the business system itself
- database writes for a domain are concentrated and auditable
- validation, numbering, state transitions, and permissions are not reimplemented ad hoc
- critical flows can be traced, explained, and changed with predictable risk

This same definition can be reused for any future repo.

---

## Current Repo Signals

These are not judgments. They are structural signals that tell us where complexity has accumulated.

### Hotspot snapshot

| Signal | Current observation |
|---|---|
| Primary boundary file | `app.go` |
| `app.go` size | `16,610` lines |
| `App` methods in `app.go` | `311` |
| Next largest Go file | `butler_ai.go` at `5,258` lines |
| Other large files | `collaboration_service.go` `2,096`, `database.go` `1,932`, `customer_invoice_service.go` `1,723` |
| Architectural smell | Wails boundary and domain logic are heavily interleaved |
| Risk zones | auth/RBAC, numbering, finance, sync, PDF/export, status transitions |

### Interpretation

This repo does not primarily need "cleanup." It needs **responsibility extraction**.

The main pattern to unwind is:

- UI boundary
- permission check
- validation
- DB access
- transaction control
- business rule enforcement
- document/export logic

all living too close together.

That is the source of convolution.

---

## Refactor Philosophy

### 1. Preserve behavior before improving structure

We do not simplify by rewriting from scratch.  
We simplify by isolating behavior, proving it, then shrinking its surface area.

### 2. Refactor by workflow slices, not by file size alone

A giant file is a signal, not a plan.  
We work on vertical slices such as:

- customer create/update
- costing to offer
- pipeline read/write
- invoice issue/pay/apply

### 3. Invariants come before extraction

Before moving code, record the rules that must remain true:

- RBAC and permission gates
- VAT and money math
- numbering uniqueness
- state machine transitions
- sync consistency
- audit trail expectations
- customer/supplier identity rules

### 4. One seam at a time

Every slice should produce one new seam:

- boundary vs domain
- orchestrator vs policy
- policy vs persistence
- read model vs write model

### 5. Verify reduction, not just change

Every phase should reduce at least one of:

- number of direct callers
- number of DB touchpoints
- number of duplicate validators/rules
- number of cross-domain dependencies
- size of god methods/files

---

## GitNexus-Driven Operating Loop

This is the core reusable method for any repo.

### Step 0: Refresh graph and establish baseline

1. Read:
   - `gitnexus://repo/{name}/context`
   - `gitnexus://repo/{name}/clusters`
   - `gitnexus://repo/{name}/processes`
2. If stale, run:

```bash
npx gitnexus analyze
```

3. Capture local structural signals:
   - biggest files
   - exported/public entrypoints
   - duplicate validators
   - duplicated SQL/write paths
   - current tests for the target flow

### Step 1: Build the invariant ledger

For the selected slice, answer:

- what must never change?
- what values/statuses are legal?
- who is allowed to do this?
- what side effects must happen exactly once?
- what can happen in one transaction?
- what failures must roll back?

If we cannot state the invariant, we are not ready to refactor that slice.

### Step 2: Map the live execution flow

Use GitNexus in this order:

1. `query` to find the workflow
2. `context` on the key symbols
3. `impact` on the likely extraction target
4. `detect_changes` after edits

Questions to answer:

- where does the flow enter from UI/Wails?
- where are permissions enforced?
- where does validation happen?
- where do writes occur?
- where are side effects triggered?
- what other flows share the same helpers?

### Step 3: Score seams

Do not pick the biggest file first. Pick the best seam first.

Score each candidate slice from `1-5`:

| Criterion | Meaning |
|---|---|
| Business importance | How painful is this area in real use? |
| Coupling | How many unrelated responsibilities are packed together? |
| Refactor safety | Can we isolate and verify it without destabilizing the whole app? |
| Reuse potential | Will the pattern we establish here help future slices? |
| Testability | Can we prove behavior before and after? |

Start with the highest total that is **not** in a critical-risk zone.

### Step 4: Refactor a single slice

The implementation order should usually be:

1. define slice boundary
2. extract domain service or orchestrator
3. move invariant checks into one place
4. centralize persistence/write path
5. update callers
6. reduce old entrypoint to a thin adapter

Target shape:

- Wails/App method: input/output adapter + permission boundary
- domain orchestrator: workflow coordination
- policy/validator: business invariants
- repository/store: persistence details
- exporter/integration adapter: external side effects

### Step 5: Verify and measure

After each slice:

- run targeted tests
- run build
- use `detect_changes()`
- inspect impacted processes
- update the ledger and backlog

Every slice should end with a short record:

- what moved
- what stayed
- what became thinner
- what remains entangled

---

## Standard GitNexus Checklist Per Slice

### Before changing code

- `query({ query: "<workflow>" })`
- `context({ name: "<key symbol>" })`
- `impact({ target: "<key symbol>", direction: "upstream" })`
- write the invariant ledger for the slice

### During refactor

- keep edits inside the chosen seam
- avoid opportunistic cleanup outside the seam
- prefer extraction over rewrite
- preserve public behavior first

### After refactor

- `detect_changes({ scope: "all" })`
- run tests/build for affected processes
- compare blast radius before vs after if possible
- document any remaining debt uncovered by the slice

---

## Definition Of "Minimal State"

A slice is closer to minimal when:

- one business action maps to one orchestrator
- one entity has one primary write path
- one rule exists in one place
- one permission decision exists at one boundary
- one state transition mechanism governs the lifecycle

A slice is not minimal when:

- validation is duplicated in UI, App, and service layers with drift
- multiple methods can mutate the same entity with different checks
- transaction boundaries are inconsistent
- naming/numbering logic is copied across flows
- the same workflow is expressed in multiple partial models

---

## Phased Plan For `ph_holdings`

This plan intentionally moves from medium-risk, high-learning slices toward high-risk financial and sync areas.

### Phase 0: Baseline And Safety Rails

Deliverables:

- confirm GitNexus index freshness
- capture hotspot inventory
- capture current builds/tests that pass
- create refactor backlog and slice scorecard

Exit criteria:

- we know the top hotspot files
- we know the first three candidate slices
- we know which zones are temporarily off-limits

### Phase 1: Invariant Ledger

Build a living invariant document for:

- customer creation and update
- costing to offer conversion
- opportunity lifecycle and pipeline reads
- invoice/payment/credit note flows
- sync and numbering systems

Exit criteria:

- each target slice has written business rules before extraction begins

### Phase 2: Pilot Slice A - Customer Create/Update

Why first:

- important to the business
- easier to verify than finance
- already has clear anchors such as `CreateCustomer` and `ValidateCustomerInput`
- likely to expose reusable patterns for validation, ID generation, and Wails boundary slimming

Primary goals:

- make `CreateCustomer` a thin boundary method
- centralize customer validation and code generation
- isolate persistence/write logic
- identify all alternate creation paths such as imports and Butler-generated creation

Exit criteria:

- one authoritative customer creation flow
- imports and AI-assisted creation either reuse it or are deliberately documented exceptions
- blast radius for customer-creation changes is reduced

### Phase 3: Pilot Slice B - Costing To Offer

Why second:

- highly representative of AI-grown complexity
- touches validation, money math, export behavior, and stateful conversion
- already known to have had iterative fixes

Primary goals:

- isolate conversion orchestration from `app.go`
- centralize quote type, VAT, discount, and terms mapping
- reduce PDF/export coupling to data assembly

Exit criteria:

- one conversion pipeline from costing data to offer persistence
- one source of truth for commercial calculations used by this flow

### Phase 4: Pipeline Read/Write Consolidation

Focus:

- canonical opportunity model
- UI read model vs persistence model
- eliminate dual or drifting pipeline representations

Exit criteria:

- clear write path for opportunities
- clear read path for screens and reports
- status semantics documented and enforced in one place

### Phase 5: Shared Cross-Cutting Extraction

After the pilot slices prove the pattern, extract common mechanisms:

- permission boundary helpers
- validation policy modules
- transaction wrappers
- numbering/ID generation
- common query/repository patterns
- export assembly helpers

Exit criteria:

- repeated patterns from earlier slices have a single reusable home

### Phase 6: Medium-Risk Domain Refactors

Candidates:

- delivery note lifecycle
- GRN and serial traceability
- reporting/export services
- Butler-related orchestration where safe

Exit criteria:

- at least two more domains follow the same seam pattern established earlier

### Phase 7: High-Risk Domains Last

Only after earlier phases succeed:

- finance posting flows
- payment reconciliation
- sync and cloud replication
- auth and license edge cases

Exit criteria:

- refactors are supported by proven patterns, tests, and clear invariant records

---

## Recommended First Pilot Order

For this branch, the recommended order is:

1. customer create/update
2. costing to offer
3. pipeline opportunities
4. delivery/serial flows
5. finance/sync

This order maximizes learning while minimizing the chance of breaking the most fragile operational paths too early.

---

## Artifacts To Maintain During The Program

To make this reusable across repos, keep the same artifact set every time:

- `baseline`: hotspot files, top symbols, build/test status
- `invariants`: non-negotiable business rules by slice
- `seams backlog`: ranked candidate slices with scores
- `slice brief`: one pager for the current slice
- `change report`: what changed, what improved, what remains

If this method is reused elsewhere, copy the artifact structure first and then populate it.

---

## Suggested Slice Brief Template

For each refactor slice, record:

### Slice

`<name>`

### Why this slice now

- business reason
- technical reason
- safety reason

### Invariants

- rule 1
- rule 2
- rule 3

### Entry points

- Wails/App methods
- service methods
- UI screens
- import paths

### Writes

- tables mutated
- external side effects

### GitNexus workflow

- `query("<workflow>")`
- `context("<key symbol>")`
- `impact("<target>")`
- `detect_changes()`

### Desired end state

- thinner boundary
- one orchestrator
- one validator/policy home
- one primary write path

### Verification

- tests run
- manual checks
- known residual risks

---

## Guardrails

- do not refactor critical finance/sync flows first
- do not mix multiple slices in one pass
- do not "clean up" without an invariant ledger
- do not widen public API surface while trying to simplify internals
- do not trust graph output blindly; confirm with code and tests
- do not measure success only by LOC reduction

---

## Working Rule For This Branch

For this R&D branch, every phase should answer:

1. what behavior are we preserving?
2. what surface area are we shrinking?
3. what responsibility are we moving to its rightful home?
4. how do we know the blast radius is smaller than before?

If a change does not improve one of those dimensions, it is probably not true simplification.

---

## Immediate Next Move

Begin with **Phase 1 and Phase 2 together**:

- build the invariant ledger for customer creation/update
- map the full customer creation flow with GitNexus
- identify all callers and alternate entrypoints
- extract a plan for making `CreateCustomer` a thin boundary

That slice should become the pattern we reuse for the rest of the codebase.
