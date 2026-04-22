<script lang="ts">
    import { onMount } from "svelte";
    import { fade } from "svelte/transition";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import { toast } from "../stores/toasts";
    import {
        CreateAccount,
        CreateJournalEntry,
        GetChartOfAccounts,
        GetJournalEntries,
        UpdateAccount,
    } from "../../../wailsjs/go/main/App";

    // Props
    export const embedded = false;

    // Nav
    const views = [
        { id: "dashboard", icon: "", label: "Financial Overview" },
        { id: "coa", icon: "", label: "Chart of Accounts" },
        { id: "journal", icon: "", label: "Journal Entries" },
        { id: "reports", icon: "", label: "Reports" },
    ];
    let currentView = "dashboard";
    let loading = false;

    // Data
    let stats = { assets: 0, liabilities: 0, equity: 0, cash: 0 };
    type AccountView = {
        id?: string;
        code: string;
        name: string;
        type: string;
        balance?: number;
        is_vat: boolean;
        vat_direction?: string;
        account_group?: string;
        parent_account_id?: string;
    };
    type JournalView = {
        id: string;
        date: string;
        desc: string;
        amount: number;
    };
    let accounts: AccountView[] = [];
    let journals: JournalView[] = [];

    // Voucher Form State per FORM_FIELD_DEFINITIONS.md Section 5
    let showVoucherModal = false;
    let newVoucher = {
        type: "Payment", // Payment, Receipt, Journal, Sales, Purchase
        voucher_no: "", // Auto-gen sequence
        date: new Date().toISOString().split("T")[0],
        reference: "", // Cheque No, Transfer Ref
        lines: [{ account: "", debit: 0, credit: 0, narration: "" }],
    };
    const voucherTypes = ["Payment", "Receipt", "Journal", "Sales", "Purchase"];

    function addVoucherLine() {
        newVoucher.lines = [
            ...newVoucher.lines,
            { account: "", debit: 0, credit: 0, narration: "" },
        ];
    }

    function removeVoucherLine(index: number) {
        newVoucher.lines = newVoucher.lines.filter((_, i) => i !== index);
    }

    $: voucherTotal = {
        debit: newVoucher.lines.reduce(
            (sum, l) => sum + (Number(l.debit) || 0),
            0,
        ),
        credit: newVoucher.lines.reduce(
            (sum, l) => sum + (Number(l.credit) || 0),
            0,
        ),
    };
    $: isBalanced =
        voucherTotal.debit === voucherTotal.credit && voucherTotal.debit > 0;

    async function createVoucher() {
        if (!isBalanced) {
            toast.warning("Debits must equal Credits");
            return;
        }

        const lines = newVoucher.lines.map((line) => {
            const account = accounts.find((acc) => acc.code === line.account);
            return {
                account_id: account?.id || "",
                account_name: account?.name || line.account,
                debit: Number(line.debit) || 0,
                credit: Number(line.credit) || 0,
                description: line.narration || newVoucher.reference || newVoucher.type,
            };
        });

        if (lines.some((line) => !line.account_id)) {
            toast.warning("Every journal line must have an account");
            return;
        }

        try {
            await CreateJournalEntry({
                entry_date: new Date(`${newVoucher.date}T00:00:00`).toISOString(),
                description: `${newVoucher.type}: ${newVoucher.reference || "Entry"}`,
                fiscal_year: new Date(newVoucher.date).getFullYear(),
                fiscal_period: new Date(newVoucher.date).getMonth() + 1,
                source_type: "manual",
                source_id: "",
                is_auto_generated: false,
                lines,
            } as any);
            showVoucherModal = false;
            newVoucher = {
                type: "Payment",
                voucher_no: "",
                date: new Date().toISOString().split("T")[0],
                reference: "",
                lines: [{ account: "", debit: 0, credit: 0, narration: "" }],
            };
            toast.success("Voucher created");
            await loadData();
        } catch (e) {
            toast.danger(`Voucher creation failed: ${String(e)}`);
        }
    }

    // === CHART OF ACCOUNTS CRUD per TECHNICAL_SPECIFICATION.md Section 2.4 ===
    let showAccountModal = false;
    let editingAccount: AccountView | null = null;
    let newAccount = {
        code: "",
        name: "",
        type: "Asset",
        parent: "",
        is_vat: false, // Bahrain VAT account flag
    };

    const accountTypes = [
        { value: "Asset", label: "Assets", group: "BS" },
        { value: "Liability", label: "Liabilities", group: "BS" },
        { value: "Equity", label: "Equity", group: "BS" },
        { value: "Revenue", label: "Revenue", group: "PL" },
        { value: "Expense", label: "Expenses", group: "PL" },
    ];

    function accountGroupForType(type: string) {
        return accountTypes.find((t) => t.value === type)?.group || "PL";
    }

    function vatDirectionForAccount(account: { type: string; is_vat: boolean }) {
        if (!account.is_vat) return "";
        return account.type === "Liability" || account.type === "Revenue"
            ? "output"
            : "input";
    }

    function normalizeBackendAccount(account: any): AccountView {
        return {
            id: account.id,
            code: account.account_code || account.code || "",
            name: account.account_name || account.name || "",
            type: account.account_type || account.type || "Asset",
            balance: Number(account.balance) || 0,
            is_vat: Boolean(account.is_vat_account || account.is_vat),
            vat_direction: account.vat_direction || "",
            account_group: account.account_group || accountGroupForType(account.account_type || account.type || "Asset"),
            parent_account_id: account.parent_account_id || "",
        };
    }

    function normalizeBackendJournal(entry: any): JournalView {
        return {
            id: entry.entry_number || entry.id,
            date: entry.entry_date ? new Date(entry.entry_date).toISOString().split("T")[0] : "",
            desc: entry.description || "Journal entry",
            amount: Number(entry.debit_total || entry.credit_total || 0),
        };
    }

    function openAddAccount() {
        editingAccount = null;
        newAccount = {
            code: "",
            name: "",
            type: "Asset",
            parent: "",
            is_vat: false,
        };
        showAccountModal = true;
    }

    function openEditAccount(account: AccountView) {
        editingAccount = account;
        newAccount = {
            code: account.code,
            name: account.name,
            type: account.type,
            parent: account.parent_account_id || "",
            is_vat: account.is_vat,
        };
        showAccountModal = true;
    }

    async function saveAccount() {
        if (!newAccount.code || !newAccount.name) {
            toast.warning("Code and Name are required");
            return;
        }

        try {
            if (editingAccount) {
                if (!editingAccount.id) {
                    toast.warning("This account is missing its backend ID");
                    return;
                }
                await UpdateAccount(editingAccount.id, {
                    account_name: newAccount.name.trim(),
                    account_type: newAccount.type,
                    is_vat_account: newAccount.is_vat,
                    vat_direction: vatDirectionForAccount(newAccount),
                    account_group: accountGroupForType(newAccount.type),
                });
                toast.success("Account updated");
            } else {
                if (accounts.find((a) => a.code === newAccount.code)) {
                    toast.warning("Account code already exists");
                    return;
                }
                await CreateAccount({
                    account_code: newAccount.code.trim(),
                    account_name: newAccount.name.trim(),
                    account_type: newAccount.type,
                    balance: 0,
                    is_active: true,
                    is_vat_account: newAccount.is_vat,
                    vat_direction: vatDirectionForAccount(newAccount),
                    parent_account_id: newAccount.parent || "",
                    account_group: accountGroupForType(newAccount.type),
                } as any);
                toast.success("Account added");
            }
            showAccountModal = false;
            await loadData();
        } catch (e) {
            toast.danger(`Account save failed: ${String(e)}`);
        }
    }

    function deleteAccount(_code: string) {
        toast.warning("Account deletion is not enabled from this screen yet");
    }

    // === BAHRAIN VAT LOGIC per TECHNICAL_SPECIFICATION.md Section 2.4 ===
    const VAT_RATE = 0.1; // Bahrain 10% VAT (as of 2022)

    // Group accounts by type for CoA view
    $: accountsByType = accountTypes.map((t) => ({
        ...t,
        accounts: accounts.filter((a) => a.type === t.value),
    }));

    // VAT Summary calculation
    $: vatAccounts = accounts.filter((a) => a.is_vat);
    $: outputVAT = journals
        .filter((j) => j.desc.toLowerCase().includes("sales"))
        .reduce((sum, j) => sum + j.amount * VAT_RATE, 0);
    $: inputVAT = journals
        .filter((j) => j.desc.toLowerCase().includes("purchase"))
        .reduce((sum, j) => sum + j.amount * VAT_RATE, 0);
    $: vatPayable = outputVAT - inputVAT;

    async function loadData() {
        loading = true;
        try {
            if (!window.go) {
                stats = {
                    assets: 1250000,
                    liabilities: 450000,
                    equity: 800000,
                    cash: 120000,
                };
                accounts = [
                    {
                        code: "1000",
                        name: "Cash",
                        type: "Asset",
                        is_vat: false,
                    },
                    {
                        code: "1100",
                        name: "Accounts Receivable",
                        type: "Asset",
                        is_vat: false,
                    },
                    {
                        code: "1200",
                        name: "Inventory",
                        type: "Asset",
                        is_vat: false,
                    },
                    {
                        code: "2000",
                        name: "Accounts Payable",
                        type: "Liability",
                        is_vat: false,
                    },
                    {
                        code: "2100",
                        name: "VAT Payable (Output)",
                        type: "Liability",
                        is_vat: true,
                    },
                    {
                        code: "2110",
                        name: "VAT Receivable (Input)",
                        type: "Asset",
                        is_vat: true,
                    },
                    {
                        code: "3000",
                        name: "Owners Equity",
                        type: "Equity",
                        is_vat: false,
                    },
                    {
                        code: "3100",
                        name: "Retained Earnings",
                        type: "Equity",
                        is_vat: false,
                    },
                    {
                        code: "4000",
                        name: "Sales Revenue",
                        type: "Revenue",
                        is_vat: false,
                    },
                    {
                        code: "4100",
                        name: "Service Revenue",
                        type: "Revenue",
                        is_vat: false,
                    },
                    {
                        code: "5000",
                        name: "Cost of Goods Sold",
                        type: "Expense",
                        is_vat: false,
                    },
                    {
                        code: "5100",
                        name: "Salaries Expense",
                        type: "Expense",
                        is_vat: false,
                    },
                    {
                        code: "5200",
                        name: "Rent Expense",
                        type: "Expense",
                        is_vat: false,
                    },
                ];
                journals = [
                    {
                        id: "JE-001",
                        date: "2025-01-01",
                        desc: "Opening Balance",
                        amount: 100000,
                    },
                    {
                        id: "JE-002",
                        date: "2025-01-15",
                        desc: "Sales Invoice #101",
                        amount: 5000,
                    },
                ];
            } else {
                const [backendAccounts, backendJournals] = await Promise.all([
                    GetChartOfAccounts("All"),
                    GetJournalEntries(new Date().getFullYear(), 0, null as any, 100),
                ]);
                accounts = backendAccounts.map(normalizeBackendAccount);
                journals = backendJournals.map(normalizeBackendJournal);
                stats = {
                    assets: accounts
                        .filter((account) => account.type === "Asset")
                        .reduce((sum, account) => sum + (Number(account.balance) || 0), 0),
                    liabilities: accounts
                        .filter((account) => account.type === "Liability")
                        .reduce((sum, account) => sum + (Number(account.balance) || 0), 0),
                    equity: accounts
                        .filter((account) => account.type === "Equity")
                        .reduce((sum, account) => sum + (Number(account.balance) || 0), 0),
                    cash: accounts
                        .filter((account) => account.type === "Asset" && /cash|bank/i.test(account.name))
                        .reduce((sum, account) => sum + (Number(account.balance) || 0), 0),
                };
            }
        } catch (e) {
            toast.danger(`Accounting load failed: ${String(e)}`);
        } finally {
            loading = false;
        }
    }

    onMount(loadData);

    function formatCurrency(n) {
        return Number(n).toLocaleString("en-BH", {
            style: "currency",
            currency: "BHD",
        });
    }
</script>

<div class="page">
    <header class="header">
        <div class="header-content">
            <h1>Accounting.</h1>
            <p class="subtitle">General Ledger & Financials</p>
        </div>
        <div class="actions">
            {#if currentView === "journal"}
                <button
                    class="btn-primary"
                    on:click={() => (showVoucherModal = true)}
                    >+ New Entry</button
                >
            {:else if currentView === "coa"}
                <button class="btn-primary" on:click={openAddAccount}
                    >+ New Account</button
                >
            {/if}
        </div>
    </header>

    <div class="layout-split">
        <aside class="sidebar">
            <nav class="nav-menu">
                {#each views as v}
                    <button
                        class="nav-item"
                        class:active={currentView === v.id}
                        on:click={() => (currentView = v.id)}
                    >
                        <span class="icon">{v.icon}</span>
                        {v.label}
                    </button>
                {/each}
            </nav>

            <div class="quick-stats">
                <div class="stat-row">
                    <span class="lbl">Cash</span>
                    <span class="val">{formatCurrency(stats.cash)}</span>
                </div>
            </div>
        </aside>

        <main class="main-view">
            {#if loading}
                <div class="loading"><WabiSpinner size="lg" /></div>
            {:else if currentView === "dashboard"}
                <div class="dashboard-grid" in:fade>
                    <div class="card">
                        <h3>Total Assets</h3>
                        <div class="big-num">
                            {formatCurrency(stats.assets)}
                        </div>
                    </div>
                    <div class="card">
                        <h3>Liabilities</h3>
                        <div class="big-num">
                            {formatCurrency(stats.liabilities)}
                        </div>
                    </div>
                    <div class="card">
                        <h3>Equity</h3>
                        <div class="big-num">
                            {formatCurrency(stats.equity)}
                        </div>
                    </div>
                </div>
            {:else if currentView === "coa"}
                <div class="coa-view" in:fade>
                    <!-- VAT Summary Card -->
                    <div class="vat-summary-card">
                        <h4>Bahrain VAT Summary (10%)</h4>
                        <div class="vat-row">
                            <span>Output VAT (Sales)</span>
                            <span class="mono">{formatCurrency(outputVAT)}</span
                            >
                        </div>
                        <div class="vat-row">
                            <span>Input VAT (Purchases)</span>
                            <span class="mono">-{formatCurrency(inputVAT)}</span
                            >
                        </div>
                        <div class="vat-row total">
                            <span>VAT Payable</span>
                            <span
                                class="mono {vatPayable >= 0
                                    ? 'negative'
                                    : 'positive'}"
                                >{formatCurrency(vatPayable)}</span
                            >
                        </div>
                    </div>

                    <!-- Grouped Accounts -->
                    {#each accountsByType as group}
                        <div class="account-group">
                            <div class="group-header">
                                <span class="group-title">{group.label}</span>
                                <span class="group-badge">{group.group}</span>
                                <span class="group-count"
                                    >{group.accounts.length}</span
                                >
                            </div>
                            <div class="account-list">
                                {#each group.accounts as acc}
                                    <div class="account-row">
                                        <span class="code">{acc.code}</span>
                                        <span class="name">
                                            {acc.name}
                                            {#if acc.is_vat}<span
                                                    class="vat-tag">VAT</span
                                                >{/if}
                                        </span>
                                        <div class="row-actions">
                                            <button
                                                class="btn-icon"
                                                on:click={() =>
                                                    openEditAccount(acc)}
                                                >Edit</button
                                            >
                                            <button
                                                class="btn-icon danger"
                                                on:click={() =>
                                                    deleteAccount(acc.code)}
                                                >Delete</button
                                            >
                                        </div>
                                    </div>
                                {/each}
                                {#if group.accounts.length === 0}
                                    <div class="empty-group">No accounts</div>
                                {/if}
                            </div>
                        </div>
                    {/each}
                </div>
            {:else if currentView === "journal"}
                <div class="table-container" in:fade>
                    <table>
                        <thead
                            ><tr
                                ><th>ID</th><th>Date</th><th>Description</th><th
                                    class="right">Amount</th
                                ></tr
                            ></thead
                        >
                        <tbody>
                            {#each journals as j}
                                <tr>
                                    <td class="mono">{j.id}</td>
                                    <td class="mono">{j.date}</td>
                                    <td>{j.desc}</td>
                                    <td class="right mono"
                                        >{formatCurrency(j.amount)}</td
                                    >
                                </tr>
                            {/each}
                        </tbody>
                    </table>
                </div>
            {:else if currentView === "reports"}
                <div class="reports-grid" in:fade>
                    <div class="report-card">
                        <div class="icon">P&L</div>
                        <h4>Profit & Loss</h4>
                        <button class="btn-sm">Generate</button>
                    </div>
                    <div class="report-card">
                        <div class="icon">Balance</div>
                        <h4>Balance Sheet</h4>
                        <button class="btn-sm">Generate</button>
                    </div>
                    <div class="report-card">
                        <div class="icon">VAT</div>
                        <h4>VAT Return</h4>
                        <button class="btn-sm">Generate</button>
                    </div>
                </div>
            {/if}
        </main>
    </div>
</div>

<!-- Voucher Entry Modal -->
{#if showVoucherModal}
    <div
        class="modal-backdrop"
        role="button"
        tabindex="0"
        on:click={() => (showVoucherModal = false)}
        on:keydown={(event) =>
            (event.key === "Enter" || event.key === " ") &&
            (showVoucherModal = false)}
    >
        <div class="modal-card" role="presentation" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
            <h3>New Accounting Entry</h3>
            <div class="form-scroll">
                <!-- HEADER SECTION -->
                <div class="section-label">Header</div>
                <div class="form-row">
                    <div class="form-group">
                        <label for="accounting-voucher-type">Voucher Type</label>
                        <select
                            id="accounting-voucher-type"
                            bind:value={newVoucher.type}
                            class="input-clean"
                        >
                            {#each voucherTypes as t}
                                <option>{t}</option>
                            {/each}
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="accounting-voucher-date">Date</label>
                        <input
                            id="accounting-voucher-date"
                            type="date"
                            bind:value={newVoucher.date}
                            class="input-clean"
                        />
                    </div>
                </div>
                <div class="form-group">
                    <label for="accounting-voucher-reference">Reference (Cheque No / Transfer Ref)</label>
                    <input
                        id="accounting-voucher-reference"
                        type="text"
                        bind:value={newVoucher.reference}
                        class="input-clean"
                        placeholder="e.g. CHQ-12345"
                    />
                </div>

                <!-- LINES SECTION -->
                <div class="section-label">
                    <span>Lines</span>
                    <button
                        type="button"
                        class="btn-add-line"
                        on:click={addVoucherLine}>+ Add Line</button
                    >
                </div>
                <div class="lines-table">
                    <div class="lines-header">
                        <span>Account</span>
                        <span>Debit</span>
                        <span>Credit</span>
                        <span></span>
                    </div>
                    {#each newVoucher.lines as line, i}
                        <div class="lines-row">
                            <select
                                bind:value={line.account}
                                class="input-clean"
                            >
                                <option value="">Select Account</option>
                                {#each accounts as acc}
                                    <option value={acc.code}
                                        >{acc.code} - {acc.name}</option
                                    >
                                {/each}
                            </select>
                            <input
                                type="number"
                                bind:value={line.debit}
                                class="input-clean"
                                placeholder="0.00"
                            />
                            <input
                                type="number"
                                bind:value={line.credit}
                                class="input-clean"
                                placeholder="0.00"
                            />
                            <button
                                type="button"
                                class="btn-remove"
                                on:click={() => removeVoucherLine(i)}>×</button
                            >
                        </div>
                    {/each}
                </div>

                <!-- TOTALS -->
                <div class="totals-row">
                    <span class="lbl">Totals:</span>
                    <span class="mono"
                        >Dr {formatCurrency(voucherTotal.debit)}</span
                    >
                    <span class="mono"
                        >Cr {formatCurrency(voucherTotal.credit)}</span
                    >
                    {#if isBalanced}
                        <span class="badge success">Balanced</span>
                    {:else}
                        <span class="badge warning">Unbalanced</span>
                    {/if}
                </div>
            </div>

            <div class="modal-actions">
                <button
                    class="btn-ghost"
                    on:click={() => (showVoucherModal = false)}>Cancel</button
                >
                <button
                    class="btn-primary"
                    on:click={createVoucher}
                    disabled={!isBalanced}
                >
                    Create Entry
                </button>
            </div>
        </div>
    </div>
{/if}

<!-- Account Modal (Add/Edit) -->
{#if showAccountModal}
    <div
        class="modal-backdrop"
        role="button"
        tabindex="0"
        on:click={() => (showAccountModal = false)}
        on:keydown={(event) =>
            (event.key === "Enter" || event.key === " ") &&
            (showAccountModal = false)}
    >
        <div class="modal-card" role="presentation" tabindex="-1" on:click|stopPropagation on:keydown|stopPropagation>
            <h3>{editingAccount ? "Edit Account" : "Add Account"}</h3>

            <div class="form-row">
                <div class="form-group">
                    <label for="accounting-account-code">Account Code *</label>
                    <input
                        id="accounting-account-code"
                        type="text"
                        bind:value={newAccount.code}
                        class="input-clean"
                        placeholder="e.g. 1000"
                        disabled={!!editingAccount}
                    />
                </div>
                <div class="form-group">
                    <label for="accounting-account-type">Account Type *</label>
                    <select id="accounting-account-type" bind:value={newAccount.type} class="input-clean">
                        {#each accountTypes as t}
                            <option value={t.value}>{t.label}</option>
                        {/each}
                    </select>
                </div>
            </div>

            <div class="form-group">
                <label for="accounting-account-name">Account Name *</label>
                <input
                    id="accounting-account-name"
                    type="text"
                    bind:value={newAccount.name}
                    class="input-clean"
                    placeholder="e.g. Cash in Bank"
                />
            </div>

            <div class="form-group checkbox-group">
                <label class="checkbox-label">
                    <input type="checkbox" bind:checked={newAccount.is_vat} />
                    <span>VAT Account (for Bahrain VAT tracking)</span>
                </label>
            </div>

            <div class="modal-actions">
                <button
                    class="btn-ghost"
                    on:click={() => (showAccountModal = false)}>Cancel</button
                >
                <button class="btn-primary" on:click={saveAccount}>
                    {editingAccount ? "Update" : "Add Account"}
                </button>
            </div>
        </div>
    </div>
{/if}

<style>
    .page {
        padding: var(--page-padding);
        height: 100vh;
        background: var(--paper);
        color: var(--ink);
        display: flex;
        flex-direction: column;
        box-sizing: border-box;
    }

    .header {
        display: flex;
        justify-content: space-between;
        align-items: flex-end;
        margin-bottom: var(--space-4);
        flex-shrink: 0;
    }
    h1 {
        font-size: var(--text-3xl);
        font-weight: var(--font-weight-light);
        margin: 0;
        letter-spacing: -0.02em;
    }
    .subtitle {
        color: var(--ink-faint);
        margin-top: var(--space-1);
        font-size: var(--text-sm);
    }
    .btn-primary {
        background: var(--ink);
        color: var(--paper);
        border: none;
        padding: 8px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
        font-size: 13px;
    }

    .layout-split {
        display: grid;
        grid-template-columns: 240px 1fr;
        gap: var(--space-4);
        flex: 1;
        min-height: 0;
    }

    .sidebar {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        border-right: 1px solid var(--border-subtle);
        padding-right: var(--space-3);
    }
    .nav-menu {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }
    .nav-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px;
        border: none;
        background: transparent;
        color: var(--ink-light);
        cursor: pointer;
        border-radius: 8px;
        font-size: 13px;
        text-align: left;
    }
    .nav-item:hover {
        background: var(--paper-subtle);
        color: var(--ink);
    }
    .nav-item.active {
        background: var(--ink);
        color: var(--paper);
        font-weight: 500;
    }

    .quick-stats {
        background: var(--paper-subtle);
        padding: var(--space-4);
        border-radius: 12px;
    }
    .stat-row {
        display: flex;
        justify-content: space-between;
        font-size: 12px;
    }
    .stat-row .lbl {
        color: var(--ink-light);
    }
    .stat-row .val {
        font-weight: 600;
        font-family: var(--font-mono);
    }

    .main-view {
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        padding-left: var(--space-2);
    }
    .loading {
        display: flex;
        justify-content: center;
        padding: 40px;
    }

    /* Dashboard */
    .dashboard-grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: var(--space-4);
    }
    .card {
        background: var(--paper-subtle);
        padding: var(--space-4);
        border-radius: 16px;
        border: 1px solid var(--border-subtle);
    }
    .card h3 {
        margin: 0 0 12px;
        font-size: 11px;
        text-transform: uppercase;
        color: var(--ink-light);
    }
    .big-num {
        font-size: 28px;
        font-weight: 300;
        font-family: var(--font-mono);
    }

    /* Tables */
    .table-container {
        background: var(--paper);
        border-radius: 12px;
        border: 1px solid var(--border-medium);
        overflow: hidden;
    }
    table {
        width: 100%;
        border-collapse: collapse;
        font-size: 13px;
    }
    th {
        text-align: left;
        padding: 10px 16px;
        background: var(--paper-subtle);
        color: var(--ink-light);
        font-weight: 500;
        font-size: 11px;
        text-transform: uppercase;
        position: sticky;
        top: 0;
    }
    td {
        padding: 10px 16px;
        border-bottom: 1px solid var(--border-subtle);
    }
    .right {
        text-align: right;
    }
    .mono {
        font-family: var(--font-mono);
        font-size: 12px;
        color: var(--ink-light);
    }
    .badge {
        padding: 2px 8px;
        border-radius: 4px;
        background: var(--paper-subtle);
        border: 1px solid var(--border-medium);
        font-size: 10px;
        text-transform: uppercase;
    }
    /* Reports */
    .reports-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
        gap: var(--space-4);
    }
    .report-card {
        background: var(--paper-subtle);
        padding: 20px;
        border-radius: 12px;
        border: 1px solid var(--border-subtle);
        text-align: center;
    }
    .report-card .icon {
        font-size: 24px;
        margin-bottom: 12px;
    }
    .report-card h4 {
        margin: 0 0 16px;
        font-weight: 500;
    }
    .btn-sm {
        padding: 6px 16px;
        border: 1px solid var(--border-medium);
        background: var(--paper);
        border-radius: 6px;
        cursor: pointer;
        font-size: 12px;
    }

    /* Modal */
    .modal-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.6);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
        backdrop-filter: blur(4px);
    }
    .modal-card {
        background: var(--paper);
        padding: 24px;
        border-radius: 16px;
        width: 640px;
        max-width: 95%;
        max-height: 85vh;
        display: flex;
        flex-direction: column;
        box-shadow: var(--shadow-xl);
    }
    .modal-card h3 {
        margin: 0 0 16px;
        font-weight: 400;
        font-family: var(--font-heading);
    }
    .form-scroll {
        flex: 1;
        overflow-y: auto;
        max-height: 55vh;
    }
    .section-label {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: var(--ink-light);
        font-weight: 500;
        margin-top: 16px;
        margin-bottom: 8px;
        padding-bottom: 4px;
        border-bottom: 1px solid var(--border-subtle);
        display: flex;
        justify-content: space-between;
        align-items: center;
    }
    .section-label:first-child {
        margin-top: 0;
    }
    .form-group {
        margin-bottom: 12px;
        flex: 1;
    }
    .form-group label {
        font-size: 10px;
        text-transform: uppercase;
        color: var(--ink-light);
        display: block;
        margin-bottom: 3px;
    }
    .form-row {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px;
    }
    .input-clean {
        width: 100%;
        padding: 8px;
        border: 1px solid var(--border-medium);
        border-radius: 6px;
        box-sizing: border-box;
        font-size: 13px;
    }

    /* Lines Table */
    .btn-add-line {
        font-size: 11px;
        padding: 2px 10px;
        background: var(--paper);
        border: 1px solid var(--border-medium);
        border-radius: 4px;
        cursor: pointer;
    }
    .lines-table {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }
    .lines-header {
        display: grid;
        grid-template-columns: 2fr 1fr 1fr 32px;
        gap: 8px;
        font-size: 10px;
        text-transform: uppercase;
        color: var(--ink-light);
        padding: 0 4px;
    }
    .lines-row {
        display: grid;
        grid-template-columns: 2fr 1fr 1fr 32px;
        gap: 8px;
        align-items: center;
    }
    .btn-remove {
        background: transparent;
        border: none;
        font-size: 18px;
        cursor: pointer;
        color: var(--ink-light);
        padding: 0;
        width: 24px;
        height: 24px;
        line-height: 24px;
    }
    .btn-remove:hover {
        color: #dc2626;
    }

    /* Totals Row */
    .totals-row {
        display: flex;
        gap: 16px;
        align-items: center;
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid var(--border-medium);
        font-size: 12px;
    }
    .totals-row .lbl {
        color: var(--ink-light);
    }
    .badge.success {
        background: #dcfce7;
        color: #166534;
    }
    .badge.warning {
        background: #fef9c3;
        color: #854d0e;
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: 12px;
        margin-top: 16px;
        padding-top: 16px;
        border-top: 1px solid var(--border-subtle);
    }
    .btn-ghost {
        background: transparent;
        border: none;
        cursor: pointer;
        font-size: 13px;
        padding: 8px 16px;
    }

    /* CoA View */
    .coa-view {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }
    .vat-summary-card {
        background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%);
        border-radius: var(--radius-lg);
        padding: var(--space-4);
        border: 1px solid #fcd34d;
    }
    .vat-summary-card h4 {
        margin: 0 0 var(--space-2);
        font-size: 14px;
    }
    .vat-row {
        display: flex;
        justify-content: space-between;
        padding: 6px 0;
        font-size: 13px;
    }
    .vat-row.total {
        border-top: 1px solid rgba(0, 0, 0, 0.1);
        margin-top: 8px;
        padding-top: 8px;
        font-weight: 600;
    }
    .positive {
        color: #22c55e;
    }
    .negative {
        color: #dc2626;
    }

    .account-group {
        background: var(--paper-subtle);
        border-radius: var(--radius-lg);
        overflow: hidden;
    }
    .group-header {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-3);
        background: rgba(0, 0, 0, 0.03);
        border-bottom: 1px solid var(--border-subtle);
    }
    .group-title {
        font-weight: 600;
        font-size: 13px;
    }
    .group-badge {
        background: var(--ink);
        color: var(--paper);
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 10px;
        font-weight: 600;
    }
    .group-count {
        margin-left: auto;
        color: var(--ink-light);
        font-size: 12px;
    }
    .account-list {
        padding: var(--space-1);
    }
    .account-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: 8px var(--space-2);
        border-radius: 6px;
    }
    .account-row:hover {
        background: rgba(0, 0, 0, 0.03);
    }
    .account-row .code {
        font-family: var(--font-mono);
        font-size: 12px;
        color: var(--ink-light);
        width: 60px;
    }
    .account-row .name {
        flex: 1;
        font-size: 13px;
    }
    .vat-tag {
        display: inline-block;
        background: #fef3c7;
        color: #92400e;
        padding: 2px 6px;
        border-radius: 4px;
        font-size: 9px;
        font-weight: 600;
        margin-left: 8px;
    }
    .row-actions {
        display: flex;
        gap: 4px;
        opacity: 0;
        transition: opacity 0.15s;
    }
    .account-row:hover .row-actions {
        opacity: 1;
    }
    .btn-icon {
        background: transparent;
        border: none;
        cursor: pointer;
        padding: 4px;
        font-size: 14px;
    }
    .btn-icon.danger:hover {
        color: #dc2626;
    }
    .empty-group {
        padding: var(--space-2);
        color: var(--ink-light);
        font-size: 12px;
        font-style: italic;
    }
    .checkbox-group {
        margin-top: var(--space-2);
    }
    .checkbox-label {
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
        font-size: 13px;
    }
</style>
