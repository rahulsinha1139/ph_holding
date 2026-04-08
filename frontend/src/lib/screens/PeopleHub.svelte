<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime";
  import { toast } from "../stores/toasts";
  import {
    createEmployeeAccessLink,
    createEmployeeProfile,
    listEmployeeAccessLinks,
    listEmployeeContributionSummaries,
    listEmployeeProfiles,
    listEmployeeProjectAssignments,
    listLicenseKeys,
    reassignEmployeeManager,
    refreshCollaborativeWorkspace,
    setEmployeeEmploymentState,
    updateEmployeeProfile,
    type EmployeeAccessLink,
    type EmployeeContributionSummary,
    type EmployeeProfile,
    type LicenseKeySummary,
    type ProjectMember,
  } from "$lib/api/collaboration";

  type PeopleTab = "directory" | "org" | "contributions";

  let loading = true;
  let saving = false;
  let activeTab: PeopleTab = "directory";

  let employees: EmployeeProfile[] = [];
  let accessLinks: EmployeeAccessLink[] = [];
  let licenseKeys: LicenseKeySummary[] = [];
  let contributions: EmployeeContributionSummary[] = [];
  let projectAssignments: ProjectMember[] = [];

  let selectedEmployeeID = "";
  let directorySearch = "";

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

  let assignmentLoadToken = 0;

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
  }

  async function load(options: { refreshRemote?: boolean } = {}) {
    const { refreshRemote = false } = options;
    loading = true;
    try {
      if (refreshRemote) {
        await refreshCollaborativeWorkspace().catch(() => undefined);
      }
      const [employeeRows, linkRows, licenseRows, contributionRows] = await Promise.all([
        listEmployeeProfiles(false),
        listEmployeeAccessLinks(),
        listLicenseKeys(),
        listEmployeeContributionSummaries(),
      ]);

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

      await loadSelectedEmployeeData(selectedEmployeeID);
    } catch (err) {
      toast.danger(`Failed to load people hub: ${String(err)}`);
    } finally {
      loading = false;
    }
  }

  async function loadSelectedEmployeeData(employeeID = selectedEmployeeID) {
    const selected = employees.find((employee) => employee.id === employeeID) || null;
    applyEmployeeToForm(selected);

    if (!selected?.id) {
      projectAssignments = [];
      return;
    }

    const token = ++assignmentLoadToken;
    try {
      const assignments = await listEmployeeProjectAssignments(selected.id);
      if (token === assignmentLoadToken) {
        projectAssignments = assignments;
      }
    } catch (err) {
      if (token === assignmentLoadToken) {
        projectAssignments = [];
      }
      toast.warning(`Could not load project assignments: ${String(err)}`);
    }
  }

  async function handleCreate() {
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

    const nextIsActive = !(selectedEmployee.is_active !== false);
    const nextStatus = nextIsActive ? "active" : "inactive";

    try {
      await setEmployeeEmploymentState(selectedEmployee.id, nextIsActive, nextStatus);
      employmentStatus = nextStatus;
      await load();
      toast.success(nextIsActive ? "Employee reactivated" : "Employee deactivated");
    } catch (err) {
      toast.danger(`Failed to update employee status: ${String(err)}`);
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
      await createEmployeeAccessLink({
        employee_id: selectedEmployeeID,
        license_key: selectedLicenseKey,
        access_status: "active",
      });
      await load();
      selectedLicenseKey = availableLicenseKeys[0]?.key || "";
      toast.success("Employee linked to license");
    } catch (err) {
      toast.danger(`Failed to link employee access: ${String(err)}`);
    }
  }

  function selectEmployee(employeeID: string) {
    selectedEmployeeID = employeeID;
    loadSelectedEmployeeData(employeeID);
  }

  $: linkedLicenseKeys = new Set(accessLinks.map((link) => link.license_key));
  $: availableLicenseKeys = licenseKeys.filter((license) => !linkedLicenseKeys.has(license.key));
  $: selectedEmployee = employees.find((employee) => employee.id === selectedEmployeeID) || null;
  $: filteredEmployees = employees.filter((employee) => {
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
    load({ refreshRemote: true });
    EventsOn("employees:updated", load);
    EventsOn("tasks:updated", load);
    EventsOn("projects:updated", load);
  });

  onDestroy(() => {
    EventsOff("employees:updated");
    EventsOff("tasks:updated");
    EventsOff("projects:updated");
  });
</script>

<div class="page">
  <header class="header">
    <div>
      <h1>People.</h1>
      <p class="subtitle">Employee directory, reporting structure, access mapping, and contribution insight.</p>
    </div>
    <div class="tabs">
      <button class:active={activeTab === "directory"} on:click={() => (activeTab = "directory")}>Directory</button>
      <button class:active={activeTab === "org"} on:click={() => (activeTab = "org")}>Org</button>
      <button class:active={activeTab === "contributions"} on:click={() => (activeTab = "contributions")}>Contributions</button>
    </div>
  </header>

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
      <h2>Link Employee Access</h2>
      <span>{accessLinks.length} access links</span>
    </div>
    <div class="grid three">
      <select bind:value={selectedEmployeeID} on:change={() => loadSelectedEmployeeData(selectedEmployeeID)}>
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
          {availableLicenseKeys.length} unlinked license keys available.
        {/if}
      </div>
    </div>
    <div class="actions">
      <button on:click={handleLinkAccess}>Link Access</button>
    </div>
  </section>

  {#if activeTab === "directory"}
    <section class="content-grid directory-layout">
      <article class="panel">
        <div class="panel-head">
          <h2>Employee Directory</h2>
          <span>{filteredEmployees.length} visible</span>
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
                class="list-row selectable"
                on:click={() => selectEmployee(employee.id)}
              >
                <div>
                  <strong>{employee.full_name}</strong>
                  <div class="meta">{employee.employee_code} • {employee.department || "No department"}</div>
                </div>
                <div class="meta right">
                  <div>{employee.job_title || "No title"}</div>
                  <div>{employee.manager_name || employee.employment_status || "Active"}</div>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </article>

      <article class="panel detail-panel">
        <div class="panel-head">
          <h2>Employee Detail</h2>
          {#if selectedEmployee}
            <span>{selectedEmployee.is_active === false ? "Inactive" : "Active"}</span>
          {/if}
        </div>

        {#if !selectedEmployee}
          <div class="empty">Select an employee to manage their profile.</div>
        {:else}
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
            <button class="secondary" on:click={handleEmploymentToggle}>
              {selectedEmployee.is_active === false ? "Reactivate" : "Deactivate"}
            </button>
          </div>

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
        {/if}
      </article>
    </section>
  {/if}

  {#if activeTab === "org"}
    <section class="panel">
      <div class="panel-head">
        <h2>Org / Reporting</h2>
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
                  <button class="list-row selectable" on:click={() => { activeTab = "directory"; selectEmployee(employee.id); }}>
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
    </section>
  {/if}

  {#if activeTab === "contributions"}
    <section class="panel">
      <div class="panel-head">
        <h2>Assignments & Contribution</h2>
        <span>{sortedContributions.length} employees tracked</span>
      </div>
      {#if loading}
        <div class="empty">Loading contribution metrics...</div>
      {:else if sortedContributions.length === 0}
        <div class="empty">No contribution history yet.</div>
      {:else}
        <div class="contribution-grid">
          {#each sortedContributions as summary}
            <button class="contribution-card" on:click={() => { activeTab = "directory"; selectEmployee(summary.employee_id); }}>
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

              <div class="meta strip">
                {summary.manager_name || "No manager"} • {summary.primary_device_name || "No device linked"}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .page {
    padding: 24px;
    display: grid;
    gap: 20px;
  }

  .header,
  .panel-head,
  .card-head,
  .org-head,
  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  h1,
  h2,
  h3,
  p {
    margin: 0;
  }

  .subtitle,
  .meta,
  label span,
  .metric-row span {
    color: var(--text-secondary);
  }

  .tabs,
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
  .contribution-card:hover,
  .tabs button.active {
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

  .org-grid,
  .contribution-grid {
    margin-top: 16px;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  }

  .org-card,
  .contribution-card {
    padding: 16px;
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

    .header,
    .panel-head,
    .card-head,
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
