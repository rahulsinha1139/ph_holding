<script lang="ts">
  import { toasts } from '$lib/stores/toasts';
  import WabiSabiToast from './WabiSabiToast.svelte';
  import { flip } from 'svelte/animate';

  // Position can be customized
  export let position: 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left' = 'top-right';
</script>

<div class="toast-container {position}" data-toast-count={$toasts.length}>
  {#each $toasts as toast (toast.id)}
    <div animate:flip={{ duration: 144 }}>
      <WabiSabiToast
        message={toast.message}
        type={toast.type}
        duration={toast.duration}
        showBrush={toast.showBrush}
        on:dismiss={() => toasts.dismiss(toast.id)}
      />
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 18px;
    pointer-events: none;
  }

  .toast-container > :global(*) {
    pointer-events: none;
  }

  /* Position variants */
  .top-right {
    top: 0;
    right: 0;
    align-items: flex-end;
  }

  .top-left {
    top: 0;
    left: 0;
    align-items: flex-start;
  }

  .bottom-right {
    bottom: 0;
    right: 0;
    align-items: flex-end;
    flex-direction: column-reverse;
  }

  .bottom-left {
    bottom: 0;
    left: 0;
    align-items: flex-start;
    flex-direction: column-reverse;
  }
</style>
