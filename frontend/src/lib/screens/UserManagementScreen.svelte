<script lang="ts">
    import { onMount } from "svelte";
    import { fade } from "svelte/transition";
    import WabiSpinner from "../components/ui/WabiSpinner.svelte";
    import {
        ListUsers,
        ListRoles,
        GetAuditLogs,
        GetRolePermissionsList,
        CreateUser,
        UpdateUser,
        CanViewUserActivityMonitoring,
        CanResolveOpportunityConflicts,
        GetWeeklyUserActivityReport,
        ListOpportunityEditConflicts,
        ResolveOpportunityEditConflict,
    } from "../../../wailsjs/go/main/App";
    import { toast } from "../stores/toasts";

    type RoleRecord = {
        id?: string | number;
        ID?: string | number;
        name?: string;
        display_name?: string;
        description?: string;
        permissions?: string;
        is_active?: boolean;
    };

    type UserRecord = {
        id?: string | number;
        username?: string;
        email?: string;
        full_name?: string;
        display_name?: string;
        department?: string;
        job_title?: string;
        role_id?: string | number;
        role_name?: string;
        role?: RoleRecord | string;
        status?: string;
        is_active?: boolean;
    };

    let activeTab = "users"; // users, roles, audit
    let loading = false;
    let canViewActivityMonitoring = false;
    let loadingActivity = false;
    let activityReport: any = null;
    let activityError = "";
    let activityWeekStart = currentWeekStart();
    let canResolveOpportunityConflicts = false;
    let loadingConflicts = false;
    let opportunityConflicts: any[] = [];
    let conflictStatusFilter = "pending";

    // Data
    let users: UserRecord[] = [];
    let roles: RoleRecord[] = [];
    let auditLogs: any[] = [];
    let searchQuery = "";
    let showUserModal = false;
    let savingUser = false;
    let editingUser: UserRecord | null = null;
    let selectedRoleForPermissions: string | null = null;
    let loadingRolePermissionsFor = "";
    let rolePermissionOverrides: Record<string, string[]> = {};
    let userForm = {
        username: "",
        email: "",
        password: "",
        full_name: "",
        department: "",
        job_title: "",
        role_id: "",
        is_active: true,
    };

    async function loadData() {
        loading = true;
        try {
            if (!window.go) {
                // Mock
                users = [
                    {
                        id: "role-admin-user",
                        username: "admin",
                        full_name: "Admin User",
                        email: "admin@example.local",
                        role_id: "role-admin",
                        role_name: "Administrator",
                        is_active: true,
                    },
                    {
                        id: "role-sales-user",
                        username: "jdoe",
                        full_name: "John Doe",
                        email: "jdoe@example.local",
                        role_id: "role-sales",
                        role_name: "Sales",
                        is_active: true,
                    },
                    {
                        id: "role-ops-user",
                        username: "asmith",
                        full_name: "Alice Smith",
                        email: "asmith@example.local",
                        role_id: "role-operations",
                        role_name: "Operations",
                        is_active: false,
                    },
                ];
                roles = [
                    {
                        id: "role-admin",
                        name: "admin",
                        display_name: "Administrator",
                        permissions: "*",
                    },
                    {
                        id: "role-sales",
                        name: "sales",
                        display_name: "Sales",
                        permissions: "customers:read, opportunities:update",
                    },
                    {
                        id: "role-operations",
                        name: "operations",
                        display_name: "Operations",
                        permissions: "purchase_orders:update, delivery_notes:update",
                    },
                ];
                auditLogs = [
                    {
                        ts: new Date().toISOString(),
                        action: "LOGIN",
                        user: "admin",
                        details: "Successful login",
                    },
                    {
                        ts: new Date().toISOString(),
                        action: "UPDATE_USER",
                        user: "admin",
                        details: "Updated user jdoe",
                    },
                ];
            } else {
                const [u, r] = await Promise.all([
                    ListUsers().catch(() => []),
                    ListRoles().catch(() => []),
                ]);
                users = u || [];
                roles = r || [];
                const [monitoringAllowed, conflictAllowed] = await Promise.all([
                    CanViewUserActivityMonitoring().catch(() => false),
                    CanResolveOpportunityConflicts().catch(() => false),
                ]);
                canViewActivityMonitoring = monitoringAllowed;
                canResolveOpportunityConflicts = conflictAllowed;
            }
        } catch (e) {
            toast.danger("Load failed");
        } finally {
            loading = false;
        }
    }

    async function fetchAudit() {
        if (auditLogs.length > 0) return; // cache slightly
        loading = true;
        try {
            if (window.go) {
                auditLogs = (await GetAuditLogs(50, "all", "")) || [];
            }
        } catch (e) {
        } finally {
            loading = false;
        }
    }

    function currentWeekStart() {
        const d = new Date();
        const day = d.getDay() || 7;
        d.setDate(d.getDate() - day + 1);
        d.setHours(0, 0, 0, 0);
        return d.toISOString().slice(0, 10);
    }

    async function fetchActivityReport(force = false) {
        if (!canViewActivityMonitoring) return;
        if (activityReport && !force) return;
        loadingActivity = true;
        activityError = "";
        try {
            if (window.go) {
                activityReport = await GetWeeklyUserActivityReport(activityWeekStart);
            }
        } catch (e) {
            activityReport = null;
            activityError = String(e);
        } finally {
            loadingActivity = false;
        }
    }

    function maxMeaningfulHours(rows: any[]) {
        return Math.max(1, ...((rows || []).map((row) => Number(row.meaningful_hours || 0))));
    }

    function barWidth(row: any, rows: any[]) {
        return `${Math.max(4, (Number(row.meaningful_hours || 0) / maxMeaningfulHours(rows)) * 100)}%`;
    }

    async function fetchOpportunityConflicts(force = false) {
        if (!canResolveOpportunityConflicts) return;
        if (opportunityConflicts.length > 0 && !force) return;
        loadingConflicts = true;
        try {
            if (window.go) {
                opportunityConflicts = await ListOpportunityEditConflicts(conflictStatusFilter, 100);
            }
        } catch (e) {
            toast.danger("Failed to load opportunity conflicts");
            opportunityConflicts = [];
        } finally {
            loadingConflicts = false;
        }
    }

    async function resolveOpportunityConflict(conflict: any, action: "apply" | "reject") {
        if (!conflict?.id) return;
        loadingConflicts = true;
        try {
            await ResolveOpportunityEditConflict(String(conflict.id), action, action === "apply" ? "Applied from admin review" : "Rejected from admin review");
            toast.success(action === "apply" ? "Conflict applied" : "Conflict rejected");
            await fetchOpportunityConflicts(true);
        } catch (e) {
            toast.danger(`Conflict resolution failed: ${e?.message || e}`);
        } finally {
            loadingConflicts = false;
        }
    }

    function conflictSummary(raw: string) {
        try {
            const data = JSON.parse(raw || "{}");
            return Object.entries(data)
                .filter(([, value]) => String(value || "").trim())
                .map(([key, value]) => `${key.replaceAll("_", " ")}: ${value}`)
                .join(" | ") || "No proposed changes";
        } catch {
            return raw || "No proposed changes";
        }
    }

    function resetUserForm() {
        userForm = {
            username: "",
            email: "",
            password: "",
            full_name: "",
            department: "",
            job_title: "",
            role_id: roles[0] ? roleId(roles[0]) : "",
            is_active: true,
        };
    }

    function openCreateUser() {
        editingUser = null;
        resetUserForm();
        showUserModal = true;
    }

    function openEditUser(user: UserRecord) {
        editingUser = user;
        userForm = {
            username: user.username || "",
            email: user.email || "",
            password: "",
            full_name: user.full_name || user.display_name || "",
            department: user.department || "",
            job_title: user.job_title || "",
            role_id: String(user.role_id || ""),
            is_active: userStatusLabel(user) === "Active",
        };
        showUserModal = true;
    }

    function closeUserModal() {
        if (savingUser) return;
        showUserModal = false;
        editingUser = null;
        resetUserForm();
    }

    function roleId(role: RoleRecord) {
        return String(role?.id || role?.ID || role?.name || "");
    }

    function roleLabel(roleOrID: RoleRecord | string | number | undefined) {
        if (typeof roleOrID === "object" && roleOrID) {
            return roleOrID.display_name || roleOrID.name || "Role";
        }
        const role = roles.find((r) => roleId(r) === String(roleOrID || ""));
        return role?.display_name || role?.name || "";
    }

    function userDisplayName(user: UserRecord) {
        return user.full_name || user.display_name || user.username || "Unnamed user";
    }

    function userInitial(user: UserRecord) {
        return userDisplayName(user).trim().charAt(0).toUpperCase() || "?";
    }

    function userRoleLabel(user: UserRecord) {
        if (typeof user.role === "string") return user.role;
        return (
            user.role_name ||
            user.role?.display_name ||
            user.role?.name ||
            roleLabel(user.role_id) ||
            "Unassigned"
        );
    }

    function userStatusLabel(user: UserRecord) {
        if (typeof user.is_active === "boolean") {
            return user.is_active ? "Active" : "Inactive";
        }
        return user.status || "Active";
    }

    function localRolePermissions(role: RoleRecord) {
        const raw = (role.permissions || "").trim();
        if (!raw) return ["No explicit permissions configured"];
        if (raw === "*") return ["Full system access"];
        try {
            const parsed = JSON.parse(raw);
            if (Array.isArray(parsed)) return parsed.map(String);
            if (parsed && typeof parsed === "object") {
                return Object.entries(parsed)
                    .filter(([, value]) => Boolean(value))
                    .map(([key]) => key);
            }
        } catch (e) {
            // permissions may be stored as a comma-separated legacy string
        }
        return raw
            .split(/[,;\n]/)
            .map((permission) => permission.trim())
            .filter(Boolean);
    }

    async function viewRolePermissions(role: RoleRecord) {
        const roleID = roleId(role);
        selectedRoleForPermissions =
            selectedRoleForPermissions === roleID ? null : roleID;
        if (!selectedRoleForPermissions || rolePermissionOverrides[roleID]) return;
        loadingRolePermissionsFor = roleID;
        try {
            if (window.go) {
                rolePermissionOverrides = {
                    ...rolePermissionOverrides,
                    [roleID]: (await GetRolePermissionsList(role?.name || roleLabel(role))) || [],
                };
            } else {
                rolePermissionOverrides = {
                    ...rolePermissionOverrides,
                    [roleID]: localRolePermissions(role),
                };
            }
        } catch (e) {
            rolePermissionOverrides = {
                ...rolePermissionOverrides,
                [roleID]: localRolePermissions(role),
            };
            toast.danger(`Failed to load permissions: ${String(e)}`);
        } finally {
            loadingRolePermissionsFor = "";
        }
    }

    async function saveUser() {
        const roleID = String(userForm.role_id || "");
        if (!userForm.full_name.trim() || !userForm.email.trim() || !roleID) {
            toast.danger("Full name, email, and role are required");
            return;
        }
        if (!editingUser && (!userForm.username.trim() || !userForm.password)) {
            toast.danger("Username and password are required for new users");
            return;
        }

        savingUser = true;
        try {
            if (!window.go) {
                if (editingUser) {
                    users = users.map((user) =>
                        user.id === editingUser?.id
                            ? {
                                  ...user,
                                  email: userForm.email,
                                  full_name: userForm.full_name,
                                  department: userForm.department,
                                  job_title: userForm.job_title,
                                  role_id: roleID,
                                  role_name: roleLabel(roleID),
                                  is_active: userForm.is_active,
                              }
                            : user,
                    );
                } else {
                    users = [
                        ...users,
                        {
                            id: crypto.randomUUID(),
                            username: userForm.username,
                            email: userForm.email,
                            full_name: userForm.full_name,
                            department: userForm.department,
                            job_title: userForm.job_title,
                            role_id: roleID,
                            role_name: roleLabel(roleID),
                            is_active: true,
                        },
                    ];
                }
            } else if (editingUser?.id) {
                await UpdateUser(
                    String(editingUser.id),
                    userForm.full_name.trim(),
                    userForm.email.trim(),
                    userForm.department.trim(),
                    userForm.job_title.trim(),
                    roleID,
                    userForm.is_active,
                );
                await loadData();
            } else {
                await CreateUser(
                    userForm.username.trim(),
                    userForm.email.trim(),
                    userForm.password,
                    userForm.full_name.trim(),
                    userForm.department.trim(),
                    userForm.job_title.trim(),
                    roleID,
                );
                await loadData();
            }
            toast.success(editingUser ? "User updated" : "User created");
            closeUserModal();
        } catch (e) {
            toast.danger(`Save failed: ${String(e)}`);
        } finally {
            savingUser = false;
        }
    }

    $: filteredUsers = users.filter(
        (u) => {
            const term = searchQuery.trim().toLowerCase();
            const haystack = [
                u.full_name,
                u.display_name,
                u.username,
                u.email,
                u.department,
                u.job_title,
                userRoleLabel(u),
            ]
                .filter(Boolean)
                .join(" ")
                .toLowerCase();
            return !term || haystack.includes(term);
        },
    );

    onMount(loadData);
</script>

<div class="page">
    <header class="header">
        <div class="header-content">
            <h1>Team.</h1>
            <p class="subtitle">User Management & Security</p>
        </div>
        {#if activeTab === "users"}
            <button class="btn-primary" on:click={openCreateUser}
                >+ Add User</button
            >
        {/if}
    </header>

    <div class="tabs">
        <button
            class="tab"
            class:active={activeTab === "users"}
            on:click={() => (activeTab = "users")}>Users</button
        >
        <button
            class="tab"
            class:active={activeTab === "roles"}
            on:click={() => (activeTab = "roles")}>Roles</button
        >
        <button
            class="tab"
            class:active={activeTab === "audit"}
            on:click={() => {
                activeTab = "audit";
                fetchAudit();
            }}>Audit Logs</button
        >
        {#if canViewActivityMonitoring}
            <button
                class="tab"
                class:active={activeTab === "activity"}
                on:click={() => {
                    activeTab = "activity";
                    fetchActivityReport();
                }}>Activity</button
            >
        {/if}
        {#if canResolveOpportunityConflicts}
            <button
                class="tab"
                class:active={activeTab === "conflicts"}
                on:click={() => {
                    activeTab = "conflicts";
                    fetchOpportunityConflicts();
                }}>Conflicts</button
            >
        {/if}
    </div>

    <main class="content-area">
        {#if loading}
            <div class="loading"><WabiSpinner size="lg" /></div>
        {:else if activeTab === "users"}
            <div class="toolbar">
                <input
                    type="text"
                    placeholder="Search users..."
                    bind:value={searchQuery}
                    class="search-input"
                />
            </div>
            <div class="table-card" in:fade>
                <table>
                    <thead
                        ><tr
                            ><th>User</th><th>Role</th><th>Status</th><th
                                class="right">Actions</th
                            ></tr
                        ></thead
                    >
                    <tbody>
                        {#each filteredUsers as u}
                            <tr>
                                <td>
                                    <div class="user-cell">
                                        <div class="avatar">
                                            {userInitial(u)}
                                        </div>
                                        <div class="info">
                                            <div class="name">
                                                {userDisplayName(u)}
                                            </div>
                                            <div class="sub">@{u.username}</div>
                                        </div>
                                    </div>
                                </td>
                                <td
                                    ><span class="badge role"
                                        >{userRoleLabel(u)}</span
                                    ></td
                                >
                                <td
                                    ><span
                                        class="dot {userStatusLabel(u) === 'Active'
                                            ? 'green'
                                            : 'red'}"
                                    ></span>
                                    {userStatusLabel(u)}</td
                                >
                                <td class="right"
                                    ><button
                                        class="btn-sm"
                                        on:click={() => openEditUser(u)}
                                        >Edit</button
                                    ></td
                                >
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {:else if activeTab === "roles"}
            <div class="grid-roles" in:fade>
                {#each roles as r}
                    <div class="card role-card">
                        <h3>{r.display_name || r.name}</h3>
                        <p>{r.description || "Permissions managed via configuration."}</p>
                        <button
                            class="btn-txt"
                            on:click={() => viewRolePermissions(r)}
                            >View Permissions</button
                        >
                        {#if selectedRoleForPermissions === roleId(r)}
                            <ul class="permission-list">
                                {#if loadingRolePermissionsFor === roleId(r)}
                                    <li>Loading permissions...</li>
                                {:else}
                                    {#each (rolePermissionOverrides[roleId(r)] || localRolePermissions(r)) as permission}
                                        <li>{permission}</li>
                                    {/each}
                                {/if}
                            </ul>
                        {/if}
                    </div>
                {/each}
            </div>
        {:else if activeTab === "audit"}
            <div class="table-card" in:fade>
                <table>
                    <thead
                        ><tr
                            ><th>Timestamp</th><th>Action</th><th>User</th><th
                                >Details</th
                            ></tr
                        ></thead
                    >
                    <tbody>
                        {#each auditLogs as log}
                            <tr>
                                <td class="mono"
                                    >{new Date(log.ts).toLocaleString()}</td
                                >
                                <td
                                    ><span class="badge neutral"
                                        >{log.action}</span
                                    ></td
                                >
                                <td class="bold">{log.user}</td>
                                <td class="dim">{log.details}</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {:else if activeTab === "activity"}
            <div class="activity-panel" in:fade>
                <div class="activity-toolbar">
                    <div>
                        <h2>User Activity</h2>
                        <p>Weekly active-hours, meaningful work, and action mix by employee.</p>
                    </div>
                    <div class="activity-controls">
                        <input type="date" bind:value={activityWeekStart} />
                        <button class="btn-sm" on:click={() => fetchActivityReport(true)} disabled={loadingActivity}>Refresh</button>
                    </div>
                </div>
                {#if loadingActivity}
                    <div class="loading"><WabiSpinner size="lg" /></div>
                {:else if activityError}
                    <div class="empty-state">{activityError}</div>
                {:else if !activityReport}
                    <div class="empty-state">No activity report loaded.</div>
                {:else}
                    <div class="summary-grid">
                        <div class="summary-tile"><span>Total Active</span><strong>{activityReport.total_active_hours || 0}h</strong></div>
                        <div class="summary-tile"><span>Meaningful</span><strong>{activityReport.total_meaningful_hours || 0}h</strong></div>
                        <div class="summary-tile"><span>Efficiency</span><strong>{activityReport.average_efficiency || 0}%</strong></div>
                        <div class="summary-tile"><span>Users</span><strong>{(activityReport.users || []).length}</strong></div>
                    </div>
                    <div class="chart-list">
                        {#each activityReport.chart_rows || [] as row}
                            <div class="chart-row">
                                <div class="chart-label">{row.label}</div>
                                <div class="chart-track">
                                    <div class="chart-bar" style={`width: ${barWidth(row, activityReport.chart_rows || [])}`}></div>
                                </div>
                                <div class="chart-value">{row.meaningful_hours}h</div>
                                <div class="chart-eff">{row.efficiency_score}%</div>
                            </div>
                        {/each}
                    </div>
                    <div class="table-card compact">
                        <table>
                            <thead>
                                <tr><th>User</th><th>Role</th><th>Active</th><th>Meaningful</th><th>Searches</th><th>Creates</th><th>Updates</th><th>Exports</th></tr>
                            </thead>
                            <tbody>
                                {#each activityReport.users || [] as user}
                                    <tr>
                                        <td class="bold">{user.employee_name}</td>
                                        <td><span class="badge role">{user.license_role || "user"}</span></td>
                                        <td>{user.active_hours}h</td>
                                        <td>{user.meaningful_hours}h</td>
                                        <td>{user.search_count}</td>
                                        <td>{user.create_count}</td>
                                        <td>{user.update_count}</td>
                                        <td>{user.export_count}</td>
                                    </tr>
                                {/each}
                            </tbody>
                        </table>
                    </div>
                {/if}
            </div>
        {:else if activeTab === "conflicts"}
            <div class="activity-panel" in:fade>
                <div class="activity-toolbar">
                    <div>
                        <h2>Opportunity Conflicts</h2>
                        <p>Concurrent opportunity edits flagged for admin decision.</p>
                    </div>
                    <div class="activity-controls">
                        <select bind:value={conflictStatusFilter} on:change={() => fetchOpportunityConflicts(true)}>
                            <option value="pending">Pending</option>
                            <option value="applied">Applied</option>
                            <option value="rejected">Rejected</option>
                            <option value="all">All</option>
                        </select>
                        <button class="btn-sm" on:click={() => fetchOpportunityConflicts(true)} disabled={loadingConflicts}>Refresh</button>
                    </div>
                </div>
                {#if loadingConflicts}
                    <div class="loading"><WabiSpinner size="lg" /></div>
                {:else if opportunityConflicts.length === 0}
                    <div class="empty-state">No opportunity edit conflicts match this filter.</div>
                {:else}
                    <div class="table-card compact">
                        <table>
                            <thead>
                                <tr><th>Opportunity</th><th>By</th><th>Operation</th><th>Versions</th><th>Proposed</th><th>Status</th><th class="right">Decision</th></tr>
                            </thead>
                            <tbody>
                                {#each opportunityConflicts as conflict}
                                    <tr>
                                        <td class="bold">{conflict.folder_number || conflict.opportunity_id}</td>
                                        <td>{conflict.attempted_by || "User"} <span class="dim">({conflict.attempted_role || "role"})</span></td>
                                        <td>{conflict.operation}</td>
                                        <td>v{conflict.expected_version} -> v{conflict.current_version}</td>
                                        <td class="dim">{conflictSummary(conflict.proposed_changes_json)}</td>
                                        <td><span class="badge neutral">{conflict.status}</span></td>
                                        <td class="right">
                                            {#if conflict.status === "pending"}
                                                <button class="btn-sm" on:click={() => resolveOpportunityConflict(conflict, "apply")}>Apply</button>
                                                <button class="btn-sm" on:click={() => resolveOpportunityConflict(conflict, "reject")}>Reject</button>
                                            {:else}
                                                <span class="dim">{conflict.resolution_action || "closed"}</span>
                                            {/if}
                                        </td>
                                    </tr>
                                {/each}
                            </tbody>
                        </table>
                    </div>
                {/if}
            </div>
        {/if}
    </main>

    {#if showUserModal}
        <div
            class="modal-backdrop"
            role="presentation"
            in:fade={{ duration: 120 }}
        >
            <div
                class="modal"
                role="dialog"
                aria-modal="true"
                aria-labelledby="user-modal-title"
                tabindex="-1"
            >
                <form on:submit|preventDefault={saveUser}>
                    <div class="modal-header">
                        <div>
                            <h2 id="user-modal-title">
                                {editingUser ? "Edit User" : "Add User"}
                            </h2>
                            <p>
                                {editingUser
                                    ? "Update account access and role."
                                    : "Create a login for a team member."}
                            </p>
                        </div>
                        <button
                            type="button"
                            class="icon-btn"
                            aria-label="Close user dialog"
                            on:click={closeUserModal}>×</button
                        >
                    </div>

                    <div class="form-grid">
                        <label>
                            <span>Username</span>
                            <input
                                bind:value={userForm.username}
                                disabled={Boolean(editingUser)}
                                required={!editingUser}
                            />
                        </label>
                        <label>
                            <span>Email</span>
                            <input
                                type="email"
                                bind:value={userForm.email}
                                required
                            />
                        </label>
                        {#if !editingUser}
                            <label>
                                <span>Temporary password</span>
                                <input
                                    type="password"
                                    bind:value={userForm.password}
                                    required
                                />
                            </label>
                        {/if}
                        <label>
                            <span>Full name</span>
                            <input bind:value={userForm.full_name} required />
                        </label>
                        <label>
                            <span>Department</span>
                            <input bind:value={userForm.department} />
                        </label>
                        <label>
                            <span>Job title</span>
                            <input bind:value={userForm.job_title} />
                        </label>
                        <label>
                            <span>Role</span>
                            <select bind:value={userForm.role_id} required>
                                <option value="">Select role</option>
                                {#each roles as role}
                                    <option value={roleId(role)}
                                        >{role.display_name || role.name}</option
                                    >
                                {/each}
                            </select>
                        </label>
                        {#if editingUser}
                            <label class="checkbox-row">
                                <input
                                    type="checkbox"
                                    bind:checked={userForm.is_active}
                                />
                                <span>Active account</span>
                            </label>
                        {/if}
                    </div>

                    <div class="modal-actions">
                        <button
                            type="button"
                            class="btn-sm"
                            on:click={closeUserModal}>Cancel</button
                        >
                        <button
                            type="submit"
                            class="btn-primary"
                            disabled={savingUser}
                            >{savingUser ? "Saving..." : "Save User"}</button
                        >
                    </div>
                </form>
            </div>
        </div>
    {/if}
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
        margin-bottom: var(--space-6);
    }
    h1 {
        font-size: var(--text-5xl);
        font-weight: var(--font-weight-light);
        margin: 0;
        letter-spacing: -0.02em;
    }
    .subtitle {
        color: var(--ink-faint);
        margin-top: var(--space-2);
    }
    .btn-primary {
        background: var(--ink);
        color: var(--paper);
        border: none;
        padding: 10px 20px;
        border-radius: var(--radius-pill);
        cursor: pointer;
    }
    .btn-primary:disabled {
        opacity: 0.6;
        cursor: wait;
    }

    .tabs {
        display: flex;
        gap: 24px;
        border-bottom: 1px solid var(--border-subtle);
        margin-bottom: 24px;
    }
    .tab {
        background: transparent;
        border: none;
        padding: 12px 0;
        font-size: 14px;
        color: var(--ink-light);
        cursor: pointer;
        position: relative;
    }
    .tab.active {
        color: var(--ink);
        font-weight: 500;
    }
    .tab.active::after {
        content: "";
        position: absolute;
        bottom: -1px;
        left: 0;
        width: 100%;
        height: 2px;
        background: var(--ink);
    }

    .content-area {
        flex: 1;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 16px;
    }
    .loading {
        display: flex;
        justify-content: center;
        padding: 40px;
    }

    .toolbar {
        display: flex;
        margin-bottom: 16px;
    }
    .search-input {
        width: 300px;
        padding: 10px;
        border: 1px solid var(--border-medium);
        border-radius: 8px;
        font-size: 13px;
    }

    .table-card {
        background: var(--paper-subtle);
        border-radius: 12px;
        border: 1px solid var(--border-subtle);
        overflow: hidden;
    }
    table {
        width: 100%;
        border-collapse: collapse;
        font-size: 13px;
    }
    th {
        text-align: left;
        padding: 12px 16px;
        border-bottom: 1px solid var(--border-medium);
        font-size: 11px;
        text-transform: uppercase;
        color: var(--ink-light);
        font-weight: 500;
    }
    td {
        padding: 12px 16px;
        border-bottom: 1px solid var(--border-subtle);
        vertical-align: middle;
    }
    .right {
        text-align: right;
    }

    .user-cell {
        display: flex;
        align-items: center;
        gap: 12px;
    }
    .avatar {
        width: 32px;
        height: 32px;
        background: var(--ink);
        color: var(--paper);
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 600;
        font-size: 14px;
    }
    .info .name {
        font-weight: 500;
    }
    .info .sub {
        font-size: 11px;
        color: var(--ink-light);
    }

    .badge {
        padding: 2px 8px;
        border-radius: 4px;
        border: 1px solid var(--border-medium);
        font-size: 10px;
        text-transform: uppercase;
    }
    .badge.role {
        background: var(--paper);
    }
    .badge.neutral {
        background: rgba(0, 0, 0, 0.05);
        border: none;
    }

    .dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        display: inline-block;
        margin-right: 4px;
    }
    .dot.green {
        background: #22c55e;
    }
    .dot.red {
        background: #ef4444;
    }

    .btn-sm {
        padding: 4px 12px;
        border: 1px solid var(--border-medium);
        background: var(--paper);
        border-radius: 4px;
        cursor: pointer;
        font-size: 11px;
    }

    .grid-roles {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
        gap: 16px;
    }
    .role-card {
        background: var(--paper-subtle);
        padding: 20px;
        border-radius: 12px;
        border: 1px solid var(--border-subtle);
    }
    .role-card h3 {
        margin: 0 0 8px;
        font-size: 16px;
        font-weight: 500;
    }
    .role-card p {
        font-size: 12px;
        color: var(--ink-light);
        margin-bottom: 16px;
    }
    .btn-txt {
        background: transparent;
        border: none;
        color: var(--ink);
        font-size: 12px;
        cursor: pointer;
        padding: 0;
        font-weight: 500;
    }
    .permission-list {
        margin: 14px 0 0;
        padding: 12px 16px;
        border-radius: 8px;
        background: var(--paper);
        border: 1px solid var(--border-subtle);
        font-size: 12px;
        color: var(--ink);
        display: grid;
        gap: 6px;
    }

    .activity-panel {
        display: flex;
        flex-direction: column;
        gap: 16px;
    }
    .activity-toolbar {
        display: flex;
        align-items: flex-end;
        justify-content: space-between;
        gap: 16px;
        padding-bottom: 4px;
    }
    .activity-toolbar h2 {
        margin: 0 0 4px;
        font-size: 22px;
        font-weight: 500;
    }
    .activity-toolbar p {
        margin: 0;
        color: var(--ink-light);
        font-size: 12px;
    }
    .activity-controls {
        display: flex;
        gap: 8px;
        align-items: center;
    }
    .activity-controls input,
    .activity-controls select {
        border: 1px solid var(--border-medium);
        border-radius: 6px;
        padding: 7px 9px;
        background: var(--paper);
        color: var(--ink);
        font-size: 12px;
    }
    .summary-grid {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
        gap: 12px;
    }
    .summary-tile {
        border: 1px solid var(--border-subtle);
        background: var(--paper-subtle);
        border-radius: 8px;
        padding: 14px;
    }
    .summary-tile span {
        display: block;
        font-size: 11px;
        color: var(--ink-light);
        text-transform: uppercase;
        margin-bottom: 8px;
    }
    .summary-tile strong {
        font-size: 24px;
        font-weight: 500;
    }
    .chart-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
        border: 1px solid var(--border-subtle);
        background: var(--paper-subtle);
        border-radius: 8px;
        padding: 14px;
    }
    .chart-row {
        display: grid;
        grid-template-columns: minmax(120px, 180px) 1fr 56px 56px;
        gap: 12px;
        align-items: center;
        font-size: 12px;
    }
    .chart-label {
        font-weight: 500;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .chart-track {
        height: 10px;
        border-radius: 999px;
        background: rgba(15, 23, 42, 0.08);
        overflow: hidden;
    }
    .chart-bar {
        height: 100%;
        border-radius: 999px;
        background: #0f766e;
    }
    .chart-value,
    .chart-eff {
        color: var(--ink-light);
        text-align: right;
        font-variant-numeric: tabular-nums;
    }
    .compact table {
        font-size: 12px;
    }
    .empty-state {
        border: 1px solid var(--border-subtle);
        background: var(--paper-subtle);
        border-radius: 8px;
        padding: 24px;
        color: var(--ink-light);
        text-align: center;
    }

    .modal-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(17, 24, 39, 0.26);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 40;
        padding: 24px;
    }
    .modal {
        width: min(720px, 100%);
        background: var(--paper);
        border: 1px solid var(--border-medium);
        border-radius: 12px;
        box-shadow: 0 24px 70px rgba(0, 0, 0, 0.18);
        padding: 24px;
    }
    .modal-header {
        display: flex;
        justify-content: space-between;
        gap: 16px;
        margin-bottom: 20px;
    }
    .modal-header h2 {
        margin: 0;
        font-size: 22px;
        font-weight: 500;
    }
    .modal-header p {
        margin: 4px 0 0;
        color: var(--ink-light);
        font-size: 13px;
    }
    .icon-btn {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        border: 1px solid var(--border-medium);
        background: var(--paper-subtle);
        cursor: pointer;
        font-size: 20px;
        line-height: 1;
    }
    .form-grid {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 14px;
    }
    label {
        display: grid;
        gap: 6px;
        color: var(--ink);
        font-size: 12px;
        font-weight: 500;
    }
    label span {
        color: var(--ink-light);
    }
    label input,
    label select {
        border: 1px solid var(--border-medium);
        border-radius: 8px;
        padding: 10px 12px;
        font-size: 13px;
        background: var(--paper);
        color: var(--ink);
    }
    label input:disabled {
        background: var(--paper-subtle);
        color: var(--ink-light);
    }
    .checkbox-row {
        display: flex;
        align-items: center;
        gap: 8px;
        align-self: end;
    }
    .checkbox-row input {
        width: 16px;
        height: 16px;
    }
    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
        margin-top: 22px;
    }

    .bold {
        font-weight: 500;
    }
    .dim {
        color: var(--ink-light);
    }
    .mono {
        font-family: var(--font-mono);
        font-size: 12px;
        color: var(--ink-light);
    }

    @media (max-width: 720px) {
        .form-grid {
            grid-template-columns: 1fr;
        }
        .modal-backdrop {
            align-items: flex-start;
            padding: 16px;
            overflow-y: auto;
        }
    }
</style>
