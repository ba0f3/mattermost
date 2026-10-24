// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// PostSeenBy is the response of GET /api/v4/posts/{post_id}/seen. By design
// it carries only user IDs: never add timestamps or other per-user fields.
type PostSeenBy struct {
	UserIds []string `json:"user_ids"`
}

// PostsSeenStatusRequest is the body of POST /api/v4/posts/seen_status.
type PostsSeenStatusRequest struct {
	PostIds []string `json:"post_ids"`
}

// PostsSeenStatus is the response of POST /api/v4/posts/seen_status. It maps
// each queryable post ID to whether at least one eligible member has seen it.
// Post IDs the caller may not query are omitted.
type PostsSeenStatus struct {
	Seen map[string]bool `json:"seen"`
}
