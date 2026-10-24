// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {combineReducers} from 'redux';

import type {PreferenceType} from '@mattermost/types/preferences';
import type {ReadReceiptsState} from '@mattermost/types/read_receipts';

import {PreferenceTypes, ReadReceiptTypes, UserTypes} from 'mattermost-redux/action_types';
import type {MMReduxAction} from 'mattermost-redux/action_types';
import {Preferences} from 'mattermost-redux/constants';

function seenStatus(state: ReadReceiptsState['seenStatus'] = {}, action: MMReduxAction): ReadReceiptsState['seenStatus'] {
    switch (action.type) {
    case ReadReceiptTypes.RECEIVED_POSTS_SEEN_STATUS:
        return {...state, ...action.data.seen};
    case ReadReceiptTypes.RECEIVED_POST_SEEN_BY: {
        const {postId, userIds} = action.data;
        const seen = userIds.length > 0;
        if (state[postId] === seen) {
            return state;
        }
        return {...state, [postId]: seen};
    }
    case ReadReceiptTypes.RECEIVED_POST_SEEN: {
        const {postId} = action.data;
        if (state[postId]) {
            return state;
        }
        return {...state, [postId]: true};
    }
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function seenBy(state: ReadReceiptsState['seenBy'] = {}, action: MMReduxAction): ReadReceiptsState['seenBy'] {
    switch (action.type) {
    case ReadReceiptTypes.RECEIVED_POST_SEEN_BY: {
        const {postId, userIds} = action.data;
        return {...state, [postId]: userIds};
    }
    case ReadReceiptTypes.RECEIVED_POST_SEEN: {
        const {postId, userId} = action.data;
        const userIds = state[postId];
        if (!userIds || userIds.includes(userId)) {
            return state;
        }
        return {...state, [postId]: [...userIds, userId]};
    }
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function unavailablePosts(state: ReadReceiptsState['unavailablePosts'] = {}, action: MMReduxAction): ReadReceiptsState['unavailablePosts'] {
    switch (action.type) {
    case ReadReceiptTypes.RECEIVED_POSTS_SEEN_STATUS: {
        // The server omits posts it won't report on, so don't keep asking about them.
        const missing = (action.data.postIds as string[]).filter((postId) => !(postId in action.data.seen));
        if (missing.length === 0) {
            return state;
        }
        const nextState = {...state};
        for (const postId of missing) {
            nextState[postId] = true;
        }
        return nextState;
    }
    case ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_POST:
        return {...state, [action.data.postId]: true};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function outOfScopeChannels(state: ReadReceiptsState['outOfScopeChannels'] = {}, action: MMReduxAction): ReadReceiptsState['outOfScopeChannels'] {
    switch (action.type) {
    case ReadReceiptTypes.READ_RECEIPTS_UNAVAILABLE_FOR_CHANNEL:
        return {...state, [action.data.channelId]: true};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function isOptOutPreference(preference: PreferenceType) {
    return preference.category === Preferences.CATEGORY_READ_RECEIPTS && preference.name === Preferences.READ_RECEIPTS_OPT_OUT;
}

function disabled(state = false, action: MMReduxAction): boolean {
    switch (action.type) {
    case ReadReceiptTypes.READ_RECEIPTS_DISABLED:
        return true;
    case PreferenceTypes.RECEIVED_PREFERENCES:
    case PreferenceTypes.DELETED_PREFERENCES:
        // Changing the opt-out setting gives the server another chance to answer.
        if (state && (action.data as PreferenceType[]).some(isOptOutPreference)) {
            return false;
        }
        return state;
    case UserTypes.LOGOUT_SUCCESS:
        return false;
    default:
        return state;
    }
}

export default combineReducers({
    seenStatus,
    seenBy,
    unavailablePosts,
    outOfScopeChannels,
    disabled,
});
