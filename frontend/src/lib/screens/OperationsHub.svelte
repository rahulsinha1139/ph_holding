<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import ModuleLayout from '$lib/components/layout/ModuleLayout.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import ErrorBoundary from '$lib/components/ErrorBoundary.svelte';
  import PurchaseOrdersScreen from './PurchaseOrdersScreen.svelte';
  // GRNScreen deprecated per user request - not needed
  import SupplierInvoicesScreen from './SupplierInvoicesScreen.svelte';
  import DeliveryNotesScreen from './DeliveryNotesScreen.svelte';
  import type { Tab } from '$lib/types/components';
  import { GetPurchaseOrders, GetSupplierInvoices, GetDeliveryNotes } from '../../../wailsjs/go/main/App';
  import { toast } from '$lib/stores/toasts';
  import { pendingDNCreate } from '$lib/stores/navigation';

  // Props
  export let embedded = false;

  // Tab state - GRNs removed per user request (deprecated)
  // Operations = Supplier-side: POs to suppliers, supplier invoices, delivery notes to customers
  const tabs: Tab[] = [
    { id: 'pos', label: 'Supplier POs', count: 0 },              // POs sent TO suppliers (clearly not customer orders)
    { id: 'supplier-invoices', label: 'Supplier Invoices', count: 0 },  // Invoices FROM suppliers
    { id: 'delivery-notes', label: 'Delivery Notes', count: 0 },        // DNs TO customers (links to customer invoices)
  ];

  let activeTab = 'pos';
  let loading = true;

  // Load counts for each tab (GRNs removed)
  async function loadCounts() {
    loading = true;
    try {
      const [pos, invoices, deliveryNotes] = await Promise.all([
        GetPurchaseOrders().catch(() => []),
        GetSupplierInvoices().catch(() => []),
        GetDeliveryNotes().catch(() => []),
      ]);

      // Update tab counts
      tabs[0].count = pos?.length || 0;
      tabs[1].count = invoices?.length || 0;
      tabs[2].count = deliveryNotes?.length || 0;
    } catch (err) {
      console.error('Failed to load counts:', err);
      toast.warning('Failed to load tab counts');
    } finally {
      loading = false;
    }
  }

  function handleTabChange(e: CustomEvent<string>) {
    activeTab = e.detail;
  }

  // Header actions based on active tab - switch tabs and dispatch modal events
  async function handleNewPO() {
    activeTab = 'pos';
    await tick();
    // Delay ensures child component has mounted and registered listeners
    setTimeout(() => {
      window.dispatchEvent(new CustomEvent('openCreatePO'));
    }, 200);
  }

  // GRN deprecated - function removed

  async function handleNewInvoice() {
    activeTab = 'supplier-invoices';
    await tick();
    // Delay ensures child component has mounted and registered listeners
    setTimeout(() => {
      window.dispatchEvent(new CustomEvent('openCreateSupplierInvoice'));
    }, 200);
  }

  async function handleNewDN() {
    activeTab = 'delivery-notes';
    await tick();
    // Delay ensures child component has mounted and registered listeners
    setTimeout(() => {
      window.dispatchEvent(new CustomEvent('openCreateDN'));
    }, 200);
  }

  // Cross-screen navigation handlers (GRN removed)
  function handleNavigateToScreen(e) {
    const { tab } = e.detail;
    if (tab) activeTab = tab;
  }

  onMount(() => {
    window.addEventListener('navigateToScreen', handleNavigateToScreen);
    loadCounts();

    // If there's a pending DN creation, switch to delivery-notes tab
    if ($pendingDNCreate) {
      activeTab = 'delivery-notes';
    }
  });

  onDestroy(() => {
    window.removeEventListener('navigateToScreen', handleNavigateToScreen);
  });
</script>

<div data-embedded={embedded ? 'true' : undefined}>
  <ErrorBoundary name="Operations Hub">
    <ModuleLayout title="Operations Hub" {tabs} {activeTab} on:tabChange={handleTabChange}>
      <svelte:fragment slot="header-actions">
        {#if activeTab === 'pos'}
          <Button variant="primary" on:click={handleNewPO}>
            + New Purchase Order
          </Button>
        {:else if activeTab === 'supplier-invoices'}
          <Button variant="primary" on:click={handleNewInvoice}>
            + New Supplier Invoice
          </Button>
        {:else if activeTab === 'delivery-notes'}
          <Button variant="primary" on:click={handleNewDN}>
            + New Delivery Note
          </Button>
        {/if}
      </svelte:fragment>

      {#if activeTab === 'pos'}
        <PurchaseOrdersScreen embedded={true} />
      {:else if activeTab === 'supplier-invoices'}
        <SupplierInvoicesScreen embedded={true} />
      {:else if activeTab === 'delivery-notes'}
        <DeliveryNotesScreen embedded={true} />
      {/if}
    </ModuleLayout>
  </ErrorBoundary>
</div>

<style>
  /* All styling now handled by Button component and ModuleLayout */
</style>
