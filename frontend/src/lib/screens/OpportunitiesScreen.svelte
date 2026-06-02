<script lang="ts">
    import { createEventDispatcher, onMount, tick } from "svelte";
    import OpportunityDetail from "../components/OpportunityDetail.svelte";
    import ContextTaskModal from "../components/ContextTaskModal.svelte";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import Button from "../components/ui/Button.svelte";

    import {
        BrowseFolder,
        GetRFQs,
        GetPipelineOpportunities,
        CreateRFQWithReference,
        DeleteOpportunity,
        DeleteRFQ,
        DeleteRFQWithCascade,
        ImportTenderFolders,
        ListCustomers,
        PreviewTenderFolders,
        UpdateOpportunityCommercialFields,
        UpdateRFQ,
        GetOpportunityComments,
        ExportReport,
        OpenExportedFile,
    } from "../../../wailsjs/go/main/App";
    import { toast } from "../stores/toasts";
    import { devLog } from "$lib/utils/devLog";
    import { debounce } from "$lib/utils/debounce";

    const dispatch = createEventDispatcher();

    let opportunities: any[] = [];
    let filteredOpportunities: any[] = [];
    let selected: any = null;
    let loadingList = true;
    let showCreateForm = false;
    let creating = false;
    let customers: any[] = [];

    let searchQuery = "";
    let debouncedSearchQuery = "";
    let activeFilter = "All";
    let activeYear = String(new Date().getFullYear());
    let sortBy = "date";
    let sortDirection = "desc";
    let availableYears: string[] = ["All"];

    let showOpportunityModal = false;
    let showDeleteConfirm = false;
    let deleteTarget: any = null;
    let deleting = false;
    let importingTenders = false;
    let editingOpportunity = false;
    let savingOpportunityEdit = false;
    let opportunityEdit: Record<string, any> = {};
    let opportunityChangeLog: any[] = [];
    let loadingOpportunityChangeLog = false;
    let extractingOpportunities = false;
    let showTaskModal = false;
    let opportunityModalEl: HTMLDivElement | null = null;

    let pipelineStats = {
        newCount: 0,
        quotedCount: 0,
        wonCount: 0,
        totalValue: 0,
        winRate: 0,
    };

    let createForm = {
        customer: "",
        project: "",
        rfq_ref: "",
        received_date: new Date().toISOString().split("T")[0],
        due_date: "",
        priority: "Normal",
        value: "",
        notes: "",
    };

    const priorityOptions = ["Normal", "Urgent", "Critical"];
    const STATUS_OPTIONS = ["All", "RFQ Received", "New", "Qualified", "Proposal", "Quoted", "Won", "Lost", "On Hold"];
    const SORT_OPTIONS = [
        { id: "date", label: "Recent" },
        { id: "key", label: "Key" },
        { id: "customer", label: "Customer" },
        { id: "value", label: "Value" },
        { id: "status", label: "Status" },
        { id: "source", label: "Source" },
    ];
    const STAGE_ORDER = ["RFQ Received", "New", "Qualified", "Proposal", "Quoted", "Won", "Lost", "On Hold"];

    const updateDebouncedSearch = debounce((value: string) => {
        debouncedSearchQuery = value;
        applyFilters();
    }, 300);

    $: updateDebouncedSearch(searchQuery);

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

    function getOpportunityYear(opp: any): string {
        return String(parseYear(opp?.year ?? opp?.created_at));
    }

    function buildOpportunityTitle(customerName: string, reference: string, fallback: string) {
        const customer = (customerName || "").trim();
        const ref = (reference || "").trim();
        if (customer && ref) return `${customer} / ${ref}`;
        if (customer) return customer;
        if (ref) return ref;
        return fallback || "Untitled";
    }

    function normalizeStage(value: any): string {
        const stage = String(value || "").trim();
        if (!stage) return "New";
        return stage;
    }

    function displayStage(value: any): string {
        const stage = normalizeStage(value);
        return DEPRECATED_STAGE_LABELS.has(stage) ? "Pipeline" : stage;
    }

    function getOpportunityTimestamp(opp: any): number {
        const candidates = [
            opp?.updated_at,
            opp?.created_at,
            opp?.offer_date,
            opp?.received_date,
            opp?.expected_date,
        ];

        for (const candidate of candidates) {
            if (!candidate) continue;
            const parsed = new Date(candidate).getTime();
            if (!Number.isNaN(parsed) && parsed > 0) {
                return parsed;
            }
        }

        return 0;
    }

    function compareOpportunityRecency(a: any, b: any): number {
        const timestampDiff = getOpportunityTimestamp(b) - getOpportunityTimestamp(a);
        if (timestampDiff !== 0) return timestampDiff;

        const folderA = String(a?.folder_number || a?.rfq_number || "").trim();
        const folderB = String(b?.folder_number || b?.rfq_number || "").trim();
        if (folderA || folderB) {
            return folderB.localeCompare(folderA, undefined, { numeric: true, sensitivity: "base" });
        }

        return String(b?.id || "").localeCompare(String(a?.id || ""), undefined, { numeric: true, sensitivity: "base" });
    }

    function naturalCompare(a: any, b: any): number {
        return String(a || "").localeCompare(String(b || ""), undefined, { numeric: true, sensitivity: "base" });
    }

    function getOpportunityKey(opp: any): string {
        return String(opp?.folder_number || opp?.rfq_number || opp?.rfq_ref || opp?.eh_ref || opp?.id || "").trim();
    }

    function getOpportunityCustomer(opp: any): string {
        return String(opp?.client || opp?.customer || opp?.customer_name || "").trim();
    }

    function getOpportunityTitle(opp: any): string {
        return String(opp?.title || opp?.project || opp?.project_name || opp?.folder_name || "").trim();
    }

    function getStageRank(value: any): number {
        const stage = normalizeStage(value);
        const index = STAGE_ORDER.indexOf(stage);
        return index >= 0 ? index : STAGE_ORDER.length;
    }

    function chooseSort(nextSortBy: string) {
        if (sortBy === nextSortBy) {
            sortDirection = sortDirection === "asc" ? "desc" : "asc";
            return;
        }
        sortBy = nextSortBy;
        sortDirection = nextSortBy === "date" || nextSortBy === "value" ? "desc" : "asc";
    }

    function normalizePipelineOpportunity(opp: any) {
        const visibleReference = opp.eh_ref || opp.folder_number || "";
        const stage = normalizeStage(opp.stage);
        return {
            ...opp,
            client: opp.customer_name,
            customer: opp.customer_name,
            project: buildOpportunityTitle(opp.customer_name, visibleReference, opp.title || opp.folder_name || opp.folder_number),
            project_name: buildOpportunityTitle(opp.customer_name, visibleReference, opp.title || opp.folder_name || opp.folder_number),
            value: Number(opp.revenue_bhd) || 0,
            status: stage,
            stage,
            rfq_number: visibleReference,
            rfq_ref: opp.eh_ref || "",
            year: parseYear(opp.year),
            _source: "pipeline",
        };
    }

    function normalizeRFQ(rfq: any) {
        const reference = rfq.rfq_ref || rfq.rfq_number || "";
        const fallbackTitle = rfq.project || rfq.project_name || "Untitled";
        const stage = normalizeStage(rfq.stage || rfq.status);
        return {
            ...rfq,
            client: rfq.client || rfq.customer || "Unknown",
            customer: rfq.client || rfq.customer || "Unknown",
            project: buildOpportunityTitle(rfq.client || rfq.customer || "", reference, fallbackTitle),
            project_name: buildOpportunityTitle(rfq.client || rfq.customer || "", reference, fallbackTitle),
            value: Number(rfq.value) || 0,
            status: stage,
            stage,
            year: parseYear(rfq.created_at),
            _source: "rfq",
        };
    }

    function refreshAvailableYears() {
        const years = [...new Set(opportunities.map(getOpportunityYear))]
            .filter(Boolean)
            .sort((a, b) => Number(b) - Number(a));
        availableYears = ["All", ...years];
        if (activeYear !== "All" && years.length > 0 && !years.includes(activeYear)) {
            activeYear = years[0];
        }
        if (years.length === 0) {
            activeYear = "All";
        }
    }

    async function loadOpportunities() {
        loadingList = true;
        try {
            const [rfqs, pipeline, custList] = await Promise.all([
                GetRFQs(200, 0).catch(() => []),
                GetPipelineOpportunities(500, 0).catch(() => []),
                ListCustomers(500, 0).catch(() => []),
            ]);

            customers = custList || [];

            const pipelineNormalized = (pipeline || []).map(normalizePipelineOpportunity);
            const liveRfqs = (rfqs || [])
                .filter((rfq: any) => String(rfq?.rfq_number || "").trim())
                .map(normalizeRFQ);

            const rfqFolders = new Set(liveRfqs.map((rfq: any) => String(rfq.rfq_number || "").trim()).filter(Boolean));
            const uniquePipeline = pipelineNormalized.filter((opp: any) => !rfqFolders.has(String(opp.folder_number || "").trim()));

            opportunities = [...uniquePipeline, ...liveRfqs].sort(compareOpportunityRecency);
            refreshAvailableYears();
            applyFilters();

            if (showOpportunityModal && selected) {
                const refreshed = opportunities.find((opp) => String(opp.id) === String(selected.id));
                selected = refreshed || selected;
            }
        } catch (err) {
            devLog.error(err);
            toast.danger("Failed to load opportunities");
        } finally {
            loadingList = false;
        }
    }

    function calculateStats(baseList: any[]) {
        const stats = {
            newCount: 0,
            quotedCount: 0,
            wonCount: 0,
            totalValue: 0,
        };
        let won = 0;
        let closed = 0;

        baseList.forEach((opp) => {
            const stage = normalizeStage(opp.stage || opp.status);
            const value = Number(opp.value) || Number(opp.revenue_bhd) || 0;

            if (DEPRECATED_STAGE_LABELS.has(stage)) stats.newCount++;
            if (stage === "Quoted") stats.quotedCount++;
            if (stage === "Won") {
                stats.wonCount++;
                won++;
                closed++;
            }
            if (stage === "Lost") {
                closed++;
            }

            stats.totalValue += value;
        });

        pipelineStats = {
            ...stats,
            winRate: closed > 0 ? (won / closed) * 100 : 0,
        };
    }

    function applyFilters() {
        let yearScoped = [...opportunities];

        if (activeYear !== "All") {
            yearScoped = yearScoped.filter((opp) => getOpportunityYear(opp) === activeYear);
        }

        calculateStats(yearScoped);

        let result = [...yearScoped];

        if (activeFilter !== "All") {
            result = result.filter((opp) => (opp.stage || opp.status) === activeFilter);
        }

        if (debouncedSearchQuery.trim()) {
            const query = debouncedSearchQuery.toLowerCase();
            result = result.filter((opp) =>
                (opp.client || opp.customer || "").toLowerCase().includes(query) ||
                (opp.project || opp.project_name || "").toLowerCase().includes(query) ||
                String(opp.rfq_number || opp.folder_number || "").toLowerCase().includes(query) ||
                String(opp.folder_name || "").toLowerCase().includes(query) ||
                String(opp.title || "").toLowerCase().includes(query) ||
                String(opp.stage || opp.status || "").toLowerCase().includes(query) ||
                String(opp.owner || opp.owner_name || "").toLowerCase().includes(query) ||
                String(opp.payment_terms || "").toLowerCase().includes(query) ||
                String(opp.delivery_terms || "").toLowerCase().includes(query) ||
                String(opp.notes || opp.comment || opp.owner_notes || "").toLowerCase().includes(query)
            );
        }

        result.sort((a, b) => {
            let comparison = 0;

            if (sortBy === "date") {
                comparison = getOpportunityTimestamp(a) - getOpportunityTimestamp(b);
            } else if (sortBy === "value") {
                comparison = getOpportunityValue(a) - getOpportunityValue(b);
            } else if (sortBy === "customer") {
                comparison = naturalCompare(getOpportunityCustomer(a), getOpportunityCustomer(b));
            } else if (sortBy === "key") {
                comparison = naturalCompare(getOpportunityKey(a), getOpportunityKey(b));
            } else if (sortBy === "status") {
                comparison = getStageRank(a.stage || a.status) - getStageRank(b.stage || b.status);
                if (comparison === 0) comparison = naturalCompare(a.stage || a.status, b.stage || b.status);
            } else if (sortBy === "source") {
                comparison = naturalCompare(a._source, b._source);
            } else {
                comparison = naturalCompare(getOpportunityTitle(a), getOpportunityTitle(b));
            }

            if (comparison === 0) {
                return compareOpportunityRecency(a, b);
            }

            return sortDirection === "asc" ? comparison : -comparison;
        });

        filteredOpportunities = result;
    }

    $: {
        activeFilter;
        activeYear;
        sortBy;
        sortDirection;
        applyFilters();
    }

    function formatCurrency(value: number) {
        return new Intl.NumberFormat("en-BH", {
            style: "currency",
            currency: "BHD",
            maximumFractionDigits: 0,
        }).format(Number(value) || 0);
    }

    function getOpportunityValue(opp: any): number {
        return Number(opp?.value ?? opp?.revenue_bhd ?? 0) || 0;
    }

    function formatOpportunityValue(opp: any): string {
        const value = getOpportunityValue(opp);
        if (value <= 0) return "Value pending";
        return formatCurrency(value);
    }

    function formatDate(value: any) {
        if (!value) return "No date";
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return "No date";
        return parsed.toLocaleDateString();
    }

    async function selectOpportunity(opp: any) {
        selected = opp;
        editingOpportunity = false;
        opportunityEdit = {};
        showOpportunityModal = true;
        loadOpportunityChangeLog(opp);
        await tick();
        opportunityModalEl?.focus();
    }

    async function loadOpportunityChangeLog(opp: any) {
        opportunityChangeLog = [];
        if (!opp || opp._source !== "pipeline" || !opp.id) return;
        loadingOpportunityChangeLog = true;
        try {
            opportunityChangeLog = await GetOpportunityComments(String(opp.id)) || [];
        } catch (err) {
            opportunityChangeLog = [{
                id: "unavailable",
                comment: "Change log is available to management/admin roles.",
                created_by: "System",
                created_at: new Date().toISOString(),
            }];
        } finally {
            loadingOpportunityChangeLog = false;
        }
    }

    function closeOpportunityModal() {
        showTaskModal = false;
        showOpportunityModal = false;
        editingOpportunity = false;
        opportunityEdit = {};
        opportunityChangeLog = [];
    }

    function toDateInputValue(value: any): string {
        if (!value) return "";
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return "";
        return parsed.toISOString().split("T")[0];
    }

    function startOpportunityEdit() {
        if (!selected) return;
        opportunityEdit = {
            folder_number: selected.folder_number || selected.rfq_number || "",
            title: selected.title || selected.project || selected.project_name || "",
            customer_name: selected.customer_name || selected.client || selected.customer || "",
            eh_ref: selected.eh_ref || selected.rfq_ref || "",
            stage: selected.stage || selected.status || "New",
            year: getOpportunityYear(selected),
            expected_date: toDateInputValue(selected.expected_date),
            salesperson: selected.salesperson || selected.owner || selected.owner_name || "",
            division: selected.division || "",
            source: selected.source || selected._source || "",
            product_type: selected.product_type || "",
            customer_grade: selected.customer_grade || "",
            payment_terms: selected.payment_terms || "",
            delivery_terms: selected.delivery_terms || "",
            revenue_bhd: Number(selected.value || selected.revenue_bhd || 0) || 0,
            cost_bhd: Number(selected.cost_bhd || 0) || 0,
            profit_bhd: Number(selected.profit_bhd || 0) || 0,
            comment: selected.comment || selected.notes || "",
            owner_notes: selected.owner_notes || "",
            product_details: selected.product_details || "",
            won_reason: selected.won_reason || "",
            lost_reason: selected.lost_reason || "",
        };
        editingOpportunity = true;
    }

    function cancelOpportunityEdit() {
        editingOpportunity = false;
        opportunityEdit = {};
    }

    async function saveOpportunityEdit() {
        if (!selected) return;
        savingOpportunityEdit = true;
        try {
            let normalized: any;
            if (selected._source === "rfq") {
                const updated = await UpdateRFQ(Number(selected.id), {
                    client: String(opportunityEdit.customer_name || "").trim(),
                    project: String(opportunityEdit.title || "").trim(),
                    rfq_ref: String(opportunityEdit.folder_number || "").trim(),
                    value: Number(opportunityEdit.revenue_bhd || 0) || 0,
                    notes: String(opportunityEdit.comment || "").trim(),
                    status: String(opportunityEdit.stage || "New").trim(),
                    visit_locations: "",
                    product_details: String(opportunityEdit.product_details || "").trim(),
                    document_hash: "",
                    source_doc_path: "",
                });
                normalized = normalizeRFQ(updated);
            } else {
                const updated = await UpdateOpportunityCommercialFields(String(selected.id), {
                    folder_number: String(opportunityEdit.folder_number || "").trim(),
                    title: String(opportunityEdit.title || "").trim(),
                    customer_name: String(opportunityEdit.customer_name || "").trim(),
                    eh_ref: String(opportunityEdit.eh_ref || "").trim(),
                    stage: String(opportunityEdit.stage || "New").trim(),
                    year: Number(opportunityEdit.year || 0) || undefined,
                    expected_date: String(opportunityEdit.expected_date || "").trim(),
                    salesperson: String(opportunityEdit.salesperson || "").trim(),
                    division: String(opportunityEdit.division || "").trim(),
                    source: String(opportunityEdit.source || "").trim(),
                    product_type: String(opportunityEdit.product_type || "").trim(),
                    customer_grade: String(opportunityEdit.customer_grade || "").trim(),
                    payment_terms: String(opportunityEdit.payment_terms || "").trim(),
                    delivery_terms: String(opportunityEdit.delivery_terms || "").trim(),
                    revenue_bhd: Number(opportunityEdit.revenue_bhd || 0) || 0,
                    cost_bhd: Number(opportunityEdit.cost_bhd || 0) || 0,
                    profit_bhd: Number(opportunityEdit.profit_bhd || 0) || 0,
                    comment: String(opportunityEdit.comment || "").trim(),
                    owner_notes: String(opportunityEdit.owner_notes || "").trim(),
                    product_details: String(opportunityEdit.product_details || "").trim(),
                    won_reason: String(opportunityEdit.won_reason || "").trim(),
                    lost_reason: String(opportunityEdit.lost_reason || "").trim(),
                });
                normalized = normalizePipelineOpportunity(updated);
            }
            selected = { ...selected, ...normalized };
            opportunities = opportunities.map((opp) => String(opp.id) === String(selected.id) ? { ...opp, ...normalized } : opp);
            editingOpportunity = false;
            opportunityEdit = {};
            applyFilters();
            await loadOpportunityChangeLog(selected);
            toast.success("Opportunity updated");
        } catch (err) {
            toast.danger(`Failed to update opportunity: ${String(err)}`);
        } finally {
            savingOpportunityEdit = false;
        }
    }

    async function handleTenderImport() {
        importingTenders = true;
        try {
            const rootPath = await BrowseFolder();
            if (!rootPath) return;
            const preview = await PreviewTenderFolders(rootPath);
            if (!Array.isArray(preview) || preview.length === 0) {
                toast.info("No tender folders found in that location");
                return;
            }
            const confirmed = window.confirm(`Import ${preview.length} tender folder${preview.length === 1 ? "" : "s"} as RFQ Received / Tender records?`);
            if (!confirmed) return;
            const imported = await ImportTenderFolders(rootPath);
            toast.success(`Imported ${(imported || []).length} tender folder${(imported || []).length === 1 ? "" : "s"}`);
            await loadOpportunities();
            activeFilter = "RFQ Received";
        } catch (err) {
            toast.danger(`Tender import failed: ${String(err)}`);
        } finally {
            importingTenders = false;
        }
    }

    async function exportOpportunityExtract() {
        extractingOpportunities = true;
        try {
            const path = await ExportReport("opportunity_extract", "excel", JSON.stringify(filteredOpportunities));
            await OpenExportedFile(path).catch(() => {});
            toast.success(`Opportunity extract exported: ${path}`);
        } catch (err) {
            toast.danger(`Failed to export opportunities: ${String(err)}`);
        } finally {
            extractingOpportunities = false;
        }
    }

    function handleOpportunityCardKeydown(event: KeyboardEvent, opp: any) {
        if (event.key !== "Enter" && event.key !== " ") return;
        if (event.target !== event.currentTarget) return;
        event.preventDefault();
        selectOpportunity(opp);
    }

    function handleGlobalKeydown(event: KeyboardEvent) {
        if (event.key !== "Escape") return;
        if (showOpportunityModal && selected) {
            closeOpportunityModal();
            return;
        }
        if (showCreateForm) {
            showCreateForm = false;
            return;
        }
        if (showDeleteConfirm) {
            showDeleteConfirm = false;
        }
    }

    function handleOpenCostingFromDetail(event: CustomEvent) {
        const opportunity = event.detail?.opportunity || selected;
        const pendingPayload = event.detail?.pendingPayload || {
            id: String(opportunity?.id || ""),
            customer_name: opportunity?.client || opportunity?.customer || "",
            folder_number: opportunity?.folder_number || opportunity?.rfq_number || "",
            rfq_ref: opportunity?.rfq_ref || opportunity?.eh_ref || "",
            title: opportunity?.project || opportunity?.project_name || opportunity?.title || "",
        };
        if (pendingPayload?.id) {
            sessionStorage.setItem("asymmflow.pendingCostingOpportunity", JSON.stringify(pendingPayload));
        }
        showTaskModal = false;
        showOpportunityModal = false;
        dispatch("navigate", { screen: "opportunities", tab: "costing" });
        window.dispatchEvent(new CustomEvent("navigateToScreen", {
            detail: { screen: "opportunities", tab: "costing" }
        }));
    }

    async function handleCreate() {
        if (!createForm.customer || !createForm.project) {
            toast.warning("Customer and Prospect/Project fields are required");
            return;
        }

        creating = true;
        try {
            await CreateRFQWithReference(
                createForm.customer,
                createForm.project,
                createForm.rfq_ref,
                parseFloat(createForm.value) || 0,
                createForm.notes,
            );
            toast.success("Opportunity created");
            showCreateForm = false;
            createForm = {
                customer: "",
                project: "",
                rfq_ref: "",
                received_date: new Date().toISOString().split("T")[0],
                due_date: "",
                priority: "Normal",
                value: "",
                notes: "",
            };
            await loadOpportunities();
        } catch (err) {
            toast.danger("Creation failed: " + err);
        } finally {
            creating = false;
        }
    }

    function confirmDelete(opp: any, event: MouseEvent) {
        event.stopPropagation();
        deleteTarget = opp;
        showDeleteConfirm = true;
    }

    async function handleDelete(cascade = false) {
        if (!deleteTarget) return;

        deleting = true;
        try {
            if (deleteTarget._source === "pipeline") {
                await DeleteOpportunity(String(deleteTarget.id));
            } else if (cascade) {
                await DeleteRFQWithCascade(deleteTarget.id, true);
            } else {
                await DeleteRFQ(deleteTarget.id);
            }
            toast.success(`Opportunity "${deleteTarget.project || "Untitled"}" deleted`);
            if (selected?.id === deleteTarget.id) {
                selected = null;
                showOpportunityModal = false;
            }
            showDeleteConfirm = false;
            deleteTarget = null;
            await loadOpportunities();
        } catch (err: any) {
            const errorMsg = err?.message || String(err);
            if (errorMsg.toLowerCase().includes("not authorized") || errorMsg.toLowerCase().includes("only admins")) {
                toast.warning("You are not authorised to delete this record. Please ask an admin.");
            } else if (errorMsg.includes("costing sheet") || errorMsg.includes("offer")) {
                toast.warning("This opportunity has linked documents. Use 'Delete All' to remove everything.");
            } else {
                toast.danger("Delete failed: " + errorMsg);
            }
        } finally {
            deleting = false;
        }
    }

    onMount(() => {
        loadOpportunities();
    });
</script>

<div class="screen-container">
    <div class="action-bar">
        <div class="kpi-row">
            <div class="mini-kpi">
                <span class="kpi-label">Pipeline</span>
                <span class="kpi-value">{formatCurrency(pipelineStats.totalValue)}</span>
            </div>
            <div class="mini-kpi">
                <span class="kpi-label">Win Rate</span>
                <span class="kpi-value">{pipelineStats.winRate.toFixed(1)}%</span>
            </div>
            <div class="mini-kpi">
                <span class="kpi-label">Open</span>
                <span class="kpi-value">{pipelineStats.newCount + pipelineStats.quotedCount}</span>
            </div>
        </div>
        <div class="primary-actions">
            <Button variant="secondary" on:click={handleTenderImport} disabled={importingTenders}>
                {importingTenders ? "Importing..." : "Import Tenders"}
            </Button>
            <Button variant="secondary" on:click={exportOpportunityExtract} disabled={extractingOpportunities || loadingList}>
                {extractingOpportunities ? "Exporting..." : "Extract"}
            </Button>
            <Button variant="primary" on:click={() => (showCreateForm = true)}>+ New Opportunity</Button>
        </div>
    </div>

    <div class="controls-bar">
        <div class="filter-tabs">
            {#each STATUS_OPTIONS as status}
                <button
                    class="filter-tab"
                    class:active={activeFilter === status}
                    on:click={() => (activeFilter = status)}
                >
                    {status}
                </button>
            {/each}
        </div>

        <div class="right-controls">
            <select bind:value={activeYear} class="year-select" aria-label="Filter opportunities by year">
                {#each availableYears as yearOption}
                    <option value={yearOption}>{yearOption === "All" ? "All Years" : yearOption}</option>
                {/each}
            </select>

            <div class="sort-controls">
                <div class="sort-chip-group" role="group" aria-label="Sort opportunities">
                    {#each SORT_OPTIONS as option}
                        <button
                            class="sort-chip"
                            class:active={sortBy === option.id}
                            on:click={() => chooseSort(option.id)}
                            type="button"
                        >
                            {option.label}
                            {#if sortBy === option.id}
                                <span>{sortDirection === "asc" ? "Asc" : "Desc"}</span>
                            {/if}
                        </button>
                    {/each}
                </div>
                <button
                    class="sort-direction-btn"
                    on:click={() => (sortDirection = sortDirection === "asc" ? "desc" : "asc")}
                    title={sortDirection === "asc" ? "Ascending" : "Descending"}
                >
                    {sortDirection === "asc" ? "Ascending" : "Descending"}
                </button>
            </div>

            <div class="search-box">
                <input
                    type="text"
                    class="search-input"
                    placeholder="Search customer, project..."
                    bind:value={searchQuery}
                    aria-label="Search opportunities"
                />
            </div>
        </div>
    </div>

    <div class="list-shell">
        <div class="list-header">
            <span class="result-count">
                {filteredOpportunities.length} opportunities
                {#if activeYear !== "All"}
                    in {activeYear}
                {/if}
            </span>
        </div>

        {#if loadingList}
            <div class="loading-state"><WabiSpinner size="md" tempo="calm" /></div>
        {:else if filteredOpportunities.length === 0}
            <div class="empty-state">
                {searchQuery || activeFilter !== "All" ? "No matching opportunities found." : "No opportunities available."}
            </div>
        {:else}
            <div class="opportunity-list">
                {#each filteredOpportunities as opp}
                    <div
                        class="opportunity-card"
                        role="button"
                        tabindex="0"
                        on:click={() => selectOpportunity(opp)}
                        on:keydown={(event) => handleOpportunityCardKeydown(event, opp)}
                    >
                        <div class="card-main">
                            <div class="card-identity">
                                <div class="eyebrow-row">
                                    <span class="opportunity-number">{opp.folder_number || opp.rfq_number || "Manual"}</span>
                                    <span class="year-pill">{getOpportunityYear(opp)}</span>
                                    <span class:source-pill={true} class:rfq={opp._source === "rfq"}>
                                        {opp._source === "rfq" ? "RFQ" : "Pipeline"}
                                    </span>
                                </div>
                                <h3>{opp.project || opp.project_name || "Untitled Opportunity"}</h3>
                                <p class="customer-name">{opp.client || opp.customer || "Unknown Customer"}</p>
                            </div>

                            <div class="card-summary">
                                <span class="status-badge">{displayStage(opp.stage || opp.status)}</span>
                                <span class="card-value" class:pending={getOpportunityValue(opp) <= 0}>
                                    {formatOpportunityValue(opp)}
                                </span>
                            </div>
                        </div>

                        <div class="card-foot">
                            <div class="meta-strip">
                                <span>Updated {formatDate(opp.updated_at || opp.created_at)}</span>
                                {#if opp.payment_terms}
                                    <span>{opp.payment_terms}</span>
                                {/if}
                                {#if opp.delivery_terms}
                                    <span>{opp.delivery_terms}</span>
                                {/if}
                            </div>

                            <div class="card-actions">
                                <button
                                    class="delete-btn"
                                    on:click={(event) => confirmDelete(opp, event)}
                                    aria-label={`Delete ${opp.project || "opportunity"}`}
                                >
                                    Delete
                                </button>
                                <span class="view-link">View details</span>
                            </div>
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
</div>

<svelte:window on:keydown={handleGlobalKeydown} />

{#if showOpportunityModal && selected}
    <div
        class="modal-backdrop"
    >
        <button type="button" class="modal-dismiss" aria-label="Close opportunity details" on:click={closeOpportunityModal}></button>
        <div
            bind:this={opportunityModalEl}
            class="modal-card opportunity-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="opportunity-modal-title"
            tabindex="-1"
        >
            <div class="modal-header">
                <div>
                    <p class="modal-kicker">{selected.folder_number || selected.rfq_number || "Opportunity"}</p>
                    <h3 id="opportunity-modal-title">{selected.project || selected.project_name || "Opportunity Details"}</h3>
                </div>
                <div class="modal-header-actions">
                    <button class="ghost-btn" on:click={() => showTaskModal = true}>Create Task</button>
                    {#if editingOpportunity}
                        <button class="ghost-btn" on:click={cancelOpportunityEdit} disabled={savingOpportunityEdit}>Cancel</button>
                        <button class="ghost-btn primary-ghost" on:click={saveOpportunityEdit} disabled={savingOpportunityEdit}>
                            {savingOpportunityEdit ? "Saving..." : "Save"}
                        </button>
                    {:else}
                        <button class="ghost-btn" on:click={startOpportunityEdit}>Edit</button>
                    {/if}
                    <button class="ghost-btn danger-ghost" on:click={(event) => confirmDelete(selected, event)}>Delete</button>
                    <button class="close-btn" on:click={closeOpportunityModal} aria-label="Close opportunity details">Close</button>
                </div>
            </div>

            {#if editingOpportunity}
                <section class="opportunity-editor">
                    <div class="editor-grid">
                        <div class="form-group">
                            <label class="label" for="edit-folder">RFQ / Folder Key</label>
                            <input id="edit-folder" class="input" bind:value={opportunityEdit.folder_number} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-title">Opportunity Name</label>
                            <input id="edit-title" class="input" bind:value={opportunityEdit.title} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-customer">Customer</label>
                            <input id="edit-customer" class="input" bind:value={opportunityEdit.customer_name} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-ref">Customer / EH Reference</label>
                            <input id="edit-ref" class="input" bind:value={opportunityEdit.eh_ref} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-stage">Status</label>
                            <select id="edit-stage" class="input" bind:value={opportunityEdit.stage}>
                                {#each STATUS_OPTIONS.filter((status) => status !== "All") as status}
                                    <option value={status}>{status}</option>
                                {/each}
                            </select>
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-value">Value (BHD)</label>
                            <input id="edit-value" class="input" type="number" step="0.001" bind:value={opportunityEdit.revenue_bhd} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-year">Year</label>
                            <input id="edit-year" class="input" type="number" min="2000" max="2100" bind:value={opportunityEdit.year} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-expected-date">Expected Close</label>
                            <input id="edit-expected-date" class="input" type="date" bind:value={opportunityEdit.expected_date} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-salesperson">Owner / Salesperson</label>
                            <input id="edit-salesperson" class="input" bind:value={opportunityEdit.salesperson} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-division">Division</label>
                            <input id="edit-division" class="input" bind:value={opportunityEdit.division} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-source">Source</label>
                            <input id="edit-source" class="input" bind:value={opportunityEdit.source} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-product-type">Product Type</label>
                            <input id="edit-product-type" class="input" bind:value={opportunityEdit.product_type} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-cost">Cost (BHD)</label>
                            <input id="edit-cost" class="input" type="number" step="0.001" bind:value={opportunityEdit.cost_bhd} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-profit">Profit (BHD)</label>
                            <input id="edit-profit" class="input" type="number" step="0.001" bind:value={opportunityEdit.profit_bhd} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-payment">Payment Terms</label>
                            <input id="edit-payment" class="input" bind:value={opportunityEdit.payment_terms} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-delivery">Delivery Terms</label>
                            <input id="edit-delivery" class="input" bind:value={opportunityEdit.delivery_terms} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-grade">Customer Grade</label>
                            <input id="edit-grade" class="input" bind:value={opportunityEdit.customer_grade} />
                        </div>
                        <div class="form-group">
                            <label class="label" for="edit-won-reason">Won Reason</label>
                            <input id="edit-won-reason" class="input" bind:value={opportunityEdit.won_reason} />
                        </div>
                        <div class="form-group wide">
                            <label class="label" for="edit-product-details">Line / Product Details</label>
                            <textarea id="edit-product-details" class="input" rows="3" bind:value={opportunityEdit.product_details}></textarea>
                        </div>
                        <div class="form-group wide">
                            <label class="label" for="edit-comment">Customer / Commercial Notes</label>
                            <textarea id="edit-comment" class="input" rows="3" bind:value={opportunityEdit.comment}></textarea>
                        </div>
                        <div class="form-group wide">
                            <label class="label" for="edit-owner-notes">Internal Notes</label>
                            <textarea id="edit-owner-notes" class="input" rows="3" bind:value={opportunityEdit.owner_notes}></textarea>
                        </div>
                        <div class="form-group wide">
                            <label class="label" for="edit-lost-reason">Lost Reason</label>
                            <textarea id="edit-lost-reason" class="input" rows="2" bind:value={opportunityEdit.lost_reason}></textarea>
                        </div>
                    </div>
                </section>
            {:else}
                <OpportunityDetail
                    opportunity={selected}
                    on:updated={loadOpportunities}
                    on:createCosting={handleOpenCostingFromDetail}
                />
            {/if}

            <section class="change-log-panel" aria-label="Opportunity change log">
                <div class="change-log-header">
                    <h4>Change Log</h4>
                    {#if loadingOpportunityChangeLog}
                        <span>Loading...</span>
                    {:else}
                        <span>{opportunityChangeLog.length} entr{opportunityChangeLog.length === 1 ? "y" : "ies"}</span>
                    {/if}
                </div>
                {#if loadingOpportunityChangeLog}
                    <div class="change-log-empty">Loading opportunity history...</div>
                {:else if opportunityChangeLog.length === 0}
                    <div class="change-log-empty">No recorded changes yet.</div>
                {:else}
                    <div class="change-log-list">
                        {#each opportunityChangeLog as entry}
                            <article class="change-log-entry">
                                <div>
                                    <strong>{entry.created_by || "System"}</strong>
                                    <p>{entry.comment}</p>
                                </div>
                                <time>{formatDate(entry.created_at)}</time>
                            </article>
                        {/each}
                    </div>
                {/if}
            </section>
        </div>
    </div>
{/if}

{#if selected}
    <ContextTaskModal
        open={showTaskModal}
        title="Create Opportunity Task"
        subtitle={`Link work to ${selected.project || selected.project_name || selected.folder_number || "this opportunity"}`}
        defaults={{
            customer_id: selected.customer_id,
            opportunity_id: String(selected.id || ""),
            seed_title: `Opportunity task: ${selected.project || selected.project_name || selected.folder_number || "Follow up"}`,
        }}
        on:close={() => showTaskModal = false}
        on:created={() => showTaskModal = false}
    />
{/if}

{#if showCreateForm}
    <div
        class="modal-backdrop"
    >
        <button type="button" class="modal-dismiss" aria-label="Close create opportunity modal" on:click={() => (showCreateForm = false)}></button>
        <div class="modal-card create-modal" role="dialog" aria-modal="true" aria-labelledby="create-modal-title">
            <h3 id="create-modal-title" class="section-title">New Opportunity</h3>

            <form on:submit|preventDefault={handleCreate}>
                <div class="form-group">
                    <label class="label" for="create-customer">Customer</label>
                    <input id="create-customer" list="cust-list" bind:value={createForm.customer} class="input" placeholder="Search customer..." required />
                    <datalist id="cust-list">
                        {#each customers as customer}
                            <option value={customer.business_name} />
                        {/each}
                    </datalist>
                </div>

                <div class="form-group">
                    <label class="label" for="create-project">Project</label>
                    <input id="create-project" type="text" bind:value={createForm.project} class="input" placeholder="Project name" required />
                </div>

                <div class="form-group">
                    <label class="label" for="create-opportunity-reference">Reference ID</label>
                    <input id="create-opportunity-reference" type="text" bind:value={createForm.rfq_ref} class="input" placeholder="Customer RFQ / enquiry ref" />
                </div>

                <div class="form-grid">
                    <div class="form-group">
                        <label class="label" for="create-value">Value (BHD)</label>
                        <input id="create-value" type="number" bind:value={createForm.value} class="input" placeholder="0.00" />
                    </div>
                    <div class="form-group">
                        <label class="label" for="create-priority">Priority</label>
                        <select id="create-priority" bind:value={createForm.priority} class="input">
                            {#each priorityOptions as priority}
                                <option>{priority}</option>
                            {/each}
                        </select>
                    </div>
                </div>

                <div class="form-group">
                    <label class="label" for="create-notes">Notes</label>
                    <textarea
                        id="create-notes"
                        bind:value={createForm.notes}
                        class="input notes-input"
                        rows="4"
                        placeholder="Add context, scope, customer request details, exclusions, or next steps..."
                    ></textarea>
                </div>

                <div class="modal-actions">
                    <Button variant="secondary" on:click={() => (showCreateForm = false)}>Cancel</Button>
                    <Button variant="primary" type="submit" disabled={creating}>
                        {creating ? "Creating..." : "Create"}
                    </Button>
                </div>
            </form>
        </div>
    </div>
{/if}

{#if showDeleteConfirm && deleteTarget}
    <div
        class="modal-backdrop"
    >
        <button type="button" class="modal-dismiss" aria-label="Close delete opportunity modal" on:click={() => (showDeleteConfirm = false)}></button>
        <div class="modal-card delete-modal" role="alertdialog" aria-modal="true" aria-labelledby="delete-modal-title">
            <div class="delete-header">
                <h3 id="delete-modal-title" class="section-title">Delete Opportunity?</h3>
            </div>

            <div class="delete-body">
                <p>Are you sure you want to delete this opportunity?</p>
                <div class="delete-details">
                    <div class="detail-row">
                        <span class="detail-label">Customer:</span>
                        <span class="detail-value">{deleteTarget.client || deleteTarget.customer || "Unknown"}</span>
                    </div>
                    <div class="detail-row">
                        <span class="detail-label">Project:</span>
                        <span class="detail-value">{deleteTarget.project || "Untitled"}</span>
                    </div>
                    <div class="detail-row">
                        <span class="detail-label">Value:</span>
                        <span class="detail-value">{formatOpportunityValue(deleteTarget)}</span>
                    </div>
                </div>
                <p class="warning-text">This action cannot be undone.</p>
            </div>

            <div class="modal-actions">
                <Button variant="secondary" on:click={() => (showDeleteConfirm = false)} disabled={deleting}>
                    Cancel
                </Button>
                <Button variant="danger" on:click={() => handleDelete(false)} disabled={deleting}>
                    {deleting ? "Deleting..." : "Delete"}
                </Button>
                {#if deleteTarget._source === "rfq"}
                    <Button variant="danger" on:click={() => handleDelete(true)} disabled={deleting}>
                        {deleting ? "..." : "Delete All"}
                    </Button>
                {/if}
            </div>
        </div>
    </div>
{/if}

<style>
    .screen-container {
        height: 100%;
        display: flex;
        flex-direction: column;
        gap: 1rem;
        position: relative;
        overflow: hidden;
    }

    .action-bar,
    .controls-bar,
    .card-main,
    .card-foot,
    .modal-header,
    .modal-actions,
    .delete-header,
    .detail-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
    }

    .kpi-row,
    .primary-actions,
    .filter-tabs,
    .right-controls,
    .sort-controls,
    .eyebrow-row,
    .meta-strip,
    .card-actions,
    .form-grid,
    .delete-details {
        display: flex;
        gap: 0.75rem;
        flex-wrap: wrap;
    }

    .mini-kpi {
        display: flex;
        flex-direction: column;
        min-width: 110px;
    }

    .kpi-label,
    .modal-kicker {
        font-size: 0.7rem;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--text-muted);
    }

    .kpi-value {
        font-size: 1.05rem;
        font-weight: 700;
        color: var(--text-primary);
    }

    .controls-bar {
        gap: 1rem;
        flex-wrap: wrap;
    }

    .filter-tab,
    .year-select,
    .sort-direction-btn,
    .search-input,
    .input,
    .delete-btn,
    .close-btn {
        border: 1px solid var(--border);
        border-radius: 12px;
        background: var(--surface);
        color: var(--text-primary);
    }

    .filter-tab {
        padding: 0.55rem 0.9rem;
        cursor: pointer;
        font-size: 0.8rem;
    }

    .filter-tab.active {
        background: var(--brand-indigo);
        color: white;
        border-color: var(--brand-indigo);
    }

    .year-select,
    .search-input,
    .input {
        padding: 0.65rem 0.8rem;
        min-height: 42px;
    }

    .search-box {
        min-width: 280px;
    }

    .search-input {
        width: 100%;
    }

    .notes-input {
        width: 100%;
        min-height: 112px;
        resize: vertical;
        line-height: 1.5;
    }

    .sort-direction-btn,
    .delete-btn,
    .close-btn {
        padding: 0.65rem 0.85rem;
        cursor: pointer;
    }

    .sort-controls {
        align-items: center;
        gap: 0.5rem;
    }

    .sort-chip-group {
        display: flex;
        flex-wrap: wrap;
        gap: 0.45rem;
        align-items: center;
    }

    .sort-chip {
        border: 1px solid rgba(15, 23, 42, 0.1);
        border-radius: 999px;
        background: rgba(255, 255, 255, 0.82);
        color: var(--text-secondary);
        min-height: 34px;
        padding: 0.45rem 0.75rem;
        font: inherit;
        font-size: 0.78rem;
        font-weight: 700;
        cursor: pointer;
        box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
        transition: background 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
    }

    .sort-chip:hover {
        transform: translateY(-1px);
        box-shadow: 0 5px 14px rgba(15, 23, 42, 0.09);
        color: var(--text-primary);
    }

    .sort-chip.active {
        background: var(--brand-indigo);
        color: #ffffff;
        border-color: var(--brand-indigo);
    }

    .sort-chip span {
        margin-left: 0.35rem;
        font-size: 0.68rem;
        opacity: 0.82;
    }

    .list-shell {
        flex: 1;
        min-height: 0;
        overflow: auto;
        padding-right: 0.25rem;
    }

    .list-header {
        margin-bottom: 0.9rem;
    }

    .result-count {
        font-size: 0.85rem;
        color: var(--text-muted);
    }

    .opportunity-list {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: 0.9rem;
    }

    .opportunity-card {
        display: flex;
        flex-direction: column;
        gap: 1rem;
        padding: 1.15rem 1.25rem;
        border-radius: 20px;
        border: 1px solid rgba(15, 23, 42, 0.08);
        background:
            linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(244, 247, 251, 0.96)),
            radial-gradient(circle at top right, rgba(56, 189, 248, 0.09), transparent 35%);
        box-shadow: 0 16px 38px rgba(15, 23, 42, 0.08);
        cursor: pointer;
        transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
    }

    .opportunity-card:hover {
        transform: translateY(-1px);
        box-shadow: 0 22px 44px rgba(15, 23, 42, 0.12);
        border-color: rgba(37, 99, 235, 0.22);
    }

    .card-identity {
        flex: 1;
        min-width: 0;
    }

    .card-identity h3,
    .modal-header h3 {
        margin: 0.35rem 0 0.2rem;
        font-size: 1.2rem;
        line-height: 1.2;
    }

    .customer-name {
        margin: 0;
        color: var(--text-secondary);
    }

    .opportunity-number,
    .year-pill,
    .source-pill,
    .status-badge,
    .view-link {
        display: inline-flex;
        align-items: center;
        border-radius: 999px;
        padding: 0.32rem 0.65rem;
        font-size: 0.72rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
    }

    .opportunity-number {
        background: rgba(16, 185, 129, 0.14);
        color: #047857;
        font-weight: 700;
    }

    .year-pill {
        background: rgba(14, 165, 233, 0.12);
        color: #0369a1;
    }

    .source-pill {
        background: rgba(15, 23, 42, 0.08);
        color: var(--text-secondary);
    }

    .source-pill.rfq {
        background: rgba(168, 85, 247, 0.12);
        color: #7c3aed;
    }

    .card-summary {
        min-width: 170px;
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 0.5rem;
    }

    .status-badge {
        background: rgba(15, 23, 42, 0.08);
        color: var(--text-primary);
        font-weight: 700;
    }

    .card-value {
        font-size: 1.15rem;
        font-weight: 800;
        color: var(--text-primary);
    }

    .card-value.pending {
        color: var(--text-muted);
        font-weight: 600;
    }

    .meta-strip {
        flex: 1;
        color: var(--text-muted);
        font-size: 0.82rem;
    }

    .view-link {
        background: rgba(37, 99, 235, 0.1);
        color: #1d4ed8;
        font-weight: 700;
    }

    .modal-backdrop {
        position: absolute;
        inset: 0;
        background: rgba(15, 23, 42, 0.45);
        backdrop-filter: blur(6px);
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 1rem;
        z-index: 50;
    }

    .modal-dismiss {
        position: absolute;
        inset: 0;
        border: 0;
        padding: 0;
        background: transparent;
        appearance: none;
        cursor: pointer;
    }

    .modal-card {
        position: relative;
        z-index: 1;
        width: min(820px, calc(100% - 2rem));
        max-height: calc(100% - 2rem);
        overflow: auto;
        background: white;
        border-radius: 24px;
        box-shadow: 0 30px 80px rgba(15, 23, 42, 0.24);
        padding: 1.25rem;
    }

    .opportunity-modal {
        width: min(860px, calc(100% - 2rem));
    }

    .modal-header {
        margin-bottom: 1rem;
        gap: 1rem;
    }

    .modal-header-actions {
        display: flex;
        gap: 0.75rem;
        align-items: center;
    }

    .ghost-btn {
        border: 1px solid var(--border);
        border-radius: 999px;
        padding: 0.65rem 1rem;
        background: white;
        font: inherit;
        color: var(--text-primary);
    }

    .primary-ghost {
        background: var(--brand-indigo);
        color: white;
        border-color: var(--brand-indigo);
    }

    .danger-ghost {
        color: #b42318;
        border-color: rgba(180, 35, 24, 0.18);
        background: rgba(254, 242, 242, 0.88);
    }

    .opportunity-editor {
        border-top: 1px solid var(--border);
        padding-top: 1rem;
    }

    .change-log-panel {
        margin-top: 1rem;
        padding-top: 1rem;
        border-top: 1px solid var(--border);
    }

    .change-log-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 1rem;
        margin-bottom: 0.75rem;
    }

    .change-log-header h4 {
        margin: 0;
        font-size: 0.9rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--text-primary);
    }

    .change-log-header span,
    .change-log-empty,
    .change-log-entry time {
        color: var(--text-muted);
        font-size: 0.78rem;
    }

    .change-log-list {
        display: flex;
        flex-direction: column;
        gap: 0.6rem;
    }

    .change-log-entry {
        display: flex;
        justify-content: space-between;
        gap: 1rem;
        padding: 0.75rem;
        border: 1px solid var(--border);
        border-radius: 12px;
        background: rgba(15, 23, 42, 0.03);
    }

    .change-log-entry strong {
        display: block;
        margin-bottom: 0.2rem;
        font-size: 0.82rem;
        color: var(--text-primary);
    }

    .change-log-entry p {
        margin: 0;
        color: var(--text-secondary);
        line-height: 1.4;
    }

    .editor-grid {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 0.9rem;
    }

    .editor-grid .wide {
        grid-column: 1 / -1;
    }

    .form-group {
        display: flex;
        flex-direction: column;
        gap: 0.45rem;
        margin-bottom: 0.9rem;
    }

    .form-grid > * {
        flex: 1 1 220px;
    }

    .label,
    .detail-label {
        font-size: 0.8rem;
        font-weight: 700;
        color: var(--text-muted);
    }

    .section-title {
        margin: 0 0 1rem;
    }

    .loading-state,
    .empty-state {
        min-height: 280px;
        display: grid;
        place-items: center;
        color: var(--text-muted);
        border: 1px dashed var(--border);
        border-radius: 20px;
        background: rgba(255, 255, 255, 0.65);
    }

    .delete-modal {
        width: min(560px, 100%);
    }

    .delete-body {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .delete-details {
        flex-direction: column;
        gap: 0.6rem;
        padding: 1rem;
        border-radius: 16px;
        background: rgba(15, 23, 42, 0.04);
    }

    .detail-row {
        gap: 1rem;
    }

    .warning-text {
        color: #b45309;
        margin: 0;
    }

    @media (max-width: 900px) {
        .card-main,
        .card-foot,
        .action-bar,
        .controls-bar,
        .modal-header,
        .primary-actions {
            flex-direction: column;
            align-items: stretch;
        }

        .editor-grid {
            grid-template-columns: 1fr;
        }

        .card-summary {
            align-items: flex-start;
        }

        .search-box {
            min-width: 100%;
        }

        .modal-backdrop {
            padding: 0.75rem;
        }
    }
</style>
