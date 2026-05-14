<script lang="ts">
  /**
   * BankReconciliationScreen - Bank Statement Import & Reconciliation
   *
   * Features:
   * - Import bank statements (PDF/CSV)
   * - Auto-match transactions to invoices/payments
   * - Manual matching for unmatched items
   * - Reconciliation workflow with verification
   * - Real-time cash position display
   * - BHD currency formatting (3 decimal places)
   *
   * Design System: Wabi-Sabi minimalism x Bloomberg data density
   */

  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';

  // Wails API imports - will be available after binding regeneration
  import {
    GetBankStatements,
    GetBankStatementLines,
    GetActiveBankAccounts,
    GetAllBankAccounts,
    CreateBankAccount,
    UpdateBankAccount,
    DeleteBankAccount,
    GetCashPosition,
    ImportBankStatementCSV,
    ImportBankStatementWithDialog,
    AutoMatchBankLines,
    ManualMatchLine,
    UnmatchLine,
    FinalizeReconciliation,
    ReopenReconciliation,
    GetReconciliationStats,
    GetReconciliationSummary,
    GetAuditTrail,
    DeleteBankStatement,
    UpdateBankStatement,
    CreateBankStatementLine,
    UpdateBankStatementLine,
    DeleteBankStatementLine,
    ListCustomerInvoices,
    GetAllPayments,
    ListCustomerReceipts,
    GetSupplierInvoices,
    GetAllSupplierPayments,
    ListExpenseEntries,
    ListUnreconciledPayrollPayouts,
    CreateSplitAllocation,
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
  export let company: 'PH Trading' | 'AHS Trading' = 'PH Trading';

  // Types - aligned with Wails CompanyBankAccount (balance is calculated separately via CashPosition)
  interface BankAccount {
    id: string;
    division?: string;
    bank_name: string;
    account_name?: string;
    account_number: string;
    iban?: string;
    swift_bic?: string;
    currency: string;
    display_order?: number;
    is_active: boolean;
    created_at?: any;
    updated_at?: any;
  }

  interface BankStatement {
    id: string;
    division?: string;
    bank_account_id: string;
    statement_number: string;
    period_start: any;
    period_end: any;
    opening_balance: number;
    closing_balance: number;
    total_debits: number;
    total_credits: number;
    status: string;
    notes?: string;
  }

  interface BankStatementLine {
    id: string;
    bank_statement_id: string;
    line_number: number;
    transaction_date: any;
    description: string;
    reference: string;
    debit: number;
    credit: number;
    balance: number;
    transaction_type: string;
    is_matched: boolean;
    match_type: string;
    match_confidence: number;
    extracted_customer: string;
    extracted_invoices: string;
  }

  interface CashPosition {
    total_bhd: number;
    by_account: Record<string, number>;
    as_of: any;
  }

  interface ReconciliationStats {
    total_statements: number;
    reconciled_statements: number;
    in_progress_statements: number;
    imported_statements: number;
    latest_statement_date: any;
  }

  interface ReconciliationSummary {
    status: string;
    matched_count: number;
    unmatched_count: number;
    matched_percent: number;
    matched_amount: number;
    total_lines: number;
    balance_verified: boolean;
  }

  // State - using any types for Wails-generated compatibility
  let bankAccounts: any[] = [];
  let allBankAccounts: any[] = []; // All accounts including inactive for management
  let statements: any[] = [];
  let selectedAccountId = '';
  let selectedStatement: any | null = null;
  let statementLines: any[] = [];
  let cashPosition: any = null;
  let reconciliationStats: ReconciliationStats | null = null;
  let reconciliationSummary: ReconciliationSummary | null = null;
  let loading = true;
  let statementsLoading = false;
  let linesLoading = false;
  let lastLoadedCompany = company;

  // Modal state
  let showImportModal = false;
  let showMatchModal = false;
  let showManageAccountsModal = false;
  let showEditStatementModal = false;
  let showEditLineModal = false;
  let showAddLineModal = false;
  let importLoading = false;
  let matchingLine: BankStatementLine | null = null;
  let matchingType = 'CUSTOMER_INVOICE';
  let matchingCandidateId = '';
  let matchingSearch = '';
  let matchAllocations: MatchAllocationDraft[] = [];
  let matchLoading = false;
  let customerInvoiceCandidates: any[] = [];
  let customerPaymentCandidates: any[] = [];
  let customerReceiptCandidates: any[] = [];
  let supplierInvoiceCandidates: any[] = [];
  let supplierPaymentCandidates: any[] = [];
  let expenseCandidates: any[] = [];
  let payrollPayoutCandidates: any[] = [];

  type MatchAllocationDraft = {
    key: string;
    allocation_type: string;
    entity_id: string;
    label: string;
    allocated_amount: number;
    max_amount: number;
  };

  // Bank account management state
  let editingAccount: BankAccount | null = null;
  let accountFormData = {
    division: 'PH Trading',
    bank_name: '',
    account_name: '',
    account_number: '',
    iban: '',
    swift_bic: '',
    currency: 'BHD',
    booking_rate: 0
  };
  let savingAccount = false;

  // Import form
  let importAccountId = '';

  // Statement edit form
  let editStatementData = {
    opening_balance: 0,
    closing_balance: 0,
    period_start: '',
    period_end: '',
    status: 'Imported',
    notes: ''
  };
  let savingStatement = false;

  // Transaction line edit/add form
  let lineFormData = {
    transaction_date: '',
    description: '',
    reference: '',
    debit: 0,
    credit: 0
  };
  let savingLine = false;
  let editingLine: BankStatementLine | null = null;

  function matchesCompany(division?: string) {
    return (division || 'PH Trading') === company;
  }

  $: visibleBankAccounts = bankAccounts.filter((account) => matchesCompany(account.division));
  $: visibleAllBankAccounts = allBankAccounts.filter((account) => matchesCompany(account.division));
  $: visibleCashAccounts = (cashPosition?.accounts || []).filter((account: any) => matchesCompany(account.division));
  $: totalVisibleCashPosition = visibleCashAccounts.reduce((sum: number, account: any) => sum + getAccountBalanceBHD(account), 0);
  $: cashPositionNotices = visibleCashAccounts.map((account: any) => account.notice).filter(Boolean);

  // KPI calculations
  $: totalUnmatched = reconciliationSummary?.unmatched_count ?? statementLines.filter(l => !l.is_matched).length;
  $: openStatements = (reconciliationStats?.imported_statements || 0) + (reconciliationStats?.in_progress_statements || 0);
  $: totalMatched = statementLines.filter(l => l.is_matched).length;
  $: unmatchedCredit = statementLines.filter(l => !l.is_matched && l.credit > 0).reduce((sum, l) => sum + l.credit, 0);
  $: matchLineAmount = matchingLine ? formatMatchAmount(matchingLine) : 0;
  $: matchAllocationTotal = roundAmount(matchAllocations.reduce((sum, allocation) => sum + (Number(allocation.allocated_amount) || 0), 0));
  $: matchAllocationRemaining = roundAmount(matchLineAmount - matchAllocationTotal);

  // DataTable columns for statements
  const statementColumns = [
    {
      key: 'statement_number',
      label: 'Statement #',
      sortable: true,
      width: '280px',
      render: (row: BankStatement) => `<span style="display:block; font-family: var(--font-mono); font-size: 12px; line-height: 1.35; white-space: normal; overflow-wrap: anywhere;">${escapeHtml(row.statement_number)}</span>`
    },
    {
      key: 'period_end',
      label: 'Period',
      sortable: true,
      width: '180px',
      render: (row: BankStatement) => `<span style="font-size: 13px;">${formatDate(row.period_start)} - ${formatDate(row.period_end)}</span>`
    },
    {
      key: 'closing_balance',
      label: 'Closing Balance',
      sortable: true,
      width: '140px',
      align: 'right' as const,
      render: (row: BankStatement) => `<span style="font-family: var(--font-mono); font-weight: 600;">${formatBHD(row.closing_balance)}</span>`
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      width: '120px',
      render: (row: BankStatement) => {
        const statusColors: Record<string, string> = {
          'Imported': '#F59E0B',
          'InProgress': '#3B82F6',
          'Reconciled': '#10B981',
          'Verified': '#059669'
        };
        const color = statusColors[row.status] || '#6B7280';
        return `<span style="color: ${color}; font-weight: 500; font-size: 12px;">${escapeHtml(formatStatementStatus(row.status))}</span>`;
      }
    }
  ];

  // DataTable columns for statement lines
  const lineColumns = [
    {
      key: 'transaction_date',
      label: 'Date',
      sortable: true,
      width: '100px',
      render: (row: BankStatementLine) => `<span style="font-family: var(--font-mono); font-size: 12px;">${formatDate(row.transaction_date)}</span>`
    },
    {
      key: 'description',
      label: 'Description',
      sortable: true,
      render: (row: BankStatementLine) => {
        const maxLen = 50;
        const desc = row.description.length > maxLen ? row.description.substring(0, maxLen) + '...' : row.description;
        return `<span style="font-size: 13px;" title="${escapeHtml(row.description)}">${escapeHtml(desc)}</span>`;
      }
    },
    {
      key: 'reference',
      label: 'Reference',
      sortable: true,
      width: '120px',
      render: (row: BankStatementLine) => `<span style="font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary);">${escapeHtml(row.reference || '-')}</span>`
    },
    {
      key: 'debit',
      label: 'Debit',
      sortable: true,
      width: '110px',
      align: 'right' as const,
      render: (row: BankStatementLine) => row.debit > 0 ? `<span style="font-family: var(--font-mono); color: #EF4444; font-size: 13px;">${formatBHD(row.debit)}</span>` : '-'
    },
    {
      key: 'credit',
      label: 'Credit',
      sortable: true,
      width: '110px',
      align: 'right' as const,
      render: (row: BankStatementLine) => row.credit > 0 ? `<span style="font-family: var(--font-mono); color: #10B981; font-size: 13px;">${formatBHD(row.credit)}</span>` : '-'
    },
    {
      key: 'transaction_type',
      label: 'Type',
      sortable: true,
      width: '130px',
      render: (row: BankStatementLine) => {
        const typeColors: Record<string, string> = {
          'CUSTOMER_PAYMENT': '#10B981',
          'SUPPLIER_PAYMENT': '#3B82F6',
          'BANK_FEE': '#F59E0B',
          'INTEREST': '#8B5CF6',
          'TRANSFER': '#6366F1',
          'UNKNOWN': '#6B7280'
        };
        const color = typeColors[row.transaction_type] || '#6B7280';
        const label = row.transaction_type?.replace(/_/g, ' ') || 'Unknown';
        return `<span style="color: ${color}; font-size: 11px; font-weight: 500;">${escapeHtml(label)}</span>`;
      }
    },
    {
      key: 'is_matched',
      label: 'Match',
      sortable: true,
      width: '100px',
      render: (row: BankStatementLine) => {
        if (row.is_matched) {
          const confidence = Math.round(row.match_confidence * 100);
          const label = row.match_type ? `${row.match_type} Match` : 'Matched';
          return `<span style="color: #10B981; font-size: 12px;">${escapeHtml(label)} (${confidence}%)</span>`;
        }
        return `<span style="color: #EF4444; font-size: 12px;">Open</span>`;
      }
    }
  ];

  // Formatters
  function formatBHD(value: number): string {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    }).format(Number(value) || 0);
  }

  function formatDate(dateValue: any): string {
    if (!dateValue) return '-';
    try {
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
    } catch {
      return '-';
    }
  }

  function formatStatementStatus(status: string): string {
    if (status === 'InProgress') return 'In Progress';
    return status || 'Unknown';
  }

  function isStatementLocked(statement: BankStatement | null | undefined): boolean {
    return Boolean(statement && ['Reconciled', 'Verified'].includes(statement.status));
  }

  function currentStatementLabel(): string {
    if (!selectedStatement) return '-';
    return formatStatementStatus(reconciliationSummary?.status || selectedStatement.status);
  }

  function formatBankAccountOption(account: any): string {
    const bank = account?.bank_name || 'Bank Account';
    const accountName = account?.account_name ? ` ${account.account_name}` : '';
    const accountNumber = account?.account_number ? ` - ${account.account_number}` : '';
    const currency = account?.currency ? ` (${account.currency})` : '';
    return `${bank}${accountName}${accountNumber}${currency}`;
  }

  function getAccountBalanceBHD(account: any): number {
    return Number(
      account?.current_balance_bhd ??
      account?.CurrentBalanceBHD ??
      account?.current_balance ??
      account?.CurrentBalance ??
      0
    ) || 0;
  }

  async function loadReconciliationStats() {
    if (!selectedAccountId) {
      reconciliationStats = null;
      return;
    }

    try {
      reconciliationStats = await GetReconciliationStats(selectedAccountId) as ReconciliationStats;
    } catch (err) {
      console.error('Failed to load reconciliation stats:', err);
      reconciliationStats = null;
    }
  }

  async function loadStatementSummary(statementID: string) {
    try {
      reconciliationSummary = await GetReconciliationSummary(statementID) as ReconciliationSummary;
    } catch (err) {
      console.error('Failed to load reconciliation summary:', err);
      reconciliationSummary = null;
    }
  }

  // escapeHtml now imported from centralized utility

  // Data loading - independent API calls to prevent one failure from blocking the other
  async function loadData() {
    loading = true;
    let accountsError = null;
    let positionError = null;

    try {
      const accountsResult = await GetActiveBankAccounts();
      bankAccounts = accountsResult || [];
      const scopedAccounts = bankAccounts.filter((account: any) => matchesCompany(account.division));
      if (scopedAccounts.length > 0 && (!selectedAccountId || !scopedAccounts.some((account: any) => account.id === selectedAccountId))) {
        selectedAccountId = scopedAccounts[0].id;
        await loadStatements();
      } else if (selectedAccountId) {
        await loadStatements();
      } else {
        statements = [];
        statementLines = [];
        selectedStatement = null;
      }
    } catch (err) {
      console.error('Failed to load bank accounts:', err);
      accountsError = err;
      toast.danger('Failed to load bank accounts');
    }

    try {
      const positionResult = await GetCashPosition();
      cashPosition = positionResult;
    } catch (err) {
      console.error('Failed to load cash position:', err);
      positionError = err;
      // Don't show toast for cash position failure - accounts are more critical
    }

    loading = false;
  }

  async function loadStatements() {
    if (!selectedAccountId) return;
    statementsLoading = true;
    try {
      const result = await GetBankStatements(selectedAccountId);
      statements = result || [];
      await loadReconciliationStats();
    } catch (err) {
      console.error('Failed to load statements:', err);
      toast.danger('Failed to load statements');
    } finally {
      statementsLoading = false;
    }
  }

  async function loadStatementLines(statement: BankStatement) {
    selectedStatement = statement;
    linesLoading = true;
    try {
      const result = await GetBankStatementLines(statement.id);
      statementLines = result || [];
      await loadStatementSummary(statement.id);
    } catch (err) {
      console.error('Failed to load statement lines:', err);
      toast.danger('Failed to load transactions');
    } finally {
      linesLoading = false;
    }
  }

  // Actions
  async function handleAutoMatch() {
    if (!selectedStatement) return;
    try {
      const result = await AutoMatchBankLines(selectedStatement.id);
      toast.success(`Auto-matched ${result?.matched_count || 0} transactions`);
      await loadStatementLines(selectedStatement);
      await loadStatements();
    } catch (err) {
      console.error('Auto-match failed:', err);
      toast.danger('Auto-match failed');
    }
  }

  async function handleFinalize() {
    if (!selectedStatement) return;
    if (totalUnmatched > 0) {
      toast.warning(`Cannot finalize with ${totalUnmatched} unmatched transactions`);
      return;
    }
    try {
      await FinalizeReconciliation(selectedStatement.id, 'admin');
      toast.success('Statement reconciled successfully');
      await loadStatements();
      const refreshed = statements.find((statement) => statement.id === selectedStatement.id);
      if (refreshed) {
        await loadStatementLines(refreshed);
      }
    } catch (err) {
      console.error('Finalize failed:', err);
      toast.danger('Finalization failed');
    }
  }

  async function handleReopen() {
    if (!selectedStatement) return;
    try {
      await ReopenReconciliation(selectedStatement.id, 'admin', 'Reopened from bank reconciliation workspace');
      toast.success('Statement reopened for reconciliation');
      await loadStatements();
      const refreshed = statements.find((statement) => statement.id === selectedStatement.id);
      if (refreshed) {
        await loadStatementLines(refreshed);
      }
    } catch (err) {
      console.error('Reopen failed:', err);
      toast.danger(`Failed to reopen statement: ${err}`);
    }
  }

  function openMatchModal(line: BankStatementLine) {
    matchingLine = line;
    matchingSearch = '';
    matchingCandidateId = '';
    matchAllocations = [];
    matchingType = line.credit > 0 ? 'CUSTOMER_INVOICE' : 'SUPPLIER_INVOICE';
    showMatchModal = true;
    void loadMatchCandidates(line);
  }

  function handleMatchingTypeChange() {
    matchingCandidateId = '';
    matchingSearch = '';
    matchAllocations = [];
  }

  async function loadMatchCandidates(line: BankStatementLine) {
    matchLoading = true;
    try {
      const [allCustomerInvoices, customerPayments, customerReceipts, allSupplierInvoices, supplierPayments, expenses] = await Promise.all([
        ListCustomerInvoices(1000, 0).catch(() => []),
        GetAllPayments(500, 0).catch(() => []),
        ListCustomerReceipts(500, 0).catch(() => []),
        GetSupplierInvoices().catch(() => []),
        GetAllSupplierPayments().catch(() => []),
        ListExpenseEntries('', false).catch(() => [])
      ]);

      customerInvoiceCandidates = (allCustomerInvoices || []).filter((invoice: any) => {
        const status = String(invoice?.status || '').toLowerCase();
        return matchesCompany(invoice?.division) && Number(invoice?.outstanding_bhd || 0) > 0 && !['cancelled', 'void', 'draft', 'proforma'].includes(status);
      });
      supplierInvoiceCandidates = (allSupplierInvoices || []).filter((invoice: any) => {
        const status = String(invoice?.status || '').toLowerCase();
        const paymentStatus = String(invoice?.payment_status || '').toLowerCase();
        return matchesCompany(invoice?.division) && paymentStatus !== 'paid' && !['cancelled', 'void', 'rejected', 'dispute'].includes(status);
      });
      supplierPaymentCandidates = (supplierPayments || []).filter((payment: any) => matchesCompany(payment?.division));
      expenseCandidates = (expenses || []).filter((expense: any) => {
        const status = String(expense?.status || '').toLowerCase();
        const paymentStatus = String(expense?.payment_status || '').toLowerCase();
        return matchesCompany(expense?.division)
          && paymentStatus !== 'paid'
          && !['cancelled', 'canceled', 'void', 'rejected'].includes(status)
          && getCandidateAmount(expense, 'EXPENSE') > 0;
      });
      customerPaymentCandidates = (customerPayments || []).filter((payment: any) => matchesCompany(payment?.division) && Number(payment?.amount_bhd || 0) > 0);
      customerReceiptCandidates = (customerReceipts || []).filter((receipt: any) => matchesCompany(receipt?.division) && String(receipt?.status || '').toLowerCase() !== 'reversed' && Number(receipt?.amount_bhd || 0) > 0);
      payrollPayoutCandidates = ((await ListUnreconciledPayrollPayouts().catch(() => [])) || []).filter((payout: any) => matchesCompany(payout?.division));

      if (line.credit > 0) {
        if (customerInvoiceCandidates.length > 0) {
          matchingType = 'CUSTOMER_INVOICE';
        } else if (customerReceiptCandidates.length > 0) {
          matchingType = 'CUSTOMER_RECEIPT';
        } else if (customerPaymentCandidates.length > 0) {
          matchingType = 'CUSTOMER_PAYMENT';
        }
      } else if (line.debit > 0) {
        if (supplierInvoiceCandidates.length > 0) {
          matchingType = 'SUPPLIER_INVOICE';
        } else if (expenseCandidates.length > 0) {
          matchingType = 'EXPENSE';
        } else if (supplierPaymentCandidates.length > 0) {
          matchingType = 'SUPPLIER_PAYMENT';
        } else if (payrollPayoutCandidates.length > 0) {
          matchingType = 'PAYROLL_PAYOUT';
        }
      }
    } finally {
      matchLoading = false;
    }
  }

  function formatMatchAmount(line: BankStatementLine) {
    return line.credit > 0 ? line.credit : line.debit;
  }

  function roundAmount(value: number) {
    return Math.round((Number(value) || 0) * 1000) / 1000;
  }

  function buildMatchCandidateLabel(candidate: any, type: string) {
    if (type === 'CUSTOMER_INVOICE') {
      return `${candidate.invoice_number || 'Invoice'} • ${candidate.customer_name || 'Customer'} • ${formatBHD(candidate.outstanding_bhd || candidate.grand_total_bhd || 0)} BHD`;
    }
    if (type === 'CUSTOMER_PAYMENT') {
      return `${candidate.reference || candidate.invoice_number || 'Payment'} • ${candidate.invoice_number || 'Invoice'} • ${formatBHD(candidate.amount_bhd || 0)} BHD`;
    }
    if (type === 'CUSTOMER_RECEIPT') {
      const unapplied = Number(candidate.unapplied_amount_bhd || 0);
      return `${candidate.receipt_number || 'Receipt'} • ${candidate.customer_name || 'Customer'} • ${formatBHD(candidate.amount_bhd || 0)} BHD${unapplied > 0 ? ` • ${formatBHD(unapplied)} unapplied` : ''}`;
    }
    if (type === 'SUPPLIER_PAYMENT') {
      return `${candidate.reference || candidate.invoice_number || 'Payment'} • ${candidate.supplier_name || 'Supplier'} • ${formatBHD(candidate.amount_bhd || 0)} BHD`;
    }
    if (type === 'PAYROLL_PAYOUT') {
      return `${candidate.run_number || 'Payroll'} • ${candidate.employee_name || 'Employee'} • ${formatBHD(candidate.amount || 0)} BHD${candidate.payment_reference ? ` • ${candidate.payment_reference}` : ''}`;
    }
    if (type === 'EXPENSE') {
      const amount = getCandidateAmount(candidate, type);
      const name = candidate.vendor_name || candidate.category_name || candidate.cost_center || 'Expense';
      return `${candidate.entry_number || 'Expense'} • ${name} • ${formatBHD(amount)} BHD`;
    }
    return `${candidate.invoice_number || 'Invoice'} • ${candidate.supplier_name || 'Supplier'} • ${formatBHD(candidate.outstanding_bhd || candidate.total_bhd || 0)} BHD`;
  }

  function getCandidateAmount(candidate: any, type: string) {
    if (type === 'CUSTOMER_INVOICE') return Number(candidate.outstanding_bhd || candidate.grand_total_bhd || 0);
    if (type === 'CUSTOMER_PAYMENT') return Number(candidate.amount_bhd || 0);
    if (type === 'CUSTOMER_RECEIPT') return Number(candidate.amount_bhd || 0);
    if (type === 'SUPPLIER_PAYMENT') return Number(candidate.amount_bhd || 0);
    if (type === 'PAYROLL_PAYOUT') return Number(candidate.amount || 0);
    if (type === 'EXPENSE') return Number(candidate.total_amount || ((Number(candidate.amount) || 0) + (Number(candidate.vat_amount) || 0)) || candidate.amount || 0);
    return Number(candidate.outstanding_bhd || candidate.total_bhd || 0);
  }

  function allocationKey(type: string, entityId: string) {
    return `${type}:${entityId}`;
  }

  function isCandidateAllocated(candidate: any) {
    return matchAllocations.some((allocation) => allocation.key === allocationKey(matchingType, candidate.id));
  }

  function addMatchAllocation(candidate: any) {
    if (!matchingLine || !candidate?.id || isCandidateAllocated(candidate)) return;
    if (matchingType === 'SUPPLIER_PAYMENT' || matchingType === 'PAYROLL_PAYOUT') {
      toast.warning('Bulk allocation is available for invoices and expenses. Match payouts one at a time.');
      return;
    }

    const candidateAmount = roundAmount(getCandidateAmount(candidate, matchingType));
    const remaining = matchAllocations.length === 0 ? formatMatchAmount(matchingLine) : matchAllocationRemaining;
    const defaultAmount = roundAmount(Math.max(0, Math.min(candidateAmount || remaining, remaining > 0 ? remaining : candidateAmount)));
    matchAllocations = [
      ...matchAllocations,
      {
        key: allocationKey(matchingType, candidate.id),
        allocation_type: matchingType,
        entity_id: candidate.id,
        label: buildMatchCandidateLabel(candidate, matchingType),
        allocated_amount: defaultAmount,
        max_amount: candidateAmount
      }
    ];
    matchingCandidateId = '';
  }

  function updateMatchAllocationAmount(key: string, event: Event) {
    const target = event.currentTarget as HTMLInputElement;
    const value = roundAmount(Number(target.value) || 0);
    matchAllocations = matchAllocations.map((allocation) =>
      allocation.key === key ? { ...allocation, allocated_amount: value } : allocation
    );
  }

  function removeMatchAllocation(key: string) {
    matchAllocations = matchAllocations.filter((allocation) => allocation.key !== key);
  }

  function normalizeCandidateText(value: any) {
    return String(value || '').toLowerCase().replace(/[^a-z0-9.]+/g, ' ').trim();
  }

  function getFilteredMatchCandidates() {
    const search = normalizeCandidateText(matchingSearch);
    const source = matchingType === 'CUSTOMER_INVOICE'
      ? customerInvoiceCandidates
      : matchingType === 'CUSTOMER_PAYMENT'
        ? customerPaymentCandidates
        : matchingType === 'CUSTOMER_RECEIPT'
          ? customerReceiptCandidates
          : matchingType === 'SUPPLIER_PAYMENT'
            ? supplierPaymentCandidates
            : matchingType === 'PAYROLL_PAYOUT'
              ? payrollPayoutCandidates
              : matchingType === 'EXPENSE'
                ? expenseCandidates
                : supplierInvoiceCandidates;

    const lineAmount = matchingLine ? formatMatchAmount(matchingLine) : 0;
    return (source || []).filter((candidate) => {
      if (!search) return true;
      const haystack = [
        candidate.invoice_number,
        candidate.receipt_number,
        candidate.po_number,
        candidate.customer_name,
        candidate.supplier_name,
        candidate.reference,
        candidate.employee_name,
        candidate.run_number,
        candidate.payment_reference,
        candidate.order_number,
        candidate.entry_number,
        candidate.description,
        candidate.category_name,
        candidate.vendor_name,
        candidate.cost_center,
        getCandidateAmount(candidate, matchingType).toFixed(3)
      ].filter(Boolean).map(normalizeCandidateText).join(' ');
      return haystack.includes(search);
    }).sort((a, b) => {
      const aDiff = Math.abs(getCandidateAmount(a, matchingType) - lineAmount);
      const bDiff = Math.abs(getCandidateAmount(b, matchingType) - lineAmount);
      return aDiff - bDiff;
    }).slice(0, 200);
  }

  async function handleManualMatch() {
    if (!matchingLine) {
      toast.warning('Select a transaction to match');
      return;
    }
    if (matchAllocations.length === 0 && !matchingCandidateId) {
      toast.warning('Select a transaction target to match');
      return;
    }
    if (matchAllocations.length > 0) {
      const invalidAllocation = matchAllocations.find((allocation) => !allocation.allocated_amount || allocation.allocated_amount <= 0);
      if (invalidAllocation) {
        toast.warning('Every allocation needs a positive amount');
        return;
      }
      if (Math.abs(matchAllocationRemaining) > 0.001) {
        toast.warning(`Allocation total must equal the bank line amount. Remaining: ${formatBHD(matchAllocationRemaining)} BHD`);
        return;
      }
    }

    matchLoading = true;
    try {
      if (matchAllocations.length > 0) {
        await CreateSplitAllocation(
          matchingLine.id,
          matchAllocations.map((allocation) => ({
            allocation_type: allocation.allocation_type,
            entity_id: allocation.entity_id,
            allocated_amount: roundAmount(allocation.allocated_amount)
          })),
          'admin'
        );
        toast.success('Transaction allocated successfully');
      } else {
        await ManualMatchLine(matchingLine.id, matchingType, matchingCandidateId, 'admin');
        toast.success('Transaction matched successfully');
      }
      showMatchModal = false;
      matchAllocations = [];
      if (selectedStatement) {
        await loadStatementLines(selectedStatement);
      }
    } catch (err) {
      console.error('Manual match failed:', err);
      toast.danger(`Manual match failed: ${err}`);
    } finally {
      matchLoading = false;
    }
  }

  async function handleUnmatch(line: BankStatementLine) {
    try {
      // UnmatchLine(lineID, user, reason)
      await UnmatchLine(line.id, 'admin', 'Manual unmatch');
      toast.success('Transaction unmatched');
      if (selectedStatement) {
        await loadStatementLines(selectedStatement);
      }
    } catch (err) {
      console.error('Unmatch failed:', err);
      toast.danger('Failed to unmatch');
    }
  }

  // Import handler - Opens native file dialog via Go backend
  async function handleImport() {
    if (!importAccountId) {
      toast.warning('Please select a bank account');
      return;
    }
    importLoading = true;
    try {
      const importedForAccount = importAccountId;
      const result = await ImportBankStatementWithDialog(importAccountId);
      if (result) {
        toast.success(`Imported statement: ${result.statement_number}`);
        showImportModal = false;
        importAccountId = '';
        // Switch to the imported account and refresh
        selectedAccountId = importedForAccount;
        await loadStatements();
        // Also refresh cash position
        try {
          const pos = await GetCashPosition();
          cashPosition = pos;
        } catch (_) {}
      } else {
        // User cancelled file selection
        toast.info('Import cancelled');
      }
    } catch (err) {
      console.error('Import failed:', err);
      toast.danger(`Import failed: ${err}`);
    } finally {
      importLoading = false;
    }
  }

  // Account change handler
  async function handleAccountChange() {
    selectedStatement = null;
    statementLines = [];
    reconciliationSummary = null;
    await loadStatements();
  }

  // Bank account management functions
  async function openManageAccountsModal() {
    showManageAccountsModal = true;
    await loadAllBankAccounts();
  }

  async function loadAllBankAccounts() {
    try {
      const result = await GetAllBankAccounts();
      allBankAccounts = result || [];
    } catch (err) {
      console.error('Failed to load all bank accounts:', err);
      toast.danger('Failed to load accounts');
    }
  }

  function resetAccountForm() {
    editingAccount = null;
    accountFormData = {
      division: company,
      bank_name: '',
      account_name: '',
      account_number: '',
      iban: '',
      swift_bic: '',
      currency: 'BHD',
      booking_rate: 0
    };
  }

  function editAccount(account: BankAccount) {
    editingAccount = account;
    accountFormData = {
      division: account.division || company,
      bank_name: account.bank_name,
      account_name: account.account_name || '',
      account_number: account.account_number,
      iban: account.iban || '',
      swift_bic: account.swift_bic || '',
      currency: account.currency || 'BHD',
      booking_rate: (account as any).booking_rate || 0
    };
  }

  async function handleSaveAccount() {
    if (savingAccount) return;

    // Validation
    if (!accountFormData.bank_name) {
      toast.warning('Bank name is required');
      return;
    }
    if (!accountFormData.account_number) {
      toast.warning('Account number is required');
      return;
    }

    savingAccount = true;
    try {
      if (editingAccount) {
        // Update existing account
        await UpdateBankAccount(editingAccount.id, accountFormData);
        toast.success('Account updated successfully');
      } else {
        // Create new account
        await CreateBankAccount(accountFormData as any);
        toast.success('Account created successfully');
      }

      // Refresh lists
      await loadAllBankAccounts();
      await loadData(); // Refresh active accounts for dropdown
      resetAccountForm();
    } catch (err) {
      console.error('Failed to save account:', err);
      toast.danger(`Failed to save account: ${err}`);
    } finally {
      savingAccount = false;
    }
  }

  async function handleDeleteAccount(account: BankAccount) {
    if (!confirm(`Are you sure you want to deactivate ${account.bank_name} - ${account.account_number}?`)) {
      return;
    }

    try {
      await DeleteBankAccount(account.id);
      toast.success('Account deactivated successfully');
      await loadAllBankAccounts();
      await loadData(); // Refresh active accounts
    } catch (err) {
      console.error('Failed to delete account:', err);
      toast.danger(`Failed to deactivate account: ${err}`);
    }
  }

  function handleImportFromStatement() {
    toast.info('Select a statement file - bank details will be auto-detected');
    showManageAccountsModal = false;
    showImportModal = true;
  }

  // Statement CRUD handlers
  async function handleDeleteStatement() {
    if (!selectedStatement) return;
    if (!confirm(`Are you sure you want to delete statement ${selectedStatement.statement_number}?`)) {
      return;
    }

    try {
      await DeleteBankStatement(selectedStatement.id);
      toast.success('Statement deleted successfully');
      selectedStatement = null;
      statementLines = [];
      await loadStatements();
    } catch (err) {
      console.error('Failed to delete statement:', err);
      toast.danger(`Failed to delete statement: ${err}`);
    }
  }

  function openEditStatementModal() {
    if (!selectedStatement) return;

    // Format dates for input type="date" (YYYY-MM-DD)
    const formatDateForInput = (dateValue: any) => {
      if (!dateValue) return '';
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      return date.toISOString().split('T')[0];
    };

    editStatementData = {
      opening_balance: selectedStatement.opening_balance,
      closing_balance: selectedStatement.closing_balance,
      period_start: formatDateForInput(selectedStatement.period_start),
      period_end: formatDateForInput(selectedStatement.period_end),
      status: selectedStatement.status,
      notes: selectedStatement.notes || ''
    };
    showEditStatementModal = true;
  }

  async function handleSaveStatement() {
    if (savingStatement) return;
    if (!selectedStatement) return;

    // Validation
    if (!editStatementData.period_start || !editStatementData.period_end) {
      toast.warning('Period start and end dates are required');
      return;
    }

    savingStatement = true;
    try {
      const statementID = selectedStatement.id;
      await UpdateBankStatement(statementID, editStatementData);
      selectedStatement = {
        ...selectedStatement,
        ...editStatementData,
      };
      toast.success('Statement updated successfully');
      showEditStatementModal = false;
      await loadStatements();
      // Reload the selected statement to show updated data
      const updatedStatement = statements.find(s => s.id === statementID);
      if (updatedStatement) {
        selectedStatement = updatedStatement;
      }
    } catch (err) {
      console.error('Failed to update statement:', err);
      toast.danger(`Failed to update statement: ${err}`);
    } finally {
      savingStatement = false;
    }
  }

  // Transaction line CRUD handlers
  function openEditLineModal(line: BankStatementLine) {
    if (isStatementLocked(selectedStatement)) {
      toast.info('Reopen the statement before editing transactions');
      return;
    }
    editingLine = line;

    // Format date for input
    const formatDateForInput = (dateValue: any) => {
      if (!dateValue) return '';
      const date = typeof dateValue === 'string' ? new Date(dateValue) : new Date(dateValue);
      return date.toISOString().split('T')[0];
    };

    lineFormData = {
      transaction_date: formatDateForInput(line.transaction_date),
      description: line.description,
      reference: line.reference || '',
      debit: line.debit,
      credit: line.credit
    };
    showEditLineModal = true;
  }

  function openAddLineModal() {
    if (!selectedStatement) return;
    if (isStatementLocked(selectedStatement)) {
      toast.info('Reopen the statement before adding transactions');
      return;
    }
    editingLine = null;
    lineFormData = {
      transaction_date: '',
      description: '',
      reference: '',
      debit: 0,
      credit: 0
    };
    showAddLineModal = true;
  }

  function flipLineDebitCredit() {
    const nextDebit = Number(lineFormData.credit) || 0;
    const nextCredit = Number(lineFormData.debit) || 0;
    lineFormData = {
      ...lineFormData,
      debit: nextDebit,
      credit: nextCredit
    };
  }

  function openOCRReviewModal(line: BankStatementLine) {
    showMatchModal = false;
    openEditLineModal(line);
  }

  async function handleSaveLine() {
    if (savingLine) return;

    // Validation
    if (!lineFormData.transaction_date) {
      toast.warning('Transaction date is required');
      return;
    }
    if (!lineFormData.description) {
      toast.warning('Description is required');
      return;
    }
    if (lineFormData.debit === 0 && lineFormData.credit === 0) {
      toast.warning('Either debit or credit must be non-zero');
      return;
    }
    if (lineFormData.debit > 0 && lineFormData.credit > 0) {
      toast.warning('Cannot have both debit and credit in the same line');
      return;
    }

    savingLine = true;
    try {
      const editedMatchedLine = Boolean(editingLine?.is_matched);
      if (editingLine) {
        // Update existing line
        await UpdateBankStatementLine(editingLine.id, lineFormData);
        toast.success(editedMatchedLine
          ? 'Transaction updated and existing match cleared for re-review'
          : 'Transaction updated successfully');
        showEditLineModal = false;
      } else {
        // Create new line
        if (!selectedStatement) {
          toast.danger('No statement selected');
          return;
        }
        await CreateBankStatementLine(selectedStatement.id, lineFormData);
        toast.success('Transaction added successfully');
        showAddLineModal = false;
      }

      // Reload lines and statement (totals may have changed)
      if (selectedStatement) {
        await loadStatementLines(selectedStatement);
        await loadStatements();
      }
    } catch (err) {
      console.error('Failed to save transaction:', err);
      toast.danger(`Failed to save transaction: ${err}`);
    } finally {
      savingLine = false;
    }
  }

  async function handleDeleteLine() {
    if (!editingLine) return;
    if (!confirm(`Are you sure you want to delete this transaction?`)) {
      return;
    }

    savingLine = true;
    try {
      await DeleteBankStatementLine(editingLine.id);
      toast.success('Transaction deleted successfully');
      showEditLineModal = false;

      // Reload lines and statement
      if (selectedStatement) {
        await loadStatementLines(selectedStatement);
        await loadStatements();
      }
    } catch (err) {
      console.error('Failed to delete transaction:', err);
      toast.danger(`Failed to delete transaction: ${err}`);
    } finally {
      savingLine = false;
    }
  }

  onMount(() => {
    loadData();
  });

  $: if (company !== lastLoadedCompany) {
    lastLoadedCompany = company;
    importAccountId = '';
    selectedAccountId = '';
    selectedStatement = null;
    statementLines = [];
    resetAccountForm();
    void loadData();
  }
</script>

<PageLayout title="Bank Reconciliation" subtitle="Import and reconcile bank statements" {embedded}>
    {#if loading}
      <div class="loading-container">
        <WabiSpinner size="lg" />
        <p>Loading bank data...</p>
      </div>
    {:else}
      <!-- Cash Position KPIs -->
      <div class="kpi-grid">
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Total Cash Position</span>
            <span class="kpi-value primary">{formatBHD(totalVisibleCashPosition || cashPosition?.total_bhd || 0)} BHD</span>
            <span class="kpi-note">Latest statement closings</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Statements Open</span>
            <span class="kpi-value success">{openStatements}</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Statements Reconciled</span>
            <span class="kpi-value success">{reconciliationStats?.reconciled_statements || 0}</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Current Statement</span>
            <span class="kpi-value">{currentStatementLabel()}</span>
          </div>
        </Card>
      </div>

      {#if cashPositionNotices.length > 0}
        <div class="cash-position-notice">
          <strong>Statement check</strong>
          <span>{cashPositionNotices.join(' ')}</span>
        </div>
      {/if}

      <!-- Account Selector and Actions -->
      <div class="toolbar">
        <div class="left">
          <FormGroup label="Bank Account" inline>
            <select bind:value={selectedAccountId} on:change={handleAccountChange} class="account-select" disabled={statementsLoading}>
              {#each visibleBankAccounts as account}
                <option value={account.id}>{formatBankAccountOption(account)}</option>
              {/each}
            </select>
          </FormGroup>
          {#if statementsLoading}
            <div class="inline-loading">
              <WabiSpinner size="sm" tempo="calm" />
              <span>Refreshing statements...</span>
            </div>
          {/if}
        </div>
        <div class="right">
          <Button variant="secondary" on:click={openManageAccountsModal}>
            Manage Accounts
          </Button>
          <Button variant="secondary" on:click={() => showImportModal = true}>
            Import Statement
          </Button>
          {#if selectedStatement}
            <Button variant="secondary" on:click={openEditStatementModal} disabled={isStatementLocked(selectedStatement)}>
              Edit Statement
            </Button>
            <Button variant="danger" on:click={handleDeleteStatement} disabled={isStatementLocked(selectedStatement)}>
              Delete Statement
            </Button>
            <Button variant="primary" on:click={handleAutoMatch} disabled={isStatementLocked(selectedStatement)}>
              Auto-Match
            </Button>
            {#if isStatementLocked(selectedStatement)}
              <Button variant="secondary" on:click={handleReopen}>
                Reopen
              </Button>
            {:else}
              <Button variant="success" on:click={handleFinalize} disabled={totalUnmatched > 0}>
                Finalize
              </Button>
            {/if}
          {/if}
        </div>
      </div>

      <!-- Split View: Statements + Lines -->
      <div class="split-view">
        <!-- Statements Panel -->
        <div class="panel statements-panel">
          <h3>Statements</h3>
          {#if statementsLoading}
            <div class="loading-container small">
              <WabiSpinner size="md" tempo="calm" />
            </div>
          {:else}
            <DataTable
              data={statements}
              columns={statementColumns}
              onRowClick={loadStatementLines}
              selectedId={selectedStatement?.id}
              emptyMessage="No statements imported"
            />
          {/if}
        </div>

        <!-- Lines Panel -->
        <div class="panel lines-panel">
          <div class="panel-header">
            <h3>
              {#if selectedStatement}
                Transactions - {selectedStatement.statement_number}
              {:else}
                Select a statement
              {/if}
            </h3>
            {#if selectedStatement}
              <Button variant="secondary" size="sm" on:click={openAddLineModal} disabled={isStatementLocked(selectedStatement)}>
                Add Transaction
              </Button>
            {/if}
          </div>
          {#if selectedStatement && reconciliationSummary}
            <div class="statement-summary">
              <span>Status: {formatStatementStatus(reconciliationSummary.status || selectedStatement.status)}</span>
              <span>{reconciliationSummary.matched_count}/{reconciliationSummary.total_lines} matched</span>
              <span>{reconciliationSummary.unmatched_count} open</span>
              <span>{Math.round(reconciliationSummary.matched_percent || 0)}% complete</span>
            </div>
          {/if}
          {#if selectedStatement?.notes}
            <div class="statement-notes-card">
              <div class="statement-notes-head">
                <span>Statement Notes</span>
                <span>OCR / AI</span>
              </div>
              <p>{selectedStatement.notes}</p>
            </div>
          {/if}
          {#if linesLoading}
            <div class="loading-container small">
              <WabiSpinner size="md" />
            </div>
          {:else if selectedStatement}
            <DataTable
              data={statementLines}
              columns={lineColumns}
              onRowClick={openMatchModal}
              emptyMessage="No transactions"
              rowClass={(row) => row.is_matched ? '' : 'unmatched-row'}
            />
          {:else}
            <div class="empty-state">
              <p>Select a statement to view transactions</p>
            </div>
          {/if}
        </div>
      </div>
    {/if}
</PageLayout>

<!-- Import Modal -->
<Modal bind:open={showImportModal} title="Import Bank Statement" size="xl">
  <div class="import-form">
    <FormGroup label="Bank Account">
      <select bind:value={importAccountId} class="form-select">
        <option value="">Select account...</option>
        {#each visibleBankAccounts as account}
          <option value={account.id}>{formatBankAccountOption(account)}</option>
        {/each}
      </select>
    </FormGroup>
    <div class="import-note">
      <p>A file dialog will open when you click Import.</p>
      <p>Supported formats: CSV and PDF</p>
    </div>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showImportModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleImport} loading={importLoading} disabled={!importAccountId}>
      Import
    </Button>
  </svelte:fragment>
</Modal>

<!-- Match Modal -->
<Modal bind:open={showMatchModal} title="Match Transaction" size="xl">
  {#if matchingLine}
    <div class="match-details">
      <div class="match-row">
        <span class="label">Date:</span>
        <span class="value">{formatDate(matchingLine.transaction_date)}</span>
      </div>
      <div class="match-row">
        <span class="label">Description:</span>
        <span class="value">{matchingLine.description}</span>
      </div>
      <div class="match-row">
        <span class="label">Amount:</span>
        <span class="value {matchingLine.credit > 0 ? 'credit' : 'debit'}">
          {matchingLine.credit > 0 ? '+' : '-'}{formatBHD(matchingLine.credit || matchingLine.debit)} BHD
        </span>
      </div>
      {#if matchingLine.extracted_customer}
        <div class="match-row">
          <span class="label">Detected Customer:</span>
          <span class="value">{matchingLine.extracted_customer}</span>
        </div>
      {/if}
      {#if matchingLine.extracted_invoices}
        <div class="match-row">
          <span class="label">Detected Invoices:</span>
          <span class="value">{matchingLine.extracted_invoices}</span>
        </div>
      {/if}
    </div>
    <div class="match-actions">
      {#if selectedStatement && isStatementLocked(selectedStatement)}
        <p class="match-hint">This statement is finalized. Reopen it to change matches.</p>
      {:else if matchingLine.is_matched}
        <Button variant="secondary" on:click={() => matchingLine && handleUnmatch(matchingLine)}>
          Unmatch
        </Button>
      {:else}
        <div class="manual-match-form">
          <div class="ocr-review-banner">
            <div>
              <strong>OCR review</strong>
              <p>If the parser flipped the amount side, correct the line before matching.</p>
            </div>
            <Button variant="secondary" size="sm" on:click={() => matchingLine && openOCRReviewModal(matchingLine)}>
              Fix Debit/Credit
            </Button>
          </div>
          <div class="form-row two-col">
            <FormGroup label="Match To">
              <select bind:value={matchingType} class="form-select" on:change={handleMatchingTypeChange}>
                {#if matchingLine.credit > 0}
                  <option value="CUSTOMER_INVOICE">Customer Open Invoice</option>
                  <option value="CUSTOMER_RECEIPT">Customer Receipt</option>
                  <option value="CUSTOMER_PAYMENT">Recorded Invoice Payment</option>
                {:else}
                  <option value="SUPPLIER_INVOICE">Supplier Invoice</option>
                  <option value="EXPENSE">Expense</option>
                  <option value="SUPPLIER_PAYMENT">Supplier Payment</option>
                  <option value="PAYROLL_PAYOUT">Payroll Payout</option>
                {/if}
              </select>
            </FormGroup>
            <FormGroup label="Search">
              <input
                type="search"
                class="form-input"
                bind:value={matchingSearch}
                placeholder="Search number, customer, supplier, amount..."
                autocomplete="off"
              />
            </FormGroup>
          </div>

          {#if matchLoading}
            <div class="loading-container small">
              <WabiSpinner size="sm" />
            </div>
          {:else}
            {#if matchAllocations.length > 0}
              <div class="allocation-panel">
                <div class="allocation-head">
                  <div>
                    <strong>Allocation plan</strong>
                    <span>{matchAllocations.length} target{matchAllocations.length === 1 ? '' : 's'}</span>
                  </div>
                  <div class="allocation-balance" class:balanced={Math.abs(matchAllocationRemaining) <= 0.001} class:over={matchAllocationRemaining < -0.001}>
                    <span>Total {formatBHD(matchAllocationTotal)} BHD</span>
                    <span>{matchAllocationRemaining >= 0 ? 'Remaining' : 'Over'} {formatBHD(Math.abs(matchAllocationRemaining))} BHD</span>
                  </div>
                </div>
                <div class="allocation-list">
                  {#each matchAllocations as allocation (allocation.key)}
                    <div class="allocation-row">
                      <div class="allocation-label">
                        <span>{allocation.label}</span>
                        <small>Open {formatBHD(allocation.max_amount)} BHD</small>
                      </div>
                      <input
                        type="number"
                        min="0.001"
                        step="0.001"
                        class="allocation-input"
                        value={allocation.allocated_amount}
                        on:input={(event) => updateMatchAllocationAmount(allocation.key, event)}
                      />
                      <button type="button" class="allocation-remove" on:click={() => removeMatchAllocation(allocation.key)}>
                        Remove
                      </button>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
            <div class="candidate-list">
              {#if getFilteredMatchCandidates().length > 0}
                {#each getFilteredMatchCandidates() as candidate}
                  <div class="candidate-option" class:in-plan={isCandidateAllocated(candidate)}>
                    <label>
                      <input type="radio" bind:group={matchingCandidateId} value={candidate.id} disabled={matchAllocations.length > 0} />
                      <span>{buildMatchCandidateLabel(candidate, matchingType)}</span>
                    </label>
                    {#if matchingType === 'CUSTOMER_INVOICE' || matchingType === 'CUSTOMER_PAYMENT' || matchingType === 'CUSTOMER_RECEIPT' || matchingType === 'SUPPLIER_INVOICE' || matchingType === 'EXPENSE'}
                      <button
                        type="button"
                        class="candidate-add"
                        on:click={() => addMatchAllocation(candidate)}
                        disabled={isCandidateAllocated(candidate)}
                      >
                        {isCandidateAllocated(candidate) ? 'Added' : 'Add'}
                      </button>
                    {/if}
                  </div>
                {/each}
              {:else}
                <p class="match-hint">No matching records found for this filter.</p>
              {/if}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
  <svelte:fragment slot="footer">
    {#if matchingLine && !matchingLine.is_matched && !(selectedStatement && isStatementLocked(selectedStatement))}
      <Button
        variant="primary"
        on:click={handleManualMatch}
        loading={matchLoading}
        disabled={(matchAllocations.length === 0 && !matchingCandidateId) || (matchAllocations.length > 0 && Math.abs(matchAllocationRemaining) > 0.001)}
      >
        {matchAllocations.length > 0 ? 'Match Allocations' : 'Match Selected'}
      </Button>
    {/if}
    <Button variant="secondary" on:click={() => showMatchModal = false}>Close</Button>
  </svelte:fragment>
</Modal>

<!-- Edit Statement Modal -->
<Modal bind:open={showEditStatementModal} title="Edit Statement" size="md">
  <div class="statement-edit-form">
    <div class="form-row two-col">
      <FormGroup label="Period Start">
        <input type="date" bind:value={editStatementData.period_start} class="form-input" />
      </FormGroup>
      <FormGroup label="Period End">
        <input type="date" bind:value={editStatementData.period_end} class="form-input" />
      </FormGroup>
    </div>
    <div class="form-row two-col">
      <FormGroup label="Opening Balance (BHD)">
        <input type="number" step="0.001" bind:value={editStatementData.opening_balance} class="form-input" />
      </FormGroup>
      <FormGroup label="Closing Balance (BHD)">
        <input type="number" step="0.001" bind:value={editStatementData.closing_balance} class="form-input" />
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Status">
        <select bind:value={editStatementData.status} class="form-select">
          <option value="Imported">Imported</option>
          <option value="InProgress">In Progress</option>
        </select>
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Notes">
        <textarea bind:value={editStatementData.notes} class="form-input form-textarea" rows="4" placeholder="OCR summary, import context, or reconciliation notes"></textarea>
      </FormGroup>
    </div>
    <p class="form-note">Use Finalize and Reopen for reconciled workflow states.</p>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showEditStatementModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleSaveStatement} loading={savingStatement}>
      Save Changes
    </Button>
  </svelte:fragment>
</Modal>

<!-- Edit Transaction Line Modal -->
<Modal bind:open={showEditLineModal} title="Edit Transaction" size="md">
  <div class="line-edit-form">
    <div class="form-row">
      <FormGroup label="Transaction Date">
        <input type="date" bind:value={lineFormData.transaction_date} class="form-input" />
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Description">
        <Input bind:value={lineFormData.description} placeholder="Transaction description" />
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Reference">
        <Input bind:value={lineFormData.reference} placeholder="Reference number (optional)" />
      </FormGroup>
    </div>
    <div class="form-row two-col">
      <FormGroup label="Debit (BHD)">
        <input type="number" step="0.001" bind:value={lineFormData.debit} class="form-input" />
      </FormGroup>
      <FormGroup label="Credit (BHD)">
        <input type="number" step="0.001" bind:value={lineFormData.credit} class="form-input" />
      </FormGroup>
    </div>
    <div class="line-fix-actions">
      <Button variant="secondary" size="sm" on:click={flipLineDebitCredit}>
        Flip Debit / Credit
      </Button>
      {#if editingLine?.is_matched}
        <span class="line-fix-hint">Saving this correction will clear the current match so it can be reviewed again.</span>
      {:else}
        <span class="line-fix-hint">Use this when OCR has placed the amount on the wrong side.</span>
      {/if}
    </div>
    <div class="form-note">
      <p>Note: Only one of Debit or Credit should be non-zero</p>
    </div>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="danger" on:click={handleDeleteLine} loading={savingLine}>
      Delete Transaction
    </Button>
    <div style="flex: 1;"></div>
    <Button variant="secondary" on:click={() => showEditLineModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleSaveLine} loading={savingLine}>
      Save Changes
    </Button>
  </svelte:fragment>
</Modal>

<!-- Add Transaction Line Modal -->
<Modal bind:open={showAddLineModal} title="Add Transaction" size="md">
  <div class="line-edit-form">
    <div class="form-row">
      <FormGroup label="Transaction Date">
        <input type="date" bind:value={lineFormData.transaction_date} class="form-input" />
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Description">
        <Input bind:value={lineFormData.description} placeholder="Transaction description" />
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Reference">
        <Input bind:value={lineFormData.reference} placeholder="Reference number (optional)" />
      </FormGroup>
    </div>
    <div class="form-row two-col">
      <FormGroup label="Debit (BHD)">
        <input type="number" step="0.001" bind:value={lineFormData.debit} class="form-input" />
      </FormGroup>
      <FormGroup label="Credit (BHD)">
        <input type="number" step="0.001" bind:value={lineFormData.credit} class="form-input" />
      </FormGroup>
    </div>
    <div class="form-note">
      <p>Note: Only one of Debit or Credit should be non-zero</p>
    </div>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => showAddLineModal = false}>Cancel</Button>
    <Button variant="primary" on:click={handleSaveLine} loading={savingLine}>
      Add Transaction
    </Button>
  </svelte:fragment>
</Modal>

<!-- Manage Bank Accounts Modal -->
<Modal bind:open={showManageAccountsModal} title="Manage Bank Accounts" size="lg">
  <div class="manage-accounts-container">
    <!-- Existing Accounts List -->
    <div class="accounts-section">
      <h4>Existing Accounts</h4>
      <div class="accounts-list">
        {#if visibleAllBankAccounts.length === 0}
          <p class="empty-message">No bank accounts found</p>
        {:else}
          {#each visibleAllBankAccounts as account}
            <div class="account-item" class:inactive={!account.is_active}>
              <div class="account-info">
                <div class="account-main">
                  <strong>{account.account_name || account.bank_name}</strong>
                  {#if account.account_name}
                    <span style="color: var(--text-secondary); font-size: 12px; margin-left: 6px;">{account.bank_name}</span>
                  {/if}
                  {#if !account.is_active}
                    <span class="inactive-badge">Inactive</span>
                  {/if}
                </div>
                <div class="account-details">
                  <span>{account.account_number}</span>
                  {#if account.iban}
                    <span class="iban">{account.iban}</span>
                  {/if}
                  <span class="currency">{account.currency}</span>
                </div>
              </div>
              <div class="account-actions">
                <button class="icon-btn" on:click={() => editAccount(account)} title="Edit">
                  ✏️
                </button>
                {#if account.is_active}
                  <button class="icon-btn danger" on:click={() => handleDeleteAccount(account)} title="Deactivate">
                    🗑️
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <!-- Add/Edit Account Form -->
    <div class="form-section">
      <h4>{editingAccount ? 'Edit Account' : 'Add New Account'}</h4>
      <div class="account-form">
        <div class="form-row">
          <FormGroup label="Company">
            <select bind:value={accountFormData.division} class="form-select">
              <option value="PH Trading">PH Trading</option>
              <option value="AHS Trading">AHS Trading</option>
            </select>
          </FormGroup>
        </div>
        <div class="form-row">
          <FormGroup label="Bank Name *">
            <Input bind:value={accountFormData.bank_name} placeholder="e.g., National Bank of Bahrain" />
          </FormGroup>
        </div>
        <div class="form-row">
          <FormGroup label="Display Name (your friendly label for this account)">
            <Input bind:value={accountFormData.account_name} placeholder="e.g., NBB - BHD Operating, NBB Euro Call Deposit" />
          </FormGroup>
        </div>
        <div class="form-row">
          <FormGroup label="Account Number *">
            <Input bind:value={accountFormData.account_number} placeholder="e.g., 99570076" />
          </FormGroup>
        </div>
        <div class="form-row">
          <FormGroup label="IBAN">
            <Input bind:value={accountFormData.iban} placeholder="e.g., BH16NBOB00000099570076" />
          </FormGroup>
        </div>
        <div class="form-row two-col">
          <FormGroup label="SWIFT/BIC">
            <Input bind:value={accountFormData.swift_bic} placeholder="e.g., NBOBBHBM" />
          </FormGroup>
          <FormGroup label="Currency">
            <select bind:value={accountFormData.currency} class="form-select">
              <option value="BHD">BHD</option>
              <option value="USD">USD</option>
              <option value="EUR">EUR</option>
              <option value="GBP">GBP</option>
              <option value="SAR">SAR</option>
            </select>
          </FormGroup>
        </div>
        {#if accountFormData.currency !== 'BHD'}
        <div class="form-row">
          <FormGroup label="Opening/Booking Rate (rate when account was opened, used for FX revaluation)">
            <Input type="number" bind:value={accountFormData.booking_rate} placeholder="e.g., 1.4523" step="0.0001" min="0" />
          </FormGroup>
        </div>
        {/if}
        <div class="form-actions">
          {#if editingAccount}
            <Button variant="secondary" on:click={resetAccountForm}>Cancel Edit</Button>
          {/if}
          <Button variant="primary" on:click={handleSaveAccount} loading={savingAccount}>
            {editingAccount ? 'Update Account' : 'Add Account'}
          </Button>
        </div>
      </div>
      <div class="import-hint">
        <p>Or import bank details from a statement:</p>
        <Button variant="secondary" size="sm" on:click={handleImportFromStatement}>
          Import from Statement
        </Button>
      </div>
    </div>
  </div>
  <svelte:fragment slot="footer">
    <Button variant="secondary" on:click={() => { showManageAccountsModal = false; resetAccountForm(); }}>Close</Button>
  </svelte:fragment>
</Modal>

<style>
  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 24px;
    gap: 12px;
  }

  .loading-container.small {
    padding: 16px;
  }

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .kpi {
    display: flex;
    flex-direction: column;
    gap: 4px;
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
  }

  .kpi-value.primary { color: var(--brand-indigo); }
  .kpi-value.success { color: #10B981; }
  .kpi-note {
    color: var(--text-secondary);
    font-size: 11px;
  }

  .cash-position-notice {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    margin: -4px 0 14px;
    padding: 10px 12px;
    border: 1px solid rgba(217, 149, 34, 0.24);
    border-radius: var(--radius-md);
    background: rgba(255, 248, 232, 0.88);
    color: #7a4d0b;
    font-size: 12px;
    line-height: 1.45;
  }

  .cash-position-notice strong {
    white-space: nowrap;
  }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    padding: 8px 12px;
    background: var(--surface-elevated);
    border-radius: var(--radius-md);
  }

  .toolbar .left,
  .toolbar .right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .account-select,
  .form-select {
    padding: 6px 12px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    font-size: 14px;
    min-width: 250px;
  }

  .form-input {
    padding: 6px 12px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    width: 100%;
  }

  .split-view {
    display: grid;
    grid-template-columns: minmax(620px, 1.05fr) minmax(760px, 1.55fr);
    gap: 12px;
    height: calc(100vh - 300px);
    min-height: 400px;
    overflow-x: auto;
  }

  .panel {
    background: var(--surface-default);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: 16px;
    overflow: auto;
  }

  .panel h3 {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .panel-header h3 {
    margin: 0;
  }

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 200px;
    color: var(--text-secondary);
  }

  .import-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .inline-loading {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-secondary);
    margin-left: 12px;
  }

  .import-note {
    padding: 12px;
    background: var(--surface-elevated);
    border-radius: var(--radius-sm);
    margin-top: 8px;
  }

  .import-note p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .match-details {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px 18px;
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--radius-md);
  }

  .match-row {
    display: flex;
    gap: 12px;
  }

  .match-row .label {
    font-weight: 500;
    color: var(--text-secondary);
    min-width: 140px;
  }

  .match-row .value {
    color: var(--text-primary);
  }

  .match-row .value.credit { color: #10B981; }
  .match-row .value.debit { color: #EF4444; }

  .match-actions {
    margin-top: 12px;
    display: flex;
    gap: 12px;
  }

  .manual-match-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
    width: 100%;
    min-width: 0;
  }

  .ocr-review-banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    border: 1px solid rgba(217, 119, 6, 0.24);
    border-radius: var(--radius-md);
    background: linear-gradient(180deg, rgba(255, 251, 235, 0.98) 0%, rgba(255, 247, 237, 0.95) 100%);
  }

  .ocr-review-banner strong {
    display: block;
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #B45309;
    margin-bottom: 4px;
  }

  .ocr-review-banner p {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .candidate-list {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    max-height: 420px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .candidate-option {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
    padding: 12px;
    border-radius: 14px;
    border: 1px solid var(--border-subtle);
    background: rgba(255, 255, 255, 0.92);
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
  }

  .candidate-option.in-plan {
    border-color: rgba(16, 185, 129, 0.35);
    background: rgba(240, 253, 244, 0.88);
  }

  .candidate-option label {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    min-width: 0;
    flex: 1;
  }

  .candidate-option input {
    margin-top: 2px;
    flex-shrink: 0;
  }

  .candidate-add,
  .allocation-remove {
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    color: var(--text-primary);
    font-size: 12px;
    font-weight: 600;
    padding: 5px 9px;
    cursor: pointer;
    flex-shrink: 0;
  }

  .candidate-add:disabled {
    cursor: default;
    color: #059669;
    border-color: rgba(16, 185, 129, 0.35);
    background: rgba(240, 253, 244, 0.9);
  }

  .allocation-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--surface-elevated);
  }

  .allocation-head,
  .allocation-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
    align-items: center;
  }

  .allocation-head strong {
    display: block;
    font-size: 13px;
    color: var(--text-primary);
  }

  .allocation-head span,
  .allocation-label small {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .allocation-balance {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    font-family: var(--font-mono);
  }

  .allocation-balance.balanced span:last-child {
    color: #059669;
  }

  .allocation-balance.over span:last-child {
    color: #DC2626;
  }

  .allocation-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .allocation-row {
    grid-template-columns: minmax(0, 1fr) 130px auto;
    padding: 8px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.82);
  }

  .allocation-label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .allocation-label span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
  }

  .allocation-input {
    width: 130px;
    padding: 6px 8px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-default);
    font-family: var(--font-mono);
    text-align: right;
  }

  @media (max-width: 900px) {
    .match-details,
    .candidate-list,
    .allocation-row {
      grid-template-columns: 1fr;
    }

    .split-view {
      grid-template-columns: 1fr;
      height: auto;
    }

    .allocation-balance {
      align-items: flex-start;
    }
  }

  .match-hint {
    color: var(--text-secondary);
    font-size: 13px;
  }

  :global(.unmatched-row) {
    background: rgba(239, 68, 68, 0.05) !important;
  }

  /* Manage Accounts Modal Styles */
  .manage-accounts-container {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
    min-height: 400px;
  }

  .accounts-section,
  .form-section {
    display: flex;
    flex-direction: column;
  }

  .accounts-section h4,
  .form-section h4 {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .accounts-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
    max-height: 450px;
  }

  .account-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    transition: all 0.15s ease;
  }

  .account-item:hover {
    border-color: var(--brand-indigo);
    background: var(--surface-elevated-hover);
  }

  .account-item.inactive {
    opacity: 0.6;
    background: var(--surface-default);
  }

  .account-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .account-main {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .account-main strong {
    font-size: 13px;
    color: var(--text-primary);
  }

  .inactive-badge {
    font-size: 10px;
    padding: 2px 6px;
    background: #F59E0B;
    color: white;
    border-radius: 3px;
    text-transform: uppercase;
    font-weight: 600;
  }

  .account-details {
    display: flex;
    gap: 12px;
    font-size: 11px;
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }

  .account-details .iban {
    color: var(--text-tertiary);
  }

  .account-details .currency {
    padding: 1px 4px;
    background: var(--brand-indigo);
    color: white;
    border-radius: 2px;
    font-weight: 600;
  }

  .account-actions {
    display: flex;
    gap: 6px;
  }

  .icon-btn {
    padding: 6px 8px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: 14px;
    transition: all 0.15s ease;
  }

  .icon-btn:hover {
    background: var(--surface-elevated);
    border-color: var(--brand-indigo);
  }

  .icon-btn.danger:hover {
    background: rgba(239, 68, 68, 0.1);
    border-color: #EF4444;
  }

  .account-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-row {
    display: flex;
    flex-direction: column;
  }

  .form-row.two-col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .form-actions {
    display: flex;
    gap: 8px;
    margin-top: 12px;
  }

  .statement-summary {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    margin-bottom: 12px;
    padding: 10px 12px;
    background: var(--surface-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-secondary);
  }

  .statement-notes-card {
    margin-bottom: 12px;
    padding: 12px 14px;
    background: linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(247,247,244,0.98) 100%);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
  }

  .statement-notes-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .statement-notes-card p {
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-primary);
    white-space: pre-wrap;
  }

  .form-textarea {
    min-height: 96px;
    resize: vertical;
  }

  .import-hint {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .import-hint p {
    font-size: 12px;
    color: var(--text-secondary);
    margin: 0;
  }

  .empty-message {
    text-align: center;
    color: var(--text-secondary);
    font-size: 13px;
    padding: 20px;
  }

  .statement-edit-form,
  .line-edit-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-note {
    padding: 8px 12px;
    background: var(--surface-elevated);
    border-left: 3px solid var(--brand-indigo);
    border-radius: var(--radius-sm);
    margin: 0;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .line-fix-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  .line-fix-hint {
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .form-note p {
    margin: 0;
    font-size: 12px;
    color: var(--text-secondary);
  }

  @media (max-width: 1200px) {
    .kpi-grid {
      grid-template-columns: repeat(2, 1fr);
    }

    .split-view {
      grid-template-columns: 1fr;
      height: auto;
    }

    .panel {
      max-height: 400px;
    }

    .manage-accounts-container {
      grid-template-columns: 1fr;
    }
  }
</style>
