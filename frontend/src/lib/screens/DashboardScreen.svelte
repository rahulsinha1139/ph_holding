<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { toast } from '$lib/stores/toasts';
    import { currentUser, permissions } from '../stores/authContext';
    import { devLog } from '$lib/utils/devLog';
    import {
        GetDashboardStats,
        ListFollowUps,
        CompleteFollowUp,
        PickFile,
    } from '../../../wailsjs/go/main/App';
    import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
    import { main } from '../../../wailsjs/go/models';
    import type { UITask } from '$lib/types';
    import { createEventDispatcher } from 'svelte';
    import Card from '../components/ui/Card.svelte';
    import KPICard from '../components/ui/KPICard.svelte';
    import WabiSpinner from '../components/ui/WabiSpinner.svelte';
    import FocusCard from '../components/ui/FocusCard.svelte';

    const dispatch = createEventDispatcher();

    // State
    let loading = true;

    // OCR Quick Capture State
    let ocrProcessing = false;
    let isDraggingOver = false;
    $: userName = $currentUser?.name || "Alex";

    // KPI Stats - mapped from backend DashboardStats
    let stats = {
        totalRevenue: 0,
        revenueMeta: "Live business value",
        revenueGrowth: 0,
        activityYear: new Date().getFullYear(),
        activeRFQs: 0,
        activeOrders: 0,
        outstandingAR: 0,
        arDaysOverdue: 0,
        pendingInvoices: 0,
        activeCustomers: 0,
        winRate: 0,
        tasksTotal: 0,
        tasksCompleted: 0,
    };

    let todaysTasks: main.FollowUpTask[] = [];
    let focusCardTasks: UITask[] = [];
    $: canViewTasks = $permissions.includes('*') || $permissions.includes('tasks:view');

    // Action Items data
    interface ActionItem {
        icon: string;
        title: string;
        count: number;
        screen: string;
        urgent: boolean;
    }

    let actionItems: ActionItem[] = [];

    async function loadData() {
        loading = true;
        try {
            const dashStats = await GetDashboardStats();
            if (dashStats) {
                stats.totalRevenue = dashStats.total_revenue || 0;
                stats.revenueMeta = dashStats.revenue_meta || `Live business value FY${dashStats.activity_year || new Date().getFullYear()}`;
                stats.revenueGrowth = dashStats.month_growth || 0;
                stats.activityYear = dashStats.activity_year || new Date().getFullYear();
                stats.activeRFQs = dashStats.active_rfqs || 0;
                stats.activeOrders = dashStats.active_orders || 0;
                stats.outstandingAR = dashStats.outstanding_ar || 0;
                stats.arDaysOverdue = dashStats.ar_days_overdue || 0;
                stats.pendingInvoices = dashStats.pending_invoices || 0;
                stats.activeCustomers = dashStats.active_customers || 0;
                stats.winRate = dashStats.win_rate || 0;
            }

            if (canViewTasks) {
                const tasks = await ListFollowUps(50);
                if (tasks) {
                    todaysTasks = tasks;
                    mapTasksToFocusCard(tasks);
                    stats.tasksTotal = todaysTasks.length;
                    stats.tasksCompleted = todaysTasks.filter(t => t.status === 'completed').length;
                }
            } else {
                todaysTasks = [];
                focusCardTasks = [];
                stats.tasksTotal = 0;
                stats.tasksCompleted = 0;
            }

            actionItems = [
                { icon: '', title: 'Overdue Invoices', count: stats.pendingInvoices, screen: 'finance', urgent: stats.arDaysOverdue > 30 || stats.pendingInvoices > 0 },
                { icon: '', title: 'Open Opportunities', count: stats.activeRFQs, screen: 'opportunities', urgent: false },
                { icon: '', title: 'Active Orders', count: stats.activeOrders, screen: 'operations', urgent: false },
            ];
        } catch (err) {
            const errorMsg = err?.message || String(err);
            toast.danger(`Failed to load dashboard data: ${errorMsg}`);
        } finally {
            loading = false;
        }
    }

    function mapTasksToFocusCard(tasks: main.FollowUpTask[]): void {
        focusCardTasks = tasks.map(t => ({
            id: t.id,
            title: t.title,
            subtitle: t.description || 'No details provided',
            time: '10:00 AM',
            priority: (t.priority as UITask['priority']) || 'Medium',
            urgent: t.priority === 'High' || t.priority === 'Urgent',
            tag: 'General',
            done: t.status === 'completed'
        })).slice(0, 5);
    }

    async function handleToggleTask(event) {
        const { id } = event.detail;
        const task = todaysTasks.find((t) => t.id === id);
        if (!task) return;

        const newStatus = task.status === 'completed' ? 'pending' : 'completed';
        const wasDone = task.status === 'completed';

        todaysTasks = todaysTasks.map(t => {
            if (t.id === id) {
                return main.FollowUpTask.createFrom({ ...t, status: newStatus });
            }
            return t;
        });
        mapTasksToFocusCard(todaysTasks);

        if (newStatus === 'completed') stats.tasksCompleted++;
        else stats.tasksCompleted--;

        try {
            await CompleteFollowUp(String(id));
        } catch (e) {
            toast.danger('Failed to update task');
            todaysTasks = todaysTasks.map(t => {
                if (t.id === id) {
                    return main.FollowUpTask.createFrom({ ...t, status: wasDone ? 'completed' : 'pending' });
                }
                return t;
            });
            mapTasksToFocusCard(todaysTasks);
            if (wasDone) stats.tasksCompleted++; else stats.tasksCompleted--;
        }
    }

    function formatCurrency(val: number): string {
        if (val >= 1000000) return 'BHD ' + (val / 1000000).toFixed(1) + 'M';
        if (val >= 1000) return 'BHD ' + (val / 1000).toFixed(0) + 'k';
        return 'BHD ' + val.toLocaleString();
    }

    onMount(() => {
        loadData();

        // Listen for data refresh events (e.g., when orders are delivered)
        EventsOn('data:refresh', (data: any) => {
            devLog.info('Dashboard received data:refresh event', data);
            loadData();
        });
    });

    onDestroy(() => {
        EventsOff('data:refresh');
    });

    function navigateToScreen(screen: string) {
        const screenMap: Record<string, string> = {
            'invoices': 'finance',
            'rfqs': 'sales',
            'grns': 'operations',
            'contracts': 'sales',
            'products': 'operations'
        };

        const targetScreen = screenMap[screen] || screen;
        dispatch('navigate', { screen: targetScreen });
    }

    async function handleBrowseFile() {
        if (ocrProcessing) return;

        try {
            const filePath = await PickFile('Select Document for OCR');
            if (filePath && filePath.trim() !== '') {
                window.dispatchEvent(new CustomEvent('requestGlobalOCR', {
                    detail: { filePath }
                }));
            }
        } catch (err) {
            toast.danger('File selection cancelled or failed');
        }
    }

    let dragCounter = 0;

    function handlePageDragEnter(e: DragEvent) {
        e.preventDefault();
        dragCounter++;
        if (dragCounter === 1) isDraggingOver = true;
    }

    function handlePageDragOver(e: DragEvent) {
        e.preventDefault();
    }

    function handlePageDragLeave(e: DragEvent) {
        e.preventDefault();
        dragCounter--;
        if (dragCounter === 0) isDraggingOver = false;
    }

    function handlePageDrop(e: DragEvent) {
        e.preventDefault();
        dragCounter = 0;
        isDraggingOver = false;
    }
</script>

<div
    class="page-container"
    class:drag-active={isDraggingOver}
    on:dragenter={handlePageDragEnter}
    on:dragover={handlePageDragOver}
    on:dragleave={handlePageDragLeave}
    on:drop={handlePageDrop}
    role="region"
    aria-label="Dashboard - drop files anywhere to analyze"
>
    <!-- Full-page drop overlay -->
    {#if isDraggingOver}
        <div class="drop-overlay" transition:fade={{ duration: 150 }}>
            <div class="drop-overlay-content">
                <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                    <polyline points="17 8 12 3 7 8"></polyline>
                    <line x1="12" y1="3" x2="12" y2="15"></line>
                </svg>
                <p>Release to analyze document</p>
            </div>
        </div>
    {/if}

    <header class="page-header">
        <div>
            <h1 class="page-title">Overview.</h1>
            <p class="meta">
                {new Date().toLocaleDateString("en-GB", {
                    weekday: "long",
                    day: "numeric",
                    month: "long",
                })}
            </p>
        </div>
    </header>

    {#if loading}
        <div class="loading-state"><WabiSpinner size="lg" tempo="calm" /></div>
    {:else}
        <div class="dashboard-layout" in:fade>
            <!-- Top KPIs Strip - 6 KPIs in responsive grid -->
            <div class="kpi-grid">
                <!-- Total Revenue with YTD trend -->
                <KPICard
                    label="Total Revenue (YTD)"
                    value={formatCurrency(stats.totalRevenue)}
                    meta={stats.revenueMeta}
                    trend={stats.revenueGrowth > 0 ? 'up' : stats.revenueGrowth < 0 ? 'down' : 'neutral'}
                    accent
                />

                <!-- Open Opportunities -->
                <KPICard
                    label="Open Opportunities"
                    value={String(stats.activeRFQs)}
                    meta="Live Pipeline"
                />

                <!-- Active Orders -->
                <KPICard
                    label="Active Orders"
                    value={String(stats.activeOrders)}
                    meta="In Fulfillment"
                />

                <!-- Outstanding AR -->
                <KPICard
                    label="Outstanding AR"
                    value={formatCurrency(stats.outstandingAR)}
                    meta={stats.arDaysOverdue > 0 ? `Avg ${stats.arDaysOverdue} days overdue` : "All current"}
                    trend={stats.arDaysOverdue > 30 ? 'down' : 'neutral'}
                />

                <!-- Pending Invoices -->
                <KPICard
                    label="Pending Invoices"
                    value={String(stats.pendingInvoices)}
                    meta="Awaiting Payment"
                />

                <!-- Active Customers -->
                <KPICard
                    label="Active Customers"
                    value={String(stats.activeCustomers)}
                    meta="In Database"
                />

                <!-- Win Rate -->
                <KPICard
                    label="Win Rate"
                    value={stats.winRate.toFixed(1) + "%"}
                    meta="Opportunities Won"
                    trend={stats.winRate >= 50 ? 'up' : stats.winRate < 30 ? 'down' : 'neutral'}
                />
            </div>

            <!-- OCR Document Intelligence - Minimal Horizontal Bar -->
            <div class="ocr-dropbar" class:processing={ocrProcessing} class:dragging={isDraggingOver}>
                <div class="dropbar-left">
                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                        <polyline points="14 2 14 8 20 8"></polyline>
                    </svg>
                    <span class="dropbar-text">
                        {#if ocrProcessing}
                            <strong>Processing document...</strong>
                        {:else if isDraggingOver}
                            <strong>Release to analyze document</strong>
                        {:else}
                            Drop files anywhere • PDF, Excel, Word, Images, Emails
                        {/if}
                    </span>
                </div>
                <button
                    class="btn-browse-mini"
                    on:click={handleBrowseFile}
                    disabled={ocrProcessing}
                >
                    {#if ocrProcessing}
                        <span class="spinner-small"></span>
                    {:else}
                        Browse
                    {/if}
                </button>
            </div>

            <!-- Main Dashboard Grid -->
            <div class="main-grid">
                
                <!-- Column 1: Focus Card (Tasks) -->
                <div class="grid-col">
                    <!-- FocusCard handles its own card styling -->
                    <FocusCard 
                        tasks={focusCardTasks} 
                        on:toggleTask={handleToggleTask} 
                    />
                </div>

                <!-- Column 2: Actions -->
                <div class="grid-col double">
                    <!-- Action Items / Compliance -->
                    <Card title="Action Items" variant="accent">
                        <div class="action-list">
                            {#each actionItems as item}
                                <button
                                    class="action-item"
                                    on:click={() => navigateToScreen(item.screen)}
                                    aria-label="{item.title}: {item.count} items"
                                >
                                    <div class="action-icon" class:urgent={item.urgent}>
                                        {item.icon}
                                    </div>
                                    <div class="action-content">
                                        <span class="action-title">{item.title}</span>
                                        <span class="action-meta">{item.count} items</span>
                                    </div>
                                    <div class="action-arrow">></div>
                                </button>
                            {/each}
                        </div>
                    </Card>
                </div>
            </div>

        </div>
    {/if}
</div>

<style>
    .page-container {
        padding: var(--page-padding);
        min-height: 100vh;
        background: var(--bg-base);
        color: var(--text-primary);
        display: flex;
        flex-direction: column;
    }

    .page-header {
        margin-bottom: var(--section-spacing);
        height: var(--header-height);
        display: flex;
        align-items: center;
        justify-content: space-between;
    }

    .loading-state {
        flex: 1;
        display: flex;
        justify-content: center;
        align-items: center;
    }

    .dashboard-layout {
        display: flex;
        flex-direction: column;
        gap: var(--section-spacing);
        flex: 1;
    }

    /* KPI GRID - Responsive 6-column layout */
    .kpi-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
        gap: calc(var(--grid-gap) + 2px);
        align-items: stretch;
    }

    .kpi-grid :global(.kpi-card) {
        min-height: clamp(112px, 13vh, 132px);
    }

    .kpi-grid :global(.card) {
        padding: 14px 16px;
    }

    @media (max-width: 1600px) {
        .kpi-grid {
            grid-template-columns: repeat(4, 1fr);
        }
    }

    @media (max-width: 900px) {
        .kpi-grid {
            grid-template-columns: repeat(2, 1fr);
        }
    }

    /* MAIN GRID */
    .main-grid {
        display: grid;
        grid-template-columns: 1fr 2fr;
        gap: var(--grid-gap);
        flex: 1;
        min-height: 0;
        align-items: stretch;
    }

    .grid-col {
        display: flex;
        flex-direction: column;
        gap: var(--grid-gap);
        min-height: 0;
    }

    .grid-col :global(.focus-card),
    .grid-col :global(.card) {
        min-height: 320px;
    }

    /* Responsive - Main Grid */
    @media (max-width: 1200px) {
        .main-grid {
            grid-template-columns: 1fr;
        }
    }

    /* Action Items */
    .action-list {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .action-item {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px;
        background: var(--surface, white);
        border: none;
        border-radius: var(--border-radius-sm, 6px);
        cursor: pointer;
        transition: background var(--transition-fast, 0.15s);
        text-align: left;
        width: 100%;
    }

    .action-item:hover {
        background: var(--brand-indigo-tint, rgba(99, 102, 241, 0.08));
    }

    .action-icon {
        width: 36px;
        height: 36px;
        display: flex;
        align-items: center;
        justify-content: center;
        background: var(--bg-base, #fafafa);
        border-radius: var(--border-radius-sm, 6px);
        font-size: 18px;
    }

    .action-icon.urgent {
        background: #FEF2F2;
        color: #EF4444;
    }

    .action-content {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .action-title {
        font-weight: 500;
        color: var(--text-primary, #000);
    }

    .action-meta {
        font-size: var(--meta-size, 11px);
        color: var(--text-muted, #999);
    }

    .action-arrow {
        margin-left: auto;
        color: var(--text-muted, #999);
        font-size: 18px;
    }

    .spinner-small {
        width: 14px;
        height: 14px;
        border: 2px solid rgba(255, 255, 255, 0.3);
        border-top-color: white;
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }

    /* Page-level drop overlay */
    .page-container {
        position: relative;
    }

    .page-container.drag-active {
        outline: 1px solid rgba(255, 255, 255, 0.22);
        box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
    }

    .drop-overlay {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.6);
        backdrop-filter: blur(4px);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 9999;
        pointer-events: none;
    }

    .drop-overlay-content {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 16px;
        color: white;
        text-align: center;
    }

    .drop-overlay-content svg {
        opacity: 0.9;
    }

    .drop-overlay-content p {
        font-size: 20px;
        font-weight: 600;
        margin: 0;
        letter-spacing: -0.02em;
    }

    /* Minimal OCR Dropbar */
    .ocr-dropbar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 16px;
        background: var(--canvas, #FFFFFF);
        border: 1px solid var(--border, #E5E5E5);
        border-radius: 8px;
        transition: all 200ms ease;
    }

    .ocr-dropbar.dragging {
        border-color: var(--carbon, #000000);
        background: rgba(0, 0, 0, 0.02);
        transform: scale(1.005);
    }

    .ocr-dropbar.processing {
        border-color: var(--steel, #86868B);
    }

    .dropbar-left {
        display: flex;
        align-items: center;
        gap: 12px;
        color: var(--steel, #86868B);
    }

    .dropbar-left svg {
        flex-shrink: 0;
    }

    .dropbar-text {
        font-size: 13px;
        color: var(--steel, #86868B);
    }

    .dropbar-text strong {
        color: var(--onyx, #1D1D1F);
        font-weight: 600;
    }

    .btn-browse-mini {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 6px 14px;
        background: var(--ether, #F5F5F7);
        border: 1px solid var(--border, #E5E5E5);
        border-radius: 6px;
        font-size: 12px;
        font-weight: 500;
        color: var(--onyx, #1D1D1F);
        cursor: pointer;
        transition: all 150ms ease;
    }

    .btn-browse-mini:hover:not(:disabled) {
        background: var(--canvas, #FFFFFF);
        border-color: var(--steel, #86868B);
    }

    .btn-browse-mini:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

</style>
