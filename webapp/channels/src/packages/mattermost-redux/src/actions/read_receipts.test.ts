// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import nock from 'nock';

import * as Actions from 'mattermost-redux/actions/read_receipts';
import {Client4} from 'mattermost-redux/client';

import TestHelper from '../../test/test_helper';
import configureStore from '../../test/test_store';

describe('Actions.ReadReceipts', () => {
    const channelId = 'channel1';
    const post1 = TestHelper.getPostMock({id: 'post1', channel_id: channelId});
    const post2 = TestHelper.getPostMock({id: 'post2', channel_id: channelId});

    function makeStore() {
        return configureStore({
            entities: {
                posts: {posts: {post1, post2}},
                users: {profiles: {user1: TestHelper.getUserMock({id: 'user1'})}},
            },
        });
    }

    let store = makeStore();

    beforeAll(() => {
        TestHelper.initBasic(Client4);
    });

    beforeEach(() => {
        store = makeStore();
    });

    afterEach(() => {
        jest.restoreAllMocks();
        nock.cleanAll();
    });

    afterAll(() => {
        TestHelper.tearDown();
    });

    describe('getPostsSeenStatus', () => {
        test('stores seen status and marks omitted posts unavailable', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status', {post_ids: ['post1', 'post2']}).
                reply(200, {seen: {post1: true}});

            const result = await store.dispatch(Actions.getPostsSeenStatus(['post1', 'post2']));

            expect(result).toEqual({data: {post1: true}});
            const {readReceipts} = store.getState().entities;
            expect(readReceipts.seenStatus).toEqual({post1: true});
            expect(readReceipts.unavailablePosts).toEqual({post2: true});
        });

        test('treats a bot response with no entries as unavailable', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(200, {seen: {}});

            await store.dispatch(Actions.getPostsSeenStatus(['post1']));

            expect(store.getState().entities.readReceipts.unavailablePosts).toEqual({post1: true});
        });

        test('disables receipts on 501', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(501, {id: 'api.post.seen.disabled.app_error', status_code: 501});

            const result = await store.dispatch(Actions.getPostsSeenStatus(['post1']));

            expect(result.error).toBeTruthy();
            expect(store.getState().entities.readReceipts.disabled).toBe(true);
        });

        test('disables receipts on 403 with an opted_out id prefix', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(403, {id: 'api.post.seen.opted_out.app_error', status_code: 403});

            await store.dispatch(Actions.getPostsSeenStatus(['post1']));

            expect(store.getState().entities.readReceipts.disabled).toBe(true);
            expect(store.getState().entities.readReceipts.outOfScopeChannels).toEqual({});
        });

        test('hides receipts for the channel on 403 with an out_of_scope id prefix', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(403, {id: 'api.post.seen.out_of_scope.app_error', status_code: 403});

            await store.dispatch(Actions.getPostsSeenStatus(['post1', 'post2']));

            const {readReceipts} = store.getState().entities;
            expect(readReceipts.outOfScopeChannels).toEqual({[channelId]: true});
            expect(readReceipts.disabled).toBe(false);
        });

        test('ignores an unrelated 403', async () => {
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(403, {id: 'api.context.permissions.app_error', status_code: 403});

            await store.dispatch(Actions.getPostsSeenStatus(['post1']));

            const {readReceipts} = store.getState().entities;
            expect(readReceipts.disabled).toBe(false);
            expect(readReceipts.outOfScopeChannels).toEqual({});
        });

        test('keeps the last state on 429', async () => {
            store.dispatch(Actions.receivedPostSeen('post1', 'user1'));
            nock(Client4.getBaseRoute()).
                post('/posts/seen_status').
                reply(429, {id: 'api.context.rate_limit.app_error', status_code: 429});

            const before = store.getState().entities.readReceipts;
            await store.dispatch(Actions.getPostsSeenStatus(['post1', 'post2']));

            expect(store.getState().entities.readReceipts).toBe(before);
        });
    });

    describe('getPostsSeenStatusBatched', () => {
        beforeEach(() => {
            jest.useFakeTimers();
        });

        afterEach(() => {
            jest.useRealTimers();
        });

        test('debounces calls into one request and dedupes ids', async () => {
            const spy = jest.spyOn(Client4, 'getPostsSeenStatus').mockResolvedValue({seen: {post1: false, post2: true}});

            store.dispatch(Actions.getPostsSeenStatusBatched(['post1']));
            store.dispatch(Actions.getPostsSeenStatusBatched(['post2']));
            store.dispatch(Actions.getPostsSeenStatusBatched(['post1']));

            expect(spy).not.toHaveBeenCalled();

            jest.advanceTimersByTime(100);
            await Promise.resolve();

            expect(spy).toHaveBeenCalledTimes(1);
            expect(spy).toHaveBeenCalledWith(['post1', 'post2']);
        });

        test('splits more than 200 ids into chunks of 200', async () => {
            const spy = jest.spyOn(Client4, 'getPostsSeenStatus').mockResolvedValue({seen: {}});
            const ids = Array.from({length: 450}, (_, i) => `post${i}`);

            store.dispatch(Actions.getPostsSeenStatusBatched(ids));
            jest.runAllTimers();
            await Promise.resolve();

            expect(spy).toHaveBeenCalledTimes(3);
            expect(spy.mock.calls.map(([batch]) => batch.length)).toEqual([200, 200, 50]);
            expect(spy.mock.calls.flatMap(([batch]) => batch)).toEqual(ids);
        });
    });

    describe('getPostSeenBy', () => {
        test('stores viewers and marks the post seen', async () => {
            nock(Client4.getBaseRoute()).
                get('/posts/post1/seen').
                reply(200, {user_ids: ['user1']});

            const result = await store.dispatch(Actions.getPostSeenBy('post1'));

            expect(result).toEqual({data: ['user1']});
            const {readReceipts} = store.getState().entities;
            expect(readReceipts.seenBy).toEqual({post1: ['user1']});
            expect(readReceipts.seenStatus).toEqual({post1: true});
        });

        test('marks the post unavailable on 404', async () => {
            nock(Client4.getBaseRoute()).
                get('/posts/post1/seen').
                reply(404, {id: 'app.post.get.app_error', status_code: 404});

            await store.dispatch(Actions.getPostSeenBy('post1'));

            expect(store.getState().entities.readReceipts.unavailablePosts).toEqual({post1: true});
        });

        test('matches error ids by prefix', async () => {
            nock(Client4.getBaseRoute()).
                get('/posts/post1/seen').
                reply(403, {id: 'api.post.seen.opted_out.other.app_error', status_code: 403});

            await store.dispatch(Actions.getPostSeenBy('post1'));

            expect(store.getState().entities.readReceipts.disabled).toBe(true);
        });
    });

    describe('receivedPostSeen', () => {
        test('marks the post seen and appends to a cached viewer list once', async () => {
            nock(Client4.getBaseRoute()).
                get('/posts/post1/seen').
                reply(200, {user_ids: ['user1']});
            await store.dispatch(Actions.getPostSeenBy('post1'));

            store.dispatch(Actions.receivedPostSeen('post1', 'user2'));
            store.dispatch(Actions.receivedPostSeen('post1', 'user2'));
            store.dispatch(Actions.receivedPostSeen('post2', 'user2'));

            const {readReceipts} = store.getState().entities;
            expect(readReceipts.seenBy).toEqual({post1: ['user1', 'user2']});
            expect(readReceipts.seenStatus).toEqual({post1: true, post2: true});
        });
    });
});
