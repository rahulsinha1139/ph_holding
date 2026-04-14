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
    GetAuditTrail,
    DeleteBankStatement,
    UpdateBankStatement,
    CreateBankStatementLine,
    UpdateBankStatementLine,
    DeleteBankStatementLine,
    ListCustomerInvoices,
    GetSupplierInvoices,
    GetAllSupplierPayments,
    ListUnreconciledPayrollPayouts,
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

  // State - using any types for Wails-generated compatibility
  let bankAccounts: any[] = [];
  let allBankAccounts: any[] = []; // All accounts including inactive for management
  let statements: any[] = [];
  let selectedAccountId = '';
  let selectedStatement: any | null = null;
  let statementLines: any[] = [];
  let cashPosition: any = null;
  let loading = true;
  let statementsLoading = false;
  let linesLoading = false;

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
  let matchLoading = false;
  let customerInvoiceCandidates: any[] = [];
  let supplierInvoiceCandidates: any[] = [];
  let supplierPaymentCandidates: any[] = [];
  let payrollPayoutCandidates: any[] = [];

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
  $: totalVisibleCashPosition = visibleCashAccounts.reduce((sum: number, account: any) => sum + (Number(account.current_balance) || 0), 0);
  $: if (company && visibleBankAccounts.length > 0 && !visibleBankAccounts.some((account: any) => account.id === selectedAccountId)) {
    selectedAccountId = visibleBankAccounts[0].id;
    loadStatements();
  }
  $: if (company && visibleBankAccounts.length === 0 && selectedAccountId) {
    selectedAccountId = '';
    selectedStatement = null;
    statementLines = [];
  }

  // KPI calculations
  $: totalUnmatched = statementLines.filter(l => !l.is_matched).length;
  $: totalMatched = statementLines.filter(l => l.is_matched).length;
  $: unmatchedDebit = statementLines.filter(l => !l.is_matched && l.debit > 0).reduce((sum, l) => sum + l.debit, 0);
  $: unmatchedCredit = statementLines.filter(l => !l.is_matched && l.credit > 0).reduce((sum, l) => sum + l.credit, 0);

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
          'In Progress': '#3B82F6',
          'Reconciled': '#10B981',
          'Verified': '#059669'
        };
        const color = statusColors[row.status] || '#6B7280';
        return `<span style="color: ${color}; font-weight: 500; font-size: 12px;">${escapeHtml(row.status)}</span>`;
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
          return `<span style="color: #10B981; font-size: 12px;">Matched (${confidence}%)</span>`;
        }
        return `<span style="color: #EF4444; font-size: 12px;">Unmatched</span>`;
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

  function formatBankAccountOption(account: any): string {
    const bank = account?.bank_name || 'Bank Account';
    const accountName = account?.account_name ? ` ${account.account_name}` : '';
    const accountNumber = account?.account_number ? ` - ${account.account_number}` : '';
    const currency = account?.currency ? ` (${account.currency})` : '';
    return `${bank}${accountName}${accountNumber}${currency}`;
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
      const scopedAccounts = (accountsResult || []).filter((account: any) => matchesCompany(account.division));
      if (scopedAccounts.length > 0 && !selectedAccountId) {
        selectedAccountId = scopedAccounts[0].id;
        await loadStatements();
      }
    } catch (err) {
      console.error('Failed to load bank accounts:', err);
      accountsError = err;
      toast.danger('Failed to load bank accounts');
    }

    try {
      const positionResult = await GetCashPosition();
      cashPosition = {
        ...(positionResult || {}),
        accounts: (positionResult?.accounts || []).filter((account: any) => matchesCompany(account.division)),
      };
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
    } catch (err) {
      console.error('Finalize failed:', err);
      toast.danger('Finalization failed');
    }
  }

  function openMatchModal(line: BankStatementLine) {
    matchingLine = line;
    matchingSearch = '';
    matchingCandidateId = '';
    matchingType = line.credit > 0 ? 'CUSTOMER_INVOICE' : 'SUPPLIER_INVOICE';
    showMatchModal = true;
    void loadMatchCandidates(line);
  }

  async function loadMatchCandidates(line: BankStatementLine) {
    matchLoading = true;
    try {
      const [allCustomerInvoices, allSupplierInvoices, supplierPayments] = await Promise.all([
        ListCustomerInvoices(1000, 0).catch(() => []),
        GetSupplierInvoices().catch(() => []),
        GetAllSupplierPayments().catch(() => [])
      ]);

      customerInvoiceCandidates = (allCustomerInvoices || []).filter((invoice: any) => {
        const status = String(invoice?.status || '').toLowerCase();
        return matchesCompany(invoice?.division) && Number(invoice?.outstanding_bhd || 0) > 0 && !['cancelled', 'void', 'draft'].includes(status);
      });
      supplierInvoiceCandidates = (allSupplierInvoices || []).filter((invoice: any) => {
        const status = String(invoice?.status || '').toLowerCase();
        const paymentStatus = String(invoice?.payment_status || '').toLowerCase();
        return matchesCompany(invoice?.division) && paymentStatus !== 'paid' && !['cancelled', 'void', 'rejected'].includes(status);
      });
      supplierPaymentCandidates = (supplierPayments || []).filter((payment: any) => matchesCompany(payment?.division));
      payrollPayoutCandidates = (await ListUnreconciledPayrollPayouts().catch(() => [])).filter((payout: any) => matchesCompany(payout?.division));
    } finally {
      matchLoading = false;
    }
  }

  function formatMatchAmount(line: BankStatementLine) {
    return line.credit > 0 ? line.credit : line.debit;
  }

  function buildMatchCandidateLabel(candidate: any, type: string) {
    if (type === 'CUSTOMER_INVOICE') {
      return `${candidate.invoice_number || 'Invoice'} • ${candidate.customer_name || 'Customer'} • ${formatBHD(candidate.grand_total_bhd || 0)} BHD`;
    }
    if (type === 'SUPPLIER_PAYMENT') {
      return `${candidate.reference || candidate.invoice_number || 'Payment'} • ${candidate.supplier_name || 'Supplier'} • ${formatBHD(candidate.amount_bhd || 0)} BHD`;
    }
    if (type === 'PAYROLL_PAYOUT') {
      return `${candidate.run_number || 'Payroll'} • ${candidate.employee_name || 'Employee'} • ${formatBHD(candidate.amount || 0)} BHD${candidate.payment_reference ? ` • ${candidate.payment_reference}` : ''}`;
    }
    return `${candidate.invoice_number || 'Invoice'} • ${candidate.supplier_name || 'Supplier'} • ${formatBHD(candidate.total_bhd || 0)} BHD`;
  }

  function getFilteredMatchCandidates() {
    const search = matchingSearch.trim().toLowerCase();
    const source = matchingType === 'CUSTOMER_INVOICE'
      ? customerInvoiceCandidates
      : matchingType === 'SUPPLIER_PAYMENT'
        ? supplierPaymentCandidates
        : matchingType === 'PAYROLL_PAYOUT'
          ? payrollPayoutCandidates
        : supplierInvoiceCandidates;

    return (source || []).filter((candidate) => {
      if (!search) return true;
      const haystack = [
        candidate.invoice_number,
        candidate.customer_name,
        candidate.supplier_name,
        candidate.reference,
        candidate.employee_name,
        candidate.run_number,
        candidate.payment_reference
      ].filter(Boolean).join(' ').toLowerCase();
      return haystack.includes(search);
    }).slice(0, 200);
  }

  async function handleManualMatch() {
    if (!matchingLine || !matchingCandidateId) {
      toast.warning('Select a transaction target to match');
      return;
    }

    matchLoading = true;
    try {
      await ManualMatchLine(matchingLine.id, matchingType, matchingCandidateId, 'admin');
      toast.success('Transaction matched successfully');
      showMatchModal = false;
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
          cashPosition = {
            ...(pos || {}),
            accounts: (pos?.accounts || []).filter((account: any) => matchesCompany(account.division)),
          };
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

  $: if (company) {
    importAccountId = '';
    resetAccountForm();
    loadData();
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
            <span class="kpi-value primary">{formatBHD(totalVisibleCashPosition)} BHD</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Matched</span>
            <span class="kpi-value success">{totalMatched}</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Unmatched</span>
            <span class="kpi-value warning">{totalUnmatched}</span>
          </div>
        </Card>
        <Card variant="elevated">
          <div class="kpi">
            <span class="kpi-label">Unmatched Credits</span>
            <span class="kpi-value">{formatBHD(unmatchedCredit)} BHD</span>
          </div>
        </Card>
      </div>

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
            <Button variant="secondary" on:click={openEditStatementModal}>
              Edit Statement
            </Button>
            <Button variant="danger" on:click={handleDeleteStatement}>
              Delete Statement
            </Button>
            <Button variant="primary" on:click={handleAutoMatch}>
              Auto-Match
            </Button>
            <Button variant="success" on:click={handleFinalize} disabled={totalUnmatched > 0}>
              Finalize
            </Button>
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
              <Button variant="secondary" size="sm" on:click={openAddLineModal}>
                Add Transaction
              </Button>
            {/if}
          </div>
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
      {#if matchingLine.is_matched}
        <Button variant="secondary" on:click={() => matchingLine && handleUnmatch(matchingLine)}>
          Unmatch
        </Button>
      {:else}
        <div class="manual-match-form">
          <div class="ocr-review-banner">
            <div>
              <strong>OCR review</strong>
              <p>If the parser flipped the polarity, correct the line before matching.</p>
            </div>
            <Button variant="secondary" size="sm" on:click={() => matchingLine && openOCRReviewModal(matchingLine)}>
              Fix Debit/Credit
            </Button>
          </div>
          <div class="form-row two-col">
            <FormGroup label="Match To">
              <select bind:value={matchingType} class="form-select">
                {#if matchingLine.credit > 0}
                  <option value="CUSTOMER_INVOICE">Customer Invoice</option>
                {:else}
                  <option value="SUPPLIER_INVOICE">Supplier Invoice</option>
                  <option value="SUPPLIER_PAYMENT">Supplier Payment</option>
                  <option value="PAYROLL_PAYOUT">Payroll Payout</option>
                {/if}
              </select>
            </FormGroup>
            <FormGroup label="Search">
              <Input bind:value={matchingSearch} placeholder="Search number, customer, supplier..." />
            </FormGroup>
          </div>

          {#if matchLoading}
            <div class="loading-container small">
              <WabiSpinner size="sm" />
            </div>
          {:else}
            <div class="candidate-list">
              {#if getFilteredMatchCandidates().length > 0}
                {#each getFilteredMatchCandidates() as candidate}
                  <label class="candidate-option">
                    <input type="radio" bind:group={matchingCandidateId} value={candidate.id} />
                    <span>{buildMatchCandidateLabel(candidate, matchingType)}</span>
                  </label>
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
    {#if matchingLine && !matchingLine.is_matched}
      <Button variant="primary" on:click={handleManualMatch} loading={matchLoading} disabled={!matchingCandidateId}>
        Match Selected
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
          <option value="In Progress">In Progress</option>
          <option value="Reconciled">Reconciled</option>
          <option value="Verified">Verified</option>
        </select>
      </FormGroup>
    </div>
    <div class="form-row">
      <FormGroup label="Notes">
        <textarea bind:value={editStatementData.notes} class="form-input form-textarea" rows="4" placeholder="OCR summary, import context, or reconciliation notes"></textarea>
      </FormGroup>
    </div>
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
  .embedded-container {
    padding: 16px;
  }

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
  .kpi-value.warning { color: #F59E0B; }

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
    grid-template-columns: minmax(520px, 0.95fr) minmax(0, 1.45fr);
    gap: 12px;
    height: calc(100vh - 300px);
    min-height: 400px;
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
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
    max-height: 320px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .candidate-option {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px;
    border-radius: 14px;
    border: 1px solid var(--border-subtle);
    background: rgba(255, 255, 255, 0.92);
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
  }

  .candidate-option input {
    margin-top: 2px;
    flex-shrink: 0;
  }

  @media (max-width: 900px) {
    .match-details,
    .candidate-list {
      grid-template-columns: 1fr;
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

  .form-note {
    padding: 8px 12px;
    background: var(--surface-elevated);
    border-left: 3px solid var(--brand-indigo);
    border-radius: var(--radius-sm);
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
