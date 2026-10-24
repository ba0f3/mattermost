// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from './channels';
import type {Post} from './posts';
import type {UserProfile} from './users';

// Response of GET /api/v4/posts/{post_id}/seen. User IDs only, never timestamps.
export type PostSeenBy = {
    user_ids: Array<UserProfile['id']>;
};

// Response of POST /api/v4/posts/seen_status. Posts the caller can't query are omitted.
export type PostsSeenStatus = {
    seen: Record<Post['id'], boolean>;
};

export type ReadReceiptsState = {

    // Whether at least one other member has seen each of the current user's posts.
    seenStatus: Record<Post['id'], boolean>;

    // Who has seen a post, once the seen-by list has been opened for it.
    seenBy: Record<Post['id'], Array<UserProfile['id']>>;

    // Posts the server returned no receipts for, so they show none.
    unavailablePosts: Record<Post['id'], true>;

    // Channels the server reported as over the member threshold.
    outOfScopeChannels: Record<Channel['id'], true>;

    // Set for the rest of the session when the server reports the feature off or the user opted out.
    disabled: boolean;
};
