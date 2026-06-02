<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';
  import {
    GetDeliveryNotes,
    GetDeliveryNoteByID,
    CreateDeliveryNote,
    UpdateDeliveryNote,
    DeleteDeliveryNote,
    DispatchDeliveryNote,
    ConfirmDeliveryNote,
    GetDeliveryNotesByOrder,
    GetOrderFulfillmentDetail,
    GenerateDNNumber,
    GenerateDeliveryNotePDF,
    OpenExportedFile,
    ListOrders,
    ListCustomers,
    CreateDNWithSerials,
    GetAvailableSerials,
    GetOrder,
  } from '../../../wailsjs/go/main/App';
  import { toast } from '$lib/stores/toasts';
  import { pendingDNCreate } from '$lib/stores/navigation';
  import { main } from '../../../wailsjs/go/models';
  import { escapeHtml } from '$lib/utils/escapeHtml';

  // Design system components
  import PageLayout from '$lib/components/layout/PageLayout.svelte';
  import DataTable from '$lib/components/ui/DataTable.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
  import Modal from '$lib/components/layout/Modal.svelte';
  import FormGroup from '$lib/components/ui/FormGroup.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import WabiSpinner from '$lib/components/ui/WabiSpinner.svelte';

  // Props
  export let embedded = false;

  // Status definitions
  const DELIVERY_STATUSES = [
    { key: 'Draft', label: 'Draft', color: 'gray' },
    { key: 'Prepared', label: 'Prepared', color: 'blue' },
    { key: 'Dispatched', label: 'Dispatched', color: 'indigo' },
    { key: 'InTransit', label: 'In Transit', color: 'purple' },
    { key: 'Delivered', label: 'Delivered', color: 'green' },
    { key: 'Signed', label: 'Signed', color: 'emerald' }
  ];

  const TRANSPORT_METHODS = [
    'Own Vehicle',
    'Courier',
    'Customer Pickup',
    'Third-Party Logistics'
  ];

  // State - using 'any' for flexibility with backend responses
  let deliveryNotes: any[] = [];
  let filteredDeliveryNotes: any[] = [];
  let loading = true;
  let activeFilter = 'All';
  let searchQuery = '';
  let availableYears: string[] = [];

  // Create/Edit modal
  let showModal = false;
  let editingNote: any = null;
  let modalMode: 'create' | 'edit' = 'create';
  let saving = false;
  let pdfGenerating = false;

  // Form state
  let formData = {
    dnNumber: '',
    orderId: '',
    orderNumber: '',
    customerId: '',
    customerName: '',
    deliveryDate: new Date().toISOString().split('T')[0],
    deliveryAddress: '',
    contactPerson: '',
    contactPhone: '',
    driverName: '',
    vehicleNumber: '',
    transportMethod: 'Own Vehicle',
    status: 'Draft',
    isPartialDelivery: false,
    deliverySequence: 1,
    totalDeliveries: 1
  };

  // Helper data
  let customers: any[] = [];
  let orders: main.Order[] = [];

  // Order fulfillment items for DN creation (E1 enhancement)
  let fulfillmentItems: any[] = []; // Items with ship_qty for the create modal
  let loadingFulfillment = false;
  let fulfillmentLoaded = false;
  let fulfillmentLoadFailed = false;

  // Detail modal
  let showDetailModal = false;
  let selectedNote: any = null;
  let noteItems: any[] = [];

  function errorMessage(err: any): string {
    return err?.message || err?.Message || err?.error || String(err || 'Unknown error');
  }

  function getDeliveryDateValue(value: any): Date | null {
    if (!value) return null;
    if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value;
    if (typeof value === 'string' || typeof value === 'number') {
      const parsed = new Date(value);
      return Number.isNaN(parsed.getTime()) ? null : parsed;
    }
    if (typeof value === 'object' && typeof value.toString === 'function') {
      const parsed = new Date(value.toString());
      return Number.isNaN(parsed.getTime()) ? null : parsed;
    }
    return null;
  }

  function deliveryNoteSortTime(note: any): number {
    return getDeliveryDateValue(note?.delivery_date)?.getTime()
      || getDeliveryDateValue(note?.created_at)?.getTime()
      || 0;
  }

  function sortDeliveryNotesLatestFirst(notes: any[]): any[] {
    return [...notes].sort((a, b) => {
      const dateDelta = deliveryNoteSortTime(b) - deliveryNoteSortTime(a);
      if (dateDelta !== 0) return dateDelta;
      return String(b?.dn_number || '').localeCompare(String(a?.dn_number || ''), undefined, {
        numeric: true,
        sensitivity: 'base'
      });
    });
  }

  function meaningfulValue(...values: any[]): string {
    for (const value of values) {
      const text = String(value || '').trim();
      if (!text || text === '-' || text === '—') continue;
      if (['n/a', 'unknown', 'undefined', 'null'].includes(text.toLowerCase())) continue;
      return text;
    }
    return '';
  }

  function resolveOrderForNote(note: any) {
    const orderId = meaningfulValue(note?.order_id);
    const orderReference = meaningfulValue(note?.order_reference);
    return orders.find(order => order.id === orderId)
      || orders.find(order => order.order_number === orderReference)
      || null;
  }

  function resolveCustomerForNote(note: any, order: any) {
    const customerId = meaningfulValue(note?.customer_id, order?.customer_id);
    return customers.find(customer => customer.id === customerId) || null;
  }

  function enrichDeliveryNoteForDisplay(note: any, fallback: any = {}) {
    const merged = { ...fallback, ...note };
    const order = resolveOrderForNote(merged) || resolveOrderForNote(fallback);
    const customer = resolveCustomerForNote(merged, order) || resolveCustomerForNote(fallback, order);

    return {
      ...merged,
      order_reference: meaningfulValue(
        merged.order_reference,
        fallback.order_reference,
        order?.order_number,
        merged.order_id
      ) || 'N/A',
      customer_name: meaningfulValue(
        merged.customer_name,
        fallback.customer_name,
        customer?.business_name,
        order?.customer_name
      ) || 'Unknown'
    };
  }

  function asNumber(value: any): number {
    const parsed = Number(value || 0);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function orderItemProductCode(item: any): string {
    return meaningfulValue(item?.product_code, item?.model, item?.product_id) || '-';
  }

  function orderItemDescription(item: any): string {
    return meaningfulValue(item?.description, item?.equipment, item?.specification, item?.detailed_description) || 'Line item';
  }

  function orderItemToFulfillmentRow(item: any) {
    const orderedQty = asNumber(item?.quantity);
    const shippedQty = asNumber(item?.quantity_shipped);
    const remainingQty = Math.max(orderedQty - shippedQty, 0);
    return {
      order_item_id: item?.id || '',
      product_id: item?.product_id || '',
      product_code: orderItemProductCode(item),
      description: orderItemDescription(item),
      ordered_qty: orderedQty,
      shipped_qty: shippedQty,
      delivered_qty: shippedQty,
      remaining_qty: remainingQty,
      ship_qty: remainingQty,
      requires_serial: false,
      available_serials: [] as any[],
      selected_serials: [] as string[],
    };
  }

  function matchOrderItem(item: any, orderItems: any[]) {
    const orderItemId = meaningfulValue(item?.order_item_id);
    const productId = meaningfulValue(item?.product_id);
    const productCode = meaningfulValue(item?.product_code);
    return orderItems.find(orderItem => orderItem.id === orderItemId)
      || orderItems.find(orderItem => productId && orderItem.product_id === productId)
      || orderItems.find(orderItem => productCode && orderItemProductCode(orderItem) === productCode)
      || null;
  }

  async function loadOrderForNote(note: any) {
    const localOrder = resolveOrderForNote(note);
    if (localOrder?.items?.length) return localOrder;
    const orderId = meaningfulValue(note?.order_id);
    if (!orderId) return localOrder;
    try {
      return await GetOrder(orderId);
    } catch (err) {
      console.warn('Failed to load order for delivery note detail:', err);
      return localOrder;
    }
  }

  async function buildDetailItems(note: any, rawItems: any[] = []) {
    const order = await loadOrderForNote(note);
    const orderItems = order?.items || [];

    if (rawItems.length > 0) {
      return rawItems.map(item => {
        const matched = matchOrderItem(item, orderItems);
        const orderedQty = asNumber(item.quantity_ordered) || asNumber(matched?.quantity);
        const deliveredQty = asNumber(item.quantity_delivered) || orderedQty;
        return {
          ...item,
          product_code: meaningfulValue(item.product_code, matched ? orderItemProductCode(matched) : '') || '-',
          description: meaningfulValue(item.description, matched ? orderItemDescription(matched) : '') || 'Line item',
          quantity_ordered: orderedQty,
          quantity_delivered: deliveredQty,
          quantity_remaining: asNumber(item.quantity_remaining) || Math.max(orderedQty - deliveredQty, 0),
        };
      });
    }

    if (meaningfulValue(note?.order_id)) {
      try {
        const fulfillment = await GetOrderFulfillmentDetail(note.order_id);
        if (fulfillment?.items?.length) {
          return fulfillment.items.map((item: any) => ({
            product_code: meaningfulValue(item.product_code) || '-',
            description: meaningfulValue(item.description) || 'Line item',
            quantity_ordered: asNumber(item.ordered_qty),
            quantity_delivered: asNumber(item.delivered_qty),
            quantity_remaining: asNumber(item.remaining_qty),
          }));
        }
      } catch (err) {
        console.warn('Failed to load fulfillment items for delivery note detail:', err);
      }
    }

    return orderItems.map((item: any) => {
      const row = orderItemToFulfillmentRow(item);
      return {
        product_code: row.product_code,
        description: row.description,
        quantity_ordered: row.ordered_qty,
        quantity_delivered: row.delivered_qty,
        quantity_remaining: row.remaining_qty,
      };
    });
  }

  // DataTable columns
  const columns = [
    {
      key: 'dn_number',
      label: 'DN Number',
      sortable: true,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'order_reference',
      label: 'Order Reference',
      sortable: true,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'customer_name',
      label: 'Customer',
      sortable: true,
      type: 'text' as const,
      render: (row) => escapeHtml(row.customer_name)
    },
    {
      key: 'delivery_date',
      label: 'Delivery Date',
      sortable: true,
      type: 'date' as const,
      width: '130px'
    },
    {
      key: 'delivery_info',
      label: 'Delivery #',
      sortable: false,
      type: 'text' as const,
      width: '100px'
    },
    {
      key: 'transport_method',
      label: 'Transport',
      sortable: false,
      type: 'text' as const,
      width: '140px'
    },
    {
      key: 'status',
      label: 'Status',
      sortable: true,
      type: 'status' as const,
      width: '130px'
    }
  ];

  // Load data
  async function loadDeliveryNotes() {
    loading = true;
    try {
      const [notesData, ordersData, customersData] = await Promise.all([
        GetDeliveryNotes(),
        ListOrders(500, 0),  // Fixed: limit=500, offset=0
        ListCustomers(500, 0)
      ]);

      customers = customersData || [];
      orders = ordersData || [];
      deliveryNotes = sortDeliveryNotesLatestFirst((notesData || []).map(note => enrichDeliveryNoteForDisplay(note)));

      const yearsSet = new Set<string>();
      deliveryNotes.forEach(note => {
        const deliveryDate = getDeliveryDateValue(note.delivery_date);
        if (deliveryDate) yearsSet.add(deliveryDate.getFullYear().toString());
      });
      availableYears = Array.from(yearsSet).sort().reverse();

      applyFilters();
    } catch (err) {
      console.error('Failed to load delivery notes:', err);
      toast.danger('Failed to load delivery notes');
    } finally {
      loading = false;
    }
  }

  // Apply filters
  function applyFilters() {
    let result = [...deliveryNotes];

    // Status filter
    if (activeFilter !== 'All') {
      result = result.filter(dn => dn.status === activeFilter);
    }

    // Search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(dn =>
        dn.dn_number?.toLowerCase().includes(query) ||
        (dn as any).customer_name?.toLowerCase().includes(query) ||
        (dn as any).order_reference?.toLowerCase().includes(query) ||
        dn.driver_name?.toLowerCase().includes(query) ||
        dn.vehicle_number?.toLowerCase().includes(query)
      );
    }

    filteredDeliveryNotes = sortDeliveryNotesLatestFirst(result);
  }

  $: {
    searchQuery;
    activeFilter;
    applyFilters();
  }

  // Stats
  $: stats = {
    total: deliveryNotes.length,
    draft: deliveryNotes.filter(dn => dn.status === 'Draft').length,
    inTransit: deliveryNotes.filter(dn => dn.status === 'InTransit').length,
    delivered: deliveryNotes.filter(dn => dn.status === 'Delivered').length,
    partial: deliveryNotes.filter(dn => dn.is_partial_delivery).length
  };

  // Open create modal
  async function openCreateModal() {
    modalMode = 'create';
    editingNote = null;

    // Generate new DN number
    try {
      const dnNumber = await GenerateDNNumber();
      formData = {
        dnNumber,
        orderId: '',
        orderNumber: '',
        customerId: '',
        customerName: '',
        deliveryDate: new Date().toISOString().split('T')[0],
        deliveryAddress: '',
        contactPerson: '',
        contactPhone: '',
        driverName: '',
        vehicleNumber: '',
        transportMethod: 'Own Vehicle',
        status: 'Draft',
        isPartialDelivery: false,
        deliverySequence: 1,
        totalDeliveries: 1
      };
    } catch (err) {
      console.error('Failed to generate DN number:', err);
      toast.warning('Could not generate DN number');
    }

    showModal = true;
  }

  // Open edit modal
  function openEditModal(note: main.DeliveryNote) {
    modalMode = 'edit';
    const noteForForm = enrichDeliveryNoteForDisplay(note);
    editingNote = noteForForm;

    // Find associated order
    const order = resolveOrderForNote(noteForForm);

    formData = {
      dnNumber: noteForForm.dn_number || '',
      orderId: noteForForm.order_id || '',
      orderNumber: order?.order_number || '',
      customerId: noteForForm.customer_id || '',
      customerName: noteForForm.customer_name || '',
      deliveryDate: noteForForm.delivery_date ? new Date(noteForForm.delivery_date as any).toISOString().split('T')[0] : '',
      deliveryAddress: noteForForm.delivery_address || '',
      contactPerson: noteForForm.contact_person || '',
      contactPhone: noteForForm.contact_phone || '',
      driverName: noteForForm.driver_name || '',
      vehicleNumber: noteForForm.vehicle_number || '',
      transportMethod: noteForForm.transport_method || 'Own Vehicle',
      status: noteForForm.status || 'Draft',
      isPartialDelivery: noteForForm.is_partial_delivery || false,
      deliverySequence: noteForForm.delivery_sequence || 1,
      totalDeliveries: noteForForm.total_deliveries || 1
    };
    showModal = true;
  }

  // Handle form submit - E1 enhanced with per-item quantities
  async function handleSubmit() {
    // Validation
    if (!formData.dnNumber.trim()) {
      toast.warning('DN Number is required');
      return;
    }

    if (!formData.orderId.trim()) {
      toast.warning('Order is required');
      return;
    }

    if (!formData.deliveryAddress.trim()) {
      toast.warning('Delivery Address is required');
      return;
    }

    saving = true;
    try {
      const baseNoteData: any = {
        id: editingNote?.id || '',
        dn_number: formData.dnNumber,
        order_id: formData.orderId,
        customer_id: formData.customerId,
        delivery_date: new Date(`${formData.deliveryDate}T00:00:00`),
        delivery_address: formData.deliveryAddress,
        contact_person: formData.contactPerson,
        contact_phone: formData.contactPhone,
        driver_name: formData.driverName,
        vehicle_number: formData.vehicleNumber,
        transport_method: formData.transportMethod,
        status: formData.status,
        is_partial_delivery: formData.isPartialDelivery,
        delivery_sequence: formData.deliverySequence,
        total_deliveries: formData.totalDeliveries,
        created_at: editingNote?.created_at || new Date(),
        updated_at: new Date(),
        version: editingNote?.version || 0,
        created_by: editingNote?.created_by || '',
        signed_by: editingNote?.signed_by || '',
        signature_image: editingNote?.signature_image || ''
      };

      if (modalMode === 'create' && fulfillmentItems.length > 0) {
        // Preserve the full DN header while still creating per-item ship quantities.
        const itemsToShip = fulfillmentItems
          .filter((item: any) => item.ship_qty > 0)
          .map((item: any) => ({
            order_item_id: item.order_item_id,
            ship_qty: item.ship_qty
          }));

        if (itemsToShip.length === 0) {
          toast.warning('At least one item must have a ship quantity > 0');
          saving = false;
          return;
        }

        // Validate quantities
        for (const item of fulfillmentItems) {
          if (item.ship_qty > item.remaining_qty + 0.001) {
            toast.warning(`Ship quantity for ${item.product_code} exceeds remaining (${item.remaining_qty})`);
            saving = false;
            return;
          }
        }

        // Phase 23: Check if any items have serial allocations
        const hasSerials = fulfillmentItems.some(
          (item: any) => item.requires_serial && item.selected_serials?.length > 0
        );

        if (hasSerials) {
          // Validate serial counts match ship qty
          for (const item of fulfillmentItems) {
            if (item.requires_serial && item.ship_qty > 0) {
              if ((item.selected_serials?.length || 0) !== item.ship_qty) {
                toast.warning(`Select ${item.ship_qty} serial(s) for ${item.product_code} (${item.selected_serials?.length || 0} selected)`);
                saving = false;
                return;
              }
            }
          }

          const serialItems = fulfillmentItems
            .filter((item: any) => item.ship_qty > 0)
            .map((item: any) => ({
              order_item_id: item.order_item_id,
              ship_qty: item.ship_qty,
              serial_nos: item.selected_serials || []
            }));
          const created = await CreateDNWithSerials(formData.orderId, serialItems);
          try {
            await UpdateDeliveryNote({
              ...baseNoteData,
              id: created.id,
              dn_number: created.dn_number,
              created_at: created.created_at,
              updated_at: new Date(),
              version: created.version || 1
            });
          } catch (metadataErr) {
            console.warn('DN was created with serials, but metadata update failed:', metadataErr);
          }
        } else {
          const itemLookup = new Map(fulfillmentItems.map((item: any) => [item.order_item_id, item]));
          await CreateDeliveryNote({
            ...baseNoteData,
            status: 'Prepared',
            items: itemsToShip.map((input: any) => {
              const source: any = itemLookup.get(input.order_item_id) || {};
              return {
                order_item_id: input.order_item_id,
                product_id: source.product_id || '',
                product_code: source.product_code || '',
                description: source.description || '',
                quantity_ordered: Number(source.ordered_qty || 0),
                quantity_delivered: Number(input.ship_qty || 0),
                quantity_remaining: Math.max(Number(source.remaining_qty || 0) - Number(input.ship_qty || 0), 0)
              };
            })
          });
        }
        toast.success('Delivery Note created with item quantities');
      } else {
        if (modalMode === 'create' && formData.orderId && fulfillmentLoadFailed) {
          toast.warning('Order items could not be loaded. Please re-select the order before creating the delivery note.');
          saving = false;
          return;
        }
        if (modalMode === 'create' && formData.orderId && fulfillmentLoaded && fulfillmentItems.length === 0) {
          toast.warning('This order has no remaining items to ship.');
          saving = false;
          return;
        }
        // Fallback: Original create/edit behavior
        if (modalMode === 'create') {
          await CreateDeliveryNote(baseNoteData);
          toast.success('Delivery Note created successfully');
        } else {
          await UpdateDeliveryNote(baseNoteData);
          toast.success('Delivery Note updated successfully');
        }
      }

      showModal = false;
      fulfillmentItems = [];
      await loadDeliveryNotes();
    } catch (err) {
      console.error('Save failed:', err);
      toast.danger('Failed to save delivery note: ' + errorMessage(err));
    } finally {
      saving = false;
    }
  }

  // Handle row click - open detail modal
  async function handleRowClick(row: main.DeliveryNote) {
    const rowSnapshot = enrichDeliveryNoteForDisplay(row);
    selectedNote = rowSnapshot;
    noteItems = [];

    // Load full delivery note details
    try {
      const fullNote = await GetDeliveryNoteByID(row.id);
      selectedNote = enrichDeliveryNoteForDisplay(fullNote, rowSnapshot);
      noteItems = await buildDetailItems(selectedNote, fullNote.items || []);
    } catch (err) {
      console.error('Failed to load delivery note details:', err);
      noteItems = await buildDetailItems(rowSnapshot, (rowSnapshot as any).items || []);
      toast.warning('Could not load full delivery note details');
    }

    showDetailModal = true;
  }

  // Quick actions
  async function handleDispatch(note: main.DeliveryNote) {
    if (!note.driver_name || !note.vehicle_number) {
      toast.warning('Driver and vehicle information required to dispatch');
      return;
    }

    try {
      await DispatchDeliveryNote(note.id, note.driver_name, note.vehicle_number);
      toast.success('Delivery Note dispatched');
      await loadDeliveryNotes();
    } catch (err) {
      console.error('Dispatch failed:', err);
      toast.danger('Failed to dispatch: ' + (err as Error).message);
    }
  }

  async function handleConfirmDelivery(note: main.DeliveryNote) {
    try {
      await ConfirmDeliveryNote(note.id, 'Auto-confirmed');
      toast.success('Delivery confirmed');
      await loadDeliveryNotes();
    } catch (err) {
      console.error('Confirm failed:', err);
      toast.danger('Failed to confirm delivery: ' + (err as Error).message);
    }
  }

  async function handleDelete(note: main.DeliveryNote) {
    if (!confirm(`Delete Delivery Note ${note.dn_number}?`)) {
      return;
    }

    try {
      await DeleteDeliveryNote(note.id);
      toast.success('Delivery Note deleted');
      await loadDeliveryNotes();
      showDetailModal = false;
    } catch (err) {
      console.error('Delete failed:', err);
      toast.danger('Failed to delete: ' + (err as Error).message);
    }
  }

  async function handleGeneratePDF(note: main.DeliveryNote) {
    if (!note?.id) return;
    pdfGenerating = true;
    try {
      const filePath = await GenerateDeliveryNotePDF(note.id);
      toast.success('Delivery Note PDF generated');
      if (filePath) {
        await OpenExportedFile(filePath);
      }
    } catch (err) {
      console.error('Delivery note PDF failed:', err);
      toast.danger('Failed to generate delivery note PDF: ' + errorMessage(err));
    } finally {
      pdfGenerating = false;
    }
  }

  // Helper functions
  function formatDate(date: any): string {
    if (!date) return '—';
    return new Date(date).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  }

  function formatDeliveryInfo(note: main.DeliveryNote): string {
    if (note.is_partial_delivery) {
      return `${note.delivery_sequence} of ${note.total_deliveries}`;
    }
    return 'Full';
  }

  // Handle order selection - E1 enhanced with fulfillment detail
  async function handleOrderChange(orderId: string) {
    formData.orderId = orderId;
    fulfillmentItems = [];
    fulfillmentLoaded = false;
    fulfillmentLoadFailed = false;

    let order: any = orders.find(o => o.id === orderId);
    if (!order) {
      try {
        order = await GetOrder(orderId);
      } catch (err) {
        console.error('Failed to load selected order for delivery note:', err);
      }
    } else if (!order.items || order.items.length === 0) {
      try {
        order = await GetOrder(orderId);
      } catch (err) {
        console.warn('Failed to hydrate selected order items for delivery note:', err);
      }
    }

    if (order) {
      formData.orderNumber = order.order_number || '';
      formData.customerId = order.customer_id || '';
      formData.customerName = order.customer_name || '';
    }

    // Load fulfillment detail for per-item ship quantities
    loadingFulfillment = true;
    try {
      const fulfillment = await GetOrderFulfillmentDetail(orderId);
      fulfillmentLoaded = true;
      fulfillmentLoadFailed = false;
      if (fulfillment && fulfillment.items) {
        fulfillmentItems = fulfillment.items
          .filter((item: any) => item.remaining_qty > 0.001)
          .map((item: any) => ({
            order_item_id: item.order_item_id,
            product_id: item.product_id || '',
            product_code: item.product_code,
            description: item.description,
            ordered_qty: item.ordered_qty,
            shipped_qty: item.shipped_qty || 0,
            delivered_qty: item.delivered_qty || 0,
            remaining_qty: item.remaining_qty,
            ship_qty: item.remaining_qty, // Default to remaining
            // Phase 23: Serial tracking
            requires_serial: item.requires_serial_tracking || false,
            available_serials: [] as any[],
            selected_serials: [] as string[],
          }));
        // Phase 23: Load available serials for serialized items
        for (const item of fulfillmentItems) {
          if (item.requires_serial && item.product_id) {
            try {
              const serials = await GetAvailableSerials(item.product_id);
              item.available_serials = serials || [];
            } catch (e) {
              console.warn('Failed to load serials for', item.product_code, e);
            }
          }
        }
      }

      if (fulfillmentItems.length === 0 && (!fulfillment?.items || fulfillment.items.length === 0)) {
        const orderItems = (order?.items || []).filter((item: any) => asNumber(item.quantity) > 0);
        if (orderItems.length > 0) {
          fulfillmentItems = orderItems
            .map(orderItemToFulfillmentRow)
            .filter((item: any) => item.remaining_qty > 0.001);
          fulfillmentLoaded = true;
          fulfillmentLoadFailed = fulfillmentItems.length === 0;
          if (fulfillmentItems.length > 0) {
            toast.warning('Loaded order items directly. Please review quantities before saving.');
          }
        }
      }

      if (fulfillmentItems.length === 0 && fulfillment?.items?.length > 0) {
        toast.info('All items have been fully shipped for this order');
      }
    } catch (err) {
      console.error('Failed to load fulfillment detail:', err);
      fulfillmentLoadFailed = true;
      try {
        const orderDetail: any = order || await GetOrder(orderId);
        const orderItems = (orderDetail?.items || []).filter((item: any) => Number(item.quantity || 0) > 0);
        fulfillmentItems = orderItems.map((item: any) => {
          const orderedQty = Number(item.quantity || 0);
          const alreadyShipped = Number(item.quantity_shipped || 0);
          const remainingQty = Math.max(orderedQty - alreadyShipped, 0);
          return {
            order_item_id: item.id,
            product_id: item.product_id || '',
            product_code: item.product_code,
            description: item.description,
            ordered_qty: orderedQty,
            shipped_qty: alreadyShipped,
            delivered_qty: alreadyShipped,
            remaining_qty: remainingQty,
            ship_qty: remainingQty,
            requires_serial: false,
            available_serials: [] as any[],
            selected_serials: [] as string[],
          };
        }).filter((item: any) => item.remaining_qty > 0.001);
        fulfillmentLoaded = true;
        fulfillmentLoadFailed = fulfillmentItems.length === 0;
        if (fulfillmentItems.length > 0) {
          toast.warning('Loaded order items directly. Please review quantities before saving.');
        }
      } catch (fallbackErr) {
        console.error('Failed to load order item fallback:', fallbackErr);
      }
    } finally {
      loadingFulfillment = false;
    }

    // Check if there are existing delivery notes for this order
    try {
      const existingNotes = await GetDeliveryNotesByOrder(orderId);
      if (existingNotes && existingNotes.length > 0) {
        formData.isPartialDelivery = true;
        formData.deliverySequence = existingNotes.length + 1;
        formData.totalDeliveries = existingNotes[0].total_deliveries || existingNotes.length + 1;
        toast.info(`Partial delivery ${formData.deliverySequence} of ${formData.totalDeliveries}`);
      }
    } catch (err) {
      console.error('Failed to check existing deliveries:', err);
    }
  }

  // Event handler for opening create modal from parent hub
  function handleOpenCreateDN() {
    openCreateModal();
  }

  // Check for pending DN creation from OrdersScreen (store-based, no timing issues)
  async function checkPendingDNCreate() {
    const pending = $pendingDNCreate;
    if (pending) {
      // Clear immediately to prevent re-triggering
      pendingDNCreate.clear();
      // Open create modal and pre-fill order data
      await openCreateModal();
      if (pending.orderId) {
        await handleOrderChange(pending.orderId);
      }
    }
  }

  onMount(() => {
    void (async () => {
      await loadDeliveryNotes();
      await checkPendingDNCreate();
    })();

    // Listen for create DN events from OperationsHub header button
    window.addEventListener('openCreateDN', handleOpenCreateDN);
  });

  onDestroy(() => {
    // Clean up event listener
    window.removeEventListener('openCreateDN', handleOpenCreateDN);
  });
</script>

{#if embedded}
  <!-- Embedded mode in Operations Hub: full DN workspace without nested page chrome -->
  <div class="delivery-notes-embedded">
    <div class="embedded-summary">
      <div class="stat-item">
        <span class="stat-value">{stats.total}</span>
        <span class="stat-label">Total DNs</span>
      </div>
      <div class="stat-item">
        <span class="stat-value">{stats.inTransit}</span>
        <span class="stat-label">In Transit</span>
      </div>
      <div class="stat-item">
        <span class="stat-value">{stats.partial}</span>
        <span class="stat-label">Partial</span>
      </div>
    </div>

    <div class="controls-bar embedded-controls">
      <div class="filter-tabs">
        <button
          class="filter-tab"
          class:active={activeFilter === 'All'}
          on:click={() => activeFilter = 'All'}
        >
          All ({deliveryNotes.length})
        </button>
        {#each DELIVERY_STATUSES as status}
          <button
            class="filter-tab"
            class:active={activeFilter === status.key}
            on:click={() => activeFilter = status.key}
          >
            {status.label} ({deliveryNotes.filter(dn => dn.status === status.key).length})
          </button>
        {/each}
      </div>

      <div class="search-box">
        <input
          type="text"
          placeholder="Search delivery notes..."
          bind:value={searchQuery}
          class="search-input"
        />
      </div>
    </div>

    <Card padding="sm">
      {#if loading}
        <div class="loading-container">
          <WabiSpinner size="lg" />
        </div>
      {:else}
        <DataTable
          {columns}
          data={filteredDeliveryNotes}
          {loading}
          emptyMessage="No delivery notes found"
          onRowClick={handleRowClick}
          keyField="id"
          stickyHeader={true}
          maxHeight="calc(100vh - 360px)"
          showBorder={false}
        >
          <div slot="cell" let:column let:row let:value>
            {#if column.key === 'status'}
              <StatusBadge status={value} />
            {:else if column.key === 'delivery_info'}
              <div class="delivery-badge">
                {formatDeliveryInfo(row)}
              </div>
            {:else if column.key === 'transport_method'}
              <span class="transport-value">{value || '—'}</span>
            {:else}
              {value}
            {/if}
          </div>
        </DataTable>
      {/if}
    </Card>
  </div>
{:else}
  <!-- Full screen mode with DataTable -->
  <PageLayout title="Delivery Notes" subtitle="Shipment & Logistics Management">
    <div slot="header-actions" class="header-actions">
      <div class="stats-row">
        <div class="stat-item">
          <span class="stat-value">{stats.total}</span>
          <span class="stat-label">Total DNs</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{stats.inTransit}</span>
          <span class="stat-label">In Transit</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{stats.partial}</span>
          <span class="stat-label">Partial</span>
        </div>
      </div>
      <Button variant="primary" on:click={openCreateModal}>
        + New Delivery Note
      </Button>
    </div>

    <!-- Filters -->
    <div class="controls-bar">
      <div class="filter-tabs">
        <button
          class="filter-tab"
          class:active={activeFilter === 'All'}
          on:click={() => activeFilter = 'All'}
        >
          All ({deliveryNotes.length})
        </button>
        {#each DELIVERY_STATUSES as status}
          <button
            class="filter-tab"
            class:active={activeFilter === status.key}
            on:click={() => activeFilter = status.key}
          >
            {status.label} ({deliveryNotes.filter(dn => dn.status === status.key).length})
          </button>
        {/each}
      </div>

      <div class="search-box">
        <input
          type="text"
          placeholder="Search delivery notes..."
          bind:value={searchQuery}
          class="search-input"
        />
      </div>
    </div>

    <!-- DataTable -->
    <Card>
      {#if loading}
        <div class="loading-container">
          <WabiSpinner size="lg" />
        </div>
      {:else}
        <DataTable
          {columns}
          data={filteredDeliveryNotes}
          {loading}
          emptyMessage="No delivery notes found"
          onRowClick={handleRowClick}
          keyField="id"
          stickyHeader={true}
          maxHeight="calc(100vh - 280px)"
        >
          <div slot="cell" let:column let:row let:value>
            {#if column.key === 'status'}
              <StatusBadge status={value} />
            {:else if column.key === 'delivery_info'}
              <div class="delivery-badge">
                {formatDeliveryInfo(row)}
              </div>
            {:else if column.key === 'transport_method'}
              <span class="transport-value">{value || '—'}</span>
            {:else}
              {value}
            {/if}
          </div>
        </DataTable>
      {/if}
    </Card>
  </PageLayout>
{/if}

<!-- Create/Edit Modal -->
{#if showModal}
  <Modal
    title={modalMode === 'create' ? 'New Delivery Note' : 'Edit Delivery Note'}
    open={showModal}
    on:close={() => showModal = false}
    size="lg"
  >
    <form on:submit|preventDefault={handleSubmit}>
      <div class="form-row">
        <FormGroup label="DN Number" required>
          <Input
            bind:value={formData.dnNumber}
            placeholder="DN-2026-0001"
            disabled={modalMode === 'edit'}
          />
        </FormGroup>

        <FormGroup label="Delivery Date" required>
          <Input type="date" bind:value={formData.deliveryDate} />
        </FormGroup>
      </div>

      <FormGroup label="Order" required>
        <select
          bind:value={formData.orderId}
          on:change={(e) => handleOrderChange(e.currentTarget.value)}
          class="form-select"
        >
          <option value="">Select order...</option>
          {#each orders as order}
            <option value={order.id}>{order.order_number} - {order.customer_name}</option>
          {/each}
        </select>
      </FormGroup>

      <!-- E1: Order Items with Ship Quantities -->
      {#if modalMode === 'create' && formData.orderId}
        <div class="fulfillment-section">
          <h4 class="section-title">Items to Ship</h4>
          {#if loadingFulfillment}
            <div class="loading-container" style="padding: 16px;">
              <WabiSpinner size="sm" />
              <span style="margin-left: 8px; font-size: 13px; color: var(--text-secondary);">Loading order items...</span>
            </div>
          {:else if fulfillmentItems.length > 0}
            <table class="fulfillment-table">
              <thead>
                <tr>
                  <th>Product</th>
                  <th>Description</th>
                  <th class="number-col">Ordered</th>
                  <th class="number-col">Already Shipped</th>
                  <th class="number-col">Remaining</th>
                  <th class="number-col">Ship Qty</th>
                </tr>
              </thead>
              <tbody>
                {#each fulfillmentItems as item, i}
                  <tr>
                    <td class="product-code">{item.product_code || '-'}</td>
                    <td class="description-cell">{item.description || '-'}</td>
                    <td class="number-col">{item.ordered_qty}</td>
                    <td class="number-col">{item.delivered_qty}</td>
                    <td class="number-col">{item.remaining_qty}</td>
                    <td class="number-col">
                      <input
                        type="number"
                        bind:value={fulfillmentItems[i].ship_qty}
                        min="0"
                        max={item.remaining_qty}
                        step="0.01"
                        class="ship-qty-input"
                      />
                    </td>
                  </tr>
                  {#if item.requires_serial && item.ship_qty > 0}
                    <tr class="serial-picker-row">
                      <td colspan="6" style="padding: 4px 8px 8px 24px;">
                        <div class="dn-serial-section">
                          <div class="dn-serial-label">
                            <span class="dn-serial-badge">Serialized</span>
                            Select {item.ship_qty} serial number(s) — {item.selected_serials?.length || 0} selected
                          </div>
                          {#if item.available_serials && item.available_serials.length > 0}
                            <div class="dn-serial-list">
                              {#each item.available_serials as serial}
                                <label class="dn-serial-option">
                                  <input
                                    type="checkbox"
                                    checked={item.selected_serials?.includes(serial.serial_no)}
                                    on:change={(e) => {
                                      if (!item.selected_serials) item.selected_serials = [];
                                      if (e.currentTarget.checked) {
                                        if (item.selected_serials.length < item.ship_qty) {
                                          item.selected_serials = [...item.selected_serials, serial.serial_no];
                                          fulfillmentItems = fulfillmentItems;
                                        } else {
                                          e.currentTarget.checked = false;
                                          toast.warning(`Max ${item.ship_qty} serials for ${item.product_code}`);
                                        }
                                      } else {
                                        item.selected_serials = item.selected_serials.filter(s => s !== serial.serial_no);
                                        fulfillmentItems = fulfillmentItems;
                                      }
                                    }}
                                  />
                                  <span class="dn-serial-no">{serial.serial_no}</span>
                                </label>
                              {/each}
                            </div>
                          {:else}
                            <p class="dn-serial-empty">No available serials in inventory for {item.product_code}</p>
                          {/if}
                        </div>
                      </td>
                    </tr>
                  {/if}
                {/each}
              </tbody>
            </table>
          {:else if formData.orderId}
            {#if fulfillmentLoadFailed}
              <p class="empty-fulfillment">Order items could not be loaded. Re-select the order or refresh the screen.</p>
            {:else}
              <p class="empty-fulfillment">All items in this order have been fully shipped.</p>
            {/if}
          {/if}
        </div>
      {/if}

      <FormGroup label="Delivery Address" required>
        <textarea
          bind:value={formData.deliveryAddress}
          placeholder="Full delivery address"
          class="form-textarea"
          rows="3"
        ></textarea>
      </FormGroup>

      <div class="form-row">
        <FormGroup label="Contact Person">
          <Input
            bind:value={formData.contactPerson}
            placeholder="Contact name"
          />
        </FormGroup>

        <FormGroup label="Contact Phone">
          <Input
            bind:value={formData.contactPhone}
            placeholder="+973 XXXX XXXX"
          />
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="Driver Name">
          <Input
            bind:value={formData.driverName}
            placeholder="Driver name"
          />
        </FormGroup>

        <FormGroup label="Vehicle Number">
          <Input
            bind:value={formData.vehicleNumber}
            placeholder="Vehicle registration"
          />
        </FormGroup>
      </div>

      <div class="form-row">
        <FormGroup label="Transport Method">
          <select bind:value={formData.transportMethod} class="form-select">
            {#each TRANSPORT_METHODS as method}
              <option value={method}>{method}</option>
            {/each}
          </select>
        </FormGroup>

        <FormGroup label="Status">
          <select bind:value={formData.status} class="form-select">
            {#each DELIVERY_STATUSES as status}
              <option value={status.key}>{status.label}</option>
            {/each}
          </select>
        </FormGroup>
      </div>

      <!-- Partial Delivery Section -->
      <div class="partial-delivery-section">
        <label class="checkbox-label">
          <input
            type="checkbox"
            bind:checked={formData.isPartialDelivery}
            class="checkbox-input"
          />
          <span>Partial Delivery</span>
        </label>

        {#if formData.isPartialDelivery}
          <div class="form-row">
            <FormGroup label="Delivery Sequence">
              <Input
                type="number"
                bind:value={formData.deliverySequence}
                min="1"
                max={formData.totalDeliveries}
              />
            </FormGroup>

            <FormGroup label="Total Deliveries">
              <Input
                type="number"
                bind:value={formData.totalDeliveries}
                min={formData.deliverySequence}
              />
            </FormGroup>
          </div>
        {/if}
      </div>

      <div class="modal-actions">
        <Button variant="ghost" on:click={() => showModal = false} disabled={saving}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={saving}>
          {saving ? 'Saving...' : modalMode === 'edit' ? 'Update Delivery Note' : 'Create Delivery Note'}
        </Button>
      </div>
    </form>
  </Modal>
{/if}

<!-- Delivery Note Detail Modal -->
{#if showDetailModal && selectedNote}
  <Modal
    title={`Delivery Note ${selectedNote.dn_number}`}
    open={showDetailModal}
    on:close={() => showDetailModal = false}
    size="xl"
  >
    <div class="note-detail">
      <!-- Header Info -->
      <div class="detail-header">
        <div class="detail-section">
          <h4>Order Reference</h4>
          <p>{selectedNote.order_reference || selectedNote.order_id || '-'}</p>
        </div>
        <div class="detail-section">
          <h4>Customer</h4>
          <p>{selectedNote.customer_name || '-'}</p>
        </div>
        <div class="detail-section">
          <h4>Delivery Date</h4>
          <p>{formatDate(selectedNote.delivery_date)}</p>
        </div>
        <div class="detail-section">
          <h4>Status</h4>
          <StatusBadge status={selectedNote.status} />
        </div>
      </div>

      <!-- Delivery Information -->
      <div class="info-section">
        <h4>Delivery Information</h4>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">Address:</span>
            <span class="info-value">{selectedNote.delivery_address || '—'}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Contact Person:</span>
            <span class="info-value">{selectedNote.contact_person || '—'}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Contact Phone:</span>
            <span class="info-value">{selectedNote.contact_phone || '—'}</span>
          </div>
        </div>
      </div>

      <!-- Transport Information -->
      <div class="info-section">
        <h4>Transport Information</h4>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">Method:</span>
            <span class="info-value">{selectedNote.transport_method || '—'}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Driver:</span>
            <span class="info-value">{selectedNote.driver_name || '—'}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Vehicle:</span>
            <span class="info-value">{selectedNote.vehicle_number || '—'}</span>
          </div>
        </div>
      </div>

      <!-- Partial Delivery Info -->
      {#if selectedNote.is_partial_delivery}
        <div class="partial-info">
          <span class="partial-badge">
            Partial Delivery: {selectedNote.delivery_sequence} of {selectedNote.total_deliveries}
          </span>
        </div>
      {/if}

      <!-- Delivery Items -->
      {#if noteItems.length > 0}
        <div class="items-section">
          <h4>Delivery Items</h4>
          <div class="items-table-wrap">
            <table class="items-table">
              <thead>
                <tr>
                  <th>Product Code</th>
                  <th>Description</th>
                  <th class="number-col">Ordered</th>
                  <th class="number-col">Delivered</th>
                  <th class="number-col">Remaining</th>
                </tr>
              </thead>
              <tbody>
                {#each noteItems as item}
                  <tr>
                    <td class="product-code">{item.product_code || '-'}</td>
                    <td>{item.description || '-'}</td>
                    <td class="number-col">{item.quantity_ordered}</td>
                    <td class="number-col">{item.quantity_delivered}</td>
                    <td class="number-col">{item.quantity_remaining}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {:else}
        <div class="items-section">
          <h4>Delivery Items</h4>
          <p class="empty-fulfillment">No line items found for this delivery note.</p>
        </div>
      {/if}

      <!-- Quick Actions -->
      <div class="quick-actions">
        <Button variant="secondary" on:click={() => handleGeneratePDF(selectedNote)} disabled={pdfGenerating}>
          {pdfGenerating ? 'Generating PDF...' : 'Download PDF'}
        </Button>
        {#if selectedNote.status === 'Draft' || selectedNote.status === 'Prepared'}
          <Button variant="primary" on:click={() => handleDispatch(selectedNote)}>
            Dispatch
          </Button>
        {/if}
        {#if selectedNote.status === 'InTransit'}
          <Button variant="success" on:click={() => handleConfirmDelivery(selectedNote)}>
            Confirm Delivery
          </Button>
        {/if}
        <Button variant="secondary" on:click={() => openEditModal(selectedNote)}>
          Edit
        </Button>
        {#if selectedNote.status === 'Draft'}
          <Button variant="danger" on:click={() => handleDelete(selectedNote)}>
            Delete
          </Button>
        {/if}
      </div>
    </div>
  </Modal>
{/if}

<style>
  /* Embedded Mode */
  .delivery-notes-embedded {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .embedded-summary {
    display: flex;
    justify-content: flex-end;
    gap: 24px;
    min-height: 28px;
  }

  .embedded-controls {
    margin-bottom: 0;
  }

  @media (max-width: 900px) {
    .embedded-summary {
      justify-content: flex-start;
      flex-wrap: wrap;
    }
  }

  /* Full Screen Mode */
  .header-actions {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .stats-row {
    display: flex;
    gap: 24px;
    margin-right: auto;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .stat-value {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.2;
  }

  .stat-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin-top: 2px;
  }

  /* Controls Bar */
  .controls-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    gap: 16px;
  }

  .filter-tabs {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .filter-tab {
    padding: 8px 16px;
    border: 1px solid var(--border);
    background: transparent;
    border-radius: var(--radius-md);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .filter-tab:hover {
    background: var(--bg-subtle);
    color: var(--text-primary);
  }

  .filter-tab.active {
    background: var(--primary);
    color: white;
    border-color: var(--primary);
  }

  .search-box {
    flex-shrink: 0;
  }

  .search-input {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    width: 280px;
    outline: none;
    transition: border-color 0.2s;
  }

  .search-input:focus {
    border-color: var(--primary);
  }

  /* Loading */
  .loading-container {
    display: flex;
    justify-content: center;
    padding: 48px;
  }

  /* Custom cells */
  .delivery-badge {
    display: inline-block;
    padding: 2px 8px;
    background: var(--bg-subtle);
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--text-primary);
  }

  .transport-value {
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* Form */
  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .form-select,
  .form-textarea {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 13px;
    outline: none;
    transition: border-color 0.2s;
    font-family: var(--font-sans);
  }

  .form-select:focus,
  .form-textarea:focus {
    border-color: var(--primary);
  }

  .form-textarea {
    resize: vertical;
  }

  .partial-delivery-section {
    padding: 16px;
    background: var(--bg-subtle);
    border-radius: var(--radius-md);
    margin-top: 16px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    margin-bottom: 12px;
  }

  .checkbox-input {
    width: 18px;
    height: 18px;
    cursor: pointer;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }

  /* Delivery Note Detail */
  .note-detail {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .detail-header {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
  }

  .detail-section,
  .info-grid {
    background: var(--bg-subtle);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
  }

  .detail-section {
    padding: 12px;
    min-width: 0;
  }

  .detail-section h4 {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin: 0 0 4px 0;
  }

  .detail-section p {
    font-size: 14px;
    color: var(--text-primary);
    margin: 0;
    overflow-wrap: anywhere;
  }

  /* Info Section */
  .info-section h4 {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .info-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    padding: 12px;
  }

  .info-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .info-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }

  .info-value {
    font-size: 13px;
    color: var(--text-primary);
  }

  /* Partial Info */
  .partial-info {
    padding: 12px;
    background: var(--primary-subtle);
    border-radius: var(--radius-md);
    text-align: center;
  }

  .partial-badge {
    font-size: 13px;
    font-weight: 600;
    color: var(--primary);
  }

  /* Items Table */
  .items-section h4 {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary);
  }

  .items-table-wrap {
    overflow-x: auto;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
  }

  .items-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
    min-width: 760px;
  }

  .items-table thead {
    background: var(--bg-subtle);
  }

  .items-table th {
    padding: 8px;
    text-align: left;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
  }

  .items-table td {
    padding: 8px;
    border-bottom: 1px solid var(--border-subtle);
    vertical-align: top;
  }

  .items-table .product-code {
    font-family: var(--font-mono, monospace);
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }

  .items-table .number-col {
    text-align: right;
    font-family: var(--font-mono);
  }

  .items-table tbody tr:hover {
    background: var(--bg-subtle);
  }

  /* Quick Actions */
  .quick-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }

  /* E1: Fulfillment Section in Create Modal */
  .fulfillment-section {
    margin: 16px 0;
    padding: 16px;
    background: var(--bg-subtle, #f9f9f9);
    border-radius: var(--radius-md, 8px);
    border: 1px solid var(--border, #e5e5e5);
  }

  .section-title {
    font-size: 13px;
    font-weight: 600;
    margin: 0 0 12px 0;
    color: var(--text-primary, #1d1d1f);
  }

  .fulfillment-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  .fulfillment-table thead {
    background: var(--bg-subtle, #f0f0f0);
  }

  .fulfillment-table th {
    padding: 6px 8px;
    text-align: left;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary, #6e6e73);
    border-bottom: 1px solid var(--border, #e5e5e5);
  }

  .fulfillment-table td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border-subtle, #f0f0f0);
    vertical-align: middle;
  }

  .fulfillment-table .number-col {
    text-align: right;
    font-family: var(--font-mono, monospace);
  }

  .fulfillment-table .product-code {
    font-family: var(--font-mono, monospace);
    font-weight: 600;
    font-size: 11px;
  }

  .fulfillment-table .description-cell {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ship-qty-input {
    width: 70px;
    padding: 4px 6px;
    border: 1px solid var(--border, #e5e5e5);
    border-radius: var(--radius-sm, 4px);
    font-size: 12px;
    font-family: var(--font-mono, monospace);
    text-align: right;
    outline: none;
  }

  .ship-qty-input:focus {
    border-color: var(--primary, #1d1d1f);
    box-shadow: 0 0 0 2px rgba(29, 29, 31, 0.1);
  }

  .empty-fulfillment {
    text-align: center;
    padding: 12px;
    color: var(--text-secondary, #6e6e73);
    font-size: 13px;
    font-style: italic;
  }

  @media (max-width: 900px) {
    .detail-header,
    .info-grid {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 23: Serial picker in DN create modal */
  .serial-picker-row td {
    background: var(--bg-tertiary, #f5f5f7) !important;
    border-top: none !important;
  }
  .dn-serial-section {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .dn-serial-label {
    font-size: 11px;
    color: var(--text-secondary, #6e6e73);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .dn-serial-badge {
    background: var(--primary, #1d1d1f);
    color: white;
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 4px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .dn-serial-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    max-height: 120px;
    overflow-y: auto;
    padding: 4px 0;
  }
  .dn-serial-option {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    font-family: var(--font-mono, monospace);
    cursor: pointer;
    padding: 2px 8px;
    border: 1px solid var(--border, #e5e5e5);
    border-radius: 4px;
    background: var(--bg-primary, #fff);
  }
  .dn-serial-option:has(input:checked) {
    border-color: var(--primary, #1d1d1f);
    background: rgba(29, 29, 31, 0.05);
  }
  .dn-serial-no {
    font-weight: 500;
  }
  .dn-serial-empty {
    font-size: 11px;
    color: var(--text-tertiary, #999);
    font-style: italic;
    padding: 4px 0;
    margin: 0;
  }
</style>
