<script lang="ts">
    import { currentUser } from "$lib/stores/authContext";

    // Reactive user data
    $: userName = $currentUser?.full_name || "User";
    $: userInitial = userName.charAt(0).toUpperCase();
</script>

<header class="header">
    <!-- Left: spacer -->
    <div class="header-left"></div>

    <!-- Right: Actions -->
    <div class="header-right">
        <!-- User -->
        <div class="user-menu">
            <div class="avatar">{userInitial}</div>
            <span class="user-name">{userName}</span>
        </div>
    </div>
</header>

<style>
    .header {
        height: var(--header-height); /* 56px */
        background: var(--surface);
        border-bottom: 1px solid var(--border);
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 var(--page-padding);
    }

    .header-right {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .user-menu {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .avatar {
        width: 32px;
        height: 32px;
        background: var(--onyx, #1D1D1F);
        color: #fff;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 13px;
        font-weight: 600;
    }

    .user-name {
        font-size: 13px;
        font-weight: 500;
        color: var(--text-primary);
    }
</style>