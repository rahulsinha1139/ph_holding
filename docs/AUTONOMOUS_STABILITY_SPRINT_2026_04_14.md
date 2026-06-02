# Autonomous Stability Sprint

Date: 2026-04-14
Branch: `feat/migrate-flyio-to-vps-asymm-engine`
Objective: keep moving the codebase toward production-grade stability by reducing real risk, collapsing noisy warning clusters, and leaving behind dated proof after every checkpoint.

## Intent

This sprint is not a generic cleanup pass.

The goal is to make the product:

- safer to change
- easier to defend to buyers
- less noisy to maintain
- more accessible and operationally coherent
- backed by repeatable proof in the repo

The operating principle is:

1. target the highest-signal seam
2. make the smallest coherent fix
3. verify immediately
4. log the proof
5. commit and push
6. continue unless a real blocker appears

## Autonomous Authority

Pre-approval for this sprint means I can continue through the bundles below without stopping between each one, provided I stay within these guardrails:

- no destructive git operations
- no schema-changing migrations unless clearly low-risk and directly required
- no behavior-changing refactors in finance, sync, or approval logic without evidence and tight scope
- no reverting unrelated user changes
- stop and ask if I hit a hidden product decision, cross-cutting semantic risk, or failing verification I cannot localize quickly

## Commit Cadence

Each bundle should end with:

- updated code
- verification run
- `docs/EVIDENCE_LOG.md` entry
- one focused commit
- push to the current branch

Target cadence:

- small bundle: 5-12 minutes
- medium bundle: 12-25 minutes
- large bundle: 25-45 minutes only if the seam is still tightly bounded

## Verification Gates

Frontend bundles:

- `npm run build`
- confirm the targeted file or warning class disappears from the warning stream

Backend/security bundles:

- targeted `go test` band
- `go build ./...`
- targeted scanner rerun where relevant (`gosec`, `govulncheck`, `gitleaks`)

Shared/mixed bundles:

- both frontend and backend verification if both layers are touched

## Ordered Bundles

### Bundle 1: Shared Modal and Primitive Cleanup

Purpose:

- remove repeated modal/backdrop warning patterns at the component layer so downstream screens get easier to clean

Primary targets:

- `frontend/src/lib/components/ui/WabiModal.svelte`
- `frontend/src/lib/components/ui/Card.svelte`

Why this bundle is next:

- `WabiModal.svelte` is still emitting direct accessibility warnings
- `Card.svelte` has a noninteractive `tabIndex` warning that can contaminate many screens
- this is high leverage and likely reduces future screen-local churn

Acceptance criteria:

- targeted component warnings disappear from `npm run build`
- behavior remains visually and functionally identical for consuming screens

### Bundle 2: Delivery and Operations Interaction Seams

Purpose:

- clean up remaining click-target accessibility issues in operations flows

Primary targets:

- `frontend/src/lib/screens/DeliveryNotesScreen.svelte`
- `frontend/src/lib/screens/OperationsHub.svelte`
- adjacent lightweight operations screens only if they are part of the same warning cluster

Why this bundle is next:

- operations screens are user-facing, high-value, and still show concrete interaction warnings
- these are relatively bounded compared with the giant finance/costing seams

Acceptance criteria:

- delivery-note clickable-card warnings removed
- any `embedded` export noise addressed cleanly where safe
- build passes and targeted warnings move off the touched files

### Bundle 3: Buyer-Facing Demo and Entry Surface Cleanup

Purpose:

- clear low-risk but credibility-visible warnings on demo and entry screens

Primary targets:

- `frontend/src/lib/screens/ShowcaseScreen.svelte`
- `frontend/src/lib/screens/ArrivalCeremony.svelte`
- `frontend/src/lib/screens/LicenseActivationScreen.svelte`

Why this bundle is worth doing:

- these are low-risk, fast wins
- they remove warning noise from demo/entry surfaces buyers may actually see
- they help the warning stream converge so the remaining issues are more meaningful

Acceptance criteria:

- label wiring added where missing
- `autofocus` usage removed or replaced safely
- build passes and the target files disappear from the warning stream

### Bundle 4: Dashboard and Hub Noise Reduction

Purpose:

- strip out dead CSS and unused-export noise from broad dashboard surfaces

Primary targets:

- `frontend/src/lib/screens/FinanceHub.svelte`
- `frontend/src/lib/screens/DashboardScreen.svelte`
- `frontend/src/lib/screens/WorkHub.svelte`
- `frontend/src/lib/screens/AHSDashboard.svelte`

Why this bundle matters:

- these warnings are not as dangerous as interaction bugs, but they create a lot of noise
- reducing them makes future builds more actionable

Acceptance criteria:

- remove dead selectors or reattach styles only where the UI still uses them
- consume or correctly demote unused `embedded` exports where appropriate
- no visual regression on core dashboard layout assumptions

### Bundle 5: Costing Sheet Accessibility Mini-Refactor

Purpose:

- begin reducing the largest remaining frontend warning cluster without turning it into an uncontrolled rewrite

Primary targets:

- `frontend/src/lib/screens/CostingSheetScreen.svelte`

Scope discipline:

- do not attempt to “finish” the whole file in one pass
- take one coherent subsection at a time:
  - header field labels
  - selector dropdowns
  - revision list interaction semantics
  - line-item field labels
  - dead CSS cleanup

Why this bundle is later:

- it is high value, but it is also a very large and behavior-sensitive screen
- by the time we get here, the broader warning stream should be quieter

Acceptance criteria:

- one clearly bounded subcluster removed per commit
- no regression in costing-sheet usability
- build passes after each sub-pass

### Bundle 6: Backend Scanner and Stability Sweep

Purpose:

- return to backend risk after the current frontend warning burn-down

Primary targets:

- next bounded `gosec` findings with real signal, especially:
  - trust-sensitive `G104` unchecked errors
  - any remaining bounded numeric conversion risks
  - file/archive/IO error handling in OCR or document paths

Possible starting commands:

- `gosec ./...`
- `gosec -fmt json ./...`
- targeted `rg` on known noisy rules
- targeted `go test` around touched services

Acceptance criteria:

- fix only bounded, clearly explainable scanner findings
- record whether a finding was remediated, accepted as noise, or deferred
- keep proof tight in `docs/EVIDENCE_LOG.md`

### Bundle 7: Evidence and Governance Consolidation

Purpose:

- turn the sprint into buyer-legible proof, not just technical progress

Targets:

- refresh `docs/EVIDENCE_LOG.md`
- update trust/security docs if the implementation shifted any statements
- optionally add a short “current quality posture” summary document if enough progress has accumulated

Acceptance criteria:

- evidence log remains chronological and decision-useful
- no claims outrun the actual repo state

## Stop Conditions

Pause and ask before continuing if any of these happen:

- a change touches business semantics rather than warning semantics
- a shared component fix causes cascading regressions
- `npm run build` or `go build ./...` fails and the cause is not clearly local to the current seam
- a scanner finding points to potential live data loss, auth bypass, payment error, or sync corruption

## Success Definition

This sprint is successful if, by the end of the approved runway, we have:

- materially fewer real frontend accessibility and interaction warnings
- cleaner shared modal/component primitives
- at least one additional backend risk seam reduced
- a continuously updated evidence trail
- a branch history that reads like disciplined production hardening rather than random cleanup

## Pre-Approval Text

If you want to authorize the whole runway in one go, the approval can be interpreted as:

"Proceed autonomously through the ordered bundles in `docs/AUTONOMOUS_STABILITY_SPRINT_2026_04_14.md`, committing and pushing after each verified checkpoint, updating `docs/EVIDENCE_LOG.md`, and only pausing if you hit a real blocker or a decision with non-obvious product risk."
