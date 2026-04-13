<script lang="ts">
  /**
   * SupplierInvoicesScreen - Production-Ready Supplier Invoice Management
   * Features:
   * - Full CRUD for supplier invoices
   * - 3-way matching (PO ↔ GRN ↔ Invoice)
   * - Match status visualization (Matched, Partial, Mismatch, Pending)
   * - Payment tracking (Pending, Partial, Paid, Overdue)
   * - Multi-currency support with BHD conversion
   * - Approval workflow (Pending → Matched → Approved → Paid)
   * - Overdue tracking with visual warnings
   */

  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';
  import type { main } from '../../../wailsjs/go/models';

  // Wails API imports
  import {
    GetSupplierInvoices,
    GetSupplierInvoiceByID,
    GetSupplierPaymentsByInvoice,
    CreateSupplierInvoice,
    UpdateSupplierInvoice,
    UpdateSupplierInvoiceWithPayment,
    DeleteSupplierInvoice,
    PerformThreeWayMatch,
    ApproveSupplierInvoice,
    MarkSupplierInvoicePaid,
    GetUnpaidSupplierInvoices,
    GetOverdueSupplierInvoices,
    ListSuppliers,
    ListOrders,
    GetSupportedCurrencies
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
  import { toast } from '$lib/stores/toasts';
  import { permissions } from '$lib/stores/authContext';

  $: canView = $permissions.includes('*') || $permissions.includes('po:view') || $permissions.includes('po:*');
  const dispatch = createEventDispatcher();

  // Props
  export let embedded = false;

  // Types
  type InvoiceFilter = 'all' | 'pending' | 'matched' | 'discrepancy' | 'paid' | 'overdue';

  function getErrorMessage(err: unknown): string {
    if (!err) return 'Unknown error';
    if (err instanceof Error && err.message) return err.message;
    if (typeof err === 'string') return err;
    if (typeof err === 'object') {
      const candidate = (err as any).message || (err as any).error || (err as any).cause;
      if (candidate) return String(candidate);
    }
    return String(err);
  }

  function roundAmount(value: number): number {
    return Math.round((Number(value) || 0) * 1000) / 1000;
  }

  function formatMoney(value: number): string {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    }).format(Number(value) || 0);
  }

  function formatDateForApi(value: string): string | null {
    if (!value) return null;
    return `${value}T00:00:00Z`;
  }

  function openBankReconciliation() {
    dispatch('navigate', { tab: 'bank_recon', source: 'supplier_invoices' });
  }

  interface SupplierInvoiceDisplay extends main.SupplierInvoice {
    // Computed fields
    days_until_due?: number;
    is_overdue?: boolean;
    outstanding_bhd?: number;
    // Note: supplier_name already exists in main.SupplierInvoice
  }

  // State
  let invoices: SupplierInvoiceDisplay[] = [];
  let filteredInvoices: SupplierInvoiceDisplay[] = [];
  let loading = true;
  let selectedFilter: InvoiceFilter = 'all';
  let suppliersMap: Map<string, string> = new Map(); // supplier_id -> supplier_name
  let suppliersList: any[] = []; // Full supplier objects for dropdown
  let ordersList: any[] = []; // Orders for internal reference dropdown

  // Search and date filter state
  let searchQuery = '';
  let dateFilter = 'all'; // 'all', 'this_month', 'last_month', 'this_quarter', 'this_year'

  // Supported currencies (dynamically loaded)
  let supportedCurrencies: any[] = [];

  // Modal state
  let showCreateModal = false;
  let showEditModal = false;
  let showDetailModal = false;
  let showMatchModal = false;
  let showPaymentModal = false;
  let editingInvoice: any = null;
  let selectedInvoice: SupplierInvoiceDisplay | null = null;
  let invoiceToMarkPaid: string | null = null;
  let createPaymentEntry = false;
  let existingPaymentCount = 0;
  let existingPaymentTotalBhd = 0;

  // Line item interface for form
  interface FormLineItem {
    description: string;
    quantity: number;
    unit_price: number;
    total_price: number;
  }

  // Form state
  let formData = {
    supplier_id: '',
    purchase_order_id: '',
    grn_id: '',
    order_id: '',
    invoice_number: '',
    invoice_date: new Date().toISOString().split('T')[0],
    due_date: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
    currency: 'BHD',
    exchange_rate: 1.0,
    subtotal_foreign: 0,
    vat_foreign: 0,
    total_foreign: 0
  };
  let formItems: FormLineItem[] = [{ description: '', quantity: 1, unit_price: 0, total_price: 0 }];
  let formLoading = false;

  // Payment form state
  let paymentData = {
    payment_reference: '',
    payment_method: 'Bank Transfer'
  };
  let paymentLoading = false;

  // System user (fallback for "Current User")
  const systemUser = 'System Admin';

  // DataTable columns configuration
  const columns = [
    {
      key: 'invoice_number',
      label: 'Invoice #',
      sortable: true,
      width: '110px',
      render: (row: SupplierInvoiceDisplay) => {
        return `<span class="invoice-num">${row.invoice_number}</span>`;
      }
    },
    {
      key: 'supplier_id',
      label: 'Supplier',
      sortable: true,
      width: '150px',
      render: (row: SupplierInvoiceDisplay) => {
        let name = row.supplier_name || '';
        // Detect UUID-like values and show friendly fallback
        if (!name || /^[0-9a-f]{8}-/i.test(name)) {
          name = 'Unknown Supplier';
        }
        return `<span class="supplier-name" title="${row.supplier_id}">${name}</span>`;
      }
    },
    {
      key: 'purchase_order_id',
      label: 'PO Reference',
      sortable: true,
      width: '100px',
      render: (row: SupplierInvoiceDisplay) => {
        if (!row.purchase_order_id) {
          return '<span style="color: var(--text-muted); font-style: italic;">No PO</span>';
        }
        return `<span class="po-ref">${row.purchase_order_id.slice(0, 8)}</span>`;
      }
    },
    {
      key: 'grn_id',
      label: 'GRN Reference',
      sortable: true,
      width: '100px',
      render: (row: SupplierInvoiceDisplay) => {
        if (!row.grn_id) {
          return '<span style="color: var(--text-muted); font-style: italic;">No GRN</span>';
        }
        return `<span class="grn-ref">${row.grn_id.slice(0, 8)}</span>`;
      }
    },
    {
      key: 'currency',
      label: 'Currency',
      sortable: true,
      width: '72px'
    },
    {
      key: 'total_bhd',
      label: 'Amount (BHD)',
      type: 'currency' as const,
      align: 'right' as const,
      sortable: true,
      width: '110px',
      render: (row: SupplierInvoiceDisplay) => {
        const outstanding = Number(row.outstanding_bhd || 0);
        const baseAmount = Number(row.total_bhd || 0);
        if (outstanding > 0 && Math.abs(outstanding - baseAmount) > 0.0001) {
          return `<span class="amount-bhd">${formatMoney(outstanding)} <span class="amount-caption">open</span></span>`;
        }
        return `<span class="amount-bhd">${formatMoney(baseAmount)}</span>`;
      }
    },
    {
      key: 'match_status',
      label: 'Match Status',
      sortable: true,
      width: '112px',
      render: (row: SupplierInvoiceDisplay) => {
        return renderMatchStatus(row);
      }
    },
    {
      key: 'payment_status',
      label: 'Payment Status',
      sortable: true,
      width: '110px',
      render: (row: SupplierInvoiceDisplay) => {
        return renderPaymentStatus(row);
      }
    },
    {
      key: 'due_date',
      label: 'Due Date',
      type: 'date' as const,
      sortable: true,
      width: '110px',
      render: (row: SupplierInvoiceDisplay) => {
        // Handle time.Time object from Go
        const dueDate = row.due_date ? new Date(row.due_date.toString()) : new Date();
        const date = dueDate.toLocaleDateString('en-US', {
          year: 'numeric',
          month: 'short',
          day: 'numeric'
        });

        if (row.is_overdue) {
          return `<span style="color: #DC2626; font-weight: 500;">${date} OVERDUE</span>`;
        } else if (row.days_until_due && row.days_until_due <= 7) {
          return `<span style="color: #F59E0B; font-weight: 500;">${date}</span>`;
        }
        return date;
      }
    },
    {
      key: 'actions',
      label: 'Actions',
      type: 'actions' as const,
      width: '150px',
      render: (row: SupplierInvoiceDisplay) => {
        return `
          <div style="display: flex; gap: 6px; justify-content: flex-start; flex-wrap: wrap;">
            <button
              class="action-btn action-btn-view"
              data-action="view"
              data-id="${row.id}"
              aria-label="View invoice details"
              title="View Details"
            >
              View
            </button>
            <button
              class="action-btn action-btn-edit"
              data-action="edit"
              data-id="${row.id}"
              aria-label="Edit invoice"
              title="Edit"
            >
              Edit
            </button>
            ${row.match_status === 'Pending' ? `
              <button
                class="action-btn action-btn-match"
                data-action="match"
                data-id="${row.id}"
                aria-label="Perform 3-way match"
                title="3-Way Match"
              >
                Match
              </button>
            ` : ''}
            ${row.status === 'Pending' && row.match_status === 'Matched' ? `
              <button
                class="action-btn action-btn-approve"
                data-action="approve"
                data-id="${row.id}"
                aria-label="Approve invoice"
                title="Approve"
              >
                Approve
              </button>
            ` : ''}
            ${row.status === 'Approved' && Number(row.outstanding_bhd || 0) > 0 ? `
              <button
                class="action-btn action-btn-pay"
                data-action="pay"
                data-id="${row.id}"
                aria-label="Mark as paid"
                title="Mark Paid"
              >
                Pay
              </button>
            ` : ''}
          </div>
        `;
      }
    }
  ];

  // Match status badge renderer
  function renderMatchStatus(invoice: SupplierInvoiceDisplay): string {
    const icons = {
      'Matched': '',
      'Pending': '',
      'Discrepancy': ''
    };

    const colors = {
      'Matched': '#10B981',
      'Pending': '#F59E0B',
      'Discrepancy': '#DC2626'
    };

    const status = invoice.match_status || 'Pending';
    const color = colors[status] || colors['Pending'];
    const icon = icons[status] || icons['Pending'];

    let tooltip = '';
    if (status === 'Matched') {
      tooltip = 'PO Pass GRN Pass';
    } else if (status === 'Discrepancy') {
      tooltip = `PO ${invoice.po_match_ok ? 'Pass' : 'Fail'} GRN ${invoice.grn_match_ok ? 'Pass' : 'Fail'}`;
    }

    return `
      <span
        style="
          display: inline-flex;
          align-items: center;
          gap: 4px;
          padding: 3px 8px;
          border-radius: 12px;
          font-size: 11px;
          font-weight: 600;
          background: ${color}15;
          color: ${color};
        "
        title="${tooltip}"
      >
        ${icon} ${status}
      </span>
    `;
  }

  // Payment status badge renderer
  function renderPaymentStatus(invoice: SupplierInvoiceDisplay): string {
    const icons = {
      'Paid': '',
      'Partial': '',
      'Unpaid': '',
      'Scheduled': '',
      'Overdue': ''
    };

    const colors = {
      'Paid': '#10B981',
      'Partial': '#F59E0B',
      'Unpaid': '#6B7280',
      'Scheduled': '#3B82F6',
      'Overdue': '#DC2626'
    };

    const status = invoice.is_overdue ? 'Overdue' : (invoice.payment_status || 'Unpaid');
    const color = colors[status] || colors['Unpaid'];
    const icon = icons[status] || icons['Unpaid'];

    return `
      <span
        style="
          display: inline-flex;
          align-items: center;
          gap: 4px;
          padding: 3px 8px;
          border-radius: 12px;
          font-size: 11px;
          font-weight: 600;
          background: ${color}15;
          color: ${color};
        "
      >
        ${icon} ${status}
      </span>
    `;
  }

  // Filter tabs
  const filterTabs: { value: InvoiceFilter; label: string; count: number }[] = [
    { value: 'all', label: 'All Invoices', count: 0 },
    { value: 'pending', label: 'Pending Match', count: 0 },
    { value: 'matched', label: 'Matched', count: 0 },
    { value: 'discrepancy', label: 'Discrepancy', count: 0 },
    { value: 'paid', label: 'Paid', count: 0 },
    { value: 'overdue', label: 'Overdue', count: 0 }
  ];

  // Update tab counts
  $: {
    filterTabs[0].count = invoices.length;
    filterTabs[1].count = invoices.filter(i => i.match_status === 'Pending').length;
    filterTabs[2].count = invoices.filter(i => i.match_status === 'Matched').length;
    filterTabs[3].count = invoices.filter(i => i.match_status === 'Discrepancy').length;
    filterTabs[4].count = invoices.filter(i => i.payment_status === 'Paid').length;
    filterTabs[5].count = invoices.filter(i => i.is_overdue).length;
  }

  // Filter invoices based on tab, search, and date
  $: {
    let result = [...invoices];

    // Apply tab filter (existing logic)
    if (selectedFilter === 'pending') {
      result = result.filter(i => i.match_status === 'Pending');
    } else if (selectedFilter === 'matched') {
      result = result.filter(i => i.match_status === 'Matched');
    } else if (selectedFilter === 'discrepancy') {
      result = result.filter(i => i.match_status === 'Discrepancy');
    } else if (selectedFilter === 'paid') {
      result = result.filter(i => i.payment_status === 'Paid');
    } else if (selectedFilter === 'overdue') {
      result = result.filter(i => i.is_overdue);
    }
    // 'all' shows everything

    // Apply search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter(i =>
        i.invoice_number?.toLowerCase().includes(query) ||
        i.supplier_name?.toLowerCase().includes(query)
      );
    }

    // Apply date filter
    if (dateFilter !== 'all') {
      result = result.filter(i => isInDateRange(i.invoice_date, dateFilter));
    }

    filteredInvoices = result;
  }

  // Date filter helper function
  function isInDateRange(dateValue: any, filter: string): boolean {
    if (!dateValue) return true;
    const date = new Date(dateValue.toString());
    const now = new Date();

    switch (filter) {
      case 'this_month':
        return date.getMonth() === now.getMonth() && date.getFullYear() === now.getFullYear();
      case 'last_month': {
        const lastMonth = new Date(now.getFullYear(), now.getMonth() - 1);
        return date.getMonth() === lastMonth.getMonth() && date.getFullYear() === lastMonth.getFullYear();
      }
      case 'this_quarter': {
        const currentQuarter = Math.floor(now.getMonth() / 3);
        const dateQuarter = Math.floor(date.getMonth() / 3);
        return dateQuarter === currentQuarter && date.getFullYear() === now.getFullYear();
      }
      case 'this_year':
        return date.getFullYear() === now.getFullYear();
      default:
        return true;
    }
  }

  // Load invoices
  async function loadInvoices() {
    loading = true;
    try {
      // Load suppliers first to enrich invoice data with names
      const suppliers = await ListSuppliers(1000, 0);
      suppliersList = suppliers || [];
      suppliersMap.clear();
      suppliers.forEach((s: any) => {
        suppliersMap.set(s.id, s.supplier_name || s.supplier_code || 'Unknown Supplier');
      });

      // Load orders for internal reference dropdown
      try {
        const orders = await ListOrders(500, 0);
        ordersList = orders || [];
      } catch (e) {
        console.warn('Failed to load orders:', e);
        ordersList = [];
      }

      // Load supported currencies
      try {
        supportedCurrencies = await GetSupportedCurrencies() || [];
      } catch (e) {
        console.error('Failed to load currencies:', e);
        // Fallback
        supportedCurrencies = [
          { code: 'BHD', name: 'Bahraini Dinar', symbol: 'BD' },
          { code: 'USD', name: 'US Dollar', symbol: '$' },
          { code: 'EUR', name: 'Euro', symbol: '€' },
        ];
      }

      // Load invoices
      const data = await GetSupplierInvoices();

      // Enrich with computed fields + supplier names
      invoices = (data || []).map(enrichInvoice);

      console.log(`Loaded ${invoices.length} supplier invoices`);
    } catch (err) {
      console.error('Failed to load supplier invoices:', err);
      toast.danger('Failed to load supplier invoices: ' + getErrorMessage(err));
      invoices = [];
    } finally {
      loading = false;
    }
  }

  // Enrich invoice with computed fields
  function enrichInvoice(invoice: main.SupplierInvoice): SupplierInvoiceDisplay {
    const now = new Date();
    const dueDate = invoice.due_date ? new Date(invoice.due_date.toString()) : now;
    const daysUntilDue = Math.ceil((dueDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
    const items = Array.isArray(invoice.items) ? invoice.items : [];
    const itemsSubtotal = items.reduce((sum: number, item: any) => {
      const lineTotal = Number(item?.total_price || 0) || ((Number(item?.quantity || 0) || 0) * (Number(item?.unit_price || 0) || 0));
      return sum + lineTotal;
    }, 0);
    const exchangeRate = Number(invoice.exchange_rate || 1) || 1;
    const subtotalForeign = Number(invoice.subtotal_foreign || 0) || (itemsSubtotal > 0 ? roundAmount(itemsSubtotal) : 0);
    const vatForeign = Number(invoice.vat_foreign || 0) || 0;
    const totalForeign = Number(invoice.total_foreign || 0) || roundAmount(subtotalForeign + vatForeign);
    const subtotalBHD = Number(invoice.subtotal_bhd || 0) || roundAmount(subtotalForeign * exchangeRate);
    const vatBHD = Number(invoice.vat_bhd || 0) || roundAmount(vatForeign * exchangeRate);
    const totalBHD = Number(invoice.total_bhd || 0) || roundAmount(totalForeign * exchangeRate);

    return {
      ...invoice,
      items,
      exchange_rate: exchangeRate,
      subtotal_foreign: subtotalForeign,
      vat_foreign: vatForeign,
      total_foreign: totalForeign,
      subtotal_bhd: subtotalBHD,
      vat_bhd: vatBHD,
      total_bhd: totalBHD,
      outstanding_bhd: Number(invoice.outstanding_bhd || 0),
      days_until_due: daysUntilDue,
      is_overdue: daysUntilDue < 0 && Number(invoice.outstanding_bhd || 0) > 0,
      // supplier_name already comes from database - no mapping needed!
      supplier_name: invoice.supplier_name || suppliersMap.get(invoice.supplier_id) || undefined
    } as SupplierInvoiceDisplay;
  }

  async function hydrateInvoice(invoiceId: string): Promise<SupplierInvoiceDisplay> {
    const invoice = await GetSupplierInvoiceByID(invoiceId);
    return enrichInvoice(invoice);
  }

  // Open create modal
  function openCreateModal() {
    formData = {
      supplier_id: '',
      purchase_order_id: '',
      grn_id: '',
      order_id: '',
      invoice_number: '',
      invoice_date: new Date().toISOString().split('T')[0],
      due_date: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
      currency: 'BHD',
      exchange_rate: 1.0,
      subtotal_foreign: 0,
      vat_foreign: 0,
      total_foreign: 0
    };
    formItems = [{ description: '', quantity: 1, unit_price: 0, total_price: 0 }];
    showCreateModal = true;
  }

  // Add line item to form
  function addLineItem() {
    formItems = [...formItems, { description: '', quantity: 1, unit_price: 0, total_price: 0 }];
  }

  // Remove line item from form
  function removeLineItem(index: number) {
    if (formItems.length <= 1) return;
    formItems = formItems.filter((_, i) => i !== index);
  }

  // Recalculate line item total and form subtotal
  function recalcLineItems() {
    formItems = formItems.map(item => ({
      ...item,
      total_price: item.quantity * item.unit_price
    }));
    formData.subtotal_foreign = formItems.reduce((sum, item) => sum + item.total_price, 0);
    formData.vat_foreign = formData.subtotal_foreign * 0.10; // 10% VAT default
  }

  // Handle supplier selection - autofill details
  function handleSupplierSelect() {
    const supplier = suppliersList.find((s: any) => s.id === formData.supplier_id);
    if (supplier) {
      // Set currency based on supplier country if available
      if (supplier.country === 'Germany' || supplier.country === 'DE') {
        formData.currency = 'EUR';
        formData.exchange_rate = 0.44;
      } else if (supplier.country === 'United Kingdom' || supplier.country === 'UK' || supplier.country === 'GB') {
        formData.currency = 'GBP';
        formData.exchange_rate = 0.52;
      } else if (supplier.country === 'United States' || supplier.country === 'US') {
        formData.currency = 'USD';
        formData.exchange_rate = 0.376;
      }
    }
  }

  // Handle create invoice
  async function handleCreateInvoice() {
    if (formLoading) return; // Prevent double-submit

    if (!formData.supplier_id || !formData.invoice_number) {
      toast.warning('Please fill supplier and invoice number');
      return;
    }

    // Recalculate before submitting
    recalcLineItems();

    formLoading = true;
    try {
      // Filter out empty line items
      const validItems = formItems
        .filter(item => item.description.trim() !== '' && item.quantity > 0)
        .map((item, i) => ({
          line_number: i + 1,
          description: item.description,
          quantity: item.quantity,
          unit_price: item.unit_price,
          total_price: item.total_price,
          currency: formData.currency
        }));

      const invoice: any = {
        supplier_id: formData.supplier_id,
        purchase_order_id: formData.purchase_order_id,
        grn_id: formData.grn_id,
        order_id: formData.order_id,
        invoice_number: formData.invoice_number,
        invoice_date: formatDateForApi(formData.invoice_date),
        due_date: formatDateForApi(formData.due_date),
        currency: formData.currency,
        exchange_rate: formData.exchange_rate,
        subtotal_foreign: formData.subtotal_foreign,
        vat_foreign: formData.vat_foreign,
        total_foreign: formData.total_foreign,
        status: 'Pending',
        payment_status: 'Unpaid',
        match_status: 'Pending',
        items: validItems
      };

      await CreateSupplierInvoice(invoice);
      toast.success('Supplier invoice created successfully');
      showCreateModal = false;
      await loadInvoices();
    } catch (err) {
      console.error('Failed to create supplier invoice:', err);
      toast.danger('Failed to create supplier invoice: ' + getErrorMessage(err));
    } finally {
      formLoading = false;
    }
  }

  // Handle 3-way match
  async function handleThreeWayMatch(invoiceId: string) {
    try {
      showMatchModal = true;
      const invoice = invoices.find(i => i.id === invoiceId);
      if (invoice) {
        selectedInvoice = invoice;
      }

      // PerformThreeWayMatch now returns ThreeWayMatchResult struct
      const result = await PerformThreeWayMatch(invoiceId);

      if (result.matched) {
        toast.success('3-way match passed! Invoice verified.');
      } else {
        toast.warning(`3-way match failed: ${result.reason || 'Unknown discrepancy'}`);
      }

      await loadInvoices();
      showMatchModal = false;
      selectedInvoice = null;
    } catch (err) {
      console.error('Failed to perform 3-way match:', err);
      toast.danger('Failed to perform 3-way match: ' + getErrorMessage(err));
      showMatchModal = false;
      selectedInvoice = null;
    }
  }

  // Handle approve invoice
  let approvingInvoice = false;
  async function handleApproveInvoice(invoiceId: string) {
    if (approvingInvoice) return;
    if (!confirm('Approve this invoice for payment?')) return;

    approvingInvoice = true;
    try {
      await ApproveSupplierInvoice(invoiceId, systemUser);
      toast.success('Invoice approved successfully');
      await loadInvoices();
    } catch (err) {
      console.error('Failed to approve invoice:', err);
      toast.danger('Failed to approve invoice: ' + getErrorMessage(err));
    } finally {
      approvingInvoice = false;
    }
  }

  // Handle mark as paid - open modal
  function handleMarkPaid(invoiceId: string) {
    invoiceToMarkPaid = invoiceId;
    paymentData = {
      payment_reference: '',
      payment_method: 'Bank Transfer'
    };
    showPaymentModal = true;
  }

  // Submit payment
  async function submitPayment() {
    if (!paymentData.payment_reference || !invoiceToMarkPaid) {
      toast.warning('Please enter a payment reference');
      return;
    }

    paymentLoading = true;
    try {
      await MarkSupplierInvoicePaid(
        invoiceToMarkPaid,
        paymentData.payment_reference,
        paymentData.payment_method
      );
      toast.success('Invoice marked as paid successfully');
      showPaymentModal = false;
      invoiceToMarkPaid = null;
      await loadInvoices();
    } catch (err) {
      console.error('Failed to mark invoice as paid:', err);
      toast.danger('Failed to mark invoice as paid: ' + getErrorMessage(err));
    } finally {
      paymentLoading = false;
    }
  }

  // Handle view invoice
  async function handleViewInvoice(invoiceId: string) {
    try {
      selectedInvoice = await hydrateInvoice(invoiceId);
      showDetailModal = true;
    } catch (err) {
      console.error('Failed to load supplier invoice details:', err);
      toast.danger('Failed to load supplier invoice details: ' + getErrorMessage(err));
    }
  }

  // Handle action button clicks
  function handleRowClick(event: CustomEvent) {
    const target = event.detail.event?.target as HTMLElement;
    if (!target || !target.dataset.action) return;

    const action = target.dataset.action;
    const id = target.dataset.id;

    if (!id) return;

    switch (action) {
      case 'view':
        void handleViewInvoice(id);
        break;
      case 'edit':
        handleEditInvoice(id);
        break;
      case 'match':
        handleThreeWayMatch(id);
        break;
      case 'approve':
        handleApproveInvoice(id);
        break;
      case 'pay':
        handleMarkPaid(id);
        break;
    }
  }

  // Edit invoice handling
  let editLoading = false;

  function formatDateForInput(value: any): string {
    if (!value) return '';
    try {
      const raw = value.toString();
      if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw;
      return new Date(raw).toISOString().split('T')[0];
    } catch {
      return '';
    }
  }

  async function handleEditInvoice(id: string) {
    try {
      const invoice = await hydrateInvoice(id);
      let existingPayments: any[] = [];
      try {
        existingPayments = await GetSupplierPaymentsByInvoice(id);
      } catch {
        existingPayments = [];
      }

      existingPaymentCount = existingPayments.length;
      existingPaymentTotalBhd = existingPayments.reduce((sum, payment) => sum + Number(payment?.amount_bhd || 0), 0);
      createPaymentEntry =
        invoice.payment_status === 'Paid' &&
        existingPaymentCount === 0 &&
        Number(invoice.outstanding_bhd || 0) > 0;
      editingInvoice = {
        ...invoice,
        supplier_id: invoice.supplier_id || '',
        invoice_date: formatDateForInput(invoice.invoice_date),
        due_date: formatDateForInput(invoice.due_date),
        payment_date: formatDateForInput(invoice.payment_date)
      };
      showEditModal = true;
    } catch (err) {
      console.error('Failed to load supplier invoice for editing:', err);
      toast.danger('Failed to load supplier invoice: ' + getErrorMessage(err));
    }
  }

  async function saveEditInvoice() {
    if (!editingInvoice) return;
    editLoading = true;
    try {
      const normalizedInvoice = {
        ...editingInvoice,
        invoice_date: formatDateForApi(editingInvoice.invoice_date),
        due_date: formatDateForApi(editingInvoice.due_date),
        payment_date: formatDateForApi(editingInvoice.payment_date)
      };
      if (normalizedInvoice.payment_status === 'Paid' && !normalizedInvoice.payment_date) {
        normalizedInvoice.payment_date = formatDateForApi(new Date().toISOString().split('T')[0]);
      }
      if (normalizedInvoice.supplier_id) {
        const supplier = suppliersList.find((s: any) => s.id === normalizedInvoice.supplier_id);
        if (supplier) {
          normalizedInvoice.supplier_name = supplier.supplier_name;
        }
      }
      if (normalizedInvoice.status === 'Approved' && normalizedInvoice.match_status !== 'Matched') {
        toast.warning('3-way match must be Matched before approval');
        return;
      }
      if (normalizedInvoice.payment_status === 'Paid' && existingPaymentCount === 0 && !createPaymentEntry) {
        toast.warning('Enable payment ledger creation or use Mark Paid to settle this invoice');
        return;
      }
      if (
        normalizedInvoice.payment_status === 'Paid' &&
        existingPaymentCount > 0 &&
        existingPaymentTotalBhd + 0.001 < Number(normalizedInvoice.total_bhd || 0)
      ) {
        toast.warning('Use Supplier Payments to settle the remaining balance on partially paid invoices');
        return;
      }
      const shouldCreatePaymentEntry =
        normalizedInvoice.payment_status === 'Paid' &&
        createPaymentEntry &&
        existingPaymentTotalBhd + 0.001 < Number(normalizedInvoice.total_bhd || 0);

      if (shouldCreatePaymentEntry) {
        await UpdateSupplierInvoiceWithPayment(normalizedInvoice as any, true);
      } else {
        await UpdateSupplierInvoice(normalizedInvoice as any);
      }

      toast.success(shouldCreatePaymentEntry ? 'Supplier invoice updated and payment entry created' : 'Supplier invoice updated');
      showEditModal = false;
      editingInvoice = null;
      createPaymentEntry = false;
      existingPaymentCount = 0;
      existingPaymentTotalBhd = 0;
      await loadInvoices();
    } catch (e) {
      toast.danger('Failed to update: ' + getErrorMessage(e));
    } finally {
      editLoading = false;
    }
  }

  // Auto-calculate totals
  $: formData.total_foreign = formData.subtotal_foreign + formData.vat_foreign;

  // Event handler for opening create modal from parent hub
  function handleOpenCreateSupplierInvoice() {
    openCreateModal();
  }

  onMount(() => {
    if (!canView) return;
    loadInvoices();

    // Listen for create supplier invoice events from OperationsHub header button
    window.addEventListener('openCreateSupplierInvoice', handleOpenCreateSupplierInvoice);
  });

  onDestroy(() => {
    // Clean up event listener
    window.removeEventListener('openCreateSupplierInvoice', handleOpenCreateSupplierInvoice);
  });
</script>

{#if embedded}
  <!-- Embedded mode for hub container -->
  <div class="supplier-invoices-embedded">
    <div class="header-embedded">
      <h2>Supplier Invoices</h2>
      <div style="display: flex; gap: 8px; flex-wrap: wrap;">
        <Button variant="secondary" size="sm" on:click={openBankReconciliation}>
          Open Bank Recon
        </Button>
        <Button variant="primary" size="sm" on:click={openCreateModal}>
          + New Invoice
        </Button>
      </div>
    </div>

    <Card padding="sm">
      <DataTable
        {columns}
        data={filteredInvoices}
        {loading}
        emptyMessage="No supplier invoices found"
        onRowClick={() => {}}
        on:rowClick={handleRowClick}
        stickyHeader={true}
        maxHeight="400px"
        showBorder={false}
      />
    </Card>
  </div>
{:else}
  <!-- Full page mode -->
  <PageLayout title="Supplier Invoices" subtitle="3-Way Matching & Payment Tracking">
    <svelte:fragment slot="header-actions">
      <Button variant="secondary" on:click={openBankReconciliation}>
        Open Bank Recon
      </Button>
      <Button variant="primary" on:click={openCreateModal}>
        + New Supplier Invoice
      </Button>
    </svelte:fragment>

    <div class="supplier-invoices-container">
      <!-- Search and Date Filter Controls -->
      <div class="search-controls" style="display: flex; gap: 16px; margin-bottom: 16px; align-items: flex-end;">
        <FormGroup label="Search" style="flex: 1; margin-bottom: 0;">
          <Input
            type="text"
            bind:value={searchQuery}
            placeholder="Search by invoice # or supplier..."
          />
        </FormGroup>

        <FormGroup label="Date Range" style="width: 180px; margin-bottom: 0;">
          <select class="select-input" bind:value={dateFilter}>
            <option value="all">All Time</option>
            <option value="this_month">This Month</option>
            <option value="last_month">Last Month</option>
            <option value="this_quarter">This Quarter</option>
            <option value="this_year">This Year</option>
          </select>
        </FormGroup>
      </div>

      <!-- Filter Tabs -->
      <Card padding="sm">
        <div class="filter-tabs" role="tablist" aria-label="Filter supplier invoices">
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

      <!-- Invoices DataTable -->
      <Card padding="sm">
        <DataTable
          {columns}
          data={filteredInvoices}
          {loading}
          emptyMessage="No supplier invoices found"
          onRowClick={() => {}}
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
            <div class="stat-label">Total Invoices</div>
            <div class="stat-value">{invoices.length}</div>
          </div>
        </Card>

        <Card padding="md">
          <div class="stat">
            <div class="stat-label">Total Amount (BHD)</div>
            <div class="stat-value">
              {formatMoney(invoices.reduce((sum, i) => sum + i.total_bhd, 0))}
            </div>
          </div>
        </Card>

        <Card padding="md">
          <div class="stat">
            <div class="stat-label">Open</div>
            <div class="stat-value stat-warning">
              {invoices.filter(i => Number(i.outstanding_bhd || 0) > 0).length}
            </div>
          </div>
        </Card>

        <Card padding="md">
          <div class="stat">
            <div class="stat-label">Overdue</div>
            <div class="stat-value stat-danger">
              {invoices.filter(i => i.is_overdue).length}
            </div>
          </div>
        </Card>

        <Card padding="md">
          <div class="stat">
            <div class="stat-label">Match Rate</div>
            <div class="stat-value stat-success">
              {invoices.length > 0
                ? ((invoices.filter(i => i.match_status === 'Matched').length / invoices.length) * 100).toFixed(1)
                : 0}%
            </div>
          </div>
        </Card>
      </div>
    </div>
  </PageLayout>
{/if}

<!-- Create Invoice Modal -->
<WabiModal bind:open={showCreateModal} title="Create Supplier Invoice" size="lg">
  <form on:submit|preventDefault={handleCreateInvoice} class="invoice-form">
    <!-- Supplier & Invoice Number -->
    <div class="form-row">
      <FormGroup label="Supplier" required>
        <select class="select-input" bind:value={formData.supplier_id} on:change={handleSupplierSelect} required>
          <option value="">-- Select Supplier --</option>
          {#each suppliersList as supplier}
            <option value={supplier.id}>{supplier.supplier_name || supplier.supplier_code || 'Unknown'}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Invoice Number" required>
        <Input
          type="text"
          bind:value={formData.invoice_number}
          placeholder="INV-2024-001"
          required
        />
      </FormGroup>
    </div>

    <!-- Internal Order Reference -->
    <div class="form-row">
      <FormGroup label="Related Customer Order">
        <select class="select-input" bind:value={formData.order_id}>
          <option value="">-- None (No internal order) --</option>
          {#each ordersList as order}
            <option value={order.id}>{order.order_number || order.id?.slice(0,8)} - {order.customer_name || 'Customer'}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Purchase Order Ref">
        <Input
          type="text"
          bind:value={formData.purchase_order_id}
          placeholder="PO reference (optional)"
        />
      </FormGroup>
    </div>

    <!-- Dates -->
    <div class="form-row">
      <FormGroup label="Invoice Date" required>
        <Input
          type="date"
          bind:value={formData.invoice_date}
          required
        />
      </FormGroup>

      <FormGroup label="Due Date" required>
        <Input
          type="date"
          bind:value={formData.due_date}
          required
        />
      </FormGroup>
    </div>

    <!-- Currency -->
    <div class="form-row">
      <FormGroup label="Currency" required>
        <select class="select-input" bind:value={formData.currency}>
          {#each supportedCurrencies as curr}
            <option value={curr.code}>{curr.code} - {curr.name}</option>
          {/each}
        </select>
      </FormGroup>

      <FormGroup label="Exchange Rate to BHD">
        <Input
          type="number"
          bind:value={formData.exchange_rate}
          min="0"
          step="0.0001"
        />
      </FormGroup>
    </div>

    <!-- Line Items Section -->
    <div class="line-items-section">
      <div class="line-items-header">
        <h4>Line Items</h4>
        <button type="button" class="add-item-btn" on:click={addLineItem}>+ Add Item</button>
      </div>

      <div class="line-items-table">
        <div class="li-header-row">
          <span class="li-col-desc">Description</span>
          <span class="li-col-qty">Qty</span>
          <span class="li-col-price">Unit Price</span>
          <span class="li-col-total">Total</span>
          <span class="li-col-action"></span>
        </div>
        {#each formItems as item, i}
          <div class="li-row">
            <input
              class="li-input li-col-desc"
              type="text"
              bind:value={item.description}
              placeholder="Item description"
              on:blur={recalcLineItems}
            />
            <input
              class="li-input li-col-qty"
              type="number"
              bind:value={item.quantity}
              min="1"
              step="1"
              on:input={recalcLineItems}
            />
            <input
              class="li-input li-col-price"
              type="number"
              bind:value={item.unit_price}
              min="0"
              step="0.001"
              on:input={recalcLineItems}
            />
            <span class="li-col-total li-total-value">{formatMoney(item.quantity * item.unit_price)}</span>
            <button type="button" class="li-remove-btn" on:click={() => removeLineItem(i)} disabled={formItems.length <= 1}>x</button>
          </div>
        {/each}
      </div>
    </div>

    <!-- Totals -->
    <div class="form-row">
      <FormGroup label="Subtotal ({formData.currency})">
        <Input
          type="number"
          bind:value={formData.subtotal_foreign}
          min="0"
          step="0.001"
        />
      </FormGroup>

      <FormGroup label="VAT (10%)">
        <Input
          type="number"
          bind:value={formData.vat_foreign}
          min="0"
          step="0.001"
        />
      </FormGroup>
    </div>

    <!-- Total (calculated) -->
    <div class="total-display">
      <span class="total-label">Total:</span>
      <span class="total-value">{formatMoney(formData.total_foreign)} {formData.currency}</span>
    </div>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showCreateModal = false}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={formLoading}
      on:click={handleCreateInvoice}
    >
      Create Invoice
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Detail View Modal -->
<WabiModal bind:open={showDetailModal} title="Invoice Details" size="lg">
  {#if selectedInvoice}
    <div class="invoice-detail">
      <div class="detail-section">
        <h4>Invoice Information</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Invoice Number</span>
            <span class="detail-value">{selectedInvoice.invoice_number}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Status</span>
            <span class="detail-value">{selectedInvoice.status}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Invoice Date</span>
            <span class="detail-value">{selectedInvoice.invoice_date ? new Date(selectedInvoice.invoice_date.toString()).toLocaleDateString() : 'N/A'}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Due Date</span>
            <span class="detail-value">{selectedInvoice.due_date ? new Date(selectedInvoice.due_date.toString()).toLocaleDateString() : 'N/A'}</span>
          </div>
        </div>
      </div>

      <div class="detail-section">
        <h4>3-Way Matching</h4>
        <div class="match-indicators">
          <div class="match-item" class:matched={selectedInvoice.po_match_ok}>
            <span class="match-icon">{selectedInvoice.po_match_ok ? 'Yes' : 'No'}</span>
            <span>PO Match</span>
          </div>
          <div class="match-item" class:matched={selectedInvoice.grn_match_ok}>
            <span class="match-icon">{selectedInvoice.grn_match_ok ? 'Yes' : 'No'}</span>
            <span>GRN Match</span>
          </div>
          <div class="match-item" class:matched={selectedInvoice.match_status === 'Matched'}>
            <span class="match-icon">{selectedInvoice.match_status === 'Matched' ? 'Yes' : 'Pending'}</span>
            <span>{selectedInvoice.match_status}</span>
          </div>
        </div>
        {#if selectedInvoice.discrepancy_reason}
          <div class="discrepancy-note">
            <strong>Discrepancy:</strong> {selectedInvoice.discrepancy_reason}
          </div>
        {/if}
      </div>

      <div class="detail-section">
        <h4>Amounts</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Currency</span>
            <span class="detail-value">{selectedInvoice.currency}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Subtotal</span>
            <span class="detail-value">{formatMoney(selectedInvoice.subtotal_foreign)} {selectedInvoice.currency}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">VAT</span>
            <span class="detail-value">{formatMoney(selectedInvoice.vat_foreign)} {selectedInvoice.currency}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Total</span>
            <span class="detail-value amount-highlight">{formatMoney(selectedInvoice.total_foreign)} {selectedInvoice.currency}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Total (BHD)</span>
            <span class="detail-value amount-highlight">{formatMoney(selectedInvoice.total_bhd)} BHD</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Outstanding</span>
            <span class="detail-value amount-highlight">{formatMoney(selectedInvoice.outstanding_bhd || 0)} BHD</span>
          </div>
        </div>
      </div>

      {#if selectedInvoice.payment_status === 'Paid' || selectedInvoice.payment_status === 'Partial'}
        <div class="detail-section">
          <h4>Payment Information</h4>
          <div class="detail-grid">
            <div class="detail-item">
              <span class="detail-label">Payment Date</span>
              <span class="detail-value">{selectedInvoice.payment_date ? new Date(selectedInvoice.payment_date.toString()).toLocaleDateString() : 'N/A'}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">Payment Reference</span>
              <span class="detail-value">{selectedInvoice.payment_ref || 'N/A'}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">Payment Method</span>
              <span class="detail-value">{selectedInvoice.payment_method || 'N/A'}</span>
            </div>
          </div>
        </div>
      {/if}

      <!-- Line Items Section -->
      {#if selectedInvoice.items && selectedInvoice.items.length > 0}
        <div class="detail-section">
          <h4>Line Items ({selectedInvoice.items.length})</h4>
          <div class="items-table">
            <div class="items-header">
              <span class="item-col-desc">Description</span>
              <span class="item-col-qty">Qty</span>
              <span class="item-col-price">Unit Price</span>
              <span class="item-col-total">Total</span>
            </div>
            {#each selectedInvoice.items as item, i}
              <div class="items-row">
                <span class="item-col-desc">{item.description || `Item ${i + 1}`}</span>
                <span class="item-col-qty">{item.quantity || 0}</span>
                <span class="item-col-price">{(item.unit_price || 0).toFixed(2)}</span>
                <span class="item-col-total">{(item.total_price || 0).toFixed(2)}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showDetailModal = false}>
      Close
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- 3-Way Match Progress Modal -->
<WabiModal bind:open={showMatchModal} title="Performing 3-Way Match" size="md">
  <div class="match-progress">
    <div class="spinner"></div>
    <p>Verifying PO, GRN, and Invoice alignment...</p>
  </div>
</WabiModal>

<!-- Payment Modal -->
<WabiModal bind:open={showPaymentModal} title="Record Payment" size="md">
  <form on:submit|preventDefault={submitPayment} class="payment-form">
    <FormGroup label="Payment Reference" required>
      <Input
        type="text"
        bind:value={paymentData.payment_reference}
        placeholder="e.g., TXN-2024-001, Cheque #12345"
        required
        autofocus
      />
    </FormGroup>

    <FormGroup label="Payment Method" required>
      <select class="select-input" bind:value={paymentData.payment_method}>
        <option value="Bank Transfer">Bank Transfer</option>
        <option value="Cheque">Cheque</option>
        <option value="Cash">Cash</option>
        <option value="Wire Transfer">Wire Transfer</option>
        <option value="Credit Card">Credit Card</option>
      </select>
    </FormGroup>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showPaymentModal = false}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={paymentLoading}
      on:click={submitPayment}
    >
      Confirm Payment
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Edit Invoice Modal -->
{#if showEditModal && editingInvoice}
  <WabiModal bind:open={showEditModal} title="Edit Supplier Invoice" size="md">
    <div class="edit-form-grid">
      <FormGroup label="Supplier">
        <select class="select-input" bind:value={editingInvoice.supplier_id}>
          <option value="">Select supplier...</option>
          {#each suppliersList as supplier}
            <option value={supplier.id}>{supplier.supplier_name}</option>
          {/each}
        </select>
      </FormGroup>
      <FormGroup label="Invoice Number">
        <Input type="text" bind:value={editingInvoice.invoice_number} />
      </FormGroup>
      <FormGroup label="Status">
        <select class="select-input" bind:value={editingInvoice.status}>
          <option value="Pending">Pending</option>
          <option value="Approved">Approved</option>
          <option value="Rejected">Rejected</option>
          <option value="Dispute">Dispute</option>
        </select>
      </FormGroup>
      <FormGroup label="Amount (Foreign)">
        <Input type="number" step="0.01" bind:value={editingInvoice.total_foreign} />
      </FormGroup>
      <FormGroup label="Currency">
        <Input type="text" bind:value={editingInvoice.currency} />
      </FormGroup>
      <FormGroup label="Invoice Date">
        <Input type="date" bind:value={editingInvoice.invoice_date} />
      </FormGroup>
      <FormGroup label="Due Date">
        <Input type="date" bind:value={editingInvoice.due_date} />
      </FormGroup>
      <FormGroup label="Payment Status">
        <select class="select-input" bind:value={editingInvoice.payment_status} disabled={existingPaymentCount > 0}>
          <option value="Unpaid">Unpaid</option>
          <option value="Partial">Partial</option>
          <option value="Paid">Paid</option>
        </select>
      </FormGroup>
      <FormGroup label="Payment Method">
        <select class="select-input" bind:value={editingInvoice.payment_method} disabled={existingPaymentCount > 0}>
          <option value="">Not Set</option>
          <option value="Bank Transfer">Bank Transfer</option>
          <option value="Cheque">Cheque</option>
          <option value="Cash">Cash</option>
          <option value="Wire Transfer">Wire Transfer</option>
          <option value="LC">LC</option>
          <option value="PDC">PDC</option>
          <option value="Other">Other</option>
        </select>
      </FormGroup>
      <FormGroup label="Payment Date">
        <Input type="date" bind:value={editingInvoice.payment_date} disabled={existingPaymentCount > 0} />
      </FormGroup>
      <FormGroup label="Payment Ref">
        <Input type="text" bind:value={editingInvoice.payment_ref} placeholder="Transfer / cheque reference" disabled={existingPaymentCount > 0} />
      </FormGroup>
    </div>
    {#if existingPaymentCount > 0}
      <div class="payment-entry-card">
        <p class="payment-entry-note">
          This invoice already has {existingPaymentCount} supplier payment entr{existingPaymentCount === 1 ? 'y' : 'ies'}
          totaling {formatMoney(existingPaymentTotalBhd)} BHD. Payment status and payment metadata are now ledger-derived.
        </p>
      </div>
    {/if}
    {#if editingInvoice.payment_status === 'Paid' && existingPaymentCount === 0}
      <div class="payment-entry-card">
        <label class="payment-entry-toggle">
          <input type="checkbox" bind:checked={createPaymentEntry} />
          <span>Create supplier payment ledger entry from this payment info</span>
        </label>
        <p class="payment-entry-note">
          This will make the payment appear in Supplier Payments and Bank Reconciliation matching.
        </p>
      </div>
    {/if}
    <svelte:fragment slot="footer">
      <Button variant="ghost" on:click={() => { showEditModal = false; editingInvoice = null; createPaymentEntry = false; existingPaymentCount = 0; existingPaymentTotalBhd = 0; }}>Cancel</Button>
      <Button variant="primary" loading={editLoading} on:click={saveEditInvoice}>Save Changes</Button>
    </svelte:fragment>
  </WabiModal>
{/if}

<style>
  .supplier-invoices-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .supplier-invoices-embedded {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .header-embedded {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .payment-entry-card {
    margin-top: 16px;
    padding: 14px 16px;
    border-radius: 16px;
    background: rgba(15, 23, 42, 0.04);
    border: 1px solid rgba(15, 23, 42, 0.08);
  }

  .payment-entry-toggle {
    display: flex;
    align-items: center;
    gap: 10px;
    font-family: var(--font-body);
    font-size: 14px;
    color: var(--text-primary);
  }

  .payment-entry-note {
    margin: 8px 0 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-secondary);
  }

  .header-embedded h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
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

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
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

  .stat-danger {
    color: #DC2626;
  }

  .stat-success {
    color: #10B981;
  }

  /* Action Buttons */
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

  :global(.action-btn-match) {
    background: rgba(59, 130, 246, 0.1);
    color: #3B82F6;
  }

  :global(.action-btn-match:hover) {
    background: #3B82F6;
    color: white;
  }

  :global(.action-btn-approve) {
    background: rgba(16, 185, 129, 0.1);
    color: #10B981;
  }

  :global(.action-btn-approve:hover) {
    background: #10B981;
    color: white;
  }

  :global(.action-btn-pay) {
    background: rgba(139, 92, 246, 0.1);
    color: #8B5CF6;
  }

  :global(.action-btn-pay:hover) {
    background: #8B5CF6;
    color: white;
  }

  :global(.action-btn-edit) {
    background: rgba(0, 0, 0, 0.05);
    color: var(--ink, #1d1d1f);
  }
  :global(.action-btn-edit:hover) {
    background: rgba(0, 0, 0, 0.12);
  }

  .edit-form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  /* Form Styles */
  .invoice-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
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

  .total-display {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 16px;
    padding: 12px 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius-sm);
    border: 2px solid var(--brand-indigo);
  }

  .total-label {
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .total-value {
    font-size: 20px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  /* Invoice Detail */
  .invoice-detail {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .detail-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .detail-section h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border);
  }

  .detail-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  :global(.supplier-invoices-container .data-table th),
  :global(.supplier-invoices-container .data-table td) {
    vertical-align: top;
  }

  :global(.supplier-invoices-container .data-table .text-right) {
    white-space: nowrap;
  }

  .detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .detail-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }

  .detail-value {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .amount-highlight {
    font-size: 16px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  /* Match Indicators */
  .match-indicators {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius);
  }

  .match-item {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1 1 180px;
    min-width: 0;
    padding: 8px 16px;
    background: var(--surface);
    border: 2px solid var(--border);
    border-radius: var(--border-radius-sm);
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .match-item.matched {
    border-color: #10B981;
    background: rgba(16, 185, 129, 0.05);
    color: #10B981;
  }

  .match-icon {
    font-size: 18px;
  }

  .discrepancy-note {
    padding: 12px;
    background: rgba(220, 38, 38, 0.05);
    border-left: 4px solid #DC2626;
    border-radius: var(--border-radius-sm);
    font-size: 13px;
    color: #DC2626;
  }

  /* Match Progress */
  .match-progress {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 32px;
  }

  .spinner {
    width: 40px;
    height: 40px;
    border: 4px solid rgba(0, 0, 0, 0.1);
    border-top-color: var(--brand-indigo);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .match-progress p {
    margin: 0;
    font-size: 14px;
    color: var(--text-secondary);
  }

  /* Payment Form */
  .payment-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* Line Items */
  .line-items-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
    background: var(--surface-elevated, #f9f9f9);
    border-radius: var(--border-radius-sm, 6px);
    border: 1px solid var(--border, #e5e5e5);
  }

  .line-items-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .line-items-header h4 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-secondary, #666);
  }

  .add-item-btn {
    padding: 4px 10px;
    font-size: 12px;
    font-weight: 500;
    background: var(--carbon, #000);
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .add-item-btn:hover {
    opacity: 0.85;
  }

  .line-items-table {
    display: grid;
    gap: 4px;
    min-width: 0;
  }

  .li-header-row {
    display: grid;
    grid-template-columns: minmax(0, 2.6fr) minmax(60px, 0.7fr) minmax(96px, 1fr) minmax(96px, 1fr) 28px;
    gap: 8px;
    padding: 4px 0;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-muted, #999);
  }

  .li-row {
    display: grid;
    grid-template-columns: minmax(0, 2.6fr) minmax(60px, 0.7fr) minmax(96px, 1fr) minmax(96px, 1fr) 28px;
    gap: 8px;
    align-items: center;
  }

  .li-col-desc { min-width: 0; }
  .li-col-qty { min-width: 0; }
  .li-col-price { min-width: 0; }
  .li-col-total { min-width: 0; text-align: right; }
  .li-col-action { width: 24px; }

  .li-input {
    width: 100%;
    min-width: 0;
    padding: 6px 8px;
    font-size: 13px;
    font-family: var(--font-family, Inter, system-ui);
    color: var(--text-primary, #1d1d1f);
    background: var(--surface, #fff);
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 4px;
  }

  .li-input:focus {
    outline: none;
    border-color: var(--carbon, #000);
  }

  .li-total-value {
    font-size: 13px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary, #1d1d1f);
    padding-right: 4px;
  }

  .li-remove-btn {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 4px;
    color: var(--text-muted, #999);
    cursor: pointer;
    font-size: 12px;
  }

  .li-remove-btn:hover:not(:disabled) {
    background: #fee;
    border-color: #f88;
    color: #c00;
  }

  .li-remove-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  /* Items Table in Detail Modal */
  .items-table {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 6px;
    overflow: hidden;
  }

  .items-header {
    display: grid;
    grid-template-columns: 2fr 0.5fr 1fr 1fr;
    gap: 8px;
    padding: 8px 12px;
    background: var(--surface-elevated, #f5f5f5);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .items-row {
    display: grid;
    grid-template-columns: 2fr 0.5fr 1fr 1fr;
    gap: 8px;
    padding: 10px 12px;
    border-top: 1px solid var(--border, #e5e5e5);
    font-size: 13px;
  }

  .items-row:hover {
    background: var(--surface-hover, #fafafa);
  }

  .item-col-qty, .item-col-price, .item-col-total {
    text-align: right;
  }

  .item-col-total {
    font-weight: 600;
  }

  /* Responsive */
  @media (max-width: 768px) {
    .form-row {
      grid-template-columns: 1fr;
    }

    .detail-grid {
      grid-template-columns: 1fr;
    }

    .stats-grid {
      grid-template-columns: 1fr;
    }

    .match-indicators {
      flex-direction: column;
    }

    .li-header-row,
    .li-row {
      grid-template-columns: minmax(0, 1.8fr) minmax(56px, 0.7fr) minmax(80px, 0.9fr) minmax(80px, 0.9fr) 24px;
    }
  }
</style>
