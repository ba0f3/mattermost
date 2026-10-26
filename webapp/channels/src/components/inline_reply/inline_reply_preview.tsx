// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useSelector} from 'react-redux';
import {useHistory} from 'react-router-dom';

import {ArrowRightBoldOutlineIcon} from '@mattermost/compass-icons/components';
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
const TARGET_HIGHLIGHT_CLASS = 'r2d-inline-reply-target--highlight';
const TARGET_HIGHLIGHT_DURATION_MS = 2200;
const targetHighlightTimers = new WeakMap<HTMLElement, number>();

function highlightQuotedPost(target: HTMLElement) {
    const pending = targetHighlightTimers.get(target);
    if (pending !== undefined) {
        window.clearTimeout(pending);
    }

    target.classList.remove(TARGET_HIGHLIGHT_CLASS);
    // Flush styles so repeated clicks restart the highlight animation.
    target.getBoundingClientRect();
    target.classList.add(TARGET_HIGHLIGHT_CLASS);

    const timer = window.setTimeout(() => {
        target.classList.remove(TARGET_HIGHLIGHT_CLASS);
        targetHighlightTimers.delete(target);
    }, TARGET_HIGHLIGHT_DURATION_MS);
    targetHighlightTimers.set(target, timer);
}

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
    const history = useHistory();

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
            <span className='r2d-inline-reply__content'>
                <strong className='r2d-inline-reply__author'>{visible ? authorName : 'Original message'}</strong>
                <span className='r2d-inline-reply__text'>{previewText}</span>
            </span>
            {visible && !inComposer && (
                <ArrowRightBoldOutlineIcon
                    size={14}
                    className='r2d-inline-reply__jump-icon'
                />
            )}
        </>
    );

    const handleJumpClick = useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
        // This is a clickable quote preview, not a traditional hyperlink.
        // Don't open a thread or trigger the enclosing message click.
        event.stopPropagation();

        // Only animate posts already mounted in the same channel.
        // Unloaded history follows Mattermost's permalink/focus mechanism.
        const target = document.getElementById(`post_${reference.post_id}`);
        const scroller = target?.closest<HTMLElement>('#postListScrollContainer');
        const postList = scroller?.closest<HTMLElement>('#postListContent');
        if (!target || !scroller || postList?.dataset.channelId !== channelId || scroller.clientHeight <= 0) {
            history.push(url);
            return;
        }

        const targetRect = target.getBoundingClientRect();
        const scrollerRect = scroller.getBoundingClientRect();
        const delta = targetRect.top - scrollerRect.top -
            ((scroller.clientHeight - targetRect.height) / 2);

        if (Math.abs(delta) > scroller.clientHeight * MAX_SMOOTH_SCROLL_VIEWPORTS) {
            history.push(url);
            return;
        }

        const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
        scroller.scrollTo({
            top: Math.max(0, scroller.scrollTop + delta),
            behavior: reducedMotion ? 'auto' : 'smooth',
        });
        // A short smooth scroll bypasses the permalink focus action, so we
        // apply its familiar themed post highlight to the original message.
        highlightQuotedPost(target);
    }, [reference.post_id, channelId, history, url]);

    if (!visible || inComposer) {
        return (
            <div
                className='r2d-inline-reply r2d-inline-reply--static'
                data-testid='r2d-inline-reply-preview'
            >
                {content}
            </div>
        );
    }

    // A semantic button makes the quote look like a compact message card,
    // while the in-app router still handles far-away destinations.
    return (
        <button
            type='button'
            className='r2d-inline-reply r2d-inline-reply--jump'
            data-testid='r2d-inline-reply-preview'
            onClick={handleJumpClick}
            aria-label={'Jump to quoted message from ' + authorName}
        >
            {content}
        </button>
    );
};

export default InlineReplyPreview;
