// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';
import type {GlobalState} from '@mattermost/types/store';

import {ChannelTypes} from 'mattermost-redux/action_types';
import {Permissions} from 'mattermost-redux/constants';
import channelsReducer from 'mattermost-redux/reducers/entities/channels';
import {canManageChannelPostingAndMembership, haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import initialState from 'mattermost-redux/store/initial_state';

import {TestHelper} from 'utils/test_helper';

function makeState(policy: Channel['posting_policy'] = 'admins', userRoles = 'system_user', channelAdmin = false, member = true, createPost = true): GlobalState {
    const user = TestHelper.getUserMock({id: 'user', roles: userRoles});
    const channel = TestHelper.getChannelMock({id: 'channel', type: 'O', team_id: 'team', posting_policy: policy, posting_user_ids: ['user']});
    return {
        ...initialState,
        entities: {
            ...initialState.entities,
            users: {...initialState.entities.users, currentUserId: user.id, profiles: {[user.id]: user}},
            teams: {...initialState.entities.teams, myMembers: {}},
            roles: {...initialState.entities.roles, roles: {system_user: TestHelper.getRoleMock({name: 'system_user', permissions: createPost ? [Permissions.CREATE_POST, Permissions.CREATE_POST_PUBLIC] : []})}},
            channels: {
                ...initialState.entities.channels,
                channels: {[channel.id]: channel},
                myMembers: member ? {[channel.id]: TestHelper.getChannelMembershipMock({user_id: user.id, channel_id: channel.id, scheme_admin: channelAdmin, roles: channelAdmin ? 'channel_admin' : 'channel_user'})} : {},
            },
        },
    };
}

describe('channel posting policy', () => {
    test.each<Channel['posting_policy']>([undefined, 'all'])('preserves posting permissions for policy %s', (policy) => {
        const state = makeState();
        state.entities.channels.channels.channel.posting_policy = policy;
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(true);
    });

    test.each([Permissions.CREATE_POST, Permissions.CREATE_POST_PUBLIC])('restricts %s even when system roles grant it', (permission) => {
        expect(haveIChannelPermission(makeState(), 'team', 'channel', permission)).toBe(false);
    });

    test.each(['system_user', 'system_guest'])('selected %s must be a member and retain existing posting permission', (role) => {
        const state = makeState('selected', role);
        state.entities.users.profiles.user.roles = `${role} system_user`;
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(true);
        state.entities.channels.myMembers = {};
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
        expect(haveIChannelPermission(makeState('selected', role, false, true, false), 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
    });

    test('unselected members cannot post', () => {
        const state = makeState('selected');
        state.entities.channels.channels.channel.posting_user_ids = [];
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
    });

    test.each([
        ['system_user system_admin', false],
        ['system_user', true],
    ])('admins bypass the additional rule: %s, channel admin %s', (roles, channelAdmin) => {
        const state = makeState('admins', roles, channelAdmin);
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(true);
        expect(canManageChannelPostingAndMembership(state, 'channel')).toBe(true);
        expect(haveIChannelPermission(makeState('admins', roles, channelAdmin, true, false), 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
    });

    test('team admins cannot manage restrictions or bypass them', () => {
        const state = makeState('admins');
        state.entities.teams.myMembers.team = TestHelper.getTeamMembershipMock({team_id: 'team', user_id: 'user', roles: 'team_admin', scheme_admin: true});
        expect(canManageChannelPostingAndMembership(state, 'channel')).toBe(false);
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
    });

    test.each<Channel['type']>(['D', 'G'])('ignores posting policy in channel type %s', (type) => {
        const state = makeState('admins', 'system_user system_admin');
        state.entities.channels.channels.channel.type = type;
        expect(haveIChannelPermission(state, '', 'channel', Permissions.CREATE_POST)).toBe(true);
        expect(canManageChannelPostingAndMembership(state, 'channel')).toBe(false);
    });

    test('received channel updates immediately change posting permission and preserve mandatory membership', () => {
        const state = makeState('all');
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(true);
        state.entities.channels = channelsReducer(state.entities.channels, {
            type: ChannelTypes.RECEIVED_CHANNEL,
            data: {...state.entities.channels.channels.channel, posting_policy: 'admins', mandatory: true},
        });
        expect(haveIChannelPermission(state, 'team', 'channel', Permissions.CREATE_POST)).toBe(false);
        expect(state.entities.channels.channels.channel.mandatory).toBe(true);
    });
});
