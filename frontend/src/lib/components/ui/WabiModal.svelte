<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { easeSabi } from '$lib/design-system/asymmetrica';

  export let open: boolean = false;
  export let title: string = '';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';
  export let closeOnOverlay: boolean = true;
  export let showClose: boolean = true;

  const dispatch = createEventDispatcher();

  function close() {
    open = false;
    dispatch('close');
  }

  function handleOverlayClick(e: MouseEvent) {
    if (closeOnOverlay && e.target === e.currentTarget) {
      close();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      close();
    }
  }

  const sizes = {
    sm: '400px',
    md: '550px',
    lg: '750px',
    xl: '950px',
  };
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div
    class="modal-overlay"
    on:click={handleOverlayClick}
    in:fade={{ duration: 200 }}
    out:fade={{ duration: 150 }}
    role="dialog"
    aria-modal="true"
    aria-labelledby="modal-title"
  >
    <div 
      class="modal-container"
      style="max-width: {sizes[size]}"
      in:scale={{ duration: 262, start: 0.95 }}
      out:scale={{ duration: 150, start: 0.95 }}
    >
      {#if title || showClose}
        <header class="modal-header">
          {#if title}
            <h2 id="modal-title" class="modal-title">{title}</h2>
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
    background: rgba(28, 28, 28, 0.4);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-3, 34px);
    z-index: 1000;
  }

  .modal-container {
    background: var(--color-paper, #fdfbf7);
    border: 1px solid rgba(0, 0, 0, 0.08);
    border-radius: var(--space-1, 13px);
    width: 100%;
    max-height: 90vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    box-shadow: 
      0 var(--space-2, 21px) var(--space-4, 55px) rgba(0, 0, 0, 0.15),
      0 0 0 1px rgba(0, 0, 0, 0.05);
    
    /* Rice paper texture */
    background-image: url("data:image/svg+xml,%3Csvg width='100' height='100' viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100' height='100' filter='url(%23noise)' opacity='0.03'/%3E%3C/svg%3E");
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-2, 21px) var(--space-3, 34px);
    border-bottom: 1px solid rgba(0, 0, 0, 0.08);
  }

  .modal-title {
    font-family: var(--font-prose, Georgia, serif);
    font-size: var(--text-xl, 26px);
    font-weight: normal;
    margin: 0;
    color: var(--color-ink, #1c1c1c);
  }

  .modal-close {
    width: 32px;
    height: 32px;
    border: none;
    background: transparent;
    font-size: 24px;
    color: var(--color-ink-light, #57534e);
    cursor: pointer;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s var(--ease-wabi);
  }

  .modal-close:hover {
    background: rgba(0, 0, 0, 0.05);
    color: var(--color-ink, #1c1c1c);
  }

  .modal-body {
    padding: var(--space-3, 34px);
    overflow-y: auto;
    flex: 1;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-1, 13px);
    padding: var(--space-2, 21px) var(--space-3, 34px);
    border-top: 1px solid rgba(0, 0, 0, 0.08);
    background: rgba(0, 0, 0, 0.02);
  }
</style>
