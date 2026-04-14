<script>
    import { onDestroy, onMount } from "svelte";
    import { fade } from "svelte/transition";
    import FinancialDashboard from "./FinancialDashboard.svelte";
    import InvoicesScreen from "./InvoicesScreen.svelte";
    import PaymentsScreen from "./PaymentsScreen.svelte";
    import SupplierPaymentsScreen from "./SupplierPaymentsScreen.svelte";
    import ExpensesScreen from "./ExpensesScreen.svelte";
    import PayrollScreen from "./PayrollScreen.svelte";

    // Bank Reconciliation Screens (Feature B)
    import BankReconciliationScreen from "./BankReconciliationScreen.svelte";
    // Commented out - developing later
    // import ChequeRegisterScreen from "./ChequeRegisterScreen.svelte";
    // import BookBankReconciliationScreen from "./BookBankReconciliationScreen.svelte";
    // import FXRevaluationScreen from "./FXRevaluationScreen.svelte";
    // import AuditTrailViewer from "./AuditTrailViewer.svelte";
    import ErrorBoundary from "../components/ErrorBoundary.svelte";
    import AHSDashboard from "./AHSDashboard.svelte";

    export let params = {};

    let activeTab = "dashboard";
    let lastAppliedRouteKey = "";

    // E2: Company selector - 'PH Trading' is the default, 'AHS Trading' is the sister company
    let selectedCompany = "PH Trading";
    const companies = ["PH Trading", "AHS Trading"];

    const tabs = [
        { id: "dashboard", label: "Dashboard" },
        { id: "invoices", label: "Customer Invoices" },
        { id: "payments", label: "Payments Received" },
        { id: "payments_made", label: "Payments Made" },
        { id: "expenses", label: "Expenses" },
        { id: "expense_approvals", label: "Approvals" },
        { id: "payroll_runs", label: "Payroll" },
        { id: "bank_recon", label: "Bank Recon" },
        // Developing later:
        // { id: "cheques", label: "Cheques" },
        // { id: "book_bank", label: "Book-Bank" },
        // { id: "fx", label: "FX" },
        // { id: "audit", label: "Audit Trail" },
    ];

    function handleNavigate(event) {
        const nextTab = event?.detail?.tab;
        if (nextTab && tabs.some((tab) => tab.id === nextTab)) {
            activeTab = nextTab;
        }
    }

    function handleFinanceNavigate(event) {
        const nextTab = event?.detail?.tab;
        const nextCompany = event?.detail?.company;
        if (nextTab && tabs.some((tab) => tab.id === nextTab)) {
            activeTab = nextTab;
        }
        if (companies.includes(nextCompany)) {
            selectedCompany = nextCompany;
        }
    }

    onMount(() => {
        window.addEventListener("finance:navigate", handleFinanceNavigate);
    });

    onDestroy(() => {
        window.removeEventListener("finance:navigate", handleFinanceNavigate);
    });

    $: {
        const requestedTab = params?.tab;
        const requestedCompany = params?.company;
        const nextRouteKey = JSON.stringify({
            tab: requestedTab || "",
            company: requestedCompany || "",
        });

        if (nextRouteKey !== lastAppliedRouteKey) {
            if (requestedTab && tabs.some((tab) => tab.id === requestedTab)) {
                activeTab = requestedTab;
            }
            if (companies.includes(requestedCompany)) {
                selectedCompany = requestedCompany;
            }
            lastAppliedRouteKey = nextRouteKey;
        }
    }
</script>

<ErrorBoundary name="Finance Hub">
    <div class="hub" in:fade={{ duration: 200 }}>
        <header class="header">
            <div class="header-top">
                <h1>Finance Hub</h1>
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

            <nav class="tabs">
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

        <main class="content">
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
            {:else if activeTab === "expense_approvals"}
                <ExpensesScreen embedded={true} mode="approvals" company={selectedCompany} />
            {:else if activeTab === "payroll_runs"}
                <PayrollScreen embedded={true} mode="workspace" company={selectedCompany} />
            {:else if activeTab === "bank_recon"}
                <BankReconciliationScreen embedded={true} company={selectedCompany} />
            <!-- Developing later:
            {:else if activeTab === "cheques"}
                <ChequeRegisterScreen embedded={true} />
            {:else if activeTab === "book_bank"}
                <BookBankReconciliationScreen embedded={true} />
            {:else if activeTab === "fx"}
                <FXRevaluationScreen embedded={true} />
            {:else if activeTab === "audit"}
                <AuditTrailViewer embedded={true} />
            -->
            {/if}
        </main>
    </div>
</ErrorBoundary>

<style>
    .hub {
        min-height: 100vh;
        background: var(--bg-base);
        font-family: var(--font-body);
    }

    .header {
        padding: 24px 24px 0;
        border-bottom: 1px solid #e5e5e5;
    }

    .header-top {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 20px;
    }

    .header h1 {
        font-family: var(--font-display);
        font-size: 24px;
        font-weight: 500;
        color: #1d1d1f;
        margin: 0;
        letter-spacing: -0.02em;
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
        padding: 8px 16px;
        background: transparent;
        border: none;
        border-right: 1px solid #e5e5e5;
        font-family: var(--font-body);
        font-size: 13px;
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
        display: flex;
        gap: 0;
    }

    .tab {
        padding: 12px 24px;
        background: transparent;
        border: none;
        border-bottom: 2px solid transparent;
        font-family: var(--font-body);
        font-size: 14px;
        font-weight: 500;
        color: #6e6e73;
        cursor: pointer;
        transition: all 0.2s ease;
    }

    .tab:hover {
        color: #1d1d1f;
    }

    .tab.active {
        color: #1d1d1f;
        border-bottom-color: #1d1d1f;
    }

    .content {
        padding: 20px 24px;
    }

    /* CashPositionWidget sidebar removed - dashboard now full width */

    .placeholder {
        padding: 48px;
        text-align: center;
        color: #6e6e73;
    }

    .placeholder h2 {
        font-family: var(--font-display);
        font-size: 18px;
        font-weight: 500;
        margin: 0 0 8px 0;
        color: #1d1d1f;
    }

    .placeholder p {
        font-size: 14px;
        margin: 0;
    }

    @media (max-width: 768px) {
        .header {
            padding: 24px 16px 0;
        }

        .header h1 {
            font-size: 24px;
        }

        .content {
            padding: 16px;
        }
    }
</style>
