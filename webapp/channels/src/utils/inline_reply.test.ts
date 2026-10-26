// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';

import {
    getQuotedMessageText,
    parseInlineReply,
    rememberInlineReplySelection,
    requestInlineReply,
    INLINE_REPLY_EVENT,
} from './inline_reply';

const id = 'abcdefghijklmnopqrstuvwxyz';

describe('inline reply reference', () => {
    test('accepts a versioned post reference and bounded UTF-16 selection', () => {
        expect(parseInlineReply({v: 1, post_id: id})).toEqual({v: 1, post_id: id});
        expect(parseInlineReply({v: 1, post_id: id, start: 2, end: 5})).toEqual({v: 1, post_id: id, start: 2, end: 5});
        expect(getQuotedMessageText('Chào bạn nhé', {v: 1, post_id: id, start: 5, end: 8})).toBe('bạn');
    });

    test('rejects malformed and potentially content-leaking props', () => {
        expect(parseInlineReply({v: 1, post_id: id, text: 'secret'})).toBeNull();
        expect(parseInlineReply({v: 1, post_id: id, start: 2})).toBeNull();
        expect(parseInlineReply({v: 1, post_id: id, start: -1, end: 1})).toBeNull();
        expect(parseInlineReply({v: 1, post_id: id, start: 2, end: 503})).toBeNull();
        expect(parseInlineReply({v: 2, post_id: id})).toBeNull();
    });

    test('captures a selection before the message menu removes the browser selection', () => {
        const element = document.createElement('div');
        element.textContent = 'Xin chào 👋';
        document.body.appendChild(element);
        const selection = window.getSelection();
        const range = document.createRange();
        range.setStart(element.firstChild!, 9);
        range.setEnd(element.firstChild!, 11);
        selection?.removeAllRanges();
        selection?.addRange(range);

        const post = {id, channel_id: id, message: 'Xin chào 👋'} as Post;
        rememberInlineReplySelection(post, element);
        selection?.removeAllRanges();

        const listener = jest.fn();
        document.addEventListener(INLINE_REPLY_EVENT, listener);
        requestInlineReply(post, true);
        expect((listener.mock.calls[0][0] as CustomEvent).detail.reference).toEqual({
            v: 1, post_id: id, start: 9, end: 11,
        });
        document.removeEventListener(INLINE_REPLY_EVENT, listener);
        element.remove();
    });

    test('keeps a normal reply in the channel timeline instead of using root_id', () => {
        const listener = jest.fn();
        document.addEventListener(INLINE_REPLY_EVENT, listener);
        requestInlineReply({id, channel_id: id, message: 'Hello'} as Post);
        expect(listener).toHaveBeenCalledTimes(1);
        expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual({
            channelId: id,
            reference: {v: 1, post_id: id},
        });
        document.removeEventListener(INLINE_REPLY_EVENT, listener);
    });
});
