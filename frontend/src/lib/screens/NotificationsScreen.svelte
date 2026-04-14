<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
  import { toast } from "../stores/toasts";
  import { listNotifications, markNotificationAsRead, refreshCollaborativeWorkspace, type NotificationItem } from "$lib/api/collaboration";

  const pendingTaskStorageKey = "asymmflow.pendingCollaborativeTaskId";
  const NOTIFICATIONS_CACHE_TTL_MS = 20_000;

  type NotificationSnapshot = {
    notifications: NotificationItem[];
    savedAt: number;
  };

  const notificationSnapshots: Record<string, NotificationSnapshot | undefined> = {};

  let loading = true;
  let unreadOnly = false;
  let notifications: NotificationItem[] = [];
  let loadRequestSeq = 0;

  function snapshotKey() {
    return unreadOnly ? "unread" : "all";
  }

  function hasNotificationData() {
    return notifications.length > 0;
  }

  function restoreSnapshot() {
    const snapshot = notificationSnapshots[snapshotKey()];
    if (!snapshot) return false;
    notifications = snapshot.notifications;
    loading = false;
    return true;
  }

  function saveSnapshot() {
    notificationSnapshots[snapshotKey()] = {
      notifications: [...notifications],
      savedAt: Date.now(),
    };
  }

  function isSnapshotStale() {
    const snapshot = notificationSnapshots[snapshotKey()];
    return !snapshot || Date.now() - snapshot.savedAt > NOTIFICATIONS_CACHE_TTL_MS;
  }

  function applyLocalReadState(notificationID: string) {
    const readAt = new Date().toISOString();
    notifications = notifications.map((notification) =>
      notification.id === notificationID
        ? { ...notification, status: "read", read_at: readAt }
        : notification
    );
  }

  async function load(options: { refreshRemote?: boolean; silent?: boolean } = {}) {
    const { refreshRemote = false, silent = false } = options;
    const requestSeq = ++loadRequestSeq;
    const shouldShowLoading = !silent || !hasNotificationData();
    if (shouldShowLoading) {
      loading = true;
    }
    try {
      if (refreshRemote) {
        await refreshCollaborativeWorkspace().catch(() => undefined);
      }
      const nextNotifications = await listNotifications(100, unreadOnly);
      if (requestSeq !== loadRequestSeq) {
        return;
      }
      notifications = nextNotifications;
      saveSnapshot();
    } catch (err) {
      if (!silent || !hasNotificationData()) {
        toast.danger(`Failed to load notifications: ${String(err)}`);
      }
    } finally {
      if (requestSeq === loadRequestSeq && shouldShowLoading) {
        loading = false;
      }
    }
  }

  async function hydrateNotifications() {
    const restored = restoreSnapshot();
    if (!restored) {
      await load();
      await load({ refreshRemote: true, silent: true });
      return;
    }
    if (hasNotificationData()) {
      await load({ refreshRemote: isSnapshotStale(), silent: true });
      return;
    }
    await load({ refreshRemote: true, silent: true });
  }

  async function markRead(notificationID: string, options: { reload?: boolean } = {}) {
    const { reload = false } = options;
    applyLocalReadState(notificationID);
    saveSnapshot();
    try {
      await markNotificationAsRead(notificationID);
      if (reload) {
        await load({ silent: true });
      }
    } catch (err) {
      await load({ silent: true }).catch(() => undefined);
      toast.danger(`Failed to update notification: ${String(err)}`);
    }
  }

  function handleUnreadToggle() {
    const restored = restoreSnapshot();
    if (!restored) {
      void load();
      return;
    }
    void load({ refreshRemote: isSnapshotStale(), silent: true });
  }

  function parsePayload(notification: NotificationItem): Record<string, any> {
    if (!notification.action_payload) return {};
    try {
      return JSON.parse(notification.action_payload);
    } catch {
      return {};
    }
  }

  function actionLabel(notification: NotificationItem): string {
    const action = String(parsePayload(notification).action || "").replaceAll("_", " ").trim();
    if (!action) return notification.notification_type || "update";
    return action;
  }

  function senderLabel(notification: NotificationItem): string {
    return String(parsePayload(notification).actor_name || "System");
  }

  function bucketLabel(notification: NotificationItem): string {
    const value = notification.created_at ? new Date(notification.created_at) : new Date();
    if (Number.isNaN(value.getTime())) return "Recent";
    const today = new Date();
    const startToday = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startValue = new Date(value.getFullYear(), value.getMonth(), value.getDate());
    const diffDays = Math.round((startToday.getTime() - startValue.getTime()) / 86400000);
    if (diffDays === 0) return "Today";
    if (diffDays === 1) return "Yesterday";
    return value.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric", year: "numeric" });
  }

  function isTaskNotification(notification: NotificationItem): boolean {
    return notification.source_type === "task" || notification.notification_type === "task";
  }

  function notificationTone(notification: NotificationItem): string {
    if (notification.status === "read") return "read";
    if (isTaskNotification(notification)) return "task";
    return "system";
  }

  async function openNotification(notification: NotificationItem) {
    const payload = parsePayload(notification);
    const taskID = String(payload.task_id || notification.source_id || "").trim();

    if (notification.status !== "read") {
      void markRead(notification.id, { reload: false });
    }

    if (taskID) {
      sessionStorage.setItem(pendingTaskStorageKey, taskID);
      window.dispatchEvent(new CustomEvent("navigateToScreen", {
        detail: { screen: "work" },
      }));
      window.setTimeout(() => {
        window.dispatchEvent(new CustomEvent("openCollaborativeTask", {
          detail: { taskID },
        }));
      }, 80);
    }
  }

  $: groupedNotifications = notifications.reduce((groups: { label: string; items: NotificationItem[] }[], notification) => {
    const label = bucketLabel(notification);
    const existing = groups.find((group) => group.label === label);
    if (existing) {
      existing.items.push(notification);
    } else {
      groups.push({ label, items: [notification] });
    }
    return groups;
  }, []);

  onMount(() => {
    void hydrateNotifications();
    EventsOn("notifications:new", () => void load({ silent: true }));
    EventsOn("notifications:updated", () => void load({ silent: true }));
  });

  onDestroy(() => {
    EventsOff("notifications:new");
    EventsOff("notifications:updated");
  });
</script>

<div class="page">
  <header class="header">
    <div>
      <h1>Notifications.</h1>
      <p class="subtitle">Persistent cross-device collaboration events and task updates.</p>
    </div>
    <label class="toggle">
      <input type="checkbox" bind:checked={unreadOnly} on:change={handleUnreadToggle} />
      <span>Unread only</span>
    </label>
  </header>

  <section class="panel">
    {#if loading}
      <div class="empty">Loading notification feed...</div>
    {:else if notifications.length === 0}
      <div class="empty">No notifications in your feed.</div>
    {:else}
      <div class="feed">
        {#each groupedNotifications as group}
          <section class="day-group">
            <div class="day-label">{group.label}</div>
            <div class="group-stack">
              {#each group.items as notification}
                <article class="notification-card {notificationTone(notification)}" class:read={notification.status === "read"}>
                  <div class="card-top">
                    <div class="card-heading">
                      <div class="card-kicker">
                        <span class="pill source">{notification.source_type || "system"}</span>
                        <span class="pill action">{actionLabel(notification)}</span>
                        <span class="pill sender">{senderLabel(notification)}</span>
                      </div>
                      <h2>{notification.title}</h2>
                      <p>{notification.message}</p>
                    </div>
                    <span class="badge {notification.status === 'read' ? 'read' : 'unread'}">
                      {notification.status || "unread"}
                    </span>
                  </div>

                  {#if isTaskNotification(notification)}
                    <div class="task-preview">
                      <strong>{parsePayload(notification).task_title || "Task assigned"}</strong>
                      <span>{parsePayload(notification).project_name || "Opens in Work"}</span>
                    </div>
                  {/if}

                  <div class="card-meta">
                    <span>{notification.created_at ? new Date(notification.created_at).toLocaleString() : "Just now"}</span>
                    {#if notification.read_at}
                      <span>Read {new Date(notification.read_at).toLocaleString()}</span>
                    {/if}
                  </div>

                  <div class="card-actions">
                    {#if isTaskNotification(notification)}
                      <button class="primary-action" on:click={() => openNotification(notification)}>Open task</button>
                    {/if}
                    {#if notification.status !== "read"}
                      <button on:click={() => markRead(notification.id)}>Mark read</button>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          </section>
        {/each}
      </div>
    {/if}
  </section>
</div>

<style>
  .page {
    padding: 24px;
    display: grid;
    gap: 20px;
  }

  .header,
  .card-top,
  .card-meta,
  .card-actions {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
  }

  h1,
  h2 {
    margin: 0;
  }

  .subtitle,
  .card-meta {
    color: var(--text-secondary);
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: 16px;
    background: var(--surface);
    padding: 18px;
  }

  .feed {
    display: grid;
    gap: 18px;
  }

  .day-group,
  .group-stack {
    display: grid;
    gap: 12px;
  }

  .day-label {
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .notification-card {
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 14px;
    background: linear-gradient(180deg, var(--bg-base) 0%, rgba(255, 255, 255, 0.94) 100%);
    display: grid;
    gap: 10px;
  }

  .notification-card.read {
    opacity: 0.78;
  }

  .notification-card.task {
    border-color: rgba(14, 116, 144, 0.22);
    box-shadow: 0 14px 32px rgba(15, 23, 42, 0.04);
  }

  .notification-card.system {
    border-color: rgba(148, 163, 184, 0.3);
  }

  .notification-card p {
    margin: 6px 0 0;
    color: var(--text-secondary);
  }

  .card-heading {
    display: grid;
    gap: 8px;
  }

  .card-kicker {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .pill {
    border-radius: 999px;
    padding: 5px 10px;
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    background: rgba(148, 163, 184, 0.14);
    color: var(--text-secondary);
  }

  .pill.action {
    background: rgba(14, 116, 144, 0.12);
    color: #0f766e;
  }

  .pill.sender {
    background: rgba(17, 24, 39, 0.08);
    color: var(--text-primary);
  }

  .task-preview {
    display: grid;
    gap: 4px;
    padding: 12px 14px;
    border-radius: 12px;
    background: rgba(14, 116, 144, 0.06);
  }

  .task-preview span {
    color: var(--text-secondary);
    font-size: 13px;
  }

  .badge {
    border-radius: 999px;
    padding: 6px 10px;
    font-size: 12px;
    text-transform: capitalize;
  }

  .badge.unread {
    background: #e0f2fe;
    color: #075985;
  }

  .badge.read {
    background: #f3f4f6;
    color: #4b5563;
  }

  button {
    font: inherit;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 8px 12px;
    background: white;
  }

  .primary-action {
    background: #111827;
    color: white;
    border-color: #111827;
  }

  .toggle {
    display: flex;
    gap: 8px;
    align-items: center;
    color: var(--text-secondary);
  }

  .empty {
    padding: 20px 8px;
    text-align: center;
    color: var(--text-secondary);
  }

  @media (max-width: 900px) {
    .header,
    .card-top,
    .card-meta,
    .card-actions {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
