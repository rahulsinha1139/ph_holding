# Incident Response Runbook

Date: 2026-04-14
Applies To: AsymmFlow / `ph_holdings`

## Purpose

This runbook gives a lightweight response path for security or integrity incidents.

## Incident Types In Scope

- authorization bypass or privilege misuse
- customer or supplier data exposure
- invoice, payment, or settlement corruption
- bank reconciliation integrity issue
- backup or restore failure
- sync misconfiguration or unexpected remote exposure
- committed secret or credential leak

## Response Goals

- contain quickly
- preserve evidence
- avoid making the situation worse
- restore trustworthy operation
- record what happened and what changed

## Initial Steps

1. identify the affected workflow, environment, and approximate time window
2. classify severity and business impact
3. stop or limit the risky behavior if containment is safe
4. preserve logs, screenshots, commands, and commit references
5. decide whether a backup, configuration rollback, or code fix is the safer next move

## Containment Examples

- disable optional sync if credential or remote-exposure risk is suspected
- restrict admin use of affected workflows until behavior is understood
- preserve the current database before attempting repair
- pause release or deployment activity if the incident is tied to recent changes

## Investigation Focus

When triaging, answer:

- what trust boundary failed
- what data could be read, changed, or lost
- whether the issue is local-only, remote-enabled, or both
- whether the issue is caused by auth, state transitions, config, or operator error
- what evidence proves the incident scope

## Recovery

Recovery should prefer:

- reversible actions
- validated backups
- targeted fixes over broad destructive changes
- post-recovery verification of the affected workflow

## Communication

Record:

- incident summary
- impact
- containment action
- remediation action
- remaining risk
- follow-up owner

## Post-Incident

After stabilization:

- update the evidence log
- update the threat model or controls matrix if the incident revealed a real gap
- create a prevention task if the issue was structural rather than accidental
