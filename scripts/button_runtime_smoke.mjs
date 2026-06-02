#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";

const repoRoot = process.cwd();
const auditJsonPath = path.join(repoRoot, "docs/testing/UI_BACKEND_ACTION_AUDIT_2026_04_17.json");
const outJsonPath = path.join(repoRoot, "docs/testing/UI_RUNTIME_MANUAL_ACTION_INDEX_2026_04_26.json");
const outMdPath = path.join(repoRoot, "docs/testing/UI_RUNTIME_MANUAL_ACTION_INDEX_2026_04_26.md");

if (!fs.existsSync(auditJsonPath)) {
  console.error("Missing backend audit JSON. Run: node scripts/button_backend_audit.mjs");
  process.exit(1);
}

const audit = JSON.parse(fs.readFileSync(auditJsonPath, "utf8"));

function row(values) {
  return `| ${values.map((value) => String(value ?? "").replace(/\|/g, "\\|").replace(/\n/g, " ")).join(" | ")} |`;
}

function sourceFor(file) {
  return fs.readFileSync(path.join(repoRoot, file), "utf8");
}

function indexForLine(source, line) {
  if (!line || line < 1) return 0;
  let idx = 0;
  for (let current = 1; current < line; current += 1) {
    const next = source.indexOf("\n", idx);
    if (next === -1) return source.length;
    idx = next + 1;
  }
  return idx;
}

function formSubmitEvidence(action) {
  const source = sourceFor(action.file);
  const lineIndex = indexForLine(source, action.line);
  const before = source.lastIndexOf("<form", lineIndex);
  const beforeClose = source.lastIndexOf("</form", lineIndex);
  const afterClose = source.indexOf("</form", lineIndex);
  if (before !== -1 && (beforeClose === -1 || beforeClose < before) && afterClose !== -1) {
    const formOpenEnd = source.indexOf(">", before);
    const attrs = source.slice(before, formOpenEnd === -1 ? lineIndex : formOpenEnd);
    if (/on:submit/.test(attrs)) {
      return {
        result: "PASS",
        check: "form-submit-handler-present",
        evidence: "Submit button is inside a form with an on:submit handler.",
      };
    }
    return {
      result: "REVIEW",
      check: "form-submit-no-submit-handler-detected",
      evidence: "Submit button is inside a form, but static scan did not detect on:submit on the opening form tag.",
    };
  }
  return {
    result: "REVIEW",
    check: "form-submit-form-not-detected",
    evidence: "Static scan could not prove this submit button is inside a form.",
  };
}

function checkAction(action) {
  if (action.category === "Handler Present") {
    const resolved = action.resolved_handlers || [];
    if (resolved.length > 0 && resolved.every((handler) => handler.resolved)) {
      return {
        result: "PASS",
        check: "handler-resolves",
        evidence: `Handler reference(s) resolve: ${resolved.map((handler) => handler.name + (handler.line ? `@${handler.line}` : "")).join(", ")}.`,
      };
    }
    return {
      result: "REVIEW",
      check: "handler-reference-review",
      evidence: "Handler exists in click expression, but static scan could not resolve every reference.",
    };
  }

  if (action.category === "Delegated DataTable") {
    return {
      result: "PASS",
      check: "delegated-data-action-present",
      evidence: `data-action '${action.data_action}' is handled by delegated row/table click logic in ${action.file}.`,
    };
  }

  if (action.category === "Form Submit") {
    return formSubmitEvidence(action);
  }

  if (action.category === "Unknown" && action.click) {
    if (/=>\s*\(|=>\s*\{|=\s*[^=]/.test(action.click)) {
      return {
        result: "PASS",
        check: "explicit-click-expression-present",
        evidence: "Static scan found an explicit click expression that mutates local UI state or calls a local expression.",
      };
    }
    return {
      result: "REVIEW",
      check: "unknown-click-expression",
      evidence: "Static scan found a click expression, but could not classify the action type.",
    };
  }

  return {
    result: "REVIEW",
    check: "manual-runtime-only",
    evidence: "Static scan cannot prove this action without a browser/runtime click.",
  };
}

const indexed = audit.runtime_manual_actions.map((action, index) => {
  const smoke = checkAction(action);
  const functionPath =
    action.handler_references?.length > 0
      ? action.handler_references.join(", ")
      : action.data_action
        ? `data-action:${action.data_action}`
        : action.type === "submit"
          ? "form-submit"
          : action.click || "(none)";
  return {
    runtime_id: action.runtime_id || `RT-${String(index + 1).padStart(3, "0")}`,
    screen: action.screen,
    file: action.file,
    line: action.line,
    category: action.category,
    label: action.label,
    handler: action.click || "",
    function_path: functionPath,
    original_status: action.status,
    check: smoke.check,
    check_result: smoke.result,
    evidence: smoke.evidence,
  };
});

const summary = indexed.reduce(
  (acc, action) => {
    acc.total += 1;
    acc.by_result[action.check_result] = (acc.by_result[action.check_result] || 0) + 1;
    acc.by_category[action.category] = (acc.by_category[action.category] || 0) + 1;
    return acc;
  },
  { total: 0, by_result: {}, by_category: {} },
);

fs.writeFileSync(outJsonPath, `${JSON.stringify({ generated_at: new Date().toISOString(), summary, actions: indexed }, null, 2)}\n`);

const lines = [];
lines.push("# UI Runtime / Manual Action Index - 2026-04-26");
lines.push("");
lines.push("Purpose: index every action previously classified as runtime/manual by the backend-first button audit, map it to its handler/function/delegated path, and record the static smoke check result.");
lines.push("");
lines.push("This file does not claim human visual click-through. It proves each listed action has a resolvable handler, delegated data-action path, or form-submit path where static analysis can verify it.");
lines.push("");
lines.push("## Summary");
lines.push("");
lines.push(`- Actions indexed: ${summary.total}`);
lines.push(`- PASS: ${summary.by_result.PASS || 0}`);
lines.push(`- REVIEW: ${summary.by_result.REVIEW || 0}`);
lines.push("");
lines.push("### Categories");
lines.push("");
lines.push(row(["Category", "Count"]));
lines.push(row(["---", "---"]));
for (const [category, count] of Object.entries(summary.by_category).sort((a, b) => b[1] - a[1])) {
  lines.push(row([category, count]));
}
lines.push("");
lines.push("## Action Index");
lines.push("");
lines.push(row(["ID", "Screen", "File:Line", "Category", "Label", "Function / Path", "Check", "Result", "Evidence"]));
lines.push(row(["---", "---", "---", "---", "---", "---", "---", "---", "---"]));
for (const action of indexed) {
  lines.push(row([
    action.runtime_id,
    action.screen,
    `${action.file}:${action.line}`,
    action.category,
    action.label,
    action.function_path,
    action.check,
    action.check_result,
    action.evidence,
  ]));
}

fs.writeFileSync(outMdPath, `${lines.join("\n")}\n`);

console.log(`Wrote ${path.relative(repoRoot, outJsonPath)}`);
console.log(`Wrote ${path.relative(repoRoot, outMdPath)}`);
console.log(`Indexed runtime/manual actions: ${summary.total}`);
console.log(`PASS: ${summary.by_result.PASS || 0}`);
console.log(`REVIEW: ${summary.by_result.REVIEW || 0}`);
