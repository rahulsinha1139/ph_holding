<script>
    import { fade } from "svelte/transition";
    import OpportunitiesScreen from "./OpportunitiesScreen.svelte";
    import OffersScreen from "./OffersScreen.svelte";
    import OrdersScreen from "./OrdersScreen.svelte";
    import CostingSheetScreen from "./CostingSheetScreen.svelte";
    // NEW DESIGN SYSTEM
    import Tabs from "../components/ui/Tabs.svelte";
    import ErrorBoundary from "../components/ErrorBoundary.svelte";

    export let params = {};

    let activeTab = "opportunities";
    let lastAppliedRouteTab = "";

    // Text-only tabs - Living Minimalist
    // Sales flow: RFQ → Costing → Offer → Customer Order
    const tabs = [
        { id: "opportunities", label: "RFQs" },
        { id: "costing", label: "Costing" },
        { id: "offers", label: "Offers" },
        { id: "orders", label: "Customer Orders" },  // Clearly distinguished from Purchase Orders
    ];

    // Phase 6: Handle navigation from child components (e.g., CostingSheetScreen → Offers)
    function handleNavigate(event) {
        const { screen, tab } = event.detail || {};
        // Map screen names to tab IDs
        if (screen === 'opportunities' && tab === 'offers') {
            activeTab = 'offers';
        } else if (screen === 'offers') {
            activeTab = 'offers';
        } else if (screen === 'costing') {
            activeTab = 'costing';
        } else if (screen === 'orders') {
            activeTab = 'orders';
        }
    }

    $: {
        const requestedTab = params?.tab;
        const isValidRequestedTab = requestedTab && tabs.some((tab) => tab.id === requestedTab);
        if (isValidRequestedTab && requestedTab !== lastAppliedRouteTab) {
            activeTab = requestedTab;
            lastAppliedRouteTab = requestedTab;
        } else if (!requestedTab) {
            lastAppliedRouteTab = "";
        }
    }
</script>

<ErrorBoundary name="Sales Hub">
    <div class="hub" in:fade={{ duration: 200 }}>
        <!-- Header with Tabs -->
        <header class="header">
            <h1 class="page-title">Opportunities</h1>

            <!-- Tab Bar -->
            <Tabs
                tabs={tabs}
                activeTab={activeTab}
                on:change={(e) => activeTab = e.detail}
            />
        </header>

        <!-- Content -->
        <main class="content">
            {#if activeTab === "opportunities"}
                <OpportunitiesScreen />
            {:else if activeTab === "costing"}
                <CostingSheetScreen on:navigate={handleNavigate} />
            {:else if activeTab === "offers"}
                <OffersScreen embedded={true} on:navigate={handleNavigate} />
            {:else if activeTab === "orders"}
                <OrdersScreen embedded={true} />
            {/if}
        </main>
    </div>
</ErrorBoundary>

<style>
    .hub {
        min-height: 100vh;
        background: var(--bg-base);
        display: flex;
        flex-direction: column;
    }

    .header {
        padding: var(--page-padding);
        padding-bottom: 0;
        border-bottom: 1px solid var(--border);
        background: var(--surface);
    }

    .header h1 {
        margin-bottom: var(--section-spacing);
    }

    .content {
        padding: var(--page-padding);
        flex: 1;
    }
</style>
