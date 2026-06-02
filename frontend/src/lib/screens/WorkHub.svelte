<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
  import { ListCustomerContacts, ListCustomers } from "../../../wailsjs/go/main/App";
  import { toast } from "../stores/toasts";
  import Modal from "$lib/components/layout/Modal.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import WabiSpinner from "$lib/components/ui/WabiSpinner.svelte";
  import {
    addProjectMember,
    addTaskComment,
    archiveProject,
    createProject,
    createTask,
    deleteProject,
    deleteTask,
    getCurrentEmployeeContext,
    getTask,
    listEmployeeProfiles,
    listProjectActivity,
    listMyTasks,
    listProjectMembers,
    listProjectTasks,
    listProjects,
    listTaskActivity,
    listTaskComments,
    listTeamTasks,
    refreshCollaborativeWorkspace,
    reassignTask,
    shelveProject,
    updateProject,
    updateTask,
    updateTaskDueDate,
    updateTaskStatus,
    type CollaborativeProject,
    type CollaborativeTask,
    type CurrentEmployeeContext,
    type EmployeeProfile,
    type ProjectMember,
    type TaskActivityItem,
    type TaskCommentItem,
  } from "$lib/api/collaboration";

  type WorkTab = "my_work" | "team_board" | "projects";
  const pendingTaskStorageKey = "asymmflow.pendingCollaborativeTaskId";
  const pendingTaskPayloadStorageKey = "asymmflow.pendingCollaborativeTaskPayload";
  const WORK_HUB_CACHE_TTL_MS = 30_000;

  type WorkHubSnapshot = {
    currentEmployee: CurrentEmployeeContext | null;
    myTasks: CollaborativeTask[];
    teamTasks: CollaborativeTask[];
    projects: CollaborativeProject[];
    employees: EmployeeProfile[];
    selectedProjectId: string;
    selectedProjectForTask: string;
    selectedAssignee: string;
    selectedTaskId: string;
    activeTab: WorkTab;
    initializedPreferredTab: boolean;
    savedAt: number;
  };

  let workHubSnapshot: WorkHubSnapshot | null = null;

  let loading = true;
  let savingTask = false;
  let savingProject = false;
  let savingComment = false;
  let savingMember = false;
  let savingProjectAdmin = false;
  let savingAssignment = false;
  let savingDueDate = false;
  let savingTaskDetails = false;
  let deletingTask = false;
  let confirmDeleteTask = false;
  let remoteRefreshing = false;

  let activeTab: WorkTab = "team_board";
  let currentEmployee: CurrentEmployeeContext | null = null;
  let myTasks: CollaborativeTask[] = [];
  let teamTasks: CollaborativeTask[] = [];
  let projects: CollaborativeProject[] = [];
  let employees: EmployeeProfile[] = [];
  let projectMembers: ProjectMember[] = [];
  let projectTasks: CollaborativeTask[] = [];
  let projectActivity: TaskActivityItem[] = [];
  let selectedTask: CollaborativeTask | null = null;
  let taskComments: TaskCommentItem[] = [];
  let taskActivity: TaskActivityItem[] = [];
  let taskModalOpen = false;
  let projectModalOpen = false;

  let selectedTaskId = "";
  let selectedProjectId = "";

  let taskTitle = "";
  let taskDescription = "";
  let taskPriority = "medium";
  let taskDueDate = "";
  let selectedProjectForTask = "";
  let selectedAssignee = "";

  let projectName = "";
  let projectType = "internal";
  let projectDescription = "";
  let projectCustomerID = "";
  let projectCustomerName = "";
  let projectEndUserName = "";
  let projectOpportunityKey = "";
  let projectPocID = "";
  let projectPocName = "";
  let projectPocEmail = "";
  let projectPocPhone = "";
  let projectCustomers: any[] = [];
  let projectContacts: any[] = [];
  let loadingProjectCustomers = false;
  let projectCustomerSearch = "";
  let showProjectCustomerSuggestions = false;
  let editingProject = false;
  let projectEditDraft: Partial<CollaborativeProject> = {};
  let projectAdminReason = "";

  let memberSelections: string[] = [];
  let memberRole = "Member";
  let commentDraft = "";
  let selectedTaskAssigneeDraft = "";
  let selectedTaskDueDateDraft = "";
  let selectedTaskBlockerDraft = "";
  let selectedTaskTitleDraft = "";
  let selectedTaskDescriptionDraft = "";
  let selectedTaskPriorityDraft = "medium";
  let teamSearch = "";
  let teamFocusFilter = "all";
  let teamAssigneeFilter = "all";
  let draggedEmployeeId = "";
  let draggedOverTaskId = "";
  let initializedPreferredTab = false;
  let loadRequestSeq = 0;
  let projectContextRequestSeq = 0;
  let taskDetailRequestSeq = 0;
  let pendingOpenTaskId = "";
  let openingPendingTask = false;
  let lastLoadFailed = false;
  let lastLoadError = "";
  let boardRecoveryAttempts = 0;
  let projectContextLoading = false;
  let projectContextLoadedFor = "";
  let projectContextFailedFor = "";
  let taskDetailLoading = false;

  const boardColumns = [
    { key: "open", label: "Open" },
    { key: "in_progress", label: "In Progress" },
    { key: "blocked", label: "Blocked" },
    { key: "completed", label: "Completed" },
  ];

  function defaultWorkTabForRole(role?: string): WorkTab {
    const normalized = (role || "").toLowerCase();
    if (["admin", "manager", "developer"].includes(normalized)) {
      return "team_board";
    }
    return "my_work";
  }

  function hasLoadedWorkspaceData() {
    return Boolean(
      currentEmployee
      || myTasks.length
      || teamTasks.length
      || projects.length
      || employees.length,
    );
  }

  function restoreSnapshot() {
    if (!workHubSnapshot) return false;
    currentEmployee = workHubSnapshot.currentEmployee;
    myTasks = workHubSnapshot.myTasks;
    teamTasks = workHubSnapshot.teamTasks;
    projects = workHubSnapshot.projects;
    employees = workHubSnapshot.employees;
    selectedProjectId = workHubSnapshot.selectedProjectId;
    selectedProjectForTask = workHubSnapshot.selectedProjectForTask;
    selectedAssignee = workHubSnapshot.selectedAssignee;
    selectedTaskId = workHubSnapshot.selectedTaskId;
    activeTab = workHubSnapshot.activeTab;
    initializedPreferredTab = workHubSnapshot.initializedPreferredTab;
    loading = false;
    return true;
  }

  function saveSnapshot() {
    workHubSnapshot = {
      currentEmployee,
      myTasks: [...myTasks],
      teamTasks: [...teamTasks],
      projects: [...projects],
      employees: [...employees],
      selectedProjectId,
      selectedProjectForTask,
      selectedAssignee,
      selectedTaskId,
      activeTab,
      initializedPreferredTab,
      savedAt: Date.now(),
    };
  }

  function isSnapshotStale() {
    return !workHubSnapshot || Date.now() - workHubSnapshot.savedAt > WORK_HUB_CACHE_TTL_MS;
  }

  function wait(ms: number) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  async function waitForWorkspaceLoad(timeoutMs = 1500) {
    const startedAt = Date.now();
    while (loading && Date.now() - startedAt < timeoutMs) {
      await wait(40);
    }
  }

  async function load(options: { refreshRemote?: boolean; forceRemote?: boolean; silent?: boolean } = {}) {
    const { refreshRemote = false, forceRemote = false, silent = false } = options;
    const requestSeq = ++loadRequestSeq;
    const shouldShowLoading = !silent || !hasLoadedWorkspaceData();
    if (shouldShowLoading) {
      loading = true;
    }
    try {
      if (refreshRemote) {
        remoteRefreshing = true;
        await refreshCollaborativeWorkspace({ force: forceRemote }).catch(() => undefined);
      }
      const [employee, myTaskRows, teamTaskRows, projectRows, employeeRows] = await Promise.all([
        getCurrentEmployeeContext(),
        listMyTasks(false),
        listTeamTasks(true),
        listProjects(true),
        listEmployeeProfiles(true),
      ]);

      if (requestSeq !== loadRequestSeq) {
        return;
      }

      currentEmployee = employee;
      myTasks = myTaskRows || [];
      teamTasks = teamTaskRows || [];
      projects = projectRows || [];
      employees = employeeRows || [];
      lastLoadFailed = false;
      lastLoadError = "";
      boardRecoveryAttempts = 0;

      if (!selectedAssignee && currentEmployee?.employee_id) {
        selectedAssignee = currentEmployee.employee_id;
      }
      if (!selectedProjectId && projects.length > 0) {
        selectedProjectId = projects[0].id;
      }
      if (!selectedProjectForTask && selectedProjectId) {
        selectedProjectForTask = selectedProjectId;
      }
      if (!selectedTaskId) {
        selectedTaskId = myTasks[0]?.id || teamTasks[0]?.id || "";
      }
      if (!initializedPreferredTab && currentEmployee) {
        activeTab = defaultWorkTabForRole(currentEmployee.license_role);
        initializedPreferredTab = true;
      }
      saveSnapshot();
    } catch (err) {
      if (requestSeq !== loadRequestSeq) {
        return;
      }
      lastLoadFailed = true;
      lastLoadError = String(err);
      if (!silent || !hasLoadedWorkspaceData()) {
        toast.danger(`Failed to load work hub: ${String(err)}`);
      }
    } finally {
      if (requestSeq === loadRequestSeq && shouldShowLoading) {
        loading = false;
      }
      if (requestSeq === loadRequestSeq && refreshRemote) {
        remoteRefreshing = false;
      }
    }
  }

  async function hydrateWorkHub() {
    const restored = restoreSnapshot();
    if (!restored) {
      await load();
      await load({ refreshRemote: true, silent: true });
      return;
    }
    if (hasLoadedWorkspaceData()) {
      await load({ refreshRemote: isSnapshotStale(), silent: true });
      return;
    }
    await load({ refreshRemote: true, silent: true });
  }

  async function recoverBoardIfNeeded() {
    if (loading) return;
    if (boardRecoveryAttempts >= 2) return;
    if (!lastLoadFailed && (teamTasks.length > 0 || employees.length > 0 || projects.length > 0)) {
      return;
    }
    boardRecoveryAttempts += 1;
    await load({ refreshRemote: true, forceRemote: true });
  }

  async function loadProjectContext(projectID: string) {
    const requestSeq = ++projectContextRequestSeq;
    if (!projectID) {
      projectMembers = [];
      projectTasks = [];
      projectActivity = [];
      projectContextLoading = false;
      projectContextLoadedFor = "";
      projectContextFailedFor = "";
      return;
    }

    projectContextLoading = true;
    projectContextFailedFor = "";
    try {
      const [memberRows, taskRows, activityRows] = await Promise.all([
        listProjectMembers(projectID),
        listProjectTasks(projectID, true),
        listProjectActivity(projectID),
      ]);
      if (requestSeq !== projectContextRequestSeq) {
        return;
      }
      projectMembers = memberRows;
      projectTasks = taskRows;
      projectActivity = activityRows;
      projectContextLoadedFor = projectID;
    } catch (err) {
      if (requestSeq === projectContextRequestSeq) {
        projectContextFailedFor = projectID;
        toast.danger(`Failed to load project context: ${String(err)}`);
      }
    } finally {
      if (requestSeq === projectContextRequestSeq) {
        projectContextLoading = false;
      }
    }
  }

  async function loadTaskDetail(taskID: string, options: { suppressErrorToast?: boolean } = {}): Promise<boolean> {
    const { suppressErrorToast = false } = options;
    const requestSeq = ++taskDetailRequestSeq;
    if (!taskID) {
      selectedTask = null;
      taskComments = [];
      taskActivity = [];
      taskDetailLoading = false;
      return false;
    }

    taskDetailLoading = true;
    try {
      const [task, comments, activity] = await Promise.all([
        getTask(taskID),
        listTaskComments(taskID),
        listTaskActivity(taskID),
      ]);
      if (requestSeq !== taskDetailRequestSeq) {
        return;
      }
      selectedTask = task;
      taskComments = comments;
      taskActivity = activity;
      selectedTaskTitleDraft = task?.title || "";
      selectedTaskDescriptionDraft = task?.description || "";
      selectedTaskPriorityDraft = task?.priority || "medium";
      selectedTaskAssigneeDraft = task?.assignee_employee_id || "";
      selectedTaskDueDateDraft = task?.due_date ? new Date(task.due_date).toISOString().slice(0, 10) : "";
      selectedTaskBlockerDraft = task?.blocked_reason || "";
      confirmDeleteTask = false;
      return true;
    } catch (err) {
      if (requestSeq === taskDetailRequestSeq) {
        selectedTask = null;
        taskComments = [];
        taskActivity = [];
      }
      if (!suppressErrorToast) {
        toast.danger(`Failed to load task detail: ${String(err)}`);
      }
      return false;
    } finally {
      if (requestSeq === taskDetailRequestSeq) {
        taskDetailLoading = false;
      }
    }
  }

  async function openTaskAfterRefresh(taskID: string) {
    if (!taskID || openingPendingTask) return;

    openingPendingTask = true;
    taskModalOpen = true;
    selectedTaskId = taskID;

    try {
      let opened = false;
      const fallbackTask = selectedTask?.id === taskID ? selectedTask : null;
      for (let attempt = 0; attempt < 3 && !opened; attempt += 1) {
        if (loading) {
          await waitForWorkspaceLoad();
        }
        if (attempt > 0) {
          await load({ refreshRemote: true, forceRemote: true });
        }
        opened = await loadTaskDetail(taskID, { suppressErrorToast: true });
        if (!opened) {
          const cachedTask = selectedTask?.id === taskID
            ? selectedTask
            : fallbackTask?.id === taskID
              ? fallbackTask
            : [...myTasks, ...teamTasks, ...projectTasks].find((task) => task.id === taskID);
          if (cachedTask) {
            primeTaskDetail(cachedTask);
            opened = true;
          }
        }
      }

      if (!opened) {
        if (pendingOpenTaskId === taskID) {
          pendingOpenTaskId = "";
        }
        if (selectedTaskId === taskID) {
          selectedTaskId = "";
        }
        taskModalOpen = false;
        toast.danger("Failed to load task detail: task not found. The task may still be syncing to this device.");
        return;
      }

      if (pendingOpenTaskId === taskID) {
        pendingOpenTaskId = "";
      }
    } finally {
      openingPendingTask = false;
    }
  }

  async function handleCreateTask() {
    if (!taskTitle.trim()) {
      toast.warning("Task title is required");
      return;
    }

    savingTask = true;
    try {
      const created = await createTask({
        title: taskTitle.trim(),
        description: taskDescription.trim(),
        priority: taskPriority,
        due_date: taskDueDate ? new Date(`${taskDueDate}T09:00:00`).toISOString() : undefined,
        project_id: selectedProjectForTask || undefined,
        assignee_employee_id: selectedAssignee || currentEmployee?.employee_id || undefined,
      });
      taskTitle = "";
      taskDescription = "";
      taskPriority = "medium";
      taskDueDate = "";
      const createdAssignee = created.assignee_employee_id || selectedAssignee || currentEmployee?.employee_id || "";
      const assignedToCurrent = !!currentEmployee?.employee_id && createdAssignee === currentEmployee.employee_id;
      myTasks = assignedToCurrent ? upsertTaskRow(myTasks, created) : myTasks.filter((task) => task.id !== created.id);
      teamTasks = upsertTaskRow(teamTasks, created);
      if (selectedProjectForTask && created.project_id === selectedProjectForTask) {
        projectTasks = upsertTaskRow(projectTasks, created);
      }
      activeTab = assignedToCurrent ? "my_work" : "team_board";
      if (!assignedToCurrent) {
        teamFocusFilter = "all";
        teamAssigneeFilter = "all";
      }
      queuePendingTaskOpen(created.id);
      primeTaskDetail(created);
      pendingOpenTaskId = "";
      void load({ refreshRemote: true, forceRemote: true, silent: true });
      void loadTaskDetail(created.id, { suppressErrorToast: true });
      toast.success("Task created");
    } catch (err) {
      toast.danger(`Failed to create task: ${String(err)}`);
    } finally {
      savingTask = false;
    }
  }

  async function loadProjectCustomers() {
    if (projectCustomers.length > 0 || loadingProjectCustomers) return;
    loadingProjectCustomers = true;
    try {
      projectCustomers = (await ListCustomers(500, 0)) || [];
    } catch (err) {
      toast.danger(`Customer list could not load: ${String(err)}`);
      projectCustomers = [];
    } finally {
      loadingProjectCustomers = false;
    }
  }

  function customerDisplayName(customer: any) {
    return customer?.business_name || customer?.trading_name || customer?.customer_id || "";
  }

  async function selectProjectCustomer(customer: any) {
    const customerID = customer?.id || customer?.customer_id || "";
    projectCustomerID = customerID;
    projectCustomerSearch = customerDisplayName(customer);
    showProjectCustomerSuggestions = false;
    projectPocID = "";
    projectContacts = [];
    projectPocName = "";
    projectPocEmail = "";
    projectPocPhone = "";

    projectCustomerName = customerDisplayName(customer);
    if (!projectEndUserName) {
      projectEndUserName = projectCustomerName;
    }

    if (!customerID) return;
    try {
      projectContacts = (await ListCustomerContacts(customerID)) || [];
      const primary = projectContacts.find((contact) => contact.is_primary_contact) || projectContacts[0];
      if (primary) {
        applyProjectContact(primary);
      }
    } catch (err) {
      toast.danger(`Customer contacts could not load: ${String(err)}`);
      projectContacts = [];
    }
  }

  function handleProjectCustomerInput() {
    const search = projectCustomerSearch.trim();
    projectCustomerName = search;
    showProjectCustomerSuggestions = search.length >= 2;
    if (projectCustomerID) {
      const selected = projectCustomers.find((row) => row.id === projectCustomerID || row.customer_id === projectCustomerID);
      if (selected && customerDisplayName(selected) !== search) {
        projectCustomerID = "";
        projectPocID = "";
        projectContacts = [];
        projectPocName = "";
        projectPocEmail = "";
        projectPocPhone = "";
      }
    }
  }

  function applyProjectContact(contact: any) {
    projectPocID = contact?.id || "";
    projectPocName = contact?.contact_name || "";
    projectPocEmail = contact?.email || "";
    projectPocPhone = contact?.phone || "";
  }

  function handleProjectContactSelect(event: Event) {
    const contactID = (event.currentTarget as HTMLSelectElement).value;
    const contact = projectContacts.find((row) => row.id === contactID);
    if (contact) {
      applyProjectContact(contact);
    } else {
      projectPocID = "";
      projectPocName = "";
      projectPocEmail = "";
      projectPocPhone = "";
    }
  }

  async function handleCreateProject() {
    if (!projectName.trim()) {
      toast.warning("Project name is required");
      return;
    }

    savingProject = true;
    try {
      const created = await createProject({
        name: projectName.trim(),
        project_type: projectType,
        description: projectDescription.trim(),
        customer_id: projectCustomerID,
        customer_name: projectCustomerName.trim(),
        end_user_name: projectEndUserName.trim(),
        opportunity_key: projectOpportunityKey.trim(),
        customer_poc_name: projectPocName.trim(),
        customer_poc_email: projectPocEmail.trim(),
        customer_poc_phone: projectPocPhone.trim(),
      });
      projectName = "";
      projectDescription = "";
      projectType = "internal";
      projectCustomerID = "";
      projectCustomerName = "";
      projectCustomerSearch = "";
      showProjectCustomerSuggestions = false;
      projectEndUserName = "";
      projectOpportunityKey = "";
      projectPocID = "";
      projectPocName = "";
      projectPocEmail = "";
      projectPocPhone = "";
      projectContacts = [];
      projects = upsertProjectRow(projects, created);
      selectedProjectId = created.id;
      selectedProjectForTask = created.id;
      projectContextRequestSeq += 1;
      projectMembers = [];
      projectTasks = [];
      projectActivity = [];
      memberSelections = [];
      projectContextLoading = false;
      projectContextLoadedFor = created.id;
      projectContextFailedFor = "";
      if (employees.length === 0) {
        employees = await listEmployeeProfiles(true).catch((err) => {
          toast.danger(`Project created, but employee directory could not load: ${String(err)}`);
          return [];
        });
      }
      saveSnapshot();
      void load({ refreshRemote: true, silent: true });
      toast.success("Project created");
    } catch (err) {
      toast.danger(`Failed to create project: ${String(err)}`);
    } finally {
      savingProject = false;
    }
  }

  async function handleAddProjectMembers() {
    if (!selectedProjectId) {
      toast.warning("Choose a project first");
      return;
    }
    if (memberSelections.length === 0) {
      toast.warning("Choose at least one employee to add");
      return;
    }

    savingMember = true;
    try {
      await Promise.all(
        memberSelections.map((employeeID) => addProjectMember(selectedProjectId, employeeID, memberRole.trim() || "Member")),
      );
      const addedCount = memberSelections.length;
      memberSelections = [];
      memberRole = "Member";
      await loadProjectContext(selectedProjectId);
      toast.success(`${addedCount} project ${addedCount === 1 ? "member" : "members"} added`);
    } catch (err) {
      toast.danger(`Failed to add project members: ${String(err)}`);
    } finally {
      savingMember = false;
    }
  }

  function toggleProjectMemberSelection(employeeID: string) {
    memberSelections = memberSelections.includes(employeeID)
      ? memberSelections.filter((id) => id !== employeeID)
      : [...memberSelections, employeeID];
  }

  async function changeStatus(task: CollaborativeTask, status: string, note = "") {
    try {
      await updateTaskStatus(task.id, status, note);
      await load();
      if (selectedTaskId === task.id) {
        await loadTaskDetail(task.id);
      }
      toast.success(`Task marked ${status.replace("_", " ")}`);
    } catch (err) {
      toast.danger(`Failed to update task: ${String(err)}`);
    }
  }

  async function handleAddComment() {
    if (!selectedTaskId || !commentDraft.trim()) {
      return;
    }

    savingComment = true;
    try {
      await addTaskComment(selectedTaskId, commentDraft.trim());
      commentDraft = "";
      await loadTaskDetail(selectedTaskId);
      toast.success("Comment added");
    } catch (err) {
      toast.danger(`Failed to add comment: ${String(err)}`);
    } finally {
      savingComment = false;
    }
  }

  async function handleReassignSelectedTask() {
    if (!selectedTask) return;
    if (!selectedTaskAssigneeChanged) {
      toast.info("Choose a different assignee before confirming reassignment");
      return;
    }
    savingAssignment = true;
    try {
      await reassignTask(selectedTask.id, selectedTaskAssigneeDraft);
      await load();
      await loadTaskDetail(selectedTask.id);
      toast.success("Task reassignment confirmed");
    } catch (err) {
      toast.danger(`Failed to reassign task: ${String(err)}`);
    } finally {
      savingAssignment = false;
    }
  }

  async function handleUpdateDueDate() {
    if (!selectedTask) return;
    savingDueDate = true;
    try {
      const dueDateISO = selectedTaskDueDateDraft ? new Date(`${selectedTaskDueDateDraft}T09:00:00`).toISOString() : "";
      await updateTaskDueDate(selectedTask.id, dueDateISO);
      await load();
      await loadTaskDetail(selectedTask.id);
      toast.success("Task due date updated");
    } catch (err) {
      toast.danger(`Failed to update due date: ${String(err)}`);
    } finally {
      savingDueDate = false;
    }
  }

  async function selectTask(taskID: string) {
    if (!taskID) return;
    const cachedTask = [...myTasks, ...teamTasks, ...projectTasks].find((task) => task.id === taskID);
    queuePendingTaskOpen(taskID, cachedTask || null);
    await openTaskAfterRefresh(taskID);
  }

  function closeTaskModal() {
    taskModalOpen = false;
    confirmDeleteTask = false;
  }

  async function handleBlockSelectedTask() {
    if (!selectedTask) return;
    if (!selectedTaskBlockerDraft.trim()) {
      toast.warning("Add a blocked reason so the team knows what is stuck");
      return;
    }
    await changeStatus(selectedTask, "blocked", selectedTaskBlockerDraft.trim());
  }

  async function handleSaveTaskDetails() {
    if (!selectedTask) return;
    if (!selectedTaskTitleDraft.trim()) {
      toast.warning("Task title is required");
      return;
    }
    savingTaskDetails = true;
    try {
      await updateTask({
        id: selectedTask.id,
        title: selectedTaskTitleDraft.trim(),
        description: selectedTaskDescriptionDraft.trim(),
        priority: selectedTaskPriorityDraft,
        task_type: selectedTask.task_type,
        project_id: selectedTask.project_id,
      });
      await load();
      await loadTaskDetail(selectedTask.id);
      toast.success("Task details updated");
    } catch (err) {
      toast.danger(`Failed to update task: ${String(err)}`);
    } finally {
      savingTaskDetails = false;
    }
  }

  async function handleDeleteSelectedTask() {
    if (!selectedTask) return;
    if (!confirmDeleteTask) {
      confirmDeleteTask = true;
      toast.warning("Press Delete again to remove this task from the work board");
      return;
    }
    deletingTask = true;
    try {
      const taskID = selectedTask.id;
      await deleteTask(taskID);
      closeTaskModal();
      selectedTask = null;
      selectedTaskId = "";
      await load();
      toast.success("Task deleted");
    } catch (err) {
      toast.danger(`Failed to delete task: ${String(err)}`);
    } finally {
      deletingTask = false;
    }
  }

  function queuePendingTaskOpen(taskID: string, fallbackTask?: CollaborativeTask | null) {
    if (!taskID) return;
    pendingOpenTaskId = taskID;
    selectedTaskId = taskID;
    taskModalOpen = true;
    if (fallbackTask) {
      primeTaskDetail(fallbackTask);
    } else {
      taskDetailLoading = true;
      selectedTask = null;
      selectedTaskTitleDraft = "";
      selectedTaskDescriptionDraft = "";
      selectedTaskPriorityDraft = "medium";
      selectedTaskAssigneeDraft = "";
      selectedTaskDueDateDraft = "";
      selectedTaskBlockerDraft = "";
    }
    taskComments = [];
    taskActivity = [];
  }

  function upsertTaskRow(rows: CollaborativeTask[], task: CollaborativeTask) {
    const existingIndex = rows.findIndex((row) => row.id === task.id);
    if (existingIndex >= 0) {
      const next = [...rows];
      next[existingIndex] = task;
      return next;
    }
    return [task, ...rows];
  }

  function upsertProjectRow(rows: CollaborativeProject[], project: CollaborativeProject) {
    const existingIndex = rows.findIndex((row) => row.id === project.id);
    if (existingIndex >= 0) {
      const next = [...rows];
      next[existingIndex] = project;
      return next;
    }
    return [project, ...rows];
  }

  function primeTaskDetail(task: CollaborativeTask) {
    selectedTask = task;
    selectedTaskId = task.id;
    taskComments = [];
    taskActivity = [];
    taskDetailLoading = false;
    selectedTaskTitleDraft = task.title || "";
    selectedTaskDescriptionDraft = task.description || "";
    selectedTaskPriorityDraft = task.priority || "medium";
    selectedTaskAssigneeDraft = task.assignee_employee_id || "";
    selectedTaskDueDateDraft = task.due_date ? new Date(task.due_date).toISOString().slice(0, 10) : "";
    selectedTaskBlockerDraft = task.blocked_reason || "";
  }

  function consumePendingTaskOpenFromStorage(): string {
    const taskID = sessionStorage.getItem(pendingTaskStorageKey)?.trim() || "";
    if (taskID) {
      sessionStorage.removeItem(pendingTaskStorageKey);
    }
    return taskID;
  }

  function consumePendingTaskPayloadFromStorage(taskID: string): CollaborativeTask | null {
    const raw = sessionStorage.getItem(pendingTaskPayloadStorageKey);
    if (raw) {
      sessionStorage.removeItem(pendingTaskPayloadStorageKey);
    }
    if (!raw) return null;
    try {
      const task = JSON.parse(raw) as CollaborativeTask;
      if (task?.id && String(task.id) === taskID && task.title) {
        return {
          ...task,
          id: String(task.id),
          title: String(task.title),
          status: task.status || "open",
          priority: task.priority || "medium",
        };
      }
    } catch {
      return null;
    }
    return null;
  }

  async function selectProject(projectID: string) {
    selectedProjectId = projectID;
    selectedProjectForTask = projectID;
    editingProject = false;
    projectEditDraft = {};
    projectAdminReason = "";
    projectContextLoadedFor = "";
    projectContextFailedFor = "";
    projectModalOpen = true;
    await loadProjectContext(projectID);
  }

  function startProjectEdit() {
    if (!selectedProject) return;
    projectEditDraft = { ...selectedProject };
    editingProject = true;
  }

  function cancelProjectEdit() {
    editingProject = false;
    projectEditDraft = {};
  }

  async function saveProjectEdit() {
    if (!selectedProject) return;
    if (!String(projectEditDraft.name || "").trim()) {
      toast.warning("Project name is required");
      return;
    }
    savingProjectAdmin = true;
    try {
      const updated = await updateProject(selectedProject.id, {
        name: String(projectEditDraft.name || "").trim(),
        project_type: String(projectEditDraft.project_type || "internal"),
        description: String(projectEditDraft.description || "").trim(),
        customer_name: String(projectEditDraft.customer_name || "").trim(),
        end_user_name: String(projectEditDraft.end_user_name || "").trim(),
        opportunity_key: String(projectEditDraft.opportunity_key || "").trim(),
        customer_poc_name: String(projectEditDraft.customer_poc_name || "").trim(),
        customer_poc_email: String(projectEditDraft.customer_poc_email || "").trim(),
        customer_poc_phone: String(projectEditDraft.customer_poc_phone || "").trim(),
        starts_on: projectEditDraft.starts_on,
        ends_on: projectEditDraft.ends_on,
        status: String(projectEditDraft.status || selectedProject.status || "active"),
      });
      projects = upsertProjectRow(projects, updated);
      editingProject = false;
      projectEditDraft = {};
      saveSnapshot();
      toast.success("Project updated");
    } catch (err) {
      toast.danger(`Failed to update project: ${String(err)}`);
    } finally {
      savingProjectAdmin = false;
    }
  }

  async function applyProjectAdminAction(action: "archive" | "shelve" | "delete") {
    if (!selectedProject) return;
    const reason = projectAdminReason.trim() || `${action} from project admin workspace`;
    savingProjectAdmin = true;
    try {
      if (action === "delete") {
        await deleteProject(selectedProject.id, reason);
        projects = projects.filter((project) => project.id !== selectedProject.id);
        selectedProjectId = projects[0]?.id || "";
        selectedProjectForTask = selectedProjectId;
        projectMembers = [];
        projectTasks = [];
        projectActivity = [];
        projectModalOpen = false;
      } else {
        const updated = action === "archive"
          ? await archiveProject(selectedProject.id, reason)
          : await shelveProject(selectedProject.id, reason);
        projects = upsertProjectRow(projects, updated);
      }
      projectAdminReason = "";
      editingProject = false;
      projectEditDraft = {};
      saveSnapshot();
      toast.success(`Project ${action === "delete" ? "deleted" : action + "d"}`);
    } catch (err) {
      toast.danger(`Failed to ${action} project: ${String(err)}`);
    } finally {
      savingProjectAdmin = false;
    }
  }

  function startEmployeeDrag(employeeID: string, event: DragEvent) {
    draggedEmployeeId = employeeID;
    event.dataTransfer?.setData("text/plain", employeeID);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = "move";
    }
  }

  function allowTaskDrop(event: DragEvent, taskID: string) {
    if (!draggedEmployeeId) return;
    event.preventDefault();
    draggedOverTaskId = taskID;
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "move";
    }
  }

  function clearDragState() {
    draggedEmployeeId = "";
    draggedOverTaskId = "";
  }

  function handleOpenTaskEvent(event: Event) {
    const detail = (event as CustomEvent<{ taskID?: string; task?: CollaborativeTask }>).detail || {};
    const taskID = detail.taskID || "";
    if (!taskID) return;
    activeTab = defaultWorkTabForRole(currentEmployee?.license_role);
    queuePendingTaskOpen(taskID, detail.task || null);
    void openTaskAfterRefresh(taskID);
  }

  async function handleTaskDrop(task: CollaborativeTask, event: DragEvent) {
    if (!draggedEmployeeId) return;
    event.preventDefault();
    const employeeID = draggedEmployeeId;
    clearDragState();
    try {
      await reassignTask(task.id, employeeID);
      await load();
      if (selectedTaskId === task.id) {
        await loadTaskDetail(task.id);
      }
      const employeeName = employees.find((employee) => employee.id === employeeID)?.full_name || "Employee";
      toast.success(`${employeeName} assigned to ${task.title}`);
    } catch (err) {
      toast.danger(`Failed to assign task: ${String(err)}`);
    }
  }

  function formatDate(value?: string) {
    if (!value) return "No date";
    return new Date(value).toLocaleString();
  }

  function projectNameFor(task: CollaborativeTask) {
    return projects.find((project) => project.id === task.project_id)?.name || "None";
  }

  function normalizedBoardStatus(task: CollaborativeTask) {
    const status = String(task.status || "open").trim().toLowerCase().replace(/[\s-]+/g, "_");
    if (["completed", "archived", "done"].includes(status)) return "completed";
    if (status === "blocked") return "blocked";
    if (["in_progress", "started", "active_progress"].includes(status)) return "in_progress";
    if (["open", "pending", "new", "active", ""].includes(status)) return "open";
    return "open";
  }

  function dueLabel(task: CollaborativeTask) {
    if (!task.due_date) return "No due date";
    const due = new Date(task.due_date);
    if (Number.isNaN(due.getTime())) return "No due date";
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const dueDay = new Date(due);
    dueDay.setHours(0, 0, 0, 0);
    if (dueDay.getTime() < today.getTime() && !["completed", "archived"].includes(task.status || "")) {
      return `Overdue • ${due.toLocaleDateString()}`;
    }
    return `Due ${due.toLocaleDateString()}`;
  }

  $: selectedProject = projects.find((project) => project.id === selectedProjectId) || null;
  $: filteredProjectCustomers = projectCustomerSearch.trim().length < 2
    ? []
    : projectCustomers
      .filter((customer) => [
        customerDisplayName(customer),
        customer?.customer_id,
        customer?.customer_code,
        customer?.short_code,
      ].filter(Boolean).some((value) => String(value).toLowerCase().includes(projectCustomerSearch.trim().toLowerCase())))
      .slice(0, 8);
  $: selectedTaskAssigneeChanged = Boolean(selectedTask) && (selectedTask.assignee_employee_id || "") !== selectedTaskAssigneeDraft;
  $: boardTaskGroups = filteredTeamTasks.reduce((groups, task) => {
    const lane = normalizedBoardStatus(task);
    const targetLane = boardColumns.some((column) => column.key === lane) ? lane : "open";
    groups[targetLane] = [...groups[targetLane], task];
    return groups;
  }, {
    open: [] as CollaborativeTask[],
    in_progress: [] as CollaborativeTask[],
    blocked: [] as CollaborativeTask[],
    completed: [] as CollaborativeTask[],
  });
  $: projectStats = {
    open: projectTasks.filter((task) => !["completed", "archived"].includes(task.status || "")).length,
    blocked: projectTasks.filter((task) => task.status === "blocked").length,
    completed: projectTasks.filter((task) => task.status === "completed").length,
    members: projectMembers.length,
  };
  $: availableProjectMembers = employees.filter((employee) => !projectMembers.some((member) => member.employee_id === employee.id));
  $: memberSelections = memberSelections.filter((employeeID) => availableProjectMembers.some((employee) => employee.id === employeeID));
  $: filteredTeamTasks = teamTasks.filter((task) => {
    const matchesSearch = !teamSearch.trim() || [task.title, task.description, task.assignee_name, task.creator_name]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(teamSearch.trim().toLowerCase()));
    const matchesAssignee = teamAssigneeFilter === "all"
      || (teamAssigneeFilter === "unassigned" && !task.assignee_employee_id)
      || task.assignee_employee_id === teamAssigneeFilter;
    const isOverdue = Boolean(task.due_date && new Date(task.due_date) < new Date() && !["completed", "archived"].includes(task.status || ""));
    const matchesFocus = teamFocusFilter === "all"
      || (teamFocusFilter === "active" && !["completed", "archived"].includes(task.status || ""))
      || (teamFocusFilter === "assigned" && !!task.assignee_employee_id)
      || (teamFocusFilter === "unassigned" && !task.assignee_employee_id)
      || (teamFocusFilter === "overdue" && isOverdue)
      || (teamFocusFilter === "blocked" && task.status === "blocked");
    return matchesSearch && matchesAssignee && matchesFocus;
  });
  $: workload = employees
    .map((employee) => ({
      id: employee.id,
      name: employee.full_name,
      department: employee.department,
      openCount: teamTasks.filter((task) => task.assignee_employee_id === employee.id && !["completed", "archived"].includes(task.status || "")).length,
      blockedCount: teamTasks.filter((task) => task.assignee_employee_id === employee.id && task.status === "blocked").length,
      urgentCount: teamTasks.filter((task) => task.assignee_employee_id === employee.id && ["high", "urgent"].includes(task.priority || "") && !["completed", "archived"].includes(task.status || "")).length,
    }))
    .sort((a, b) => b.openCount - a.openCount || b.blockedCount - a.blockedCount);
  $: activeWorkload = workload.filter((item) => item.openCount > 0 || item.blockedCount > 0);
  $: modalTask = selectedTask || [...myTasks, ...teamTasks, ...projectTasks].find((task) => task.id === selectedTaskId) || null;
  $: if (
    activeTab === "projects"
    && selectedProjectId
    && projectContextLoadedFor !== selectedProjectId
    && projectContextFailedFor !== selectedProjectId
    && !projectContextLoading
    && !loading
  ) {
    void loadProjectContext(selectedProjectId);
  }

  $: if (pendingOpenTaskId && !loading && activeTab !== "projects") {
    void openTaskAfterRefresh(pendingOpenTaskId);
  }

  $: if (selectedTaskId && activeTab !== "projects" && !pendingOpenTaskId && !openingPendingTask && (!selectedTask || selectedTask.id !== selectedTaskId) && !loading) {
    void loadTaskDetail(selectedTaskId, { suppressErrorToast: true });
  }

  $: if (activeTab === "team_board" && !loading && (lastLoadFailed || (teamTasks.length === 0 && employees.length === 0 && projects.length === 0))) {
    void recoverBoardIfNeeded();
  }

  onMount(() => {
    void hydrateWorkHub();
    void loadProjectCustomers();
    EventsOn("tasks:updated", () => void load({ silent: true }));
    EventsOn("projects:updated", () => {
      void load({ silent: true });
      if (selectedProjectId) {
        projectContextLoadedFor = "";
        projectContextFailedFor = "";
        void loadProjectContext(selectedProjectId);
      }
    });
    EventsOn("employees:updated", () => void load({ silent: true }));
    window.addEventListener("openCollaborativeTask", handleOpenTaskEvent);
    const pendingTaskID = consumePendingTaskOpenFromStorage();
    if (pendingTaskID) {
      queuePendingTaskOpen(pendingTaskID, consumePendingTaskPayloadFromStorage(pendingTaskID));
    }
  });

  onDestroy(() => {
    EventsOff("tasks:updated", "projects:updated", "employees:updated");
    window.removeEventListener("openCollaborativeTask", handleOpenTaskEvent);
  });
</script>

<div class="page">
  <header class="work-topbar">
    <section class="tabs">
      <button class:active={activeTab === "my_work"} on:click={() => activeTab = "my_work"}>My Work</button>
      <button class:active={activeTab === "team_board"} on:click={() => activeTab = "team_board"}>Team Board</button>
      <button class:active={activeTab === "projects"} on:click={() => activeTab = "projects"}>Projects</button>
    </section>

    {#if currentEmployee}
      <div class="identity-card">
        <span class="label">Active Employee</span>
        <strong>{currentEmployee.employee_name}</strong>
        <span class="meta">{currentEmployee.license_role || currentEmployee.resolved_by}</span>
        {#if remoteRefreshing}
          <span class="meta">Syncing latest changes</span>
        {/if}
      </div>
    {:else if remoteRefreshing}
      <div class="identity-card">
        <span class="label">Workspace</span>
        <strong>Syncing latest changes</strong>
      </div>
    {/if}
  </header>

  {#if activeTab !== "team_board"}
    <section class="composer">
      <div class="section-head">
        <div>
          <h2>Create Task</h2>
          <p>Assign work to a person, attach it to a project, and make it visible across devices.</p>
        </div>
      </div>
      <div class="composer-grid">
        <input bind:value={taskTitle} placeholder="Task title" />
        <select bind:value={taskPriority}>
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high">High</option>
          <option value="urgent">Urgent</option>
        </select>
        <input bind:value={taskDueDate} type="date" />
        <select bind:value={selectedProjectForTask}>
          <option value="">No project</option>
          {#each projects as project}
            <option value={project.id}>{project.name}</option>
          {/each}
        </select>
        <select bind:value={selectedAssignee}>
          <option value="">Assign to me</option>
          {#each employees as employee}
            <option value={employee.id}>{employee.full_name}{employee.department ? ` • ${employee.department}` : ""}</option>
          {/each}
        </select>
      </div>
      <textarea bind:value={taskDescription} rows="3" placeholder="Context, blockers, or expected outcome"></textarea>
      <div class="actions">
        <button class="primary" on:click={handleCreateTask} disabled={savingTask}>Create Task</button>
      </div>
    </section>
  {/if}

  {#if activeTab === "my_work"}
    <section class="panel task-gallery-panel">
      <div class="section-head">
        <div>
          <h2>My Work</h2>
          <p>Compact task cards for quick scanning. Open any card to inspect comments, assignee, and history.</p>
        </div>
        <span class="count">{myTasks.length} active</span>
      </div>
      {#if loading}
        <div class="empty">Loading your work queue...</div>
      {:else if myTasks.length === 0}
        <div class="empty">No tasks assigned yet.</div>
      {:else}
        <div class="task-gallery two-up">
          {#each myTasks as task}
            <button class="task-shell selectable" class:selected={selectedTaskId === task.id} on:click={() => selectTask(task.id)}>
              <div class="task-shell-top">
                <span class="task-status">{(task.status || "open").replaceAll("_", " ")}</span>
                <span class="pill {task.priority || 'medium'}">{task.priority || "medium"}</span>
              </div>
              <div class="task-shell-body">
                <h3>{task.title}</h3>
                <p>{task.description || "No task details provided."}</p>
                {#if task.status === "blocked" && task.blocked_reason}
                  <p class="blocker-copy">Blocked because: {task.blocked_reason}</p>
                {/if}
              </div>
              <div class="task-shell-meta">
                <span>{projectNameFor(task)}</span>
                <span>{dueLabel(task)}</span>
                <span>{task.assignee_name || "Unassigned"}</span>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  {/if}

  {#if activeTab === "team_board"}
    <section class="metrics-grid">
      <article class="metric-card">
        <span class="label">Open Team Tasks</span>
        <strong>{teamTasks.filter((task) => !["completed", "archived"].includes(task.status || "")).length}</strong>
      </article>
      <article class="metric-card">
        <span class="label">Blocked</span>
        <strong>{teamTasks.filter((task) => task.status === "blocked").length}</strong>
      </article>
      <article class="metric-card">
        <span class="label">Overdue</span>
        <strong>{teamTasks.filter((task) => task.due_date && new Date(task.due_date) < new Date() && !["completed", "archived"].includes(task.status || "")).length}</strong>
      </article>
      <article class="metric-card">
        <span class="label">Active Employees</span>
        <strong>{workload.length}</strong>
      </article>
    </section>

    <section class="panel board-filters">
      <div class="filter-grid">
        <input bind:value={teamSearch} placeholder="Search tasks, assignees, or descriptions" />
        <select bind:value={teamFocusFilter}>
          <option value="all">All focus</option>
          <option value="active">Active only</option>
          <option value="assigned">Assigned only</option>
          <option value="unassigned">Unassigned</option>
          <option value="overdue">Overdue</option>
          <option value="blocked">Blocked</option>
        </select>
        <select bind:value={teamAssigneeFilter}>
          <option value="all">All assignees</option>
          <option value="unassigned">Unassigned only</option>
          {#each employees as employee}
            <option value={employee.id}>{employee.full_name}</option>
          {/each}
        </select>
      </div>
    </section>

    <section class="team-overview-grid">
      <article class="panel overview-panel">
        <div class="section-head">
          <h2>Admin Snapshot</h2>
          <span class="count">{projects.length} projects</span>
        </div>
        <div class="insight-grid">
          <div class="insight-row">
            <strong>{teamTasks.filter((task) => !task.assignee_employee_id && !["completed", "archived"].includes(task.status || "")).length}</strong>
            <span>unassigned tasks waiting for an owner</span>
          </div>
          <div class="insight-row">
            <strong>{teamTasks.filter((task) => ["high", "urgent"].includes(task.priority || "") && !["completed", "archived"].includes(task.status || "")).length}</strong>
            <span>high-priority tasks still in flight</span>
          </div>
          <div class="insight-row">
            <strong>{activeWorkload.filter((person) => person.openCount >= 3).length}</strong>
            <span>employees carrying three or more active tasks</span>
          </div>
        </div>
      </article>

      <article class="panel roster-panel">
        <div class="section-head">
          <div>
            <h2>Assignment Roster</h2>
            <p>Drag an employee card onto any task card to assign it. One employee can hold multiple tasks.</p>
          </div>
          <span class="count">{employees.length} people</span>
        </div>
        {#if employees.length === 0}
          <div class="empty">No employees available for assignment yet.</div>
        {:else}
          <div class="roster-strip">
            {#each workload as person}
              <button
                class="employee-chip"
                class:dragging={draggedEmployeeId === person.id}
                draggable="true"
                on:dragstart={(event) => startEmployeeDrag(person.id, event)}
                on:dragend={clearDragState}
                on:click={() => (teamAssigneeFilter = person.id)}
              >
                <strong>{person.name}</strong>
                <span>{person.department || "Team"}</span>
                <span>{person.openCount} active • {person.blockedCount} blocked</span>
              </button>
            {/each}
          </div>
        {/if}
      </article>
    </section>

    <section class="panel board-panel">
        <div class="section-head">
          <div>
            <h2>Team Board</h2>
            <p>Small task cards grouped by status. Click a card to open the full task modal.</p>
          </div>
          <span class="count">{filteredTeamTasks.length} visible</span>
        </div>
        {#if loading}
          <div class="empty">Loading team board...</div>
        {:else if lastLoadFailed}
          <div class="empty">
            Team board is retrying connection to the collaborative workspace.
            {#if lastLoadError}
              <span class="load-error-detail">{lastLoadError}</span>
            {/if}
          </div>
        {:else}
        <div class="board-sections">
          {#each boardColumns as column}
            <section class="board-lane">
              <div class="board-lane-head">
                <h3>{column.label}</h3>
                <span>{boardTaskGroups[column.key]?.length || 0}</span>
              </div>
              {#if (boardTaskGroups[column.key]?.length || 0) === 0}
                <div class="empty compact lane-empty">No tasks in this lane.</div>
              {:else}
                <div class="task-gallery lane-gallery">
                  {#each boardTaskGroups[column.key] as task}
                    <button
                      class="task-shell board-card selectable"
                      class:selected={selectedTaskId === task.id}
                      class:drop-target={draggedOverTaskId === task.id}
                      on:click={() => selectTask(task.id)}
                      on:dragover={(event) => allowTaskDrop(event, task.id)}
                      on:dragleave={() => draggedOverTaskId = ""}
                      on:drop={(event) => handleTaskDrop(task, event)}
                    >
                      <div class="task-shell-top">
                        <span class="task-status">{column.label}</span>
                        <span class="pill {task.priority || 'medium'}">{task.priority || "medium"}</span>
                      </div>
                      <div class="task-shell-body">
                        <h3>{task.title}</h3>
                        <p>{task.description || "No task details provided."}</p>
                        {#if task.status === "blocked" && task.blocked_reason}
                          <p class="blocker-copy">Blocked because: {task.blocked_reason}</p>
                        {/if}
                      </div>
                      <div class="task-shell-meta">
                        <span>{task.assignee_name || "Unassigned"}</span>
                        <span>{projectNameFor(task)}</span>
                        <span>{dueLabel(task)}</span>
                      </div>
                    </button>
                  {/each}
                </div>
              {/if}
            </section>
          {/each}
        </div>
        {/if}
    </section>
  {/if}

  {#if activeTab === "projects"}
    <section class="workspace">
      <article class="panel">
        <div class="section-head">
          <div>
            <h2>Projects</h2>
            <p>Customer, internal, and admin work containers.</p>
          </div>
          <span class="count">{projects.length}</span>
        </div>

        <div class="project-composer">
          <input bind:value={projectName} placeholder="Project name" />
          <select bind:value={projectType}>
            <option value="internal">Internal</option>
            <option value="customer">Customer</option>
            <option value="admin">Admin</option>
          </select>
          <input bind:value={projectOpportunityKey} placeholder="RFQ / folder key" />
          <div class="autocomplete-field">
            <input
              bind:value={projectCustomerSearch}
              placeholder={loadingProjectCustomers ? "Loading customers..." : "Type 2+ letters for customer"}
              on:input={handleProjectCustomerInput}
              on:focus={() => showProjectCustomerSuggestions = projectCustomerSearch.trim().length >= 2}
              on:blur={() => setTimeout(() => showProjectCustomerSuggestions = false, 120)}
            />
            {#if showProjectCustomerSuggestions}
              <div class="suggestion-list">
                {#each filteredProjectCustomers as customer}
                  <button type="button" on:mousedown|preventDefault={() => selectProjectCustomer(customer)}>
                    <strong>{customerDisplayName(customer)}</strong>
                    <span>{customer.customer_id || customer.customer_code || "Customer"}</span>
                  </button>
                {:else}
                  <div class="suggestion-empty">No matching customers.</div>
                {/each}
              </div>
            {/if}
          </div>
          <input bind:value={projectEndUserName} placeholder="End user" />
          <select bind:value={projectPocID} on:change={handleProjectContactSelect} disabled={!projectCustomerID || projectContacts.length === 0}>
            <option value="">{projectCustomerID ? "Select customer POC" : "Choose customer first"}</option>
            {#each projectContacts as contact}
              <option value={contact.id}>{contact.contact_name}{contact.job_title ? ` - ${contact.job_title}` : ""}</option>
            {/each}
          </select>
          <input bind:value={projectPocEmail} placeholder="POC email" />
          <input bind:value={projectPocPhone} placeholder="POC phone" />
          <textarea bind:value={projectDescription} rows="2" placeholder="What is this project for?"></textarea>
          <div class="actions">
            <button class="primary" on:click={handleCreateProject} disabled={savingProject}>Create Project</button>
          </div>
        </div>

        <div class="project-list">
          {#each projects as project}
            <button class="project-row selectable" class:selected={selectedProjectId === project.id} on:click={() => selectProject(project.id)}>
              <div>
                <strong>{project.name}</strong>
                <div class="meta">{project.project_type || "internal"} • {project.status || "active"}</div>
              </div>
              <span class="count">{projectTasks.filter((task) => task.project_id === project.id).length}</span>
            </button>
          {/each}
        </div>
      </article>

      <article class="panel roster-panel">
        <div class="section-head">
          <div>
            <h2>Assignment Roster</h2>
            <p>Choose an employee here to prefill the task assignee while you are working inside projects.</p>
          </div>
          <span class="count">{employees.length} people</span>
        </div>
        {#if employees.length === 0}
          <div class="empty">No employees available for assignment yet.</div>
        {:else}
          <div class="roster-strip">
            {#each workload as person}
              <button
                class="employee-chip"
                class:selected={selectedAssignee === person.id}
                on:click={() => selectedAssignee = person.id}
              >
                <strong>{person.name}</strong>
                <span>{person.department || "Team"}</span>
                <span>{person.openCount} active • {person.blockedCount} blocked</span>
              </button>
            {/each}
          </div>
        {/if}
      </article>

      <Modal bind:open={projectModalOpen} title={selectedProject?.name || "Project Detail"} size="xl" on:close={() => { projectModalOpen = false; }}>
        <div class="section-head">
          <h2>{selectedProject?.name || "Project Detail"}</h2>
          {#if selectedProject}
            <div class="inline-actions">
              <span class="count">{selectedProject.project_type || "internal"}</span>
              {#if editingProject}
                <button class="ghost-btn" on:click={cancelProjectEdit} disabled={savingProjectAdmin}>Cancel</button>
                <button class="primary mini" on:click={saveProjectEdit} disabled={savingProjectAdmin}>Save</button>
              {:else}
                <button class="ghost-btn" on:click={startProjectEdit}>Edit</button>
              {/if}
            </div>
          {/if}
        </div>

        {#if !selectedProject}
          <div class="empty">Choose a project to view members and project-linked tasks.</div>
        {:else}
          <div class="detail-stack">
          <div class="detail-hero">
            <div>
              {#if editingProject}
                <div class="project-admin-grid">
                  <input bind:value={projectEditDraft.name} placeholder="Project name" />
                  <select bind:value={projectEditDraft.project_type}>
                    <option value="internal">Internal</option>
                    <option value="customer">Customer</option>
                    <option value="admin">Admin</option>
                  </select>
                  <select bind:value={projectEditDraft.status}>
                    <option value="active">Active</option>
                    <option value="shelved">Shelved</option>
                    <option value="archived">Archived</option>
                  </select>
                  <input bind:value={projectEditDraft.opportunity_key} placeholder="RFQ / folder key" />
                  <input bind:value={projectEditDraft.customer_name} placeholder="Customer" />
                  <input bind:value={projectEditDraft.end_user_name} placeholder="End user" />
                  <input bind:value={projectEditDraft.customer_poc_name} placeholder="Customer POC" />
                  <input bind:value={projectEditDraft.customer_poc_email} placeholder="POC email" />
                  <input bind:value={projectEditDraft.customer_poc_phone} placeholder="POC phone" />
                  <textarea bind:value={projectEditDraft.description} rows="2" placeholder="Project description"></textarea>
                </div>
              {:else}
                <h3>{selectedProject.name}</h3>
                <p>{selectedProject.description || "No project description provided."}</p>
                <div class="project-facts">
                  <span>{selectedProject.opportunity_key || "No workflow key"}</span>
                  <span>{selectedProject.customer_name || "No customer"}</span>
                  <span>{selectedProject.end_user_name || "No end user"}</span>
                  <span>{selectedProject.customer_poc_name || "No customer POC"}</span>
                </div>
              {/if}
            </div>
          </div>
          <div class="subpanel project-admin-actions">
            <div class="section-head">
              <h3>Admin Actions</h3>
              <span class="count">Audited</span>
            </div>
            <input bind:value={projectAdminReason} placeholder="Reason for archive, shelve, or delete" />
            <div class="actions">
              <button class="ghost-btn" on:click={() => applyProjectAdminAction("shelve")} disabled={savingProjectAdmin}>Shelve</button>
              <button class="ghost-btn" on:click={() => applyProjectAdminAction("archive")} disabled={savingProjectAdmin}>Archive</button>
              <button class="danger-btn" on:click={() => applyProjectAdminAction("delete")} disabled={savingProjectAdmin}>Delete</button>
            </div>
          </div>
          <div class="metrics-grid compact">
            <article class="metric-card">
              <span class="label">Open</span>
              <strong>{projectStats.open}</strong>
            </article>
            <article class="metric-card">
              <span class="label">Blocked</span>
              <strong>{projectStats.blocked}</strong>
            </article>
            <article class="metric-card">
              <span class="label">Completed</span>
              <strong>{projectStats.completed}</strong>
            </article>
            <article class="metric-card">
              <span class="label">Members</span>
              <strong>{projectStats.members}</strong>
            </article>
          </div>

            <div class="subpanel">
              <div class="section-head">
                <h3>Project Members</h3>
                <span class="count">{projectMembers.length}</span>
              </div>
              <div class="member-composer">
                <input bind:value={memberRole} placeholder="Role on project" />
                <button class="primary" on:click={handleAddProjectMembers} disabled={savingMember || memberSelections.length === 0}>
                  {memberSelections.length > 0 ? `Add ${memberSelections.length} Member${memberSelections.length === 1 ? "" : "s"}` : "Add Members"}
                </button>
              </div>
              {#if projectContextLoading}
                <div class="section-loading">
                  <WabiSpinner size="md" tempo="calm" />
                  <p>Loading members...</p>
                </div>
              {:else if availableProjectMembers.length > 0}
                <div class="member-picker">
                  {#each availableProjectMembers as employee}
                    <button
                      type="button"
                      class="member-pick"
                      class:selected={memberSelections.includes(employee.id)}
                      on:click={() => toggleProjectMemberSelection(employee.id)}
                    >
                      <strong>{employee.full_name}</strong>
                      <span>{employee.department || employee.job_title || "Team member"}</span>
                    </button>
                  {/each}
                </div>
              {/if}
              {#if projectMembers.length === 0}
                <div class="empty compact">No members assigned yet.</div>
              {:else}
                <div class="timeline member-timeline">
                  {#each projectMembers as member}
                    <div class="timeline-item">
                      <strong>{member.employee_name || "Unknown employee"}</strong>
                      <p>{member.role || "Member"}</p>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="subpanel">
              <div class="section-head">
                <h3>Project Tasks</h3>
                <span class="count">{projectTasks.length}</span>
              </div>
              {#if projectContextLoading}
                <div class="section-loading">
                  <WabiSpinner size="md" tempo="calm" />
                  <p>Loading project tasks...</p>
                </div>
              {:else if projectTasks.length === 0}
                <div class="empty compact">No tasks attached to this project yet.</div>
              {:else}
                <div class="timeline">
                  {#each projectTasks as task}
                    <button class="timeline-item selectable" class:selected={selectedTaskId === task.id} on:click={() => selectTask(task.id)}>
                      <strong>{task.title}</strong>
                      <p>{task.assignee_name || "Unassigned"} • {task.status || "open"}</p>
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="subpanel">
              <div class="section-head">
                <h3>Project Activity</h3>
                <span class="count">{projectActivity.length}</span>
              </div>
              {#if projectContextLoading}
                <div class="section-loading">
                  <WabiSpinner size="md" tempo="calm" />
                  <p>Loading project activity...</p>
                </div>
              {:else if projectActivity.length === 0}
                <div class="empty compact">No project activity yet.</div>
              {:else}
                <div class="timeline">
                  {#each projectActivity as item}
                    <div class="timeline-item">
                      <strong>{item.employee_name || "System"}</strong>
                      <p>{item.detail}</p>
                      <span class="meta">{formatDate(item.created_at)}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </Modal>
    </section>
  {/if}
</div>

<Modal bind:open={taskModalOpen} title={modalTask ? modalTask.title : "Task Detail"} size="lg" on:close={closeTaskModal}>
  {#if taskDetailLoading && !modalTask}
    <div class="modal-loading-state">
      <WabiSpinner size="lg" tempo="calm" />
      <p>Loading task history...</p>
    </div>
  {:else if !modalTask}
    <div class="empty compact">Choose a task to inspect comments and history.</div>
  {:else}
    <div class="task-modal-body">
      <div class="detail-hero">
        <div>
          <p class="eyebrow">{(modalTask.status || "open").replaceAll("_", " ")}</p>
          <h3>{modalTask.title}</h3>
          <p>{modalTask.description || "No task details provided."}</p>
        </div>
        <span class="pill {modalTask.priority || 'medium'}">{modalTask.priority || "medium"}</span>
      </div>

      <div class="task-meta chips">
        <span>Assignee: {modalTask.assignee_name || "Unassigned"}</span>
        <span>Project: {projectNameFor(modalTask)}</span>
        <span>{dueLabel(modalTask)}</span>
        <span>Created by: {modalTask.creator_name || "System"}</span>
      </div>

      <div class="subpanel modal-subpanel">
        <div class="section-head">
          <h3>Task Details</h3>
          <Button variant="secondary" size="sm" on:click={handleSaveTaskDetails} disabled={savingTaskDetails}>
            {savingTaskDetails ? "Saving..." : "Save Details"}
          </Button>
        </div>
        <label class="full-width">
          <span>Title</span>
          <input bind:value={selectedTaskTitleDraft} placeholder="Task title" />
        </label>
        <div class="form-grid compact-grid">
          <label>
            <span>Priority</span>
            <select bind:value={selectedTaskPriorityDraft}>
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
            </select>
          </label>
        </div>
        <label class="full-width">
          <span>Description</span>
          <textarea bind:value={selectedTaskDescriptionDraft} rows="3" placeholder="Context, outcome, or working notes"></textarea>
        </label>
      </div>

      <div class="subpanel modal-subpanel">
        <div class="section-head">
          <h3>Blocker</h3>
          <span class="count">{modalTask.status === "blocked" ? "Active" : "Optional"}</span>
        </div>
        <textarea
          bind:value={selectedTaskBlockerDraft}
          rows="3"
          placeholder="What is blocking this task? Include the dependency, owner, or next step."
        ></textarea>
      </div>

      <div class="editor-grid">
        <label class="assignment-field">
          <span>Assignee</span>
          <select bind:value={selectedTaskAssigneeDraft}>
            <option value="">Unassigned</option>
            {#each employees as employee}
              <option value={employee.id}>{employee.full_name}{employee.department ? ` • ${employee.department}` : ""}</option>
            {/each}
          </select>
        </label>
        <Button variant={selectedTaskAssigneeChanged ? "primary" : "secondary"} size="sm" on:click={handleReassignSelectedTask} disabled={savingAssignment || !selectedTaskAssigneeChanged}>
          {savingAssignment ? "Confirming..." : selectedTaskAssigneeChanged ? "Confirm Reassignment" : "Assignee Saved"}
        </Button>
        <input bind:value={selectedTaskDueDateDraft} type="date" />
        <Button variant="secondary" size="sm" on:click={handleUpdateDueDate} disabled={savingDueDate}>
          {savingDueDate ? "Saving..." : "Save Due Date"}
        </Button>
      </div>

      <div class="task-actions">
        {#if modalTask.status === "blocked"}
          <Button variant="success" size="sm" on:click={() => changeStatus(modalTask, "open")}>Unblock</Button>
        {/if}
        {#if modalTask.status !== "in_progress"}
          <Button variant="secondary" size="sm" on:click={() => changeStatus(modalTask, "in_progress")}>Start</Button>
        {/if}
        {#if modalTask.status !== "completed"}
          <Button variant="primary" size="sm" on:click={() => changeStatus(modalTask, "completed")}>Complete</Button>
        {/if}
        {#if modalTask.status !== "blocked"}
          <Button variant="warning" size="sm" on:click={handleBlockSelectedTask}>Block</Button>
        {/if}
        <Button variant={confirmDeleteTask ? "danger" : "ghost"} size="sm" on:click={handleDeleteSelectedTask} disabled={deletingTask}>
          {deletingTask ? "Deleting..." : confirmDeleteTask ? "Confirm Delete" : "Delete"}
        </Button>
      </div>

      <div class="task-modal-grid">
        <div class="subpanel modal-subpanel">
          <div class="section-head">
            <h3>Comments</h3>
            <span class="count">{taskComments.length}</span>
          </div>
          {#if taskDetailLoading}
            <div class="section-loading">
              <WabiSpinner size="md" tempo="calm" />
              <p>Loading comments...</p>
            </div>
          {:else if taskComments.length === 0}
            <div class="empty compact">No comments yet.</div>
          {:else}
            <div class="timeline">
              {#each taskComments as comment}
                <div class="timeline-item">
                  <strong>{comment.employee_name || "Unknown employee"}</strong>
                  <p>{comment.body}</p>
                  <span class="meta">{formatDate(comment.created_at)}</span>
                </div>
              {/each}
            </div>
          {/if}
          <div class="comment-composer">
            <textarea bind:value={commentDraft} rows="2" placeholder="Add a progress note or blocker update"></textarea>
            <Button variant="primary" size="sm" on:click={handleAddComment} disabled={savingComment || taskDetailLoading}>Comment</Button>
          </div>
        </div>

        <div class="subpanel modal-subpanel">
          <div class="section-head">
            <h3>Activity Timeline</h3>
            <span class="count">{taskActivity.length}</span>
          </div>
          {#if taskDetailLoading}
            <div class="section-loading">
              <WabiSpinner size="md" tempo="calm" />
              <p>Loading activity...</p>
            </div>
          {:else if taskActivity.length === 0}
            <div class="empty compact">No activity yet.</div>
          {:else}
            <div class="timeline">
              {#each taskActivity as item}
                <div class="timeline-item">
                  <strong>{item.employee_name || "System"}</strong>
                  <p>{item.detail}</p>
                  <span class="meta">{formatDate(item.created_at)}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</Modal>

<style>
  .page {
    padding: 16px;
    display: grid;
    gap: 12px;
  }

  .header,
  .work-topbar,
  .section-head,
  .task-top,
  .task-actions,
  .detail-hero,
  .board-column-head {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
  }

  .header {
    align-items: flex-start;
  }

  .work-topbar {
    align-items: stretch;
    min-height: 44px;
    gap: 8px;
  }

  h2,
  h3,
  p {
    margin: 0;
  }

  .subtitle,
  .meta,
  .count,
  .label,
  .section-head p {
    color: var(--text-secondary);
  }

  .identity-card,
  .tabs,
  .composer,
  .panel,
  .metric-card {
    border: 1px solid var(--border);
    border-radius: 16px;
    background: var(--surface);
  }

  .identity-card,
  .metric-card {
    padding: 10px 12px;
  }

  .identity-card {
    min-width: 180px;
    max-width: 240px;
    display: grid;
    gap: 2px;
  }

  .tabs {
    display: flex;
    gap: 4px;
    padding: 4px;
    align-items: center;
    flex: 1;
  }

  .tabs button,
  .selectable,
  .actions button {
    font: inherit;
  }

  .tabs button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: none;
    border-radius: 999px;
    min-height: 32px;
    padding: 7px 12px;
    background: transparent;
    color: var(--text-secondary);
    line-height: 1;
    white-space: nowrap;
    transition: background 140ms ease, color 140ms ease, box-shadow 140ms ease;
  }

  .tabs button.active {
    background: var(--onyx);
    color: white;
    box-shadow: 0 8px 18px rgba(15, 23, 42, 0.12);
  }

  .composer,
  .panel {
    padding: 12px;
  }

  .composer-grid,
  .workspace,
  .metrics-grid,
  .filter-grid,
  .editor-grid,
  .team-overview-grid,
  .task-gallery,
  .task-modal-grid,
  .insight-grid {
    display: grid;
    gap: 8px;
  }

  .composer-grid {
    grid-template-columns: 2fr repeat(4, 1fr);
    margin-top: 8px;
    margin-bottom: 8px;
  }

  .workspace {
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
  }

  .side-stack,
  .insight-list,
  .roster,
  .task-signals,
  .task-foot {
    display: grid;
    gap: 12px;
  }

  .metrics-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .metrics-grid.compact {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .team-overview-grid {
    grid-template-columns: minmax(320px, 0.7fr) minmax(0, 1.3fr);
  }

  .filter-grid {
    grid-template-columns: 2fr 1fr 1fr;
  }

  .editor-grid {
    grid-template-columns: 1.5fr auto 1fr auto;
  }

  .assignment-field {
    display: grid;
    gap: 6px;
  }

  .assignment-field span {
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
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
    padding: 8px 10px;
    background: var(--bg-base);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
  }

  .section-loading,
  .modal-loading-state {
    display: grid;
    place-items: center;
    gap: 10px;
    padding: 24px;
    color: var(--text-secondary);
    text-align: center;
  }

  .primary {
    border: none;
    border-radius: 10px;
    padding: 8px 12px;
    background: var(--onyx);
    color: white;
  }

  .primary.mini,
  .ghost-btn,
  .danger-btn {
    border-radius: 8px;
    padding: 8px 10px;
    font-size: 13px;
    font-weight: 700;
  }

  .ghost-btn {
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-primary);
  }

  .danger-btn {
    border: 1px solid #fecdd3;
    background: #fff1f2;
    color: #9f1239;
  }

  .danger {
    background: #fff1f2;
    color: #9f1239;
  }

  .task-list,
  .detail-stack,
  .timeline,
  .board-stack {
    display: grid;
    gap: 12px;
  }

  .project-list {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 12px;
  }

  .task-gallery {
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

  .task-gallery.two-up {
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  }

  .task-shell,
  .task-card,
  .mini-card,
  .timeline-item,
  .project-row,
  .employee-chip {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 10px 12px;
    background: var(--bg-base);
    text-align: left;
  }

  .project-row {
    min-height: 82px;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .task-card.selectable,
  .mini-card.selectable,
  .timeline-item.selectable,
  .project-row.selectable,
  .employee-chip {
    cursor: pointer;
  }

  .selected {
    border-color: var(--onyx);
    box-shadow: 0 0 0 1px var(--onyx);
  }

  .task-shell {
    display: grid;
    gap: 8px;
    min-height: 136px;
    transition: transform 140ms ease, box-shadow 140ms ease, border-color 140ms ease;
  }

  .task-shell:hover {
    transform: translateY(-1px);
    box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
  }

  .task-shell-top,
  .task-shell-meta,
  .board-lane-head {
    display: flex;
    justify-content: space-between;
    gap: 10px;
    align-items: center;
  }

  .task-shell-body {
    display: grid;
    gap: 5px;
    align-content: start;
  }

  .task-shell-body h3 {
    font-size: 14px;
    line-height: 1.3;
  }

  .task-shell-body p {
    color: var(--text-secondary);
    font-size: 13px;
    line-height: 1.35;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .task-shell-body .blocker-copy {
    color: #9f1239;
    font-weight: 500;
  }

  .task-shell-meta {
    flex-wrap: wrap;
    align-items: flex-start;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .task-shell-meta span {
    padding: 3px 7px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--bg-base) 82%, #e2e8f0 18%);
  }

  .task-status,
  .eyebrow {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .eyebrow {
    margin-bottom: 6px;
  }

  .timeline-item p,
  .employee-chip span {
    margin-top: 6px;
    color: var(--text-secondary);
  }

  .board-card {
    gap: 10px;
  }

  .task-signals,
  .task-foot {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .task-foot {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .task-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .compact-grid {
    grid-template-columns: minmax(0, 220px);
  }

  .pill {
    border-radius: 999px;
    padding: 4px 8px;
    font-size: 11.5px;
    text-transform: capitalize;
    background: #eef2ff;
    color: #312e81;
  }

  .pill.high,
  .pill.urgent {
    background: #fff1f2;
    color: #9f1239;
  }

  .subpanel {
    border-top: 1px solid var(--border);
    padding-top: 12px;
    display: grid;
    gap: 12px;
    align-content: start;
    grid-auto-rows: max-content;
  }

  .inline-actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .project-admin-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .project-admin-grid textarea {
    grid-column: 1 / -1;
  }

  .project-facts {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 10px;
  }

  .project-facts span {
    padding: 4px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .project-admin-actions .section-head {
    margin-bottom: 0;
  }

  .comment-composer,
  .member-composer,
  .project-composer {
    display: grid;
    gap: 10px;
  }

  .autocomplete-field {
    position: relative;
    min-width: 0;
  }

  .autocomplete-field input {
    width: 100%;
  }

  .suggestion-list {
    position: absolute;
    z-index: 20;
    top: calc(100% + 6px);
    left: 0;
    right: 0;
    max-height: 260px;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--surface);
    box-shadow: 0 14px 34px rgba(15, 23, 42, 0.14);
    padding: 6px;
  }

  .suggestion-list button {
    width: 100%;
    border: 0;
    border-radius: 8px;
    background: transparent;
    padding: 9px 10px;
    text-align: left;
    display: grid;
    gap: 2px;
    color: var(--text-primary);
  }

  .suggestion-list button:hover {
    background: var(--bg-base);
  }

  .suggestion-list span,
  .suggestion-empty {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .suggestion-empty {
    padding: 10px;
  }

  .member-picker {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 10px;
  }

  .member-pick {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 12px;
    background: color-mix(in srgb, var(--bg-base) 92%, #dbeafe 8%);
    text-align: left;
    display: grid;
    gap: 4px;
  }

  .member-pick.selected {
    border-color: var(--onyx);
    box-shadow: 0 0 0 1px var(--onyx);
    background: color-mix(in srgb, var(--bg-base) 82%, #dbeafe 18%);
  }

  .member-pick span {
    color: var(--text-secondary);
    font-size: 13px;
  }

  .member-timeline {
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  }

  .board-filters {
    padding: 8px 10px;
  }

  .board-sections {
    display: grid;
    gap: 10px;
    margin-top: 8px;
  }

  .board-lane {
    border: 1px solid var(--border);
    border-radius: 14px;
    background: var(--bg-base);
    padding: 10px;
    display: grid;
    gap: 8px;
  }

  .lane-gallery {
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

  .lane-empty {
    border: 1px dashed var(--border);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.45);
  }

  .drop-target {
    border-color: var(--onyx);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--onyx) 20%, transparent);
    background: color-mix(in srgb, var(--bg-base) 88%, var(--onyx) 12%);
  }

  .employee-chip {
    display: grid;
    gap: 2px;
    background: color-mix(in srgb, var(--bg-base) 88%, #dbeafe 12%);
    min-width: 170px;
    flex: 0 0 170px;
  }

  .employee-chip.dragging {
    opacity: 0.55;
  }

  .roster-strip {
    display: flex;
    gap: 8px;
    overflow-x: auto;
    padding-bottom: 4px;
  }

  .overview-panel {
    background: color-mix(in srgb, var(--surface) 88%, #ecfccb 12%);
  }

  .insight-row {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-base);
  }

  .insight-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .timeline-item strong,
  .project-row strong,
  .employee-chip strong {
    display: block;
  }

  .empty {
    padding: 16px 8px;
    color: var(--text-secondary);
    text-align: center;
  }

  .empty.compact {
    padding: 12px 8px;
  }

  .task-gallery-panel {
    display: grid;
    gap: 10px;
  }

  .roster-panel {
    display: grid;
    gap: 10px;
  }

  .task-modal-body {
    display: grid;
    gap: 12px;
  }

  .task-modal-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-items: start;
  }

  .modal-subpanel {
    border-top: none;
    padding-top: 0;
    align-self: start;
    height: auto;
    min-height: 0;
  }

  .task-meta.chips span {
    padding: 4px 8px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--bg-base) 85%, #e2e8f0 15%);
  }

  .task-modal-grid .timeline {
    align-content: start;
    grid-auto-rows: max-content;
  }

  .task-modal-grid .timeline-item {
    align-self: start;
    height: auto;
    min-height: 0;
  }

  .task-modal-grid .comment-composer,
  .task-modal-grid .section-loading,
  .task-modal-grid .empty {
    align-self: start;
  }

  @media (max-width: 1100px) {
    .workspace,
    .metrics-grid,
    .composer-grid,
    .filter-grid,
    .editor-grid,
    .metrics-grid.compact,
    .team-overview-grid,
    .task-modal-grid,
    .insight-grid,
    .project-admin-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 900px) {
    .header,
    .work-topbar,
    .task-top,
    .task-actions,
    .detail-hero,
    .section-head,
    .board-column-head {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
