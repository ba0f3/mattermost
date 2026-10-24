// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ServerError} from '@mattermost/types/errors';
import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';

import {ReadReceiptTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';
import type {ActionFuncAsync, DispatchFunc, GetStateFunc} from 'mattermost-redux/types/actions';
import {DelayedDataLoader} from 'mattermost-redux/utils/data_loader';

import {forceLogoutIfNecessary} from './helpers';
import {getMissingProfilesByIds} from './users';

// Server error ids end in ".app_error", so these are matched as prefixes.
export const OPTED_OUT_ERROR_ID = 'api.post.seen.opted_out';
export const OUT_OF_SCOPE_ERROR_ID = 'api.post.seen.out_of_scope';

const MAX_BATCH_SIZE = 200;
const BATCH_WAIT = 100;

// Turns a receipts error into state so the UI hides itself. Rate limits and anything
// unexpected are dropped so the last known state stays on screen.
function handleReadReceiptsError(error: ServerError, postIds: Array<Post['id']>, dispatch: DispatchFunc, getState: GetStateFunc) {
    forceLogoutIfNecessary(error, dispatch, getState);

    const errorId = error.server_error_id ?? '';
    switch (error.status_code) {
    case 501:
        dispatch({type: ReadReceiptTypes.READ_RECEIPTS_DISABLED});
        break;
    case 403:
        if (errorId.startsWith(OPTED_OUT_ERROR_ID)) {
            dispatch({type: ReadReceiptTypes.READ_RECEIPTS_DISABLED});
        } else if (errorId.startsWith(OUT_OF_SCOPE_ERROR_ID)) {
            const posts = getState().entities.posts.posts;
            const channelIds = new Set(postIds.map((postId) => posts[postId]?.channel_id).filter(Boolean));
            for (const channelId of channelIds) {
                dispatch({type: ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_CHANNEL, data: {channelId}});
            }
        }
        break;
    case 404:
        for (const postId of postIds) {
            dispatch({type: ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_POST, data: {postId}});
        }
        break;
    }
}

export function getPostsSeenStatus(postIds: Array<Post['id']>): ActionFuncAsync<Record<Post['id'], boolean>> {
    return async (dispatch, getState) => {
        let data;
        try {
            data = await Client4.getPostsSeenStatus(postIds);
        } catch (error) {
            handleReadReceiptsError(error, postIds, dispatch, getState);
            return {error};
        }

        const seen = data.seen ?? {};
        dispatch({
            type: ReadReceiptTypes.RECEIVED_POSTS_SEEN_STATUS,
            data: {postIds, seen},
        });

        return {data: seen};
    };
}

// Queues posts for one shared seen_status request so a channel full of posts costs one call per 200.
export function getPostsSeenStatusBatched(postIds: Array<Post['id']>): ActionFuncAsync<true> {
    return async (dispatch, getState, {loaders}: any) => {
        if (!loaders.postsSeenStatusLoader) {
            loaders.postsSeenStatusLoader = new DelayedDataLoader<Post['id']>({
                fetchBatch: (ids) => dispatch(getPostsSeenStatus(ids)),
                maxBatchSize: MAX_BATCH_SIZE,
                wait: BATCH_WAIT,
            });
        }

        loaders.postsSeenStatusLoader.queue(postIds);

        return {data: true};
    };
}

export function getPostSeenBy(postId: Post['id']): ActionFuncAsync<Array<UserProfile['id']>> {
    return async (dispatch, getState) => {
        let data;
        try {
            data = await Client4.getPostSeenBy(postId);
        } catch (error) {
            handleReadReceiptsError(error, [postId], dispatch, getState);
            return {error};
        }

        const userIds = data.user_ids ?? [];
        dispatch({
            type: ReadReceiptTypes.RECEIVED_POST_SEEN_BY,
            data: {postId, userIds},
        });

        if (userIds.length > 0) {
            dispatch(getMissingProfilesByIds(userIds));
        }

        return {data: userIds};
    };
}

export function receivedPostSeen(postId: Post['id'], userId: UserProfile['id']) {
    return {
        type: ReadReceiptTypes.RECEIVED_POST_SEEN,
        data: {postId, userId},
    };
}
