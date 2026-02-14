<script lang="ts">
  /**
   * Enterprise Form Group Component
   * Philosophy: Consistent wrapper for all form fields
   * Provides: Label, slot for input, error/hint display, proper spacing
   */

  export let label: string = '';
  export let required: boolean = false;
  export let error: string = '';
  export let hint: string = '';
  export let labelFor: string = '';
  export let horizontal: boolean = false;
  export let style: string = '';
  export let inline: boolean = false;
</script>

<div class="form-group" class:horizontal class:inline class:has-error={!!error} {style}>
  {#if label}
    <label class="form-label" for={labelFor}>
      {label}
      {#if required}<span class="required" aria-label="required">*</span>{/if}
    </label>
  {/if}

  <div class="form-field">
    <slot />

    {#if hint && !error}
      <span class="form-hint">{hint}</span>
    {/if}

    {#if error}
      <span class="form-error" role="alert">{error}</span>
    {/if}
  </div>
</div>

<style>
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: var(--section-spacing);
  }

  .form-group.horizontal {
    flex-direction: row;
    align-items: flex-start;
    gap: 16px;
  }

  .form-group.horizontal .form-label {
    flex: 0 0 140px;
    padding-top: 8px; /* Align with input */
  }

  .form-group.horizontal .form-field {
    flex: 1;
  }

  .form-label {
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

  .form-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-hint {
    font-size: 12px;
    color: var(--text-muted);
    line-height: var(--line-height-base);
  }

  .form-error {
    font-size: 12px;
    color: #DC2626;
    line-height: var(--line-height-tight);
  }

  /* Remove margin from last form-group in a container */
  .form-group:last-child {
    margin-bottom: 0;
  }
</style>
