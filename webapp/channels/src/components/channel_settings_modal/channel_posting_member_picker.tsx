// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import {Button} from '@mattermost/compass-ui/components/button';
import {Checkbox} from '@mattermost/compass-ui/components/checkbox';
import {SearchInput} from '@mattermost/compass-ui/components/search-input';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';

const PAGE_SIZE = 50;
const MAX_SELECTED_MEMBERS = 1000;
const MAX_SEARCH_RESULTS = 1000;

type Props = {
    channelId: string;
    selectedIds: string[];
    onChange: (ids: string[]) => void;
    disabled: boolean;
};

export default function ChannelPostingMemberPicker({channelId, selectedIds, onChange, disabled}: Props) {
    const {formatMessage} = useIntl();
    const [term, setTerm] = useState('');
    const [page, setPage] = useState(0);
    const [users, setUsers] = useState<UserProfile[]>([]);
    const [selectedProfiles, setSelectedProfiles] = useState<Record<string, UserProfile>>({});
    const [loading, setLoading] = useState(true);
    const [hasMore, setHasMore] = useState(false);
    const [error, setError] = useState('');
    const [retry, setRetry] = useState(0);
    const requestedProfileIds = useRef(new Set<string>());

    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        setError('');
        const timeout = setTimeout(async () => {
            try {
                // Search supports a limit instead of a page. Request one extra
                // result so Load more only appears when another result exists.
                const results = term.trim() ? await Client4.searchUsers(term.trim(), {in_channel_id: channelId, allow_inactive: false, limit: Math.min(((page + 1) * PAGE_SIZE) + 1, MAX_SEARCH_RESULTS)}) : await Client4.getProfilesInChannel(channelId, page, PAGE_SIZE, '', {active: true});
                if (cancelled) {
                    return;
                }
                setUsers((previous) => (term.trim() ? results.slice(0, (page + 1) * PAGE_SIZE) : Array.from(new Map([...(page ? previous : []), ...results].map((user) => [user.id, user])).values())));
                setHasMore(term.trim() ? results.length > ((page + 1) * PAGE_SIZE) : results.length === PAGE_SIZE);
                results.forEach((user) => requestedProfileIds.current.add(user.id));
                setSelectedProfiles((previous) => ({...previous, ...Object.fromEntries(results.map((user) => [user.id, user]))}));
            } catch (err) {
                if (!cancelled) {
                    setError(err.message);
                }
            } finally {
                if (!cancelled) {
                    setLoading(false);
                }
            }
        }, term ? 200 : 0);
        return () => {
            cancelled = true;
            clearTimeout(timeout);
        };
    }, [channelId, term, page, retry]);

    useEffect(() => {
        const requestedIds = requestedProfileIds.current;
        const missingIds = selectedIds.filter((id) => !requestedIds.has(id));
        if (!missingIds.length) {
            return undefined;
        }
        let cancelled = false;
        let completed = false;
        missingIds.forEach((id) => requestedIds.add(id));
        Client4.getProfilesByIds(missingIds).then((profiles) => {
            completed = true;
            if (!cancelled) {
                setSelectedProfiles((previous) => ({...previous, ...Object.fromEntries(profiles.map((user) => [user.id, user]))}));
            }
        }).catch((err) => {
            completed = true;
            if (!cancelled) {
                setError(err.message);
            }
        });
        return () => {
            cancelled = true;
            if (!completed) {
                missingIds.forEach((id) => requestedIds.delete(id));
            }
        };
    }, [selectedIds, retry]);

    const search = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        setTerm(event.target.value);
        setPage(0);
        setUsers([]);
    }, []);
    const clearSearch = useCallback(() => {
        setTerm('');
        setPage(0);
        setUsers([]);
    }, []);
    const toggleMember = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        const id = event.target.value;
        onChange(event.target.checked ? [...selectedIds, id] : selectedIds.filter((selected) => selected !== id));
    }, [onChange, selectedIds]);
    const loadMore = useCallback(() => setPage((previous) => previous + 1), []);
    const retryLoad = useCallback(() => {
        requestedProfileIds.current.clear();
        setRetry((previous) => previous + 1);
    }, []);

    return (
        <div className='ChannelPostingMemberPicker'>
            <SearchInput
                label={formatMessage({id: 'channel_settings.posting.search', defaultMessage: 'Search members'})}
                clearLabel={formatMessage({id: 'channel_settings.posting.clear_search', defaultMessage: 'Clear search'})}
                value={term}
                onChange={search}
                onClear={clearSearch}
                disabled={disabled}
            />
            <div className='ChannelPostingMemberPicker__members'>
                {[...selectedIds, ...users.filter((user) => !selectedIds.includes(user.id)).map((user) => user.id)].map((id) => {
                    const selected = selectedIds.includes(id);
                    return (
                        <Checkbox
                            key={id}
                            value={id}
                            checked={selected}
                            onChange={toggleMember}
                            disabled={disabled || (!selected && selectedIds.length >= MAX_SELECTED_MEMBERS)}
                        >
                            {selectedProfiles[id] ? selectedProfiles[id].username : (
                                <FormattedMessage
                                    id='channel_settings.posting.unavailable_member'
                                    defaultMessage='Unavailable member'
                                />
                            )}
                        </Checkbox>
                    );
                })}
            </div>
            {selectedIds.length >= MAX_SELECTED_MEMBERS && (
                <p>
                    <FormattedMessage
                        id='channel_settings.posting.member_limit'
                        defaultMessage='You can select up to {limit} members.'
                        values={{limit: MAX_SELECTED_MEMBERS}}
                    />
                </p>
            )}
            {loading && (
                <p role='status'>
                    <FormattedMessage
                        id='channel_settings.posting.loading'
                        defaultMessage='Loading members…'
                    />
                </p>
            )}
            {!loading && !error && users.length === 0 && (
                <p>
                    <FormattedMessage
                        id='channel_settings.posting.no_members'
                        defaultMessage='No members found.'
                    />
                </p>
            )}
            {error && (
                <>
                    <p role='alert'>{error}</p>
                    <Button
                        emphasis='tertiary'
                        onClick={retryLoad}
                        disabled={disabled || loading}
                    >
                        <FormattedMessage
                            id='channel_settings.posting.retry'
                            defaultMessage='Retry'
                        />
                    </Button>
                </>
            )}
            {hasMore && !error && (
                <Button
                    emphasis='tertiary'
                    onClick={loadMore}
                    disabled={disabled || loading}
                >
                    <FormattedMessage
                        id='channel_settings.posting.load_more'
                        defaultMessage='Load more members'
                    />
                </Button>
            )}
        </div>
    );
}
