import {
  ApprovePayrollRun,
  CreatePayrollPeriod,
  GeneratePayrollRun,
  GetPayrollRun,
  ListEmployeeCompensationProfiles,
  ListPayrollDashboardSummary,
  ListPayrollPayouts,
  ListPayrollPeriods,
  ListPayrollRuns,
  MarkPayrollRunPaid,
  PostPayrollRun,
  UpsertEmployeeCompensationProfile,
} from "../../../wailsjs/go/main/App";

const isDesktop = () => Boolean((window as any)?.go?.main?.App);

export interface EmployeeCompensationProfile {
  id: string;
  employee_id: string;
  division?: string;
  employee_name?: string;
  job_title?: string;
  pay_frequency?: string;
  currency?: string;
  base_salary: number;
  housing_allowance: number;
  transport_allowance: number;
  other_allowance: number;
  standard_deduction: number;
  tax_deduction: number;
  employer_cost: number;
  effective_from?: string;
  effective_to?: string;
  is_active?: boolean;
  notes?: string;
}

export interface PayrollPeriod {
  id: string;
  name: string;
  division?: string;
  period_start: string;
  period_end: string;
  payment_date?: string;
  status?: string;
  notes?: string;
}

export interface PayrollComponent {
  id: string;
  payroll_run_item_id: string;
  component_type: string;
  code: string;
  name: string;
  amount: number;
}

export interface PayrollRunItem {
  id: string;
  payroll_run_id: string;
  employee_id: string;
  employee_name?: string;
  employee_name_snapshot?: string;
  job_title_snapshot?: string;
  base_salary: number;
  allowances_total: number;
  deductions_total: number;
  employer_cost_total: number;
  gross_pay: number;
  net_pay: number;
  status?: string;
  payout_id?: string;
  payout_status?: string;
  payout_paid_at?: string;
  components?: PayrollComponent[];
}

export interface PayrollPayout {
  id: string;
  payroll_run_id: string;
  payroll_run_item_id: string;
  employee_id: string;
  division?: string;
  employee_name?: string;
  run_number?: string;
  scheduled_at?: string;
  paid_at?: string;
  amount: number;
  currency?: string;
  status?: string;
  payment_reference?: string;
  bank_account_id?: string;
  bank_statement_line_id?: string;
}

export interface PayrollRun {
  id: string;
  run_number: string;
  payroll_period_id: string;
  division?: string;
  period_name?: string;
  status: string;
  generated_at?: string;
  approved_at?: string;
  posted_at?: string;
  paid_at?: string;
  payment_reference?: string;
  bank_account_id?: string;
  journal_entry_id?: string;
  payout_journal_entry_id?: string;
  total_employees: number;
  gross_total: number;
  deductions_total: number;
  net_total: number;
  employer_cost_total: number;
  currency?: string;
  notes?: string;
  items?: PayrollRunItem[];
  payouts?: PayrollPayout[];
}

export interface PayrollDashboardSummary {
  active_profiles: number;
  open_periods: number;
  draft_runs: number;
  approved_unpaid_runs: number;
  month_to_date_net_payroll: number;
  upcoming_payroll_liability: number;
}

export async function listEmployeeCompensationProfiles(activeOnly = true): Promise<EmployeeCompensationProfile[]> {
  if (!isDesktop()) return [];
  return await ListEmployeeCompensationProfiles(activeOnly) as EmployeeCompensationProfile[];
}

export async function upsertEmployeeCompensationProfile(
  profile: Partial<EmployeeCompensationProfile>,
): Promise<EmployeeCompensationProfile> {
  return await UpsertEmployeeCompensationProfile(profile as Record<string, any>) as EmployeeCompensationProfile;
}

export async function listPayrollPeriods(includeClosed = true): Promise<PayrollPeriod[]> {
  if (!isDesktop()) return [];
  return await ListPayrollPeriods(includeClosed) as PayrollPeriod[];
}

export async function createPayrollPeriod(period: Partial<PayrollPeriod>): Promise<PayrollPeriod> {
  return await CreatePayrollPeriod(period as Record<string, any>) as PayrollPeriod;
}

export async function listPayrollRuns(payrollPeriodID = ""): Promise<PayrollRun[]> {
  if (!isDesktop()) return [];
  return await ListPayrollRuns(payrollPeriodID) as PayrollRun[];
}

export async function getPayrollRun(runID: string): Promise<PayrollRun> {
  return await GetPayrollRun(runID) as PayrollRun;
}

export async function generatePayrollRun(payrollPeriodID: string): Promise<PayrollRun> {
  return await GeneratePayrollRun(payrollPeriodID) as PayrollRun;
}

export async function approvePayrollRun(runID: string, notes = ""): Promise<PayrollRun> {
  return await ApprovePayrollRun(runID, notes) as PayrollRun;
}

export async function postPayrollRun(runID: string): Promise<PayrollRun> {
  return await PostPayrollRun(runID) as PayrollRun;
}

export async function markPayrollRunPaid(
  runID: string,
  paidAtISO = "",
  paymentReference = "",
  bankAccountID = "",
): Promise<PayrollRun> {
  return await MarkPayrollRunPaid(runID, paidAtISO, paymentReference, bankAccountID) as PayrollRun;
}

export async function listPayrollPayouts(payrollRunID = ""): Promise<PayrollPayout[]> {
  if (!isDesktop()) return [];
  return await ListPayrollPayouts(payrollRunID) as PayrollPayout[];
}

export async function getPayrollDashboardSummary(): Promise<PayrollDashboardSummary> {
  if (!isDesktop()) {
    return {
      active_profiles: 0,
      open_periods: 0,
      draft_runs: 0,
      approved_unpaid_runs: 0,
      month_to_date_net_payroll: 0,
      upcoming_payroll_liability: 0,
    };
  }
  return await ListPayrollDashboardSummary() as PayrollDashboardSummary;
}
