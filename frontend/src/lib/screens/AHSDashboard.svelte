<script lang="ts">
    /**
     * AHSDashboard - Financial dashboard for AHS Trading WLL (sister company)
     * E2: Shows division-filtered financial data for AHS Trading
     */
    import { onMount } from 'svelte';
    import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
    import { toast } from '$lib/stores/toasts';
    import { GetFinancialDashboardByDivision, GetFinancialReportYears } from '../../../wailsjs/go/main/App';

    export let embedded = false;

    let loading = true;
    let data: any = null;
    let availableYears: number[] = [];
    let selectedYear = new Date().getFullYear();

    const fmtCurrency = (val: number) =>
        new Intl.NumberFormat('en-BH', { style: 'currency', currency: 'BHD', minimumFractionDigits: 3, maximumFractionDigits: 3 }).format(val || 0);

    const fmtCompact = (val: number) => {
        if (val === null || val === undefined || val === 0) return 'BHD 0';
        const abs = Math.abs(val);
        const sign = val < 0 ? '-' : '';
        if (abs >= 1000000) return `${sign}BHD ${(abs / 1000000).toFixed(2)}M`;
        if (abs >= 1000) return `${sign}BHD ${(abs / 1000).toFixed(1)}K`;
        return `${sign}BHD ${abs.toFixed(0)}`;
    };

    async function loadYears() {
        try {
            const years = await GetFinancialReportYears();
            if (years && years.length > 0) {
                availableYears = years;
                selectedYear = years[0] || new Date().getFullYear();
            } else {
                availableYears = [new Date().getFullYear(), 2025, 2024, 2023];
            }
        } catch (err) {
            availableYears = [new Date().getFullYear(), 2025, 2024, 2023];
        }
    }

    async function loadDashboard() {
        loading = true;
        try {
            data = await GetFinancialDashboardByDivision(selectedYear, 'AHS Trading');
        } catch (err) {
            console.error('Failed to load AHS dashboard:', err);
            toast.danger(`Failed to load AHS Trading data for ${selectedYear}`);
            data = { division: 'AHS Trading', year: selectedYear, has_data: false };
        } finally {
            loading = false;
        }
    }

    $: if (selectedYear) {
        loadDashboard();
    }

    onMount(async () => {
        await loadYears();
        await loadDashboard();
    });
</script>

<div class="ahs-dashboard" data-embedded={embedded ? 'true' : undefined}>
    <!-- Year Selector -->
    <div class="controls">
        <div class="year-selector">
            <label for="ahs-year">Financial Year:</label>
            <select id="ahs-year" bind:value={selectedYear} class="year-select">
                {#each availableYears as year}
                    <option value={year}>FY{year}</option>
                {/each}
            </select>
        </div>
        <div class="company-badge">AHS Trading WLL</div>
    </div>

    {#if loading}
        <div class="loading-container">
            <WabiSpinner size="lg" />
        </div>
    {:else if !data || !data.has_data}
        <!-- No data state -->
        <div class="no-data-container">
            <div class="no-data-icon">
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="#d1d1d6" stroke-width="1.5">
                    <path d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
            </div>
            <h3>No Data for AHS Trading in FY{selectedYear}</h3>
            <p>
                AHS Trading financial data will appear here once invoices and orders
                are tagged with the "AHS Trading" division in the costing sheet.
            </p>
            <p class="hint">
                To tag data: Create a costing sheet with Division = "AHS Trading",
                then convert it to an offer and order. The division will carry through
                to invoices automatically.
            </p>
        </div>
    {:else}
        <!-- KPI Cards -->
        <div class="kpi-grid">
            <div class="kpi-card">
                <div class="kpi-label">Revenue</div>
                <div class="kpi-value">{fmtCompact(data.revenue)}</div>
                <div class="kpi-sub">{data.invoice_count} invoices</div>
            </div>
            <div class="kpi-card">
                <div class="kpi-label">Orders</div>
                <div class="kpi-value">{data.order_count}</div>
                <div class="kpi-sub">FY{selectedYear}</div>
            </div>
            <div class="kpi-card">
                <div class="kpi-label">Open Receivables</div>
                <div class="kpi-value">{fmtCompact(data.outstanding_ar)}</div>
                <div class="kpi-sub">{data.overdue_count} overdue receivables</div>
            </div>
            <div class="kpi-card">
                <div class="kpi-label">Receipts Collected</div>
                <div class="kpi-value">{fmtCompact(data.paid_amount)}</div>
                <div class="kpi-sub">Avg invoice: {fmtCurrency(data.avg_invoice_size)}</div>
            </div>
        </div>

        <!-- Summary Table -->
        <div class="summary-section">
            <h4>Financial Summary - AHS Trading WLL</h4>
            <table class="summary-table">
                <thead>
                    <tr>
                        <th>Metric</th>
                        <th class="number-col">Value</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>Invoiced Revenue</td>
                        <td class="number-col">{fmtCurrency(data.revenue)}</td>
                    </tr>
                    <tr>
                        <td>Invoice Count</td>
                        <td class="number-col">{data.invoice_count}</td>
                    </tr>
                    <tr>
                        <td>Order Count</td>
                        <td class="number-col">{data.order_count}</td>
                    </tr>
                    <tr>
                        <td>Payments Collected</td>
                        <td class="number-col">{fmtCurrency(data.paid_amount)}</td>
                    </tr>
                    <tr>
                        <td>Outstanding Receivables</td>
                        <td class="number-col">{fmtCurrency(data.outstanding_ar)}</td>
                    </tr>
                    <tr class:overdue={data.overdue_amount > 0}>
                        <td>Overdue Receivables</td>
                        <td class="number-col">{fmtCurrency(data.overdue_amount)}</td>
                    </tr>
                    <tr>
                        <td>Average Invoice Size</td>
                        <td class="number-col">{fmtCurrency(data.avg_invoice_size)}</td>
                    </tr>
                </tbody>
            </table>
        </div>
    {/if}
</div>

<style>
    .ahs-dashboard {
        max-width: 1200px;
    }

    .controls {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 24px;
    }

    .year-selector {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .year-selector label {
        font-size: 13px;
        font-weight: 500;
        color: #6e6e73;
    }

    .year-select {
        padding: 6px 12px;
        border: 1px solid #e5e5e5;
        border-radius: 6px;
        font-size: 13px;
        font-family: "DM Sans", sans-serif;
        outline: none;
        cursor: pointer;
    }

    .year-select:focus {
        border-color: #1d1d1f;
    }

    .company-badge {
        padding: 6px 12px;
        background: #f0f0f5;
        border-radius: 6px;
        font-size: 12px;
        font-weight: 600;
        color: #6e6e73;
        letter-spacing: 0.02em;
    }

    .loading-container {
        display: flex;
        justify-content: center;
        padding: 80px 0;
    }

    /* No Data State */
    .no-data-container {
        text-align: center;
        padding: 60px 40px;
        background: #ffffff;
        border: 1px solid #e5e5e5;
        border-radius: 12px;
    }

    .no-data-icon {
        margin-bottom: 16px;
    }

    .no-data-container h3 {
        font-family: "Arvo", "Georgia", serif;
        font-size: 18px;
        font-weight: 400;
        color: #1d1d1f;
        margin: 0 0 12px 0;
    }

    .no-data-container p {
        font-size: 14px;
        color: #6e6e73;
        margin: 0 0 8px 0;
        max-width: 500px;
        margin-left: auto;
        margin-right: auto;
        line-height: 1.5;
    }

    .hint {
        font-size: 12px !important;
        color: #8e8e93 !important;
        padding-top: 8px;
        border-top: 1px solid #f0f0f0;
        margin-top: 16px !important;
    }

    /* KPI Cards */
    .kpi-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 16px;
        margin-bottom: 24px;
    }

    .kpi-card {
        background: #ffffff;
        border: 1px solid #e5e5e5;
        border-radius: 12px;
        padding: 20px;
    }

    .kpi-label {
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: #8e8e93;
        margin-bottom: 8px;
    }

    .kpi-value {
        font-family: "Arvo", "Georgia", serif;
        font-size: 24px;
        font-weight: 400;
        color: #1d1d1f;
        margin-bottom: 4px;
    }

    .kpi-sub {
        font-size: 12px;
        color: #8e8e93;
    }

    /* Summary Table */
    .summary-section {
        background: #ffffff;
        border: 1px solid #e5e5e5;
        border-radius: 12px;
        padding: 20px;
    }

    .summary-section h4 {
        font-family: "Arvo", "Georgia", serif;
        font-size: 15px;
        font-weight: 400;
        color: #1d1d1f;
        margin: 0 0 16px 0;
    }

    .summary-table {
        width: 100%;
        border-collapse: collapse;
    }

    .summary-table th {
        padding: 8px 12px;
        text-align: left;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: #8e8e93;
        border-bottom: 1px solid #e5e5e5;
    }

    .summary-table td {
        padding: 10px 12px;
        font-size: 13px;
        color: #1d1d1f;
        border-bottom: 1px solid #f0f0f0;
    }

    .summary-table .number-col {
        text-align: right;
        font-family: "SF Mono", "Menlo", monospace;
        font-size: 13px;
    }

    .summary-table tr.overdue td {
        color: #ff3b30;
        font-weight: 500;
    }

    @media (max-width: 768px) {
        .kpi-grid {
            grid-template-columns: repeat(2, 1fr);
        }

        .controls {
            flex-direction: column;
            gap: 12px;
            align-items: flex-start;
        }
    }
</style>
