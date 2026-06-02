<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import { toast } from "$lib/stores/toasts";
  import {
    GetDashboardStats,
    GetReportData,
    ExportReport,
    OpenExportedFile,
  } from "../../../wailsjs/go/main/App";
  import type { main } from "../../../wailsjs/go/models";
  import type { ReportCategory } from "$lib/types";
  import WabiSpinner from "$lib/components/ui/WabiSpinner.svelte";

  let activeCategory: ReportCategory = "sales";
  let loading = false;
  let exporting = false;
  let reportData: Record<string, any> | null = null;
  let dashboardMetrics: main.DashboardStats | null = null;
  let showExportModal = false;
  let exportFormat = "excel";
  let exportDateRange = {
    start: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000)
      .toISOString()
      .split("T")[0],
    end: new Date().toISOString().split("T")[0],
  };

  const categories: Array<{ id: ReportCategory; label: string; icon: string }> = [
    { id: "sales", label: "Sales", icon: "" },
    { id: "customers", label: "Customers", icon: "" },
    { id: "operations", label: "Operations", icon: "" },
    { id: "inventory", label: "Inventory", icon: "" },
    { id: "financial", label: "Financial", icon: "" },
  ];

  const reportCatalog = [
    { title: "P&L", detail: "Revenue, COGS, expenses, and operating profit", group: "Financial" },
    { title: "Balance Sheet", detail: "Cash, receivables, payables, and equity view", group: "Financial" },
    { title: "General Ledger", detail: "Filterable journal and operational source ledger", group: "Accounting" },
    { title: "Customer Ledger", detail: "Customer-specific invoices, receipts, credits, and balances", group: "Customers" },
    { title: "Supplier Ledger", detail: "Supplier invoices, settlements, and open payables", group: "Suppliers" },
    { title: "Expense Ledger", detail: "Approved, posted, paid, and locked expense records", group: "Expenses" },
    { title: "RFQ / Tender Tracker", detail: "Received RFQs, tenders, status, owner, and value", group: "Sales" },
    { title: "Offer Pipeline", detail: "Offers by status, value, customer, and prepared-by", group: "Sales" },
    { title: "Order Backlog", detail: "Open orders, partial fulfillment, delivery status", group: "Operations" },
    { title: "Inventory Movement", detail: "Inbound, outbound, adjustment, GRN, and DN movement", group: "Inventory" },
    { title: "VAT / Tax Pack", detail: "VAT filing support with export-ready evidence", group: "Audit" },
    { title: "Audit Pack", detail: "Document trail and filtered export for annual review", group: "Audit" },
  ];

  function dataValue(...keys: string[]) {
    if (!reportData) return undefined;
    for (const key of keys) {
      const value = reportData[key];
      if (value !== undefined && value !== null) return value;
    }
    return undefined;
  }

  function numberValue(...keys: string[]) {
    const value = dataValue(...keys);
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function formatBHD(value: number) {
    return `${value.toLocaleString(undefined, { maximumFractionDigits: 3 })} BHD`;
  }

  function formatPercent(value: number) {
    return `${(value * 100).toFixed(1)}%`;
  }

  async function loadReportData() {
    loading = true;
    reportData = null;
    try {
      dashboardMetrics = await GetDashboardStats().catch(() => null);
      // Simulate date range logic
      let dateRange = "month"; // simplified logic for now

      // Real API call - no mock fallback
      reportData = await GetReportData(activeCategory, dateRange);

      // Enrich financial data with dashboard metrics if available
      if (activeCategory === "financial" && dashboardMetrics) {
        if (!dataValue("cashBalance", "cash_balance") && dashboardMetrics.total_revenue) {
          reportData.cash_balance = dashboardMetrics.total_revenue;
        }
        if (!dataValue("runwayMonths", "runway_months") && dashboardMetrics.runway_months) {
          reportData.runway_months = dashboardMetrics.runway_months;
        }
      }
    } catch (error) {
      console.error("Failed to load report data:", error);
      toast.danger(`Failed to load ${activeCategory} report`);
      reportData = null;
    } finally {
      loading = false;
    }
  }

  async function handleExport() {
    exporting = true;
    try {
      const path = await ExportReport(
        activeCategory,
        exportFormat,
        JSON.stringify(reportData),
      );
      await OpenExportedFile(path).catch(() => {});
      toast.success(`Exported to ${path || "Documents"}`);
      showExportModal = false;
    } catch (e) {
      toast.danger("Export failed");
    } finally {
      exporting = false;
    }
  }

  onMount(loadReportData);
</script>

<div class="page">
  <header class="header">
    <div class="header-content">
      <h1>Reports & Exports</h1>
      <p class="subtitle">Operational reporting with export-ready finance packs</p>
    </div>
    <div class="actions">
      <div class="date-picker">
        <input
          type="date"
          bind:value={exportDateRange.start}
          class="input-date"
        />
        <span class="sep">to</span>
        <input
          type="date"
          bind:value={exportDateRange.end}
          class="input-date"
        />
        <button class="btn-icon" on:click={loadReportData}>Refresh</button>
      </div>
      <button class="btn-primary" on:click={() => (showExportModal = true)}
        >Export Report</button
      >
    </div>
  </header>

  <div class="tabs-bar">
    {#each categories as cat}
      <button
        class="tab"
        class:active={activeCategory === cat.id}
        on:click={() => {
          activeCategory = cat.id;
          loadReportData();
        }}
      >
        <span class="icon">{cat.icon}</span> <span>{cat.label}</span>
      </button>
    {/each}
  </div>

  <main class="main-panel">
    {#if loading}
      <div class="loading"><WabiSpinner size="lg" tempo="calm" /></div>
    {:else if reportData}
      <div class="report-container" in:fade>
        {#if activeCategory === "sales"}
          <div class="stats-row">
            <div class="stat-card">
              <div class="val">{formatPercent(numberValue("winRate", "win_rate"))}</div>
              <div class="lbl">Win Rate</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {formatPercent(numberValue("conversionRate", "conversion_rate"))}
              </div>
              <div class="lbl">Conversion</div>
            </div>
            <div class="stat-card">
              <div class="val">{formatBHD(numberValue("avgDealSize", "avg_deal_size"))}</div>
              <div class="lbl">Avg Deal Size</div>
            </div>
          </div>
          <div class="chart-panel">
            <h3>Pipeline Value</h3>
            <!-- Simple Bar Chart Visualization -->
            <div class="bars">
              {#each dataValue("pipeline") || [] as item}
                <div class="bar-row">
                  <span class="bar-lbl">{item.stage}</span>
                  <div class="bar-track">
                    <div
                      class="bar-fill"
                      style="width: {(item.value / 50000) * 100}%"
                    ></div>
                  </div>
                  <span class="bar-val">{item.value.toLocaleString()}</span>
                </div>
              {/each}
              {#if !(dataValue("pipeline") || []).length}
                <div class="empty-inline">No sales rows in this period.</div>
              {/if}
            </div>
          </div>
        {:else if activeCategory === "financial"}
          <div class="stats-row">
            <div class="stat-card" class:danger={numberValue("runwayMonths", "runway_months") > 0 && numberValue("runwayMonths", "runway_months") < 3}>
              <div class="val">{numberValue("runwayMonths", "runway_months").toFixed(1)} Months</div>
              <div class="lbl">Runway</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {formatBHD(numberValue("cashBalance", "cash_balance"))}
              </div>
              <div class="lbl">Cash Balance</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {formatBHD(numberValue("monthlyBurn", "monthly_burn"))}
              </div>
              <div class="lbl">Monthly Burn</div>
            </div>
            <div class="stat-card">
              <div class="val">{formatBHD(numberValue("collected"))}</div>
              <div class="lbl">Collected</div>
            </div>
            <div class="stat-card">
              <div class="val">{formatBHD(numberValue("collectionTarget", "collection_target"))}</div>
              <div class="lbl">Collection Target</div>
            </div>
          </div>
          <div class="chart-panel">
            <h3>Collection and reporting workspace</h3>
            <div class="progress-lg">
              <div
                class="prog-fill"
                style="width: {Math.min(100, (numberValue('mrr') / Math.max(1, numberValue('mrrTarget', 'mrr_target'))) * 100)}%"
              ></div>
              <span class="prog-text"
                >{formatBHD(numberValue("mrr"))} / {formatBHD(numberValue("mrrTarget", "mrr_target"))}</span
              >
            </div>
          </div>
          <div class="report-grid">
            {#each reportCatalog as report}
              <div class="report-card">
                <div>
                  <span class="report-group">{report.group}</span>
                  <h4>{report.title}</h4>
                  <p>{report.detail}</p>
                </div>
                <button class="btn-ghost small" on:click={() => (showExportModal = true)}>Export</button>
              </div>
            {/each}
          </div>
        {:else}
          <!-- Generic display for other categories to save space/time, following pattern -->
          <div class="stats-row">
            {#each Object.entries(reportData).filter(([k, v]) => typeof v === "number") as [k, v]}
              <div class="stat-card">
                <div class="val">
                  {typeof v === "number" && v > 1
                    ? v.toLocaleString()
                    : (v * 100).toFixed(0) + "%"}
                </div>
                <div class="lbl">{k.replace(/([A-Z])/g, " $1")}</div>
              </div>
            {/each}
            {#if !Object.entries(reportData).filter(([k, v]) => typeof v === "number").length}
              <div class="empty-panel">No report metrics are available for the selected period.</div>
            {/if}
          </div>
        {/if}
      </div>
    {:else}
      <div class="empty-panel">No report data loaded. Refresh to try again.</div>
    {/if}
  </main>
</div>

{#if showExportModal}
  <div class="modal-backdrop" transition:fade>
    <div class="modal">
      <h3>Export Report</h3>
      <p class="modal-helper">
        Operational report exports are Excel-compatible CSV files so finance users can filter, pivot, and share them immediately.
      </p>
      <div class="form-group">
        <span class="field-label">Format</span>
        <div class="toggle-group">
          <button
            class:active={exportFormat === "excel"}
            on:click={() => (exportFormat = "excel")}>Excel CSV</button
          >
          <button
            class:active={exportFormat === "csv"}
            on:click={() => (exportFormat = "csv")}>CSV</button
          >
        </div>
      </div>
      <div class="modal-actions">
        <button class="btn-ghost" on:click={() => (showExportModal = false)}
          >Cancel</button
        >
        <button
          class="btn-primary"
          on:click={handleExport}
          disabled={exporting}
        >
          {exporting ? "Exporting..." : "Download"}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .page {
    padding: 18px 20px;
    min-height: 0;
    background: var(--paper);
    color: var(--ink);
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    margin-bottom: 14px;
    flex-shrink: 0;
  }
  h1 {
    font-size: 22px;
    font-weight: 650;
    margin: 0;
    letter-spacing: 0;
  }
  .subtitle {
    color: var(--ink-faint);
    margin: 2px 0 0;
    font-size: var(--text-sm);
  }

  .actions {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  .date-picker {
    display: flex;
    align-items: center;
    background: var(--paper-subtle);
    padding: 4px 8px;
    border-radius: 8px;
    border: 1px solid var(--border-medium);
  }
  .input-date {
    background: transparent;
    border: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink);
  }
  .sep {
    padding: 0 8px;
    font-size: 11px;
    color: var(--ink-light);
  }
  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 0 4px;
  }

  .btn-primary {
    background: var(--ink);
    color: var(--paper);
    border: none;
    padding: 8px 16px;
    border-radius: var(--radius-pill);
    cursor: pointer;
    font-size: 13px;
  }

  .tabs-bar {
    display: flex;
    gap: 8px;
    margin-bottom: 14px;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 8px;
    flex-wrap: wrap;
  }
  .tab {
    background: transparent;
    border: 1px solid transparent;
    padding: 6px 14px;
    border-radius: 20px;
    cursor: pointer;
    display: flex;
    gap: 6px;
    align-items: center;
    color: var(--ink-light);
    font-size: 13px;
  }
  .tab:hover {
    background: var(--paper-subtle);
  }
  .tab.active {
    background: var(--ink);
    color: var(--paper);
  }
  .tab .icon {
    font-size: 14px;
  }

  .main-panel {
    flex: 1;
    overflow-y: auto;
  }
  .loading {
    display: flex;
    justify-content: center;
    padding-top: 40px;
  }

  .report-container {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .stats-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 12px;
  }
  .stat-card {
    background: var(--paper-subtle);
    padding: 14px;
    border-radius: 8px;
    border: 1px solid var(--border-subtle);
  }
  .stat-card .val {
    font-size: 24px;
    font-weight: 300;
    margin-bottom: 4px;
  }
  .stat-card .lbl {
    font-size: 10px;
    text-transform: uppercase;
    color: var(--ink-light);
    letter-spacing: 1px;
  }
  .stat-card.danger {
    background: #fee2e2;
    border-color: #fca5a5;
    color: #991b1b;
  }
  .stat-card.danger .lbl {
    color: #7f1d1d;
  }

  .chart-panel {
    background: var(--paper-subtle);
    padding: 14px;
    border-radius: 8px;
    border: 1px solid var(--border-subtle);
  }
  .chart-panel h3 {
    margin: 0 0 16px;
    font-size: 13px;
    text-transform: uppercase;
    color: var(--ink-light);
  }

  .bar-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
    font-size: 13px;
  }
  .bar-lbl {
    width: 100px;
    text-align: right;
  }
  .bar-track {
    flex: 1;
    height: 8px;
    background: rgba(0, 0, 0, 0.05);
    border-radius: 4px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    background: var(--ink);
  }
  .bar-val {
    width: 80px;
    font-family: var(--font-mono);
  }

  .progress-lg {
    height: 24px;
    background: rgba(0, 0, 0, 0.05);
    border-radius: 12px;
    position: relative;
    overflow: hidden;
  }

  .report-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 10px;
  }

  .report-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 86px;
    padding: 13px 14px;
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    background: #fff;
  }

  .report-card h4 {
    margin: 2px 0 4px;
    font-size: 15px;
    font-weight: 650;
  }

  .report-card p {
    margin: 0;
    color: var(--ink-light);
    font-size: 12px;
    line-height: 1.35;
  }

  .report-group {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--ink-faint);
    font-weight: 700;
  }

  .empty-panel,
  .empty-inline {
    padding: 18px;
    border: 1px dashed var(--border-medium);
    border-radius: 8px;
    color: var(--ink-light);
    background: var(--paper-subtle);
    text-align: center;
  }

  .empty-inline {
    padding: 10px;
    text-align: left;
    font-size: 13px;
  }
  .prog-fill {
    height: 100%;
    background: #059669;
  }
  .prog-text {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    color: #fff;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }

  /* Modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal {
    background: var(--paper);
    padding: 32px;
    border-radius: 16px;
    width: min(420px, calc(100vw - 32px));
  }
  .modal-helper {
    margin: 0 0 16px;
    color: var(--ink-light);
    font-size: 13px;
    line-height: 1.45;
  }
  .toggle-group {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }
  .toggle-group button {
    flex: 1;
    padding: 8px;
    border: 1px solid var(--border-medium);
    background: transparent;
    border-radius: 6px;
    cursor: pointer;
  }
  .toggle-group button.active {
    background: var(--ink);
    color: var(--paper);
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 24px;
  }
  .btn-ghost {
    background: transparent;
    border: none;
    cursor: pointer;
  }

  .btn-ghost.small {
    border: 1px solid var(--border-medium);
    border-radius: 7px;
    padding: 7px 10px;
    font-size: 12px;
    white-space: nowrap;
  }

  @media (max-width: 900px) {
    .header {
      align-items: flex-start;
      flex-direction: column;
    }
    .actions,
    .date-picker {
      width: 100%;
      flex-wrap: wrap;
    }
  }
</style>
