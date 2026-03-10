<script lang="ts">
    /**
     * FinancialDashboard - McKinsey Standard Executive View
     * Dynamically loads data from Tally imports or falls back to FY2024 baseline
     */
    import { onMount } from 'svelte';
    import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
    import { toast } from '$lib/stores/toasts';
    import { permissions } from '$lib/stores/authContext';
    import { GetFinancialDashboardForYear, GetFinancialReportYears, GetCashPosition } from '../../../wailsjs/go/main/App';

    $: canView = $permissions.includes('*') || $permissions.includes('finance:view') || $permissions.includes('finance:*');

    export let embedded = false;

    let loading = true;
    let data: any = null;
    let availableYears: number[] = [];
    let selectedYear = new Date().getFullYear(); // Default to current year
    let noDataForYear = false;
    let liveCashTotal = 0; // Sum of closing balances from all bank accounts

    const fmtCurrency = (val: number) =>
        new Intl.NumberFormat('en-BH', { style: 'currency', currency: 'BHD', minimumFractionDigits: 3, maximumFractionDigits: 3 }).format(val);

    // Compact currency for KPI cards - uses K/M for large numbers
    const fmtCompact = (val: number) => {
        if (val === null || val === undefined) return 'BHD 0';
        const abs = Math.abs(val);
        const sign = val < 0 ? '-' : '';
        if (abs >= 1000000) return `${sign}BHD ${(abs / 1000000).toFixed(2)}M`;
        if (abs >= 1000) return `${sign}BHD ${(abs / 1000).toFixed(1)}K`;
        return `${sign}BHD ${abs.toFixed(0)}`;
    };

    const fmtPct = (val: number) => `${val.toFixed(1)}%`;
    const fmtRatio = (val: number) => `${val.toFixed(2)}x`;

    // Years with Grant Thornton audited data from FS2024
    const AUDITED_YEARS = [2023, 2024];

    $: isAuditedYear = AUDITED_YEARS.includes(selectedYear);

    async function loadAvailableYears() {
        try {
            const years = await GetFinancialReportYears();
            if (years && years.length > 0) {
                availableYears = years;
                // Default to latest available year
                selectedYear = years[0] || new Date().getFullYear();
            } else {
                // Fallback: current year + audited years
                const curYear = new Date().getFullYear();
                availableYears = [curYear, 2025, 2024, 2023];
                selectedYear = curYear;
            }
        } catch (err) {
            console.error('Failed to load available years:', err);
            // Fallback: current year + audited years
            availableYears = [new Date().getFullYear(), 2025, 2024, 2023];
            selectedYear = 2024;
        }
    }

    async function loadDashboard() {
        loading = true;
        noDataForYear = false;
        try {
            const result = await GetFinancialDashboardForYear(selectedYear);
            data = result;

            // Check if we're using Tally data or fallback
            if (!availableYears.includes(selectedYear) && selectedYear !== 2024) {
                noDataForYear = true;
            }
        } catch (err) {
            console.error('Failed to load dashboard:', err);
            toast.danger(`Failed to load financial data for ${selectedYear}`);
            // Fallback to audited data
            data = {
                period: 'FY2024',
                prior_year: 'FY2023',
                as_of_date: '2024-12-31',
                revenue: 1605794,
                revenue_yoy: -69.3,
                cogs: 1302630,
                gross_profit: 289998,
                gross_margin: 18.2,
                opex: 255857,
                ebitda: 54960,
                ebitda_margin: 3.4,
                net_profit: 20715,
                net_margin: 1.3,
                total_assets: 1127325,
                current_assets: 626518,
                non_current_assets: 500807,
                total_liabilities: 165642,
                current_liabilities: 133256,
                total_equity: 961683,
                cash_and_equiv: 413295,
                fixed_deposits: 50000,
                total_liquidity: 463295,
                trade_receivables: 123393,
                inventory: 5104,
                trade_payables: 123882,
                working_capital: 493262,
                current_ratio: 4.7,
                quick_ratio: 4.7,
                cash_ratio: 3.1,
                debt_to_equity: 0.04,
                equity_ratio: 85.3,
                dso: 17,
                dio: 1.4,
                dpo: 34.7,
                cash_conv_cycle: -16.3,
                asset_turnover: 1.41,
                receivables_turn: 13.9,
                roa: 1.8,
                roe: 2.2,
                ar_current: 28848,
                ar_30_60: 26533,
                ar_60_90: 7090,
                ar_over_90: 53231,
                ar_overdue: 86854,
                ar_overdue_pct: 35,
                py_revenue: 5237432,
                py_gross_profit: 883580,
                py_net_profit: 672908,
                py_total_assets: 1232384,
            };
        } finally {
            loading = false;
        }
    }

    $: totalAR = data ? data.ar_current + data.ar_30_60 + data.ar_60_90 + data.ar_over_90 : 0;

    async function handleYearChange() {
        await loadDashboard();
    }

    async function loadCashPosition() {
        try {
            const pos = await GetCashPosition();
            if (pos && typeof pos.total_bhd === 'number') {
                liveCashTotal = pos.total_bhd + (pos.total_eur || 0);
            }
        } catch (err) {
            console.warn('GetCashPosition not available, using fallback:', err);
        }
    }

    onMount(async () => {
        await loadAvailableYears();
        await Promise.all([loadDashboard(), loadCashPosition()]);
    });
</script>

<div class="dashboard" class:embedded>
    {#if !canView}
        <div class="loading-state">
            <p style="color: var(--text-secondary); font-size: 14px;">You don't have permission to view financial data.</p>
        </div>
    {:else if loading}
        <div class="loading-state">
            <WabiSpinner size="lg" tempo="calm" />
            <p>Loading financial data...</p>
        </div>
    {:else if data}
        <!-- Header -->
        <div class="dashboard-header">
            <div class="header-left">
                <h2>Financial Performance</h2>
                <span class="period-badge">{data.period}</span>
                {#if noDataForYear}
                    <span class="no-data-badge">Using FY2024 baseline</span>
                {/if}
            </div>
            <div class="header-right">
                <div class="year-selector">
                    <label for="year-select">Fiscal Year:</label>
                    <select id="year-select" bind:value={selectedYear} on:change={handleYearChange}>
                        {#each availableYears as year}
                            <option value={year}>FY{year}</option>
                        {/each}
                    </select>
                </div>
                {#if isAuditedYear}
                    <span class="audit-badge audited">Audited by Grant Thornton</span>
                {:else}
                    <span class="audit-badge unaudited">From Tally Data</span>
                {/if}
            </div>
        </div>

        <!-- Primary KPIs Row -->
        <div class="kpi-row">
            <div class="kpi-card">
                <div class="kpi-label">Revenue</div>
                <div class="kpi-value">{fmtCompact(data.revenue)}</div>
                <div class="kpi-change" class:negative={data.revenue_yoy < 0} class:positive={data.revenue_yoy >= 0}>
                    {data.revenue_yoy >= 0 ? '+' : '-'} {fmtPct(Math.abs(data.revenue_yoy))} YoY
                </div>
            </div>
            <div class="kpi-card">
                <div class="kpi-label">Gross Profit</div>
                <div class="kpi-value">{fmtCompact(data.gross_profit)}</div>
                <div class="kpi-sub">Margin: {fmtPct(data.gross_margin)}</div>
            </div>
            <div class="kpi-card">
                <div class="kpi-label">Net Profit</div>
                <div class="kpi-value">{fmtCompact(data.net_profit)}</div>
                <div class="kpi-sub">Margin: {fmtPct(data.net_margin)}</div>
            </div>
            <div class="kpi-card highlight">
                <div class="kpi-label">Cash Position</div>
                <div class="kpi-value">{fmtCompact(liveCashTotal > 0 ? liveCashTotal : data.total_liquidity)}</div>
                <div class="kpi-sub">{liveCashTotal > 0 ? 'From bank statements' : `Ratio: ${fmtRatio(data.current_ratio)}`}</div>
            </div>
        </div>

        <!-- Main Grid -->
        <div class="main-grid">
            <!-- Balance Sheet Card -->
            <div class="card">
                <div class="card-header">
                    <h3>Balance Sheet</h3>
                    <span class="card-date">As of {data.as_of_date}</span>
                </div>
                <div class="card-body">
                    <div class="bs-section">
                        <div class="bs-title">Assets</div>
                        <div class="bs-row">
                            <span>Current Assets</span>
                            <span class="bs-val">{fmtCurrency(data.current_assets)}</span>
                        </div>
                        <div class="bs-bar">
                            <div class="bs-fill blue" style="width: {data.current_assets / data.total_assets * 100}%"></div>
                        </div>
                        <div class="bs-row">
                            <span>Non-Current Assets</span>
                            <span class="bs-val">{fmtCurrency(data.non_current_assets)}</span>
                        </div>
                        <div class="bs-bar">
                            <div class="bs-fill purple" style="width: {data.non_current_assets / data.total_assets * 100}%"></div>
                        </div>
                        <div class="bs-row total">
                            <span>Total Assets</span>
                            <span class="bs-val">{fmtCurrency(data.total_assets)}</span>
                        </div>
                    </div>
                    <div class="bs-divider"></div>
                    <div class="bs-section">
                        <div class="bs-title">Liabilities & Equity</div>
                        <div class="bs-row">
                            <span>Total Liabilities</span>
                            <span class="bs-val">{fmtCurrency(data.total_liabilities)}</span>
                        </div>
                        <div class="bs-bar">
                            <div class="bs-fill red" style="width: {data.total_liabilities / data.total_assets * 100}%"></div>
                        </div>
                        <div class="bs-row">
                            <span>Total Equity</span>
                            <span class="bs-val">{fmtCurrency(data.total_equity)}</span>
                        </div>
                        <div class="bs-bar">
                            <div class="bs-fill green" style="width: {data.total_equity / data.total_assets * 100}%"></div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Key Ratios Card -->
            <div class="card">
                <div class="card-header">
                    <h3>Key Financial Ratios</h3>
                </div>
                <div class="card-body">
                    <div class="ratios-grid">
                        <div class="ratio-section">
                            <div class="ratio-title">Liquidity</div>
                            <div class="ratio-row">
                                <span>Current Ratio</span>
                                <span class="ratio-val good">{fmtRatio(data.current_ratio)}</span>
                            </div>
                            <div class="ratio-row">
                                <span>Quick Ratio</span>
                                <span class="ratio-val good">{fmtRatio(data.quick_ratio)}</span>
                            </div>
                            <div class="ratio-row">
                                <span>Cash Ratio</span>
                                <span class="ratio-val good">{fmtRatio(data.cash_ratio)}</span>
                            </div>
                        </div>
                        <div class="ratio-section">
                            <div class="ratio-title">Solvency</div>
                            <div class="ratio-row">
                                <span>Debt/Equity</span>
                                <span class="ratio-val good">{fmtRatio(data.debt_to_equity)}</span>
                            </div>
                            <div class="ratio-row">
                                <span>Equity Ratio</span>
                                <span class="ratio-val good">{fmtPct(data.equity_ratio)}</span>
                            </div>
                        </div>
                        <div class="ratio-section">
                            <div class="ratio-title">Efficiency</div>
                            <div class="ratio-row">
                                <span>DSO</span>
                                <span class="ratio-val good">{typeof data.dso === 'number' ? data.dso.toFixed(1) : data.dso} days</span>
                            </div>
                            <div class="ratio-row">
                                <span>Asset Turnover</span>
                                <span class="ratio-val">{fmtRatio(data.asset_turnover)}</span>
                            </div>
                        </div>
                        <div class="ratio-section">
                            <div class="ratio-title">Profitability</div>
                            <div class="ratio-row">
                                <span>Gross Margin</span>
                                <span class="ratio-val">{fmtPct(data.gross_margin)}</span>
                            </div>
                            <div class="ratio-row">
                                <span>ROE</span>
                                <span class="ratio-val caution">{fmtPct(data.roe)}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- AR Aging Card -->
            <div class="card">
                <div class="card-header">
                    <h3>Receivables Aging</h3>
                    <span class="card-badge warning">{fmtPct(data.ar_overdue_pct)} overdue</span>
                </div>
                <div class="card-body">
                    <div class="ar-total">
                        <span>Total AR</span>
                        <span class="ar-total-val">{fmtCurrency(totalAR)}</span>
                    </div>
                    <div class="ar-buckets">
                        <div class="ar-bucket">
                            <div class="ar-bar-wrap">
                                <div class="ar-bar green" style="height: {data.ar_current / totalAR * 100}%"></div>
                            </div>
                            <div class="ar-label">0-30d</div>
                            <div class="ar-val">{fmtCurrency(data.ar_current)}</div>
                        </div>
                        <div class="ar-bucket">
                            <div class="ar-bar-wrap">
                                <div class="ar-bar lime" style="height: {data.ar_30_60 / totalAR * 100}%"></div>
                            </div>
                            <div class="ar-label">31-60d</div>
                            <div class="ar-val">{fmtCurrency(data.ar_30_60)}</div>
                        </div>
                        <div class="ar-bucket">
                            <div class="ar-bar-wrap">
                                <div class="ar-bar orange" style="height: {data.ar_60_90 / totalAR * 100}%"></div>
                            </div>
                            <div class="ar-label">61-90d</div>
                            <div class="ar-val">{fmtCurrency(data.ar_60_90)}</div>
                        </div>
                        <div class="ar-bucket">
                            <div class="ar-bar-wrap">
                                <div class="ar-bar red" style="height: {data.ar_over_90 / totalAR * 100}%"></div>
                            </div>
                            <div class="ar-label">>90d</div>
                            <div class="ar-val">{fmtCurrency(data.ar_over_90)}</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Working Capital Card -->
            <div class="card">
                <div class="card-header">
                    <h3>Working Capital</h3>
                </div>
                <div class="card-body">
                    <div class="wc-rows">
                        <div class="wc-row">
                            <span>Trade Receivables</span>
                            <span class="wc-val">{fmtCurrency(data.trade_receivables)}</span>
                        </div>
                        <div class="wc-row">
                            <span>Inventory</span>
                            <span class="wc-val">{fmtCurrency(data.inventory)}</span>
                        </div>
                        <div class="wc-row">
                            <span>Trade Payables</span>
                            <span class="wc-val">({fmtCurrency(data.trade_payables)})</span>
                        </div>
                        <div class="wc-row total">
                            <span>Net Working Capital</span>
                            <span class="wc-val">{fmtCurrency(data.working_capital)}</span>
                        </div>
                    </div>
                    <div class="ccc-box">
                        <div class="ccc-title">Cash Conversion Cycle</div>
                        <div class="ccc-formula">
                            <span class="ccc-item">{typeof data.dso === 'number' ? data.dso.toFixed(1) : data.dso}d DSO</span>
                            <span class="ccc-op">+</span>
                            <span class="ccc-item">{typeof data.dio === 'number' ? data.dio.toFixed(1) : data.dio}d DIO</span>
                            <span class="ccc-op">-</span>
                            <span class="ccc-item">{typeof data.dpo === 'number' ? data.dpo.toFixed(1) : data.dpo}d DPO</span>
                            <span class="ccc-op">=</span>
                            <span class="ccc-result">{typeof data.cash_conv_cycle === 'number' ? data.cash_conv_cycle.toFixed(1) : data.cash_conv_cycle}d</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- YoY Comparison -->
        <div class="card wide">
            <div class="card-header">
                <h3>Year-over-Year Comparison</h3>
                <span class="card-date">{data.period} vs {data.prior_year}</span>
            </div>
            <div class="card-body">
                <div class="yoy-grid">
                    <div class="yoy-item">
                        <div class="yoy-label">Revenue</div>
                        <div class="yoy-compare">
                            <div class="yoy-row">
                                <span class="yoy-year">{data.prior_year}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar gray" style="width: 100%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.py_revenue)}</span>
                            </div>
                            <div class="yoy-row">
                                <span class="yoy-year">{data.period}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar blue" style="width: {data.py_revenue > 0 ? Math.min(data.revenue / data.py_revenue * 100, 100) : 0}%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.revenue)}</span>
                            </div>
                        </div>
                        <div class="yoy-change" class:negative={data.revenue_yoy < 0} class:positive={data.revenue_yoy >= 0}>
                            {data.revenue_yoy >= 0 ? '+' : '-'} {fmtPct(Math.abs(data.revenue_yoy))}
                        </div>
                    </div>
                    <div class="yoy-item">
                        <div class="yoy-label">Gross Profit</div>
                        <div class="yoy-compare">
                            <div class="yoy-row">
                                <span class="yoy-year">{data.prior_year}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar gray" style="width: 100%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.py_gross_profit)}</span>
                            </div>
                            <div class="yoy-row">
                                <span class="yoy-year">{data.period}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar blue" style="width: {data.py_gross_profit > 0 ? Math.min(data.gross_profit / data.py_gross_profit * 100, 100) : 0}%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.gross_profit)}</span>
                            </div>
                        </div>
                        <div class="yoy-change" class:negative={data.gross_profit < data.py_gross_profit} class:positive={data.gross_profit >= data.py_gross_profit}>
                            {data.py_gross_profit > 0 ? `${data.gross_profit >= data.py_gross_profit ? '+' : '-'} ${fmtPct(Math.abs((data.gross_profit - data.py_gross_profit) / data.py_gross_profit * 100))}` : '—'}
                        </div>
                    </div>
                    <div class="yoy-item">
                        <div class="yoy-label">Net Profit</div>
                        <div class="yoy-compare">
                            <div class="yoy-row">
                                <span class="yoy-year">{data.prior_year}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar gray" style="width: 100%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.py_net_profit)}</span>
                            </div>
                            <div class="yoy-row">
                                <span class="yoy-year">{data.period}</span>
                                <div class="yoy-bar-bg">
                                    <div class="yoy-bar blue" style="width: {data.py_net_profit > 0 ? Math.min(data.net_profit / data.py_net_profit * 100, 100) : 0}%"></div>
                                </div>
                                <span class="yoy-val">{fmtCurrency(data.net_profit)}</span>
                            </div>
                        </div>
                        <div class="yoy-change" class:negative={data.net_profit < data.py_net_profit} class:positive={data.net_profit >= data.py_net_profit}>
                            {data.py_net_profit > 0 ? `${data.net_profit >= data.py_net_profit ? '+' : '-'} ${fmtPct(Math.abs((data.net_profit - data.py_net_profit) / data.py_net_profit * 100))}` : '—'}
                        </div>
                    </div>
                </div>
                <div class="yoy-note">
                    {#if selectedYear === 2024}
                        <strong>Note:</strong> FY2023 was anomalous (EWA mega-contract = 93% revenue). FY2024 represents normal business operations. Structural margins remain healthy at {data.gross_margin?.toFixed(1) || '18.2'}% gross.
                    {:else if selectedYear === 2025}
                        <strong>Note:</strong> FY2025 data is from Tally imports (unaudited). Revenue decline reflects return to normal operations post-EWA contract.
                    {:else if selectedYear === 2023}
                        <strong>Note:</strong> FY2023 included the EWA smart meter mega-contract (93% of revenue). This was a one-time project, not recurring business.
                    {:else if selectedYear === 2026}
                        <strong>Note:</strong> FY2026 estimates based on active offer pipeline and bank statement data. Revenue projections use 40% pipeline conversion rate.
                    {:else}
                        <strong>Note:</strong> Comparing {data.period} to {data.prior_year}. All figures in BHD.
                    {/if}
                </div>
            </div>
        </div>

        <!-- Footer -->
        <div class="dashboard-footer">
            <span>Source: {data?.source || (isAuditedYear ? 'Audited Financial Statements FS2024' : `Live Data FY${selectedYear}`)}</span>
            {#if isAuditedYear}
                <span>Auditor: Grant Thornton - Abdulaal (May 2025)</span>
            {:else}
                <span>Note: FY{selectedYear} data pending audit</span>
            {/if}
        </div>
    {/if}
</div>

<style>
    .dashboard {
        display: flex;
        flex-direction: column;
        gap: 20px;
    }

    .loading-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        min-height: 400px;
        gap: 16px;
        color: var(--text-secondary);
    }

    /* Header */
    .dashboard-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding-bottom: 16px;
        border-bottom: 1px solid var(--border);
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .header-left h2 {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
        color: var(--onyx, #1D1D1F);
    }

    .header-right {
        display: flex;
        align-items: center;
        gap: 16px;
    }

    .period-badge {
        background: var(--carbon, #000);
        color: var(--canvas, #FFF);
        padding: 4px 12px;
        border-radius: 4px;
        font-size: 12px;
        font-weight: 600;
    }

    .no-data-badge {
        background: var(--steel, #86868B);
        color: var(--canvas, #FFF);
        padding: 4px 10px;
        border-radius: 4px;
        font-size: 11px;
        font-weight: 500;
    }

    .year-selector {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .year-selector label {
        font-size: 12px;
        font-weight: 500;
        color: var(--steel, #86868B);
    }

    .year-selector select {
        padding: 6px 12px;
        background: var(--canvas, #FFF);
        border: 1px solid var(--border, #E5E5E5);
        border-radius: 4px;
        font-size: 13px;
        font-weight: 500;
        color: var(--onyx, #1D1D1F);
        cursor: pointer;
        transition: all 120ms ease;
    }

    .year-selector select:hover {
        border-color: var(--onyx, #1D1D1F);
    }

    .year-selector select:focus {
        outline: none;
        border-color: var(--carbon, #000);
        box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.1);
    }

    .audit-badge {
        font-size: 11px;
        padding: 4px 10px;
        border-radius: 4px;
        font-weight: 500;
    }

    .audit-badge.audited {
        background: rgba(21, 128, 61, 0.12);
        color: #15803d;
    }

    .audit-badge.unaudited {
        background: rgba(217, 119, 6, 0.12);
        color: #d97706;
    }

    /* KPI Row */
    .kpi-row {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 16px;
    }

    .kpi-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 20px;
        display: flex;
        flex-direction: column;
        gap: 8px;
        min-width: 0; /* Allow shrinking in grid */
        overflow: hidden; /* Prevent overflow */
    }

    .kpi-card.highlight {
        background: var(--brand-indigo-tint, rgba(99, 102, 241, 0.08));
        border-color: var(--brand-indigo);
    }

    .kpi-label {
        font-size: 12px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        color: var(--text-muted);
        font-weight: 500;
    }

    .kpi-value {
        font-size: clamp(18px, 2.5vw, 28px); /* Responsive font size */
        font-weight: 700;
        font-family: var(--font-mono, monospace);
        color: var(--text-primary);
        word-break: break-word;
        overflow-wrap: break-word;
        line-height: 1.2;
    }

    .kpi-sub {
        font-size: 13px;
        color: var(--text-secondary);
    }

    .kpi-change {
        font-size: 13px;
        font-weight: 500;
    }

    .kpi-change.positive { color: #15803d; }
    .kpi-change.negative { color: #ef4444; }

    /* Main Grid */
    .main-grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 16px;
    }

    /* Cards */
    .card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 8px;
        overflow: hidden;
    }

    .card.wide {
        grid-column: span 2;
    }

    .card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px 20px;
        background: var(--bg-base);
        border-bottom: 1px solid var(--border);
    }

    .card-header h3 {
        margin: 0;
        font-size: 14px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .card-date {
        font-size: 12px;
        color: var(--text-muted);
    }

    .card-badge {
        font-size: 11px;
        padding: 3px 8px;
        border-radius: 4px;
        font-weight: 500;
    }

    .card-badge.warning {
        background: rgba(217, 119, 6, 0.15);
        color: #d97706;
    }

    .card-body {
        padding: 20px;
    }

    /* Balance Sheet */
    .bs-section {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .bs-title {
        font-size: 12px;
        font-weight: 600;
        text-transform: uppercase;
        color: var(--text-muted);
        margin-bottom: 4px;
    }

    .bs-row {
        display: flex;
        justify-content: space-between;
        font-size: 13px;
    }

    .bs-row.total {
        font-weight: 600;
        margin-top: 8px;
        padding-top: 8px;
        border-top: 1px solid var(--border);
    }

    .bs-val {
        font-family: var(--font-mono, monospace);
    }

    .bs-bar {
        height: 8px;
        background: var(--bg-base);
        border-radius: 4px;
        overflow: hidden;
    }

    .bs-fill {
        height: 100%;
        border-radius: 4px;
    }

    .bs-fill.blue { background: var(--brand-indigo); }
    .bs-fill.purple { background: #8b5cf6; }
    .bs-fill.red { background: #ef4444; }
    .bs-fill.green { background: #15803d; }

    .bs-divider {
        height: 1px;
        background: var(--border);
        margin: 16px 0;
    }

    /* Ratios */
    .ratios-grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 20px;
    }

    .ratio-section {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .ratio-title {
        font-size: 11px;
        font-weight: 600;
        text-transform: uppercase;
        color: var(--text-muted);
        padding-bottom: 6px;
        border-bottom: 1px solid var(--border);
    }

    .ratio-row {
        display: flex;
        justify-content: space-between;
        font-size: 13px;
    }

    .ratio-val {
        font-family: var(--font-mono, monospace);
        font-weight: 500;
    }

    .ratio-val.good { color: #15803d; }
    .ratio-val.caution { color: #d97706; }

    /* AR Aging */
    .ar-total {
        display: flex;
        justify-content: space-between;
        margin-bottom: 20px;
        font-size: 14px;
    }

    .ar-total-val {
        font-weight: 600;
        font-family: var(--font-mono, monospace);
    }

    .ar-buckets {
        display: flex;
        justify-content: space-around;
        align-items: flex-end;
        height: 140px;
        padding-top: 20px;
    }

    .ar-bucket {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 8px;
        flex: 1;
    }

    .ar-bar-wrap {
        width: 40px;
        height: 100px;
        background: var(--bg-base);
        border-radius: 4px;
        display: flex;
        align-items: flex-end;
        overflow: hidden;
    }

    .ar-bar {
        width: 100%;
        border-radius: 4px 4px 0 0;
        transition: height 0.3s ease;
    }

    .ar-bar.green { background: #15803d; }
    .ar-bar.lime { background: #22c55e; }
    .ar-bar.orange { background: #d97706; }
    .ar-bar.red { background: #ef4444; }

    .ar-label {
        font-size: 11px;
        color: var(--text-muted);
    }

    .ar-val {
        font-size: 11px;
        font-family: var(--font-mono, monospace);
    }

    /* Working Capital */
    .wc-rows {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .wc-row {
        display: flex;
        justify-content: space-between;
        font-size: 13px;
    }

    .wc-row.total {
        font-weight: 600;
        margin-top: 8px;
        padding-top: 8px;
        border-top: 1px solid var(--border);
    }

    .wc-val {
        font-family: var(--font-mono, monospace);
    }

    .ccc-box {
        margin-top: 20px;
        padding: 16px;
        background: var(--bg-base);
        border-radius: 8px;
        text-align: center;
    }

    .ccc-title {
        font-size: 11px;
        text-transform: uppercase;
        color: var(--text-muted);
        margin-bottom: 12px;
    }

    .ccc-formula {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 8px;
        flex-wrap: wrap;
    }

    .ccc-item {
        font-size: 13px;
        font-family: var(--font-mono, monospace);
    }

    .ccc-op {
        color: var(--text-muted);
    }

    .ccc-result {
        font-size: 16px;
        font-weight: 700;
        color: var(--brand-indigo);
        font-family: var(--font-mono, monospace);
    }

    /* YoY Comparison */
    .yoy-grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 24px;
    }

    .yoy-item {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .yoy-label {
        font-size: 13px;
        font-weight: 500;
    }

    .yoy-compare {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .yoy-row {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .yoy-year {
        font-size: 11px;
        color: var(--text-muted);
        width: 50px;
    }

    .yoy-bar-bg {
        flex: 1;
        height: 16px;
        background: var(--bg-base);
        border-radius: 4px;
        overflow: hidden;
    }

    .yoy-bar {
        height: 100%;
        border-radius: 4px;
    }

    .yoy-bar.gray { background: #94a3b8; }
    .yoy-bar.blue { background: var(--brand-indigo); }

    .yoy-val {
        font-size: 11px;
        font-family: var(--font-mono, monospace);
        width: 90px;
        text-align: right;
    }

    .yoy-change {
        font-size: 13px;
        font-weight: 600;
        text-align: center;
    }

    .yoy-change.positive { color: #15803d; }
    .yoy-change.negative { color: #ef4444; }

    .yoy-note {
        margin-top: 20px;
        padding: 12px 16px;
        background: rgba(217, 119, 6, 0.1);
        border-radius: 6px;
        font-size: 12px;
        color: #92400e;
    }

    /* Footer */
    .dashboard-footer {
        display: flex;
        justify-content: space-between;
        font-size: 11px;
        color: var(--text-muted);
        padding-top: 16px;
        border-top: 1px solid var(--border);
    }

    @media (max-width: 1024px) {
        .kpi-row {
            grid-template-columns: repeat(2, 1fr);
        }

        .main-grid {
            grid-template-columns: 1fr;
        }

        .card.wide {
            grid-column: span 1;
        }

        .yoy-grid {
            grid-template-columns: 1fr;
        }
    }
</style>
