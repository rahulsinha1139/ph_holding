<script lang="ts">
  /**
   * AuditTrailViewer - Bank Reconciliation Audit Trail
   *
   * Features:
   * - Complete audit history for all reconciliation actions
   * - Filter by statement, action type, date range
   * - Action reversal capability
   * - Export audit report
   * - Automatic vs Manual action distinction
   *
   * Design System: Wabi-Sabi minimalism x Bloomberg data density
   */

  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports
  import {
    GetBankStatements,
    GetActiveBankAccounts,
    GetAuditTrail,
    GetAuditTrailByDateRange,
    ReverseAction,
  } from '../../../wailsjs/go/main/App';

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
  import { escapeHtml } from '$lib/utils/escapeHtml';

  // Props
  export let embedded = false;
  export let statementId = ''; // Pre-filter by statement

  // Types
  interface BankAccount {
    id: string;
    bank_name: string;
    account_number: string;
  }

  interface BankStatement {
    id: string;
    statement_number: string;
    period_end: any;
  }

  // AuditLogEntry - compatible with BankReconciliationAuditLog from Wails
  interface AuditLogEntry {
    id: string;
    created_at?: any;
    updated_at?: any;
    version?: number;
    created_by?: string;
    bank_statement_id: string;
    bank_statement_line_id?: string;
    action: string;
    action_detail: string;
    performed_by: string;
    performed_at: any;
    is_automatic: boolean;
    confidence_score: number;
    reason: string;
    is_reversed: boolean;
    reversed_by: string;
    reversed_at?: any;
    reversal_reason: string;
  }

  type ActionFilter = 'all' | 'IMPORT' | 'MATCH' | 'UNMATCH' | 'SPLIT' | 'CATEGORIZE' | 'RECONCILE' | 'VERIFY';

  // State
  let bankAccounts: BankAccount[] = [];
  let statements: BankStatement[] = [];
  let auditLogs: AuditLogEntry[] = [];
  let filteredLogs: AuditLogEntry[] = [];
  let selectedAccountId = '';
  let selectedStatementId = '';
  let actionFilter: ActionFilter = 'all';
  let showReversed = false;
  let loading = true;

  // Modal state
  let showReverseModal = false;
  let reverseLoading = false;
  let selectedLog: AuditLogEntry | null = null;
  let reverseReason = '';

  // Stats
  $: totalActions = auditLogs.length;
  $: autoActions = auditLogs.filter(l => l.is_automatic).length;
  $: manualActions = auditLogs.filter(l => !l.is_automatic).length;
  $: reversedActions = auditLogs.filter(l => l.is_reversed).length;

  // Action types for filter
  const actionTypes: ActionFilter[] = ['all', 'IMPORT', 'MATCH', 'UNMATCH', 'SPLIT', 'CATEGORIZE', 'RECONCILE', 'VERIFY'];

  // DataTable columns
  const columns = [
    {
      key: 'performed_at',
      label: 'Timestamp',
      sortable: true,
      width: '150px',
      render: (row: AuditLogEntry) => `<span style="font-family: var(--font-mono); font-size: 11px;">${formatDateTime(row.performed_at)}</span>`
    },
    {
      key: 'action',
      label: 'Action',
      sortable: true,
      width: '110px',
      render: (row: AuditLogEntry) => {
        const actionColors: Record<string, string> = {
          'IMPORT': '#3B82F6',
          'MATCH': '#10B981',
          'UNMATCH': '#F59E0B',
          'SPLIT': '#8B5CF6',
          'CATEGORIZE': '#6366F1',
          'RECONCILE': '#059669',
          'VERIFY': '#0D9488'
        };
        const color = actionColors[row.action] || '#6B7280';
        return `<span style="color: ${color}; font-weight: 600; font-size: 12px;">${row.action}</span>`;
      }
    },
    {
      key: 'is_automatic',
      label: 'Type',
      width: '80px',
      render: (row: AuditLogEntry) => {
        if (row.is_automatic) {
          const confidence = Math.round(row.confidence_score * 100);
          return `<span style="color: #8B5CF6; font-size: 11px;">Auto (${confidence}%)</span>`;
        }
        return `<span style="color: #6B7280; font-size: 11px;">Manual</span>`;
      }
    },
    {
      key: 'performed_by',
      label: 'User',
      sortable: true,
      width: '100px',
      render: (row: AuditLogEntry) => `<span style="font-size: 12px;">${row.performed_by || 'System'}</span>`
    },
    {
      key: 'reason',
      label: 'Details',
      render: (row: AuditLogEntry) => {
        let text = row.reason || '';
        if (row.action_detail) {
          try {
            const detail = JSON.parse(row.action_detail);
            if (detail.invoice_id) text = `Invoice: ${detail.invoice_id}`;
            else if (detail.matched_count) text = `Matched: ${detail.matched_count}`;
            else if (detail.reason) text = detail.reason;
          } catch {
            text = row.action_detail.substring(0, 50);
          }
        }
        const maxLen = 60;
        const display = text.length > maxLen ? text.substring(0, maxLen) + '...' : text;
        return `<span style="font-size: 12px; color: var(--text-secondary);" title="${escapeHtml(text)}">${escapeHtml(display) || '-'}</span>`;
      }
    },
    {
      key: 'is_reversed',
      label: 'Status',
      width: '90px',
      render: (row: AuditLogEntry) => {
        if (row.is_reversed) {
          return `<span style="color: #EF4444; font-size: 11px; text-decoration: line-through;">Reversed</span>`;
        }
        return `<span style="color: #10B981; font-size: 11px;">Active</span>`;
      }
    }
  ];

  // Formatters
  function formatDateTime(dateValue: any): string {
    if (!dateValue) return '-';
    try {
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      return date.toLocaleString('en-GB', {
        day: '2-digit',
        month: 'short',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    } catch {
      return '-';
    }
  }

  // escapeHtml now imported from centralized utility

  // Filtering
  $: {
    let logs = auditLogs;

    if (actionFilter !== 'all') {
      logs = logs.filter(l => l.action === actionFilter);
    }

    if (!showReversed) {
      logs = logs.filter(l => !l.is_reversed);
    }

    filteredLogs = logs;
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

      await loadStatements();

      // If pre-filtered, load that statement's audit trail
      if (statementId) {
        selectedStatementId = statementId;
        await loadAuditTrail();
      }
    } catch (err) {
      console.error('Failed to load data:', err);
      toast.danger('Failed to load audit data');
    } finally {
      loading = false;
    }
  }

  async function loadStatements() {
    if (!selectedAccountId) return;
    try {
      const result = await GetBankStatements(selectedAccountId);
      statements = result || [];
      if (statements.length > 0 && !selectedStatementId) {
        selectedStatementId = statements[0].id;
        await loadAuditTrail();
      }
    } catch (err) {
      console.error('Failed to load statements:', err);
    }
  }

  async function loadAuditTrail() {
    if (!selectedStatementId) return;
    try {
      const result = await GetAuditTrail(selectedStatementId);
      auditLogs = result || [];
    } catch (err) {
      console.error('Failed to load audit trail:', err);
      auditLogs = [];
    }
  }

  // Actions
  function openReverseModal(log: AuditLogEntry) {
    if (log.is_reversed) {
      toast.warning('This action has already been reversed');
      return;
    }
    selectedLog = log;
    reverseReason = '';
    showReverseModal = true;
  }

  async function handleReverse() {
    if (!selectedLog || !reverseReason) {
      toast.warning('Please provide a reason for reversal');
      return;
    }

    reverseLoading = true;
    try {
      await ReverseAction(selectedLog.id, 'admin', reverseReason);
      toast.success('Action reversed');
      showReverseModal = false;
      await loadAuditTrail();
    } catch (err: any) {
      console.error('Failed to reverse:', err);
      toast.danger(err?.message || 'Failed to reverse action');
    } finally {
      reverseLoading = false;
    }
  }

  async function handleAccountChange() {
    selectedStatementId = '';
    auditLogs = [];
    await loadStatements();
  }

  onMount(() => {
    loadData();
  });
</script>

<PageLayout title="Audit Trail" subtitle="Bank reconciliation action history" {embedded}>
  {#if loading}
    <div class="loading-container">
      <WabiSpinner size="lg" />
      <p>Loading audit trail...</p>
    </div>
  {:else}
    <!-- KPIs -->
    <div class="kpi-grid">
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Total Actions</span>
          <span class="kpi-value">{totalActions}</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Automatic</span>
          <span class="kpi-value auto">{autoActions}</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Manual</span>
          <span class="kpi-value manual">{manualActions}</span>
        </div>
      </Card>
      <Card variant="elevated">
        <div class="kpi">
          <span class="kpi-label">Reversed</span>
          <span class="kpi-value reversed">{reversedActions}</span>
        </div>
      </Card>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="left">
        <select bind:value={selectedAccountId} on:change={handleAccountChange} class="select">
          {#each bankAccounts as account}
            <option value={account.id}>{account.bank_name} - {account.account_number}</option>
          {/each}
        </select>
        <select bind:value={selectedStatementId} on:change={loadAuditTrail} class="select">
          {#each statements as stmt}
            <option value={stmt.id}>{stmt.statement_number}</option>
          {/each}
        </select>
      </div>
      <div class="right">
        <select bind:value={actionFilter} class="select filter">
          {#each actionTypes as type}
            <option value={type}>{type === 'all' ? 'All Actions' : type}</option>
          {/each}
        </select>
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={showReversed} />
          Show Reversed
        </label>
      </div>
    </div>

    <!-- Audit Log Table -->
    <DataTable
      data={filteredLogs}
      columns={columns}
      onRowClick={openReverseModal}
      emptyMessage="No audit entries found"
      rowClass={(row) => row.is_reversed ? 'reversed-row' : ''}
    />
  {/if}
</PageLayout>

<!-- Reverse Modal -->
<Modal bind:open={showReverseModal} title="Reverse Action" size="md">
  {#if selectedLog}
    <div class="reverse-details">
      <div class="detail-row">
        <span class="label">Action:</span>
        <span class="value">{selectedLog.action}</span>
      </div>
      <div class="detail-row">
        <span class="label">Performed:</span>
        <span class="value">{formatDateTime(selectedLog.performed_at)}</span>
      </div>
      <div class="detail-row">
        <span class="label">By:</span>
        <span class="value">{selectedLog.performed_by || 'System'}</span>
      </div>
      {#if selectedLog.reason}
        <div class="detail-row">
          <span class="label">Original Reason:</span>
          <span class="value">{selectedLog.reason}</span>
        </div>
      {/if}
    </div>
    <FormGroup label="Reversal Reason (required)">
      <textarea bind:value={reverseReason} rows="3" class="form-textarea" placeholder="Explain why this action is being reversed..."></textarea>
    </FormGroup>
    <div class="warning-box">
      Reversing this action may affect related reconciliation data. This action is logged for audit purposes.
    </div>
  {/if}
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showReverseModal = false}>Cancel</Button>
    <Button variant="warning" on:click={handleReverse} loading={reverseLoading} disabled={!reverseReason}>
      Reverse Action
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
    font-family: var(--font-mono);
    color: var(--text-primary);
  }

  .kpi-value.auto { color: #8B5CF6; }
  .kpi-value.manual { color: #6B7280; }
  .kpi-value.reversed { color: #EF4444; }

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

  .select {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    font-size: 14px;
    min-width: 180px;
  }

  .select.filter {
    min-width: 140px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: 13px;
    color: var(--text-secondary);
    cursor: pointer;
  }

  .checkbox-label input {
    cursor: pointer;
  }

  .reverse-details {
    padding: var(--space-3);
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
    margin-bottom: var(--space-4);
  }

  .detail-row {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-1) 0;
  }

  .detail-row .label {
    color: var(--text-secondary);
    min-width: 120px;
    font-size: 13px;
  }

  .detail-row .value {
    color: var(--text-primary);
    font-size: 13px;
  }

  .form-textarea {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    font-size: 14px;
    font-family: inherit;
    resize: vertical;
  }

  .warning-box {
    padding: var(--space-3);
    background: #FEF3C7;
    border: 1px solid #F59E0B;
    border-radius: var(--radius-sm);
    color: #92400E;
    font-size: 12px;
    margin-top: var(--space-3);
  }

  :global(.reversed-row) {
    opacity: 0.6;
    background: rgba(239, 68, 68, 0.05) !important;
  }

  @media (max-width: 900px) {
    .kpi-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
