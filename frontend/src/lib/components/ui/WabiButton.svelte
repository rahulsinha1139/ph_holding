<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import WabiSpinner from './WabiSpinner.svelte';

  export let variant: 'primary' | 'secondary' | 'ghost' | 'danger' = 'primary';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let disabled: boolean = false;
  export let loading: boolean = false;
  export let fullWidth: boolean = false;
  export let type: 'button' | 'submit' | 'reset' = 'button';

  const dispatch = createEventDispatcher();

  function handleClick(e: MouseEvent) {
    if (!disabled && !loading) {
      dispatch('click', e);
    }
  }
</script>

<button
  {type}
  class="wabi-button {variant} {size}"
  class:full-width={fullWidth}
  class:loading
  {disabled}
  on:click={handleClick}
  {...$$restProps}
>
  {#if loading}
    <span class="spinner-wrapper">
      <WabiSpinner size="sm" color={variant === 'primary' ? '#fdfbf7' : '#1c1c1c'} tempo="alert" />
    </span>
  {/if}
  <span class="button-content" class:hidden={loading}>
    <slot />
  </span>
</button>

<style>
  .wabi-button {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--fib-2);
    font-family: var(--font-prose, Georgia, serif);
    border-radius: var(--fib-1);
    cursor: pointer;
    transition: all 0.2s var(--ease-wabi, cubic-bezier(0, 0, 0.2, 1));
    white-space: nowrap;
  }

  /* Sizes */
  .wabi-button.sm {
    padding: 6px var(--fib-2);
    font-size: var(--text-sm, 12px);
  }

  .wabi-button.md {
    padding: 10px var(--fib-3);
    font-size: var(--text-base, 16px);
  }

  .wabi-button.lg {
    padding: var(--fib-2) var(--fib-4);
    font-size: var(--text-lg, 20px);
  }

  /* Variants */
  .wabi-button.primary {
    background: var(--color-ink, #1c1c1c);
    color: var(--color-paper, #fdfbf7);
    border: 1px solid var(--color-ink, #1c1c1c);
  }

  .wabi-button.primary:hover:not(:disabled) {
    background: #2d2d2d;
    border-color: #2d2d2d;
  }

  .wabi-button.secondary {
    background: transparent;
    color: var(--color-ink, #1c1c1c);
    border: 1px solid var(--color-ink, #1c1c1c);
  }

  .wabi-button.secondary:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.05);
  }

  .wabi-button.ghost {
    background: transparent;
    color: var(--color-ink, #1c1c1c);
    border: 1px solid transparent;
  }

  .wabi-button.ghost:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.05);
  }

  .wabi-button.danger {
    background: var(--color-danger, #ef4444);
    color: white;
    border: 1px solid var(--color-danger, #ef4444);
  }

  .wabi-button.danger:hover:not(:disabled) {
    background: #dc2626;
    border-color: #dc2626;
  }

  /* States */
  .wabi-button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .wabi-button.loading {
    cursor: wait;
  }

  .wabi-button.full-width {
    width: 100%;
  }

  /* Focus ring */
  .wabi-button:focus-visible {
    outline: 2px solid var(--color-ink, #1c1c1c);
    outline-offset: 2px;
  }

  /* Content */
  .button-content {
    display: inline-flex;
    align-items: center;
    gap: var(--fib-2);
  }

  .button-content.hidden {
    visibility: hidden;
  }

  .spinner-wrapper {
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
  }
</style>
