<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import { toast } from "$lib/stores/toasts";
    import { permissions } from '$lib/stores/authContext';
    $: permissionList = Array.isArray($permissions) ? $permissions : [];
    $: canView =
        permissionList.includes('*') ||
        permissionList.includes('costing:read') ||
        permissionList.includes('costing:*') ||
        permissionList.includes('offers:view') ||
        permissionList.includes('offers:create') ||
        permissionList.includes('rfq:view') ||
        permissionList.includes('orders:view');
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import Button from "../components/ui/Button.svelte";
    import Card from "../components/ui/Card.svelte";
    import { createEventDispatcher } from "svelte";
    import {
        ListCustomers,
        ListSuppliers,
        GetCostingSheets,
        GetRFQs,
        GetPipelineOpportunities,
        GetOpportunityLineItems,
        ExportCostingToPDF,
        ExportCostingToExcel,
        OpenExportedFile,
        SaveCostingAsOffer,
        CreateCostingSheet,
        UpdateCostingSheet,
        GetCostingsByRFQ,
        SetActiveCostingRevision,
        CloneCostingAsNewRevision,
        GetSettings,
        GetCurrentUserStub,
        AttachCostingSheetFileWithDialog,
        ListCostingSheetAttachments,
        DeleteCostingSheetAttachment,
        OpenCostingSheetAttachment,
    } from "../../../wailsjs/go/main/App";

    const dispatch = createEventDispatcher();
    const COSTING_DRAFT_STORAGE_KEY = 'asymmflow.costingSheet.unsavedDraft.v2';
    const COSTING_DRAFT_RESTORE_PAUSED_KEY = 'asymmflow.costingSheet.restorePaused.v1';

    // Props
    export let embedded = false;
    export let active = true;

    // State
    let loading = true;
    let hasLoadedInitialData = false;
    let customers: any[] = [];
    let costingSheets: any[] = [];
    let opportunities: any[] = [];
    let previewOpportunities: any[] = [];
    let selectedOpportunityId: string = '';
    let selectedOpportunity: any = null;
    let linkOpportunityId: string = '';
    let sourceOfferId = '';
    let sourceOfferNumber = '';
    let showCostingForm = false;
    let calculating = false;
    let saving = false;
    let exporting = false;

    // Settings-driven defaults (loaded from Settings screen)
    let vatRate = 10;      // Percentage, loaded from settings
    let defaultMargin = 20; // Percentage, loaded from settings
    const defaultFreightPercent = 9;

    // Revision tracking state (Feature D)
    let rfqCostings: any[] = [];
    let selectedRevision: any = null;
    let loadingRevisions = false;
    let isDirty = false; // Track unsaved changes
    let showAdvancedHeader = false; // Toggle for compliance/certificate fields
    let suppressDraftPersistence = false;
    let lastPersistedDraftJSON = '';
    let hasStoredDraft = false;
    let showBackWarning = false;
    let attachmentScopeId = '';
    let costingAttachments: any[] = [];
    let attachmentLoading = false;

    function parseYear(value: any): number {
        const numericYear = Number(value);
        if (Number.isFinite(numericYear) && numericYear >= 2000 && numericYear <= 2100) {
            return numericYear;
        }

        if (value) {
            const date = new Date(value);
            if (!Number.isNaN(date.getTime())) {
                return date.getFullYear();
            }
        }

        return new Date().getFullYear();
    }

    function normalizePipelineOpportunity(opp: any) {
        return {
            ...opp,
            client: opp.customer_name,
            customer: opp.customer_name,
            project: opp.title || opp.folder_name || opp.folder_number,
            project_name: opp.title || opp.folder_name || opp.folder_number,
            value: Number(opp.revenue_bhd) || 0,
            status: opp.stage || "New",
            stage: opp.stage || "New",
            rfq_number: opp.folder_number,
            year: parseYear(opp.year),
            _source: "pipeline",
        };
    }

    function normalizeRFQ(rfq: any) {
        return {
            ...rfq,
            client: rfq.client || rfq.customer || "Unknown",
            customer: rfq.client || rfq.customer || "Unknown",
            project: rfq.project || rfq.project_name || "Untitled",
            project_name: rfq.project || rfq.project_name || "Untitled",
            value: Number(rfq.value) || 0,
            status: rfq.status || rfq.stage || "New",
            stage: rfq.stage || rfq.status || "New",
            year: parseYear(rfq.created_at),
            _source: "rfq",
        };
    }

    function parseOpportunitySeedItems(productDetails?: string): any[] {
        if (!productDetails) return [];
        try {
            const parsed = JSON.parse(productDetails);
            const items = Array.isArray(parsed) ? parsed : (parsed && typeof parsed === 'object' ? [parsed] : []);
            return items.filter((item: any) => !isSyntheticSeedItem(item));
        } catch {
            return [];
        }
    }

    function isSyntheticSeedItem(item: any): boolean {
        const values = [
            item?.description,
            item?.equipment,
            item?.name,
            item?.part_number,
            item?.model,
            item?.product_code,
        ];
        return values.some((value) => /^line item\s+\d+\s*-?$/i.test(String(value || '').trim()));
    }

    function mapSeedItemsToCosting(seedItems: any[]): LineItem[] {
        const mapped = seedItems.slice(0, 10).map((seedItem: any) => {
            const item = createNewLineItem();
            const quantity = Number(seedItem.quantity) || 1;
            const explicitBHDUnitPrice = Number(seedItem.unit_price_bhd) || 0;
            const totalPrice = Number(seedItem.total_price) || 0;
            const foreignUnitPrice = Number(seedItem.unit_price) || (totalPrice > 0 ? totalPrice / quantity : 0);
            const currency = explicitBHDUnitPrice > 0 ? 'BHD' : String(seedItem.currency || 'BHD').trim().toUpperCase();
            const currencyRate = currencyOptions.find((option) => option.code === currency)?.rate || 1.0;

            item.equipment = seedItem.description || seedItem.equipment || seedItem.name || '';
            item.model = seedItem.part_number || seedItem.model || seedItem.product_code || seedItem.sku || '';
            item.longCode = seedItem.long_code || '';
            item.specification = seedItem.specification || '';
            item.detailedDescription = seedItem.detailed_description || '';
            item.quantity = quantity;
            item.currency = currencyOptions.some((option) => option.code === currency) ? currency : 'BHD';
            item.exchangeRate = item.currency === 'BHD' ? 1.0 : currencyRate;
            item.fobForeign = explicitBHDUnitPrice || foreignUnitPrice || 0;
            return item;
        }).filter((item: LineItem) => item.equipment || item.model || item.detailedDescription);

        return mapped.length > 0 ? mapped : [createNewLineItem()];
    }

    function isRFQOpportunity(opp: any): boolean {
        return Boolean(opp && opp._source === "rfq" && Number.isFinite(Number(opp.id)));
    }

    function getSelectedRFQId(): number {
        return isRFQOpportunity(selectedOpportunity) ? Number(selectedOpportunity.id) : 0;
    }

    function getOpportunityDisplayReference(opp: any): string {
        return opp?.rfq_ref || opp?.eh_ref || opp?.folder_number || opp?.rfq_number || "Manual";
    }

    function getOpportunityUserReference(opp: any): string {
        return String(opp?.rfq_ref || opp?.eh_ref || opp?.customer_reference || opp?.rfqReference || opp?.folder_number || opp?.rfq_number || '').trim();
    }

    function getOpportunityFolderReference(opp: any): string {
        return String(opp?.folder_number || (opp?._source === 'rfq' ? opp?.rfq_number : '') || '').trim();
    }

    function getSelectedOpportunityRecordId(): string {
        return selectedOpportunity?._source === 'pipeline' ? String(selectedOpportunity.id || '') : '';
    }

    function getOpportunityValue(opp: any): number {
        return Number(opp?.value ?? opp?.revenue_bhd ?? 0) || 0;
    }

    function formatOpportunityValue(opp: any): string {
        const value = getOpportunityValue(opp);
        if (value <= 0) {
            return 'Value pending';
        }
        return new Intl.NumberFormat('en-BH', {
            style: 'currency',
            currency: 'BHD',
        }).format(value);
    }

    // ========================================
    // HEADER SECTION - Matches Excel "Costing Sheet" header
    // ========================================
    const defaultDeliveryTermsForDivision = (division: string) =>
        division === 'AHS Trading' ? 'DAP Bahrain at your store or AHS Trading' : 'DAP Bahrain at your store or PH Trading';

    const normaliseDeliveryTermsForDivision = (deliveryTerms: string, division: string) => {
        const trimmed = (deliveryTerms || '').trim();
        if (!trimmed || trimmed === 'DAP Bahrain at your store or PH Trading' || trimmed === 'DAP Bahrain at your store or AHS Trading') {
            return defaultDeliveryTermsForDivision(division);
        }
        return trimmed;
    };

    function createDefaultHeader(division = 'PH Trading') {
        return {
            division,
            date: new Date().toISOString().split('T')[0],
            preparedBy: '',
            customerId: '',
            customerName: '',
            contactPerson: '',
            rfqReference: '',
            folderNumber: '',
            costingId: '',
            subject: '',
            quoteType: 'Quotation',
            estDelivery: '5-7 weeks',
            deliveryTerms: defaultDeliveryTermsForDivision(division),
            paymentTerms: '30 days from Date of Delivery',
            orderType: 'General',
            countryOfOrigin: 'DE',
            cocCoo: 'No',
            testCertificate: 'Additional charges applicable as per OEM terms',
            installation: 'No',
            commissioning: 'No',
            testing: 'No',
            // VAT/Tax compliance fields (Bahrain NBR)
            placeOfSupply: 'Kingdom of Bahrain',
            taxCategory: 'Standard',
            customerTRN: '',
        };
    }

    let header = createDefaultHeader();

    const defaultQuotationBody = `We thank you for the opportunity and are pleased to submit our techno-commercial offer for your review.

Please find our pricing and scope below. We trust the proposal meets your requirement and look forward to your valued order.`;
    let quotationBody = defaultQuotationBody;

    // Division options (sister companies)
    const divisionOptions = ['PH Trading', 'AHS Trading'];
    let lastAppliedDivision = header.division;
    let currentPreparedByName = '';

    function resolveUserDisplayName(user: any): string {
        return String(user?.display_name || user?.full_name || user?.username || user?.email || '').trim();
    }

    // Dropdown options from Excel "Code" sheet
    const deliveryOptions = [
        '3-5 weeks', '4-6 weeks', '5-7 weeks', '7-9 weeks', '9-11 weeks',
        '12-14 weeks', '16-18 weeks', 'On request'
    ];
    const deliveryTermsOptions = [
        'DAP Bahrain at your store or PH Trading',
        'DAP Bahrain at your store or AHS Trading',
        'Ex-Works (EXW)',
        'Free Carrier (FCA)',
        'Delivered Duty Paid (DDP)',
    ];
    const paymentTermsOptions = [
        '100% Advance Payment with PO',
        '100% Payment Against Delivery',
        '30 days from Date of Delivery',
        '60 days from Date of Delivery',
        'Letter of Credit (LC)',
        'Project Stage Payments',
        '50% Advance + 50% Against Delivery',
    ];
    const orderTypeOptions = ['General', 'Spareparts', 'Items + Spareparts'];
    const countryOptions = [
        { code: 'DE', name: 'Germany' },
        { code: 'CH', name: 'Switzerland' },
        { code: 'FR', name: 'France' },
        { code: 'UK', name: 'United Kingdom (GB)' },
        { code: 'US', name: 'United States of America' },
        { code: 'SL', name: 'Slovenia' },
        { code: 'GR', name: 'Greece' },
        { code: 'IT', name: 'Italy' },
    ];
    let supplierOptions: { code: string; name: string; statement: string }[] = [
        { code: 'EH', name: 'Endress+Hauser', statement: '(Authorized Endress Hauser Agency in Bahrain)' },
        { code: 'SM', name: 'Servomex', statement: '(Authorized Servomex Agency in Bahrain)' },
        { code: 'GI', name: 'GIC/Gauges Bourdon', statement: '(Authorized GIC & Guages Bourdon Agency in Bahrain)' },
        { code: 'LG', name: 'Landis+Gyr', statement: '(Authorized landys+Gyr Agency in Bahrain)' },
        { code: 'IS', name: 'Iskraemeco', statement: '(Authorized Iskraemeco Agency in Bahrain)' },
    ];
    const currencyOptions = [
        { code: 'BHD', rate: 1.0 },
        { code: 'EUR', rate: 0.45 },
        { code: 'USD', rate: 0.376 },
        { code: 'GBP', rate: 0.52 },
        { code: 'CHF', rate: 0.43 },
    ];
    const certificateOptions = ['Yes', 'No', 'Additional charges applicable as per OEM terms'];

    // ========================================
    // TERMS AND CONDITIONS (printed on separate page)
    // ========================================
    let termsAndConditions = `1. QUOTATION VALIDITY
This quotation is valid for thirty (30) days from the date of issue.

2. PRICES
All prices are in Bahraini Dinars (BHD) unless otherwise stated. Prices are exclusive of VAT (${vatRate}%) which will be added to the invoice.

3. PAYMENT TERMS
As per the payment terms specified in this quotation. Late payments may incur interest charges.

4. DELIVERY
Delivery times are estimates and subject to manufacturer's confirmation. PH Trading shall not be liable for delays beyond our control.

5. WARRANTY
All products carry the manufacturer's standard warranty. Extended warranty options are available upon request.

6. INSTALLATION & COMMISSIONING
Installation and commissioning services are available at additional cost unless included in the quotation.

7. FORCE MAJEURE
PH Trading shall not be liable for failure to perform due to causes beyond reasonable control.

8. GOVERNING LAW
This quotation is governed by the laws of the Kingdom of Bahrain.`;

    // ========================================
    // LINE ITEMS - Up to 10 products per Excel template
    // ========================================
    interface LineItem {
        // Core item fields (supplier removed per user request - invoices link to suppliers instead)
        equipment: string;
        model: string;
        serialNumber: string;
        longCode: string;  // Long supplier order/configuration code (e.g. E+H instrumentation codes)
        specification: string;
        // NEW: Wide description field for detailed instrumentation specs (prints under line item)
        detailedDescription: string;
        currency: string;
        quantity: number;
        fobForeign: number;
        freightPercent: number;
        freightForeign: number;
        // User-editable selling price (overrides calculated suggested price)
        userPrice: number;
        userPriceSet: boolean;
        // Calculated fields
        exchangeRate: number;
        fobBHD: number;
        freightBHD: number;
        cf: number;
        insurance: number;
        customsPercent: number;
        customsBHD: number;
        landedCost: number;
        handlingPercent: number;
        handlingBHD: number;
        financePercent: number;
        financeBHD: number;
        otherCosts: number;
        totalCost: number;
        // Markup percentage applied on cost: SellPrice = Cost * (1 + Markup/100)
        marginPercent: number;
        marginBHD: number;
        sellingPrice: number;
        suggestedPriceUnit: number;
        totalSuggestedPrice: number;
    }

    let lineItems: LineItem[] = [createNewLineItem()];

    function createNewLineItem(): LineItem {
        return {
            equipment: '',
            model: '',
            serialNumber: '', // Serial number for traceability (GRN → DN → Invoice)
            longCode: '',  // Long supplier order/configuration code
            specification: '',
            detailedDescription: '', // Wide field for detailed instrumentation specs
            currency: 'BHD',
            quantity: 1,
            fobForeign: 0,
            freightPercent: defaultFreightPercent,
            freightForeign: 0,
            userPrice: 0,
            userPriceSet: false,
            exchangeRate: 1.0,
            fobBHD: 0,
            freightBHD: 0,
            cf: 0,
            insurance: 0,
            customsPercent: 5,
            customsBHD: 0,
            landedCost: 0,
            handlingPercent: 4,
            handlingBHD: 0,
            financePercent: 1,
            financeBHD: 0,
            otherCosts: 0,
            totalCost: 0,
            marginPercent: defaultMargin, // From Settings (default 20%)
            marginBHD: 0,
            sellingPrice: 0,
            suggestedPriceUnit: 0,
            totalSuggestedPrice: 0,
        };
    }

    function addLineItem() {
        if (lineItems.length < 10) {
            lineItems = [...lineItems, createNewLineItem()];
            calculateAll();
        } else {
            toast.warning('Maximum 10 line items allowed');
        }
    }

    function removeLineItem(index: number) {
        if (lineItems.length > 1) {
            lineItems = lineItems.filter((_, i) => i !== index);
            calculateAll();
        }
    }

    function toFiniteNumber(value: unknown, fallback = 0): number {
        if (value === null || value === undefined || value === '') return fallback;
        const numeric = typeof value === 'number' ? value : Number(value);
        return Number.isFinite(numeric) ? numeric : fallback;
    }

    function nonNegativeNumber(value: unknown, fallback = 0): number {
        return Math.max(0, toFiniteNumber(value, fallback));
    }

    // Issue #7: Copy first item's costs to all subsequent items
    function copyFirstItemCostsToAll() {
        if (lineItems.length <= 1) {
            toast.info('Only one item - nothing to copy to');
            return;
        }

        const first = lineItems[0];
        for (let i = 1; i < lineItems.length; i++) {
            lineItems[i].customsPercent = first.customsPercent;
            lineItems[i].handlingPercent = first.handlingPercent;
            lineItems[i].financePercent = first.financePercent;
            lineItems[i].marginPercent = first.marginPercent;
        }

        // Force reactive update
        lineItems = [...lineItems];
        calculateAll();

        toast.success(`Copied costs to ${lineItems.length - 1} items`);
    }

    // Issue #12: Filter out empty line items for validation/export
    function getValidLineItems(): LineItem[] {
        return lineItems.filter(item =>
            item.equipment?.trim() || nonNegativeNumber(item.fobForeign) > 0
        );
    }

    // ========================================
    // CALCULATIONS - Matches Excel formulas
    // ========================================
    function calculateLineItem(item: LineItem): LineItem {
        // Get exchange rate from currency
        const curr = currencyOptions.find(c => c.code === item.currency);
        item.exchangeRate = toFiniteNumber(item.exchangeRate, curr?.rate || 0.45);
        if (!(item.exchangeRate > 0)) item.exchangeRate = curr?.rate || 0.45;
        item.quantity = Math.max(1, toFiniteNumber(item.quantity, 1));
        item.fobForeign = nonNegativeNumber(item.fobForeign);
        item.insurance = nonNegativeNumber(item.insurance);
        item.customsPercent = nonNegativeNumber(item.customsPercent, 5);
        item.handlingPercent = nonNegativeNumber(item.handlingPercent, 4);
        item.financePercent = nonNegativeNumber(item.financePercent, 1);
        item.otherCosts = nonNegativeNumber(item.otherCosts);
        item.userPrice = nonNegativeNumber(item.userPrice);

        // FOB and Freight in BHD
        item.fobBHD = item.fobForeign * item.exchangeRate;
        item.freightPercent = nonNegativeNumber(item.freightPercent);
        item.freightForeign = item.fobForeign * (item.freightPercent / 100);
        item.freightBHD = item.freightForeign * item.exchangeRate;

        // C&F = FOB + Freight
        item.cf = item.fobBHD + item.freightBHD;

        // Customs
        item.customsBHD = item.cf * (item.customsPercent / 100);

        // Landed Cost = C&F + Insurance + Customs
        item.landedCost = item.cf + item.insurance + item.customsBHD;

        // Handling
        item.handlingBHD = item.landedCost * (item.handlingPercent / 100);

        // Finance Charges
        item.financeBHD = item.landedCost * (item.financePercent / 100);

        // Total Cost
        item.totalCost = item.landedCost + item.handlingBHD + item.financeBHD + item.otherCosts;

        // MARKUP CALCULATION
        item.marginPercent = nonNegativeNumber(item.marginPercent);
        item.sellingPrice = item.totalCost * (1 + item.marginPercent / 100);

        // Profit amount from markup for display
        item.marginBHD = item.sellingPrice - item.totalCost;

        // Suggested Price (rounded to nearest whole number)
        item.suggestedPriceUnit = Math.ceil(item.sellingPrice);

        // If user set a custom price, use that; otherwise use suggested
        const effectivePrice = item.userPriceSet && item.userPrice > 0
            ? item.userPrice
            : item.suggestedPriceUnit;

        // Total Suggested Price
        item.totalSuggestedPrice = effectivePrice * item.quantity;

        return item;
    }

    function recalculateLineItems() {
        lineItems = lineItems.map(item => calculateLineItem(item));
        lineItems = [...lineItems]; // Trigger reactivity
    }

    function calculateAll() {
        recalculateLineItems();
        markDirty();
    }

    function buildCostingDraftPayload() {
        return {
            schemaVersion: 2,
            savedAt: new Date().toISOString(),
            showCostingForm,
            selectedOpportunityId,
            selectedOpportunity,
            sourceOfferId,
            sourceOfferNumber,
            attachmentScopeId,
            selectedRevisionId: selectedRevision?.id || null,
            currentCostingId,
            header: { ...header },
            lineItems: lineItems.map((item) => ({ ...item })),
            discount,
            hiddenCharges,
            vatRate,
            quotationBody,
            termsAndConditions,
            attachments: serialiseCostingAttachments(true),
        };
    }

    function persistCostingDraft() {
        if (suppressDraftPersistence || !showCostingForm || typeof localStorage === 'undefined') {
            return;
        }
        try {
            const draftJSON = JSON.stringify(buildCostingDraftPayload());
            if (draftJSON !== lastPersistedDraftJSON) {
                localStorage.setItem(COSTING_DRAFT_STORAGE_KEY, draftJSON);
                lastPersistedDraftJSON = draftJSON;
            }
            hasStoredDraft = true;
        } catch (err) {
            console.warn('Failed to persist costing draft:', err);
        }
    }

    function clearCostingDraft() {
        if (typeof localStorage !== 'undefined') {
            localStorage.removeItem(COSTING_DRAFT_STORAGE_KEY);
        }
        if (typeof sessionStorage !== 'undefined') {
            sessionStorage.removeItem(COSTING_DRAFT_RESTORE_PAUSED_KEY);
        }
        lastPersistedDraftJSON = '';
        hasStoredDraft = false;
    }

    function refreshStoredDraftState() {
        hasStoredDraft = typeof localStorage !== 'undefined' && Boolean(localStorage.getItem(COSTING_DRAFT_STORAGE_KEY));
    }

    function pauseDraftRestoreForSession() {
        if (typeof sessionStorage !== 'undefined') {
            sessionStorage.setItem(COSTING_DRAFT_RESTORE_PAUSED_KEY, '1');
        }
    }

    function resumeDraftRestoreForSession() {
        if (typeof sessionStorage !== 'undefined') {
            sessionStorage.removeItem(COSTING_DRAFT_RESTORE_PAUSED_KEY);
        }
    }

    function isDraftRestorePausedForSession() {
        return typeof sessionStorage !== 'undefined' && sessionStorage.getItem(COSTING_DRAFT_RESTORE_PAUSED_KEY) === '1';
    }

    function markDirty() {
        if (suppressDraftPersistence) return;
        isDirty = true;
        persistCostingDraft();
    }

    function restoreLocalCostingDraft(payload: any) {
        resumeDraftRestoreForSession();
        suppressDraftPersistence = true;
        try {
            const draftHeader = payload?.header || {};
            const draftDivision = draftHeader.division || header.division || 'PH Trading';
            header = {
                ...createDefaultHeader(draftDivision),
                ...draftHeader,
                deliveryTerms: normaliseDeliveryTermsForDivision(draftHeader.deliveryTerms || '', draftDivision),
            };
            selectedOpportunityId = String(payload?.selectedOpportunityId || '');
            selectedOpportunity = opportunities.find((opp: any) => String(opp.id) === selectedOpportunityId) || payload?.selectedOpportunity || null;
            sourceOfferId = String(payload?.sourceOfferId || '');
            sourceOfferNumber = String(payload?.sourceOfferNumber || '');
            attachmentScopeId = resolveAttachmentScope(payload);
            costingAttachments = Array.isArray(payload?.attachments) ? payload.attachments : [];
            selectedRevision = null;
            currentCostingId = payload?.currentCostingId || null;
            rfqCostings = [];
            lineItems = Array.isArray(payload?.lineItems) && payload.lineItems.length > 0
                ? payload.lineItems.map((item: any, index: number) => normalisePersistedLineItem(item, index))
                : [createNewLineItem()];
            discount = Number(payload?.discount ?? 0) || 0;
            hiddenCharges = Number(payload?.hiddenCharges ?? 0) || 0;
            vatRate = Number(payload?.vatRate ?? vatRate) || vatRate;
            quotationBody = payload?.quotationBody || defaultQuotationBody;
            termsAndConditions = payload?.termsAndConditions || termsAndConditions;
            showCostingForm = true;
            recalculateLineItems();
            void loadCostingAttachments(attachmentScopeId);
        } finally {
            suppressDraftPersistence = false;
        }
        isDirty = true;
        persistCostingDraft();
        toast.info('Restored an unsaved costing draft from this device.');
    }

    function maybeRestoreLocalCostingDraft() {
        if (showCostingForm || typeof localStorage === 'undefined') return;
        const draftJSON = localStorage.getItem(COSTING_DRAFT_STORAGE_KEY);
        hasStoredDraft = Boolean(draftJSON);
        if (!draftJSON) return;
        if (isDraftRestorePausedForSession()) return;
        try {
            const payload = JSON.parse(draftJSON);
            if (payload?.schemaVersion) {
                lastPersistedDraftJSON = draftJSON;
                restoreLocalCostingDraft(payload);
            }
        } catch (err) {
            console.warn('Discarding unreadable costing draft:', err);
            clearCostingDraft();
        }
    }

    function resumeStoredCostingDraft() {
        if (typeof localStorage === 'undefined') return;
        const draftJSON = localStorage.getItem(COSTING_DRAFT_STORAGE_KEY);
        if (!draftJSON) {
            refreshStoredDraftState();
            toast.warning('No saved costing draft found on this device.');
            return;
        }
        try {
            const payload = JSON.parse(draftJSON);
            if (!payload?.schemaVersion) {
                throw new Error('Draft format is not supported');
            }
            lastPersistedDraftJSON = draftJSON;
            restoreLocalCostingDraft(payload);
        } catch (err) {
            console.warn('Discarding unreadable costing draft:', err);
            clearCostingDraft();
            isDirty = false;
            toast.warning('That saved costing draft could not be reopened and was discarded.');
        }
    }

    function discardStoredCostingDraft() {
        clearCostingDraft();
        isDirty = false;
        toast.success('Saved costing draft discarded.');
    }

    function handleBeforeUnload(event: BeforeUnloadEvent) {
        if (!isDirty) return;
        persistCostingDraft();
        event.preventDefault();
        event.returnValue = 'You have unsaved costing changes. Save them or discard the draft before closing.';
    }

    // Summary calculations
    $: subtotal = lineItems.reduce((sum, item) => sum + toFiniteNumber(item.totalSuggestedPrice), 0);
    let discount = 0;
    let hiddenCharges = 0;
    $: effectiveDiscount = nonNegativeNumber(discount);
    $: effectiveHiddenCharges = nonNegativeNumber(hiddenCharges);
    $: effectiveVatRate = Math.min(100, nonNegativeNumber(vatRate, 10));
    $: netAmount = Math.max(0, subtotal - effectiveDiscount);
    $: vat = netAmount * (effectiveVatRate / 100);
    $: grandTotal = netAmount + vat;
    $: totalCost = lineItems.reduce((sum, item) => sum + (toFiniteNumber(item.totalCost) * Math.max(1, toFiniteNumber(item.quantity, 1))), 0) + effectiveHiddenCharges;
    $: profit = netAmount - totalCost;
    $: profitPercent = netAmount > 0 ? (profit / netAmount) * 100 : 0;

    // Keep T&C VAT label in sync when user changes VAT rate
    $: termsAndConditions = termsAndConditions.replace(/VAT \(\d+\.?\d*%\)/, `VAT (${effectiveVatRate}%)`);

    // ========================================
    // DATA LOADING
    // ========================================
    async function loadData() {
        loading = true;
        try {
            const [c, s, rfqs, pipeline, settingsResult, currentUserResult] = await Promise.all([
                ListCustomers(500, 0),
                GetCostingSheets(20).catch(() => []),
                GetRFQs(200, 0).catch(() => []),
                GetPipelineOpportunities(500, 0).catch(() => []),
                GetSettings().catch(() => null),
                GetCurrentUserStub().catch(() => null),
            ]);
            customers = c || [];
            costingSheets = s || [];
            currentPreparedByName = resolveUserDisplayName(currentUserResult);
            if (currentPreparedByName && (!header.preparedBy || header.preparedBy.toLowerCase() === 'system')) {
                header.preparedBy = currentPreparedByName;
            }
            const pipelineNormalized = (pipeline || []).map(normalizePipelineOpportunity);
            const liveRfqs = (rfqs || [])
                .filter((rfq: any) => String(rfq?.rfq_number || '').trim())
                .map(normalizeRFQ);

            const rfqFolders = new Set(
                liveRfqs
                    .map((rfq: any) => String(rfq.rfq_number || '').trim())
                    .filter(Boolean)
            );
            const uniquePipeline = pipelineNormalized.filter(
                (opp: any) => !rfqFolders.has(String(opp.folder_number || '').trim())
            );

            opportunities = [...uniquePipeline, ...liveRfqs].sort((a: any, b: any) => {
                const aTime = new Date(a.updated_at || a.created_at || 0).getTime();
                const bTime = new Date(b.updated_at || b.created_at || 0).getTime();
                return bTime - aTime;
            });
            previewOpportunities = [...opportunities]
                .sort((a: any, b: any) => {
                    const aHasValue = getOpportunityValue(a) > 0 ? 1 : 0;
                    const bHasValue = getOpportunityValue(b) > 0 ? 1 : 0;
                    if (aHasValue !== bHasValue) {
                        return bHasValue - aHasValue;
                    }
                    const aTime = new Date(a.updated_at || a.created_at || 0).getTime();
                    const bTime = new Date(b.updated_at || b.created_at || 0).getTime();
                    return bTime - aTime;
                })
                .slice(0, 6);

            // Apply settings-driven defaults
            if (settingsResult?.business) {
                if (typeof settingsResult.business.vat_rate === 'number') {
                    vatRate = settingsResult.business.vat_rate;
                }
                if (typeof settingsResult.business.default_margin === 'number') {
                    defaultMargin = settingsResult.business.default_margin;
                }
                // Update terms with actual VAT rate from settings
                termsAndConditions = termsAndConditions.replace(/VAT \(\d+%\)/, `VAT (${vatRate}%)`);
            }

            // Load suppliers from database
            try {
                const dbSuppliers = await ListSuppliers(1000, 0);
                if (dbSuppliers && dbSuppliers.length > 0) {
                    supplierOptions = dbSuppliers.map((s: any) => ({
                        code: s.supplier_code || s.id?.slice(0, 4)?.toUpperCase() || 'XX',
                        name: s.supplier_name || 'Unknown',
                        statement: s.category ? `(${s.category})` : ''
                    }));
                }
            } catch (e) {
                console.warn('Using default supplier options:', e);
            }

            let restoredLaunchPayload = false;
            const pendingCostingOpportunity = sessionStorage.getItem('asymmflow.pendingCostingOpportunity');
            if (pendingCostingOpportunity) {
                try {
                    const pending = JSON.parse(pendingCostingOpportunity);
                    const match = findOpportunityFromPendingLaunch(pending);
                    if (match) {
                        restoredLaunchPayload = true;
                        selectedOpportunityId = String(match.id);
                        await handleOpportunitySelect();
                    } else {
                        toast.warning('Could not reopen that opportunity in costing. Please select it from the list.');
                    }
                } finally {
                    sessionStorage.removeItem('asymmflow.pendingCostingOpportunity');
                }
            }

            const pendingCostingOffer = sessionStorage.getItem('asymmflow.pendingCostingOffer');
            if (pendingCostingOffer) {
                try {
                    const pending = JSON.parse(pendingCostingOffer);
                    restoredLaunchPayload = true;
                    restoreOfferCostingPayload(pending);
                } finally {
                    sessionStorage.removeItem('asymmflow.pendingCostingOffer');
                }
            }

            if (!restoredLaunchPayload && active) {
                maybeRestoreLocalCostingDraft();
            } else {
                refreshStoredDraftState();
            }
        } catch (err) {
            console.error('Failed to load data:', err);
            toast.danger("Failed to load data");
        } finally {
            loading = false;
        }
    }

    async function handleOpportunitySelect() {
        if (!selectedOpportunityId) {
            showCostingForm = false;
            selectedOpportunity = null;
            sourceOfferId = '';
            sourceOfferNumber = '';
            rfqCostings = [];
            selectedRevision = null;
            currentCostingId = null;
            attachmentScopeId = '';
            costingAttachments = [];
            return;
        }

        resumeDraftRestoreForSession();
        selectedOpportunity = opportunities.find(o => String(o.id) === selectedOpportunityId);
        if (selectedOpportunity) {
            sourceOfferId = '';
            sourceOfferNumber = '';
            let restoredRevision = false;
            if (isRFQOpportunity(selectedOpportunity)) {
                restoredRevision = await loadRevisionsForRFQ(Number(selectedOpportunity.id));
            } else {
                rfqCostings = [];
                selectedRevision = null;
                currentCostingId = null;
            }

            // Pre-fill the commercial header from the selected opportunity, including
            // the user-facing reference so it survives the costing -> offer flow.
            const customerName = selectedOpportunity.client || selectedOpportunity.customer || '';
            const projectName = selectedOpportunity.project || selectedOpportunity.project_name || '';
            const opportunityReference = getOpportunityUserReference(selectedOpportunity);
            const opportunityFolder = getOpportunityFolderReference(selectedOpportunity);
            const opportunitySubject = selectedOpportunity.title || selectedOpportunity.project_name || selectedOpportunity.project || '';
            const formattedOpportunitySubject = opportunitySubject
                ? `Sub: ${String(opportunitySubject).replace(/^Sub:\s*/i, '').trim()}`
                : '';

            // Find matching customer in customers list
            const matchedCustomer = findMatchingCustomerByName(customerName);

            if (!restoredRevision) {
                const nextDivision = selectedOpportunity.division || header.division || 'PH Trading';
                header = {
                    ...header,
                    customerName: matchedCustomer?.business_name || customerName,
                    customerId: matchedCustomer?.id || '',
                    contactPerson: matchedCustomer?.contact_person || matchedCustomer?.primary_contact || '',
                    rfqReference: opportunityReference,
                    folderNumber: opportunityFolder,
                    costingId: opportunityReference || opportunityFolder,
                    subject: formattedOpportunitySubject,
                    division: nextDivision,
                    deliveryTerms: normaliseDeliveryTermsForDivision(header.deliveryTerms, nextDivision),
                };
                attachmentScopeId = resolveAttachmentScope();
                costingAttachments = [];
                void loadCostingAttachments(attachmentScopeId);
            }

            // ========================================
            // AUTO-FILL LINE ITEMS from structured opportunity data
            // ========================================
            if (!restoredRevision) {
                const seedItems = parseOpportunitySeedItems(selectedOpportunity.product_details);

                if (seedItems.length > 0) {
                    lineItems = mapSeedItemsToCosting(seedItems);
                    calculateAll();
                    toast.success(`Loaded ${lineItems.length} line items from opportunity`);
                } else if (selectedOpportunity._source === 'pipeline') {
                    try {
                        const linkedItems = await GetOpportunityLineItems(String(selectedOpportunity.id));
                        if (Array.isArray(linkedItems) && linkedItems.length > 0) {
                            lineItems = mapSeedItemsToCosting(linkedItems);
                            calculateAll();
                            toast.success(`Loaded ${lineItems.length} line items from linked opportunity data`);
                        } else {
                            lineItems = [createNewLineItem()];
                            toast.info('No structured line items were captured for this opportunity yet.');
                        }
                    } catch (e) {
                        console.warn('Failed to load structured opportunity line items:', e);
                        lineItems = [createNewLineItem()];
                    }
                } else {
                    lineItems = [createNewLineItem()];
                }
            }

            showCostingForm = true;
            if (!restoredRevision) {
                markDirty();
            }
            toast.success(`Loaded opportunity: ${projectName}`);
        }
    }

    // ========================================
    // REVISION MANAGEMENT (Feature D)
    // ========================================
    async function loadRevisionsForRFQ(rfqId: number): Promise<boolean> {
        loadingRevisions = true;
        try {
            rfqCostings = await GetCostingsByRFQ(rfqId) || [];

            // Auto-select the active revision if exists
            const active = rfqCostings.find(c => c.is_active);
            if (active) {
                selectedRevision = active;
                const payload = JSON.parse(active.items || '[]');
                restoreCostingPayload(active, payload);
                toast.info(`Loaded active revision ${active.revision_number}`);
                return true;
            } else {
                // No existing revisions - start fresh
                selectedRevision = null;
                currentCostingId = null;
                lineItems = [createNewLineItem()];
                return false;
            }
        } catch (err) {
            console.error('Failed to load revisions:', err);
            rfqCostings = [];
            return false;
        } finally {
            loadingRevisions = false;
        }
    }

    async function selectRevision(rev: any) {
        selectedRevision = rev;

        // Load the revision's items into the form
        try {
            const payload = JSON.parse(rev.items || '[]');
            restoreCostingPayload(rev, payload);
            toast.success(`Loaded revision ${rev.revision_number}`);
        } catch (err) {
            toast.danger('Failed to load revision: ' + String(err));
        }
    }

    async function handleCreateNewRevision() {
        if (!selectedOpportunity) return;
        if (!isRFQOpportunity(selectedOpportunity)) {
            toast.info('Revision history is available for RFQ records only.');
            return;
        }

        try {
            // Clone current revision or create fresh
            const sourceId = selectedRevision?.id;
            let newCosting;

            if (sourceId) {
                newCosting = await CloneCostingAsNewRevision(sourceId, header.preparedBy || 'System');
            } else {
                // Create from current form data
                const itemsJSON = JSON.stringify(sanitiseCostingForSyncedPersistence(buildExportData()));
                newCosting = await CreateCostingSheet(Number(selectedOpportunity.id), itemsJSON, header.preparedBy || 'System');
            }

            toast.success(`Created Revision ${newCosting.revision_number}`);
            await loadRevisionsForRFQ(Number(selectedOpportunity.id));
        } catch (err) {
            toast.danger('Failed to create revision: ' + String(err));
        }
    }

    async function handleSetActiveRevision(costingId: number) {
        if (!isRFQOpportunity(selectedOpportunity)) {
            toast.info('Revision history is available for RFQ records only.');
            return;
        }
        try {
            await SetActiveCostingRevision(costingId);
            toast.success('Revision set as current');
            await loadRevisionsForRFQ(Number(selectedOpportunity.id));
        } catch (err) {
            toast.danger('Failed to set active revision: ' + String(err));
        }
    }

    function formatDate(dateStr: string): string {
        const d = new Date(dateStr);
        return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
    }

    function handleCustomerChange() {
        const customer = customers.find(c => c.id === header.customerId);
        if (customer) {
            header = {
                ...header,
                customerName: customer.business_name || customer.name || '',
                contactPerson: customer.contact_person || customer.primary_contact || '',
            };
            markDirty();
        }
    }

    function normalizeCustomerName(value: string) {
        return (value || '').trim().toLowerCase().replace(/\s+/g, ' ');
    }

    function normalizePartyLookupName(value: string) {
        return normalizeCustomerName(value)
            .replace(/&/g, ' and ')
            .replace(/[^a-z0-9]+/g, ' ')
            .replace(/\b(wll|llc|bsc|ltd|limited|company|co|with|liability)\b/g, ' ')
            .replace(/\s+/g, ' ')
            .trim();
    }

    function namesRepresentSameParty(a: string, b: string) {
        const left = normalizePartyLookupName(a);
        const right = normalizePartyLookupName(b);
        if (!left || !right) return false;
        if (left === right) return true;
        return left.length >= 8 && right.length >= 8 && (left.includes(right) || right.includes(left));
    }

    function findMatchingCustomerByName(customerName: string) {
        const normalizedName = normalizePartyLookupName(customerName);
        if (!normalizedName) return null;
        return customers.find((c: any) => {
            const names = [
                c.business_name,
                c.name,
                c.customer_name,
                c.customer_id,
                c.customer_code,
            ];
            return names.some((candidate) => namesRepresentSameParty(String(candidate || ''), customerName));
        }) || null;
    }

    function findOpportunityFromPendingLaunch(pending: any) {
        const pendingId = String(pending?.id || '').trim();
        const pendingReference = String(pending?.rfq_ref || pending?.eh_ref || pending?.folder_number || pending?.rfq_number || '').trim();
        const pendingCustomer = String(pending?.customer_name || pending?.client || pending?.customer || '').trim();
        return opportunities.find((opp: any) => {
            const oppId = String(opp?.id || '').trim();
            const oppReference = getOpportunityUserReference(opp);
            if (pendingId && oppId === pendingId) return true;
            if (pendingReference && oppReference && oppReference === pendingReference) return true;
            return Boolean(pendingCustomer && namesRepresentSameParty(opp?.client || opp?.customer || opp?.customer_name || '', pendingCustomer));
        }) || null;
    }

    function resolveCustomerForCostingPayload(customerId: string, customerName: string) {
        const byId = customerId ? customers.find((c: any) => c.id === customerId) : null;
        if (byId) {
            return {
                customerId: byId.id,
                customerName: byId.business_name || customerName || '',
            };
        }

        const byName = findMatchingCustomerByName(customerName);

        return {
            customerId: byName?.id || customerId || '',
            customerName: byName?.business_name || customerName || '',
        };
    }

    function startNewCosting() {
        // Start a blank costing without opportunity
        resumeDraftRestoreForSession();
        selectedOpportunityId = '';
        selectedOpportunity = null;
        linkOpportunityId = '';
        sourceOfferId = '';
        sourceOfferNumber = '';
        showCostingForm = true;
        // Reset header
        header = createDefaultHeader();
        lineItems = [createNewLineItem()];
        quotationBody = defaultQuotationBody;
        attachmentScopeId = createDraftAttachmentScope();
        costingAttachments = [];
        isDirty = true;
        persistCostingDraft();
    }

    function linkBlankCostingToOpportunity() {
        const match = opportunities.find((opp: any) => String(opp.id) === String(linkOpportunityId));
        if (!match) {
            toast.warning('Select an opportunity to link this costing sheet.');
            return;
        }

        selectedOpportunityId = String(match.id);
        selectedOpportunity = match;

        const customerName = match.client || match.customer || match.customer_name || header.customerName || '';
        const projectName = match.project || match.project_name || match.title || '';
        const opportunityReference = getOpportunityUserReference(match);
        const opportunityFolder = getOpportunityFolderReference(match);
        const customerResolution = resolveCustomerForCostingPayload('', customerName);

        header = {
            ...header,
            customerId: customerResolution.customerId,
            customerName: customerResolution.customerName || customerName,
            rfqReference: opportunityReference,
            folderNumber: opportunityFolder || opportunityReference,
            costingId: opportunityReference || opportunityFolder || header.costingId,
            subject: header.subject || (projectName ? `Sub: ${projectName}` : header.subject),
            division: match.division || header.division || 'PH Trading',
            paymentTerms: header.paymentTerms || match.payment_terms || '30 days from Date of Delivery',
            deliveryTerms: header.deliveryTerms || match.delivery_terms || 'DAP Bahrain at your store or PH Trading',
        };

        linkOpportunityId = '';
        isDirty = true;
        persistCostingDraft();
        toast.success(`Linked costing to ${getOpportunityDisplayReference(match)}`);
    }

    function returnToCostingList() {
        if (isDirty) {
            persistCostingDraft();
            toast.info('Costing draft saved on this device. Resume it from the Costing page when needed.');
        }
        showBackWarning = false;
        showCostingForm = false;
        selectedOpportunityId = '';
        selectedOpportunity = null;
        selectedRevision = null;
        currentCostingId = null;
        rfqCostings = [];
        attachmentScopeId = '';
        costingAttachments = [];
        refreshStoredDraftState();
        if (hasStoredDraft) {
            pauseDraftRestoreForSession();
        }
        dispatch('navigate', { screen: 'opportunities', tab: 'costing' });
    }

    function handleBackToOpportunities() {
        if (isDirty) {
            persistCostingDraft();
            showBackWarning = true;
            return;
        }
        returnToCostingList();
    }

    function handleCurrencyChange(index: number) {
        const item = lineItems[index];
        const curr = currencyOptions.find(c => c.code === item.currency);
        if (curr) {
            item.exchangeRate = curr.rate;
            calculateLineItem(item);
            lineItems = [...lineItems];
        }
    }

    // Format currency
    function formatBHD(val: unknown): string {
        return toFiniteNumber(val).toLocaleString('en-US', {
            minimumFractionDigits: 3,
            maximumFractionDigits: 3
        }) + ' BHD';
    }

    function formatPercent(val: unknown): string {
        return toFiniteNumber(val).toFixed(0) + '%';
    }

    function formatNumber(val: unknown, decimals = 0): string {
        return toFiniteNumber(val).toLocaleString('en-US', {
            minimumFractionDigits: decimals,
            maximumFractionDigits: decimals
        });
    }

    function formatFileSize(bytes: unknown): string {
        const size = toFiniteNumber(bytes);
        if (size >= 1024 * 1024) {
            return `${(size / (1024 * 1024)).toFixed(1)} MB`;
        }
        if (size >= 1024) {
            return `${(size / 1024).toFixed(0)} KB`;
        }
        return `${size.toFixed(0)} B`;
    }

    function serialiseCostingAttachment(attachment: any, includeLocalPath = false) {
        const safeAttachment: any = {
            id: attachment.id,
            scope_id: attachment.scope_id,
            costing_number: attachment.costing_number,
            file_name: attachment.file_name,
            file_ext: attachment.file_ext,
            mime_type: attachment.mime_type,
            file_size: attachment.file_size,
            file_hash: attachment.file_hash,
            storage_mode: attachment.storage_mode || 'database',
            notes: attachment.notes,
            uploaded_by: attachment.uploaded_by,
            created_at: attachment.created_at,
        };
        if (includeLocalPath && safeAttachment.storage_mode === 'local_file') {
            safeAttachment.local_path = attachment.local_path;
        }
        return safeAttachment;
    }

    function serialiseCostingAttachments(includeLocalPath = false) {
        return costingAttachments.map((attachment) => serialiseCostingAttachment(attachment, includeLocalPath));
    }

    function sanitiseCostingForSyncedPersistence(data: any) {
        return {
            ...data,
            attachments: (data.attachments || [])
                .filter((attachment: any) => (attachment.storage_mode || 'database') !== 'local_file')
                .map((attachment: any) => serialiseCostingAttachment(attachment, false)),
        };
    }

    function createDraftAttachmentScope(): string {
        const randomPart = typeof crypto !== 'undefined' && 'randomUUID' in crypto
            ? crypto.randomUUID()
            : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
        return `draft-${randomPart}`;
    }

    function resolveAttachmentScope(fallbackPayload: any = null): string {
        const existing = String(
            attachmentScopeId ||
            fallbackPayload?.attachmentScopeId ||
            fallbackPayload?.attachment_scope_id ||
            ''
        ).trim();
        if (existing) return existing;

        const reference = String(
            header.costingId ||
            header.rfqReference ||
            header.folderNumber ||
            sourceOfferId ||
            selectedRevision?.id ||
            currentCostingId ||
            selectedOpportunityId ||
            ''
        ).trim();
        if (reference) {
            return `costing-${reference.replace(/[^A-Za-z0-9_.:-]+/g, '-')}`;
        }
        return createDraftAttachmentScope();
    }

    async function loadCostingAttachments(scope = attachmentScopeId) {
        const resolvedScope = String(scope || '').trim();
        if (!resolvedScope) {
            costingAttachments = [];
            return;
        }
        try {
            costingAttachments = await ListCostingSheetAttachments(resolvedScope) || [];
        } catch (err) {
            console.warn('Failed to load costing attachments:', err);
            costingAttachments = [];
        }
    }

    async function handleAttachDatasheet() {
        attachmentScopeId = resolveAttachmentScope();
        attachmentLoading = true;
        try {
            const attached = await AttachCostingSheetFileWithDialog(
                attachmentScopeId,
                header.costingId || header.rfqReference || header.folderNumber || sourceOfferNumber || 'Costing Sheet',
                header.customerName || selectedOpportunity?.customer_name || selectedOpportunity?.customer || '',
                ''
            );
            if (attached) {
                await loadCostingAttachments(attachmentScopeId);
                markDirty();
                toast.success(attached.storage_mode === 'local_file'
                    ? 'Large PDF stored locally and will be bundled when printing.'
                    : 'Technical datasheet attached.');
            }
        } catch (err) {
            toast.danger('Failed to attach datasheet: ' + String(err));
        } finally {
            attachmentLoading = false;
        }
    }

    async function handleOpenAttachment(attachmentId: string) {
        try {
            await OpenCostingSheetAttachment(attachmentId);
        } catch (err) {
            toast.danger('Failed to open attachment: ' + String(err));
        }
    }

    async function handleDeleteAttachment(attachmentId: string) {
        if (!attachmentId || attachmentLoading) return;
        attachmentLoading = true;
        try {
            await DeleteCostingSheetAttachment(attachmentId);
            costingAttachments = costingAttachments.filter((attachment) => attachment.id !== attachmentId);
            markDirty();
            toast.success('Attachment removed.');
        } catch (err) {
            toast.danger('Failed to remove attachment: ' + String(err));
        } finally {
            attachmentLoading = false;
        }
    }

    function normalisePersistedLineItem(item: any, index: number) {
        const rawMargin = typeof item.markupPercent === 'number'
            ? item.markupPercent
            : (typeof item.marginPercent === 'number' ? item.marginPercent : (item.margin_percent || 0));
        const marginPercent = rawMargin > 0 && rawMargin <= 1 ? rawMargin * 100 : rawMargin;
        const quantity = Number(item.quantity || 1) || 1;
        const userPrice = Number(item.userPrice ?? item.user_price ?? item.selling_price ?? 0) || 0;
        const suggestedPrice = Number(item.suggestedPrice ?? item.suggested_price ?? userPrice ?? 0) || 0;
        const fobForeign = Number(item.fob ?? item.unit_cost ?? 0) || 0;
        const freightPercent = Number(item.freightPercent ?? item.freight_percent ?? 0) || (
            fobForeign > 0 ? ((Number(item.freight ?? 0) || 0) / fobForeign) * 100 : defaultFreightPercent
        );

        return {
            ...createNewLineItem(),
            slNo: item.slNo || item.sl_no || index + 1,
            equipment: item.equipment || item.description || '',
            model: item.model || item.product_code || item.product_id || '',
            serialNumber: item.serialNumber || item.serial_number || '',
            longCode: item.longCode || item.long_code || '',
            specification: item.specification || '',
            detailedDescription: item.detailedDescription || item.detailed_description || '',
            currency: item.currency || 'BHD',
            quantity,
            fobForeign,
            freightPercent,
            freightForeign: Number(item.freight ?? 0) || 0,
            totalCost: Number(item.totalCost ?? item.total_cost ?? 0) || 0,
            marginPercent,
            suggestedPriceUnit: suggestedPrice || Number(item.suggestedPriceUnit ?? 0) || 0,
            totalSuggestedPrice: Number(item.totalPrice ?? item.total_price ?? (suggestedPrice * quantity)) || 0,
            exchangeRate: Number(item.exchangeRate ?? item.exchange_rate ?? 1) || 1,
            fobBHD: Number(item.fobBHD ?? item.fob_bhd ?? 0) || 0,
            freightBHD: Number(item.freightBHD ?? item.freight_bhd ?? 0) || 0,
            insurance: Number(item.insurance ?? 0) || 0,
            customsPercent: Number(item.customsPercent ?? item.customs_percent ?? 5) || 0,
            customsBHD: Number(item.customsBHD ?? item.customs_bhd ?? 0) || 0,
            handlingPercent: Number(item.handlingPercent ?? item.handling_percent ?? 4) || 0,
            handlingBHD: Number(item.handlingBHD ?? item.handling_bhd ?? 0) || 0,
            financePercent: Number(item.financePercent ?? item.finance_percent ?? 1) || 0,
            financeBHD: Number(item.financeBHD ?? item.finance_bhd ?? 0) || 0,
            otherCosts: Number(item.otherCosts ?? item.other_costs ?? 0) || 0,
            userPrice,
            userPriceSet: Boolean(item.userPriceSet ?? item.user_price_set ?? userPrice > 0),
        };
    }

    function restoreCostingPayload(revision: any, parsed: any) {
        const payload = Array.isArray(parsed) ? { lineItems: parsed } : (parsed || {});
        const persistedLineItems = Array.isArray(payload.lineItems) ? payload.lineItems : [];
        sourceOfferId = String(payload.offerId || payload.offer_id || '');
        sourceOfferNumber = String(payload.offerNumber || payload.offer_number || '');
        const resolvedCustomer = resolveCustomerForCostingPayload(
            payload.customerId || payload.customer_id || header.customerId,
            payload.customerName || payload.customer_name || header.customerName,
        );

        lineItems = persistedLineItems.length > 0
            ? persistedLineItems.map((item: any, index: number) => normalisePersistedLineItem(item, index))
            : [createNewLineItem()];

        header = {
            ...header,
            division: payload.division || header.division,
            date: payload.date || header.date,
            preparedBy: payload.preparedBy || revision.created_by || header.preparedBy,
            customerName: resolvedCustomer.customerName || header.customerName,
            contactPerson: payload.contactPerson || header.contactPerson,
            customerId: resolvedCustomer.customerId || header.customerId,
            rfqReference: payload.rfqReference || header.rfqReference,
            folderNumber: payload.folderNumber || header.folderNumber,
            costingId: payload.costingId || header.costingId,
            subject: payload.subject || header.subject,
            estDelivery: payload.estDelivery || header.estDelivery,
            deliveryTerms: normaliseDeliveryTermsForDivision(payload.deliveryTerms || header.deliveryTerms, payload.division || header.division),
            paymentTerms: payload.paymentTerms || header.paymentTerms,
            orderType: payload.orderType || header.orderType,
            countryOfOrigin: payload.countryOfOrigin || header.countryOfOrigin,
            cocCoo: payload.cocCoo || header.cocCoo,
            testCertificate: payload.testCertificate || header.testCertificate,
            installation: payload.installation || header.installation,
            commissioning: payload.commissioning || header.commissioning,
            testing: payload.testing || header.testing,
            placeOfSupply: payload.placeOfSupply || header.placeOfSupply,
            taxCategory: payload.taxCategory || header.taxCategory,
            customerTRN: payload.customerTRN || header.customerTRN,
            quoteType: payload.quoteType || header.quoteType,
        };

        discount = Number(payload.discount ?? 0) || 0;
        hiddenCharges = Number(payload.hiddenCharges ?? 0) || 0;
        vatRate = Number(payload.vatRate ?? vatRate) || vatRate;
        quotationBody = payload.body || defaultQuotationBody;
        termsAndConditions = payload.termsAndConditions || termsAndConditions;
        currentCostingId = revision.id || null;
        attachmentScopeId = resolveAttachmentScope(payload);
        costingAttachments = Array.isArray(payload.attachments) ? payload.attachments : [];
        void loadCostingAttachments(attachmentScopeId);
        recalculateLineItems();
        isDirty = false;
    }

    function restoreOfferCostingPayload(payload: any) {
        selectedOpportunityId = '';
        selectedOpportunity = null;
        rfqCostings = [];
        selectedRevision = null;
        currentCostingId = null;
        restoreCostingPayload({ id: null, created_by: payload.preparedBy || '' }, payload);
        showCostingForm = true;
        markDirty();
        toast.success(`Loaded ${payload.offerNumber || 'offer'} into the costing sheet`);
    }

    // ========================================
    // EXPORT FUNCTIONS
    // ========================================
    function buildExportData() {
        recalculateLineItems();
        const opportunityReference = getOpportunityUserReference(selectedOpportunity);
        const rfqReference = header.rfqReference || opportunityReference;
        const folderNumber = header.folderNumber || getOpportunityFolderReference(selectedOpportunity);
        const costingId = header.costingId || rfqReference || folderNumber;
        attachmentScopeId = resolveAttachmentScope();

        return {
            division: header.division,
            source: sourceOfferId ? 'offer' : '',
            offerId: sourceOfferId,
            offerNumber: sourceOfferNumber,
            date: header.date,
            preparedBy: header.preparedBy,
            customerId: header.customerId,
            customerName: header.customerName,
            contactPerson: header.contactPerson,
            rfqReference,
            folderNumber,
            costingId,
            subject: header.subject,
            estDelivery: header.estDelivery,
            deliveryTerms: normaliseDeliveryTermsForDivision(header.deliveryTerms, header.division),
            paymentTerms: header.paymentTerms,
            orderType: header.orderType,
            countryOfOrigin: header.countryOfOrigin,
            cocCoo: header.cocCoo,
            testCertificate: header.testCertificate,
            installation: header.installation,
            commissioning: header.commissioning,
            testing: header.testing,
            lineItems: getValidLineItems().map((item, i) => {
                const quantity = Math.max(1, toFiniteNumber(item.quantity, 1));
                const userPrice = nonNegativeNumber(item.userPrice);
                const suggestedPriceUnit = nonNegativeNumber(item.suggestedPriceUnit);
                const effectivePrice = item.userPriceSet && userPrice > 0
                    ? userPrice
                    : suggestedPriceUnit;
                return {
                    slNo: i + 1,
                    // supplier removed - invoices link to suppliers instead
                    supplier: '',
                    equipment: item.equipment,
                    model: item.model,
                    serialNumber: item.serialNumber || '',
                    longCode: item.longCode || '',
                    specification: item.specification || '',
                    detailedDescription: item.detailedDescription || '',
                    currency: item.currency,
                    quantity,
                    fob: nonNegativeNumber(item.fobForeign),
                    freight: nonNegativeNumber(item.freightForeign),
                    freightPercent: nonNegativeNumber(item.freightPercent),
                    totalCost: nonNegativeNumber(item.totalCost),
                    marginPercent: nonNegativeNumber(item.marginPercent),
                    markupPercent: nonNegativeNumber(item.marginPercent),
                    suggestedPrice: effectivePrice,
                    totalPrice: effectivePrice * quantity,
                    // Full cost breakdown for persistence
                    exchangeRate: toFiniteNumber(item.exchangeRate, 1),
                    fobBHD: nonNegativeNumber(item.fobBHD),
                    freightBHD: nonNegativeNumber(item.freightBHD),
                    insurance: nonNegativeNumber(item.insurance),
                    customsPercent: nonNegativeNumber(item.customsPercent),
                    customsBHD: nonNegativeNumber(item.customsBHD),
                    handlingPercent: nonNegativeNumber(item.handlingPercent),
                    handlingBHD: nonNegativeNumber(item.handlingBHD),
                    financePercent: nonNegativeNumber(item.financePercent),
                    financeBHD: nonNegativeNumber(item.financeBHD),
                    otherCosts: nonNegativeNumber(item.otherCosts),
                    userPrice,
                    userPriceSet: item.userPriceSet && userPrice > 0,
                };
            }),
            subtotal: toFiniteNumber(subtotal),
            discount: effectiveDiscount,
            netAmount: toFiniteNumber(netAmount),
            vat: toFiniteNumber(vat),
            grandTotal: toFiniteNumber(grandTotal),
            totalCost: toFiniteNumber(totalCost),
            profit: toFiniteNumber(profit),
            profitPercent: toFiniteNumber(profitPercent),
            opportunityId: getSelectedRFQId(),
            opportunityRecordId: getSelectedOpportunityRecordId(),
            projectName: selectedOpportunity?.title || selectedOpportunity?.project_name || selectedOpportunity?.project || '',
            body: quotationBody,
            termsAndConditions: termsAndConditions,
            attachmentScopeId,
            attachments: serialiseCostingAttachments(true),
            quoteType: header.quoteType || 'Quotation',
            vatRate: effectiveVatRate,
            hiddenCharges: effectiveHiddenCharges,
            // VAT/Tax compliance
            placeOfSupply: header.placeOfSupply || 'Kingdom of Bahrain',
            taxCategory: header.taxCategory || 'Standard',
            customerTRN: header.customerTRN || '',
        };
    }

    async function handleExportPDF() {
        const validItems = getValidLineItems();
        if (validItems.length === 0) {
            toast.warning('Please add at least one line item with equipment or price');
            return;
        }

        exporting = true;
        try {
            const data = buildExportData();
            const filePath = await ExportCostingToPDF(data as any);
            if (filePath) {
                toast.success('PDF quotation generated!');
                await OpenExportedFile(filePath);
            }
        } catch (err) {
            console.error('Export PDF failed:', err);
            toast.danger('Failed to generate PDF: ' + String(err));
        } finally {
            exporting = false;
        }
    }

    async function handleExportExcel() {
        const validItems = getValidLineItems();
        if (validItems.length === 0) {
            toast.warning('Please add at least one line item with equipment or price');
            return;
        }

        exporting = true;
        try {
            const data = buildExportData();
            const filePath = await ExportCostingToExcel(data as any);
            if (filePath) {
                toast.success('Excel costing sheet generated!');
                await OpenExportedFile(filePath);
            }
        } catch (err) {
            console.error('Export Excel failed:', err);
            toast.danger('Failed to generate Excel: ' + String(err));
        } finally {
            exporting = false;
        }
    }

    let savingOffer = false;
    let currentCostingId: number | null = null;

    async function handleSaveAsOffer() {
        if (savingOffer) return; // Prevent double-click

        const validItems = getValidLineItems();
        if (validItems.length === 0) {
            toast.warning('Please add at least one line item with equipment or price');
            return;
        }

        savingOffer = true;
        try {
            const persistedCosting = buildExportData();

            // Save costing sheet to DB (links back to RFQ)
            const rfqId = getSelectedRFQId();
            if (rfqId > 0) {
                try {
                    if (selectedRevision?.id || currentCostingId) {
                        await UpdateCostingSheet(selectedRevision?.id || currentCostingId, {
                            ...selectedRevision,
                            items: JSON.stringify(sanitiseCostingForSyncedPersistence(persistedCosting)),
                            created_by: header.preparedBy || 'System',
                        } as any);
                    } else {
                        const saved = await CreateCostingSheet(rfqId, JSON.stringify(sanitiseCostingForSyncedPersistence(persistedCosting)), header.preparedBy || 'System');
                        currentCostingId = saved?.id || null;
                    }
                } catch (err) {
                    console.warn('Costing sheet save warning:', err);
                    // Non-blocking - continue to save offer even if costing sheet fails
                }
            }

            // Save as offer
            const data = buildExportData();
            const offer = await SaveCostingAsOffer(data as any);
            if (offer) {
                const actionText = sourceOfferId ? 'updated' : 'created';
                toast.success(`Offer ${offer.offer_number} ${actionText}! View it in Sales Hub - Offers`);

                // Refresh recent sheets list
                costingSheets = await GetCostingSheets(20).catch(() => []);

                showCostingForm = false;
                selectedOpportunityId = '';
                selectedOpportunity = null;
                selectedRevision = null;
                currentCostingId = null;
                sourceOfferId = '';
                sourceOfferNumber = '';
                isDirty = false;
                clearCostingDraft();
                dispatch('navigate', { screen: 'opportunities', tab: 'offers' });
            }
        } catch (err) {
            console.error('Save as offer failed:', err);
            toast.danger('Failed to save offer: ' + String(err));
        } finally {
            savingOffer = false;
        }
    }

    onMount(() => {
        window.addEventListener('beforeunload', handleBeforeUnload);
        if (!canView) {
            loading = false;
        }
    });

    onDestroy(() => {
        window.removeEventListener('beforeunload', handleBeforeUnload);
    });

    $: if (canView && active && !hasLoadedInitialData) {
        hasLoadedInitialData = true;
        void loadData();
    }
</script>

<div class="costing-page" class:embedded>
    <header class="page-header">
        <div>
            <h1>Costing Sheet</h1>
            <p class="subtitle">PH Trading W.L.L - Pricing Calculator</p>
        </div>
        <div class="header-actions">
            <Button variant="secondary" on:click={loadData}>Refresh</Button>
            {#if showCostingForm}
                <Button variant="secondary" on:click={handleExportExcel} disabled={exporting}>
                    {exporting ? 'Exporting...' : 'Export Excel'}
                </Button>
                <Button variant="secondary" on:click={handleExportPDF} disabled={exporting}>
                    {exporting ? 'Exporting...' : 'Export PDF'}
                </Button>
                <Button variant="primary" on:click={handleSaveAsOffer} disabled={savingOffer}>
                    {savingOffer ? 'Saving...' : 'Save as Offer'}
                </Button>
            {/if}
        </div>
    </header>

    {#if loading}
        <div class="loading-state">
            <WabiSpinner size="lg" />
            <p>Loading data...</p>
        </div>
    {:else}
        <!-- Opportunity Selection Section -->
        {#if !showCostingForm}
            <Card title="Select Opportunity" variant="elevated">
                <div class="opportunity-selector">
                    <p class="selector-description">
                        Select an opportunity to create a costing sheet. Customer details will be pre-filled automatically.
                    </p>
                    {#if hasStoredDraft}
                        <div class="draft-recovery-panel">
                            <div>
                                <span class="field-label">Saved Draft</span>
                                <p>Unsaved costing draft saved on this device.</p>
                            </div>
                            <div class="draft-recovery-actions">
                                <Button variant="secondary" size="sm" on:click={resumeStoredCostingDraft}>
                                    Resume Draft
                                </Button>
                                <Button variant="danger" size="sm" on:click={discardStoredCostingDraft}>
                                    Discard
                                </Button>
                            </div>
                        </div>
                    {/if}
                    <div class="selector-grid">
                        <div class="form-group selector-dropdown">
                            <span class="field-label">Opportunity / RFQ</span>
                            <select bind:value={selectedOpportunityId} on:change={handleOpportunitySelect} class="input">
                                <option value="">-- Select an Opportunity --</option>
                                {#each opportunities as opp}
                                    <option value={String(opp.id)}>
                                        {getOpportunityDisplayReference(opp)} • {opp.client || opp.customer || 'Unknown'} - {opp.project || opp.project_name || 'New RFQ'} ({opp.status})
                                    </option>
                                {/each}
                            </select>
                        </div>
                        <div class="selector-divider">
                            <span>OR</span>
                        </div>
                        <Button variant="secondary" on:click={startNewCosting}>
                            Start Blank Costing
                        </Button>
                    </div>

                    <!-- Opportunities List Preview -->
                    {#if opportunities.length > 0}
                        <div class="opp-preview-list">
                            <h4>Recent Opportunities</h4>
                            {#each previewOpportunities as opp}
                                <button
                                    class="opp-preview-item"
                                    class:selected={selectedOpportunityId === String(opp.id)}
                                    on:click={() => { selectedOpportunityId = String(opp.id); handleOpportunitySelect(); }}
                                >
                                    <div class="opp-preview-header">
                                        <span class="opp-customer">{opp.client || opp.customer || 'Unknown'}</span>
                                        <span class="opp-status badge badge-{opp.status?.toLowerCase()}">{opp.status || 'New'}</span>
                                    </div>
                                    <div class="opp-preview-body">
                                        <span class="opp-project-ref">{getOpportunityDisplayReference(opp)}</span>
                                        <span class="opp-project">{opp.project || opp.project_name || 'New Requirements'}</span>
                                    </div>
                                    <div class="opp-preview-footer">
                                        <span class="opp-value" class:pending={getOpportunityValue(opp) <= 0}>{formatOpportunityValue(opp)}</span>
                                        <span class="opp-date">{opp.created_at ? new Date(opp.created_at).toLocaleDateString() : ''}</span>
                                    </div>
                                </button>
                            {/each}
                        </div>
                    {:else}
                        <div class="empty-opportunities">
                            <p>No opportunities found. Create an opportunity in the Sales Hub first, or start a blank costing.</p>
                        </div>
                    {/if}
                </div>
            </Card>
        {:else}
            <!-- Back Button and Selected Opportunity Info -->
            <div class="selected-opportunity-bar">
                <Button variant="ghost" on:click={handleBackToOpportunities}>
                    Back to Opportunities
                </Button>
                {#if selectedOpportunity}
                    <div class="selected-info">
                        <span class="label">Linked Opportunity:</span>
                        <span class="reference">{getOpportunityDisplayReference(selectedOpportunity)}</span>
                        <span class="customer">{selectedOpportunity.client || selectedOpportunity.customer}</span>
                        <span class="project">- {selectedOpportunity.project || selectedOpportunity.project_name}</span>
                        <span class="badge badge-{selectedOpportunity.status?.toLowerCase()}">{selectedOpportunity.status}</span>
                    </div>
                {:else}
                    <div class="selected-info">
                        <span class="label">Blank Costing</span>
                        <span class="hint">(not linked to an opportunity)</span>
                    </div>
                    <div class="link-opportunity-control">
                        <select bind:value={linkOpportunityId} class="input-sm" aria-label="Link blank costing to opportunity">
                            <option value="">Link opportunity...</option>
                            {#each opportunities as opp}
                                <option value={String(opp.id)}>
                                    {getOpportunityDisplayReference(opp)} • {opp.client || opp.customer || 'Unknown'} - {opp.project || opp.project_name || 'New RFQ'}
                                </option>
                            {/each}
                        </select>
                        <Button variant="secondary" size="sm" on:click={linkBlankCostingToOpportunity} disabled={!linkOpportunityId}>
                            Link Opportunity
                        </Button>
                    </div>
                {/if}
            </div>

            {#if showBackWarning}
                <div
                    class="modal-backdrop costing-backdrop"
                    role="button"
                    tabindex="0"
                    on:click|self={() => (showBackWarning = false)}
                    on:keydown={(event) => {
                        if (event.currentTarget !== event.target) return;
                        if (event.key === 'Escape') {
                            event.preventDefault();
                            showBackWarning = false;
                        }
                    }}
                >
                    <section class="confirm-card" role="dialog" aria-modal="true" aria-labelledby="costing-back-title">
                        <span class="confirm-kicker">Unsaved Changes</span>
                        <h2 id="costing-back-title">Leave this costing sheet?</h2>
                        <p>
                            Your changes have not been saved as an offer yet. A local draft will be kept on this device so it can be resumed from the Costing page.
                        </p>
                        <div class="confirm-actions">
                            <Button variant="secondary" on:click={() => (showBackWarning = false)}>
                                Stay on Sheet
                            </Button>
                            <Button variant="warning" on:click={returnToCostingList}>
                                Keep Draft & Go Back
                            </Button>
                        </div>
                    </section>
                </div>
            {/if}

            <!-- Revision Selector (Feature D - only show if RFQ selected) -->
            {#if isRFQOpportunity(selectedOpportunity) && rfqCostings.length > 0}
                <Card title="Costing Revisions">
                    <div class="revision-list">
                        {#each rfqCostings as rev}
                            <div
                                class="revision-item"
                                class:active={rev.is_active}
                                class:selected={selectedRevision?.id === rev.id}
                                on:click={() => selectRevision(rev)}
                                on:keydown={(event) => (event.key === 'Enter' || event.key === ' ') && selectRevision(rev)}
                                role="button"
                                tabindex="0"
                            >
                                <div class="rev-header">
                                    <span class="rev-number">Rev {rev.revision_number}</span>
                                    {#if rev.is_active}
                                        <span class="badge badge-success">Current</span>
                                    {/if}
                                    <span class="rev-status badge">{rev.status}</span>
                                </div>
                                <div class="rev-details">
                                    <span>{formatDate(rev.created_at)}</span>
                                    <span>by {rev.created_by}</span>
                                    <span>{formatBHD(rev.final_price || 0)}</span>
                                </div>
                            </div>
                        {/each}
                    </div>

                    <div class="revision-actions">
                        <Button
                            variant="secondary"
                            size="sm"
                            on:click={handleCreateNewRevision}
                            title="Create a new revision based on the current one"
                        >
                            + New Revision
                        </Button>

                        {#if selectedRevision && !selectedRevision.is_active}
                            <Button
                                variant="primary"
                                size="sm"
                                on:click={() => handleSetActiveRevision(selectedRevision.id)}
                            >
                                Make Current
                            </Button>
                        {/if}
                    </div>
                </Card>
            {/if}

        <div class="costing-layout" on:input={markDirty} on:change={markDirty}>
            <!-- Left: Costing Form -->
            <div class="main-form">
                <!-- Header Section -->
                <Card>
                    <!-- Row 1: Core fields always visible -->
                    <div class="header-row-3col tight">
                        <div class="field-group">
                            <span class="field-label">Customer *</span>
                            <select bind:value={header.customerId} on:change={handleCustomerChange} class="input">
                                <option value="">Select customer...</option>
                                {#each customers as c}
                                    <option value={c.id}>{c.business_name}</option>
                                {/each}
                            </select>
                        </div>
                        <div class="field-group">
                            <span class="field-label">Contact Person</span>
                            <input type="text" bind:value={header.contactPerson} class="input" placeholder="Name" />
                        </div>
                        <div class="field-group">
                            <span class="field-label">RFQ Reference</span>
                            <input type="text" bind:value={header.rfqReference} class="input" placeholder="RFQ / Enquiry ref" />
                        </div>
                    </div>

                    <!-- Row 2: Document + commercial -->
                    <div class="header-row-6col tight">
                        <div class="field-group">
                            <span class="field-label">Division</span>
                            <select bind:value={header.division} class="input-sm">
                                {#each divisionOptions as div}<option value={div}>{div}</option>{/each}
                            </select>
                        </div>
                        <div class="field-group">
                            <span class="field-label">Date</span>
                            <input type="date" bind:value={header.date} class="input-sm" />
                        </div>
                        <div class="field-group">
                            <span class="field-label">Prepared By</span>
                            <input type="text" value={header.preparedBy || currentPreparedByName || 'System'} class="input-sm readonly-input" aria-label="Prepared By" readonly />
                        </div>
                        <div class="field-group">
                            <span class="field-label">Doc Type</span>
                            <select bind:value={header.quoteType} class="input-sm">
                                <option value="Quotation">Quotation</option>
                                <option value="Budgetary Quote">Budgetary Quote</option>
                                <option value="Budgetary Estimate">Budgetary Est.</option>
                                <option value="Technical Offer">Technical</option>
                                <option value="Commercial Offer">Commercial</option>
                            </select>
                        </div>
                        <div class="field-group">
                            <span class="field-label">Folder No.</span>
                            <input type="text" bind:value={header.folderNumber} class="input-sm" placeholder="Enter folder no." />
                        </div>
                        <div class="field-group">
                            <span class="field-label">Costing ID</span>
                            <input type="text" bind:value={header.costingId} class="input-sm" placeholder="Enter costing ID" />
                        </div>
                    </div>

                    <!-- Row 3: Terms (always visible - needed for every quote) -->
                    <div class="header-row-3col tight">
                        <div class="field-group">
                            <span class="field-label">Payment Terms</span>
                            <select bind:value={header.paymentTerms} class="input-sm">
                                {#each paymentTermsOptions as opt}<option value={opt}>{opt}</option>{/each}
                            </select>
                        </div>
                        <div class="field-group">
                            <span class="field-label">Delivery Terms</span>
                            <select bind:value={header.deliveryTerms} class="input-sm">
                                {#each deliveryTermsOptions as opt}<option value={opt}>{opt}</option>{/each}
                            </select>
                        </div>
                        <div class="field-group">
                            <span class="field-label">Est. Delivery</span>
                            <select bind:value={header.estDelivery} class="input-sm">
                                {#each deliveryOptions as opt}<option value={opt}>{opt}</option>{/each}
                            </select>
                        </div>
                    </div>
                    <div class="header-row-3col tight">
                        <div class="field-group full-span">
                            <span class="field-label">Subject</span>
                            <input type="text" bind:value={header.subject} class="input-sm" placeholder="Subject line for customer PDF" />
                        </div>
                    </div>
                    <div class="header-row-3col tight">
                        <div class="field-group full-span">
                            <span class="field-label">PDF Body</span>
                            <textarea bind:value={quotationBody} class="input-sm textarea-sm" rows="4" placeholder="Opening body / cover note shown before the line items"></textarea>
                        </div>
                    </div>

                    <div class="technical-attachments">
                        <div class="attachments-header">
                            <div>
                                <span class="field-label">Technical Datasheets</span>
                            </div>
                            <Button
                                variant="secondary"
                                size="sm"
                                on:click={handleAttachDatasheet}
                                disabled={attachmentLoading}
                                aria-label="Attach PDF, JPG, JPEG, or PNG datasheet"
                            >
                                {attachmentLoading ? 'Attaching...' : '+ Attach File'}
                            </Button>
                        </div>
                        {#if costingAttachments.length > 0}
                            <div class="attachment-list">
                                {#each costingAttachments as attachment}
                                    <div class="attachment-row">
                                        <div>
                                            <strong>{attachment.file_name}</strong>
                                            <span>{(attachment.storage_mode || 'database') === 'local_file' ? 'LOCAL PDF' : String(attachment.file_ext || '').toUpperCase()} · {formatFileSize(attachment.file_size)}</span>
                                        </div>
                                        <div class="attachment-actions">
                                            <button on:click={() => handleOpenAttachment(attachment.id)}>Open</button>
                                            <button class="danger" on:click={() => handleDeleteAttachment(attachment.id)} disabled={attachmentLoading}>Remove</button>
                                        </div>
                                    </div>
                                {/each}
                            </div>
                        {:else}
                            <p class="attachments-empty">No datasheets attached yet.</p>
                        {/if}
                    </div>

                    <!-- Expandable: Compliance & certificates -->
                    <button class="expand-toggle" on:click={() => showAdvancedHeader = !showAdvancedHeader}>
                        {showAdvancedHeader ? '▾' : '▸'} Compliance, Certificates & VAT
                    </button>

                    {#if showAdvancedHeader}
                    <div class="advanced-section">
                        <div class="header-row-6col tight">
                            <div class="field-group">
                                <span class="field-label">Order Type</span>
                                <select bind:value={header.orderType} class="input-sm">
                                    {#each orderTypeOptions as opt}<option value={opt}>{opt}</option>{/each}
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Origin</span>
                                <select bind:value={header.countryOfOrigin} class="input-sm">
                                    {#each countryOptions as opt}<option value={opt.code}>{opt.code}</option>{/each}
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">COC/COO</span>
                                <select bind:value={header.cocCoo} class="input-sm">
                                    {#each certificateOptions as opt}<option value={opt}>{opt}</option>{/each}
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Test Cert</span>
                                <select bind:value={header.testCertificate} class="input-sm">
                                    {#each certificateOptions as opt}<option value={opt}>{opt}</option>{/each}
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Install</span>
                                <select bind:value={header.installation} class="input-sm">
                                    <option value="Yes">Yes</option><option value="No">No</option>
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Commission</span>
                                <select bind:value={header.commissioning} class="input-sm">
                                    <option value="Yes">Yes</option><option value="No">No</option>
                                </select>
                            </div>
                        </div>
                        <div class="header-row-3col tight">
                            <div class="field-group">
                                <span class="field-label">Place of Supply</span>
                                <select bind:value={header.placeOfSupply} class="input-sm">
                                    <option value="Kingdom of Bahrain">Kingdom of Bahrain</option>
                                    <option value="GCC">GCC Member State</option>
                                    <option value="Export">Export (Outside GCC)</option>
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Tax Category</span>
                                <select bind:value={header.taxCategory} class="input-sm">
                                    <option value="Standard">Standard Rate (10%)</option>
                                    <option value="Zero-rated">Zero-rated</option>
                                    <option value="Exempt">Exempt</option>
                                    <option value="Out-of-scope">Out of Scope</option>
                                </select>
                            </div>
                            <div class="field-group">
                                <span class="field-label">Customer TRN</span>
                                <input type="text" bind:value={header.customerTRN} class="input-sm" placeholder="Tax Reg. Number" />
                            </div>
                        </div>
                    </div>
                    {/if}
                </Card>

                <!-- Line Items Section -->
                <Card title="Line Items">

                    {#each lineItems as item, i}
                        <div class="line-item-group" class:alt-row={i % 2 === 1}>
                        <!-- Line 1: Product identification -->
                        <div class="line-row-top">
                            <span class="sl-no">{i + 1}</span>
                            <div class="field-inline">
                                <span class="field-label">Equipment</span>
                                <input type="text" bind:value={item.equipment} class="input-sm" placeholder="Product name" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Model</span>
                                <input type="text" bind:value={item.model} class="input-sm" placeholder="Model #" />
                            </div>
                            <div class="field-inline compact">
                                <span class="field-label">Qty</span>
                                <input type="number" bind:value={item.quantity} class="input-sm qty" min="1" on:change={calculateAll} />
                            </div>
                            <div class="field-inline compact">
                                <span class="field-label">Currency</span>
                                <select bind:value={item.currency} class="input-sm" on:change={() => handleCurrencyChange(i)}>
                                    {#each currencyOptions as c}
                                        <option value={c.code}>{c.code}</option>
                                    {/each}
                                </select>
                            </div>
                            <div class="field-inline exchange-rate">
                                <span class="field-label">Exch. Rate</span>
                                <input type="number" bind:value={item.exchangeRate} class="input-sm money" step="0.0001" min="0.0001" on:change={calculateAll} />
                            </div>
                            {#if lineItems.length > 1}
                                <button class="btn-remove" on:click={() => removeLineItem(i)} title="Remove item" aria-label={`Remove line item ${i + 1}`}>×</button>
                            {:else}
                                <span class="remove-slot" aria-hidden="true"></span>
                            {/if}
                        </div>

                        <!-- Line 2: Pricing -->
                        <div class="line-row-pricing">
                            <div class="field-inline">
                                <span class="field-label">Unit Price (FOB)</span>
                                <input type="number" bind:value={item.fobForeign} class="input-sm money" step="0.001" on:change={calculateAll} placeholder="0.000" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Freight %</span>
                                <input type="number" bind:value={item.freightPercent} class="input-sm money" step="0.01" min="0" on:input={() => isDirty = true} on:change={calculateAll} placeholder="0.00" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Misc Cost</span>
                                <input type="number" bind:value={item.otherCosts} class="input-sm money" step="0.001" min="0" on:change={calculateAll} placeholder="0.000" title="Internal extra cost added before margin" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Quote Price / Unit (BHD)</span>
                                <input
                                    type="number"
                                    class="input-sm money sell-price"
                                    class:user-overridden={item.userPriceSet && item.userPrice > 0}
                                    step="0.001"
                                    placeholder={`Auto ${formatNumber(item.suggestedPriceUnit, 0)}`}
                                    bind:value={item.userPrice}
                                    on:input={() => { item.userPriceSet = nonNegativeNumber(item.userPrice) > 0; calculateAll(); }}
                                    title={`Optional unit selling price override. Leave blank or 0 to use the suggested price: ${formatBHD(item.suggestedPriceUnit)}`}
                                />
                            </div>
                            <div class="line-total">
                                <span class="field-label">Total (BHD)</span>
                                <span class="calc-value highlight">{formatBHD(item.totalSuggestedPrice)}</span>
                            </div>
                        </div>

                        <!-- Order Code - full width input for long supplier order/configuration codes -->
                        <div class="long-code-row">
                            <span class="long-code-label">Long Code:</span>
                            <input
                                type="text"
                                bind:value={item.longCode}
                                class="long-code-input"
                                placeholder="e.g. FMU90-R11CA131AA3A  /  71452626+Z1C5+Z1E3+Z1F1+Z1G1..."
                            />
                        </div>

                        <!-- Wide Detailed Description Field - for instrumentation specifications -->
                        <div class="detailed-description-row">
                            <textarea
                                bind:value={item.detailedDescription}
                                class="detailed-description-input"
                                rows="2"
                                placeholder="Detailed specs, approvals, HS codes, country of origin... (prints under line item in PDF)"
                            ></textarea>
                        </div>

                        <!-- Expanded cost breakdown -->
                        <div class="cost-breakdown">
                            <div class="cost-row">
                                <span class="cost-label">Rate: {item.exchangeRate}</span>
                                <span class="cost-label">FOB: {formatBHD(item.fobBHD)}</span>
                                <span class="cost-label">Freight %: {formatPercent(item.freightPercent)}</span>
                                <span class="cost-label">Freight: {formatBHD(item.freightBHD)}</span>
                                <span class="cost-label">C&F: {formatBHD(item.cf)}</span>
                            </div>
                            <div class="cost-row">
                                <div class="cost-input">
                                    <span class="field-label">Customs %</span>
                                    <input type="number" bind:value={item.customsPercent} class="input-xs" on:change={calculateAll} />
                                </div>
                                <div class="cost-input">
                                    <span class="field-label">Handling %</span>
                                    <input type="number" bind:value={item.handlingPercent} class="input-xs" on:change={calculateAll} />
                                </div>
                                <div class="cost-input">
                                    <span class="field-label">Finance %</span>
                                    <input type="number" bind:value={item.financePercent} class="input-xs" on:change={calculateAll} />
                                </div>
                                <div class="cost-input">
                                    <span class="field-label">Markup %</span>
                                    <input type="number" bind:value={item.marginPercent} class="input-xs" min="0" max="99" on:change={calculateAll} />
                                </div>
                                <div class="cost-input">
                                    <span class="field-label">Total Cost</span>
                                    <span class="cost-value">{formatBHD(item.totalCost)}</span>
                                </div>
                                <!-- Issue #7: Copy costs button - only on first item -->
                                {#if i === 0 && lineItems.length > 1}
                                    <button
                                        class="btn-copy-costs"
                                        on:click={copyFirstItemCostsToAll}
                                        title="Copy customs, handling, finance, and margin to all items below"
                                    >
                                        Copy to All
                                    </button>
                                {/if}
                            </div>
                            <div class="cost-row">
                                <span class="cost-label">Customs: {formatBHD(item.customsBHD)}</span>
                                <span class="cost-label">Landed: {formatBHD(item.landedCost)}</span>
                                <span class="cost-label">Handling: {formatBHD(item.handlingBHD)}</span>
                                <span class="cost-label">Markup: {formatBHD(item.marginBHD)}</span>
                                <span class="cost-label suggested-highlight">Suggested: {formatBHD(item.suggestedPriceUnit)}</span>
                            </div>
                        </div>
                        </div>
                    {/each}

                    <div class="add-item-row">
                        <Button variant="secondary" size="sm" on:click={addLineItem}>+ Add Line Item</Button>
                        <span class="item-count">{lineItems.length}/10 items</span>
                    </div>
                </Card>
            </div>

            <!-- Right: Summary Panel -->
            <div class="summary-panel">
                <Card title="Summary" variant="elevated">
                    <div class="summary-rows">
                        <div class="summary-row">
                            <span>Subtotal</span>
                            <span class="money">{formatBHD(subtotal)}</span>
                        </div>
                        <div class="summary-row">
                            <span>Discount</span>
                            <input type="number" bind:value={discount} class="input-sm money" step="0.001" min="0" on:input={() => isDirty = true} on:change={calculateAll} />
                        </div>
                        <div class="summary-row">
                            <span title="Additional charges not shown on quotation">Hidden Charges</span>
                            <input type="number" bind:value={hiddenCharges} class="input-sm money" step="0.001" min="0" on:input={() => isDirty = true} />
                        </div>
                        <div class="summary-row vat-row">
                            <div class="vat-control">
                                <span>VAT</span>
                                <input type="number" bind:value={vatRate} class="vat-input" min="0" max="100" step="0.5" on:input={() => isDirty = true} />
                                <span>%</span>
                            </div>
                            <span class="money">{formatBHD(vat)}</span>
                        </div>
                        <div class="summary-row total">
                            <span>Grand Total</span>
                            <span class="money">{formatBHD(grandTotal)}</span>
                        </div>
                    </div>

                    <div class="profit-section">
                        <h4>Profit Analysis</h4>
                        <div class="profit-grid">
                            <div class="profit-item">
                                <span class="label">PO Expected</span>
                                <span class="value">{formatBHD(netAmount)}</span>
                            </div>
                            <div class="profit-item">
                                <span class="label">PH Cost</span>
                                <span class="value">{formatBHD(totalCost)}</span>
                            </div>
                            <div class="profit-item muted">
                                <span class="label">Internal Hidden Charges</span>
                                <span class="value">{formatBHD(effectiveHiddenCharges)}</span>
                            </div>
                            <div class="profit-item highlight">
                                <span class="label">Profit</span>
                                <span class="value">{formatBHD(profit)}</span>
                            </div>
                            <div class="profit-item highlight">
                                <span class="label">Profit %</span>
                                <span class="value">{formatPercent(profitPercent)}</span>
                            </div>
                        </div>
                    </div>
                </Card>

                <!-- Recent Sheets -->
                <Card title="Recent Sheets">
                    <div class="recent-list">
                        {#if costingSheets.length === 0}
                            <p class="empty">No recent costing sheets</p>
                        {:else}
                            {#each costingSheets.slice(0, 5) as sheet}
                                <div class="recent-item">
                                    <span class="ref">{sheet.ReferenceNo || 'N/A'}</span>
                                    <span class="customer">{sheet.CustomerName || '-'}</span>
                                    <span class="amount">{formatBHD(sheet.TotalSellBHD || 0)}</span>
                                </div>
                            {/each}
                        {/if}
                    </div>
                </Card>

                <!-- Terms & Conditions -->
                <Card title="Terms & Conditions">
                    <div class="tc-section">
                        <p class="tc-note">Printed on a separate page in PDF exports</p>
                        <textarea
                            class="tc-textarea"
                            bind:value={termsAndConditions}
                            rows="8"
                            placeholder="Enter terms and conditions..."
                        ></textarea>
                    </div>
                </Card>
            </div>
        </div>
        {/if}
    {/if}
</div>

<style>
    .costing-page {
        padding: var(--page-padding);
        height: 100%;
        display: flex;
        flex-direction: column;
        background: var(--bg-base);
        overflow-y: auto;
    }

    .costing-page.embedded {
        padding: 0;
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: var(--spacing-lg);
        padding-bottom: var(--spacing-md);
        border-bottom: 1px solid var(--border);
    }

    .page-header h1 {
        font-size: var(--page-title-size);
        font-weight: var(--page-title-weight);
        margin: 0;
    }

    .subtitle {
        color: var(--text-secondary);
        font-size: var(--label-size);
        margin: 4px 0 0;
    }

    .header-actions {
        display: flex;
        gap: var(--spacing-sm);
    }

    .loading-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 300px;
        gap: var(--spacing-md);
        color: var(--text-secondary);
    }

    .costing-layout {
        display: grid;
        grid-template-columns: 1fr 350px;
        gap: var(--spacing-lg);
        flex: 1;
        min-height: 0;
    }

    .main-form {
        display: flex;
        flex-direction: column;
        gap: var(--spacing-md);
        overflow-y: auto;
    }

    .header-grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 12px 16px;
        margin-bottom: 16px;
    }

    .header-grid.compact {
        grid-template-columns: repeat(3, 1fr);
        gap: 8px 12px;
        margin-bottom: 12px;
    }

    .header-row-3col {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 6px 14px;
        margin-bottom: 6px;
    }

    .header-row-3col.tight {
        margin-bottom: 4px;
    }

    .header-row-6col {
        display: grid;
        grid-template-columns: repeat(6, 1fr);
        gap: 6px 10px;
        margin-bottom: 4px;
    }

    .header-row-6col.tight {
        margin-bottom: 4px;
    }

    .field-group {
        display: flex;
        flex-direction: column;
        gap: 1px;
    }

    .field-group.full-span {
        grid-column: 1 / -1;
    }

    .field-group .field-label {
        font-size: 10px;
        font-weight: 600;
        color: var(--text-secondary, #6b7280);
        text-transform: uppercase;
        letter-spacing: 0.3px;
    }

    .field-group .input,
    .field-group .input-sm {
        font-size: 12px;
        padding: 4px 6px;
        border: 1px solid var(--border-subtle, #e5e7eb);
        border-radius: 4px;
        background: var(--surface-base, #fff);
        height: 28px;
    }

    .field-group .input:focus,
    .field-group .input-sm:focus {
        border-color: var(--accent-primary, #6366f1);
        outline: none;
        box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
    }

    .field-group .textarea-sm {
        min-height: 96px;
        height: auto;
        resize: vertical;
        padding-top: 8px;
        line-height: 1.4;
    }

    .technical-attachments {
        margin: 10px 0 8px;
        padding: 12px;
        border: 1px solid var(--border-subtle, #e5e7eb);
        border-radius: 6px;
        background: var(--surface-elevated, #f8fafc);
    }

    .attachments-header {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: 12px;
        margin-bottom: 10px;
    }

    .attachments-empty {
        margin: 3px 0 0;
        color: var(--text-secondary, #6b7280);
        font-size: 12px;
        line-height: 1.4;
    }

    .attachment-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .attachment-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        padding: 8px 10px;
        border: 1px solid var(--border-subtle, #e5e7eb);
        border-radius: 6px;
        background: var(--surface-base, #fff);
    }

    .attachment-row strong {
        display: block;
        max-width: 520px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 13px;
        color: var(--text-primary, #111827);
    }

    .attachment-row span {
        display: block;
        margin-top: 2px;
        font-size: 11px;
        color: var(--text-secondary, #6b7280);
    }

    .attachment-actions {
        display: flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
    }

    .attachment-actions button {
        min-height: 28px;
        padding: 4px 10px;
        border: 1px solid var(--border-subtle, #e5e7eb);
        border-radius: 5px;
        background: var(--surface-base, #fff);
        color: var(--text-primary, #111827);
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
    }

    .attachment-actions button.danger {
        color: #b91c1c;
        background: #fef2f2;
        border-color: #fee2e2;
    }

    .attachment-actions button:disabled {
        opacity: 0.55;
        cursor: not-allowed;
    }

    .expand-toggle {
        background: none;
        border: none;
        color: var(--text-secondary, #6b7280);
        font-size: 11px;
        cursor: pointer;
        padding: 4px 0;
        margin-top: 2px;
        text-align: left;
    }

    .expand-toggle:hover {
        color: var(--accent-primary, #6366f1);
    }

    .advanced-section {
        padding-top: 6px;
        border-top: 1px solid var(--border-subtle, #e5e7eb);
        margin-top: 4px;
    }

    .form-group {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .form-group .field-label {
        font-size: 11px;
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .input {
        padding: 8px 12px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 13px;
        background: var(--bg-base);
    }

    .input:focus {
        outline: none;
        border-color: var(--brand-indigo);
        box-shadow: 0 0 0 2px var(--brand-indigo-tint);
    }

    .input.readonly {
        background: var(--surface-elevated);
        color: var(--text-secondary);
    }

    .input-sm {
        padding: 6px 8px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 12px;
        background: var(--bg-base);
        width: 100%;
        min-width: 0;
        box-sizing: border-box;
        min-height: 32px;
        height: 32px;
        line-height: 1.2;
    }

    .input-xs {
        padding: 4px 6px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 11px;
        width: 60px;
        background: var(--bg-base);
    }

    .input-sm.qty {
        width: 100%;
        text-align: center;
    }

    .input-sm.money {
        width: 100%;
        text-align: right;
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
    }

    .input-sm.sell-price {
        background: #f0f4ff;
        border-color: var(--brand-indigo);
        font-weight: 500;
    }

    .input-sm.sell-price:focus {
        background: #fff;
    }

    /* Line Items */
    .line-items-header {
        display: grid;
        /* Sl No | Equipment | Model | Qty | Currency | Unit Price | Freight | Misc Cost | Quote Price | Total (BHD) | Remove */
        grid-template-columns: 35px 2fr 120px 45px 60px 80px 70px 70px 85px 90px 28px;
        gap: 8px;
        padding: 8px 10px;
        font-size: 10px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.03em;
        color: var(--text-secondary);
        border-bottom: 2px solid var(--border);
        margin-bottom: 8px;
        align-items: end;
    }

    .line-row-top {
        display: grid;
        grid-template-columns: 35px minmax(220px, 2.2fr) minmax(140px, 1.2fr) 68px 88px 132px 28px;
        gap: 8px;
        padding: 8px 10px 2px;
        align-items: end;
        background: var(--surface-elevated);
        border-radius: var(--border-radius-sm) var(--border-radius-sm) 0 0;
        border-left: 3px solid transparent;
    }

    .line-row-pricing {
        display: grid;
        grid-template-columns: 1fr 1fr 1fr 1fr 130px;
        gap: 8px;
        padding: 2px 10px 8px;
        margin-left: 43px;
        align-items: end;
        background: var(--surface-elevated);
        border-radius: 0 0 var(--border-radius-sm) var(--border-radius-sm);
    }

    .field-inline {
        display: flex;
        flex-direction: column;
        gap: 1px;
    }

    .field-inline.compact {
        max-width: 88px;
    }

    .field-inline.exchange-rate {
        min-width: 132px;
    }

    .field-inline .field-label {
        font-size: 9px;
        font-weight: 600;
        color: var(--text-tertiary, #9ca3af);
        text-transform: uppercase;
        letter-spacing: 0.3px;
    }

    .line-total {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 1px;
    }

    .line-total .field-label {
        font-size: 9px;
        font-weight: 600;
        color: var(--text-tertiary, #9ca3af);
        text-transform: uppercase;
    }

    .line-item-group.alt-row .line-row-top,
    .line-item-group.alt-row .line-row-pricing {
        background: rgba(99, 102, 241, 0.04);
    }

    .line-item-group {
        margin-bottom: 4px;
    }

    .sl-no {
        font-size: 12px;
        font-weight: 600;
        color: var(--text-secondary);
        text-align: center;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    /* Order code row - full width for long supplier configuration codes */
    .long-code-row {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 4px 10px 4px 10px;
    }

    .long-code-label {
        font-size: 10px;
        font-weight: 600;
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.03em;
        white-space: nowrap;
        min-width: 80px;
    }

    .long-code-input {
        flex: 1;
        padding: 5px 10px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 12px;
        font-family: 'SF Mono', 'Fira Code', monospace;
        background: var(--bg-base);
        letter-spacing: 0.02em;
    }

    .long-code-input:focus {
        outline: none;
        border-color: var(--brand-indigo);
        box-shadow: 0 0 0 2px var(--brand-indigo-tint);
    }

    .long-code-input::placeholder {
        font-family: inherit;
        opacity: 0.4;
    }

    /* Wide detailed description field for instrumentation specs */
    .detailed-description-row {
        padding: 0 10px 8px 10px;
        margin-bottom: 8px;
        border-bottom: 1px solid var(--border);
    }

    .detailed-description-input {
        width: 100%;
        padding: 10px 12px;
        font-size: 12px;
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        line-height: 1.5;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        background: var(--surface);
        color: var(--text-primary);
        resize: vertical;
        min-height: 60px;
    }

    .detailed-description-input:focus {
        outline: none;
        border-color: var(--brand-indigo, #4F46E5);
        box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.1);
    }

    .detailed-description-input::placeholder {
        color: var(--text-muted);
        font-style: italic;
    }

    /* Suggested price display */
    .suggested-price {
        background: var(--ether, #F5F5F7);
        padding: 6px 8px;
        border-radius: var(--border-radius-sm);
        font-weight: 500;
        color: var(--steel, #86868B);
    }

    /* User override highlight */
    .sell-price.user-overridden {
        background: rgba(79, 70, 229, 0.1);
        border-color: var(--brand-indigo, #4F46E5);
    }

    /* Margin input styling */
    .input-sm.margin {
        text-align: center;
        font-weight: 500;
    }

    .calc-value {
        font-size: 12px;
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        color: var(--text-secondary);
        text-align: right;
    }

    .calc-value.highlight {
        color: var(--brand-indigo);
        font-weight: 600;
    }

    .btn-remove {
        background: transparent;
        border: none;
        color: #ef4444;
        cursor: pointer;
        font-size: 18px;
        padding: 0;
        width: 24px;
        height: 24px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
    }

    .btn-remove:hover {
        background: rgba(239, 68, 68, 0.1);
    }

    .remove-slot {
        width: 24px;
        height: 24px;
        display: block;
    }

    .cost-breakdown {
        padding: 8px 12px 12px;
        margin-bottom: 12px;
        background: var(--bg-base);
        border: 1px dashed var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 11px;
    }

    .cost-row {
        display: flex;
        gap: 16px;
        margin-bottom: 8px;
        flex-wrap: wrap;
    }

    .cost-label {
        color: var(--text-secondary);
    }

    .cost-label.suggested-highlight {
        color: var(--brand-indigo, #4F46E5);
        font-weight: 600;
        background: rgba(79, 70, 229, 0.08);
        padding: 2px 8px;
        border-radius: 4px;
    }

    .cost-input {
        display: flex;
        align-items: center;
        gap: 4px;
    }

    .cost-input .field-label {
        font-size: 10px;
        color: var(--text-muted);
    }

    /* Issue #7: Copy costs button styling */
    .btn-copy-costs {
        background: var(--brand-indigo, #6366f1);
        color: white;
        border: none;
        padding: 4px 10px;
        border-radius: 4px;
        font-size: 0.75rem;
        cursor: pointer;
        white-space: nowrap;
        transition: background 0.15s;
        margin-left: auto;
    }

    .btn-copy-costs:hover {
        background: var(--brand-indigo-hover, #4f46e5);
    }

    .add-item-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px;
        border-top: 1px solid var(--border);
        margin-top: 8px;
    }

    .item-count {
        font-size: 12px;
        color: var(--text-secondary);
    }

    /* Summary Panel */
    .summary-panel {
        display: flex;
        flex-direction: column;
        gap: var(--spacing-md);
    }

    .summary-rows {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .summary-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-size: 14px;
    }

    .summary-row.total {
        padding-top: 12px;
        border-top: 2px solid #059669;
        font-weight: 700;
        font-size: 18px;
        color: #059669;
    }

    .summary-row .money {
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        font-weight: 500;
    }

    .profit-section {
        margin-top: 20px;
        padding-top: 16px;
        border-top: 1px solid var(--border);
    }

    .profit-section h4 {
        font-size: 12px;
        text-transform: uppercase;
        color: var(--text-secondary);
        margin: 0 0 12px;
    }

    .profit-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px;
    }

    .profit-item {
        padding: 12px;
        background: var(--surface-elevated);
        border-radius: var(--border-radius-sm);
    }

    .profit-item.highlight {
        background: var(--brand-indigo-tint);
    }

    .profit-item.muted {
        background: rgba(15, 23, 42, 0.04);
    }

    .profit-item .label {
        display: block;
        font-size: 10px;
        color: var(--text-secondary);
        text-transform: uppercase;
        margin-bottom: 4px;
    }

    .profit-item .value {
        font-size: 16px;
        font-weight: 600;
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
    }

    .profit-item.highlight .value {
        color: var(--brand-indigo);
    }

    /* Recent Sheets */
    .recent-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .recent-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 8px;
        background: var(--surface-elevated);
        border-radius: var(--border-radius-sm);
        font-size: 12px;
    }

    .recent-item .ref {
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        font-weight: 500;
    }

    .recent-item .customer {
        color: var(--text-secondary);
        flex: 1;
        margin: 0 8px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .recent-item .amount {
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        font-weight: 600;
    }

    .empty {
        color: var(--text-muted);
        font-style: italic;
        font-size: 13px;
        text-align: center;
        padding: 16px;
    }

    /* Opportunity Selector Styles */
    .opportunity-selector {
        padding: 16px 0;
    }

    .selector-description {
        color: var(--text-secondary);
        font-size: 14px;
        margin: 0 0 20px;
    }

    .draft-recovery-panel {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        padding: 14px 16px;
        margin-bottom: 20px;
        border: 1px solid var(--border);
        border-left: 3px solid var(--brand-indigo);
        border-radius: var(--border-radius-sm);
        background: var(--surface-elevated);
    }

    .draft-recovery-panel p {
        margin: 4px 0 0;
        color: var(--text-secondary);
        font-size: 13px;
        line-height: 1.4;
    }

    .draft-recovery-actions {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-shrink: 0;
    }

    .selector-grid {
        display: flex;
        align-items: flex-end;
        gap: 16px;
        margin-bottom: 24px;
    }

    .selector-dropdown {
        flex: 1;
        max-width: 500px;
    }

    .selector-divider {
        display: flex;
        align-items: center;
        padding: 0 8px 8px;
        color: var(--text-muted);
        font-size: 12px;
        text-transform: uppercase;
    }

    .opp-preview-list {
        margin-top: 24px;
        padding-top: 20px;
        border-top: 1px solid var(--border);
    }

    .opp-preview-list h4 {
        font-size: 12px;
        text-transform: uppercase;
        color: var(--text-secondary);
        letter-spacing: 0.05em;
        margin: 0 0 12px;
    }

    .opp-preview-item {
        display: block;
        width: 100%;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        padding: 12px 16px;
        margin-bottom: 8px;
        text-align: left;
        cursor: pointer;
        transition: all var(--transition-fast);
    }

    .opp-preview-item:hover {
        border-color: var(--text-muted);
        box-shadow: var(--shadow-sm);
    }

    .opp-preview-item.selected {
        border-color: var(--brand-indigo);
        background: var(--brand-indigo-tint);
    }

    .opp-preview-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 4px;
    }

    .opp-customer {
        font-weight: 600;
        font-size: 14px;
        color: var(--text-primary);
    }

    .opp-status {
        font-size: 10px;
        padding: 2px 8px;
        border-radius: 10px;
        text-transform: uppercase;
    }

    .badge-new { background: var(--brand-indigo-tint); color: var(--brand-indigo); }
    .badge-quoted { background: rgba(245, 158, 11, 0.1); color: #d97706; }
    .badge-won { background: rgba(34, 197, 94, 0.1); color: #16a34a; }
    .badge-lost { background: rgba(239, 68, 68, 0.1); color: #ef4444; }

    .opp-preview-body {
        margin-bottom: 8px;
    }

    .opp-project {
        font-size: 13px;
        color: var(--text-secondary);
    }

    .opp-preview-footer {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .opp-value {
        font-family: var(--font-mono, 'JetBrains Mono', monospace);
        font-size: 13px;
        font-weight: 600;
        color: var(--text-primary);
    }

    .opp-value.pending {
        color: var(--text-muted);
        font-weight: 500;
    }

    .opp-date {
        font-size: 11px;
        color: var(--text-muted);
    }

    .empty-opportunities {
        padding: 32px;
        text-align: center;
        color: var(--text-secondary);
        background: var(--surface-elevated);
        border-radius: var(--border-radius);
        margin-top: 16px;
    }

    .empty-opportunities p {
        margin: 0;
        font-size: 14px;
    }

    /* Selected Opportunity Bar */
    .selected-opportunity-bar {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 12px 16px;
        background: var(--surface-elevated);
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        margin-bottom: 16px;
    }

    .selected-info {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
    }

    .selected-info .label {
        color: var(--text-muted);
    }

    .selected-info .customer {
        font-weight: 600;
        color: var(--text-primary);
    }

    .selected-info .project {
        color: var(--text-secondary);
    }

    .selected-info .hint {
        color: var(--text-muted);
        font-style: italic;
    }

    .link-opportunity-control {
        margin-left: auto;
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: min(520px, 48vw);
    }

    .link-opportunity-control .input-sm {
        flex: 1;
        min-width: 220px;
    }

    .costing-backdrop {
        position: fixed;
        inset: 0;
        z-index: 1200;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 24px;
        background: rgba(15, 23, 42, 0.36);
    }

    .confirm-card {
        width: min(440px, 100%);
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        box-shadow: var(--shadow-xl, 0 24px 60px rgba(15, 23, 42, 0.22));
        padding: 24px;
    }

    .confirm-kicker {
        display: block;
        color: #b45309;
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        margin-bottom: 8px;
    }

    .confirm-card h2 {
        margin: 0 0 10px;
        font-size: 20px;
        line-height: 1.25;
        color: var(--text-primary);
    }

    .confirm-card p {
        margin: 0;
        color: var(--text-secondary);
        font-size: 14px;
        line-height: 1.6;
    }

    .confirm-actions {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
        margin-top: 22px;
    }

    @media (max-width: 1200px) {
        .costing-layout {
            grid-template-columns: 1fr;
        }

        .summary-panel {
            order: -1;
        }

        .header-grid {
            grid-template-columns: repeat(2, 1fr);
        }
    }

    /* Disable pop animations on costing sheet - scale transforms look buggy */
    .costing-page :global(.onyx-card:hover),
    .costing-page :global([class*="phi-card"]:hover),
    .costing-page :global(.card:hover) {
        transform: none !important;
    }

    /* FIX #23: Disable hover animation "dance" on costing inputs */
    /* Prevents headache-inducing box highlighting when hovering over qty/price/margin fields */
    .cost-breakdown input:hover,
    .cost-breakdown input:focus,
    .input-sm:hover,
    .input-sm:focus,
    .input-xs:hover,
    .input-xs:focus {
        transition: none !important;
        transform: none !important;
    }

    /* Remove any parent container hover effects that cause visual dance */
    .line-item-row:hover {
        transform: none !important;
        transition: none !important;
        border-left-color: var(--brand-indigo, #6366f1);
        background: transparent !important;
    }

    .cost-breakdown:hover {
        transform: none !important;
        transition: none !important;
    }

    /* Terms & Conditions Section */
    .tc-section {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .tc-note {
        font-size: 11px;
        color: var(--text-muted);
        margin: 0;
    }

    .tc-textarea {
        width: 100%;
        min-height: 150px;
        padding: 12px;
        font-size: 12px;
        font-family: inherit;
        line-height: 1.5;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        background: var(--bg-base);
        color: var(--text-primary);
        resize: vertical;
    }

    .tc-textarea:focus {
        outline: none;
        border-color: var(--onyx);
    }

    /* VAT editable control */
    .vat-row .vat-control {
        display: flex;
        align-items: center;
        gap: 4px;
    }

    .vat-input {
        width: 50px;
        padding: 2px 6px;
        border: 1px solid var(--border);
        border-radius: var(--border-radius-sm);
        font-size: 13px;
        text-align: center;
        background: var(--bg-base);
    }

    .vat-input:focus {
        outline: none;
        border-color: var(--brand-indigo);
    }

    /* Global anti-flicker: disable ALL transitions and animations on the costing page */
    .costing-page * {
        animation: none !important;
    }

    .costing-page :global(*) {
        animation: none !important;
    }

    /* Revision Selector (Feature D) */
    .revision-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-bottom: 16px;
    }

    .revision-item {
        padding: 12px;
        border-radius: 8px;
        background: var(--surface-elevated);
        cursor: pointer;
        transition: all 0.15s;
        border: 2px solid transparent;
    }

    .revision-item:hover {
        background: var(--surface-hover);
    }

    .revision-item.selected {
        border-color: var(--accent-primary);
        background: var(--accent-bg);
    }

    .revision-item.active {
        border-left: 4px solid var(--success);
    }

    .rev-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 4px;
    }

    .rev-number {
        font-weight: 600;
        font-size: 0.95rem;
    }

    .rev-details {
        display: flex;
        gap: 12px;
        font-size: 0.85rem;
        color: var(--text-muted);
    }

    .revision-actions {
        display: flex;
        gap: 12px;
        padding-top: 12px;
        border-top: 1px solid var(--border-color);
    }

    .badge {
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 0.7rem;
        text-transform: uppercase;
    }

    .badge-success {
        background: rgba(46, 213, 115, 0.2);
        color: #2ed573;
    }
</style>
