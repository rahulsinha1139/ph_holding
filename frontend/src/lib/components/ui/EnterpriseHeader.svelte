<script lang="ts">
    import { currentUser } from "$lib/stores/authContext";
    import { setTextScalePreset, textScalePreset, type TextScalePreset } from "$lib/stores/textScale";

    export let title = "";
    export let subtitle = "";

    // Reactive user data
    $: userName = $currentUser?.full_name || "User";
    $: userInitial = userName.charAt(0).toUpperCase();

    const textSizeButtons: Array<{ preset: TextScalePreset; label: string; title: string }> = [
        { preset: "standard", label: "A", title: "Standard text" },
        { preset: "comfortable", label: "A+", title: "Comfortable text" },
        { preset: "large", label: "A++", title: "Large text" },
    ];
</script>

<header class="header">
    <!-- Left: current workspace title -->
    <div class="header-left" aria-live="polite">
        {#if title}
            <strong>{title}</strong>
            {#if subtitle}
                <span>{subtitle}</span>
            {/if}
        {/if}
    </div>

    <!-- Right: Actions -->
    <div class="header-right">
        <div class="text-size-control" aria-label="Text size">
            {#each textSizeButtons as option}
                <button
                    type="button"
                    class:active={$textScalePreset === option.preset}
                    title={option.title}
                    aria-label={option.title}
                    aria-pressed={$textScalePreset === option.preset}
                    on:click={() => setTextScalePreset(option.preset)}
                >
                    {option.label}
                </button>
            {/each}
        </div>

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

    .header-left {
        min-width: 0;
        display: flex;
        align-items: baseline;
        gap: 10px;
    }

    .header-left strong {
        color: var(--text-primary);
        font-family: var(--font-display);
        font-size: var(--section-title-size-scaled, 18px);
        font-weight: 700;
        letter-spacing: 0;
        white-space: nowrap;
    }

    .header-left span {
        color: var(--text-muted);
        font-size: var(--meta-text-size-scaled, 12px);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .text-size-control {
        display: inline-flex;
        align-items: center;
        gap: 2px;
        padding: 2px;
        border: 1px solid var(--border);
        border-radius: 8px;
        background: var(--surface-elevated);
    }

    .text-size-control button {
        min-width: 34px;
        height: 28px;
        border: 0;
        border-radius: 6px;
        background: transparent;
        color: var(--text-secondary);
        font-family: var(--font-display);
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
    }

    .text-size-control button:nth-child(2) {
        font-size: 13px;
    }

    .text-size-control button:nth-child(3) {
        font-size: 14px;
    }

    .text-size-control button:hover,
    .text-size-control button.active {
        background: var(--onyx, #1d1d1f);
        color: #fff;
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
