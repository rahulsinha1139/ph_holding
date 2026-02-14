<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import { toast } from "$lib/stores/toasts";
  import {
    GetDashboardStats,
    GetReportData,
    ExportReport,
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
  let exportFormat = "pdf";
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
        if (!reportData.cashBalance && dashboardMetrics.total_revenue) {
          reportData.cashBalance = dashboardMetrics.total_revenue;
        }
        if (!reportData.runwayMonths && dashboardMetrics.runway_months) {
          reportData.runwayMonths = dashboardMetrics.runway_months;
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
      <h1>Business Intelligence.</h1>
      <p class="subtitle">Reports & Analytics</p>
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
              <div class="val">{(reportData.winRate * 100).toFixed(1)}%</div>
              <div class="lbl">Win Rate</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {(reportData.conversionRate * 100).toFixed(1)}%
              </div>
              <div class="lbl">Conversion</div>
            </div>
            <div class="stat-card">
              <div class="val">{reportData.avgDealSize} BHD</div>
              <div class="lbl">Avg Deal Size</div>
            </div>
          </div>
          <div class="chart-panel">
            <h3>Pipeline Value</h3>
            <!-- Simple Bar Chart Visualization -->
            <div class="bars">
              {#each reportData.pipeline || [] as item}
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
            </div>
          </div>
        {:else if activeCategory === "financial"}
          <div class="stats-row">
            <div class="stat-card" class:danger={reportData.runwayMonths < 3}>
              <div class="val">{reportData.runwayMonths.toFixed(1)} Months</div>
              <div class="lbl">Runway</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {reportData.cashBalance.toLocaleString()} BHD
              </div>
              <div class="lbl">Cash Balance</div>
            </div>
            <div class="stat-card">
              <div class="val">
                {reportData.monthlyBurn.toLocaleString()} BHD
              </div>
              <div class="lbl">Monthly Burn</div>
            </div>
          </div>
          <div class="chart-panel">
            <h3>MRR vs Target</h3>
            <div class="progress-lg">
              <div
                class="prog-fill"
                style="width: {(reportData.mrr / reportData.mrrTarget) * 100}%"
              ></div>
              <span class="prog-text"
                >{reportData.mrr.toLocaleString()} / {reportData.mrrTarget.toLocaleString()}
                BHD</span
              >
            </div>
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
          </div>
        {/if}
      </div>
    {/if}
  </main>
</div>

{#if showExportModal}
  <div class="modal-backdrop" transition:fade>
    <div class="modal">
      <h3>Export Report</h3>
      <div class="form-group">
        <label>Format</label>
        <div class="toggle-group">
          <button
            class:active={exportFormat === "pdf"}
            on:click={() => (exportFormat = "pdf")}>PDF</button
          >
          <button
            class:active={exportFormat === "excel"}
            on:click={() => (exportFormat = "excel")}>Excel</button
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
    padding: var(--page-padding);
    height: 100vh;
    background: var(--paper);
    color: var(--ink);
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    margin-bottom: var(--space-4);
    flex-shrink: 0;
  }
  h1 {
    font-size: var(--text-3xl);
    font-weight: var(--font-weight-light);
    margin: 0;
    letter-spacing: -0.02em;
  }
  .subtitle {
    color: var(--ink-faint);
    margin-top: var(--space-1);
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
    margin-bottom: var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 8px;
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
    gap: var(--space-4);
  }

  .stats-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: var(--space-4);
  }
  .stat-card {
    background: var(--paper-subtle);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
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
    padding: var(--space-4);
    border-radius: var(--radius-lg);
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
    width: 320px;
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
</style>
