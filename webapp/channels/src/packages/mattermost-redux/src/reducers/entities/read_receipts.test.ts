// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {ReadReceiptTypes, UserTypes} from 'mattermost-redux/action_types';
import reducer from 'mattermost-redux/reducers/entities/read_receipts';

type ReducerState = ReturnType<typeof reducer>;

const initialState = reducer(undefined, {type: 'INIT'});

describe('reducers.entities.readReceipts', () => {
    test('initial state', () => {
        expect(initialState).toEqual({
            seenStatus: {},
            seenBy: {},
            unavailablePosts: {},
            outOfScopeChannels: {},
            disabled: false,
        });
    });

    test('RECEIVED_POSTS_SEEN_STATUS merges statuses and marks omitted posts unavailable', () => {
        let state = reducer(initialState, {
            type: ReadReceiptTypes.RECEIVED_POSTS_SEEN_STATUS,
            data: {postIds: ['a'], seen: {a: false}},
        });
        state = reducer(state, {
            type: ReadReceiptTypes.RECEIVED_POSTS_SEEN_STATUS,
            data: {postIds: ['b', 'c'], seen: {b: true}},
        });

        expect(state.seenStatus).toEqual({a: false, b: true});
        expect(state.unavailablePosts).toEqual({c: true});
    });

    test('RECEIVED_POST_SEEN_BY stores viewers and derives seen', () => {
        let state = reducer(initialState, {
            type: ReadReceiptTypes.RECEIVED_POST_SEEN_BY,
            data: {postId: 'a', userIds: []},
        });
        expect(state.seenStatus).toEqual({a: false});
        expect(state.seenBy).toEqual({a: []});

        state = reducer(state, {
            type: ReadReceiptTypes.RECEIVED_POST_SEEN_BY,
            data: {postId: 'a', userIds: ['u1']},
        });
        expect(state.seenStatus).toEqual({a: true});
        expect(state.seenBy).toEqual({a: ['u1']});
    });

    test('RECEIVED_POST_SEEN sets seen and appends only to cached lists', () => {
        let state: ReducerState = {
            ...initialState,
            seenStatus: {a: false},
            seenBy: {a: ['u1']},
        };

        state = reducer(state, {type: ReadReceiptTypes.RECEIVED_POST_SEEN, data: {postId: 'a', userId: 'u2'}});
        state = reducer(state, {type: ReadReceiptTypes.RECEIVED_POST_SEEN, data: {postId: 'a', userId: 'u1'}});
        state = reducer(state, {type: ReadReceiptTypes.RECEIVED_POST_SEEN, data: {postId: 'b', userId: 'u1'}});

        expect(state.seenStatus).toEqual({a: true, b: true});
        expect(state.seenBy).toEqual({a: ['u1', 'u2']});
    });

    test('unavailable posts and channels are remembered', () => {
        let state = reducer(initialState, {type: ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_POST, data: {postId: 'a'}});
        state = reducer(state, {type: ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_CHANNEL, data: {channelId: 'c'}});

        expect(state.unavailablePosts).toEqual({a: true});
        expect(state.outOfScopeChannels).toEqual({c: true});
    });

    test('LOGOUT_SUCCESS resets everything', () => {
        const state: ReducerState = {
            seenStatus: {a: true},
            seenBy: {a: ['u1']},
            unavailablePosts: {b: true},
            outOfScopeChannels: {c: true},
            disabled: true,
        };

        expect(reducer(state, {type: UserTypes.LOGOUT_SUCCESS})).toEqual(initialState);
    });
});
