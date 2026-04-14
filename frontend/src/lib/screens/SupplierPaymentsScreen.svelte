<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import {
    GetAllSupplierPayments,
    GetSupplierPayment,
    RecordSupplierPayment,
    GetSupplierInvoices,
    UpdateSupplierPayment,
    DeleteSupplierPayment
  } from '../../../wailsjs/go/main/App';

  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { listExpenseEntries, type ExpenseEntry } from '$lib/api/expenses';
  import { toast } from '$lib/stores/toasts';
  import { permissions } from '$lib/stores/authContext';
  const dispatch = createEventDispatcher();

  export let embedded = false;
  export let company: 'PH Trading' | 'AHS Trading' = 'PH Trading';

  let payments: any[] = [];
  let invoices: any[] = [];
  let loading = true;
  let showPaymentModal = false;
  let summary: any = { total_paid_bhd: 0, outstanding_count: 0, overdue_count: 0 };
  let invoicesLoaded = false;
  let paymentModalMode: 'create' | 'edit' = 'create';
  let editingPaymentId = '';
  let paymentSaving = false;
  let selectedPaymentId = '';
  let selectedPayment: any = null;
  let deleteConfirmPaymentId = '';

  function matchesCompany(division?: string) {
    return (division || 'PH Trading') === company;
  }

  $: canCreate = $permissions.includes('*') || $permissions.includes('payments:record') || $permissions.includes('payments:*') || $permissions.includes('finance:*');
  $: canUpdate = $permissions.includes('*') || $permissions.includes('supplier_payments:update') || $permissions.includes('supplier_payments:*') || $permissions.includes('finance:*');
  $: canDelete = $permissions.includes('*') || $permissions.includes('supplier_payments:delete') || $permissions.includes('supplier_payments:*') || $permissions.includes('finance:*');

  // Payment form
  let paymentForm = {
    invoice_id: '',
    amount: 0,
    currency: 'BHD',
    method: 'Bank Transfer',
    date: new Date().toISOString().split('T')[0],
    reference: ''
  };

  // Helper to safely display string values
  function safeStr(value: any): string {
    if (value === null || value === undefined || value === '') return '—';
    return String(value);
  }

  // Helper to format date - handles Go Time objects
  function fmtDate(dateStr: any): string {
    if (!dateStr) return '—';
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return '—';
    return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
  }

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

  function normalizeSupplierPaymentMethod(method: string): string {
    if (method === 'Letter of Credit') return 'LC';
    return method;
  }

  function isExpenseSettlement(row: any): boolean {
    return row?.source === 'Expense' || String(row?.id || '').startsWith('expense-');
  }

  function isCurrentYear(dateStr: any): boolean {
    if (!dateStr) return false;
    const dateVal = typeof dateStr === 'string' ? dateStr : String(dateStr);
    const date = new Date(dateVal);
    if (isNaN(date.getTime())) return false;
    return date.getFullYear() === new Date().getFullYear();
  }

  function openBankReconciliation() {
    dispatch('navigate', { tab: 'bank_recon', source: 'supplier_payments' });
  }

  const columns = [
    { key: 'source', label: 'Source', width: '120px' },
    {
      key: 'payment_date',
      label: 'Date',
      sortable: true,
      width: '120px',
      render: (row: any) => `<span style="font-family: var(--font-mono); font-size: 13px;">${fmtDate(row.payment_date)}</span>`
    },
    {
      key: 'supplier_name',
      label: 'Supplier',
      sortable: true,
      width: '200px',
      render: (row: any) => {
        const name = row.supplier_name || row.counterparty_name || row.SupplierName || row.supplierName ||
                     row.SupplierInvoice?.supplier_name || row.supplier_invoice?.supplier_name || '';
        return `<span style="font-weight: 500;">${safeStr(name)}</span>`;
      }
    },
    {
      key: 'invoice_number',
      label: 'Invoice #',
      sortable: true,
      width: '140px',
      render: (row: any) => {
        const invNum = row.invoice_number || row.InvoiceNumber || row.invoiceNumber ||
                       row.SupplierInvoice?.invoice_number || row.supplier_invoice?.invoice_number || '';
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: var(--brand-indigo);">${safeStr(invNum)}</span>`;
      }
    },
    { key: 'amount_bhd', label: 'Amount (BHD)', type: 'currency' as const, align: 'right' as const, sortable: true, width: '140px' },
    { key: 'currency', label: 'Currency', width: '80px' },
    { key: 'payment_method', label: 'Method', width: '130px' },
    {
      key: 'reference',
      label: 'Reference',
      width: '150px',
      render: (row: any) => {
        const ref = row.reference || row.Reference || row.payment_reference || '';
        return `<span style="font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary);">${safeStr(ref)}</span>`;
      }
    }
  ];

  const paymentMethods = [
    'Bank Transfer', 'Cheque', 'LC', 'Cash', 'Wire Transfer', 'PDC', 'Other'
  ];

  async function loadData() {
    loading = true;
    try {
      const [paymentsRes, supplierInvoicesRes, expenseEntries] = await Promise.all([
        GetAllSupplierPayments(),
        GetSupplierInvoices(),
        listExpenseEntries('', true)
      ]);
      const paidExpenses = (expenseEntries || []).filter((entry: ExpenseEntry) => entry.payment_status === 'paid');
      const expensePaymentRows = paidExpenses.map((entry: ExpenseEntry) => ({
        id: `expense-${entry.id}`,
        source: 'Expense',
        payment_date: entry.paid_at || entry.expense_date,
        supplier_name: entry.vendor_name || entry.category_name || 'Expense',
        counterparty_name: entry.vendor_name || entry.category_name || 'Expense',
        invoice_number: entry.entry_number,
        amount_bhd: Number(entry.total_amount) || 0,
        currency: entry.currency || 'BHD',
        payment_method: entry.payment_method || 'NEFT',
        reference: entry.payment_reference || '',
      }));
      const supplierPaymentRows = (paymentsRes || []).filter((row: any) => matchesCompany(row.division)).map((row: any) => ({
        ...row,
        source: row.source || 'Supplier Invoice',
      }));
      const scopedExpensePaymentRows = company === 'PH Trading' ? expensePaymentRows : [];
      payments = [...supplierPaymentRows, ...scopedExpensePaymentRows].sort((left: any, right: any) => {
        const leftTime = new Date(left.payment_date || 0).getTime();
        const rightTime = new Date(right.payment_date || 0).getTime();
        return rightTime - leftTime;
      });
      if (selectedPaymentId) {
        selectedPayment = payments.find((payment: any) => payment.id === selectedPaymentId) || null;
        if (!selectedPayment) {
          selectedPaymentId = '';
          deleteConfirmPaymentId = '';
        }
      }

      const paidExpenseTotal = paidExpenses.reduce((sum: number, entry: ExpenseEntry) => sum + (Number(entry.total_amount) || 0), 0);
      const unpaidExpenseEntries = (expenseEntries || []).filter((entry: ExpenseEntry) => entry.payment_status !== 'paid' && ['approved', 'posted'].includes(entry.status));
      const overdueExpenseCount = unpaidExpenseEntries.filter((entry: ExpenseEntry) => {
        if (!entry.due_date) return false;
        return new Date(entry.due_date).getTime() < Date.now();
      }).length;

      const filteredInvoices = (supplierInvoicesRes || []).filter((invoice: any) => matchesCompany(invoice.division));
      const outstandingInvoices = filteredInvoices.filter((invoice: any) => String(invoice?.payment_status || '').toLowerCase() !== 'paid');
      const overdueInvoices = outstandingInvoices.filter((invoice: any) => {
        if (!invoice?.due_date) return false;
        return new Date(invoice.due_date).getTime() < Date.now();
      });

      summary = {
        total_paid_bhd: supplierPaymentRows.reduce((sum: number, row: any) => sum + (Number(row.amount_bhd) || 0), 0) + (company === 'PH Trading' ? paidExpenseTotal : 0),
        outstanding_count: outstandingInvoices.length + (company === 'PH Trading' ? unpaidExpenseEntries.length : 0),
        overdue_count: overdueInvoices.length + (company === 'PH Trading' ? overdueExpenseCount : 0),
      };
    } catch (e) {
      console.error('Failed to load supplier payments:', e);
    } finally {
      loading = false;
    }
  }

  async function openPaymentModal() {
    if (!canCreate) {
      toast.warning('You do not have permission to record supplier payments');
      return;
    }

    paymentModalMode = 'create';
    editingPaymentId = '';
    showPaymentModal = true;
    if (!invoicesLoaded) {
      try {
        invoices = (await GetSupplierInvoices() || []).filter((invoice: any) => matchesCompany(invoice.division));
        invoicesLoaded = true;
      } catch (e) {
        console.error('Failed to load invoices:', e);
        invoices = [];
        toast.danger('Failed to load invoices');
      }
    }
    paymentForm = {
      invoice_id: '',
      amount: 0,
      currency: 'BHD',
      method: 'Bank Transfer',
      date: new Date().toISOString().split('T')[0],
      reference: ''
    };
  }

  function handlePaymentRowClick(row: any) {
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

  async function openEditPaymentModal() {
    if (!selectedPayment || isExpenseSettlement(selectedPayment) || !canUpdate) {
      return;
    }

    paymentSaving = true;
    try {
      const payment = await GetSupplierPayment(selectedPayment.id);
      paymentModalMode = 'edit';
      editingPaymentId = payment.id;
      paymentForm = {
        invoice_id: payment.supplier_invoice_id,
        amount: Number(payment.amount_foreign || payment.amount_bhd || 0),
        currency: payment.currency || 'BHD',
        method: payment.payment_method || 'Bank Transfer',
        date: toInputDate(payment.payment_date),
        reference: payment.reference || ''
      };
      showPaymentModal = true;
    } catch (e) {
      console.error('Failed to load supplier payment:', e);
      toast.danger(`Failed to load payment details: ${String(e)}`);
    } finally {
      paymentSaving = false;
    }
  }

  async function handleDeleteSelectedPayment() {
    if (!selectedPayment || isExpenseSettlement(selectedPayment) || !canDelete) {
      return;
    }

    if (deleteConfirmPaymentId !== selectedPayment.id) {
      deleteConfirmPaymentId = selectedPayment.id;
      return;
    }

    paymentSaving = true;
    try {
      await DeleteSupplierPayment(selectedPayment.id);
      toast.success(`Deleted supplier payment ${selectedPayment.invoice_number || ''}`.trim());
      selectedPaymentId = '';
      selectedPayment = null;
      deleteConfirmPaymentId = '';
      await loadData();
    } catch (e) {
      toast.danger(`Failed to delete payment: ${String(e)}`);
    } finally {
      paymentSaving = false;
    }
  }

  async function recordPayment() {
    if (!paymentForm.invoice_id || paymentForm.amount <= 0) {
      toast.danger('Please select an invoice and enter a valid amount');
      return;
    }

    if ((paymentForm.method === 'Bank Transfer' || paymentForm.method === 'Wire Transfer') && !paymentForm.reference.trim()) {
      toast.danger('Reference is required for bank transfers');
      return;
    }

    try {
      paymentSaving = true;
      if (paymentModalMode === 'create') {
        await RecordSupplierPayment(
          paymentForm.invoice_id,
          paymentForm.amount,
          paymentForm.currency,
          normalizeSupplierPaymentMethod(paymentForm.method),
          paymentForm.date,
          paymentForm.reference.trim()
        );
        toast.success('Payment recorded successfully');
      } else {
        const existing = await GetSupplierPayment(editingPaymentId);
        const amountForeign = paymentForm.amount;
        const exchangeRate = Number(existing.exchange_rate || (paymentForm.currency === 'BHD' ? 1 : 1));
        const amountBHD = Number((paymentForm.currency === 'BHD' ? amountForeign : amountForeign * exchangeRate).toFixed(3));

        await UpdateSupplierPayment(editingPaymentId, {
          ...existing,
          amount_foreign: amountForeign,
          currency: paymentForm.currency,
          exchange_rate: exchangeRate,
          amount_bhd: amountBHD,
          payment_method: normalizeSupplierPaymentMethod(paymentForm.method),
          payment_date: new Date(`${paymentForm.date}T00:00:00`).toISOString(),
          reference: paymentForm.reference.trim(),
        });
        toast.success('Payment updated successfully');
      }

      showPaymentModal = false;
      paymentModalMode = 'create';
      editingPaymentId = '';
      await loadData();
    } catch (e) {
      toast.danger(`Failed to ${paymentModalMode === 'create' ? 'record' : 'update'} payment: ${String(e)}`);
    } finally {
      paymentSaving = false;
    }
  }

  onMount(loadData);
  $: if (company) {
    invoicesLoaded = false;
    loadData();
  }

  $: totalPaidYTD = payments
    .filter((payment: any) => isCurrentYear(payment.payment_date))
    .reduce((sum: number, payment: any) => sum + (Number(payment.amount_bhd) || 0), 0);
</script>

<div class="screen" in:fade={{ duration: 200 }}>
  {#if !embedded}
    <PageLayout title="Payments Made" subtitle="Track outgoing payments across supplier invoices and expense settlements">
      <svelte:fragment slot="actions">
        <Button variant="secondary" on:click={openBankReconciliation}>Open Bank Recon</Button>
        <Button variant="primary" on:click={openPaymentModal} disabled={!canCreate}>Record Payment</Button>
      </svelte:fragment>
    </PageLayout>
  {:else}
    <div class="embedded-header">
      <Button variant="secondary" size="sm" on:click={openBankReconciliation}>Open Bank Recon</Button>
      <Button variant="primary" size="sm" on:click={openPaymentModal} disabled={!canCreate}>Record Payment</Button>
    </div>
  {/if}

  <!-- KPI Summary -->
  <div class="kpis">
    <div class="kpi">
      <span class="kpi-label">Total Paid (YTD)</span>
      <span class="kpi-value">{formatBHD(totalPaidYTD)} BHD</span>
    </div>
    <div class="kpi">
      <span class="kpi-label">Outstanding Invoices</span>
      <span class="kpi-value">{summary.outstanding_count || 0}</span>
    </div>
    <div class="kpi">
      <span class="kpi-label">Overdue</span>
      <span class="kpi-value kpi-alert">{summary.overdue_count || 0}</span>
    </div>
  </div>

  <!-- Payments Table -->
  {#if selectedPayment}
    <div class="selection-bar">
      <div class="selection-copy">
        <span class="selection-label">Selected Row</span>
        <span class="selection-title">{selectedPayment.invoice_number || selectedPayment.entry_number || 'Payment'} · {selectedPayment.supplier_name || selectedPayment.counterparty_name || 'Counterparty'}</span>
        <span class="selection-meta">
          {formatBHD(selectedPayment.amount_bhd || 0)} {selectedPayment.currency || 'BHD'} on {fmtDate(selectedPayment.payment_date)}
        </span>
      </div>
      <div class="selection-actions">
        {#if isExpenseSettlement(selectedPayment)}
          <span class="selection-note">Expense settlements are managed from Expenses.</span>
        {:else}
          {#if canUpdate}
            <Button variant="secondary" size="sm" on:click={openEditPaymentModal} disabled={paymentSaving}>Edit</Button>
          {/if}
          {#if canDelete}
            <Button variant={deleteConfirmPaymentId === selectedPayment.id ? 'primary' : 'ghost'} size="sm" on:click={handleDeleteSelectedPayment} disabled={paymentSaving}>
              {deleteConfirmPaymentId === selectedPayment.id ? 'Confirm Delete' : 'Delete'}
            </Button>
          {/if}
        {/if}
      </div>
    </div>
  {/if}

  <DataTable
    data={payments}
    {columns}
    {loading}
    selectedId={selectedPaymentId}
    onRowClick={handlePaymentRowClick}
    emptyMessage="No outgoing payments recorded yet"
  />
</div>

<!-- Record Payment Modal -->
{#if showPaymentModal}
  <div class="modal-overlay" on:click|self={() => showPaymentModal = false}>
    <div class="modal" in:fade={{ duration: 150 }}>
      <h2>Record Supplier Payment</h2>
      {#if paymentModalMode === 'edit'}
        <p class="modal-intro">Editing the existing supplier payment. Invoice linkage stays locked for audit safety.</p>
      {/if}

      <div class="form-group">
        <label>Supplier Invoice</label>
        {#if paymentModalMode === 'create'}
          <select bind:value={paymentForm.invoice_id} disabled={paymentSaving}>
            <option value="">Select invoice...</option>
            {#each invoices as inv}
              <option value={inv.id}>
                {inv.invoice_number || inv.id?.slice(0,8)} - {formatBHD(inv.total_bhd)} BHD
              </option>
            {/each}
          </select>
        {:else}
          <div class="locked-field">
            <div class="locked-title">{selectedPayment?.invoice_number || 'Linked supplier invoice'}</div>
            <div class="locked-subtitle">{selectedPayment?.supplier_name || selectedPayment?.counterparty_name || 'Supplier payment'}</div>
          </div>
        {/if}
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Amount</label>
          <input type="number" step="0.001" bind:value={paymentForm.amount} disabled={paymentSaving} />
        </div>
        <div class="form-group">
          <label>Currency</label>
          <select bind:value={paymentForm.currency} disabled={paymentSaving}>
            <option value="BHD">BHD</option>
            <option value="USD">USD</option>
            <option value="EUR">EUR</option>
            <option value="GBP">GBP</option>
          </select>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Payment Method</label>
          <select bind:value={paymentForm.method} disabled={paymentSaving}>
            {#each paymentMethods as method}
              <option value={method}>{method}</option>
            {/each}
          </select>
        </div>
        <div class="form-group">
          <label>Date</label>
          <input type="date" bind:value={paymentForm.date} disabled={paymentSaving} />
        </div>
      </div>

      <div class="form-group">
        <label>Reference</label>
        <input type="text" bind:value={paymentForm.reference} placeholder="Cheque/transfer reference" disabled={paymentSaving} />
      </div>

      <div class="modal-actions">
        <Button variant="secondary" on:click={() => {
          showPaymentModal = false;
          paymentModalMode = 'create';
          editingPaymentId = '';
        }} disabled={paymentSaving}>Cancel</Button>
        <Button variant="primary" on:click={recordPayment} disabled={paymentSaving}>
          {paymentSaving ? (paymentModalMode === 'create' ? 'Recording...' : 'Saving...') : (paymentModalMode === 'create' ? 'Record Payment' : 'Save Changes')}
        </Button>
      </div>
    </div>
  </div>
{/if}

<style>
  .screen {
    padding: 0;
  }

  .embedded-header {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 16px;
  }

  .kpis {
    display: flex;
    gap: 24px;
    margin-bottom: 24px;
    padding: 0 4px;
  }

  .kpi {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .kpi-label {
    font-size: 12px;
    color: var(--steel, #86868B);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .kpi-value {
    font-size: 20px;
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
    font-variant-numeric: tabular-nums lining-nums;
  }

  .kpi-alert {
    color: #d00;
  }

  .selection-bar {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    align-items: center;
    padding: 12px 14px;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 12px;
    margin-bottom: 16px;
    background: rgba(255,255,255,0.72);
  }

  .selection-copy {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .selection-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--steel, #86868B);
  }

  .selection-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
  }

  .selection-meta {
    font-size: 12px;
    color: var(--steel, #86868B);
    font-family: var(--font-mono, monospace);
  }

  .selection-actions {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
  }

  .selection-note {
    font-size: 12px;
    color: var(--steel, #86868B);
  }

  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal {
    background: var(--canvas, #fff);
    border-radius: 12px;
    padding: 32px;
    width: 480px;
    max-width: 90vw;
    box-shadow: 0 20px 60px rgba(0,0,0,0.15);
  }

  .modal h2 {
    margin: 0 0 24px;
    font-size: 18px;
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
  }

  .modal-intro {
    margin: -12px 0 18px;
    font-size: 13px;
    color: var(--steel, #86868B);
  }

  .form-group {
    margin-bottom: 16px;
  }

  .form-group label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    color: var(--steel, #86868B);
    margin-bottom: 6px;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  .form-group input,
  .form-group select {
    width: 100%;
    padding: 10px 12px;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    font-size: 14px;
    background: var(--canvas, #fff);
    color: var(--onyx, #1D1D1F);
    transition: border-color 0.15s;
  }

  .form-group input:focus,
  .form-group select:focus {
    outline: none;
    border-color: var(--onyx, #1D1D1F);
  }

  .form-row {
    display: flex;
    gap: 16px;
  }

  .form-row .form-group {
    flex: 1;
  }

  .locked-field {
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    background: rgba(247, 247, 245, 0.92);
    padding: 11px 12px;
  }

  .locked-title {
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
    font-family: var(--font-mono, monospace);
  }

  .locked-subtitle {
    margin-top: 4px;
    font-size: 13px;
    color: var(--steel, #86868B);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--border, #E5E5E5);
  }
</style>
