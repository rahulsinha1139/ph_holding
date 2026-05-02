<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import { fade } from "svelte/transition";
    import { toast } from "$lib/stores/toasts";
    import { currentUser, permissions } from "../stores/authContext";
    import { devLog } from "$lib/utils/devLog";
    import { GetDashboardStats, ListFollowUps } from "../../../wailsjs/go/main/App";
    import { EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime";
    import { main } from "../../../wailsjs/go/models";
    import ContextTaskModal from "../components/ContextTaskModal.svelte";
    import { listMyTasks, listTeamTasks, refreshCollaborativeWorkspace, type CollaborativeTask } from "$lib/api/collaboration";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";

    let loading = true;
    let now = new Date();
    let clockTimer: ReturnType<typeof setInterval> | undefined;

    $: userName = formatUserName($currentUser);
    $: greeting = getTimeGreeting(now);
    $: activePermissions = Array.isArray($permissions) ? $permissions : [];
    function hasPermission(permission: string): boolean {
        const permissionList = Array.isArray(activePermissions) ? activePermissions : [];
        if (permissionList.includes("*") || permissionList.includes(permission)) return true;
        const [resource] = permission.split(":");
        return permissionList.includes(`${resource}:*`);
    }
    $: canViewTasks = hasPermission("tasks:view");
    $: canViewFinance = hasPermission("finance:view");

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

    $: activeDashboardTasks = dashboardTasks
        .filter((task) => !["completed", "archived", "cancelled", "canceled"].includes((task.status || "").toLowerCase()))
        .slice(0, 5);
    $: followUpSummary = buildFollowUpSummary(followUpTasks, activeDashboardTasks);
    $: totalTaskSignals = followUpTasks.length + activeDashboardTasks.length;

    $: pressureLevel = canViewFinance
        ? stats.cashBalance > 0
            ? "Cash position is the primary decision surface today"
            : stats.pipelineValue > stats.outstandingAR
              ? "Pipeline quality is the primary decision surface today"
              : "Commercial finance is the primary decision surface today"
        : totalTaskSignals > 0
          ? "Follow-ups and pipeline conversion are the primary decision surface today"
          : "Pipeline conversion is the primary decision surface today";

    $: kpis = canViewFinance
        ? [
              {
                  label: `Revenue FY${stats.activityYear}`,
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
                  meta: `FY${stats.activityYear} closed opportunities`,
                  tone: stats.winRate >= 20 ? "positive" : "warning",
                  delta: "won / closed",
              },
          ];

    $: operatingFocus = canViewFinance
        ? [
              {
                  title: "Cash balance",
                  detail:
                      stats.cashPositionNote
                          ? stats.cashPositionNote
                          : `Bank cash is ${formatCurrency(stats.cashBalance)}`,
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

    $: alerts = canViewFinance
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
                    `Confirmed business FY${rawStats.activity_year || new Date().getFullYear()}`;
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
            const errorMsg = err?.message || String(err);
            toast.danger(`Failed to load dashboard data: ${errorMsg}`);
        } finally {
            loading = false;
        }

        void loadTaskSignals({ refreshRemote: true });
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

        if (options.refreshRemote) {
            await refreshCollaborativeWorkspace({ minIntervalMs: 5_000 }).catch((err) => {
                devLog.warn("Dashboard collaborative refresh failed", err);
            });
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
            dashboardTasks = await listTeamTasks(false).catch((err) => {
                devLog.warn("Dashboard team tasks failed", err);
                return [];
            });
        }
    }

    $: if (!canViewTasks && taskSignalsPermissionLoaded) {
        taskSignalsPermissionLoaded = false;
    }

    $: if (canViewTasks && !taskSignalsPermissionLoaded) {
        taskSignalsPermissionLoaded = true;
        void loadTaskSignals({ refreshRemote: true });
    }

    function formatCurrency(val: number): string {
        return `${Number(val || 0).toLocaleString("en-US", {
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
        })} BHD`;
    }

    function formatCompactCurrency(val: number): string {
        const amount = Number(val || 0);
        return `${amount.toLocaleString("en-US", { maximumFractionDigits: 0 })} BHD`;
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

    function buildFollowUpSummary(
        legacyFollowUps: main.FollowUpTask[],
        activeTasks: CollaborativeTask[],
    ): string {
        if (legacyFollowUps.length > 0) {
            const first = legacyFollowUps[0];
            return `* ${summarizeText(first.title || first.description || first.notes || "Follow-up pending")}`;
        }
        if (activeTasks.length > 0) {
            return `* ${summarizeText(activeTasks[0].title || activeTasks[0].description || "Task pending")}`;
        }
        return "* No follow-up issues";
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
        <time datetime={now.toISOString()}>{formatDate(now)}</time>
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

                {#if canViewFinance}
                    <article class="panel receivables-panel">
                        <div class="panel-head">
                            <span>Collections</span>
                            <strong>{stats.pendingInvoices} open</strong>
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
                    </article>
                {/if}

                <article class="panel activity-panel" class:wide={!canViewFinance}>
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
        color: #17202a;
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
        color: #2d66bd;
        font-size: 12px;
        font-weight: 700;
    }

    .dashboard-header h1 {
        margin: 0;
        color: #43546b;
        font-size: 28px;
        font-weight: 500;
        line-height: 1.1;
    }

    .dashboard-subtitle {
        margin: 6px 0 0;
        color: #708299;
        font-size: 12px;
        font-weight: 600;
    }

    .dashboard-header time {
        color: #7d8fa6;
        font-size: 12px;
        white-space: nowrap;
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
        border: 1px solid rgba(205, 216, 226, 0.82);
        border-radius: 8px;
        box-shadow: 0 10px 28px rgba(39, 54, 73, 0.035);
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
        background: #d6e0ea;
    }

    .metric-card[data-tone="positive"]::before {
        background: #35a66f;
    }

    .metric-card[data-tone="negative"]::before {
        background: #d94a59;
    }

    .metric-card[data-tone="warning"]::before {
        background: #d99522;
    }

    .metric-label,
    .panel-head span {
        color: #8190a2;
        font-size: 10px;
        font-weight: 800;
        letter-spacing: 0;
        text-transform: uppercase;
    }

    .metric-value {
        margin-top: 10px;
        color: #253244;
        font-size: 24px;
        line-height: 1;
        font-weight: 700;
    }

    .metric-meta {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        margin-top: 8px;
        color: #7a899b;
        font-size: 11px;
        line-height: 1.3;
    }

    .metric-meta strong {
        color: #2c8a5f;
        font-weight: 700;
        text-align: right;
    }

    .metric-card[data-tone="negative"] .metric-meta strong {
        color: #ce3c4b;
    }

    .metric-card[data-tone="warning"] .metric-meta strong {
        color: #b87511;
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
        color: #8da0b4;
        font-size: 11px;
        font-weight: 700;
    }

    .panel-action {
        padding: 6px 9px;
        border: 1px solid #c9d6e4;
        border-radius: 5px;
        background: #ffffff;
        color: #2b5f9e;
        font-size: 11px;
        font-weight: 800;
        cursor: pointer;
    }

    .panel-action:hover {
        border-color: #8fb1d5;
        background: #f4f8fc;
    }

    .operating-panel h2 {
        margin: 0 0 18px;
        color: #293447;
        font-size: 18px;
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
        background: #f4f7fa;
    }

    .focus-row {
        padding: 12px 14px;
        border-left: 3px solid #a8bbd1;
    }

    .focus-row[data-tone="critical"] {
        background: #fff0f1;
        border-color: #dc3446;
    }

    .focus-row[data-tone="warning"] {
        background: #fff7e9;
        border-color: #e69a22;
    }

    .focus-row[data-tone="info"] {
        background: #edf5ff;
        border-color: #2f70c7;
    }

    .focus-row strong,
    .activity-row strong,
    .receivable-row strong {
        display: block;
        color: #2d394a;
        font-size: 13px;
        font-weight: 800;
    }

    .focus-row span,
    .activity-row div span,
    .receivable-row span,
    .alert-row span {
        display: block;
        margin-top: 4px;
        color: #6d7e91;
        font-size: 11px;
        line-height: 1.35;
    }

    .focus-action {
        min-width: 58px;
        padding: 7px 10px;
        border: 0;
        border-radius: 5px;
        background: #2265b9;
        color: white;
        font-size: 11px;
        font-weight: 800;
        text-align: center;
        cursor: pointer;
        transition:
            background 0.15s ease,
            transform 0.15s ease,
            box-shadow 0.15s ease;
    }

    .focus-action:hover {
        background: #1a5298;
        box-shadow: 0 6px 14px rgba(34, 101, 185, 0.16);
        transform: translateY(-1px);
    }

    .focus-action:focus-visible {
        outline: 2px solid #79a9df;
        outline-offset: 2px;
    }

    .alert-row {
        align-items: flex-start;
        justify-content: flex-start;
        padding: 12px 14px;
        border-left: 3px solid #b8c7d7;
    }

    .alert-row[data-tone="critical"] {
        background: #fff0f2;
        border-color: #dc3446;
    }

    .alert-row[data-tone="warning"] {
        background: #fff7e8;
        border-color: #e69a22;
    }

    .alert-row[data-tone="info"] {
        background: #eef6ff;
        border-color: #4f89d8;
    }

    .alert-row[data-tone="soft"] {
        background: #f6f8fb;
        border-color: #a9b8c8;
    }

    .alert-row strong {
        min-width: 96px;
        color: #334154;
        font-size: 12px;
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
        background: #fff7f8;
    }

    .receivable-row small {
        color: #ce3c4b;
        font-size: 11px;
        font-weight: 800;
        white-space: nowrap;
    }

    .collection-bar {
        height: 8px;
        margin: 18px 0 14px;
        border-radius: 999px;
        background: #dce6ef;
        overflow: hidden;
    }

    .collection-bar span {
        display: block;
        height: 100%;
        border-radius: inherit;
        background: #35a66f;
    }

    .receivable-meta {
        display: grid;
        grid-template-columns: 1fr auto 1fr auto;
        gap: 8px;
        align-items: center;
        color: #8190a2;
        font-size: 11px;
    }

    .receivable-meta strong {
        color: #2d394a;
    }

    .activity-row {
        padding: 10px 0;
        background: transparent;
        border-bottom: 1px solid #e3ebf2;
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
        background: #e8f1f8;
        color: #47719c;
        font-size: 10px;
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
        color: #8da0b4;
        font-size: 10px;
        font-weight: 700;
        white-space: nowrap;
    }

    .empty-state {
        min-height: 160px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: #8da0b4;
        font-size: 12px;
    }

    .empty-action {
        width: 100%;
        border: 1px dashed #cbd8e4;
        border-radius: 6px;
        background: #f8fbfd;
        cursor: pointer;
    }

    .empty-action:hover {
        color: #2b5f9e;
        border-color: #9bb9d8;
        background: #f3f8fd;
    }

    @media (max-width: 1180px) {
        .kpi-strip,
        .decision-grid {
            grid-template-columns: 1fr 1fr;
        }
    }

    @media (max-width: 760px) {
        .dashboard-page {
            padding: 18px;
        }

        .dashboard-header {
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
    }
</style>
