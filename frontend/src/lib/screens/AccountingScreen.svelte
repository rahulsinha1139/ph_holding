<script lang="ts">
  import { onMount } from 'svelte';
  import WabiSpinner from '../components/ui/WabiSpinner.svelte';
  import {
    GetAllCustomers,
    GetCustomerLedger,
    GetExpenseLedger,
    GetJournalEntries,
    GetOperationalBalanceSheet,
    GetOperationalLedger,
    GetOperationalProfitLoss,
    GetSupplierLedger,
    ListSuppliers,
  } from '../../../wailsjs/go/main/App';
  import { toast } from '../stores/toasts';

  export let embedded = false;
  export let company: 'PH Trading' | 'AHS Trading' = 'PH Trading';

  type ViewID = 'overview' | 'general' | 'customer' | 'supplier' | 'expenses' | 'pl' | 'balance' | 'journal';

  const views: { id: ViewID; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'general', label: 'General Ledger' },
    { id: 'customer', label: 'Customer Ledger' },
    { id: 'supplier', label: 'Supplier Ledger' },
    { id: 'expenses', label: 'Expense Ledger' },
    { id: 'pl', label: 'P&L' },
    { id: 'balance', label: 'Balance Sheet' },
    { id: 'journal', label: 'Journal Audit' },
  ];

  let currentView: ViewID = 'overview';
  let loading = true;
  let startDate = `${new Date().getFullYear()}-01-01`;
  let endDate = new Date().toISOString().slice(0, 10);
  let selectedCustomerId = '';
  let selectedSupplierId = '';
  let selectedSourceType = '';
  let statusFilter = '';

  let ledger: any[] = [];
  let plReport: any = null;
  let balanceSheet: any = null;
  let journalEntries: any[] = [];
  let customers: any[] = [];
  let suppliers: any[] = [];

  $: activeRows = ledger;
  $: revenue = Number(plReport?.revenue_bhd || 0);
  $: expenses = Number(plReport?.expenses_bhd || 0);
  $: netIncome = Number(plReport?.net_income_bhd || 0);
  $: cash = Number(balanceSheet?.cash_bhd || 0);
  $: ar = Number(balanceSheet?.accounts_receivable_bhd || 0);
  $: ap = Number(balanceSheet?.accounts_payable_bhd || 0);
  $: customerCredits = Number(balanceSheet?.customer_credits_bhd || 0);

  function money(value: number) {
    return `${Number(value || 0).toLocaleString('en-US', { minimumFractionDigits: 3, maximumFractionDigits: 3 })} BHD`;
  }

  function signedMoney(debit: number, credit: number) {
    const value = Number(debit || 0) - Number(credit || 0);
    return `${value >= 0 ? '' : '-'}${money(Math.abs(value))}`;
  }

  function sourceLabel(source: string) {
    return String(source || '')
      .replace(/_/g, ' ')
      .toLowerCase()
      .replace(/\b\w/g, (char) => char.toUpperCase());
  }

  function matchesCompany(division?: string) {
    return (division || 'PH Trading') === company;
  }

  function baseFilter() {
    return {
      division: company,
      start_date: startDate,
      end_date: endDate,
      customer_id: selectedCustomerId,
      supplier_id: selectedSupplierId,
      vendor_id: '',
      source_type: selectedSourceType,
      status: statusFilter,
    };
  }

  async function loadReferenceData() {
    const [customerRows, supplierRows] = await Promise.all([
      GetAllCustomers().catch(() => []),
      ListSuppliers(500, 0).catch(() => []),
    ]);
    customers = (customerRows || []).filter((customer: any) => matchesCompany(customer.division));
    suppliers = (supplierRows || []).filter((supplier: any) => matchesCompany(supplier.division));
  }

  async function loadAccountingData() {
    loading = true;
    try {
      await loadReferenceData();

      if (currentView === 'customer' && selectedCustomerId) {
        ledger = await GetCustomerLedger(selectedCustomerId, startDate, endDate);
      } else if (currentView === 'supplier' && selectedSupplierId) {
        ledger = await GetSupplierLedger(selectedSupplierId, startDate, endDate);
      } else if (currentView === 'expenses') {
        ledger = await GetExpenseLedger({ ...baseFilter(), source_type: 'EXPENSE' });
      } else if (currentView === 'journal') {
        const year = new Date(startDate).getFullYear();
        journalEntries = await GetJournalEntries(year, 0, null, 500);
        ledger = [];
      } else {
        ledger = await GetOperationalLedger(baseFilter());
      }

      const [pl, bs] = await Promise.all([
        GetOperationalProfitLoss(baseFilter()),
        GetOperationalBalanceSheet(endDate, company),
      ]);
      plReport = pl;
      balanceSheet = bs;
    } catch (err) {
      toast.danger(`Accounting load failed: ${String(err)}`);
      ledger = [];
    } finally {
      loading = false;
    }
  }

  function applyPreset(preset: 'month' | 'quarter' | 'year') {
    const now = new Date();
    let start = new Date(now.getFullYear(), 0, 1);
    if (preset === 'month') start = new Date(now.getFullYear(), now.getMonth(), 1);
    if (preset === 'quarter') start = new Date(now.getFullYear(), Math.floor(now.getMonth() / 3) * 3, 1);
    startDate = start.toISOString().slice(0, 10);
    endDate = now.toISOString().slice(0, 10);
    void loadAccountingData();
  }

  $: if (company) {
    void loadAccountingData();
  }

  onMount(loadAccountingData);
</script>

<div class:embedded class="accounting-page">
  <header class="accounting-header">
    <div>
      <h1>Accounting</h1>
      <p>Operational ledgers and financial statements from live ERP activity.</p>
    </div>
    <button class="primary-action" on:click={loadAccountingData} disabled={loading}>
      Refresh
    </button>
  </header>

  <nav class="tabs" aria-label="Accounting views">
    {#each views as view}
      <button class:active={currentView === view.id} on:click={() => { currentView = view.id; loadAccountingData(); }}>
        {view.label}
      </button>
    {/each}
  </nav>

  <section class="filters">
    <div class="preset-group">
      <button on:click={() => applyPreset('month')}>This Month</button>
      <button on:click={() => applyPreset('quarter')}>This Quarter</button>
      <button on:click={() => applyPreset('year')}>This Year</button>
    </div>
    <label>
      <span>From</span>
      <input type="date" bind:value={startDate} />
    </label>
    <label>
      <span>To</span>
      <input type="date" bind:value={endDate} />
    </label>
    {#if currentView === 'customer'}
      <label>
        <span>Customer</span>
        <select bind:value={selectedCustomerId}>
          <option value="">All customers</option>
          {#each customers as customer}
            <option value={customer.id}>{customer.business_name}</option>
          {/each}
        </select>
      </label>
    {:else if currentView === 'supplier'}
      <label>
        <span>Supplier</span>
        <select bind:value={selectedSupplierId}>
          <option value="">All suppliers</option>
          {#each suppliers as supplier}
            <option value={supplier.id}>{supplier.supplier_name}</option>
          {/each}
        </select>
      </label>
    {:else if currentView === 'general'}
      <label>
        <span>Source</span>
        <select bind:value={selectedSourceType}>
          <option value="">All activity</option>
          <option value="CUSTOMER_INVOICE">Customer invoices</option>
          <option value="CUSTOMER_RECEIPT">Customer receipts</option>
          <option value="CUSTOMER_PAYMENT">Legacy customer payments</option>
          <option value="SUPPLIER_INVOICE">Supplier invoices</option>
          <option value="SUPPLIER_PAYMENT">Supplier payments</option>
          <option value="EXPENSE">Expenses</option>
        </select>
      </label>
    {/if}
    <label>
      <span>Status</span>
      <input bind:value={statusFilter} placeholder="Any status" />
    </label>
    <button class="secondary-action" on:click={loadAccountingData}>Apply Filters</button>
  </section>

  {#if loading}
    <div class="loading"><WabiSpinner size="lg" /></div>
  {:else}
    {#if currentView === 'overview'}
      <section class="metric-grid">
        <article><span>Revenue</span><strong>{money(revenue)}</strong></article>
        <article><span>Expenses</span><strong>{money(expenses)}</strong></article>
        <article class:positive={netIncome >= 0} class:negative={netIncome < 0}><span>Net Income</span><strong>{money(netIncome)}</strong></article>
        <article><span>Cash Position</span><strong>{money(cash)}</strong></article>
        <article><span>Receivables</span><strong>{money(ar)}</strong></article>
        <article><span>Payables</span><strong>{money(ap)}</strong></article>
      </section>
    {:else if currentView === 'pl'}
      <section class="statement">
        <div class="statement-row total"><span>Revenue</span><strong>{money(plReport?.revenue_bhd)}</strong></div>
        <div class="statement-row"><span>Cost of Goods Sold</span><strong>{money(plReport?.cogs_bhd)}</strong></div>
        <div class="statement-row total"><span>Gross Profit</span><strong>{money(plReport?.gross_profit_bhd)}</strong></div>
        <div class="statement-row"><span>Operating Expenses</span><strong>{money(plReport?.expenses_bhd)}</strong></div>
        <div class="statement-row grand" class:negative={netIncome < 0}><span>Net Income</span><strong>{money(netIncome)}</strong></div>
        {#if plReport?.expense_breakdown?.length}
          <h3>Expense Breakdown</h3>
          {#each plReport.expense_breakdown as item}
            <div class="statement-row"><span>{item.account_name}</span><strong>{money(item.balance)}</strong></div>
          {/each}
        {/if}
      </section>
    {:else if currentView === 'balance'}
      <section class="statement">
        <div class="statement-row"><span>Cash</span><strong>{money(cash)}</strong></div>
        <div class="statement-row"><span>Accounts Receivable</span><strong>{money(ar)}</strong></div>
        <div class="statement-row"><span>Customer Credits / On Account</span><strong>{money(customerCredits)}</strong></div>
        <div class="statement-row"><span>Accounts Payable</span><strong>{money(ap)}</strong></div>
        <div class="statement-row"><span>Expense Liabilities</span><strong>{money(balanceSheet?.expense_liability_bhd)}</strong></div>
        <div class="statement-row grand"><span>Net Position</span><strong>{money(balanceSheet?.net_position_bhd)}</strong></div>
      </section>
    {:else if currentView === 'journal'}
      <section class="table-card">
        <table>
          <thead><tr><th>Date</th><th>Entry</th><th>Description</th><th class="right">Debit</th><th class="right">Credit</th><th>Status</th></tr></thead>
          <tbody>
            {#each journalEntries as entry}
              <tr>
                <td>{entry.entry_date ? new Date(entry.entry_date).toISOString().slice(0, 10) : '-'}</td>
                <td>{entry.entry_number || entry.id}</td>
                <td>{entry.description}</td>
                <td class="right">{money(entry.debit_total)}</td>
                <td class="right">{money(entry.credit_total)}</td>
                <td>{entry.is_posted ? 'Posted' : 'Draft'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/if}
  {/if}
{#if currentView !== 'pl' && currentView !== 'balance' && currentView !== 'journal' && !loading}
  <section class="table-card ledger-host">
    <table>
      <thead>
        <tr>
          <th>Date</th>
          <th>Type</th>
          <th>Reference</th>
          <th>Party</th>
          <th>Description</th>
          <th class="right">Debit</th>
          <th class="right">Credit</th>
          <th class="right">Balance</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {#if activeRows.length === 0}
          <tr><td colspan="9" class="empty">No ledger activity for these filters.</td></tr>
        {:else}
          {#each activeRows as row}
            <tr>
              <td>{row.entry_date || '-'}</td>
              <td>{sourceLabel(row.source_type)}</td>
              <td>{row.source_number || '-'}</td>
              <td>{row.party_name || '-'}</td>
              <td>{row.description || '-'}</td>
              <td class="right">{row.debit_bhd ? money(row.debit_bhd) : '-'}</td>
              <td class="right">{row.credit_bhd ? money(row.credit_bhd) : '-'}</td>
              <td class="right">{money(row.balance_bhd)}</td>
              <td>{row.status || '-'}</td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </section>
{/if}
</div>

<style>
  .accounting-page {
    min-height: 100%;
    padding: 24px;
    background: var(--bg-base);
    color: var(--text-primary);
  }

  .accounting-page.embedded {
    padding: 0;
  }

  .accounting-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 18px;
    margin-bottom: 18px;
  }

  .accounting-header h1 {
    margin: 0;
    font-size: 26px;
    font-weight: 650;
  }

  .accounting-header p {
    margin: 6px 0 0;
    color: var(--text-secondary);
  }

  .tabs {
    display: flex;
    flex-wrap: nowrap;
    gap: 8px;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--border-subtle, #dde8dd);
    overflow-x: auto;
    overflow-y: hidden;
    padding-bottom: 6px;
    scrollbar-width: thin;
  }

  .tabs button,
  .preset-group button,
  .primary-action,
  .secondary-action {
    border: 1px solid var(--border-subtle, #dde8dd);
    border-radius: 8px;
    background: #fff;
    color: var(--text-primary);
    padding: 9px 13px;
    font-weight: 700;
    cursor: pointer;
  }

  .tabs button {
    flex: 0 0 auto;
    border-color: transparent;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
    white-space: nowrap;
  }

  .tabs button.active {
    border-bottom: 3px solid #1f3329;
    color: #1f3329;
  }

  .primary-action,
  .secondary-action {
    background: #4f8f45;
    color: white;
    border-color: #4f8f45;
  }

  .secondary-action {
    align-self: end;
  }

  .filters {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 12px;
    align-items: end;
    margin-bottom: 18px;
    padding: 14px;
    border: 1px solid var(--border-subtle, #dde8dd);
    border-radius: 8px;
    background: #fff;
  }

  .preset-group {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  label span {
    display: block;
    margin-bottom: 5px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    font-weight: 700;
  }

  input,
  select {
    width: 100%;
    min-height: 38px;
    border: 1px solid var(--border-subtle, #dde8dd);
    border-radius: 8px;
    padding: 8px 10px;
    background: #fff;
    color: var(--text-primary);
  }

  .metric-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 12px;
    margin-bottom: 16px;
  }

  .metric-grid article,
  .statement,
  .table-card {
    border: 1px solid var(--border-subtle, #dde8dd);
    border-radius: 8px;
    background: #fff;
  }

  .metric-grid article {
    padding: 18px;
  }

  .metric-grid span {
    display: block;
    margin-bottom: 8px;
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-secondary);
    font-weight: 700;
  }

  .metric-grid strong {
    font-size: 24px;
  }

  .positive strong { color: #4f8f45; }
  .negative strong { color: #c2413a; }

  .statement {
    max-width: 820px;
    padding: 18px;
  }

  .statement h3 {
    margin: 24px 0 10px;
  }

  .statement-row {
    display: flex;
    justify-content: space-between;
    gap: 18px;
    padding: 12px 0;
    border-bottom: 1px solid #edf2ed;
  }

  .statement-row.total {
    font-weight: 750;
  }

  .statement-row.grand {
    margin-top: 8px;
    border-bottom: 0;
    font-size: 20px;
    font-weight: 800;
  }

  .table-card {
    overflow: auto;
  }

  .ledger-host {
    margin-top: -16px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    min-width: 980px;
  }

  th,
  td {
    padding: 12px 14px;
    border-bottom: 1px solid #edf2ed;
    text-align: left;
    vertical-align: top;
  }

  th {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    background: #f7faf7;
  }

  .right {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .empty,
  .loading {
    padding: 36px;
    text-align: center;
    color: var(--text-secondary);
  }
</style>
