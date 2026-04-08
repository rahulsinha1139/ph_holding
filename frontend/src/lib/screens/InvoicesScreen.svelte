<script lang="ts">
  /**
   * InvoicesScreen - Real Customer Invoice Management
   *
   * Features:
   * - List all customer invoices from database
   * - Create invoices from unfulfilled orders
   * - Send invoices (mark as Sent)
   * - Delete invoices with confirmation
   * - Filter by status: Draft, Sent, Paid, Overdue, PartiallyPaid
   * - Search by invoice number or customer name
   * - KPI dashboard: Total Revenue, Outstanding, Overdue count
   * - BHD currency with 3 decimal places
   *
   * Design System: Wabi-Sabi minimalism × Bloomberg data density
   */

  import { createEventDispatcher, onMount } from 'svelte';
  import {
    ListCustomerInvoices,
    CreateInvoiceWithOptions,
    SendCustomerInvoice,
    DeleteCustomerInvoice,
    ListOrders,
    UpdateCustomerInvoice,
    GenerateInvoicePDF,
    GetCustomerInvoiceByID,
    GetAvailableDeliveryNotesForOrder,
    ListCreditNotes,
    CreateCreditNote,
    ApplyCreditNote,
    GenerateCreditNotePDF,
  } from '../../../wailsjs/go/main/App';
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Modal from '$lib/components/layout/Modal.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import { toast } from '$lib/stores/toasts';
  import { devLog } from '$lib/utils/devLog';
  import { escapeHtml } from '$lib/utils/escapeHtml';
  import { debounce } from '$lib/utils/debounce';

  const dispatch = createEventDispatcher();

  // Props
  export let embedded = false;

  // Types
  type StatusFilter = 'All' | 'Draft' | 'Sent' | 'Paid' | 'Overdue' | 'PartiallyPaid';

  interface InvoiceItem {
    id: string;
    description: string;
    quantity: number;
    rate: number;
    total_bhd: number;
    line_number?: number;
    product_code?: string;
    equipment?: string;
    model?: string;
    specification?: string;
    detailed_description?: string;
  }

  interface Invoice {
    id: string;
    invoice_number: string;
    invoice_date: any; // time.Time from Go
    customer_id: string;
    customer_name: string;
    order_id?: string;
    customer_po_number: string;
    grand_total_bhd: number;
    subtotal_bhd: number;
    outstanding_bhd: number;
    status: string;
    due_date: any; // time.Time from Go
    delivery_note_id?: string;
    delivery_note_number?: string;
    items?: InvoiceItem[];  // Line items from Preload
    rfq_id?: string;
    offer_id?: string;
    offer_number?: string;
    customer_reference?: string;
    attention_person?: string;
    attention_company?: string;
    delivery_weeks?: string;
    country_of_origin?: string;
    payment_terms?: string;
    delivery_terms?: string;
  }

  interface Order {
    id: string;
    order_number: string;
    customer_name: string;
    customer_po_number: string;
    order_date: any;
    grand_total_bhd: number;
    status: string;
  }

  interface DeliveryNote {
    id: string;
    dn_number: string;
    delivery_date: any;
    status: string;
    order_id: string;
  }

  // State
  let invoices: Invoice[] = [];
  let filteredInvoices: Invoice[] = [];
  let orders: Order[] = [];
  let loading = true;
  let selectedFilter: StatusFilter = 'All';
  let searchQuery = '';

  // Pagination state
  const PAGE_SIZE = 50;
  let currentPage = 0;
  let hasMore = true;
  let loadingMore = false;
  let totalLoaded = 0;

  // Debounced search state
  let debouncedQuery = '';

  // Debounced search update (300ms delay to prevent excessive filtering)
  const updateDebouncedQuery = debounce((value: string) => {
    debouncedQuery = value;
  }, 300);

  // Watch searchQuery changes and debounce
  $: updateDebouncedQuery(searchQuery);

  // Modal state
  let showCreateModal = false;
  let showDeleteModal = false;
  let createLoading = false;
  let deleteLoading = false;
  let selectedOrderId = '';
  let invoiceToDelete: Invoice | null = null;

  // Delivery Note linking state
  let availableDeliveryNotes: DeliveryNote[] = [];
  let selectedDeliveryNoteId = '';
  let loadingDeliveryNotes = false;

  // Phase 23: Credit Notes state
  type MainView = 'invoices' | 'credit-notes';
  let mainView: MainView = 'invoices';
  let creditNotes: any[] = [];
  let loadingCreditNotes = false;
  let showCNModal = false;
  let cnInvoiceId = '';
  let cnReason = '';
  let cnItems: { description: string; quantity: number; rate: number }[] = [{ description: '', quantity: 1, rate: 0 }];
  let cnSaving = false;

  async function loadCreditNotes() {
    loadingCreditNotes = true;
    try {
      const data = await ListCreditNotes(100, 0);
      creditNotes = data || [];
    } catch (err) {
      console.error('Failed to load credit notes:', err);
      creditNotes = [];
    } finally {
      loadingCreditNotes = false;
    }
  }

  function openCNModal(invoiceId?: string) {
    cnInvoiceId = invoiceId || '';
    cnReason = '';
    cnItems = [{ description: '', quantity: 1, rate: 0 }];
    showCNModal = true;
  }

  function addCNItem() {
    cnItems = [...cnItems, { description: '', quantity: 1, rate: 0 }];
  }

  function removeCNItem(index: number) {
    if (cnItems.length <= 1) return;
    cnItems = cnItems.filter((_, i) => i !== index);
  }

  async function handleCreateCN() {
    if (!cnInvoiceId) { toast.warning('Select an invoice'); return; }
    if (!cnReason.trim()) { toast.warning('Reason is required'); return; }
    const validItems = cnItems.filter(i => i.description.trim() && i.quantity > 0 && i.rate > 0);
    if (validItems.length === 0) { toast.warning('At least one valid item required'); return; }

    cnSaving = true;
    try {
      await CreateCreditNote(cnInvoiceId, cnReason, validItems);
      toast.success('Credit Note created');
      showCNModal = false;
      await loadCreditNotes();
    } catch (err) {
      toast.danger('Failed: ' + (err as Error).message);
    } finally {
      cnSaving = false;
    }
  }

  async function handleApplyCN(cnId: string) {
    try {
      await ApplyCreditNote(cnId);
      toast.success('Credit Note applied — invoice outstanding updated');
      await Promise.all([loadCreditNotes(), loadInvoices()]);
    } catch (err) {
      toast.danger('Failed: ' + (err as Error).message);
    }
  }

  async function handleCNPDF(cnId: string) {
    try {
      const path = await GenerateCreditNotePDF(cnId);
      toast.success('Credit Note PDF saved');
    } catch (err) {
      toast.danger('Failed: ' + (err as Error).message);
    }
  }

  // Field visibility options (user can select what appears on the invoice)
  const defaultFieldVisibility = {
    show_equipment: true,        // Equipment/Product name
    show_specification: true,    // Short specification
    show_detailed_desc: true,    // Full detailed description
    show_fob: false,             // FOB cost
    show_freight: false,         // Freight cost
    show_cost: false,            // Total cost
    show_margin: false,          // Margin %
    show_contact: true,          // Contact person/company
    show_rfq: true,              // RFQ reference
    show_currency: false,        // Source currency
    show_country_origin: true,   // Country of origin
    show_delivery_weeks: true,   // Delivery time
  };
  let fieldVisibility = { ...defaultFieldVisibility };

  // Reset field visibility to defaults (prevents data leak between modal sessions)
  function resetFieldVisibility() {
    fieldVisibility = { ...defaultFieldVisibility };
  }

  // Edit state
  let showEditModal = false;
  let editLoading = false;
  let editInvoice: any = null;
  let pdfLoadingMap: Record<string, boolean> = {};

  // P1-1 FIX: Track which invoice is being sent to prevent double-clicks
  let sendingInvoiceId: string | null = null;

  // Detail modal state
  let showDetailModal = false;
  let selectedInvoice: Invoice | null = null;

  async function openDetailModal(invoice: Invoice) {
    console.log('[InvoicesScreen] Opening detail modal, invoice items:', invoice.items);
    selectedInvoice = invoice;
    showDetailModal = true;
    try {
      selectedInvoice = await GetCustomerInvoiceByID(invoice.id) as Invoice;
    } catch {
      selectedInvoice = invoice;
    }
  }

  // Status color mapping
  const statusColors: Record<string, string> = {
    Draft: '#6B7280',
    Sent: '#3B82F6',
    Paid: '#10B981',
    Overdue: '#EF4444',
    PartiallyPaid: '#F59E0B',
  };

  function openEditModal(invoice: Invoice) {
    editInvoice = { ...invoice };
    showEditModal = true;
  }

  async function saveInvoiceEdit() {
    if (!editInvoice) return;

    // Validate amounts > 0
    if (editInvoice.grand_total_bhd <= 0) {
      toast.warning('Invoice amount must be greater than 0');
      return;
    }
    if (editInvoice.outstanding_bhd < 0) {
      toast.warning('Outstanding amount cannot be negative');
      return;
    }
    if (editInvoice.outstanding_bhd > editInvoice.grand_total_bhd) {
      toast.warning('Outstanding amount cannot exceed invoice total');
      return;
    }

    editLoading = true;
    try {
      await UpdateCustomerInvoice(editInvoice);
      toast.success('Invoice updated successfully');
      showEditModal = false;
      editInvoice = null;
      await loadInvoices();
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to update invoice: ${errorMsg}`);
    } finally {
      editLoading = false;
    }
  }

  async function downloadPDF(invoice: Invoice) {
    if (pdfLoadingMap[invoice.id]) return;
    pdfLoadingMap[invoice.id] = true;
    pdfLoadingMap = { ...pdfLoadingMap };
    try {
      const path = await GenerateInvoicePDF(invoice.id);
      toast.success(`PDF saved: ${path}`);
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to generate PDF: ${errorMsg}`);
    } finally {
      pdfLoadingMap[invoice.id] = false;
      pdfLoadingMap = { ...pdfLoadingMap };
    }
  }

  // DataTable columns configuration
  const columns = [
    {
      key: 'invoice_number',
      label: 'Invoice #',
      sortable: true,
      width: '140px',
      render: (row: Invoice) => {
        return `<span style="font-family: var(--font-mono); font-size: 13px; color: var(--brand-indigo); font-weight: 600;">${escapeHtml(row.invoice_number)}</span>`;
      },
    },
    {
      key: 'customer_name',
      label: 'Customer',
      sortable: true,
      render: (row: Invoice) => {
        return `<span style="font-weight: 500;">${escapeHtml(row.customer_name)}</span>`;
      },
    },
    {
      key: 'invoice_date',
      label: 'Date',
      type: 'date' as const,
      sortable: true,
      width: '110px',
      render: (row: Invoice) => {
        return `<span style="font-family: var(--font-mono); font-size: 12px;">${formatDate(row.invoice_date)}</span>`;
      },
    },
    {
      key: 'due_date',
      label: 'Due Date',
      type: 'date' as const,
      sortable: true,
      width: '110px',
      render: (row: Invoice) => {
        const isOverdue = isDateOverdue(row.due_date) && row.outstanding_bhd > 0;
        const color = isOverdue ? '#EF4444' : 'var(--text-primary)';
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: ${color}; font-weight: ${isOverdue ? '600' : '400'};">${formatDate(row.due_date)}</span>`;
      },
    },
    {
      key: 'grand_total_bhd',
      label: 'Amount (BHD)',
      sortable: true,
      width: '140px',
      align: 'right' as const,
      render: (row: Invoice) => {
        return `<span style="font-family: var(--font-mono); font-weight: 600; font-size: 14px;">${formatBHD(row.grand_total_bhd)}</span>`;
      },
    },
    {
      key: 'outstanding_bhd',
      label: 'Outstanding (BHD)',
      sortable: true,
      width: '160px',
      align: 'right' as const,
      render: (row: Invoice) => {
        const color = row.outstanding_bhd > 0 ? '#F59E0B' : '#10B981';
        return `<span style="font-family: var(--font-mono); font-weight: 600; font-size: 14px; color: ${color};">${formatBHD(row.outstanding_bhd)}</span>`;
      },
    },
    {
      key: 'delivery_note_number',
      label: 'DN #',
      sortable: true,
      width: '100px',
      render: (row: Invoice) => {
        if (!row.delivery_note_number) {
          return `<span style="color: var(--text-secondary); font-size: 12px;">—</span>`;
        }
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: #059669; font-weight: 500;">${escapeHtml(row.delivery_note_number)}</span>`;
      },
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      width: '110px',
      render: (row: Invoice) => {
        const color = statusColors[row.status] || '#6B7280';
        return `<span style="display: inline-block; padding: 4px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; background: ${color}15; color: ${color};">${row.status}</span>`;
      },
    },
    {
      key: 'actions',
      label: 'Actions',
      width: '140px',
      align: 'center' as const,
      render: (row: Invoice) => {
        return ''; // Handled by slot
      },
    },
  ];

  // Filter tabs
  const filterTabs: { value: StatusFilter; label: string; count: number }[] = [
    { value: 'All', label: 'All', count: 0 },
    { value: 'Draft', label: 'Draft', count: 0 },
    { value: 'Sent', label: 'Sent', count: 0 },
    { value: 'Paid', label: 'Paid', count: 0 },
    { value: 'Overdue', label: 'Overdue', count: 0 },
    { value: 'PartiallyPaid', label: 'Partial', count: 0 },
  ];

  // Computed: Update tab counts
  $: {
    filterTabs[0].count = invoices.length;
    filterTabs[1].count = invoices.filter((i) => i.status === 'Draft').length;
    filterTabs[2].count = invoices.filter((i) => i.status === 'Sent').length;
    filterTabs[3].count = invoices.filter((i) => i.status === 'Paid').length;
    filterTabs[4].count = invoices.filter((i) => i.status === 'Overdue').length;
    filterTabs[5].count = invoices.filter((i) => i.status === 'PartiallyPaid').length;
  }

  // Computed: Filter and search (uses debouncedQuery for performance)
  $: {
    let result = [...invoices];

    // Status filter
    if (selectedFilter !== 'All') {
      result = result.filter((i) => i.status === selectedFilter);
    }

    // Search filter (debounced to prevent excessive re-renders)
    if (debouncedQuery) {
      const q = debouncedQuery.toLowerCase();
      result = result.filter(
        (i) =>
          i.invoice_number.toLowerCase().includes(q) ||
          i.customer_name.toLowerCase().includes(q)
      );
    }

    filteredInvoices = result.sort((a, b) => {
      const dateA = parseGoDate(a.invoice_date);
      const dateB = parseGoDate(b.invoice_date);
      return dateB.getTime() - dateA.getTime();
    });
  }

  // Computed: KPI stats
  $: stats = {
    totalRevenueYTD: invoices
      .filter((invoice) => parseGoDate(invoice.invoice_date).getFullYear() === new Date().getFullYear())
      .reduce((sum, i) => sum + i.grand_total_bhd, 0),
    outstanding: invoices.reduce((sum, i) => sum + i.outstanding_bhd, 0),
    overdueCount: invoices.filter(
      (i) => i.status === 'Overdue' && i.outstanding_bhd > 0
    ).length,
  };

  // Currency formatter - BHD with 3 decimal places
  function formatBHD(amount: number): string {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    }).format(Number(amount) || 0);
  }

  function openBankReconciliation() {
    dispatch('navigate', { tab: 'bank_recon', source: 'customer_invoices' });
  }

  // Parse Go time.Time to JS Date
  function parseGoDate(dateValue: any): Date {
    if (!dateValue) return new Date(0);

    // If it's already a Date object
    if (dateValue instanceof Date) return dateValue;

    // If it's a string (ISO format)
    if (typeof dateValue === 'string') {
      return new Date(dateValue);
    }

    // If it's a time.Time object (has properties)
    if (typeof dateValue === 'object') {
      // Try to convert to string first
      const str = dateValue.toString?.() || String(dateValue);
      if (str && str !== '[object Object]') {
        return new Date(str);
      }
    }

    return new Date(0);
  }

  // Format date for display
  function formatDate(dateValue: any): string {
    const date = parseGoDate(dateValue);
    if (date.getTime() === 0) return '—';

    return date.toLocaleDateString('en-GB', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
    });
  }

  // Check if date is overdue
  function isDateOverdue(dateValue: any): boolean {
    const date = parseGoDate(dateValue);
    if (date.getTime() === 0) return false;
    return date < new Date();
  }

  // Load invoices from database with pagination
  async function loadInvoices() {
    loading = true;
    currentPage = 0;
    hasMore = true;
    try {
      const data = await ListCustomerInvoices(PAGE_SIZE, 0);
      invoices = data || [];
      currentPage = 1;
      totalLoaded = invoices.length;
      hasMore = invoices.length === PAGE_SIZE;
      // DEBUG: Check if items are loaded
      const withItems = invoices.filter(inv => inv.items && inv.items.length > 0);
      console.log(`[InvoicesScreen] Loaded ${invoices.length} invoices, ${withItems.length} have items`);
      if (invoices[0]) console.log('[InvoicesScreen] First invoice items:', invoices[0].items);
      devLog.info(`Loaded ${invoices.length} invoices (page 1, hasMore: ${hasMore})`);
    } catch (err) {
      toast.danger('Failed to load invoices');
      invoices = [];
      hasMore = false;
    } finally {
      loading = false;
    }
  }

  // Load more invoices (pagination)
  async function loadMore() {
    if (loadingMore || !hasMore) return;

    loadingMore = true;
    try {
      const offset = currentPage * PAGE_SIZE;
      const data = await ListCustomerInvoices(PAGE_SIZE, offset);

      if (data && data.length > 0) {
        invoices = [...invoices, ...data];
        currentPage++;
        totalLoaded = invoices.length;
        hasMore = data.length === PAGE_SIZE;
        devLog.info(`Loaded ${data.length} more invoices (total: ${totalLoaded}, hasMore: ${hasMore})`);
      } else {
        hasMore = false;
      }
    } catch (err) {
      console.error('Failed to load more:', err);
      toast.danger('Failed to load more invoices');
    } finally {
      loadingMore = false;
    }
  }

  // Load unfulfilled orders for invoice creation
  async function loadUnfulfilledOrders() {
    try {
      const allOrders = await ListOrders(500, 0);
      orders = (allOrders || []).filter(
        (order) => order.status !== 'Complete' && order.status !== 'Invoiced'
      );
      devLog.info(`Loaded ${orders.length} unfulfilled orders`);
    } catch (err) {
      toast.danger('Failed to load orders');
      orders = [];
    }
  }

  // Open create invoice modal
  async function openCreateModal() {
    await loadUnfulfilledOrders();

    if (orders.length === 0) {
      toast.warning('No unfulfilled orders available. All orders are complete or invoiced.');
      return;
    }

    selectedOrderId = '';
    selectedDeliveryNoteId = '';
    availableDeliveryNotes = [];
    showCreateModal = true;
  }

  // Load delivery notes when order is selected
  async function loadDeliveryNotesForOrder(orderId: string) {
    if (!orderId) {
      availableDeliveryNotes = [];
      selectedDeliveryNoteId = '';
      return;
    }

    loadingDeliveryNotes = true;
    try {
      const dns = await GetAvailableDeliveryNotesForOrder(orderId);
      availableDeliveryNotes = dns || [];
      selectedDeliveryNoteId = '';
      devLog.info(`Loaded ${availableDeliveryNotes.length} available DNs for order ${orderId}`);
    } catch (err) {
      availableDeliveryNotes = [];
    } finally {
      loadingDeliveryNotes = false;
    }
  }

  // React to order selection changes
  $: if (showCreateModal && selectedOrderId) {
    loadDeliveryNotesForOrder(selectedOrderId);
  }

  // Create invoice from order (with optional DN linkage and field visibility)
  async function handleCreateInvoice() {
    if (!selectedOrderId) {
      toast.warning('Please select an order');
      return;
    }

    createLoading = true;
    try {
      const visibilityJSON = JSON.stringify(fieldVisibility);

      const newInvoice = await CreateInvoiceWithOptions(
        selectedOrderId,
        selectedDeliveryNoteId || '',
        visibilityJSON
      );

      let successMsg = `Invoice ${newInvoice.invoice_number} created`;
      if (selectedDeliveryNoteId) {
        const dnNumber = availableDeliveryNotes.find(dn => dn.id === selectedDeliveryNoteId)?.dn_number || '';
        successMsg += ` and linked to DN ${dnNumber}`;
      }
      toast.success(successMsg);

      showCreateModal = false;
      resetFieldVisibility();
      await loadInvoices();
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to create invoice: ${errorMsg}`);
    } finally {
      createLoading = false;
    }
  }

  // Send invoice (mark as Sent)
  async function handleSendInvoice(invoice: Invoice) {
    if (invoice.status === 'Sent') {
      toast.info('Invoice already sent');
      return;
    }

    // P1-1 FIX: Prevent double-clicks
    if (sendingInvoiceId === invoice.id) return;
    sendingInvoiceId = invoice.id;

    try {
      await SendCustomerInvoice(invoice.id);
      toast.success(`Invoice ${invoice.invoice_number} marked as sent`);
      await loadInvoices();
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to send invoice: ${errorMsg}`);
    } finally {
      sendingInvoiceId = null;
    }
  }

  // Open delete confirmation
  function openDeleteModal(invoice: Invoice) {
    invoiceToDelete = invoice;
    showDeleteModal = true;
  }

  // Delete invoice
  async function handleDeleteInvoice() {
    if (!invoiceToDelete) return;

    deleteLoading = true;
    try {
      await DeleteCustomerInvoice(invoiceToDelete.id);
      toast.success(`Invoice ${invoiceToDelete.invoice_number} deleted`);
      showDeleteModal = false;
      invoiceToDelete = null;
      await loadInvoices();
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to delete invoice: ${errorMsg}`);
    } finally {
      deleteLoading = false;
    }
  }

  onMount(() => {
    loadInvoices();
    loadCreditNotes();
  });
</script>

<PageLayout title="Invoices" subtitle="Customer billing and receivables management" {embedded}>
  <svelte:fragment slot="header-actions">
    {#if mainView === 'invoices'}
      <Button variant="secondary" on:click={loadInvoices}>Refresh</Button>
      <Button variant="secondary" on:click={openBankReconciliation}>Open Bank Recon</Button>
      <Button variant="primary" on:click={openCreateModal}>+ Create Invoice</Button>
    {:else}
      <Button variant="secondary" on:click={loadCreditNotes}>Refresh</Button>
      <Button variant="secondary" on:click={openBankReconciliation}>Open Bank Recon</Button>
      <Button variant="primary" on:click={() => openCNModal()}>+ Issue Credit Note</Button>
    {/if}
  </svelte:fragment>

  <div class="invoices-container">
    <!-- Phase 23: Main View Tabs -->
    <div class="main-view-tabs">
      <button class="main-tab" class:main-tab-active={mainView === 'invoices'} on:click={() => mainView = 'invoices'}>
        Invoices <span class="main-tab-count">{invoices.length}</span>
      </button>
      <button class="main-tab" class:main-tab-active={mainView === 'credit-notes'} on:click={() => mainView = 'credit-notes'}>
        Credit Notes <span class="main-tab-count">{creditNotes.length}</span>
      </button>
    </div>

  {#if mainView === 'invoices'}
    <!-- KPI Stats -->
    <div class="kpi-grid">
      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Total Revenue (YTD)</div>
          <div class="kpi-value kpi-primary">
            {formatBHD(stats.totalRevenueYTD)} <span class="kpi-currency">BHD</span>
          </div>
          <div class="kpi-sublabel">{new Date().getFullYear()} billings</div>
        </div>
      </Card>

      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Outstanding</div>
          <div class="kpi-value" class:kpi-warning={stats.outstanding > 0}>
            {formatBHD(stats.outstanding)} <span class="kpi-currency">BHD</span>
          </div>
          <div class="kpi-sublabel">Unpaid receivables</div>
        </div>
      </Card>

      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Overdue</div>
          <div class="kpi-value" class:kpi-danger={stats.overdueCount > 0}>
            {stats.overdueCount}
          </div>
          <div class="kpi-sublabel">
            {stats.overdueCount === 1 ? 'Invoice' : 'Invoices'} past due
          </div>
        </div>
      </Card>
    </div>

    <!-- Filter Tabs and Search -->
    <div class="controls-row">
      <Card padding="sm" style="flex: 1;">
        <div class="filter-tabs" role="tablist" aria-label="Filter invoices by status">
          {#each filterTabs as tab}
            <button
              class="filter-tab"
              class:active={selectedFilter === tab.value}
              role="tab"
              aria-selected={selectedFilter === tab.value}
              on:click={() => (selectedFilter = tab.value)}
            >
              {tab.label}
              <span class="tab-count">{tab.count}</span>
            </button>
          {/each}
        </div>
      </Card>

      <Input
        type="text"
        placeholder="Search invoices..."
        bind:value={searchQuery}
        style="width: 300px;"
        aria-label="Search invoices by number or customer"
      />
    </div>

    <!-- Invoices DataTable -->
    <Card padding="sm">
      {#if loading}
        <div class="loading-container">
          <WabiSpinner size="lg" />
          <p>Loading invoices...</p>
        </div>
      {:else}
        <DataTable
          {columns}
          data={filteredInvoices}
          {loading}
          emptyMessage="No invoices found. Click 'Create Invoice' to add one from an order."
          onRowClick={(row) => openDetailModal(row)}
          stickyHeader={!embedded}
          maxHeight={embedded ? '400px' : 'calc(100vh - 520px)'}
          showBorder={false}
        >
          <svelte:fragment slot="cell" let:row let:column>
            {#if column.key === 'actions'}
              <div class="action-buttons">
                <button
                  class="action-btn action-edit"
                  on:click|stopPropagation={() => openEditModal(row)}
                  title="Edit Invoice"
                >
                  Edit
                </button>
                <button
                  class="action-btn action-pdf"
                  on:click|stopPropagation={() => downloadPDF(row)}
                  title="Download PDF"
                  disabled={pdfLoadingMap[row.id]}
                >
                  {pdfLoadingMap[row.id] ? '...' : 'PDF'}
                </button>
                {#if row.status === 'Draft'}
                  <button
                    class="action-btn action-send"
                    on:click|stopPropagation={() => handleSendInvoice(row)}
                    title="Send Invoice"
                    disabled={sendingInvoiceId === row.id}
                  >
                    {sendingInvoiceId === row.id ? '...' : 'Send'}
                  </button>
                {/if}
                {#if row.status !== 'Paid'}
                  <button
                    class="action-btn action-delete"
                    on:click|stopPropagation={() => openDeleteModal(row)}
                    title="Delete Invoice"
                  >
                    Delete
                  </button>
                {/if}
              </div>
            {/if}
          </svelte:fragment>
        </DataTable>
      {/if}
    </Card>

    <!-- Pagination Controls -->
    {#if hasMore && !loading}
      <div class="pagination-controls">
        <button
          class="load-more-btn"
          on:click={loadMore}
          disabled={loadingMore}
          aria-label={loadingMore ? 'Loading more invoices' : `Load more invoices, ${totalLoaded} currently loaded`}
        >
          {#if loadingMore}
            Loading more...
          {:else}
            Load More ({totalLoaded} loaded)
          {/if}
        </button>
      </div>
    {/if}

    {#if !hasMore && totalLoaded > 0 && !loading}
      <p class="all-loaded">All {totalLoaded} invoices loaded</p>
    {/if}

  {:else}
    <!-- Phase 23: Credit Notes View -->
    <Card padding="md">
      {#if loadingCreditNotes}
        <div style="text-align: center; padding: 32px;">
          <WabiSpinner size="md" />
          <p style="margin-top: 8px; color: var(--text-secondary);">Loading credit notes...</p>
        </div>
      {:else if creditNotes.length === 0}
        <div style="text-align: center; padding: 40px; color: var(--text-secondary);">
          <p style="font-size: 14px; font-weight: 500;">No credit notes yet</p>
          <p style="font-size: 12px;">Issue a credit note against an invoice for returns or adjustments.</p>
          <Button variant="primary" on:click={() => openCNModal()}>+ Issue Credit Note</Button>
        </div>
      {:else}
        <table class="cn-table">
          <thead>
            <tr>
              <th>CN Number</th>
              <th>Date</th>
              <th>Invoice Ref</th>
              <th>Customer</th>
              <th class="right">Amount (BHD)</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each creditNotes as cn}
              <tr>
                <td class="mono">{cn.credit_note_number}</td>
                <td>{formatDate(cn.credit_note_date)}</td>
                <td class="mono">{cn.invoice_number}</td>
                <td>{cn.customer_name}</td>
                <td class="right mono">{(cn.grand_total_bhd || 0).toFixed(3)}</td>
                <td>
                  <span class="cn-status" class:cn-draft={cn.status === 'Draft'} class:cn-issued={cn.status === 'Issued'} class:cn-applied={cn.status === 'Applied'}>
                    {cn.status}
                  </span>
                </td>
                <td>
                  <div style="display: flex; gap: 6px;">
                    <button class="action-btn action-secondary" on:click={() => handleCNPDF(cn.id)}>PDF</button>
                    {#if cn.status !== 'Applied'}
                      <button class="action-btn action-primary" on:click={() => handleApplyCN(cn.id)}>Apply</button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </Card>
  {/if}
  </div>
</PageLayout>

<!-- Create Invoice Modal -->
{#if showCreateModal}
  <Modal
    title="Create Invoice from Order"
    open={showCreateModal}
    on:close={() => { showCreateModal = false; resetFieldVisibility(); }}
    size="md"
  >
    <div class="create-form">
      <FormGroup label="Select Unfulfilled Order" required>
        <select
          class="select-input"
          bind:value={selectedOrderId}
          disabled={createLoading}
        >
          <option value="">-- Select an order --</option>
          {#each orders as order}
            <option value={order.id}>
              {order.order_number} - {order.customer_name} ({formatBHD(order.grand_total_bhd)} BHD) - {order.status}
            </option>
          {/each}
        </select>
      </FormGroup>

      <!-- Delivery Note Selector (appears after order is selected) -->
      {#if selectedOrderId}
        <FormGroup label="Link Delivery Note (Optional)">
          {#if loadingDeliveryNotes}
            <div class="dn-loading">Loading delivery notes...</div>
          {:else if availableDeliveryNotes.length === 0}
            <div class="dn-empty-hint">
              <p>No delivery notes available for this order. You can create the invoice without a DN link.</p>
            </div>
          {:else}
            <select
              class="select-input"
              bind:value={selectedDeliveryNoteId}
              disabled={createLoading}
            >
              <option value="">-- No DN (create invoice without DN) --</option>
              {#each availableDeliveryNotes as dn}
                <option value={dn.id}>
                  {dn.dn_number} - {formatDate(dn.delivery_date)} - {dn.status}
                </option>
              {/each}
            </select>
            <p class="dn-hint">Select a delivery note to link with this invoice, or leave blank to create invoice without DN.</p>
          {/if}
        </FormGroup>

        <!-- Field Visibility Options -->
        <div class="field-visibility-section">
          <div class="visibility-header">
            <h4>Invoice Fields (from Costing Sheet)</h4>
            <p class="visibility-hint">Select which fields to show on the invoice PDF</p>
          </div>

          <div class="visibility-grid">
            <!-- Line Item Fields -->
            <div class="visibility-group">
              <span class="group-label">Line Items</span>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_equipment} />
                Equipment Name
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_specification} />
                Specification
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_detailed_desc} />
                Detailed Description
              </label>
            </div>

            <!-- Cost Fields -->
            <div class="visibility-group">
              <span class="group-label">Cost Data (Internal)</span>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_fob} />
                FOB Cost
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_freight} />
                Freight Cost
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_cost} />
                Total Cost
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_margin} />
                Margin %
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_currency} />
                Source Currency
              </label>
            </div>

            <!-- Contact/RFQ Fields -->
            <div class="visibility-group">
              <span class="group-label">Header Details</span>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_contact} />
                Contact Person
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_rfq} />
                RFQ Reference
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_country_origin} />
                Country of Origin
              </label>
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={fieldVisibility.show_delivery_weeks} />
                Delivery Time
              </label>
            </div>
          </div>
        </div>
      {/if}

      {#if orders.length === 0}
        <div class="empty-hint">
          <p>No unfulfilled orders available. All orders are complete or already invoiced.</p>
        </div>
      {/if}
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => { showCreateModal = false; resetFieldVisibility(); }} disabled={createLoading}>
        Cancel
      </Button>
      <Button
        variant="primary"
        on:click={handleCreateInvoice}
        disabled={createLoading || !selectedOrderId}
      >
        {createLoading ? 'Creating...' : selectedDeliveryNoteId ? 'Create Invoice + Link DN' : 'Create Invoice'}
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && invoiceToDelete}
  <Modal
    title="Delete Invoice"
    open={showDeleteModal}
    on:close={() => (showDeleteModal = false)}
    size="sm"
  >
    <div class="delete-confirmation">
      <p>Are you sure you want to delete invoice <strong>{invoiceToDelete.invoice_number}</strong>?</p>
      <p class="warning-text">This action cannot be undone.</p>
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => (showDeleteModal = false)} disabled={deleteLoading}>
        Cancel
      </Button>
      <Button variant="danger" on:click={handleDeleteInvoice} disabled={deleteLoading}>
        {deleteLoading ? 'Deleting...' : 'Delete Invoice'}
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

{#if showEditModal && editInvoice}
  <Modal
    title="Edit Invoice"
    open={showEditModal}
    on:close={() => { showEditModal = false; editInvoice = null; }}
    size="md"
  >
    <div class="edit-form">
      <div class="form-row">
        <div class="form-field">
          <label>Invoice Number</label>
          <input type="text" value={editInvoice.invoice_number} class="input-field" disabled title="Invoice numbers are system-generated" />
        </div>
        <div class="form-field">
          <label>Status</label>
          <select bind:value={editInvoice.status} class="input-field">
            <option value="Draft">Draft</option>
            <option value="Sent">Sent</option>
            <option value="Paid">Paid</option>
            <option value="PartiallyPaid">Partially Paid</option>
            <option value="Overdue">Overdue</option>
          </select>
        </div>
      </div>
      <div class="form-row">
        <div class="form-field">
          <label>Amount (BHD) <span class="required">*</span></label>
          <input
            type="number"
            step="0.001"
            min="0.001"
            bind:value={editInvoice.grand_total_bhd}
            class="input-field"
            required
          />
        </div>
        <div class="form-field">
          <label>Outstanding (BHD) <span class="required">*</span></label>
          <input
            type="number"
            step="0.001"
            min="0"
            bind:value={editInvoice.outstanding_bhd}
            class="input-field"
            required
          />
        </div>
      </div>
      <div class="form-field">
        <label>Customer PO Number</label>
        <input type="text" bind:value={editInvoice.customer_po_number} class="input-field" />
      </div>
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => { showEditModal = false; editInvoice = null; }} disabled={editLoading}>
        Cancel
      </Button>
      <Button variant="primary" on:click={saveInvoiceEdit} disabled={editLoading}>
        {editLoading ? 'Saving...' : 'Save Changes'}
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

<!-- Phase 23: Credit Note Creation Modal -->
{#if showCNModal}
  <Modal
    title="Issue Credit Note"
    open={showCNModal}
    on:close={() => showCNModal = false}
    size="md"
  >
    <div class="cn-form">
      <FormGroup label="Against Invoice" required>
        <select class="select-input" bind:value={cnInvoiceId} disabled={cnSaving}>
          <option value="">-- Select an invoice --</option>
          {#each invoices.filter(i => i.status !== 'Draft') as inv}
            <option value={inv.id}>
              {inv.invoice_number} - {inv.customer_name} ({formatBHD(inv.grand_total_bhd)} BHD)
            </option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Reason" required>
        <textarea
          class="cn-textarea"
          bind:value={cnReason}
          placeholder="e.g. Goods returned, pricing correction, quantity adjustment..."
          rows="3"
          disabled={cnSaving}
        ></textarea>
      </FormGroup>

      <div class="cn-items-section">
        <div class="cn-items-header">
          <h4>Credit Note Items</h4>
          <button class="cn-add-item-btn" on:click={addCNItem} disabled={cnSaving}>+ Add Item</button>
        </div>

        {#each cnItems as item, i}
          <div class="cn-item-row">
            <div class="cn-item-desc">
              <input
                type="text"
                class="input-field"
                placeholder="Description"
                bind:value={item.description}
                disabled={cnSaving}
              />
            </div>
            <div class="cn-item-qty">
              <input
                type="number"
                class="input-field"
                placeholder="Qty"
                min="1"
                step="1"
                bind:value={item.quantity}
                disabled={cnSaving}
              />
            </div>
            <div class="cn-item-rate">
              <input
                type="number"
                class="input-field"
                placeholder="Rate (BHD)"
                min="0"
                step="0.001"
                bind:value={item.rate}
                disabled={cnSaving}
              />
            </div>
            <div class="cn-item-total">
              {(item.quantity * item.rate).toFixed(3)}
            </div>
            <button
              class="cn-remove-btn"
              on:click={() => removeCNItem(i)}
              disabled={cnSaving || cnItems.length <= 1}
              title="Remove item"
            >
              &times;
            </button>
          </div>
        {/each}

        <div class="cn-total-row">
          <span>Subtotal:</span>
          <span class="cn-total-value">
            {cnItems.reduce((s, it) => s + (it.quantity * it.rate), 0).toFixed(3)} BHD
          </span>
        </div>
      </div>
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => showCNModal = false} disabled={cnSaving}>
        Cancel
      </Button>
      <Button variant="primary" on:click={handleCreateCN} disabled={cnSaving || !cnInvoiceId}>
        {cnSaving ? 'Creating...' : 'Issue Credit Note'}
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

<!-- Invoice Detail Modal -->
{#if showDetailModal && selectedInvoice}
  <Modal
    title="Invoice Details"
    open={showDetailModal}
    on:close={() => { showDetailModal = false; selectedInvoice = null; }}
    size="lg"
  >
    <div class="invoice-detail">
      <!-- Header Info -->
      <div class="detail-header">
        <div class="detail-main">
          <h2 class="invoice-number">{selectedInvoice.invoice_number}</h2>
          <span class="status-badge" style="background: {statusColors[selectedInvoice.status] || '#6B7280'}15; color: {statusColors[selectedInvoice.status] || '#6B7280'};">
            {selectedInvoice.status}
          </span>
        </div>
        <div class="detail-customer">{selectedInvoice.customer_name}</div>
      </div>

      <!-- Key Info Grid -->
      <div class="detail-grid">
        <div class="detail-item">
          <span class="detail-label">Invoice Date</span>
          <span class="detail-value">{formatDate(selectedInvoice.invoice_date)}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Due Date</span>
          <span class="detail-value" class:overdue={isDateOverdue(selectedInvoice.due_date) && selectedInvoice.outstanding_bhd > 0}>
            {formatDate(selectedInvoice.due_date)}
          </span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Amount (BHD)</span>
          <span class="detail-value mono">{formatBHD(selectedInvoice.grand_total_bhd)}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">Outstanding (BHD)</span>
          <span class="detail-value mono" class:warning={selectedInvoice.outstanding_bhd > 0}>
            {formatBHD(selectedInvoice.outstanding_bhd)}
          </span>
        </div>
        {#if selectedInvoice.customer_po_number}
          <div class="detail-item">
            <span class="detail-label">Customer PO</span>
            <span class="detail-value">{selectedInvoice.customer_po_number}</span>
          </div>
        {/if}
        {#if selectedInvoice.delivery_note_number}
          <div class="detail-item">
            <span class="detail-label">Delivery Note</span>
            <span class="detail-value dn-link">{selectedInvoice.delivery_note_number}</span>
          </div>
        {/if}
        {#if selectedInvoice.offer_number}
          <div class="detail-item">
            <span class="detail-label">Offer</span>
            <span class="detail-value">{selectedInvoice.offer_number}</span>
          </div>
        {/if}
        {#if selectedInvoice.customer_reference}
          <div class="detail-item">
            <span class="detail-label">RFQ / Reference</span>
            <span class="detail-value">{selectedInvoice.customer_reference}</span>
          </div>
        {/if}
        {#if selectedInvoice.attention_person || selectedInvoice.attention_company}
          <div class="detail-item">
            <span class="detail-label">Attention</span>
            <span class="detail-value">
              {selectedInvoice.attention_person || '—'}{selectedInvoice.attention_company ? ` · ${selectedInvoice.attention_company}` : ''}
            </span>
          </div>
        {/if}
        {#if selectedInvoice.delivery_weeks}
          <div class="detail-item">
            <span class="detail-label">Delivery Time</span>
            <span class="detail-value">{selectedInvoice.delivery_weeks}</span>
          </div>
        {/if}
        {#if selectedInvoice.country_of_origin}
          <div class="detail-item">
            <span class="detail-label">Country of Origin</span>
            <span class="detail-value">{selectedInvoice.country_of_origin}</span>
          </div>
        {/if}
        {#if selectedInvoice.payment_terms}
          <div class="detail-item">
            <span class="detail-label">Payment Terms</span>
            <span class="detail-value">{selectedInvoice.payment_terms}</span>
          </div>
        {/if}
        {#if selectedInvoice.delivery_terms}
          <div class="detail-item">
            <span class="detail-label">Delivery Terms</span>
            <span class="detail-value">{selectedInvoice.delivery_terms}</span>
          </div>
        {/if}
      </div>

      <!-- Line Items Section -->
      {#if selectedInvoice.items && selectedInvoice.items.length > 0}
        <div class="line-items-section">
          <h4>Line Items ({selectedInvoice.items.length})</h4>
          <div class="items-table">
            <div class="items-header">
              <span class="item-col-num">#</span>
              <span class="item-col-desc">Description</span>
              <span class="item-col-qty">Qty</span>
              <span class="item-col-rate">Rate</span>
              <span class="item-col-total">Total (BHD)</span>
            </div>
            {#each selectedInvoice.items as item, i}
              <div class="items-row">
                <span class="item-col-num">{item.line_number || i + 1}</span>
                <span class="item-col-desc">
                  <strong>{item.equipment || item.description || `Item ${i + 1}`}</strong>
                  {#if item.model || item.product_code}
                    <small>{item.model || item.product_code}</small>
                  {/if}
                  {#if item.specification}
                    <small>{item.specification}</small>
                  {/if}
                  {#if item.detailed_description}
                    <small>{item.detailed_description}</small>
                  {/if}
                </span>
                <span class="item-col-qty">{item.quantity || 0}</span>
                <span class="item-col-rate">{(item.rate || 0).toFixed(3)}</span>
                <span class="item-col-total">{(item.total_bhd || 0).toFixed(3)}</span>
              </div>
            {/each}
            <div class="items-footer">
              <span class="footer-label">Grand Total</span>
              <span class="footer-value">{formatBHD(selectedInvoice.grand_total_bhd)} BHD</span>
            </div>
          </div>
        </div>
      {:else}
        <div class="no-items-hint">
          <p>No line items found for this invoice.</p>
        </div>
      {/if}
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => { showDetailModal = false; selectedInvoice = null; }}>
        Close
      </Button>
      <Button variant="secondary" on:click={() => downloadPDF(selectedInvoice)} disabled={pdfLoadingMap[selectedInvoice?.id || '']}>
        {pdfLoadingMap[selectedInvoice?.id || ''] ? 'Generating...' : 'Download PDF'}
      </Button>
      <Button variant="primary" on:click={() => { showDetailModal = false; openEditModal(selectedInvoice); }}>
        Edit Invoice
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

<style>
  .invoices-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* KPI Grid */
  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 16px;
  }

  .kpi-card {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .kpi-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 600;
    color: var(--text-secondary);
  }

  .kpi-value {
    font-size: 28px;
    font-weight: 700;
    font-family: var(--font-mono);
    color: var(--text-primary);
    line-height: 1.2;
  }

  .kpi-currency {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-secondary);
    margin-left: 4px;
  }

  .kpi-primary {
    color: var(--brand-indigo);
  }

  .kpi-warning {
    color: #f59e0b;
  }

  .kpi-danger {
    color: #ef4444;
  }

  .kpi-sublabel {
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* Controls Row */
  .controls-row {
    display: flex;
    gap: 16px;
    align-items: center;
  }

  /* Filter Tabs */
  .filter-tabs {
    display: flex;
    gap: 8px;
    overflow-x: auto;
  }

  .filter-tab {
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

  .filter-tab:hover {
    background: var(--interactive-hover);
    color: var(--text-primary);
  }

  .filter-tab.active {
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

  .filter-tab.active .tab-count {
    background: rgba(255, 255, 255, 0.2);
  }

  /* Loading Container */
  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 64px 24px;
  }

  .loading-container p {
    font-size: 14px;
    color: var(--text-secondary);
  }

  /* Action Buttons in Table */
  .action-buttons {
    display: flex;
    gap: 8px;
    justify-content: center;
  }

  .action-btn {
    padding: 6px 12px;
    font-size: 12px;
    font-weight: 600;
    border: none;
    border-radius: var(--border-radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .action-send {
    background: var(--brand-indigo);
    color: white;
  }

  .action-send:hover {
    background: var(--brand-indigo-hover);
    transform: translateY(-1px);
  }

  .action-delete {
    background: #fee2e2;
    color: #991b1b;
  }

  .action-delete:hover {
    background: #fecaca;
    transform: translateY(-1px);
  }

  .action-edit {
    background: var(--bg-secondary, #f3f4f6);
    color: var(--ink, #1d1d1f);
  }
  .action-edit:hover {
    background: var(--border-medium, #e5e5e5);
    transform: translateY(-1px);
  }
  .action-pdf {
    background: #ede9fe;
    color: #5b21b6;
  }
  .action-pdf:hover {
    background: #ddd6fe;
    transform: translateY(-1px);
  }

  .edit-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .form-row {
    display: flex;
    gap: 12px;
  }
  .form-field {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .form-field label {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .input-field {
    padding: 8px 12px;
    border: 1px solid var(--border-medium, #e5e5e5);
    border-radius: 6px;
    font-size: 14px;
    font-family: inherit;
  }
  .input-field:focus {
    border-color: var(--ink, #1d1d1f);
    outline: none;
  }

  /* Modal Forms */
  .create-form {
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 4px 0;
  }

  .select-input {
    width: 100%;
    padding: 10px 12px;
    font-size: 14px;
    font-family: var(--font-family);
    color: var(--text-primary);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--border-radius-sm);
    transition: all var(--transition-fast);
  }

  .select-input:focus {
    outline: none;
    border-color: var(--brand-indigo);
    box-shadow: 0 0 0 3px var(--brand-indigo-tint);
  }

  .select-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .empty-hint {
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius-sm);
    border-left: 3px solid #f59e0b;
  }

  .empty-hint p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
  }

  /* Delivery Note selector styles */
  .dn-loading {
    padding: 12px;
    font-size: 13px;
    color: var(--text-secondary);
    font-style: italic;
  }

  .dn-empty-hint {
    padding: 12px;
    background: var(--surface-elevated, #f9fafb);
    border-radius: var(--border-radius-sm);
    border-left: 3px solid var(--steel, #86868b);
  }

  .dn-empty-hint p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .dn-hint {
    margin: 8px 0 0 0;
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* Field Visibility Section */
  .field-visibility-section {
    margin-top: 20px;
    padding: 16px;
    background: var(--surface-elevated, #f9fafb);
    border-radius: var(--border-radius-sm, 6px);
    border: 1px solid var(--border, #e5e5e5);
  }

  .visibility-header {
    margin-bottom: 16px;
  }

  .visibility-header h4 {
    margin: 0 0 4px 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .visibility-hint {
    margin: 0;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .visibility-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
  }

  .visibility-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .group-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    padding-bottom: 4px;
    border-bottom: 1px solid var(--border, #e5e5e5);
    margin-bottom: 4px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text-primary);
    cursor: pointer;
  }

  .checkbox-label input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--carbon, #000);
    cursor: pointer;
  }

  @media (max-width: 768px) {
    .visibility-grid {
      grid-template-columns: 1fr;
    }
  }

  .delete-confirmation {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .delete-confirmation p {
    margin: 0;
    font-size: 14px;
    color: var(--text-primary);
  }

  .warning-text {
    color: #ef4444;
    font-weight: 500;
    font-size: 13px;
  }

  /* Required field indicator */
  .required {
    color: #ef4444;
    margin-left: 2px;
    font-weight: 600;
  }

  /* Invoice Detail Modal */
  .invoice-detail {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .detail-header {
    border-bottom: 2px solid var(--text-primary);
    padding-bottom: 16px;
  }

  .detail-main {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 4px;
  }

  .invoice-number {
    margin: 0;
    font-size: 24px;
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--brand-indigo);
  }

  .status-badge {
    display: inline-block;
    padding: 4px 12px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
  }

  .detail-customer {
    font-size: 16px;
    color: var(--text-secondary);
  }

  .detail-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 16px;
    padding: 16px;
    background: var(--surface-elevated, #f9fafb);
    border-radius: 8px;
  }

  .detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .detail-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .detail-value {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .detail-value.mono {
    font-family: var(--font-mono);
  }

  .detail-value.overdue {
    color: #ef4444;
    font-weight: 600;
  }

  .detail-value.warning {
    color: #f59e0b;
  }

  .detail-value.dn-link {
    color: #059669;
    font-family: var(--font-mono);
  }

  /* Line Items in Detail Modal */
  .line-items-section {
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 8px;
    overflow: hidden;
  }

  .line-items-section h4 {
    margin: 0;
    padding: 12px 16px;
    font-size: 14px;
    font-weight: 600;
    background: var(--surface-elevated, #f9fafb);
    border-bottom: 1px solid var(--border, #e5e5e5);
  }

  .items-table {
    width: 100%;
  }

  .items-header {
    display: grid;
    grid-template-columns: 40px 2fr 80px 100px 120px;
    gap: 8px;
    padding: 10px 16px;
    background: rgba(0, 0, 0, 0.02);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .items-row {
    display: grid;
    grid-template-columns: 40px 2fr 80px 100px 120px;
    gap: 8px;
    padding: 10px 16px;
    border-bottom: 1px solid rgba(0, 0, 0, 0.05);
    font-size: 13px;
  }

  .items-row:last-child {
    border-bottom: none;
  }

  .item-col-num {
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }

  .item-col-desc {
    display: grid;
    gap: 4px;
    min-width: 0;
  }

  .item-col-desc strong {
    font-weight: 600;
    color: var(--text-primary);
  }

  .item-col-desc small {
    color: var(--text-secondary);
    line-height: 1.45;
  }

  .item-col-qty,
  .item-col-rate,
  .item-col-total {
    text-align: right;
    font-family: var(--font-mono);
  }

  .items-footer {
    display: flex;
    justify-content: flex-end;
    gap: 24px;
    padding: 12px 16px;
    background: var(--surface-elevated, #f9fafb);
    border-top: 2px solid var(--text-primary);
  }

  .footer-label {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .footer-value {
    font-size: 16px;
    font-weight: 700;
    font-family: var(--font-mono);
    color: var(--text-primary);
  }

  .no-items-hint {
    padding: 24px;
    text-align: center;
    background: var(--surface-elevated, #f9fafb);
    border-radius: 8px;
  }

  .no-items-hint p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
  }

  /* Pagination Controls */
  .pagination-controls {
    display: flex;
    justify-content: center;
    padding: 1rem;
  }

  .load-more-btn {
    padding: 0.75rem 2rem;
    background: var(--brand-indigo, #6366f1);
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all var(--transition-fast);
  }

  .load-more-btn:hover:not(:disabled) {
    background: var(--brand-indigo-hover, #4f46e5);
    transform: translateY(-1px);
  }

  .load-more-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .all-loaded {
    text-align: center;
    color: var(--text-secondary, #a0a0a0);
    padding: 1rem;
    font-size: 13px;
  }

  /* Phase 23: Main View Tabs */
  .main-view-tabs {
    display: flex;
    gap: 0;
    border-bottom: 2px solid var(--border, #e5e5e5);
    margin-bottom: 4px;
  }

  .main-tab {
    padding: 10px 20px;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--transition-fast);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .main-tab:hover {
    color: var(--text-primary);
  }

  .main-tab-active {
    color: var(--brand-indigo);
    border-bottom-color: var(--brand-indigo);
    font-weight: 600;
  }

  .main-tab-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 22px;
    height: 18px;
    padding: 0 6px;
    border-radius: 9px;
    font-size: 11px;
    font-weight: 600;
    background: var(--surface-elevated, #f3f4f6);
    color: var(--text-secondary);
  }

  .main-tab-active .main-tab-count {
    background: var(--brand-indigo);
    color: white;
  }

  /* Phase 23: Credit Notes Table */
  .cn-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .cn-table th {
    text-align: left;
    padding: 10px 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    border-bottom: 2px solid var(--border, #e5e5e5);
  }

  .cn-table th.right {
    text-align: right;
  }

  .cn-table td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border, #e5e5e5);
    vertical-align: middle;
  }

  .cn-table td.right {
    text-align: right;
  }

  .cn-table td.mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .cn-table tr:hover {
    background: var(--interactive-hover, rgba(0,0,0,0.02));
  }

  .cn-status {
    display: inline-block;
    padding: 3px 10px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
  }

  .cn-draft {
    background: #6b728015;
    color: #6b7280;
  }

  .cn-issued {
    background: #3b82f615;
    color: #3b82f6;
  }

  .cn-applied {
    background: #10b98115;
    color: #10b981;
  }

  .action-secondary {
    background: #ede9fe;
    color: #5b21b6;
  }

  .action-secondary:hover {
    background: #ddd6fe;
  }

  .action-primary {
    background: var(--brand-indigo, #6366f1);
    color: white;
  }

  .action-primary:hover {
    background: var(--brand-indigo-hover, #4f46e5);
  }

  /* Phase 23: Credit Note Form */
  .cn-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .cn-textarea {
    width: 100%;
    padding: 10px 12px;
    font-size: 14px;
    font-family: inherit;
    color: var(--text-primary);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--border-radius-sm);
    resize: vertical;
  }

  .cn-textarea:focus {
    outline: none;
    border-color: var(--brand-indigo);
    box-shadow: 0 0 0 3px var(--brand-indigo-tint);
  }

  .cn-items-section {
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 8px;
    padding: 12px;
  }

  .cn-items-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .cn-items-header h4 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .cn-add-item-btn {
    padding: 4px 12px;
    font-size: 12px;
    font-weight: 600;
    background: var(--brand-indigo, #6366f1);
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .cn-add-item-btn:hover {
    background: var(--brand-indigo-hover, #4f46e5);
  }

  .cn-item-row {
    display: grid;
    grid-template-columns: 2fr 80px 100px 80px 32px;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
  }

  .cn-item-row .input-field {
    padding: 6px 8px;
    font-size: 13px;
  }

  .cn-item-total {
    text-align: right;
    font-family: var(--font-mono);
    font-size: 13px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .cn-remove-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #fee2e2;
    color: #991b1b;
    border: none;
    border-radius: 4px;
    font-size: 16px;
    cursor: pointer;
  }

  .cn-remove-btn:hover:not(:disabled) {
    background: #fecaca;
  }

  .cn-remove-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .cn-total-row {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--border, #e5e5e5);
    margin-top: 4px;
  }

  .cn-total-row span {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .cn-total-value {
    font-family: var(--font-mono);
    font-weight: 600 !important;
    color: var(--text-primary) !important;
  }

  /* Responsive */
  @media (max-width: 768px) {
    .kpi-grid {
      grid-template-columns: 1fr;
    }

    .controls-row {
      flex-direction: column;
      align-items: stretch;
    }

    .filter-tabs {
      flex-wrap: wrap;
    }

    .items-header,
    .items-row {
      grid-template-columns: 30px 1fr 60px 80px 90px;
      font-size: 11px;
    }
  }
</style>
