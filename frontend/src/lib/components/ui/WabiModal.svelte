<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';

  export let open: boolean = false;
  export let title: string = '';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';
  export let closeOnOverlay: boolean = true;
  export let showClose: boolean = true;

  const dispatch = createEventDispatcher();
  const modalTitleId = `wabi-modal-title-${Math.random().toString(36).slice(2, 8)}`;

  function close() {
    open = false;
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      close();
    }
  }

  const sizes = {
    sm: '380px',
    md: '500px',
    lg: '680px',
    xl: '820px',
  };
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="modal-overlay"
    in:fade={{ duration: 200 }}
    out:fade={{ duration: 150 }}
  >
    <button
      type="button"
      class="modal-backdrop"
      aria-label="Close modal"
      tabindex="-1"
      disabled={!closeOnOverlay}
      on:click={close}
    ></button>
    <div
      class="modal-container"
      style="max-width: {sizes[size]}"
      in:fade={{ duration: 150 }}
      out:fade={{ duration: 100 }}
      role="dialog"
      aria-modal="true"
      aria-labelledby={title ? modalTitleId : undefined}
      aria-label={!title ? 'Modal dialog' : undefined}
    >
      {#if title || showClose}
        <header class="modal-header">
          {#if title}
            <h2 id={modalTitleId} class="modal-title">{title}</h2>
          {/if}
          {#if showClose}
            <button class="modal-close" on:click={close} aria-label="Close modal">
              ×
            </button>
          {/if}
        </header>
      {/if}

      <div class="modal-body">
        <slot />
      </div>

      {#if $$slots.footer}
        <footer class="modal-footer">
          <slot name="footer" />
        </footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: calc(var(--header-height, 56px) + 24px) clamp(14px, 2vw, 24px) 24px;
    overflow-y: auto;
    z-index: 10000;
  }

  .modal-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    padding: 0;
    margin: 0;
    background: rgba(28, 28, 28, 0.4);
    backdrop-filter: blur(4px);
    cursor: pointer;
  }

  .modal-backdrop:disabled {
    cursor: default;
  }

  .modal-container {
    position: relative;
    z-index: 1;
    background: var(--surface, #ffffff);
    border: 1px solid rgba(0, 0, 0, 0.08);
    border-radius: 18px;
    width: 100%;
    max-height: calc(100vh - var(--header-height, 56px) - 48px);
    margin: 0 auto;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    box-shadow:
      0 var(--space-2, 21px) var(--space-4, 55px) rgba(0, 0, 0, 0.15),
      0 0 0 1px rgba(0, 0, 0, 0.05);
    font-family: var(--font-body);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 18px 14px;
    border-bottom: 1px solid rgba(0, 0, 0, 0.08);
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--surface, #ffffff);
  }

  .modal-title {
    font-family: var(--font-display);
    font-size: var(--modal-title-size, 22px);
    font-weight: var(--modal-title-weight, 500);
    line-height: 1.2;
    letter-spacing: -0.02em;
    margin: 0;
    color: var(--text-primary, #1c1c1c);
  }

  .modal-close {
    width: 32px;
    height: 32px;
    border: none;
    background: transparent;
    font-size: 24px;
    color: var(--text-secondary, #57534e);
    cursor: pointer;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s var(--ease-wabi);
  }

  .modal-close:hover {
    color: var(--text-primary, #1c1c1c);
  }

  .modal-body {
    padding: 16px 18px 18px;
    overflow-y: auto;
    flex: 1;
    font-family: var(--font-body);
    font-size: var(--modal-body-size, 14px);
    line-height: var(--modal-line-height, 1.6);
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-1, 13px);
    padding: 14px 18px 16px;
    border-top: 1px solid rgba(0, 0, 0, 0.08);
    background: rgba(0, 0, 0, 0.02);
  }

  :global(.modal-container .card:hover) {
    transform: none !important;
    box-shadow: var(--shadow-sm) !important;
  }

  @media (max-width: 720px) {
    .modal-overlay {
      align-items: center;
      padding: 16px;
    }

    .modal-container {
      max-height: calc(100vh - 32px);
    }
  }
</style>
