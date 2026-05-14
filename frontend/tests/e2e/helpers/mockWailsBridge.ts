import type { Page } from '@playwright/test';

type MockWailsBridgeOptions = {
  butlerResponse?: string;
  notificationReadDelayMs?: number;
  collaborativeDataReadyAfterRefreshCount?: number;
  taskDetailsReadyAfterRefreshCount?: number;
  projectContextDelayMs?: number;
  currentEmployeeRole?: string;
};

export async function installMockWailsBridge(page: Page, options: MockWailsBridgeOptions = {}) {
  await page.addInitScript((opts: MockWailsBridgeOptions) => {
    const CONVERSATION_STORE_ID = '__ph_e2e_butler_conversations__';
    const MESSAGE_STORE_ID = '__ph_e2e_butler_messages__';
    const ACTIVE_CONVERSATION_KEY = 'butler_active_conversation_id';
    const WORKFLOW_STATE_KEY = '__ph_e2e_workflow_state__';
    (window as any).__wailsFileDropCallbacks = [];
    (window as any).__wailsOCRCalls = [];
    (window as any).__wailsDocumentSaveCalls = [];

    const now = () => new Date().toISOString();
    const safeJson = (value: string | null, fallback: any) => {
      if (!value) return fallback;
      try {
        return JSON.parse(value);
      } catch {
        return fallback;
      }
    };
    const readConversations = () => safeJson(window.localStorage.getItem(CONVERSATION_STORE_ID), []);
    const writeConversations = (value: any[]) => {
      window.localStorage.setItem(CONVERSATION_STORE_ID, JSON.stringify(value));
    };
    const readMessages = () => safeJson(window.localStorage.getItem(MESSAGE_STORE_ID), {});
    const writeMessages = (value: Record<string, any[]>) => {
      window.localStorage.setItem(MESSAGE_STORE_ID, JSON.stringify(value));
    };
    const defaultWorkflowState = () => ({
      customers: [
        {
          id: 'cust-1',
          business_name: 'BAPCO',
          customer_id: 'C01011',
          customer_type: 'Government',
          payment_grade: 'A',
          city: 'Awali',
          industry: 'Energy',
          trn: '200010357800002',
          relation_years: 15,
          active_invoices: 4,
          outstanding_bhd: 1200,
          total_orders_value: 45000,
          total_orders_count: 12,
          avg_payment_days: 21,
          contacts: [
            {
              contact_name: 'Amina Yusuf',
              job_title: 'Procurement Manager',
              email: 'amina@bapco.bh',
              phone: '+973 1700 1000',
              is_primary_contact: true,
            },
          ],
          notes: [
            {
              note_type: 'general',
              content: 'Existing relationship note for regression coverage.',
              created_at: now(),
            },
          ],
          recent_orders: [
            { order_number: 'ORD-2026-001', status: 'In Progress', created_at: now(), grand_total_bhd: 45000 },
          ],
          recent_invoices: [
            { invoice_number: 'INV-2026-001', status: 'Sent', created_at: now(), grand_total_bhd: 12000 },
          ],
          recent_rfqs: [
            { project: 'Refinery Expansion Phase 4', status: 'Quoted', created_at: now(), value: 25000 },
          ],
          rfqs_floated: 8,
          rfqs_won: 5,
          win_rate: 62.5,
        },
        {
          id: 'cust-2',
          business_name: 'SEAPEAK Bahrain',
          customer_id: 'C01012',
          customer_type: 'Corporate',
          payment_grade: 'B',
          city: 'Hidd',
          industry: 'Marine',
          trn: '200010357800003',
          relation_years: 8,
          active_invoices: 2,
          outstanding_bhd: 800,
          total_orders_value: 18000,
          total_orders_count: 6,
          avg_payment_days: 34,
          contacts: [],
          notes: [],
          recent_orders: [],
          recent_invoices: [],
          recent_rfqs: [],
          rfqs_floated: 3,
          rfqs_won: 1,
          win_rate: 33.3,
        },
        {
          id: 'cust-3',
          business_name: 'ACTIUM PG',
          customer_id: 'C01013',
          customer_type: 'Corporate',
          payment_grade: 'C',
          city: 'Manama',
          industry: 'Industrial',
          trn: '200010357800004',
          relation_years: 3,
          active_invoices: 1,
          outstanding_bhd: 0,
          total_orders_value: 3463,
          total_orders_count: 2,
          avg_payment_days: 47,
          contacts: [],
          notes: [],
          recent_orders: [],
          recent_invoices: [],
          recent_rfqs: [],
          rfqs_floated: 2,
          rfqs_won: 1,
          win_rate: 50,
        },
      ],
      suppliers: [
        { id: 'sup-1', supplier_name: 'Endress+Hauser', supplier_code: 'SUP001' },
        { id: 'sup-2', supplier_name: 'Servomex', supplier_code: 'SUP002' },
      ],
      rfqs: [
        {
          id: 1,
          client: 'BAPCO',
          customer: 'BAPCO',
          project: 'Refinery Expansion Phase 4',
          project_name: 'Refinery Expansion Phase 4',
          status: 'New',
          stage: 'New',
          value: 25000,
          created_at: now(),
          rfq_number: 'RFQ-2026-001',
        },
      ],
      pipeline: [
        {
          id: 'pipe-1',
          customer_name: 'ACTIUM PG',
          customer: 'ACTIUM PG',
          client: 'ACTIUM PG',
          title: 'Gas Analyzer Package',
          folder_number: '2026-311',
          revenue_bhd: 3463,
          stage: 'Qualified',
          year: 2026,
          comment: 'Initial technical scope captured from OCR.',
          owner_notes: 'Prioritize management review before pricing.',
          product_details: JSON.stringify([
            {
              description: 'Gas analyzer transmitter',
              quantity: 2,
              unit_price: 540.758,
              total_price: 1081.516,
              equipment: 'Analyzer transmitter',
              specification: '4-20mA with HART',
            },
            {
              description: 'Sample conditioning panel',
              quantity: 1,
              unit_price: 1299.968,
              total_price: 1299.968,
              equipment: 'Conditioning panel',
              specification: 'SS316 enclosure',
            },
          ]),
        },
      ],
      offers: [
        {
          id: 'offer-1',
          offer_number: 'PHO-2026-001',
          customer_name: 'BAPCO',
          project_name: 'Refinery Expansion Phase 4',
          folder_number: '43-26',
          payment_terms: 'Net 30',
          delivery_terms: 'DAP Bahrain',
          customer_reference: 'RFQ-2026-001',
          attention_person: 'Amina Yusuf',
          attention_company: 'BAPCO',
          attention_phone: '+973 1700 1000',
          issued_by: 'Peri',
          stage: 'Quoted',
          quotation_date: now(),
          validity_date: now(),
          total_value_bhd: 25000,
          created_at: now(),
          items: [{ description: 'Pressure transmitter', quantity: 2, unit_price: 1250, total_price: 2500 }],
        },
      ],
      orders: [
        {
          id: 'order-1',
          order_number: 'ORD-2026-001',
          customer_name: 'BAPCO',
          status: 'In Progress',
          total_amount: 45000,
          date: now(),
        },
      ],
      supplierInvoices: [
        {
          id: 'supp-inv-1',
          supplier_id: 'sup-1',
          supplier_name: 'Endress+Hauser',
          invoice_number: '6017300440',
          invoice_date: '2026-01-24',
          due_date: '2026-02-23',
          currency: 'BHD',
          exchange_rate: 1,
          subtotal_foreign: 5788.085,
          subtotal_bhd: 5788.085,
          vat_foreign: 578.809,
          vat_bhd: 578.809,
          total_foreign: 6366.894,
          total_bhd: 6366.894,
          status: 'Approved',
          payment_status: 'Unpaid',
          payment_method: '',
          payment_ref: '',
          match_status: 'Pending',
        },
      ],
      supplierPayments: [],
      customerInvoices: [
        {
          id: 'inv-1',
          invoice_number: 'INV-2026-001',
          customer_id: 'cust-1',
          customer_name: 'BAPCO',
          invoice_date: '2026-03-10',
          due_date: '2026-04-09',
          subtotal_bhd: 500,
          grand_total_bhd: 500,
          outstanding_bhd: 500,
          total_supplier_cost_bhd: 300,
          status: 'Sent',
          division: 'PH Trading',
        },
      ],
      payments: [],
      customerReceipts: [
        {
          id: 'receipt-1',
          receipt_number: 'RCT-2026-0001',
          customer_id: 'cust-1',
          customer_name: 'BAPCO',
          receipt_date: '2026-03-22',
          amount_bhd: 500,
          applied_amount_bhd: 0,
          unapplied_amount_bhd: 500,
          payment_method: 'Bank Transfer',
          reference: 'RCT-500',
          status: 'OnAccount',
          division: 'PH Trading',
          created_at: now(),
        },
      ],
      expenseEntries: [
        {
          id: 'exp-1',
          entry_number: 'EXP-2026-0001',
          expense_date: '2026-03-12',
          description: 'Office supplies regression expense',
          category_id: 'cat-office',
          category_name: 'Office Supplies',
          amount: 75,
          total_amount: 75,
          currency: 'BHD',
          status: 'approved',
          payment_status: 'unpaid',
          division: 'PH Trading',
        },
        {
          id: 'exp-locked',
          entry_number: 'EXP-2026-LOCKED',
          expense_date: '2026-03-13',
          description: 'Posted locked expense',
          category_id: 'cat-office',
          category_name: 'Office Supplies',
          amount: 40,
          total_amount: 40,
          currency: 'BHD',
          status: 'posted',
          payment_status: 'paid',
          division: 'PH Trading',
        },
      ],
      expenseCategories: [
        { id: 'cat-office', name: 'Office Supplies', code: 'OFFICE', is_active: true, default_tax_rate: 0 },
      ],
      expenseVendors: [],
      recurringExpenses: [],
      bankExpenseCandidates: [],
      journalEntries: [],
      bankAccounts: [
        {
          id: 'bank-1',
          bank_name: 'National Bank of Bahrain',
          account_name: 'NBB Operating',
          account_number: '99570076',
          currency: 'BHD',
          is_active: true,
        },
      ],
      bankStatements: [
        {
          id: 'stmt-1',
          bank_account_id: 'bank-1',
          statement_number: 'NBB-2026-03',
          period_start: '2026-03-01',
          period_end: '2026-03-31',
          opening_balance: 1000,
          closing_balance: 1500,
          total_debits: 6366.894,
          total_credits: 0,
          status: 'Imported',
        },
      ],
      bankStatementLines: [
        {
          id: 'line-1',
          bank_statement_id: 'stmt-1',
          line_number: 1,
          transaction_date: '2026-03-20',
          description: 'Supplier payment - Endress+Hauser',
          reference: 'TXN-1001',
          debit: 6366.894,
          credit: 0,
          balance: -5366.894,
          transaction_type: 'Supplier Payment',
          is_matched: false,
          match_type: 'Unmatched',
          match_confidence: 0,
          extracted_customer: '',
          extracted_invoices: '',
        },
        {
          id: 'line-2',
          bank_statement_id: 'stmt-1',
          line_number: 2,
          transaction_date: '2026-03-22',
          description: 'Customer receipt - BAPCO',
          reference: 'RCT-500',
          debit: 0,
          credit: 500,
          balance: -4866.894,
          transaction_type: 'Customer Receipt',
          is_matched: false,
          match_type: 'Unmatched',
          match_confidence: 0,
          extracted_customer: 'BAPCO',
          extracted_invoices: 'INV-2026-001',
        },
      ],
      employeeProfiles: [
        {
          id: 'emp-admin',
          employee_code: 'EMP-001',
          full_name: 'Developer Admin',
          department: 'Management',
          job_title: 'Administrator',
          is_active: true,
        },
        {
          id: 'emp-peri',
          employee_code: 'EMP-002',
          full_name: 'Peri',
          department: 'Operations',
          job_title: 'Project Coordinator',
          is_active: true,
        },
      ],
      employeeAccessLinks: [
        {
          id: 'access-admin',
          employee_id: 'emp-admin',
          employee_name: 'Developer Admin',
          license_key: 'PH-ADM-E2E001',
          user_id: 'e2e-user',
          device_id: 'device-e2e',
          device_name: 'E2E Device',
          access_status: 'active',
          is_primary: true,
        },
      ],
      licenseKeys: [
        {
          id: 'license-admin',
          key: 'PH-ADM-E2E001',
          role: 'admin',
          display_name: 'Developer Admin',
          activated: true,
          device_hash: 'device-e2e',
        },
        {
          id: 'license-new',
          key: 'PH-STF-OPEN01',
          role: 'staff',
          display_name: 'Open Staff',
          activated: false,
        },
      ],
      employeeArchiveRequests: [],
      collaborativeProjects: [
        {
          id: 'proj-1',
          name: 'Butler Rollout',
          project_type: 'internal',
          description: 'Regression project used for E2E task routing coverage.',
          status: 'active',
        },
      ],
      collaborativeProjectMembers: [
        {
          id: 'project-member-1',
          project_id: 'proj-1',
          employee_id: 'emp-peri',
          employee_name: 'Peri',
          project_name: 'Butler Rollout',
          role: 'Member',
          allocation_percent: 100,
          is_active: true,
          joined_at: now(),
        },
      ],
      collaborativeTasks: [
        {
          id: 'task-1',
          title: 'Review salary expense forecast',
          description: 'Confirm this month\'s HR and payroll forecast.',
          task_type: 'internal',
          status: 'open',
          priority: 'high',
          project_id: 'proj-1',
          creator_employee_id: 'emp-admin',
          assignee_employee_id: 'emp-peri',
          creator_name: 'Developer Admin',
          assignee_name: 'Peri',
          due_date: '2026-04-20T09:00:00.000Z',
        },
      ],
      collaborativeTaskComments: [
        {
          id: 'task-comment-1',
          task_id: 'task-1',
          employee_id: 'emp-admin',
          employee_name: 'Developer Admin',
          body: 'Please sanity-check the payroll assumptions before sign-off.',
          created_at: now(),
        },
      ],
      opportunityComments: [],
      rfqComments: [],
      collaborativeTaskActivity: [
        {
          id: 'task-activity-1',
          task_id: 'task-1',
          employee_id: 'emp-admin',
          employee_name: 'Developer Admin',
          activity_type: 'created',
          detail: 'Task created for regression coverage.',
          metadata_json: '{}',
          created_at: now(),
        },
      ],
      notificationFeed: [
        {
          id: 'notif-task-1',
          notification_type: 'task',
          title: 'Task assigned',
          message: 'Review salary expense forecast has been assigned to you.',
          status: 'unread',
          source_type: 'task',
          source_id: 'task-1',
          action_route: 'work',
          action_payload: JSON.stringify({
            action: 'open_task',
            task_id: 'task-1',
            task_title: 'Review salary expense forecast',
            project_name: 'Butler Rollout',
            actor_name: 'Developer Admin',
          }),
          created_at: now(),
          read_at: null,
          employee_id: 'emp-peri',
        },
      ],
      collaborativeWorkspaceRefreshCount: 0,
      purchaseOrders: [
        {
          id: 'po-1',
          po_number: 'PO-2026-007',
          order_id: 'order-1',
          supplier_id: 'sup-1',
          supplier_name: 'Endress+Hauser',
          po_date: '2026-03-28',
          expected_delivery: '2026-05-09',
          currency: 'EUR',
          exchange_rate: 0.41,
          subtotal_foreign: 6200,
          subtotal_bhd: 2542,
          vat_amount: 620,
          total_foreign: 6820,
          total_bhd: 2796.2,
          payment_terms: 'Net 30',
          status: 'Draft',
          items: [
            {
              id: 'po-item-1',
              purchase_order_id: 'po-1',
              description: 'Temperature transmitter',
              quantity: 4,
              unit_price_foreign: 850,
              total_foreign: 3400,
              total_bhd: 1394,
            },
            {
              id: 'po-item-2',
              purchase_order_id: 'po-1',
              description: 'Signal isolator',
              quantity: 4,
              unit_price_foreign: 700,
              total_foreign: 2800,
              total_bhd: 1148,
            },
          ],
        },
      ],
      costingSheets: [],
      nextRFQId: 2,
      nextOfferId: 2,
      nextReceiptId: 2,
      nextPaymentId: 1,
      nextPOId: 8,
    });
    const readWorkflowState = () => safeJson(window.localStorage.getItem(WORKFLOW_STATE_KEY), defaultWorkflowState());
    const writeWorkflowState = (value: any) => {
      window.localStorage.setItem(WORKFLOW_STATE_KEY, JSON.stringify(value));
    };
    const collaborativeDataReady = (state: any) =>
      Number(state.collaborativeWorkspaceRefreshCount || 0) >= Number(opts.collaborativeDataReadyAfterRefreshCount ?? 2);
    const taskDetailsReady = (state: any) =>
      Number(state.collaborativeWorkspaceRefreshCount || 0) >= Number(opts.taskDetailsReadyAfterRefreshCount ?? 3);
    const buildCustomerDashboard = (state: any) => {
      const customers = state.customers || [];
      const topCustomers = [...customers]
        .sort((a: any, b: any) => Number(b.total_orders_value || 0) - Number(a.total_orders_value || 0))
        .slice(0, 10)
        .map((customer: any) => ({
          id: customer.id,
          business_name: customer.business_name,
          total_revenue: Number(customer.total_orders_value || 0),
        }));

      const totalRevenue = customers.reduce((sum: number, customer: any) => sum + Number(customer.total_orders_value || 0), 0);
      const totalOutstanding = customers.reduce((sum: number, customer: any) => sum + Number(customer.outstanding_bhd || 0), 0);
      const overdueAmount = customers.reduce((sum: number, customer: any) => sum + Number(customer.overdue_bhd || 0), 0);
      const activeCustomers = customers.length;
      const top3Revenue = topCustomers.slice(0, 3).reduce((sum: number, customer: any) => sum + Number(customer.total_revenue || 0), 0);
      const top5Revenue = topCustomers.slice(0, 5).reduce((sum: number, customer: any) => sum + Number(customer.total_revenue || 0), 0);
      const top10Revenue = topCustomers.slice(0, 10).reduce((sum: number, customer: any) => sum + Number(customer.total_revenue || 0), 0);
      const countGrade = (grade: string) => customers.filter((customer: any) => customer.payment_grade === grade);
      const revenueFor = (grade: string) =>
        countGrade(grade).reduce((sum: number, customer: any) => sum + Number(customer.total_orders_value || 0), 0);

      return {
        total_customers: activeCustomers,
        active_customers: activeCustomers,
        total_revenue: totalRevenue,
        revenue_yoy: 12.4,
        total_outstanding: totalOutstanding,
        overdue_amount: overdueAmount,
        overdue_pct: totalOutstanding > 0 ? (overdueAmount / totalOutstanding) * 100 : 0,
        top_customers: topCustomers,
        top3_revenue_pct: totalRevenue > 0 ? (top3Revenue / totalRevenue) * 100 : 0,
        top5_revenue_pct: totalRevenue > 0 ? (top5Revenue / totalRevenue) * 100 : 0,
        top10_revenue_pct: totalRevenue > 0 ? (top10Revenue / totalRevenue) * 100 : 0,
        grade_a_count: countGrade('A').length,
        grade_b_count: countGrade('B').length,
        grade_c_count: countGrade('C').length,
        grade_d_count: countGrade('D').length,
        grade_a_revenue: revenueFor('A'),
        grade_b_revenue: revenueFor('B'),
        grade_c_revenue: revenueFor('C'),
        grade_d_revenue: revenueFor('D'),
        customers,
      };
    };
    const findCustomer = (state: any, customerId: string) =>
      state.customers.find((item: any) => String(item.id) === String(customerId) || String(item.customer_id) === String(customerId));
    const findPipelineOpportunity = (state: any, opportunityId: string) =>
      (state.pipeline || []).find((item: any) => String(item.id) === String(opportunityId));
    const findSupplierInvoice = (state: any, invoiceId: string) =>
      (state.supplierInvoices || []).find((item: any) => String(item.id) === String(invoiceId));
    const deriveSupplierPayments = (state: any) => {
      const explicitPayments = [...(state.supplierPayments || [])];
      const keyed = new Set(explicitPayments.map((item: any) => String(item.supplier_invoice_id || item.invoice_number || item.id)));

      (state.supplierInvoices || []).forEach((invoice: any) => {
        if (invoice.payment_status !== 'Paid') return;
        const key = String(invoice.id || invoice.invoice_number || '');
        if (keyed.has(key)) return;
        explicitPayments.push({
          id: `derived-${invoice.id}`,
          supplier_invoice_id: invoice.id,
          supplier_id: invoice.supplier_id,
          supplier_name: invoice.supplier_name,
          invoice_number: invoice.invoice_number,
          amount_foreign: Number(invoice.total_foreign || 0),
          amount_bhd: Number(invoice.total_bhd || 0),
          currency: invoice.currency || 'BHD',
          exchange_rate: Number(invoice.exchange_rate || 1),
          payment_date: invoice.payment_date || now(),
          payment_method: invoice.payment_method || 'Bank Transfer',
          reference: invoice.payment_ref || invoice.invoice_number || 'PAYMENT',
        });
      });

      return explicitPayments;
    };
    const deriveCustomerPayments = (state: any) => state.payments || [];
    const withinRange = (dateValue: any, start?: string, end?: string) => {
      const date = String(dateValue || '').slice(0, 10);
      if (!date) return false;
      if (start && date < String(start).slice(0, 10)) return false;
      if (end && date > String(end).slice(0, 10)) return false;
      return true;
    };
    const matchesDivision = (item: any, division?: string) => !division || String(item?.division || 'PH Trading') === String(division);
    const buildOperationalLedger = (state: any, filter: any = {}) => {
      const start = filter.start_date || '2026-01-01';
      const end = filter.end_date || '2026-12-31';
      const sourceType = String(filter.source_type || '');
      const includeSource = (source: string) => !sourceType || sourceType === source;
      const rows: any[] = [];

      if (includeSource('CUSTOMER_INVOICE')) {
        (state.customerInvoices || []).forEach((invoice: any) => {
          if (!withinRange(invoice.invoice_date || invoice.created_at || now(), start, end) || !matchesDivision(invoice, filter.division)) return;
          if (filter.customer_id && String(invoice.customer_id || '') !== String(filter.customer_id)) return;
          rows.push({
            id: `invoice:${invoice.id}`,
            source_type: 'CUSTOMER_INVOICE',
            source_id: invoice.id,
            source_number: invoice.invoice_number,
            entry_date: String(invoice.invoice_date || now()).slice(0, 10),
            party_id: invoice.customer_id,
            party_name: invoice.customer_name,
            party_type: 'Customer',
            description: 'Customer invoice',
            debit_bhd: Number(invoice.grand_total_bhd || 0),
            credit_bhd: 0,
            status: invoice.status || 'Sent',
            division: invoice.division || 'PH Trading',
          });
        });
      }
      if (includeSource('CUSTOMER_RECEIPT')) {
        (state.customerReceipts || []).forEach((receipt: any) => {
          if (!withinRange(receipt.receipt_date || now(), start, end) || !matchesDivision(receipt, filter.division)) return;
          if (filter.customer_id && String(receipt.customer_id || '') !== String(filter.customer_id)) return;
          rows.push({
            id: `receipt:${receipt.id}`,
            source_type: 'CUSTOMER_RECEIPT',
            source_id: receipt.id,
            source_number: receipt.receipt_number,
            entry_date: String(receipt.receipt_date || now()).slice(0, 10),
            party_id: receipt.customer_id,
            party_name: receipt.customer_name,
            party_type: 'Customer',
            description: Number(receipt.unapplied_amount_bhd || 0) > 0 ? 'Customer receipt - on account' : 'Customer receipt',
            debit_bhd: 0,
            credit_bhd: Number(receipt.amount_bhd || 0),
            status: receipt.status || 'Applied',
            division: receipt.division || 'PH Trading',
          });
        });
      }
      if (includeSource('CUSTOMER_PAYMENT')) {
        deriveCustomerPayments(state).filter((payment: any) => !payment.receipt_id).forEach((payment: any) => {
          if (!withinRange(payment.payment_date || now(), start, end) || !matchesDivision(payment, filter.division)) return;
          const invoice = (state.customerInvoices || []).find((item: any) => String(item.id) === String(payment.invoice_id));
          if (filter.customer_id && String(invoice?.customer_id || '') !== String(filter.customer_id)) return;
          rows.push({
            id: `payment:${payment.id}`,
            source_type: 'CUSTOMER_PAYMENT',
            source_id: payment.id,
            source_number: payment.reference || payment.invoice_number,
            entry_date: String(payment.payment_date || now()).slice(0, 10),
            party_id: invoice?.customer_id || '',
            party_name: invoice?.customer_name || payment.customer_name || 'Customer',
            party_type: 'Customer',
            description: 'Customer payment',
            debit_bhd: 0,
            credit_bhd: Number(payment.amount_bhd || 0),
            status: payment.status || 'Received',
            division: payment.division || invoice?.division || 'PH Trading',
          });
        });
      }
      if (includeSource('SUPPLIER_INVOICE')) {
        (state.supplierInvoices || []).forEach((invoice: any) => {
          if (!withinRange(invoice.invoice_date || now(), start, end) || !matchesDivision(invoice, filter.division)) return;
          if (filter.supplier_id && String(invoice.supplier_id || '') !== String(filter.supplier_id)) return;
          rows.push({
            id: `supplier_invoice:${invoice.id}`,
            source_type: 'SUPPLIER_INVOICE',
            source_id: invoice.id,
            source_number: invoice.invoice_number,
            entry_date: String(invoice.invoice_date || now()).slice(0, 10),
            party_id: invoice.supplier_id,
            party_name: invoice.supplier_name,
            party_type: 'Supplier',
            description: 'Supplier invoice',
            debit_bhd: 0,
            credit_bhd: Number(invoice.total_bhd || 0),
            status: invoice.payment_status || invoice.status,
            division: invoice.division || 'PH Trading',
          });
        });
      }
      if (includeSource('SUPPLIER_PAYMENT')) {
        deriveSupplierPayments(state).forEach((payment: any) => {
          if (!withinRange(payment.payment_date || now(), start, end) || !matchesDivision(payment, filter.division)) return;
          if (filter.supplier_id && String(payment.supplier_id || '') !== String(filter.supplier_id)) return;
          rows.push({
            id: `supplier_payment:${payment.id}`,
            source_type: 'SUPPLIER_PAYMENT',
            source_id: payment.id,
            source_number: payment.reference || payment.invoice_number,
            entry_date: String(payment.payment_date || now()).slice(0, 10),
            party_id: payment.supplier_id,
            party_name: payment.supplier_name,
            party_type: 'Supplier',
            description: 'Supplier settlement',
            debit_bhd: Number(payment.amount_bhd || 0),
            credit_bhd: 0,
            status: 'Paid',
            division: payment.division || 'PH Trading',
          });
        });
      }
      if (includeSource('EXPENSE')) {
        (state.expenseEntries || []).forEach((expense: any) => {
          if (!withinRange(expense.expense_date || now(), start, end) || !matchesDivision(expense, filter.division)) return;
          if (!['approved', 'posted', 'paid'].includes(String(expense.status || '').toLowerCase())) return;
          rows.push({
            id: `expense:${expense.id}`,
            source_type: 'EXPENSE',
            source_id: expense.id,
            source_number: expense.entry_number,
            entry_date: String(expense.expense_date || now()).slice(0, 10),
            party_id: expense.vendor_id || expense.category_id,
            party_name: expense.vendor_name || expense.category_name || 'Expense',
            party_type: 'Expense',
            description: expense.description,
            debit_bhd: Number(expense.total_amount || 0),
            credit_bhd: 0,
            status: expense.payment_status || expense.status,
            division: expense.division || 'PH Trading',
          });
        });
      }

      rows.sort((a, b) => String(a.entry_date).localeCompare(String(b.entry_date)) || String(a.source_number).localeCompare(String(b.source_number)));
      let balance = 0;
      rows.forEach((row) => {
        balance += Number(row.debit_bhd || 0) - Number(row.credit_bhd || 0);
        row.balance_bhd = Math.round(balance * 1000) / 1000;
      });
      return rows;
    };
    const buildProfitLoss = (state: any, filter: any = {}) => {
      const start = filter.start_date || '2026-01-01';
      const end = filter.end_date || '2026-12-31';
      const invoices = (state.customerInvoices || []).filter((invoice: any) =>
        withinRange(invoice.invoice_date || now(), start, end) &&
        matchesDivision(invoice, filter.division) &&
        ['Sent', 'Paid', 'PartiallyPaid', 'Overdue'].includes(String(invoice.status || 'Sent'))
      );
      const expenses = (state.expenseEntries || []).filter((expense: any) =>
        withinRange(expense.expense_date || now(), start, end) &&
        matchesDivision(expense, filter.division) &&
        ['approved', 'posted', 'paid'].includes(String(expense.status || '').toLowerCase())
      );
      const revenue = invoices.reduce((sum: number, invoice: any) => sum + Number(invoice.subtotal_bhd || invoice.grand_total_bhd || 0), 0);
      const cogs = invoices.reduce((sum: number, invoice: any) => sum + Number(invoice.total_supplier_cost_bhd || 0), 0);
      const expenseTotal = expenses.reduce((sum: number, expense: any) => sum + Number(expense.total_amount || 0), 0);
      return {
        start_date: start,
        end_date: end,
        division: filter.division || 'PH Trading',
        revenue_bhd: revenue,
        cogs_bhd: cogs,
        gross_profit_bhd: revenue - cogs,
        gross_margin_percent: revenue > 0 ? ((revenue - cogs) / revenue) * 100 : 0,
        expenses_bhd: expenseTotal,
        net_income_bhd: revenue - cogs - expenseTotal,
        net_margin_percent: revenue > 0 ? ((revenue - cogs - expenseTotal) / revenue) * 100 : 0,
        invoice_count: invoices.length,
        expense_count: expenses.length,
        expense_breakdown: [{ account_name: 'Office Supplies', balance: expenseTotal }],
      };
    };
    const buildBalanceSheet = (state: any, asOfDate?: string, division?: string) => {
      const asOf = asOfDate || '2026-12-31';
      const receipts = (state.customerReceipts || []).filter((receipt: any) => withinRange(receipt.receipt_date || now(), undefined, asOf) && matchesDivision(receipt, division));
      const payments = deriveCustomerPayments(state).filter((payment: any) => !payment.receipt_id && withinRange(payment.payment_date || now(), undefined, asOf) && matchesDivision(payment, division));
      const invoices = (state.customerInvoices || []).filter((invoice: any) => withinRange(invoice.invoice_date || now(), undefined, asOf) && matchesDivision(invoice, division));
      const expenses = (state.expenseEntries || []).filter((expense: any) => withinRange(expense.expense_date || now(), undefined, asOf) && matchesDivision(expense, division));
      const cash = receipts.reduce((sum: number, receipt: any) => sum + Number(receipt.amount_bhd || 0), 0) + payments.reduce((sum: number, payment: any) => sum + Number(payment.amount_bhd || 0), 0);
      const ar = invoices.reduce((sum: number, invoice: any) => sum + Number(invoice.outstanding_bhd || 0), 0);
      const credits = receipts.reduce((sum: number, receipt: any) => sum + Number(receipt.unapplied_amount_bhd || 0), 0);
      const expenseLiability = expenses.filter((expense: any) => String(expense.payment_status || '').toLowerCase() !== 'paid').reduce((sum: number, expense: any) => sum + Number(expense.total_amount || 0), 0);
      return {
        as_of_date: asOf,
        division: division || 'PH Trading',
        cash_bhd: cash,
        accounts_receivable_bhd: ar,
        customer_credits_bhd: credits,
        accounts_payable_bhd: 0,
        expense_liability_bhd: expenseLiability,
        net_position_bhd: cash + ar - credits - expenseLiability,
      };
    };
    const buildCustomerProfile = (customer: any) => {
      if (!customer) return null;
      return {
        ...customer,
        address_line1: customer.address_line1 || 'Building 12, Road 45',
        country: customer.country || 'Bahrain',
        contacts: customer.contacts || [],
        notes: customer.notes || [],
        recent_orders: customer.recent_orders || [],
        recent_invoices: customer.recent_invoices || [],
        recent_rfqs: customer.recent_rfqs || [],
        rfqs_floated: customer.rfqs_floated || 0,
        rfqs_won: customer.rfqs_won || 0,
        win_rate: customer.win_rate || 0,
      };
    };

    const mockButlerResponse = opts.butlerResponse || 'Mock Butler response persisted.';
    const mockOCRResult = {
      documentType: 'BankStatement',
      document_type: 'BankStatement',
      confidence: 0.96,
      engine: 'mock-ocr',
      processingTimeMS: 41,
      processing_time_ms: 41,
      text: [
        'BANK STATEMENT',
        'Ahli United Bank',
        'Opening Balance 1,000.000',
        '02/03/2026 Deposit 500.000 1,500.000',
        'Closing Balance 1,500.000',
      ].join('\n'),
      extractedData: {
        bank_account_id: 'bank-1',
        bank_name: 'National Bank of Bahrain',
        account_number: '99570076',
        opening_balance: '1,000.000',
        closing_balance: '1,500.000',
        period_start: '2026-03-01',
        period_end: '2026-03-31',
        line_items: [],
      },
      extracted_data: {
        bank_account_id: 'bank-1',
        bank_name: 'National Bank of Bahrain',
        account_number: '99570076',
        opening_balance: '1,000.000',
        closing_balance: '1,500.000',
        period_start: '2026-03-01',
        period_end: '2026-03-31',
        line_items: [],
      },
      line_items: [],
    };

    const mockButlerInsight = {
      summary: 'Mock Butler analysis: one credit transaction and a clean balance trail.',
      confidence: 0.97,
      detected_customer: 'PH Trading',
      detected_project: 'Bank reconciliation smoke test',
      extracted_items: [
        {
          date: '2026-03-02',
          description: 'Deposit',
          reference: 'DEP-001',
          debit: 0,
          credit: 500,
          balance: 1500,
        },
      ],
      metadata: {
        bank_name: 'Ahli United Bank',
        account_number: '123456789',
        opening_balance: '1,000.000',
        closing_balance: '1,500.000',
        period_start: '2026-03-01',
        period_end: '2026-03-31',
      },
      document_type: 'BankStatement',
    };

    const createConversation = (conversationId: string, message: string) => {
      const conversations = readConversations();
      const conversation = conversations.find((item: any) => String(item.id) === String(conversationId));
      if (!conversation) {
        conversations.unshift({
          id: conversationId,
          title: message.slice(0, 42) || 'Butler Chat',
          updated_at: now(),
        });
      } else {
        conversation.title = conversation.title || message.slice(0, 42) || 'Butler Chat';
        conversation.updated_at = now();
      }
      writeConversations(conversations);

      const messages = readMessages();
      const thread = Array.isArray(messages[conversationId]) ? messages[conversationId] : [];
      thread.push({
        id: `msg-user-${Date.now()}`,
        role: 'user',
        content: message,
      });
      thread.push({
        id: `msg-bot-${Date.now()}`,
        role: 'assistant',
        content: mockButlerResponse,
      });
      messages[conversationId] = thread;
      writeMessages(messages);
      window.localStorage.setItem(ACTIVE_CONVERSATION_KEY, conversationId);
    };

    if (!window.localStorage.getItem(WORKFLOW_STATE_KEY)) {
      writeWorkflowState(defaultWorkflowState());
    }

    const defaultVoid = () => Promise.resolve();
    const defaultValue = (value: any) => Promise.resolve(value);
    const delayedValue = (value: any, delayMs = 0) => new Promise((resolve) => {
      if (delayMs <= 0) {
        resolve(value);
        return;
      }
      window.setTimeout(() => resolve(value), delayMs);
    });

    const app = new Proxy({}, {
      get(_, prop) {
        return (...args: any[]) => {
          switch (String(prop)) {
            case 'ValidateLicense':
              return defaultValue({
                valid: true,
                role: 'admin',
                display_name: 'Developer',
                permissions: ['*'],
              });
            case 'NeedsSetup':
            case 'NeedsLicenseActivation':
              return defaultValue(false);
            case 'CheckDeviceStatus':
              return defaultValue({
                status: 'approved',
                user_id: 'e2e-user',
                user_name: 'Developer',
                role_name: 'admin',
                permissions: ['*'],
              });
            case 'GetDBSyncStatus':
              return defaultValue({
                configured: false,
                online: false,
                sync_enabled: false,
                last_sync: null,
              });
            case 'GetDashboardStats':
              return defaultValue({
                total_revenue: 12000,
                revenue_meta: 'Smoke test YTD 2026',
                month_growth: 1.8,
                activity_year: 2026,
                active_rfqs: readWorkflowState().rfqs.length + readWorkflowState().pipeline.length,
                active_orders: readWorkflowState().orders.length,
                outstanding_ar: 200,
                ar_days_overdue: 0,
                pending_invoices: 1,
                active_customers: readWorkflowState().customers.length,
                win_rate: 64.2,
              });
            case 'GetMonthlyRevenueByCustomer':
              return defaultValue([
                { customer_id: 'cust-1', customer_name: 'BAPCO UPSTREAM W.L.L', revenue: 275000, invoice_count: 9 },
                { customer_id: 'cust-2', customer_name: 'AL EZZEL OPERATION AND MAINTENANCE COMPANY W.L.L', revenue: 188000, invoice_count: 6 },
                { customer_id: 'cust-3', customer_name: 'SEAPEAK BAHRAIN OPERATIONS W.L.L', revenue: 74000, invoice_count: 3 },
              ]);
            case 'GetDashboardPipelineByStageYTD':
              return defaultValue([
                { stage: 'Qualified', count: 5, value: 320000, color: '#3E8F47' },
                { stage: 'Won', count: 3, value: 125000, color: '#2F8F4E' },
                { stage: 'Lost', count: 2, value: 45000, color: '#C94B4B' },
              ]);
            case 'GetDashboardARAgingReportYTD':
              return defaultValue({
                current: 0,
                days_30: 0,
                days_60: 0,
                days_90: 0,
                days_120_plus: 200,
                total: 200,
              });
            case 'ListFollowUps':
              return defaultValue([]);
            case 'GetCRMCustomerDashboard':
            case 'GetCRMCustomerDashboardByYear':
              return defaultValue(buildCustomerDashboard(readWorkflowState()));
            case 'ListCustomers':
            case 'GetAllCustomers':
              return defaultValue(readWorkflowState().customers);
            case 'GetCustomerFullProfile': {
              const state = readWorkflowState();
              const customer = findCustomer(state, String(args[0] || ''));
              if (!customer) {
                return Promise.reject(new Error('Customer not found'));
              }
              return defaultValue(buildCustomerProfile(customer));
            }
            case 'AddCustomerNote': {
              const [customerId, noteType, content] = args;
              const state = readWorkflowState();
              const customer = findCustomer(state, String(customerId || ''));
              if (!customer) {
                return Promise.reject(new Error('Customer not found'));
              }
              customer.notes = customer.notes || [];
              customer.notes.unshift({
                note_type: String(noteType || 'general'),
                content: String(content || ''),
                created_at: now(),
              });
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'AddCustomerContact': {
              const [payload] = args;
              const state = readWorkflowState();
              const customer = findCustomer(state, String(payload?.customer_id || ''));
              if (!customer) {
                return Promise.reject(new Error('Customer not found'));
              }
              customer.contacts = customer.contacts || [];
              customer.contacts.unshift({
                contact_name: payload?.contact_name || 'Unnamed Contact',
                job_title: payload?.job_title || '',
                email: payload?.email || '',
                phone: payload?.phone || '',
                address: payload?.address || '',
                is_primary_contact: Boolean(payload?.is_primary_contact),
              });
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'UpdateCustomer': {
              const [payload] = args;
              const state = readWorkflowState();
              const customer = findCustomer(state, String(payload?.id || payload?.customer_id || ''));
              if (!customer) {
                return Promise.reject(new Error('Customer not found'));
              }
              Object.assign(customer, payload || {});
              writeWorkflowState(state);
              return defaultValue(customer);
            }
            case 'DeleteCustomer': {
              const state = readWorkflowState();
              state.customers = state.customers.filter((item: any) => String(item.id) !== String(args[0]));
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'ListSuppliers':
              return defaultValue(readWorkflowState().suppliers);
            case 'GetRFQs':
              return defaultValue(readWorkflowState().rfqs);
            case 'GetRFQ': {
              const state = readWorkflowState();
              return defaultValue(state.rfqs.find((item: any) => String(item.id) === String(args[0])) || null);
            }
            case 'CreateRFQ': {
              const [customer, project, value, notes] = args;
              const state = readWorkflowState();
              const rfq = {
                id: state.nextRFQId,
                client: String(customer),
                customer: String(customer),
                project: String(project),
                project_name: String(project),
                status: 'New',
                stage: 'New',
                value: Number(value) || 0,
                notes: String(notes || ''),
                created_at: now(),
                rfq_number: `RFQ-2026-${String(state.nextRFQId).padStart(3, '0')}`,
              };
              state.nextRFQId += 1;
              state.rfqs.unshift(rfq);
              writeWorkflowState(state);
              return defaultValue(rfq);
            }
            case 'CreateRFQWithReference': {
              const [customer, project, reference, value, notes] = args;
              const state = readWorkflowState();
              const rfq = {
                id: state.nextRFQId,
                client: String(customer),
                customer: String(customer),
                project: String(project),
                project_name: String(project),
                status: 'New',
                stage: 'New',
                value: Number(value) || 0,
                notes: String(notes || ''),
                comment: String(notes || ''),
                owner_notes: String(notes || ''),
                rfq_ref: String(reference || ''),
                eh_ref: String(reference || ''),
                created_at: now(),
                rfq_number: `RFQ-2026-${String(state.nextRFQId).padStart(3, '0')}`,
              };
              state.nextRFQId += 1;
              state.rfqs.unshift(rfq);
              writeWorkflowState(state);
              return defaultValue(rfq);
            }
            case 'GetPipelineOpportunities':
              return defaultValue(readWorkflowState().pipeline);
            case 'GetOpportunityLineItems': {
              const state = readWorkflowState();
              const opportunity = findPipelineOpportunity(state, String(args[0]));
              if (!opportunity?.product_details) {
                return defaultValue([]);
              }
              try {
                return defaultValue(JSON.parse(opportunity.product_details));
              } catch {
                return defaultValue([]);
              }
            }
            case 'UpdateOpportunityDetails': {
              const [opportunityId, comment, ownerNotes] = args;
              const state = readWorkflowState();
              const opportunity = findPipelineOpportunity(state, String(opportunityId));
              if (!opportunity) {
                return Promise.reject(new Error('Opportunity not found'));
              }
              opportunity.comment = String(comment || '').trim();
              opportunity.owner_notes = String(ownerNotes || '').trim();
              writeWorkflowState(state);
              return defaultValue(opportunity);
            }
            case 'UpdateRFQNotes': {
              const [rfqId, notes] = args;
              const state = readWorkflowState();
              const rfq = state.rfqs.find((item: any) => Number(item.id) === Number(rfqId));
              if (!rfq) {
                return Promise.reject(new Error('RFQ not found'));
              }
              rfq.notes = String(notes || '').trim();
              writeWorkflowState(state);
              return defaultValue(rfq);
            }
            case 'GetCurrentUserRole':
              return defaultValue('admin');
            case 'GetCurrentEmployeeContext':
              return defaultValue({
                employee_id: 'emp-admin',
                employee_name: 'Developer Admin',
                license_key: 'PH-ADM-E2E001',
                license_role: opts.currentEmployeeRole || 'admin',
                device_id: 'device-e2e',
                user_id: 'e2e-user',
                resolved_by: 'user',
                permissions: ['*'],
              });
            case 'ListEmployeeProfiles': {
              const state = readWorkflowState();
              return defaultValue(state.employeeProfiles || []);
            }
            case 'CreateEmployeeProfile': {
              const state = readWorkflowState();
              const payload = args[0] || {};
              const employee = {
                id: `emp-created-${Date.now()}`,
                employee_code: `EMP-${String((state.employeeProfiles || []).length + 1).padStart(3, '0')}`,
                full_name: String(payload.full_name || '').trim(),
                preferred_name: String(payload.preferred_name || payload.full_name || '').trim().split(/\s+/)[0],
                email: String(payload.email || ''),
                phone: String(payload.phone || ''),
                department: String(payload.department || ''),
                job_title: String(payload.job_title || ''),
                employment_status: 'active',
                is_active: true,
                created_at: now(),
                updated_at: now(),
              };
              state.employeeProfiles = [employee, ...(state.employeeProfiles || [])];
              writeWorkflowState(state);
              return defaultValue(employee);
            }
            case 'UpdateEmployeeProfile': {
              const state = readWorkflowState();
              const payload = args[0] || {};
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(payload.id));
              if (!employee) {
                return Promise.reject(new Error('Employee not found'));
              }
              Object.assign(employee, payload, { updated_at: now() });
              writeWorkflowState(state);
              return defaultValue(employee);
            }
            case 'SetEmployeeEmploymentState': {
              const state = readWorkflowState();
              const [employeeID, isActive, status] = args;
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(employeeID));
              if (!employee) {
                return Promise.reject(new Error('Employee not found'));
              }
              employee.is_active = Boolean(isActive);
              employee.employment_status = String(status || (isActive ? 'active' : 'inactive'));
              employee.updated_at = now();
              writeWorkflowState(state);
              return defaultValue(employee);
            }
            case 'ReassignEmployeeManager': {
              const state = readWorkflowState();
              const [employeeID, managerEmployeeID] = args;
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(employeeID));
              if (!employee) {
                return Promise.reject(new Error('Employee not found'));
              }
              employee.manager_employee_id = String(managerEmployeeID || '');
              employee.manager_name = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(managerEmployeeID))?.full_name || '';
              writeWorkflowState(state);
              return defaultValue(employee);
            }
            case 'ListEmployeeAccessLinks': {
              const state = readWorkflowState();
              return defaultValue(state.employeeAccessLinks || []);
            }
            case 'ListLicenseKeys': {
              const state = readWorkflowState();
              return defaultValue(state.licenseKeys || []);
            }
            case 'CreateEmployeeAccessLink':
            case 'ReassignEmployeeLicenseAccess': {
              const state = readWorkflowState();
              const employeeID = String(args[0]?.employee_id || args[0] || '');
              const licenseKey = String(args[0]?.license_key || args[1] || '');
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === employeeID);
              if (!employee) {
                return Promise.reject(new Error('Employee not found'));
              }
              const link = {
                id: `access-${Date.now()}`,
                employee_id: employeeID,
                employee_name: employee.full_name,
                license_key: licenseKey,
                user_id: String(args[0]?.user_id || ''),
                device_id: '',
                device_name: '',
                access_status: 'active',
                is_primary: true,
              };
              state.employeeAccessLinks = [link, ...(state.employeeAccessLinks || [])];
              writeWorkflowState(state);
              return defaultValue(link);
            }
            case 'RequestEmployeeArchive': {
              const state = readWorkflowState();
              const [employeeID, reason] = args;
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(employeeID));
              if (!employee) {
                return Promise.reject(new Error('Employee not found'));
              }
              const request = {
                id: `archive-${Date.now()}`,
                employee_id: employee.id,
                employee_name: employee.full_name,
                requested_by: 'emp-admin',
                requested_by_name: 'Developer Admin',
                reason: String(reason || ''),
                status: 'approved',
                required_approvals: 1,
                first_approved_by: 'emp-admin',
                first_approved_by_name: 'Developer Admin',
                first_approved_at: now(),
                review_notes: 'Archived by admin',
                created_at: now(),
              };
              employee.is_active = false;
              employee.employment_status = 'archived';
              employee.archive_request_id = request.id;
              employee.archive_reason = request.reason;
              employee.archived_at = now();
              employee.archived_by = 'emp-admin';
              state.employeeArchiveRequests = [request, ...(state.employeeArchiveRequests || [])];
              writeWorkflowState(state);
              return defaultValue(request);
            }
            case 'ReviewEmployeeArchiveRequest': {
              const state = readWorkflowState();
              const [requestID, decision, notes] = args;
              const request = (state.employeeArchiveRequests || []).find((item: any) => String(item.id) === String(requestID));
              if (!request) {
                return Promise.reject(new Error('Employee archive request not found'));
              }
              request.status = decision === 'approve' ? 'approved' : 'rejected';
              request.review_notes = String(notes || '');
              if (decision === 'approve') {
                request.second_approved_by = 'emp-admin';
                request.second_approved_by_name = 'Developer Admin';
                request.second_approved_at = now();
                const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(request.employee_id));
                if (employee) {
                  employee.is_active = false;
                  employee.employment_status = 'archived';
                  employee.archive_request_id = request.id;
                  employee.archive_reason = request.reason;
                  employee.archived_at = now();
                }
              }
              state.notificationFeed = (state.notificationFeed || []).map((notification: any) =>
                String(notification.source_id) === String(requestID)
                  ? { ...notification, status: 'read', read_at: now() }
                  : notification
              );
              writeWorkflowState(state);
              return defaultValue(request);
            }
            case 'ListEmployeeContributionSummaries': {
              const state = readWorkflowState();
              const tasks = state.collaborativeTasks || [];
              return defaultValue((state.employeeProfiles || []).map((employee: any) => {
                const employeeTasks = tasks.filter((task: any) => String(task.assignee_employee_id || '') === String(employee.id));
                const completed = employeeTasks.filter((task: any) => String(task.status || '') === 'completed').length;
                return {
                  employee_id: employee.id,
                  employee_code: employee.employee_code,
                  employee_name: employee.full_name,
                  department: employee.department,
                  job_title: employee.job_title,
                  employment_status: employee.employment_status || 'active',
                  is_active: employee.is_active !== false,
                  active_project_count: (state.collaborativeProjectMembers || []).filter((member: any) => String(member.employee_id) === String(employee.id)).length,
                  active_task_count: employeeTasks.filter((task: any) => !['completed', 'archived'].includes(String(task.status || ''))).length,
                  completed_task_count: completed,
                  blocked_task_count: employeeTasks.filter((task: any) => String(task.status || '') === 'blocked').length,
                  overdue_task_count: 0,
                  completion_rate: employeeTasks.length > 0 ? completed / employeeTasks.length : 0,
                };
              }));
            }
            case 'ListCollaborativeProjects': {
              const state = readWorkflowState();
              return defaultValue(collaborativeDataReady(state) ? (state.collaborativeProjects || []) : []);
            }
            case 'CreateCollaborativeProject': {
              const state = readWorkflowState();
              const payload = args[0] || {};
              const project = {
                id: `proj-created-${Date.now()}`,
                name: String(payload.name || '').trim(),
                project_type: String(payload.project_type || 'internal'),
                description: String(payload.description || ''),
                status: 'active',
                owner_employee_id: 'emp-peri',
                owner_name: 'Peri',
                created_at: now(),
                updated_at: now(),
              };
              state.collaborativeProjects = [project, ...(state.collaborativeProjects || [])];
              writeWorkflowState(state);
              return defaultValue(project);
            }
            case 'ListMyCollaborativeTasks': {
              const state = readWorkflowState();
              return defaultValue(collaborativeDataReady(state) ? (state.collaborativeTasks || []) : []);
            }
            case 'ListCollaborativeTeamTasks': {
              const state = readWorkflowState();
              return defaultValue(collaborativeDataReady(state) ? (state.collaborativeTasks || []) : []);
            }
            case 'ListCollaborativeProjectTasks': {
              const state = readWorkflowState();
              const projectID = String(args[0] || '');
              const tasks = collaborativeDataReady(state)
                ? (state.collaborativeTasks || []).filter((task: any) => String(task.project_id) === projectID)
                : [];
              return delayedValue(tasks, Number(opts.projectContextDelayMs || 0));
            }
            case 'ListCollaborativeProjectMembers': {
              const state = readWorkflowState();
              const projectID = String(args[0] || '');
              if (!collaborativeDataReady(state)) {
                return delayedValue([], Number(opts.projectContextDelayMs || 0));
              }
              const members = (state.collaborativeProjectMembers || []).filter((member: any) => String(member.project_id) === projectID);
              return delayedValue(members, Number(opts.projectContextDelayMs || 0));
            }
            case 'AddCollaborativeProjectMember': {
              const state = readWorkflowState();
              const [projectID, employeeID, role] = args;
              const project = (state.collaborativeProjects || []).find((item: any) => String(item.id) === String(projectID));
              const employee = (state.employeeProfiles || []).find((item: any) => String(item.id) === String(employeeID));
              if (!project || !employee) {
                return Promise.reject(new Error('Project or employee not found'));
              }
              const member = {
                id: `project-member-${Date.now()}`,
                project_id: project.id,
                employee_id: employee.id,
                employee_name: employee.full_name,
                project_name: project.name,
                role: String(role || 'Member'),
                allocation_percent: 100,
                is_active: true,
                joined_at: now(),
              };
              state.collaborativeProjectMembers = [member, ...(state.collaborativeProjectMembers || [])];
              writeWorkflowState(state);
              return defaultValue(member);
            }
            case 'ListCollaborativeProjectActivity': {
              const state = readWorkflowState();
              if (!collaborativeDataReady(state)) {
                return delayedValue([], Number(opts.projectContextDelayMs || 0));
              }
              const activity = (state.collaborativeTaskActivity || []).filter((item: any) => {
                const task = (state.collaborativeTasks || []).find((candidate: any) => String(candidate.id) === String(item.task_id));
                return String(task?.project_id || '') === String(args[0] || '');
              });
              return delayedValue(activity, Number(opts.projectContextDelayMs || 0));
            }
            case 'GetCollaborativeTask': {
              const state = readWorkflowState();
              const taskID = String(args[0] || '');
              const task = (state.collaborativeTasks || []).find((item: any) => String(item.id) === taskID);
              if (!task || !taskDetailsReady(state)) {
                return Promise.reject(new Error('task not found'));
              }
              return defaultValue(task);
            }
            case 'ListCollaborativeTaskComments': {
              const state = readWorkflowState();
              if (!taskDetailsReady(state)) {
                return Promise.reject(new Error('task not found'));
              }
              return defaultValue((state.collaborativeTaskComments || []).filter((item: any) => String(item.task_id) === String(args[0] || '')));
            }
            case 'ListCollaborativeTaskActivity': {
              const state = readWorkflowState();
              if (!taskDetailsReady(state)) {
                return Promise.reject(new Error('task not found'));
              }
              return defaultValue((state.collaborativeTaskActivity || []).filter((item: any) => String(item.task_id) === String(args[0] || '')));
            }
            case 'ListNotificationFeed': {
              const state = readWorkflowState();
              const limit = Number(args[0] || 50);
              const unreadOnly = Boolean(args[1]);
              const feed = (state.notificationFeed || []).filter((item: any) => !unreadOnly || item.status !== 'read');
              return defaultValue(feed.slice(0, limit));
            }
            case 'GetUnreadNotificationsCount': {
              const state = readWorkflowState();
              return defaultValue((state.notificationFeed || []).filter((item: any) => item.status !== 'read').length);
            }
            case 'MarkNotificationAsRead': {
              const state = readWorkflowState();
              const notification = (state.notificationFeed || []).find((item: any) => String(item.id) === String(args[0] || ''));
              return new Promise((resolve) => {
                window.setTimeout(() => {
                  if (notification) {
                    notification.status = 'read';
                    notification.read_at = now();
                    writeWorkflowState(state);
                  }
                  resolve(undefined);
                }, Number(opts.notificationReadDelayMs || 0));
              });
            }
            case 'RefreshCollaborativeWorkspace': {
              const state = readWorkflowState();
              state.collaborativeWorkspaceRefreshCount = Number(state.collaborativeWorkspaceRefreshCount || 0) + 1;
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'GetSettings':
              return defaultValue({ defaultVatRate: 10, defaultMargin: 20 });
            case 'GetCostingSheets':
              return defaultValue(readWorkflowState().costingSheets);
            case 'ListConversations':
              return defaultValue(readConversations());
            case 'GetConversationMessages':
              return defaultValue(readMessages()[String(args[0])] || []);
            case 'ChatWithButlerPersistent': {
              const [conversationId, message] = args;
              const id = String(conversationId || window.localStorage.getItem(ACTIVE_CONVERSATION_KEY) || `conv-${Date.now()}`);
              createConversation(id, String(message || ''));
              return defaultValue({
                conversation_id: id,
                response: mockButlerResponse,
                confidence: 0.99,
                actions: [],
              });
            }
            case 'ChatWithButler':
              return defaultValue({
                message: mockButlerResponse,
                confidence: 0.99,
                actions: [],
              });
            case 'DeleteConversation':
              return defaultValue(undefined);
            case 'PurgeAllConversations':
              writeConversations([]);
              writeMessages({});
              window.localStorage.removeItem(ACTIVE_CONVERSATION_KEY);
              return defaultValue(undefined);
            case 'PickFile':
              return defaultValue('/tmp/ph-holdings-smoke/bank-statement.pdf');
            case 'ProcessDocumentWithOCR':
              (window as any).__wailsOCRCalls.push({
                filePath: args[0],
                docType: args[1],
              });
              return defaultValue(mockOCRResult);
            case 'AnalyzeDocumentWithButler':
              return defaultValue(mockButlerInsight);
            case 'SaveDocumentToEntity':
              (window as any).__wailsDocumentSaveCalls.push({
                fileName: args[0],
                filePath: args[1],
                documentType: args[2],
                extractedText: args[3],
                confidence: args[4],
                processingTimeMs: args[5],
                engine: args[6],
                extractedDataJSON: args[7],
              });
              return defaultValue({ routed: true, entity_id: 'ocr-doc-1' });
            case 'SaveOCRDocument':
            case 'UpdateRFQStage':
            case 'GetPurchaseOrdersByOrder':
            case 'GetRFQTraceability':
              return defaultVoid();
            case 'GetRFQComments': {
              const state = readWorkflowState();
              return defaultValue((state.rfqComments || []).filter((item: any) => String(item.rfq_id) === String(args[0])));
            }
            case 'GetOpportunityComments': {
              const state = readWorkflowState();
              return defaultValue((state.opportunityComments || []).filter((item: any) => String(item.opportunity_id) === String(args[0])));
            }
            case 'AddRFQComment': {
              const [rfqId, comment, visibility] = args;
              const state = readWorkflowState();
              state.rfqComments = state.rfqComments || [];
              state.rfqComments.unshift({
                id: `rfq-comment-${Date.now()}`,
                rfq_id: String(rfqId),
                comment: String(comment || ''),
                content: String(comment || ''),
                visibility: String(visibility || ''),
                created_by: 'Developer',
                created_at: now(),
              });
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'AddOpportunityComment': {
              const [opportunityId, comment] = args;
              const state = readWorkflowState();
              state.opportunityComments = state.opportunityComments || [];
              state.opportunityComments.unshift({
                id: `opportunity-comment-${Date.now()}`,
                opportunity_id: String(opportunityId),
                comment: String(comment || ''),
                content: String(comment || ''),
                created_by: 'Developer',
                created_at: now(),
              });
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'DeleteOpportunityComment': {
              const commentId = String(args[0] || '');
              const state = readWorkflowState();
              state.opportunityComments = (state.opportunityComments || []).filter((item: any) => String(item.id) !== commentId);
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'CreateCostingSheet':
            case 'UpdateCostingSheet':
              return defaultVoid();
            case 'SaveCostingAsOffer': {
              const state = readWorkflowState();
              const payload = args[0] || {};
              const offer = {
                id: `offer-${state.nextOfferId}`,
                offer_number: `PHO-2026-${String(state.nextOfferId).padStart(3, '0')}`,
                customer_name: payload.customerName || payload.customer_name || 'BAPCO',
                project_name: payload.projectName || payload.project_name || 'Generated Offer',
                stage: 'Quoted',
                quotation_date: now(),
                validity_date: now(),
                total_value_bhd: Number(payload.grandTotal || payload.total_value_bhd || 0),
                created_at: now(),
                items: payload.items || payload.line_items || [],
              };
              state.nextOfferId += 1;
              state.offers.unshift(offer);
              writeWorkflowState(state);
              return defaultValue(offer);
            }
            case 'UpdateOfferFull': {
              const [offerId, payload] = args;
              const state = readWorkflowState();
              const offer = (state.offers || []).find((item: any) => String(item.id) === String(offerId));
              if (!offer) {
                return Promise.reject(new Error('Offer not found'));
              }
              const { items, ...headerPayload } = payload || {};
              Object.assign(offer, headerPayload);
              if (Array.isArray(items) && items.length > 0) {
                offer.items = items;
                offer.total_value_bhd = items.reduce((sum: number, item: any) => (
                  sum + (Number(item.total_price) || ((Number(item.quantity) || 0) * (Number(item.unit_price) || 0)))
                ), 0);
              }
              writeWorkflowState(state);
              return defaultValue(offer);
            }
            case 'GetAllOffers':
              return defaultValue(readWorkflowState().offers);
            case 'GetPendingFollowUps':
            case 'GetOffersWithNoItems':
              return defaultValue([]);
            case 'ListOrders':
              return defaultValue(readWorkflowState().orders);
            case 'GetSupplierInvoices':
              return defaultValue(readWorkflowState().supplierInvoices);
            case 'GetSupplierInvoiceByID': {
              const state = readWorkflowState();
              return defaultValue(findSupplierInvoice(state, String(args[0])) || null);
            }
            case 'GetSupplierPaymentsByInvoice': {
              const state = readWorkflowState();
              return defaultValue(deriveSupplierPayments(state).filter((item: any) => String(item.supplier_invoice_id) === String(args[0])));
            }
            case 'GetUnpaidSupplierInvoices': {
              const state = readWorkflowState();
              return defaultValue((state.supplierInvoices || []).filter((item: any) => item.payment_status !== 'Paid'));
            }
            case 'GetOverdueSupplierInvoices': {
              const state = readWorkflowState();
              return defaultValue((state.supplierInvoices || []).filter((item: any) => item.payment_status !== 'Paid'));
            }
            case 'GetAllSupplierPayments':
              return defaultValue(deriveSupplierPayments(readWorkflowState()));
            case 'UpdateSupplierInvoice': {
              const [payload] = args;
              const state = readWorkflowState();
              const invoice = findSupplierInvoice(state, String(payload?.id || ''));
              if (!invoice) {
                return Promise.reject(new Error('Supplier invoice not found'));
              }
              Object.assign(invoice, payload || {});
              writeWorkflowState(state);
              return defaultValue(invoice);
            }
            case 'UpdateSupplierInvoiceWithPayment': {
              const [payload, createPaymentEntry] = args;
              const state = readWorkflowState();
              const invoice = findSupplierInvoice(state, String(payload?.id || ''));
              if (!invoice) {
                return Promise.reject(new Error('Supplier invoice not found'));
              }
              Object.assign(invoice, payload || {});
              if (invoice.payment_status === 'Paid') {
                invoice.status = 'Paid';
              }
              if (createPaymentEntry && invoice.payment_status === 'Paid') {
                const existing = (state.supplierPayments || []).filter((item: any) => String(item.supplier_invoice_id) === String(invoice.id));
                if (existing.length === 0) {
                  state.supplierPayments.unshift({
                    id: `supp-pay-${Date.now()}`,
                    supplier_invoice_id: invoice.id,
                    supplier_id: invoice.supplier_id,
                    supplier_name: invoice.supplier_name,
                    invoice_number: invoice.invoice_number,
                    amount_foreign: Number(invoice.total_foreign || 0),
                    amount_bhd: Number(invoice.total_bhd || 0),
                    currency: invoice.currency || 'BHD',
                    exchange_rate: Number(invoice.exchange_rate || 1),
                    payment_date: invoice.payment_date || now(),
                    payment_method: invoice.payment_method || 'Bank Transfer',
                    reference: invoice.payment_ref || '',
                  });
                }
              }
              writeWorkflowState(state);
              return defaultValue(invoice);
            }
            case 'MarkSupplierInvoicePaid': {
              const [invoiceId, paymentReference, paymentMethod] = args;
              const state = readWorkflowState();
              const invoice = findSupplierInvoice(state, String(invoiceId));
              if (!invoice) {
                return Promise.reject(new Error('Supplier invoice not found'));
              }
              invoice.payment_status = 'Paid';
              invoice.status = 'Paid';
              invoice.payment_ref = String(paymentReference || '');
              invoice.payment_method = String(paymentMethod || 'Bank Transfer');
              invoice.payment_date = now();
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'PerformThreeWayMatch':
              return defaultValue({ matched: true, reason: '' });
            case 'ApproveSupplierInvoice':
              return defaultVoid();
            case 'GetSupportedCurrencies':
              return defaultValue([{ code: 'BHD', name: 'Bahraini Dinar' }, { code: 'USD', name: 'US Dollar' }]);
            case 'GetInvoicesByStatus': {
              const state = readWorkflowState();
              return defaultValue((state.customerInvoices || []).filter((item: any) => String(item.status) === String(args[0])));
            }
            case 'ListCustomerInvoices':
              return defaultValue(readWorkflowState().customerInvoices || []);
            case 'GetAllPayments':
              return defaultValue(deriveCustomerPayments(readWorkflowState()));
            case 'ListCustomerReceipts':
              return defaultValue(readWorkflowState().customerReceipts || []);
            case 'GetCustomerReceiptAllocations':
              return defaultValue([]);
            case 'CreateCustomerReceipt': {
              const [payload] = args;
              const state = readWorkflowState();
              const receiptDate = String(payload?.receipt_date || now()).slice(0, 10);
              let invoice = null;
              if (payload?.invoice_id) {
                invoice = (state.customerInvoices || []).find((item: any) => String(item.id) === String(payload.invoice_id));
                if (!invoice) {
                  return Promise.reject(new Error('Invoice not found'));
                }
              }
              let customer = null;
              if (invoice) {
                customer = (state.customers || []).find((item: any) => String(item.id) === String(invoice.customer_id));
              } else if (payload?.customer_id) {
                customer = findCustomer(state, String(payload.customer_id));
              }
              const amount = Number(payload?.amount_bhd || 0);
              const receipt = {
                id: `receipt-${state.nextReceiptId || 1}`,
                receipt_number: `RCT-2026-${String(state.nextReceiptId || 1).padStart(4, '0')}`,
                customer_id: invoice?.customer_id || payload?.customer_id || customer?.id || '',
                customer_name: invoice?.customer_name || payload?.customer_name || customer?.business_name || 'Customer',
                receipt_date: receiptDate,
                amount_bhd: amount,
                applied_amount_bhd: invoice ? amount : 0,
                unapplied_amount_bhd: invoice ? 0 : amount,
                payment_method: payload?.payment_method || 'Bank Transfer',
                reference: payload?.reference || '',
                status: invoice ? 'Applied' : 'OnAccount',
                division: payload?.division || invoice?.division || 'PH Trading',
                notes: payload?.notes || '',
                created_at: now(),
              };
              state.nextReceiptId = Number(state.nextReceiptId || 1) + 1;
              state.customerReceipts = [receipt, ...(state.customerReceipts || [])];
              if (invoice) {
                invoice.outstanding_bhd = Math.max(0, Number(invoice.outstanding_bhd || invoice.grand_total_bhd || 0) - amount);
                invoice.status = invoice.outstanding_bhd <= 0 ? 'Paid' : 'PartiallyPaid';
                const payment = {
                  id: `pay-${state.nextPaymentId || 1}`,
                  invoice_id: invoice.id,
                  invoice_number: invoice.invoice_number,
                  customer_name: invoice.customer_name,
                  amount_bhd: amount,
                  payment_date: receiptDate,
                  payment_method: receipt.payment_method,
                  reference: receipt.reference,
                  receipt_id: receipt.id,
                  days_to_payment: 0,
                  status: 'Received',
                  division: receipt.division,
                  created_at: now(),
                };
                state.nextPaymentId = Number(state.nextPaymentId || 1) + 1;
                state.payments = [payment, ...(state.payments || [])];
              }
              writeWorkflowState(state);
              return defaultValue(receipt);
            }
            case 'ApplyCustomerReceiptToInvoice':
              return defaultValue({ id: `alloc-${Date.now()}`, allocated_amount_bhd: Number(args[2] || 0), status: 'Applied' });
            case 'RecordPayment': {
              const [invoiceId, amountRaw, method, paymentDate, reference] = args;
              const state = readWorkflowState();
              const invoice = (state.customerInvoices || []).find((item: any) => String(item.id) === String(invoiceId));
              if (!invoice) {
                return Promise.reject(new Error('Invoice not found'));
              }
              const amount = Number(amountRaw || 0);
              const payment = {
                id: `pay-${state.nextPaymentId || 1}`,
                invoice_id: invoice.id,
                invoice_number: invoice.invoice_number,
                customer_name: invoice.customer_name,
                amount_bhd: amount,
                payment_date: String(paymentDate || now()).slice(0, 10),
                payment_method: method || 'Bank Transfer',
                reference: reference || '',
                days_to_payment: 0,
                status: 'Received',
                division: invoice.division || 'PH Trading',
                created_at: now(),
              };
              state.nextPaymentId = Number(state.nextPaymentId || 1) + 1;
              state.payments = [payment, ...(state.payments || [])];
              invoice.outstanding_bhd = Math.max(0, Number(invoice.outstanding_bhd || invoice.grand_total_bhd || 0) - amount);
              invoice.status = invoice.outstanding_bhd <= 0 ? 'Paid' : 'PartiallyPaid';
              writeWorkflowState(state);
              return defaultValue(payment);
            }
            case 'GetPayment': {
              const state = readWorkflowState();
              return defaultValue((state.payments || []).find((item: any) => String(item.id) === String(args[0])) || null);
            }
            case 'UpdatePayment': {
              const [paymentId, payload] = args;
              const state = readWorkflowState();
              const payment = (state.payments || []).find((item: any) => String(item.id) === String(paymentId));
              if (!payment) {
                return Promise.reject(new Error('Payment not found'));
              }
              Object.assign(payment, payload || {});
              writeWorkflowState(state);
              return defaultValue(payment);
            }
            case 'DeletePayment': {
              const state = readWorkflowState();
              state.payments = (state.payments || []).filter((item: any) => String(item.id) !== String(args[0]));
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'GetOperationalLedger':
              return defaultValue(buildOperationalLedger(readWorkflowState(), args[0] || {}));
            case 'GetCustomerLedger':
              return defaultValue(buildOperationalLedger(readWorkflowState(), { customer_id: args[0], start_date: args[1], end_date: args[2] }));
            case 'GetSupplierLedger':
              return defaultValue(buildOperationalLedger(readWorkflowState(), { supplier_id: args[0], start_date: args[1], end_date: args[2] }));
            case 'GetExpenseLedger':
              return defaultValue(buildOperationalLedger(readWorkflowState(), { ...(args[0] || {}), source_type: 'EXPENSE' }));
            case 'GetOperationalProfitLoss':
              return defaultValue(buildProfitLoss(readWorkflowState(), args[0] || {}));
            case 'GetOperationalBalanceSheet':
              return defaultValue(buildBalanceSheet(readWorkflowState(), args[0], args[1]));
            case 'GetJournalEntries':
              return defaultValue(readWorkflowState().journalEntries || []);
            case 'ListExpenseCategories':
              return defaultValue(readWorkflowState().expenseCategories || []);
            case 'ListExpenseVendors':
              return defaultValue(readWorkflowState().expenseVendors || []);
            case 'ListRecurringExpenses':
              return defaultValue(readWorkflowState().recurringExpenses || []);
            case 'ListBankExpenseCandidates':
              return defaultValue(readWorkflowState().bankExpenseCandidates || []);
            case 'ListUnreconciledPayrollPayouts':
              return defaultValue(readWorkflowState().payrollPayouts || []);
            case 'ListExpenseDashboardSummary': {
              const entries = readWorkflowState().expenseEntries || [];
              return defaultValue({
                total_drafts: entries.filter((entry: any) => String(entry.status || '') === 'draft').length,
                total_submitted: entries.filter((entry: any) => String(entry.status || '') === 'submitted').length,
                total_approved_unpaid: entries.filter((entry: any) => ['approved', 'posted'].includes(String(entry.status || '')) && String(entry.payment_status || '') !== 'paid').length,
                total_recurring: 0,
                month_to_date_spend: entries.reduce((sum: number, entry: any) => sum + Number(entry.total_amount || 0), 0),
                upcoming_commitments: entries.filter((entry: any) => String(entry.payment_status || '') !== 'paid').reduce((sum: number, entry: any) => sum + Number(entry.total_amount || 0), 0),
              });
            }
            case 'ListExpenseEntries': {
              const state = readWorkflowState();
              const status = String(args[0] || '').toLowerCase();
              const includePaid = Boolean(args[1]);
              return defaultValue((state.expenseEntries || []).filter((entry: any) => {
                if (status && String(entry.status || '').toLowerCase() !== status) return false;
                if (!includePaid && String(entry.payment_status || '').toLowerCase() === 'paid') return false;
                return true;
              }));
            }
            case 'DeleteExpenseEntry': {
              const state = readWorkflowState();
              const entry = (state.expenseEntries || []).find((item: any) => String(item.id) === String(args[0]));
              if (!entry) {
                return Promise.reject(new Error('Expense entry not found'));
              }
              if (['posted', 'paid'].includes(String(entry.status || '').toLowerCase()) || String(entry.payment_status || '').toLowerCase() === 'paid') {
                return Promise.reject(new Error('posted or paid expenses cannot be deleted'));
              }
              state.expenseEntries = (state.expenseEntries || []).filter((item: any) => String(item.id) !== String(args[0]));
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'GetActiveBankAccounts':
              return defaultValue((readWorkflowState().bankAccounts || []).filter((item: any) => item.is_active !== false));
            case 'GetAllBankAccounts':
              return defaultValue(readWorkflowState().bankAccounts);
            case 'GetCashPosition': {
              const state = readWorkflowState();
              const total = (state.bankAccounts || []).reduce((sum: number, _item: any) => sum + 1500, 0);
              return defaultValue({ total_bhd: total, by_account: { 'bank-1': 1500 }, as_of: now() });
            }
            case 'GetBankStatements': {
              const state = readWorkflowState();
              return defaultValue((state.bankStatements || []).filter((item: any) => String(item.bank_account_id) === String(args[0])));
            }
            case 'GetBankStatementLines': {
              const state = readWorkflowState();
              return defaultValue((state.bankStatementLines || []).filter((item: any) => String(item.bank_statement_id) === String(args[0])));
            }
            case 'ManualMatchLine': {
              const [lineId, entityType, entityId] = args;
              const state = readWorkflowState();
              const line = (state.bankStatementLines || []).find((item: any) => String(item.id) === String(lineId));
              if (!line) {
                return Promise.reject(new Error('Bank statement line not found'));
              }
              line.is_matched = true;
              line.match_type = 'Manual';
              line.match_confidence = 1;
              if (entityType === 'SUPPLIER_PAYMENT' || entityType === 'CUSTOMER_PAYMENT' || entityType === 'CUSTOMER_RECEIPT') {
                line.matched_payment_id = String(entityId);
              } else if (entityType === 'SUPPLIER_INVOICE') {
                line.matched_payment_ids = String(entityId);
              } else if (entityType === 'CUSTOMER_INVOICE') {
                line.matched_invoice_ids = String(entityId);
              }
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'CreateSplitAllocation': {
              const [lineId, allocations] = args;
              const state = readWorkflowState();
              const line = (state.bankStatementLines || []).find((item: any) => String(item.id) === String(lineId));
              if (!line) {
                return Promise.reject(new Error('Bank statement line not found'));
              }
              line.is_matched = true;
              line.match_type = 'Split';
              line.match_confidence = 1;
              line.matched_invoice_ids = JSON.stringify((allocations || []).map((item: any) => `${item.allocation_type}:${item.entity_id}`));
              writeWorkflowState(state);
              return defaultVoid();
            }
            case 'GetPurchaseOrders':
              return defaultValue(readWorkflowState().purchaseOrders);
            case 'GetPurchaseOrderByID': {
              const state = readWorkflowState();
              return defaultValue(state.purchaseOrders.find((item: any) => item.id === args[0]) || state.purchaseOrders[0]);
            }
            case 'CreatePurchaseOrder': {
              const state = readWorkflowState();
              const payload = args[0] || {};
              const supplier = state.suppliers.find((item: any) => item.id === payload.supplier_id);
              const po = {
                id: `po-${state.nextPOId}`,
                po_number: `PO-2026-${String(state.nextPOId).padStart(3, '0')}`,
                supplier_name: supplier?.supplier_name || 'Unknown Supplier',
                payment_due_date: payload.expected_delivery,
                ...payload,
              };
              state.nextPOId += 1;
              state.purchaseOrders.unshift(po);
              writeWorkflowState(state);
              return defaultValue(po);
            }
            default:
              return defaultVoid();
          }
        };
      },
    });

    const runtime = new Proxy({}, {
      get(_, prop) {
        switch (String(prop)) {
          case 'EventsOn':
            return () => () => {};
          case 'OnFileDrop':
            return (callback: Function, useDropTarget = true) => {
              (window as any).__wailsFileDropCallbacks.push({ callback, useDropTarget });
              return () => {};
            };
          case 'EventsOff':
          case 'OnFileDropOff':
          case 'EventsOffAll':
          case 'BrowserOpenURL':
          case 'WindowReload':
          case 'WindowReloadApp':
          case 'LogInfo':
          case 'LogWarning':
          case 'LogError':
          case 'LogDebug':
          case 'LogTrace':
          case 'LogPrint':
            return () => {};
          default:
            return () => {};
        }
      },
    });

    window.go = { main: { App: app } } as any;
    window.runtime = runtime as any;
  }, options);
}
