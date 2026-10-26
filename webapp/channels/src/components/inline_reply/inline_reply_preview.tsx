// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

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

// Only animate a nearby, already-rendered post. Scrolling across an unloaded
// virtualized message range causes remeasurement jumps instead of smooth motion.
const MAX_SMOOTH_SCROLL_VIEWPORTS = 2;

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
            return undefined;
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

    let previewText = 'Loading message…';
    if (visible) {
        previewText = text || 'Attachment';
    } else if (unavailable || source) {
        previewText = 'Message unavailable';
    }

    const content = (
        <>
            <strong className='r2d-inline-reply__author'>{visible ? authorName : 'Original message'}</strong>
            <span className='r2d-inline-reply__text'>{previewText}</span>
        </>
    );

    const handleJumpClick = useCallback((event: React.MouseEvent<HTMLAnchorElement>) => {
        // Let modified clicks use standard link behavior (open in a new tab,
        // etc.). A normal click must not also trigger the parent post click.
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
            return;
        }
        event.stopPropagation();

        // Posts are virtualized: only scroll smoothly when the original post
        // is actually mounted in this channel. Distant/unloaded posts continue
        // through Mattermost's existing permalink loader.
        const target = document.getElementById(`post_${reference.post_id}`);
        const scroller = target?.closest<HTMLElement>('#postListScrollContainer');
        const postList = scroller?.closest<HTMLElement>('#postListContent');
        if (!target || !scroller || postList?.dataset.channelId !== channelId || scroller.clientHeight <= 0) {
            return;
        }

        const targetRect = target.getBoundingClientRect();
        const scrollerRect = scroller.getBoundingClientRect();
        const delta = targetRect.top - scrollerRect.top -
            ((scroller.clientHeight - targetRect.height) / 2);

        if (Math.abs(delta) > scroller.clientHeight * MAX_SMOOTH_SCROLL_VIEWPORTS) {
            return;
        }

        event.preventDefault();
        const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
        scroller.scrollTo({
            top: Math.max(0, scroller.scrollTop + delta),
            behavior: reducedMotion ? 'auto' : 'smooth',
        });
    }, [reference.post_id, channelId]);

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

    // Keep permalink navigation inside the app. The permalink route focuses
    // the original post (loading older messages if needed), whereas a plain
    // <a href> reloads Mattermost and loses the current channel scroll state.
    return (
        <Link
            className='r2d-inline-reply r2d-inline-reply--link'
            data-testid='r2d-inline-reply-preview'
            to={url}
            onClick={handleJumpClick}
            aria-label={'Jump to quoted message from ' + authorName}
        >
            {content}
        </Link>
    );
};

export default InlineReplyPreview;
