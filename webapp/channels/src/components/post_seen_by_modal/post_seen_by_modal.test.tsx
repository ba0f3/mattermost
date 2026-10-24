// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {getPostSeenBy} from 'mattermost-redux/actions/read_receipts';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import PostSeenByModal from './post_seen_by_modal';

jest.mock('mattermost-redux/actions/read_receipts', () => ({
    getPostSeenBy: jest.fn(() => () => Promise.resolve({data: []})),
}));

jest.mock('mattermost-redux/actions/users', () => ({
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_GET_MISSING_PROFILES'})),
}));

describe('PostSeenByModal', () => {
    const alice = TestHelper.getUserMock({id: 'alice', username: 'alice', first_name: 'Alice', last_name: 'A'});
    const bob = TestHelper.getUserMock({id: 'bob', username: 'bob'});
    const post = TestHelper.getPostMock({id: 'post1', type: ''});

    function makeState(overrides: DeepPartial<GlobalState['entities']> = {}): DeepPartial<GlobalState> {
        return {
            entities: {
                users: {currentUserId: 'me', profiles: {alice, bob}},
                posts: {posts: {post1: post}, reactions: {}},
                ...overrides,
            },
        };
    }

    beforeEach(() => {
        jest.mocked(getPostSeenBy).mockClear();
        jest.mocked(getMissingProfilesByIds).mockClear();
    });

    test('fetches viewers and lists them by display name', async () => {
        renderWithContext(<PostSeenByModal postId='post1'/>, makeState({readReceipts: {seenBy: {post1: ['alice', 'bob']}}}));

        expect(getPostSeenBy).toHaveBeenCalledWith('post1');
        expect(await screen.findByText('alice')).toBeInTheDocument();
        expect(screen.getByText('bob')).toBeInTheDocument();
        expect(screen.queryByTestId('postSeenByModalEmpty')).not.toBeInTheDocument();
    });

    test('shows the empty state when no one has seen the post', async () => {
        renderWithContext(<PostSeenByModal postId='post1'/>, makeState());

        expect(await screen.findByText('No one has seen this message yet.')).toBeInTheDocument();
    });

    test('shows who reacted with which emoji', async () => {
        renderWithContext(<PostSeenByModal postId='post1'/>, makeState({
            posts: {
                posts: {post1: post},
                reactions: {post1: {'bob-smile': {user_id: 'bob', post_id: 'post1', emoji_name: 'smile', create_at: 1}}},
            },
        }));

        expect(await screen.findByText('Reactions')).toBeInTheDocument();
        expect(screen.getByText('bob')).toBeInTheDocument();
        expect(getMissingProfilesByIds).toHaveBeenCalledWith(['bob']);
    });

    test('renders display names as text, not HTML', async () => {
        const mallory = TestHelper.getUserMock({id: 'mallory', username: '<img src=x onerror=alert(1)>'});
        renderWithContext(<PostSeenByModal postId='post1'/>, makeState({
            users: {currentUserId: 'me', profiles: {mallory}},
            readReceipts: {seenBy: {post1: ['mallory']}},
        }));

        expect(await screen.findByText('<img src=x onerror=alert(1)>')).toBeInTheDocument();
        expect(document.querySelector('img[src="x"]')).toBeNull();
    });
});
