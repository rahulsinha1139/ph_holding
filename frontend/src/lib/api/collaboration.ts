import {
  AddCollaborativeProjectMember,
  AddCollaborativeTaskComment,
  CreateCollaborativeProject,
  CreateCollaborativeTask,
  DeleteCollaborativeTask,
  CreateEmployeeAccessLink,
  CreateEmployeeProfile,
  GetCollaborativeTask,
  GetCurrentEmployeeContext,
  GetUnreadNotificationsCount,
  ListEmployeeContributionSummaries,
  ListCollaborativeProjects,
  ListCollaborativeProjectMembers,
  ListCollaborativeProjectActivity,
  ListCollaborativeProjectTasks,
  ListCollaborativeTaskActivity,
  ListCollaborativeTaskComments,
  ListCollaborativeTeamTasks,
  ListEmployeeAccessLinks,
  ListEmployeeProjectAssignments,
  ListEmployeeProfiles,
  ListLicenseKeys,
  ListMyCollaborativeTasks,
  ListNotificationFeed,
  MarkNotificationAsRead,
  ReassignCollaborativeTask,
  ReassignEmployeeManager,
  RefreshCollaborativeWorkspace,
  SetEmployeeEmploymentState,
  UpdateCollaborativeTaskDueDate,
  UpdateCollaborativeTask,
  UpdateCollaborativeTaskStatus,
  UpdateEmployeeProfile,
} from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";
import { buildWailsInput, normalizeWailsDateTime } from "$lib/utils/wailsInterop";

export interface EmployeeProfile {
  id: string;
  employee_code: string;
  full_name: string;
  preferred_name?: string;
  email?: string;
  phone?: string;
  department?: string;
  job_title?: string;
  employment_status?: string;
  manager_employee_id?: string;
  manager_name?: string;
  start_date?: string;
  end_date?: string;
  emergency_contact?: string;
  notes?: string;
  is_active?: boolean;
}

export interface EmployeeAccessLink {
  id: string;
  employee_id: string;
  employee_name?: string;
  license_key: string;
  user_id?: string;
  device_id?: string;
  device_name?: string;
  access_status?: string;
  is_primary?: boolean;
}

export interface CurrentEmployeeContext {
  employee_id: string;
  employee_name: string;
  license_key?: string;
  license_role?: string;
  device_id?: string;
  user_id?: string;
  resolved_by?: string;
  permissions?: string[];
}

export interface CollaborativeProject {
  id: string;
  name: string;
  project_type?: string;
  description?: string;
  status?: string;
  customer_id?: string;
  opportunity_id?: string;
  order_id?: string;
  starts_on?: string;
  ends_on?: string;
}

export interface ProjectMember {
  id: string;
  project_id: string;
  employee_id: string;
  employee_name?: string;
  project_name?: string;
  role?: string;
  allocation_percent?: number;
  is_active?: boolean;
  joined_at?: string;
  left_at?: string;
}

export interface CollaborativeTask {
  id: string;
  title: string;
  description?: string;
  task_type?: string;
  status?: string;
  blocked_reason?: string;
  priority?: string;
  due_date?: string;
  customer_id?: string;
  opportunity_id?: string;
  order_id?: string;
  project_id?: string;
  creator_employee_id?: string;
  assignee_employee_id?: string;
  creator_name?: string;
  assignee_name?: string;
  completed_at?: string;
  started_at?: string;
  last_comment_at?: string;
}

export interface TaskCommentItem {
  id: string;
  task_id: string;
  employee_id: string;
  employee_name?: string;
  body: string;
  created_at?: string;
}

export interface TaskActivityItem {
  id: string;
  task_id: string;
  employee_id: string;
  employee_name?: string;
  activity_type?: string;
  detail: string;
  metadata_json?: string;
  created_at?: string;
}

export interface NotificationItem {
  id: string;
  notification_type?: string;
  title: string;
  message: string;
  status?: string;
  source_type?: string;
  source_id?: string;
  action_route?: string;
  action_payload?: string;
  read_at?: string;
  created_at?: string;
}

export interface LicenseKeySummary {
  id: string;
  key: string;
  role: string;
  device_id?: string;
  assigned_to?: string;
  display_name?: string;
  status?: string;
}

export interface EmployeeContributionSummary {
  employee_id: string;
  employee_code: string;
  employee_name: string;
  department?: string;
  job_title?: string;
  manager_employee_id?: string;
  manager_name?: string;
  employment_status?: string;
  is_active?: boolean;
  active_project_count: number;
  active_task_count: number;
  completed_task_count: number;
  blocked_task_count: number;
  overdue_task_count: number;
  completion_rate: number;
  primary_license_key?: string;
  primary_device_name?: string;
}

const isDesktop = () => Boolean((window as any)?.go?.main?.App);
const COLLABORATIVE_REFRESH_INTERVAL_MS = 15_000;

let collaborativeRefreshInFlight: Promise<void> | null = null;
let lastCollaborativeRefreshAt = 0;

function toEmployeeProfile(employee: main.Employee): EmployeeProfile {
  return {
    id: employee.id,
    employee_code: employee.employee_code,
    full_name: employee.full_name,
    preferred_name: employee.preferred_name,
    email: employee.email,
    phone: employee.phone,
    department: employee.department,
    job_title: employee.job_title,
    employment_status: employee.employment_status,
    manager_employee_id: employee.manager_employee_id,
    manager_name: employee.manager_name,
    start_date: normalizeWailsDateTime(employee.start_date),
    end_date: normalizeWailsDateTime(employee.end_date),
    emergency_contact: employee.emergency_contact,
    notes: employee.notes,
    is_active: employee.is_active,
  };
}

function toEmployeeAccessLink(link: main.EmployeeAccessLink): EmployeeAccessLink {
  return {
    id: link.id,
    employee_id: link.employee_id,
    employee_name: link.employee_name,
    license_key: link.license_key,
    user_id: link.user_id,
    device_id: link.device_id,
    device_name: link.device_name,
    access_status: link.access_status,
    is_primary: link.is_primary,
  };
}

function toProject(project: main.Project): CollaborativeProject {
  return {
    id: project.id,
    name: project.name,
    project_type: project.project_type,
    description: project.description,
    status: project.status,
    customer_id: project.customer_id,
    opportunity_id: project.opportunity_id,
    order_id: project.order_id,
    starts_on: normalizeWailsDateTime(project.starts_on),
    ends_on: normalizeWailsDateTime(project.ends_on),
  };
}

function toProjectMember(member: main.ProjectMember): ProjectMember {
  return {
    id: member.id,
    project_id: member.project_id,
    employee_id: member.employee_id,
    employee_name: member.employee_name,
    project_name: member.project_name,
    role: member.role,
    allocation_percent: member.allocation_percent,
    is_active: member.is_active,
    joined_at: normalizeWailsDateTime(member.joined_at),
    left_at: normalizeWailsDateTime(member.left_at),
  };
}

function toTask(task: main.TaskItem): CollaborativeTask {
  return {
    id: task.id,
    title: task.title,
    description: task.description,
    task_type: task.task_type,
    status: task.status,
    blocked_reason: task.blocked_reason,
    priority: task.priority,
    due_date: normalizeWailsDateTime(task.due_date),
    customer_id: task.customer_id,
    opportunity_id: task.opportunity_id,
    order_id: task.order_id,
    project_id: task.project_id,
    creator_employee_id: task.creator_employee_id,
    assignee_employee_id: task.assignee_employee_id,
    creator_name: task.creator_name,
    assignee_name: task.assignee_name,
    completed_at: normalizeWailsDateTime(task.completed_at),
    started_at: normalizeWailsDateTime(task.started_at),
    last_comment_at: normalizeWailsDateTime(task.last_comment_at),
  };
}

function toTaskComment(comment: main.TaskComment): TaskCommentItem {
  return {
    id: comment.id,
    task_id: comment.task_id,
    employee_id: comment.employee_id,
    employee_name: comment.employee_name,
    body: comment.body,
    created_at: normalizeWailsDateTime(comment.created_at),
  };
}

function toTaskActivity(activity: main.TaskActivity): TaskActivityItem {
  return {
    id: activity.id,
    task_id: activity.task_id,
    employee_id: activity.employee_id,
    employee_name: activity.employee_name,
    activity_type: activity.activity_type,
    detail: activity.detail,
    metadata_json: activity.metadata_json,
    created_at: normalizeWailsDateTime(activity.created_at),
  };
}

function toNotification(notification: main.Notification): NotificationItem {
  return {
    id: notification.id,
    notification_type: notification.notification_type,
    title: notification.title,
    message: notification.message,
    status: notification.status,
    source_type: notification.source_type,
    source_id: notification.source_id,
    action_route: notification.action_route,
    action_payload: notification.action_payload,
    read_at: normalizeWailsDateTime(notification.read_at),
    created_at: normalizeWailsDateTime(notification.created_at),
  };
}

function toLicenseKeySummary(key: main.LicenseKey): LicenseKeySummary {
  return {
    id: String(key.id),
    key: key.key,
    role: key.role,
    device_id: key.device_hash || undefined,
    assigned_to: key.display_name || undefined,
    display_name: key.display_name || undefined,
    status: key.activated ? "Activated" : "Available",
  };
}

export async function getCurrentEmployeeContext(): Promise<CurrentEmployeeContext | null> {
  if (!isDesktop()) return null;
  try {
    return await GetCurrentEmployeeContext() as CurrentEmployeeContext;
  } catch {
    return null;
  }
}

export async function listEmployeeProfiles(activeOnly = true): Promise<EmployeeProfile[]> {
  if (!isDesktop()) return [];
  return (await ListEmployeeProfiles(activeOnly)).map(toEmployeeProfile);
}

export async function createEmployeeProfile(employee: Partial<EmployeeProfile>): Promise<EmployeeProfile> {
  return toEmployeeProfile(await CreateEmployeeProfile(buildWailsInput(main.Employee, employee as Record<string, any>)));
}

export async function updateEmployeeProfile(employee: Partial<EmployeeProfile>): Promise<EmployeeProfile> {
  return toEmployeeProfile(await UpdateEmployeeProfile(buildWailsInput(main.Employee, employee as Record<string, any>)));
}

export async function setEmployeeEmploymentState(
  employeeID: string,
  isActive: boolean,
  employmentStatus: string,
): Promise<EmployeeProfile> {
  return toEmployeeProfile(await SetEmployeeEmploymentState(employeeID, isActive, employmentStatus));
}

export async function reassignEmployeeManager(employeeID: string, managerEmployeeID: string): Promise<EmployeeProfile> {
  return toEmployeeProfile(await ReassignEmployeeManager(employeeID, managerEmployeeID));
}

export async function listEmployeeAccessLinks(): Promise<EmployeeAccessLink[]> {
  if (!isDesktop()) return [];
  return (await ListEmployeeAccessLinks()).map(toEmployeeAccessLink);
}

export async function listEmployeeContributionSummaries(): Promise<EmployeeContributionSummary[]> {
  if (!isDesktop()) return [];
  return await ListEmployeeContributionSummaries() as EmployeeContributionSummary[];
}

export async function listEmployeeProjectAssignments(employeeID: string): Promise<ProjectMember[]> {
  if (!isDesktop()) return [];
  return (await ListEmployeeProjectAssignments(employeeID)).map(toProjectMember);
}

export async function createEmployeeAccessLink(link: Partial<EmployeeAccessLink>): Promise<EmployeeAccessLink> {
  return toEmployeeAccessLink(await CreateEmployeeAccessLink(buildWailsInput(main.EmployeeAccessLink, link as Record<string, any>)));
}

export async function listLicenseKeys(): Promise<LicenseKeySummary[]> {
  if (!isDesktop()) return [];
  return (await ListLicenseKeys()).map(toLicenseKeySummary);
}

export async function listProjects(activeOnly = true): Promise<CollaborativeProject[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeProjects(activeOnly)).map(toProject);
}

export async function createProject(project: Partial<CollaborativeProject>): Promise<CollaborativeProject> {
  return toProject(await CreateCollaborativeProject(buildWailsInput(main.Project, project as Record<string, any>)));
}

export async function addProjectMember(projectID: string, employeeID: string, role: string): Promise<ProjectMember> {
  return toProjectMember(await AddCollaborativeProjectMember(projectID, employeeID, role));
}

export async function listProjectMembers(projectID: string): Promise<ProjectMember[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeProjectMembers(projectID)).map(toProjectMember);
}

export async function listProjectActivity(projectID: string): Promise<TaskActivityItem[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeProjectActivity(projectID)).map(toTaskActivity);
}

export async function listMyTasks(includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return (await ListMyCollaborativeTasks(includeCompleted)).map(toTask);
}

export async function listTeamTasks(includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeTeamTasks(includeCompleted)).map(toTask);
}

export async function listProjectTasks(projectID: string, includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeProjectTasks(projectID, includeCompleted)).map(toTask);
}

export async function getTask(taskID: string): Promise<CollaborativeTask | null> {
  if (!isDesktop()) return null;
  return toTask(await GetCollaborativeTask(taskID));
}

export async function createTask(task: Partial<CollaborativeTask>): Promise<CollaborativeTask> {
  return toTask(await CreateCollaborativeTask(buildWailsInput(main.TaskItem, task as Record<string, any>)));
}

export async function updateTask(task: Partial<CollaborativeTask>): Promise<CollaborativeTask> {
  return toTask(await UpdateCollaborativeTask(buildWailsInput(main.TaskItem, task as Record<string, any>)));
}

export async function updateTaskStatus(taskID: string, status: string, note = ""): Promise<void> {
  await UpdateCollaborativeTaskStatus(taskID, status, note);
}

export async function reassignTask(taskID: string, assigneeEmployeeID: string): Promise<void> {
  await ReassignCollaborativeTask(taskID, assigneeEmployeeID);
}

export async function updateTaskDueDate(taskID: string, dueDateISO: string): Promise<void> {
  await UpdateCollaborativeTaskDueDate(taskID, dueDateISO);
}

export async function addTaskComment(taskID: string, body: string): Promise<Record<string, any>> {
  return await AddCollaborativeTaskComment(taskID, body);
}

export async function deleteTask(taskID: string): Promise<void> {
  await DeleteCollaborativeTask(taskID);
}

export async function listTaskComments(taskID: string): Promise<TaskCommentItem[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeTaskComments(taskID)).map(toTaskComment);
}

export async function listTaskActivity(taskID: string): Promise<TaskActivityItem[]> {
  if (!isDesktop()) return [];
  return (await ListCollaborativeTaskActivity(taskID)).map(toTaskActivity);
}

export async function listNotifications(limit = 50, unreadOnly = false): Promise<NotificationItem[]> {
  if (!isDesktop()) return [];
  return (await ListNotificationFeed(limit, unreadOnly)).map(toNotification);
}

export async function getUnreadNotificationsCount(): Promise<number> {
  if (!isDesktop()) return 0;
  try {
    return await GetUnreadNotificationsCount();
  } catch {
    return 0;
  }
}

export async function markNotificationAsRead(notificationID: string): Promise<void> {
  await MarkNotificationAsRead(notificationID);
}

export async function refreshCollaborativeWorkspace(
  options: { force?: boolean; minIntervalMs?: number } = {},
): Promise<void> {
  if (!isDesktop()) return;
  const {
    force = false,
    minIntervalMs = COLLABORATIVE_REFRESH_INTERVAL_MS,
  } = options;
  const now = Date.now();

  if (!force && collaborativeRefreshInFlight) {
    return await collaborativeRefreshInFlight;
  }

  if (!force && lastCollaborativeRefreshAt > 0 && now - lastCollaborativeRefreshAt < minIntervalMs) {
    return;
  }

  collaborativeRefreshInFlight = RefreshCollaborativeWorkspace()
    .then(() => {
      lastCollaborativeRefreshAt = Date.now();
    })
    .finally(() => {
      collaborativeRefreshInFlight = null;
    });

  return await collaborativeRefreshInFlight;
}
