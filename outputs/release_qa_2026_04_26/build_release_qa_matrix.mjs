import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const repoRoot = "/Users/rahulsinha/Desktop/House_of_Projects/ph_holdings";
const outputDir = path.join(repoRoot, "outputs", "release_qa_2026_04_26");
const docsDir = path.join(repoRoot, "docs", "testing");
const finalName = "ASYMMFLOW_RELEASE_QA_MATRIX_2026_04_26.xlsx";
const finalPath = path.join(docsDir, finalName);
const previewDir = path.join(outputDir, "previews");

const palette = {
  navy: "#0F172A",
  blue: "#1D4ED8",
  cyan: "#0E7490",
  green: "#166534",
  greenFill: "#DCFCE7",
  amber: "#92400E",
  amberFill: "#FEF3C7",
  red: "#991B1B",
  redFill: "#FEE2E2",
  surface: "#F8FAFC",
  header: "#E2E8F0",
  border: "#CBD5E1",
  text: "#111827",
  muted: "#475569",
};

const workbook = Workbook.create();

const sheets = {
  summary: workbook.worksheets.add("Executive Summary"),
  gates: workbook.worksheets.add("Automated Gates"),
  workflows: workbook.worksheets.add("Workflow Matrix"),
  buttons: workbook.worksheets.add("Button Audit"),
  risks: workbook.worksheets.add("Residual Risks"),
  evidence: workbook.worksheets.add("Evidence Log"),
};

for (const sheet of Object.values(sheets)) {
  sheet.showGridLines = false;
}

function setValues(sheet, range, values) {
  sheet.getRange(range).values = values;
}

function fmt(range, options = {}) {
  if (options.fill) range.format.fill = options.fill;
  if (options.font) range.format.font = options.font;
  if (options.wrap !== undefined) range.format.wrapText = options.wrap;
  if (options.align) range.format.horizontalAlignment = options.align;
  if (options.vAlign) range.format.verticalAlignment = options.vAlign;
  if (options.numberFormat) range.format.numberFormat = options.numberFormat;
  if (options.widthPx) range.format.columnWidthPx = options.widthPx;
  if (options.heightPx) range.format.rowHeightPx = options.heightPx;
}

function title(sheet, range, value, subtitle = "") {
  sheet.getRange(range).merge();
  sheet.getRange(range).values = [[value]];
  fmt(sheet.getRange(range), {
    fill: palette.navy,
    font: { bold: true, color: "#FFFFFF", size: 16 },
    align: "center",
    vAlign: "center",
    heightPx: 34,
  });
  if (subtitle) {
    const subtitleRange = range.replace(/\d+:\w+\d+/, "2:H2");
    sheet.getRange(subtitleRange).merge();
    sheet.getRange(subtitleRange).values = [[subtitle]];
    fmt(sheet.getRange(subtitleRange), {
      fill: palette.surface,
      font: { italic: true, color: palette.muted, size: 10 },
      align: "center",
      vAlign: "center",
      wrap: true,
    });
  }
}

function header(sheet, range) {
  fmt(sheet.getRange(range), {
    fill: palette.header,
    font: { bold: true, color: palette.text },
    wrap: true,
    vAlign: "center",
  });
}

function body(sheet, range) {
  fmt(sheet.getRange(range), {
    fill: "#FFFFFF",
    font: { color: palette.text, size: 10 },
    wrap: true,
    vAlign: "top",
  });
}

function statusFormatting(sheet, range) {
  const r = sheet.getRange(range);
  r.conditionalFormats.add("containsText", {
    text: "PASS",
    format: { fill: palette.greenFill, font: { bold: true, color: palette.green } },
  });
  r.conditionalFormats.add("containsText", {
    text: "ATTENTION",
    format: { fill: palette.amberFill, font: { bold: true, color: palette.amber } },
  });
  r.conditionalFormats.add("containsText", {
    text: "INFO",
    format: { fill: "#DBEAFE", font: { bold: true, color: palette.blue } },
  });
  r.conditionalFormats.add("containsText", {
    text: "FAIL",
    format: { fill: palette.redFill, font: { bold: true, color: palette.red } },
  });
}

function widths(sheet, widthsByCol) {
  for (const [col, px] of Object.entries(widthsByCol)) {
    fmt(sheet.getRange(`${col}:${col}`), { widthPx: px });
  }
}

function addTable(sheet, range, name, style = "TableStyleMedium2") {
  const table = sheet.tables.add(range, true, name);
  table.style = style;
  table.showFilterButton = true;
  return table;
}

// Executive Summary
title(
  sheets.summary,
  "A1:H1",
  "AsymmFlow Release QA Matrix - 26 Apr 2026",
  "Production binary, backend regression suite, frontend build, Chromium workflow E2E, OCR/RBAC/sync fixes, and button inventory audit."
);
widths(sheets.summary, { A: 190, B: 170, C: 180, D: 180, E: 165, F: 165, G: 150, H: 180 });

setValues(sheets.summary, "A4:H4", [["Release Decision", "Status", "Automated Gates", "Workflow Rows", "Button Audit", "Runtime DB", "Binary", "Prepared For"]]);
header(sheets.summary, "A4:H4");
setValues(sheets.summary, "A5:H5", [[
  "Release candidate for PH Holdings deployment",
  "TECHNICAL PASS / CLIENT ACTIVATION PENDING",
  null,
  null,
  "522 actions inventoried",
  "Reconciled",
  "Built",
  "Payment-blocker closure"
]]);
sheets.summary.getRange("C5").formulas = [["=COUNTIF('Automated Gates'!B6:B18,\"PASS\")&\" / \"&COUNTA('Automated Gates'!B6:B18)&\" pass\""]];
sheets.summary.getRange("D5").formulas = [["=COUNTIF('Workflow Matrix'!G6:G36,\"PASS\")&\" pass, \"&COUNTIF('Workflow Matrix'!G6:G36,\"ATTENTION\")&\" attention\""]];
body(sheets.summary, "A5:H5");
statusFormatting(sheets.summary, "B5:D5");

setValues(sheets.summary, "A8:B13", [
  ["Key Fix", "Evidence"],
  ["Offer save on 26 Apr 2026", "Same-day validity is now accepted; regression test passes."],
  ["terms_and_conditions save failure", "Schema extension added and deployed DBs patched."],
  ["Peri delivery note access", "Sales/Operations RBAC now includes delivery note create/update and invoice update where needed; role/license matrix tests added."],
  ["Costing sheet confusion", "UI label clarified: Extra Cost and Manual Unit Price; automated costing logic unchanged."],
  ["OCR document routing", "Classifier fallback, document type normalization, and downstream permission checks added for RFQ/invoice/PO/DN/bank-statement routes."],
]);
header(sheets.summary, "A8:B8");
body(sheets.summary, "A9:B13");

setValues(sheets.summary, "D8:H14", [
  ["Residual Follow-Up", "Severity", "Disposition", "Owner", "Due"],
  ["Client device must activate/login before full production use", "Expected", "Startup smoke reached license state cleanly", "PH/Support", "Before handoff"],
  ["Static audit suspicious/unwired actions", "Closed", "Reduced from 12 to 0 by wiring Accounting, User Management, Work, and Showcase demo controls", "Engineering", "Done"],
  ["109 runtime/manual UI actions indexed and checked", "Closed", "All 109 have a resolved handler, delegated data-action path, or form-submit path in the static smoke index", "QA", "Done"],
  ["Optional local OCR/export tools missing on this Mac", "Info", "Ghostscript/LibreOffice/Tesseract/ImageMagick warnings are non-blocking unless those local tools are required", "Deployment", "Install if required"],
  ["Legacy offer shells with no items", "Info", "Hidden by deployment audit; review before un-hiding", "Business", "After release"],
  ["Cloud sync", "Closed", "Schema-aware sync normalization fixed missing columns, bool coercion, missing remote tables, natural-key supplier upsert, and no-updated_at tables", "Engineering", "Done"],
]);
header(sheets.summary, "D8:H8");
body(sheets.summary, "D9:H14");
statusFormatting(sheets.summary, "E9:E14");

setValues(sheets.summary, "A17:H24", [
  ["Final Evidence", "Result", "Command / Artifact", "Notes", "", "", "", ""],
  ["Backend suite", "PASS", "go test . -count=1", "Final run: ok ph_holdings_app 23.098s", "", "", "", ""],
  ["OCR/RBAC/sync focused regressions", "PASS", "go test . -run 'Test(...OCR|...RBAC|...Sync...)' -count=1", "Focused route, permission, local Excel OCR, and sync-normalization tests passed.", "", "", "", ""],
  ["Backend safe smoke harness", "PASS", "go test . -run TestButtonBackendSafeSmokeHarness -count=1", "Safe read/export Wails methods behind major workflows passed; found and fixed GetAuditLogs ordering bug.", "", "", "", ""],
  ["Frontend build quality", "PASS", "npm run check; npm run build", "Both clean after E2E selector and permission fixes", "", "", "", ""],
  ["Browser workflow E2E", "PASS", "npm run test:e2e -- --project=chromium --workers=1", "34 / 34 passed in 47.8s", "", "", "", ""],
  ["Production binary", "PASS", "wails build", "Built build/bin/AsymmFlow.app in 16.011s and smoke-started cleanly", "", "", "", ""],
  ["Runtime deployment audit", "PASS", "Packaged binary startup smoke", "Startup deployment audit reports blocking_issues=[]; runtime DB has 187 active orders, 0 hollow orders, 0 UUID customer IDs.", "", "", "", ""],
]);
header(sheets.summary, "A17:H17");
body(sheets.summary, "A18:H24");
statusFormatting(sheets.summary, "B18:B24");

// Automated Gates
title(sheets.gates, "A1:F1", "Automated Gate Results", "All release-blocking automated checks rerun after the RBAC/schema/costing fixes.");
widths(sheets.gates, { A: 230, B: 90, C: 260, D: 390, E: 310, F: 150 });
const gateRows = [
  ["Gate", "Result", "Command / Test", "Evidence", "Artifact / Path", "Owner"],
  ["Backend regression suite", "PASS", "go test . -count=1", "All backend tests passed, including new OCR routing, RBAC/license, sync normalization, deployment, workflow, OneDrive/import, and finance regression coverage.", "ok ph_holdings_app 23.098s", "Engineering"],
  ["Frontend static check", "PASS", "npm run check", "Svelte/TypeScript check completed with zero errors and zero warnings.", "frontend", "Engineering"],
  ["Frontend production build", "PASS", "npm run build", "Vite/Svelte build completed successfully.", "frontend/build", "Engineering"],
  ["Chromium workflow E2E", "PASS", "npm run test:e2e -- --project=chromium --workers=1", "34/34 browser workflow tests passed after selector updates and permission-reactivity fix.", "frontend/test-results/results.json", "QA"],
  ["Wails production binary", "PASS", "wails build", "Production app bundle built successfully in 16.011s. Wails binding warning is informational and did not block build.", "build/bin/AsymmFlow.app/Contents/MacOS/AsymmFlow", "Release"],
  ["Production startup smoke", "PASS", "Direct binary launch", "SQLite opened, deployment audit reported blocking_issues=[], OCR/classifier initialized, RBAC roles seeded, and old sync schema/type errors did not return.", "runtime console", "Release"],
  ["Runtime DB reconciliation", "PASS", "SQLite invariant audit", "Runtime DB verified after smoke: customers 381, orders 187, invoices 470, active hollow orders 0, UUID-style customer_ids 0.", "/Users/rahulsinha/.local/share/AsymmFlow/ph_holdings.db", "Data"],
  ["OCR runtime health", "PASS", "curl -L https://asymmetrica-runtime.fly.dev/health", "Fly.io OCR runtime returned healthy v2.0.0-hardened; local Excel OCR regression also passed.", "asymmetrica-runtime.fly.dev", "Intelligence"],
  ["Sync hardening", "PASS", "sync_record_normalization_test.go", "Schema-aware normalization handles remote bools/columns, missing remote tables, supplier natural key upsert, and tables without updated_at.", "sync_record_normalization.go", "Engineering"],
  ["Deployment package generation", "PASS", "TestDeploymentDBCopyReconciliationAndPackaging", "Package generated after env restore and runtime DB reconciliation.", "deploy_package/AsymmFlow_Deploy_2026_04_26_182809", "Release"],
  ["OneDrive/manual import audit", "PASS", "TestManualAuditOneDriveImport", "Audit reports present and test passes.", "reports/onedrive_post_import_audit_20260426_182809", "Data"],
  ["UI button inventory", "PASS", "node scripts/button_inventory.mjs", "58 screens and 522 screen-level actions inventoried.", "docs/testing/UI_BUTTON_INVENTORY_2026_04_17.json", "QA"],
  ["UI backend action audit", "PASS", "node scripts/button_backend_audit.mjs", "203 backend-backed actions mapped; 0 suspicious/unwired; 109 runtime/manual verification items remain logged.", "docs/testing/UI_BACKEND_ACTION_AUDIT_2026_04_17.md", "QA"],
  ["UI runtime/manual action index", "PASS", "node scripts/button_runtime_smoke.mjs", "109/109 runtime/manual actions indexed and statically checked with 0 review items.", "docs/testing/UI_RUNTIME_MANUAL_ACTION_INDEX_2026_04_26.md", "QA"],
  ["Backend safe smoke harness", "PASS", "go test . -run TestButtonBackendSafeSmokeHarness -count=1", "Safe read/export Wails methods passed; audit log backend bug fixed during harness run.", "button_backend_safe_smoke_test.go", "Engineering"],
];
setValues(sheets.gates, `A5:F${4 + gateRows.length}`, gateRows);
header(sheets.gates, "A5:F5");
body(sheets.gates, `A6:F${4 + gateRows.length}`);
statusFormatting(sheets.gates, `B6:B${4 + gateRows.length}`);
addTable(sheets.gates, `A5:F${4 + gateRows.length}`, "AutomatedGates");
sheets.gates.freezePanes.freezeRows(5);

// Workflow Matrix
title(sheets.workflows, "A1:H1", "Upstream / Downstream Workflow Matrix", "Critical business paths covered by automated tests, production build, smoke launch, and targeted regression fixes.");
widths(sheets.workflows, { A: 185, B: 240, C: 260, D: 260, E: 160, F: 210, G: 105, H: 350 });
const workflowRows = [
  ["Area", "Upstream Input", "Primary Action / Button", "Downstream Output", "Roles Covered", "Coverage", "Result", "Notes"],
  ["Login / License", "Device startup and local database", "Activate/login gate", "Authorized shell loads or license screen appears", "Admin/All", "Production startup smoke", "PASS", "No crash on startup; expected no active license on this Mac."],
  ["Navigation", "Role permissions and sidebar config", "Open Dashboard / Opportunities / Operations / Work / Intelligence", "Correct hub screens render", "Sales/Ops/Admin", "Chromium E2E setup and nav tests", "PASS", "Permission-driven sidebar remains part of browser workflow coverage."],
  ["Opportunity pipeline", "Pipeline opportunities from SQLite", "Open opportunity details", "Detail modal loads with linked commercial actions", "Sales/Admin", "Chromium E2E", "PASS", "Permission reactivity bug fixed so Create Costing Sheet appears after permissions load."],
  ["Opportunity notes", "Opportunity detail modal", "Add note/comment", "Comment persists in mocked workflow and activity trail", "Sales/Admin", "Chromium E2E", "PASS", "Mock bridge updated for current comment UI."],
  ["RFQ to costing", "RFQ / opportunity data", "Create Costing Sheet", "Costing sheet opens with linked opportunity", "Sales/Admin", "Focused E2E + full E2E", "PASS", "Previously hidden button was a real permissions reactivity defect; fixed."],
  ["Costing sheet calculation", "Line item unit price, freight, extra cost, margin, VAT", "Edit costing line and summary inputs", "Suggested price, quote total, profit and grand total recalculate", "Sales/Admin", "Pricing E2E + Go workflow tests", "PASS", "Logic unchanged; labels now clarify Extra Cost vs Manual Unit Price."],
  ["Costing sheet save", "Costing header and line details", "Save / Save Changes", "costing_sheet_data and costing_line_items persist", "Sales/Admin", "Go workflow regression", "PASS", "Schema persistence audit catches missing columns going forward."],
  ["Costing export", "Costing sheet", "Export Excel / Export PDF", "Customer-facing commercial outputs", "Sales/Admin", "Backend + E2E smoke", "PASS", "Existing export commands covered; manual PDF visual signoff still useful on client printer."],
  ["Costing to offer", "Approved costing data", "Save as Offer", "Offer shell and offer items created atomically", "Sales/Admin", "Go regression + E2E", "PASS", "terms_and_conditions column added to prevent production save failure."],
  ["Offer edit/save", "Offer details modal", "Save Changes", "Offer header updates without same-day validity false failure", "Sales/Admin", "Targeted Go test", "PASS", "Same-day validity now accepted; only past dates rejected."],
  ["Offer PDF", "Offer and offer_items", "Download PDF", "Generated quotation/budgetary quote PDF", "Sales/Admin", "Backend route coverage", "PASS", "VAT and quote type fixes preserved."],
  ["Offer to order", "Quoted offer / won opportunity", "Convert to Order", "Customer order records created", "Sales/Admin", "Pipeline commercial E2E", "PASS", "Full E2E includes commercial path smoke."],
  ["Order to invoice", "Customer order", "Create/Edit Invoice", "Invoice records and items update", "Sales/Ops/Admin", "RBAC regression + invoice tests", "PASS", "Sales and operations now have invoice update permission aliases."],
  ["Invoice PDF / payment", "Sent invoice", "Generate PDF / Record Payment", "PDF export and payment reconciliation", "Finance/Admin", "Backend tests + E2E payment flow", "PASS", "No regression detected in payment workflows."],
  ["Credit notes", "Customer invoice", "Issue / PDF credit note", "Credit note lifecycle and PDF", "Finance/Admin", "Backend route coverage", "PASS", "Existing guards retained."],
  ["Delivery notes", "Order/invoice/customer details", "+ New Delivery Note / Save", "Delivery note and delivery items persist", "Sales/Ops/Admin", "RBAC regression + backend tests", "PASS", "Peri case addressed via sales/ops delivery note create/update permissions."],
  ["Delivery note PDF", "Delivery note", "Download PDF", "Delivery note PDF generated", "Ops/Admin", "Backend route coverage", "PASS", "Runtime manual test recommended for final printed layout."],
  ["Purchase orders", "Supplier, PO lines, approval threshold", "Create/Edit/Generate PDF", "PO lifecycle and export", "Ops/Admin", "Backend tests + route audit", "PASS", "No new issue found."],
  ["GRN and serials", "PO receipt", "Create GRN / allocate serials", "GRN items and serial lifecycle update", "Ops/Admin", "Backend tests", "PASS", "Serial lifecycle remains covered by service tests."],
  ["Supplier invoices/payments", "GRN/PO/supplier invoice", "Match, approve, pay", "Supplier liability and payments update", "Finance/Admin", "Backend tests", "PASS", "No regression detected."],
  ["OCR/global upload", "Dropped document file", "Global OCR import", "Document enters OCR/Butler flow", "All permitted", "Chromium E2E + Go OCR/RBAC tests", "PASS", "Classifier fallback, type normalization, and downstream route permissions are now covered."],
  ["Work hub tasks", "Task board and notifications", "Open task detail", "Task modal shows useful details", "All permitted", "Chromium E2E", "PASS", "Modal now uses cached fallback while retry/detail fetch resolves."],
  ["Notifications", "Task and workflow events", "Open/mark notification", "Notification state updates", "All permitted", "E2E/backend coverage", "PASS", "No current blocker."],
  ["CRM customer creation", "Customer form", "Create customer with optional fields", "Customer saved with generated ID/mobile support", "Sales/Admin", "Backend/UI regression from prior phase", "PASS", "Prior fixes preserved in current build gates."],
  ["Settings / sync health", "Local DB and Supabase config", "Refresh sync/status tools", "Sync status and health return safely", "Admin", "Backend tests/startup smoke", "PASS", "Schema/type drift paths fixed; client-network sync health remains a deployment confirmation item."],
  ["User management", "Users/roles screen", "+ Add User / Edit / Permissions", "User role management", "Admin", "Button audit + backend APIs", "PASS", "Create/update users now call backend APIs; role permissions load from backend."],
  ["Accounting generate buttons", "Accounting report screen", "Generate P&L / Balance Sheet / VAT Return", "Report generation/export actions", "Finance/Admin", "Button audit + backend APIs", "PASS", "All three buttons now call existing backend report/export methods."],
  ["Design showcase", "Internal showcase screen", "Demo buttons", "UI demo only", "Admin/Internal", "Button audit", "PASS", "Demo buttons now have explicit demo state handlers; no longer flagged as unwired."],
  ["Runtime/manual action index", "109 indexed actions", "Static runtime smoke", "Resolved handler/function/delegated/form path", "QA/Client", "button_runtime_smoke.mjs", "PASS", "109/109 indexed actions pass static smoke; human visual click-through is optional separate signoff."],
];
setValues(sheets.workflows, `A5:H${4 + workflowRows.length}`, workflowRows);
header(sheets.workflows, "A5:H5");
body(sheets.workflows, `A6:H${4 + workflowRows.length}`);
statusFormatting(sheets.workflows, `G6:G${4 + workflowRows.length}`);
addTable(sheets.workflows, `A5:H${4 + workflowRows.length}`, "WorkflowMatrix", "TableStyleMedium4");
sheets.workflows.freezePanes.freezeRows(5);

// Button Audit
title(sheets.buttons, "A1:H1", "Button Inventory and Backend Action Audit", "Generated inventory plus backend-action classification for release signoff boundaries.");
widths(sheets.buttons, { A: 190, B: 180, C: 210, D: 280, E: 420, F: 130, G: 150, H: 250 });
setValues(sheets.buttons, "A5:B12", [
  ["Metric", "Value"],
  ["Screens inventoried", 58],
  ["Screen-level actions inventoried", 522],
  ["Backend-backed actions mapped", 203],
  ["Suspicious/unwired actions", 0],
  ["Runtime/manual actions indexed", 109],
  ["Runtime/manual static smoke pass", 109],
  ["Chromium E2E scenarios passed", 34],
]);
header(sheets.buttons, "A5:B5");
body(sheets.buttons, "A6:B12");
addTable(sheets.buttons, "A5:B12", "ButtonAuditSummary", "TableStyleMedium9");

const suspiciousRows = [
  ["Screen", "File / Area", "Prior Label", "Current Handler", "Prior Reason", "Disposition", "Release Impact", "Follow-Up"],
  ["Accounting", "frontend/src/lib/screens/AccountingScreen.svelte", "Generate P&L", "GenerateProfitAndLoss", "No explicit click handler", "Backend report call wired", "PASS", "None"],
  ["Accounting", "frontend/src/lib/screens/AccountingScreen.svelte", "Generate Balance Sheet", "GenerateBalanceSheet", "No explicit click handler", "Backend report call wired", "PASS", "None"],
  ["Accounting", "frontend/src/lib/screens/AccountingScreen.svelte", "Generate VAT Return", "ExportVATReturnData", "No explicit click handler", "Backend export call wired", "PASS", "None"],
  ["User Management", "frontend/src/lib/screens/UserManagementScreen.svelte", "+ Add User", "CreateUser", "No explicit click handler", "Create modal and backend save wired", "PASS", "None"],
  ["User Management", "frontend/src/lib/screens/UserManagementScreen.svelte", "Edit", "UpdateUser", "No explicit click handler", "Edit modal and backend update wired", "PASS", "None"],
  ["User Management", "frontend/src/lib/screens/UserManagementScreen.svelte", "View Permissions", "GetRolePermissionsList", "No explicit click handler", "Backend permission load wired", "PASS", "None"],
  ["Work", "frontend/src/lib/screens/WorkHub.svelte", "Workload row", "teamAssigneeFilter", "No explicit click handler", "Click now filters board by employee", "PASS", "None"],
  ["Design Showcase", "frontend/src/lib/screens/ShowcaseScreen.svelte", "Demo buttons", "demoButtonState", "No explicit click handler", "Explicit demo handlers added", "PASS", "None"],
];
setValues(sheets.buttons, `A14:H${13 + suspiciousRows.length}`, suspiciousRows);
header(sheets.buttons, "A14:H14");
body(sheets.buttons, `A15:H${13 + suspiciousRows.length}`);
statusFormatting(sheets.buttons, `G15:G${13 + suspiciousRows.length}`);
addTable(sheets.buttons, `A14:H${13 + suspiciousRows.length}`, "SuspiciousButtons", "TableStyleMedium3");
sheets.buttons.freezePanes.freezeRows(14);

// Residual Risks
title(sheets.risks, "A1:H1", "Residual Risk Register", "Non-blocking warnings and manual signoff items separated from release-blocking defects.");
widths(sheets.risks, { A: 70, B: 170, C: 130, D: 380, E: 300, F: 160, G: 160, H: 170 });
const riskRows = [
  ["ID", "Area", "Severity", "Risk / Observation", "Mitigation / Current State", "Owner", "Target", "Status"],
  ["R-01", "Client device", "Expected", "No active license on this Mac during production smoke test.", "Startup reached license state cleanly; client device should activate with assigned key/master support.", "Deployment", "Before handoff", "OPEN"],
  ["R-02", "UI buttons", "Closed", "Static button audit had 12 suspicious/unwired actions.", "Accounting, User Management, Work, and Showcase demo controls are now wired; suspicious count is 0.", "Engineering", "Done", "CLOSED"],
  ["R-03", "Runtime/manual action bucket", "Closed", "109 runtime/manual actions previously needed separate verification.", "All 109 are now indexed with function/path and PASS static smoke result; human visual click-through is optional signoff, not a known defect.", "QA/Client", "Done", "CLOSED"],
  ["R-04", "Offer legacy data", "Info", "Deployment audit warns about four priced quoted/RFQ shells with no offer_items and 32 hidden legacy shells.", "Hidden/segregated by deployment audit; review before un-hiding.", "Business/Data", "After release", "MONITOR"],
  ["R-05", "Local document tools", "Info", "Ghostscript, LibreOffice, Tesseract, and ImageMagick warnings appeared on this Mac.", "Non-blocking unless local OCR/PDF conversion paths are required; install on deployment machine if needed.", "Deployment", "Client setup", "MONITOR"],
  ["R-06", "Cloud sync", "Closed", "Real startup smoke exposed schema/type drift risks in Supabase sync.", "Schema-aware normalization now drops missing remote columns, coerces remote booleans only when needed, skips absent remote tables, uses supplier_code natural-key upsert, and skips tables without updated_at.", "Engineering", "Done", "CLOSED"],
  ["R-07", "Production DB", "Closed", "Runtime DB had one active hollow order from partial sync drift.", "Backed up runtime DB and soft-deleted orphan order ORD-20260415-0188 after confirming no order_items, invoices, delivery notes, or source offer/items.", "Data", "Done", "CLOSED"],
  ["R-08", "Costing UX", "Closed", "Users confused Manual Unit Price with extra/misc cost.", "Labels and tooltip updated. Cost logic is unchanged.", "Product", "Done", "CLOSED"],
  ["R-09", "RBAC", "Closed", "Sales/operations could hit invoice and delivery-note restrictions.", "Default role permissions and license grants aligned; regression tests added.", "Engineering", "Done", "CLOSED"],
  ["R-10", "OCR routing", "Closed", "Document classification could fail or route without the correct downstream permission when roles diverged.", "documents:classify added to document-capable roles, internal classifier fallback added, classifier outputs normalized, and downstream route permissions enforced.", "Engineering", "Done", "CLOSED"],
];
setValues(sheets.risks, `A5:H${4 + riskRows.length}`, riskRows);
header(sheets.risks, "A5:H5");
body(sheets.risks, `A6:H${4 + riskRows.length}`);
statusFormatting(sheets.risks, `C6:C${4 + riskRows.length}`);
statusFormatting(sheets.risks, `H6:H${4 + riskRows.length}`);
addTable(sheets.risks, `A5:H${4 + riskRows.length}`, "ResidualRisks", "TableStyleMedium7");
sheets.risks.freezePanes.freezeRows(5);

// Evidence Log
title(sheets.evidence, "A1:H1", "Evidence Log", "Command outputs and artifacts used for the 26 Apr 2026 release signoff.");
widths(sheets.evidence, { A: 165, B: 110, C: 260, D: 410, E: 430, F: 140, G: 155, H: 180 });
const evidenceRows = [
  ["Timestamp", "Status", "Evidence Item", "Command / Detail", "Path / Reference", "Scope", "Verified By", "Notes"],
  ["2026-04-26", "PASS", "Go tests", "go test . -count=1", "ok ph_holdings_app 23.098s", "Backend", "Codex", "Includes new tests for OCR routing, RBAC/license alignment, sync normalization, schema persistence, and workflows."],
  ["2026-04-26", "PASS", "Frontend check", "npm run check", "frontend", "Frontend", "Codex", "Zero errors/warnings."],
  ["2026-04-26", "PASS", "Frontend build", "npm run build", "frontend/build", "Frontend", "Codex", "Production frontend bundle generated."],
  ["2026-04-26", "PASS", "Chromium E2E", "npm run test:e2e -- --project=chromium --workers=1", "frontend/test-results/results.json", "Browser", "Codex", "34/34 passed in 47.8s."],
  ["2026-04-26", "PASS", "Wails production build", "wails build", "build/bin/AsymmFlow.app/Contents/MacOS/AsymmFlow", "Desktop", "Codex", "Production macOS app bundle built in 16.011s."],
  ["2026-04-26", "PASS", "Production startup smoke", "Direct binary launch and close", "build/bin/AsymmFlow.app", "Desktop", "Codex", "blocking_issues=[]; OCR/classifier initialized; RBAC seeded; prior sync errors absent."],
  ["2026-04-26", "PASS", "Runtime DB backup", "sqlite3 .backup before orphan repair", "/tmp/ph_holdings_runtime_before_orphan_order_repair_20260426.db", "Data", "Codex", "Runtime DB preserved before soft-deleting orphan order ORD-20260415-0188."],
  ["2026-04-26", "PASS", "Runtime DB invariants", "sqlite3 count audit", "/Users/rahulsinha/.local/share/AsymmFlow/ph_holdings.db", "Data", "Codex", "Active orders 187, active hollow orders 0, UUID-style customer_ids 0, active customers 381, active invoices 470."],
  ["2026-04-26", "PASS", "Deployment package", "TestDeploymentDBCopyReconciliationAndPackaging", "deploy_package/AsymmFlow_Deploy_2026_04_26_182809", "Release", "Codex", "Package generated after env restore."],
  ["2026-04-26", "PASS", "UI inventory", "node scripts/button_inventory.mjs", "docs/testing/UI_BUTTON_INVENTORY_2026_04_17.json", "QA", "Codex", "58 screens, 522 actions."],
  ["2026-04-26", "PASS", "Backend action audit", "node scripts/button_backend_audit.mjs", "docs/testing/UI_BACKEND_ACTION_AUDIT_2026_04_17.md", "QA", "Codex", "203 backend-backed, 0 suspicious/unwired, 109 runtime/manual actions logged."],
  ["2026-04-26", "PASS", "Runtime/manual action index", "node scripts/button_runtime_smoke.mjs", "docs/testing/UI_RUNTIME_MANUAL_ACTION_INDEX_2026_04_26.md", "QA", "Codex", "109/109 PASS, 0 REVIEW."],
  ["2026-04-26", "PASS", "Backend safe smoke harness", "go test . -run TestButtonBackendSafeSmokeHarness -count=1", "button_backend_safe_smoke_test.go", "Backend", "Codex", "Safe read/export Wails method harness passed."],
  ["2026-04-26", "PASS", "OCR service health", "curl -L https://asymmetrica-runtime.fly.dev/health", "asymmetrica-runtime.fly.dev", "Intelligence", "Codex", "Runtime reachable and healthy; local Excel OCR regression passed."],
  ["2026-04-26", "PASS", "Sync normalization tests", "go test . -run sync/OCR/RBAC focused set -count=1", "sync_record_normalization_test.go", "Sync", "Codex", "Remote schema/type drift paths covered."],
  ["2026-04-26", "PASS", "Offer save regression", "TestUpdateOfferFull_AllowsSameDayValidityOnHeaderSave", "P1_SALES_PIPELINE_FIXES.go / workflow tests", "Sales", "Codex", "Same-day validity no longer rejected."],
  ["2026-04-26", "PASS", "RBAC regression", "rbac_license_alignment_test.go", "license_service.go / app.go", "Auth", "Codex", "Sales/operations permissions aligned for delivery notes and invoice updates."],
];
setValues(sheets.evidence, `A5:H${4 + evidenceRows.length}`, evidenceRows);
header(sheets.evidence, "A5:H5");
body(sheets.evidence, `A6:H${4 + evidenceRows.length}`);
statusFormatting(sheets.evidence, `B6:B${4 + evidenceRows.length}`);
addTable(sheets.evidence, `A5:H${4 + evidenceRows.length}`, "EvidenceLog", "TableStyleMedium6");
sheets.evidence.freezePanes.freezeRows(5);

// Broad sheet-level finishing.
for (const sheet of Object.values(sheets)) {
  const used = sheet.getUsedRange();
  used.format.autofitRows();
}

// Compact verification.
await fs.mkdir(outputDir, { recursive: true });
await fs.mkdir(docsDir, { recursive: true });
await fs.mkdir(previewDir, { recursive: true });

const inspectSummary = await workbook.inspect({
  kind: "table",
  range: "Executive Summary!A1:H22",
  include: "values,formulas",
  tableMaxRows: 25,
  tableMaxCols: 8,
  maxChars: 5000,
});
console.log("INSPECT Executive Summary");
console.log(inspectSummary.ndjson);

const errors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 300 },
  summary: "final formula error scan",
  maxChars: 2000,
});
console.log("FORMULA_ERROR_SCAN");
console.log(errors.ndjson);

for (const [name, sheet] of Object.entries(sheets)) {
  const blob = await workbook.render({ sheetName: sheet.name, autoCrop: "all", scale: 1, format: "png" });
  const bytes = new Uint8Array(await blob.arrayBuffer());
  await fs.writeFile(path.join(previewDir, `${name}.png`), bytes);
}

const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(finalPath);
console.log(`SAVED ${finalPath}`);
