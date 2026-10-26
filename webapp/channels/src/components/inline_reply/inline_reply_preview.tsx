// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {Client4} from 'mattermost-redux/client';
import {Posts} from 'mattermost-redux/constants';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import {getPermalinkURL} from 'selectors/urls';

import {getQuotedMessageText} from 'utils/inline_reply';
import type {InlineReplyReference} from 'utils/inline_reply';

import type {GlobalState} from 'types/store';

import './inline_reply_preview.scss';

type Props = {
    reference: InlineReplyReference;
    channelId: string;
    onHeightChange?: () => void;
    inComposer?: boolean;
};

const InlineReplyPreview = ({reference, channelId, onHeightChange, inComposer = false}: Props) => {
    const cached = useSelector((state: GlobalState) => getPost(state, reference.post_id));
    const [fetched, setFetched] = useState<Post | null>(null);
    const [unavailable, setUnavailable] = useState(false);
    const teamId = useSelector(getCurrentTeamId);
    const url = useSelector((state: GlobalState) => getPermalinkURL(state, teamId, reference.post_id));

    useEffect(() => {
        setFetched(null);
        setUnavailable(false);
        if (cached) {
            return;
        }
        let active = true;
        // A quoted post can be outside the loaded message window. Fetch it
        // through the authenticated API, never from props or a copied snapshot.
        Client4.getPost(reference.post_id).then((post) => {
            if (active) {
                setFetched(post);
            }
        }).catch(() => {
            if (active) {
                setUnavailable(true);
            }
        });
        return () => {
            active = false;
        };
    }, [reference.post_id, cached?.id]);

    const source = cached || fetched;
    const visible = source?.channel_id === channelId && !source.delete_at && source.state !== Posts.POST_DELETED && (source.type === '' || source.type === 'me');
    const author = useSelector((state: GlobalState) => getUser(state, visible && source ? source.user_id : ''));
    const text = visible && source ? getQuotedMessageText(source.message, reference) : '';
    const authorName = author?.username || 'Message';

    useEffect(() => {
        onHeightChange?.();
    }, [source?.id, source?.message, unavailable, onHeightChange]);

    const content = (
        <>
            <strong className='r2d-inline-reply__author'>{visible ? authorName : 'Original message'}</strong>
            <span className='r2d-inline-reply__text'>{visible ? (text || 'Attachment') : (unavailable || source ? 'Message unavailable' : 'Loading message…')}</span>
        </>
    );

    if (!visible || inComposer) {
        return (
            <div
                className='r2d-inline-reply'
                data-testid='r2d-inline-reply-preview'
            >
                {content}
            </div>
        );
    }

    return (
        <a
            className='r2d-inline-reply r2d-inline-reply--link'
            data-testid='r2d-inline-reply-preview'
            href={url}
            aria-label={'Jump to quoted message from ' + authorName}
        >
            {content}
        </a>
    );
};

export default InlineReplyPreview;
