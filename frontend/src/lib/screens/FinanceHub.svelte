<script>
    import { fade } from "svelte/transition";
    import FinancialDashboard from "./FinancialDashboard.svelte";
    import InvoicesScreen from "./InvoicesScreen.svelte";
    import SupplierInvoicesScreen from "./SupplierInvoicesScreen.svelte";
    import PaymentsScreen from "./PaymentsScreen.svelte";
    import SupplierPaymentsScreen from "./SupplierPaymentsScreen.svelte";

    // Bank Reconciliation Screens (Feature B)
    import BankReconciliationScreen from "./BankReconciliationScreen.svelte";
    // Commented out - developing later
    // import ChequeRegisterScreen from "./ChequeRegisterScreen.svelte";
    // import BookBankReconciliationScreen from "./BookBankReconciliationScreen.svelte";
    // import FXRevaluationScreen from "./FXRevaluationScreen.svelte";
    // import AuditTrailViewer from "./AuditTrailViewer.svelte";
    import ErrorBoundary from "../components/ErrorBoundary.svelte";

    let activeTab = "dashboard";

    const tabs = [
        { id: "dashboard", label: "Dashboard" },
        { id: "invoices", label: "Customer Invoices" },
        { id: "supplier_invoices", label: "Supplier Invoices" },
        { id: "payments", label: "Payments Received" },
        { id: "payments_made", label: "Payments Made" },
        { id: "bank_recon", label: "Bank Recon" },
        // Developing later:
        // { id: "cheques", label: "Cheques" },
        // { id: "book_bank", label: "Book-Bank" },
        // { id: "fx", label: "FX" },
        // { id: "audit", label: "Audit Trail" },
    ];
</script>

<ErrorBoundary name="Finance Hub">
    <div class="hub" in:fade={{ duration: 200 }}>
        <header class="header">
            <h1>Finance Hub</h1>

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
                <FinancialDashboard embedded={true} />
            {:else if activeTab === "invoices"}
                <InvoicesScreen embedded={true} />
            {:else if activeTab === "supplier_invoices"}
                <SupplierInvoicesScreen embedded={true} />
            {:else if activeTab === "payments"}
                <PaymentsScreen embedded={true} />
            {:else if activeTab === "payments_made"}
                <SupplierPaymentsScreen embedded={true} />
            {:else if activeTab === "bank_recon"}
                <BankReconciliationScreen embedded={true} />
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
        background: #fafafa;
    }

    .header {
        padding: 24px 24px 0;
        border-bottom: 1px solid #e5e5e5;
    }

    .header h1 {
        font-family: "Arvo", "Georgia", serif;
        font-size: 24px;
        font-weight: 400;
        color: #1d1d1f;
        margin: 0 0 20px 0;
        letter-spacing: -0.02em;
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
        font-family:
            "DM Sans",
            -apple-system,
            BlinkMacSystemFont,
            sans-serif;
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
        font-family: "DM Sans", sans-serif;
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
