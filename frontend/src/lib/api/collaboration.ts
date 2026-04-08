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
  return await ListEmployeeProfiles(activeOnly) as EmployeeProfile[];
}

export async function createEmployeeProfile(employee: Partial<EmployeeProfile>): Promise<EmployeeProfile> {
  return await CreateEmployeeProfile(employee as Record<string, any>) as EmployeeProfile;
}

export async function updateEmployeeProfile(employee: Partial<EmployeeProfile>): Promise<EmployeeProfile> {
  return await UpdateEmployeeProfile(employee as Record<string, any>) as EmployeeProfile;
}

export async function setEmployeeEmploymentState(
  employeeID: string,
  isActive: boolean,
  employmentStatus: string,
): Promise<EmployeeProfile> {
  return await SetEmployeeEmploymentState(employeeID, isActive, employmentStatus) as EmployeeProfile;
}

export async function reassignEmployeeManager(employeeID: string, managerEmployeeID: string): Promise<EmployeeProfile> {
  return await ReassignEmployeeManager(employeeID, managerEmployeeID) as EmployeeProfile;
}

export async function listEmployeeAccessLinks(): Promise<EmployeeAccessLink[]> {
  if (!isDesktop()) return [];
  return await ListEmployeeAccessLinks() as EmployeeAccessLink[];
}

export async function listEmployeeContributionSummaries(): Promise<EmployeeContributionSummary[]> {
  if (!isDesktop()) return [];
  return await ListEmployeeContributionSummaries() as EmployeeContributionSummary[];
}

export async function listEmployeeProjectAssignments(employeeID: string): Promise<ProjectMember[]> {
  if (!isDesktop()) return [];
  return await ListEmployeeProjectAssignments(employeeID) as ProjectMember[];
}

export async function createEmployeeAccessLink(link: Partial<EmployeeAccessLink>): Promise<EmployeeAccessLink> {
  return await CreateEmployeeAccessLink(link as Record<string, any>) as EmployeeAccessLink;
}

export async function listLicenseKeys(): Promise<LicenseKeySummary[]> {
  if (!isDesktop()) return [];
  return await ListLicenseKeys() as LicenseKeySummary[];
}

export async function listProjects(activeOnly = true): Promise<CollaborativeProject[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeProjects(activeOnly) as CollaborativeProject[];
}

export async function createProject(project: Partial<CollaborativeProject>): Promise<CollaborativeProject> {
  return await CreateCollaborativeProject(project as Record<string, any>) as CollaborativeProject;
}

export async function addProjectMember(projectID: string, employeeID: string, role: string): Promise<ProjectMember> {
  return await AddCollaborativeProjectMember(projectID, employeeID, role) as ProjectMember;
}

export async function listProjectMembers(projectID: string): Promise<ProjectMember[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeProjectMembers(projectID) as ProjectMember[];
}

export async function listProjectActivity(projectID: string): Promise<TaskActivityItem[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeProjectActivity(projectID) as TaskActivityItem[];
}

export async function listMyTasks(includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return await ListMyCollaborativeTasks(includeCompleted) as CollaborativeTask[];
}

export async function listTeamTasks(includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeTeamTasks(includeCompleted) as CollaborativeTask[];
}

export async function listProjectTasks(projectID: string, includeCompleted = false): Promise<CollaborativeTask[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeProjectTasks(projectID, includeCompleted) as CollaborativeTask[];
}

export async function getTask(taskID: string): Promise<CollaborativeTask | null> {
  if (!isDesktop()) return null;
  return await GetCollaborativeTask(taskID) as CollaborativeTask;
}

export async function createTask(task: Partial<CollaborativeTask>): Promise<CollaborativeTask> {
  return await CreateCollaborativeTask(task as Record<string, any>) as CollaborativeTask;
}

export async function updateTask(task: Partial<CollaborativeTask>): Promise<CollaborativeTask> {
  return await UpdateCollaborativeTask(task as Record<string, any>) as CollaborativeTask;
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
  return await ListCollaborativeTaskComments(taskID) as TaskCommentItem[];
}

export async function listTaskActivity(taskID: string): Promise<TaskActivityItem[]> {
  if (!isDesktop()) return [];
  return await ListCollaborativeTaskActivity(taskID) as TaskActivityItem[];
}

export async function listNotifications(limit = 50, unreadOnly = false): Promise<NotificationItem[]> {
  if (!isDesktop()) return [];
  return await ListNotificationFeed(limit, unreadOnly) as NotificationItem[];
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

export async function refreshCollaborativeWorkspace(): Promise<void> {
  if (!isDesktop()) return;
  await RefreshCollaborativeWorkspace();
}
