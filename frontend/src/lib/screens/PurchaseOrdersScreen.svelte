<script lang="ts">
  /**
   * PurchaseOrdersScreen - Production-Ready Purchase Orders Management
   *
   * Features:
   * - View all purchase orders with filtering by status
   * - Create new POs from orders or manually
   * - Multi-currency support (EUR, USD, BHD)
   * - Status tracking: Draft → Sent → Acknowledged → PartiallyReceived → FullyReceived → Closed
   * - Link to parent orders and suppliers
   * - GRN creation from received POs
   * - Payment tracking with due dates
   *
   * Design System: Wabi-Sabi minimalism × Bloomberg data density
   */

  import { onMount, onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports
  import {
    GetPurchaseOrders,
    GetPurchaseOrderByID,
    CreatePurchaseOrder,
    UpdatePurchaseOrder,
    UpdatePOStatus,
    DeletePurchaseOrder,
    GeneratePurchaseOrderPDF,
    ListOrders,
    ListSuppliers,
  } from '../../../wailsjs/go/main/App';

  // Design system components
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
  import WabiModal from '$lib/components/ui/WabiModal.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import { toast } from '$lib/stores/toasts';
  import { escapeHtml } from '$lib/utils/escapeHtml';

  // Props
  export let embedded = false;

  // Types
  type POStatus = 'all' | 'Draft' | 'Sent' | 'Acknowledged' | 'PartiallyReceived' | 'FullyReceived' | 'Closed' | 'Cancelled';

  interface PODisplay {
    id: string;
    po_number: string;
    order_id?: string;
    rfq_id?: string;
    supplier_id: string;
    supplier_name?: string;
    po_date: string;
    expected_delivery: string;
    currency: string;
    exchange_rate: number;
    subtotal_foreign: number;
    subtotal_bhd: number;
    vat_amount: number;
    total_foreign: number;
    total_bhd: number;
    payment_terms: string;
    payment_due_date: string;
    status: string;
    items?: any[];
    created_at?: string;
    updated_at?: string;
  }

  // State
  let purchaseOrders: PODisplay[] = [];
  let filteredPOs: PODisplay[] = [];
  let loading = true;
  let selectedStatus: POStatus = 'all';

  // Modal state
  let showCreateModal = false;
  let showEditModal = false;
  let showViewModal = false;
  let editingPO: PODisplay | null = null;
  let viewingPO: PODisplay | null = null;

  // Reference data
  let orders: any[] = [];
  let suppliers: any[] = [];

  // Form state
  let formData = {
    order_id: '',
    supplier_id: '',
    po_date: new Date().toISOString().split('T')[0],
    expected_delivery: new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0], // +14 days
    currency: 'BHD',
    exchange_rate: 1.0,
    payment_terms: 'Net 30',
    items: [] as { product_id: string; description: string; quantity: number; unit_price_foreign: number; }[]
  };
  let formLoading = false;
  let statusLoading = false;
  let pdfLoading = false;

  // Currency options
  const currencies = [
    { value: 'BHD', label: 'BHD - Bahraini Dinar' },
    { value: 'USD', label: 'USD - US Dollar' },
    { value: 'EUR', label: 'EUR - Euro' },
    { value: 'GBP', label: 'GBP - British Pound' },
    { value: 'AED', label: 'AED - UAE Dirham' },
    { value: 'SAR', label: 'SAR - Saudi Riyal' }
  ];

  // Payment terms options
  const paymentTerms = [
    'Net 15',
    'Net 30',
    'Net 45',
    'Net 60',
    'Net 90',
    'Advance Payment',
    'COD',
    'Due on Receipt'
  ];

  // DataTable columns configuration
  const columns = [
    {
      key: 'po_number',
      label: 'PO #',
      sortable: true,
      width: '120px'
    },
    {
      key: 'supplier_name',
      label: 'Supplier',
      sortable: true,
      render: (row) => escapeHtml(row.supplier_name)
    },
    {
      key: 'po_date',
      label: 'PO Date',
      type: 'date' as const,
      sortable: true,
      width: '120px'
    },
    {
      key: 'currency',
      label: 'Currency',
      sortable: true,
      width: '90px',
      render: (row: PODisplay) => {
        return `<span style="font-family: var(--font-mono); font-size: 12px;">${row.currency}</span>`;
      }
    },
    {
      key: 'total_foreign',
      label: 'Total (Foreign)',
      type: 'currency' as const,
      align: 'right' as const,
      sortable: true,
      width: '140px',
      render: (row: PODisplay) => {
        return `<span style="font-family: var(--font-mono); font-weight: 500;">${row.total_foreign.toFixed(3)} ${row.currency}</span>`;
      }
    },
    {
      key: 'total_bhd',
      label: 'Total (BHD)',
      type: 'currency' as const,
      align: 'right' as const,
      sortable: true,
      width: '130px'
    },
    {
      key: 'status',
      label: 'Status',
      type: 'status' as const,
      sortable: true,
      width: '140px'
    },
    {
      key: 'actions',
      label: 'Actions',
      type: 'actions' as const,
      width: '280px',
      render: (row: PODisplay) => {
        return `
          <div style="display: flex; gap: 6px; justify-content: flex-end; flex-wrap: wrap;">
            <button
              class="action-btn action-btn-view"
              data-action="view"
              data-id="${row.id}"
              aria-label="View PO details"
            >
              View
            </button>
            <button
              class="action-btn action-btn-edit"
              data-action="edit"
              data-id="${row.id}"
              aria-label="Edit PO"
            >
              Edit
            </button>
            <button
              class="action-btn action-btn-pdf"
              data-action="pdf"
              data-id="${row.id}"
              aria-label="Generate PDF"
            >
              PDF
            </button>
            ${['Sent', 'Acknowledged', 'PartiallyReceived'].includes(row.status) ? `
              <button
                class="action-btn action-btn-grn"
                data-action="grn"
                data-id="${row.id}"
                aria-label="Create GRN"
              >
                GRN
              </button>
            ` : ''}
          </div>
        `;
      }
    }
  ];

  // Status filter tabs
  const statusTabs: { value: POStatus; label: string; count: number }[] = [
    { value: 'all', label: 'All POs', count: 0 },
    { value: 'Draft', label: 'Draft', count: 0 },
    { value: 'Sent', label: 'Sent', count: 0 },
    { value: 'Acknowledged', label: 'Acknowledged', count: 0 },
    { value: 'PartiallyReceived', label: 'Partially Received', count: 0 },
    { value: 'FullyReceived', label: 'Fully Received', count: 0 },
    { value: 'Closed', label: 'Closed', count: 0 }
  ];

  // Computed: Update tab counts
  $: {
    statusTabs[0].count = purchaseOrders.length;
    statusTabs[1].count = purchaseOrders.filter(p => p.status === 'Draft').length;
    statusTabs[2].count = purchaseOrders.filter(p => p.status === 'Sent').length;
    statusTabs[3].count = purchaseOrders.filter(p => p.status === 'Acknowledged').length;
    statusTabs[4].count = purchaseOrders.filter(p => p.status === 'PartiallyReceived').length;
    statusTabs[5].count = purchaseOrders.filter(p => p.status === 'FullyReceived').length;
    statusTabs[6].count = purchaseOrders.filter(p => p.status === 'Closed').length;
  }

  // Computed: Filter POs by selected status
  $: {
    if (selectedStatus === 'all') {
      filteredPOs = purchaseOrders;
    } else {
      filteredPOs = purchaseOrders.filter(p => p.status === selectedStatus);
    }
  }

  // Load purchase orders and reference data
  async function loadPurchaseOrders() {
    loading = true;
    try {
      const [posData, ordersData, suppliersData] = await Promise.all([
        GetPurchaseOrders(),
        ListOrders(1000, 0),
        ListSuppliers(1000, 0)
      ]);

      suppliers = suppliersData || [];
      orders = ordersData || [];
      purchaseOrders = (posData || []).map(enrichPO);

      console.log(`Loaded ${purchaseOrders.length} POs, ${orders.length} orders, ${suppliers.length} suppliers`);
    } catch (err) {
      console.error('Failed to load purchase orders:', err);
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to load purchase orders: ${errorMsg}`);
      purchaseOrders = [];
    } finally {
      loading = false;
    }
  }

  // Enrich PO with supplier name
  function enrichPO(po: any): PODisplay {
    // If PO already has supplier_name from backend, use it
    if (po.supplier_name) {
      return po;
    }

    // Otherwise, lookup from suppliers array
    const supplier = suppliers.find(s => s.id === po.supplier_id);
    return {
      ...po,
      supplier_name: supplier?.supplier_name || 'Unknown Supplier'
    };
  }

  // Open create modal
  function openCreateModal() {
    formData = {
      order_id: '',
      supplier_id: '',
      po_date: new Date().toISOString().split('T')[0],
      expected_delivery: new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
      currency: 'BHD',
      exchange_rate: 1.0,
      payment_terms: 'Net 30',
      items: []
    };
    showCreateModal = true;
  }

  // Open edit modal
  async function openEditModal(poId: string) {
    try {
      const po = await GetPurchaseOrderByID(poId);
      editingPO = enrichPO(po);

      formData = {
        order_id: po.order_id || '',
        supplier_id: po.supplier_id || '',
        po_date: po.po_date ? new Date(po.po_date as any).toISOString().split('T')[0] : new Date().toISOString().split('T')[0],
        expected_delivery: po.expected_delivery ? new Date(po.expected_delivery as any).toISOString().split('T')[0] : '',
        currency: po.currency || 'BHD',
        exchange_rate: po.exchange_rate || 1.0,
        payment_terms: po.payment_terms || 'Net 30',
        items: (po.items || []).map((item: any) => ({
          product_id: item.product_id || '',
          description: item.description || '',
          quantity: item.quantity || 0,
          unit_price_foreign: item.unit_price_foreign || 0
        }))
      };

      showEditModal = true;
    } catch (err) {
      console.error('Failed to load PO for editing:', err);
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to load purchase order: ${errorMsg}`);
    }
  }

  // Open view modal
  async function openViewModal(poId: string) {
    try {
      const po = await GetPurchaseOrderByID(poId);
      viewingPO = enrichPO(po);
      showViewModal = true;
    } catch (err) {
      console.error('Failed to load PO for viewing:', err);
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to load purchase order: ${errorMsg}`);
    }
  }

  // Handle create PO
  async function handleCreatePO() {
    if (!formData.supplier_id || !formData.po_date) {
      toast.warning('Please fill all required fields');
      return;
    }

    // Validate dates
    const poDate = new Date(formData.po_date);
    const expectedDelivery = new Date(formData.expected_delivery);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    if (poDate > today) {
      toast.warning('PO date cannot be in the future');
      return;
    }

    if (expectedDelivery <= poDate) {
      toast.warning('Expected delivery must be after PO date');
      return;
    }

    // Validate at least one item
    if (formData.items.length === 0) {
      toast.warning('Please add at least one line item');
      return;
    }

    // Validate all items
    for (let i = 0; i < formData.items.length; i++) {
      const item = formData.items[i];
      if (item.quantity < 1) {
        toast.warning(`Item ${i + 1}: Quantity must be at least 1`);
        return;
      }
      if (item.unit_price_foreign <= 0) {
        toast.warning(`Item ${i + 1}: Unit price must be greater than 0`);
        return;
      }
      if (!item.description || !item.description.trim()) {
        toast.warning(`Item ${i + 1}: Description is required`);
        return;
      }
    }

    // Validate exchange rate
    if (formData.exchange_rate <= 0) {
      toast.warning('Exchange rate must be greater than 0');
      return;
    }

    formLoading = true;
    try {
      const poData: any = {
        order_id: formData.order_id || undefined,
        supplier_id: formData.supplier_id,
        po_date: new Date(formData.po_date).toISOString(),
        expected_delivery: new Date(formData.expected_delivery).toISOString(),
        currency: formData.currency,
        exchange_rate: formData.exchange_rate,
        payment_terms: formData.payment_terms,
        status: 'Draft',
        items: formData.items.map(item => ({
          ...item,
          total_foreign: item.quantity * item.unit_price_foreign
        }))
      };

      // Calculate totals
      const subtotal = poData.items.reduce((sum: number, item: any) => sum + item.total_foreign, 0);
      poData.subtotal_foreign = subtotal;
      poData.subtotal_bhd = subtotal * formData.exchange_rate;
      poData.vat_amount = subtotal * 0.10; // 10% VAT
      poData.total_foreign = subtotal + poData.vat_amount;
      poData.total_bhd = poData.total_foreign * formData.exchange_rate;

      await CreatePurchaseOrder(poData);
      toast.success('Purchase order created successfully');
      showCreateModal = false;
      await loadPurchaseOrders();
    } catch (err) {
      console.error('Failed to create PO:', err);
      toast.danger('Failed to create purchase order: ' + (err as Error).message);
    } finally {
      formLoading = false;
    }
  }

  // Handle edit PO
  async function handleEditPO() {
    if (!editingPO) return;

    // Same validations as create
    if (!formData.supplier_id || !formData.po_date) {
      toast.warning('Please fill all required fields');
      return;
    }

    const poDate = new Date(formData.po_date);
    const expectedDelivery = new Date(formData.expected_delivery);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    if (poDate > today) {
      toast.warning('PO date cannot be in the future');
      return;
    }

    if (expectedDelivery <= poDate) {
      toast.warning('Expected delivery must be after PO date');
      return;
    }

    if (formData.items.length === 0) {
      toast.warning('Please add at least one line item');
      return;
    }

    for (let i = 0; i < formData.items.length; i++) {
      const item = formData.items[i];
      if (item.quantity < 1) {
        toast.warning(`Item ${i + 1}: Quantity must be at least 1`);
        return;
      }
      if (item.unit_price_foreign <= 0) {
        toast.warning(`Item ${i + 1}: Unit price must be greater than 0`);
        return;
      }
      if (!item.description || !item.description.trim()) {
        toast.warning(`Item ${i + 1}: Description is required`);
        return;
      }
    }

    if (formData.exchange_rate <= 0) {
      toast.warning('Exchange rate must be greater than 0');
      return;
    }

    formLoading = true;
    try {
      const poData: any = {
        ...editingPO,
        order_id: formData.order_id || undefined,
        supplier_id: formData.supplier_id,
        po_date: new Date(formData.po_date).toISOString(),
        expected_delivery: new Date(formData.expected_delivery).toISOString(),
        currency: formData.currency,
        exchange_rate: formData.exchange_rate,
        payment_terms: formData.payment_terms,
        items: formData.items.map(item => ({
          ...item,
          total_foreign: item.quantity * item.unit_price_foreign
        }))
      };

      // Recalculate totals
      const subtotal = poData.items.reduce((sum: number, item: any) => sum + item.total_foreign, 0);
      poData.subtotal_foreign = subtotal;
      poData.subtotal_bhd = subtotal * formData.exchange_rate;
      poData.vat_amount = subtotal * 0.10;
      poData.total_foreign = subtotal + poData.vat_amount;
      poData.total_bhd = poData.total_foreign * formData.exchange_rate;

      await UpdatePurchaseOrder(poData);
      toast.success('Purchase order updated successfully');
      showEditModal = false;
      editingPO = null;
      await loadPurchaseOrders();
    } catch (err) {
      console.error('Failed to update PO:', err);
      toast.danger('Failed to update purchase order: ' + (err as Error).message);
    } finally {
      formLoading = false;
    }
  }

  // Handle status change
  async function handleStatusChange(poId: string, newStatus: string) {
    if (statusLoading) return;

    // Confirm destructive action
    if (newStatus === 'Closed' && !confirm('Close this PO? This cannot be undone.')) {
      return;
    }

    statusLoading = true;
    try {
      await UpdatePOStatus(poId, newStatus);
      toast.success(`PO status updated to ${newStatus}`);
      await loadPurchaseOrders();
    } catch (err) {
      console.error('Failed to update PO status:', err);
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to update status: ${errorMsg}`);
    } finally {
      statusLoading = false;
    }
  }

  // Handle create GRN - dispatch event for OperationsHub to switch tabs
  function handleCreateGRN(poId: string) {
    window.dispatchEvent(new CustomEvent('navigateToGRN', { detail: { poId } }));
  }

  // Handle PDF generation
  async function handleGeneratePDF(poId: string) {
    if (pdfLoading) return;

    pdfLoading = true;
    try {
      toast.info('Generating PDF...');
      const filePath = await GeneratePurchaseOrderPDF(poId);

      // Show success with file path
      toast.success(`PDF generated successfully! Saved to: ${filePath}`);

      // Optional: Open the exports folder in file explorer
      // This requires a backend function to open the folder
      console.log(`PO PDF saved: ${filePath}`);
    } catch (err) {
      console.error('Failed to generate PDF:', err);
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to generate PDF: ${errorMsg}`);
    } finally {
      pdfLoading = false;
    }
  }

  // Handle action button clicks (delegated from DataTable)
  function handleRowClick(event: CustomEvent) {
    const target = event.detail.event?.target as HTMLElement;
    if (!target || !target.dataset.action) return;

    const action = target.dataset.action;
    const id = target.dataset.id;

    switch (action) {
      case 'view':
        openViewModal(id!);
        break;
      case 'edit':
        openEditModal(id!);
        break;
      case 'pdf':
        if (!pdfLoading) {
          handleGeneratePDF(id!);
        }
        break;
      case 'grn':
        handleCreateGRN(id!);
        break;
    }
  }

  // Supplier selection handler
  function handleSupplierSelect(e: Event) {
    const select = e.target as HTMLSelectElement;
    formData.supplier_id = select.value;
  }

  // Order selection handler
  function handleOrderSelect(e: Event) {
    const select = e.target as HTMLSelectElement;
    formData.order_id = select.value;
  }

  // Currency change handler - update exchange rate
  function handleCurrencyChange(e: Event) {
    const select = e.target as HTMLSelectElement;
    formData.currency = select.value;

    // Set default exchange rates (these should come from a service in production)
    const rates: Record<string, number> = {
      'BHD': 1.0,
      'USD': 2.65,  // 1 USD = 2.65 BHD
      'EUR': 2.85,  // 1 EUR = 2.85 BHD
      'GBP': 3.35,  // 1 GBP = 3.35 BHD
      'AED': 0.72,  // 1 AED = 0.72 BHD
      'SAR': 0.71   // 1 SAR = 0.71 BHD
    };

    formData.exchange_rate = rates[formData.currency] || 1.0;
  }

  // Add line item
  function addLineItem() {
    formData.items = [...formData.items, {
      product_id: '',
      description: '',
      quantity: 1,
      unit_price_foreign: 0
    }];
  }

  // Remove line item
  function removeLineItem(index: number) {
    formData.items = formData.items.filter((_, i) => i !== index);
  }

  // Computed: Total value
  $: totalForeign = formData.items.reduce((sum, item) => sum + (item.quantity * item.unit_price_foreign), 0);
  $: vatAmount = totalForeign * 0.10;
  $: totalWithVAT = totalForeign + vatAmount;
  $: totalBHD = totalWithVAT * formData.exchange_rate;

  // Event handler for opening create modal from parent hub
  function handleOpenCreatePO() {
    openCreateModal();
  }

  // Update PDF button disabled state when loading changes
  $: {
    if (typeof document !== 'undefined') {
      const pdfButtons = document.querySelectorAll('.action-btn-pdf');
      pdfButtons.forEach((btn) => {
        if (pdfLoading) {
          btn.setAttribute('disabled', 'true');
          (btn as HTMLElement).style.opacity = '0.5';
          (btn as HTMLElement).style.cursor = 'not-allowed';
        } else {
          btn.removeAttribute('disabled');
          (btn as HTMLElement).style.opacity = '1';
          (btn as HTMLElement).style.cursor = 'pointer';
        }
      });
    }
  }

  onMount(() => {
    loadPurchaseOrders();

    // Listen for create PO events from OperationsHub header button
    window.addEventListener('openCreatePO', handleOpenCreatePO);
  });

  onDestroy(() => {
    // Clean up event listener
    window.removeEventListener('openCreatePO', handleOpenCreatePO);
  });
</script>

<PageLayout title="Purchase Orders" subtitle="Procurement Management" {embedded}>
  <svelte:fragment slot="header-actions">
    <Button variant="primary" on:click={openCreateModal}>
      + New PO
    </Button>
  </svelte:fragment>

  <div class="po-container">
    <!-- Status Filter Tabs -->
    <Card padding="sm">
      <div class="status-tabs" role="tablist" aria-label="Filter POs by status">
        {#each statusTabs as tab}
          <button
            class="status-tab"
            class:active={selectedStatus === tab.value}
            role="tab"
            aria-selected={selectedStatus === tab.value}
            on:click={() => selectedStatus = tab.value}
          >
            {tab.label}
            <span class="tab-count">{tab.count}</span>
          </button>
        {/each}
      </div>
    </Card>

    <!-- Purchase Orders DataTable -->
    <Card padding="sm">
      <DataTable
        {columns}
        data={filteredPOs}
        {loading}
        emptyMessage="No purchase orders found"
        onRowClick={(row) => {}}
        on:rowClick={handleRowClick}
        stickyHeader={true}
        maxHeight="calc(100vh - 300px)"
        showBorder={false}
      />
    </Card>

    <!-- Summary Stats -->
    <div class="stats-grid">
      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Total POs</div>
          <div class="stat-value">{purchaseOrders.length}</div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Total Value (BHD)</div>
          <div class="stat-value">
            {purchaseOrders.reduce((sum, p) => sum + p.total_bhd, 0).toFixed(3)}
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Pending Receipts</div>
          <div class="stat-value stat-warning">
            {purchaseOrders.filter(p => ['Sent', 'Acknowledged', 'PartiallyReceived'].includes(p.status)).length}
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Fully Received</div>
          <div class="stat-value stat-success">
            {purchaseOrders.filter(p => p.status === 'FullyReceived').length}
          </div>
        </div>
      </Card>
    </div>
  </div>
</PageLayout>

<!-- Create PO Modal -->
<WabiModal bind:open={showCreateModal} title="Create Purchase Order" size="xl">
  <form on:submit|preventDefault={handleCreatePO} class="po-form">
    <!-- Supplier and Order Selection -->
    <div class="form-row">
      <FormGroup label="Supplier" required>
        <select
          class="select-input"
          bind:value={formData.supplier_id}
          on:change={handleSupplierSelect}
          required
        >
          <option value="">Select supplier...</option>
          {#each suppliers as supplier}
            <option value={supplier.id}>{supplier.supplier_name}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Link to Order (Optional)">
        <select
          class="select-input"
          bind:value={formData.order_id}
          on:change={handleOrderSelect}
        >
          <option value="">No order link...</option>
          {#each orders as order}
            <option value={order.id}>{order.order_number} - {order.customer_name}</option>
          {/each}
        </select>
      </FormGroup>
    </div>

    <!-- Dates -->
    <div class="form-row">
      <FormGroup label="PO Date" required>
        <Input
          type="date"
          bind:value={formData.po_date}
          max={new Date().toISOString().split('T')[0]}
          required
        />
      </FormGroup>

      <FormGroup label="Expected Delivery" required>
        <Input
          type="date"
          bind:value={formData.expected_delivery}
          min={formData.po_date}
          required
        />
      </FormGroup>
    </div>

    <!-- Currency and Payment -->
    <div class="form-row">
      <FormGroup label="Currency" required>
        <select
          class="select-input"
          bind:value={formData.currency}
          on:change={handleCurrencyChange}
          required
        >
          {#each currencies as curr}
            <option value={curr.value}>{curr.label}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Exchange Rate (to BHD)" required>
        <Input
          type="number"
          bind:value={formData.exchange_rate}
          step="0.0001"
          min="0.0001"
          required
        />
        <div class="exchange-hint">1 {formData.currency} = {formData.exchange_rate.toFixed(4)} BHD</div>
      </FormGroup>
    </div>

    <FormGroup label="Payment Terms" required>
      <select
        class="select-input"
        bind:value={formData.payment_terms}
        required
      >
        {#each paymentTerms as term}
          <option value={term}>{term}</option>
        {/each}
      </select>
    </FormGroup>

    <!-- Line Items -->
    <div class="line-items-section">
      <div class="section-header">
        <h4>Line Items</h4>
        <Button variant="secondary" size="sm" on:click={addLineItem}>
          + Add Item
        </Button>
      </div>

      {#if formData.items.length === 0}
        <div class="empty-items">
          No items added yet. Click "Add Item" to start.
        </div>
      {:else}
        <div class="items-list">
          {#each formData.items as item, index}
            <div class="item-row" transition:fade>
              <Input
                label="Description"
                bind:value={item.description}
                placeholder="Product or service description"
              />
              <Input
                label="Quantity"
                type="number"
                bind:value={item.quantity}
                min="1"
                step="1"
                required
              />
              <Input
                label="Unit Price ({formData.currency})"
                type="number"
                bind:value={item.unit_price_foreign}
                min="0.001"
                step="0.001"
                required
              />
              <div class="item-total">
                <div class="label">Total</div>
                <div class="value">{(item.quantity * item.unit_price_foreign).toFixed(3)} {formData.currency}</div>
              </div>
              <button
                type="button"
                class="btn-remove"
                on:click={() => removeLineItem(index)}
                aria-label="Remove item"
              >
                ×
              </button>
            </div>
          {/each}
        </div>

        <!-- Totals Summary -->
        <div class="totals-section">
          <div class="total-row">
            <div class="total-label">Subtotal:</div>
            <div class="total-value">{totalForeign.toFixed(3)} {formData.currency}</div>
          </div>
          <div class="total-row">
            <div class="total-label">VAT (10%):</div>
            <div class="total-value">{vatAmount.toFixed(3)} {formData.currency}</div>
          </div>
          <div class="total-row total-grand">
            <div class="total-label">Total ({formData.currency}):</div>
            <div class="total-value">{totalWithVAT.toFixed(3)} {formData.currency}</div>
          </div>
          {#if formData.currency !== 'BHD'}
            <div class="total-row total-bhd">
              <div class="total-label">Total (BHD):</div>
              <div class="total-value">{totalBHD.toFixed(3)} BHD</div>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showCreateModal = false}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={formLoading}
      on:click={handleCreatePO}
    >
      Create PO
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Edit PO Modal (same as create) -->
<WabiModal bind:open={showEditModal} title="Edit Purchase Order" size="xl">
  <form on:submit|preventDefault={handleEditPO} class="po-form">
    <!-- Same form fields as create modal -->
    <div class="form-row">
      <FormGroup label="Supplier" required>
        <select
          class="select-input"
          bind:value={formData.supplier_id}
          on:change={handleSupplierSelect}
          required
        >
          <option value="">Select supplier...</option>
          {#each suppliers as supplier}
            <option value={supplier.id}>{supplier.supplier_name}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Link to Order (Optional)">
        <select
          class="select-input"
          bind:value={formData.order_id}
          on:change={handleOrderSelect}
        >
          <option value="">No order link...</option>
          {#each orders as order}
            <option value={order.id}>{order.order_number} - {order.customer_name}</option>
          {/each}
        </select>
      </FormGroup>
    </div>

    <div class="form-row">
      <FormGroup label="PO Date" required>
        <Input
          type="date"
          bind:value={formData.po_date}
          required
        />
      </FormGroup>

      <FormGroup label="Expected Delivery" required>
        <Input
          type="date"
          bind:value={formData.expected_delivery}
          required
        />
      </FormGroup>
    </div>

    <div class="form-row">
      <FormGroup label="Currency" required>
        <select
          class="select-input"
          bind:value={formData.currency}
          on:change={handleCurrencyChange}
          required
        >
          {#each currencies as curr}
            <option value={curr.value}>{curr.label}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Exchange Rate (to BHD)" required>
        <Input
          type="number"
          bind:value={formData.exchange_rate}
          step="0.0001"
          min="0.0001"
          required
        />
        <div class="exchange-hint">1 {formData.currency} = {formData.exchange_rate.toFixed(4)} BHD</div>
      </FormGroup>
    </div>

    <FormGroup label="Payment Terms" required>
      <select
        class="select-input"
        bind:value={formData.payment_terms}
        required
      >
        {#each paymentTerms as term}
          <option value={term}>{term}</option>
        {/each}
      </select>
    </FormGroup>

    <div class="line-items-section">
      <div class="section-header">
        <h4>Line Items</h4>
        <Button variant="secondary" size="sm" on:click={addLineItem}>
          + Add Item
        </Button>
      </div>

      {#if formData.items.length === 0}
        <div class="empty-items">
          No items added yet. Click "Add Item" to start.
        </div>
      {:else}
        <div class="items-list">
          {#each formData.items as item, index}
            <div class="item-row" transition:fade>
              <Input
                label="Description"
                bind:value={item.description}
                placeholder="Product or service description"
              />
              <Input
                label="Quantity"
                type="number"
                bind:value={item.quantity}
                min="1"
                step="1"
                required
              />
              <Input
                label="Unit Price ({formData.currency})"
                type="number"
                bind:value={item.unit_price_foreign}
                min="0.001"
                step="0.001"
                required
              />
              <div class="item-total">
                <div class="label">Total</div>
                <div class="value">{(item.quantity * item.unit_price_foreign).toFixed(3)} {formData.currency}</div>
              </div>
              <button
                type="button"
                class="btn-remove"
                on:click={() => removeLineItem(index)}
                aria-label="Remove item"
              >
                ×
              </button>
            </div>
          {/each}
        </div>

        <div class="totals-section">
          <div class="total-row">
            <div class="total-label">Subtotal:</div>
            <div class="total-value">{totalForeign.toFixed(3)} {formData.currency}</div>
          </div>
          <div class="total-row">
            <div class="total-label">VAT (10%):</div>
            <div class="total-value">{vatAmount.toFixed(3)} {formData.currency}</div>
          </div>
          <div class="total-row total-grand">
            <div class="total-label">Total ({formData.currency}):</div>
            <div class="total-value">{totalWithVAT.toFixed(3)} {formData.currency}</div>
          </div>
          {#if formData.currency !== 'BHD'}
            <div class="total-row total-bhd">
              <div class="total-label">Total (BHD):</div>
              <div class="total-value">{totalBHD.toFixed(3)} BHD</div>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showEditModal = false}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={formLoading}
      on:click={handleEditPO}
    >
      Save Changes
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- View PO Modal -->
{#if viewingPO}
  <WabiModal bind:open={showViewModal} title="Purchase Order Details" size="lg">
    <div class="po-details">
      <!-- Header Info -->
      <div class="details-grid">
        <div class="detail-item">
          <div class="detail-label">PO Number</div>
          <div class="detail-value mono">{viewingPO.po_number}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Status</div>
          <StatusBadge status={viewingPO.status} />
        </div>
        <div class="detail-item">
          <div class="detail-label">Supplier</div>
          <div class="detail-value">{viewingPO.supplier_name}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">PO Date</div>
          <div class="detail-value">{new Date(viewingPO.po_date).toLocaleDateString()}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Expected Delivery</div>
          <div class="detail-value">{new Date(viewingPO.expected_delivery).toLocaleDateString()}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Payment Terms</div>
          <div class="detail-value">{viewingPO.payment_terms}</div>
        </div>
      </div>

      <!-- Items Table -->
      {#if viewingPO.items && viewingPO.items.length > 0}
        <div class="items-section">
          <h4>Items</h4>
          <table class="items-table">
            <thead>
              <tr>
                <th>Description</th>
                <th class="right">Quantity</th>
                <th class="right">Unit Price</th>
                <th class="right">Total</th>
              </tr>
            </thead>
            <tbody>
              {#each viewingPO.items as item}
                <tr>
                  <td>{item.description}</td>
                  <td class="right mono">{item.quantity}</td>
                  <td class="right mono">{item.unit_price_foreign.toFixed(3)} {viewingPO.currency}</td>
                  <td class="right mono">{item.total_foreign.toFixed(3)} {viewingPO.currency}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Totals -->
      <div class="view-totals">
        <div class="total-row">
          <div class="label">Subtotal:</div>
          <div class="value mono">{viewingPO.subtotal_foreign.toFixed(3)} {viewingPO.currency}</div>
        </div>
        <div class="total-row">
          <div class="label">VAT:</div>
          <div class="value mono">{viewingPO.vat_amount.toFixed(3)} {viewingPO.currency}</div>
        </div>
        <div class="total-row grand">
          <div class="label">Total ({viewingPO.currency}):</div>
          <div class="value mono">{viewingPO.total_foreign.toFixed(3)} {viewingPO.currency}</div>
        </div>
        {#if viewingPO.currency !== 'BHD'}
          <div class="total-row bhd">
            <div class="label">Total (BHD):</div>
            <div class="value mono">{viewingPO.total_bhd.toFixed(3)} BHD</div>
          </div>
        {/if}
      </div>

      <!-- Status Actions -->
      <div class="status-actions">
        <h4>Update Status</h4>
        <div class="status-buttons">
          {#each ['Draft', 'Sent', 'Acknowledged', 'PartiallyReceived', 'FullyReceived', 'Closed'] as status}
            <Button
              variant={viewingPO.status === status ? 'primary' : 'secondary'}
              size="sm"
              disabled={viewingPO.status === status || statusLoading}
              on:click={() => handleStatusChange(viewingPO.id, status)}
            >
              {status}
            </Button>
          {/each}
        </div>
      </div>
    </div>

    <svelte:fragment slot="footer">
      <Button variant="secondary" on:click={() => openEditModal(viewingPO.id)}>
        Edit PO
      </Button>
      <Button variant="ghost" on:click={() => showViewModal = false}>
        Close
      </Button>
    </svelte:fragment>
  </WabiModal>
{/if}

<style>
  .po-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* Status Tabs */
  .status-tabs {
    display: flex;
    gap: 8px;
    overflow-x: auto;
  }

  .status-tab {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: transparent;
    border: none;
    border-radius: var(--border-radius-sm);
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--transition-fast);
    white-space: nowrap;
  }

  .status-tab:hover {
    background: var(--interactive-hover);
    color: var(--text-primary);
  }

  .status-tab.active {
    background: var(--brand-indigo);
    color: white;
  }

  .tab-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 24px;
    height: 20px;
    padding: 0 6px;
    background: rgba(0, 0, 0, 0.1);
    border-radius: 10px;
    font-size: 12px;
    font-weight: 600;
  }

  .status-tab.active .tab-count {
    background: rgba(255, 255, 255, 0.2);
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
  }

  .stat {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .stat-label {
    font-size: var(--label-size);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .stat-value {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .stat-warning {
    color: #F59E0B;
  }

  .stat-success {
    color: #10B981;
  }

  /* Action Buttons in Table */
  :global(.action-btn) {
    padding: 4px 10px;
    font-size: 12px;
    font-weight: 500;
    border: none;
    border-radius: var(--border-radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  :global(.action-btn-view) {
    background: var(--surface-elevated);
    color: var(--text-primary);
  }

  :global(.action-btn-view:hover) {
    background: var(--interactive-hover);
  }

  :global(.action-btn-edit) {
    background: var(--brand-indigo-tint);
    color: var(--brand-indigo);
  }

  :global(.action-btn-edit:hover) {
    background: var(--brand-indigo);
    color: white;
  }

  :global(.action-btn-pdf) {
    background: rgba(245, 158, 11, 0.1);
    color: #F59E0B;
  }

  :global(.action-btn-pdf:hover:not([disabled])) {
    background: #F59E0B;
    color: white;
  }

  :global(.action-btn-pdf[disabled]) {
    opacity: 0.5;
    cursor: not-allowed;
    pointer-events: none;
  }

  :global(.action-btn-grn) {
    background: rgba(16, 185, 129, 0.1);
    color: #10B981;
  }

  :global(.action-btn-grn:hover) {
    background: #10B981;
    color: white;
  }

  /* Form Styles */
  .po-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .select-input {
    width: 100%;
    padding: 8px 12px;
    font-size: 14px;
    font-family: var(--font-family);
    color: var(--text-primary);
    background: var(--surface);
    border: var(--border-width) solid var(--border);
    border-radius: var(--border-radius-sm);
    transition: all var(--transition-fast);
  }

  .select-input:focus {
    outline: none;
    border-color: var(--brand-indigo);
    box-shadow: 0 0 0 3px var(--brand-indigo-tint);
  }

  /* Line Items Section */
  .line-items-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius);
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .section-header h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .empty-items {
    padding: 24px;
    text-align: center;
    color: var(--text-muted);
    font-style: italic;
    font-size: 14px;
  }

  .items-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .item-row {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr 1fr 32px;
    gap: 12px;
    align-items: end;
    padding: 12px;
    background: var(--surface);
    border-radius: var(--border-radius-sm);
    border: 1px solid var(--border);
  }

  .item-total {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .item-total .label {
    font-size: var(--label-size);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .item-total .value {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .btn-remove {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 50%;
    font-size: 24px;
    color: var(--text-muted);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .btn-remove:hover {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  /* Totals Section */
  .totals-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 16px;
    background: var(--surface);
    border-radius: var(--border-radius-sm);
    border: 1px solid var(--border);
  }

  .total-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 0;
  }

  .total-label {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .total-value {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .total-grand {
    padding-top: 8px;
    border-top: 2px solid var(--border);
  }

  .total-grand .total-label,
  .total-grand .total-value {
    font-size: 16px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  .total-bhd {
    padding-top: 8px;
    border-top: 1px dashed var(--border);
  }

  .total-bhd .total-value {
    color: #10B981;
  }

  /* View Modal Styles */
  .po-details {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .details-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .detail-label {
    font-size: var(--label-size);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .detail-value {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .mono {
    font-family: var(--font-mono);
  }

  .items-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .items-section h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .items-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .items-table th {
    text-align: left;
    padding: 8px 12px;
    background: var(--surface-elevated);
    color: var(--text-secondary);
    font-weight: 600;
    border-bottom: 2px solid var(--border);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .items-table td {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
  }

  .items-table .right {
    text-align: right;
  }

  .view-totals {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius-sm);
  }

  .view-totals .total-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .view-totals .label {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .view-totals .value {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .view-totals .grand {
    padding-top: 8px;
    border-top: 2px solid var(--border);
  }

  .view-totals .grand .label,
  .view-totals .grand .value {
    font-size: 16px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  .view-totals .bhd {
    padding-top: 8px;
    border-top: 1px dashed var(--border);
  }

  .view-totals .bhd .value {
    color: #10B981;
  }

  .status-actions {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .status-actions h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .status-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  /* Exchange hint */
  .exchange-hint {
    font-size: 11px;
    color: var(--text-secondary);
    margin-top: 4px;
    font-family: var(--font-mono);
  }

  /* Responsive */
  @media (max-width: 768px) {
    .form-row {
      grid-template-columns: 1fr;
    }

    .item-row {
      grid-template-columns: 1fr;
      gap: 8px;
    }

    .stats-grid {
      grid-template-columns: 1fr;
    }

    .details-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
