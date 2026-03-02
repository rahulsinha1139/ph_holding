<script lang="ts">
  /**
   * SerialTraceScreen - Serial Number Traceability (Phase 23)
   *
   * Provides full lifecycle traceability for serialized process instrumentation:
   *   PO Receipt (GRN) → Inventory → Dispatch (DN) → Delivery → Invoice
   *
   * Government clients (BAPCO, Alba, EWA) require serial-level traceability.
   */

  import {
    SearchSerials,
    GetSerialByNumber,
  } from '../../../wailsjs/go/main/App';
  import { toast } from '$lib/stores/toasts';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';

  let searchQuery = '';
  let searching = false;
  let results: any[] = [];
  let selectedSerial: any = null;

  async function handleSearch() {
    const q = searchQuery.trim();
    if (!q) {
      toast.warning('Enter a serial number to search');
      return;
    }

    searching = true;
    selectedSerial = null;
    try {
      const data = await SearchSerials(q, 50);
      results = data || [];
      if (results.length === 0) {
        toast.info('No serial numbers found matching "' + q + '"');
      } else if (results.length === 1) {
        selectedSerial = results[0];
      }
    } catch (err) {
      console.error('Serial search failed:', err);
      toast.danger('Search failed: ' + (err as Error).message);
      results = [];
    } finally {
      searching = false;
    }
  }

  function selectSerial(serial: any) {
    selectedSerial = serial;
  }

  function formatDate(d: any): string {
    if (!d) return '-';
    try {
      const date = new Date(d);
      if (isNaN(date.getTime())) return '-';
      return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
    } catch {
      return '-';
    }
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'Available': return '#10b981';
      case 'Reserved': return '#f59e0b';
      case 'Shipped': return '#3b82f6';
      case 'Delivered': return '#6366f1';
      default: return '#6b7280';
    }
  }

  function isWarrantyActive(serial: any): boolean {
    if (!serial.warranty_end_date) return false;
    return new Date(serial.warranty_end_date) > new Date();
  }

  function isCalibrationOverdue(serial: any): boolean {
    if (!serial.calibration_due_date) return false;
    return new Date(serial.calibration_due_date) < new Date();
  }
</script>

<div class="serial-trace-container">
  <!-- Search Bar -->
  <div class="search-section">
    <div class="search-bar">
      <input
        type="text"
        class="search-input"
        bind:value={searchQuery}
        placeholder="Search by serial number, product code, or customer..."
        on:keydown={(e) => e.key === 'Enter' && handleSearch()}
      />
      <Button variant="primary" on:click={handleSearch} disabled={searching}>
        {searching ? 'Searching...' : 'Search'}
      </Button>
    </div>
  </div>

  <div class="content-area">
    <!-- Results List -->
    {#if results.length > 1 && !selectedSerial}
      <div class="results-list">
        <h3 class="results-title">{results.length} serials found</h3>
        {#each results as serial}
          <button class="result-item" on:click={() => selectSerial(serial)}>
            <span class="result-serial">{serial.serial_no}</span>
            <span class="result-product">{serial.product_code}</span>
            <span class="result-status" style="color: {getStatusColor(serial.status)}">{serial.status}</span>
          </button>
        {/each}
      </div>
    {/if}

    <!-- Serial Detail Card -->
    {#if selectedSerial}
      <div class="serial-detail">
        {#if results.length > 1}
          <button class="back-button" on:click={() => selectedSerial = null}>
            &larr; Back to results
          </button>
        {/if}

        <Card>
          <div class="detail-header">
            <div class="detail-title">
              <h2 class="serial-number">{selectedSerial.serial_no}</h2>
              <span class="status-pill" style="background: {getStatusColor(selectedSerial.status)}15; color: {getStatusColor(selectedSerial.status)}; border: 1px solid {getStatusColor(selectedSerial.status)}40">
                {selectedSerial.status}
              </span>
            </div>
            <div class="product-info">
              <span class="product-code">{selectedSerial.product_code}</span>
              {#if selectedSerial.lot_number}
                <span class="lot-number">Lot: {selectedSerial.lot_number}</span>
              {/if}
            </div>
          </div>

          <!-- Traceability Timeline -->
          <div class="timeline-section">
            <h3 class="section-title">Traceability</h3>
            <div class="timeline">
              <!-- Received -->
              <div class="timeline-item" class:timeline-active={selectedSerial.grn_number}>
                <div class="timeline-dot" class:timeline-dot-active={selectedSerial.grn_number}></div>
                <div class="timeline-content">
                  <div class="timeline-label">Received</div>
                  {#if selectedSerial.grn_number}
                    <div class="timeline-value">
                      GRN <span class="mono">{selectedSerial.grn_number}</span> on {formatDate(selectedSerial.received_date)}
                    </div>
                    {#if selectedSerial.po_number}
                      <div class="timeline-sub">From PO {selectedSerial.po_number}</div>
                    {/if}
                  {:else}
                    <div class="timeline-pending">Pending</div>
                  {/if}
                </div>
              </div>

              <!-- Shipped -->
              <div class="timeline-item" class:timeline-active={selectedSerial.dn_number}>
                <div class="timeline-dot" class:timeline-dot-active={selectedSerial.dn_number}></div>
                <div class="timeline-content">
                  <div class="timeline-label">Shipped</div>
                  {#if selectedSerial.dn_number}
                    <div class="timeline-value">
                      DN <span class="mono">{selectedSerial.dn_number}</span> on {formatDate(selectedSerial.shipped_date)}
                    </div>
                  {:else}
                    <div class="timeline-pending">Pending</div>
                  {/if}
                </div>
              </div>

              <!-- Delivered -->
              <div class="timeline-item" class:timeline-active={selectedSerial.status === 'Delivered'}>
                <div class="timeline-dot" class:timeline-dot-active={selectedSerial.status === 'Delivered'}></div>
                <div class="timeline-content">
                  <div class="timeline-label">Delivered to</div>
                  {#if selectedSerial.customer_name}
                    <div class="timeline-value">{selectedSerial.customer_name}</div>
                  {:else}
                    <div class="timeline-pending">Pending</div>
                  {/if}
                </div>
              </div>

              <!-- Invoiced -->
              <div class="timeline-item" class:timeline-active={selectedSerial.invoice_number}>
                <div class="timeline-dot" class:timeline-dot-active={selectedSerial.invoice_number}></div>
                <div class="timeline-content">
                  <div class="timeline-label">Invoiced</div>
                  {#if selectedSerial.invoice_number}
                    <div class="timeline-value">
                      Invoice <span class="mono">{selectedSerial.invoice_number}</span>
                    </div>
                  {:else}
                    <div class="timeline-pending">Pending</div>
                  {/if}
                </div>
              </div>
            </div>
          </div>

          <!-- Warranty & Calibration -->
          <div class="info-grid">
            <div class="info-card">
              <h4 class="info-label">Warranty</h4>
              {#if selectedSerial.warranty_end_date}
                {#if isWarrantyActive(selectedSerial)}
                  <div class="info-value info-ok">Active until {formatDate(selectedSerial.warranty_end_date)}</div>
                {:else}
                  <div class="info-value info-expired">Expired {formatDate(selectedSerial.warranty_end_date)}</div>
                {/if}
                {#if selectedSerial.warranty_months}
                  <div class="info-sub">{selectedSerial.warranty_months} month warranty</div>
                {/if}
              {:else}
                <div class="info-value info-na">Not set</div>
              {/if}
            </div>

            <div class="info-card">
              <h4 class="info-label">Calibration</h4>
              {#if selectedSerial.calibration_due_date}
                {#if isCalibrationOverdue(selectedSerial)}
                  <div class="info-value info-expired">OVERDUE — was due {formatDate(selectedSerial.calibration_due_date)}</div>
                {:else}
                  <div class="info-value info-ok">Due {formatDate(selectedSerial.calibration_due_date)}</div>
                {/if}
                {#if selectedSerial.calibration_date}
                  <div class="info-sub">Last calibrated: {formatDate(selectedSerial.calibration_date)}</div>
                {/if}
              {:else}
                <div class="info-value info-na">Not tracked</div>
              {/if}
            </div>
          </div>

          {#if selectedSerial.notes}
            <div class="notes-section">
              <h4 class="info-label">Notes</h4>
              <p class="notes-text">{selectedSerial.notes}</p>
            </div>
          {/if}
        </Card>
      </div>
    {/if}

    <!-- Empty State -->
    {#if !searching && results.length === 0 && !selectedSerial}
      <div class="empty-state">
        <div class="empty-icon">S/N</div>
        <h3>Serial Number Traceability</h3>
        <p>Search for a serial number to view its full lifecycle — from PO receipt through to customer delivery and invoicing.</p>
      </div>
    {/if}
  </div>
</div>

<style>
  .serial-trace-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 16px;
    padding: 16px;
  }

  .search-section {
    flex-shrink: 0;
  }

  .search-bar {
    display: flex;
    gap: 8px;
    max-width: 600px;
  }

  .search-input {
    flex: 1;
    padding: 8px 12px;
    font-size: 14px;
    border: 1px solid var(--border-primary, #2a2a4a);
    border-radius: 8px;
    background: var(--bg-secondary, #1a1a2e);
    color: var(--text-primary, #e0e0e0);
    font-family: var(--font-mono);
    outline: none;
  }

  .search-input:focus {
    border-color: var(--brand-indigo, #6366f1);
    box-shadow: 0 0 0 2px var(--brand-indigo-tint, rgba(99, 102, 241, 0.2));
  }

  .search-input::placeholder {
    color: var(--text-tertiary, #666);
  }

  .content-area {
    flex: 1;
    overflow-y: auto;
  }

  /* Results List */
  .results-list {
    max-width: 600px;
  }

  .results-title {
    font-size: 13px;
    color: var(--text-secondary);
    margin: 0 0 8px 0;
    font-weight: 500;
  }

  .result-item {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 10px 12px;
    background: var(--bg-secondary, #1a1a2e);
    border: 1px solid var(--border-primary, #2a2a4a);
    border-radius: 8px;
    cursor: pointer;
    margin-bottom: 4px;
    text-align: left;
    color: var(--text-primary);
    font-size: 13px;
  }

  .result-item:hover {
    border-color: var(--brand-indigo, #6366f1);
    background: var(--bg-tertiary, #1e1e38);
  }

  .result-serial {
    font-family: var(--font-mono);
    font-weight: 600;
    min-width: 120px;
  }

  .result-product {
    color: var(--text-secondary);
    flex: 1;
  }

  .result-status {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  /* Detail Card */
  .serial-detail {
    max-width: 700px;
  }

  .back-button {
    background: none;
    border: none;
    color: var(--brand-indigo, #6366f1);
    cursor: pointer;
    font-size: 13px;
    padding: 4px 0;
    margin-bottom: 8px;
  }

  .back-button:hover {
    text-decoration: underline;
  }

  .detail-header {
    margin-bottom: 20px;
  }

  .detail-title {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .serial-number {
    font-size: 24px;
    font-family: var(--font-mono);
    font-weight: 700;
    margin: 0;
    color: var(--text-primary);
  }

  .status-pill {
    font-size: 11px;
    font-weight: 600;
    padding: 3px 10px;
    border-radius: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .product-info {
    display: flex;
    gap: 16px;
    margin-top: 4px;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .product-code {
    font-family: var(--font-mono);
  }

  .lot-number {
    font-family: var(--font-mono);
    opacity: 0.7;
  }

  /* Timeline */
  .section-title {
    font-size: 14px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .timeline {
    display: flex;
    flex-direction: column;
    gap: 0;
    margin-left: 8px;
    padding-bottom: 16px;
  }

  .timeline-item {
    display: flex;
    gap: 16px;
    position: relative;
    padding-bottom: 16px;
  }

  .timeline-item:not(:last-child)::before {
    content: '';
    position: absolute;
    left: 6px;
    top: 16px;
    bottom: 0;
    width: 2px;
    background: var(--border-primary, #2a2a4a);
  }

  .timeline-item.timeline-active:not(:last-child)::before {
    background: var(--brand-indigo, #6366f1);
  }

  .timeline-dot {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--bg-secondary, #1a1a2e);
    border: 2px solid var(--border-primary, #2a2a4a);
    flex-shrink: 0;
    margin-top: 2px;
  }

  .timeline-dot-active {
    background: var(--brand-indigo, #6366f1);
    border-color: var(--brand-indigo, #6366f1);
  }

  .timeline-content {
    flex: 1;
  }

  .timeline-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 2px;
  }

  .timeline-value {
    font-size: 13px;
    color: var(--text-primary);
  }

  .timeline-sub {
    font-size: 11px;
    color: var(--text-tertiary);
    margin-top: 2px;
  }

  .timeline-pending {
    font-size: 12px;
    color: var(--text-tertiary);
    font-style: italic;
  }

  .mono {
    font-family: var(--font-mono);
    font-weight: 500;
  }

  /* Info Grid */
  .info-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    margin-top: 16px;
  }

  .info-card {
    padding: 12px;
    background: var(--bg-tertiary, #1e1e38);
    border-radius: 8px;
    border: 1px solid var(--border-primary, #2a2a4a);
  }

  .info-label {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-secondary);
    margin: 0 0 6px 0;
  }

  .info-value {
    font-size: 13px;
    font-weight: 500;
  }

  .info-ok { color: #10b981; }
  .info-expired { color: #ef4444; }
  .info-na { color: var(--text-tertiary); font-style: italic; }

  .info-sub {
    font-size: 11px;
    color: var(--text-tertiary);
    margin-top: 2px;
  }

  /* Notes */
  .notes-section {
    margin-top: 16px;
    padding: 12px;
    background: var(--bg-tertiary, #1e1e38);
    border-radius: 8px;
    border: 1px solid var(--border-primary, #2a2a4a);
  }

  .notes-text {
    font-size: 13px;
    color: var(--text-primary);
    margin: 0;
    line-height: 1.5;
  }

  /* Empty State */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 60px 20px;
    color: var(--text-secondary);
  }

  .empty-icon {
    font-size: 32px;
    font-family: var(--font-mono);
    font-weight: 700;
    color: var(--brand-indigo, #6366f1);
    opacity: 0.5;
    margin-bottom: 12px;
    padding: 16px 20px;
    border: 2px dashed var(--border-primary, #2a2a4a);
    border-radius: 12px;
  }

  .empty-state h3 {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 6px 0;
  }

  .empty-state p {
    font-size: 13px;
    max-width: 400px;
    margin: 0;
    line-height: 1.5;
  }
</style>
