<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import { fade } from "svelte/transition";
    import { toast } from "$lib/stores/toasts";
    import { currentUser, permissions } from "$lib/stores/authContext";
    import { devLog } from "$lib/utils/devLog";
    import {
        GetDashboardStats,
        GetDashboardARAgingReportYTD,
        GetDashboardPipelineByStageYTD,
        GetMonthlyRevenueByCustomer,
        ListFollowUps,
    } from "../../../wailsjs/go/main/App";
    import { EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime";
    import { main } from "../../../wailsjs/go/models";
    import ContextTaskModal from "../components/ContextTaskModal.svelte";
    import ExecutiveRevenueChart from "../components/charts/ExecutiveRevenueChart.svelte";
    import EChartsSurface from "../components/charts/EChartsSurface.svelte";
    import {
        listMyTasks,
        listTeamTasks,
        refreshCollaborativeWorkspace,
        type CollaborativeTask,
    } from "$lib/api/collaboration";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import type { RevenueChartData } from "$lib/types";
    import type { EChartsOption } from "echarts";

    type Tone = "positive" | "negative" | "warning" | "neutral";
    type PipelineStage = {
        stage: string;
        label: string;
        count: number;
        value: number;
        color: string;
        share: number;
    };
    type AgingBucket = {
        label: string;
        value: number;
        color: string;
    };
    const DASHBOARD_CHART_YEAR = 2026;

    let loading = true;
    let chartsLoading = false;
    let now = new Date();
    let clockTimer: ReturnType<typeof setInterval> | undefined;
    let chartsLoadedForPermissionKey = "";

    $: userName = formatUserName($currentUser);
    $: greeting = getTimeGreeting(now);
    $: activePermissions = Array.isArray($permissions) ? $permissions : [];
    $: currentRoleName = String(
        ($currentUser as any)?.role_name ||
            ($currentUser as any)?.role?.display_name ||
            ($currentUser as any)?.role?.name ||
            ($currentUser as any)?.job_title ||
            "",
    )
        .trim()
        .toLowerCase();
    $: permissionKey = activePermissions.join("|");
    function permissionMatches(permissionList: string[], permission: string): boolean {
        if (permissionList.includes("*") || permissionList.includes(permission)) return true;
        const [resource] = permission.split(":");
        return permissionList.includes(`${resource}:*`);
    }
    function hasPermission(permission: string): boolean {
        const permissionList = Array.isArray(activePermissions) ? activePermissions : [];
        if (["admin", "administrator"].includes(currentRoleName)) return true;
        return permissionMatches(permissionList, permission);
    }
    $: isCurrentAdmin = ["admin", "administrator"].includes(currentRoleName);
    $: canViewTasks = isCurrentAdmin || permissionMatches(activePermissions, "tasks:view");
    $: canViewFinance = isCurrentAdmin || permissionMatches(activePermissions, "finance:view");

    let stats = {
        activeRFQs: 0,
        activeOrders: 0,
        winRate: 0,
        totalRevenue: 0,
        revenueMeta: "Confirmed business",
        revenueGrowth: 0,
        activityYear: new Date().getFullYear(),
        pipelineValue: 0,
        collectionRate: 0,
        outstandingAR: 0,
        arDaysOverdue: 0,
        pendingInvoices: 0,
        cashBalance: 0,
        cashPositionNote: "",
        freshStartDate: "",
    };

    let followUpTasks: main.FollowUpTask[] = [];
    let dashboardTasks: CollaborativeTask[] = [];
    let activeDashboardTasks: CollaborativeTask[] = [];
    let followUpSummary = "* No follow-up issues";
    let totalTaskSignals = 0;
    let taskModalOpen = false;
    let taskSignalsPermissionLoaded = false;
    let taskSignalRequestSeq = 0;
    let taskSignalsRemoteRefreshing = false;

    let revenueChartData: RevenueChartData[] = [];
    let revenueChartAvailable = true;
    let pipelineStages: PipelineStage[] = [];
    let pipelineChartAvailable = true;
    let financeChartAvailable = false;
    let arAgingBuckets: AgingBucket[] = [];
    let hoveredPipelineStage: string | null = null;
    let hoveredAgingBucket: string | null = null;

    $: activeDashboardTasks = (Array.isArray(dashboardTasks) ? dashboardTasks : [])
        .filter((task) => !["completed", "archived", "cancelled", "canceled"].includes((task.status || "").toLowerCase()))
        .slice(0, 5);
    $: followUpSummary = buildFollowUpSummary(followUpTasks, activeDashboardTasks);
    $: totalTaskSignals = followUpTasks.length + activeDashboardTasks.length;
    $: pipelineStageTotal = pipelineStages.reduce((sum, stage) => sum + stage.count, 0);
    $: arAgingTotal = arAgingBuckets.reduce((sum, bucket) => sum + bucket.value, 0);
    $: pipelineStageValueTotal = pipelineStages.reduce((sum, stage) => sum + stage.value, 0);
    $: revenueChartYear = Number(revenueChartData[0]?.year || DASHBOARD_CHART_YEAR);
    $: revenueChartPeriod =
        revenueChartYear === DASHBOARD_CHART_YEAR ? `YTD ${revenueChartYear}` : `FY ${revenueChartYear}`;
    $: selectedPipelineStage =
        hoveredPipelineStage
            ? pipelineStages.find((stage) => stage.stage === hoveredPipelineStage) || pipelineStages[0]
            : pipelineStages[0];
    $: selectedAgingBucket =
        hoveredAgingBucket
            ? arAgingBuckets.find((bucket) => bucket.label === hoveredAgingBucket) || arAgingBuckets[0]
            : arAgingBuckets[0];
    $: pipelineOption = buildPipelineOption(pipelineStages);
    $: agingOption = buildAgingOption(arAgingBuckets);

    $: canShowFinance = canViewFinance || financeChartAvailable;

    $: pressureLevel = canShowFinance
        ? stats.cashBalance > 0
            ? "Cash position is the primary decision surface today"
            : stats.pipelineValue > stats.outstandingAR
              ? "Pipeline quality is the primary decision surface today"
              : "Commercial finance is the primary decision surface today"
        : totalTaskSignals > 0
          ? "Follow-ups and pipeline conversion are the primary decision surface today"
          : "Pipeline conversion is the primary decision surface today";

    $: kpis = canShowFinance
        ? [
              {
                  label: `Revenue YTD ${DASHBOARD_CHART_YEAR}`,
                  value: formatCompactCurrency(stats.totalRevenue),
                  meta: stats.revenueMeta,
                  tone: stats.revenueGrowth >= 0 ? "positive" : "negative",
                  delta:
                      stats.revenueGrowth === 0
                          ? "flat"
                          : `${stats.revenueGrowth > 0 ? "+" : ""}${stats.revenueGrowth.toFixed(1)}% vs last month`,
              },
              {
                  label: "Cash Balance",
                  value: formatCompactCurrency(stats.cashBalance),
                  meta: "Latest bank statements",
                  tone: stats.cashPositionNote ? "warning" : "positive",
                  delta: stats.cashPositionNote ? "check statements" : "current",
              },
              {
                  label: "Accounts Receivable",
                  value: formatCompactCurrency(stats.outstandingAR),
                  meta:
                      stats.pendingInvoices > 0
                          ? `${stats.pendingInvoices} invoice/order exposure${stats.pendingInvoices === 1 ? "" : "s"} open`
                          : "No open AR",
                  tone: stats.pendingInvoices > 0 ? "negative" : "positive",
                  delta:
                      stats.arDaysOverdue > 0
                          ? `${stats.arDaysOverdue} days average overdue`
                          : "current",
              },
              {
                  label: "Pipeline",
                  value: formatCompactCurrency(stats.pipelineValue),
                  meta: "Active commercial exposure",
                  tone: "neutral",
                  delta: "weighted by live opportunities",
              },
          ]
        : [
              {
                  label: "Active RFQs",
                  value: formatCount(stats.activeRFQs),
                  meta: "Open sales opportunities",
                  tone: stats.activeRFQs > 0 ? "neutral" : "positive",
                  delta: "needs qualification",
              },
              {
                  label: "Pipeline",
                  value: formatCompactCurrency(stats.pipelineValue),
                  meta: "Active quotation value",
                  tone: "neutral",
                  delta: "live opportunity data",
              },
              {
                  label: "Active Orders",
                  value: formatCount(stats.activeOrders),
                  meta: "Confirmed handoffs",
                  tone: stats.activeOrders > 0 ? "positive" : "neutral",
                  delta: "operations visible",
              },
              {
                  label: "Win Rate",
                  value: `${Math.round(stats.winRate)}%`,
                  meta: `YTD ${DASHBOARD_CHART_YEAR} closed opportunities`,
                  tone: stats.winRate >= 20 ? "positive" : "warning",
                  delta: "won / closed",
              },
          ] as { label: string; value: string; meta: string; tone: Tone; delta: string }[];

    $: operatingFocus = canShowFinance
        ? [
              {
                  title: "Cash balance",
                  detail: summarizeCashPosition(stats.cashPositionNote, stats.cashBalance),
                  tone: stats.cashPositionNote ? "warning" : "clear",
                  action: "Review",
                  actionId: "review_cash",
              },
              {
                  title: "Accounts receivable",
                  detail:
                      stats.outstandingAR > 0
                          ? `Open receivables are ${formatCurrency(stats.outstandingAR)}`
                          : "No open receivables",
                  tone: stats.outstandingAR > 0 ? "warning" : "clear",
                  action: "Open",
                  actionId: "review_ar",
              },
              {
                  title: "Next follow-up",
                  detail:
                      followUpTasks.length > 0
                          ? summarizeText(followUpTasks[0].title || followUpTasks[0].description || "Follow-up pending", 96)
                          : activeDashboardTasks.length > 0
                          ? activeDashboardTasks[0].title || "Task pending"
                          : "No follow-up task queued",
                  tone: totalTaskSignals > 0 ? "info" : "clear",
                  action: "Open",
                  actionId: "open_followups",
              },
          ]
        : [
              {
                  title: "Next follow-up",
                  detail:
                      followUpTasks.length > 0
                          ? summarizeText(followUpTasks[0].title || followUpTasks[0].description || "Follow-up pending", 96)
                          : activeDashboardTasks.length > 0
                          ? activeDashboardTasks[0].title || "Task pending"
                          : "No follow-up task queued",
                  tone: totalTaskSignals > 0 ? "info" : "clear",
                  action: "Open",
                  actionId: "open_followups",
              },
              {
                  title: "Opportunity pipeline",
                  detail: `${stats.activeRFQs} active RFQs with ${formatCurrency(stats.pipelineValue)} in quoted/open value`,
                  tone: stats.activeRFQs > 0 ? "info" : "clear",
                  action: "Open",
                  actionId: "review_pipeline",
              },
              {
                  title: "Order handoff",
                  detail: `${stats.activeOrders} active orders visible for sales coordination`,
                  tone: stats.activeOrders > 0 ? "clear" : "soft",
                  action: "Open",
                  actionId: "review_orders",
              },
          ];

    $: alerts = canShowFinance
        ? [
              {
                  label: "Statements",
                  text: stats.cashPositionNote || `Cash balance ${formatCurrency(stats.cashBalance)}`,
                  tone: stats.cashPositionNote ? "warning" : "clear",
              },
              {
                  label: "Receivables",
                  text:
                      stats.pendingInvoices > 0
                          ? `${stats.pendingInvoices} invoice/order exposure${stats.pendingInvoices === 1 ? "" : "s"} open`
                          : "No open invoices or uninvoiced orders",
                  tone: stats.pendingInvoices > 0 ? "warning" : "clear",
              },
              {
                  label: "Pipeline",
                  text: `Commercial exposure at ${formatCurrency(stats.pipelineValue)}`,
                  tone: "info",
              },
              {
                  label: "Follow-ups",
                  text: followUpSummary,
                  tone: totalTaskSignals > 0 ? "info" : "soft",
              },
          ]
        : [
              {
                  label: "Follow-ups",
                  text: followUpSummary,
                  tone: totalTaskSignals > 0 ? "info" : "soft",
              },
              {
                  label: "RFQs",
                  text: `${stats.activeRFQs} active opportunities need stage discipline`,
                  tone: stats.activeRFQs > 0 ? "info" : "clear",
              },
              {
                  label: "Orders",
                  text: `${stats.activeOrders} confirmed orders in active handoff`,
                  tone: stats.activeOrders > 0 ? "info" : "clear",
              },
          ];

    async function loadData() {
        loading = true;
        try {
            const dashStats = await GetDashboardStats();
            if (dashStats) {
                const rawStats = dashStats as any;
                stats.activeRFQs = rawStats.active_rfqs || 0;
                stats.activeOrders = rawStats.active_orders || 0;
                stats.winRate = rawStats.win_rate || 0;
                stats.totalRevenue = rawStats.total_revenue || 0;
                stats.revenueMeta =
                    rawStats.revenue_meta ||
                    `Confirmed business YTD ${DASHBOARD_CHART_YEAR}`;
                stats.revenueGrowth = rawStats.month_growth || 0;
                stats.activityYear = rawStats.activity_year || new Date().getFullYear();
                stats.pipelineValue = rawStats.pipeline_value_bhd || 0;
                stats.collectionRate = rawStats.collection_rate || 0;
                stats.outstandingAR = rawStats.outstanding_ar || 0;
                stats.arDaysOverdue = rawStats.ar_days_overdue || 0;
                stats.pendingInvoices = rawStats.pending_invoices || 0;
                stats.cashBalance = rawStats.cash_balance_bhd || 0;
                stats.cashPositionNote = rawStats.cash_position_note || "";
                stats.freshStartDate = rawStats.fresh_start_date || "";
            }
        } catch (err) {
            const errorMsg = errorMessage(err);
            toast.danger(`Failed to load dashboard data: ${errorMsg}`);
        } finally {
            loading = false;
        }

        void loadDashboardCharts();
        void loadTaskSignals({ refreshRemote: true });
    }

    async function loadDashboardCharts() {
        chartsLoading = true;
        chartsLoadedForPermissionKey = permissionKey;

        const [revenueResult, pipelineResult, agingResult] = await Promise.allSettled([
            GetMonthlyRevenueByCustomer(),
            loadPipelineStages(),
            GetDashboardARAgingReportYTD(),
        ]);

        if (revenueResult.status === "fulfilled") {
            revenueChartData = Array.isArray(revenueResult.value) ? revenueResult.value : [];
            revenueChartAvailable = true;
        } else {
            revenueChartData = [];
            revenueChartAvailable = !isPermissionError(revenueResult.reason);
            devLog.warn("Dashboard revenue chart failed", revenueResult.reason);
        }

        if (pipelineResult.status === "fulfilled") {
            pipelineStages = pipelineResult.value;
            pipelineChartAvailable = true;
        } else {
            pipelineStages = fallbackPipelineStages();
            pipelineChartAvailable = pipelineStages.length > 0 || !isPermissionError(pipelineResult.reason);
            devLog.warn("Dashboard pipeline chart failed", pipelineResult.reason);
        }

        if (agingResult.status === "fulfilled" && agingResult.value) {
            financeChartAvailable = true;
            arAgingBuckets = normalizeAgingBuckets(agingResult.value as any);
        } else {
            financeChartAvailable = false;
            arAgingBuckets = [];
            if (agingResult.status === "rejected") {
                devLog.warn("Dashboard AR aging chart failed", agingResult.reason);
            }
        }

        chartsLoading = false;
    }

    async function loadPipelineStages(): Promise<PipelineStage[]> {
        try {
            const rows = await GetDashboardPipelineByStageYTD();
            if (Array.isArray(rows) && rows.length > 0) {
                return normalizePipelineRows(rows);
            }
        } catch (err) {
            if (!isPermissionError(err)) {
                devLog.warn("Dashboard pipeline stage query failed", err);
            }
        }

        return [];
    }

    function normalizePipelineRows(rows: any[]): PipelineStage[] {
        const total = rows.reduce((sum, row) => sum + Number(row?.count || 0), 0);
        return rows
            .map((row) => ({
                stage: String(row?.stage || "Unstaged"),
                label: formatStageLabel(row?.stage || "Unstaged"),
                count: Number(row?.count || 0),
                value: Number(row?.value || 0),
                color: stageColor(row?.stage || "Unstaged"),
                share: total > 0 ? (Number(row?.count || 0) / total) * 100 : 0,
            }))
            .filter((row) => row.count > 0)
            .sort((a, b) => b.count - a.count);
    }

    function normalizeAgingBuckets(report: any): AgingBucket[] {
        return [
            { label: "Current", value: Number(report?.current || 0), color: "#3E8F47" },
            { label: "30 Days", value: Number(report?.days_30 || 0), color: "#8BAA74" },
            { label: "60 Days", value: Number(report?.days_60 || 0), color: "#C9A227" },
            { label: "90 Days", value: Number(report?.days_90 || 0), color: "#D07A2D" },
            { label: "120+ Days", value: Number(report?.days_120_plus || 0), color: "#C94B4B" },
        ];
    }

    function fallbackPipelineStages(): PipelineStage[] {
        if (stats.activeRFQs <= 0) return [];
        return [
            {
                stage: "active",
                label: "Active RFQs",
                count: stats.activeRFQs,
                value: stats.pipelineValue,
                color: "#3E8F47",
                share: 100,
            },
        ];
    }

    function stageColor(stage: string): string {
        const normalized = String(stage || "").toLowerCase();
        const colors: Record<string, string> = {
            pending: "#C98D2E",
            qualified: "#3E8F47",
            proposal: "#5E7D68",
            quoted: "#6E9A75",
            negotiation: "#8A6F25",
            won: "#2F8F4E",
            lost: "#C94B4B",
        };
        return colors[normalized] || "#667569";
    }

    function formatStageLabel(stage: string): string {
        return String(stage || "Unstaged")
            .replace(/[_-]+/g, " ")
            .replace(/\b\w/g, (match) => match.toUpperCase());
    }

    function buildPipelineOption(stages: PipelineStage[]): EChartsOption {
        const totalCount = stages.reduce((sum, stage) => sum + stage.count, 0);
        const totalValue = stages.reduce((sum, stage) => sum + stage.value, 0);
        return {
            backgroundColor: "transparent",
            aria: { enabled: true },
            animationDuration: 700,
            animationEasing: "cubicOut",
            color: stages.map((stage) => stage.color),
            tooltip: {
                trigger: "item",
                backgroundColor: "rgba(37, 49, 43, 0.96)",
                borderWidth: 0,
                padding: [10, 12],
                textStyle: { color: "#FFFFFF", fontSize: 12, fontWeight: 600 },
                formatter(params: any) {
                    const data = params?.data || {};
                    return [
                        `<strong>${escapeHtml(data.name || "Pipeline")}</strong>`,
                        `Records: ${(Number(data.value || 0)).toLocaleString("en-US")}`,
                        `Value: ${formatCurrency(Number(data.pipelineValue || 0))}`,
                        `Share: ${Number(params?.percent || 0).toFixed(1)}%`,
                    ].join("<br/>");
                },
            },
            graphic: [
                {
                    type: "text",
                    left: "center",
                    top: "43%",
                    style: {
                        text: totalCount.toLocaleString("en-US"),
                        fill: "#25312B",
                        fontSize: 24,
                        fontWeight: 850,
                        align: "center",
                    },
                },
                {
                    type: "text",
                    left: "center",
                    top: "55%",
                    style: {
                        text: formatCompactCurrency(totalValue),
                        fill: "#667569",
                        fontSize: 11,
                        fontWeight: 800,
                        align: "center",
                    },
                },
            ],
            series: [
                {
                    name: "Pipeline",
                    type: "pie",
                    radius: ["56%", "78%"],
                    center: ["50%", "50%"],
                    minAngle: 3,
                    avoidLabelOverlap: true,
                    itemStyle: {
                        borderColor: "#FFFFFF",
                        borderRadius: 8,
                        borderWidth: 3,
                    },
                    label: {
                        color: "#25312B",
                        fontSize: 11,
                        fontWeight: 800,
                        width: 80,
                        overflow: "truncate",
                        formatter(params: any) {
                            return Number(params?.percent || 0) < 4 ? "" : String(params?.name || "");
                        },
                    },
                    labelLine: {
                        length: 10,
                        length2: 6,
                        lineStyle: { color: "#9AA89B" },
                    },
                    emphasis: {
                        scale: true,
                        scaleSize: 7,
                        label: { fontSize: 12, fontWeight: 850 },
                    },
                    data: stages.map((stage) => ({
                        name: stage.label,
                        value: stage.count,
                        stage: stage.stage,
                        pipelineValue: stage.value,
                        itemStyle: { color: stage.color },
                    })),
                },
            ],
        };
    }

    function buildAgingOption(buckets: AgingBucket[]): EChartsOption {
        return {
            backgroundColor: "transparent",
            aria: { enabled: true },
            animationDuration: 650,
            animationEasing: "cubicOut",
            grid: { top: 8, right: 18, bottom: 30, left: 84, containLabel: true },
            tooltip: {
                trigger: "axis",
                axisPointer: { type: "shadow", shadowStyle: { color: "rgba(201, 75, 75, 0.06)" } },
                backgroundColor: "rgba(37, 49, 43, 0.96)",
                borderWidth: 0,
                padding: [10, 12],
                textStyle: { color: "#FFFFFF", fontSize: 12, fontWeight: 600 },
                formatter(params: any) {
                    const item = Array.isArray(params) ? params[0] : params;
                    const bucket = buckets[item?.dataIndex || 0];
                    if (!bucket) return "";
                    return [
                        `<strong>${escapeHtml(bucket.label)}</strong>`,
                        `Outstanding: ${formatCurrency(bucket.value)}`,
                        `Status: ${agingRiskLabel(bucket)}`,
                    ].join("<br/>");
                },
            },
            xAxis: {
                type: "value",
                axisLabel: {
                    color: "#7B8A7D",
                    fontSize: 10,
                    margin: 10,
                    hideOverlap: true,
                    formatter: (value: number) => formatAxisCurrency(value),
                },
                splitLine: { lineStyle: { color: "rgba(93, 116, 94, 0.12)" } },
                axisLine: { show: false },
                axisTick: { show: false },
            },
            yAxis: {
                type: "category",
                inverse: true,
                data: buckets.map((bucket) => bucket.label),
                axisLabel: { color: "#25312B", fontSize: 11, fontWeight: 800 },
                axisLine: { show: false },
                axisTick: { show: false },
            },
            series: [
                {
                    name: "Receivables",
                    type: "bar",
                    barWidth: 15,
                    itemStyle: {
                        borderRadius: [0, 8, 8, 0],
                        color(params: any) {
                            return buckets[params.dataIndex]?.color || "#C94B4B";
                        },
                    },
                    emphasis: {
                        focus: "series",
                        itemStyle: { shadowBlur: 15, shadowColor: "rgba(201, 75, 75, 0.2)" },
                    },
                    data: buckets.map((bucket) => ({
                        value: Number(bucket.value || 0),
                        bucketLabel: bucket.label,
                    })),
                },
            ],
        };
    }

    function handlePipelineChartHover(event: CustomEvent<any>) {
        hoveredPipelineStage = event.detail?.data?.stage || null;
    }

    function handleAgingChartHover(event: CustomEvent<any>) {
        hoveredAgingBucket = event.detail?.data?.bucketLabel || null;
    }

    function clearChartHover() {
        hoveredPipelineStage = null;
        hoveredAgingBucket = null;
    }

    function openDocumentCapture() {
        window.dispatchEvent(new CustomEvent("document-capture:open"));
    }

    function agingRiskLabel(bucket?: AgingBucket): string {
        if (!bucket) return "watch";
        if (bucket.label === "Current") return "healthy";
        if (bucket.label.includes("120")) return "critical";
        if (bucket.label.includes("90")) return "high risk";
        if (bucket.label.includes("60")) return "watch";
        return "attention";
    }

    function errorMessage(err: unknown): string {
        if (err instanceof Error) return err.message;
        return String(err || "Unknown error");
    }

    function isPermissionError(err: unknown): boolean {
        return /(permission|access denied|not authorized|unauthorized)/i.test(errorMessage(err));
    }

    async function loadTaskSignals(options: { refreshRemote?: boolean } = {}) {
        const requestSeq = ++taskSignalRequestSeq;
        if (!canViewTasks) {
            if (activePermissions.length > 0) {
                followUpTasks = [];
                dashboardTasks = [];
            }
            return;
        }

        const [legacyRows, myTaskRows] = await Promise.all([
            ListFollowUps(5).catch((err) => {
                devLog.warn("Dashboard follow-ups failed", err);
                return [];
            }),
            listMyTasks(false).catch((err) => {
                devLog.warn("Dashboard personal tasks failed", err);
                return [];
            }),
        ]);
        if (requestSeq !== taskSignalRequestSeq) return;
        followUpTasks = legacyRows || [];
        dashboardTasks = myTaskRows || [];
        if (dashboardTasks.length === 0 && Array.isArray(activePermissions) && activePermissions.includes("*")) {
            dashboardTasks = (await listTeamTasks(false).catch((err) => {
                devLog.warn("Dashboard team tasks failed", err);
                return [];
            })) || [];
        }
        if (options.refreshRemote) {
            void refreshTaskSignalsFromRemote();
        }
    }

    async function refreshTaskSignalsFromRemote() {
        if (taskSignalsRemoteRefreshing) return;
        taskSignalsRemoteRefreshing = true;
        try {
            await refreshCollaborativeWorkspace();
            await loadTaskSignals();
        } catch (err) {
            devLog.warn("Dashboard collaborative refresh failed", err);
        } finally {
            taskSignalsRemoteRefreshing = false;
        }
    }

    $: if (!canViewTasks && taskSignalsPermissionLoaded) {
        taskSignalsPermissionLoaded = false;
    }

    $: if (canViewTasks && !taskSignalsPermissionLoaded) {
        taskSignalsPermissionLoaded = true;
        void loadTaskSignals({ refreshRemote: true });
    }

    $: if (!loading && permissionKey && permissionKey !== chartsLoadedForPermissionKey) {
        void loadDashboardCharts();
    }

    function formatCurrency(val: number): string {
        return `${Number(val || 0).toLocaleString("en-US", {
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
        })} BHD`;
    }

    function formatCompactCurrency(val: number): string {
        const amount = Number(val || 0);
        if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}M BHD`;
        if (amount >= 1_000) return `${Math.round(amount / 1_000).toLocaleString("en-US")}k BHD`;
        return `${amount.toLocaleString("en-US", { maximumFractionDigits: 0 })} BHD`;
    }

    function formatAxisCurrency(val: number): string {
        const amount = Number(val || 0);
        if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}M`;
        if (amount >= 1_000) return `${Math.round(amount / 1_000).toLocaleString("en-US")}k`;
        return amount.toLocaleString("en-US", { maximumFractionDigits: 0 });
    }

    function escapeHtml(value: string): string {
        return String(value || "")
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;");
    }

    function formatCount(val: number): string {
        return Number(val || 0).toLocaleString("en-US", { maximumFractionDigits: 0 });
    }

    function formatDate(value: Date = new Date()): string {
        return value.toLocaleDateString("en-GB", {
            weekday: "long",
            day: "numeric",
            month: "long",
            year: "numeric",
        });
    }

    function getTimeGreeting(value: Date): string {
        const hour = value.getHours();
        if (hour < 12) return "Good morning";
        if (hour < 16) return "Good afternoon";
        return "Good evening";
    }

    function formatUserName(user: any): string {
        const raw =
            user?.preferred_name ||
            user?.full_name ||
            user?.display_name ||
            user?.name ||
            user?.username ||
            "";
        const cleaned = String(raw).replace(/[._-]+/g, " ").trim();
        if (!cleaned) return "there";
        const firstName = cleaned.split(/\s+/)[0];
        return firstName.charAt(0).toUpperCase() + firstName.slice(1);
    }

    function summarizeText(value: string | undefined, maxLength = 78): string {
        const compact = String(value || "").replace(/\s+/g, " ").trim();
        if (!compact) return "No issue summary";
        if (compact.length <= maxLength) return compact;
        const clipped = compact.slice(0, maxLength - 3);
        const lastSpace = clipped.lastIndexOf(" ");
        return `${clipped.slice(0, lastSpace > 42 ? lastSpace : clipped.length)}...`;
    }

    function summarizeCashPosition(note: string | undefined, cashBalance: number): string {
        const compact = String(note || "").replace(/\s+/g, " ").trim();
        if (!compact) return `Bank cash is ${formatCurrency(cashBalance)}`;
        if (/no statement imported/i.test(compact)) {
            const matches = [...compact.matchAll(/No statement imported for ([^.]+?)(?: latest statement|\.|$)/gi)]
                .map((match) => String(match[1] || "").trim())
                .filter(Boolean);
            if (matches.length > 0) {
                const first = matches[0];
                const extra = matches.length > 1 ? ` +${matches.length - 1} more` : "";
                return `Missing bank statement: ${summarizeText(first, 44)}${extra}`;
            }
            return "Missing bank statements need review";
        }
        return summarizeText(compact, 92);
    }

    function buildFollowUpSummary(
        legacyFollowUps: main.FollowUpTask[],
        activeTasks: CollaborativeTask[],
    ): string {
        if (legacyFollowUps.length > 0) {
            const first = legacyFollowUps[0];
            return summarizeText(first.title || first.description || first.notes || "Follow-up pending");
        }
        if (activeTasks.length > 0) {
            return summarizeText(activeTasks[0].title || activeTasks[0].description || "Task pending");
        }
        return "No follow-up issues";
    }

    function taskAge(task: any): string {
        const rawDate = task.due_date || task.last_comment_at || task.created_at || task.updated_at || task.started_at;
        if (!rawDate) return "recent";
        const date = new Date(rawDate as any);
        if (Number.isNaN(date.getTime())) return "recent";
        const diffDays = Math.round((date.getTime() - Date.now()) / (24 * 60 * 60 * 1000));
        if (diffDays === 0) return "today";
        if (diffDays === 1) return "tomorrow";
        if (diffDays > 1) return `${diffDays}d`;
        if (diffDays === -1) return "yesterday";
        return `${Math.abs(diffDays)}d ago`;
    }

    function taskInitial(task: any): string {
        return (task.priority || task.status || "T").slice(0, 1).toUpperCase();
    }

    function taskMeta(task: CollaborativeTask): string {
        const pieces = [
            task.priority ? `${task.priority} priority` : "",
            task.assignee_name ? `assigned to ${task.assignee_name}` : "",
            task.status || "",
        ].filter(Boolean);
        return pieces.join(" / ") || "Task";
    }

    function navigateTo(target: Record<string, string>) {
        window.dispatchEvent(new CustomEvent("navigateToScreen", { detail: target }));
    }

    function handleDashboardAction(actionId: string) {
        switch (actionId) {
            case "review_cash":
                navigateTo({ screen: "finance", tab: "bank-recon", company: "PH Trading" });
                return;
            case "review_credit":
            case "chase_overdue":
            case "review_ar":
                navigateTo({ screen: "finance", tab: "invoices", company: "PH Trading" });
                return;
            case "open_followups":
                navigateTo({ screen: "work" });
                return;
            case "review_pipeline":
                navigateTo({ screen: "opportunities" });
                return;
            case "review_orders":
                navigateTo({ screen: "operations" });
                return;
            default:
                toast.warning("Dashboard action is not available yet");
        }
    }

    function openTaskModal() {
        taskModalOpen = true;
    }

    async function handleTaskCreated() {
        taskModalOpen = false;
        await loadData();
    }

    onMount(() => {
        now = new Date();
        clockTimer = setInterval(() => {
            now = new Date();
        }, 60_000);
        loadData();

        EventsOn("data:refresh", (data: any) => {
            devLog.info("Dashboard received data:refresh event", data);
            loadData();
        });
        EventsOn("tasks:updated", () => void loadTaskSignals());
        EventsOn("notifications:updated", () => void loadTaskSignals());
        EventsOn("employees:updated", () => void loadTaskSignals({ refreshRemote: true }));
    });

    onDestroy(() => {
        if (clockTimer) {
            clearInterval(clockTimer);
        }
        EventsOff("data:refresh", "tasks:updated", "notifications:updated", "employees:updated");
    });
</script>

<div class="dashboard-page">
    <header class="dashboard-header">
        <div>
            <p class="brand-kicker">AsymmFlow</p>
            <h1>{greeting}, {userName}</h1>
            <p class="dashboard-subtitle">{pressureLevel}</p>
        </div>
        <div class="dashboard-actions">
            <time datetime={now.toISOString()}>{formatDate(now)}</time>
            <button type="button" class="capture-button" on:click={openDocumentCapture}>Browse</button>
        </div>
    </header>

    {#if loading}
        <div class="loading-state"><WabiSpinner size="lg" tempo="calm" /></div>
    {:else}
        <main class="dashboard-shell" in:fade>
            <section class="kpi-strip" aria-label="Primary dashboard metrics">
                {#each kpis as kpi}
                    <article class="metric-card" data-tone={kpi.tone}>
                        <div class="metric-label">{kpi.label}</div>
                        <div class="metric-value">{kpi.value}</div>
                        <div class="metric-meta">
                            <span>{kpi.meta}</span>
                            <strong>{kpi.delta}</strong>
                        </div>
                    </article>
                {/each}
            </section>

            <section class="decision-grid">
                <article class="panel operating-panel">
                    <div class="panel-head">
                        <span>Operating Focus</span>
                        <strong>{totalTaskSignals} active follow-ups</strong>
                    </div>
                    <h2>{pressureLevel}</h2>
                    <div class="focus-list">
                        {#each operatingFocus as item}
                            <div class="focus-row" data-tone={item.tone}>
                                <div>
                                    <strong>{item.title}</strong>
                                    <span>{item.detail}</span>
                                </div>
                                <button
                                    class="focus-action"
                                    type="button"
                                    on:click={() => handleDashboardAction(item.actionId)}
                                    aria-label={`${item.action} ${item.title}`}
                                >
                                    {item.action}
                                </button>
                            </div>
                        {/each}
                    </div>
                </article>

                <article class="panel alerts-panel">
                    <div class="panel-head">
                        <span>Alerts</span>
                        <strong>{alerts.filter((alert) => alert.tone !== "clear").length} live</strong>
                    </div>
                    <div class="alert-list">
                        {#each alerts as alert}
                            <div class="alert-row" data-tone={alert.tone}>
                                <strong>{alert.label}</strong>
                                <span>{alert.text}</span>
                            </div>
                        {/each}
                    </div>
                </article>

                {#if revenueChartAvailable}
                    <article class="panel chart-panel revenue-panel">
                        <div class="panel-head">
                            <span>Top Customer Revenue</span>
                            <strong>{revenueChartPeriod}</strong>
                        </div>
                        <div class="revenue-chart-wrap">
                            <ExecutiveRevenueChart data={revenueChartData} loading={chartsLoading} />
                        </div>
                    </article>
                {/if}

                {#if pipelineChartAvailable}
                    <article class="panel chart-panel pipeline-panel">
                        <div class="panel-head">
                            <span>Pipeline by Stage</span>
                            <strong>{pipelineStageTotal} YTD {DASHBOARD_CHART_YEAR} records</strong>
                        </div>
                        {#if chartsLoading}
                            <div class="chart-empty">Loading chart data...</div>
                        {:else if pipelineStages.length === 0}
                            <div class="chart-empty">No pipeline data available</div>
                        {:else}
                            <div class="pipeline-executive">
                                <div class="pipeline-chart-frame">
                                    <EChartsSurface
                                        option={pipelineOption}
                                        height={255}
                                        ariaLabel="Pipeline stage distribution interactive donut"
                                        on:hover={handlePipelineChartHover}
                                        on:leave={clearChartHover}
                                    />
                                </div>

                                <div class="pipeline-ledger">
                                    <div class="pipeline-selected">
                                        <span>{selectedPipelineStage?.label || "Pipeline"}</span>
                                        <strong>{formatCompactCurrency(selectedPipelineStage?.value || 0)}</strong>
                                        <small>{selectedPipelineStage?.count || 0} records</small>
                                    </div>
                                    {#each pipelineStages as stage}
                                        <button
                                            class="pipeline-row"
                                            class:active={hoveredPipelineStage === stage.stage}
                                            type="button"
                                            on:mouseenter={() => (hoveredPipelineStage = stage.stage)}
                                            on:mouseleave={() => (hoveredPipelineStage = null)}
                                            on:focus={() => (hoveredPipelineStage = stage.stage)}
                                            on:blur={() => (hoveredPipelineStage = null)}
                                        >
                                            <span class="stage-dot" style={`background: ${stage.color}`}></span>
                                            <span class="stage-name">{stage.label}</span>
                                            <strong>{stage.count}</strong>
                                            <em>{formatCompactCurrency(stage.value)}</em>
                                        </button>
                                    {/each}
                                </div>
                            </div>
                        {/if}
                    </article>
                {/if}

                {#if canShowFinance}
                    <article class="panel receivables-panel">
                        <div class="panel-head">
                            <span>Receivables Aging</span>
                            <strong>{stats.pendingInvoices} open · YTD {DASHBOARD_CHART_YEAR}</strong>
                        </div>
                        <div class="receivable-row primary">
                            <div>
                                <span>Outstanding balance</span>
                                <strong>{formatCurrency(stats.outstandingAR)}</strong>
                            </div>
                            <small>{stats.arDaysOverdue > 0 ? `${stats.arDaysOverdue}d average overdue` : "current"}</small>
                        </div>
                        <div class="collection-bar">
                            <span style={`width: ${Math.min(100, Math.max(0, stats.collectionRate))}%`}></span>
                        </div>
                        <div class="receivable-meta">
                            <span>Collected</span>
                            <strong>{Math.round(stats.collectionRate)}%</strong>
                            <span>At risk</span>
                            <strong>{formatCurrency(stats.outstandingAR)}</strong>
                        </div>

                        {#if chartsLoading}
                            <div class="chart-empty compact">Loading chart data...</div>
                        {:else if arAgingTotal <= 0}
                            <div class="chart-empty compact">No receivables aging data</div>
                        {:else}
                            <div class="aging-executive">
                                <div class="aging-selected">
                                    <span>{selectedAgingBucket?.label || "Receivables"}</span>
                                    <strong>{formatCompactCurrency(selectedAgingBucket?.value || 0)}</strong>
                                    <small>{agingRiskLabel(selectedAgingBucket)}</small>
                                </div>
                                <div class="aging-chart-frame">
                                    <EChartsSurface
                                        option={agingOption}
                                        height={212}
                                        ariaLabel="Receivables aging interactive bar chart"
                                        on:hover={handleAgingChartHover}
                                        on:leave={clearChartHover}
                                    />
                                </div>
                            </div>
                        {/if}
                    </article>
                {/if}

                <article class="panel activity-panel" class:wide={!canShowFinance}>
                    <div class="panel-head">
                        <span>Tasks</span>
                        <button class="panel-action" type="button" on:click={openTaskModal}>New Task</button>
                    </div>
                    {#if activeDashboardTasks.length > 0}
                        <div class="activity-list">
                            {#each activeDashboardTasks as task}
                                <div class="activity-row">
                                    <span class="activity-token">{taskInitial(task)}</span>
                                    <div>
                                        <strong>{task.title}</strong>
                                        <span>{summarizeText(task.description || taskMeta(task), 110)}</span>
                                    </div>
                                    <time>{taskAge(task)}</time>
                                </div>
                            {/each}
                        </div>
                    {:else}
                        <button class="empty-state empty-action" type="button" on:click={openTaskModal}>No active tasks. Create one.</button>
                    {/if}
                </article>
            </section>
        </main>
    {/if}
</div>

<ContextTaskModal
    open={taskModalOpen}
    title="Create Task"
    subtitle="Add a follow-up from the dashboard"
    on:created={handleTaskCreated}
    on:close={() => (taskModalOpen = false)}
/>

<style>
    .dashboard-page {
        min-height: 100vh;
        padding: 22px 28px 28px;
        background: var(--bg-base);
        color: var(--text-primary);
        box-sizing: border-box;
    }

    .dashboard-header {
        display: flex;
        justify-content: space-between;
        gap: 24px;
        align-items: flex-start;
        margin-bottom: 18px;
    }

    .brand-kicker {
        margin: 0 0 12px;
        color: var(--brand-indigo, #3E8F47);
        font-size: var(--text-12, 12px);
        font-weight: 700;
    }

    .dashboard-header h1 {
        margin: 0;
        color: var(--onyx, #25312B);
        font-size: var(--text-28, 28px);
        font-weight: 500;
        line-height: 1.1;
    }

    .dashboard-subtitle {
        margin: 6px 0 0;
        color: var(--text-secondary, #667569);
        font-size: var(--text-12, 12px);
        font-weight: 600;
    }

    .dashboard-actions {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .dashboard-header time {
        color: var(--text-muted, #8A978B);
        font-size: var(--text-12, 12px);
        white-space: nowrap;
    }

    .capture-button {
        border: 1px solid var(--border, #D6DED3);
        border-radius: 8px;
        background: var(--surface, #FFFFFF);
        color: var(--brand-indigo, #3E8F47);
        cursor: pointer;
        font-size: var(--text-12, 12px);
        font-weight: 700;
        min-height: 34px;
        padding: 0 14px;
    }

    .capture-button:hover {
        border-color: var(--brand-indigo, #3E8F47);
        background: var(--surface-raised, #F7FAF6);
    }

    .loading-state {
        min-height: 60vh;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .dashboard-shell {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .kpi-strip {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
        gap: 14px;
    }

    .metric-card,
    .panel {
        background: rgba(255, 255, 255, 0.78);
        border: 1px solid var(--border, #DDE8DD);
        border-radius: 8px;
        box-shadow: 0 10px 28px rgba(36, 48, 39, 0.035);
    }

    .metric-card {
        min-height: 86px;
        padding: 18px 20px 14px;
        position: relative;
        overflow: hidden;
    }

    .metric-card::before {
        content: "";
        position: absolute;
        inset: 0 0 auto 0;
        height: 3px;
        background: #DCE8DC;
    }

    .metric-card[data-tone="positive"]::before {
        background: #3E8F47;
    }

    .metric-card[data-tone="negative"]::before {
        background: #C94B4B;
    }

    .metric-card[data-tone="warning"]::before {
        background: #C98D2E;
    }

    .metric-label,
    .panel-head span {
        color: var(--text-muted, #8A978B);
        font-size: var(--text-10, 10px);
        font-weight: 800;
        letter-spacing: 0;
        text-transform: uppercase;
    }

    .metric-value {
        margin-top: 10px;
        color: var(--onyx, #25312B);
        font-size: var(--text-24, 24px);
        line-height: 1;
        font-weight: 700;
    }

    .metric-meta {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        margin-top: 8px;
        color: var(--text-secondary, #667569);
        font-size: var(--text-11, 11px);
        line-height: 1.3;
    }

    .metric-meta strong {
        color: var(--brand-indigo, #3E8F47);
        font-weight: 700;
        text-align: right;
    }

    .metric-card[data-tone="negative"] .metric-meta strong {
        color: #C94B4B;
    }

    .metric-card[data-tone="warning"] .metric-meta strong {
        color: #8A6F25;
    }

    .decision-grid {
        display: grid;
        grid-template-columns: minmax(0, 1.15fr) minmax(320px, 0.85fr);
        gap: 14px;
    }

    .panel {
        padding: 20px;
        min-height: 210px;
        box-sizing: border-box;
    }

    .panel-head {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
        margin-bottom: 12px;
    }

    .panel-head strong {
        color: var(--text-muted, #8A978B);
        font-size: var(--text-11, 11px);
        font-weight: 700;
        line-height: 1.25;
        text-align: right;
    }

    .panel-action {
        padding: 6px 9px;
        border: 1px solid var(--border, #DDE8DD);
        border-radius: 5px;
        background: #ffffff;
        color: var(--brand-indigo, #3E8F47);
        font-size: var(--text-11, 11px);
        font-weight: 800;
        cursor: pointer;
    }

    .panel-action:hover {
        border-color: #9CB99D;
        background: #F4F8F4;
    }

    .operating-panel h2 {
        margin: 0 0 18px;
        color: var(--onyx, #25312B);
        font-size: var(--text-18, 18px);
        line-height: 1.35;
        font-weight: 700;
    }

    .focus-list,
    .alert-list,
    .activity-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .focus-row,
    .alert-row,
    .activity-row,
    .receivable-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        border-radius: 6px;
        background: #F3F7F2;
    }

    .focus-row {
        padding: 12px 14px;
        border-left: 3px solid #AFC2AF;
    }

    .focus-row[data-tone="critical"] {
        background: #FFF0F1;
        border-color: #C94B4B;
    }

    .focus-row[data-tone="warning"] {
        background: #FFF7E9;
        border-color: #C98D2E;
    }

    .focus-row[data-tone="info"] {
        background: #EEF7EE;
        border-color: var(--brand-indigo, #3E8F47);
    }

    .focus-row strong,
    .activity-row strong,
    .receivable-row strong {
        display: block;
        color: var(--onyx, #25312B);
        font-size: var(--text-13, 13px);
        font-weight: 800;
    }

    .focus-row span,
    .activity-row div span,
    .receivable-row span,
    .alert-row span {
        display: block;
        margin-top: 4px;
        color: var(--text-secondary, #667569);
        font-size: var(--text-11, 11px);
        line-height: 1.35;
    }

    .focus-action {
        min-width: 58px;
        padding: 7px 10px;
        border: 0;
        border-radius: 5px;
        background: var(--brand-indigo, #3E8F47);
        color: white;
        font-size: var(--text-11, 11px);
        font-weight: 800;
        text-align: center;
        cursor: pointer;
        transition:
            background 0.15s ease,
            transform 0.15s ease,
            box-shadow 0.15s ease;
    }

    .focus-action:hover {
        background: var(--brand-indigo-hover, #2F7638);
        box-shadow: 0 6px 14px rgba(62, 143, 71, 0.16);
        transform: translateY(-1px);
    }

    .focus-action:focus-visible {
        outline: 2px solid #98C99E;
        outline-offset: 2px;
    }

    .alert-row {
        align-items: flex-start;
        justify-content: flex-start;
        padding: 12px 14px;
        border-left: 3px solid #B7C8B5;
    }

    .alert-row[data-tone="critical"] {
        background: #FFF0F2;
        border-color: #C94B4B;
    }

    .alert-row[data-tone="warning"] {
        background: #FFF7E8;
        border-color: #C98D2E;
    }

    .alert-row[data-tone="info"] {
        background: #EFF8EF;
        border-color: var(--brand-indigo, #3E8F47);
    }

    .alert-row[data-tone="soft"] {
        background: #F6F8F5;
        border-color: #B9C8B8;
    }

    .alert-row strong {
        min-width: 96px;
        color: var(--onyx, #25312B);
        font-size: var(--text-12, 12px);
    }

    .chart-panel {
        min-height: 300px;
    }

    .revenue-panel {
        grid-column: 1 / -1;
    }

    .revenue-chart-wrap {
        min-height: 300px;
    }

    .pipeline-executive {
        display: grid;
        grid-template-columns: minmax(250px, 0.95fr) minmax(0, 1fr);
        gap: 18px;
        align-items: center;
    }

    .pipeline-chart-frame,
    .aging-chart-frame {
        min-width: 0;
        border: 1px solid rgba(93, 116, 94, 0.12);
        border-radius: 8px;
        background:
            linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(247, 250, 247, 0.94)),
            #FFFFFF;
        box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.84);
    }

    .pipeline-ledger {
        display: flex;
        min-width: 0;
        flex-direction: column;
        gap: 8px;
    }

    .pipeline-selected {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        gap: 4px 12px;
        padding: 12px 14px;
        border: 1px solid #D3E1D2;
        border-radius: 8px;
        background: linear-gradient(180deg, #FFFFFF 0%, #F7FAF7 100%);
    }

    .pipeline-selected span,
    .aging-selected span {
        color: var(--text-muted, #8A978B);
        font-size: var(--text-10, 10px);
        font-weight: 800;
        text-transform: uppercase;
        letter-spacing: 0;
    }

    .pipeline-selected strong,
    .aging-selected strong {
        color: var(--onyx, #25312B);
        font-size: var(--text-15, 15px);
        font-weight: 850;
        white-space: nowrap;
    }

    .pipeline-selected small,
    .aging-selected small {
        grid-column: 1 / -1;
        color: var(--text-secondary, #667569);
        font-size: var(--text-11, 11px);
        font-weight: 700;
        text-transform: capitalize;
    }

    .pipeline-row {
        display: grid;
        grid-template-columns: 10px minmax(0, 1fr) auto auto;
        gap: 10px;
        align-items: center;
        width: 100%;
        min-height: 34px;
        padding: 8px 10px;
        border: 1px solid transparent;
        border-radius: 8px;
        background: transparent;
        color: inherit;
        text-align: left;
        cursor: pointer;
        transition:
            background 0.16s ease,
            border-color 0.16s ease,
            opacity 0.16s ease,
            transform 0.16s ease;
    }

    .pipeline-row:hover,
    .pipeline-row:focus-visible,
    .pipeline-row.active {
        border-color: #B9CEB8;
        background: rgba(62, 143, 71, 0.07);
        outline: none;
        transform: translateX(2px);
    }

    .stage-dot {
        width: 9px;
        height: 9px;
        border-radius: 999px;
    }

    .stage-name {
        min-width: 0;
        overflow: hidden;
        color: var(--onyx, #25312B);
        font-size: var(--text-12, 12px);
        font-weight: 800;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .pipeline-row strong,
    .pipeline-row em {
        color: var(--text-secondary, #667569);
        font-size: var(--text-11, 11px);
        font-style: normal;
        font-weight: 800;
        font-variant-numeric: tabular-nums;
        white-space: nowrap;
    }

    .pipeline-row em {
        color: var(--text-muted, #8A978B);
    }

    .receivables-panel,
    .activity-panel {
        min-height: 260px;
    }

    .activity-panel.wide {
        grid-column: 1 / -1;
        min-height: 220px;
    }

    .receivable-row {
        padding: 14px;
    }

    .receivable-row.primary {
        background: #FFF7F8;
    }

    .receivable-row small {
        color: #C94B4B;
        font-size: var(--text-11, 11px);
        font-weight: 800;
        white-space: nowrap;
    }

    .collection-bar {
        height: 8px;
        margin: 18px 0 14px;
        border-radius: 999px;
        background: #E1E9DF;
        overflow: hidden;
    }

    .collection-bar span {
        display: block;
        height: 100%;
        border-radius: inherit;
        background: var(--brand-indigo, #3E8F47);
    }

    .receivable-meta {
        display: grid;
        grid-template-columns: 1fr auto 1fr auto;
        gap: 8px;
        align-items: center;
        color: var(--text-muted, #8A978B);
        font-size: var(--text-11, 11px);
    }

    .receivable-meta strong {
        color: var(--onyx, #25312B);
    }

    .aging-executive {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-top: 18px;
    }

    .aging-selected {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        gap: 4px 12px;
        padding: 11px 13px;
        border: 1px solid #E6D5D5;
        border-radius: 8px;
        background: linear-gradient(180deg, #FFFFFF 0%, #FFF8F8 100%);
    }

    .chart-empty {
        min-height: 180px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--text-muted, #8A978B);
        font-size: var(--text-12, 12px);
        border: 1px dashed var(--border, #DDE8DD);
        border-radius: 8px;
        background: #F7FAF7;
    }

    .chart-empty.compact {
        min-height: 84px;
        margin-top: 18px;
    }

    .activity-row {
        padding: 10px 0;
        background: transparent;
        border-bottom: 1px solid #E2EBE0;
    }

    .activity-row:last-child {
        border-bottom: none;
    }

    .activity-token {
        width: 22px;
        height: 22px;
        min-width: 22px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        box-sizing: border-box;
        border-radius: 5px;
        background: #E8F1E5;
        color: var(--brand-indigo, #3E8F47);
        font-size: var(--text-10, 10px);
        font-weight: 800;
        line-height: 1;
        text-align: center;
        flex: 0 0 22px;
    }

    .activity-row div {
        flex: 1;
        min-width: 0;
    }

    .activity-row time {
        color: var(--text-muted, #8A978B);
        font-size: var(--text-10, 10px);
        font-weight: 700;
        white-space: nowrap;
    }

    .empty-state {
        min-height: 160px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--text-muted, #8A978B);
        font-size: var(--text-12, 12px);
    }

    .empty-action {
        width: 100%;
        border: 1px dashed var(--border, #DDE8DD);
        border-radius: 6px;
        background: #F8FBF7;
        cursor: pointer;
    }

    .empty-action:hover {
        color: var(--brand-indigo, #3E8F47);
        border-color: #9CB99D;
        background: #F3F8F3;
    }

    @media (max-width: 1180px) {
        .kpi-strip,
        .decision-grid {
            grid-template-columns: 1fr 1fr;
        }

        .pipeline-executive {
            grid-template-columns: 1fr;
        }
    }

    @media (max-width: 760px) {
        .dashboard-page {
            padding: 18px;
        }

        .dashboard-header {
            flex-direction: column;
        }

        .dashboard-actions {
            align-items: flex-start;
            flex-direction: column;
        }

        .kpi-strip,
        .decision-grid {
            grid-template-columns: 1fr;
        }

        .metric-meta,
        .focus-row,
        .receivable-row {
            align-items: flex-start;
            flex-direction: column;
        }

        .receivable-meta {
            grid-template-columns: 1fr auto;
        }

        .pipeline-row {
            grid-template-columns: 10px minmax(0, 1fr) auto;
        }

        .pipeline-row em {
            grid-column: 2 / -1;
            text-align: left;
        }

        .pipeline-selected,
        .aging-selected {
            grid-template-columns: 1fr;
        }
    }
</style>
