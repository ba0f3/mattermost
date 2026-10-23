// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import ChannelSettingsPostingTab from './channel_settings_posting_tab';

const channel = TestHelper.getChannelMock({id: 'channel', type: 'O'});
const alice = TestHelper.getUserMock({id: 'alice', username: 'alice'});
const bob = TestHelper.getUserMock({id: 'bob', username: 'bob'});

function renderTab(overrides: Partial<Channel> = {}) {
    const dirty = jest.fn();
    function Tab() {
        const currentChannel = useSelector((state: GlobalState) => getChannel(state, channel.id));
        if (!currentChannel) {
            throw new Error('Test channel was not loaded');
        }
        return (
            <ChannelSettingsPostingTab
                channel={currentChannel}
                setAreThereUnsavedChanges={dirty}
            />
        );
    }
    renderWithContext(<Tab/>, {entities: {channels: {channels: {[channel.id]: {...channel, ...overrides}}}}});
    return dirty;
}

describe('ChannelSettingsPostingTab', () => {
    beforeEach(() => {
        jest.spyOn(Client4, 'getProfilesInChannel').mockResolvedValue([alice]);
        jest.spyOn(Client4, 'getProfilesByIds').mockResolvedValue([alice]);
        jest.spyOn(Client4, 'searchUsers').mockResolvedValue([bob]);
        jest.spyOn(Client4, 'patchChannel').mockImplementation(async (id, patch) => ({...channel, ...patch, total_msg_count: 0, total_msg_count_root: 0}));
    });

    test('changes posting policy and mandatory membership together, then clears dirty state', async () => {
        const dirty = renderTab();
        expect(screen.getByRole('radio', {name: 'Everyone'})).toBeChecked();
        expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('radio', {name: 'Admins only'}));
        await userEvent.click(screen.getByRole('checkbox', {name: 'Require all team members'}));
        expect(dirty).toHaveBeenLastCalledWith(true);
        expect(screen.getByText(/including guests/)).toBeVisible();
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(Client4.patchChannel).toHaveBeenCalledWith(channel.id, {posting_policy: 'admins', posting_user_ids: [], mandatory: true}));
        expect(await screen.findByText('Settings saved')).toBeVisible();
        expect(dirty).toHaveBeenLastCalledWith(false);
    });

    test('undo restores saved values without a request', async () => {
        const dirty = renderTab();
        await userEvent.click(screen.getByRole('radio', {name: 'Admins only'}));
        await userEvent.click(screen.getByRole('button', {name: 'Undo'}));
        expect(screen.getByRole('radio', {name: 'Everyone'})).toBeChecked();
        expect(dirty).toHaveBeenLastCalledWith(false);
        expect(Client4.patchChannel).not.toHaveBeenCalled();
    });

    test('keeps edits after a failed save and permits retry', async () => {
        jest.spyOn(Client4, 'patchChannel').mockRejectedValueOnce(new Error('Unable to save'));
        const dirty = renderTab();
        await userEvent.click(screen.getByRole('radio', {name: 'Admins only'}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Unable to save');
        expect(dirty).toHaveBeenLastCalledWith(true);
        expect(screen.getByRole('radio', {name: 'Admins only'})).toBeChecked();
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByText('Settings saved')).toBeVisible();
    });

    test.each([{group_constrained: true}, {policy_enforced: true}])('disables conflicting mandatory membership for %s', (overrides) => {
        renderTab(overrides);
        expect(screen.getByRole('checkbox', {name: 'Require all team members'})).toBeDisabled();
        expect(screen.getByRole('radio', {name: 'Admins only'})).toBeEnabled();
    });

    test('searches channel members and retains selected members outside the search results', async () => {
        renderTab();
        await userEvent.click(screen.getByRole('radio', {name: 'Admins and selected members'}));
        await userEvent.click(await screen.findByRole('checkbox', {name: 'alice'}));
        await userEvent.type(screen.getByRole('searchbox'), 'bob');
        await userEvent.click(await screen.findByRole('checkbox', {name: 'bob'}));
        expect(screen.getByRole('checkbox', {name: 'alice'})).toBeChecked();
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(Client4.patchChannel).toHaveBeenCalledWith(channel.id, {posting_policy: 'selected', posting_user_ids: ['alice', 'bob']}));
        expect(Client4.searchUsers).toHaveBeenLastCalledWith('bob', {in_channel_id: channel.id, allow_inactive: false, limit: 51});
    });

    test('preserves keyboard focus when selecting and deselecting a member moves the row', async () => {
        jest.spyOn(Client4, 'getProfilesInChannel').mockResolvedValue([alice, bob]);
        renderTab({posting_policy: 'selected'});
        const member = await screen.findByRole('checkbox', {name: 'bob'});
        await userEvent.click(screen.getByRole('searchbox'));
        await userEvent.tab();
        await userEvent.tab();
        expect(member).toHaveFocus();
        await userEvent.keyboard('[Space]');
        expect(member).toBeChecked();
        expect(member).toHaveFocus();
        await userEvent.keyboard('[Space]');
        expect(member).not.toBeChecked();
        expect(member).toHaveFocus();
    });

    test('keeps unavailable saved members removable without retrying profile requests', async () => {
        jest.spyOn(Client4, 'getProfilesByIds').mockResolvedValue([]);
        renderTab({posting_policy: 'selected', posting_user_ids: ['deleted']});
        expect(await screen.findByRole('checkbox', {name: 'Unavailable member'})).toBeChecked();
        await screen.findByRole('checkbox', {name: 'alice'});
        await userEvent.click(screen.getByRole('checkbox', {name: 'Unavailable member'}));
        expect(screen.queryByRole('checkbox', {name: 'Unavailable member'})).not.toBeInTheDocument();
        expect(Client4.getProfilesByIds).toHaveBeenCalledTimes(1);
    });

    test('loads another page of channel members', async () => {
        jest.spyOn(Client4, 'getProfilesInChannel').mockResolvedValueOnce(Array.from({length: 50}, (_, index) => TestHelper.getUserMock({id: `user${index}`, username: `user${index}`}))).mockResolvedValueOnce([bob]);
        renderTab({posting_policy: 'selected'});
        await userEvent.click(await screen.findByRole('button', {name: 'Load more members'}));
        expect(await screen.findByRole('checkbox', {name: 'bob'})).toBeVisible();
        expect(Client4.getProfilesInChannel).toHaveBeenLastCalledWith(channel.id, 1, 50, '', {active: true});
        expect(screen.queryByRole('button', {name: 'Load more members'})).not.toBeInTheDocument();
    });
});
