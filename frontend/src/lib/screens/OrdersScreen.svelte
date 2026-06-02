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
    CreateProformaInvoice,
    CreatePOsFromOrder,
    QuickMarkOrderDelivered,
    PreviewOrderDeleteCascade,
    GetOrdersWithNoItems,
    ExportReport,
    OpenExportedFile
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
  import ContextTaskModal from '$lib/components/ContextTaskModal.svelte';

  // Props
  export let embedded = false;

  // Status definitions matching backend statuses
  const ORDER_STATUSES = [
    { key: 'Confirmed', label: 'Confirmed', color: 'blue' },
    { key: 'InProgress', label: 'In Progress', color: 'indigo' },
    { key: 'PartiallyDelivered', label: 'Partially Delivered', color: 'amber' },
    { key: 'FullyDelivered', label: 'Fully Delivered', color: 'green' },
    { key: 'Invoiced', label: 'Invoiced', color: 'emerald' },
    { key: 'Cancelled', label: 'Cancelled', color: 'red' }
  ];

  function errorMessage(err: unknown) {
    if (err instanceof Error && err.message) return err.message;
    if (typeof err === 'string') return err;
    if (err && typeof err === 'object' && 'message' in err) {
      return String((err as { message?: unknown }).message || err);
    }
    return String(err || 'Unknown error');
  }

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
  let ordersWithNoItems: any[] = [];
  let exportingExtract = false;

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
  interface EditableOrderItem {
    id?: string;
    product_id?: string;
    product_code: string;
    description: string;
    quantity: number;
    unit_price_bhd: number;
    total_price: number;
    quantity_shipped?: number;
    quantity_invoiced?: number;
    equipment?: string;
    model?: string;
    specification?: string;
    detailed_description?: string;
    currency?: string;
    fob?: number;
    freight?: number;
    total_cost?: number;
    margin_percent?: number;
    line_number?: number;
  }
  let formItems: EditableOrderItem[] = [];

  // Helper data
  let customers: any[] = [];

  function normalizeCustomerName(value: string) {
    return (value || '').trim().toLowerCase().replace(/\s+/g, ' ');
  }

  function resolveCustomerId(customerId: string, customerName: string) {
    if (customerId && customers.some((customer) => customer.id === customerId)) {
      return customerId;
    }
    const normalizedName = normalizeCustomerName(customerName);
    if (!normalizedName) return '';
    const match = customers.find((customer) => normalizeCustomerName(customer.business_name || '') === normalizedName);
    return match?.id || '';
  }

  // Detail modal
  let showDetailModal = false;
  let selectedOrder: main.Order | null = null;
  let showTaskModal = false;
  let orderItems: EditableOrderItem[] = [];
  let deliveryProgress = { delivered: 0, total: 0, percentage: 0 };
  let fulfillmentStatus: any = null;
  type ConfirmVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success' | 'warning';
  let confirmDialogOpen = false;
  let confirmDialogTitle = '';
  let confirmDialogMessage = '';
  let confirmDialogConfirmLabel = 'Confirm';
  let confirmDialogVariant: ConfirmVariant = 'primary';
  let confirmDialogResolve: ((confirmed: boolean) => void) | null = null;

  function getOrderDateValue(value: any): Date | null {
    if (!value) return null;
    if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value;
    if (typeof value === 'string' || typeof value === 'number') {
      const parsed = new Date(value);
      return Number.isNaN(parsed.getTime()) ? null : parsed;
    }
    if (typeof value === 'object' && typeof value.toString === 'function') {
      const parsed = new Date(value.toString());
      return Number.isNaN(parsed.getTime()) ? null : parsed;
    }
    return null;
  }

  function createEmptyFormItem(lineNumber: number): EditableOrderItem {
    return {
      line_number: lineNumber,
      product_code: '',
      description: '',
      quantity: 1,
      unit_price_bhd: 0,
      total_price: 0,
      quantity_shipped: 0,
      quantity_invoiced: 0,
      equipment: '',
      model: '',
      specification: '',
      detailed_description: '',
      currency: 'BHD',
      fob: 0,
      freight: 0,
      total_cost: 0,
      margin_percent: 0
    };
  }

  function roundMoney(value: number): number {
    const numeric = Number(value) || 0;
    return Math.round(numeric * 1000) / 1000;
  }

  function safeTrim(value: unknown): string {
    return typeof value === 'string' ? value.trim() : '';
  }

  function normalizeOrderStatusKey(value: unknown): string {
    const raw = safeTrim(value);
    const compact = raw.toLowerCase().replace(/[\s_-]+/g, '');

    if (!compact || compact === 'all') return '';
    if (compact.includes('invoice')) return 'Invoiced';
    if (compact.includes('partial') && compact.includes('deliver')) return 'PartiallyDelivered';
    if (compact.includes('deliver') || compact.includes('complete') || compact.includes('closed')) return 'FullyDelivered';
    if (compact.includes('progress') || compact.includes('process')) return 'InProgress';
    if (compact.includes('cancel') || compact.includes('void')) return 'Cancelled';
    if (compact.includes('confirm') || compact.includes('open') || compact.includes('pending')) return 'Confirmed';

    return raw;
  }

  function orderMatchesStatus(order: main.Order, statusKey: string): boolean {
    if (statusKey === 'All') return true;
    return normalizeOrderStatusKey(order.status) === statusKey;
  }

  function isSummaryLineItem(item: Partial<EditableOrderItem>): boolean {
    const description = safeTrim(item.description).toLowerCase().replace(/\s+/g, ' ');
    const productCode = safeTrim(item.product_code).toLowerCase().replace(/\s+/g, ' ');
    return description.startsWith('total for order') || productCode === 'total for order';
  }

  function sanitizeFormItems(items: EditableOrderItem[], targetTotal = 0): EditableOrderItem[] {
    const cleaned = items
      .filter((item) => !isSummaryLineItem(item))
      .map((item) => {
        const quantity = Number(item.quantity) || 0;
        const incomingTotal = Number(item.total_price) || 0;
        let unitPrice = Number(item.unit_price_bhd) || 0;
        if (unitPrice <= 0 && quantity > 0 && incomingTotal > 0) {
          unitPrice = roundMoney(incomingTotal / quantity);
        }
        const totalPrice = incomingTotal > 0 ? roundMoney(incomingTotal) : roundMoney(quantity * unitPrice);
        return {
          ...item,
          quantity,
          unit_price_bhd: roundMoney(unitPrice),
          total_price: totalPrice
        };
      })
      .filter((item) => {
        const hasIdentity = safeTrim(item.description) || safeTrim(item.product_code) || safeTrim(item.equipment) || safeTrim(item.model);
        const hasValue = (item.quantity || 0) > 0 && ((item.unit_price_bhd || 0) > 0 || (item.total_price || 0) > 0);
        return Boolean(hasIdentity || hasValue);
      });

    const deduped: EditableOrderItem[] = [];
    const seen = new Set<string>();
    for (const item of cleaned) {
      const signature = [
        safeTrim(item.product_code).toLowerCase(),
        safeTrim(item.description).toLowerCase(),
        roundMoney(item.quantity || 0),
        roundMoney(item.unit_price_bhd || 0),
        roundMoney(item.total_price || 0),
      ].join("|");
      if (seen.has(signature)) continue;
      seen.add(signature);
      deduped.push(item);
    }

    const normalizedTarget = roundMoney(targetTotal || 0);
    if (normalizedTarget > 0) {
      const exactMatch = deduped.find((item) => Math.abs(roundMoney(item.total_price || 0) - normalizedTarget) <= 0.01);
      if (exactMatch) {
        return [{ ...exactMatch, line_number: 1 }];
      }
    }

    return deduped.map((item, index) => ({
      ...item,
      line_number: index + 1,
    }));
  }

  function recalculateFormItems() {
    const normalizedItems = sanitizeFormItems(formItems);
    formItems = normalizedItems.length > 0 ? normalizedItems : [createEmptyFormItem(1)];
    formData.totalValue = roundMoney(normalizedItems.reduce((sum, item) => sum + (item.total_price || 0), 0));
  }

  function addFormItem() {
    const normalizedItems = sanitizeFormItems(formItems);
    formItems = [...normalizedItems, createEmptyFormItem(normalizedItems.length + 1)];
    formData.totalValue = roundMoney(normalizedItems.reduce((sum, item) => sum + (item.total_price || 0), 0));
  }

  function removeFormItem(index: number) {
    formItems = formItems.filter((_, itemIndex) => itemIndex !== index);
    if (formItems.length === 0) {
      formItems = [createEmptyFormItem(1)];
    }
    recalculateFormItems();
  }

  function getLineItemIssues(items: Array<{ quantity?: number; unit_price_bhd?: number; total_price?: number; description?: string; product_code?: string }>) {
    const normalized = items || [];
    return {
      missingItems: normalized.length === 0,
      zeroPriceItems: normalized.filter(item =>
        ((item.quantity || 0) > 0 || (item.description || '').trim() || (item.product_code || '').trim()) &&
        ((item.unit_price_bhd || 0) <= 0 || (item.total_price || 0) <= 0)
      ).length
    };
  }

  function askActionConfirmation(options: {
    title: string;
    message: string;
    confirmLabel?: string;
    variant?: ConfirmVariant;
  }): Promise<boolean> {
    return new Promise((resolve) => {
      if (confirmDialogResolve) {
        confirmDialogResolve(false);
      }
      confirmDialogTitle = options.title;
      confirmDialogMessage = options.message;
      confirmDialogConfirmLabel = options.confirmLabel || 'Confirm';
      confirmDialogVariant = options.variant || 'primary';
      confirmDialogResolve = resolve;
      confirmDialogOpen = true;
    });
  }

  function resolveActionConfirmation(confirmed: boolean) {
    const resolver = confirmDialogResolve;
    confirmDialogOpen = false;
    confirmDialogResolve = null;
    if (resolver) {
      resolver(confirmed);
    }
  }

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
        const orderDate = getOrderDateValue(o.order_date);
        if (orderDate) {
          yearsSet.add(orderDate.getFullYear().toString());
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
      result = result.filter(o => orderMatchesStatus(o, activeFilter));
    }

    // Year filter
    if (selectedYear !== 'All') {
      result = result.filter(o => {
        const orderDate = getOrderDateValue(o.order_date);
        if (!orderDate) return false;
        return orderDate.getFullYear().toString() === selectedYear;
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

  async function exportOrdersExtract() {
    exportingExtract = true;
    try {
      const path = await ExportReport('customer_order_extract', 'excel', JSON.stringify(filteredOrders));
      await OpenExportedFile(path).catch(() => {});
      toast.success(`Customer order extract exported: ${path}`);
    } catch (err) {
      toast.danger(`Failed to export orders: ${errorMessage(err)}`);
    } finally {
      exportingExtract = false;
    }
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
    confirmed: orders.filter(o => orderMatchesStatus(o, 'Confirmed')).length,
    inProgress: orders.filter(o => orderMatchesStatus(o, 'InProgress')).length,
    delivered: orders.filter(o => orderMatchesStatus(o, 'FullyDelivered')).length,
    totalValue: orders.reduce((sum, o) => sum + (o.total_value_bhd || 0), 0)
  };

  $: zeroValueOrders = orders.filter((order) => (order.total_value_bhd || 0) <= 0);

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
    formItems = [createEmptyFormItem(1)];
    showModal = true;
  }

  // Open edit modal
  async function openEditModal(order: main.Order) {
    modalMode = 'edit';
    const fullOrder = (order.items && order.items.length > 0) ? order : await GetOrder(order.id);
    editingOrder = fullOrder;
    const resolvedCustomerId = resolveCustomerId(fullOrder.customer_id || '', fullOrder.customer_name || '');
    formData = {
      orderNumber: fullOrder.order_number || '',
      customerPONumber: fullOrder.customer_po_number || '',
      customerId: resolvedCustomerId,
      customerName: fullOrder.customer_name || '',
      orderDate: fullOrder.order_date ? new Date(fullOrder.order_date as any).toISOString().split('T')[0] : '',
      requiredDate: fullOrder.required_date ? new Date(fullOrder.required_date as any).toISOString().split('T')[0] : '',
      totalValue: fullOrder.total_value_bhd || 0,
      status: normalizeOrderStatusKey(fullOrder.status) || 'Confirmed',
      paymentTerms: fullOrder.payment_terms || 'Net 30',
      deliveryTerms: fullOrder.delivery_terms || 'Ex-Works'
    };
    formItems = fullOrder.items && fullOrder.items.length > 0
      ? sanitizeFormItems(fullOrder.items.map((item) => ({ ...item } as EditableOrderItem)), fullOrder.total_value_bhd || fullOrder.grand_total_bhd || 0)
      : [createEmptyFormItem(1)];
    showDetailModal = false;
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

    const normalizedItems = sanitizeFormItems(formItems).filter(item =>
      safeTrim(item.description) || safeTrim(item.product_code) || item.quantity > 0 || item.unit_price_bhd > 0
    );

    if (normalizedItems.length === 0) {
      toast.warning('Add at least one line item with quantity and price before saving this order');
      return;
    }

    for (const item of normalizedItems) {
      if (item.quantity <= 0) {
        toast.warning('Each line item must have a quantity greater than zero');
        return;
      }
      if (item.unit_price_bhd <= 0) {
        toast.warning('Each line item must have a unit price greater than zero');
        return;
      }
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
      const resolvedCustomerId = resolveCustomerId(formData.customerId, formData.customerName);
      const computedTotalValue = roundMoney(normalizedItems.reduce((sum, item) => {
        const lineTotal = roundMoney((Number(item.quantity) || 0) * (Number(item.unit_price_bhd) || 0));
        return sum + lineTotal;
      }, 0));
      formData.customerId = resolvedCustomerId;
      formData.totalValue = computedTotalValue;

      if (modalMode === 'create') {
        const createdOrder = await CreateOrder(
          formData.orderNumber,
          formData.customerName,
          formData.totalValue,
          formData.orderDate,
          formData.status
        );

        const createdOrderWithItems = {
          ...createdOrder,
          order_number: formData.orderNumber,
          customer_po_number: formData.customerPONumber,
          customer_id: resolvedCustomerId,
          customer_name: formData.customerName,
          order_date: formData.orderDate,
          required_date: formData.requiredDate,
          total_value_bhd: computedTotalValue,
          grand_total_bhd: computedTotalValue,
          status: formData.status,
          payment_terms: formData.paymentTerms,
          delivery_terms: formData.deliveryTerms,
          items: normalizedItems.map((item, index) => ({
            ...item,
            line_number: index + 1,
            total_price: roundMoney(item.quantity * item.unit_price_bhd)
          }))
        };

        await UpdateOrder(createdOrder.id, createdOrderWithItems as any);
        toast.success('Order created successfully');
      } else if (modalMode === 'edit' && editingOrder) {
        // Send full order update, not just status
        const updatedOrder = {
          ...editingOrder,
          order_number: formData.orderNumber,
          customer_po_number: formData.customerPONumber,
          customer_id: resolvedCustomerId,
          customer_name: formData.customerName,
          order_date: formData.orderDate,
          required_date: formData.requiredDate,
          total_value_bhd: computedTotalValue,
          status: formData.status,
          payment_terms: formData.paymentTerms,
          delivery_terms: formData.deliveryTerms,
          items: normalizedItems.map((item, index) => ({
            ...item,
            line_number: index + 1,
            total_price: roundMoney(item.quantity * item.unit_price_bhd)
          }))
        };
        await UpdateOrder(editingOrder.id, updatedOrder as any);
        toast.success('Order updated successfully');
      }

      showModal = false;
      await Promise.all([loadOrders(), loadOrdersWithNoItems()]);
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger('Failed to save order: ' + errorMsg);
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
      orderItems = sanitizeFormItems(
        (fullOrder.items || []).map((item) => ({ ...item } as EditableOrderItem)),
        fullOrder.total_value_bhd || fullOrder.grand_total_bhd || 0,
      );
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

  function handleOrderItemKeydown(event: KeyboardEvent, row: main.Order) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      void handleRowClick(row);
    }
  }

  // Quick actions
  async function handleCreateDeliveryNote(order: main.Order) {
    if (isProcessingAction) return;

    if (!orderItems || orderItems.length === 0) {
      toast.warning('This order has no saved line items. Delivery notes need the actual items to ship; edit or re-import the order before creating a DN.');
      return;
    }

    // Set pending DN creation in store (DeliveryNotesScreen will consume on mount)
    pendingDNCreate.request(order.id, order.order_number || '', order.customer_name || '');
    showDetailModal = false;
    // Navigate to operations screen with delivery-notes tab
    window.dispatchEvent(new CustomEvent('navigateToScreen', {
      detail: { screen: 'operations', tab: 'delivery-notes' }
    }));
  }

  async function handleCreateInvoice(order: main.Order) {
    if (isProcessingAction) return;

    const confirmed = await askActionConfirmation({
      title: 'Create Invoice',
      message: `Create invoice for order ${order.order_number}? This will generate a billing record for ${order.customer_name}.`,
      confirmLabel: 'Create Invoice',
      variant: 'primary'
    });
    if (!confirmed) {
      return;
    }

    isProcessingAction = true;
    try {
      const invoice = await CreateInvoiceFromOrder(order.id);
      toast.success(`Invoice ${invoice.invoice_number} created`);
      showDetailModal = false;
      window.dispatchEvent(new CustomEvent('navigateToScreen', {
        detail: { screen: 'finance', tab: 'invoices', company: order.division || 'PH Trading' }
      }));
      window.dispatchEvent(new CustomEvent('finance:navigate', {
        detail: { tab: 'invoices', company: order.division || 'PH Trading' }
      }));
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to create invoice: ' + errorMessage(err));
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleCreatePurchaseOrder(order: main.Order) {
    if (isProcessingAction) return;

    if (!orderItems || orderItems.length === 0) {
      toast.warning('This order has no saved line items. Supplier orders need item detail; edit or re-import the order before creating a PO.');
      return;
    }

    const confirmed = await askActionConfirmation({
      title: 'Create Supplier Order',
      message: `Create a draft supplier order from ${order.order_number}? If the order contains items from multiple suppliers, create supplier orders separately.`,
      confirmLabel: 'Create Supplier Order',
      variant: 'secondary'
    });
    if (!confirmed) {
      return;
    }

    isProcessingAction = true;
    try {
      const po = await CreatePOFromOrder(order.id, '', []);
      toast.success(`Supplier order ${po.po_number} created`);
      showDetailModal = false;
      window.dispatchEvent(new CustomEvent('navigateToScreen', {
        detail: { screen: 'operations', tab: 'pos' }
      }));
    } catch (err) {
      toast.danger('Failed to create supplier order: ' + errorMessage(err));
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleCreateProforma(order: main.Order) {
    if (isProcessingAction) return;

    const confirmed = await askActionConfirmation({
      title: 'Create Proforma',
      message: `Create proforma invoice for order ${order.order_number}? This is a reference document, not a tax invoice.`,
      confirmLabel: 'Create Proforma',
      variant: 'secondary'
    });
    if (!confirmed) {
      return;
    }

    isProcessingAction = true;
    try {
      const invoice = await CreateProformaInvoice(order.id);
      toast.success(`Proforma Invoice ${invoice.invoice_number} created`);
      showDetailModal = false;
      window.dispatchEvent(new CustomEvent('navigateToScreen', {
        detail: { screen: 'finance', tab: 'invoices', company: order.division || 'PH Trading' }
      }));
      window.dispatchEvent(new CustomEvent('finance:navigate', {
        detail: { tab: 'invoices', company: order.division || 'PH Trading' }
      }));
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to create proforma: ' + errorMessage(err));
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleCreatePO(order: main.Order) {
    if (isProcessingAction) return;

    if (!orderItems || orderItems.length === 0) {
      toast.warning('This order has no saved line items. Supplier orders need item detail; edit or re-import the order before creating a PO.');
      return;
    }

    const confirmed = await askActionConfirmation({
      title: 'Create Supplier Order',
      message: `Create draft supplier order(s) from ${order.order_number}? Items will be split by supplier when the order contains multiple suppliers.`,
      confirmLabel: 'Create Supplier Order',
      variant: 'secondary'
    });
    if (!confirmed) {
      return;
    }

    isProcessingAction = true;
    try {
      const selectedItemIDs = orderItems
        .map((item) => item.id)
        .filter((id): id is string => Boolean(id));
      const pos = await CreatePOsFromOrder(order.id, selectedItemIDs);
      if (pos.length === 1) {
        toast.success(`Supplier order ${pos[0].po_number} created`);
      } else {
        toast.success(`${pos.length} supplier orders created`);
      }
      showDetailModal = false;
      window.dispatchEvent(new CustomEvent('navigateToScreen', {
        detail: { screen: 'operations', tab: 'pos' }
      }));
    } catch (err) {
      toast.danger('Failed to create supplier order: ' + errorMessage(err));
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleQuickMarkDelivered() {
    if (isProcessingAction) return;
    if (!selectedOrder) return;

    const confirmed = await askActionConfirmation({
      title: 'Mark Delivered',
      message: `Mark order ${selectedOrder.order_number} as fully delivered? This updates the order status without creating a GRN record.`,
      confirmLabel: 'Mark Delivered',
      variant: 'primary'
    });
    if (!confirmed) {
      return;
    }

    isProcessingAction = true;
    try {
      const message = await QuickMarkOrderDelivered(selectedOrder.id);
      toast.success(message);
      showDetailModal = false;
      await loadOrders();
    } catch (err) {
      toast.danger('Failed to mark as delivered: ' + errorMessage(err));
    } finally {
      isProcessingAction = false;
    }
  }

  async function handleDeleteOrder(order: main.Order) {
    if (isDeleting) return;

    let deletePreview: any = null;
    try {
      deletePreview = await PreviewOrderDeleteCascade(order.id);
    } catch (err) {
      toast.danger('Could not inspect linked records: ' + errorMessage(err));
      return;
    }

    if (deletePreview?.blocked) {
      toast.danger(deletePreview.block_reason || 'This order has protected linked records and cannot be deleted.');
      return;
    }

    const summary = Array.isArray(deletePreview?.summary) && deletePreview.summary.length > 0
      ? deletePreview.summary.map((item: string) => `- ${item}`).join('\n')
      : '- order record';

    const confirmed = await askActionConfirmation({
      title: 'Delete Order',
      message: `Delete order ${order.order_number}? This cannot be undone.\n\nThis will also delete:\n${summary}`,
      confirmLabel: 'Delete Order',
      variant: 'danger'
    });
    if (!confirmed) {
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
      toast.danger('Delete failed: ' + errorMessage(err));
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
    loadOrdersWithNoItems();
  });

  async function loadOrdersWithNoItems() {
    try {
      const data = await GetOrdersWithNoItems();
      ordersWithNoItems = data || [];
    } catch (err) {
      // Non-critical — silently ignore
      ordersWithNoItems = [];
    }
  }
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
      <Button variant="secondary" on:click={exportOrdersExtract} disabled={exportingExtract || loading}>
        {exportingExtract ? 'Exporting...' : 'Extract'}
      </Button>
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
        {@const yearOrders = filteredOrders.filter(o => {
          const orderDate = getOrderDateValue(o.order_date);
          return orderDate ? orderDate.getFullYear().toString() === year : false;
        })}
        {#if yearOrders.length > 0}
          <div class="year-group">
            <div class="year-header">
              <span class="year-label">{year}</span>
              <span class="year-count">{yearOrders.length} orders</span>
              <span class="year-value">{formatCurrency(yearOrders.reduce((sum, o) => sum + (o.total_value_bhd || 0), 0))}</span>
            </div>
            <div class="orders-list">
              {#each yearOrders as order}
                <div
                  class="order-item"
                  role="button"
                  tabindex="0"
                  aria-label={`Open order ${order.order_number}${order.customer_name ? ` for ${order.customer_name}` : ''}`}
                  on:click={() => handleRowClick(order)}
                  on:keydown={(event) => handleOrderItemKeydown(event, order)}
                >
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
              <div
                class="order-item"
                role="button"
                tabindex="0"
                aria-label={`Open order ${order.order_number}${order.customer_name ? ` for ${order.customer_name}` : ''}`}
                on:click={() => handleRowClick(order)}
                on:keydown={(event) => handleOrderItemKeydown(event, order)}
              >
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

    <!-- Warning: Orders with no line items -->
    {#if ordersWithNoItems.length > 0}
      <div class="no-items-warning">
        <span class="warning-icon">&#9888;</span>
        <div class="warning-content">
          <strong>{ordersWithNoItems.length} order{ordersWithNoItems.length === 1 ? '' : 's'} ha{ordersWithNoItems.length === 1 ? 's' : 've'} no line items.</strong>
          Edit these orders to add items before invoicing.
          <div class="warning-list">
            {#each ordersWithNoItems as item}
              <span class="warning-order-number">{escapeHtml(item.order_number || item.id)}</span>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    {#if zeroValueOrders.length > 0}
      <div class="no-items-warning no-items-warning-critical">
        <span class="warning-icon">&#9888;</span>
        <div class="warning-content">
          <strong>{zeroValueOrders.length} order{zeroValueOrders.length === 1 ? '' : 's'} currently have zero total value.</strong>
          Review the line items and unit prices before using them for delivery, invoicing, or procurement.
          <div class="warning-list">
            {#each zeroValueOrders.slice(0, 10) as item}
              <span class="warning-order-number">{escapeHtml(item.order_number || item.id)}</span>
            {/each}
          </div>
        </div>
      </div>
    {/if}

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
            {status.label} ({orders.filter(o => orderMatchesStatus(o, status.key)).length})
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
      <Button variant="secondary" on:click={exportOrdersExtract} disabled={exportingExtract || loading}>
        {exportingExtract ? 'Exporting...' : 'Extract Orders'}
      </Button>
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
    size="xl"
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
            readonly
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

      <div class="line-items-panel">
          <div class="line-items-panel-header">
            <div>
              <h4>Order Line Items</h4>
              <p>{modalMode === 'edit' ? 'Update each line separately so order totals, delivery notes, and invoices stay accurate.' : 'Capture each line separately so delivery notes, invoices, and purchase orders stay accurate.'}</p>
            </div>
            <Button variant="ghost" type="button" on:click={addFormItem}>
              + Add Line
            </Button>
          </div>
          <Button variant="ghost" type="button" on:click={addFormItem}>
            + Add Line
          </Button>
        </div>

        {#if getLineItemIssues(formItems).missingItems}
          <div class="line-item-alert">
            This order has no line items yet. Add them now so downstream documents carry the right quantities and values.
          </div>
        {:else if getLineItemIssues(formItems).zeroPriceItems > 0}
          <div class="line-item-alert">
            {getLineItemIssues(formItems).zeroPriceItems} line item{getLineItemIssues(formItems).zeroPriceItems === 1 ? '' : 's'} still have zero pricing. Please complete them before saving.
          </div>
        {/if}

        <div class="line-items-list">
          {#each formItems as item, index}
            <div class="line-item-editor">
              <div class="line-item-editor-head">
                <span>Line {index + 1}</span>
                <button
                  type="button"
                  class="line-remove-btn"
                  on:click={() => removeFormItem(index)}
                  disabled={formItems.length <= 1}
                  aria-label="Remove order line"
                >
                  Remove
                </button>
              </div>

              <div class="line-item-top-row">
                <label class="line-field">
                  <span>Code</span>
                  <input
                    class="form-input compact"
                    bind:value={formItems[index].product_code}
                    placeholder="Code"
                    on:blur={recalculateFormItems}
                  />
                </label>
                <label class="line-field description-field">
                  <span>Description</span>
                  <input
                    class="form-input compact"
                    bind:value={formItems[index].description}
                    placeholder="Description"
                    on:blur={recalculateFormItems}
                  />
                </label>
              </div>

              <div class="line-item-bottom-row">
                <label class="line-field">
                  <span>Quantity</span>
                  <input
                    class="form-input compact number-input"
                    type="number"
                    min="0"
                    step="0.001"
                    bind:value={formItems[index].quantity}
                    on:input={recalculateFormItems}
                  />
                </label>
                <label class="line-field">
                  <span>Unit Price</span>
                  <input
                    class="form-input compact number-input"
                    type="number"
                    min="0"
                    step="0.001"
                    bind:value={formItems[index].unit_price_bhd}
                    on:input={recalculateFormItems}
                  />
                </label>
                <div class="line-total-cell">
                  <span>Line Total</span>
                  <strong>{formatCurrency(roundMoney((item.quantity || 0) * (item.unit_price_bhd || 0)))}</strong>
                </div>
              </div>
            </div>
          {/each}
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
    on:close={() => { showDetailModal = false; showTaskModal = false; }}
    size="lg"
  >
    <div class="order-detail">
      {#if getLineItemIssues(orderItems).missingItems || getLineItemIssues(orderItems).zeroPriceItems > 0 || (selectedOrder.total_value_bhd || 0) <= 0}
        <div class="line-item-alert detail-alert">
          {#if getLineItemIssues(orderItems).missingItems}
            This order does not have any saved line items yet.
          {:else if getLineItemIssues(orderItems).zeroPriceItems > 0}
            {getLineItemIssues(orderItems).zeroPriceItems} line item{getLineItemIssues(orderItems).zeroPriceItems === 1 ? '' : 's'} still have zero pricing.
          {:else}
            This order total is zero and should be reviewed.
          {/if}
          Use <strong>Edit Order</strong> to correct the commercial data before creating downstream documents.
        </div>
      {/if}

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
            <span class="chain-label">RFQ / Enquiry</span>
            <span class="chain-value">{selectedOrder.customer_reference || selectedOrder.rfq_id || 'N/A'}</span>
          </div>
          <span class="chain-arrow">></span>
          <div class="chain-link">
            <span class="chain-label">Offer</span>
            <span class="chain-value">{selectedOrder.offer_number || 'N/A'}</span>
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
                  <td class="number-col">{formatCurrency(item.total_price || (item.quantity * item.unit_price_bhd))}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Quick Actions (conditional on order state) -->
      <div class="quick-actions">
        {#if selectedOrder.status !== 'Cancelled' && (!fulfillmentStatus || fulfillmentStatus?.fulfillment_pct < 1.0)}
          <Button variant="secondary" on:click={() => handleCreateDeliveryNote(selectedOrder)} disabled={isProcessingAction}>
            Create Delivery Note
          </Button>
          <Button variant="secondary" on:click={() => handleCreatePurchaseOrder(selectedOrder)} disabled={isProcessingAction}>
            Create Supplier Order
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
          <Button variant="ghost" on:click={() => handleCreateProforma(selectedOrder)} disabled={isProcessingAction}>
            Proforma
          </Button>
        {/if}
        <Button variant="secondary" on:click={() => showTaskModal = true} disabled={isProcessingAction}>
          Create Task
        </Button>
        <Button variant="ghost" on:click={() => void openEditModal(selectedOrder)} disabled={isProcessingAction || isDeleting}>
          Edit Order
        </Button>
        <Button variant="ghost" on:click={() => handleDeleteOrder(selectedOrder)} disabled={isDeleting || isProcessingAction} style="color: #EF4444;">
          Delete Order
        </Button>
      </div>
    </div>
  </Modal>
{/if}

{#if confirmDialogOpen}
  <Modal
    title={confirmDialogTitle}
    open={confirmDialogOpen}
    on:close={() => resolveActionConfirmation(false)}
    size="sm"
  >
    <div class="confirm-dialog">
      <p>{confirmDialogMessage}</p>
      <div class="confirm-actions">
        <Button variant="secondary" on:click={() => resolveActionConfirmation(false)}>
          Cancel
        </Button>
        <Button variant={confirmDialogVariant} on:click={() => resolveActionConfirmation(true)}>
          {confirmDialogConfirmLabel}
        </Button>
      </div>
    </div>
  </Modal>
{/if}

{#if selectedOrder}
  <ContextTaskModal
    open={showTaskModal}
    title="Create Order Task"
    subtitle={`Link work to order ${selectedOrder.order_number}`}
    defaults={{
      customer_id: selectedOrder.customer_id,
      order_id: selectedOrder.id,
      seed_title: `Order task: ${selectedOrder.order_number}`,
    }}
    on:close={() => showTaskModal = false}
    on:created={() => showTaskModal = false}
  />
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

  .order-item:focus-visible {
    outline: 2px solid var(--primary, #4f46e5);
    outline-offset: 2px;
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

  .line-items-panel {
    margin-top: 20px;
    padding: 16px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--bg-subtle);
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .line-items-panel-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .line-items-panel-header h4 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .line-items-panel-header p {
    margin: 4px 0 0;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .line-items-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .line-item-editor {
    background: var(--surface);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .line-item-editor-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .line-item-top-row {
    display: grid;
    grid-template-columns: minmax(140px, 0.45fr) minmax(320px, 1fr);
    gap: 12px;
  }

  .line-item-bottom-row {
    display: grid;
    grid-template-columns: minmax(120px, 0.35fr) minmax(150px, 0.45fr) minmax(160px, 0.5fr);
    gap: 12px;
    align-items: end;
  }

  .line-field {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }

  .line-field span,
  .line-total-cell span {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .compact {
    min-width: 0;
  }

  .number-input {
    text-align: right;
    font-family: var(--font-mono);
  }

  .line-total-cell {
    min-height: 36px;
    padding: 8px 10px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--bg-subtle);
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    text-align: right;
    font-size: 12px;
    font-family: var(--font-mono);
    color: var(--text-primary);
  }

  .line-total-cell strong {
    font-size: 13px;
    font-weight: 700;
  }

  .line-remove-btn {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    border-radius: var(--radius-sm);
    padding: 8px 10px;
    font-size: 12px;
    cursor: pointer;
  }

  .line-remove-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  @media (max-width: 840px) {
    .form-row,
    .line-item-top-row,
    .line-item-bottom-row {
      grid-template-columns: 1fr;
    }

    .line-total-cell {
      text-align: left;
    }
  }

  .line-item-alert {
    padding: 10px 12px;
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.28);
    border-radius: var(--radius-sm);
    color: #92400e;
    font-size: 12px;
    line-height: 1.5;
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
    flex-wrap: wrap;
  }

  .confirm-dialog {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .confirm-dialog p {
    margin: 0;
    color: var(--text-primary);
    font-size: 14px;
    line-height: 1.5;
    white-space: pre-line;
  }

  .confirm-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  /* Warning banner for orders with no line items */
  .no-items-warning {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    background: #fef9e7;
    border: 1px solid #f0d050;
    border-radius: 8px;
    margin-bottom: 12px;
    font-size: 13px;
    color: #7a6200;
  }

  .no-items-warning-critical {
    background: rgba(239, 68, 68, 0.08);
    border-color: rgba(239, 68, 68, 0.24);
    color: #991b1b;
  }

  .no-items-warning .warning-icon {
    font-size: 18px;
    flex-shrink: 0;
    line-height: 1.4;
  }

  .no-items-warning .warning-content {
    flex: 1;
    line-height: 1.5;
  }

  .no-items-warning .warning-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 6px;
  }

  .no-items-warning .warning-order-number {
    display: inline-block;
    padding: 2px 8px;
    background: rgba(240, 208, 80, 0.3);
    border-radius: 4px;
    font-weight: 600;
    font-size: 12px;
    cursor: default;
  }

  .detail-alert {
    margin-top: -4px;
  }

</style>
