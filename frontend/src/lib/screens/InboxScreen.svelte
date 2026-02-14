<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { fade } from "svelte/transition";
    import { toast } from "../stores/toasts";
    import {
        ProcessInboxDocument,
        GetInboxDocuments,
        GetInboxStats,
        MarkInboxDocumentProcessed,
    } from "../../../wailsjs/go/main/App";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import { isAbortError } from "../utils/abortable";

    // State
    let documents = [];
    let stats = {
        total_documents: 0,
        ready: 0,
        needs_review: 0,
        processed: 0,
        by_type: {},
    };
    let loading = false;
    let selectedDoc = null;
    let filterStatus = "";

    // Controllers
    let loadController = null;
    let statsController = null;

    async function loadDocuments() {
        loadController?.abort();
        loadController = new AbortController();
        loading = true;
        try {
            documents = await GetInboxDocuments(filterStatus);
        } catch (e) {
            if (!isAbortError(e)) toast.danger("Failed to update inbox");
        } finally {
            loading = false;
        }
    }

    async function loadStats() {
        statsController?.abort();
        statsController = new AbortController();
        try {
            const result = await GetInboxStats();
            if (result && !statsController.signal.aborted) {
                stats = {
                    total_documents: result.total_documents ?? 0,
                    ready: result.ready ?? 0,
                    needs_review: result.needs_review ?? 0,
                    processed: result.processed ?? 0,
                    by_type: result.by_type ?? {},
                };
            }
        } catch (e) {
            if (!isAbortError(e)) console.error(e);
        }
    }

    async function handleProcess(doc) {
        if (!doc) return;
        toast.info("Processing...");
        try {
            // Re-process finding logic or just open detail?
            // The prompt says "ProcessInboxDocument".
            const result = await ProcessInboxDocument(doc.path, "");
            if (result) {
                toast.success(`Classified: ${result.detected_type}`);
                await loadDocuments();
                await loadStats();
            }
        } catch (e) {
            toast.danger("Processing failed");
        }
    }

    async function handleMarkProcessed(doc) {
        try {
            await MarkInboxDocumentProcessed(doc.id, "archived");
            toast.success("Archived");
            await loadDocuments();
            await loadStats();
            if (selectedDoc?.id === doc.id) selectedDoc = null;
        } catch (e) {
            toast.danger("Failed to archive");
        }
    }

    function selectDocument(doc) {
        selectedDoc = doc;
    }

    onMount(() => {
        loadDocuments();
        loadStats();
    });

    onDestroy(() => {
        loadController?.abort();
        statsController?.abort();
    });
</script>

<div class="page">
    <header class="header">
        <div class="header-content">
            <h1>Inbox.</h1>
            <p class="subtitle">Smart Document Processing</p>
        </div>
        <div class="stats-row">
            <div class="stat">
                <span class="val">{stats.ready}</span>
                <span class="lbl">Ready</span>
            </div>
            <div class="stat highlight">
                <span class="val">{stats.needs_review}</span>
                <span class="lbl">Review</span>
            </div>
            <div class="stat">
                <span class="val">{stats.processed}</span>
                <span class="lbl">Archived</span>
            </div>
        </div>
    </header>

    <div class="layout-split">
        <!-- List Panel -->
        <aside class="list-panel">
            <div class="filter-bar">
                <button
                    class:active={filterStatus === ""}
                    on:click={() => {
                        filterStatus = "";
                        loadDocuments();
                    }}>All</button
                >
                <button
                    class:active={filterStatus === "new"}
                    on:click={() => {
                        filterStatus === "new";
                        loadDocuments();
                    }}>New</button
                >
                <button
                    class:active={filterStatus === "review"}
                    on:click={() => {
                        filterStatus === "review";
                        loadDocuments();
                    }}>Review</button
                >
            </div>

            {#if loading}
                <div class="loading"><WabiSpinner size="sm" /></div>
            {:else if documents.length === 0}
                <div class="empty">Inbox is empty.</div>
            {:else}
                <div class="doc-list">
                    {#each documents as doc}
                        <div
                            class="doc-item"
                            class:selected={selectedDoc?.id === doc.id}
                            on:click={() => selectDocument(doc)}
                        >
                            <div class="doc-icon">
                                {doc.type === "Invoice"
                                    ? ""
                                    : doc.type === "RFQ"
                                      ? ""
                                      : ""}
                            </div>
                            <div class="doc-info">
                                <span class="doc-name">{doc.filename}</span>
                                <span class="doc-meta"
                                    >{new Date(
                                        doc.created_at || Date.now(),
                                    ).toLocaleDateString()} • {doc.status ||
                                        "New"}</span
                                >
                            </div>
                            {#if doc.confidence && doc.confidence < 0.8}
                                <span class="warning-dot"></span>
                            {/if}
                        </div>
                    {/each}
                </div>
            {/if}
        </aside>

        <!-- Detail/Action Panel -->
        <main class="detail-panel">
            {#if !selectedDoc}
                <div class="empty-state">Select a document to process.</div>
            {:else}
                <div class="doc-card" in:fade>
                    <div class="card-header">
                        <h2>{selectedDoc.filename || "Untitled Scan"}</h2>
                        <span class="status-badge {selectedDoc.status}"
                            >{selectedDoc.status || "New"}</span
                        >
                    </div>

                    <div class="meta-grid">
                        <div class="meta-item">
                            <span class="label">Detected Type</span>
                            <span class="value"
                                >{selectedDoc.detected_type || "Unknown"}</span
                            >
                        </div>
                        <div class="meta-item">
                            <span class="label">Confidence</span>
                            <span class="value"
                                >{(selectedDoc.confidence * 100).toFixed(
                                    0,
                                )}%</span
                            >
                        </div>
                    </div>

                    <div class="preview-box">
                        <!-- Simulate extracted text preview -->
                        <p class="preview-text">
                            {selectedDoc.extracted_text
                                ? selectedDoc.extracted_text.substring(0, 500) +
                                  "..."
                                : "No text extracted yet. Click 'Process' to analyze."}
                        </p>
                    </div>

                    <div class="actions">
                        {#if selectedDoc.status !== "Processed"}
                            <button
                                class="btn-primary"
                                on:click={() => handleProcess(selectedDoc)}
                            >
                                Reprocess / Analyze
                            </button>
                            <button
                                class="btn-ghost"
                                on:click={() =>
                                    handleMarkProcessed(selectedDoc)}
                            >
                                Archive
                            </button>
                        {/if}
                    </div>
                </div>
            {/if}
        </main>
    </div>
</div>

<style>
    .page {
        padding: var(--page-padding);
        height: 100vh;
        background: var(--paper);
        color: var(--ink);
        display: flex;
        flex-direction: column;
        box-sizing: border-box;
    }

    .header {
        display: flex;
        justify-content: space-between;
        align-items: flex-end;
        margin-bottom: var(--space-4);
        flex-shrink: 0;
    }
    h1 {
        font-size: var(--text-3xl);
        font-weight: var(--font-weight-light);
        margin: 0;
        letter-spacing: -0.01em;
    }
    .subtitle {
        color: var(--ink-faint);
        margin-top: var(--space-1);
        font-size: var(--text-sm);
    }

    .stats-row {
        display: flex;
        gap: var(--space-4);
    }
    .stat {
        display: flex;
        flex-direction: column;
        align-items: center;
    }
    .stat .val {
        font-size: 16px;
        font-weight: 500;
    }
    .stat .lbl {
        font-size: 9px;
        text-transform: uppercase;
        color: var(--ink-light);
    }
    .stat.highlight .val {
        color: #f59e0b;
    }

    .layout-split {
        display: grid;
        grid-template-columns: 240px 1fr;
        gap: var(--space-4);
        flex: 1;
        min-height: 0;
    }

    /* List */
    .list-panel {
        display: flex;
        flex-direction: column;
        border-right: 1px solid var(--border-subtle);
        padding-right: var(--space-3);
    }

    .filter-bar {
        display: flex;
        gap: 4px;
        margin-bottom: var(--space-4);
    }
    .filter-bar button {
        background: transparent;
        border: none;
        padding: 4px 10px;
        border-radius: 12px;
        font-size: 11px;
        cursor: pointer;
        color: var(--ink-light);
    }
    .filter-bar button.active {
        background: var(--paper-subtle);
        color: var(--ink);
        font-weight: 600;
    }

    .doc-list {
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }
    .doc-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px;
        border-radius: var(--radius-md);
        cursor: pointer;
        transition: background 0.2s;
    }
    .doc-item:hover,
    .doc-item.selected {
        background: var(--paper-subtle);
    }
    .doc-icon {
        font-size: 18px;
        opacity: 0.7;
    }
    .doc-info {
        display: flex;
        flex-direction: column;
        flex: 1;
        overflow: hidden;
    }
    .doc-name {
        font-size: 13px;
        font-weight: 500;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .doc-meta {
        font-size: 11px;
        color: var(--ink-light);
    }
    .warning-dot {
        font-size: 10px;
    }
    .empty {
        color: var(--ink-light);
        text-align: center;
        margin-top: 20px;
        font-style: italic;
    }

    /* Detail */
    .detail-panel {
        padding-left: var(--space-4);
        overflow-y: auto;
    }
    .empty-state {
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--ink-light);
        font-style: italic;
    }

    .doc-card {
        background: var(--paper-subtle);
        padding: var(--space-6);
        border-radius: var(--radius-xl);
        border: 1px solid var(--border-subtle);
    }
    .card-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: var(--space-6);
    }
    h2 {
        margin: 0;
        font-size: 20px;
        font-weight: 500;
    }
    .status-badge {
        font-size: 11px;
        background: var(--ink);
        color: var(--paper);
        padding: 4px 10px;
        border-radius: 12px;
    }

    .meta-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: var(--space-4);
        margin-bottom: var(--space-6);
    }
    .meta-item {
        display: flex;
        flex-direction: column;
    }
    .label {
        font-size: 10px;
        text-transform: uppercase;
        color: var(--ink-light);
    }
    .value {
        font-size: 14px;
        font-weight: 500;
    }

    .preview-box {
        background: var(--paper);
        padding: var(--space-4);
        border-radius: var(--radius-lg);
        margin-bottom: var(--space-6);
        min-height: 150px;
        border: 1px solid var(--border-medium);
    }
    .preview-text {
        font-family: var(--font-mono);
        font-size: 12px;
        color: var(--ink-light);
        line-height: 1.6;
    }

    .actions {
        display: flex;
        gap: var(--space-4);
    }
    .btn-primary {
        background: var(--ink);
        color: var(--paper);
        border: none;
        padding: 10px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
    }
    .btn-ghost {
        background: transparent;
        border: 1px solid var(--border-medium);
        padding: 10px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
    }
</style>
