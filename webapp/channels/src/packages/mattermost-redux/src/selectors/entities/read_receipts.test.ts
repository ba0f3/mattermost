// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import merge from 'lodash/merge';

import type {Post} from '@mattermost/types/posts';
import type {GlobalState} from '@mattermost/types/store';
import type {DeepPartial} from '@mattermost/types/utilities';

import {General, Posts} from 'mattermost-redux/constants';
import {
    canShowPostReadReceipts,
    isChannelEligibleForReadReceipts,
    isPostEligibleForReadReceipts,
    isReadReceiptsEnabled,
} from 'mattermost-redux/selectors/entities/read_receipts';

import TestHelper from '../../../test/test_helper';
import {makeInitialState} from '../../../test/test_store';

const me = 'me';
const channelId = 'channel1';

function makeState(overrides: DeepPartial<GlobalState> = {}) {
    return makeInitialState(merge({
        entities: {
            general: {config: {EnableReadReceipts: 'true', ReadReceiptsMaxChannelMembers: '5'}},
            users: {currentUserId: me},
            channels: {
                channels: {
                    [channelId]: TestHelper.getChannelMock({id: channelId, type: General.OPEN_CHANNEL}),
                    dm: TestHelper.getChannelMock({id: 'dm', type: General.DM_CHANNEL}),
                    gm: TestHelper.getChannelMock({id: 'gm', type: General.GM_CHANNEL}),
                },
                stats: {
                    [channelId]: {channel_id: channelId, member_count: 5},
                    dm: {channel_id: 'dm', member_count: 2},
                    gm: {channel_id: 'gm', member_count: 100},
                },
            },
        },
    }, overrides));
}

function makePost(overrides: Partial<Post> = {}) {
    return TestHelper.getPostMock({id: 'post1', user_id: me, channel_id: channelId, ...overrides});
}

describe('selectors.entities.readReceipts', () => {
    describe('isReadReceiptsEnabled', () => {
        test('requires the feature flag', () => {
            expect(isReadReceiptsEnabled(makeState())).toBe(true);
            expect(isReadReceiptsEnabled(makeState({entities: {general: {config: {EnableReadReceipts: 'false'}}}}))).toBe(false);
            expect(isReadReceiptsEnabled(makeState({entities: {general: {config: {EnableReadReceipts: ''}}}}))).toBe(false);
        });

        test('is off once the server disabled it for the session', () => {
            expect(isReadReceiptsEnabled(makeState({entities: {readReceipts: {disabled: true}}}))).toBe(false);
        });
    });

    describe('isChannelEligibleForReadReceipts', () => {
        test('allows DMs and GMs regardless of size', () => {
            const state = makeState();
            expect(isChannelEligibleForReadReceipts(state, 'dm')).toBe(true);
            expect(isChannelEligibleForReadReceipts(state, 'gm')).toBe(true);
        });

        test('compares channel member count to the configured max', () => {
            expect(isChannelEligibleForReadReceipts(makeState(), channelId)).toBe(true);
            const bigger = makeState({entities: {channels: {stats: {[channelId]: {member_count: 6}}}}});
            expect(isChannelEligibleForReadReceipts(bigger, channelId)).toBe(false);
        });

        test('defaults the max to 50', () => {
            const withCount = (count: number) => makeState({entities: {
                general: {config: {ReadReceiptsMaxChannelMembers: ''}},
                channels: {stats: {[channelId]: {member_count: count}}},
            }});
            expect(isChannelEligibleForReadReceipts(withCount(50), channelId)).toBe(true);
            expect(isChannelEligibleForReadReceipts(withCount(51), channelId)).toBe(false);
        });

        test('is false for channels the server reported out of scope', () => {
            const state = makeState({entities: {readReceipts: {outOfScopeChannels: {dm: true}}}});
            expect(isChannelEligibleForReadReceipts(state, 'dm')).toBe(false);
        });
    });

    describe('isPostEligibleForReadReceipts', () => {
        const state = makeState();

        test('allows my regular posts', () => {
            expect(isPostEligibleForReadReceipts(state, makePost())).toBe(true);
        });

        test.each([
            ['someone else', {user_id: 'other'}],
            ['system', {type: Posts.POST_TYPES.JOIN_CHANNEL}],
            ['ephemeral', {type: Posts.POST_TYPES.EPHEMERAL}],
            ['webhook', {props: {from_webhook: 'true'}}],
            ['bot', {props: {from_bot: 'true'}}],
            ['pending', {pending_post_id: 'post1'}],
            ['failed', {failed: true}],
            ['deleted', {delete_at: 1}],
        ] as Array<[string, Partial<Post>]>)('rejects a %s post', (_, overrides) => {
            expect(isPostEligibleForReadReceipts(state, makePost(overrides))).toBe(false);
        });
    });

    describe('canShowPostReadReceipts', () => {
        test('combines feature, post and channel checks', () => {
            expect(canShowPostReadReceipts(makeState(), makePost())).toBe(true);
            expect(canShowPostReadReceipts(makeState(), undefined)).toBe(false);
        });

        test('is false for posts the server would not report on', () => {
            const state = makeState({entities: {readReceipts: {unavailablePosts: {post1: true}}}});
            expect(canShowPostReadReceipts(state, makePost())).toBe(false);
        });
    });
});
