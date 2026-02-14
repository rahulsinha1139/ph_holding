<script lang="ts">
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import {
    GetAllSupplierPayments,
    GetSupplierPaymentsSummary,
    RecordSupplierPayment,
    GetSupplierInvoices
  } from '../../../wailsjs/go/main/App';

  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { toast } from '$lib/stores/toasts';

  export let embedded = false;

  let payments: any[] = [];
  let invoices: any[] = [];
  let loading = true;
  let showPaymentModal = false;
  let summary: any = { total_paid_bhd: 0, outstanding_count: 0, overdue_count: 0 };
  let invoicesLoaded = false;

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

  const columns = [
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
        const name = row.supplier_name || row.SupplierName || row.supplierName ||
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
    'Bank Transfer', 'Cheque', 'Letter of Credit', 'Cash', 'Wire Transfer'
  ];

  async function loadData() {
    loading = true;
    try {
      const [paymentsRes, summaryRes] = await Promise.all([
        GetAllSupplierPayments(),
        GetSupplierPaymentsSummary()
      ]);
      payments = paymentsRes || [];
      summary = summaryRes || { total_paid_bhd: 0, outstanding_count: 0, overdue_count: 0 };
    } catch (e) {
      console.error('Failed to load supplier payments:', e);
    } finally {
      loading = false;
    }
  }

  async function openPaymentModal() {
    showPaymentModal = true;
    if (!invoicesLoaded) {
      try {
        invoices = await GetSupplierInvoices() || [];
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

  async function recordPayment() {
    if (!paymentForm.invoice_id || paymentForm.amount <= 0) {
      toast.danger('Please select an invoice and enter a valid amount');
      return;
    }

    try {
      await RecordSupplierPayment(
        paymentForm.invoice_id,
        paymentForm.amount,
        paymentForm.currency,
        paymentForm.method,
        paymentForm.date,
        paymentForm.reference
      );
      toast.success('Payment recorded successfully');
      showPaymentModal = false;
      await loadData();
    } catch (e) {
      toast.danger(`Failed to record payment: ${e}`);
    }
  }

  onMount(loadData);
</script>

<div class="screen" in:fade={{ duration: 200 }}>
  {#if !embedded}
    <PageLayout title="Supplier Payments" subtitle="Track payments made to suppliers">
      <svelte:fragment slot="actions">
        <Button variant="primary" on:click={openPaymentModal}>Record Payment</Button>
      </svelte:fragment>
    </PageLayout>
  {:else}
    <div class="embedded-header">
      <Button variant="primary" size="sm" on:click={openPaymentModal}>Record Payment</Button>
    </div>
  {/if}

  <!-- KPI Summary -->
  <div class="kpis">
    <div class="kpi">
      <span class="kpi-label">Total Paid</span>
      <span class="kpi-value">{summary.total_paid_bhd?.toFixed(3) || '0.000'} BHD</span>
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
  <DataTable
    data={payments}
    {columns}
    {loading}
    emptyMessage="No supplier payments recorded yet"
  />
</div>

<!-- Record Payment Modal -->
{#if showPaymentModal}
  <div class="modal-overlay" on:click|self={() => showPaymentModal = false}>
    <div class="modal" in:fade={{ duration: 150 }}>
      <h2>Record Supplier Payment</h2>

      <div class="form-group">
        <label>Supplier Invoice</label>
        <select bind:value={paymentForm.invoice_id}>
          <option value="">Select invoice...</option>
          {#each invoices as inv}
            <option value={inv.id}>
              {inv.invoice_number || inv.id?.slice(0,8)} - {inv.total_bhd?.toFixed(3)} BHD
            </option>
          {/each}
        </select>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label>Amount</label>
          <input type="number" step="0.001" bind:value={paymentForm.amount} />
        </div>
        <div class="form-group">
          <label>Currency</label>
          <select bind:value={paymentForm.currency}>
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
          <select bind:value={paymentForm.method}>
            {#each paymentMethods as method}
              <option value={method}>{method}</option>
            {/each}
          </select>
        </div>
        <div class="form-group">
          <label>Date</label>
          <input type="date" bind:value={paymentForm.date} />
        </div>
      </div>

      <div class="form-group">
        <label>Reference</label>
        <input type="text" bind:value={paymentForm.reference} placeholder="Cheque/transfer reference" />
      </div>

      <div class="modal-actions">
        <Button variant="secondary" on:click={() => showPaymentModal = false}>Cancel</Button>
        <Button variant="primary" on:click={recordPayment}>Record Payment</Button>
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

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--border, #E5E5E5);
  }
</style>
