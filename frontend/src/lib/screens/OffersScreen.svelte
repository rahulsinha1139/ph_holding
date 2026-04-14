<script lang="ts">
  /**
   * OffersScreen - Production-Ready Offers Management
   * Features:
   * - View all offers/quotes with filtering by status
   * - Create new offers with customer and product selection
   * - Edit existing offers
   * - Convert accepted offers to orders
   * - Track validity dates with expiry warnings
   * - PDF generation for quotations
   * - Link to parent RFQs
   */

  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { main } from '../../../wailsjs/go/models';
  import {
    GetAllOffers,
    SaveCostingAsOffer,
    UpdateOfferFull,
    MarkOfferWon,
    MarkOfferLost,
    ListCustomers,
    AddOfferFollowUp,
    GetPendingFollowUps,
    CompleteOfferFollowUp,
    GenerateOfferPDF,
    OpenExportedFile,
    AddOfferNote,
    GetOfferNotes,
    DeleteOfferNote,
    GetOffersWithNoItems
  } from '../../../wailsjs/go/main/App';
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
  import WabiModal from '$lib/components/ui/WabiModal.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import { toast } from '$lib/stores/toasts';
  import { escapeHtml } from '$lib/utils/escapeHtml';

  // Props
export let embedded = false;
$: embedded;

  type OfferStage = 'all' | 'RFQ' | 'Quoted' | 'Won' | 'Lost';

  interface OfferDisplay {
    id: string;
    costing_id?: string;
    customer_name: string;
    project_name?: string;
    amount: number;
    status: string;
    stage?: string;
    offer_number?: string;
    quotation_date?: string;
    validity_date?: string;
    total_value_bhd?: number;
    pdf_path?: string;
    sent_at?: any;
    created_at?: any;
    updated_at?: any;
    estimated_margin?: number;
    items?: any[];
    division?: string;
    days_until_expiry?: number;
    is_expiring_soon?: boolean;
    is_expired?: boolean;
    // Editable header fields
    folder_number?: string;
    payment_terms?: string;
    delivery_terms?: string;
    delivery_weeks?: string;
    country_of_origin?: string;
    issued_by?: string;
    contact_phone?: string;
    customer_reference?: string;
    attention_person?: string;
    attention_company?: string;
    attention_phone?: string;
    attention_address?: string;
    discount_percent?: number;
    coc_coo?: string;
    test_certificate?: string;
    installation?: string;
    commissioning?: string;
    testing?: string;
    has_abb_competition?: boolean;
    lost_reason?: string;
  }

  // State
  let offers: OfferDisplay[] = [];
  let filteredOffers: OfferDisplay[] = [];
  let loading = true;
  let selectedCompany: 'PH Trading' | 'AHS Trading' = 'PH Trading';
  let selectedStage: OfferStage = 'all';
  let offersWithNoItems: any[] = [];
  let legacyOfferShells: any[] = [];

  // Modal state
  let showCreateModal = false;
  let showEditModal = false;
  let editingOffer: OfferDisplay | null = null;

  // Won/Lost modal state
  let showWonModal = false;
  let showLostModal = false;
  let actionOffer: OfferDisplay | null = null;
  let customerPONumber = '';
  let lostReason = '';
  let actionLoading = false;
  let showViewModal = false;
  let viewOffer: OfferDisplay | null = null;

  // View modal save state
  let viewSaving = false;

  // Notes state
  let offerNotes: any[] = [];
  let newNoteContent = '';
  let notesLoading = false;

  // Follow-up state
  let showFollowUpModal = false;
  let followUpOffer: OfferDisplay | null = null;
  let followUpDate = '';
  let followUpNotes = '';
  let pendingFollowUps: any[] = [];
  let followUpLoading = false;
  let pdfLoading = false;

  async function openFollowUpModal(offer: OfferDisplay) {
    followUpOffer = offer;
    followUpDate = '';
    followUpNotes = '';
    showFollowUpModal = true;
  }

  async function scheduleFollowUp() {
    if (!followUpOffer || !followUpDate) return;

    // Validate date is in the future
    const selectedDate = new Date(followUpDate);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    if (selectedDate < today) {
      toast.danger('Follow-up date must be in the future');
      return;
    }

    followUpLoading = true;
    try {
      await AddOfferFollowUp(followUpOffer.id, followUpDate, followUpNotes);
      showFollowUpModal = false;
      toast.success('Follow-up scheduled');
      await loadPendingFollowUps();
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to schedule follow-up: ${errorMsg}`);
    } finally {
      followUpLoading = false;
    }
  }

  async function completeFollowUp(id: string) {
    followUpLoading = true;
    try {
      await CompleteOfferFollowUp(id);
      toast.success('Follow-up completed');
      await loadPendingFollowUps();
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to complete follow-up: ${errorMsg}`);
    } finally {
      followUpLoading = false;
    }
  }

  async function loadPendingFollowUps() {
    try {
      pendingFollowUps = await GetPendingFollowUps() || [];
    } catch {
      pendingFollowUps = [];
    }
  }

  // Form state
  let customers: main.CustomerMaster[] = [];

  interface EditableItem {
    product_id: string;
    description: string;
    equipment: string;
    model: string;
    specification: string;
    detailed_description: string;
    currency: string;
    quantity: number;
    fob: number;
    freight: number;
    total_cost: number;
    margin_percent: number;
    unit_price: number;
    total_price: number;
  }

  let formData = {
    division: 'PH Trading',
    customer_id: '',
    customer_name: '',
    project_name: '',
    folder_number: '',
    quotation_date: new Date().toISOString().split('T')[0],
    validity_date: '',
    payment_terms: '',
    delivery_terms: '',
    delivery_weeks: '',
    country_of_origin: '',
    contact_phone: '',
    customer_reference: '',
    attention_person: '',
    attention_company: '',
    attention_phone: '',
    attention_address: '',
    issued_by: '',
    items: [] as EditableItem[]
  };
  let formLoading = false;

  // DataTable columns configuration
  const columns = [
    {
      key: 'offer_number',
      label: 'Offer #',
      sortable: true,
      width: '120px'
    },
    {
      key: 'customer_name',
      label: 'Customer',
      sortable: true,
      render: (row: OfferDisplay) => escapeHtml(row.customer_name)
    },
    {
      key: 'quotation_date',
      label: 'Date',
      type: 'date' as const,
      sortable: true,
      width: '120px'
    },
    {
      key: 'validity_date',
      label: 'Valid Until',
      type: 'date' as const,
      sortable: true,
      width: '120px',
      render: (row: OfferDisplay) => {
        const date = new Date(row.validity_date).toLocaleDateString('en-US', {
          year: 'numeric',
          month: 'short',
          day: 'numeric'
        });

        if (row.is_expired) {
          return `<span style="color: #DC2626; font-weight: 500;">${date}</span>`;
        } else if (row.is_expiring_soon) {
          return `<span style="color: #F59E0B; font-weight: 500;">${date}</span>`;
        }
        return date;
      }
    },
    {
      key: 'total_value_bhd',
      label: 'Total Value',
      type: 'currency' as const,
      align: 'right' as const,
      sortable: true,
      width: '140px'
    },
    {
      key: 'stage',
      label: 'Status',
      type: 'status' as const,
      sortable: true,
      width: '110px'
    },
    {
      key: 'actions',
      label: 'Actions',
      type: 'actions' as const,
      width: '220px',
      render: (row: OfferDisplay) => {
        const isQuoted = row.stage === 'Quoted';
        const isLost = row.stage === 'Lost';
        const hasItems = (row.items?.length || 0) > 0;
        return `
          <div style="display: flex; gap: 6px; justify-content: flex-end; flex-wrap: wrap;">
            ${hasItems ? `
              <button
                class="action-btn action-btn-view"
                data-action="view"
                data-id="${row.id}"
                aria-label="View full details"
              >
                View
              </button>
            ` : ''}
            <button
              class="action-btn action-btn-edit"
              data-action="edit"
              data-id="${row.id}"
              aria-label="Edit offer"
            >
              Edit
            </button>
            <button
              class="action-btn action-btn-pdf"
              data-action="pdf"
              data-id="${row.id}"
              aria-label="Download PDF"
              style="background: var(--carbon); color: white;"
            >
              PDF
            </button>
            ${isQuoted ? `
              <button
                class="action-btn action-btn-won"
                data-action="won"
                data-id="${row.id}"
                aria-label="Mark as won"
              >
                Won
              </button>
              <button
                class="action-btn action-btn-lost"
                data-action="lost"
                data-id="${row.id}"
                aria-label="Mark as lost"
              >
                Lost
              </button>
            ` : ''}
            ${isLost ? `
              <button
                class="action-btn action-btn-requote"
                data-action="requote"
                data-id="${row.id}"
                aria-label="Re-quote this offer"
              >
                Re-quote
              </button>
            ` : ''}
            ${!isLost ? `
              <button
                class="action-btn action-btn-followup"
                data-action="followup"
                data-id="${row.id}"
                aria-label="Schedule follow-up"
              >
                Follow-Up
              </button>
            ` : ''}
          </div>
        `;
      }
    }
  ];

  // Stage filter tabs
  const stageTabs: { value: OfferStage; label: string; count: number }[] = [
    { value: 'all', label: 'All Offers', count: 0 },
    { value: 'RFQ', label: 'RFQ', count: 0 },
    { value: 'Quoted', label: 'Quoted', count: 0 },
    { value: 'Won', label: 'Won', count: 0 },
    { value: 'Lost', label: 'Lost', count: 0 }
  ];

  $: companyScopedOffers = offers.filter((offer) => (offer.division || 'PH Trading') === selectedCompany);

  // Computed: Update tab counts
  $: {
    stageTabs[0].count = companyScopedOffers.length;
    stageTabs[1].count = companyScopedOffers.filter(o => o.stage === 'RFQ').length;
    stageTabs[2].count = companyScopedOffers.filter(o => o.stage === 'Quoted').length;
    stageTabs[3].count = companyScopedOffers.filter(o => o.stage === 'Won').length;
    stageTabs[4].count = companyScopedOffers.filter(o => o.stage === 'Lost').length;
  }

  $: legacyOfferShells = offersWithNoItems.filter((item) => item?.is_legacy_shell);
  $: actionableOffersWithNoItems = offersWithNoItems.filter((item) => !item?.is_legacy_shell);

  // Computed: Filter offers by selected stage
  $: {
    if (selectedStage === 'all') {
      filteredOffers = companyScopedOffers;
    } else {
      filteredOffers = companyScopedOffers.filter(o => o.stage === selectedStage);
    }
  }

  async function loadOffers() {
    loading = true;
    try {
      const [offersData, customersData] = await Promise.all([
        GetAllOffers(),
        ListCustomers(1000, 0)
      ]);

      offers = (offersData || []).map(enrichOfferWithExpiry);
      customers = customersData || [];
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to load offers: ${errorMsg}`);
      offers = [];
    } finally {
      loading = false;
    }
  }

  function enrichOfferWithExpiry(offer: any): OfferDisplay {
    const now = new Date();
    const validityDate = offer.validity_date ? new Date(offer.validity_date) : new Date(offer.created_at || now);
    const daysUntilExpiry = Math.ceil((validityDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

    return {
      id: offer.id,
      costing_id: offer.costing_id,
      customer_name: offer.customer_name || '',
      project_name: offer.project_name || '',
      amount: offer.total_value_bhd || 0,
      status: offer.stage || 'Quoted',
      stage: offer.stage || 'Quoted',
      offer_number: offer.offer_number || `OFR-${(offer.id || '').toString().slice(0, 8)}`,
      quotation_date: offer.quotation_date || offer.created_at,
      validity_date: offer.validity_date,
      total_value_bhd: offer.total_value_bhd || 0,
      estimated_margin: offer.estimated_margin,
      pdf_path: offer.pdf_path,
      sent_at: offer.sent_at,
      created_at: offer.created_at,
      updated_at: offer.updated_at,
      items: offer.items || [],
      division: offer.division || 'PH Trading',
      days_until_expiry: daysUntilExpiry,
      is_expired: daysUntilExpiry < 0,
      is_expiring_soon: daysUntilExpiry >= 0 && daysUntilExpiry <= 7,
      // Header fields for view/edit modal
      folder_number: offer.folder_number || '',
      payment_terms: offer.payment_terms || '',
      delivery_terms: offer.delivery_terms || '',
      delivery_weeks: offer.delivery_weeks || '',
      country_of_origin: offer.country_of_origin || '',
      issued_by: offer.issued_by || '',
      contact_phone: offer.contact_phone || '',
      customer_reference: offer.customer_reference || '',
      attention_person: offer.attention_person || '',
      attention_company: offer.attention_company || '',
      attention_phone: offer.attention_phone || '',
      attention_address: offer.attention_address || '',
      discount_percent: offer.discount_percent,
      coc_coo: offer.coc_coo || '',
      test_certificate: offer.test_certificate || '',
      installation: offer.installation || '',
      commissioning: offer.commissioning || '',
      testing: offer.testing || '',
      has_abb_competition: offer.has_abb_competition,
      lost_reason: offer.lost_reason || '',
    };
  }

  function normalizeName(value: string) {
    return (value || '').trim().toLowerCase().replace(/\s+/g, ' ');
  }

  function handleCompanySelect(company: string) {
    selectedCompany = company === 'AHS Trading' ? 'AHS Trading' : 'PH Trading';
    selectedStage = 'all';
  }

  function resolveCustomerFromOffer(customerId: string, customerName: string) {
    const byId = customerId ? customers.find(c => c.id === customerId) : null;
    if (byId) {
      return {
        customer_id: byId.id,
        customer_name: byId.business_name || customerName || ''
      };
    }

    const normalizedCustomerName = normalizeName(customerName);
    if (!normalizedCustomerName) {
      return {
        customer_id: customerId || '',
        customer_name: customerName || ''
      };
    }

    const byName = customers.find((c) => normalizeName(c.business_name || '') === normalizedCustomerName);
    return {
      customer_id: byName?.id || '',
      customer_name: byName?.business_name || customerName || ''
    };
  }

  // Open create modal - REDIRECTS to Costing Sheet (Issue #7)
  // Offers MUST come from costing sheet to ensure proper line items and pricing
  function openCreateModal() {
    // Dispatch navigation event to parent to switch to CostingSheetScreen
    // This ensures all offers have proper line items from costing calculations
    toast.info('Opening Costing Sheet to create a new offer with line items');

    // Dispatch custom event for navigation (App.svelte handles routing)
    const event = new CustomEvent('navigate', {
      bubbles: true,
      detail: { screen: 'costing', message: 'Create offer from costing sheet' }
    });
    document.dispatchEvent(event);
  }

  function openEditModal(offer: OfferDisplay) {
    editingOffer = offer;
    const defaultDate = new Date().toISOString().split('T')[0];
    const defaultValidity = new Date(Date.now() + 30*24*60*60*1000).toISOString().split('T')[0];
    const resolvedCustomer = resolveCustomerFromOffer((offer as any).customer_id || '', offer.customer_name || '');

    formData = {
      division: offer.division || selectedCompany,
      customer_id: resolvedCustomer.customer_id,
      customer_name: resolvedCustomer.customer_name,
      project_name: offer.project_name || '',
      folder_number: offer.folder_number || '',
      quotation_date: offer.quotation_date ? new Date(offer.quotation_date).toISOString().split('T')[0] : defaultDate,
      validity_date: offer.validity_date ? new Date(offer.validity_date).toISOString().split('T')[0] : defaultValidity,
      payment_terms: offer.payment_terms || '',
      delivery_terms: offer.delivery_terms || '',
      delivery_weeks: offer.delivery_weeks || '',
      country_of_origin: offer.country_of_origin || '',
      contact_phone: offer.contact_phone || '',
      customer_reference: offer.customer_reference || '',
      attention_person: offer.attention_person || '',
      attention_company: offer.attention_company || '',
      attention_phone: offer.attention_phone || '',
      attention_address: offer.attention_address || '',
      issued_by: offer.issued_by || '',
      items: (offer.items || []).map((item: any) => ({
        product_id: item.product_code || '',
        description: item.description || '',
        equipment: item.equipment || item.description || '',
        model: item.model || item.product_code || '',
        specification: item.specification || '',
        detailed_description: item.detailed_description || '',
        currency: item.currency || 'BHD',
        quantity: item.quantity || 0,
        fob: item.fob || 0,
        freight: item.freight || 0,
        total_cost: item.total_cost || 0,
        margin_percent: item.margin_percent || 0,
        unit_price: item.unit_price_bhd || item.unit_price || 0,
        total_price: item.total_price || 0
      }))
    };
    showEditModal = true;
  }

  // Handle create offer (direct from screen)
  async function handleCreateOffer() {
    if (!formData.customer_id || !formData.quotation_date || !formData.validity_date) {
      toast.warning('Please fill all required fields');
      return;
    }

    // Validate dates
    const quotationDate = new Date(formData.quotation_date);
    const validityDate = new Date(formData.validity_date);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    if (quotationDate > today) {
      toast.warning('Quotation date cannot be in the future');
      return;
    }

    if (validityDate <= quotationDate) {
      toast.warning('Validity date must be after quotation date');
      return;
    }

    // Validate at least one item
    if (formData.items.length === 0) {
      toast.warning('Please add at least one line item');
      return;
    }

    // Validate all items have quantity > 0 and price > 0
    for (let i = 0; i < formData.items.length; i++) {
      const item = formData.items[i];
      if (item.quantity < 1) {
        toast.warning(`Item ${i + 1}: Quantity must be at least 1`);
        return;
      }
      if (item.unit_price <= 0) {
        toast.warning(`Item ${i + 1}: Unit price must be greater than 0`);
        return;
      }
      if (!item.description || !item.description.trim()) {
        toast.warning(`Item ${i + 1}: Description is required`);
        return;
      }
    }

    // Validate margin calculations
    const totalValue = formData.items.reduce((sum, item) => sum + (item.quantity * item.unit_price), 0);
    if (totalValue <= 0) {
      toast.warning('Total offer value must be greater than 0');
      return;
    }

    formLoading = true;
    try {
      const customer = customers.find(c => c.id === formData.customer_id);
      const costingData = {
        division: formData.division || selectedCompany,
        date: formData.quotation_date,
        preparedBy: '',
        customerName: customer?.business_name || formData.customer_name,
        contactPerson: '',
        rfqReference: '',
        folderNumber: '',
        costingId: '',
        estDelivery: '5-7 weeks',
        deliveryTerms: 'DAP Bahrain',
        paymentTerms: 'Net 30',
        orderType: 'General',
        countryOfOrigin: 'DE',
        lineItems: formData.items.map((item, i) => ({
          slNo: i + 1,
          supplier: '',
          equipment: item.description,
          model: item.product_id || '',
          specification: '',
          currency: 'BHD',
          quantity: item.quantity,
          fob: 0,
          freight: 0,
          totalCost: 0,
          suggestedPrice: item.unit_price,
          totalPrice: item.quantity * item.unit_price,
        })),
        subtotal: totalValue,
        discount: 0,
        netAmount: totalValue,
        vat: totalValue * 0.10,
        grandTotal: totalValue * 1.10,
        totalCost: 0,
        profit: 0,
        profitPercent: 0,
        opportunityId: 0,
        projectName: '',
      };

      await SaveCostingAsOffer(costingData as any);
      toast.success('Offer created successfully');
      showCreateModal = false;
      await loadOffers();
    } catch (err) {
      toast.danger('Failed to create offer: ' + (err as Error).message);
    } finally {
      formLoading = false;
    }
  }

  // Handle edit offer
  async function handleEditOffer() {
    if (!editingOffer) return;

    // Same validations as create
    if (!formData.customer_name || !formData.quotation_date || !formData.validity_date) {
      toast.warning('Please fill all required fields');
      return;
    }

    const quotationDate = new Date(formData.quotation_date);
    const validityDate = new Date(formData.validity_date);
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    if (quotationDate > today) {
      toast.warning('Quotation date cannot be in the future');
      return;
    }

    if (validityDate <= quotationDate) {
      toast.warning('Validity date must be after quotation date');
      return;
    }

    if (formData.items.length === 0) {
      toast.warning('Please add at least one line item');
      return;
    }

    for (let i = 0; i < formData.items.length; i++) {
      const item = formData.items[i];
      if (item.quantity < 1) {
        toast.warning(`Item ${i + 1}: Quantity must be at least 1`);
        return;
      }
      if (item.unit_price <= 0) {
        toast.warning(`Item ${i + 1}: Unit price must be greater than 0`);
        return;
      }
      if (!item.description || !item.description.trim()) {
        toast.warning(`Item ${i + 1}: Description is required`);
        return;
      }
    }

    formLoading = true;
    try {
      await UpdateOfferFull(editingOffer.id, {
        customer_id: formData.customer_id,
        customer_name: formData.customer_name,
        project_name: formData.project_name,
        folder_number: formData.folder_number,
        quotation_date: formData.quotation_date,
        validity_date: formData.validity_date,
        payment_terms: formData.payment_terms,
        delivery_terms: formData.delivery_terms,
        delivery_weeks: formData.delivery_weeks,
        country_of_origin: formData.country_of_origin,
        contact_phone: formData.contact_phone,
        customer_reference: formData.customer_reference,
        attention_person: formData.attention_person,
        attention_company: formData.attention_company,
        attention_phone: formData.attention_phone,
        attention_address: formData.attention_address,
        issued_by: formData.issued_by,
        discount: 0,
        items: formData.items.map(item => ({
          description: item.description,
          model: item.model || '',
          product_code: item.product_id || '',
          supplier: '',
          quantity: item.quantity,
          unit_price: item.unit_price,
          // Extended costing fields
          equipment: item.equipment || '',
          specification: item.specification || '',
          detailed_description: item.detailed_description || '',
          currency: item.currency || 'BHD',
          fob: item.fob || 0,
          freight: item.freight || 0,
          total_cost: item.total_cost || 0,
          margin_percent: item.margin_percent || 0,
          total_price: item.total_price || (item.quantity * item.unit_price) || 0,
        })),
      } as any);

      toast.success('Offer updated successfully');
      showEditModal = false;
      editingOffer = null;
      await loadOffers();
    } catch (err) {
      toast.danger('Failed to update offer: ' + (err as Error).message);
    } finally {
      formLoading = false;
    }
  }

  async function handleMarkWon() {
    if (actionLoading) return;
    if (!actionOffer) return;

    if (!confirm(`Confirm marking offer ${actionOffer.offer_number} as WON?\n\nThis will create an order and cannot be undone.`)) {
      return;
    }

    actionLoading = true;
    try {
      await MarkOfferWon(actionOffer.id, customerPONumber);
      toast.success(`Offer ${actionOffer.offer_number} WON! Order created in pipeline.`);
      showWonModal = false;
      actionOffer = null;
      customerPONumber = '';
      await loadOffers();
    } catch (err) {
      toast.danger('Failed: ' + (err as Error).message);
    } finally {
      actionLoading = false;
    }
  }

  async function handleMarkLost() {
    if (actionLoading) return;
    if (!actionOffer || !lostReason) {
      toast.warning('Please provide a reason');
      return;
    }

    if (!confirm(`Confirm marking offer ${actionOffer.offer_number} as LOST?\n\nReason: ${lostReason}\n\nThis action cannot be undone.`)) {
      return;
    }

    actionLoading = true;
    try {
      await MarkOfferLost(actionOffer.id, lostReason);
      toast.success(`Offer ${actionOffer.offer_number} marked as lost.`);
      showLostModal = false;
      actionOffer = null;
      lostReason = '';
      await loadOffers();
    } catch (err) {
      toast.danger('Failed: ' + (err as Error).message);
    } finally {
      actionLoading = false;
    }
  }

  // Handle Re-quote action (create new revision from lost offer)
  async function handleRequote(offer: OfferDisplay) {
    if (!confirm(`Create a new revision of ${offer.offer_number} for ${offer.customer_name}?`)) return;

    try {
      // Open edit modal pre-filled with the lost offer's data for a new quote
      const resolvedCustomer = resolveCustomerFromOffer((offer as any).customer_id || '', offer.customer_name || '');
      formData = {
        division: offer.division || selectedCompany,
        customer_id: resolvedCustomer.customer_id,
        customer_name: resolvedCustomer.customer_name,
        project_name: offer.project_name || '',
        folder_number: offer.folder_number || '',
        quotation_date: new Date().toISOString().split('T')[0],
        validity_date: new Date(Date.now() + 30*24*60*60*1000).toISOString().split('T')[0],
        payment_terms: offer.payment_terms || '',
        delivery_terms: offer.delivery_terms || '',
        delivery_weeks: offer.delivery_weeks || '',
        country_of_origin: offer.country_of_origin || '',
        contact_phone: offer.contact_phone || '',
        customer_reference: offer.customer_reference || '',
        attention_person: offer.attention_person || '',
        attention_company: offer.attention_company || '',
        attention_phone: offer.attention_phone || '',
        attention_address: offer.attention_address || '',
        issued_by: offer.issued_by || '',
        items: (offer.items || []).map((item: any) => ({
          product_id: item.product_code || '',
          description: item.description || '',
          equipment: item.equipment || item.description || '',
          model: item.model || item.product_code || '',
          specification: item.specification || '',
          detailed_description: item.detailed_description || '',
          currency: item.currency || 'BHD',
          quantity: item.quantity || 0,
          fob: item.fob || 0,
          freight: item.freight || 0,
          total_cost: item.total_cost || 0,
          margin_percent: item.margin_percent || 0,
          unit_price: item.unit_price_bhd || item.unit_price || 0,
          total_price: item.total_price || 0
        })),
      };
      showCreateModal = true;
      toast.info(`Re-quoting ${offer.offer_number} - adjust pricing and save as new offer`);
    } catch (err) {
      toast.danger('Failed to prepare re-quote');
    }
  }

  // Handle action button clicks (delegated from DataTable)
  function handleRowClick(event: CustomEvent) {
    const rawTarget = event.detail.event?.target as HTMLElement;
    if (!rawTarget) return;
    // Use closest() to find the action button (handles clicks on child text nodes)
    const target = rawTarget.closest('[data-action]') as HTMLElement;
    if (!target || !target.dataset.action) return;

    const action = target.dataset.action;
    const id = target.dataset.id;
    const offer = offers.find(o => o.id === id);

    if (!offer) return;

    switch (action) {
      case 'view':
        viewOffer = offer;
        showViewModal = true;
        loadOfferNotes(offer.id);
        break;
      case 'edit':
        openEditModal(offer);
        break;
      case 'pdf':
        handleDownloadPDF(offer);
        break;
      case 'won':
        actionOffer = offer;
        customerPONumber = '';
        showWonModal = true;
        break;
      case 'lost':
        actionOffer = offer;
        lostReason = '';
        showLostModal = true;
        break;
      case 'requote':
        handleRequote(offer);
        break;
      case 'followup':
        openFollowUpModal(offer);
        break;
    }
  }

  async function handleDownloadPDF(offer: OfferDisplay) {
    if (pdfLoading) return;
    pdfLoading = true;
    try {
      toast.info(`Generating PDF for ${offer.offer_number}...`);
      const filePath = await GenerateOfferPDF(offer.id);
      toast.success(`PDF generated: ${filePath}`);
    } catch (err) {
      const errorMsg = err?.message || String(err);
      toast.danger(`Failed to generate PDF: ${errorMsg}`);
    } finally {
      pdfLoading = false;
    }
  }

  // Save editable header fields from view modal
  async function handleSaveViewOffer() {
    if (!viewOffer || viewSaving) return;
    viewSaving = true;
    try {
      await UpdateOfferFull(viewOffer.id, {
        customer_id: '',
        customer_name: viewOffer.customer_name || '',
        division: viewOffer.division || 'PH Trading',
        project_name: viewOffer.project_name || '',
        folder_number: viewOffer.folder_number || '',
        quotation_date: viewOffer.quotation_date ? new Date(viewOffer.quotation_date).toISOString().split('T')[0] : '',
        validity_date: viewOffer.validity_date ? new Date(viewOffer.validity_date).toISOString().split('T')[0] : '',
        payment_terms: viewOffer.payment_terms || '',
        delivery_terms: viewOffer.delivery_terms || '',
        delivery_weeks: viewOffer.delivery_weeks || '',
        country_of_origin: viewOffer.country_of_origin || '',
        contact_phone: viewOffer.contact_phone || '',
        customer_reference: viewOffer.customer_reference || '',
        attention_person: viewOffer.attention_person || '',
        attention_company: viewOffer.attention_company || '',
        attention_phone: viewOffer.attention_phone || '',
        attention_address: viewOffer.attention_address || '',
        issued_by: viewOffer.issued_by || '',
        discount: 0,
        stage: '',
        items: [],
      } as any);
      toast.success('Offer details saved');
      await loadOffers();
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to save: ${errorMsg}`);
    } finally {
      viewSaving = false;
    }
  }

  // Notes functions
  async function loadOfferNotes(offerId: string) {
    notesLoading = true;
    try {
      offerNotes = await GetOfferNotes(offerId) || [];
    } catch {
      offerNotes = [];
    } finally {
      notesLoading = false;
    }
  }

  async function handleAddNote() {
    if (!viewOffer || !newNoteContent.trim()) return;
    notesLoading = true;
    try {
      await AddOfferNote(viewOffer.id, newNoteContent.trim());
      newNoteContent = '';
      await loadOfferNotes(viewOffer.id);
      toast.success('Note added');
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to add note: ${errorMsg}`);
    } finally {
      notesLoading = false;
    }
  }

  async function handleDeleteNote(noteId: string) {
    if (!viewOffer) return;
    notesLoading = true;
    try {
      await DeleteOfferNote(noteId);
      await loadOfferNotes(viewOffer.id);
      toast.success('Note deleted');
    } catch (e) {
      const errorMsg = e?.message || String(e);
      toast.danger(`Failed to delete note: ${errorMsg}`);
    } finally {
      notesLoading = false;
    }
  }

  // Customer selection handler
  function handleCustomerSelect(e: Event) {
    const target = e.target as HTMLInputElement | HTMLSelectElement;
    const selectedValue = target.value;
    const customer = customers.find((c) =>
      c.id === selectedValue || normalizeName(c.business_name || '') === normalizeName(selectedValue)
    );

    if (customer) {
      formData.customer_id = customer.id;
      formData.customer_name = customer.business_name;
    } else {
      formData.customer_id = '';
      formData.customer_name = selectedValue;
    }
  }

  // Add line item with all costing fields
  function addLineItem() {
    formData.items = [...formData.items, {
      product_id: '',
      description: '',
      equipment: '',
      model: '',
      specification: '',
      detailed_description: '',
      currency: 'BHD',
      quantity: 1,
      fob: 0,
      freight: 0,
      total_cost: 0,
      margin_percent: 20,
      unit_price: 0,
      total_price: 0
    }];
  }

  // Remove line item
  function removeLineItem(index: number) {
    formData.items = formData.items.filter((_, i) => i !== index);
  }

  // Computed: Total value
  $: totalValue = formData.items.reduce((sum, item) => sum + (item.quantity * item.unit_price), 0);

  onMount(() => {
    loadOffers();
    loadPendingFollowUps();
    loadOffersWithNoItems();
  });

  async function loadOffersWithNoItems() {
    try {
      const data = await GetOffersWithNoItems();
      offersWithNoItems = data || [];
    } catch (err) {
      // Non-critical — silently ignore
      offersWithNoItems = [];
    }
  }
</script>

<PageLayout title="Offers" subtitle="Quotations & Proposals">
  <svelte:fragment slot="header-actions">
    <Button variant="primary" on:click={openCreateModal}>
      + New Offer (via Costing)
    </Button>
  </svelte:fragment>

  <div class="offers-container">
    <!-- Warning: Offers with no line items -->
    {#if actionableOffersWithNoItems.length > 0}
      <div class="no-items-warning">
        <span class="warning-icon">&#9888;</span>
        <div class="warning-content">
          <strong>{actionableOffersWithNoItems.length} active operational offer{actionableOffersWithNoItems.length === 1 ? '' : 's'} still {actionableOffersWithNoItems.length === 1 ? 'has' : 'have'} no line items.</strong>
          These records still need commercial line items before PDF or invoice workflows are reliable.
          <div class="warning-list">
            {#each actionableOffersWithNoItems as item}
              <span class="warning-offer-number">{escapeHtml(item.offer_number || item.id)}</span>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    {#if legacyOfferShells.length > 0}
      <div class="legacy-shell-note">
        <span class="warning-icon">&#9432;</span>
        <div class="warning-content">
          <strong>{legacyOfferShells.length} legacy quoted/RFQ shell{legacyOfferShells.length === 1 ? '' : 's'} {legacyOfferShells.length === 1 ? 'is' : 'are'} hidden from the default offer list.</strong>
          Review these from the Deployment audit before release instead of treating them as live commercial records.
        </div>
      </div>
    {/if}

    <!-- Stage Filter Tabs -->
    <Card padding="sm">
      <div class="company-toggle" role="tablist" aria-label="Filter offers by company">
        {#each ['PH Trading', 'AHS Trading'] as company}
          <button
            class="company-toggle-btn"
            class:active={selectedCompany === company}
            role="tab"
            aria-selected={selectedCompany === company}
            on:click={() => handleCompanySelect(company)}
          >
            {company}
          </button>
        {/each}
      </div>
      <div class="stage-tabs" role="tablist" aria-label="Filter offers by stage">
        {#each stageTabs as tab}
          <button
            class="stage-tab"
            class:active={selectedStage === tab.value}
            role="tab"
            aria-selected={selectedStage === tab.value}
            on:click={() => selectedStage = tab.value}
          >
            {tab.label}
            <span class="tab-count">{tab.count}</span>
          </button>
        {/each}
      </div>
    </Card>

    <!-- Offers DataTable -->
    <Card padding="sm">
      <DataTable
        {columns}
        data={filteredOffers}
        {loading}
        emptyMessage="No offers found"
        onRowClick={(row) => {}}
        on:rowClick={handleRowClick}
        stickyHeader={true}
        maxHeight="calc(100vh - 300px)"
        showBorder={false}
      />
    </Card>

    <!-- Summary Stats -->
    <div class="stats-grid">
      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Total Offers</div>
          <div class="stat-value">{companyScopedOffers.length}</div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Total Value</div>
          <div class="stat-value">
            {companyScopedOffers.reduce((sum, o) => sum + o.total_value_bhd, 0).toFixed(3)} BHD
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Expiring Soon</div>
          <div class="stat-value stat-warning">
            {companyScopedOffers.filter(o => o.is_expiring_soon).length}
          </div>
        </div>
      </Card>

      <Card padding="md">
        <div class="stat">
          <div class="stat-label">Win Rate</div>
          <div class="stat-value stat-success">
            {(() => {
              const won = companyScopedOffers.filter(o => o.stage === 'Won').length;
              const lost = companyScopedOffers.filter(o => o.stage === 'Lost').length;
              const closed = won + lost;
              return closed > 0 ? ((won / closed) * 100).toFixed(1) : '0.0';
            })()}%
          </div>
        </div>
      </Card>
    </div>
  </div>
</PageLayout>

<!-- Create Offer Modal -->
<WabiModal bind:open={showCreateModal} title="Create New Offer" size="lg">
  <form on:submit|preventDefault={handleCreateOffer} class="offer-form">
    <!-- Customer Selection -->
    <FormGroup label="Customer" required>
      <select
        class="select-input"
        bind:value={formData.customer_id}
        on:change={handleCustomerSelect}
        required
      >
        <option value="">Select customer...</option>
        {#each customers as customer}
          <option value={customer.id}>{customer.business_name}</option>
        {/each}
      </select>
    </FormGroup>

    <!-- Dates -->
    <div class="form-row">
      <FormGroup label="Quotation Date" required>
        <Input
          type="date"
          bind:value={formData.quotation_date}
          max={new Date().toISOString().split('T')[0]}
          required
        />
      </FormGroup>

      <FormGroup label="Validity Date" required>
        <Input
          type="date"
          bind:value={formData.validity_date}
          min={formData.quotation_date}
          required
        />
      </FormGroup>
    </div>

    <!-- Line Items -->
    <div class="line-items-section">
      <div class="section-header">
        <h4>Line Items</h4>
        <Button variant="secondary" size="sm" on:click={addLineItem}>
          + Add Item
        </Button>
      </div>

      {#if formData.items.length === 0}
        <div class="empty-items">
          No items added yet. Click "Add Item" to start.
        </div>
      {:else}
        <div class="items-list">
          {#each formData.items as item, index}
            <div class="item-row" transition:fade>
              <Input
                label="Description"
                bind:value={item.description}
                placeholder="Product or service description"
              />
              <Input
                label="Quantity"
                type="number"
                bind:value={item.quantity}
                min="1"
                step="1"
                required
              />
              <Input
                label="Unit Price (BHD)"
                type="number"
                bind:value={item.unit_price}
                min="0.001"
                step="0.001"
                required
              />
              <div class="item-total">
                <div class="label">Total</div>
                <div class="value">{(item.quantity * item.unit_price).toFixed(3)}</div>
              </div>
              <button
                type="button"
                class="btn-remove"
                on:click={() => removeLineItem(index)}
                aria-label="Remove item"
              >
                ×
              </button>
            </div>
          {/each}
        </div>

        <div class="total-section">
          <div class="total-label">Total Value:</div>
          <div class="total-value">{totalValue.toFixed(3)} BHD</div>
        </div>
      {/if}
    </div>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showCreateModal = false}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={formLoading}
      on:click={handleCreateOffer}
    >
      Create Offer
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Edit Offer Modal - Enhanced with full costing data -->
<WabiModal bind:open={showEditModal} title="Edit Offer" size="xl">
  <form on:submit|preventDefault={handleEditOffer} class="offer-form">
    <!-- Customer & Dates -->
    <FormGroup label="Customer" required>
      <input
        list="offer-customer-edit-list"
        class="input-field"
        bind:value={formData.customer_name}
        on:input={handleCustomerSelect}
        placeholder="Search customer..."
        required
      />
      <datalist id="offer-customer-edit-list">
        {#each customers as customer}
          <option value={customer.business_name}>{customer.business_name}</option>
        {/each}
      </datalist>
      {#if formData.customer_name && !formData.customer_id}
        <div class="date-hint date-hint-warning">
          Using the stored customer name. Select a matching customer record if you want to relink it.
        </div>
      {/if}
    </FormGroup>

    <FormGroup label="Project / Reference">
      <Input
        type="text"
        bind:value={formData.project_name}
        placeholder="Project name or reference"
      />
    </FormGroup>

    <div class="form-row">
      <FormGroup label="Quotation Date" required>
        <Input type="date" bind:value={formData.quotation_date} required />
      </FormGroup>
      <FormGroup label="Validity Date" required>
        <Input type="date" bind:value={formData.validity_date} min={formData.quotation_date} required />
        {#if formData.validity_date}
          {@const daysUntilExpiry = Math.ceil((new Date(formData.validity_date).getTime() - new Date().getTime()) / (1000 * 60 * 60 * 24))}
          {#if daysUntilExpiry < 0}
            <div class="date-hint date-hint-error">This date is in the past</div>
          {:else if daysUntilExpiry <= 7}
            <div class="date-hint date-hint-warning">Expires in {daysUntilExpiry} day{daysUntilExpiry !== 1 ? 's' : ''}</div>
          {:else}
            <div class="date-hint date-hint-success">{daysUntilExpiry} days validity</div>
          {/if}
        {/if}
      </FormGroup>
    </div>

    <!-- Line Items with Full Costing Data -->
    <div class="line-items-section">
      <div class="section-header">
        <h4>Line Items (Full Costing Details)</h4>
        <Button variant="secondary" size="sm" on:click={addLineItem}>+ Add Item</Button>
      </div>

      {#if formData.items.length === 0}
        <div class="empty-items">No items added yet.</div>
      {:else}
        <div class="items-list-detailed">
          {#each formData.items as item, index}
            <div class="item-card" transition:fade>
              <div class="item-card-header">
                <span class="item-number">Item #{index + 1}</span>
                <button type="button" class="btn-remove-card" on:click={() => removeLineItem(index)} aria-label="Remove">×</button>
              </div>

              <!-- Row 1: Equipment & Model -->
              <div class="form-row-3">
                <div class="form-field">
                  <span class="field-label">Equipment</span>
                  <input type="text" bind:value={item.equipment} placeholder="Product/Equipment name" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Model</span>
                  <input type="text" bind:value={item.model} placeholder="Model number" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Currency</span>
                  <select bind:value={item.currency} class="input-field">
                    <option value="BHD">BHD</option>
                    <option value="EUR">EUR</option>
                    <option value="USD">USD</option>
                    <option value="GBP">GBP</option>
                  </select>
                </div>
              </div>

              <!-- Row 2: Specification -->
              <div class="form-field full-width">
                <span class="field-label">Specification</span>
                <input type="text" bind:value={item.specification} placeholder="Technical specification" class="input-field" />
              </div>

              <!-- Row 3: Detailed Description -->
              <div class="form-field full-width">
                <span class="field-label">Detailed Description</span>
                <textarea bind:value={item.detailed_description} rows="2" placeholder="Extended specifications, codes, approvals..." class="input-field textarea"></textarea>
              </div>

              <!-- Row 4: Cost Fields -->
              <div class="form-row-4">
                <div class="form-field">
                  <span class="field-label">Quantity</span>
                  <input type="number" bind:value={item.quantity} min="1" step="1" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">FOB</span>
                  <input type="number" bind:value={item.fob} min="0" step="0.001" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Freight</span>
                  <input type="number" bind:value={item.freight} min="0" step="0.001" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Total Cost</span>
                  <input type="number" bind:value={item.total_cost} min="0" step="0.001" class="input-field" readonly />
                </div>
              </div>

              <!-- Row 5: Pricing -->
              <div class="form-row-4">
                <div class="form-field">
                  <span class="field-label">Margin %</span>
                  <input type="number" bind:value={item.margin_percent} min="0" max="99" step="1" class="input-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Unit Price (BHD)</span>
                  <input type="number" bind:value={item.unit_price} min="0" step="0.001" class="input-field highlight-field" />
                </div>
                <div class="form-field">
                  <span class="field-label">Line Total</span>
                  <div class="calculated-value">{(item.quantity * item.unit_price).toFixed(3)} BHD</div>
                </div>
                <div></div>
              </div>
            </div>
          {/each}
        </div>

        <div class="total-section">
          <div class="total-label">Total Value:</div>
          <div class="total-value">{totalValue.toFixed(3)} BHD</div>
        </div>
      {/if}
    </div>
  </form>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => showEditModal = false}>Cancel</Button>
    <Button variant="primary" loading={formLoading} on:click={handleEditOffer}>Save Changes</Button>
  </svelte:fragment>
</WabiModal>

<!-- Mark as Won Modal -->
<WabiModal bind:open={showWonModal} title="Mark Offer as Won" size="sm">
  <div class="won-form">
    <p class="modal-desc">
      This will create an order in the operations pipeline from offer
      <strong>{actionOffer?.offer_number}</strong> for
      <strong>{actionOffer?.customer_name}</strong>.
    </p>

    <FormGroup label="Customer PO Number">
      <Input
        bind:value={customerPONumber}
        placeholder="Enter customer's PO reference"
      />
    </FormGroup>
    <div class="modal-value">
      <span class="label">Order Value:</span>
      <span class="value">{(actionOffer?.total_value_bhd || 0).toFixed(3)} BHD</span>
    </div>
  </div>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => { showWonModal = false; actionOffer = null; }}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={actionLoading}
      on:click={handleMarkWon}
    >
      Confirm Won
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Mark as Lost Modal -->
<WabiModal bind:open={showLostModal} title="Mark Offer as Lost" size="sm">
  <div class="lost-form">
    <p class="modal-desc">
      Mark offer <strong>{actionOffer?.offer_number}</strong> for
      <strong>{actionOffer?.customer_name}</strong> as lost.
    </p>
    <FormGroup label="Reason for Loss" required>
      <select class="select-input" bind:value={lostReason}>
        <option value="">Select reason...</option>
        <option value="Successfully Closed">Successfully Closed</option>
        <option value="Price Too High">Price Too High</option>
        <option value="Competitor Offered Better Price">Competitor Offered Better Price</option>
        <option value="Competitor Offered Better Product">Competitor Offered Better Product</option>
        <option value="Customer's Budget Constraints">Customer's Budget Constraints</option>
        <option value="Customer Went With Existing Supplier">Customer Went With Existing Supplier</option>
        <option value="Product Out of Stock">Product Out of Stock</option>
        <option value="Delivery Time Too Long">Delivery Time Too Long</option>
        <option value="Product Not Suitable">Product Not Suitable</option>
        <option value="Insufficient Product Knowledge">Insufficient Product Knowledge</option>
        <option value="Lack of Customer Trust">Lack of Customer Trust</option>
        <option value="Customer's Needs Changed">Customer's Needs Changed</option>
        <option value="Poor Customer Service">Poor Customer Service</option>
        <option value="Technical Issues">Technical Issues</option>
        <option value="Contractual Disputes">Contractual Disputes</option>
        <option value="Logistical Issues">Logistical Issues</option>
        <option value="Incorrect Target Audience">Incorrect Target Audience</option>
        <option value="Lack of Follow-Up">Lack of Follow-Up</option>
        <option value="Unfavorable Payment Terms">Unfavorable Payment Terms</option>
        <option value="Customer Decision-Making Delays">Customer Decision-Making Delays</option>
        <option value="Lost To Other OEM Channel">Lost To Other OEM Channel</option>
        <option value="Legal or Regulatory Issues">Legal or Regulatory Issues</option>
      </select>
    </FormGroup>
    <div class="modal-value lost">
      <span class="label">Lost Value:</span>
      <span class="value">{(actionOffer?.total_value_bhd || 0).toFixed(3)} BHD</span>
    </div>
  </div>

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => { showLostModal = false; actionOffer = null; }}>
      Cancel
    </Button>
    <Button
      variant="primary"
      loading={actionLoading}
      on:click={handleMarkLost}
    >
      Confirm Lost
    </Button>
  </svelte:fragment>
</WabiModal>

<!-- Follow-Up Modal -->
{#if showFollowUpModal}
  <div
    class="followup-overlay"
    role="button"
    tabindex="0"
    on:click|self={() => showFollowUpModal = false}
    on:keydown={(event) => (event.key === "Enter" || event.key === " ") && (showFollowUpModal = false)}
  >
    <div class="followup-modal" in:fade={{ duration: 150 }}>
      <h3>Schedule Follow-Up</h3>
      <p class="followup-offer">Offer: {followUpOffer?.offer_number || followUpOffer?.id?.slice(0,8)}</p>
      <div class="followup-form">
        <div class="form-group">
          <label for="offer-followup-date">Follow-Up Date</label>
          <input id="offer-followup-date" type="date" bind:value={followUpDate} min={new Date().toISOString().split('T')[0]} />
        </div>
        <div class="form-group">
          <label for="offer-followup-notes">Notes</label>
          <textarea id="offer-followup-notes" bind:value={followUpNotes} placeholder="What to follow up on..." rows="3"></textarea>
        </div>
        <div class="followup-actions">
          <button class="btn-cancel" on:click={() => showFollowUpModal = false}>Cancel</button>
          <button class="btn-confirm" on:click={scheduleFollowUp} disabled={!followUpDate || followUpLoading}>
            {followUpLoading ? 'Processing...' : 'Schedule'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Pending Follow-Ups Panel -->
{#if pendingFollowUps.length > 0}
  <div class="pending-followups">
    <h4>Pending Follow-Ups ({pendingFollowUps.length})</h4>
    {#each pendingFollowUps.slice(0, 5) as fu}
      <div class="followup-item">
        <span class="followup-date">{new Date(fu.follow_up_date).toLocaleDateString()}</span>
        <span class="followup-notes">{fu.notes || 'No notes'}</span>
        <button class="followup-complete" on:click={() => completeFollowUp(fu.id)} disabled={followUpLoading}>
          {followUpLoading ? 'Processing...' : 'Done'}
        </button>
      </div>
    {/each}
  </div>
{/if}

<!-- View Full Offer Details Modal (Fix #5 - Show ALL costing data) -->
<WabiModal bind:open={showViewModal} title="Offer Details" size="xl">
  {#if viewOffer}
    <div class="view-offer-details">
      <!-- Header Info - Editable Fields -->
      <div class="view-header">
        <div class="view-header-main">
          <div class="editable-offer-number">
            <span class="meta-label">Offer #</span>
            <input type="text" bind:value={viewOffer.offer_number} class="input-inline input-inline-lg" placeholder="Offer number" />
          </div>
          <StatusBadge status={viewOffer.stage || 'Quoted'} />
        </div>
        <div class="view-header-meta">
          <!-- Row 1: Core Info (read-only) -->
          <div class="meta-item">
            <span class="meta-label">Customer</span>
            <span class="meta-value">{viewOffer.customer_name}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Quotation Date</span>
            <span class="meta-value">{viewOffer.quotation_date ? new Date(viewOffer.quotation_date).toLocaleDateString() : 'N/A'}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Valid Until</span>
            <span class="meta-value" class:expired={viewOffer.is_expired} class:expiring={viewOffer.is_expiring_soon}>
              {viewOffer.validity_date ? new Date(viewOffer.validity_date).toLocaleDateString() : 'N/A'}
              {#if viewOffer.is_expired}(Expired){:else if viewOffer.is_expiring_soon}(Expiring Soon){/if}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Total Value</span>
            <span class="meta-value meta-value-highlight">{(viewOffer.total_value_bhd || 0).toFixed(3)} BHD</span>
          </div>
          <!-- Row 2: Editable Reference & Terms -->
          <div class="meta-item">
            <span class="meta-label">Folder Number</span>
            <input type="text" bind:value={viewOffer.folder_number} class="input-inline" placeholder="Folder #" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Customer Reference</span>
            <input type="text" bind:value={viewOffer.customer_reference} class="input-inline" placeholder="RFQ / enquiry ref" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Payment Terms</span>
            <input type="text" bind:value={viewOffer.payment_terms} class="input-inline" placeholder="e.g. Net 30" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Delivery Terms</span>
            <input type="text" bind:value={viewOffer.delivery_terms} class="input-inline" placeholder="e.g. DAP Bahrain" />
          </div>
          <!-- Row 3: Attention Details (editable) -->
          <div class="meta-item">
            <span class="meta-label">Attention Person</span>
            <input type="text" bind:value={viewOffer.attention_person} class="input-inline" placeholder="Contact name" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Company</span>
            <input type="text" bind:value={viewOffer.attention_company} class="input-inline" placeholder="Company name" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Attention Phone</span>
            <input type="text" bind:value={viewOffer.attention_phone} class="input-inline" placeholder="Phone" />
          </div>
          <div class="meta-item">
            <span class="meta-label">Issued By</span>
            <input type="text" bind:value={viewOffer.issued_by} class="input-inline" placeholder="Name" />
          </div>
        </div>
      </div>

      <!-- Line Items Table (Full Costing Data) -->
      <div class="view-items-section">
        <h3>Line Items</h3>
        {#if viewOffer.items && viewOffer.items.length > 0}
          <div class="items-table-wrapper">
            <table class="items-table">
              <thead>
                <tr>
                  <th class="col-no">#</th>
                  <th class="col-equipment">Equipment / Model</th>
                  <th class="col-spec">Specification</th>
                  <th class="col-qty">Qty</th>
                  <th class="col-currency">Currency</th>
                  <th class="col-cost">FOB</th>
                  <th class="col-cost">Freight</th>
                  <th class="col-cost">Total Cost</th>
                  <th class="col-margin">Margin %</th>
                  <th class="col-price">Unit Price</th>
                  <th class="col-price">Total</th>
                </tr>
              </thead>
              <tbody>
                {#each viewOffer.items as item, idx}
                  <tr>
                    <td class="col-no">{idx + 1}</td>
                    <td class="col-equipment">
                      <div class="equipment-cell">
                        <span class="equipment-name">{item.equipment || item.description || '-'}</span>
                        {#if item.model || item.product_code}
                          <span class="equipment-model">{item.model || item.product_code}</span>
                        {/if}
                      </div>
                    </td>
                    <td class="col-spec">
                      <div class="spec-cell">
                        {item.specification || '-'}
                        {#if item.detailed_description}
                          <div class="detailed-desc">{item.detailed_description}</div>
                        {/if}
                      </div>
                    </td>
                    <td class="col-qty">{item.quantity || 0}</td>
                    <td class="col-currency">{item.currency || 'BHD'}</td>
                    <td class="col-cost">{(item.fob || 0).toFixed(3)}</td>
                    <td class="col-cost">{(item.freight || 0).toFixed(3)}</td>
                    <td class="col-cost">{(item.total_cost || 0).toFixed(3)}</td>
                    <td class="col-margin">{(item.margin_percent || 0).toFixed(1)}%</td>
                    <td class="col-price">{(item.unit_price_bhd || item.unit_price || 0).toFixed(3)}</td>
                    <td class="col-price total-col">{(item.total_price || ((item.quantity || 0) * (item.unit_price_bhd || item.unit_price || 0))).toFixed(3)}</td>
                  </tr>
                {/each}
              </tbody>
              <tfoot>
                <tr class="totals-row">
                  <td colspan="7"></td>
                  <td class="col-cost subtotal">{viewOffer.items.reduce((s, i) => s + (i.total_cost || 0), 0).toFixed(3)}</td>
                  <td></td>
                  <td></td>
                  <td class="col-price total-col grand-total">{(viewOffer.total_value_bhd || 0).toFixed(3)} BHD</td>
                </tr>
              </tfoot>
            </table>
          </div>
        {:else}
          <div class="no-items">No line items available</div>
        {/if}
      </div>

      <!-- Notes Section -->
      <div class="notes-section">
        <h3>Notes & Comments</h3>
        <div class="note-add">
          <textarea
            bind:value={newNoteContent}
            placeholder="Add a note about this opportunity..."
            rows="2"
            class="note-textarea"
          ></textarea>
          <button class="btn-add-note" on:click={handleAddNote} disabled={!newNoteContent.trim() || notesLoading}>
            {notesLoading ? 'Adding...' : 'Add Note'}
          </button>
        </div>
        {#if offerNotes.length > 0}
          <div class="notes-list">
            {#each offerNotes as note}
              <div class="note-item">
                <div class="note-header">
                  <span class="note-date">{new Date(note.note_date || note.created_at).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                  <span class="note-author">{note.created_by || 'System'}</span>
                  <button class="note-delete" on:click={() => handleDeleteNote(note.id)} title="Delete note" disabled={notesLoading}>&times;</button>
                </div>
                <div class="note-content">{note.content}</div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="notes-empty">No notes yet. Add one above to track this opportunity.</div>
        {/if}
      </div>
    </div>
  {/if}

  <svelte:fragment slot="footer">
    <Button variant="ghost" on:click={() => { showViewModal = false; viewOffer = null; }}>
      Close
    </Button>
    <Button variant="secondary" on:click={() => viewOffer && handleDownloadPDF(viewOffer)}>
      Download PDF
    </Button>
    <Button variant="primary" loading={viewSaving} on:click={handleSaveViewOffer}>
      Save Changes
    </Button>
  </svelte:fragment>
</WabiModal>

<style>
  .offers-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .company-toggle {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px;
    margin-bottom: 12px;
    background: var(--surface-muted, #f3f4f6);
    border-radius: 12px;
    width: fit-content;
  }

  .company-toggle-btn {
    border: none;
    background: transparent;
    color: var(--text-secondary);
    padding: 8px 14px;
    border-radius: 10px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .company-toggle-btn:hover {
    color: var(--text-primary);
    background: rgba(255, 255, 255, 0.5);
  }

  .company-toggle-btn.active {
    background: var(--carbon, #1f2937);
    color: white;
    box-shadow: 0 8px 20px rgba(15, 23, 42, 0.12);
  }

  /* Stage Tabs */
  .stage-tabs {
    display: flex;
    gap: 8px;
    overflow-x: auto;
  }

  .stage-tab {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: transparent;
    border: none;
    border-radius: var(--border-radius-sm);
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--transition-fast);
    white-space: nowrap;
  }

  .stage-tab:hover {
    background: var(--interactive-hover);
    color: var(--text-primary);
  }

  .stage-tab.active {
    background: var(--brand-indigo);
    color: white;
  }

  .tab-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 24px;
    height: 20px;
    padding: 0 6px;
    background: rgba(0, 0, 0, 0.1);
    border-radius: 10px;
    font-size: 12px;
    font-weight: 600;
  }

  .stage-tab.active .tab-count {
    background: rgba(255, 255, 255, 0.2);
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
  }

  .stat {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .stat-label {
    font-size: var(--label-size);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .stat-value {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .stat-warning {
    color: #F59E0B;
  }

  .stat-success {
    color: #10B981;
  }

  /* Action Buttons in Table */
  :global(.action-btn) {
    padding: 4px 10px;
    font-size: 12px;
    font-weight: 500;
    border: none;
    border-radius: var(--border-radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  :global(.action-btn-view) {
    background: var(--surface-elevated);
    color: var(--text-primary);
  }

  :global(.action-btn-view:hover) {
    background: var(--interactive-hover);
  }

  :global(.action-btn-edit) {
    background: var(--brand-indigo-tint);
    color: var(--brand-indigo);
  }

  :global(.action-btn-edit:hover) {
    background: var(--brand-indigo);
    color: white;
  }

  :global(.action-btn-won) {
    background: rgba(16, 185, 129, 0.1);
    color: #10B981;
  }

  :global(.action-btn-won:hover) {
    background: #10B981;
    color: white;
  }

  :global(.action-btn-lost) {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  :global(.action-btn-lost:hover) {
    background: #DC2626;
    color: white;
  }

  :global(.action-btn-requote) {
    background: rgba(99, 102, 241, 0.1);
    color: var(--brand-indigo);
  }

  :global(.action-btn-requote:hover) {
    background: var(--brand-indigo);
    color: white;
  }

  :global(.action-btn-followup) {
    background: rgba(0, 0, 0, 0.05);
    color: var(--onyx, #1D1D1F);
  }

  :global(.action-btn-followup:hover) {
    background: var(--onyx, #1D1D1F);
    color: white;
  }

  /* Follow-Up Modal */
  .followup-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .followup-modal {
    background: var(--surface, #fff);
    border-radius: 12px;
    padding: 28px;
    width: 400px;
    max-width: 90vw;
    box-shadow: 0 20px 60px rgba(0,0,0,0.15);
  }

  .followup-modal h3 {
    margin: 0 0 4px;
    font-size: 18px;
    font-weight: 600;
  }

  .followup-offer {
    margin: 0 0 20px;
    font-size: 13px;
    color: var(--steel, #86868B);
  }

  .followup-form .form-group {
    margin-bottom: 16px;
  }

  .followup-form label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    color: var(--steel, #86868B);
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .followup-form input,
  .followup-form textarea {
    width: 100%;
    padding: 10px 12px;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    font-size: 14px;
  }

  .followup-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 20px;
  }

  .btn-cancel {
    padding: 8px 16px;
    background: transparent;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
  }

  .btn-confirm {
    padding: 8px 16px;
    background: var(--carbon, #000);
    color: #fff;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
  }

  .btn-confirm:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  /* Pending Follow-Ups Panel */
  .pending-followups {
    margin-top: 24px;
    padding: 16px;
    background: var(--surface, #fff);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
  }

  .pending-followups h4 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
  }

  .followup-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border, #E5E5E5);
    font-size: 13px;
  }

  .followup-item:last-child {
    border-bottom: none;
  }

  .followup-date {
    font-weight: 500;
    min-width: 90px;
    font-variant-numeric: tabular-nums;
  }

  .followup-notes {
    flex: 1;
    color: var(--steel, #86868B);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .followup-complete {
    padding: 4px 10px;
    background: var(--ether, #F5F5F7);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 4px;
    font-size: 12px;
    cursor: pointer;
  }

  .followup-complete:hover {
    background: var(--carbon, #000);
    color: #fff;
  }

  /* Won/Lost Modal Styles */
  .won-form, .lost-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .modal-desc {
    margin: 0;
    font-size: 14px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .modal-value {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    background: rgba(16, 185, 129, 0.05);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: var(--border-radius-sm);
  }

  .modal-value.lost {
    background: rgba(220, 38, 38, 0.05);
    border-color: rgba(220, 38, 38, 0.2);
  }

  .modal-value .label {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .modal-value .value {
    font-size: 18px;
    font-weight: 700;
    color: var(--text-primary);
  }

  /* Form Styles */
  .offer-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .select-input {
    width: 100%;
    padding: 8px 12px;
    font-size: 14px;
    font-family: var(--font-family);
    color: var(--text-primary);
    background: var(--surface);
    border: var(--border-width) solid var(--border);
    border-radius: var(--border-radius-sm);
    transition: all var(--transition-fast);
  }

  .select-input:focus {
    outline: none;
    border-color: var(--brand-indigo);
    box-shadow: 0 0 0 3px var(--brand-indigo-tint);
  }

  /* Line Items Section */
  .line-items-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: var(--surface-elevated);
    border-radius: var(--border-radius);
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .section-header h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .empty-items {
    padding: 24px;
    text-align: center;
    color: var(--text-muted);
    font-style: italic;
    font-size: 14px;
  }

  .items-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .item-row {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr 1fr 32px;
    gap: 12px;
    align-items: end;
    padding: 12px;
    background: var(--surface);
    border-radius: var(--border-radius-sm);
    border: 1px solid var(--border);
  }

  .item-total {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .item-total .label {
    font-size: var(--label-size);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .item-total .value {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .btn-remove {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 50%;
    font-size: 24px;
    color: var(--text-muted);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .btn-remove:hover {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  .total-section {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 16px;
    padding: 12px 16px;
    background: var(--surface);
    border-radius: var(--border-radius-sm);
    border: 2px solid var(--brand-indigo);
  }

  .total-label {
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .total-value {
    font-size: 20px;
    font-weight: 700;
    color: var(--brand-indigo);
  }

  /* View Offer Details Modal Styles */
  .view-offer-details {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .view-header {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border, #E5E5E5);
  }

  .view-header-main {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .editable-offer-number {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  /* Inline editable inputs for view modal */
  .input-inline {
    width: 100%;
    padding: 6px 8px;
    font-size: 13px;
    font-weight: 500;
    color: var(--onyx, #1D1D1F);
    background: var(--canvas, #fff);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 4px;
    transition: border-color 0.15s;
  }

  .input-inline:focus {
    outline: none;
    border-color: var(--brand-indigo, #4F46E5);
    box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.1);
  }

  .input-inline::placeholder {
    color: var(--steel, #86868B);
    font-weight: 400;
    font-style: italic;
  }

  .input-inline-lg {
    font-size: 18px;
    font-weight: 700;
    padding: 6px 10px;
  }

  .view-header-meta {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }

  .meta-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .meta-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--steel, #86868B);
  }

  .meta-value {
    font-size: 14px;
    font-weight: 500;
    color: var(--onyx, #1D1D1F);
  }

  .meta-value.expired {
    color: #DC2626;
  }

  .meta-value.expiring {
    color: #F59E0B;
  }

  .meta-value-highlight {
    font-size: 18px;
    font-weight: 700;
    color: var(--carbon, #000);
  }

  .view-items-section h3 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--steel, #86868B);
  }

  .items-table-wrapper {
    overflow-x: auto;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
  }

  .items-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .items-table th,
  .items-table td {
    padding: 10px 12px;
    text-align: left;
    border-bottom: 1px solid var(--border, #E5E5E5);
  }

  .items-table th {
    background: var(--ether, #F5F5F7);
    font-weight: 600;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--steel, #86868B);
  }

  .items-table tbody tr:hover {
    background: rgba(0, 0, 0, 0.02);
  }

  .items-table .col-no {
    width: 40px;
    text-align: center;
    color: var(--steel, #86868B);
  }

  .items-table .col-equipment {
    min-width: 180px;
  }

  .items-table .col-spec {
    min-width: 200px;
  }

  .items-table .col-qty,
  .items-table .col-currency {
    width: 60px;
    text-align: center;
  }

  .items-table .col-cost,
  .items-table .col-price,
  .items-table .col-margin {
    width: 90px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .equipment-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .equipment-name {
    font-weight: 500;
    color: var(--onyx, #1D1D1F);
  }

  .equipment-model {
    font-size: 11px;
    color: var(--steel, #86868B);
    font-family: 'JetBrains Mono', monospace;
  }

  .spec-cell {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .detailed-desc {
    font-size: 11px;
    color: var(--steel, #86868B);
    font-style: italic;
    margin-top: 4px;
    padding-top: 4px;
    border-top: 1px dashed var(--border, #E5E5E5);
    white-space: pre-wrap;
  }

  .total-col {
    font-weight: 600;
    color: var(--onyx, #1D1D1F);
  }

  .items-table tfoot {
    background: var(--ether, #F5F5F7);
  }

  .totals-row td {
    border-bottom: none;
  }

  .subtotal {
    font-weight: 600;
    color: var(--steel, #86868B);
  }

  .grand-total {
    font-size: 15px;
    font-weight: 700;
    color: var(--carbon, #000);
  }

  .no-items {
    padding: 32px;
    text-align: center;
    color: var(--steel, #86868B);
    font-style: italic;
    background: var(--ether, #F5F5F7);
    border-radius: 8px;
  }

  /* Notes Section */
  .notes-section {
    border-top: 1px solid var(--border, #E5E5E5);
    padding-top: 16px;
  }

  .notes-section h3 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--steel, #86868B);
  }

  .note-add {
    display: flex;
    gap: 8px;
    align-items: flex-end;
    margin-bottom: 16px;
  }

  .note-textarea {
    flex: 1;
    padding: 10px 12px;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    font-size: 13px;
    font-family: inherit;
    resize: vertical;
    min-height: 44px;
    color: var(--onyx, #1D1D1F);
    background: var(--canvas, #fff);
  }

  .note-textarea:focus {
    outline: none;
    border-color: var(--brand-indigo, #4F46E5);
    box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.1);
  }

  .btn-add-note {
    padding: 10px 16px;
    background: var(--carbon, #000);
    color: #fff;
    border: none;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
    transition: opacity 0.15s;
  }

  .btn-add-note:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .btn-add-note:hover:not(:disabled) {
    opacity: 0.85;
  }

  .notes-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .note-item {
    padding: 10px 12px;
    background: var(--ether, #F5F5F7);
    border-radius: 8px;
    border: 1px solid var(--border, #E5E5E5);
  }

  .note-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
  }

  .note-date {
    font-size: 11px;
    font-weight: 500;
    color: var(--steel, #86868B);
    font-variant-numeric: tabular-nums;
  }

  .note-author {
    font-size: 11px;
    color: var(--brand-indigo, #4F46E5);
    font-weight: 500;
  }

  .note-delete {
    margin-left: auto;
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    font-size: 16px;
    color: var(--steel, #86868B);
    cursor: pointer;
    transition: all 0.15s;
  }

  .note-delete:hover:not(:disabled) {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  .note-delete:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .note-content {
    font-size: 13px;
    line-height: 1.5;
    color: var(--onyx, #1D1D1F);
    white-space: pre-wrap;
    word-break: break-word;
  }

  .notes-empty {
    padding: 16px;
    text-align: center;
    color: var(--steel, #86868B);
    font-size: 13px;
    font-style: italic;
  }

  /* Date hint styles */
  .date-hint {
    font-size: 11px;
    margin-top: 4px;
    padding: 4px 8px;
    border-radius: 4px;
    display: inline-block;
  }

  .date-hint-error {
    color: #DC2626;
    background: rgba(220, 38, 38, 0.1);
  }

  .date-hint-warning {
    color: #F59E0B;
    background: rgba(245, 158, 11, 0.1);
  }

  .date-hint-success {
    color: #10B981;
    background: rgba(16, 185, 129, 0.1);
  }

  /* Responsive */
  @media (max-width: 768px) {
    .form-row {
      grid-template-columns: 1fr;
    }

    .item-row {
      grid-template-columns: 1fr;
      gap: 8px;
    }

    .stats-grid {
      grid-template-columns: 1fr;
    }

    .view-header-meta {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  /* Enhanced Edit Form Styles */
  .items-list-detailed {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .item-card {
    background: var(--surface, #fff);
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 8px;
    padding: 16px;
  }

  .item-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border, #E5E5E5);
  }

  .item-number {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--steel, #86868B);
  }

  .btn-remove-card {
    width: 24px;
    height: 24px;
    border: none;
    background: transparent;
    color: var(--steel, #86868B);
    font-size: 18px;
    cursor: pointer;
    border-radius: 4px;
    transition: all 0.15s;
  }

  .btn-remove-card:hover {
    background: rgba(220, 38, 38, 0.1);
    color: #DC2626;
  }

  .form-row-3 {
    display: grid;
    grid-template-columns: 1fr 1fr 100px;
    gap: 12px;
    margin-bottom: 12px;
  }

  .form-row-4 {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
    margin-bottom: 12px;
  }

  .form-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-field.full-width {
    grid-column: 1 / -1;
  }

  .form-field .field-label {
    font-size: 11px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--steel, #86868B);
  }

  .input-field {
    padding: 8px 10px;
    font-size: 13px;
    border: 1px solid var(--border, #E5E5E5);
    border-radius: 6px;
    background: var(--canvas, #fff);
    color: var(--onyx, #1D1D1F);
    transition: border-color 0.15s;
  }

  .input-field:focus {
    outline: none;
    border-color: var(--carbon, #000);
  }

  .input-field.textarea {
    resize: vertical;
    min-height: 48px;
    font-family: inherit;
  }

  .input-field.highlight-field {
    background: rgba(79, 70, 229, 0.05);
    border-color: var(--brand-indigo, #4F46E5);
    font-weight: 600;
  }

  .calculated-value {
    padding: 8px 10px;
    font-size: 13px;
    font-weight: 600;
    background: var(--ether, #F5F5F7);
    border-radius: 6px;
    color: var(--onyx, #1D1D1F);
  }

  /* Warning banner for offers with no line items */
  .no-items-warning {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    background: #fef9e7;
    border: 1px solid #f0d050;
    border-radius: 8px;
    margin-bottom: 12px;
    font-size: 13px;
    color: #7a6200;
  }

  .no-items-warning .warning-icon {
    font-size: 18px;
    flex-shrink: 0;
    line-height: 1.4;
  }

  .no-items-warning .warning-content {
    flex: 1;
    line-height: 1.5;
  }

  .no-items-warning .warning-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 6px;
  }

  .no-items-warning .warning-offer-number {
    display: inline-block;
    padding: 2px 8px;
    background: rgba(240, 208, 80, 0.3);
    border-radius: 4px;
    font-weight: 600;
    font-size: 12px;
    cursor: default;
  }

  .legacy-shell-note {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 16px;
    background: rgba(21, 94, 117, 0.08);
    border: 1px solid rgba(21, 94, 117, 0.18);
    border-radius: 8px;
    margin-bottom: 12px;
    font-size: 13px;
    color: #155e75;
  }
</style>
