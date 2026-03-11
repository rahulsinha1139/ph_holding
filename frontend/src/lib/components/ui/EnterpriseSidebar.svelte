<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from "svelte";
    import { currentUser, permissions } from "$lib/stores/authContext";
    import { GetDBSyncStatus, SyncNowWithProgress, GetDBSyncSettings } from "../../../../wailsjs/go/main/App";
    import SyncProgress from "./SyncProgress.svelte";

    const dispatch = createEventDispatcher();

    // Sync status
    let syncStatus: {
        configured: boolean;
        online: boolean;
        sync_enabled: boolean;
        last_sync: string | null;
    } = {
        configured: false,
        online: false,
        sync_enabled: false,
        last_sync: null
    };
    let syncLoading = false;
    let statusInterval: number;
    let showSyncProgress = false;

    // Poll sync status every 10 seconds
    async function checkSyncStatus() {
        try {
            const status = await GetDBSyncStatus() as Record<string, any>;
            syncStatus = {
                configured: status.configured ?? false,
                online: status.online ?? false,
                sync_enabled: status.sync_enabled ?? false,
                last_sync: status.last_sync ?? null
            };
        } catch (e) {
            console.warn("Failed to get sync status:", e);
        }
    }

    // Manual sync trigger with progress UI
    async function triggerSync() {
        if (syncLoading || !syncStatus.configured) return;
        syncLoading = true;
        showSyncProgress = true;

        try {
            const result = await SyncNowWithProgress();
            console.log("Manual sync result:", result);
            await checkSyncStatus();
        } catch (e) {
            console.error("Manual sync failed:", e);
        } finally {
            syncLoading = false;
            // SyncProgress component auto-hides after completion
        }
    }

    onMount(() => {
        checkSyncStatus();
        statusInterval = setInterval(checkSyncStatus, 10000) as unknown as number;
    });

    onDestroy(() => {
        if (statusInterval) clearInterval(statusInterval);
    });

    // Status indicator color
    $: statusColor = syncStatus.online ? "var(--status-success, #22c55e)"
                   : syncStatus.configured ? "var(--status-warning, #f59e0b)"
                   : "var(--text-muted, #c7c7c7)";
    $: statusLabel = syncStatus.online ? "Cloud sync active"
                   : syncStatus.configured ? "Connecting..."
                   : "Offline mode";

    // Props
    export let currentScreen = "dashboard";

    // Navigation Items with required permissions
    // Items without a permission are always visible
    // Items with permission are HIDDEN (not disabled) if user lacks permission
    // NOTE: Settings removed temporarily due to click bug (all clicks trigger save)
    const navItems = [
        { id: "dashboard", label: "Dashboard", icon: "grid", permission: null },
        { id: "opportunities", label: "Opportunities", icon: "target", permission: "offers:view" },
        { id: "operations", label: "Operations", icon: "layers", permission: "po:view" }, // Supplier-side ops: POs, Supplier Invoices - requires po:view
        { id: "finance", label: "Finance", icon: "bar-chart", permission: "finance:view" }, // HIDDEN for staff/sales
        { id: "relationships", label: "Relationships", icon: "users", permission: "customers:view" },
        { id: "intelligence", label: "Intelligence", icon: "zap", permission: "intelligence:chat" },
        { id: "onedrive_import", label: "OneDrive Import", icon: "folder", permission: "settings:view" },
        { id: "settings", label: "Settings", icon: "settings", permission: "settings:view" },
    ];

    // Check if user has a specific permission
    function hasPermission(perm: string | null): boolean {
        if (!perm) return true; // No permission required
        if (!$permissions || $permissions.length === 0) return false;

        // Wildcard access (admin)
        if ($permissions.includes('*')) return true;

        // Direct match
        if ($permissions.includes(perm)) return true;

        // Resource wildcard (e.g., "finance:*" matches "finance:view")
        const [resource] = perm.split(':');
        if ($permissions.includes(`${resource}:*`)) return true;

        return false;
    }

    // Filter navigation items based on permissions
    // Filters nav items for license-based role access
    $: visibleNavItems = navItems.filter(item => hasPermission(item.permission));

    function navigate(screenId: string) {
        dispatch("navigate", { screen: screenId });
    }

    // User info
    $: userName = $currentUser?.full_name || "Guest";
    $: userInitial = userName.charAt(0).toUpperCase();
    $: userRole = $currentUser?.role_name || $currentUser?.role?.display_name || "User";
</script>

<aside class="sidebar">
    <!-- BRAND HEADER -->
    <div class="sidebar-header">
        <div class="logo-mark">PH</div>
        <span class="brand-name">Trading</span>
    </div>

    <!-- NAVIGATION -->
    <nav class="nav-list">
        {#each visibleNavItems as item}
            <a
                href="#{item.id}"
                class="nav-link"
                class:active={currentScreen === item.id}
                on:click|preventDefault={() => navigate(item.id)}
            >
                <span class="nav-label">{item.label}</span>
                {#if currentScreen === item.id}
                    <div class="active-indicator"></div>
                {/if}
            </a>
        {/each}
    </nav>
    
    <!-- SYNC STATUS -->
    <div class="sync-status" on:click={triggerSync} title={syncStatus.online ? "Click to sync now" : statusLabel}>
        <div class="sync-indicator" style="background-color: {statusColor}"></div>
        <span class="sync-label">{statusLabel}</span>
        {#if syncLoading}
            <span class="sync-spinner"></span>
        {/if}
    </div>

    <!-- USER FOOTER -->
    <div class="sidebar-footer">
         <div class="user-profile">
             <div class="avatar">{userInitial}</div>
             <div class="user-info">
                 <span class="user-name">{userName}</span>
                 <span class="user-role">{userRole}</span>
             </div>
         </div>
    </div>
</aside>

<!-- Sync Progress Modal -->
<SyncProgress bind:show={showSyncProgress} title="Syncing with Cloud" />

<style>
    /* SIDEBAR - CLEAN ENTERPRISE STYLE */
    .sidebar {
        position: fixed;
        left: 0;
        top: 0;
        bottom: 0;
        width: var(--sidebar-width); /* 220px */
        background-color: var(--surface);
        border-right: 1px solid var(--border);
        display: flex;
        flex-direction: column;
        z-index: 50;
    }

    /* HEADER */
    .sidebar-header {
        height: var(--header-height); /* 56px */
        display: flex;
        align-items: center;
        padding: 0 20px;
        border-bottom: 1px solid var(--border);
        gap: 12px;
    }

    .logo-mark {
        width: 28px;
        height: 28px;
        background: var(--carbon);
        color: var(--canvas);
        border-radius: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 700;
        font-size: 12px;
        letter-spacing: -0.02em;
    }

    .brand-name {
        font-weight: 600;
        font-size: 14px;
        color: var(--text-primary);
        letter-spacing: -0.01em;
    }

    /* NAV LIST */
    .nav-list {
        display: flex;
        flex-direction: column;
        padding: 16px 12px;
        gap: 4px;
        flex: 1;
        overflow-y: auto;
    }

    .nav-link {
        display: flex;
        align-items: center;
        justify-content: space-between;
        text-decoration: none;
        color: var(--text-secondary);
        font-size: 14px;
        font-weight: 500;
        padding: 8px 12px;
        border-radius: var(--border-radius-sm);
        transition: all var(--transition-fast);
        position: relative;
    }

    .nav-link:hover {
        background-color: var(--bg-base);
        color: var(--text-primary);
    }

    .nav-link.active {
        background-color: var(--onyx-tint-strong);
        color: var(--onyx);
        font-weight: 600;
    }

    /* FOOTER */
    .sidebar-footer {
        padding: 16px;
        border-top: 1px solid var(--border);
    }

    .user-profile {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .avatar {
        width: 32px;
        height: 32px;
        background: var(--bg-base);
        color: var(--text-secondary);
        border: 1px solid var(--border);
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 12px;
        font-weight: 600;
    }

    .user-info {
        display: flex;
        flex-direction: column;
    }

    .user-name {
        font-size: 13px;
        font-weight: 600;
        color: var(--text-primary);
        line-height: 1.2;
    }

    .user-role {
        font-size: 11px;
        color: var(--text-muted);
    }

    /* SYNC STATUS INDICATOR */
    .sync-status {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 16px;
        border-top: 1px solid var(--border);
        cursor: pointer;
        transition: background-color var(--transition-fast);
    }

    .sync-status:hover {
        background-color: var(--bg-base);
    }

    .sync-indicator {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        flex-shrink: 0;
        animation: pulse 2s ease-in-out infinite;
    }

    @keyframes pulse {
        0%, 100% { opacity: 1; }
        50% { opacity: 0.5; }
    }

    .sync-label {
        font-size: 11px;
        color: var(--text-muted);
        flex: 1;
    }

    .sync-spinner {
        width: 12px;
        height: 12px;
        border: 2px solid var(--border);
        border-top-color: var(--text-secondary);
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }

    /* Mobile Hide */
    @media (max-width: 1024px) {
        .sidebar {
            display: none; /* In production, would be a drawer */
        }
    }
</style>