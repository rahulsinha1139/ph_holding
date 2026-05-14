<script lang="ts">
    import type { EChartsOption } from "echarts";
    import type { RevenueChartData } from "$lib/types";
    import EChartsSurface from "./EChartsSurface.svelte";

    export let data: RevenueChartData[] = [];
    export let loading = false;

    let hoveredId: string | null = null;

    $: rows = [...data]
        .sort((a, b) => Number(b.revenue || 0) - Number(a.revenue || 0))
        .slice(0, 8);
    $: totalRevenue = rows.reduce((sum, row) => sum + Number(row.revenue || 0), 0);
    $: totalInvoices = rows.reduce((sum, row) => sum + Number(row.invoice_count || 0), 0);
    $: selected = hoveredId ? rows.find((row) => row.customer_id === hoveredId) || rows[0] : rows[0];
    $: revenueOption = buildRevenueOption(rows);

    function buildRevenueOption(chartRows: RevenueChartData[]): EChartsOption {
        const palette = ["#2F7638", "#3E8F47", "#5F9F67", "#7DAE83", "#9FBEA2", "#B8CDBA", "#C8D8C8", "#D7E4D7"];
        return {
            backgroundColor: "transparent",
            color: palette,
            aria: { enabled: true },
            animationDuration: 650,
            animationEasing: "cubicOut",
            grid: { top: 8, right: 18, bottom: 22, left: 150, containLabel: false },
            tooltip: {
                trigger: "axis",
                axisPointer: { type: "shadow", shadowStyle: { color: "rgba(62, 143, 71, 0.08)" } },
                backgroundColor: "rgba(37, 49, 43, 0.96)",
                borderWidth: 0,
                padding: [10, 12],
                textStyle: { color: "#FFFFFF", fontSize: 12, fontWeight: 600 },
                formatter(params: any) {
                    const item = Array.isArray(params) ? params[0] : params;
                    const row = chartRows[item?.dataIndex || 0];
                    if (!row) return "";
                    return [
                        `<strong>${escapeHtml(row.customer_name || "Unknown customer")}</strong>`,
                        `Revenue: ${formatCurrency(row.revenue)}`,
                        `Invoices: ${(row.invoice_count || 0).toLocaleString("en-US")}`,
                        `Share: ${share(row.revenue).toFixed(1)}%`,
                    ].join("<br/>");
                },
            },
            xAxis: {
                type: "value",
                axisLabel: { color: "#7B8A7D", formatter: (value: number) => formatCompactCurrency(value) },
                splitLine: { lineStyle: { color: "rgba(93, 116, 94, 0.12)" } },
                axisLine: { show: false },
                axisTick: { show: false },
            },
            yAxis: {
                type: "category",
                inverse: true,
                data: chartRows.map((row) => cleanName(row.customer_name, 24)),
                axisLabel: {
                    color: "#25312B",
                    fontSize: 11,
                    fontWeight: 700,
                    overflow: "truncate",
                    width: 136,
                },
                axisLine: { show: false },
                axisTick: { show: false },
            },
            series: [
                {
                    name: "Revenue",
                    type: "bar",
                    realtimeSort: false,
                    barWidth: 16,
                    itemStyle: {
                        borderRadius: [0, 8, 8, 0],
                        color(params: any) {
                            return palette[params.dataIndex % palette.length];
                        },
                    },
                    emphasis: {
                        focus: "series",
                        itemStyle: { shadowBlur: 16, shadowColor: "rgba(62, 143, 71, 0.25)" },
                    },
                    data: chartRows.map((row) => ({
                        value: Number(row.revenue || 0),
                        customerId: row.customer_id,
                    })),
                },
            ],
        };
    }

    function handleHover(event: CustomEvent<any>) {
        const row = rows[event.detail?.dataIndex || 0];
        hoveredId = row?.customer_id || null;
    }

    function handleLeave() {
        hoveredId = null;
    }

    function share(value: number): number {
        if (totalRevenue <= 0) return 0;
        return (Number(value || 0) / totalRevenue) * 100;
    }

    function formatCurrency(value: number): string {
        const amount = Number(value || 0);
        if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}M BHD`;
        if (amount >= 1_000) return `${Math.round(amount / 1_000).toLocaleString("en-US")}k BHD`;
        return `${amount.toLocaleString("en-US", { maximumFractionDigits: 0 })} BHD`;
    }

    function formatCompactCurrency(value: number): string {
        const amount = Number(value || 0);
        if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}M`;
        if (amount >= 1_000) return `${Math.round(amount / 1_000).toLocaleString("en-US")}k`;
        return amount.toLocaleString("en-US", { maximumFractionDigits: 0 });
    }

    function cleanName(name: string, length = 34): string {
        const compact = String(name || "Unknown customer").replace(/\s+/g, " ").trim();
        return compact.length > length ? `${compact.slice(0, length - 3)}...` : compact;
    }

    function escapeHtml(value: string): string {
        return String(value || "")
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;");
    }
</script>

<div class="executive-revenue-chart">
    {#if loading}
        <div class="chart-state">Loading revenue portfolio...</div>
    {:else if rows.length === 0}
        <div class="chart-state">No revenue data available</div>
    {:else}
        <div class="chart-summary">
            <div>
                <span>Visible revenue</span>
                <strong>{formatCurrency(totalRevenue)}</strong>
            </div>
            <div>
                <span>Invoice count</span>
                <strong>{totalInvoices.toLocaleString("en-US")}</strong>
            </div>
            <div>
                <span>Top account share</span>
                <strong>{share(rows[0]?.revenue || 0).toFixed(1)}%</strong>
            </div>
        </div>

        <div class="chart-body">
            <div class="chart-frame">
                <EChartsSurface
                    option={revenueOption}
                    height={292}
                    ariaLabel="Top customer revenue interactive ranking"
                    on:hover={handleHover}
                    on:leave={handleLeave}
                />
            </div>

            <aside class="selected-account">
                <span>Selected account</span>
                <strong>{cleanName(selected?.customer_name || rows[0].customer_name)}</strong>
                <div class="account-metric">
                    <span>Revenue</span>
                    <b>{formatCurrency(selected?.revenue || 0)}</b>
                </div>
                <div class="account-metric">
                    <span>Share</span>
                    <b>{share(selected?.revenue || 0).toFixed(1)}%</b>
                </div>
                <div class="account-metric">
                    <span>Invoices</span>
                    <b>{(selected?.invoice_count || 0).toLocaleString("en-US")}</b>
                </div>
            </aside>
        </div>

        <div class="revenue-table-wrap">
            <table aria-label="Customer revenue table">
                <thead>
                    <tr>
                        <th>Customer</th>
                        <th>Revenue</th>
                        <th>Invoices</th>
                        <th>Share</th>
                    </tr>
                </thead>
                <tbody>
                    {#each rows as row}
                        <tr class:active={hoveredId === row.customer_id}>
                            <td title={row.customer_name || "Unknown customer"}>
                                {cleanName(row.customer_name, 44)}
                            </td>
                            <td>{formatCurrency(row.revenue)}</td>
                            <td>{(row.invoice_count || 0).toLocaleString("en-US")}</td>
                            <td>{share(row.revenue).toFixed(1)}%</td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>

<style>
    .executive-revenue-chart {
        width: 100%;
        min-height: 300px;
    }

    .chart-state {
        min-height: 280px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--text-muted, #8A978B);
        font-size: 12px;
        border: 1px dashed var(--border, #DDE8DD);
        border-radius: 8px;
        background: #F7FAF7;
    }

    .chart-summary {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: 10px;
        margin-bottom: 16px;
    }

    .chart-summary div {
        padding: 12px 14px;
        border: 1px solid var(--border, #DDE8DD);
        border-radius: 8px;
        background: linear-gradient(180deg, #FFFFFF 0%, #F7FAF7 100%);
    }

    .chart-summary span,
    .selected-account span,
    .account-metric span {
        display: block;
        color: var(--text-muted, #8A978B);
        font-size: 10px;
        font-weight: 800;
        text-transform: uppercase;
        letter-spacing: 0;
    }

    .chart-summary strong {
        display: block;
        margin-top: 5px;
        color: var(--onyx, #25312B);
        font-size: 17px;
        font-weight: 800;
    }

    .chart-body {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 220px;
        gap: 18px;
        align-items: stretch;
    }

    .chart-frame {
        min-width: 0;
        border: 1px solid rgba(93, 116, 94, 0.12);
        border-radius: 8px;
        background: linear-gradient(180deg, #FFFFFF 0%, #F8FBF8 100%);
        padding: 8px 4px 2px 0;
    }

    .selected-account {
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 16px;
        border: 1px solid #CFE0CF;
        border-radius: 8px;
        background:
            linear-gradient(180deg, rgba(62, 143, 71, 0.1), rgba(62, 143, 71, 0.025)),
            #FFFFFF;
    }

    .revenue-table-wrap {
        margin-top: 16px;
        border: 1px solid var(--border, #DDE8DD);
        border-radius: 8px;
        overflow: hidden;
        background: #FFFFFF;
    }

    table {
        width: 100%;
        border-collapse: collapse;
        table-layout: fixed;
    }

    th,
    td {
        padding: 10px 12px;
        border-bottom: 1px solid rgba(93, 116, 94, 0.12);
        font-size: 12px;
        line-height: 1.25;
        text-align: right;
        white-space: nowrap;
    }

    th {
        color: var(--text-muted, #8A978B);
        background: #F7FAF7;
        font-size: 10px;
        font-weight: 800;
        letter-spacing: 0;
        text-transform: uppercase;
    }

    th:first-child,
    td:first-child {
        width: 48%;
        text-align: left;
    }

    td {
        color: var(--text-secondary, #667569);
        font-weight: 750;
        font-variant-numeric: tabular-nums;
    }

    td:first-child {
        overflow: hidden;
        color: var(--onyx, #25312B);
        font-weight: 800;
        text-overflow: ellipsis;
    }

    tbody tr:last-child td {
        border-bottom: none;
    }

    tbody tr.active td,
    tbody tr:hover td {
        background: rgba(62, 143, 71, 0.06);
    }

    .selected-account strong {
        color: var(--onyx, #25312B);
        font-size: 17px;
        line-height: 1.25;
        font-weight: 800;
    }

    .account-metric {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        padding-top: 10px;
        border-top: 1px solid rgba(93, 116, 94, 0.16);
    }

    .account-metric b {
        color: var(--brand-indigo, #3E8F47);
        font-size: 13px;
        font-weight: 800;
        white-space: nowrap;
    }

    @media (max-width: 1040px) {
        .chart-body {
            grid-template-columns: 1fr;
        }

        .selected-account {
            display: grid;
            grid-template-columns: 1.5fr repeat(3, 1fr);
            align-items: center;
        }

        .account-metric {
            border-top: none;
            padding-top: 0;
        }
    }

    @media (max-width: 760px) {
        .chart-summary {
            grid-template-columns: 1fr;
        }

        .selected-account {
            grid-template-columns: 1fr;
        }

        th,
        td {
            padding: 9px 8px;
            font-size: 11px;
        }

        th:first-child,
        td:first-child {
            width: 42%;
        }
    }
</style>
