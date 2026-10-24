// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';
import type {GlobalState} from '@mattermost/types/store';
import type {UserProfile} from '@mattermost/types/users';

import {General} from 'mattermost-redux/constants';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/common';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {isFromWebhook, isPostEphemeral, isPostPendingOrFailed, isSystemMessage} from 'mattermost-redux/utils/post_utils';

const DEFAULT_MAX_CHANNEL_MEMBERS = 50;

export function isReadReceiptsFeatureEnabled(state: GlobalState): boolean {
    return getConfig(state).EnableReadReceipts === 'true';
}

// Whether receipts can be shown anywhere for the current user this session.
export function isReadReceiptsEnabled(state: GlobalState): boolean {
    return isReadReceiptsFeatureEnabled(state) &&
        !state.entities.readReceipts.disabled;
}

function getMaxChannelMembers(state: GlobalState): number {
    const max = parseInt(getConfig(state).ReadReceiptsMaxChannelMembers ?? '', 10);
    return Number.isNaN(max) ? DEFAULT_MAX_CHANNEL_MEMBERS : max;
}

export function isChannelEligibleForReadReceipts(state: GlobalState, channelId: string): boolean {
    if (state.entities.readReceipts.outOfScopeChannels[channelId]) {
        return false;
    }

    const channel = state.entities.channels.channels[channelId];
    if (channel?.type === General.DM_CHANNEL || channel?.type === General.GM_CHANNEL) {
        return true;
    }

    // Without a member count the server decides, and it omits posts from channels that are too big.
    const memberCount = state.entities.channels.stats[channelId]?.member_count;
    return memberCount === undefined || memberCount <= getMaxChannelMembers(state);
}

export function isPostEligibleForReadReceipts(state: GlobalState, post: Post | undefined): post is Post {
    if (!post || post.user_id !== getCurrentUserId(state)) {
        return false;
    }

    if (isSystemMessage(post) || isPostEphemeral(post) || isFromWebhook(post) || post.props?.from_bot === 'true') {
        return false;
    }

    if (isPostPendingOrFailed(post) || post.delete_at > 0) {
        return false;
    }

    return true;
}

export function canShowPostReadReceipts(state: GlobalState, post: Post | undefined): boolean {
    return isReadReceiptsEnabled(state) &&
        isPostEligibleForReadReceipts(state, post) &&
        !state.entities.readReceipts.unavailablePosts[post.id] &&
        isChannelEligibleForReadReceipts(state, post.channel_id);
}

export function getPostSeenStatus(state: GlobalState, postId: Post['id']): boolean | undefined {
    return state.entities.readReceipts.seenStatus[postId];
}

export function getPostSeenBy(state: GlobalState, postId: Post['id']): Array<UserProfile['id']> | undefined {
    return state.entities.readReceipts.seenBy[postId];
}
