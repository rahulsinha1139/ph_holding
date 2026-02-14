<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';

  export let open: boolean = false;
  export let title: string = '';
  export let size: 'sm' | 'md' | 'lg' | 'full' = 'md';
  export let closable: boolean = true;

  const dispatch = createEventDispatcher();

  let modalElement: HTMLElement;
  let previousFocus: HTMLElement | null = null;

  $: if (open) {
    openModal();
  } else {
    closeModal();
  }

  function openModal() {
    previousFocus = document.activeElement as HTMLElement;
    document.body.style.overflow = 'hidden';

    setTimeout(() => {
      // Focus first focusable element or close button
      const focusable = modalElement?.querySelector(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      ) as HTMLElement;
      focusable?.focus();
    }, 100);
  }

  function closeModal() {
    document.body.style.overflow = '';
    previousFocus?.focus();
  }

  function handleClose() {
    if (closable) {
      open = false;
      dispatch('close');
    }
  }

  function handleBackdropClick(event: MouseEvent) {
    if (closable && event.target === event.currentTarget) {
      handleClose();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && closable) {
      handleClose();
    }

    // Trap focus within modal
    if (event.key === 'Tab' && open) {
      const focusableElements = modalElement?.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (!focusableElements || focusableElements.length === 0) return;

      const firstElement = focusableElements[0] as HTMLElement;
      const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

      if (event.shiftKey && document.activeElement === firstElement) {
        event.preventDefault();
        lastElement.focus();
      } else if (!event.shiftKey && document.activeElement === lastElement) {
        event.preventDefault();
        firstElement.focus();
      }
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
  });

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
    document.body.style.overflow = '';
  });
</script>

{#if open}
  <div
    class="modal-backdrop"
    on:click={handleBackdropClick}
    role="presentation"
  >
    <div
      class="modal modal-{size}"
      bind:this={modalElement}
      role="dialog"
      aria-modal="true"
      aria-labelledby={title ? 'modal-title' : undefined}
    >
      <!-- Header -->
      {#if title || closable || $$slots.header}
        <header class="modal-header">
          {#if $$slots.header}
            <slot name="header" />
          {:else if title}
            <h2 id="modal-title" class="modal-title">{title}</h2>
          {/if}

          {#if closable}
            <button
              class="modal-close"
              on:click={handleClose}
              aria-label="Close modal"
            >
              <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
                <path
                  d="M5 5L15 15M15 5L5 15"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </button>
          {/if}
        </header>
      {/if}

      <!-- Content -->
      <div class="modal-content">
        <slot />
      </div>

      <!-- Footer -->
      {#if $$slots.footer}
        <footer class="modal-footer">
          <slot name="footer" />
        </footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: 16px;
    animation: fadeIn var(--transition-base) var(--easing-smooth);
  }

  .modal {
    background: var(--surface);
    border-radius: var(--border-radius-lg);
    box-shadow: var(--shadow-lg);
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 32px);
    animation: slideUp var(--transition-base) var(--easing-smooth);
  }

  .modal-sm {
    width: 100%;
    max-width: 400px;
  }

  .modal-md {
    width: 100%;
    max-width: 600px;
  }

  .modal-lg {
    width: 100%;
    max-width: 900px;
  }

  .modal-full {
    width: calc(100vw - 32px);
    height: calc(100vh - 32px);
  }

  .modal-header {
    padding: 16px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-shrink: 0;
  }

  .modal-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .modal-close {
    background: none;
    border: none;
    padding: 4px;
    cursor: pointer;
    color: var(--text-secondary);
    transition: color var(--transition-fast);
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
  }

  .modal-close:hover {
    color: var(--text-primary);
    background: var(--surface-elevated);
  }

  .modal-close:focus-visible {
    outline: 2px solid var(--brand-indigo);
    outline-offset: 2px;
  }

  .modal-content {
    padding: 16px;
    overflow-y: auto;
    flex: 1;
    min-height: 0;
  }

  .modal-footer {
    padding: 16px;
    border-top: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    flex-shrink: 0;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes slideUp {
    from {
      transform: translateY(20px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }
</style>
