<script lang="ts">
    import { onMount, onDestroy, createEventDispatcher } from "svelte";
    import { fly } from "svelte/transition";
    import {
        ChatWithButler,
        CreateConversation,
        ListConversations,
        GetConversationMessages,
        ChatWithButlerPersistent,
        DeleteConversation
    } from "../../../wailsjs/go/main/App";
    import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";

    const dispatch = createEventDispatcher();

    let messages = [];
    let userInput = "";
    let loading = false;
    let confidence = 0.85;
    let insights = [];

    // Conversation persistence
    let conversations: any[] = [];
    let activeConversationId = "";
    let showConversations = true;

    function addMessage(role, text, actions = []) {
        messages = [...messages, { id: Date.now(), role, text, actions }];
        scrollToBottom();
    }

    function scrollToBottom() {
        setTimeout(() => {
            const el = document.getElementById("chat-feed");
            if (el) el.scrollTop = el.scrollHeight;
        }, 100);
    }

    async function send() {
        if (!userInput.trim()) return;
        const txt = userInput;
        userInput = "";
        addMessage("user", txt);
        loading = true;

        try {
            if (!window.go) {
                await new Promise((r) => setTimeout(r, 1000));
                addMessage("butler", "I have noted your request. Is there anything else?");
            } else {
                // Use persistent chat if we have a conversation
                const res = await ChatWithButlerPersistent(activeConversationId, txt);
                if (res && res.response) {
                    addMessage("butler", res.response, res.actions || []);
                    if (res.conversation_id && !activeConversationId) {
                        activeConversationId = res.conversation_id;
                        await loadConversations();
                    }
                } else if (res && res.response) {
                    // Fallback for response without conversation_id
                    addMessage("butler", res.response, res.actions || []);
                }
                if (res?.confidence) confidence = res.confidence;
            }
        } catch (e) {
            // Fallback to non-persistent chat
            try {
                const res = await ChatWithButler(txt);
                if (res && res.message) {
                    addMessage("butler", res.message, res.actions || []);
                }
            } catch {
                addMessage("butler", "I apologize, I encountered an error processing that.");
            }
        } finally {
            loading = false;
        }
    }

    async function loadConversations() {
        try {
            conversations = await ListConversations() || [];
        } catch {
            conversations = [];
        }
    }

    async function selectConversation(conv: any) {
        activeConversationId = conv.id;
        messages = [];
        try {
            const msgs = await GetConversationMessages(conv.id);
            if (msgs && msgs.length > 0) {
                for (const m of msgs) {
                    addMessage(m.role === "user" ? "user" : "butler", m.content);
                }
            }
        } catch {
            addMessage("butler", "Failed to load conversation history.");
        }
    }

    async function startNewConversation() {
        activeConversationId = "";
        messages = [];
        addMessage("butler", "Good day. How can I help you?");
        await loadConversations();
    }

    async function deleteConv(convId: string) {
        if (!confirm('Delete this conversation? This cannot be undone.')) return;
        try {
            await DeleteConversation(convId);
            if (activeConversationId === convId) {
                startNewConversation();
            }
            await loadConversations();
        } catch {
            // Ignore delete errors
        }
    }

    function handleButlerEvent(e) {
        const msg = `Observation: ${e.Type} at ${e.Path}`;
        insights = [
            { id: Date.now(), text: msg, type: "observation" },
            ...insights,
        ].slice(0, 10);
    }

    function handleAction(action) {
        if (action.type === "navigate") {
            dispatch("navigate", { screen: action.target });
        } else if (action.type === "analyze" || action.type === "fetch") {
            userInput = `Tell me more about ${action.target}`;
            send();
        }
    }

    onMount(async () => {
        await loadConversations();
        addMessage("butler", "Good day. I can answer questions about customers, suppliers, financials, and operations. I can also generate detailed PDF intelligence reports on request.");
        if (window.runtime) EventsOn("butler:event", handleButlerEvent);
    });

    onDestroy(() => {
        if (window.runtime) EventsOff("butler:event");
    });
</script>

<div class="butler-layout">
    <!-- Conversations Sidebar -->
    <div class="conv-sidebar">
        <button class="new-conv-btn" on:click={startNewConversation}>+ New Chat</button>
        <div class="conv-list">
            {#each conversations as conv (conv.id)}
                <div
                    class="conv-item"
                    class:active={activeConversationId === conv.id}
                    on:click={() => selectConversation(conv)}
                >
                    <span class="conv-title">{conv.title || 'Untitled'}</span>
                    <button class="conv-delete" on:click|stopPropagation={() => deleteConv(conv.id)}>&times;</button>
                </div>
            {/each}
            {#if conversations.length === 0}
                <p class="conv-empty">No conversations yet</p>
            {/if}
        </div>
    </div>

    <!-- Main Chat Panel -->
    <div class="chat-panel">
        <div class="messages" id="chat-feed">
            {#each messages as m (m.id)}
                <div class="msg-row {m.role}" in:fly={{ y: 8, duration: 200 }}>
                    {#if m.role === "butler"}
                        <div class="avatar">B</div>
                    {/if}
                    <div class="bubble-wrap">
                        <div class="bubble">{m.text}</div>
                        {#if m.actions && m.actions.length > 0}
                            <div class="actions-row">
                                {#each m.actions as action}
                                    <button class="action-chip" on:click={() => handleAction(action)}>
                                        {#if action.type === "navigate"}
                                            <span class="chip-icon">&rarr;</span>
                                        {:else if action.type === "analyze"}
                                            <span class="chip-icon">&bull;</span>
                                        {:else}
                                            <span class="chip-icon">+</span>
                                        {/if}
                                        <span>{action.data || action.target}</span>
                                    </button>
                                {/each}
                            </div>
                        {/if}
                    </div>
                </div>
            {/each}
            {#if loading}
                <div class="msg-row butler">
                    <div class="avatar">B</div>
                    <div class="bubble typing">
                        <span class="dot-pulse"></span>
                    </div>
                </div>
            {/if}
        </div>

        <div class="input-area">
            <input
                class="chat-input"
                type="text"
                bind:value={userInput}
                placeholder="Ask about financials, customers, suppliers..."
                on:keydown={(e) => e.key === "Enter" && send()}
                disabled={loading}
            />
            <button class="send-btn" on:click={send} disabled={loading || !userInput}>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="12" y1="19" x2="12" y2="5"/>
                    <polyline points="5 12 12 5 19 12"/>
                </svg>
            </button>
        </div>
    </div>

</div>

<style>
    .butler-layout {
        display: grid;
        grid-template-columns: 200px 1fr;
        gap: 16px;
        flex: 1;
        min-height: 0;
        height: calc(100vh - 180px);
    }

    /* Conversations Sidebar */
    .conv-sidebar {
        background: var(--surface, #fff);
        border-radius: var(--border-radius, 8px);
        border: 1px solid var(--border, #E5E5E5);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .new-conv-btn {
        padding: 12px 16px;
        background: var(--carbon, #000);
        color: #fff;
        border: none;
        font-size: 13px;
        font-weight: 500;
        cursor: pointer;
        transition: opacity 0.15s;
    }

    .new-conv-btn:hover {
        opacity: 0.85;
    }

    .conv-list {
        flex: 1;
        overflow-y: auto;
        padding: 8px;
    }

    .conv-item {
        display: flex;
        align-items: center;
        padding: 10px 12px;
        border-radius: 6px;
        cursor: pointer;
        font-size: 13px;
        color: var(--steel, #86868B);
        transition: all 0.15s;
    }

    .conv-item:hover {
        background: var(--ether, #F5F5F7);
        color: var(--onyx, #1D1D1F);
    }

    .conv-item.active {
        background: var(--ether, #F5F5F7);
        color: var(--onyx, #1D1D1F);
        font-weight: 500;
    }

    .conv-title {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .conv-delete {
        opacity: 0;
        background: none;
        border: none;
        color: var(--steel, #86868B);
        font-size: 16px;
        cursor: pointer;
        padding: 2px 6px;
        border-radius: 4px;
    }

    .conv-item:hover .conv-delete {
        opacity: 1;
    }

    .conv-delete:hover {
        color: #d00;
    }

    .conv-empty {
        padding: 16px;
        text-align: center;
        font-size: 12px;
        color: var(--steel, #86868B);
    }

    /* Chat Panel */
    .chat-panel {
        background: var(--surface);
        border-radius: var(--border-radius);
        border: 1px solid var(--border);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .messages {
        flex: 1;
        overflow-y: auto;
        padding: 20px;
        display: flex;
        flex-direction: column;
        gap: 14px;
        scroll-behavior: smooth;
    }

    .msg-row {
        display: flex;
        gap: 10px;
        align-items: flex-end;
        max-width: 75%;
    }

    .msg-row.user {
        align-self: flex-end;
        flex-direction: row-reverse;
    }

    .avatar {
        width: 26px;
        height: 26px;
        background: var(--carbon);
        color: var(--canvas);
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 11px;
        font-weight: 600;
        flex-shrink: 0;
    }

    .bubble-wrap {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .bubble {
        padding: 10px 14px;
        border-radius: 14px;
        font-size: 13px;
        line-height: 1.55;
    }

    .msg-row.butler .bubble {
        background: var(--surface-elevated);
        border: 1px solid var(--border);
        border-bottom-left-radius: 4px;
        color: var(--text-primary);
    }

    .msg-row.user .bubble {
        background: var(--carbon);
        color: var(--canvas);
        border-bottom-right-radius: 4px;
    }

    .typing {
        display: flex;
        align-items: center;
        padding: 12px 16px;
    }

    .dot-pulse {
        width: 6px;
        height: 6px;
        background: var(--text-muted);
        border-radius: 50%;
        animation: pulse 1s infinite;
    }

    @keyframes pulse {
        0%, 100% { opacity: 0.3; }
        50% { opacity: 1; }
    }

    .actions-row {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
    }

    .action-chip {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 4px 10px;
        font-size: 11px;
        font-family: var(--font-family);
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 100px;
        cursor: pointer;
        color: var(--text-secondary);
        transition: all var(--transition-fast);
    }

    .action-chip:hover {
        background: var(--carbon);
        color: var(--canvas);
        border-color: var(--carbon);
    }

    .chip-icon {
        font-weight: 700;
        font-size: 12px;
    }

    /* Input Area */
    .input-area {
        padding: 12px 16px;
        border-top: 1px solid var(--border);
        display: flex;
        gap: 10px;
        align-items: center;
        background: var(--surface);
    }

    .chat-input {
        flex: 1;
        border: none;
        outline: none;
        font-size: 13px;
        background: transparent;
        font-family: var(--font-family);
        color: var(--text-primary);
    }

    .chat-input::placeholder {
        color: var(--text-muted);
    }

    .send-btn {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        background: var(--carbon);
        color: var(--canvas);
        border: none;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: opacity var(--transition-fast);
    }

    .send-btn:disabled {
        opacity: 0.3;
        cursor: default;
    }

    /* Side Panel */
    .side-panel {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .panel-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--border-radius);
        padding: 14px;
    }

    .panel-label {
        font-size: var(--label-size);
        font-weight: var(--label-weight);
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.08em;
        margin: 0 0 10px;
    }

    .meter-track {
        height: 4px;
        background: var(--surface-elevated);
        border-radius: 2px;
        overflow: hidden;
        margin-bottom: 6px;
    }

    .meter-fill {
        height: 100%;
        background: var(--carbon);
        border-radius: 2px;
        transition: width 0.3s var(--easing-smooth);
    }

    .meter-value {
        font-size: 11px;
        color: var(--text-muted);
        font-variant-numeric: tabular-nums;
    }

    .empty-feed {
        font-size: 12px;
        color: var(--text-muted);
        font-style: italic;
        margin: 0;
    }

    .feed-list {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .feed-item {
        font-size: 11px;
        padding: 6px 8px;
        background: var(--surface-elevated);
        border-radius: 6px;
        color: var(--text-primary);
        border: 1px solid var(--border);
    }

    /* Prompt Chips */
    .prompts {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .prompt-chip {
        width: 100%;
        text-align: left;
        padding: 8px 12px;
        font-size: 12px;
        font-family: var(--font-family);
        background: var(--surface-elevated);
        border: 1px solid var(--border);
        border-radius: 8px;
        cursor: pointer;
        color: var(--text-secondary);
        transition: all var(--transition-fast);
    }

    .prompt-chip:hover {
        border-color: var(--onyx);
        color: var(--text-primary);
        background: var(--surface);
    }
</style>
