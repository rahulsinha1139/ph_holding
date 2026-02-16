<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ListOrders,
    GetOrder,
    CreateOrder,
    UpdateOrder,
    DeleteOrder,
    UpdateOrderStage,
    ListCustomers,
    GetOrderDeliveryStatus,
    GetOrderFulfillmentStatus,
    CreateInvoiceFromOrder,
    CreatePOFromOrder,
    QuickMarkOrderDelivered
  } from '../../../wailsjs/go/main/App';
  import { toast } from '$lib/stores/toasts';
  import { pendingDNCreate } from '$lib/stores/navigation';
  import { main } from '../../../wailsjs/go/models';
  import { escapeHtml } from '$lib/utils/escapeHtml';
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
  import Modal from '$lib/components/layout/Modal.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';

  // Props
  export let embedded = false;

  // Status definitions matching backend statuses
  const ORDER_STATUSES = [
    { key: 'Confirmed', label: 'Confirmed', color: 'blue' },
    { key: 'InProgress', label: 'In Progress', color: 'indigo' },
    { key: 'PartiallyDelivered', label: 'Partially Delivered', color: 'amber' },
    { key: 'FullyDelivered', label: 'Fully Delivered', color: 'green' },
    { key: 'Cancelled', label: 'Cancelled', color: 'red' }
  ];

  // State
  let orders: main.Order[] = [];
  let filteredOrders: main.Order[] = [];
  let loading = true;
  let activeFilter = 'All';
  let searchQuery = '';
  let selectedYear = 'All';
  let selectedCustomer = 'All';
  let availableYears: string[] = [];
  let availableCustomers: string[] = [];

  // Create/Edit modal
  let showModal = false;
  let editingOrder: main.Order | null = null;
  let modalMode: 'create' | 'edit' = 'create';
  let saving = false;
  let isDeleting = false;
  let isProcessingAction = false;

  // Form state
  let formData = {
    orderNumber: '',
    customerPONumber: '',
    customerId: '',
    customerName: '',
    orderDate: new Date().toISOString().split('T')[0],
    requiredDate: '',
    totalValue: 0,
    status: 'Confirmed',
    paymentTerms: 'Net 30',
    deliveryTerms: 'Ex-Works'
  };

  // Helper data
  let customers: any[] = [];

  // Detail modal
  let showDetailModal = false;
  let selectedOrder: main.Order | null = null;
  let orderItems: main.OrderItem[] = [];
  let deliveryProgress = { delivered: 0, total: 0, percentage: 0 };
  let fulfillmentStatus: any = null;

  // DataTable columns
  const columns = [
    {
      key: 'order_number',
      label: 'Order Number',
      sortable: true,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'customer_name',
      label: 'Customer',
      sortable: true,
      type: 'text' as const,
      render: (row) => escapeHtml(row.customer_name)
    },
    {
      key: 'customer_po_number',
      label: 'RFQ/Offer Ref',
      sortable: false,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'total_value_bhd',
      label: 'Total (BHD)',
      sortable: true,
      type: 'currency' as const,
      align: 'right' as const,
      width: '140px'
    },
    {
      key: 'order_date',
      label: 'Order Date',
      sortable: true,
      type: 'date' as const,
      width: '120px'
    },
    {
      key: 'delivery_status',
      label: 'Delivery',
      sortable: false,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      type: 'status' as const,
      width: '160px'
    }
  ];

  // Load data
  async function loadOrders() {
    loading = true;
    try {
      const [ordersData, customersData] = await Promise.all([
        ListOrders(10000, 0),
        ListCustomers(500, 0)
      ]);

      orders = ordersData || [];
      customers = customersData || [];

      // Load delivery status for each order
      await Promise.all(
        orders.map(async (order) => {
          try {
            const deliveryStatus = await GetOrderDeliveryStatus(order.id);
            (order as any).deliveryStatus = deliveryStatus;
          } catch (err) {
            (order as any).deliveryStatus = null;
          }
        })
      );

      // Extract unique years from order dates
      const yearsSet = new Set<string>();
      const customersSet = new Set<string>();
      orders.forEach(o => {
        if (o.order_date) {
          yearsSet.add(new Date(o.order_date).getFullYear().toString());
        }
        if (o.customer_name) {
          customersSet.add(o.customer_name);
        }
      });
      availableYears = Array.from(yearsSet).sort().reverse();
      availableCustomers = Array.from(customersSet).sort();

      applyFilters();
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to load orders: ${errorMsg}`);
    } finally {
      loading = false;
    }
  }

  // Apply filters
  function applyFilters() {
    let result = [...orders];

    // Status filter
    if (activeFilter !== 'All') {
      result = result.filter(o => o.status === activeFilter);
    }

    // Year filter
    if (selectedYear !== 'All') {
      result = result.filter(o => {
        if (!o.order_date) return false;
        return new Date(o.order_date).getFullYear().toString() === selectedYear;
      });
    }

    // Customer filter
    if (selectedCustomer !== 'All') {
      result = result.filter(o => o.customer_name === selectedCustomer);
    }

    // Search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(o =>
        o.order_number?.toLowerCase().includes(query) ||
        o.customer_name?.toLowerCase().includes(query) ||
        o.customer_po_number?.toLowerCase().includes(query)
      );
    }

    filteredOrders = result;
  }

  $: {
    searchQuery;
    activeFilter;
    selectedYear;
    selectedCustomer;
    applyFilters();
  }

  // Stats
  $: stats = {
    total: orders.length,
    confirmed: orders.filter(o => o.status === 'Confirmed').length,
    inProgress: orders.filter(o => o.status === 'InProgress').length,
    delivered: orders.filter(o => o.status === 'FullyDelivered').length,
    totalValue: orders.reduce((sum, o) => sum + (o.total_value_bhd || 0), 0)
  };

  // Open create modal
  function openCreateModal() {
    modalMode = 'create';
    editingOrder = null;
    formData = {
      orderNumber: '',
      customerPONumber: '',
      customerId: '',
      customerName: '',
      orderDate: new Date().toISOString().split('T')[0],
      requiredDate: '',
      totalValue: 0,
      status: 'Confirmed',
      paymentTerms: 'Net 30',
      deliveryTerms: 'Ex-Works'
    };
    showModal = true;
  }

  // Open edit modal
  function openEditModal(order: main.Order) {
    modalMode = 'edit';
    editingOrder = order;
    formData = {
      orderNumber: order.order_number || '',
      customerPONumber: order.customer_po_number || '',
      customerId: order.customer_id || '',
      customerName: order.customer_name || '',
      orderDate: order.order_date ? new Date(order.order_date as any).toISOString().split('T')[0] : '',
      requiredDate: order.required_date ? new Date(order.required_date as any).toISOString().split('T')[0] : '',
      totalValue: order.total_value_bhd || 0,
      status: order.status || 'Confirmed',
      paymentTerms: order.payment_terms || 'Net 30',
      deliveryTerms: order.delivery_terms || 'Ex-Works'
    };
    showModal = true;
  }

  // Handle form submit
  async function handleSubmit() {
    // Validation
    if (!formData.orderNumber.trim()) {
      toast.warning('Order Number is required');
      return;
    }

    if (!formData.customerName.trim() && !formData.customerId.trim()) {
      toast.warning('Customer is required');
      return;
    }

    // Validate total value is non-negative
    if (formData.totalValue < 0) {
      toast.warning('Total Value cannot be negative');
      return;
    }

    // Validate dates
    const orderDate = new Date(formData.orderDate);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    if (orderDate > today) {
      toast.warning('Order date cannot be in the future');
      return;
    }

    if (formData.requiredDate) {
      const requiredDate = new Date(formData.requiredDate);
      if (requiredDate < orderDate) {
        toast.warning('Required date must be after order date');
        return;
      }
    }

    saving = true;
    try {
      if (modalMode === 'create') {
        // Create new order
        await CreateOrder(
          formData.orderNumber,
          formData.customerName,
          formData.totalValue,
          formData.orderDate,
          formData.status
        );
        toast.success('Order created successfully');
      } else if (modalMode === 'edit' && editingOrder) {
        // Send full order update, not just status
        const updatedOrder = {
          ...editingOrder,
          order_number: formData.orderNumber,
          customer_po_number: formData.customerPONumber,
          customer_id: formData.customerId,
          customer_name: formData.customerName,
          order_date: formData.orderDate,
          required_date: formData.requiredDate,
          total_value_bhd: formData.totalValue,
          status: formData.status,
          payment_terms: formData.paymentTerms,
          delivery_terms: formData.deliveryTerms,
        };
        await UpdateOrder(editingOrder.id, updatedOrder as any);
        toast.success('Order updated successfully');
      }

      showModal = false;
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to save order: ' + (err as Error).message);
    } finally {
      saving = false;
    }
  }

  // Handle row click - open detail modal
  async function handleRowClick(row: main.Order) {
    selectedOrder = row;

    try {
      const [fullOrder, fulfillment] = await Promise.all([
        GetOrder(row.id),
        GetOrderFulfillmentStatus(row.id).catch(() => null)
      ]);

      selectedOrder = fullOrder;
      orderItems = fullOrder.items || [];
      fulfillmentStatus = fulfillment;

      if (orderItems.length > 0) {
        const totalQty = orderItems.reduce((sum, item) => sum + item.quantity, 0);
        const shippedQty = orderItems.reduce((sum, item) => sum + (item.quantity_shipped || 0), 0);
        const percentage = totalQty > 0 ? Math.round((shippedQty / totalQty) * 100) : 0;
        deliveryProgress = { delivered: shippedQty, total: totalQty, percentage };
      } else {
        deliveryProgress = { delivered: 0, total: 0, percentage: 0 };
      }
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.warning(`Could not load full order details: ${errorMsg}`);
    }

    showDetailModal = true;
  }

  // Quick actions
  async function handleCreateDeliveryNote(order: main.Order) {
    // Set pending DN creation in store (DeliveryNotesScreen will consume on mount)
    pendingDNCreate.request(order.id, order.order_number || '', order.customer_name || '');
    // Navigate to operations screen with delivery-notes tab
    window.dispatchEvent(new CustomEvent('navigateToScreen', {
      detail: { screen: 'operations', tab: 'delivery-notes' }
    }));
  }

  async function handleCreateInvoice(order: main.Order) {
    if (isProcessingAction) return;

    if (!confirm(`Create invoice for order ${order.order_number}?\nThis will generate a billing record for ${order.customer_name}.`)) {
      return;
    }

    isProcessingAction = true;
    try {
      const invoice = await CreateInvoiceFromOrder(order.id);
      toast.success(`Invoice ${invoice.invoice_number} created`);
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to create invoice: ' + (err as Error).message);
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleCreatePO(order: main.Order) {
    if (isProcessingAction) return;

    if (!orderItems || orderItems.length === 0) {
      toast.warning('No order items loaded. Please close and reopen the order details.');
      return;
    }

    isProcessingAction = true;
    try {
      const itemIDs = orderItems.map(item => item.id);
      const po = await CreatePOFromOrder(order.id, '', itemIDs);
      toast.success(`PO ${po.po_number} created`);
    } catch (err) {
      toast.danger('Failed to create PO: ' + (err as Error).message);
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleQuickMarkDelivered() {
    if (isProcessingAction) return;
    if (!selectedOrder) return;

    if (!confirm(`Mark order ${selectedOrder.order_number} as fully delivered?\n\nThis will update the order status without creating a GRN record.`)) {
      return;
    }

    isProcessingAction = true;
    try {
      const message = await QuickMarkOrderDelivered(selectedOrder.id);
      toast.success(message);
      showDetailModal = false;
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to mark as delivered: ' + (err as Error).message);
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleDeleteOrder(order: main.Order) {
    if (isDeleting) return;

    if (!confirm(`Are you sure you want to delete order ${order.order_number}? This cannot be undone.`)) {
      return;
    }

    isDeleting = true;
    try {
      await DeleteOrder(order.id);
      toast.success(`Order ${order.order_number} deleted`);
      showDetailModal = false;
      selectedOrder = null;
      await loadOrders();
    } catch (err) {
      toast.danger('Delete failed: ' + (err as Error).message);
    } finally {
      isDeleting = false;
    }
  }

  // Format currency
  function formatCurrency(value: number): string {
    return new Intl.NumberFormat('en-BH', {
      style: 'currency',
      currency: 'BHD',
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    }).format(value);
  }

  // Format date
  function formatDate(date: any): string {
    if (!date) return '—';
    return new Date(date).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  }

  // Format delivery status
  function formatDeliveryStatus(order: main.Order): string {
    const deliveryStatus = (order as any).deliveryStatus;
    if (!deliveryStatus) return '—';

    const totalItems = Object.keys(deliveryStatus).length;
    const deliveredItems = Object.values(deliveryStatus).filter((qty: any) => qty > 0).length;

    if (deliveredItems === 0) return 'Not Started';
    if (deliveredItems === totalItems) return '100% Complete';
    return `${Math.round((deliveredItems / totalItems) * 100)}% (${deliveredItems}/${totalItems} items)`;
  }

  onMount(() => {
    loadOrders();
  });
</script>

{#if embedded}
  <!-- Embedded mode - ALL orders grouped by year -->
  <div class="orders-embedded">
    <!-- Summary bar -->
    <div class="embedded-header">
      <h3>All Orders ({orders.length})</h3>
      <div class="embedded-stats">
        <span class="embedded-stat">{formatCurrency(stats.totalValue)} total</span>
        <span class="embedded-stat">{stats.confirmed} confirmed</span>
        <span class="embedded-stat">{stats.delivered} delivered</span>
      </div>
    </div>

    <!-- Filters -->
    <div class="embedded-filters">
      <input
        type="text"
        placeholder="Search orders..."
        bind:value={searchQuery}
        class="embedded-search"
      />
      <select class="embedded-select" bind:value={activeFilter}>
        <option value="All">All Status</option>
        {#each ORDER_STATUSES as status}
          <option value={status.key}>{status.label}</option>
        {/each}
      </select>
    </div>

    {#if loading}
      <div class="loading-container">
        <WabiSpinner size="md" />
      </div>
    {:else if filteredOrders.length === 0}
      <p class="empty-message">No orders found</p>
    {:else}
      <!-- Group orders by year -->
      {#each availableYears as year}
        {@const yearOrders = filteredOrders.filter(o => o.order_date && new Date(o.order_date).getFullYear().toString() === year)}
        {#if yearOrders.length > 0}
          <div class="year-group">
            <div class="year-header">
              <span class="year-label">{year}</span>
              <span class="year-count">{yearOrders.length} orders</span>
              <span class="year-value">{formatCurrency(yearOrders.reduce((sum, o) => sum + (o.total_value_bhd || 0), 0))}</span>
            </div>
            <div class="orders-list">
              {#each yearOrders as order}
                <div class="order-item" on:click={() => handleRowClick(order)}>
                  <div class="order-left">
                    <span class="order-number">{order.order_number}</span>
                    <span class="order-customer">{order.customer_name}</span>
                    {#if order.customer_po_number}
                      <span class="order-ref">Ref: {order.customer_po_number}</span>
                    {/if}
                  </div>
                  <div class="order-right">
                    <StatusBadge status={order.status} size="sm" />
                    <span class="order-value">{formatCurrency(order.total_value_bhd || 0)}</span>
                    <span class="order-date">{formatDate(order.order_date)}</span>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      {/each}

      <!-- Orders without dates -->
      {@const undatedOrders = filteredOrders.filter(o => !o.order_date)}
      {#if undatedOrders.length > 0}
        <div class="year-group">
          <div class="year-header">
            <span class="year-label">No Date</span>
            <span class="year-count">{undatedOrders.length} orders</span>
          </div>
          <div class="orders-list">
            {#each undatedOrders as order}
              <div class="order-item" on:click={() => handleRowClick(order)}>
                <div class="order-left">
                  <span class="order-number">{order.order_number}</span>
                  <span class="order-customer">{order.customer_name}</span>
                </div>
                <div class="order-right">
                  <StatusBadge status={order.status} size="sm" />
                  <span class="order-value">{formatCurrency(order.total_value_bhd || 0)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  </div>
{:else}
  <!-- Full screen mode with DataTable -->
  <PageLayout title="Orders" subtitle="Sales Order Management">
    <div slot="header-actions" class="header-actions">
      <div class="stats-row">
        <div class="stat-item">
          <span class="stat-value">{stats.total}</span>
          <span class="stat-label">Total Orders</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{formatCurrency(stats.totalValue)}</span>
          <span class="stat-label">Total Value</span>
        </div>
      </div>
      <Button variant="primary" on:click={openCreateModal}>
        + New Order
      </Button>
    </div>

    <!-- Filters -->
    <div class="controls-bar">
      <div class="filter-tabs">
        <button
          class="filter-tab"
          class:active={activeFilter === 'All'}
          on:click={() => activeFilter = 'All'}
        >
          All ({orders.length})
        </button>
        {#each ORDER_STATUSES as status}
          <button
            class="filter-tab"
            class:active={activeFilter === status.key}
            on:click={() => activeFilter = status.key}
          >
            {status.label} ({orders.filter(o => o.status === status.key).length})
          </button>
        {/each}
      </div>

      <div class="dropdown-filters">
        <select class="filter-select" bind:value={selectedYear}>
          <option value="All">All Years</option>
          {#each availableYears as year}
            <option value={year}>{year}</option>
          {/each}
        </select>

        <select class="filter-select" bind:value={selectedCustomer}>
          <option value="All">All Customers</option>
          {#each availableCustomers as customer}
            <option value={customer}>{customer}</option>
          {/each}
        </select>
      </div>

      <div class="search-box">
        <input
          type="text"
          placeholder="Search orders..."
          bind:value={searchQuery}
          class="search-input"
        />
      </div>
    </div>

    <!-- DataTable -->
    <Card>
      {#if loading}
        <div class="loading-container">
          <WabiSpinner size="lg" />
        </div>
      {:else}
        <DataTable
          {columns}
          data={filteredOrders}
          {loading}
          emptyMessage="No orders found"
          onRowClick={handleRowClick}
          keyField="id"
          stickyHeader={true}
          maxHeight="calc(100vh - 280px)"
        >
          <div slot="cell" let:column let:row let:value>
            {#if column.key === 'status'}
              <StatusBadge status={value} />
            {:else if column.key === 'delivery_status'}
              <div class="delivery-progress">
                <span class="progress-text">{formatDeliveryStatus(row)}</span>
              </div>
            {:else if column.key === 'customer_po_number'}
              <span class="ref-value">{value || '—'}</span>
            {:else}
              {value}
            {/if}
          </div>
        </DataTable>
      {/if}
    </Card>
  </PageLayout>
{/if}

<!-- Create/Edit Modal -->
{#if showModal}
  <Modal
    title={modalMode === 'create' ? 'New Order' : 'Edit Order'}
    open={showModal}
    on:close={() => showModal = false}
  >
    <form on:submit|preventDefault={handleSubmit}>
      <div class="form-row">
        <FormGroup label="Order Number" required>
          <Input
            bind:value={formData.orderNumber}
            placeholder="ORD-2025-001"
            disabled={modalMode === 'edit'}
          />
        </FormGroup>

        <FormGroup label="Customer PO Number">
          <Input
            bind:value={formData.customerPONumber}
            placeholder="Customer's PO reference"
          />
        </FormGroup>
      </div>

      <FormGroup label="Customer" required>
        <input
          list="customers-list"
          bind:value={formData.customerName}
          placeholder="Search customer..."
          class="form-input"
        />
        <datalist id="customers-list">
          {#each customers as customer}
            <option value={customer.business_name}>{customer.business_name}</option>
          {/each}
        </datalist>
      </FormGroup>

      <div class="form-row">
        <FormGroup label="Order Date" required>
          <Input type="date" bind:value={formData.orderDate} />
        </FormGroup>

        <FormGroup label="Required Date">
          <Input type="date" bind:value={formData.requiredDate} />
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="Total Value (BHD)">
          <Input
            type="number"
            step="0.001"
            min="0"
            bind:value={formData.totalValue}
            placeholder="0.000"
          />
        </FormGroup>

        <FormGroup label="Status" required>
          <select bind:value={formData.status} class="form-select">
            {#each ORDER_STATUSES as status}
              <option value={status.key}>{status.label}</option>
            {/each}
          </select>
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="Payment Terms">
          <select bind:value={formData.paymentTerms} class="form-select">
            <option value="Net 7">Net 7</option>
            <option value="Net 15">Net 15</option>
            <option value="Net 30">Net 30</option>
            <option value="Net 45">Net 45</option>
            <option value="Net 60">Net 60</option>
            <option value="Cash on Delivery">Cash on Delivery</option>
            <option value="Advance Payment">Advance Payment</option>
          </select>
        </FormGroup>

        <FormGroup label="Delivery Terms">
          <select bind:value={formData.deliveryTerms} class="form-select">
            <option value="Ex-Works">Ex-Works</option>
            <option value="FOB">FOB</option>
            <option value="CIF">CIF</option>
            <option value="DDP">DDP</option>
          </select>
        </FormGroup>
      </div>

      <div class="modal-actions">
        <Button variant="ghost" on:click={() => showModal = false} disabled={saving}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={saving}>
          {saving ? 'Saving...' : modalMode === 'edit' ? 'Update Order' : 'Create Order'}
        </Button>
      </div>
    </form>
  </Modal>
{/if}

<!-- Order Detail Modal -->
{#if showDetailModal && selectedOrder}
  <Modal
    title={`Order ${selectedOrder.order_number}`}
    open={showDetailModal}
    on:close={() => showDetailModal = false}
    size="lg"
  >
    <div class="order-detail">
      <!-- Header Info -->
      <div class="detail-header">
        <div class="detail-section">
          <h4>Customer</h4>
          <p>{selectedOrder.customer_name}</p>
        </div>
        <div class="detail-section">
          <h4>Order Date</h4>
          <p>{formatDate(selectedOrder.order_date)}</p>
        </div>
        <div class="detail-section">
          <h4>Status</h4>
          <StatusBadge status={selectedOrder.status} />
        </div>
        <div class="detail-section">
          <h4>Total Value</h4>
          <p class="value-highlight">{formatCurrency(selectedOrder.total_value_bhd || 0)}</p>
        </div>
      </div>

      <!-- Traceability Chain -->
      <div class="traceability-section">
        <h4>Traceability Chain</h4>
        <div class="chain">
          <div class="chain-link">
            <span class="chain-label">RFQ</span>
            <span class="chain-value">{selectedOrder.customer_po_number || 'N/A'}</span>
          </div>
          <span class="chain-arrow">></span>
          <div class="chain-link">
            <span class="chain-label">Offer</span>
            <span class="chain-value">—</span>
          </div>
          <span class="chain-arrow">></span>
          <div class="chain-link active">
            <span class="chain-label">Order</span>
            <span class="chain-value">{selectedOrder.order_number}</span>
          </div>
        </div>
      </div>

      <!-- Delivery Progress -->
      {#if orderItems.length > 0}
        <div class="delivery-section">
          <h4>Delivery Progress</h4>
          <div class="progress-bar-container">
            <div class="progress-bar">
              <div
                class="progress-fill"
                style="width: {deliveryProgress.percentage}%"
              />
            </div>
            <span class="progress-label">
              {deliveryProgress.delivered} / {deliveryProgress.total} items delivered ({deliveryProgress.percentage}%)
            </span>
          </div>
        </div>
      {/if}

      <!-- Order Items -->
      {#if orderItems.length > 0}
        <div class="items-section">
          <h4>Order Items</h4>
          <table class="items-table">
            <thead>
              <tr>
                <th>Line</th>
                <th>Product Code</th>
                <th>Description</th>
                <th class="number-col">Quantity</th>
                <th class="number-col">Shipped</th>
                <th class="number-col">Invoiced</th>
                <th class="number-col">Unit Price</th>
                <th class="number-col">Total</th>
              </tr>
            </thead>
            <tbody>
              {#each orderItems as item}
                <tr>
                  <td>{item.line_number}</td>
                  <td>{item.product_code}</td>
                  <td>{item.description}</td>
                  <td class="number-col">{item.quantity}</td>
                  <td class="number-col">{item.quantity_shipped || 0}</td>
                  <td class="number-col">{item.quantity_invoiced || 0}</td>
                  <td class="number-col">{formatCurrency(item.unit_price_bhd)}</td>
                  <td class="number-col">{formatCurrency(item.quantity * item.unit_price_bhd)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Quick Actions (conditional on order state) -->
      <div class="quick-actions">
        {#if selectedOrder.status !== 'Cancelled'}
          <Button variant="secondary" on:click={() => handleCreatePO(selectedOrder)} disabled={isProcessingAction}>
            Create PO
          </Button>
        {/if}
        {#if selectedOrder.status !== 'Cancelled' && (!fulfillmentStatus || fulfillmentStatus?.fulfillment_pct < 1.0)}
          <Button variant="secondary" on:click={() => handleCreateDeliveryNote(selectedOrder)} disabled={isProcessingAction}>
            Create Delivery Note
          </Button>
        {/if}
        {#if selectedOrder.status !== 'Cancelled' && selectedOrder.status !== 'FullyDelivered'}
          <Button variant="primary" on:click={handleQuickMarkDelivered} disabled={isProcessingAction}>
            Mark as Delivered
          </Button>
        {/if}
        {#if selectedOrder.status !== 'Cancelled' && (!fulfillmentStatus || fulfillmentStatus?.invoicing_pct < 1.0)}
          <Button variant="secondary" on:click={() => handleCreateInvoice(selectedOrder)} disabled={isProcessingAction}>
            Create Invoice
          </Button>
        {/if}
        <Button variant="ghost" on:click={() => openEditModal(selectedOrder)} disabled={isProcessingAction || isDeleting}>
          Edit Order
        </Button>
        <Button variant="ghost" on:click={() => handleDeleteOrder(selectedOrder)} disabled={isDeleting || isProcessingAction} style="color: #EF4444;">
          Delete Order
        </Button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  /* Embedded Mode */
  .orders-embedded {
    padding: 16px;
    max-height: calc(100vh - 200px);
    overflow-y: auto;
  }

  .embedded-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .orders-embedded h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .embedded-stats {
    display: flex;
    gap: 16px;
  }

  .embedded-stat {
    font-size: 11px;
    color: var(--text-secondary);
    font-family: var(--font-mono, monospace);
  }

  .embedded-filters {
    display: flex;
    gap: 8px;
    margin-bottom: 16px;
  }

  .embedded-search {
    flex: 1;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 6px);
    font-size: 12px;
    outline: none;
  }

  .embedded-search:focus {
    border-color: var(--primary, #4F46E5);
  }

  .embedded-select {
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 6px);
    font-size: 12px;
    background: transparent;
    outline: none;
    cursor: pointer;
    min-width: 120px;
  }

  /* Year Groups */
  .year-group {
    margin-bottom: 20px;
  }

  .year-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 2px solid var(--text-primary, #1a1a1a);
    margin-bottom: 8px;
  }

  .year-label {
    font-size: 18px;
    font-weight: 700;
    color: var(--text-primary);
    font-family: var(--font-mono, monospace);
  }

  .year-count {
    font-size: 11px;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .year-value {
    margin-left: auto;
    font-size: 13px;
    font-weight: 600;
    font-family: var(--font-mono, monospace);
    color: var(--text-primary);
  }

  .orders-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .order-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 12px;
    background: var(--surface, #fafafa);
    border: 1px solid var(--border, #e5e5e5);
    border-radius: var(--radius-md, 6px);
    cursor: pointer;
    transition: all 0.15s;
  }

  .order-item:hover {
    background: var(--bg-hover, #f0f0f0);
    border-color: var(--text-muted, #999);
  }

  .order-left {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  .order-number {
    font-size: 12px;
    font-weight: 600;
    font-family: var(--font-mono, monospace);
    color: var(--text-primary);
    white-space: nowrap;
  }

  .order-customer {
    font-size: 12px;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 200px;
  }

  .order-ref {
    font-size: 10px;
    color: var(--text-muted, #999);
    font-family: var(--font-mono, monospace);
    white-space: nowrap;
  }

  .order-right {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-shrink: 0;
  }

  .order-value {
    font-size: 12px;
    font-family: var(--font-mono, monospace);
    font-weight: 600;
    color: var(--text-primary);
    min-width: 100px;
    text-align: right;
  }

  .order-date {
    font-size: 11px;
    color: var(--text-muted, #999);
    min-width: 80px;
    text-align: right;
  }

  /* Full Screen Mode */
  .header-actions {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .stats-row {
    display: flex;
    gap: 24px;
    margin-right: auto;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .stat-value {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.2;
  }

  .stat-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin-top: 2px;
  }

  /* Controls Bar */
  .controls-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    gap: 16px;
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
    border-radius: var(--radius-md);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .filter-tab:hover {
    background: var(--bg-subtle);
    color: var(--text-primary);
  }

  .filter-tab.active {
    background: var(--primary);
    color: white;
    border-color: var(--primary);
  }

  .dropdown-filters {
    display: flex;
    gap: 8px;
  }

  .filter-select {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    background: transparent;
    color: var(--text-primary);
    outline: none;
    cursor: pointer;
    min-width: 140px;
    transition: border-color 0.2s;
  }

  .filter-select:focus {
    border-color: var(--primary);
  }

  .search-box {
    flex-shrink: 0;
  }

  .search-input {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    width: 280px;
    outline: none;
    transition: border-color 0.2s;
  }

  .search-input:focus {
    border-color: var(--primary);
  }

  /* Loading */
  .loading-container {
    display: flex;
    justify-content: center;
    padding: 48px;
  }

  .empty-message {
    text-align: center;
    padding: 24px;
    color: var(--text-secondary);
    font-size: 13px;
  }

  /* Custom cells */
  .delivery-progress {
    font-size: 12px;
    font-family: var(--font-mono);
    color: var(--text-secondary);
  }

  .ref-value {
    font-size: 12px;
    font-family: var(--font-mono);
    color: var(--text-secondary);
  }

  /* Form */
  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .form-input,
  .form-select {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    outline: none;
    transition: border-color 0.2s;
    font-family: var(--font-sans);
  }

  .form-input:focus,
  .form-select:focus {
    border-color: var(--primary);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }

  /* Order Detail */
  .order-detail {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .detail-header {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border);
  }

  .detail-section h4 {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin: 0 0 4px 0;
  }

  .detail-section p {
    font-size: 14px;
    color: var(--text-primary);
    margin: 0;
  }

  .value-highlight {
    font-size: 16px !important;
    font-weight: 600;
    color: var(--primary);
  }

  /* Traceability Chain */
  .traceability-section h4 {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .chain {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    background: var(--bg-subtle);
    border-radius: var(--radius-md);
  }

  .chain-link {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px 16px;
    background: white;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    flex: 1;
  }

  .chain-link.active {
    border-color: var(--primary);
    background: var(--primary-subtle);
  }

  .chain-label {
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .chain-value {
    font-size: 12px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--text-primary);
  }

  .chain-arrow {
    color: var(--text-secondary);
    font-size: 16px;
  }

  /* Delivery Progress */
  .delivery-section h4 {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .progress-bar-container {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .progress-bar {
    height: 8px;
    background: var(--bg-subtle);
    border-radius: 4px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--primary), var(--primary-light));
    transition: width 0.3s ease;
  }

  .progress-label {
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* Items Table */
  .items-section h4 {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .items-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .items-table thead {
    background: var(--bg-subtle);
  }

  .items-table th {
    padding: 8px;
    text-align: left;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
  }

  .items-table td {
    padding: 8px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .items-table .number-col {
    text-align: right;
    font-family: var(--font-mono);
  }

  .items-table tbody tr:hover {
    background: var(--bg-subtle);
  }

  /* Quick Actions */
  .quick-actions {
    display: flex;
    gap: 8px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
</style>
