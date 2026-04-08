<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { GetCRMSupplierDashboard, GetCRMSupplierDashboardByYear, CreateSupplier } from '../../../wailsjs/go/main/App';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import { toast } from '$lib/stores/toasts';

  const dispatch = createEventDispatcher();

  let loading = true;
  let dashboard: any = null;
  let searchQuery = '';
  let selectedYear = 2025;
  const availableYears = [0, 2023, 2024, 2025];
  let showCreateModal = false;
  let newSupplier = {
    supplier_name: '',
    supplier_code: '',
    supplier_type: 'Manufacturer',
    country: 'United Arab Emirates',
    primary_contact: '',
    email: '',
    phone: '',
    address: '',
    lead_time_days: 30,
    payment_terms: 'Net 30',
    rating: 3
  };

  async function loadDashboard() {
    loading = true;
    try {
      if (selectedYear === 0) {
        dashboard = await GetCRMSupplierDashboard();
      } else {
        dashboard = await GetCRMSupplierDashboardByYear(selectedYear);
      }
      console.log('Supplier Dashboard loaded:', dashboard);
    } catch (err) {
      console.error('Failed to load supplier dashboard:', err);
      toast.danger('Failed to load supplier dashboard');
    } finally {
      loading = false;
    }
  }

  function handleYearChange() {
    loadDashboard();
  }

  function selectSupplier(supplierId: string) {
    dispatch('select', { id: supplierId });
  }

  async function saveSupplier() {
    if (!newSupplier.supplier_name.trim()) {
      toast.warning('Please enter supplier name');
      return;
    }
    try {
      await CreateSupplier(newSupplier as any);
      toast.success('Supplier created successfully');
      showCreateModal = false;
      resetForm();
      await loadDashboard();
    } catch (err) {
      console.error('Failed to create supplier:', err);
      toast.danger('Failed to create supplier');
    }
  }

  function resetForm() {
    newSupplier = {
      supplier_name: '',
      supplier_code: '',
      supplier_type: 'Manufacturer',
      country: 'United Arab Emirates',
      primary_contact: '',
      email: '',
      phone: '',
      address: '',
      lead_time_days: 30,
      payment_terms: 'Net 30',
      rating: 3
    };
  }

  function formatCurrency(value: number): string {
    if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
    if (value >= 1000) return `${(value / 1000).toFixed(0)}K`;
    return value.toFixed(0);
  }

  function renderRating(rating: number): string {
    if (!rating || rating <= 0) return '';
    return rating + '/5';
  }

  $: filteredSuppliers = dashboard?.suppliers?.filter((s: any) => {
    return !searchQuery ||
      s.supplier_name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      s.brands_handled?.toLowerCase().includes(searchQuery.toLowerCase());
  }) || [];

  onMount(loadDashboard);
</script>

<div class="dashboard">
  {#if loading}
    <div class="loading-state">
      <WabiSpinner size="lg" />
    </div>
  {:else if dashboard}
    <!-- Year Selector -->
    <div class="year-selector-row">
      <div class="year-selector">
        <label class="year-label">FINANCIAL YEAR</label>
        <select bind:value={selectedYear} on:change={handleYearChange} class="year-select">
          {#each availableYears as yr}
            <option value={yr}>{yr === 0 ? 'All Years' : yr}</option>
          {/each}
        </select>
      </div>
    </div>

    <!-- KPI Row -->
    <div class="kpi-row">
      <Card padding="md">
        <div class="kpi">
          <div class="kpi-label">SUPPLIERS</div>
          <div class="kpi-value">{dashboard.total_suppliers}</div>
          <div class="kpi-subtext">{dashboard.active_suppliers} active</div>
        </div>
      </Card>
      <Card padding="md">
        <div class="kpi">
          <div class="kpi-label">YTD PURCHASES</div>
          <div class="kpi-value">{formatCurrency(dashboard.total_purchases)} BHD</div>
        </div>
      </Card>
      <Card padding="md">
        <div class="kpi">
          <div class="kpi-label">PAYABLES</div>
          <div class="kpi-value">{formatCurrency(dashboard.outstanding_payables)} BHD</div>
        </div>
      </Card>
      <Card padding="md">
        <div class="kpi">
          <div class="kpi-label">OVERDUE</div>
          <div class="kpi-value">{formatCurrency(dashboard.overdue_payables)} BHD</div>
          <div class="kpi-subtext" class:warning={dashboard.overdue_payables > 0}>
            {dashboard.outstanding_payables > 0
              ? ((dashboard.overdue_payables / dashboard.outstanding_payables) * 100).toFixed(0)
              : 0}%
          </div>
        </div>
      </Card>
    </div>

    <!-- Analytics Row -->
    <div class="analytics-row">
      <!-- Top Suppliers -->
      <Card padding="md">
        <h3 class="section-title">TOP SUPPLIERS BY PURCHASES</h3>
        <div class="top-list">
          {#each dashboard.top_suppliers || [] as supplier, i}
            <div class="top-item" on:click={() => selectSupplier(supplier.id)}>
              <span class="rank">{i + 1}</span>
              <div class="supplier-info">
                <span class="name">{supplier.supplier_name}</span>
                {#if renderRating(supplier.rating)}
                  <span class="rating">{renderRating(supplier.rating)}</span>
                {/if}
              </div>
              <span class="amount">{formatCurrency(supplier.total_purchases)} BHD</span>
            </div>
          {/each}
        </div>
      </Card>

      <!-- Quick Stats -->
      <Card padding="md">
        <h3 class="section-title">ACTIVE POs</h3>
        <div class="quick-stats">
          {#each dashboard.top_suppliers?.slice(0, 5) || [] as supplier}
            <div class="quick-stat-item">
              <span class="stat-name">{supplier.supplier_name}</span>
              <span class="stat-value">{supplier.active_pos} POs</span>
            </div>
          {/each}
        </div>
      </Card>
    </div>

    <!-- Supplier Cards Section -->
    <Card padding="sm">
      <div class="suppliers-header">
        <h3 class="section-title">ALL SUPPLIERS</h3>
        <div class="filters">
          <input
            type="text"
            placeholder="Search suppliers or brands..."
            bind:value={searchQuery}
            class="search-input"
          />
          <Button variant="primary" on:click={() => showCreateModal = true}>+ New Supplier</Button>
        </div>
      </div>

      <div class="supplier-grid">
        {#each filteredSuppliers as supplier}
          <div class="supplier-card" on:click={() => selectSupplier(supplier.id)}>
            <div class="card-header">
              <span class="supplier-name">{supplier.supplier_name}</span>
              {#if renderRating(supplier.rating)}
                <span class="rating-stars">{renderRating(supplier.rating)}</span>
              {/if}
            </div>
            <div class="card-type">{supplier.supplier_type || 'Supplier'}</div>
            <div class="card-metrics">
              <div class="metric">
                <span class="metric-value">{formatCurrency(supplier.total_purchases || supplier.ytd_purchases || 0)}</span>
                <span class="metric-label">YTD</span>
              </div>
              <div class="metric">
                <span class="metric-value">{formatCurrency(supplier.outstanding_bhd || 0)}</span>
                <span class="metric-label">due</span>
              </div>
              <div class="metric">
                <span class="metric-value">{supplier.active_pos || 0}</span>
                <span class="metric-label">POs</span>
              </div>
            </div>
            {#if supplier.brands_handled}
              <div class="card-brands">{supplier.brands_handled}</div>
            {/if}
            {#if supplier.country}
              <div class="card-country">{supplier.country}</div>
            {/if}
          </div>
        {/each}
      </div>
    </Card>
  {/if}
</div>

{#if showCreateModal}
  <div class="modal-backdrop" on:click={() => showCreateModal = false}>
    <div class="modal" on:click|stopPropagation>
      <h3>Create New Supplier</h3>
      <div class="form-grid">
        <div class="form-group">
          <label>Supplier Name <span class="required">*</span></label>
          <input type="text" bind:value={newSupplier.supplier_name} placeholder="Company name" />
        </div>
        <div class="form-group">
          <label>Supplier Code</label>
          <input type="text" bind:value={newSupplier.supplier_code} placeholder="Auto-generated if empty" />
        </div>
        <div class="form-group">
          <label>Supplier Type</label>
          <select bind:value={newSupplier.supplier_type}>
            <option value="Manufacturer">Manufacturer</option>
            <option value="Distributor">Distributor</option>
            <option value="Agent">Agent</option>
          </select>
        </div>
        <div class="form-group">
          <label>Country</label>
          <input type="text" bind:value={newSupplier.country} placeholder="Country" />
        </div>
        <div class="form-group">
          <label>Lead Time (Days)</label>
          <input type="number" bind:value={newSupplier.lead_time_days} placeholder="30" />
        </div>
        <div class="form-group">
          <label>Payment Terms <span class="required">*</span></label>
          <select bind:value={newSupplier.payment_terms}>
            <option value="Net 30">Net 30</option>
            <option value="Net 60">Net 60</option>
            <option value="Net 90">Net 90</option>
            <option value="CIA">CIA (Cash in Advance)</option>
            <option value="COD">COD (Cash on Delivery)</option>
            <option value="LC">Letter of Credit</option>
            <option value="30/70">30% Advance / 70% on Delivery</option>
            <option value="50/50">50% Advance / 50% on Delivery</option>
          </select>
        </div>
        <div class="form-group">
          <label>Rating</label>
          <select bind:value={newSupplier.rating}>
            <option value={1}>1 Star</option>
            <option value={2}>2 Stars</option>
            <option value={3}>3 Stars</option>
            <option value={4}>4 Stars</option>
            <option value={5}>5 Stars</option>
          </select>
        </div>
        <div class="form-group">
          <label>Primary Contact</label>
          <input type="text" bind:value={newSupplier.primary_contact} placeholder="Contact person name" />
        </div>
        <div class="form-group">
          <label>Email</label>
          <input type="email" bind:value={newSupplier.email} placeholder="email@example.com" />
        </div>
        <div class="form-group">
          <label>Phone</label>
          <input type="text" bind:value={newSupplier.phone} placeholder="+973-XXXX-XXXX" />
        </div>
        <div class="form-group full-width">
          <label>Address</label>
          <textarea bind:value={newSupplier.address} rows="2" placeholder="Full address"></textarea>
        </div>
      </div>
      <div class="modal-actions">
        <Button variant="ghost" on:click={() => showCreateModal = false}>Cancel</Button>
        <Button variant="primary" on:click={saveSupplier}>Create Supplier</Button>
      </div>
    </div>
  </div>
{/if}

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
  }

  .loading-state {
    display: flex;
    justify-content: center;
    align-items: center;
    height: 400px;
  }

  /* Year Selector */
  .year-selector-row {
    display: flex;
    justify-content: flex-end;
    margin-bottom: -8px;
  }

  .year-selector {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .year-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    font-weight: 600;
  }

  .year-select {
    padding: 6px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 14px;
    font-weight: 600;
    background: var(--surface);
    color: var(--text-primary);
    cursor: pointer;
    min-width: 120px;
  }

  .year-select:focus {
    border-color: var(--brand-indigo);
    outline: none;
  }

  /* KPI Row */
  .kpi-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }

  .kpi {
    text-align: center;
  }

  .kpi-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }

  .kpi-value {
    font-size: 28px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .kpi-subtext {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 4px;
  }

  .kpi-subtext.warning {
    color: #F59E0B;
  }

  /* Analytics Row */
  .analytics-row {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 16px;
  }

  .section-title {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin: 0 0 16px 0;
    font-weight: 600;
  }

  /* Top Suppliers List */
  .top-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .top-item {
    display: grid;
    grid-template-columns: 24px 1fr auto;
    align-items: center;
    gap: 12px;
    padding: 8px 4px;
    cursor: pointer;
    border-radius: 4px;
  }

  .top-item:hover {
    background: var(--interactive-hover);
  }

  .rank {
    font-size: 12px;
    color: var(--text-muted);
    text-align: center;
  }

  .supplier-info {
    display: flex;
    flex-direction: column;
  }

  .name {
    font-size: 13px;
    font-weight: 500;
  }

  .rating {
    font-size: 11px;
    color: #F59E0B;
  }

  .amount {
    font-size: 13px;
    font-weight: 600;
    font-family: 'JetBrains Mono', monospace;
  }

  /* Quick Stats */
  .quick-stats {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .quick-stat-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }

  .quick-stat-item:last-child {
    border-bottom: none;
  }

  .stat-name {
    font-size: 13px;
    color: var(--text-secondary);
  }

  .stat-value {
    font-size: 14px;
    font-weight: 600;
    color: var(--brand-indigo);
  }

  /* Suppliers Header */
  .suppliers-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .filters {
    display: flex;
    gap: 12px;
  }

  .search-input {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 14px;
    background: var(--surface);
    color: var(--text-primary);
    width: 280px;
  }

  /* Supplier Grid */
  .supplier-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 16px;
    max-height: calc(100vh - 500px);
    overflow-y: auto;
    padding: 4px;
  }

  .supplier-card {
    background: var(--surface-elevated);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 16px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .supplier-card:hover {
    border-color: var(--brand-indigo);
    box-shadow: 0 4px 12px rgba(99, 102, 241, 0.1);
    transform: translateY(-2px);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 4px;
  }

  .supplier-name {
    font-weight: 600;
    font-size: 14px;
    line-height: 1.3;
    word-break: break-word;
    overflow-wrap: break-word;
    max-width: 180px;
  }

  .rating-stars {
    font-size: 12px;
    color: #F59E0B;
  }

  .card-type {
    font-size: 12px;
    color: var(--text-muted);
    margin-bottom: 12px;
  }

  .card-metrics {
    display: flex;
    gap: 16px;
    margin-bottom: 8px;
  }

  .metric {
    display: flex;
    flex-direction: column;
  }

  .metric-value {
    font-size: 16px;
    font-weight: 600;
    font-family: 'JetBrains Mono', monospace;
    word-break: break-word;
    overflow-wrap: break-word;
  }

  .metric-label {
    font-size: 10px;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  .card-brands {
    font-size: 11px;
    color: var(--brand-indigo);
    margin-bottom: 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .card-country {
    font-size: 11px;
    color: var(--text-muted);
  }

  /* Responsive */
  @media (max-width: 1024px) {
    .analytics-row {
      grid-template-columns: 1fr;
    }

    .kpi-row {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  /* Modal styles */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal {
    background: var(--surface);
    padding: 24px;
    border-radius: 12px;
    width: 600px;
    max-height: 80vh;
    overflow-y: auto;
  }

  .modal h3 {
    margin: 0 0 20px 0;
    font-size: 18px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 20px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
  }

  .form-group.full-width {
    grid-column: 1 / -1;
  }

  .form-group label {
    font-size: 12px;
    text-transform: uppercase;
    color: var(--text-muted);
    margin-bottom: 4px;
  }

  .required {
    color: #EF4444;
  }

  .form-group input,
  .form-group select,
  .form-group textarea {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 14px;
    background: var(--surface);
    color: var(--text-primary);
    font-family: var(--font-family);
  }

  .form-group textarea {
    resize: vertical;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 20px;
  }
</style>
