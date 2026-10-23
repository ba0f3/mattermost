// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GroupMembership, GroupPermissions} from '@mattermost/types/groups';
import type {Role} from '@mattermost/types/roles';
import type {GlobalState} from '@mattermost/types/store';

import {General, Permissions} from 'mattermost-redux/constants';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getCurrentChannelId, getCurrentUserId} from 'mattermost-redux/selectors/entities/common';
import type {PermissionsOptions} from 'mattermost-redux/selectors/entities/roles_helpers';
import {
    getMySystemPermissions as getMySystemPermissionsInternal,
    getMySystemRoles as getMySystemRolesInternal,
    getPermissionsForRoles,
    getRoles as getRolesInternal,
    haveISystemPermission as haveISystemPermissionInternal,
} from 'mattermost-redux/selectors/entities/roles_helpers';
import {getTeamMemberships, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {isSyncableSource} from 'mattermost-redux/utils/group_utils';

// Re-define these types to ensure that these are typed correctly when mattermost-redux is published
export const getMySystemPermissions: (state: GlobalState) => Set<string> = getMySystemPermissionsInternal;
export const getMySystemRoles: (state: GlobalState) => Set<string> = getMySystemRolesInternal;
export const getRoles: (state: GlobalState) => Record<string, Role> = getRolesInternal;
export const haveISystemPermission: (state: GlobalState, options: PermissionsOptions) => boolean = haveISystemPermissionInternal;

export const getGroupMemberships: (state: GlobalState) => Record<string, GroupMembership> = createSelector(
    'getGroupMemberships',
    (state) => state.entities.groups.myGroups,
    getCurrentUserId,
    (myGroupIDs: string[], currentUserID: string) => {
        const groupMemberships: Record<string, GroupMembership> = {};
        myGroupIDs.forEach((groupID) => {
            groupMemberships[groupID] = {user_id: currentUserID, roles: General.CUSTOM_GROUP_USER_ROLE};
        });
        return groupMemberships;
    },
);

export const getMyGroupRoles: (state: GlobalState) => Record<string, Set<string>> = createSelector(
    'getMyGroupRoles',
    getGroupMemberships,
    (groupMemberships) => {
        const roles: Record<string, Set<string>> = {};
        if (groupMemberships) {
            for (const key in groupMemberships) {
                if (Object.hasOwn(groupMemberships, key) && groupMemberships[key].roles) {
                    roles[key] = new Set<string>(groupMemberships[key].roles.split(' '));
                }
            }
        }
        return roles;
    },
);

/**
 * Returns a map of permissions, keyed by group id, for all groups that are mentionable and not deleted.
 */
export const getGroupListPermissions: (state: GlobalState) => Record<string, GroupPermissions> = createSelector(
    'getGroupListPermissions',
    getMyGroupRoles,
    getRoles,
    getMySystemPermissions,
    (state) => state.entities.groups.groups,
    (myGroupRoles, roles, systemPermissions, allGroups) => {
        const groups = Object.entries(allGroups).filter((entry) => (entry[1].allow_reference)).map((entry) => entry[1]);

        const permissions = new Set<string>();
        groups.forEach((group) => {
            const roleNames = myGroupRoles[group.id!];
            if (roleNames) {
                for (const roleName of roleNames) {
                    if (roles[roleName]) {
                        for (const permission of roles[roleName].permissions) {
                            permissions.add(permission);
                        }
                    }
                }
            }
        });

        for (const permission of systemPermissions) {
            permissions.add(permission);
        }

        const groupPermissionsMap: Record<string, GroupPermissions> = {};
        groups.forEach((g) => {
            groupPermissionsMap[g.id] = {
                can_delete: permissions.has(Permissions.DELETE_CUSTOM_GROUP) && !isSyncableSource(g.source) && g.delete_at === 0,
                can_manage_members: permissions.has(Permissions.MANAGE_CUSTOM_GROUP_MEMBERS) && !isSyncableSource(g.source) && g.delete_at === 0,
                can_restore: permissions.has(Permissions.RESTORE_CUSTOM_GROUP) && !isSyncableSource(g.source) && g.delete_at !== 0,
            };
        });
        return groupPermissionsMap;
    },
);

export const getMyTeamRoles: (state: GlobalState) => Record<string, Set<string>> = createSelector(
    'getMyTeamRoles',
    getTeamMemberships,
    (teamsMemberships) => {
        const roles: Record<string, Set<string>> = {};
        if (teamsMemberships) {
            for (const key in teamsMemberships) {
                if (Object.hasOwn(teamsMemberships, key) && teamsMemberships[key].roles) {
                    roles[key] = new Set<string>(teamsMemberships[key].roles.split(' '));
                }
            }
        }
        return roles;
    },
);

export function getMyChannelRoles(state: GlobalState): Record<string, Set<string>> {
    return state.entities.channels.roles;
}

export const getRolesById: (state: GlobalState) => Record<string, Role> = createSelector(
    'getRolesById',
    getRoles,
    (rolesByName) => {
        const rolesById: Record<string, Role> = {};
        for (const role of Object.values(rolesByName)) {
            rolesById[role.id] = role;
        }
        return rolesById;
    },
);

export const getMyPermissionsByTeam = createSelector(
    'getMyPermissionsByTeam',
    getMyTeamRoles,
    getRoles,
    (myTeamRoles, allRoles) => {
        const permissionsByTeam: Record<string, Set<string>> = {};

        for (const [teamId, roles] of Object.entries(myTeamRoles)) {
            permissionsByTeam[teamId] = getPermissionsForRoles(allRoles, roles);
        }

        return permissionsByTeam;
    },
);

const getMyPermissionsByGroup = createSelector(
    'getMyPermissionsByGroup',
    getMyGroupRoles,
    getRoles,
    (myGroupRoles, allRoles) => {
        const permissionsByGroup: Record<string, Set<string>> = {};

        for (const [groupId, roles] of Object.entries(myGroupRoles)) {
            permissionsByGroup[groupId] = getPermissionsForRoles(allRoles, roles);
        }

        return permissionsByGroup;
    },
);

export const getMyPermissionsByChannel = createSelector(
    'getMyPermissionsByChannel',
    getMyChannelRoles,
    getRoles,
    (myChannelRoles, allRoles) => {
        const permissionsByChannel: Record<string, Set<string>> = {};

        for (const [channelId, roles] of Object.entries(myChannelRoles)) {
            permissionsByChannel[channelId] = getPermissionsForRoles(allRoles, roles);
        }

        return permissionsByChannel;
    },
);

export function haveITeamPermission(state: GlobalState, teamId: string | undefined, permission: string) {
    if (getMySystemPermissions(state).has(permission)) {
        return true;
    }
    if (!teamId) {
        return false;
    }

    return getMyPermissionsByTeam(state)[teamId]?.has(permission);
}

export const haveIGroupPermission: (state: GlobalState, groupID: string, permission: string) => boolean = createSelector(
    'haveIGroupPermission',
    getMySystemPermissions,
    getMyPermissionsByGroup,
    (state: GlobalState, groupID: string) => state.entities.groups.groups[groupID],
    (state: GlobalState, groupID: string, permission: string) => permission,
    (systemPermissions, permissionGroups, group, permission) => {
        if (permission === Permissions.RESTORE_CUSTOM_GROUP) {
            if ((!isSyncableSource(group.source) && group.delete_at !== 0) && (systemPermissions.has(permission) || (permissionGroups[group.id] && permissionGroups[group.id].has(permission)))) {
                return true;
            }
            return false;
        }

        if (isSyncableSource(group.source) || group.delete_at !== 0) {
            return false;
        }

        if (systemPermissions.has(permission)) {
            return true;
        }

        if (permissionGroups[group.id] && permissionGroups[group.id].has(permission)) {
            return true;
        }
        return false;
    },
);

export function haveIChannelPermission(state: GlobalState, teamId: string | undefined, channelId: string | undefined, permission: string): boolean {
    if (channelId && (permission === Permissions.CREATE_POST || permission === Permissions.CREATE_POST_PUBLIC) && !canPostWithChannelPostingPolicy(state, channelId)) {
        return false;
    }

    if (getMySystemPermissions(state).has(permission)) {
        return true;
    }

    if (teamId && getMyPermissionsByTeam(state)[teamId]?.has(permission)) {
        return true;
    }

    if (channelId && getMyPermissionsByChannel(state)[channelId]?.has(permission)) {
        return true;
    }

    return false;
}

/** Only system admins and channel admins can manage the additional channel restrictions. */
export function canManageChannelPostingAndMembership(state: GlobalState, channelId: string): boolean {
    const channel = state.entities.channels.channels[channelId];
    if (!channel || (channel.type !== General.OPEN_CHANNEL && channel.type !== General.PRIVATE_CHANNEL)) {
        return false;
    }

    const membership = state.entities.channels.myMembers?.[channelId];
    return getMySystemRoles(state).has(General.SYSTEM_ADMIN_ROLE) || Boolean(membership?.scheme_admin || membership?.roles?.split(' ').includes(General.CHANNEL_ADMIN_ROLE));
}

/** This is an additional restriction; callers must still check the create_post permission. */
export function canPostWithChannelPostingPolicy(state: GlobalState, channelId: string): boolean {
    const channel = state.entities.channels.channels[channelId];
    if (!channel || (channel.type !== General.OPEN_CHANNEL && channel.type !== General.PRIVATE_CHANNEL) || !channel.posting_policy || channel.posting_policy === 'all') {
        return true;
    }

    if (canManageChannelPostingAndMembership(state, channelId)) {
        return true;
    }

    return channel.posting_policy === 'selected' && Boolean(state.entities.channels.myMembers?.[channelId]) && Boolean(channel.posting_user_ids?.includes(getCurrentUserId(state)));
}

export function haveICurrentTeamPermission(state: GlobalState, permission: string): boolean {
    return haveITeamPermission(state, getCurrentTeamId(state), permission);
}

export function haveICurrentChannelPermission(state: GlobalState, permission: string): boolean {
    return haveIChannelPermission(state, getCurrentTeamId(state), getCurrentChannelId(state), permission);
}
