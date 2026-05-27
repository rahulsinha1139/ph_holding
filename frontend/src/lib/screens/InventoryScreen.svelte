<script lang="ts">
  import { onMount } from "svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import WabiSpinner from "$lib/components/ui/WabiSpinner.svelte";
  import {
    CreateInventoryItem,
    CreateWarehouse,
    GetInventoryItems,
    GetInventoryMovementsWorkspace,
    GetInventoryPendingFulfillmentReport,
    GetWarehouses,
    RecordStockMovement,
    UpdateInventoryItem,
  } from "../../../wailsjs/go/main/App";
  import { formatBHDValue } from "$lib/utils/formatters";
  import { toast } from "$lib/stores/toasts";

  type InventoryTab = "stock" | "movements" | "pending" | "master";

  export let embedded = false;

  let activeTab: InventoryTab = "stock";
  let loading = true;
  let stockRows: any[] = [];
  let movementRows: any[] = [];
  let pendingRows: any[] = [];
  let warehouses: any[] = [];
  let search = "";
  let savingManualStock = false;
  let savingWarehouse = false;
  let showWarehouseForm = false;
  let warehouseCode = "";
  let warehouseName = "";
  let manualStock = {
    productCode: "",
    productId: "",
    warehouseId: "",
    quantity: "",
    unitCost: "",
    reorderPoint: "",
    minimumStock: "",
    maximumStock: "",
    movementDate: new Date().toISOString().slice(0, 10),
    referenceType: "legacy_opening_stock",
    referenceNumber: "",
    counterpartyName: "",
    notes: "",
  };

  const tabs: { id: InventoryTab; label: string }[] = [
    { id: "stock", label: "Stock Position" },
    { id: "movements", label: "Movements" },
    { id: "pending", label: "Pending Fulfillment" },
    { id: "master", label: "Item Master" },
  ];

  async function loadInventory() {
    loading = true;
    try {
      const [stock, movements, pending] = await Promise.all([
        GetInventoryItems(null, "All", false).catch(() => []),
        GetInventoryMovementsWorkspace(250).catch(() => []),
        GetInventoryPendingFulfillmentReport(300).catch(() => []),
      ]);
      stockRows = stock || [];
      movementRows = movements || [];
      pendingRows = pending || [];
      warehouses = await GetWarehouses().catch(() => warehouses);
    } catch (err) {
      toast.danger(`Failed to load inventory workspace: ${String(err)}`);
    } finally {
      loading = false;
    }
  }

  function formatDate(value: any) {
    if (!value) return "-";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "-";
    return date.toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });
  }

  function includesSearch(row: any) {
    const term = search.trim().toLowerCase();
    if (!term) return true;
    return [
      row.product_code,
      row.stock_status,
      row.movement_type,
      row.movement_number,
      row.order_number,
      row.customer_name,
      row.description,
      row.inventory_item_id,
      row.reference_number,
      row.counterparty_name,
    ].filter(Boolean).some((value) => String(value).toLowerCase().includes(term));
  }

  function toNumber(value: string) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function resetManualStockForm() {
    manualStock = {
      productCode: "",
      productId: "",
      warehouseId: "",
      quantity: "",
      unitCost: "",
      reorderPoint: "",
      minimumStock: "",
      maximumStock: "",
      movementDate: new Date().toISOString().slice(0, 10),
      referenceType: "legacy_opening_stock",
      referenceNumber: "",
      counterpartyName: "",
      notes: "",
    };
  }

  function findExistingManualItem(productCode: string, productId: string, warehouseId: string) {
    return stockRows.find((item) => {
      const sameWarehouse = String(item.warehouse_id || "") === warehouseId;
      if (!sameWarehouse) return false;
      if (productId && String(item.product_id || "") === productId) return true;
      return productCode && String(item.product_code || "").toLowerCase() === productCode.toLowerCase();
    });
  }

  async function handleCreateWarehouse() {
    if (!warehouseCode.trim() || !warehouseName.trim()) {
      toast.warning("Warehouse code and name are required");
      return;
    }

    savingWarehouse = true;
    try {
      const created = await CreateWarehouse({
        code: warehouseCode.trim().toUpperCase(),
        name: warehouseName.trim(),
        is_active: true,
      } as any);
      warehouses = [...warehouses.filter((row) => row.id !== created.id), created];
      manualStock.warehouseId = created.id;
      warehouseCode = "";
      warehouseName = "";
      showWarehouseForm = false;
      toast.success("Warehouse created");
    } catch (err) {
      toast.danger(`Failed to create warehouse: ${String(err)}`);
    } finally {
      savingWarehouse = false;
    }
  }

  async function handleManualStockEntry() {
    const quantity = toNumber(manualStock.quantity);
    const productCode = manualStock.productCode.trim();
    const productId = manualStock.productId.trim();
    if (!productCode && !productId) {
      toast.warning("Enter a product code or product ID");
      return;
    }
    if (quantity <= 0) {
      toast.warning("Opening quantity must be greater than zero");
      return;
    }

    savingManualStock = true;
    try {
      const inventoryUpdates = {
        reorder_point: toNumber(manualStock.reorderPoint),
        minimum_stock: toNumber(manualStock.minimumStock),
        maximum_stock: toNumber(manualStock.maximumStock),
      };
      let item = findExistingManualItem(productCode, productId, manualStock.warehouseId);
      if (item?.id) {
        await UpdateInventoryItem(item.id, inventoryUpdates as any).catch(() => undefined);
      } else {
        item = await CreateInventoryItem({
          product_id: productId,
          product_code: productCode,
          warehouse_id: manualStock.warehouseId,
          ...inventoryUpdates,
          is_active: true,
        } as any);
      }

      await RecordStockMovement({
        inventory_item_id: item.id,
        movement_type: "Manual Opening Stock",
        reference_type: manualStock.referenceType,
        reference_number: manualStock.referenceNumber.trim(),
        counterparty_name: manualStock.counterpartyName.trim(),
        quantity,
        direction: "IN",
        movement_date: new Date(`${manualStock.movementDate}T09:00:00`).toISOString(),
        unit_cost: toNumber(manualStock.unitCost),
        notes: manualStock.notes.trim(),
      } as any);

      resetManualStockForm();
      await loadInventory();
      toast.success("Opening stock added");
    } catch (err) {
      toast.danger(`Failed to add opening stock: ${String(err)}`);
    } finally {
      savingManualStock = false;
    }
  }

  $: filteredStock = stockRows.filter(includesSearch);
  $: filteredMovements = movementRows.filter(includesSearch);
  $: filteredPending = pendingRows.filter(includesSearch);
  $: inventoryValue = stockRows.reduce((sum, row) => sum + (Number(row.total_value) || 0), 0);
  $: availableQuantity = stockRows.reduce((sum, row) => sum + (Number(row.quantity_available) || 0), 0);
  $: pendingQuantity = pendingRows.reduce((sum, row) => sum + Math.max(0, Number(row.pending_quantity) || 0), 0);

  onMount(loadInventory);
</script>

<section class="inventory-workspace" data-embedded={embedded ? "true" : undefined}>
  <header class="workspace-head">
    <div>
      <h2>Inventory</h2>
      <p>Operational visibility across stock, movements, fulfillment, and item references.</p>
    </div>
    <Button variant="secondary" size="sm" on:click={loadInventory} disabled={loading}>Refresh</Button>
  </header>

  <div class="kpi-strip">
    <div class="kpi">
      <span>Stock Lines</span>
      <strong>{stockRows.length}</strong>
    </div>
    <div class="kpi">
      <span>Available Qty</span>
      <strong>{availableQuantity.toLocaleString("en-US", { maximumFractionDigits: 3 })}</strong>
    </div>
    <div class="kpi">
      <span>Pending Fulfillment</span>
      <strong>{pendingQuantity.toLocaleString("en-US", { maximumFractionDigits: 3 })}</strong>
    </div>
    <div class="kpi">
      <span>Inventory Value</span>
      <strong>{formatBHDValue(inventoryValue)} BHD</strong>
    </div>
  </div>

  <section class="manual-panel">
    <div class="panel-head">
      <div>
        <h3>Manual Stock Entry</h3>
        <p>Add legacy or physically counted inventory without waiting for a supplier invoice or GRN.</p>
      </div>
      <Button variant="secondary" size="sm" on:click={() => showWarehouseForm = !showWarehouseForm}>
        {showWarehouseForm ? "Close Warehouse" : "New Warehouse"}
      </Button>
    </div>

    {#if showWarehouseForm}
      <div class="warehouse-form">
        <input bind:value={warehouseCode} placeholder="Warehouse code" />
        <input bind:value={warehouseName} placeholder="Warehouse name" />
        <Button variant="secondary" size="sm" on:click={handleCreateWarehouse} disabled={savingWarehouse}>
          {savingWarehouse ? "Saving..." : "Save Warehouse"}
        </Button>
      </div>
    {/if}

    <div class="manual-grid">
      <input bind:value={manualStock.productCode} placeholder="Product / model code" />
      <input bind:value={manualStock.productId} placeholder="Product ID (optional)" />
      <select bind:value={manualStock.warehouseId}>
        <option value="">No warehouse / default</option>
        {#each warehouses as warehouse}
          <option value={warehouse.id}>{warehouse.code} - {warehouse.name}</option>
        {/each}
      </select>
      <input bind:value={manualStock.quantity} type="number" min="0" step="0.001" placeholder="Opening qty" />
      <input bind:value={manualStock.unitCost} type="number" min="0" step="0.001" placeholder="Unit cost BHD" />
      <input bind:value={manualStock.reorderPoint} type="number" min="0" step="0.001" placeholder="Reorder point" />
      <input bind:value={manualStock.minimumStock} type="number" min="0" step="0.001" placeholder="Minimum stock" />
      <input bind:value={manualStock.maximumStock} type="number" min="0" step="0.001" placeholder="Maximum stock" />
      <input bind:value={manualStock.movementDate} type="date" />
      <select bind:value={manualStock.referenceType}>
        <option value="legacy_opening_stock">Legacy opening stock</option>
        <option value="supplier_invoice">Supplier invoice</option>
        <option value="customer_invoice">Customer invoice</option>
        <option value="manual_count">Manual count</option>
        <option value="stock_correction">Stock correction</option>
      </select>
      <input bind:value={manualStock.referenceNumber} placeholder="Invoice / reference no." />
      <input bind:value={manualStock.counterpartyName} placeholder="Supplier / customer" />
      <textarea bind:value={manualStock.notes} rows="2" placeholder="Notes"></textarea>
      <div class="manual-actions">
        <Button variant="primary" size="sm" on:click={handleManualStockEntry} disabled={savingManualStock}>
          {savingManualStock ? "Adding..." : "Add Opening Stock"}
        </Button>
      </div>
    </div>
  </section>

  <div class="toolbar">
    <nav class="tabs" aria-label="Inventory views">
      {#each tabs as tab}
        <button class:active={activeTab === tab.id} on:click={() => activeTab = tab.id}>{tab.label}</button>
      {/each}
    </nav>
    <input bind:value={search} placeholder="Search item, customer, movement, order" />
  </div>

  {#if loading}
    <div class="state">
      <WabiSpinner size="md" tempo="calm" />
      <span>Loading inventory visibility...</span>
    </div>
  {:else if activeTab === "stock"}
    <div class="table-shell">
      <table>
        <thead>
          <tr>
            <th>Item</th>
            <th>Status</th>
            <th class="num">On Hand</th>
            <th class="num">Reserved</th>
            <th class="num">Available</th>
            <th class="num">Unit Cost</th>
            <th class="num">Value</th>
            <th>Last Movement</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredStock as item}
            <tr>
              <td><strong>{item.product_code || item.product_id || item.id}</strong></td>
              <td>{item.stock_status || "Active"}</td>
              <td class="num">{Number(item.quantity_on_hand || 0).toLocaleString()}</td>
              <td class="num">{Number(item.quantity_reserved || 0).toLocaleString()}</td>
              <td class="num">{Number(item.quantity_available || 0).toLocaleString()}</td>
              <td class="num">{formatBHDValue(Number(item.unit_cost || 0))}</td>
              <td class="num">{formatBHDValue(Number(item.total_value || 0))}</td>
              <td>{formatDate(item.last_movement_at)}</td>
            </tr>
          {:else}
            <tr><td colspan="8" class="empty">No stock rows match the current search.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else if activeTab === "movements"}
    <div class="table-shell">
      <table>
        <thead>
          <tr>
            <th>Date</th>
            <th>Movement</th>
            <th>Reference</th>
            <th>Linked Party</th>
            <th>Direction</th>
            <th class="num">Qty</th>
            <th class="num">Before</th>
            <th class="num">After</th>
            <th class="num">Value</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredMovements as move}
            <tr>
              <td>{formatDate(move.movement_date)}</td>
              <td>{move.movement_type || "-"}</td>
              <td>{move.reference_number || move.movement_number || move.inventory_item_id || "-"}</td>
              <td>{move.counterparty_name || move.reference_type || "-"}</td>
              <td>{move.direction || "-"}</td>
              <td class="num">{Number(move.quantity || 0).toLocaleString()}</td>
              <td class="num">{Number(move.balance_before || 0).toLocaleString()}</td>
              <td class="num">{Number(move.balance_after || 0).toLocaleString()}</td>
              <td class="num">{formatBHDValue(Number(move.total_value || 0))}</td>
            </tr>
          {:else}
            <tr><td colspan="9" class="empty">No movement rows match the current search.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else if activeTab === "pending"}
    <div class="table-shell">
      <table>
        <thead>
          <tr>
            <th>Order</th>
            <th>Customer</th>
            <th>Item</th>
            <th>Description</th>
            <th class="num">Ordered</th>
            <th class="num">Delivered</th>
            <th class="num">Invoiced</th>
            <th class="num">Pending</th>
            <th class="num">Available</th>
            <th class="num">Need to Order</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredPending as row}
            <tr class:exception={Number(row.pending_quantity || 0) > 0}>
              <td><strong>{row.order_number || row.order_id}</strong></td>
              <td>{row.customer_name || "-"}</td>
              <td>{row.product_code || "-"}</td>
              <td>{row.description || "-"}</td>
              <td class="num">{Number(row.ordered_quantity || 0).toLocaleString()}</td>
              <td class="num">{Number(row.delivered_quantity || 0).toLocaleString()}</td>
              <td class="num">{Number(row.invoiced_quantity || 0).toLocaleString()}</td>
              <td class="num"><strong>{Number(row.pending_quantity || 0).toLocaleString()}</strong></td>
              <td class="num">{Number(row.available_quantity || 0).toLocaleString()}</td>
              <td class="num"><strong>{Number(row.shortage_quantity || 0).toLocaleString()}</strong></td>
            </tr>
          {:else}
            <tr><td colspan="10" class="empty">No pending fulfillment rows match the current search.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else}
    <div class="table-shell">
      <table>
        <thead>
          <tr>
            <th>Item Code</th>
            <th>Product ID</th>
            <th>Warehouse</th>
            <th class="num">Reorder</th>
            <th class="num">Minimum</th>
            <th class="num">Maximum</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredStock as item}
            <tr>
              <td><strong>{item.product_code || item.id}</strong></td>
              <td>{item.product_id || "-"}</td>
              <td>{item.warehouse_id || "-"}</td>
              <td class="num">{Number(item.reorder_point || 0).toLocaleString()}</td>
              <td class="num">{Number(item.minimum_stock || 0).toLocaleString()}</td>
              <td class="num">{Number(item.maximum_stock || 0).toLocaleString()}</td>
              <td>{item.stock_status || "Active"}</td>
            </tr>
          {:else}
            <tr><td colspan="7" class="empty">No item master rows match the current search.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<style>
  .inventory-workspace {
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .workspace-head,
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  h2 {
    margin: 0;
    font-size: var(--page-title-size-scaled, 24px);
  }

  p {
    margin: 4px 0 0;
    color: var(--text-muted, #6b7280);
  }

  .kpi-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .kpi {
    border: 1px solid var(--border, #e5e7eb);
    border-radius: 8px;
    padding: 14px;
    background: var(--surface, #fff);
  }

  .manual-panel {
    border: 1px solid var(--border, #e5e7eb);
    border-radius: 8px;
    background: var(--surface, #fff);
    padding: 16px;
    display: grid;
    gap: 12px;
  }

  .panel-head,
  .warehouse-form,
  .manual-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .panel-head h3 {
    margin: 0;
    font-size: 17px;
  }

  .warehouse-form {
    justify-content: flex-start;
    padding: 10px;
    border-radius: 8px;
    background: rgba(79, 143, 79, 0.07);
  }

  .warehouse-form input {
    min-width: 180px;
  }

  .manual-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(150px, 1fr));
    gap: 10px;
  }

  .manual-grid textarea {
    grid-column: span 3;
  }

  .manual-actions {
    justify-content: flex-end;
  }

  .kpi span {
    display: block;
    color: var(--text-muted, #6b7280);
    font-size: var(--modal-label-size, 12px);
    font-weight: 700;
    text-transform: uppercase;
  }

  .kpi strong {
    display: block;
    margin-top: 8px;
    font-size: 22px;
  }

  .tabs {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .tabs button {
    padding: 8px 12px;
    border: 1px solid var(--border, #e5e7eb);
    border-radius: 8px;
    background: var(--surface, #fff);
    color: var(--text-muted, #6b7280);
    font-weight: 700;
  }

  .tabs button.active {
    color: white;
    background: var(--forest, #4f8f4f);
    border-color: var(--forest, #4f8f4f);
  }

  .toolbar input {
    min-width: 320px;
    padding: 10px 12px;
  }

  input,
  select,
  textarea {
    border: 1px solid var(--border, #e5e7eb);
    border-radius: 8px;
    background: #fff;
    color: var(--text-primary, #111827);
    font: inherit;
    padding: 10px 12px;
  }

  .table-shell {
    border: 1px solid var(--border, #e5e7eb);
    border-radius: 8px;
    overflow: auto;
    background: var(--surface, #fff);
    max-height: calc(100vh - 360px);
  }

  table {
    width: 100%;
    min-width: 1100px;
    border-collapse: collapse;
    font-size: var(--table-text-size, 14px);
  }

  th,
  td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border, #e5e7eb);
    text-align: left;
    vertical-align: top;
  }

  th {
    position: sticky;
    top: 0;
    background: var(--surface, #fff);
    z-index: 1;
    color: var(--text-muted, #6b7280);
    font-size: var(--modal-label-size, 12px);
    text-transform: uppercase;
  }

  .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  tr.exception {
    background: rgba(245, 158, 11, 0.08);
  }

  .empty,
  .state {
    padding: 32px;
    text-align: center;
    color: var(--text-muted, #6b7280);
  }

  .state {
    display: flex;
    justify-content: center;
    gap: 12px;
  }

  @media (max-width: 900px) {
    .inventory-workspace {
      padding: 16px;
    }

    .workspace-head,
    .toolbar {
      align-items: stretch;
      flex-direction: column;
    }

    .kpi-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .manual-grid {
      grid-template-columns: 1fr;
    }

    .manual-grid textarea {
      grid-column: auto;
    }

    .panel-head,
    .warehouse-form {
      align-items: stretch;
      flex-direction: column;
    }

    .toolbar input {
      min-width: 0;
      width: 100%;
    }
  }
</style>
