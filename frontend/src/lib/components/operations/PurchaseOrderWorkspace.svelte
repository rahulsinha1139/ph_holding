<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';
  import Button from '$lib/components/ui/Button.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';

  export let mode: 'create' | 'edit' | 'view' = 'create';
  export let formData: any;
  export let suppliers: any[] = [];
  export let orders: any[] = [];
  export let currencies: { value: string; label: string }[] = [];
  export let paymentTerms: string[] = [];
  export let formLoading = false;
  export let statusLoading = false;
  export let viewingPO: any = null;
  export let todayDate = '';

  const dispatch = createEventDispatcher();

  const statusOptions = [
    'Draft',
    'Pending Approval',
    'Approved',
    'Sent',
    'Acknowledged',
    'Partially Received',
    'Received',
    'Closed'
  ];

  $: totalForeign = (formData?.items || []).reduce(
    (sum: number, item: any) => sum + (Number(item.quantity || 0) * Number(item.unit_price_foreign || 0)),
    0
  );
  $: vatAmount = totalForeign * 0.10;
  $: totalWithVAT = totalForeign + vatAmount;
  $: totalBHD = totalWithVAT * Number(formData?.exchange_rate || 1);

  function displayDate(value: any): string {
    if (!value) return '-';
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? '-' : parsed.toLocaleDateString();
  }

  function amount(value: any): string {
    return Number(value || 0).toFixed(3);
  }
</script>

<section class="po-workspace" aria-label="Supplier purchase order workspace">
  <header class="workspace-header">
    <div>
      <span class="workspace-kicker">Supplier PO</span>
      <h2>
        {#if mode === 'create'}
          New Purchase Order
        {:else if mode === 'edit'}
          Edit Purchase Order
        {:else}
          {viewingPO?.po_number || 'Purchase Order Details'}
        {/if}
      </h2>
      <p>
        {#if mode === 'view'}
          Review supplier, line items, totals, and status without losing table context.
        {:else}
          Build the supplier-side order in the page workspace with full line-item visibility.
        {/if}
      </p>
    </div>
    <div class="workspace-actions">
      {#if mode === 'view' && viewingPO}
        <Button variant="secondary" size="sm" on:click={() => dispatch('edit', viewingPO.id)}>Edit PO</Button>
        <Button variant="secondary" size="sm" on:click={() => dispatch('pdf', viewingPO.id)}>PDF</Button>
      {/if}
      <Button variant="ghost" size="sm" on:click={() => dispatch('cancel')}>Back to Supplier POs</Button>
    </div>
  </header>

  {#if mode === 'view' && viewingPO}
    <div class="view-grid">
      <section class="detail-strip">
        <div>
          <span>PO Number</span>
          <strong class="mono">{viewingPO.po_number}</strong>
        </div>
        <div>
          <span>Status</span>
          <StatusBadge status={viewingPO.status} />
        </div>
        <div>
          <span>Supplier</span>
          <strong>{viewingPO.supplier_name}</strong>
        </div>
        <div>
          <span>PO Date</span>
          <strong>{displayDate(viewingPO.po_date)}</strong>
        </div>
        <div>
          <span>Expected Delivery</span>
          <strong>{displayDate(viewingPO.expected_delivery)}</strong>
        </div>
        <div>
          <span>Payment Terms</span>
          <strong>{viewingPO.payment_terms || '-'}</strong>
        </div>
      </section>

      {#if viewingPO.items && viewingPO.items.length > 0}
        <section class="items-section">
          <div class="section-header">
            <h3>Items</h3>
          </div>
          <table class="items-table">
            <thead>
              <tr>
                <th>Description</th>
                <th class="right">Qty</th>
                <th class="right">Unit ({viewingPO.currency})</th>
                <th class="right">Total ({viewingPO.currency})</th>
              </tr>
            </thead>
            <tbody>
              {#each viewingPO.items as item}
                <tr>
                  <td>{item.description}</td>
                  <td class="right mono">{amount(item.quantity)}</td>
                  <td class="right mono">{amount(item.unit_price_foreign)}</td>
                  <td class="right mono">{amount(item.total_foreign || Number(item.quantity || 0) * Number(item.unit_price_foreign || 0))}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>
      {/if}

      <section class="view-side">
        <div class="totals-panel">
          <div><span>Net</span><strong>{amount(viewingPO.subtotal_foreign)} {viewingPO.currency}</strong></div>
          <div><span>VAT</span><strong>{amount(viewingPO.vat_amount)} {viewingPO.currency}</strong></div>
          <div class="grand"><span>Total incl. VAT</span><strong>{amount(viewingPO.total_foreign)} {viewingPO.currency}</strong></div>
          {#if viewingPO.currency !== 'BHD'}
            <div><span>Total BHD</span><strong>{amount(viewingPO.total_bhd)} BHD</strong></div>
          {/if}
        </div>

        <div class="status-panel">
          <h3>Update Status</h3>
          <div class="status-buttons">
            {#each statusOptions as status}
              <Button
                variant={viewingPO.status === status ? 'primary' : 'secondary'}
                size="sm"
                disabled={viewingPO.status === status || statusLoading}
                on:click={() => dispatch('statusChange', status)}
              >
                {status}
              </Button>
            {/each}
          </div>
        </div>
      </section>
    </div>
  {:else}
    <form on:submit|preventDefault={() => dispatch(mode === 'create' ? 'create' : 'update')} class="po-form">
      <div class="form-row">
        <FormGroup label="Supplier" required>
          <select class="select-input" bind:value={formData.supplier_id} required>
            <option value="">Select supplier...</option>
            {#each suppliers as supplier}
              <option value={supplier.id}>{supplier.supplier_name}</option>
            {/each}
          </select>
        </FormGroup>

        <FormGroup label="Link to Order (Optional)">
          <select class="select-input" bind:value={formData.order_id}>
            <option value="">No order link...</option>
            {#each orders as order}
              <option value={order.id}>{order.order_number} - {order.customer_name}</option>
            {/each}
          </select>
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="PO Date" required>
          <Input type="date" bind:value={formData.po_date} max={todayDate} required />
        </FormGroup>

        <FormGroup label="Expected Delivery" required>
          <Input type="date" bind:value={formData.expected_delivery} min={formData.po_date} required />
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="Currency" required>
          <select
            class="select-input"
            bind:value={formData.currency}
            on:change={() => dispatch('currencyChange', formData.currency)}
            required
          >
            {#each currencies as curr}
              <option value={curr.value}>{curr.label}</option>
            {/each}
          </select>
        </FormGroup>

        <FormGroup label="Exchange Rate (to BHD)" required>
          <Input type="number" bind:value={formData.exchange_rate} step="0.0001" min="0.0001" required />
          <div class="exchange-hint">1 {formData.currency} = {Number(formData.exchange_rate || 0).toFixed(4)} BHD</div>
        </FormGroup>
      </div>

      <FormGroup label="Payment Terms" required>
        <select class="select-input" bind:value={formData.payment_terms} required>
          {#each paymentTerms as term}
            <option value={term}>{term}</option>
          {/each}
        </select>
      </FormGroup>

      <section class="line-items-section">
        <div class="section-header">
          <h3>Line Items</h3>
          <Button variant="secondary" size="sm" on:click={() => dispatch('addItem')}>+ Add Item</Button>
        </div>

        {#if formData.items.length === 0}
          <div class="empty-items">No items added yet.</div>
        {:else}
          <div class="items-list">
            {#each formData.items as item, index}
              <div class="item-row" transition:fade>
                <Input label="Description" bind:value={item.description} placeholder="Product or service description" />
                <Input label="Quantity" type="number" bind:value={item.quantity} min="1" step="1" required />
                <Input label="Unit Price ({formData.currency})" type="number" bind:value={item.unit_price_foreign} min="0.001" step="0.001" required />
                <div class="item-total">
                  <span>Total</span>
                  <strong>{amount(Number(item.quantity || 0) * Number(item.unit_price_foreign || 0))} {formData.currency}</strong>
                </div>
                <button type="button" class="btn-remove" on:click={() => dispatch('removeItem', index)} aria-label="Remove item">x</button>
              </div>
            {/each}
          </div>
        {/if}
      </section>

      <div class="form-footer">
        <div class="totals-inline">
          <div><span>Subtotal</span><strong>{amount(totalForeign)} {formData.currency}</strong></div>
          <div><span>VAT 10%</span><strong>{amount(vatAmount)} {formData.currency}</strong></div>
          <div class="grand"><span>Total</span><strong>{amount(totalWithVAT)} {formData.currency}</strong></div>
          {#if formData.currency !== 'BHD'}
            <div><span>BHD</span><strong>{amount(totalBHD)} BHD</strong></div>
          {/if}
        </div>
        <div class="workspace-actions">
          <Button variant="ghost" on:click={() => dispatch('cancel')}>Cancel</Button>
          <Button variant="primary" type="submit" loading={formLoading}>
            {mode === 'create' ? 'Create PO' : 'Save Changes'}
          </Button>
        </div>
      </div>
    </form>
  {/if}
</section>

<style>
  .po-workspace {
    display: grid;
    gap: 14px;
    padding: 14px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    box-shadow: var(--shadow-sm);
  }

  .workspace-header,
  .section-header,
  .form-footer {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .workspace-header h2,
  .section-header h3,
  .status-panel h3 {
    margin: 0;
    color: var(--text-primary);
  }

  .workspace-header h2 {
    font-size: 20px;
    line-height: 1.15;
  }

  .workspace-header p {
    margin: 4px 0 0;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .workspace-kicker,
  .detail-strip span,
  .totals-panel span,
  .totals-inline span,
  .item-total span {
    display: block;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    font-weight: 650;
  }

  .workspace-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    flex-wrap: wrap;
  }

  .po-form {
    display: grid;
    gap: 12px;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .select-input {
    width: 100%;
    padding: 8px 10px;
    font-size: 14px;
    font-family: var(--font-family);
    color: var(--text-primary);
    background: var(--surface);
    border: var(--border-width) solid var(--border);
    border-radius: var(--border-radius-sm);
  }

  .line-items-section,
  .view-side,
  .status-panel {
    display: grid;
    gap: 10px;
  }

  .line-items-section {
    padding: 10px;
    background: var(--surface-elevated);
    border-radius: 8px;
  }

  .items-list {
    display: grid;
    gap: 8px;
  }

  .item-row {
    display: grid;
    grid-template-columns: minmax(220px, 2fr) minmax(88px, 0.65fr) minmax(120px, 0.8fr) minmax(120px, 0.8fr) 28px;
    gap: 8px;
    align-items: end;
    padding: 8px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
  }

  .item-total {
    display: grid;
    gap: 5px;
  }

  .item-total strong,
  .mono {
    font-family: var(--font-mono);
  }

  .btn-remove {
    width: 28px;
    height: 28px;
    border: 0;
    border-radius: 999px;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
    font-weight: 700;
  }

  .btn-remove:hover {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  .empty-items {
    padding: 18px;
    text-align: center;
    color: var(--text-secondary);
    border: 1px dashed var(--border);
    border-radius: 8px;
    background: var(--surface);
  }

  .form-footer {
    align-items: center;
    padding-top: 2px;
  }

  .totals-inline {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .totals-inline div,
  .totals-panel div,
  .detail-strip div {
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
  }

  .totals-inline strong,
  .totals-panel strong {
    font-family: var(--font-mono);
    font-size: 14px;
  }

  .totals-inline .grand,
  .totals-panel .grand {
    border-color: color-mix(in srgb, var(--brand-indigo) 35%, var(--border));
    color: var(--brand-indigo);
  }

  .view-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
    gap: 12px;
    align-items: start;
  }

  .detail-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    grid-column: 1 / -1;
  }

  .detail-strip strong {
    display: block;
    margin-top: 4px;
    font-size: 14px;
    color: var(--text-primary);
  }

  .items-section {
    min-width: 0;
    display: grid;
    gap: 8px;
  }

  .items-table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    font-size: 13px;
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }

  .items-table th,
  .items-table td {
    padding: 7px 9px;
    border-bottom: 1px solid var(--border);
    overflow-wrap: anywhere;
  }

  .items-table th {
    text-align: left;
    background: var(--surface-elevated);
    color: var(--text-secondary);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .right {
    text-align: right;
  }

  .totals-panel {
    display: grid;
    gap: 8px;
  }

  .status-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .exchange-hint {
    margin-top: 4px;
    color: var(--text-secondary);
    font-size: 11px;
    font-family: var(--font-mono);
  }

  @media (max-width: 980px) {
    .view-grid,
    .detail-strip,
    .form-row,
    .item-row {
      grid-template-columns: 1fr;
    }

    .workspace-header,
    .form-footer {
      flex-direction: column;
    }

    .workspace-actions {
      justify-content: flex-start;
    }
  }
</style>
