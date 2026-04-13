# Customer Creation Slice Brief

**Project**: `ph_holdings`  
**Date**: 2026-04-13  
**Phase**: Phase 1 invariant ledger + Phase 2 pilot slice planning  
**Status**: Ready for implementation planning

---

## Slice

`customer create/update`

This slice covers:

- manual customer creation from the main customer screen
- lightweight creation from the CRM dashboard modal
- Butler-assisted customer creation
- backend customer create/update behavior
- alternate customer creation paths that currently bypass the main flow

This slice does **not** yet refactor:

- full seed/import architecture
- customer contacts lifecycle
- downstream invoice/order/customer analytics

Those are noted where relevant but remain out of scope for the first pass.

---

## Why This Slice First

This is the right pilot slice because it is:

- business-relevant and easy to reason about
- less dangerous than finance or sync
- broad enough to expose real architectural drift
- narrow enough to refactor without destabilizing the whole ERP

It also surfaces a core anti-pattern that likely exists elsewhere in the repo:

- one entity
- multiple write paths
- multiple identifier strategies
- partial duplication of validation/defaulting rules

If we solve that pattern here, we get a reusable template for later slices.

---

## Current Flow Map

### Frontend entrypoints

| Path | Current behavior |
|---|---|
| `frontend/src/lib/screens/CustomersScreen.svelte:112-188` | Main UI create flow. Pre-generates `customer_id` in the browser, assembles a `CustomerMaster`, calls `CreateCustomer`. |
| `frontend/src/lib/screens/CRMCustomerDashboard.svelte:56-67` | Dashboard modal create flow. Calls `CreateCustomer` with a lighter payload. |
| `frontend/src/lib/screens/ButlerScreen.svelte:1421-1442` | Butler action creates a request payload, then calls `CreateCustomerFromButler`. |
| `frontend/wailsjs/go/main/App.js:397-406` | Generated Wails bindings for `CreateCustomer` and `CreateCustomerFromButler`. |

### Backend entrypoints

| Path | Current behavior |
|---|---|
| `app.go:7995-8046` | `CreateCustomer` checks permission, validates input, requires `BusinessName`, auto-generates `CustomerCode`, defaults `CustomerID` from `CustomerCode`, then inserts. |
| `app.go:15212-15262` | `CreateCustomerFromButler` normalizes Butler data, defaults `Country` and `PaymentGrade`, then delegates to `CreateCustomer`. |
| `app.go:8362-8389` | `UpdateCustomer` checks permission and saves after preserving audit fields, but does not reuse the same validation path as `CreateCustomer`. |

### Alternate write paths

| Path | Current behavior |
|---|---|
| `import_2026_data.go:289-314` | `findOrCreateCustomer` directly creates a customer if lookup fails. Uses its own ID generation scheme. |
| `pkg/data/ssot_importer.go:243-410` | SSOT importer batches customer creation and uses its own customer ID generation rules. |
| `batch_operations.go:98-106` | `BatchCreateCustomers` inserts rows directly with no shared policy layer. |
| `app.go:7865-7992` | `SeedCustomerDatabase` inserts sample customers directly. |
| `app.go:13020-13074` | `SeedDatabaseFromOpportunities` creates missing customers during seed import. |
| `app.go:8075-8103` | `BackfillBusinessCustomerIDs` repairs existing records using another ID-derivation rule. |
| `benchmark_runner.go:244-252` | Benchmark creates customers directly via the DB, bypassing app logic. |

---

## Invariant Ledger

These are the rules the refactor must preserve unless we explicitly choose to change them later.

### Business invariants

- `BusinessName` must be non-empty.
- Customer creation must produce a usable business identifier.
- Default country for Butler-assisted customer creation is `Bahrain`.
- Default Butler `PaymentGrade` falls back to `B` if missing or invalid.
- Valid customer status values are `Active`, `Inactive`, `Blacklisted`.
- Valid customer types are `Corporate`, `Government`, `Individual`, `SME`, `EC`.
- Valid payment grades are `A`, `B`, `C`, `D`.
- `CreditLimitBHD` must remain non-negative.
- `PaymentTermsDays` must remain within `0..365`.

### Permission invariants

- Manual and Butler customer creation must require `customers:create`.
- Customer updates must require the current update permission boundary, currently `customers:edit`.
- Refactoring may relocate the permission checks, but it must not weaken them.

### Identifier invariants

- A customer record must end the create flow with a non-empty `customer_code`.
- A customer record must end the create flow with a non-empty `customer_id`.
- The system must tolerate missing client-provided identifiers and generate them server-side.
- Identifier generation must remain deterministic enough for operational use and unique enough to avoid collisions.

### Persistence invariants

- Create must persist a valid `CustomerMaster` row to `customers`.
- Update must preserve audit history fields such as `CreatedAt` and `CreatedBy`.
- The refactor must not silently widen which fields can be blank or malformed.

### Behavioral invariants

- Butler-assisted creation must continue to reuse the primary manual create policy rather than becoming a separate business flow.
- Existing UI screens must keep working against the Wails boundary after the first extraction pass.
- Import/seed paths may remain separate initially, but they must be accounted for in the seam design.

---

## Drift And Bloat Findings

These are the main reasons this slice is worth doing first.

### 1. Identifier logic is split across several places

- `CustomersScreen.svelte` pre-generates `customer_id` client-side.
- `CreateCustomer` auto-generates `customer_code` and falls back `customer_id` server-side.
- `findOrCreateCustomer` uses a different import-specific format.
- the SSOT importer uses another identifier strategy.
- `BackfillBusinessCustomerIDs` uses yet another derivation rule.

This means identifier policy is not a policy yet. It is scattered behavior.

### 2. Customer write paths are fragmented

There is not one authoritative creation path for customers. There are several:

- manual create
- Butler create
- import create-or-find
- batch create
- seed create
- benchmark direct insert

That increases drift risk and makes refactoring harder than it needs to be.

### 3. Validation is not consistently applied

- `CreateCustomer` uses `ValidateCustomerInput`.
- `CreateCustomerFromButler` benefits because it delegates to `CreateCustomer`.
- `UpdateCustomer` does not currently run the same validator path.
- direct DB insert paths bypass the validator entirely.

### 4. UI and backend both try to own business shaping

The frontend composes address fields, maps payment terms, assigns defaults, and sometimes pre-generates identifiers before the backend sees the payload.

That means the Wails boundary is not a thin adapter yet.

### 5. Test coverage is uneven

- core create/update CRUD is covered
- validator unit tests exist
- Butler path exists
- import/seed paths exist

But:

- there is no true frontend e2e for customer creation
- mocked e2e bridges do not cover `CreateCustomer`
- many tests insert customers directly and bypass the real business path

---

## Blast Radius Notes

The direct `CreateCustomer` refactor is medium risk, not high risk.

### Why it is safe enough for a pilot

- customer create is important but not the most financially sensitive flow
- Butler already delegates to the primary create path
- the main path is easy to reason about
- current tests cover the basic backend CRUD behavior

### What makes it risky

- alternate create paths bypass the main logic
- identifier drift is already present
- the frontend assumes some identifier/defaulting behavior
- update behavior is not fully aligned with create behavior

### Refactor rule for this slice

Do not rewrite imports and seeding in the first pass.  
Instead, define a seam that imports can later consume.

---

## Desired End State

After the first refactor pass, the target shape should be:

- `App.CreateCustomer`: Wails adapter + permission boundary
- `App.UpdateCustomer`: Wails adapter + permission boundary
- `CustomerPolicy`: validation and business defaults
- `CustomerIdentifierService`: generation and normalization of `customer_id` and `customer_code`
- `CustomerRepository`: create/update/find-or-create/persistence helpers
- `CustomerCommandService` or equivalent orchestrator: the single home for manual create/update behavior

This does not require full repo-wide customer unification on day one.  
It does require creating the seam that everything else can gradually move behind.

---

## Recommended Extraction Order

### Step A: codify the contract

Create a small customer command contract in code:

- what fields are required
- what defaults are applied
- what identifiers are generated
- what validation runs for create vs update

### Step B: extract identifier policy

Move ID and code derivation into one backend helper/service.

This is the highest-value first extraction because it reduces drift immediately.

### Step C: extract create orchestration

Move the body of `CreateCustomer` into a customer command service while keeping the App method as a facade.

### Step D: align update with create policy

Make `UpdateCustomer` intentionally reuse the same validation/defaulting model where appropriate.

This should be done carefully so updates do not accidentally overwrite preserved business values.

### Step E: document and queue alternate paths

Do not force all seed/import flows onto the new seam immediately.

Instead:

- list which paths still bypass it
- convert the safest ones first later
- leave benchmark-only direct inserts as an explicit exception if needed

---

## Verification Plan

Before code changes in this slice, the minimum safety checks should be:

- confirm `customer_supplier_creation_test.go` still covers backend create permission and generated IDs
- confirm `comprehensive_e2e_test.go` still covers customer CRUD
- confirm validator tests still cover accepted and rejected inputs

As part of this slice or immediately after it, we should add:

- one test proving create and update share the intended validation contract
- one test proving server-side ID generation works when the frontend sends empty identifiers
- one test proving Butler create still delegates to the same policy path
- one frontend mocked flow or e2e path that actually exercises `CreateCustomer`

---

## GitNexus Workflow For This Slice

### Discovery

- `query({ query: "customer creation flow" })`
- `context({ name: "ValidateCustomerInput", file_path: "security_enhancements.go" })`
- `impact({ target: "CreateCustomer", direction: "upstream" })`

### Practical note

For this repo, GitNexus is useful for orientation, but customer creation discovery was more reliable when combined with direct source search using `rg`.

Use both:

- GitNexus for flow and impact framing
- local source inspection for exact callers and drift detection

### After implementation

- `detect_changes({ scope: "all" })`
- inspect whether only the intended customer slice moved
- run targeted Go tests for customer creation and validation

---

## Proposed Success Criteria For The First Refactor Pass

We can consider the first customer slice pass successful if:

- `CreateCustomer` becomes visibly thinner
- identifier generation exists in one backend home
- create validation is no longer half-owned by the frontend
- `UpdateCustomer` has an explicit relationship to the same policy layer
- the documentable number of customer write paths with unique identifier logic decreases

The first pass does **not** need to solve every import path.

---

## Follow-On Queue After This Slice

Once the customer slice is stable, the next best follow-on is:

1. costing to offer
2. pipeline opportunity model consolidation
3. delivery and serial flows
4. finance and sync

That preserves the overall branch strategy set in the main framework.
