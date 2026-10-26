// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import InlineReplyPreview from './inline_reply_preview';

const id = 'abcdefghijklmnopqrstuvwxyz';
const source = {
    id,
    channel_id: 'channel1',
    user_id: 'user1',
    message: 'Xin chào mọi người',
    type: '',
    delete_at: 0,
} as Post;

const initialState = {
    entities: {
        posts: {posts: {[id]: source}},
        users: {profiles: {user1: {id: 'user1', username: 'Minh'}}},
        teams: {
            currentTeamId: 'team1',
            teams: {team1: {id: 'team1', name: 'team1'}},
        },
    },
};

describe('InlineReplyPreview', () => {
    test('renders the current authorized source and a direct permalink', () => {
        renderWithContext(
            <InlineReplyPreview
                channelId='channel1'
                reference={{v: 1, post_id: id, start: 0, end: 8}}
            />,
            initialState,
        );
        expect(screen.getByText('Minh')).toBeInTheDocument();
        expect(screen.getByText('Xin chào')).toBeInTheDocument();
        expect(screen.getByRole('link')).toHaveAttribute('href', `/team1/pl/${id}`);
    });

    test('does not display quoted content when a message is deleted', () => {
        const deletedState = {
            entities: {
                ...initialState.entities,
                posts: {posts: {[id]: {...source, delete_at: 1}}},
            },
        };
        renderWithContext(
            <InlineReplyPreview
                channelId='channel1'
                reference={{v: 1, post_id: id}}
            />,
            deletedState,
        );
        expect(screen.getByText('Message unavailable')).toBeInTheDocument();
        expect(screen.queryByText('Xin chào mọi người')).not.toBeInTheDocument();
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });
});
