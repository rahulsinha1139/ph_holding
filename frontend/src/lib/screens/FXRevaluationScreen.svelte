<script lang="ts">
  /**
   * FXRevaluationScreen - Foreign Exchange Revaluation
   *
   * Features:
   * - FX rate management and history
   * - Unrealized gain/loss calculation
   * - Multi-currency exposure report
   * - Batch revaluation for all foreign currency accounts
   * - Post/reverse revaluation entries
   *
   * Design System: Wabi-Sabi minimalism x Bloomberg data density
   */

  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports
  import {
    GetActiveBankAccounts,
    GetLatestFXRate,
    CreateFXRate,
    GetFXRevaluations,
    CalculateFXRevaluation,
    RevalueAllForeignAccounts,
    PostFXRevaluation,
    GetFXExposureReport,
    GetFXGainLossSummary,
  } from '../../../wailsjs/go/main/App';
  import { escapeHtml } from '$lib/utils/escapeHtml';
  import { formatNumber } from '$lib/utils/formatters';

  // Design system components
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Modal from '$lib/components/layout/Modal.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import { toast } from '$lib/stores/toasts';

  // Props
  export let embedded = false;

  // Types - aligned with Wails CompanyBankAccount
  interface BankAccount {
    id: string;
    bank_name: string;
    account_name?: string;
    account_number: string;
    currency: string;
    is_active?: boolean;
  }

  interface FXRate {
    id: string;
    from_currency: string;
    to_currency: string;
    rate_date: any;
    rate: number;
    source: string;
  }

  interface FXRevaluation {
    id: string;
    bank_account_id: string;
    revaluation_date: any;
    foreign_currency: string;
    foreign_balance: number;
    previous_rate: number;
    previous_bhd: number;
    current_rate: number;
    current_bhd: number;
    gain_loss_bhd: number;
    is_posted: boolean;
    posted_by: string;
    posted_at: any;
  }

  interface ExposureReport {
    currency: string;
    account_count: number;
    total_foreign: number;
    current_rate: number;
    total_bhd: number;
    unrealized_gain: number;
    percent_exposure: number;
  }

  type ViewTab = 'exposure' | 'revaluations' | 'rates';

  // State - using any[] to accommodate Wails-generated classes
  let bankAccounts: any[] = [];
  let foreignAccounts: any[] = [];
  let revaluations: any[] = [];
  let exposureReport: any[] = [];
  let totalExposure = 0;
  let selectedAccountId = '';
  let activeTab: ViewTab = 'exposure';
  let loading = true;

  // Summary
  let ytdGain = 0;
  let ytdLoss = 0;
  let netGainLoss = 0;
  let unpostedNet = 0;

  // Modal state
  let showRateModal = false;
  let showRevalModal = false;
  let modalLoading = false;

  // Rate form
  let rateCurrency = 'USD';
  let rateValue = '';
  let rateSource = 'CBB';

  // Common currencies
  const currencies = ['USD', 'EUR', 'GBP', 'SAR', 'AED', 'KWD', 'QAR', 'OMR'];

  // DataTable columns for revaluations
  const revalColumns = [
    {
      key: 'revaluation_date',
      label: 'Date',
      sortable: true,
      width: '100px',
      render: (row: FXRevaluation) => `<span style="font-family: var(--font-mono); font-size: 12px;">${formatDate(row.revaluation_date)}</span>`
    },
    {
      key: 'foreign_currency',
      label: 'Currency',
      width: '80px',
      render: (row: FXRevaluation) => `<span style="font-weight: 600;">${escapeHtml(row.foreign_currency || '')}</span>`
    },
    {
      key: 'foreign_balance',
      label: 'Foreign Bal',
      width: '120px',
      align: 'right' as const,
      render: (row: FXRevaluation) => `<span style="font-family: var(--font-mono);">${formatAmount(row.foreign_balance, row.foreign_currency)}</span>`
    },
    {
      key: 'current_rate',
      label: 'Rate',
      width: '90px',
      align: 'right' as const,
      render: (row: FXRevaluation) => `<span style="font-family: var(--font-mono); font-size: 12px;">${row.current_rate.toFixed(6)}</span>`
    },
    {
      key: 'current_bhd',
      label: 'BHD Value',
      width: '120px',
      align: 'right' as const,
      render: (row: FXRevaluation) => `<span style="font-family: var(--font-mono);">${formatBHD(row.current_bhd)}</span>`
    },
    {
      key: 'gain_loss_bhd',
      label: 'Gain/Loss',
      sortable: true,
      width: '110px',
      align: 'right' as const,
      render: (row: FXRevaluation) => {
        const color = row.gain_loss_bhd >= 0 ? '#10B981' : '#EF4444';
        const sign = row.gain_loss_bhd >= 0 ? '+' : '';
        return `<span style="font-family: var(--font-mono); color: ${color}; font-weight: 600;">${sign}${formatBHD(row.gain_loss_bhd)}</span>`;
      }
    },
    {
      key: 'is_posted',
      label: 'Status',
      width: '90px',
      render: (row: FXRevaluation) => {
        if (row.is_posted) {
          return `<span style="color: #10B981; font-size: 12px;">Posted</span>`;
        }
        return `<span style="color: #F59E0B; font-size: 12px;">Pending</span>`;
      }
    }
  ];

  // Exposure columns
  const exposureColumns = [
    {
      key: 'currency',
      label: 'Currency',
      width: '80px',
      render: (row: ExposureReport) => `<span style="font-weight: 600;">${escapeHtml(row.currency || '')}</span>`
    },
    {
      key: 'account_count',
      label: 'Accounts',
      width: '80px',
      align: 'center' as const
    },
    {
      key: 'total_foreign',
      label: 'Foreign Balance',
      width: '130px',
      align: 'right' as const,
      render: (row: ExposureReport) => `<span style="font-family: var(--font-mono);">${formatAmount(row.total_foreign, row.currency)}</span>`
    },
    {
      key: 'current_rate',
      label: 'Rate',
      width: '90px',
      align: 'right' as const,
      render: (row: ExposureReport) => `<span style="font-family: var(--font-mono); font-size: 12px;">${row.current_rate.toFixed(6)}</span>`
    },
    {
      key: 'total_bhd',
      label: 'BHD Value',
      width: '130px',
      align: 'right' as const,
      render: (row: ExposureReport) => `<span style="font-family: var(--font-mono); font-weight: 600;">${formatBHD(row.total_bhd)}</span>`
    },
    {
      key: 'percent_exposure',
      label: 'Exposure %',
      width: '100px',
      align: 'right' as const,
      render: (row: ExposureReport) => `<span style="font-size: 13px;">${row.percent_exposure.toFixed(1)}%</span>`
    },
    {
      key: 'unrealized_gain',
      label: 'Unrealized',
      width: '110px',
      align: 'right' as const,
      render: (row: ExposureReport) => {
        const color = row.unrealized_gain >= 0 ? '#10B981' : '#EF4444';
        return `<span style="font-family: var(--font-mono); color: ${color};">${formatBHD(row.unrealized_gain)}</span>`;
      }
    }
  ];

  // Formatters
  function formatBHD(value: number): string {
    return formatNumber(value || 0, 3);
  }

  function formatAmount(value: number, currency: string): string {
    const decimals = currency === 'BHD' || currency === 'KWD' || currency === 'OMR' ? 3 : 2;
    return formatNumber(value || 0, decimals);
  }

  function formatDate(dateValue: any): string {
    if (!dateValue) return '-';
    try {
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
    } catch {
      return '-';
    }
  }

  // Data loading
  async function loadData() {
    loading = true;
    try {
      const accounts = await GetActiveBankAccounts();
      bankAccounts = accounts || [];
      foreignAccounts = bankAccounts.filter((a: BankAccount) => a.currency !== 'BHD');

      if (foreignAccounts.length > 0 && !selectedAccountId) {
        selectedAccountId = foreignAccounts[0].id;
      }

      await Promise.all([
        loadExposure(),
        loadRevaluations(),
        loadSummary()
      ]);
    } catch (err) {
      console.error('Failed to load data:', err);
      toast.danger('Failed to load FX data');
    } finally {
      loading = false;
    }
  }

  async function loadExposure() {
    try {
      const report = await GetFXExposureReport();
      exposureReport = report || [];
      // Calculate total exposure from report
      totalExposure = exposureReport.reduce((sum: number, r: any) => sum + (r.total_bhd || 0), 0);
    } catch (err) {
      console.error('Failed to load exposure:', err);
    }
  }

  async function loadRevaluations() {
    if (!selectedAccountId) return;
    try {
      const result = await GetFXRevaluations(selectedAccountId);
      revaluations = result || [];
    } catch (err) {
      console.error('Failed to load revaluations:', err);
    }
  }

  async function loadSummary() {
    try {
      const year = new Date().getFullYear();
      const summary = await GetFXGainLossSummary(year);
      ytdGain = summary?.total_gain || 0;
      ytdLoss = summary?.total_loss || 0;
      netGainLoss = summary?.net_gain_loss || 0;
      unpostedNet = summary?.unposted_net || 0;
    } catch (err) {
      console.error('Failed to load summary:', err);
    }
  }

  // Actions
  async function handleAddRate() {
    if (!rateValue) {
      toast.warning('Please enter a rate');
      return;
    }

    modalLoading = true;
    try {
      await CreateFXRate(rateCurrency, 'BHD', parseFloat(rateValue), new Date(), rateSource);
      toast.success(`${rateCurrency}/BHD rate updated`);
      showRateModal = false;
      rateValue = '';
      await loadExposure();
    } catch (err: any) {
      console.error('Failed to add rate:', err);
      toast.danger(err?.message || 'Failed to add rate');
    } finally {
      modalLoading = false;
    }
  }

  async function handleRevalue() {
    if (!selectedAccountId) return;

    modalLoading = true;
    try {
      await CalculateFXRevaluation(selectedAccountId, new Date());
      toast.success('Revaluation calculated');
      await loadRevaluations();
      await loadSummary();
    } catch (err: any) {
      console.error('Failed to revalue:', err);
      toast.danger(err?.message || 'Failed to calculate revaluation');
    } finally {
      modalLoading = false;
    }
  }

  async function handleBatchRevalue() {
    modalLoading = true;
    try {
      const revals = await RevalueAllForeignAccounts(new Date());
      const count = (revals || []).length;
      const total = (revals || []).reduce((sum: number, r: any) => sum + (r.gain_loss_bhd || 0), 0);
      toast.success(`Revalued ${count} accounts. Net: ${formatBHD(total)} BHD`);
      await Promise.all([loadExposure(), loadRevaluations(), loadSummary()]);
    } catch (err: any) {
      console.error('Batch revalue failed:', err);
      toast.danger(err?.message || 'Batch revaluation failed');
    } finally {
      modalLoading = false;
    }
  }

  async function handlePost(reval: FXRevaluation) {
    try {
      await PostFXRevaluation(reval.id, 'admin');
      toast.success('Revaluation posted');
      await loadRevaluations();
      await loadSummary();
    } catch (err: any) {
      console.error('Failed to post:', err);
      toast.danger(err?.message || 'Failed to post');
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<PageLayout title="FX Revaluation" subtitle="Foreign exchange exposure and revaluation" {embedded}>
  {#if loading}
    <div class="loading-container">
      <WabiSpinner size="lg" />
      <p>Loading FX data...</p>
    </div>
  {:else}
    <!-- KPIs -->
    <div class="kpi-grid">
      <Card variant="elevated" padding="sm">
        <div class="kpi">
          <span class="kpi-label">FX Exposure</span>
          <span class="kpi-value primary">{formatBHD(totalExposure)} BHD</span>
        </div>
      </Card>
      <Card variant="elevated" padding="sm">
        <div class="kpi">
          <span class="kpi-label">YTD Gain</span>
          <span class="kpi-value success">+{formatBHD(ytdGain)}</span>
        </div>
      </Card>
      <Card variant="elevated" padding="sm">
        <div class="kpi">
          <span class="kpi-label">YTD Loss</span>
          <span class="kpi-value danger-text">{formatBHD(ytdLoss)}</span>
        </div>
      </Card>
      <Card variant="elevated" padding="sm">
        <div class="kpi">
          <span class="kpi-label">Net (Unposted)</span>
          <span class="kpi-value" class:success={netGainLoss >= 0} class:danger-text={netGainLoss < 0}>
            {formatBHD(netGainLoss)} ({formatBHD(unpostedNet)})
          </span>
        </div>
      </Card>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="left">
        <div class="tabs">
          <button class:active={activeTab === 'exposure'} on:click={() => activeTab = 'exposure'}>
            Exposure
          </button>
          <button class:active={activeTab === 'revaluations'} on:click={() => activeTab = 'revaluations'}>
            Revaluations
          </button>
        </div>
        {#if activeTab === 'revaluations'}
          <select bind:value={selectedAccountId} on:change={loadRevaluations} class="account-select">
            {#each foreignAccounts as account}
              <option value={account.id}>{account.bank_name} ({account.currency})</option>
            {/each}
          </select>
        {/if}
      </div>
      <div class="right">
        <Button variant="secondary" on:click={() => showRateModal = true}>
          Update Rate
        </Button>
        <Button variant="primary" on:click={handleBatchRevalue} loading={modalLoading}>
          Revalue All
        </Button>
      </div>
    </div>

    <!-- Content -->
    {#if activeTab === 'exposure'}
      <DataTable
        data={exposureReport}
        columns={exposureColumns}
        emptyMessage="No foreign currency exposure"
        compact={true}
      />
    {:else if activeTab === 'revaluations'}
      <div class="reval-actions">
        <Button variant="secondary" size="sm" on:click={handleRevalue} disabled={!selectedAccountId}>
          Calculate for Selected Account
        </Button>
      </div>
      <DataTable
        data={revaluations}
        columns={revalColumns}
        emptyMessage="No revaluations for this account"
        onRowClick={(row) => !row.is_posted && handlePost(row)}
        compact={true}
      />
    {/if}
  {/if}
</PageLayout>

<!-- Rate Modal -->
<Modal bind:open={showRateModal} title="Update FX Rate" size="sm">
  <div class="form-grid">
    <FormGroup label="Currency">
      <select bind:value={rateCurrency} class="form-select">
        {#each currencies as c}
          <option value={c}>{c}/BHD</option>
        {/each}
      </select>
    </FormGroup>
    <FormGroup label="Rate">
      <Input type="number" step="0.000001" bind:value={rateValue} placeholder="0.000000" />
    </FormGroup>
    <FormGroup label="Source">
      <select bind:value={rateSource} class="form-select">
        <option value="CBB">CBB (Central Bank)</option>
        <option value="MANUAL">Manual Entry</option>
        <option value="API">API Feed</option>
      </select>
    </FormGroup>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showRateModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleAddRate} loading={modalLoading}>
      Save Rate
    </Button>
  </svelte:fragment>
</Modal>

<style>
  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 32px;
    gap: 14px;
  }

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 10px;
    margin-bottom: 12px;
  }

  .kpi {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .kpi-label {
    font-size: 11px;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .kpi-value {
    font-size: 18px;
    font-weight: 600;
    font-family: var(--font-mono);
  }

  .kpi-value.primary { color: var(--brand-indigo); }
  .kpi-value.success { color: #10B981; }
  .kpi-value.danger-text { color: #EF4444; }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
    padding: 8px;
    background: var(--surface-elevated);
    border-radius: var(--radius-md);
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar .left,
  .toolbar .right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .tabs {
    display: flex;
    gap: 4px;
  }

  .tabs button {
    padding: 7px 10px;
    border: none;
    background: transparent;
    font-size: 12.5px;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: all 0.2s;
  }

  .tabs button:hover {
    background: var(--surface-default);
  }

  .tabs button.active {
    background: var(--brand-indigo);
    color: white;
  }

  .account-select,
  .form-select {
    padding: 7px 10px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    font-size: 13px;
    min-width: 200px;
  }

  .reval-actions {
    margin-bottom: 8px;
  }

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  @media (max-width: 900px) {
    .kpi-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
