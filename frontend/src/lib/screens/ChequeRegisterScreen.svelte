<script lang="ts">
  /**
   * ChequeRegisterScreen - Cheque Lifecycle Management
   *
   * Features:
   * - Cheque book registration
   * - Cheque issuance workflow
   * - Status tracking (Issued -> Presented -> Cleared/Stale/Bounced)
   * - Outstanding cheques report
   * - Stale cheque identification (>6 months)
   *
   * Design System: Wabi-Sabi minimalism x Bloomberg data density
   */

  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports
  import {
    GetActiveBankAccounts,
    GetChequeRegisters,
    CreateChequeRegister,
    GetOutstandingCheques,
    GetStaleCheques,
    IssueCheque,
    MarkChequeCleared,
    MarkChequeStale,
    CancelCheque,
    GetNextChequeNumber,
  } from '../../../wailsjs/go/main/App';

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
  import { escapeHtml } from '$lib/utils/escapeHtml';

  // Props
  export let embedded = false;

  // Types
  interface BankAccount {
    id: string;
    bank_name: string;
    account_number: string;
    currency: string;
  }

  interface ChequeRegister {
    id: string;
    bank_account_id: string;
    cheque_book_no: string;
    start_number: number;
    end_number: number;
    current_number: number;
    status: string;
    issued_date: any;
  }

  interface OutstandingCheque {
    id: string;
    bank_account_id: string;
    cheque_number: string;
    amount: number;
    currency: string;
    issued_date: any;
    payee_name: string;
    payee_type: string;
    purpose: string;
    status: string;
    is_stale: boolean;
  }

  type ViewTab = 'outstanding' | 'registers' | 'stale';

  // State
  let bankAccounts: BankAccount[] = [];
  let registers: ChequeRegister[] = [];
  let cheques: OutstandingCheque[] = [];
  let staleCheques: OutstandingCheque[] = [];
  let selectedAccountId = '';
  let activeTab: ViewTab = 'outstanding';
  let loading = true;
  let totalOutstanding = 0;

  // Modal state
  let showRegisterModal = false;
  let showIssueModal = false;
  let registerLoading = false;
  let issueLoading = false;

  // Register form
  let newBookNo = '';
  let newStartNumber = '';
  let newEndNumber = '';

  // Issue form
  let issueAmount = '';
  let issuePayee = '';
  let issuePayeeType = 'SUPPLIER';
  let issuePurpose = '';
  let nextChequeNumber = '';

  // DataTable columns for cheques
  const chequeColumns = [
    {
      key: 'cheque_number',
      label: 'Cheque #',
      sortable: true,
      width: '100px',
      render: (row: OutstandingCheque) => `<span style="font-family: var(--font-mono); font-weight: 600;">${escapeHtml(row.cheque_number)}</span>`
    },
    {
      key: 'issued_date',
      label: 'Issued',
      sortable: true,
      width: '100px',
      render: (row: OutstandingCheque) => `<span style="font-size: 12px;">${formatDate(row.issued_date)}</span>`
    },
    {
      key: 'payee_name',
      label: 'Payee',
      sortable: true,
      render: (row: OutstandingCheque) => `<span style="font-weight: 500;">${escapeHtml(row.payee_name)}</span>`
    },
    {
      key: 'amount',
      label: 'Amount',
      sortable: true,
      width: '130px',
      align: 'right' as const,
      render: (row: OutstandingCheque) => `<span style="font-family: var(--font-mono); font-weight: 600;">${formatBHD(row.amount)} ${row.currency}</span>`
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      width: '110px',
      render: (row: OutstandingCheque) => {
        const statusColors: Record<string, string> = {
          'ISSUED': '#3B82F6',
          'PRESENTED': '#F59E0B',
          'CLEARED': '#10B981',
          'STALE': '#EF4444',
          'CANCELLED': '#6B7280',
          'BOUNCED': '#DC2626'
        };
        const color = statusColors[row.status] || '#6B7280';
        return `<span style="color: ${color}; font-weight: 500; font-size: 12px;">${row.status}</span>`;
      }
    },
    {
      key: 'is_stale',
      label: 'Age',
      width: '80px',
      render: (row: OutstandingCheque) => {
        if (row.is_stale) {
          return `<span style="color: #EF4444; font-size: 11px;">STALE</span>`;
        }
        const days = getDaysOld(row.issued_date);
        return `<span style="color: var(--text-secondary); font-size: 11px;">${days}d</span>`;
      }
    }
  ];

  // Register columns
  const registerColumns = [
    {
      key: 'cheque_book_no',
      label: 'Book #',
      sortable: true,
      width: '120px',
      render: (row: ChequeRegister) => `<span style="font-family: var(--font-mono); font-weight: 600;">${escapeHtml(String(row.cheque_book_no))}</span>`
    },
    {
      key: 'start_number',
      label: 'Range',
      width: '140px',
      render: (row: ChequeRegister) => `<span style="font-family: var(--font-mono); font-size: 12px;">${escapeHtml(String(row.start_number))} - ${escapeHtml(String(row.end_number))}</span>`
    },
    {
      key: 'current_number',
      label: 'Next',
      width: '100px',
      render: (row: ChequeRegister) => `<span style="font-family: var(--font-mono); color: var(--brand-indigo);">${escapeHtml(String(row.current_number))}</span>`
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      width: '110px',
      render: (row: ChequeRegister) => {
        const statusColors: Record<string, string> = {
          'ACTIVE': '#10B981',
          'EXHAUSTED': '#F59E0B',
          'CANCELLED': '#6B7280'
        };
        const color = statusColors[row.status] || '#6B7280';
        return `<span style="color: ${color}; font-weight: 500; font-size: 12px;">${row.status}</span>`;
      }
    },
    {
      key: 'usage',
      label: 'Used',
      width: '100px',
      render: (row: ChequeRegister) => {
        const total = row.end_number - row.start_number + 1;
        const used = row.current_number - row.start_number;
        const pct = Math.round((used / total) * 100);
        return `<span style="font-size: 12px;">${used}/${total} (${pct}%)</span>`;
      }
    }
  ];

  // Formatters
  function formatBHD(value: number): string {
    return value?.toFixed(3) || '0.000';
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

  function getDaysOld(dateValue: any): number {
    if (!dateValue) return 0;
    try {
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      const diff = Date.now() - date.getTime();
      return Math.floor(diff / (1000 * 60 * 60 * 24));
    } catch {
      return 0;
    }
  }

  // Data loading
  async function loadData() {
    loading = true;
    try {
      const accounts = await GetActiveBankAccounts();
      bankAccounts = accounts || [];

      if (bankAccounts.length > 0 && !selectedAccountId) {
        selectedAccountId = bankAccounts[0].id;
      }

      await loadAccountData();
    } catch (err) {
      console.error('Failed to load data:', err);
      toast.danger('Failed to load cheque data');
    } finally {
      loading = false;
    }
  }

  async function loadAccountData() {
    if (!selectedAccountId) return;

    try {
      const [registersResult, chequesResult, staleResult] = await Promise.all([
        GetChequeRegisters(selectedAccountId),
        GetOutstandingCheques(selectedAccountId),
        GetStaleCheques(selectedAccountId)
      ]);

      registers = registersResult || [];
      cheques = chequesResult || [];
      // Calculate total from outstanding cheques array
      totalOutstanding = (chequesResult || []).reduce((sum: number, c: any) => sum + (c.amount || 0), 0);
      staleCheques = staleResult || [];

      // Get next cheque number
      try {
        nextChequeNumber = await GetNextChequeNumber(selectedAccountId);
      } catch {
        nextChequeNumber = 'N/A';
      }
    } catch (err) {
      console.error('Failed to load account data:', err);
    }
  }

  // Actions
  async function handleCreateRegister() {
    if (!newBookNo || !newStartNumber || !newEndNumber) {
      toast.warning('Please fill all fields');
      return;
    }

    registerLoading = true;
    try {
      await CreateChequeRegister(
        selectedAccountId,
        newBookNo,
        parseInt(newStartNumber),
        parseInt(newEndNumber)
      );
      toast.success('Cheque book registered');
      showRegisterModal = false;
      newBookNo = '';
      newStartNumber = '';
      newEndNumber = '';
      await loadAccountData();
    } catch (err: any) {
      console.error('Failed to create register:', err);
      toast.danger(err?.message || 'Failed to register cheque book');
    } finally {
      registerLoading = false;
    }
  }

  async function handleIssueCheque() {
    if (!issueAmount || !issuePayee || !issuePurpose) {
      toast.warning('Please fill all fields');
      return;
    }

    issueLoading = true;
    try {
      await IssueCheque(
        selectedAccountId,
        parseFloat(issueAmount),
        issuePayee,
        issuePayeeType,
        null,
        issuePurpose
      );
      toast.success('Cheque issued successfully');
      showIssueModal = false;
      issueAmount = '';
      issuePayee = '';
      issuePurpose = '';
      await loadAccountData();
    } catch (err: any) {
      console.error('Failed to issue cheque:', err);
      toast.danger(err?.message || 'Failed to issue cheque');
    } finally {
      issueLoading = false;
    }
  }

  async function handleMarkStale(cheque: OutstandingCheque) {
    try {
      await MarkChequeStale(cheque.cheque_number);
      toast.success(`Cheque #${cheque.cheque_number} marked as stale`);
      await loadAccountData();
    } catch (err) {
      console.error('Failed to mark stale:', err);
      toast.danger('Failed to mark cheque as stale');
    }
  }

  async function handleCancel(cheque: OutstandingCheque) {
    try {
      await CancelCheque(cheque.cheque_number, 'Cancelled by user');
      toast.success(`Cheque #${cheque.cheque_number} cancelled`);
      await loadAccountData();
    } catch (err) {
      console.error('Failed to cancel:', err);
      toast.danger('Failed to cancel cheque');
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<PageLayout title="Cheque Register" subtitle="Manage cheque books and issuance" {embedded}>
  {#if loading}
    <div class="loading-container">
      <WabiSpinner size="lg" />
      <p>Loading cheque data...</p>
    </div>
  {:else}
    <!-- KPIs -->
    <div class="kpi-grid">
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Outstanding Total</span>
          <span class="kpi-value primary">{formatBHD(totalOutstanding)} BHD</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Outstanding Cheques</span>
          <span class="kpi-value">{cheques.length}</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Stale Cheques</span>
          <span class="kpi-value warning">{staleCheques.length}</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Next Cheque #</span>
          <span class="kpi-value mono">{nextChequeNumber}</span>
        </div>
      </Card>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="left">
        <select bind:value={selectedAccountId} on:change={loadAccountData} class="account-select">
          {#each bankAccounts as account}
            <option value={account.id}>{account.bank_name} - {account.account_number}</option>
          {/each}
        </select>
        <div class="tabs">
          <button class:active={activeTab === 'outstanding'} on:click={() => activeTab = 'outstanding'}>
            Outstanding ({cheques.length})
          </button>
          <button class:active={activeTab === 'registers'} on:click={() => activeTab = 'registers'}>
            Registers ({registers.length})
          </button>
          <button class:active={activeTab === 'stale'} on:click={() => activeTab = 'stale'}>
            Stale ({staleCheques.length})
          </button>
        </div>
      </div>
      <div class="right">
        <Button variant="secondary" on:click={() => showRegisterModal = true}>
          New Cheque Book
        </Button>
        <Button variant="primary" on:click={() => showIssueModal = true} disabled={nextChequeNumber === 'N/A'}>
          Issue Cheque
        </Button>
      </div>
    </div>

    <!-- Content based on tab -->
    {#if activeTab === 'outstanding'}
      <DataTable
        data={cheques}
        columns={chequeColumns}
        emptyMessage="No outstanding cheques"
      />
    {:else if activeTab === 'registers'}
      <DataTable
        data={registers}
        columns={registerColumns}
        emptyMessage="No cheque books registered"
      />
    {:else if activeTab === 'stale'}
      <DataTable
        data={staleCheques}
        columns={chequeColumns}
        emptyMessage="No stale cheques"
      />
    {/if}
  {/if}
</PageLayout>

<!-- Register Modal -->
<Modal bind:open={showRegisterModal} title="Register Cheque Book" size="md">
  <div class="form-grid">
    <FormGroup label="Cheque Book Number">
      <Input bind:value={newBookNo} placeholder="e.g., CB-2026-001" />
    </FormGroup>
    <FormGroup label="Start Number">
      <Input type="number" bind:value={newStartNumber} placeholder="e.g., 100001" />
    </FormGroup>
    <FormGroup label="End Number">
      <Input type="number" bind:value={newEndNumber} placeholder="e.g., 100050" />
    </FormGroup>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showRegisterModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleCreateRegister} loading={registerLoading}>
      Register
    </Button>
  </svelte:fragment>
</Modal>

<!-- Issue Modal -->
<Modal bind:open={showIssueModal} title="Issue Cheque" size="md">
  <div class="form-grid">
    <div class="cheque-preview">
      <span class="label">Next Cheque Number</span>
      <span class="number">{nextChequeNumber}</span>
    </div>
    <FormGroup label="Amount (BHD)">
      <Input type="number" step="0.001" bind:value={issueAmount} placeholder="0.000" />
    </FormGroup>
    <FormGroup label="Payee Name">
      <Input bind:value={issuePayee} placeholder="Enter payee name" />
    </FormGroup>
    <FormGroup label="Payee Type">
      <select bind:value={issuePayeeType} class="form-select">
        <option value="SUPPLIER">Supplier</option>
        <option value="EMPLOYEE">Employee</option>
        <option value="OTHER">Other</option>
      </select>
    </FormGroup>
    <FormGroup label="Purpose">
      <Input bind:value={issuePurpose} placeholder="Enter purpose" />
    </FormGroup>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showIssueModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleIssueCheque} loading={issueLoading}>
      Issue Cheque
    </Button>
  </svelte:fragment>
</Modal>

<style>
  .embedded-container {
    padding: var(--space-4);
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-8);
    gap: var(--space-4);
  }

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-4);
    margin-bottom: var(--space-6);
  }

  .kpi {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .kpi-label {
    font-size: 12px;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .kpi-value {
    font-size: 24px;
    font-weight: 600;
  }

  .kpi-value.primary { color: var(--brand-indigo); }
  .kpi-value.warning { color: #F59E0B; }
  .kpi-value.mono { font-family: var(--font-mono); }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-4);
    padding: var(--space-3);
    background: var(--surface-elevated);
    border-radius: var(--radius-md);
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .toolbar .left,
  .toolbar .right {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .account-select,
  .form-select {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    font-size: 14px;
    min-width: 200px;
  }

  .tabs {
    display: flex;
    gap: var(--space-1);
  }

  .tabs button {
    padding: var(--space-2) var(--space-3);
    border: none;
    background: transparent;
    font-size: 13px;
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

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .cheque-preview {
    padding: var(--space-3);
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
    text-align: center;
  }

  .cheque-preview .label {
    display: block;
    font-size: 11px;
    color: var(--text-secondary);
    margin-bottom: var(--space-1);
  }

  .cheque-preview .number {
    font-family: var(--font-mono);
    font-size: 24px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  @media (max-width: 900px) {
    .kpi-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
