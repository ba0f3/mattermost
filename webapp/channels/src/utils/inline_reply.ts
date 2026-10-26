// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';

export const INLINE_REPLY_PROP = 'r2d_inline_reply';
export const INLINE_REPLY_EVENT = 'mattermost:r2d-inline-reply';
export const MAX_QUOTED_SELECTION_LENGTH = 500;

export type InlineReplyReference = {
    v: 1;
    post_id: string;
    start?: number;
    end?: number;
};

export type InlineReplyRequest = {
    channelId: string;
    reference: InlineReplyReference;
};

// A browser selection often disappears when the user opens the post menu.
// Capture its position in the originating post, not the selected text itself.
let lastSelectedRange: {postId: string; start: number; end: number; at: number} | undefined;

export function rememberInlineReplySelection(post: Post, element: HTMLElement): void {
    const selection = window.getSelection();
    if (!selection || selection.isCollapsed || !selection.anchorNode || !selection.focusNode ||
        !element.contains(selection.anchorNode) || !element.contains(selection.focusNode)) {
        return;
    }
    const selected = selection.toString().trim();
    const start = post.message.indexOf(selected);
    if (!selected || start < 0 || selected.length > MAX_QUOTED_SELECTION_LENGTH) {
        return;
    }

    lastSelectedRange = {postId: post.id, start, end: start + selected.length, at: Date.now()};
}

export function parseInlineReply(value: unknown): InlineReplyReference | null {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
        return null;
    }
    const ref = value as Record<string, unknown>;
    if (ref.v !== 1 || typeof ref.post_id !== 'string' || !/^[a-z0-9]{26}$/.test(ref.post_id)) {
        return null;
    }
    const hasStart = Object.hasOwn(ref, 'start');
    const hasEnd = Object.hasOwn(ref, 'end');
    if (hasStart !== hasEnd || Object.keys(ref).length !== (hasStart ? 4 : 2)) {
        return null;
    }
    if (!hasStart) {
        return {v: 1, post_id: ref.post_id};
    }
    if (typeof ref.start !== 'number' || typeof ref.end !== 'number' ||
        !Number.isInteger(ref.start) || !Number.isInteger(ref.end) || ref.start < 0 ||
        ref.end <= ref.start || ref.end - ref.start > MAX_QUOTED_SELECTION_LENGTH) {
        return null;
    }
    return {v: 1, post_id: ref.post_id, start: ref.start, end: ref.end};
}

export function requestInlineReply(post: Post, quoteSelection = false): void {
    const reference: InlineReplyReference = {v: 1, post_id: post.id};
    const range = lastSelectedRange;
    if (quoteSelection && range?.postId === post.id && Date.now() - range.at < 30000) {
        reference.start = range.start;
        reference.end = range.end;
    }
    lastSelectedRange = undefined;
    document.dispatchEvent(new CustomEvent<InlineReplyRequest>(INLINE_REPLY_EVENT, {
        detail: {channelId: post.channel_id, reference},
    }));
}

export function getQuotedMessageText(message: string, reference: InlineReplyReference): string {
    const selection = typeof reference.start === 'number' && typeof reference.end === 'number' &&
        reference.end <= message.length ? message.slice(reference.start, reference.end) : message;
    return selection.replace(/\s+/g, ' ').trim().slice(0, 240);
}
