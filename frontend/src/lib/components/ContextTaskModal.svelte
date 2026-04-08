<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { toast } from "$lib/stores/toasts";
  import {
    createTask,
    listEmployeeProfiles,
    listProjects,
    type CollaborativeProject,
    type CollaborativeTask,
    type EmployeeProfile,
  } from "$lib/api/collaboration";

  export let open = false;
  export let title = "Create Task";
  export let subtitle = "";
  export let defaults: Partial<CollaborativeTask> & { seed_title?: string } = {};

  const dispatch = createEventDispatcher();

  let loading = false;
  let saving = false;
  let employees: EmployeeProfile[] = [];
  let projects: CollaborativeProject[] = [];

  let taskTitle = "";
  let description = "";
  let priority = "medium";
  let dueDate = "";
  let assigneeEmployeeId = "";
  let projectId = "";

  $: if (open) {
    initializeModal();
  }

  async function initializeModal() {
    if (loading || saving) return;
    loading = true;
    try {
      const [employeeRows, projectRows] = await Promise.all([
        listEmployeeProfiles(true),
        listProjects(true),
      ]);
      employees = employeeRows;
      projects = projectRows;

      taskTitle = defaults.seed_title || defaults.title || "";
      description = defaults.description || "";
      priority = defaults.priority || "medium";
      dueDate = defaults.due_date ? new Date(defaults.due_date).toISOString().slice(0, 10) : "";
      assigneeEmployeeId = defaults.assignee_employee_id || "";
      projectId = defaults.project_id || "";
    } catch (err) {
      toast.danger(`Failed to prepare task form: ${String(err)}`);
    } finally {
      loading = false;
    }
  }

  function close() {
    dispatch("close");
  }

  async function save() {
    if (!taskTitle.trim()) {
      toast.warning("Task title is required");
      return;
    }

    saving = true;
    try {
      const created = await createTask({
        title: taskTitle.trim(),
        description: description.trim(),
        priority,
        due_date: dueDate ? new Date(`${dueDate}T09:00:00`).toISOString() : undefined,
        assignee_employee_id: assigneeEmployeeId || undefined,
        project_id: projectId || undefined,
        customer_id: defaults.customer_id,
        opportunity_id: defaults.opportunity_id,
        order_id: defaults.order_id,
      });
      toast.success("Task created");
      dispatch("created", created);
      close();
    } catch (err) {
      toast.danger(`Failed to create task: ${String(err)}`);
    } finally {
      saving = false;
    }
  }
</script>

{#if open}
  <div
    class="modal-backdrop"
    role="presentation"
    tabindex="0"
    on:click={close}
    on:keydown={(event) => event.key === "Escape" && close()}
  >
    <div
      class="modal-card"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      on:click|stopPropagation
      on:keydown|stopPropagation
    >
      <div class="modal-head">
        <div>
          <h3>{title}</h3>
          {#if subtitle}
            <p>{subtitle}</p>
          {/if}
        </div>
        <button class="close-btn" on:click={close} aria-label="Close task modal">Close</button>
      </div>

      {#if loading}
        <div class="empty">Preparing task form...</div>
      {:else}
        <div class="form-grid">
          <input bind:value={taskTitle} placeholder="Task title" />
          <select bind:value={priority}>
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
            <option value="urgent">Urgent</option>
          </select>
          <input bind:value={dueDate} type="date" />
          <select bind:value={projectId}>
            <option value="">No project</option>
            {#each projects as project}
              <option value={project.id}>{project.name}</option>
            {/each}
          </select>
          <select bind:value={assigneeEmployeeId}>
            <option value="">Assign later</option>
            {#each employees as employee}
              <option value={employee.id}>{employee.full_name}{employee.department ? ` • ${employee.department}` : ""}</option>
            {/each}
          </select>
        </div>

        <textarea bind:value={description} rows="4" placeholder="Describe the work, expected outcome, blockers, or context"></textarea>

        <div class="actions">
          <button class="ghost" on:click={close} disabled={saving}>Cancel</button>
          <button class="primary" on:click={save} disabled={saving}>{saving ? "Creating..." : "Create Task"}</button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1200;
    padding: 20px;
  }

  .modal-card {
    width: min(720px, 100%);
    border-radius: 18px;
    border: 1px solid var(--border);
    background: var(--surface);
    padding: 20px;
    display: grid;
    gap: 16px;
  }

  .modal-head,
  .actions {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
  }

  h3,
  p {
    margin: 0;
  }

  p,
  .empty {
    color: var(--text-secondary);
  }

  .form-grid {
    display: grid;
    grid-template-columns: 2fr repeat(4, 1fr);
    gap: 12px;
  }

  input,
  select,
  textarea,
  button {
    font: inherit;
  }

  input,
  select,
  textarea {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 10px 12px;
    background: var(--bg-base);
  }

  button {
    border-radius: 10px;
    padding: 10px 14px;
  }

  .close-btn,
  .ghost {
    border: 1px solid var(--border);
    background: white;
  }

  .primary {
    border: none;
    background: var(--onyx);
    color: white;
  }

  @media (max-width: 900px) {
    .form-grid {
      grid-template-columns: 1fr;
    }

    .modal-head,
    .actions {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
