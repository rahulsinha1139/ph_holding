<script lang="ts">
  export let label: string;
  export let value: string;
  export let meta: string = '';
  export let trend: 'up' | 'down' | 'neutral' = 'neutral';
  export let accent: boolean = false;
</script>

<div class="kpi-card card" class:card-accent={accent}>
  <div class="kpi-content">
    <div class="kpi-label">{label}</div>
    <div class="kpi-value-row">
      <div class="kpi-value">{value}</div>
      {#if trend !== 'neutral'}
        <div class="kpi-trend kpi-trend-{trend}">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            {#if trend === 'up'}
              <path d="M8 4L12 8L8 12M12 8H4" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            {:else}
              <path d="M8 12L4 8L8 4M4 8H12" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            {/if}
          </svg>
        </div>
      {/if}
    </div>
    {#if meta}
      <div class="kpi-meta">{meta}</div>
    {/if}
    <slot name="footer" />
  </div>
</div>

<style>
  .kpi-card {
    height: var(--kpi-card-height);
    display: flex;
    flex-direction: column;
  }

  .kpi-content {
    display: flex;
    flex-direction: column;
    gap: 8px;
    height: 100%;
  }

  .kpi-label {
    font-size: var(--label-size);
    font-weight: var(--label-weight);
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .kpi-value-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .kpi-value {
    font-size: 32px;
    font-weight: 700;
    color: var(--text-primary);
    line-height: 1.2;
  }

  .kpi-trend {
    display: flex;
    align-items: center;
    font-size: var(--label-size);
    font-weight: 600;
  }

  .kpi-trend-up {
    color: #10B981; /* Green */
  }

  .kpi-trend-down {
    color: #EF4444; /* Red */
  }

  .kpi-meta {
    font-size: var(--meta-size);
    color: var(--text-muted);
    margin-top: auto;
  }
</style>
