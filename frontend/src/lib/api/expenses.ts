import {
  ApproveExpenseEntry,
  CreateExpenseCategory,
  CreateExpenseEntry,
  CreateExpenseFromBankCandidate,
  CreateExpenseVendor,
  CreateRecurringExpense,
  GenerateRecurringExpenses,
  ListBankExpenseCandidates,
  ListExpenseCategories,
  ListExpenseDashboardSummary,
  ListExpenseEntries,
  ListExpenseVendors,
  ListRecurringExpenses,
  MarkExpenseEntryPaid,
  PostExpenseEntry,
  RejectExpenseEntry,
  SubmitExpenseEntry,
} from "../../../wailsjs/go/main/App";

const isDesktop = () => Boolean((window as any)?.go?.main?.App);

export interface ExpenseCategory {
  id: string;
  name: string;
  code: string;
  description?: string;
  gl_account_id?: string;
  gl_account_name?: string;
  default_tax_rate?: number;
  is_active?: boolean;
}

export interface ExpenseVendor {
  id: string;
  name: string;
  contact_name?: string;
  email?: string;
  phone?: string;
  payment_terms?: string;
  tax_number?: string;
  notes?: string;
  is_active?: boolean;
}

export interface ExpenseEntry {
  id: string;
  entry_number: string;
  expense_date: string;
  due_date?: string;
  description: string;
  category_id: string;
  category_name?: string;
  vendor_id?: string;
  vendor_name?: string;
  source_type?: string;
  source_ref_id?: string;
  bank_expense_entry_id?: string;
  project_id?: string;
  customer_id?: string;
  opportunity_id?: string;
  order_id?: string;
  cost_center?: string;
  currency?: string;
  amount: number;
  vat_amount: number;
  total_amount: number;
  status: string;
  payment_status?: string;
  payment_method?: string;
  payment_reference?: string;
  bank_account_id?: string;
  notes?: string;
  approved_at?: string;
  approved_by?: string;
  posted_at?: string;
  paid_at?: string;
}

export interface RecurringExpense {
  id: string;
  name: string;
  description?: string;
  category_id: string;
  category_name?: string;
  vendor_id?: string;
  vendor_name?: string;
  frequency?: string;
  interval_value?: number;
  next_run_date: string;
  last_generated_at?: string;
  default_amount: number;
  default_vat_amount: number;
  currency?: string;
  cost_center?: string;
  project_id?: string;
  is_active?: boolean;
  auto_submit?: boolean;
}

export interface BankExpenseCandidate {
  id: string;
  bank_statement_line_id: string;
  expense_date: string;
  description: string;
  category: string;
  amount: number;
  currency?: string;
  vat_amount?: number;
  is_posted?: boolean;
}

export interface ExpenseDashboardSummary {
  total_drafts: number;
  total_submitted: number;
  total_approved_unpaid: number;
  total_recurring: number;
  month_to_date_spend: number;
  upcoming_commitments: number;
}

export async function listExpenseCategories(activeOnly = true): Promise<ExpenseCategory[]> {
  if (!isDesktop()) return [];
  return await ListExpenseCategories(activeOnly) as ExpenseCategory[];
}

export async function createExpenseCategory(category: Partial<ExpenseCategory>): Promise<ExpenseCategory> {
  return await CreateExpenseCategory(category as Record<string, any>) as ExpenseCategory;
}

export async function listExpenseVendors(activeOnly = true): Promise<ExpenseVendor[]> {
  if (!isDesktop()) return [];
  return await ListExpenseVendors(activeOnly) as ExpenseVendor[];
}

export async function createExpenseVendor(vendor: Partial<ExpenseVendor>): Promise<ExpenseVendor> {
  return await CreateExpenseVendor(vendor as Record<string, any>) as ExpenseVendor;
}

export async function listExpenseEntries(status = "", includePaid = true): Promise<ExpenseEntry[]> {
  if (!isDesktop()) return [];
  return await ListExpenseEntries(status, includePaid) as ExpenseEntry[];
}

export async function createExpenseEntry(entry: Partial<ExpenseEntry>): Promise<ExpenseEntry> {
  return await CreateExpenseEntry(entry as Record<string, any>) as ExpenseEntry;
}

export async function submitExpenseEntry(entryID: string): Promise<ExpenseEntry> {
  return await SubmitExpenseEntry(entryID) as ExpenseEntry;
}

export async function approveExpenseEntry(entryID: string, notes = ""): Promise<ExpenseEntry> {
  return await ApproveExpenseEntry(entryID, notes) as ExpenseEntry;
}

export async function rejectExpenseEntry(entryID: string, reason: string): Promise<ExpenseEntry> {
  return await RejectExpenseEntry(entryID, reason) as ExpenseEntry;
}

export async function postExpenseEntry(entryID: string): Promise<ExpenseEntry> {
  return await PostExpenseEntry(entryID) as ExpenseEntry;
}

export async function markExpenseEntryPaid(
  entryID: string,
  paidAtISO = "",
  paymentReference = "",
  bankAccountID = "",
  paymentMethod = "",
): Promise<ExpenseEntry> {
  return await MarkExpenseEntryPaid(entryID, paidAtISO, paymentReference, bankAccountID, paymentMethod) as ExpenseEntry;
}

export async function listRecurringExpenses(activeOnly = true): Promise<RecurringExpense[]> {
  if (!isDesktop()) return [];
  return await ListRecurringExpenses(activeOnly) as RecurringExpense[];
}

export async function createRecurringExpense(recurring: Partial<RecurringExpense>): Promise<RecurringExpense> {
  return await CreateRecurringExpense(recurring as Record<string, any>) as RecurringExpense;
}

export async function generateRecurringExpenses(cutoffISO = ""): Promise<ExpenseEntry[]> {
  return await GenerateRecurringExpenses(cutoffISO) as ExpenseEntry[];
}

export async function listBankExpenseCandidates(includeLinked = false): Promise<BankExpenseCandidate[]> {
  if (!isDesktop()) return [];
  return await ListBankExpenseCandidates(includeLinked) as BankExpenseCandidate[];
}

export async function createExpenseFromBankCandidate(bankExpenseID: string, categoryID = ""): Promise<ExpenseEntry> {
  return await CreateExpenseFromBankCandidate(bankExpenseID, categoryID) as ExpenseEntry;
}

export async function getExpenseDashboardSummary(): Promise<ExpenseDashboardSummary> {
  if (!isDesktop()) {
    return {
      total_drafts: 0,
      total_submitted: 0,
      total_approved_unpaid: 0,
      total_recurring: 0,
      month_to_date_spend: 0,
      upcoming_commitments: 0,
    };
  }
  return await ListExpenseDashboardSummary() as ExpenseDashboardSummary;
}
