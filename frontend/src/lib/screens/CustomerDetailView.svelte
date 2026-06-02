<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';
  import ContextTaskModal from '$lib/components/ContextTaskModal.svelte';
  import { toast } from '$lib/stores/toasts';
  import { GetCustomerFullProfile, AddCustomerNote, UpdateCustomer, AddCustomerContact, UpdateCustomerContact, DeleteCustomerContact, DeleteCustomer } from '../../../wailsjs/go/main/App';
  import type { main } from '../../../wailsjs/go/models';

  export let customerId: string;

  const dispatch = createEventDispatcher();

  let loading = true;
  let error: string | null = null;
  let profile: main.CustomerFullProfile | null = null;
  let activeTab = 'overview';
  let showNoteModal = false;
  let newNoteType = 'general';
  let newNoteContent = '';
  let editMode = false;
  let editForm: any = {};
  let saveLoading = false;
  let showContactModal = false;
  let contactManagerMode = false;
  let editingContactId = '';
  let contactsScrollEl: HTMLDivElement;
  const emptyContactForm = () => ({
    contact_name: '',
    job_title: '',
    email: '',
    phone: '',
    address: '',
    is_primary_contact: false,
    is_decision_maker: false,
    decision_role: '',
    reports_to_contact_id: '',
    reports_to_name: '',
  });
  let contactForm = emptyContactForm();
  let showDeleteConfirm = false;
  let deleteLoading = false;
  let customerDeleted = false;
  let showTaskModal = false;

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (showDeleteConfirm) showDeleteConfirm = false;
      else if (showTaskModal) showTaskModal = false;
      else if (showNoteModal) showNoteModal = false;
      else if (showContactModal) showContactModal = false;
    }
  }

  async function handleDeleteCustomer() {
    deleteLoading = true;
    try {
      await DeleteCustomer(customerId);
      customerDeleted = true;
      toast.success('Customer deleted successfully');
      dispatch('back', { refresh: true });
    } catch (err: any) {
      toast.danger('Failed to delete customer: ' + (err?.message || err));
    } finally {
      deleteLoading = false;
      showDeleteConfirm = false;
    }
  }

  function startEdit() {
    if (profile) {
      editForm = { ...profile };
      editMode = true;
    }
  }

  function cancelEdit() {
    editMode = false;
    editForm = {};
  }

  async function saveEdit() {
    saveLoading = true;
    try {
      await UpdateCustomer(editForm);
      toast.success('Customer updated successfully');
      editMode = false;
      await loadProfile();
    } catch (err) {
      toast.danger('Failed to update customer: ' + err);
    } finally {
      saveLoading = false;
    }
  }

  const tabs = [
    { id: 'overview', label: 'Overview' },
    { id: 'orders', label: 'Orders' },
    { id: 'invoices', label: 'Invoices' },
    { id: 'rfqs', label: 'RFQs' },
    { id: 'notes', label: 'Notes' },
  ];

  async function loadProfile() {
    loading = true;
    error = null;
    try {
      profile = await GetCustomerFullProfile(customerId);
    } catch (err) {
      console.error('Failed to load customer profile:', err);
      error = err instanceof Error ? err.message : 'Failed to load customer profile';
      toast.danger(error);
    } finally {
      loading = false;
    }
  }

  async function saveNote() {
    if (!newNoteContent.trim()) {
      toast.warning('Please enter note content');
      return;
    }
    try {
      await AddCustomerNote(customerId, newNoteType, newNoteContent);
      toast.success('Note added');
      showNoteModal = false;
      newNoteContent = '';
      await loadProfile();
    } catch (err) {
      console.error('Failed to add note:', err);
      toast.danger('Failed to add note');
    }
  }

  async function saveContact() {
    if (!contactForm.contact_name.trim()) { toast.warning('Name is required'); return; }
    try {
      const payload = { ...contactForm, customer_id: customerId, id: editingContactId } as any;
      if (editingContactId) {
        await UpdateCustomerContact(payload);
        toast.success('Contact updated');
      } else {
        await AddCustomerContact(payload);
        toast.success('Contact added');
      }
      showContactModal = false;
      editingContactId = '';
      contactForm = emptyContactForm();
      await loadProfile();
    } catch (err) {
      toast.danger('Failed to save contact: ' + err);
    }
  }

  function openAddContact() {
    editingContactId = '';
    contactForm = emptyContactForm();
    showContactModal = true;
  }

  function editContact(contact: any) {
    editingContactId = contact?.id || '';
    contactForm = {
      contact_name: contact?.contact_name || '',
      job_title: contact?.job_title || '',
      email: contact?.email || '',
      phone: contact?.phone || '',
      address: contact?.address || '',
      is_primary_contact: Boolean(contact?.is_primary_contact),
      is_decision_maker: Boolean(contact?.is_decision_maker),
      decision_role: contact?.decision_role || '',
      reports_to_contact_id: contact?.reports_to_contact_id || '',
      reports_to_name: contact?.reports_to_name || '',
    };
    showContactModal = true;
  }

  async function deleteContact(contact: any) {
    if (!contact?.id || !confirm(`Delete contact ${contact.contact_name || ''}?`)) return;
    try {
      await DeleteCustomerContact(contact.id);
      toast.success('Contact removed');
      await loadProfile();
    } catch (err) {
      toast.danger('Failed to remove contact: ' + err);
    }
  }

  function scrollContacts(direction: number) {
    contactsScrollEl?.scrollBy({ left: direction * 360, behavior: 'smooth' });
  }

  function customerContacts(): any[] {
    return (profile?.contacts || []) as any[];
  }

  function goBack() {
    dispatch('back');
  }

  function closeDeleteConfirm() {
    showDeleteConfirm = false;
  }

  function closeNoteModal() {
    showNoteModal = false;
  }

  function closeContactModal() {
    showContactModal = false;
    editingContactId = '';
    contactForm = emptyContactForm();
  }

  function handleBackdropClick(event: MouseEvent, closeAction: () => void) {
    if (event.target !== event.currentTarget) return;
    closeAction();
  }

  function handleBackdropKeydown(event: KeyboardEvent, closeAction: () => void) {
    if (event.key === 'Escape' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      closeAction();
    }
  }

  function formatDate(date: any): string {
    if (!date) return 'N/A';
    try {
      // Handle time.Time objects from Go backend
      const d = typeof date === 'string' ? new Date(date) : new Date(date);
      if (isNaN(d.getTime())) return 'N/A';
      return d.toLocaleDateString('en-US', {
        month: 'short', day: 'numeric', year: 'numeric'
      });
    } catch {
      return 'N/A';
    }
  }

  function formatCurrency(value: number): string {
    return Number(value || 0).toLocaleString('en-US', {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3
    });
  }

  function getInvoiceDisplayStatus(status: string): string {
    switch (String(status || '')) {
      case 'Sent':
        return 'Open';
      case 'PartiallyPaid':
        return 'Partial';
      case 'Paid':
        return 'Settled';
      default:
        return String(status || '');
    }
  }

  onMount(loadProfile);
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="detail-view">
  <header class="header">
    <button class="back-btn" on:click={goBack}>Back to Customers</button>
    {#if profile}
      <div class="header-info">
        <h1>{profile.business_name}</h1>
        <div class="header-badges">
          <span class="grade-badge grade-{profile.payment_grade?.toLowerCase()}">{profile.payment_grade}</span>
          <span class="type-badge">{profile.customer_type}</span>
          {#if !editMode}
            <Button variant="secondary" size="sm" on:click={() => showTaskModal = true}>Create Task</Button>
            <Button variant="secondary" size="sm" on:click={startEdit}>Edit</Button>
            {#if !customerDeleted}
              <Button variant="secondary" size="sm" on:click={() => showDeleteConfirm = true} style="color: #e74c3c; border-color: #e74c3c;">Delete</Button>
            {/if}
          {/if}
        </div>
      </div>
    {/if}
  </header>

  {#if loading}
    <div class="loading-state"><WabiSpinner size="lg" /></div>
  {:else if error}
    <div class="error-state">
      <p class="error-message">{error}</p>
      <Button variant="primary" on:click={loadProfile}>Retry</Button>
    </div>
  {:else if profile}
    <!-- Contacts Strip - Full Width -->
    <div class="contacts-strip">
      <div class="contacts-strip-header">
        <h3 class="strip-title">CONTACTS</h3>
        <div class="contacts-actions">
          <button class="scroll-contact-btn" on:click={() => scrollContacts(-1)} aria-label="Scroll contacts left">&lt;</button>
          <button class="scroll-contact-btn" on:click={() => scrollContacts(1)} aria-label="Scroll contacts right">&gt;</button>
          <button class="add-contact-btn-inline" on:click={() => contactManagerMode = !contactManagerMode}>{contactManagerMode ? 'Done' : 'Manage'}</button>
          <button class="add-contact-btn-inline" on:click={openAddContact}>+ Add</button>
        </div>
      </div>
      <div class="contacts-scroll-container" bind:this={contactsScrollEl}>
        {#if customerContacts().length > 0}
          {#each customerContacts() as contact}
            <div class="contact-card">
              <div class="contact-card-avatar">
                {contact.contact_name?.charAt(0)?.toUpperCase() || '?'}
              </div>
              <div class="contact-card-info">
                <span class="contact-card-name">
                  {contact.contact_name}
                {#if contact.is_primary_contact}<span class="primary-tag">Primary</span>{/if}
                  {#if contact.is_decision_maker}<span class="primary-tag decision">Decision</span>{/if}
                </span>
                {#if contact.job_title}<span class="contact-card-role">{contact.job_title}</span>{/if}
                {#if contact.decision_role}<span class="contact-card-detail">{contact.decision_role}</span>{/if}
                {#if contact.reports_to_name}<span class="contact-card-detail">Reports to {contact.reports_to_name}</span>{/if}
                {#if contact.email}<span class="contact-card-detail">{contact.email}</span>{/if}
                {#if contact.phone}<span class="contact-card-detail">{contact.phone}</span>{/if}
                {#if contactManagerMode}
                  <div class="contact-card-actions">
                    <button on:click={() => editContact(contact)}>Edit</button>
                    <button class="danger-link" on:click={() => deleteContact(contact)}>Delete</button>
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        {:else}
          <div class="contact-card empty-contact">
            <span class="empty-contact-text">No contacts on file</span>
          </div>
        {/if}
      </div>
    </div>

    <div class="content-layout">
      <!-- Left Sidebar -->
      <aside class="sidebar">
        <Card padding="md">
          <h3 class="sidebar-title">OVERVIEW</h3>
          <div class="info-grid">
            <div class="info-item">
              <span class="label">Customer ID</span>
              <span class="value code">{profile.customer_id}</span>
            </div>
            <div class="info-item">
              <span class="label">TRN</span>
              <span class="value code">{profile.trn || 'N/A'}</span>
            </div>
            <div class="info-item">
              <span class="label">Industry</span>
              <span class="value">{profile.industry || 'N/A'}</span>
            </div>
            <div class="info-item">
              <span class="label">Relationship</span>
              <span class="value">{profile.relation_years} years</span>
            </div>
            <div class="info-item">
              <span class="label">Location</span>
              <span class="value">{profile.city}, {profile.country}</span>
            </div>
          </div>
        </Card>

        <Card padding="md">
          <h3 class="sidebar-title">FINANCIAL</h3>
          <div class="info-grid">
            <div class="info-item">
              <span class="label">Payment Terms</span>
              <span class="value">{profile.payment_terms_days} days</span>
            </div>
            <div class="info-item">
              <span class="label">Open Receivables</span>
              <span class="value">{formatCurrency(profile.outstanding_bhd)} BHD</span>
            </div>
            <div class="info-item">
              <span class="label">Overdue Receivables</span>
              <span class="value" class:danger={profile.overdue_bhd > 0}>{formatCurrency(profile.overdue_bhd)} BHD</span>
            </div>
            <div class="info-item">
              <span class="label">Credit Blocked</span>
              <span class="value">{profile.is_credit_blocked ? 'Yes' : 'No'}</span>
            </div>
          </div>
        </Card>
      </aside>

      <!-- Main Content -->
      <main class="main">
        <nav class="tabs">
          {#each tabs as tab}
            <button
              class="tab"
              class:active={activeTab === tab.id}
              on:click={() => activeTab = tab.id}
            >
              {tab.label}
            </button>
          {/each}
        </nav>

        <div class="tab-content">
          {#if activeTab === 'overview'}
            {#if editMode}
              <Card padding="md">
                <h3 class="section-title">EDIT CUSTOMER</h3>
                <div class="edit-form">
                  <div class="form-row">
                    <div class="form-group"><label for="customer-business-name">Business Name</label><input id="customer-business-name" type="text" bind:value={editForm.business_name} /></div>
                    <div class="form-group"><label for="customer-code">Customer Code</label><input id="customer-code" type="text" bind:value={editForm.customer_code} /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-type">Customer Type</label><input id="customer-type" type="text" bind:value={editForm.customer_type} /></div>
                    <div class="form-group"><label for="customer-industry">Industry</label><input id="customer-industry" type="text" bind:value={editForm.industry} /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-address">Address</label><input id="customer-address" type="text" bind:value={editForm.address_line1} /></div>
                    <div class="form-group"><label for="customer-city">City</label><input id="customer-city" type="text" bind:value={editForm.city} /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-country">Country</label><input id="customer-country" type="text" bind:value={editForm.country} /></div>
                    <div class="form-group"><label for="customer-trn">TRN</label><input id="customer-trn" type="text" bind:value={editForm.trn} /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-phone">Phone</label><input id="customer-phone" type="text" bind:value={editForm.phone} placeholder="+973 17 XXXXXX" /></div>
                    <div class="form-group"><label for="customer-mobile">Mobile Number</label><input id="customer-mobile" type="text" bind:value={editForm.mobile_number} placeholder="+973 3XXX XXXX" /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-email">Email</label><input id="customer-email" type="email" bind:value={editForm.email} placeholder="email@company.com" /></div>
                    <div class="form-group"><label for="customer-vat">VAT Number</label><input id="customer-vat" type="text" bind:value={editForm.vat_number} /></div>
                  </div>
                  <div class="form-row">
                    <div class="form-group"><label for="customer-payment-grade">Payment Grade</label>
                      <select id="customer-payment-grade" bind:value={editForm.payment_grade}>
                        <option value="A">A</option><option value="B">B</option>
                        <option value="C">C</option><option value="D">D</option>
                      </select>
                    </div>
                    <div class="form-group"><label for="customer-payment-terms">Payment Terms (Days)</label><input id="customer-payment-terms" type="number" bind:value={editForm.payment_terms_days} /></div>
                  </div>
                  <div class="form-actions">
                    <Button variant="secondary" on:click={cancelEdit} disabled={saveLoading}>Cancel</Button>
                    <Button variant="primary" on:click={saveEdit} disabled={saveLoading}>
                      {saveLoading ? 'Saving...' : 'Save Changes'}
                    </Button>
                  </div>
                </div>
              </Card>
            {/if}
            <div class="metrics-grid">
              <Card padding="md">
                <div class="metric">
                  <span class="metric-label">Commercial Value</span>
                  <span class="metric-value">{formatCurrency(profile.total_revenue)}</span>
                  <span class="metric-currency">BHD all time</span>
                </div>
              </Card>
              <Card padding="md">
                <div class="metric">
                  <span class="metric-label">Total Orders</span>
                  <span class="metric-value">{profile.total_orders}</span>
                </div>
              </Card>
              <Card padding="md">
                <div class="metric">
                  <span class="metric-label">Avg Order Value</span>
                  <span class="metric-value">{formatCurrency(profile.avg_order_value)}</span>
                  <span class="metric-currency">BHD</span>
                </div>
              </Card>
              <Card padding="md">
                <div class="metric">
                  <span class="metric-label">Win Rate</span>
                  <span class="metric-value">{profile.win_rate?.toFixed(0) || 0}%</span>
                </div>
              </Card>
            </div>

            <Card padding="md">
              <h3 class="section-title">AR AGING · OPEN INVOICES ONLY</h3>
              <div class="aging-grid">
                <div class="aging-item">
                  <span class="aging-label">Current</span>
                  <span class="aging-value">{formatCurrency(profile.ar_aging_buckets?.current)}</span>
                  <span class="aging-currency">BHD</span>
                </div>
                <div class="aging-item">
                  <span class="aging-label">31-60 Days</span>
                  <span class="aging-value">{formatCurrency(profile.ar_aging_buckets?.days_30_60)}</span>
                  <span class="aging-currency">BHD</span>
                </div>
                <div class="aging-item">
                  <span class="aging-label">61-90 Days</span>
                  <span class="aging-value">{formatCurrency(profile.ar_aging_buckets?.days_60_90)}</span>
                  <span class="aging-currency">BHD</span>
                </div>
                <div class="aging-item">
                  <span class="aging-label">90+ Days</span>
                  <span class="aging-value danger">{formatCurrency((profile.ar_aging_buckets?.days_90_120 || 0) + (profile.ar_aging_buckets?.days_120_plus || 0))}</span>
                  <span class="aging-currency danger">BHD</span>
                </div>
              </div>
            </Card>

          {:else if activeTab === 'orders'}
            <Card padding="md">
              <h3 class="section-title">RECENT ORDERS</h3>
              {#if profile.recent_orders?.length > 0}
                <div class="list">
                  {#each profile.recent_orders as order}
                    <div class="list-item">
                      <div class="item-main">
                        <span class="item-code">{order.order_number}</span>
                        <span class="item-date">{formatDate(order.order_date)}</span>
                      </div>
                      <div class="item-meta">
                        <span class="item-status">{order.status}</span>
                        <span class="item-amount">{formatCurrency(order.total_value_bhd)} BHD</span>
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="empty-text">No orders found</p>
              {/if}
            </Card>

          {:else if activeTab === 'invoices'}
            <Card padding="md">
              <h3 class="section-title">RECENT BILLINGS</h3>
              {#if profile.recent_invoices?.length > 0}
                <div class="list">
                  {#each profile.recent_invoices as invoice}
                    <div class="list-item">
                      <div class="item-main">
                        <span class="item-code">{invoice.invoice_number}</span>
                        <span class="item-date">{formatDate(invoice.invoice_date)}</span>
                      </div>
                      <div class="item-meta">
                        <span class="item-status status-{invoice.status?.toLowerCase()}">{getInvoiceDisplayStatus(invoice.status)}</span>
                        <span class="item-amount" class:open-balance={(invoice.outstanding_bhd || 0) > 0}>
                          {#if (invoice.outstanding_bhd || 0) > 0}
                            {formatCurrency(invoice.outstanding_bhd)} BHD open
                          {:else}
                            {formatCurrency(invoice.grand_total_bhd)} BHD settled
                          {/if}
                        </span>
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="empty-text">No billings found</p>
              {/if}
            </Card>

          {:else if activeTab === 'rfqs'}
            <Card padding="md">
              <h3 class="section-title">RFQ HISTORY</h3>
              <div class="rfq-stats">
                <span>Floated: {profile.rfqs_floated}</span>
                <span>Won: {profile.rfqs_won}</span>
                <span>Win Rate: {profile.win_rate?.toFixed(0) || 0}%</span>
              </div>
              {#if profile.recent_rfqs?.length > 0}
                <div class="list">
                  {#each profile.recent_rfqs as rfq}
                    <div class="list-item">
                      <div class="item-main">
                        <span class="item-code">{rfq.project}</span>
                        <span class="item-date">{formatDate(rfq.created_at)}</span>
                      </div>
                      <div class="item-meta">
                        <span class="item-status">{rfq.status}</span>
                        <span class="item-amount">{formatCurrency(rfq.value)} BHD</span>
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="empty-text">No RFQs found</p>
              {/if}
            </Card>

          {:else if activeTab === 'notes'}
            <Card padding="md">
              <div class="notes-header">
                <h3 class="section-title">NOTES</h3>
                <Button variant="primary" on:click={() => showNoteModal = true}>+ Add Note</Button>
              </div>
              {#if profile.notes?.length > 0}
                <div class="notes-list">
                  {#each profile.notes as note}
                    <div class="note-item">
                      <div class="note-header">
                        <span class="note-type">{note.note_type}</span>
                        <span class="note-date">{formatDate(note.created_at)}</span>
                      </div>
                      <p class="note-content">{note.content}</p>
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="empty-text">No notes yet</p>
              {/if}
            </Card>
          {/if}
        </div>
      </main>
    </div>
  {/if}

  {#if showDeleteConfirm}
    <div
      class="delete-overlay"
      role="button"
      tabindex="0"
      aria-label="Close delete customer confirmation"
      on:click={(event) => handleBackdropClick(event, closeDeleteConfirm)}
      on:keydown={(event) => handleBackdropKeydown(event, closeDeleteConfirm)}
    >
      <div class="delete-modal" role="dialog" aria-modal="true" aria-labelledby="delete-customer-title">
        <h3 id="delete-customer-title" style="margin: 0 0 12px 0; color: #e74c3c;">Delete Customer</h3>
        <p style="margin: 0 0 20px 0; color: var(--text-secondary);">
          Are you sure you want to delete <strong>{profile.business_name}</strong>? This action will soft-delete the customer record.
        </p>
        <div style="display: flex; gap: 8px; justify-content: flex-end;">
          <Button variant="secondary" size="sm" on:click={closeDeleteConfirm} disabled={deleteLoading}>Cancel</Button>
          <Button variant="primary" size="sm" on:click={handleDeleteCustomer} disabled={deleteLoading} style="background: #e74c3c; border-color: #e74c3c;">
            {deleteLoading ? 'Deleting...' : 'Delete'}
          </Button>
        </div>
      </div>
    </div>
  {/if}
</div>

{#if showNoteModal}
  <div
    class="modal-backdrop"
    role="button"
    tabindex="0"
    aria-label="Close add note modal"
    on:click={(event) => handleBackdropClick(event, closeNoteModal)}
    on:keydown={(event) => handleBackdropKeydown(event, closeNoteModal)}
  >
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="add-note-title">
      <h3 id="add-note-title">Add Note</h3>
      <div class="form-group">
        <label for="note-type">Type</label>
        <select id="note-type" bind:value={newNoteType}>
          <option value="general">General</option>
          <option value="delivery">Delivery</option>
          <option value="payment">Payment</option>
          <option value="issue">Issue</option>
        </select>
      </div>
      <div class="form-group">
        <label for="note-content">Content</label>
        <textarea id="note-content" bind:value={newNoteContent} rows="4" placeholder="Enter note..."></textarea>
      </div>
      <div class="modal-actions">
        <Button variant="ghost" on:click={closeNoteModal}>Cancel</Button>
        <Button variant="primary" on:click={saveNote}>Save Note</Button>
      </div>
    </div>
  </div>
{/if}

{#if showContactModal}
  <div
    class="modal-backdrop"
    role="button"
    tabindex="0"
    aria-label="Close contact modal"
    on:click={(event) => handleBackdropClick(event, closeContactModal)}
    on:keydown={(event) => handleBackdropKeydown(event, closeContactModal)}
  >
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="add-contact-title">
      <h3 id="add-contact-title">{editingContactId ? 'Edit Contact' : 'Add Contact'}</h3>
      <div class="form-group">
        <label for="contact-name">Name *</label>
        <input id="contact-name" type="text" bind:value={contactForm.contact_name} placeholder="Contact person name" />
      </div>
      <div class="form-group">
        <label for="contact-job-title">Job Title</label>
        <input id="contact-job-title" type="text" bind:value={contactForm.job_title} placeholder="e.g. Procurement Manager" />
      </div>
      <div class="form-group">
        <label for="contact-email">Email</label>
        <input id="contact-email" type="email" bind:value={contactForm.email} placeholder="email@company.com" />
      </div>
      <div class="form-group">
        <label for="contact-phone">Phone</label>
        <input id="contact-phone" type="text" bind:value={contactForm.phone} placeholder="+973 1234 5678" />
      </div>
      <div class="form-group">
        <label for="contact-address">Address</label>
        <textarea id="contact-address" bind:value={contactForm.address} rows="2" placeholder="Address"></textarea>
      </div>
      <div class="form-group">
        <label for="contact-decision-role">Decision Role</label>
        <select id="contact-decision-role" bind:value={contactForm.decision_role}>
          <option value="">Not set</option>
          <option value="Decision Maker">Decision Maker</option>
          <option value="Influencer">Influencer</option>
          <option value="Technical Evaluator">Technical Evaluator</option>
          <option value="Finance Approver">Finance Approver</option>
          <option value="End User">End User</option>
          <option value="Other">Other</option>
        </select>
      </div>
      <div class="form-group">
        <label for="contact-reports-to">Reports To</label>
        <input id="contact-reports-to" type="text" bind:value={contactForm.reports_to_name} placeholder="Manager or decision chain" />
      </div>
      <div class="form-group">
        <label for="contact-primary">
          <input id="contact-primary" type="checkbox" bind:checked={contactForm.is_primary_contact} /> Primary Contact
        </label>
      </div>
      <div class="form-group">
        <label for="contact-decision-maker">
          <input id="contact-decision-maker" type="checkbox" bind:checked={contactForm.is_decision_maker} /> Decision Maker
        </label>
      </div>
      <div class="modal-actions">
        <Button variant="ghost" on:click={closeContactModal}>Cancel</Button>
        <Button variant="primary" on:click={saveContact}>{editingContactId ? 'Update Contact' : 'Save Contact'}</Button>
      </div>
    </div>
  </div>
{/if}

{#if profile}
  <ContextTaskModal
    open={showTaskModal}
    title="Create Customer Task"
    subtitle={`Link work directly to ${profile.business_name}`}
    defaults={{
      customer_id: customerId,
      seed_title: `Customer follow-up: ${profile.business_name}`,
    }}
    on:close={() => showTaskModal = false}
    on:created={() => showTaskModal = false}
  />
{/if}

<style>
  .detail-view { padding: 16px; }

  .header { margin-bottom: 24px; }
  .back-btn { background: none; border: none; color: var(--brand-indigo); cursor: pointer; font-size: 14px; padding: 0; margin-bottom: 12px; }
  .back-btn:hover { text-decoration: underline; }

  .header-info { display: flex; justify-content: space-between; align-items: center; }
  .header-info h1 { margin: 0; font-size: 24px; font-weight: 500; }
  .header-badges { display: flex; gap: 8px; }

  .grade-badge { padding: 4px 12px; border-radius: 4px; font-size: 12px; font-weight: 600; }
  .grade-badge.grade-a { background: #DCFCE7; color: #166534; }
  .grade-badge.grade-b { background: #DBEAFE; color: #1E40AF; }
  .grade-badge.grade-c { background: #FEF9C3; color: #854D0E; }
  .grade-badge.grade-d { background: #FEE2E2; color: #991B1B; }

  .type-badge { padding: 4px 12px; background: var(--surface-elevated); border: 1px solid var(--border); border-radius: 4px; font-size: 12px; }

  .loading-state { display: flex; justify-content: center; padding: 100px; }

  .error-state { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 100px; gap: 16px; }
  .error-message { color: var(--text-danger, #EF4444); font-size: 14px; margin: 0; }

  .content-layout { display: grid; grid-template-columns: 280px 1fr; gap: 24px; }

  .sidebar { display: flex; flex-direction: column; gap: 16px; }
  .sidebar-title { font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-secondary); margin: 0 0 12px 0; }

  .info-grid { display: flex; flex-direction: column; gap: 12px; }
  .info-item { display: flex; flex-direction: column; gap: 2px; }
  .label { font-size: 11px; color: var(--text-muted); text-transform: uppercase; }
  .value { font-size: 14px; font-weight: 500; }
  .value.code { font-family: 'JetBrains Mono', monospace; color: var(--brand-indigo); }
  .value.danger { color: #EF4444; }

  .contacts-strip {
    margin-bottom: 24px;
    background: var(--surface, #fff);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 12px;
    padding: 16px;
  }

  .contacts-strip-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .contacts-actions { display: flex; align-items: center; gap: 6px; }

  .strip-title {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin: 0;
  }

  .add-contact-btn-inline {
    background: none;
    border: 1px solid var(--border);
    padding: 4px 12px;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.2s;
  }

  .add-contact-btn-inline:hover {
    border-color: var(--brand-indigo, #1D1D1F);
    color: var(--text-primary);
  }

  .scroll-contact-btn {
    width: 30px;
    height: 30px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface);
    color: var(--text-secondary);
    cursor: pointer;
    font-weight: 700;
  }

  .contacts-scroll-container {
    display: flex;
    gap: 12px;
    overflow-x: auto;
    padding-bottom: 8px;
    scroll-snap-type: x mandatory;
  }

  /* Scrollbar styling */
  .contacts-scroll-container::-webkit-scrollbar {
    height: 4px;
  }
  .contacts-scroll-container::-webkit-scrollbar-track {
    background: transparent;
  }
  .contacts-scroll-container::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 2px;
  }

  .contact-card {
    display: flex;
    gap: 12px;
    align-items: flex-start;
    padding: 12px 16px;
    background: var(--surface-elevated, #F8F8F8);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    min-width: 240px;
    max-width: 280px;
    flex-shrink: 0;
    scroll-snap-align: start;
    transition: border-color 0.2s;
  }

  .contact-card:hover {
    border-color: var(--brand-indigo, #6366F1);
  }

  .contact-card-avatar {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: var(--brand-indigo, #1D1D1F);
    color: white;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    font-size: 14px;
    flex-shrink: 0;
  }

  .contact-card-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .contact-card-name {
    font-weight: 600;
    font-size: 13px;
    color: var(--text-primary);
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .primary-tag {
    font-size: 9px;
    background: var(--brand-indigo, #1D1D1F);
    color: white;
    padding: 1px 6px;
    border-radius: 3px;
    font-weight: 600;
    text-transform: uppercase;
  }

  .primary-tag.decision {
    background: #eaf7ee;
    color: #2f7d42;
  }

  .contact-card-role {
    font-size: 12px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .contact-card-detail {
    font-size: 11px;
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .contact-card-actions {
    display: flex;
    gap: 6px;
    margin-top: 8px;
  }

  .contact-card-actions button {
    border: 1px solid var(--border);
    border-radius: 5px;
    background: var(--surface);
    color: var(--text-primary);
    cursor: pointer;
    font-size: 11px;
    padding: 4px 8px;
  }

  .contact-card-actions .danger-link {
    color: #b42318;
    border-color: #f2c7c3;
    background: #fff7f6;
  }

  .empty-contact {
    border-style: dashed;
    justify-content: center;
    min-width: 200px;
  }

  .empty-contact-text {
    font-size: 13px;
    color: var(--text-muted);
    font-style: italic;
  }

  .empty-text { font-size: 13px; color: var(--text-muted); font-style: italic; }

  .tabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); margin-bottom: 16px; }
  .tab { padding: 12px 20px; background: none; border: none; border-bottom: 2px solid transparent; font-size: 14px; font-weight: 500; color: var(--text-secondary); cursor: pointer; transition: all var(--transition-fast); }
  .tab:hover { color: var(--text-primary); }
  .tab.active { color: var(--text-primary); border-bottom-color: var(--brand-indigo); }

  .metrics-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 16px; }
  .metric { text-align: center; display: flex; flex-direction: column; align-items: center; }
  .metric-label { display: block; font-size: 11px; text-transform: uppercase; color: var(--text-secondary); margin-bottom: 4px; }
  .metric-value { font-size: 24px; font-weight: 600; }
  .metric-currency { font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-secondary); margin-top: 4px; }

  .section-title { font-size: 12px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-secondary); margin: 0 0 16px 0; }

  .aging-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }
  .aging-item { text-align: center; padding: 12px; background: var(--surface-elevated); border-radius: 8px; display: flex; flex-direction: column; align-items: center; }
  .aging-label { display: block; font-size: 11px; color: var(--text-muted); margin-bottom: 4px; }
  .aging-value { font-size: 16px; font-weight: 600; font-family: 'JetBrains Mono', monospace; }
  .aging-value.danger { color: #EF4444; }
  .aging-currency { font-size: 10px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-secondary); margin-top: 4px; }
  .aging-currency.danger { color: #EF4444; }

  .list { display: flex; flex-direction: column; gap: 8px; }
  .list-item { display: flex; justify-content: space-between; align-items: center; padding: 12px; background: var(--surface-elevated); border-radius: 6px; transition: all var(--transition-fast); }
  .list-item:hover { background: var(--surface-hover); }
  .item-main { display: flex; flex-direction: column; gap: 2px; }
  .item-code { font-weight: 600; font-family: 'JetBrains Mono', monospace; color: var(--brand-indigo); }
  .item-date { font-size: 12px; color: var(--text-muted); }
  .item-meta { display: flex; align-items: center; gap: 16px; }
  .item-status { font-size: 12px; padding: 2px 8px; background: var(--surface); border-radius: 4px; }
  .item-status.status-sent { color: #3B82F6; }
  .item-status.status-paid { color: #10B981; }
  .item-status.status-partiallypaid { color: #F59E0B; }
  .item-status.status-overdue { color: #EF4444; }
  .item-amount { font-weight: 600; font-family: 'JetBrains Mono', monospace; }
  .item-amount.open-balance { color: #F59E0B; }

  .rfq-stats { display: flex; gap: 24px; margin-bottom: 16px; font-size: 14px; color: var(--text-secondary); }

  .notes-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
  .notes-list { display: flex; flex-direction: column; gap: 12px; }
  .note-item { padding: 12px; background: var(--surface-elevated); border-radius: 6px; }
  .note-header { display: flex; justify-content: space-between; margin-bottom: 8px; }
  .note-type { font-size: 11px; text-transform: uppercase; padding: 2px 8px; background: var(--brand-indigo-tint); color: var(--brand-indigo); border-radius: 4px; }
  .note-date { font-size: 12px; color: var(--text-muted); }
  .note-content { margin: 0; font-size: 14px; line-height: 1.5; }

  .modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
  .modal { background: var(--surface); padding: 24px; border-radius: 12px; width: 440px; max-height: 88vh; overflow-y: auto; }
  .modal h3 { margin: 0 0 16px 0; }
  .form-group { margin-bottom: 16px; }
  .form-group label { display: block; font-size: 12px; text-transform: uppercase; color: var(--text-muted); margin-bottom: 4px; }
  .form-group select, .form-group textarea { width: 100%; padding: 8px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; background: var(--surface); color: var(--text-primary); font-family: var(--font-family); }
  .form-group textarea { resize: vertical; }
  .modal-actions { display: flex; justify-content: flex-end; gap: 12px; }
  .edit-form { display: flex; flex-direction: column; gap: 12px; }
  .edit-form .form-row { display: flex; gap: 16px; }
  .edit-form .form-row .form-group { flex: 1; }
  .edit-form .form-group label { display: block; font-size: 11px; font-weight: 500; color: var(--steel, #86868B); margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.03em; }
  .edit-form .form-group input, .edit-form .form-group select { width: 100%; padding: 8px 10px; border: 1px solid var(--border, #E5E5E5); border-radius: 6px; font-size: 14px; }
  .edit-form .form-group input:focus, .edit-form .form-group select:focus { outline: none; border-color: var(--onyx, #1D1D1F); }
  .form-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 8px; }

  .delete-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .delete-modal {
    background: var(--surface, #fff);
    border-radius: 12px;
    padding: 24px;
    max-width: 420px;
    width: 90%;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  }
</style>
