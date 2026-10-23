// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {FormattedMessage} from 'react-intl';
import {useDispatch} from 'react-redux';

import {Checkbox} from '@mattermost/compass-ui/components/checkbox';
import {Radio} from '@mattermost/compass-ui/components/radio';
import type {Channel} from '@mattermost/types/channels';

import {patchChannel} from 'mattermost-redux/actions/channels';

import SaveChangesPanel, {type SaveChangesPanelState} from 'components/widgets/modals/components/save_changes_panel';

import ChannelPostingMemberPicker from './channel_posting_member_picker';

import './channel_settings_posting_tab.scss';

type Props = {
    channel: Channel;
    setAreThereUnsavedChanges: (unsaved: boolean) => void;
    showTabSwitchError?: boolean;
};

type PostingSettings = {
    posting_policy: NonNullable<Channel['posting_policy']>;
    posting_user_ids: string[];
    mandatory: boolean;
};

function settingsFromChannel(channel: Channel): PostingSettings {
    return {
        posting_policy: channel.posting_policy || 'all',
        posting_user_ids: channel.posting_user_ids || [],
        mandatory: Boolean(channel.mandatory),
    };
}

function sameSettings(a: PostingSettings, b: PostingSettings): boolean {
    return a.posting_policy === b.posting_policy && a.mandatory === b.mandatory &&
        (a.posting_policy !== 'selected' || (a.posting_user_ids.length === b.posting_user_ids.length && a.posting_user_ids.every((id) => b.posting_user_ids.includes(id))));
}

export default function ChannelSettingsPostingTab({channel, setAreThereUnsavedChanges, showTabSwitchError}: Props) {
    const dispatch = useDispatch();
    const [saved, setSaved] = useState(() => settingsFromChannel(channel));
    const [settings, setSettings] = useState(saved);
    const [panelState, setPanelState] = useState<SaveChangesPanelState>();
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const dirty = !sameSettings(settings, saved);
    const membershipManaged = channel.group_constrained || channel.policy_enforced;

    useEffect(() => {
        setAreThereUnsavedChanges(dirty);
    }, [dirty, setAreThereUnsavedChanges]);

    // Refresh a clean form when another administrator changes the channel.
    useEffect(() => {
        if (!dirty) {
            const next = settingsFromChannel(channel);
            setSaved(next);
            setSettings(next);
        }
    }, [channel, dirty]);

    const changePolicy = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        const policy = event.target.value;
        if (policy === 'all' || policy === 'admins' || policy === 'selected') {
            setSettings((previous) => ({...previous, posting_policy: policy}));
            setPanelState('editing');
            setError('');
        }
    }, []);

    const changeMembers = useCallback((ids: string[]) => {
        setSettings((previous) => ({...previous, posting_user_ids: ids}));
        setPanelState('editing');
        setError('');
    }, []);

    const changeMandatory = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        const mandatory = event.target.checked;
        setSettings((previous) => ({...previous, mandatory}));
        setPanelState('editing');
        setError('');
    }, []);

    const save = useCallback(async () => {
        setSaving(true);
        setError('');
        const patch: Partial<Channel> = {};
        if (settings.posting_policy !== saved.posting_policy || (settings.posting_policy === 'selected' && !sameSettings({...settings, mandatory: saved.mandatory}, saved))) {
            patch.posting_policy = settings.posting_policy;
            patch.posting_user_ids = settings.posting_policy === 'selected' ? settings.posting_user_ids : [];
        }
        if (!membershipManaged && settings.mandatory !== saved.mandatory) {
            patch.mandatory = settings.mandatory;
        }
        const result = await dispatch(patchChannel(channel.id, patch));
        setSaving(false);
        if (result.error) {
            setError(result.error.message);
            return;
        }
        const next = settingsFromChannel(result.data || {...channel, ...patch});
        setSaved(next);
        setSettings(next);
        setPanelState('saved');
    }, [channel, dispatch, membershipManaged, saved, settings]);

    const reset = useCallback(() => {
        const next = settingsFromChannel(channel);
        setSaved(next);
        setSettings(next);
        setError('');
        setPanelState(undefined);
    }, [channel]);

    const closePanel = useCallback(() => setPanelState(undefined), []);

    return (
        <div className='ChannelSettingsPostingTab'>
            <h3>
                <FormattedMessage
                    id='channel_settings.tab.posting_membership'
                    defaultMessage='Posting & Membership'
                />
            </h3>
            <fieldset disabled={saving}>
                <legend>
                    <FormattedMessage
                        id='channel_settings.posting.who_can_post'
                        defaultMessage='Who can post?'
                    />
                </legend>
                <p>
                    <FormattedMessage
                        id='channel_settings.posting.description'
                        defaultMessage='Choose who can send messages and replies. Existing posting permissions still apply. Admins include channel admins and system admins.'
                    />
                </p>
                <Radio
                    name='channel-posting-policy'
                    value='all'
                    checked={settings.posting_policy === 'all'}
                    onChange={changePolicy}
                >
                    <FormattedMessage
                        id='channel_settings.posting.all'
                        defaultMessage='Everyone'
                    />
                </Radio>
                <Radio
                    name='channel-posting-policy'
                    value='admins'
                    checked={settings.posting_policy === 'admins'}
                    onChange={changePolicy}
                >
                    <FormattedMessage
                        id='channel_settings.posting.admins'
                        defaultMessage='Admins only'
                    />
                </Radio>
                <Radio
                    name='channel-posting-policy'
                    value='selected'
                    checked={settings.posting_policy === 'selected'}
                    onChange={changePolicy}
                >
                    <FormattedMessage
                        id='channel_settings.posting.selected'
                        defaultMessage='Admins and selected members'
                    />
                </Radio>
                {settings.posting_policy === 'selected' && (
                    <ChannelPostingMemberPicker
                        channelId={channel.id}
                        selectedIds={settings.posting_user_ids}
                        onChange={changeMembers}
                        disabled={saving}
                    />
                )}
            </fieldset>
            <fieldset disabled={saving}>
                <legend>
                    <FormattedMessage
                        id='channel_settings.mandatory.title'
                        defaultMessage='Membership'
                    />
                </legend>
                <Checkbox
                    checked={settings.mandatory}
                    disabled={Boolean(membershipManaged)}
                    onChange={changeMandatory}
                    aria-describedby='channel-mandatory-description'
                >
                    <FormattedMessage
                        id='channel_settings.mandatory.label'
                        defaultMessage='Require all team members'
                    />
                </Checkbox>
                <p id='channel-mandatory-description'>
                    <FormattedMessage
                        id='channel_settings.mandatory.description'
                        defaultMessage='Add all existing and new team members, including guests, to this channel. They cannot leave or be removed while they belong to the team. Turning this off keeps current channel members.'
                    />
                </p>
                {membershipManaged && (
                    <p>
                        <FormattedMessage
                            id='channel_settings.mandatory.managed'
                            defaultMessage='Mandatory membership is unavailable while this channel is managed by a group or access policy.'
                        />
                    </p>
                )}
            </fieldset>
            {error && <p role='alert'>{error}</p>}
            {(dirty || panelState === 'saved') && (
                <SaveChangesPanel
                    handleSubmit={save}
                    handleCancel={reset}
                    handleClose={closePanel}
                    state={panelState}
                    saving={saving}
                    tabChangeError={showTabSwitchError}
                />
            )}
        </div>
    );
}
