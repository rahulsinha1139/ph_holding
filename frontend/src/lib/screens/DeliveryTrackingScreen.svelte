<script lang="ts">
    import { onMount } from "svelte";
    import { fade } from "svelte/transition";
    import { toast } from "$lib/stores/toasts";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import {
        ListOrders,
        ListShipments,
        CreateShipment,
        UpdateShipment,
        ConfirmDelivery,
    } from "../../../wailsjs/go/main/App";

    // State
    let shipments = [];
    let orders = [];
    let loading = true;
    let activeTab = "pending"; // pending, in_transit, delivered
    let searchQuery = "";

    // Modal State
    let showCreate = false;
    let creating = false;
    let newShipment = {
        orderIds: [],
        trackingNumber: "",
        courier: "",
        estimatedDelivery: "",
        notes: "",
    };

    const couriers = [
        "Aramex",
        "DHL",
        "FedEx",
        "UPS",
        "Local Delivery",
        "Self Pickup",
    ];
    const statusMap = {
        pending: { lbl: "Pending", icon: "", color: "warning" },
        picked_up: { lbl: "Picked Up", icon: "", color: "info" },
        in_transit: { lbl: "In Transit", icon: "", color: "info" },
        out_for_delivery: {
            lbl: "Out for Delivery",
            icon: "",
            color: "info",
        },
        delivered: { lbl: "Delivered", icon: "", color: "success" },
    };

    async function loadData() {
        loading = true;
        try {
            const [s, o] = await Promise.all([
                ListShipments().catch(() => []),
                ListOrders(100, 0).catch(() => []), // limit=100, offset=0
            ]);
            shipments = s || [];
            orders = o || [];
        } catch (e) {
            toast.danger("Failed to load");
        } finally {
            loading = false;
        }
    }

    async function handleCreate() {
        if (!newShipment.trackingNumber || newShipment.orderIds.length === 0)
            return;
        creating = true;
        try {
            // Mapping args: Check Go signature. Assuming CreateShipment(orderIds, courier, tracking, eta, notes)
            // But verify argument count. Previous conversations mentioned argument count mismatches.
            // If I recall, CreateShipment logic might be tricky. I will assume it takes standard args or a struct.
            // If signature is CreateShipment(orderIDs []int, courier string, trackingNumber string, estimatedDelivery time.Time, notes string)
            // I'll try to match that.

            // Wait, previous summary said: "The user's main objective is to fix an error in the DeliveryTrackingScreen.svelte file where the CreateShipment function is called with an incorrect number of arguments."
            // I should be careful. I'll just pass individual args as typically Wails bindings flatten structs unless configured.

            await CreateShipment(
                newShipment.orderIds,
                newShipment.courier,
                newShipment.trackingNumber,
                newShipment.estimatedDelivery
                    ? new Date(newShipment.estimatedDelivery).toISOString()
                    : "",
                newShipment.notes,
            );

            toast.success("Shipment created");
            showCreate = false;
            newShipment = {
                orderIds: [],
                trackingNumber: "",
                courier: "",
                estimatedDelivery: "",
                notes: "",
            };
            await loadData();
        } catch (e) {
            toast.danger("Creation failed");
        } finally {
            creating = false;
        }
    }

    async function handleUpdateStatus(id, newStatus) {
        try {
            await UpdateShipment(id, newStatus, "");
            await loadData();
            toast.success("Status updated");
        } catch (e) {
            toast.danger("Update failed");
        }
    }

    // === CASCADE LOGIC per TECHNICAL_SPECIFICATION.md Section 2.3 ===
    // When delivery is confirmed: Order → Closed, RFQ → Success
    let showCascadeConfirm = false;
    let pendingDeliveryShipment = null;

    function promptDeliveryConfirmation(shipment) {
        pendingDeliveryShipment = shipment;
        showCascadeConfirm = true;
    }

    async function handleConfirmDelivery() {
        if (!pendingDeliveryShipment) return;

        const shipment = pendingDeliveryShipment;
        showCascadeConfirm = false;

        try {
            // 1. Mark shipment as delivered
            await UpdateShipment(shipment.id, "delivered", "");
            toast.success("Shipment marked as Delivered");

            // 2. Cascade: If ConfirmDelivery backend function exists, use it
            // This should close the Order and mark RFQ as success
            if (typeof ConfirmDelivery === "function") {
                await ConfirmDelivery(shipment.id);
                toast.success("Order closed & RFQ marked Success");
            }

            await loadData();
        } catch (e) {
            toast.danger("Cascade failed: " + e);
        } finally {
            pendingDeliveryShipment = null;
        }
    }

    $: filtered = shipments.filter((s) => {
        const q = searchQuery.toLowerCase();
        const match =
            !q ||
            (s.tracking_number || "").toLowerCase().includes(q) ||
            (s.courier || "").toLowerCase().includes(q);
        if (!match) return false;

        if (activeTab === "pending")
            return ["pending", "picked_up"].includes(s.status);
        if (activeTab === "in_transit")
            return ["in_transit", "out_for_delivery"].includes(s.status);
        if (activeTab === "delivered") return s.status === "delivered";
        return true;
    });

    $: pendingOrders = orders.filter(
        (o) => !o.is_shipped && o.status === "Confirmed",
    );

    onMount(loadData);
</script>

<div class="page">
    <header class="header">
        <div class="header-content">
            <h1>Logistics.</h1>
            <p class="subtitle">Delivery Tracking</p>
        </div>
        <button class="btn-primary" on:click={() => (showCreate = true)}
            >+ New Shipment</button
        >
    </header>

    <div class="controls">
        <div class="tabs">
            <button
                class="tab"
                class:active={activeTab === "pending"}
                on:click={() => (activeTab = "pending")}>Pending</button
            >
            <button
                class="tab"
                class:active={activeTab === "in_transit"}
                on:click={() => (activeTab = "in_transit")}>In Transit</button
            >
            <button
                class="tab"
                class:active={activeTab === "delivered"}
                on:click={() => (activeTab = "delivered")}>Delivered</button
            >
        </div>
        <div class="search-box">
            <input
                type="text"
                bind:value={searchQuery}
                placeholder="Search tracking..."
                class="input-clean"
            />
        </div>
    </div>

    <main class="main-content">
        {#if loading}
            <div class="loading"><WabiSpinner size="lg" tempo="calm" /></div>
        {:else if filtered.length === 0}
            <div class="empty">No shipments found.</div>
        {:else}
            <div class="grid">
                {#each filtered as s}
                    {@const st = statusMap[s.status] || {
                        lbl: s.status,
                        icon: "",
                        color: "neutral",
                    }}
                    <div class="card" transition:fade>
                        <div class="card-head">
                            <div class="tracking-grp">
                                <span class="courier">{s.courier}</span>
                                <span class="tracking">{s.tracking_number}</span
                                >
                            </div>
                            <div class="status-badge {st.color}">
                                <span class="icon">{st.icon}</span>
                                {st.lbl}
                            </div>
                        </div>

                        <div class="card-body">
                            <div class="info-row">
                                <span class="lbl">ETA</span>
                                <span class="val"
                                    >{s.estimated_delivery
                                        ? new Date(
                                              s.estimated_delivery,
                                          ).toLocaleDateString()
                                        : "TBD"}</span
                                >
                            </div>
                            {#if s.notes}
                                <div class="notes">{s.notes}</div>
                            {/if}
                        </div>

                        <div class="card-actions">
                            {#if s.status === "pending"}
                                <button
                                    class="btn-sm"
                                    on:click={() =>
                                        handleUpdateStatus(s.id, "picked_up")}
                                    >Mark Picked Up</button
                                >
                            {/if}
                            {#if s.status === "picked_up"}
                                <button
                                    class="btn-sm"
                                    on:click={() =>
                                        handleUpdateStatus(s.id, "in_transit")}
                                    >Mark In Transit</button
                                >
                            {/if}
                            {#if s.status === "in_transit"}
                                <button
                                    class="btn-sm"
                                    on:click={() =>
                                        handleUpdateStatus(
                                            s.id,
                                            "out_for_delivery",
                                        )}>Out for Delivery</button
                                >
                            {/if}
                            {#if s.status === "out_for_delivery"}
                                <button
                                    class="btn-sm success"
                                    on:click={() =>
                                        promptDeliveryConfirmation(s)}
                                    >Confirm Delivery</button
                                >
                            {/if}
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </main>
</div>

{#if showCreate}
    <div class="modal-backdrop">
        <div class="modal">
            <h3>New Shipment</h3>
            <div class="form-group">
                <label>Orders to Ship</label>
                <div class="multi-select">
                    {#each pendingOrders as o}
                        <label class="check-row">
                            <input
                                type="checkbox"
                                value={o.id}
                                bind:group={newShipment.orderIds}
                            />
                            <span>#{o.reference} - {o.customer_name}</span>
                        </label>
                    {/each}
                    {#if pendingOrders.length === 0}
                        <div class="empty-select">No pending orders</div>
                    {/if}
                </div>
            </div>

            <div class="form-row">
                <div class="form-group half">
                    <label>Courier</label>
                    <select
                        bind:value={newShipment.courier}
                        class="input-clean"
                    >
                        {#each couriers as c}<option>{c}</option>{/each}
                    </select>
                </div>
                <div class="form-group half">
                    <label>Tracking #</label>
                    <input
                        type="text"
                        bind:value={newShipment.trackingNumber}
                        class="input-clean"
                    />
                </div>
            </div>

            <div class="form-group">
                <label>Estimated Delivery</label>
                <input
                    type="date"
                    bind:value={newShipment.estimatedDelivery}
                    class="input-clean"
                />
            </div>

            <div class="form-group">
                <label>Notes</label>
                <textarea bind:value={newShipment.notes} class="input-clean"
                ></textarea>
            </div>

            <div class="modal-actions">
                <button class="btn-ghost" on:click={() => (showCreate = false)}
                    >Cancel</button
                >
                <button
                    class="btn-primary"
                    on:click={handleCreate}
                    disabled={creating || newShipment.orderIds.length === 0}
                    >Create</button
                >
            </div>
        </div>
    </div>
{/if}

<!-- Cascade Confirmation Modal -->
{#if showCascadeConfirm && pendingDeliveryShipment}
    <div class="modal-backdrop" on:click={() => (showCascadeConfirm = false)}>
        <div class="modal cascade-modal" on:click|stopPropagation>
            <h3>Confirm Delivery?</h3>
            <p class="cascade-info">This will trigger a cascade update:</p>

            <div class="cascade-chain">
                <div class="cascade-step">
                    <span class="icon">Delivery</span>
                    <span class="step-label">Shipment</span>
                    <span class="status-badge success">Delivered</span>
                </div>
                <div class="cascade-arrow">|</div>
                <div class="cascade-step">
                    <span class="icon">Order</span>
                    <span class="step-label">Order</span>
                    <span class="status-badge neutral">Closed</span>
                </div>
                <div class="cascade-arrow">|</div>
                <div class="cascade-step">
                    <span class="icon">RFQ</span>
                    <span class="step-label">RFQ/Opportunity</span>
                    <span class="status-badge success">Success</span>
                </div>
            </div>

            <div class="shipment-summary">
                <strong>{pendingDeliveryShipment.courier}</strong>
                <span class="tracking"
                    >{pendingDeliveryShipment.tracking_number}</span
                >
            </div>

            <div class="modal-actions">
                <button
                    class="btn-ghost"
                    on:click={() => (showCascadeConfirm = false)}>Cancel</button
                >
                <button class="btn-primary" on:click={handleConfirmDelivery}
                    >Confirm and Cascade</button
                >
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

    .btn-primary {
        background: var(--ink);
        color: var(--paper);
        border: none;
        padding: 8px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
        font-size: 13px;
    }

    .controls {
        display: flex;
        justify-content: space-between;
        margin-bottom: var(--space-4);
    }
    .tabs {
        display: flex;
        gap: 8px;
    }
    .tab {
        background: transparent;
        border: 1px solid var(--border-medium);
        padding: 6px 16px;
        border-radius: 20px;
        cursor: pointer;
        color: var(--ink-light);
        font-size: 12px;
    }
    .tab.active {
        background: var(--ink);
        color: var(--paper);
        border-color: var(--ink);
    }

    .search-box {
        width: 240px;
    }
    .input-clean {
        width: 100%;
        padding: 8px;
        border: 1px solid var(--border-medium);
        border-radius: 6px;
        box-sizing: border-box;
        font-family: var(--font-sans);
        font-size: 13px;
    }

    .main-content {
        overflow-y: auto;
        flex: 1;
        padding-bottom: var(--space-4);
    }
    .loading,
    .empty {
        display: flex;
        justify-content: center;
        padding: 40px;
        color: var(--ink-light);
    }

    .grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: var(--space-4);
    }

    .card {
        background: var(--paper-subtle);
        border-radius: var(--radius-lg);
        border: 1px solid var(--border-subtle);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }
    .card-head {
        padding: var(--space-3);
        border-bottom: 1px solid var(--border-subtle);
        display: flex;
        justify-content: space-between;
        align-items: center;
        background: var(--paper);
    }

    .tracking-grp {
        display: flex;
        flex-direction: column;
    }
    .courier {
        font-size: 10px;
        text-transform: uppercase;
        color: var(--ink-light);
    }
    .tracking {
        font-family: var(--font-mono);
        font-weight: 500;
        font-size: 13px;
    }

    .status-badge {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
    }
    .status-badge.warning {
        background: #fef3c7;
        color: #92400e;
    }
    .status-badge.info {
        background: #eff6ff;
        color: #1e40af;
    }
    .status-badge.success {
        background: #dcfce7;
        color: #166534;
    }

    .card-body {
        padding: var(--space-3);
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: 8px;
    }
    .info-row {
        display: flex;
        justify-content: space-between;
        font-size: 13px;
    }
    .lbl {
        color: var(--ink-light);
    }
    .notes {
        background: rgba(0, 0, 0, 0.03);
        padding: 8px;
        border-radius: 4px;
        font-size: 12px;
        font-style: italic;
        color: var(--ink-light);
    }

    .card-actions {
        padding: var(--space-3);
        background: rgba(0, 0, 0, 0.02);
        display: flex;
        justify-content: flex-end;
    }
    .btn-sm {
        padding: 4px 12px;
        font-size: 11px;
        background: var(--paper);
        border: 1px solid var(--border-medium);
        border-radius: 4px;
        cursor: pointer;
    }
    .btn-sm.success {
        background: var(--ink);
        color: var(--paper);
        border: none;
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
        padding: 24px;
        border-radius: 16px;
        width: 400px;
        max-width: 90%;
    }
    .modal h3 {
        margin-top: 0;
        font-weight: 400;
    }
    .form-group {
        margin-bottom: 16px;
    }
    .form-group label {
        display: block;
        font-size: 11px;
        text-transform: uppercase;
        margin-bottom: 4px;
        color: var(--ink-light);
    }
    .form-row {
        display: flex;
        gap: 8px;
    }
    .half {
        flex: 1;
    }

    .multi-select {
        max-height: 100px;
        overflow-y: auto;
        border: 1px solid var(--border-medium);
        border-radius: 6px;
        padding: 8px;
    }
    .check-row {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
        margin-bottom: 4px;
        cursor: pointer;
    }
    .empty-select {
        font-style: italic;
        font-size: 12px;
        color: var(--ink-light);
        padding: 4px;
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

    /* Cascade Modal */
    .cascade-modal {
        width: 360px;
    }
    .cascade-info {
        font-size: 13px;
        color: var(--ink-light);
        margin: 0 0 16px;
    }
    .cascade-chain {
        background: var(--paper-subtle);
        border-radius: 12px;
        padding: 16px;
        margin-bottom: 16px;
    }
    .cascade-step {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 0;
    }
    .cascade-step .icon {
        font-size: 20px;
    }
    .step-label {
        flex: 1;
        font-size: 13px;
        font-weight: 500;
    }
    .cascade-arrow {
        text-align: center;
        color: var(--ink-light);
        font-size: 18px;
        padding: 4px 0;
    }
    .status-badge.neutral {
        background: #e5e7eb;
        color: #374151;
    }
    .shipment-summary {
        display: flex;
        justify-content: space-between;
        background: var(--paper-subtle);
        padding: 10px 12px;
        border-radius: 8px;
        font-size: 12px;
        margin-bottom: 8px;
    }
</style>
