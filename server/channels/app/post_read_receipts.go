// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"net/http"
	"slices"

	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Read receipts ("seen by").
//
// Semantics: a user has "seen" a post when their ChannelMembers.LastViewedAt
// for the post's channel is >= post.CreateAt. This means the user viewed (or
// marked as read) the channel after the post was created; it does NOT prove
// the message itself was read. Marking a channel unread from an earlier post
// moves LastViewedAt back, so a user can drop out of a seen-by list again.
//
// Privacy rules (secaudit):
//   - only the post author, while still a channel member, may read the list;
//   - a missing post, someone else's post and a channel the author left all
//     return the identical 404 error;
//   - only user IDs are ever returned or emitted, never timestamps;
//   - only DM, GM and channels with at most
//     ServiceSettings.ReadReceiptsMaxChannelMembers members are in scope;
//   - bots, deactivated users, non-members, the author and users that opted
//     out (preference read_receipts/opt_out = "true") are never listed;
//   - opting out is two-way: an opted-out caller gets 403
//     api.post.seen.opted_out.app_error and receives no post_seen events.

const (
	// ReadReceiptsMaxSeenStatusPosts caps the post IDs accepted by
	// POST /posts/seen_status.
	ReadReceiptsMaxSeenStatusPosts = 200

	// readReceiptsMaxPostsPerView caps how many of the channel's most recent
	// posts are considered for post_seen events when a member views a channel.
	readReceiptsMaxPostsPerView = 10
	// readReceiptsMaxChannelsPerView caps how many channels of one
	// mark-as-viewed call are considered for post_seen events.
	readReceiptsMaxChannelsPerView = 5

	// Per-user rate limit shared by GET /posts/{post_id}/seen and
	// POST /posts/seen_status.
	readReceiptsGetPerSec   = 5
	readReceiptsGetMaxBurst = 30
	// post_seen emission rate limit, per (viewer, channel): sustained one
	// emission round every 5 seconds with a small burst. Throttled rounds are
	// dropped (clients can always refetch the list); the channel view itself is
	// never throttled.
	readReceiptsEmitPerMin   = 12
	readReceiptsEmitMaxBurst = 2

	readReceiptsLimiterStoreSize = 20000
)

type readReceiptsLimiters struct {
	get  *RateLimiter
	emit *throttled.GCRARateLimiterCtx
}

// ReadReceiptsRateLimiter returns the per-user rate limiter guarding
// GET /api/v4/posts/{post_id}/seen and POST /api/v4/posts/seen_status.
// It is independent of RateLimitSettings,
// so the endpoint is always rate limited.
func (s *Server) ReadReceiptsRateLimiter() *RateLimiter {
	return s.readReceiptsLimiters().get
}

func (s *Server) readReceiptsLimiters() *readReceiptsLimiters {
	s.readReceiptsOnce.Do(func() {
		limiters := &readReceiptsLimiters{}

		getSettings := model.RateLimitSettings{
			Enable:           new(true),
			PerSec:           new(readReceiptsGetPerSec),
			MaxBurst:         new(readReceiptsGetMaxBurst),
			MemoryStoreSize:  new(readReceiptsLimiterStoreSize),
			VaryByUser:       new(true),
			VaryByRemoteAddr: new(false),
		}
		getLimiter, err := NewRateLimiter(&getSettings, nil)
		if err != nil {
			s.Log().Error("Failed to create read receipts rate limiter", mlog.Err(err))
		}
		limiters.get = getLimiter

		emitStore, err := memstore.NewCtx(readReceiptsLimiterStoreSize)
		if err == nil {
			limiters.emit, err = throttled.NewGCRARateLimiterCtx(emitStore, throttled.RateQuota{
				MaxRate:  throttled.PerMin(readReceiptsEmitPerMin),
				MaxBurst: readReceiptsEmitMaxBurst,
			})
		}
		if err != nil {
			s.Log().Error("Failed to create read receipts emission rate limiter", mlog.Err(err))
		}

		s.readReceipts = limiters
	})
	return s.readReceipts
}

func (a *App) readReceiptsEnabled() bool {
	return model.SafeDereference(a.Config().ServiceSettings.EnableReadReceipts)
}

// readReceiptsChannelInScope reports whether read receipts apply to channel:
// DMs and GMs always, other message channels only up to the configured size.
func (a *App) readReceiptsChannelInScope(rctx request.CTX, channel *model.Channel) (bool, *model.AppError) {
	if channel.IsGroupOrDirect() {
		return true, nil
	}
	if !channel.IsMessageChannel() {
		return false, nil
	}
	count, appErr := a.GetChannelMemberCount(rctx, channel.Id)
	if appErr != nil {
		return false, appErr
	}
	return count <= int64(*a.Config().ServiceSettings.ReadReceiptsMaxChannelMembers), nil
}

func (a *App) userOptedOutOfReadReceipts(userID string) (bool, error) {
	pref, err := a.Srv().Store().Preference().Get(userID, model.PreferenceCategoryReadReceipts, model.PreferenceNameReadReceiptsOptOut)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return pref.Value == "true", nil
}

func (a *App) checkReadReceiptsCallerNotOptedOut(callerID string) *model.AppError {
	optedOut, err := a.userOptedOutOfReadReceipts(callerID)
	if err != nil {
		return model.NewAppError("checkReadReceiptsCallerNotOptedOut", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if optedOut {
		return model.NewAppError("checkReadReceiptsCallerNotOptedOut", "api.post.seen.opted_out.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// GetPostSeenBy returns the IDs of the users who have seen postID, see the
// package level comment above for the exact rules. callerID must be the
// session user.
func (a *App) GetPostSeenBy(rctx request.CTX, postID, callerID string) ([]string, *model.AppError) {
	if !a.readReceiptsEnabled() {
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	// Every authorization failure returns this exact error so a caller cannot
	// tell a missing post from someone else's post or a channel they left.
	notFound := func() *model.AppError {
		return model.NewAppError("GetPostSeenBy", "api.post.seen.not_found.app_error", nil, "", http.StatusNotFound)
	}

	post, err := a.Srv().Store().Post().GetSingle(rctx, postID, false)
	if err != nil {
		if isNotFoundError(err) {
			return nil, notFound()
		}
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if post.UserId != callerID {
		return nil, notFound()
	}

	if _, err := a.Srv().Store().Channel().GetMember(rctx, post.ChannelId, callerID); err != nil {
		if isNotFoundError(err) {
			return nil, notFound()
		}
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if appErr := a.checkReadReceiptsCallerNotOptedOut(callerID); appErr != nil {
		return nil, appErr
	}

	channel, appErr := a.GetChannel(rctx, post.ChannelId)
	if appErr != nil {
		return nil, appErr
	}
	inScope, appErr := a.readReceiptsChannelInScope(rctx, channel)
	if appErr != nil {
		return nil, appErr
	}
	if !inScope {
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.out_of_scope.app_error", nil, "", http.StatusForbidden)
	}

	limit := max(*a.Config().ServiceSettings.ReadReceiptsMaxChannelMembers, model.ChannelGroupMaxUsers)
	userIDs, err := a.Srv().Store().Channel().GetReadReceiptUserIds(rctx, post.ChannelId, post.CreateAt, callerID, limit)
	if err != nil {
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return userIDs, nil
}

// GetPostsSeenStatus reports, for each of postIDs that the caller may query,
// whether at least one eligible member has seen it (same rules as
// GetPostSeenBy). Posts that are missing, deleted, not authored by the caller,
// in channels the caller is no longer a member of, or in channels out of scope
// are silently omitted, so the response never distinguishes those cases.
func (a *App) GetPostsSeenStatus(rctx request.CTX, postIDs []string, callerID string) (map[string]bool, *model.AppError) {
	if !a.readReceiptsEnabled() {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	if appErr := a.checkReadReceiptsCallerNotOptedOut(callerID); appErr != nil {
		return nil, appErr
	}

	result := map[string]bool{}
	if len(postIDs) == 0 {
		return result, nil
	}

	posts, err := a.Srv().Store().Post().GetPostsByIds(postIDs)
	if err != nil && !isNotFoundError(err) {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	ownPosts := make([]*model.Post, 0, len(posts))
	channelIDSet := map[string]bool{}
	for _, post := range posts {
		if post.UserId != callerID || post.DeleteAt != 0 {
			continue
		}
		ownPosts = append(ownPosts, post)
		channelIDSet[post.ChannelId] = true
	}
	if len(ownPosts) == 0 {
		return result, nil
	}

	channelIDs := make([]string, 0, len(channelIDSet))
	for id := range channelIDSet {
		channelIDs = append(channelIDs, id)
	}

	// The caller must still be a member of the channel.
	memberships, err := a.Srv().Store().Channel().GetMembersByChannelIds(channelIDs, callerID)
	if err != nil {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	channels, err := a.Srv().Store().Channel().GetChannelsByIds(channelIDs, true)
	if err != nil {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	channelByID := make(map[string]*model.Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	allowed := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		channel := channelByID[membership.ChannelId]
		if channel == nil || membership.UserId != callerID {
			continue
		}
		inScope, appErr := a.readReceiptsChannelInScope(rctx, channel)
		if appErr != nil {
			return nil, appErr
		}
		if inScope {
			allowed = append(allowed, channel.Id)
		}
	}
	if len(allowed) == 0 {
		return result, nil
	}

	maxViewed, err := a.Srv().Store().Channel().GetReadReceiptMaxLastViewedAt(rctx, allowed, callerID)
	if err != nil {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = true
	}
	for _, post := range ownPosts {
		if !allowedSet[post.ChannelId] {
			continue
		}
		lastViewedAt, ok := maxViewed[post.ChannelId]
		result[post.Id] = ok && lastViewedAt >= post.CreateAt
	}

	return result, nil
}

// schedulePostSeenEvents runs publishPostSeenEvents off the request path, so
// the seen-by work never delays or throttles the channel view itself.
func (a *App) schedulePostSeenEvents(rctx request.CTX, channelIDs []string, viewer *model.User) {
	if !a.readReceiptsEnabled() || len(channelIDs) == 0 {
		return
	}
	channelIDs = slices.Clone(channelIDs)
	bgCtx := rctx.WithContext(context.WithoutCancel(rctx.Context()))
	a.Srv().Go(func() {
		a.publishPostSeenEvents(bgCtx, channelIDs, viewer)
	})
}

// publishPostSeenEvents notifies the authors of the most recent posts in the
// viewed channels that viewer has now seen them. Each post_seen event carries
// only {post_id, user_id} and is sent to the post author alone. It is meant to
// run asynchronously after a successful mark-as-viewed and is bounded by
// readReceiptsMaxChannelsPerView, readReceiptsMaxPostsPerView and a per
// (viewer, channel) emission rate limit.
func (a *App) publishPostSeenEvents(rctx request.CTX, channelIDs []string, viewer *model.User) {
	if !a.readReceiptsEnabled() || viewer.IsBot || viewer.DeleteAt != 0 {
		return
	}
	optedOut, err := a.userOptedOutOfReadReceipts(viewer.Id)
	if err != nil {
		rctx.Logger().Warn("Failed to read read receipts preference", mlog.String("user_id", viewer.Id), mlog.Err(err))
		return
	}
	if optedOut {
		return
	}

	limiter := a.Srv().readReceiptsLimiters().emit
	if limiter == nil {
		return
	}

	if len(channelIDs) > readReceiptsMaxChannelsPerView {
		channelIDs = channelIDs[:readReceiptsMaxChannelsPerView]
	}

	for _, channelID := range channelIDs {
		channel, appErr := a.GetChannel(rctx, channelID)
		if appErr != nil {
			continue
		}
		inScope, appErr := a.readReceiptsChannelInScope(rctx, channel)
		if appErr != nil || !inScope {
			continue
		}

		limited, _, err := limiter.RateLimitCtx(rctx.Context(), viewer.Id+":"+channelID, 1)
		if err != nil || limited {
			continue
		}

		postList, err := a.Srv().Store().Post().GetPosts(rctx, model.GetPostsOptions{
			ChannelId:        channelID,
			Page:             0,
			PerPage:          readReceiptsMaxPostsPerView,
			SkipFetchThreads: true,
		}, true, map[string]bool{})
		if err != nil {
			rctx.Logger().Warn("Failed to get posts for read receipts", mlog.String("channel_id", channelID), mlog.Err(err))
			continue
		}

		// Recipients must still be current members of the (small, in scope)
		// channel; the member list is bounded by the scope check above.
		memberIDs, err := a.Srv().Store().Channel().GetAllChannelMemberIdsByChannelId(channelID)
		if err != nil {
			rctx.Logger().Warn("Failed to get channel members for read receipts", mlog.String("channel_id", channelID), mlog.Err(err))
			continue
		}
		members := make(map[string]bool, len(memberIDs))
		for _, id := range memberIDs {
			members[id] = true
		}

		authorOptedOut := map[string]bool{}
		for _, postID := range postList.Order {
			post := postList.Posts[postID]
			if post == nil || post.UserId == viewer.Id || post.DeleteAt != 0 || post.IsSystemMessage() || !members[post.UserId] {
				continue
			}
			// Opt-out is two-way: opted-out authors get no seen events.
			optedOut, known := authorOptedOut[post.UserId]
			if !known {
				var prefErr error
				optedOut, prefErr = a.userOptedOutOfReadReceipts(post.UserId)
				optedOut = optedOut || prefErr != nil
				authorOptedOut[post.UserId] = optedOut
			}
			if optedOut {
				continue
			}
			message := model.NewWebSocketEvent(model.WebsocketEventPostSeen, "", "", post.UserId, nil, "")
			message.Add("post_id", post.Id)
			message.Add("user_id", viewer.Id)
			a.Publish(message)
		}
	}
}
