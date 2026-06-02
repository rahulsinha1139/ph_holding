<script>
    import { onDestroy, onMount } from "svelte";
    import { fade } from "svelte/transition";
    import FinancialDashboard from "./FinancialDashboard.svelte";
    import InvoicesScreen from "./InvoicesScreen.svelte";
    import PaymentsScreen from "./PaymentsScreen.svelte";
    import SupplierPaymentsScreen from "./SupplierPaymentsScreen.svelte";
    import ExpensesScreen from "./ExpensesScreen.svelte";
    import AccountingScreen from "./AccountingScreen.svelte";
    import ReportsScreen from "./ReportsScreen.svelte";
    import AuditTrailViewer from "./AuditTrailViewer.svelte";
    import PayrollScreen from "./PayrollScreen.svelte";

    // Bank Reconciliation Screens (Feature B)
    import BankReconciliationScreen from "./BankReconciliationScreen.svelte";
    import ChequeRegisterScreen from "./ChequeRegisterScreen.svelte";
    import FXRevaluationScreen from "./FXRevaluationScreen.svelte";
    import AHSDashboard from "./AHSDashboard.svelte";
    import ErrorBoundary from "../components/ErrorBoundary.svelte";

    export let params = {};

    let activeTab = "dashboard";
    let lastAppliedRouteKey = "";

    // E2: Company selector - 'PH Trading' is the default, 'AHS Trading' is the sister company
    /** @typedef {"PH Trading" | "AHS Trading"} CompanyName */
    /** @type {CompanyName} */
    let selectedCompany = "PH Trading";
    /** @type {CompanyName[]} */
    const companies = ["PH Trading", "AHS Trading"];

    /**
     * @param {unknown} value
     * @returns {CompanyName | null}
     */
    function normalizeCompanyName(value) {
        if (value === "PH Trading" || value === "AHS Trading") {
            return value;
        }
        return null;
    }

    const tabs = [
        { id: "dashboard", label: "Dashboard" },
        { id: "invoices", label: "Customer Invoices" },
        { id: "payments", label: "Receipts" },
        { id: "payments_made", label: "Supplier Settlements" },
        { id: "expenses", label: "Expenses" },
        { id: "accounting", label: "Accounting" },
        { id: "reports", label: "Reports & Exports" },
        { id: "audit", label: "Audit Trail" },
        { id: "expense_approvals", label: "Approvals" },
        { id: "payroll_runs", label: "Payroll" },
        { id: "bank_recon", label: "Bank Recon" },
        { id: "cheques", label: "Cheques" },
        { id: "fx", label: "FX" },
    ];

    function normalizeFinanceTab(value) {
        const raw = String(value || "");
        if (raw === "bank-recon" || raw === "book-bank" || raw === "book_bank") {
            return "bank_recon";
        }
        return raw;
    }

    function handleNavigate(event) {
        const nextTab = normalizeFinanceTab(event?.detail?.tab);
        if (nextTab && tabs.some((tab) => tab.id === nextTab)) {
            activeTab = nextTab;
        }
    }

    function handleFinanceNavigate(event) {
        const nextTab = normalizeFinanceTab(event?.detail?.tab);
        const nextCompany = event?.detail?.company;
        if (nextTab && tabs.some((tab) => tab.id === nextTab)) {
            activeTab = nextTab;
        }
        const normalizedCompany = normalizeCompanyName(nextCompany);
        if (normalizedCompany) {
            selectedCompany = normalizedCompany;
        }
    }

    onMount(() => {
        window.addEventListener("finance:navigate", handleFinanceNavigate);
    });

    onDestroy(() => {
        window.removeEventListener("finance:navigate", handleFinanceNavigate);
    });

    $: {
        const requestedTab = normalizeFinanceTab(params?.tab);
        const requestedCompany = params?.company;
        const nextRouteKey = JSON.stringify({
            tab: requestedTab || "",
            company: requestedCompany || "",
        });

        if (nextRouteKey !== lastAppliedRouteKey) {
            if (requestedTab && tabs.some((tab) => tab.id === requestedTab)) {
                activeTab = requestedTab;
            }
            const normalizedCompany = normalizeCompanyName(requestedCompany);
            if (normalizedCompany) {
                selectedCompany = normalizedCompany;
            }
            lastAppliedRouteKey = nextRouteKey;
        }
    }
</script>

<ErrorBoundary name="Finance Hub">
    <div class="hub" in:fade={{ duration: 120 }}>
        <header class="header">
            <div class="header-top">
                <!-- E2: Company Selector -->
                <div class="company-selector">
                    {#each companies as company}
                        <button
                            class="company-btn"
                            class:active={selectedCompany === company}
                            on:click={() => selectedCompany = company}
                        >
                            {company}
                        </button>
                    {/each}
                </div>
            </div>

            <nav class="tabs" aria-label="Finance sections">
                {#each tabs as tab}
                    <button
                        class="tab"
                        class:active={activeTab === tab.id}
                        on:click={() => (activeTab = tab.id)}
                    >
                        {tab.label}
                    </button>
                {/each}
            </nav>
        </header>

        <main class="content" class:reportsActive={activeTab === "reports"}>
            {#if activeTab === "dashboard"}
                {#if selectedCompany === 'AHS Trading'}
                    <AHSDashboard embedded={true} />
                {:else}
                    <FinancialDashboard embedded={true} />
                {/if}
            {:else if activeTab === "invoices"}
                <InvoicesScreen embedded={true} company={selectedCompany} on:navigate={handleNavigate} />
            {:else if activeTab === "payments"}
                <PaymentsScreen embedded={true} company={selectedCompany} on:navigate={handleNavigate} />
            {:else if activeTab === "payments_made"}
                <SupplierPaymentsScreen embedded={true} company={selectedCompany} on:navigate={handleNavigate} />
            {:else if activeTab === "expenses"}
                <ExpensesScreen embedded={true} mode="workspace" company={selectedCompany} />
            {:else if activeTab === "accounting"}
                <AccountingScreen embedded={true} company={selectedCompany} />
            {:else if activeTab === "reports"}
                <ReportsScreen />
            {:else if activeTab === "audit"}
                <AuditTrailViewer embedded={true} />
            {:else if activeTab === "expense_approvals"}
                <ExpensesScreen embedded={true} mode="approvals" company={selectedCompany} />
            {:else if activeTab === "payroll_runs"}
                <PayrollScreen embedded={true} mode="workspace" company={selectedCompany} />
            {:else if activeTab === "bank_recon"}
                <BankReconciliationScreen embedded={true} company={selectedCompany} />
            {:else if activeTab === "cheques"}
                <ChequeRegisterScreen embedded={true} />
            {:else if activeTab === "fx"}
                <FXRevaluationScreen embedded={true} />
            {/if}
        </main>
    </div>
</ErrorBoundary>

<style>
    .hub {
        min-height: 100vh;
        background: var(--bg-base);
        font-family: var(--font-body);
        display: flex;
        flex-direction: column;
    }

    .header {
        padding: 10px 16px 0;
        border-bottom: 1px solid #e5e5e5;
        flex-shrink: 0;
    }

    .header-top {
        display: flex;
        justify-content: flex-end;
        align-items: center;
        margin-bottom: 8px;
    }

    /* E2: Company Selector */
    .company-selector {
        display: flex;
        gap: 0;
        border: 1px solid #e5e5e5;
        border-radius: 8px;
        overflow: hidden;
    }

    .company-btn {
        padding: 6px 12px;
        background: transparent;
        border: none;
        border-right: 1px solid #e5e5e5;
        font-family: var(--font-body);
        font-size: var(--control-text-size-scaled, 13px);
        font-weight: 500;
        color: #6e6e73;
        cursor: pointer;
        transition: all 0.2s ease;
    }

    .company-btn:last-child {
        border-right: none;
    }

    .company-btn:hover {
        background: #f5f5f7;
        color: #1d1d1f;
    }

    .company-btn.active {
        background: #1d1d1f;
        color: #ffffff;
    }

    .tabs {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(118px, 1fr));
        gap: 6px;
        padding-bottom: 8px;
        height: auto;
        min-height: 0;
        border-bottom: 0;
        align-items: stretch;
        overflow: visible;
    }

    .tab {
        width: 100%;
        min-height: 30px;
        padding: 6px 9px;
        background: rgba(255, 255, 255, 0.72);
        border: 1px solid rgba(29, 29, 31, 0.08);
        border-radius: 8px;
        box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
        font-family: var(--font-body);
        font-size: var(--tab-text-size-scaled, 12.5px);
        font-weight: 650;
        color: #6e6e73;
        cursor: pointer;
        transition: background 0.18s ease, border-color 0.18s ease, color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
        line-height: 1.1;
        white-space: normal;
        overflow-wrap: anywhere;
        text-align: center;
    }

    .tab:hover {
        background: #ffffff;
        color: #1d1d1f;
        border-color: rgba(29, 29, 31, 0.18);
        box-shadow: 0 5px 14px rgba(15, 23, 42, 0.09);
        transform: translateY(-1px);
    }

    .tab.active {
        color: #ffffff;
        background: #1d1d1f;
        border-color: #1d1d1f;
        box-shadow: 0 6px 18px rgba(29, 29, 31, 0.18);
    }

    .content {
        flex: 1;
        min-height: 0;
        padding: 12px 16px;
        overflow: auto;
    }

    .content.reportsActive {
        padding: 0;
        overflow: hidden;
    }

    @media (max-width: 768px) {
        .header {
            padding: 24px 16px 0;
        }

        .content {
            padding: 16px;
        }
    }
</style>
