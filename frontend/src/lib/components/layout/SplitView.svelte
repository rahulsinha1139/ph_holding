<script lang="ts">
  export let listWidth: string = '400px';
  export let minListWidth: string = '300px';

  // Future feature: Resizable divider
  // export let resizable: boolean = false;
</script>

<div class="split-view">
  <!-- Left: List -->
  <aside
    class="split-list"
    style="width: {listWidth}; min-width: {minListWidth};"
  >
    <slot name="list" />
  </aside>

  <!-- Divider -->
  <div class="split-divider" aria-hidden="true"></div>

  <!-- Right: Detail -->
  <main class="split-detail">
    <slot name="detail">
      <div class="empty-detail">
        <p class="text-muted">Select an item to view details</p>
      </div>
    </slot>
  </main>
</div>

<style>
  .split-view {
    display: grid;
    grid-template-columns: var(--list-width, 400px) 1px 1fr;
    height: 100%;
    min-height: 0;
  }

  .split-list {
    overflow-y: auto;
    background: var(--surface);
    border-right: 1px solid var(--border);
  }

  .split-divider {
    background: var(--border);
    width: 1px;
  }

  .split-detail {
    overflow-y: auto;
    background: var(--bg-base);
    min-width: 0; /* Prevent flex blowout */
  }

  .empty-detail {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 300px;
  }

  .empty-detail p {
    font-size: 14px;
    color: var(--text-muted);
  }

  /* Scrollbar styling */
  .split-list::-webkit-scrollbar,
  .split-detail::-webkit-scrollbar {
    width: 8px;
  }

  .split-list::-webkit-scrollbar-track,
  .split-detail::-webkit-scrollbar-track {
    background: transparent;
  }

  .split-list::-webkit-scrollbar-thumb,
  .split-detail::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 4px;
  }

  .split-list::-webkit-scrollbar-thumb:hover,
  .split-detail::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
  }
</style>
