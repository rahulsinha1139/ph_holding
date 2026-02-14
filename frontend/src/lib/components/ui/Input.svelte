<script lang="ts">
  /**
   * Enterprise Input Component
   * Philosophy: Clean, accessible, consistent with design system
   * Follows: Apple-level polish × Bloomberg-level data density
   */
  import { createEventDispatcher } from 'svelte';

  export let type: 'text' | 'email' | 'password' | 'number' | 'tel' | 'search' | 'date' = 'text';
  export let value: string | number = '';
  export let placeholder: string = '';
  export let label: string = '';
  export let error: string = '';
  export let disabled: boolean = false;
  export let required: boolean = false;
  export let readonly: boolean = false;
  export let id: string = `input-${Math.random().toString(36).substr(2, 9)}`;
  export let name: string = '';
  export let autocomplete: string = '';
  export let maxlength: number | undefined = undefined;

  const dispatch = createEventDispatcher();

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = type === 'number' && target.value ? parseFloat(target.value) : target.value;
    dispatch('input', { value, event: e });
  }

  function handleChange(e: Event) {
    dispatch('change', { value, event: e });
  }

  function handleFocus(e: FocusEvent) {
    dispatch('focus', e);
  }

  function handleBlur(e: FocusEvent) {
    dispatch('blur', e);
  }

  function handleKeyDown(e: KeyboardEvent) {
    dispatch('keydown', e);
  }
</script>

<div class="input-wrapper" class:has-error={!!error} class:disabled>
  {#if label}
    <label for={id} class="label">
      {label}
      {#if required}<span class="required" aria-label="required">*</span>{/if}
    </label>
  {/if}

  <input
    {id}
    {type}
    {name}
    {placeholder}
    {disabled}
    {readonly}
    {required}
    {autocomplete}
    {maxlength}
    value={value}
    class="input"
    on:input={handleInput}
    on:change={handleChange}
    on:focus={handleFocus}
    on:blur={handleBlur}
    on:keydown={handleKeyDown}
    aria-invalid={!!error}
    aria-describedby={error ? `${id}-error` : undefined}
    {...$$restProps}
  />

  {#if error}
    <span id="{id}-error" class="error-message" role="alert">
      {error}
    </span>
  {/if}
</div>

<style>
  .input-wrapper {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 100%;
  }

  .label {
    font-size: var(--label-size);
    font-weight: var(--label-weight);
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    line-height: var(--line-height-tight);
  }

  .required {
    color: #DC2626;
    margin-left: 2px;
  }

  .input {
    width: 100%;
    padding: 8px 12px;
    font-size: 14px;
    font-family: var(--font-family);
    color: var(--text-primary);
    background: var(--surface);
    border: var(--border-width) solid var(--border);
    border-radius: var(--border-radius-sm);
    transition: all var(--transition-fast);
    line-height: var(--line-height-base);
  }

  .input::placeholder {
    color: var(--text-muted);
  }

  .input:hover:not(:disabled):not(:readonly) {
    border-color: var(--text-muted);
  }

  .input:focus {
    outline: none;
    border-color: var(--brand-indigo);
    box-shadow: 0 0 0 3px var(--brand-indigo-tint);
  }

  .input:disabled,
  .input:readonly {
    opacity: 0.5;
    cursor: not-allowed;
    background: var(--surface-elevated);
  }

  .has-error .input {
    border-color: #DC2626;
  }

  .has-error .input:focus {
    border-color: #DC2626;
    box-shadow: 0 0 0 3px rgba(220, 38, 38, 0.1);
  }

  .error-message {
    font-size: 12px;
    color: #DC2626;
    line-height: var(--line-height-tight);
  }

  .disabled {
    opacity: 0.6;
  }

  /* Number input: hide spinners for cleaner look */
  .input[type="number"]::-webkit-inner-spin-button,
  .input[type="number"]::-webkit-outer-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  .input[type="number"] {
    -moz-appearance: textfield;
  }
</style>
