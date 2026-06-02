<script lang="ts">
  /**
   * PaymentsScreen - Customer receipts and cash application
   *
   * Features:
   * - Apply new receipts against open invoices
   * - View all receipts with filtering
   * - Real-time KPI dashboard (Total Collected, This Month, Avg Days)
   * - Invoice selection with open balance display
   * - Receipt method tracking (Bank Transfer, Cheque, Cash, Wire, Online)
   * - BHD currency formatting (3 decimal places)
   * - Filter tabs: All, This Month, This Quarter
   * - Embedded mode support for FinanceHub
   *
   * Design System: Wabi-Sabi minimalism × Bloomberg data density
   */

  import { createEventDispatcher, onMount } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports
  import {
    GetAllPayments,
    GetPayment,
    CreateCustomerReceipt,
    ListCustomerReceipts,
    UpdatePayment,
    DeletePayment,
    ListCustomerInvoices,
    GetAllCustomers,
  } from '../../../wailsjs/go/main/App';
  import { main } from '../../../wailsjs/go/models';

  // Security utilities
  import { escapeHtml } from '$lib/utils/escapeHtml';
  import { debounce } from '$lib/utils/debounce';

  // Design system components
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
  import Modal from '$lib/components/layout/Modal.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import { toast } from '$lib/stores/toasts';
  import { permissions } from '$lib/stores/authContext';
  import { buildWailsInput, toWailsDate } from '$lib/utils/wailsInterop';

  $: permissionList = Array.isArray($permissions) ? $permissions : [];
  $: canView = permissionList.includes('*') || permissionList.includes('finance:view') || permissionList.includes('finance:*');
  $: canCreate = permissionList.includes('*') || permissionList.includes('payments:create') || permissionList.includes('payments:*') || permissionList.includes('finance:*');
  $: canUpdate = permissionList.includes('*') || permissionList.includes('payments:update') || permissionList.includes('payments:*') || permissionList.includes('finance:*');
  $: canDelete = permissionList.includes('*') || permissionList.includes('payments:delete') || permissionList.includes('payments:*') || permissionList.includes('finance:*');
  const dispatch = createEventDispatcher();

  // Props
  export let embedded = false;
  export let company: 'PH Trading' | 'AHS Trading' = 'PH Trading';

  // Types
  type TimeFilter = 'All' | 'This Month' | 'This Quarter';

  interface Payment {
    id: string;
    source_kind?: 'payment' | 'receipt';
    receipt_id?: string;
    receipt_number?: string;
    receipt_status?: string;
    unapplied_amount_bhd?: number;
    customer_id?: string;
    invoice_id: string;
    invoice_number: string;
    customer_name?: string;
    amount_bhd: number;
    payment_date: any; // Go Time type serializes as object, not string
    payment_method: string;
    days_to_payment: number;
    reference?: string;
    division?: string;
  }

  interface CustomerReceipt {
    id: string;
    receipt_number: string;
    customer_id: string;
    customer_name: string;
    amount_bhd: number;
    applied_amount_bhd: number;
    unapplied_amount_bhd: number;
    receipt_date: any;
    payment_method: string;
    reference?: string;
    status: string;
    division?: string;
  }

  interface Invoice {
    id: string;
    invoice_number: string;
    customer_name: string;
    grand_total_bhd: number;
    outstanding_bhd: number;
    status: string;
    invoice_date?: any; // Go Time type serializes as object, not string
    division?: string;
  }

  interface CustomerOption {
    id: string;
    customer_id?: string;
    business_name: string;
    customer_code?: string;
    division?: string;
  }

  // State
  let payments: Payment[] = [];
  let filteredPayments: Payment[] = [];
  let invoices: Invoice[] = [];
  let customers: CustomerOption[] = [];
  let loading = true;
  let selectedFilter: TimeFilter = 'All';

  // Pagination state
  const PAGE_SIZE = 50;
  let currentPage = 0;
  let hasMore = true;
  let loadingMore = false;
  let totalLoaded = 0;

  // Modal state
  let showRecordModal = false;
  let recordLoading = false;
  let paymentModalMode: 'create' | 'edit' = 'create';
  let receiptMode: 'invoice' | 'on_account' = 'invoice';
  let editingPaymentId = '';
  let selectedInvoiceId = '';
  let selectedInvoice: Invoice | null = null;
  let selectedCustomerId = '';
  let selectedCustomer: CustomerOption | null = null;
  let customerSearchQuery = '';
  let paymentAmount = '';
  let paymentMethod = 'Bank Transfer';
  let paymentDate = new Date().toISOString().split('T')[0];
  let paymentReference = '';
  let invoiceSearchQuery = '';
  let debouncedInvoiceSearch = '';
  let selectedPaymentId = '';
  let selectedPayment: Payment | null = null;
  let deleteConfirmPaymentId = '';

  // Debounced invoice search (300ms delay)
  const updateDebouncedInvoiceSearch = debounce((value: string) => {
    debouncedInvoiceSearch = value;
  }, 300);

  // Watch invoiceSearchQuery changes and debounce
  $: updateDebouncedInvoiceSearch(invoiceSearchQuery);

  // Payment method options
  const paymentMethods = [
    'Bank Transfer',
    'Cheque',
    'Cash',
    'Credit Card',
    'LC',
    'PDC',
    'Other',
    'Wire Transfer',
    'Online'
  ];

  function matchesCompany(division?: string) {
    return (division || 'PH Trading') === company;
  }

  // DataTable columns configuration
  const columns = [
    {
      key: 'payment_date',
      label: 'Date',
      type: 'date' as const,
      sortable: true,
      width: '120px',
      render: (row: Payment) => {
        return `<span style="font-family: var(--font-mono); font-size: 13px;">${formatDate(row.payment_date)}</span>`;
      }
    },
    {
      key: 'invoice_number',
      label: 'Invoice #',
      sortable: true,
      width: '140px',
      render: (row: Payment) => {
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: var(--brand-indigo);">${escapeHtml(row.invoice_number || '')}</span>`;
      }
    },
    {
      key: 'customer_name',
      label: 'Customer',
      sortable: true,
      render: (row: Payment) => {
        return `<span style="font-weight: 500;">${escapeHtml(row.customer_name || '—')}</span>`;
      }
    },
    {
      key: 'amount_bhd',
      label: 'Amount (BHD)',
      sortable: true,
      width: '140px',
      align: 'right' as const,
      render: (row: Payment) => {
        return `<span style="font-family: var(--font-mono); font-weight: 600; font-size: 14px;">${formatBHD(row.amount_bhd)}</span>`;
      }
    },
    {
      key: 'payment_method',
      label: 'Method',
      sortable: true,
      width: '140px',
      render: (row: Payment) => {
        const colorMap: Record<string, string> = {
          'Bank Transfer': '#10B981',
          'Wire Transfer': '#3B82F6',
          'Cheque': '#F59E0B',
          'Cash': '#EF4444',
          'Online': '#8B5CF6'
        };
        const color = colorMap[row.payment_method] || '#6B7280';
        return `<span style="color: ${color}; font-size: 12px; font-weight: 500;">${row.payment_method || 'N/A'}</span>`;
      }
    },
    {
      key: 'days_to_payment',
      label: 'Days to Payment',
      sortable: true,
      width: '140px',
      align: 'center' as const,
      render: (row: Payment) => {
        const days = row.days_to_payment;
        const color = days <= 30 ? '#10B981' : days <= 60 ? '#F59E0B' : '#EF4444';
        return `<span style="font-family: var(--font-mono); font-weight: 600; color: ${color};">${days}d</span>`;
      }
    },
    {
      key: 'reference',
      label: 'Reference',
      sortable: true,
      width: '140px',
      render: (row: Payment) => {
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary);">${escapeHtml(row.reference || '—')}</span>`;
      }
    }
  ];

  // Filter tabs
  const filterTabs: { value: TimeFilter; label: string; count: number }[] = [
    { value: 'All', label: 'All Receipts', count: 0 },
    { value: 'This Month', label: 'This Month', count: 0 },
    { value: 'This Quarter', label: 'This Quarter', count: 0 }
  ];

  // Computed: Update tab counts
  $: {
    filterTabs[0].count = payments.length;
    filterTabs[1].count = payments.filter(p => isThisMonth(p.payment_date)).length;
    filterTabs[2].count = payments.filter(p => isThisQuarter(p.payment_date)).length;
  }

  // Computed: Filter payments by time
  $: {
    let result = [...payments];

    if (selectedFilter === 'This Month') {
      result = result.filter(p => isThisMonth(p.payment_date));
    } else if (selectedFilter === 'This Quarter') {
      result = result.filter(p => isThisQuarter(p.payment_date));
    }

    filteredPayments = result.sort((a, b) =>
      new Date(b.payment_date).getTime() - new Date(a.payment_date).getTime()
    );
  }

  // Computed: KPI stats
  $: stats = {
    totalCollectedYTD: payments.filter((p) => isCurrentYear(p.payment_date)).reduce((sum, p) => sum + p.amount_bhd, 0),
    paymentsThisMonth: payments.filter(p => isThisMonth(p.payment_date)).length,
    avgDaysToPayment: filteredPayments.length > 0
      ? filteredPayments.reduce((sum, p) => sum + p.days_to_payment, 0) / filteredPayments.length
      : 0,
    count: filteredPayments.length
  };

  // Computed: Update selected invoice when dropdown changes
  $: if (paymentModalMode === 'create') {
    if (selectedInvoiceId) {
      selectedInvoice = invoices.find(inv => inv.id === selectedInvoiceId) || null;
      if (selectedInvoice) {
        // Pre-fill payment amount with outstanding balance
        paymentAmount = selectedInvoice.outstanding_bhd.toFixed(3);
      }
    } else {
      selectedInvoice = null;
      paymentAmount = '';
    }
  }

  $: if (paymentModalMode === 'create' && receiptMode === 'on_account') {
    selectedCustomer = customers.find((customer) => customer.id === selectedCustomerId || customer.customer_id === selectedCustomerId) || null;
  }

  // Filter invoices for dropdown search (debounced for performance)
  $: filteredInvoicesForDropdown = debouncedInvoiceSearch
    ? invoices.filter(inv => {
        const query = debouncedInvoiceSearch.toLowerCase();
        return (
          inv.invoice_number?.toLowerCase().includes(query) ||
          inv.customer_name?.toLowerCase().includes(query)
        );
      })
    : invoices;

  $: filteredCustomersForDropdown = customerSearchQuery
    ? customers.filter((customer) => {
        const query = customerSearchQuery.toLowerCase();
        return (
          customer.business_name?.toLowerCase().includes(query) ||
          customer.customer_id?.toLowerCase().includes(query) ||
          customer.customer_code?.toLowerCase().includes(query)
        );
      })
    : customers;

  // Currency formatter - BHD with 3 decimal places
  function formatBHD(amount: number): string {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    }).format(Number(amount) || 0);
  }

  function toInputDate(dateStr: any): string {
    if (!dateStr) return new Date().toISOString().split('T')[0];
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return new Date().toISOString().split('T')[0];
    return date.toISOString().split('T')[0];
  }

  function openBankReconciliation() {
    dispatch('navigate', { tab: 'bank_recon', source: 'payments' });
  }

  // Format date - handles both string and Go Time object
  function formatDate(dateStr: any): string {
    if (!dateStr) return '—';
    // Convert to string if it's an object (Go Time type)
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return '—';
    return date.toLocaleDateString('en-GB', {
      day: '2-digit',
      month: 'short',
      year: 'numeric'
    });
  }

  // Check if date is this month - handles both string and Go Time object
  function isThisMonth(dateStr: any): boolean {
    if (!dateStr) return false;
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return false;
    const now = new Date();
    return date.getMonth() === now.getMonth() && date.getFullYear() === now.getFullYear();
  }

  // Check if date is this quarter - handles both string and Go Time object
  function isThisQuarter(dateStr: any): boolean {
    if (!dateStr) return false;
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return false;
    const now = new Date();
    const currentQuarter = Math.floor(now.getMonth() / 3);
    const dateQuarter = Math.floor(date.getMonth() / 3);
    return dateQuarter === currentQuarter && date.getFullYear() === now.getFullYear();
  }

  function isCurrentYear(dateStr: any): boolean {
    if (!dateStr) return false;
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return false;
    return date.getFullYear() === new Date().getFullYear();
  }

  // Load payments and unpaid invoices with pagination
  async function loadData() {
    loading = true;
    currentPage = 0;
    hasMore = true;
    try {
      const [paymentsData, invoicesData, receiptData, customerData] = await Promise.all([
        GetAllPayments(PAGE_SIZE, 0),
        ListCustomerInvoices(500, 0), // Load more invoices for dropdown
        ListCustomerReceipts(500, 0).catch(() => []),
        GetAllCustomers().catch(() => [])
      ]);

      const receiptRows = ((receiptData || []) as CustomerReceipt[])
        .filter((receipt) => matchesCompany(receipt.division))
        .map((receipt) => ({
          id: receipt.id,
          source_kind: 'receipt' as const,
          receipt_id: receipt.id,
          receipt_number: receipt.receipt_number,
          receipt_status: receipt.status,
          unapplied_amount_bhd: Number(receipt.unapplied_amount_bhd || 0),
          customer_id: receipt.customer_id,
          invoice_id: '',
          invoice_number: receipt.receipt_number || 'On-account receipt',
          customer_name: receipt.customer_name,
          amount_bhd: Number(receipt.amount_bhd || 0),
          payment_date: receipt.receipt_date,
          payment_method: receipt.payment_method,
          days_to_payment: 0,
          reference: receipt.reference || '',
          division: receipt.division,
        }));
      const receiptIds = new Set(receiptRows.map((receipt) => receipt.id));
      const legacyPaymentRows = ((paymentsData || []) as Payment[])
        .filter((payment) => matchesCompany(payment.division))
        .filter((payment) => !payment.receipt_id || !receiptIds.has(payment.receipt_id))
        .map((payment) => ({ ...payment, source_kind: 'payment' as const }));
      payments = [...receiptRows, ...legacyPaymentRows];
      customers = ((customerData || []) as CustomerOption[]).filter((customer) => matchesCompany(customer.division));
      if (selectedPaymentId) {
        selectedPayment = payments.find((payment) => payment.id === selectedPaymentId) || null;
        if (!selectedPayment) {
          selectedPaymentId = '';
          deleteConfirmPaymentId = '';
        }
      }
      currentPage = 1;
      totalLoaded = payments.length;
      hasMore = payments.length === PAGE_SIZE;

      // Filter to only unpaid/partially paid invoices for dropdown
      invoices = (invoicesData || []).filter((inv) => {
        const status = String(inv?.status || '').toLowerCase();
        return matchesCompany(inv?.division) && Number(inv?.outstanding_bhd || 0) > 0 && !['draft', 'cancelled', 'void', 'proforma'].includes(status);
      });

      console.log(`Loaded ${payments.length} receipts (page 1), ${invoices.length} open invoices, ${customers.length} customers`);
    } catch (err) {
      console.error('Failed to load payment data:', err);
      toast.danger('Failed to load receipt data');
      payments = [];
      invoices = [];
      customers = [];
      hasMore = false;
    } finally {
      loading = false;
    }
  }

  // Load more payments (pagination)
  async function loadMore() {
    if (loadingMore || !hasMore) return;

    loadingMore = true;
    try {
      const offset = currentPage * PAGE_SIZE;
      const data = await GetAllPayments(PAGE_SIZE, offset);

      if (data && data.length > 0) {
        const existingReceiptIds = new Set(payments.map((payment) => payment.receipt_id).filter(Boolean));
        payments = [
          ...payments,
          ...data
            .filter((payment) => matchesCompany(payment.division))
            .filter((payment) => !payment.receipt_id || !existingReceiptIds.has(payment.receipt_id))
            .map((payment) => ({ ...payment, source_kind: 'payment' as const }))
        ];
        currentPage++;
        totalLoaded = payments.length;
        hasMore = data.length === PAGE_SIZE;
      console.log(`Loaded ${data.length} more receipts (total: ${totalLoaded})`);
      } else {
        hasMore = false;
      }
    } catch (err) {
      console.error('Failed to load more payments:', err);
      toast.danger('Failed to load more receipts');
    } finally {
      loadingMore = false;
    }
  }

  // Open record payment modal
  function openRecordModal() {
    if (!canCreate) {
      toast.warning('You do not have permission to apply customer receipts');
      return;
    }

    paymentModalMode = 'create';
    receiptMode = invoices.length > 0 ? 'invoice' : 'on_account';
    editingPaymentId = '';
    // Reset form
    selectedInvoiceId = '';
    selectedInvoice = null;
    selectedCustomerId = '';
    selectedCustomer = null;
    customerSearchQuery = '';
    paymentAmount = '';
    paymentMethod = 'Bank Transfer';
    paymentDate = new Date().toISOString().split('T')[0];
    paymentReference = '';
    invoiceSearchQuery = '';

    showRecordModal = true;
  }

  function handlePaymentRowClick(row: Payment) {
    if (selectedPaymentId === row.id) {
      selectedPaymentId = '';
      selectedPayment = null;
      deleteConfirmPaymentId = '';
      return;
    }

    selectedPaymentId = row.id;
    selectedPayment = row;
    deleteConfirmPaymentId = '';
  }

  async function openEditModal() {
    if (!selectedPayment || !canUpdate) {
      return;
    }
    if (selectedPayment.source_kind === 'receipt') {
      toast.info('Receipt headers are audit records. Apply on-account balances from the receipt workflow instead.');
      return;
    }

    if ((paymentMethod === 'Bank Transfer' || paymentMethod === 'Wire Transfer') && !paymentReference.trim()) {
      toast.warning('Reference is required for bank transfers');
      return;
    }

    recordLoading = true;
    try {
      const payment = await GetPayment(selectedPayment.id);
      paymentModalMode = 'edit';
      receiptMode = 'invoice';
      editingPaymentId = payment.id;
      selectedInvoiceId = payment.invoice_id || selectedPayment.invoice_id || '';
      selectedInvoice = {
        id: selectedInvoiceId,
        invoice_number: payment.invoice_number || selectedPayment.invoice_number,
        customer_name: selectedPayment.customer_name || '—',
        grand_total_bhd: 0,
        outstanding_bhd: 0,
        status: '',
        invoice_date: null,
      };
      paymentAmount = Number(payment.amount_bhd || 0).toFixed(3);
      paymentMethod = payment.payment_method || 'Bank Transfer';
      paymentDate = toInputDate(payment.payment_date);
      paymentReference = payment.reference || '';
      showRecordModal = true;
    } catch (err) {
      console.error('Failed to load receipt for editing:', err);
      toast.danger('Failed to load receipt details');
    } finally {
      recordLoading = false;
    }
  }

  async function handleDeleteSelectedPayment() {
    if (!selectedPayment || !canDelete) {
      return;
    }
    if (selectedPayment.source_kind === 'receipt') {
      toast.warning('Receipt headers are protected audit records and cannot be deleted here');
      return;
    }

    if (deleteConfirmPaymentId !== selectedPayment.id) {
      deleteConfirmPaymentId = selectedPayment.id;
      return;
    }

    recordLoading = true;
    try {
      await DeletePayment(selectedPayment.id);
      toast.success(`Deleted receipt ${selectedPayment.invoice_number || ''}`.trim());
      selectedPaymentId = '';
      selectedPayment = null;
      deleteConfirmPaymentId = '';
      await loadData();
    } catch (err) {
      console.error('Failed to delete receipt:', err);
      toast.danger(`Failed to delete receipt: ${String(err)}`);
    } finally {
      recordLoading = false;
    }
  }

  // Record payment
  async function handleRecordPayment() {
    // Validation
    if (paymentModalMode === 'create' && receiptMode === 'invoice' && !selectedInvoiceId) {
      toast.warning('Please select an invoice');
      return;
    }
    if (paymentModalMode === 'create' && receiptMode === 'on_account' && !selectedCustomerId) {
      toast.warning('Please select a customer');
      return;
    }

    const amount = parseFloat(paymentAmount);
    if (isNaN(amount) || amount <= 0) {
      toast.warning('Please enter a valid receipt amount');
      return;
    }

    if (!selectedInvoice && paymentModalMode === 'create' && receiptMode === 'invoice') {
      toast.danger('Selected invoice not found');
      return;
    }

    if (paymentModalMode === 'create' && receiptMode === 'invoice' && selectedInvoice && amount > selectedInvoice.outstanding_bhd) {
      toast.warning(`Receipt amount (${formatBHD(amount)} BHD) exceeds open balance (${formatBHD(selectedInvoice.outstanding_bhd)} BHD)`);
      return;
    }

    if (!paymentDate) {
      toast.warning('Please select a receipt date');
      return;
    }

    if ((paymentMethod === 'Bank Transfer' || paymentMethod === 'Wire Transfer') && !paymentReference.trim()) {
      toast.warning('Reference is required for bank transfers');
      return;
    }

    recordLoading = true;
    try {
      if (paymentModalMode === 'create') {
        await CreateCustomerReceipt({
          invoice_id: receiptMode === 'invoice' ? selectedInvoiceId : '',
          customer_id: receiptMode === 'on_account' ? selectedCustomerId : '',
          customer_name: receiptMode === 'on_account' ? (selectedCustomer?.business_name || '') : '',
          amount_bhd: amount,
          receipt_date: paymentDate,
          payment_method: paymentMethod,
          reference: paymentReference.trim(),
          division: company,
          notes: receiptMode === 'on_account' ? 'Received on account' : '',
        });
        toast.success(
          receiptMode === 'invoice'
            ? `Receipt of ${formatBHD(amount)} BHD applied successfully`
            : `On-account receipt of ${formatBHD(amount)} BHD recorded`
        );
      } else {
        const existingPayment = await GetPayment(editingPaymentId);
        const originalDate = toWailsDate(existingPayment.payment_date) || new Date();
        const nextDate = new Date(`${paymentDate}T00:00:00`);
        const inferredInvoiceDate = new Date(originalDate.getTime() - ((existingPayment.days_to_payment || 0) * 24 * 60 * 60 * 1000));
        const recalculatedDaysToPayment = Math.max(
          0,
          Math.round((nextDate.getTime() - inferredInvoiceDate.getTime()) / (24 * 60 * 60 * 1000))
        );

        await UpdatePayment(editingPaymentId, buildWailsInput(main.Payment, {
          ...existingPayment,
          amount_bhd: amount,
          payment_method: paymentMethod,
          payment_date: nextDate.toISOString(),
          reference: paymentReference.trim(),
          days_to_payment: recalculatedDaysToPayment,
        }));
        toast.success(`Receipt updated to ${formatBHD(amount)} BHD`);
      }

      showRecordModal = false;
      deleteConfirmPaymentId = '';

      // Reload data to reflect changes
      await loadData();
    } catch (err) {
      console.error('Failed to apply receipt:', err);
      toast.danger('Failed to apply receipt. Please try again.');
    } finally {
      recordLoading = false;
    }
  }

  // Handle quick amount selection (full outstanding)
  function selectFullOutstanding() {
    if (selectedInvoice) {
      paymentAmount = selectedInvoice.outstanding_bhd.toFixed(3);
    }
  }

  onMount(() => {
    if (!canView) return;
    loadData();
  });

  $: if (company && canView) {
    loadData();
  }
</script>

<PageLayout
  title="Receipts"
  subtitle="Customer receipts and cash application"
  {embedded}
>
  <svelte:fragment slot="header-actions">
    <Button variant="secondary" on:click={loadData}>
      Refresh
    </Button>
    <Button variant="secondary" on:click={openBankReconciliation}>
      Open Bank Recon
    </Button>
    <Button variant="primary" on:click={openRecordModal} disabled={!canCreate}>
      + Apply Receipt
    </Button>
  </svelte:fragment>

  <div class="payments-container">
    <!-- KPI Stats -->
    <div class="kpi-grid">
      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Total Collected (YTD)</div>
          <div class="kpi-value kpi-success">{formatBHD(stats.totalCollectedYTD)} <span class="kpi-currency">BHD</span></div>
          <div class="kpi-sublabel">{new Date().getFullYear()} collections</div>
        </div>
      </Card>

      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Receipts This Month</div>
          <div class="kpi-value">{stats.paymentsThisMonth}</div>
          <div class="kpi-sublabel">Current month cash application</div>
        </div>
      </Card>

      <Card padding="md" variant="elevated">
        <div class="kpi-card">
          <div class="kpi-label">Avg Days to Collection</div>
          <div class="kpi-value" class:kpi-success={stats.avgDaysToPayment <= 30} class:kpi-warning={stats.avgDaysToPayment > 30 && stats.avgDaysToPayment <= 60} class:kpi-danger={stats.avgDaysToPayment > 60}>
            {stats.avgDaysToPayment.toFixed(1)}<span class="kpi-unit">d</span>
          </div>
          <div class="kpi-sublabel">Collection velocity</div>
        </div>
      </Card>
    </div>

    <!-- Filter Tabs -->
    <Card padding="sm">
      <div class="filter-tabs" role="tablist" aria-label="Filter receipts by time">
        {#each filterTabs as tab}
          <button
            class="filter-tab"
            class:active={selectedFilter === tab.value}
            role="tab"
            aria-selected={selectedFilter === tab.value}
            on:click={() => selectedFilter = tab.value}
          >
            {tab.label}
            <span class="tab-count">{tab.count}</span>
          </button>
        {/each}
      </div>
    </Card>

    <Card padding="sm">
      <div class="receipts-toolbar">
        <div>
          <strong>Add customer receipt</strong>
          <span>Apply to an invoice or hold customer money on account.</span>
        </div>
        <Button variant="primary" on:click={openRecordModal} disabled={!canCreate}>
          + Add Receipt
        </Button>
      </div>
    </Card>

    <!-- Payments DataTable -->
    <Card padding="sm">
      {#if selectedPayment}
        <div class="selection-bar">
          <div class="selection-copy">
            <span class="selection-label">Selected Receipt</span>
            <span class="selection-title">{selectedPayment.invoice_number} · {selectedPayment.customer_name || 'Customer receipt'}</span>
            <span class="selection-meta">{formatBHD(selectedPayment.amount_bhd)} BHD on {formatDate(selectedPayment.payment_date)}</span>
          </div>
          <div class="selection-actions">
            {#if canUpdate}
              <Button variant="secondary" size="sm" on:click={openEditModal} disabled={recordLoading}>
                Edit
              </Button>
            {/if}
            {#if canDelete}
              <Button variant={deleteConfirmPaymentId === selectedPayment.id ? 'primary' : 'ghost'} size="sm" on:click={handleDeleteSelectedPayment} disabled={recordLoading}>
                {deleteConfirmPaymentId === selectedPayment.id ? 'Confirm Delete' : 'Delete'}
              </Button>
            {/if}
          </div>
        </div>
      {/if}

      {#if loading}
        <div class="loading-container">
          <WabiSpinner size="lg" />
          <p>Loading receipts...</p>
        </div>
      {:else}
        {#if filteredPayments.length === 0}
          <div class="empty-receipts-cta">
            <div>
              <strong>No receipts recorded yet.</strong>
              <span>Start with an invoice receipt or an on-account customer receipt.</span>
            </div>
            <Button variant="primary" on:click={openRecordModal} disabled={!canCreate}>
              + Add Receipt
            </Button>
          </div>
        {/if}
        <DataTable
          {columns}
          data={filteredPayments}
          {loading}
          emptyMessage="No receipts recorded yet."
          selectedId={selectedPaymentId}
          onRowClick={handlePaymentRowClick}
          stickyHeader={!embedded}
          maxHeight={embedded ? "400px" : "calc(100vh - 480px)"}
          showBorder={false}
        />
      {/if}
    </Card>

    <!-- Pagination Controls -->
    {#if hasMore && !loading}
      <div class="pagination-controls">
        <button
          class="load-more-btn"
          on:click={loadMore}
          disabled={loadingMore}
          aria-label={loadingMore ? 'Loading more receipts' : `Load more receipts, ${totalLoaded} currently loaded`}
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
      <p class="all-loaded">All {totalLoaded} receipts loaded</p>
    {/if}
  </div>
</PageLayout>

<!-- Apply Receipt Modal -->
{#if showRecordModal}
  <Modal
    title={paymentModalMode === 'create' ? 'Apply Customer Receipt' : 'Edit Customer Receipt'}
    open={showRecordModal}
    on:close={() => {
      showRecordModal = false;
      paymentModalMode = 'create';
      editingPaymentId = '';
    }}
    size="md"
  >
    <div class="record-form">
      {#if paymentModalMode === 'create'}
        <div class="receipt-mode-switch" role="tablist" aria-label="Receipt mode">
          <button
            type="button"
            class:active={receiptMode === 'invoice'}
            on:click={() => {
              receiptMode = 'invoice';
              selectedCustomerId = '';
              selectedCustomer = null;
            }}
            disabled={recordLoading}
          >
            Apply to Invoice
          </button>
          <button
            type="button"
            class:active={receiptMode === 'on_account'}
            on:click={() => {
              receiptMode = 'on_account';
              selectedInvoiceId = '';
              selectedInvoice = null;
              invoiceSearchQuery = '';
              paymentAmount = '';
            }}
            disabled={recordLoading}
          >
            On Account
          </button>
        </div>
      {/if}

      <!-- Invoice Selection with Search -->
      <FormGroup label={receiptMode === 'invoice' ? 'Select Open Invoice' : 'Select Customer'} required>
        {#if paymentModalMode === 'create' && receiptMode === 'invoice'}
          <!-- Search input -->
          <Input
            type="text"
            bind:value={invoiceSearchQuery}
            placeholder="Search by invoice # or customer..."
            disabled={recordLoading}
          />

          <!-- Filtered dropdown -->
          <select
            class="select-input"
            bind:value={selectedInvoiceId}
            disabled={recordLoading}
            style="margin-top: 8px;"
          >
            <option value="">-- Select an open invoice ({filteredInvoicesForDropdown.length} shown) --</option>
            {#each filteredInvoicesForDropdown as invoice}
              <option value={invoice.id}>
                {invoice.invoice_number} - {invoice.customer_name} ({formatBHD(invoice.outstanding_bhd)} BHD open)
              </option>
            {/each}
          </select>

          {#if invoiceSearchQuery && filteredInvoicesForDropdown.length === 0}
            <p class="no-results" style="color: var(--text-muted); margin-top: 4px; font-size: 0.85rem;">
              No invoices match "{invoiceSearchQuery}"
            </p>
          {/if}
        {:else if paymentModalMode === 'create' && receiptMode === 'on_account'}
          <Input
            type="text"
            bind:value={customerSearchQuery}
            placeholder="Search customer name or code..."
            disabled={recordLoading}
          />

          <select
            class="select-input"
            bind:value={selectedCustomerId}
            disabled={recordLoading}
            style="margin-top: 8px;"
          >
            <option value="">-- Select a customer ({filteredCustomersForDropdown.length} shown) --</option>
            {#each filteredCustomersForDropdown as customer}
              <option value={customer.id}>
                {customer.business_name} {customer.customer_code ? `(${customer.customer_code})` : ''}
              </option>
            {/each}
          </select>

          {#if customerSearchQuery && filteredCustomersForDropdown.length === 0}
            <p class="no-results" style="color: var(--text-muted); margin-top: 4px; font-size: 0.85rem;">
              No customers match "{customerSearchQuery}"
            </p>
          {/if}
        {:else if selectedPayment}
          <div class="locked-field">
            <div class="locked-title">{selectedPayment.invoice_number}</div>
            <div class="locked-subtitle">{selectedPayment.customer_name || 'Customer receipt'}</div>
          </div>
        {/if}
      </FormGroup>

      <!-- Invoice Details Display -->
      {#if selectedInvoice}
        <div class="invoice-details" transition:fade={{ duration: 200 }}>
          <div class="detail-row">
            <span class="detail-label">Customer:</span>
            <span class="detail-value">{selectedInvoice.customer_name}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Invoice Total:</span>
            <span class="detail-value detail-mono">{formatBHD(selectedInvoice.grand_total_bhd)} BHD</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Open Balance:</span>
            <span class="detail-value detail-mono detail-highlight">{formatBHD(selectedInvoice.outstanding_bhd)} BHD</span>
          </div>
          {#if selectedInvoice.invoice_date}
            <div class="detail-row">
              <span class="detail-label">Invoice Date:</span>
              <span class="detail-value">{formatDate(selectedInvoice.invoice_date)}</span>
            </div>
          {/if}
          <button class="quick-fill-btn" on:click={selectFullOutstanding} disabled={recordLoading}>
            Apply Full Open Balance
          </button>
        </div>
      {/if}

      <!-- Amount Input -->
      <FormGroup label="Receipt Amount (BHD)" required>
        <Input
          type="number"
          step="0.001"
          min="0.001"
          placeholder="0.000"
          bind:value={paymentAmount}
          disabled={recordLoading || (paymentModalMode === 'create' && receiptMode === 'invoice' && !selectedInvoiceId) || (paymentModalMode === 'create' && receiptMode === 'on_account' && !selectedCustomerId)}
        />
        {#if selectedInvoice && paymentAmount}
          <div class="amount-helper">
            {#if parseFloat(paymentAmount) > selectedInvoice.outstanding_bhd}
              <span class="helper-warning">Amount exceeds open balance</span>
            {:else if parseFloat(paymentAmount) === selectedInvoice.outstanding_bhd}
              <span class="helper-success">Full settlement (invoice will be marked as settled)</span>
            {:else}
              <span class="helper-info">Partial settlement (remaining: {formatBHD(selectedInvoice.outstanding_bhd - parseFloat(paymentAmount))} BHD open)</span>
            {/if}
          </div>
        {/if}
      </FormGroup>

      <!-- Receipt Method -->
      <FormGroup label="Receipt Method" required>
        <select
          class="select-input"
          bind:value={paymentMethod}
          disabled={recordLoading}
        >
          {#each paymentMethods as method}
            <option value={method}>{method}</option>
          {/each}
        </select>
      </FormGroup>

      <!-- Receipt Date -->
      <FormGroup label="Receipt Date" required>
        <Input
          type="date"
          bind:value={paymentDate}
          disabled={recordLoading}
          max={new Date().toISOString().split('T')[0]}
        />
      </FormGroup>

      <!-- Reference -->
      <FormGroup label="Reference" hint="Bank ref, cheque number, deposit ref, etc. (optional)">
        <Input
          type="text"
          placeholder="e.g., TXN123456, CHQ001"
          bind:value={paymentReference}
          disabled={recordLoading}
        />
      </FormGroup>
    </div>

    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => {
        showRecordModal = false;
        paymentModalMode = 'create';
        editingPaymentId = '';
      }} disabled={recordLoading}>
        Cancel
      </Button>
      <Button
        variant="primary"
        on:click={handleRecordPayment}
        disabled={recordLoading || !paymentAmount || (paymentModalMode === 'create' && receiptMode === 'invoice' && !selectedInvoiceId) || (paymentModalMode === 'create' && receiptMode === 'on_account' && !selectedCustomerId) || (paymentModalMode === 'create' && receiptMode === 'invoice' && selectedInvoice && parseFloat(paymentAmount) > selectedInvoice.outstanding_bhd)}
      >
        {#if recordLoading}
          {paymentModalMode === 'create' ? 'Applying...' : 'Saving...'}
        {:else}
          {paymentModalMode === 'create' ? 'Apply Receipt' : 'Save Changes'}
        {/if}
      </Button>
    </svelte:fragment>
  </Modal>
{/if}

<style>
  .payments-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .receipts-toolbar,
  .empty-receipts-cta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
  }

  .receipts-toolbar strong,
  .empty-receipts-cta strong {
    display: block;
    font-size: 14px;
    color: var(--text-primary);
  }

  .receipts-toolbar span,
  .empty-receipts-cta span {
    display: block;
    margin-top: 3px;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .empty-receipts-cta {
    padding: 18px;
    margin-bottom: 10px;
    border: 1px dashed var(--border);
    border-radius: 10px;
    background: var(--surface-subtle, #f8faf8);
  }

  .receipt-mode-switch {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px;
    padding: 4px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--surface-subtle, #f8f7f4);
  }

  .receipt-mode-switch button {
    border: 0;
    border-radius: 8px;
    padding: 10px 12px;
    background: transparent;
    color: var(--text-secondary);
    font-weight: 700;
    cursor: pointer;
  }

  .receipt-mode-switch button.active {
    background: var(--brand-primary, #4f8f45);
    color: #fff;
  }

  .selection-bar {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    align-items: center;
    padding: 10px 12px 14px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 8px;
  }

  .selection-copy {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .selection-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    font-weight: 600;
  }

  .selection-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .selection-meta {
    font-size: 12px;
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }

  .selection-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .locked-field {
    border: 1px solid var(--border);
    background: var(--surface-subtle, #f8f7f4);
    border-radius: 10px;
    padding: 12px 14px;
  }

  .locked-title {
    font-weight: 600;
    color: var(--text-primary);
    font-family: var(--font-mono);
  }

  .locked-subtitle {
    margin-top: 4px;
    font-size: 13px;
    color: var(--text-secondary);
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

  .kpi-currency,
  .kpi-unit {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-secondary);
    margin-left: 4px;
  }

  .kpi-success {
    color: #10B981;
  }

  .kpi-warning {
    color: #F59E0B;
  }

  .kpi-danger {
    color: #EF4444;
  }

  .kpi-sublabel {
    font-size: 12px;
    color: var(--text-secondary);
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

  /* Record Form */
  .record-form {
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

  /* Invoice Details */
  .invoice-details {
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius-sm);
    border-left: 3px solid var(--brand-indigo);
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
  }

  .detail-label {
    font-weight: 500;
    color: var(--text-secondary);
  }

  .detail-value {
    font-weight: 600;
    color: var(--text-primary);
  }

  .detail-mono {
    font-family: var(--font-mono);
    font-size: 14px;
  }

  .detail-highlight {
    color: var(--brand-indigo);
    font-size: 16px;
  }

  .quick-fill-btn {
    margin-top: 8px;
    padding: 8px 14px;
    background: var(--brand-indigo);
    color: white;
    border: none;
    border-radius: var(--border-radius-sm);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .quick-fill-btn:hover:not(:disabled) {
    background: var(--brand-indigo-hover);
    transform: translateY(-1px);
  }

  .quick-fill-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Amount Helper */
  .amount-helper {
    margin-top: 6px;
    font-size: 12px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .helper-warning {
    color: #F59E0B;
    font-weight: 500;
  }

  .helper-success {
    color: #10B981;
    font-weight: 500;
  }

  .helper-info {
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

  /* Responsive */
  @media (max-width: 768px) {
    .kpi-grid {
      grid-template-columns: 1fr;
    }

    .filter-tabs {
      flex-wrap: wrap;
    }
  }
</style>
