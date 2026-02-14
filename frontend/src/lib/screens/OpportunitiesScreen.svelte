<script lang="ts">
    import { onMount, createEventDispatcher } from "svelte";
    import { fade } from "svelte/transition";
    import OpportunityDetail from "../components/OpportunityDetail.svelte";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    
    // NEW DESIGN SYSTEM
    import Card from "../components/ui/Card.svelte";
    import Button from "../components/ui/Button.svelte";

    import {
        GetRFQs,
        CreateRFQ,
        DeleteRFQ,
        DeleteRFQWithCascade,
        ListCustomers,
    } from "../../../wailsjs/go/main/App";
    import { toast } from "../stores/toasts";
    import { devLog } from "$lib/utils/devLog";
    import { debounce } from "$lib/utils/debounce";

    const dispatch = createEventDispatcher();

    // State
    let opportunities = [];
    let filteredOpportunities = [];
    let selected = null;
    let loadingList = true;
    let showCreateForm = false;
    let creating = false;
    let customers = [];

    // Search & Filter State
    let searchQuery = '';
    let debouncedSearchQuery = '';
    let activeFilter = 'All';
    let sortBy = 'date'; // 'date', 'value', 'customer'
    let sortDirection = 'desc'; // 'asc' or 'desc'

    // Debounced search (300ms delay)
    const updateDebouncedSearch = debounce((value: string) => {
        debouncedSearchQuery = value;
        applyFilters();
    }, 300);

    // Watch searchQuery changes and debounce
    $: updateDebouncedSearch(searchQuery);

    // Delete confirmation state
    let showDeleteConfirm = false;
    let deleteTarget = null;
    let deleting = false;
    let pipelineStats = {
        newCount: 0,
        quotedCount: 0,
        wonCount: 0,
        totalValue: 0,
        winRate: 0,
    };

    // Create form
    let createForm = {
        customer: "",
        project: "",
        rfq_ref: "",
        received_date: new Date().toISOString().split("T")[0],
        due_date: "",
        priority: "Normal",
        value: "",
        notes: "",
    };

    const priorityOptions = ["Normal", "Urgent", "Critical"];
    const STATUS_OPTIONS = ['All', 'New', 'Quoted', 'Won', 'Lost', 'On Hold'];

    async function loadOpportunities() {
        loadingList = true;
        try {
            opportunities = (await GetRFQs(200, 0)) || [];
            calculateStats();
            applyFilters();
            if (filteredOpportunities.length > 0 && !selected)
                selected = filteredOpportunities[0];

            // Load customers for autocomplete
            const custList = await ListCustomers(100, 0);
            customers = custList || [];
        } catch (err) {
            devLog.error(err);
            toast.danger("Failed to load opportunities");
        } finally {
            loadingList = false;
        }
    }

    function calculateStats() {
        const stats = {
            newCount: 0,
            quotedCount: 0,
            wonCount: 0,
            totalValue: 0,
        };
        let won = 0,
            closed = 0;

        opportunities.forEach((o) => {
            const val = parseFloat(o.value) || 0;
            if (o.status === "New") stats.newCount++;
            if (o.status === "Quoted") stats.quotedCount++;
            if (o.status === "Won") {
                stats.wonCount++;
                won++;
                closed++;
            }
            if (o.status === "Lost") closed++;

            stats.totalValue += val;
        });

        pipelineStats = {
            ...stats,
            winRate: closed > 0 ? (won / closed) * 100 : 0,
        };
    }

    // Apply filters and sorting (uses debouncedSearchQuery for performance)
    function applyFilters() {
        let result = [...opportunities];

        // Status filter
        if (activeFilter !== 'All') {
            result = result.filter(o => o.status === activeFilter);
        }

        // Search filter (customer, project) - uses debounced value
        if (debouncedSearchQuery.trim()) {
            const q = debouncedSearchQuery.toLowerCase();
            result = result.filter(o =>
                o.client?.toLowerCase().includes(q) ||
                o.project?.toLowerCase().includes(q) ||
                o.customer?.toLowerCase().includes(q) ||
                o.project_name?.toLowerCase().includes(q)
            );
        }

        // Sorting
        result.sort((a, b) => {
            let aVal, bVal;

            if (sortBy === 'date') {
                aVal = new Date(a.created_at || 0).getTime();
                bVal = new Date(b.created_at || 0).getTime();
            } else if (sortBy === 'value') {
                aVal = parseFloat(a.value) || 0;
                bVal = parseFloat(b.value) || 0;
            } else if (sortBy === 'customer') {
                aVal = (a.client || a.customer || '').toLowerCase();
                bVal = (b.client || b.customer || '').toLowerCase();
            }

            if (sortDirection === 'asc') {
                return aVal > bVal ? 1 : aVal < bVal ? -1 : 0;
            } else {
                return aVal < bVal ? 1 : aVal > bVal ? -1 : 0;
            }
        });

        filteredOpportunities = result;
    }

    // Reactive updates
    $: {
        searchQuery;
        activeFilter;
        sortBy;
        sortDirection;
        applyFilters();
    }

    function selectOpportunity(opp) {
        selected = opp;
    }

    async function handleCreate() {
        if (!createForm.customer || !createForm.project) {
            toast.warning("Customer and Prospect/Project fields are required");
            return;
        }
        creating = true;
        try {
            await CreateRFQ(
                createForm.customer,
                createForm.project,
                parseFloat(createForm.value) || 0,
                createForm.notes,
            );
            toast.success("Opportunity Created");
            showCreateForm = false;
            // Reset form
            createForm = {
                customer: "",
                project: "",
                rfq_ref: "",
                received_date: new Date().toISOString().split("T")[0],
                due_date: "",
                priority: "Normal",
                value: "",
                notes: "",
            };
            await loadOpportunities();
        } catch (err) {
            toast.danger("Creation failed: " + err);
        } finally {
            creating = false;
        }
    }

    function formatCurrency(val) {
        return new Intl.NumberFormat("en-BH", { style: "currency", currency: "BHD", maximumFractionDigits: 0 }).format(val);
    }

    // Delete opportunity
    function confirmDelete(opp, event) {
        event.stopPropagation(); // Prevent selecting the item
        deleteTarget = opp;
        showDeleteConfirm = true;
    }

    async function handleDelete(cascade = false) {
        if (!deleteTarget) return;

        deleting = true;
        try {
            if (cascade) {
                await DeleteRFQWithCascade(deleteTarget.id, true);
            } else {
                await DeleteRFQ(deleteTarget.id);
            }
            toast.success(`Opportunity "${deleteTarget.project || 'Untitled'}" deleted`);

            // Clear selection if deleted item was selected
            if (selected?.id === deleteTarget.id) {
                selected = null;
            }

            showDeleteConfirm = false;
            deleteTarget = null;
            await loadOpportunities();
        } catch (err) {
            const errorMsg = err?.message || String(err);
            if (errorMsg.includes('costing sheet') || errorMsg.includes('offer')) {
                toast.warning("This opportunity has linked documents. Use 'Delete All' to remove everything.");
            } else {
                toast.danger("Delete failed: " + errorMsg);
            }
        } finally {
            deleting = false;
        }
    }

    onMount(() => {
        loadOpportunities();
    });
</script>

<div class="screen-container">
    <!-- Sub-header Actions -->
    <div class="action-bar">
        <div class="kpi-row">
            <div class="mini-kpi">
                <span class="kpi-label">Pipeline</span>
                <span class="kpi-value">{formatCurrency(pipelineStats.totalValue)}</span>
            </div>
            <div class="mini-kpi">
                <span class="kpi-label">Win Rate</span>
                <span class="kpi-value">{pipelineStats.winRate.toFixed(1)}%</span>
            </div>
            <div class="mini-kpi">
                <span class="kpi-label">Open</span>
                <span class="kpi-value">{pipelineStats.newCount + pipelineStats.quotedCount}</span>
            </div>
        </div>
        <Button variant="primary" on:click={() => (showCreateForm = true)}>+ New Opportunity</Button>
    </div>

    <!-- Filters Bar -->
    <div class="controls-bar">
        <div class="filter-tabs">
            {#each STATUS_OPTIONS as status}
                <button
                    class="filter-tab"
                    class:active={activeFilter === status}
                    on:click={() => activeFilter = status}
                >
                    {status} ({status === 'All' ? opportunities.length : opportunities.filter(o => o.status === status).length})
                </button>
            {/each}
        </div>

        <div class="right-controls">
            <div class="sort-controls">
                <select bind:value={sortBy} class="sort-select">
                    <option value="date">Date</option>
                    <option value="value">Value</option>
                    <option value="customer">Customer</option>
                </select>
                <button
                    class="sort-direction-btn"
                    on:click={() => sortDirection = sortDirection === 'asc' ? 'desc' : 'asc'}
                    title={sortDirection === 'asc' ? 'Ascending' : 'Descending'}
                    aria-label={sortDirection === 'asc' ? 'Sort ascending, click to sort descending' : 'Sort descending, click to sort ascending'}
                >
                    {sortDirection === 'asc' ? '(asc)' : '(desc)'}
                </button>
            </div>

            <div class="search-box">
                <input
                    type="text"
                    class="search-input"
                    placeholder="Search customer, project..."
                    bind:value={searchQuery}
                    aria-label="Search opportunities by customer or project"
                />
            </div>
        </div>
    </div>

    <div class="split-view">
        <!-- List Pane -->
        <div class="list-pane">
            <div class="list-header">
                <span class="result-count">
                    {filteredOpportunities.length} of {opportunities.length} opportunities
                </span>
            </div>
            <div class="list-content">
                {#if loadingList}
                    <div class="loading-state"><WabiSpinner size="md" tempo="calm" /></div>
                {:else if filteredOpportunities.length === 0}
                    <div class="empty-state">
                        {searchQuery || activeFilter !== 'All' ? 'No matching opportunities found.' : 'No opportunities.'}
                    </div>
                {:else}
                    {#each filteredOpportunities as opp}
                        <div
                            class="list-item"
                            class:selected={selected?.id === opp.id}
                            on:click={() => selectOpportunity(opp)}
                            on:keypress={(e) => e.key === 'Enter' && selectOpportunity(opp)}
                            role="button"
                            tabindex="0"
                        >
                            <div class="item-header">
                                <span class="cust-name">{opp.client || opp.customer || 'Unknown'}</span>
                                <div class="header-actions">
                                    {#if opp.rfq_number}
                                        <span class="rfq-num">{opp.rfq_number}</span>
                                    {/if}
                                    <span class="time">{new Date(opp.created_at).toLocaleDateString()}</span>
                                    <button
                                        class="delete-btn"
                                        on:click={(e) => confirmDelete(opp, e)}
                                        title="Delete opportunity"
                                        aria-label="Delete opportunity {opp.project || opp.project_name || 'Untitled'}"
                                    >Delete</button>
                                </div>
                            </div>
                            <div class="item-body">
                                <span class="project">{opp.project || opp.project_name || "New Requirements"}</span>
                            </div>
                            <div class="item-footer">
                                <span class="badge badge-neutral">{opp.stage || opp.status || 'New'}</span>
                                <span class="val">{formatCurrency(opp.value)}</span>
                            </div>
                        </div>
                    {/each}
                {/if}
            </div>
        </div>

        <!-- Detail Pane -->
        <div class="detail-pane">
            {#if selected}
                <div class="detail-content" in:fade={{ duration: 150 }}>
                    <OpportunityDetail
                        opportunity={selected}
                        on:updated={loadOpportunities}
                    />
                </div>
            {:else}
                <div class="empty-detail">
                    <p class="text-muted">Select an opportunity to view details</p>
                </div>
            {/if}
        </div>
    </div>
</div>

<!-- Create Modal -->
{#if showCreateForm}
    <div class="modal-backdrop" on:click={() => (showCreateForm = false)} on:keydown={(e) => e.key === 'Escape' && (showCreateForm = false)} role="dialog" aria-modal="true" aria-labelledby="create-modal-title">
        <div class="modal-card" on:click|stopPropagation role="document">
            <h3 id="create-modal-title" class="section-title">New Opportunity</h3>

            <form on:submit|preventDefault={handleCreate}>
                <div class="form-group">
                    <label class="label">Customer</label>
                    <input list="cust-list" bind:value={createForm.customer} class="input" placeholder="Search customer..." required />
                    <datalist id="cust-list">{#each customers as c}<option value={c.business_name} />{/each}</datalist>
                </div>

                <div class="form-group">
                    <label class="label">Project</label>
                    <input type="text" bind:value={createForm.project} class="input" placeholder="Project name" required />
                </div>

                <div class="grid grid-cols-2 gap-3">
                    <div class="form-group">
                        <label class="label">Value (BHD)</label>
                        <input type="number" bind:value={createForm.value} class="input" placeholder="0.00" />
                    </div>
                    <div class="form-group">
                        <label class="label">Priority</label>
                        <select bind:value={createForm.priority} class="input">
                            {#each priorityOptions as p}<option>{p}</option>{/each}
                        </select>
                    </div>
                </div>

                <div class="modal-actions">
                    <Button variant="secondary" on:click={() => (showCreateForm = false)}>Cancel</Button>
                    <Button variant="primary" type="submit" disabled={creating}>
                        {creating ? "Creating..." : "Create"}
                    </Button>
                </div>
            </form>
        </div>
    </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteConfirm && deleteTarget}
    <div class="modal-backdrop" on:click={() => showDeleteConfirm = false} on:keydown={(e) => e.key === 'Escape' && (showDeleteConfirm = false)} role="alertdialog" aria-modal="true" aria-labelledby="delete-modal-title">
        <div class="modal-card delete-modal" on:click|stopPropagation role="document">
            <div class="delete-header">
                <span class="delete-icon" aria-hidden="true"></span>
                <h3 id="delete-modal-title" class="section-title">Delete Opportunity?</h3>
            </div>

            <div class="delete-body">
                <p>Are you sure you want to delete this opportunity?</p>
                <div class="delete-details">
                    <div class="detail-row">
                        <span class="detail-label">Customer:</span>
                        <span class="detail-value">{deleteTarget.client || deleteTarget.customer || 'Unknown'}</span>
                    </div>
                    <div class="detail-row">
                        <span class="detail-label">Project:</span>
                        <span class="detail-value">{deleteTarget.project || 'Untitled'}</span>
                    </div>
                    <div class="detail-row">
                        <span class="detail-label">Value:</span>
                        <span class="detail-value">{formatCurrency(deleteTarget.value)}</span>
                    </div>
                </div>
                <p class="warning-text">This action cannot be undone.</p>
            </div>

            <div class="modal-actions delete-actions">
                <Button variant="secondary" on:click={() => showDeleteConfirm = false} disabled={deleting}>
                    Cancel
                </Button>
                <Button variant="danger" on:click={() => handleDelete(false)} disabled={deleting}>
                    {deleting ? "Deleting..." : "Delete"}
                </Button>
                <Button variant="danger" on:click={() => handleDelete(true)} disabled={deleting} title="Delete with all linked costing sheets and offers">
                    {deleting ? "..." : "Delete All"}
                </Button>
            </div>
        </div>
    </div>
{/if}

<style>
    .screen-container {
        height: 100%;
        display: flex;
        flex-direction: column;
        gap: var(--section-spacing);
    }

    .action-bar {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .kpi-row {
        display: flex;
        gap: 24px;
    }

    .mini-kpi {
        display: flex;
        flex-direction: column;
    }

    .kpi-label {
        font-size: 11px;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .kpi-value {
        font-size: 16px;
        font-weight: 600;
        color: var(--text-primary);
    }

    .split-view {
        display: grid;
        grid-template-columns: 320px 1fr;
        gap: var(--section-spacing); /* 16px */
        flex: 1;
        min-height: 0;
    }

    /* Controls Bar */
    .controls-bar {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
        margin-bottom: 16px;
        flex-wrap: wrap;
    }

    .filter-tabs {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
    }

    .filter-tab {
        padding: 8px 16px;
        border: 1px solid var(--border);
        background: transparent;
        border-radius: var(--border-radius);
        font-size: 12px;
        cursor: pointer;
        transition: all 0.2s;
        color: var(--text-secondary);
        white-space: nowrap;
    }

    .filter-tab:hover {
        background: var(--surface);
        color: var(--text-primary);
        border-color: var(--text-muted);
    }

    .filter-tab.active {
        background: var(--brand-indigo);
        color: white;
        border-color: var(--brand-indigo);
    }

    .right-controls {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .sort-controls {
        display: flex;
        align-items: center;
        gap: 4px;
    }

    .sort-select {
        padding: 8px 12px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        font-size: 12px;
        background: var(--surface);
        color: var(--text-primary);
        cursor: pointer;
        outline: none;
        transition: border-color 0.2s;
    }

    .sort-select:focus {
        border-color: var(--brand-indigo);
    }

    .sort-direction-btn {
        padding: 8px 12px;
        border: 1px solid var(--border);
        background: var(--surface);
        border-radius: var(--border-radius);
        font-size: 16px;
        cursor: pointer;
        transition: all 0.2s;
        line-height: 1;
    }

    .sort-direction-btn:hover {
        background: var(--background);
        border-color: var(--text-muted);
    }

    .search-box {
        flex-shrink: 0;
    }

    .search-input {
        padding: 8px 12px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        font-size: 13px;
        width: 240px;
        outline: none;
        transition: border-color 0.2s;
        background: var(--surface);
        color: var(--text-primary);
    }

    .search-input:focus {
        border-color: var(--brand-indigo);
    }

    .search-input::placeholder {
        color: var(--text-muted);
    }

    /* List Pane */
    .list-pane {
        display: flex;
        flex-direction: column;
        border-right: 1px solid var(--border);
        padding-right: var(--section-spacing);
    }

    .list-header {
        padding-bottom: 8px;
        margin-bottom: 8px;
        border-bottom: 1px solid var(--border);
    }

    .result-count {
        font-size: 11px;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .list-content {
        flex: 1;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .loading-state,
    .empty-state {
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 200px;
        color: var(--text-muted);
        font-size: 13px;
    }

    .list-item {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        padding: 12px;
        text-align: left;
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .list-item:hover {
        border-color: var(--text-muted);
        box-shadow: var(--shadow-sm);
    }

    .list-item.selected {
        border-color: var(--brand-indigo);
        background: var(--brand-indigo-tint);
    }

    .item-header {
        display: flex;
        justify-content: space-between;
        margin-bottom: 4px;
    }

    .cust-name {
        font-size: 13px;
        font-weight: 600;
        color: var(--text-primary);
    }

    .time {
        font-size: 11px;
        color: var(--text-muted);
    }

    .rfq-num {
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        font-size: 11px;
        font-weight: 600;
        color: var(--brand-indigo);
        background: var(--brand-indigo-tint);
        padding: 2px 6px;
        border-radius: 4px;
    }

    .project {
        font-size: 13px;
        color: var(--text-secondary);
    }

    .item-footer {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-top: 8px;
    }

    .val {
        font-size: 12px;
        font-weight: 600;
        color: var(--text-primary);
        font-family: var(--font-mono, monospace);
    }

    /* Detail Pane */
    .detail-pane {
        background: var(--surface);
        border-radius: var(--border-radius-lg);
        border: 1px solid var(--border);
        overflow: hidden;
        display: flex;
        flex-direction: column;
    }

    .detail-content {
        flex: 1;
        overflow-y: auto;
        padding: 24px;
    }

    .empty-detail {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
    }

    /* Modal */
    .modal-backdrop {
        position: fixed;
        top: 0; left: 0; right: 0; bottom: 0;
        background: rgba(0, 0, 0, 0.4);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 100;
        backdrop-filter: blur(2px);
    }

    .modal-card {
        background: var(--surface);
        padding: 24px;
        border-radius: var(--border-radius-lg);
        width: 400px;
        box-shadow: var(--shadow-lg);
    }

    .form-group {
        margin-bottom: 16px;
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: 12px;
        margin-top: 24px;
    }

    /* Delete Button & Modal */
    .header-actions {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .delete-btn {
        background: none;
        border: none;
        padding: 2px 6px;
        cursor: pointer;
        font-size: 12px;
        opacity: 0;
        transition: opacity 0.2s ease;
        border-radius: 4px;
    }

    .list-item:hover .delete-btn {
        opacity: 0.6;
    }

    .delete-btn:hover {
        opacity: 1 !important;
        background: rgba(220, 38, 38, 0.1);
    }

    .delete-modal {
        max-width: 420px;
    }

    .delete-header {
        display: flex;
        align-items: center;
        gap: 12px;
        margin-bottom: 16px;
    }

    .delete-icon {
        font-size: 24px;
    }

    .delete-body p {
        margin: 0 0 12px 0;
        color: var(--text-secondary);
    }

    .delete-details {
        background: var(--background);
        border-radius: var(--border-radius);
        padding: 12px;
        margin: 12px 0;
    }

    .detail-row {
        display: flex;
        justify-content: space-between;
        padding: 4px 0;
    }

    .detail-label {
        color: var(--text-muted);
        font-size: 12px;
    }

    .detail-value {
        color: var(--text-primary);
        font-size: 12px;
        font-weight: 500;
    }

    .warning-text {
        color: #dc2626;
        font-size: 12px;
        font-weight: 500;
    }

    .delete-actions {
        gap: 8px;
    }

    /* Button variant: danger */
    :global(.btn-danger) {
        background: #dc2626;
        color: white;
    }
    :global(.btn-danger:hover) {
        background: #b91c1c;
    }
</style>