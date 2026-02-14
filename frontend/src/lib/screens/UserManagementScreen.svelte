<script lang="ts">
    import { onMount } from "svelte";
    import { fade } from "svelte/transition";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import {
        ListUsers,
        ListRoles,
        GetAuditLogs,
        CreateUser,
    } from "../../../wailsjs/go/main/App";
    import { toast } from "../stores/toasts";

    let activeTab = "users"; // users, roles, audit
    let loading = false;

    // Data
    let users = [];
    let roles = [];
    let auditLogs = [];
    let searchQuery = "";

    async function loadData() {
        loading = true;
        try {
            if (!window.go) {
                // Mock
                users = [
                    {
                        id: 1,
                        username: "admin",
                        full_name: "Admin User",
                        role: "Administrator",
                        status: "Active",
                    },
                    {
                        id: 2,
                        username: "jdoe",
                        full_name: "John Doe",
                        role: "Sales",
                        status: "Active",
                    },
                    {
                        id: 3,
                        username: "asmith",
                        full_name: "Alice Smith",
                        role: "Operations",
                        status: "Inactive",
                    },
                ];
                roles = [
                    { id: 1, name: "Administrator" },
                    { id: 2, name: "Sales" },
                    { id: 3, name: "Operations" },
                ];
                auditLogs = [
                    {
                        ts: new Date().toISOString(),
                        action: "LOGIN",
                        user: "admin",
                        details: "Successful login",
                    },
                    {
                        ts: new Date().toISOString(),
                        action: "UPDATE_USER",
                        user: "admin",
                        details: "Updated user jdoe",
                    },
                ];
            } else {
                const [u, r] = await Promise.all([
                    ListUsers().catch(() => []),
                    ListRoles().catch(() => []),
                ]);
                users = u || [];
                roles = r || [];
            }
        } catch (e) {
            toast.danger("Load failed");
        } finally {
            loading = false;
        }
    }

    async function fetchAudit() {
        if (auditLogs.length > 0) return; // cache slightly
        loading = true;
        try {
            if (window.go) {
                auditLogs = (await GetAuditLogs(50, "all", "")) || [];
            }
        } catch (e) {
        } finally {
            loading = false;
        }
    }

    $: filteredUsers = users.filter(
        (u) =>
            !searchQuery ||
            u.full_name.toLowerCase().includes(searchQuery.toLowerCase()),
    );

    onMount(loadData);
</script>

<div class="page">
    <header class="header">
        <div class="header-content">
            <h1>Team.</h1>
            <p class="subtitle">User Management & Security</p>
        </div>
        {#if activeTab === "users"}
            <button class="btn-primary">+ Add User</button>
        {/if}
    </header>

    <div class="tabs">
        <button
            class="tab"
            class:active={activeTab === "users"}
            on:click={() => (activeTab = "users")}>Users</button
        >
        <button
            class="tab"
            class:active={activeTab === "roles"}
            on:click={() => (activeTab = "roles")}>Roles</button
        >
        <button
            class="tab"
            class:active={activeTab === "audit"}
            on:click={() => {
                activeTab = "audit";
                fetchAudit();
            }}>Audit Logs</button
        >
    </div>

    <main class="content-area">
        {#if loading}
            <div class="loading"><WabiSpinner size="lg" /></div>
        {:else if activeTab === "users"}
            <div class="toolbar">
                <input
                    type="text"
                    placeholder="Search users..."
                    bind:value={searchQuery}
                    class="search-input"
                />
            </div>
            <div class="table-card" in:fade>
                <table>
                    <thead
                        ><tr
                            ><th>User</th><th>Role</th><th>Status</th><th
                                class="right">Actions</th
                            ></tr
                        ></thead
                    >
                    <tbody>
                        {#each filteredUsers as u}
                            <tr>
                                <td>
                                    <div class="user-cell">
                                        <div class="avatar">
                                            {u.full_name[0]}
                                        </div>
                                        <div class="info">
                                            <div class="name">
                                                {u.full_name}
                                            </div>
                                            <div class="sub">@{u.username}</div>
                                        </div>
                                    </div>
                                </td>
                                <td><span class="badge role">{u.role}</span></td
                                >
                                <td
                                    ><span
                                        class="dot {u.status === 'Active'
                                            ? 'green'
                                            : 'red'}"
                                    ></span>
                                    {u.status}</td
                                >
                                <td class="right"
                                    ><button class="btn-sm">Edit</button></td
                                >
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {:else if activeTab === "roles"}
            <div class="grid-roles" in:fade>
                {#each roles as r}
                    <div class="card role-card">
                        <h3>{r.name}</h3>
                        <p>Permissions managed via configuration.</p>
                        <button class="btn-txt">View Permissions</button>
                    </div>
                {/each}
            </div>
        {:else if activeTab === "audit"}
            <div class="table-card" in:fade>
                <table>
                    <thead
                        ><tr
                            ><th>Timestamp</th><th>Action</th><th>User</th><th
                                >Details</th
                            ></tr
                        ></thead
                    >
                    <tbody>
                        {#each auditLogs as log}
                            <tr>
                                <td class="mono"
                                    >{new Date(log.ts).toLocaleString()}</td
                                >
                                <td
                                    ><span class="badge neutral"
                                        >{log.action}</span
                                    ></td
                                >
                                <td class="bold">{log.user}</td>
                                <td class="dim">{log.details}</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {/if}
    </main>
</div>

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
        margin-bottom: var(--space-6);
    }
    h1 {
        font-size: var(--text-5xl);
        font-weight: var(--font-weight-light);
        margin: 0;
        letter-spacing: -0.02em;
    }
    .subtitle {
        color: var(--ink-faint);
        margin-top: var(--space-2);
    }
    .btn-primary {
        background: var(--ink);
        color: var(--paper);
        border: none;
        padding: 10px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
    }

    .tabs {
        display: flex;
        gap: 24px;
        border-bottom: 1px solid var(--border-subtle);
        margin-bottom: 24px;
    }
    .tab {
        background: transparent;
        border: none;
        padding: 12px 0;
        font-size: 14px;
        color: var(--ink-light);
        cursor: pointer;
        position: relative;
    }
    .tab.active {
        color: var(--ink);
        font-weight: 500;
    }
    .tab.active::after {
        content: "";
        position: absolute;
        bottom: -1px;
        left: 0;
        width: 100%;
        height: 2px;
        background: var(--ink);
    }

    .content-area {
        flex: 1;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 16px;
    }
    .loading {
        display: flex;
        justify-content: center;
        padding: 40px;
    }

    .toolbar {
        display: flex;
        margin-bottom: 16px;
    }
    .search-input {
        width: 300px;
        padding: 10px;
        border: 1px solid var(--border-medium);
        border-radius: 8px;
        font-size: 13px;
    }

    .table-card {
        background: var(--paper-subtle);
        border-radius: 12px;
        border: 1px solid var(--border-subtle);
        overflow: hidden;
    }
    table {
        width: 100%;
        border-collapse: collapse;
        font-size: 13px;
    }
    th {
        text-align: left;
        padding: 12px 16px;
        border-bottom: 1px solid var(--border-medium);
        font-size: 11px;
        text-transform: uppercase;
        color: var(--ink-light);
        font-weight: 500;
    }
    td {
        padding: 12px 16px;
        border-bottom: 1px solid var(--border-subtle);
        vertical-align: middle;
    }
    .right {
        text-align: right;
    }

    .user-cell {
        display: flex;
        align-items: center;
        gap: 12px;
    }
    .avatar {
        width: 32px;
        height: 32px;
        background: var(--ink);
        color: var(--paper);
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 600;
        font-size: 14px;
    }
    .info .name {
        font-weight: 500;
    }
    .info .sub {
        font-size: 11px;
        color: var(--ink-light);
    }

    .badge {
        padding: 2px 8px;
        border-radius: 4px;
        border: 1px solid var(--border-medium);
        font-size: 10px;
        text-transform: uppercase;
    }
    .badge.role {
        background: var(--paper);
    }
    .badge.neutral {
        background: rgba(0, 0, 0, 0.05);
        border: none;
    }

    .dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        display: inline-block;
        margin-right: 4px;
    }
    .dot.green {
        background: #22c55e;
    }
    .dot.red {
        background: #ef4444;
    }

    .btn-sm {
        padding: 4px 12px;
        border: 1px solid var(--border-medium);
        background: var(--paper);
        border-radius: 4px;
        cursor: pointer;
        font-size: 11px;
    }

    .grid-roles {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
        gap: 16px;
    }
    .role-card {
        background: var(--paper-subtle);
        padding: 20px;
        border-radius: 12px;
        border: 1px solid var(--border-subtle);
    }
    .role-card h3 {
        margin: 0 0 8px;
        font-size: 16px;
        font-weight: 500;
    }
    .role-card p {
        font-size: 12px;
        color: var(--ink-light);
        margin-bottom: 16px;
    }
    .btn-txt {
        background: transparent;
        border: none;
        color: var(--ink);
        font-size: 12px;
        cursor: pointer;
        padding: 0;
        font-weight: 500;
    }

    .bold {
        font-weight: 500;
    }
    .dim {
        color: var(--ink-light);
    }
    .mono {
        font-family: var(--font-mono);
        font-size: 12px;
        color: var(--ink-light);
    }
</style>
