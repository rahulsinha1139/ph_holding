<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime";
  import { toast } from "../stores/toasts";
  import {
    createEmployeeProfile,
    getCurrentEmployeeContext,
    listEmployeeAccessLinks,
    listEmployeeContributionSummaries,
    listEmployeeProfiles,
    listEmployeeProjectAssignments,
    listEmployeeTasks,
    listLicenseKeys,
    reassignEmployeeLicenseAccess,
    reassignEmployeeManager,
    refreshCollaborativeWorkspace,
    requestEmployeeArchive,
    setEmployeeEmploymentState,
    updateEmployeeProfile,
    type EmployeeAccessLink,
    type EmployeeContributionSummary,
    type EmployeeProfile,
    type LicenseKeySummary,
    type ProjectMember,
    type CollaborativeTask,
  } from "$lib/api/collaboration";

  let loading = true;
  let saving = false;
  let remoteRefreshing = false;
  let archiveSubmitting = false;
  let currentRole = "";

  let employees: EmployeeProfile[] = [];
  let accessLinks: EmployeeAccessLink[] = [];
  let licenseKeys: LicenseKeySummary[] = [];
  let contributions: EmployeeContributionSummary[] = [];
  let projectAssignments: ProjectMember[] = [];
  let selectedEmployeeTasks: CollaborativeTask[] = [];

  let selectedEmployeeID = "";
  let directorySearch = "";
  let directoryStatusFilter: "active" | "archive" | "all" = "active";
  let contributionSection: HTMLElement | null = null;

  let createFullName = "";
  let createDepartment = "";
  let createJobTitle = "";
  let selectedLicenseKey = "";

  let fullName = "";
  let preferredName = "";
  let email = "";
  let phone = "";
  let department = "";
  let jobTitle = "";
  let managerEmployeeID = "";
  let employmentStatus = "active";
  let startDate = "";
  let emergencyContact = "";
  let notes = "";
  let archiveReason = "";

  let assignmentLoadToken = 0;
  let loadRequestSeq = 0;
  let selectedEmployeeLoading = false;

  function toDateInput(value?: string) {
    if (!value) return "";
    return value.slice(0, 10);
  }

  function toISODate(value: string) {
    const trimmed = value.trim();
    return trimmed ? `${trimmed}T00:00:00Z` : undefined;
  }

  function applyEmployeeToForm(employee: EmployeeProfile | null) {
    fullName = employee?.full_name || "";
    preferredName = employee?.preferred_name || "";
    email = employee?.email || "";
    phone = employee?.phone || "";
    department = employee?.department || "";
    jobTitle = employee?.job_title || "";
    managerEmployeeID = employee?.manager_employee_id || "";
    employmentStatus = employee?.employment_status || (employee?.is_active === false ? "inactive" : "active");
    startDate = toDateInput(employee?.start_date);
    emergencyContact = employee?.emergency_contact || "";
    notes = employee?.notes || "";
    archiveReason = employee?.archive_reason || "";
  }

  async function load(options: { refreshRemote?: boolean } = {}) {
    const { refreshRemote = false } = options;
    const requestSeq = ++loadRequestSeq;
    loading = employees.length === 0;
    try {
      const [employeeRows, linkRows, licenseRows, contributionRows] = await Promise.all([
        listEmployeeProfiles(false),
        listEmployeeAccessLinks(),
        listLicenseKeys(),
        listEmployeeContributionSummaries(),
      ]);

      if (requestSeq !== loadRequestSeq) {
        return;
      }

      employees = employeeRows;
      accessLinks = linkRows;
      licenseKeys = licenseRows;
      contributions = contributionRows;

      if (!selectedEmployeeID && employees.length > 0) {
        selectedEmployeeID = employees[0].id;
      } else if (selectedEmployeeID && !employees.some((employee) => employee.id === selectedEmployeeID)) {
        selectedEmployeeID = employees[0]?.id || "";
      }

      if (!selectedLicenseKey && availableLicenseKeys.length > 0) {
        selectedLicenseKey = availableLicenseKeys[0].key;
      }

      void loadSelectedEmployeeData(selectedEmployeeID);
    } catch (err) {
      if (requestSeq !== loadRequestSeq) {
        return;
      }
      toast.danger(`Failed to load people hub: ${String(err)}`);
    } finally {
      if (requestSeq === loadRequestSeq) {
        loading = false;
      }
      if (requestSeq === loadRequestSeq && refreshRemote) {
        void refreshPeopleFromRemote();
      }
    }
  }

  async function refreshPeopleFromRemote() {
    if (remoteRefreshing) return;
    remoteRefreshing = true;
    try {
      await refreshCollaborativeWorkspace();
      await load();
    } catch {
      // Local SQLite is the working view; sync failures surface through the sidebar status.
    } finally {
      remoteRefreshing = false;
    }
  }

  function formatBHD(value: number | undefined): string {
    return `${Number(value || 0).toLocaleString('en-US', { minimumFractionDigits: 3, maximumFractionDigits: 3 })} BHD`;
  }

  async function loadSelectedEmployeeData(employeeID = selectedEmployeeID) {
    const selected = employees.find((employee) => employee.id === employeeID) || null;
    applyEmployeeToForm(selected);

    if (!selected?.id) {
      projectAssignments = [];
      selectedEmployeeTasks = [];
      selectedEmployeeLoading = false;
      return;
    }

    const token = ++assignmentLoadToken;
    selectedEmployeeLoading = true;
    try {
      const [assignments, tasks] = await Promise.all([
        listEmployeeProjectAssignments(selected.id),
        listEmployeeTasks(selected.id, true),
      ]);
      if (token === assignmentLoadToken) {
        projectAssignments = assignments;
        selectedEmployeeTasks = tasks;
      }
    } catch (err) {
      if (token === assignmentLoadToken) {
        projectAssignments = [];
        selectedEmployeeTasks = [];
      }
      toast.warning(`Could not load selected employee work: ${String(err)}`);
    } finally {
      if (token === assignmentLoadToken) {
        selectedEmployeeLoading = false;
      }
    }
  }

  async function handleCreate() {
    if (!isAdmin) {
      toast.warning("Only admin can add employee profiles");
      return;
    }
    if (!createFullName.trim()) {
      toast.warning("Employee name is required");
      return;
    }

    try {
      const employee = await createEmployeeProfile({
        full_name: createFullName.trim(),
        department: createDepartment.trim(),
        job_title: createJobTitle.trim(),
      });
      createFullName = "";
      createDepartment = "";
      createJobTitle = "";
      selectedEmployeeID = employee.id;
      await load();
      toast.success("Employee profile created");
    } catch (err) {
      toast.danger(`Failed to create employee profile: ${String(err)}`);
    }
  }

  async function handleSaveProfile() {
    if (!selectedEmployee?.id) {
      toast.warning("Choose an employee first");
      return;
    }
    if (!fullName.trim()) {
      toast.warning("Employee name is required");
      return;
    }

    saving = true;
    try {
      await updateEmployeeProfile({
        id: selectedEmployee.id,
        employee_code: selectedEmployee.employee_code,
        full_name: fullName.trim(),
        preferred_name: preferredName.trim(),
        email: email.trim(),
        phone: phone.trim(),
        department: department.trim(),
        job_title: jobTitle.trim(),
        employment_status: employmentStatus,
        start_date: toISODate(startDate),
        emergency_contact: emergencyContact.trim(),
        notes: notes.trim(),
        is_active: selectedEmployee.is_active !== false,
      });

      if ((selectedEmployee.manager_employee_id || "") !== managerEmployeeID) {
        await reassignEmployeeManager(selectedEmployee.id, managerEmployeeID);
      }

      await load();
      toast.success("Employee profile updated");
    } catch (err) {
      toast.danger(`Failed to update employee profile: ${String(err)}`);
    } finally {
      saving = false;
    }
  }

  async function handleEmploymentToggle() {
    if (!selectedEmployee?.id) {
      toast.warning("Choose an employee first");
      return;
    }
    if (!isAdmin) {
      toast.warning("Only admin can reactivate employee profiles");
      return;
    }
    if (selectedEmployee.is_active !== false) {
      toast.warning("Use employee archive for removals");
      return;
    }

    try {
      await setEmployeeEmploymentState(selectedEmployee.id, true, "active");
      employmentStatus = "active";
      await load();
      toast.success("Employee reactivated");
    } catch (err) {
      toast.danger(`Failed to update employee status: ${String(err)}`);
    }
  }

  async function handleArchiveRequest() {
    if (!selectedEmployee?.id) {
      toast.warning("Choose an employee first");
      return;
    }
    if (!isAdmin) {
      toast.warning("Only admin can archive employees");
      return;
    }
    if (selectedEmployee.is_active === false) {
      toast.warning("This employee is already archived or inactive");
      return;
    }
    if (!archiveReason.trim()) {
      toast.warning("Archive reason is required");
      return;
    }

    archiveSubmitting = true;
    try {
      await requestEmployeeArchive(selectedEmployee.id, archiveReason.trim());
      toast.success("Employee archived");
      archiveReason = "";
      await load({ refreshRemote: true });
    } catch (err) {
      toast.danger(`Failed to archive employee: ${String(err)}`);
    } finally {
      archiveSubmitting = false;
    }
  }

  async function handleLinkAccess() {
    if (!selectedEmployeeID) {
      toast.warning("Choose an employee first");
      return;
    }
    if (!selectedLicenseKey) {
      toast.warning("Choose a license key to link");
      return;
    }

    try {
      await reassignEmployeeLicenseAccess(selectedEmployeeID, selectedLicenseKey, true);
      await load();
      selectedLicenseKey = availableLicenseKeys[0]?.key || "";
      toast.success("Employee license assignment updated");
    } catch (err) {
      toast.danger(`Failed to assign employee license: ${String(err)}`);
    }
  }

  function selectEmployee(employeeID: string, focusContribution = true) {
    selectedEmployeeID = employeeID;
    void loadSelectedEmployeeData(employeeID);
    if (focusContribution) {
      void tick().then(() => {
        contributionSection?.scrollIntoView({ behavior: "smooth", block: "start" });
      });
    }
  }

  function normalizeTaskStatus(status?: string) {
    return (status || "open").toLowerCase();
  }

  function formatTaskStatus(status?: string) {
    return normalizeTaskStatus(status).replaceAll("_", " ");
  }

  function isCompletedTask(task: CollaborativeTask) {
    const status = normalizeTaskStatus(task.status);
    return status === "completed" || status === "done" || status === "archived";
  }

  function isBlockedTask(task: CollaborativeTask) {
    return normalizeTaskStatus(task.status) === "blocked" || Boolean(task.blocked_reason);
  }

  function isOverdueTask(task: CollaborativeTask) {
    if (!task.due_date || isCompletedTask(task)) return false;
    const due = new Date(task.due_date);
    if (Number.isNaN(due.getTime())) return false;
    return due.getTime() < Date.now();
  }

  function formatShortDate(value?: string) {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    return date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
  }

  function isArchivedEmployee(employee: EmployeeProfile) {
    return employee.is_active === false || (employee.employment_status || "").toLowerCase() === "archived";
  }

  function employeeStatusLabel(employee: EmployeeProfile) {
    if ((employee.employment_status || "").toLowerCase() === "archived") return "Archived";
    if (employee.is_active === false) return "Inactive";
    return employee.employment_status || "Active";
  }

  function taskSortTime(task: CollaborativeTask) {
    const value = task.due_date || task.completed_at || task.last_comment_at || task.started_at || "";
    const parsed = value ? new Date(value).getTime() : 0;
    return Number.isNaN(parsed) ? 0 : parsed;
  }

  $: activeLinkedLicenseKeys = new Set(
    accessLinks
      .filter((link) => (link.access_status || "active").toLowerCase() !== "inactive")
      .map((link) => link.license_key)
  );
  $: availableLicenseKeys = licenseKeys.filter((license) => !activeLinkedLicenseKeys.has(license.key));
  $: selectedEmployee = employees.find((employee) => employee.id === selectedEmployeeID) || null;
  $: isAdmin = ["admin", "administrator", "developer"].includes(currentRole.toLowerCase());
  $: activeEmployeeCount = employees.filter((employee) => !isArchivedEmployee(employee)).length;
  $: archivedEmployeeCount = employees.filter(isArchivedEmployee).length;
  $: filteredEmployees = employees.filter((employee) => {
    if (directoryStatusFilter === "active" && isArchivedEmployee(employee)) return false;
    if (directoryStatusFilter === "archive" && !isArchivedEmployee(employee)) return false;
    const search = directorySearch.trim().toLowerCase();
    if (!search) return true;
    const haystack = [
      employee.full_name,
      employee.employee_code,
      employee.department,
      employee.job_title,
      employee.manager_name,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return haystack.includes(search);
  });
  $: selectedAccessLinks = accessLinks.filter((link) => link.employee_id === selectedEmployeeID);
  $: contributionByEmployee = new Map(contributions.map((summary) => [summary.employee_id, summary]));
  $: selectedContribution = contributionByEmployee.get(selectedEmployeeID) || null;
  $: selectedOpenTasks = selectedEmployeeTasks.filter((task) => !isCompletedTask(task));
  $: selectedCompletedTasks = selectedEmployeeTasks.filter(isCompletedTask);
  $: selectedBlockedTasks = selectedEmployeeTasks.filter(isBlockedTask);
  $: selectedOverdueTasks = selectedEmployeeTasks.filter(isOverdueTask);
  $: computedCompletionRate = selectedEmployeeTasks.length
    ? (selectedCompletedTasks.length / selectedEmployeeTasks.length) * 100
    : 0;
  $: selectedTaskTimeline = [...selectedEmployeeTasks].sort((left, right) => taskSortTime(right) - taskSortTime(left));
  $: selectedActiveTaskCount = selectedContribution?.active_task_count ?? selectedOpenTasks.length;
  $: selectedCompletedTaskCount = selectedContribution?.completed_task_count ?? selectedCompletedTasks.length;
  $: selectedBlockedTaskCount = selectedContribution?.blocked_task_count ?? selectedBlockedTasks.length;
  $: selectedOverdueTaskCount = selectedContribution?.overdue_task_count ?? selectedOverdueTasks.length;
  $: selectedCompletionRate = selectedContribution?.completion_rate ?? computedCompletionRate;
  $: sortedContributions = [...contributions].sort((left, right) => {
    if ((right.completed_task_count || 0) !== (left.completed_task_count || 0)) {
      return (right.completed_task_count || 0) - (left.completed_task_count || 0);
    }
    return (right.active_task_count || 0) - (left.active_task_count || 0);
  });
  $: orgGroups = employees.reduce((groups, employee) => {
    const managerLabel = employee.manager_name || "Leadership / Unassigned";
    if (!groups[managerLabel]) {
      groups[managerLabel] = [];
    }
    groups[managerLabel].push(employee);
    return groups;
  }, {} as Record<string, EmployeeProfile[]>);
  $: orgGroupEntries = Object.entries(orgGroups).sort((left, right) => left[0].localeCompare(right[0]));

  onMount(() => {
    getCurrentEmployeeContext().then((ctx) => {
      currentRole = ctx?.license_role || "";
    }).catch(() => undefined);
    void load({ refreshRemote: true });
    EventsOn("employees:updated", () => void load());
    EventsOn("tasks:updated", () => void load());
    EventsOn("projects:updated", () => void load());
  });

  onDestroy(() => {
    EventsOff("employees:updated");
    EventsOff("tasks:updated");
    EventsOff("projects:updated");
  });
</script>

<div class="page">
  {#if remoteRefreshing}
    <div class="page-status">
      <span class="status-chip">Syncing latest changes</span>
    </div>
  {/if}

  {#if isAdmin}
    <section class="composer">
      <div class="panel-head">
        <h2>Add Employee</h2>
        <span>{employees.length} total</span>
      </div>
      <div class="grid three">
        <input bind:value={createFullName} placeholder="Full name" />
        <input bind:value={createDepartment} placeholder="Department" />
        <input bind:value={createJobTitle} placeholder="Job title" />
      </div>
      <div class="actions">
        <button on:click={handleCreate}>Create Employee</button>
      </div>
    </section>

    <section class="composer">
      <div class="panel-head">
        <h2>Assign Employee Access</h2>
        <span>{accessLinks.length} access links</span>
      </div>
      <div class="grid three">
        <select bind:value={selectedEmployeeID} on:change={() => selectEmployee(selectedEmployeeID, false)}>
          <option value="">Select employee</option>
          {#each employees as employee}
            <option value={employee.id}>{employee.full_name} {employee.department ? `• ${employee.department}` : ""}</option>
          {/each}
        </select>

        <select bind:value={selectedLicenseKey}>
          <option value="">Select license key</option>
          {#each availableLicenseKeys as license}
            <option value={license.key}>
              {license.key} • {license.display_name || license.assigned_to || license.role}
            </option>
          {/each}
        </select>

        <div class="hint-card">
          {#if availableLicenseKeys.length === 0}
            All visible license keys are already linked.
          {:else}
            {availableLicenseKeys.length} available license keys.
          {/if}
        </div>
      </div>
      <div class="actions">
        <button on:click={handleLinkAccess}>Assign Access</button>
      </div>
    </section>
  {/if}

  <section class="content-grid directory-layout">
    <article class="panel">
      <div class="panel-head">
        <h2>Employee Directory</h2>
        <span>{activeEmployeeCount} active · {archivedEmployeeCount} archived</span>
      </div>
      <div class="filter-pills">
        <button class:active={directoryStatusFilter === "active"} on:click={() => directoryStatusFilter = "active"}>Active</button>
        <button class:active={directoryStatusFilter === "archive"} on:click={() => directoryStatusFilter = "archive"}>Archive</button>
        <button class:active={directoryStatusFilter === "all"} on:click={() => directoryStatusFilter = "all"}>All</button>
      </div>
      <input bind:value={directorySearch} placeholder="Search by name, code, department, or title" />
      {#if loading}
        <div class="empty">Loading employees...</div>
      {:else if filteredEmployees.length === 0}
        <div class="empty">No employee profiles match this search.</div>
      {:else}
        <div class="list">
          {#each filteredEmployees as employee}
            <button
              class:selected={employee.id === selectedEmployeeID}
              class:archived={isArchivedEmployee(employee)}
              class="list-row selectable"
              on:click={() => selectEmployee(employee.id)}
            >
              <div>
                <strong>{employee.full_name}</strong>
                <div class="meta">{employee.employee_code} • {employee.department || "No department"}</div>
              </div>
              <div class="meta right">
                <div>{employee.job_title || "No title"}</div>
                <div>{employee.manager_name || employeeStatusLabel(employee)}</div>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </article>

    <article class="panel detail-panel" bind:this={contributionSection}>
      <div class="panel-head">
        <h2>Employee Detail</h2>
        {#if selectedEmployee}
          <span>{selectedEmployee.is_active === false ? "Inactive" : "Active"}</span>
        {/if}
      </div>

      {#if !selectedEmployee}
        <div class="empty">Select an employee to manage their profile and contribution history.</div>
      {:else}
        <div class="employee-summary">
          <div>
            <h3>Contribution & Tasks</h3>
            <p class="meta">
              {selectedEmployee.full_name} • {selectedEmployee.department || "No department"} • {selectedEmployee.job_title || "No title"}
            </p>
          </div>
          <span class="status-chip" class:inactive={isArchivedEmployee(selectedEmployee)}>
            {employeeStatusLabel(selectedEmployee)}
          </span>
        </div>

        <div class="metric-row">
          <div>
            <span>Active Tasks</span>
            <strong>{selectedActiveTaskCount}</strong>
          </div>
          <div>
            <span>Completed</span>
            <strong>{selectedCompletedTaskCount}</strong>
          </div>
          <div>
            <span>Projects</span>
            <strong>{selectedContribution?.active_project_count ?? projectAssignments.filter((assignment) => assignment.is_active !== false).length}</strong>
          </div>
        </div>

        <div class="metric-row secondary-row">
          <div>
            <span>Blocked</span>
            <strong>{selectedBlockedTaskCount}</strong>
          </div>
          <div>
            <span>Overdue</span>
            <strong>{selectedOverdueTaskCount}</strong>
          </div>
          <div>
            <span>Completion</span>
            <strong>{selectedCompletionRate.toFixed(0)}%</strong>
          </div>
        </div>

        <div class="metric-row commercial-row">
          <div>
            <span>Opportunities YTD</span>
            <strong>{selectedContribution?.opportunity_ytd ?? 0}</strong>
          </div>
          <div>
            <span>Won / Lost</span>
            <strong>{selectedContribution?.opportunity_won_ytd ?? 0} / {selectedContribution?.opportunity_lost_ytd ?? 0}</strong>
          </div>
          <div>
            <span>Revenue YTD</span>
            <strong>{formatBHD(selectedContribution?.revenue_ytd)}</strong>
          </div>
        </div>

        <div class="subsection">
          <div class="panel-head compact">
            <h3>Task History</h3>
            <span>{selectedEmployeeTasks.length} records</span>
          </div>
          {#if selectedEmployeeLoading}
            <div class="empty small">Loading selected employee work...</div>
          {:else if selectedTaskTimeline.length === 0}
            <div class="empty small">No assigned task history yet.</div>
          {:else}
            <div class="task-list">
              {#each selectedTaskTimeline as task}
                <div class="task-row" class:completed={isCompletedTask(task)} class:blocked={isBlockedTask(task)}>
                  <div>
                    <strong>{task.title}</strong>
                    <div class="meta">
                      {task.task_type || "Task"}
                      {task.due_date ? ` • Due ${formatShortDate(task.due_date)}` : ""}
                      {task.completed_at ? ` • Completed ${formatShortDate(task.completed_at)}` : ""}
                    </div>
                  </div>
                  <span class="task-status">{formatTaskStatus(task.status)}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <div class="subsection">
          <div class="panel-head compact">
            <h3>Profile</h3>
            <span>{selectedEmployee.employee_code}</span>
          </div>
          <div class="grid two">
            <label>
              <span>Full Name</span>
              <input bind:value={fullName} placeholder="Full name" />
            </label>
            <label>
              <span>Preferred Name</span>
              <input bind:value={preferredName} placeholder="Preferred name" />
            </label>
            <label>
              <span>Email</span>
              <input bind:value={email} placeholder="Email" />
            </label>
            <label>
              <span>Phone</span>
              <input bind:value={phone} placeholder="Phone" />
            </label>
            <label>
              <span>Department</span>
              <input bind:value={department} placeholder="Department" />
            </label>
            <label>
              <span>Job Title</span>
              <input bind:value={jobTitle} placeholder="Job title" />
            </label>
            <label>
              <span>Manager</span>
              <select bind:value={managerEmployeeID}>
                <option value="">No manager</option>
                {#each employees.filter((employee) => employee.id !== selectedEmployee.id) as employee}
                  <option value={employee.id}>{employee.full_name}</option>
                {/each}
              </select>
            </label>
            <label>
              <span>Status</span>
              <select bind:value={employmentStatus}>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
                <option value="on_leave">On Leave</option>
                <option value="probation">Probation</option>
                <option value="contract">Contract</option>
              </select>
            </label>
            <label>
              <span>Start Date</span>
              <input bind:value={startDate} type="date" />
            </label>
            <label>
              <span>Emergency Contact</span>
              <input bind:value={emergencyContact} placeholder="Emergency contact" />
            </label>
          </div>

          <label class="stack">
            <span>Notes</span>
            <textarea bind:value={notes} placeholder="Role notes, responsibilities, or context"></textarea>
          </label>

          <div class="stat-grid">
            <div class="stat-card">
              <span>Projects</span>
              <strong>{projectAssignments.filter((assignment) => assignment.is_active !== false).length}</strong>
            </div>
            <div class="stat-card">
              <span>Licenses</span>
              <strong>{selectedAccessLinks.length}</strong>
            </div>
            <div class="stat-card">
              <span>Manager</span>
              <strong>{selectedEmployee.manager_name || "None"}</strong>
            </div>
          </div>

          <div class="actions">
            <button disabled={saving} on:click={handleSaveProfile}>Save Profile</button>
            {#if isAdmin && selectedEmployee.is_active === false}
              <button class="secondary" on:click={handleEmploymentToggle}>Reactivate</button>
            {/if}
          </div>

          {#if isAdmin && selectedEmployee.is_active !== false}
            <div class="archive-panel">
              <div>
                <strong>Employee Archive</strong>
                <p>Any admin can archive this employee. Linked tasks, offers, orders, invoices, and audit history stay attached to the archived profile.</p>
              </div>
              <textarea bind:value={archiveReason} placeholder="Reason for archive"></textarea>
              <div class="actions">
                <button class="danger" disabled={archiveSubmitting} on:click={handleArchiveRequest}>
                  {archiveSubmitting ? "Archiving..." : "Archive Employee"}
                </button>
              </div>
            </div>
          {:else if isArchivedEmployee(selectedEmployee)}
            <div class="archive-panel archived-panel">
              <div>
                <strong>Archived Employee</strong>
                <p>
                  {selectedEmployee.archive_reason || "Employee is inactive. Historical work remains linked to this profile."}
                  {selectedEmployee.archived_at ? ` Archived ${formatShortDate(selectedEmployee.archived_at)}.` : ""}
                </p>
              </div>
            </div>
          {/if}
        </div>

        <div class="detail-split">
          <div class="subsection">
            <div class="panel-head compact">
              <h3>Project Assignments</h3>
              <span>{projectAssignments.length} records</span>
            </div>
            {#if projectAssignments.length === 0}
              <div class="empty small">No project assignments yet.</div>
            {:else}
              <div class="list compact-list">
                {#each projectAssignments as assignment}
                  <div class="list-row">
                    <div>
                      <strong>{assignment.project_name || "Untitled project"}</strong>
                      <div class="meta">{assignment.role || "Member"}</div>
                    </div>
                    <div class="meta right">
                      <div>{assignment.is_active === false ? "Inactive" : "Active"}</div>
                      <div>{assignment.joined_at ? toDateInput(assignment.joined_at) : "No join date"}</div>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>

          <div class="subsection">
            <div class="panel-head compact">
              <h3>Access Mapping</h3>
              <span>{selectedAccessLinks.length} links</span>
            </div>
            {#if selectedAccessLinks.length === 0}
              <div class="empty small">No access links mapped to this employee yet.</div>
            {:else}
              <div class="list compact-list">
                {#each selectedAccessLinks as link}
                  <div class="list-row">
                    <div>
                      <strong>{link.license_key}</strong>
                      <div class="meta">{link.device_name || "No device mapped"}</div>
                    </div>
                    <div class="meta right">
                      <div>{link.access_status || "active"}</div>
                      <div>{link.is_primary ? "Primary" : "Secondary"}</div>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </article>
  </section>

  <section class="content-grid lower-layout">
    <article class="panel org-compact-panel">
      <div class="panel-head">
        <h2>Reporting Groups</h2>
        <span>{orgGroupEntries.length} reporting groups</span>
      </div>
      {#if loading}
        <div class="empty">Loading reporting structure...</div>
      {:else if orgGroupEntries.length === 0}
        <div class="empty">No reporting structure yet.</div>
      {:else}
        <div class="org-grid">
          {#each orgGroupEntries as [managerLabel, reports]}
            <div class="org-card">
              <div class="org-head">
                <strong>{managerLabel}</strong>
                <span>{reports.length} reports</span>
              </div>
              <div class="list compact-list">
                {#each reports as employee}
                  <button class="list-row selectable" on:click={() => selectEmployee(employee.id)}>
                    <div>
                      <strong>{employee.full_name}</strong>
                      <div class="meta">{employee.department || "No department"}</div>
                    </div>
                    <div class="meta right">
                      <div>{employee.job_title || "No title"}</div>
                      <div>{employee.is_active === false ? "Inactive" : "Active"}</div>
                    </div>
                  </button>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </article>

    <article class="panel team-overview-panel">
      <div class="panel-head">
        <h2>Team Contribution Overview</h2>
        <span>{sortedContributions.length} employees tracked</span>
      </div>
      {#if loading}
        <div class="empty">Loading contribution metrics...</div>
      {:else if sortedContributions.length === 0}
        <div class="empty">No contribution history yet.</div>
      {:else}
        <div class="contribution-grid">
          {#each sortedContributions as summary}
            <button class="contribution-card" on:click={() => selectEmployee(summary.employee_id)}>
              <div class="card-head">
                <div>
                  <strong>{summary.employee_name}</strong>
                  <div class="meta">
                    {summary.employee_code} • {summary.department || "No department"}
                  </div>
                </div>
                <span class:inactive={summary.is_active === false}>
                  {summary.is_active === false ? "Inactive" : "Active"}
                </span>
              </div>

              <div class="metric-row">
                <div>
                  <span>Active Tasks</span>
                  <strong>{summary.active_task_count}</strong>
                </div>
                <div>
                  <span>Completed</span>
                  <strong>{summary.completed_task_count}</strong>
                </div>
                <div>
                  <span>Projects</span>
                  <strong>{summary.active_project_count}</strong>
                </div>
              </div>

              <div class="metric-row secondary-row">
                <div>
                  <span>Blocked</span>
                  <strong>{summary.blocked_task_count}</strong>
                </div>
                <div>
                  <span>Overdue</span>
                  <strong>{summary.overdue_task_count}</strong>
                </div>
                <div>
                  <span>Completion</span>
                  <strong>{summary.completion_rate.toFixed(0)}%</strong>
                </div>
              </div>

              <div class="metric-row commercial-row">
                <div>
                  <span>Opps YTD</span>
                  <strong>{summary.opportunity_ytd}</strong>
                </div>
                <div>
                  <span>Won / Lost</span>
                  <strong>{summary.opportunity_won_ytd} / {summary.opportunity_lost_ytd}</strong>
                </div>
                <div>
                  <span>Revenue YTD</span>
                  <strong>{formatBHD(summary.revenue_ytd)}</strong>
                </div>
              </div>

              <div class="meta strip">
                {summary.manager_name || "No manager"} • {summary.primary_device_name || "No device linked"}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </article>
  </section>
</div>

<style>
  .page {
    padding: 24px;
    display: grid;
    gap: 20px;
  }

  .page-status,
  .panel-head,
  .card-head,
  .org-head,
  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .page-status {
    justify-content: flex-end;
  }

  h2,
  h3,
  p {
    margin: 0;
  }

  .meta,
  label span,
  .metric-row span {
    color: var(--text-secondary);
  }

  .actions {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
  }

  .composer,
  .panel,
  .org-card,
  .contribution-card,
  .stat-card,
  .hint-card {
    border: 1px solid var(--border);
    border-radius: 16px;
    background: var(--surface);
  }

  .composer,
  .panel {
    padding: 18px;
  }

  .grid,
  .content-grid,
  .stat-grid,
  .org-grid,
  .contribution-grid {
    display: grid;
    gap: 12px;
  }

  .grid {
    margin-top: 12px;
  }

  .grid.two {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .grid.three {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .directory-layout {
    grid-template-columns: minmax(320px, 0.9fr) minmax(420px, 1.1fr);
  }

  .lower-layout {
    grid-template-columns: minmax(0, 1.55fr) minmax(260px, 0.45fr);
    align-items: start;
  }

  .team-overview-panel {
    order: 1;
  }

  .org-compact-panel {
    order: 2;
  }

  .detail-split {
    display: grid;
    gap: 12px;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .list {
    display: grid;
    gap: 10px;
    margin-top: 14px;
  }

  .list-row {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 12px 14px;
    background: color-mix(in srgb, var(--surface) 92%, var(--accent-primary) 8%);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    text-align: left;
  }

  .selectable {
    cursor: pointer;
  }

  .selectable.selected,
  .selectable:hover,
  .contribution-card:hover {
    border-color: color-mix(in srgb, var(--accent-primary) 62%, var(--border) 38%);
    background: color-mix(in srgb, var(--surface) 76%, var(--accent-primary) 24%);
  }

  .detail-panel,
  .org-card,
  .contribution-card {
    display: grid;
    gap: 14px;
  }

  label {
    display: grid;
    gap: 6px;
    font-size: 0.92rem;
  }

  .stack {
    margin-top: 14px;
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
    border-radius: 12px;
    border: 1px solid var(--border);
    background: color-mix(in srgb, var(--surface) 94%, white 6%);
    color: var(--text-primary);
    padding: 12px 14px;
    box-sizing: border-box;
  }

  textarea {
    min-height: 110px;
    resize: vertical;
  }

  button {
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 10px 14px;
    background: color-mix(in srgb, var(--surface) 84%, var(--accent-primary) 16%);
    color: var(--text-primary);
  }

  button.secondary {
    background: color-mix(in srgb, var(--surface) 88%, #d97706 12%);
  }

  button.danger {
    background: color-mix(in srgb, var(--surface) 82%, #dc2626 18%);
    border-color: color-mix(in srgb, #dc2626 46%, var(--border) 54%);
    color: #7f1d1d;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.64;
  }

  .filter-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin: 12px 0;
  }

  .filter-pills button {
    border-radius: 999px;
    padding: 8px 12px;
    background: color-mix(in srgb, var(--surface) 92%, var(--accent-primary) 8%);
  }

  .filter-pills button.active {
    background: color-mix(in srgb, var(--accent-primary) 24%, var(--surface) 76%);
    border-color: color-mix(in srgb, var(--accent-primary) 62%, var(--border) 38%);
  }

  .list-row.archived {
    background: color-mix(in srgb, var(--surface) 86%, #92400e 14%);
  }

  .archive-panel {
    border: 1px solid color-mix(in srgb, #dc2626 24%, var(--border) 76%);
    border-radius: 14px;
    padding: 14px;
    display: grid;
    gap: 12px;
    background: color-mix(in srgb, var(--surface) 90%, #fee2e2 10%);
    margin-top: 12px;
  }

  .archive-panel p {
    margin-top: 6px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .archived-panel {
    border-color: color-mix(in srgb, #92400e 30%, var(--border) 70%);
    background: color-mix(in srgb, var(--surface) 88%, #fef3c7 12%);
  }

  .hint-card,
  .stat-card {
    padding: 14px;
  }

  .stat-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    margin-top: 12px;
  }

  .stat-card span,
  .metric-row span {
    display: block;
    font-size: 0.82rem;
  }

  .stat-card strong,
  .metric-row strong {
    display: block;
    margin-top: 6px;
    font-size: 1.1rem;
  }

  .employee-summary {
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 14px;
    background: color-mix(in srgb, var(--surface) 88%, var(--accent-primary) 12%);
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .status-chip,
  .task-status {
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 6px 10px;
    background: color-mix(in srgb, var(--surface) 84%, var(--accent-primary) 16%);
    color: var(--text-primary);
    font-size: 0.82rem;
    white-space: nowrap;
  }

  .task-list {
    display: grid;
    gap: 8px;
    max-height: 360px;
    overflow: auto;
    padding-right: 2px;
  }

  .task-row {
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 10px 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    background: color-mix(in srgb, var(--surface) 94%, var(--accent-primary) 6%);
  }

  .task-row.completed {
    opacity: 0.74;
  }

  .task-row.blocked {
    border-color: color-mix(in srgb, #dc2626 42%, var(--border) 58%);
  }

  .subsection {
    display: grid;
    gap: 10px;
    margin-top: 10px;
  }

  .compact {
    align-items: end;
  }

  .compact-list {
    margin-top: 0;
  }

  .empty {
    border: 1px dashed var(--border);
    border-radius: 14px;
    padding: 20px;
    color: var(--text-secondary);
    margin-top: 14px;
  }

  .empty.small {
    margin-top: 0;
    padding: 14px;
  }

  .org-grid {
    margin-top: 16px;
    grid-template-columns: 1fr;
    max-height: 560px;
    overflow: auto;
    padding-right: 2px;
  }

  .contribution-grid {
    margin-top: 16px;
    grid-template-columns: repeat(2, minmax(320px, 1fr));
  }

  .org-card,
  .contribution-card {
    padding: 16px;
  }

  .org-compact-panel .org-card {
    padding: 12px;
  }

  .org-compact-panel .list-row {
    border-radius: 10px;
    padding: 8px 10px;
  }

  .metric-row {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .secondary-row {
    padding-top: 4px;
    border-top: 1px solid var(--border);
  }

  .commercial-row {
    padding-top: 4px;
    border-top: 1px solid var(--border);
  }

  .commercial-row strong {
    font-size: 0.96rem;
    word-break: break-word;
  }

  .strip {
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }

  .inactive {
    color: #b45309;
  }

  .right {
    text-align: right;
  }

  @media (max-width: 1100px) {
    .directory-layout,
    .lower-layout,
    .detail-split,
    .grid.two,
    .grid.three,
    .stat-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    .page {
      padding: 16px;
    }

    .panel-head,
    .card-head,
    .employee-summary,
    .org-head,
    .actions {
      align-items: stretch;
      flex-direction: column;
    }

    .metric-row {
      grid-template-columns: 1fr;
    }
  }
</style>
