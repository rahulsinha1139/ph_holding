<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { onMount } from 'svelte';
    import { GetRFQTraceability, GetPurchaseOrdersByOrder, GetRFQ, AddRFQComment, GetRFQComments, UpdateRFQStage } from '../../../wailsjs/go/main/App';
    import { devLog } from '$lib/utils/devLog';
    import { toast } from '$lib/stores/toasts';

    type Opportunity = {
        id?: number;
        rfq_number?: string;  // RFQData rfq_number field
        stage?: string;       // RFQData stage field (9-stage pipeline)
        status?: keyof typeof statusPalette;
        title?: string;
        project?: string;  // RFQData uses 'project' field
        customer?: string;
        client?: string;   // RFQData uses 'client' field
        value?: number;
        notes?: string;    // Description/notes field
        product_details?: string;  // JSON line items from OCR
        created_at?: any;  // time.Time from Go
        updated_at?: any;  // time.Time from Go
    };

    // Parse product_details JSON into an array
    interface LineItem {
        description?: string;
        quantity?: number;
        unit_price?: number;
        total_price?: number;
        part_number?: string;
        unit?: string;
    }

    function parseProductDetails(productDetails?: string): LineItem[] {
        if (!productDetails) return [];
        try {
            const parsed = JSON.parse(productDetails);
            // Handle both array and single object formats
            if (Array.isArray(parsed)) return parsed;
            if (typeof parsed === 'object') return [parsed];
            return [];
        } catch {
            return [];
        }
    }

    $: lineItems = parseProductDetails(opportunity?.product_details);

    type HistoryEntry = {
        status?: keyof typeof statusPalette;
        note?: string;
        createdAt?: string | number | Date | null;
    };

    type Capture = {
        text?: string;
    };

    // Compatible with Wails-generated main.PurchaseOrder
    interface PurchaseOrder {
        id: string;
        created_at?: any;
        updated_at?: any;
        version?: number;
        created_by?: string;
        order_id?: string;
        rfq_id?: string;
        supplier_id?: string;
        supplier_name: string;
        po_number: string;
        po_date: any;
        expected_delivery: any;
        currency?: string;
        exchange_rate?: number;
        subtotal_foreign?: number;
        subtotal_bhd?: number;
        vat_amount?: number;
        total_foreign?: number;
        total_bhd: number;
        payment_terms?: string;
        payment_due_date?: any;
        status: string;
        approved_by?: string;
        approved_at?: any;
        updated_by?: string;
        items?: any[];
    }

    // Using any to accommodate Wails RFQData which has additional fields
    export let opportunity: any = null;
    export let history: HistoryEntry[] = [];
    export let quickCaptures: Capture[] = [];
    export let loading = false;
    export let error = '';

    const dispatch = createEventDispatcher();

    // Tab state
    let activeTab: 'details' | 'purchase-orders' = 'details';

    // PO state
    let purchaseOrders: PurchaseOrder[] = [];
    let loadingPOs = false;
    let poError = '';
    let orderId = '';

    // Comments state (Phase 3)
    let comments: any[] = [];
    let newComment = '';
    let addingComment = false;
    let updatingStage = false;

    // 9-stage pipeline options (Phase 2)
    const stageOptions = [
        'RFQ Received', 'Offer Sent', 'Follow-up/Eval',
        'PO/LOI Received', 'Order Placed', 'In Process',
        'Delivered', 'Closed (Payment)', 'Closed (Lost)'
    ];

    // Load POs when tab is activated and we have an opportunity
    async function loadPurchaseOrders() {
        if (!opportunity?.id) return;

        loadingPOs = true;
        poError = '';

        try {
            // First, get the order_id from RFQ traceability
            const traceability = await GetRFQTraceability(opportunity.id.toString());

            if (traceability && traceability.order_id) {
                orderId = traceability.order_id;

                // Then fetch POs for that order
                const pos = await GetPurchaseOrdersByOrder(orderId);
                purchaseOrders = pos || [];
            } else {
                purchaseOrders = [];
                orderId = '';
            }
        } catch (err) {
            devLog.error('Failed to load purchase orders:', err);
            poError = 'Failed to load purchase orders';
            purchaseOrders = [];
        } finally {
            loadingPOs = false;
        }
    }

    // Watch for tab changes
    $: if (activeTab === 'purchase-orders' && opportunity?.id && purchaseOrders.length === 0) {
        loadPurchaseOrders();
    }

    // Phase 4: Refresh opportunity data on tab change
    $: if (activeTab && opportunity?.id) {
        refreshOpportunityData();
    }

    // Phase 3: Load comments when opportunity changes
    $: if (opportunity?.id) {
        loadComments();
    }

    // Refresh opportunity data
    async function refreshOpportunityData() {
        if (!opportunity?.id) return;
        try {
            const fresh = await GetRFQ(opportunity.id);
            if (fresh) {
                opportunity = { ...opportunity, ...fresh };
            }
        } catch (err) {
            devLog.error('Failed to refresh opportunity:', err);
        }
    }

    // Load comments for this opportunity
    async function loadComments() {
        if (!opportunity?.id) return;
        try {
            comments = await GetRFQComments(opportunity.id) || [];
        } catch (err) {
            devLog.error('Failed to load comments:', err);
            comments = [];
        }
    }

    // Add a new comment
    async function handleAddComment() {
        if (!newComment.trim() || !opportunity?.id) return;
        addingComment = true;
        try {
            await AddRFQComment(opportunity.id, newComment.trim(), 'User');
            newComment = '';
            await loadComments();
            toast.success('Comment added');
        } catch (err) {
            devLog.error('Failed to add comment:', err);
            toast.danger('Failed to add comment');
        } finally {
            addingComment = false;
        }
    }

    // Update stage
    async function handleStageChange(event: Event) {
        const newStage = (event.target as HTMLSelectElement).value;
        if (!opportunity?.id || !newStage) return;
        updatingStage = true;
        try {
            await UpdateRFQStage(opportunity.id, newStage);
            opportunity = { ...opportunity, stage: newStage };
            dispatch('updated');
            toast.success(`Stage updated to ${newStage}`);
        } catch (err) {
            devLog.error('Failed to update stage:', err);
            toast.danger('Failed to update stage');
        } finally {
            updatingStage = false;
        }
    }

    const statusPalette = {
        New: 'var(--color-ink)',
        Quoted: 'var(--color-gold)',
        Won: 'var(--color-safe)',
        Lost: 'var(--color-danger)',
    };

    const formatter = new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD',
        maximumFractionDigits: 0,
    });

    $: allowed = opportunity && opportunity.status
        ? opportunity.status === 'New'
            ? ['Quoted', 'Won', 'Lost']
            : opportunity.status === 'Quoted'
                ? ['Won', 'Lost']
                : []
        : [];

    function requestStatusChange(next) {
        dispatch('statuschange', next);
    }
</script>

<div class="detail">
    {#if loading}
        <div class="skeleton title"></div>
        <div class="skeleton line"></div>
        <div class="skeleton line short"></div>
    {:else if error}
        <p class="error">{error}</p>
    {:else if opportunity}
        <header class="header">
            <div>
                <div class="header-meta">
                    {#if opportunity.rfq_number}
                        <span class="rfq-number">{opportunity.rfq_number}</span>
                    {/if}
                    <select
                        class="stage-select"
                        value={opportunity.stage || 'RFQ Received'}
                        on:change={handleStageChange}
                        disabled={updatingStage}
                    >
                        {#each stageOptions as stage}
                            <option value={stage}>{stage}</option>
                        {/each}
                    </select>
                </div>
                <h2>{opportunity.project || opportunity.title || 'Untitled'}</h2>
                <p class="subtitle">{opportunity.client || opportunity.customer || 'Unknown'}</p>
            </div>
            <div class="value">{formatter.format(opportunity.value || 0)}</div>
        </header>

        <!-- Description/Notes Section -->
        {#if opportunity.notes}
            <div class="description">
                <p class="mono">Description</p>
                <div class="description-content">
                    {opportunity.notes}
                </div>
            </div>
        {/if}

        <!-- Tabs Navigation -->
        <div class="tabs-nav">
            <button
                class="tab-btn"
                class:active={activeTab === 'details'}
                on:click={() => activeTab = 'details'}
            >
                Details
            </button>
            <button
                class="tab-btn"
                class:active={activeTab === 'purchase-orders'}
                on:click={() => activeTab = 'purchase-orders'}
            >
                Purchase Orders
                {#if purchaseOrders.length > 0}
                    <span class="tab-count">{purchaseOrders.length}</span>
                {/if}
            </button>
        </div>

        <!-- Tab Content -->
        {#if activeTab === 'details'}
            <!-- Line Items from OCR (product_details) -->
            {#if lineItems.length > 0}
                <div class="line-items-section">
                    <p class="mono">Line Items ({lineItems.length})</p>
                    <div class="items-table">
                        <div class="items-header">
                            <span class="item-col-desc">Description</span>
                            <span class="item-col-qty">Qty</span>
                            <span class="item-col-price">Unit Price</span>
                            <span class="item-col-total">Total</span>
                        </div>
                        {#each lineItems as item, i}
                            <div class="items-row">
                                <span class="item-col-desc">
                                    {item.description || `Item ${i + 1}`}
                                    {#if item.part_number}
                                        <span class="part-number">{item.part_number}</span>
                                    {/if}
                                </span>
                                <span class="item-col-qty">{item.quantity || 1}{item.unit ? ` ${item.unit}` : ''}</span>
                                <span class="item-col-price">{(item.unit_price || 0).toFixed(2)}</span>
                                <span class="item-col-total">{(item.total_price || 0).toFixed(2)}</span>
                            </div>
                        {/each}
                    </div>
                </div>
            {/if}
        {:else if activeTab === 'purchase-orders'}
            <!-- Purchase Orders Tab Content -->
            <div class="po-tab-content">
                {#if loadingPOs}
                    <div class="po-loading">
                        <div class="skeleton line"></div>
                        <div class="skeleton line"></div>
                        <div class="skeleton line short"></div>
                    </div>
                {:else if poError}
                    <p class="error">{poError}</p>
                {:else if purchaseOrders.length === 0}
                    <div class="po-empty">
                        <p class="mono">No Purchase Orders</p>
                        <p class="muted">No purchase orders have been created for this opportunity yet.</p>
                        {#if orderId}
                            <p class="info-text">Order ID: {orderId}</p>
                        {/if}
                    </div>
                {:else}
                    <div class="po-list">
                        <p class="mono">Purchase Orders ({purchaseOrders.length})</p>
                        {#each purchaseOrders as po}
                            <div class="po-card">
                                <div class="po-header">
                                    <div class="po-number">
                                        <span class="mono">{po.po_number}</span>
                                        <span class="po-status" data-status={po.status}>{po.status}</span>
                                    </div>
                                    <div class="po-amount">
                                        {po.total_bhd.toFixed(3)} BHD
                                    </div>
                                </div>
                                <div class="po-details">
                                    <div class="po-detail-row">
                                        <span class="label">Supplier:</span>
                                        <span class="value">{po.supplier_name || 'Unknown'}</span>
                                    </div>
                                    <div class="po-detail-row">
                                        <span class="label">PO Date:</span>
                                        <span class="value">{new Date(po.po_date).toLocaleDateString()}</span>
                                    </div>
                                    <div class="po-detail-row">
                                        <span class="label">Expected Delivery:</span>
                                        <span class="value">{new Date(po.expected_delivery).toLocaleDateString()}</span>
                                    </div>
                                </div>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        {/if}

        <div class="status-actions">
            {#if allowed.length === 0}
                <p class="mono">Status locked.</p>
            {:else}
                {#each allowed as next}
                    <button class="status-btn" on:click={() => requestStatusChange(next)}>
                        <span class="dot" style={`background:${statusPalette[next]}`}></span>
                        {next}
                    </button>
                {/each}
            {/if}
        </div>

        <div class="timeline">
            <p class="mono">Timeline</p>
            {#if history.length === 0}
                <p class="muted">No history yet.</p>
            {:else}
                {#each history as event}
                    <div class="event">
                        <div class="event-head">
                            <span class="dot" style={`background:${statusPalette[event.status || 'New'] || 'var(--color-ink)'}`}></span>
                            <strong>{event.status || 'Unknown'}</strong>
                            <span class="mono small">{event.createdAt ? new Date(event.createdAt).toLocaleString() : ''}</span>
                        </div>
                        {#if event.note}
                            <p class="note">{event.note}</p>
                        {/if}
                    </div>
                {/each}
            {/if}
        </div>

        <div class="comments-section">
            <p class="mono">Comments & Notes</p>

            <!-- Comment input -->
            <div class="comment-input">
                <textarea
                    bind:value={newComment}
                    placeholder="Add a comment..."
                    class="comment-textarea"
                    rows="2"
                />
                <button
                    class="comment-btn"
                    on:click={handleAddComment}
                    disabled={!newComment.trim() || addingComment}
                >
                    {addingComment ? 'Posting...' : 'Post'}
                </button>
            </div>

            <!-- Comment list -->
            <div class="comment-list">
                {#if comments.length === 0}
                    <p class="muted">No comments yet.</p>
                {:else}
                    {#each comments as comment}
                        <div class="comment-item">
                            <div class="comment-header">
                                <span class="comment-author">{comment.created_by || 'User'}</span>
                                <span class="comment-time">
                                    {comment.created_at ? new Date(comment.created_at).toLocaleString() : ''}
                                </span>
                            </div>
                            <p class="comment-text">{comment.comment || comment.content}</p>
                        </div>
                    {/each}
                {/if}
            </div>
        </div>
    {:else}
        <p class="muted">Select an opportunity to view details.</p>
    {/if}
</div>

<style>
    .detail {
        background: rgba(255, 255, 255, 0.6);
        border: 1px solid rgba(0, 0, 0, 0.08);
        padding: 1.5rem;
        min-height: 320px;
    }

    .header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        gap: 1rem;
        border-bottom: 2px solid var(--color-ink);
        padding-bottom: 1rem;
        margin-bottom: 1rem;
    }

    .mono {
        font-family: var(--font-mono, 'Courier New', monospace);
        text-transform: uppercase;
        letter-spacing: 2px;
        font-size: 0.7rem;
        margin: 0 0 0.25rem;
        color: var(--color-ink-light);
    }

    h2 {
        margin: 0;
        font-weight: 500;
        letter-spacing: -0.5px;
        color: var(--color-ink);
    }

    .subtitle {
        margin: 0.2rem 0 0;
        color: var(--color-ink-light);
        font-family: var(--font-serif, Georgia, serif);
    }

    .value {
        font-family: var(--font-mono, 'Courier New', monospace);
        background: var(--color-ink);
        color: var(--color-paper);
        padding: 0.5rem 0.75rem;
        letter-spacing: 1px;
    }

    .description {
        margin-bottom: 1.25rem;
        padding: 1rem;
        background: rgba(0, 0, 0, 0.02);
        border: 1px solid rgba(0, 0, 0, 0.06);
        border-radius: 4px;
    }

    .description-content {
        white-space: pre-wrap;
        font-family: var(--font-sans, system-ui, sans-serif);
        font-size: 0.9rem;
        line-height: 1.6;
        color: var(--color-ink);
        max-height: 300px;
        overflow-y: auto;
    }

    .status-actions {
        display: flex;
        gap: 0.5rem;
        margin-bottom: 1.25rem;
        flex-wrap: wrap;
    }

    .status-btn {
        background: rgba(0, 0, 0, 0.04);
        border: 1px solid rgba(0, 0, 0, 0.1);
        padding: 0.45rem 0.75rem;
        display: inline-flex;
        align-items: center;
        gap: 0.4rem;
        cursor: pointer;
        font-family: var(--font-mono, 'Courier New', monospace);
        letter-spacing: 1px;
        text-transform: uppercase;
    }

    .status-btn:hover {
        background: rgba(0, 0, 0, 0.08);
    }

    .dot {
        width: 10px;
        height: 10px;
        border-radius: 50%;
        display: inline-block;
        border: 1px solid rgba(0, 0, 0, 0.08);
    }

    .dot.small { width: 8px; height: 8px; }

    .timeline, .captures {
        margin-bottom: 1.25rem;
    }

    .timeline .event {
        padding: 0.65rem 0;
        border-bottom: 1px solid rgba(0, 0, 0, 0.05);
    }

    .event-head {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        font-family: var(--font-serif, Georgia, serif);
    }

    .event-head .small {
        font-size: 0.7rem;
        color: var(--color-ink-light);
        margin-left: auto;
    }

    .note { margin: 0.25rem 0 0; color: var(--color-ink-light); }

    .captures ul {
        list-style: none;
        padding: 0;
        margin: 0;
        display: grid;
        gap: 0.4rem;
    }

    .captures li {
        display: flex;
        align-items: center;
        gap: 0.4rem;
        font-family: var(--font-serif, Georgia, serif);
    }

    .muted { color: var(--color-ink-light); margin: 0; }
    .error { color: var(--color-danger); }

    /* Line Items Table */
    .line-items-section {
        margin-bottom: 1.25rem;
        padding: 1rem;
        background: rgba(0, 0, 0, 0.02);
        border: 1px solid rgba(0, 0, 0, 0.06);
        border-radius: 4px;
    }

    .items-table {
        margin-top: 0.75rem;
        font-size: 0.85rem;
    }

    .items-header {
        display: grid;
        grid-template-columns: 2fr 0.5fr 0.75fr 0.75fr;
        gap: 0.5rem;
        padding: 0.5rem;
        background: rgba(0, 0, 0, 0.04);
        font-weight: 600;
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        border-radius: 4px 4px 0 0;
    }

    .items-row {
        display: grid;
        grid-template-columns: 2fr 0.5fr 0.75fr 0.75fr;
        gap: 0.5rem;
        padding: 0.5rem;
        border-bottom: 1px solid rgba(0, 0, 0, 0.05);
    }

    .items-row:last-child {
        border-bottom: none;
    }

    .item-col-desc {
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .item-col-qty,
    .item-col-price,
    .item-col-total {
        text-align: right;
        font-family: var(--font-mono, 'Courier New', monospace);
    }

    .part-number {
        display: block;
        font-size: 0.7rem;
        color: var(--color-ink-light);
        font-family: var(--font-mono, 'Courier New', monospace);
    }

    .skeleton {
        background: rgba(0,0,0,0.05);
        border-radius: 4px;
        animation: pulse 1.5s ease-in-out infinite;
    }

    .skeleton.title { height: 22px; width: 70%; }
    .skeleton.line { height: 14px; width: 60%; margin-top: 0.5rem; }
    .skeleton.short { width: 40%; }

    @keyframes pulse {
        0% { opacity: 0.55; }
        50% { opacity: 1; }
        100% { opacity: 0.55; }
    }

    /* Tabs Navigation */
    .tabs-nav {
        display: flex;
        gap: 0.5rem;
        margin-bottom: 1.25rem;
        border-bottom: 2px solid rgba(0, 0, 0, 0.1);
        padding-bottom: 0.5rem;
    }

    .tab-btn {
        background: transparent;
        border: none;
        padding: 0.5rem 1rem;
        font-family: var(--font-mono, 'Courier New', monospace);
        text-transform: uppercase;
        letter-spacing: 1px;
        font-size: 0.75rem;
        cursor: pointer;
        color: var(--color-ink-light);
        border-bottom: 2px solid transparent;
        transition: all 0.2s ease;
        display: flex;
        align-items: center;
        gap: 0.5rem;
        position: relative;
        margin-bottom: -2px;
    }

    .tab-btn:hover {
        color: var(--color-ink);
        background: rgba(0, 0, 0, 0.02);
    }

    .tab-btn.active {
        color: var(--color-ink);
        border-bottom-color: var(--color-ink);
        font-weight: 600;
    }

    .tab-count {
        background: var(--color-ink);
        color: var(--color-paper);
        padding: 0.1rem 0.4rem;
        border-radius: 10px;
        font-size: 0.7rem;
        min-width: 1.2rem;
        text-align: center;
    }

    /* PO Tab Content */
    .po-tab-content {
        margin-top: 1rem;
    }

    .po-loading,
    .po-empty {
        padding: 2rem 1rem;
        text-align: center;
    }

    .po-empty .info-text {
        margin-top: 0.5rem;
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 0.75rem;
        color: var(--color-ink-light);
    }

    .po-list {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .po-card {
        background: rgba(0, 0, 0, 0.02);
        border: 1px solid rgba(0, 0, 0, 0.08);
        border-radius: 4px;
        padding: 1rem;
        transition: all 0.2s ease;
    }

    .po-card:hover {
        background: rgba(0, 0, 0, 0.04);
        border-color: rgba(0, 0, 0, 0.12);
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
    }

    .po-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: 0.75rem;
        padding-bottom: 0.75rem;
        border-bottom: 1px solid rgba(0, 0, 0, 0.06);
    }

    .po-number {
        display: flex;
        flex-direction: column;
        gap: 0.4rem;
    }

    .po-number .mono {
        font-size: 0.9rem;
        font-weight: 600;
        letter-spacing: 0.5px;
    }

    .po-status {
        display: inline-block;
        padding: 0.2rem 0.5rem;
        font-size: 0.7rem;
        font-family: var(--font-mono, 'Courier New', monospace);
        text-transform: uppercase;
        letter-spacing: 0.5px;
        border-radius: 3px;
        background: rgba(0, 0, 0, 0.08);
        color: var(--color-ink);
    }

    .po-status[data-status="Draft"] {
        background: rgba(100, 100, 100, 0.15);
        color: #666;
    }

    .po-status[data-status="Sent"] {
        background: rgba(100, 149, 237, 0.15);
        color: #4682B4;
    }

    .po-status[data-status="Acknowledged"] {
        background: rgba(70, 130, 180, 0.15);
        color: #4682B4;
    }

    .po-status[data-status="PartiallyReceived"] {
        background: rgba(255, 165, 0, 0.15);
        color: #FF8C00;
    }

    .po-status[data-status="FullyReceived"] {
        background: rgba(34, 139, 34, 0.15);
        color: #228B22;
    }

    .po-status[data-status="Closed"] {
        background: rgba(0, 0, 0, 0.1);
        color: #333;
    }

    .po-amount {
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 1rem;
        font-weight: 600;
        color: var(--color-ink);
    }

    .po-details {
        display: flex;
        flex-direction: column;
        gap: 0.4rem;
    }

    .po-detail-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-size: 0.85rem;
    }

    .po-detail-row .label {
        color: var(--color-ink-light);
        font-family: var(--font-mono, 'Courier New', monospace);
        text-transform: uppercase;
        font-size: 0.7rem;
        letter-spacing: 0.5px;
    }

    .po-detail-row .value {
        color: var(--color-ink);
        font-family: var(--font-serif, Georgia, serif);
    }

    /* Header meta (RFQ number + Stage) */
    .header-meta {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        margin-bottom: 0.5rem;
    }

    .rfq-number {
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 0.8rem;
        font-weight: 600;
        color: var(--brand-indigo, #4F46E5);
        background: rgba(79, 70, 229, 0.1);
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
    }

    .stage-select {
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 0.7rem;
        padding: 0.3rem 0.5rem;
        border: 1px solid rgba(0, 0, 0, 0.15);
        border-radius: 4px;
        background: rgba(0, 0, 0, 0.02);
        color: var(--color-ink);
        cursor: pointer;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .stage-select:hover:not(:disabled) {
        border-color: var(--brand-indigo, #4F46E5);
    }

    .stage-select:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    /* Comments Section */
    .comments-section {
        margin-bottom: 1.25rem;
    }

    .comment-input {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        margin-top: 0.75rem;
    }

    .comment-textarea {
        width: 100%;
        padding: 0.5rem;
        border: 1px solid rgba(0, 0, 0, 0.1);
        border-radius: 4px;
        font-family: inherit;
        font-size: 0.85rem;
        resize: vertical;
        min-height: 50px;
    }

    .comment-textarea:focus {
        outline: none;
        border-color: var(--brand-indigo, #4F46E5);
    }

    .comment-btn {
        align-self: flex-start;
        padding: 0.4rem 0.75rem;
        background: var(--color-ink);
        color: var(--color-paper);
        border: none;
        border-radius: 4px;
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 1px;
        cursor: pointer;
    }

    .comment-btn:hover:not(:disabled) {
        background: rgba(0, 0, 0, 0.8);
    }

    .comment-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .comment-list {
        margin-top: 1rem;
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
    }

    .comment-item {
        padding: 0.75rem;
        background: rgba(0, 0, 0, 0.02);
        border: 1px solid rgba(0, 0, 0, 0.06);
        border-radius: 4px;
    }

    .comment-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.4rem;
    }

    .comment-author {
        font-weight: 600;
        font-size: 0.8rem;
        color: var(--color-ink);
    }

    .comment-time {
        font-family: var(--font-mono, 'Courier New', monospace);
        font-size: 0.65rem;
        color: var(--color-ink-light);
    }

    .comment-text {
        margin: 0;
        font-size: 0.85rem;
        line-height: 1.5;
        color: var(--color-ink);
    }
</style>
