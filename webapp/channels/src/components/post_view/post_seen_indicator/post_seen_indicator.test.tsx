// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {getPostsSeenStatusBatched} from 'mattermost-redux/actions/read_receipts';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import PostSeenIndicator from './post_seen_indicator';

jest.mock('mattermost-redux/actions/read_receipts', () => ({
    getPostsSeenStatusBatched: jest.fn(() => ({type: 'MOCK_GET_POSTS_SEEN_STATUS_BATCHED'})),
}));

describe('PostSeenIndicator', () => {
    const me = TestHelper.getUserMock({id: 'me'});
    const channel = TestHelper.getChannelMock({id: 'dm', type: 'D'});
    const post = TestHelper.getPostMock({id: 'post1', user_id: me.id, channel_id: channel.id, type: ''});

    function makeState(overrides: DeepPartial<GlobalState> = {}): DeepPartial<GlobalState> {
        return {
            ...overrides,
            entities: {
                ...overrides.entities,
                general: {config: {EnableReadReceipts: 'true'}, ...overrides.entities?.general},
                users: {currentUserId: me.id, profiles: {[me.id]: me}},
                channels: {channels: {[channel.id]: channel}},
            },
        };
    }

    beforeEach(() => {
        jest.mocked(getPostsSeenStatusBatched).mockClear();
    });

    test('shows a single check when sent and queues a status fetch', () => {
        renderWithContext(<PostSeenIndicator post={post}/>, makeState());

        const indicator = screen.getByRole('img', {name: 'Sent'});
        expect(indicator).not.toHaveClass('PostSeenIndicator--seen');
        expect(getPostsSeenStatusBatched).toHaveBeenCalledWith([post.id]);
    });

    test('shows a single check without refetching once the status is known', () => {
        renderWithContext(<PostSeenIndicator post={post}/>, makeState({entities: {readReceipts: {seenStatus: {[post.id]: false}}}}));

        expect(screen.getByRole('img', {name: 'Sent'})).toBeInTheDocument();
        expect(getPostsSeenStatusBatched).not.toHaveBeenCalled();
    });

    test('shows a double check when seen', () => {
        renderWithContext(<PostSeenIndicator post={post}/>, makeState({entities: {readReceipts: {seenStatus: {[post.id]: true}}}}));

        expect(screen.getByRole('img', {name: 'Seen'})).toHaveClass('PostSeenIndicator--seen');
    });

    test.each([
        ['the feature is off', {entities: {general: {config: {EnableReadReceipts: 'false'}}}}],
        ['the server disabled it', {entities: {readReceipts: {disabled: true}}}],
        ['the post is unavailable', {entities: {readReceipts: {unavailablePosts: {post1: true}}}}],
        ['the channel is out of scope', {entities: {readReceipts: {outOfScopeChannels: {dm: true}}}}],
    ] as Array<[string, DeepPartial<GlobalState>]>)('renders nothing when %s', (_, overrides) => {
        renderWithContext(<PostSeenIndicator post={post}/>, makeState(overrides));

        expect(screen.queryByTestId(`post_seen_indicator_${post.id}`)).not.toBeInTheDocument();
        expect(getPostsSeenStatusBatched).not.toHaveBeenCalled();
    });

    test('renders nothing on someone else\'s post', () => {
        renderWithContext(<PostSeenIndicator post={{...post, user_id: 'other'}}/>, makeState());

        expect(screen.queryByTestId(`post_seen_indicator_${post.id}`)).not.toBeInTheDocument();
    });
});
