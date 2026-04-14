<script lang="ts">
    import { onMount } from "svelte";
    import { toast } from "$lib/stores/toasts";
    import { permissions } from '$lib/stores/authContext';
    $: canView =
        $permissions.includes('*') ||
        $permissions.includes('costing:read') ||
        $permissions.includes('costing:*') ||
        $permissions.includes('offers:view') ||
        $permissions.includes('offers:create') ||
        $permissions.includes('rfq:view') ||
        $permissions.includes('orders:view');
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
        DeleteCostingSheet,
        GetCostingsByRFQ,
        SetActiveCostingRevision,
        CloneCostingAsNewRevision,
        GetSettings,
    } from "../../../wailsjs/go/main/App";

    const dispatch = createEventDispatcher();

    // Props
    export let embedded = false;

    // State
    let loading = true;
    let hasLoadedInitialData = false;
    let customers: any[] = [];
    let costingSheets: any[] = [];
    let opportunities: any[] = [];
    let previewOpportunities: any[] = [];
    let selectedOpportunityId: string = '';
    let selectedOpportunity: any = null;
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
            if (Array.isArray(parsed)) return parsed;
            if (parsed && typeof parsed === 'object') return [parsed];
        } catch {
            return [];
        }
        return [];
    }

    function mapSeedItemsToCosting(seedItems: any[]): LineItem[] {
        const mapped = seedItems.slice(0, 10).map((seedItem: any) => {
            const item = createNewLineItem();
            item.equipment = seedItem.description || seedItem.equipment || seedItem.name || '';
            item.model = seedItem.part_number || seedItem.model || seedItem.product_code || seedItem.sku || '';
            item.longCode = seedItem.long_code || '';
            item.specification = seedItem.specification || '';
            item.detailedDescription = seedItem.detailed_description || '';
            item.quantity = Number(seedItem.quantity) || 1;
            item.currency = seedItem.currency || 'BHD';
            item.fobForeign = Number(seedItem.unit_price_bhd || seedItem.unit_price || 0) || 0;
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
        return opp?.folder_number || opp?.rfq_number || opp?.eh_ref || "Manual";
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
    // Staff list for "Prepared By" dropdown
    const staffList = [
        { name: 'Stanley Vaz', title: 'COO', phone: '+973 3662 3662', email: 'stanley.vaz@platinumholdings.com.bh' },
        { name: 'Abhie Kore', title: 'Business Head', phone: '+973 3677 1513', email: 'abhi.kore@platinumholdings.com.bh' },
        { name: 'V.M. Sundar', title: 'Manager', phone: '+973 3826 0157', email: 'vm.sundar@platinumholdings.com.bh' },
        { name: 'Ebin Chacko', title: 'Technical Sales Engineer', phone: '+973 3641 8072', email: 'ebin.chacko@platinumholdings.com.bh' },
        { name: 'Periyasamy N', title: 'Technical Sales Engineer', phone: '', email: '' },
        { name: 'Ramya Baskaran', title: 'Sales Support', phone: '+973 3889 0513', email: 'support@platinumholdings.com.bh' },
    ];

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
        { code: 'EUR', rate: 0.44 },
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
            item.equipment?.trim() || item.fobForeign > 0
        );
    }

    // ========================================
    // CALCULATIONS - Matches Excel formulas
    // ========================================
    function calculateLineItem(item: LineItem): LineItem {
        // Get exchange rate from currency
        const curr = currencyOptions.find(c => c.code === item.currency);
        if (!(item.exchangeRate > 0)) {
            item.exchangeRate = curr?.rate || 0.44;
        }

        // FOB and Freight in BHD
        item.fobBHD = item.fobForeign * item.exchangeRate;
        item.freightPercent = Math.max(0, Number(item.freightPercent) || 0);
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
        item.marginPercent = Math.max(0, Number(item.marginPercent) || 0);
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

    function calculateAll() {
        lineItems = lineItems.map(item => calculateLineItem(item));
        lineItems = [...lineItems]; // Trigger reactivity
        isDirty = true;
    }

    // Summary calculations
    $: subtotal = lineItems.reduce((sum, item) => sum + item.totalSuggestedPrice, 0);
    let discount = 0;
    let hiddenCharges = 0;
    $: netAmount = subtotal - discount;
    $: vat = netAmount * (vatRate / 100);
    $: grandTotal = netAmount + vat;
    $: totalCost = lineItems.reduce((sum, item) => sum + (item.totalCost * item.quantity), 0) + hiddenCharges;
    $: profit = netAmount - totalCost;
    $: profitPercent = netAmount > 0 ? (profit / netAmount) * 100 : 0;

    // Keep T&C VAT label in sync when user changes VAT rate
    $: termsAndConditions = termsAndConditions.replace(/VAT \(\d+\.?\d*%\)/, `VAT (${vatRate}%)`);

    // ========================================
    // DATA LOADING
    // ========================================
    async function loadData() {
        loading = true;
        try {
            const [c, s, rfqs, pipeline, settingsResult] = await Promise.all([
                ListCustomers(500, 0),
                GetCostingSheets(20).catch(() => []),
                GetRFQs(200, 0).catch(() => []),
                GetPipelineOpportunities(500, 0).catch(() => []),
                GetSettings().catch(() => null),
            ]);
            customers = c || [];
            costingSheets = s || [];
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

            const pendingCostingOpportunity = sessionStorage.getItem('asymmflow.pendingCostingOpportunity');
            if (pendingCostingOpportunity) {
                try {
                    const pending = JSON.parse(pendingCostingOpportunity);
                    const match = opportunities.find((opp: any) => String(opp.id) === String(pending?.id));
                    if (match) {
                        selectedOpportunityId = String(match.id);
                        await handleOpportunitySelect();
                    }
                } finally {
                    sessionStorage.removeItem('asymmflow.pendingCostingOpportunity');
                }
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
            rfqCostings = [];
            selectedRevision = null;
            currentCostingId = null;
            return;
        }

        selectedOpportunity = opportunities.find(o => String(o.id) === selectedOpportunityId);
        if (selectedOpportunity) {
            let restoredRevision = false;
            if (isRFQOpportunity(selectedOpportunity)) {
                restoredRevision = await loadRevisionsForRFQ(Number(selectedOpportunity.id));
            } else {
                rfqCostings = [];
                selectedRevision = null;
                currentCostingId = null;
            }

            // Pre-fill header from opportunity data
            const customerName = selectedOpportunity.client || selectedOpportunity.customer || '';
            const projectName = selectedOpportunity.project || selectedOpportunity.project_name || '';
            const opportunityReference = getOpportunityDisplayReference(selectedOpportunity);

            // Find matching customer in customers list
            const matchedCustomer = customers.find(c =>
                c.business_name?.toLowerCase() === customerName.toLowerCase() ||
                c.name?.toLowerCase() === customerName.toLowerCase()
            );

            if (!restoredRevision) {
                const nextDivision = selectedOpportunity.division || header.division || 'PH Trading';
                header.customerName = customerName;
                header.customerId = matchedCustomer?.id || '';
                header.contactPerson = matchedCustomer?.contact_person || matchedCustomer?.primary_contact || '';
                header.rfqReference = opportunityReference;
                header.division = nextDivision;
                header.deliveryTerms = normaliseDeliveryTermsForDivision(header.deliveryTerms, nextDivision);

                // Generate folder number and costing ID
                const year = String(parseYear(selectedOpportunity.year || selectedOpportunity.created_at)).slice(-2);
                const num = costingSheets.length + 1; // Sequential based on existing sheets
                header.folderNumber = opportunityReference !== 'Manual' ? opportunityReference : `${num}-${year}`;
                header.costingId = `Offer ${num}-${year} ${customerName.split(' ')[0] || 'New'}`;
                header.subject = projectName ? `Sub: ${projectName}` : `Sub: ${header.customerName || customerName}`;
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
                const itemsJSON = JSON.stringify(buildExportData());
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
            header.customerName = customer.business_name || '';
            header.contactPerson = customer.contact_person || '';
            // Generate folder number and costing ID
            const year = new Date().getFullYear().toString().slice(-2);
            const num = costingSheets.length + 1; // Sequential based on existing sheets
            header.folderNumber = `${num}-${year}`;
            header.costingId = `Offer ${num}-${year} ${header.customerName.split(' ')[0]}`;
            header.subject = header.customerName ? `Sub: ${header.customerName}` : '';
        }
    }

    function startNewCosting() {
        // Start a blank costing without opportunity
        selectedOpportunityId = '';
        selectedOpportunity = null;
        showCostingForm = true;
        // Reset header
        header = createDefaultHeader();
        lineItems = [createNewLineItem()];
        quotationBody = defaultQuotationBody;
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
    function formatBHD(val: number): string {
        return val.toFixed(3) + ' BHD';
    }

    function formatPercent(val: number): string {
        return val.toFixed(0) + '%';
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
            equipment: item.equipment || '',
            model: item.model || '',
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

        lineItems = persistedLineItems.length > 0
            ? persistedLineItems.map((item: any, index: number) => normalisePersistedLineItem(item, index))
            : [createNewLineItem()];

        header = {
            ...header,
            division: payload.division || header.division,
            date: payload.date || header.date,
            preparedBy: payload.preparedBy || revision.created_by || header.preparedBy,
            customerName: payload.customerName || header.customerName,
            contactPerson: payload.contactPerson || header.contactPerson,
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
        calculateAll();
    }

    // ========================================
    // EXPORT FUNCTIONS
    // ========================================
    function buildExportData() {
        return {
            division: header.division,
            date: header.date,
            preparedBy: header.preparedBy,
            customerName: header.customerName,
            contactPerson: header.contactPerson,
            rfqReference: header.rfqReference,
            folderNumber: header.folderNumber,
            costingId: header.costingId,
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
                const effectivePrice = item.userPriceSet && item.userPrice > 0
                    ? item.userPrice
                    : item.suggestedPriceUnit;
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
                    quantity: item.quantity,
                    fob: item.fobForeign,
                    freight: item.freightForeign,
                    freightPercent: item.freightPercent,
                    totalCost: item.totalCost,
                    marginPercent: item.marginPercent,
                    markupPercent: item.marginPercent,
                    suggestedPrice: effectivePrice,
                    totalPrice: item.totalSuggestedPrice,
                    // Full cost breakdown for persistence
                    exchangeRate: item.exchangeRate,
                    fobBHD: item.fobBHD,
                    freightBHD: item.freightBHD,
                    insurance: item.insurance,
                    customsPercent: item.customsPercent,
                    customsBHD: item.customsBHD,
                    handlingPercent: item.handlingPercent,
                    handlingBHD: item.handlingBHD,
                    financePercent: item.financePercent,
                    financeBHD: item.financeBHD,
                    otherCosts: item.otherCosts,
                    userPrice: item.userPrice,
                    userPriceSet: item.userPriceSet,
                };
            }),
            subtotal: subtotal,
            discount: discount,
            netAmount: netAmount,
            vat: vat,
            grandTotal: grandTotal,
            totalCost: totalCost,
            profit: profit,
            profitPercent: profitPercent,
            opportunityId: getSelectedRFQId(),
            projectName: selectedOpportunity?.project || selectedOpportunity?.project_name || '',
            body: quotationBody,
            termsAndConditions: termsAndConditions,
            quoteType: header.quoteType || 'Quotation',
            vatRate: vatRate,
            hiddenCharges: hiddenCharges,
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
    let deletingCosting = false;
    let currentCostingId: number | null = null;

    // Phase 5: Delete costing sheet handler
    async function handleDeleteCosting() {
        const rfqId = getSelectedRFQId();
        if (!rfqId) {
            toast.warning('No costing sheet to delete');
            return;
        }

        const confirmed = confirm('Are you sure you want to delete this costing sheet? This cannot be undone.');
        if (!confirmed) return;

        deletingCosting = true;
        try {
            // Try to find the costing sheet ID linked to this RFQ
            const sheet = costingSheets.find(s => Number(s.RFQID) === rfqId);
            if (sheet?.ID) {
                await DeleteCostingSheet(sheet.ID);
                toast.success('Costing sheet deleted');
                // Navigate back to opportunity selection
                showCostingForm = false;
                selectedOpportunityId = '';
                selectedOpportunity = null;
                await loadData();
            } else {
                // No saved costing sheet yet - just clear the form
                showCostingForm = false;
                selectedOpportunityId = '';
                selectedOpportunity = null;
                toast.info('Form cleared (no saved costing)');
            }
        } catch (err) {
            console.error('Failed to delete costing:', err);
            toast.danger('Failed to delete: ' + String(err));
        } finally {
            deletingCosting = false;
        }
    }

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
                            items: JSON.stringify(persistedCosting),
                            created_by: header.preparedBy || 'System',
                        } as any);
                    } else {
                        const saved = await CreateCostingSheet(rfqId, JSON.stringify(persistedCosting), header.preparedBy || 'System');
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
                toast.success(`Offer ${offer.offer_number} created! View it in Sales Hub - Offers`);

                // Refresh recent sheets list
                costingSheets = await GetCostingSheets(20).catch(() => []);

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
        if (!canView) {
            loading = false;
        }
    });

    $: if (canView && !hasLoadedInitialData) {
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
                <Button variant="danger" on:click={handleDeleteCosting} disabled={deletingCosting}>
                    {deletingCosting ? 'Deleting...' : 'Delete'}
                </Button>
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
                <Button variant="ghost" on:click={() => {
                    if (isDirty && !confirm('You have unsaved changes. Leave anyway?')) return;
                    showCostingForm = false; selectedOpportunityId = ''; selectedOpportunity = null; isDirty = false;
                }}>
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
                {/if}
            </div>

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

        <div class="costing-layout">
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
                            <select bind:value={header.preparedBy} class="input-sm">
                                <option value="">Select...</option>
                                {#each staffList as staff}<option value={staff.name}>{staff.name}</option>{/each}
                            </select>
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
                            <input type="text" bind:value={header.folderNumber} class="input-sm" placeholder="42-26" />
                        </div>
                        <div class="field-group">
                            <span class="field-label">Costing ID</span>
                            <input type="text" bind:value={header.costingId} class="input-sm" placeholder="Auto" />
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
                                <input type="number" bind:value={item.freightPercent} class="input-sm money" step="0.01" min="0" on:change={calculateAll} placeholder="0.00" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Misc Cost</span>
                                <input type="number" bind:value={item.otherCosts} class="input-sm money" step="0.001" min="0" on:change={calculateAll} placeholder="0.000" title="Miscellaneous costs (internal)" />
                            </div>
                            <div class="field-inline">
                                <span class="field-label">Quote Price</span>
                                <input
                                    type="number"
                                    class="input-sm money sell-price"
                                    class:user-overridden={item.userPriceSet && item.userPrice > 0}
                                    step="0.001"
                                    placeholder={item.suggestedPriceUnit.toFixed(0)}
                                    bind:value={item.userPrice}
                                    on:input={() => { item.userPriceSet = item.userPrice > 0; calculateAll(); }}
                                    title="Enter selling price (suggested: {item.suggestedPriceUnit.toFixed(3)})"
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
                            <input type="number" bind:value={discount} class="input-sm money" step="0.001" on:change={calculateAll} />
                        </div>
                        <div class="summary-row">
                            <span title="Additional charges not shown on quotation">Hidden Charges</span>
                            <input type="number" bind:value={hiddenCharges} class="input-sm money" step="0.001" />
                        </div>
                        <div class="summary-row vat-row">
                            <div class="vat-control">
                                <span>VAT</span>
                                <input type="number" bind:value={vatRate} class="vat-input" min="0" max="100" step="0.5" />
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
        /* Sl No | Equipment | Model | Qty | Currency | Unit Price | Freight | Misc Cost | Suggested Price | Total (BHD) | Remove */
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
