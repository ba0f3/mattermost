// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"maps"
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
// the message itself was read. While read receipts are enabled, the APIs
// that would move LastViewedAt back (marking a post, thread or channel
// unread) are refused, so once a user has seen a post they stay listed.
//
// Privacy rules (secaudit):
//   - only the post author, while still a channel member, may read the list;
//   - a missing post, someone else's post and a channel the author left all
//     return the identical 404 error;
//   - only user IDs are ever returned or emitted, never timestamps;
//   - only DM, GM and channels with at most
//     ServiceSettings.ReadReceiptsMaxChannelMembers members are in scope;
//   - bots, deactivated users, non-members and the author are never listed;
//   - there is no per-user opt-out: read receipts are a workspace setting
//     (ServiceSettings.EnableReadReceipts, on by default) that only an admin
//     can turn off;
//   - bot sessions never get receipts: GET returns the not-found 404 and
//     the batch endpoint returns an empty "seen" map, as if none of the
//     posts were the caller's;
//   - system messages and posts made by webhooks or bots (from_webhook /
//     from_bot) have no receipts: GET returns the not-found 404 and the batch
//     endpoint reports false;
//   - post_seen events are only emitted when the session user marks their
//     own channel as viewed (never when an admin or plugin does it on their
//     behalf), and only for posts created after the viewer's previous
//     LastViewedAt, so already seen posts never leak presence again.

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
	// post_seen emission rate limits. The per viewer limit is checked first,
	// before any database work; the per (viewer, channel) limit is a second
	// layer. Throttled rounds are dropped (clients can always refetch the
	// list); the channel view itself is never throttled.
	readReceiptsEmitViewerPerMin   = 30
	readReceiptsEmitViewerMaxBurst = 10
	readReceiptsEmitPerMin         = 12
	readReceiptsEmitMaxBurst       = 2

	readReceiptsLimiterStoreSize = 20000
)

type readReceiptsLimiters struct {
	get        *RateLimiter
	emitViewer *throttled.GCRARateLimiterCtx
	emit       *throttled.GCRARateLimiterCtx
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

		newEmitLimiter := func(perMin, burst int) *throttled.GCRARateLimiterCtx {
			emitStore, err := memstore.NewCtx(readReceiptsLimiterStoreSize)
			var limiter *throttled.GCRARateLimiterCtx
			if err == nil {
				limiter, err = throttled.NewGCRARateLimiterCtx(emitStore, throttled.RateQuota{
					MaxRate:  throttled.PerMin(perMin),
					MaxBurst: burst,
				})
			}
			if err != nil {
				s.Log().Error("Failed to create read receipts emission rate limiter", mlog.Err(err))
				return nil
			}
			return limiter
		}
		limiters.emitViewer = newEmitLimiter(readReceiptsEmitViewerPerMin, readReceiptsEmitViewerMaxBurst)
		limiters.emit = newEmitLimiter(readReceiptsEmitPerMin, readReceiptsEmitMaxBurst)

		s.readReceipts = limiters
	})
	return s.readReceipts
}

func (a *App) readReceiptsEnabled() bool {
	return model.SafeDereference(a.Config().ServiceSettings.EnableReadReceipts)
}

// CheckMarkUnreadAllowed refuses moving a read pointer backward (marking a
// post, channel or thread unread) while read receipts are enabled, so users
// cannot hide that they have read a message.
func (a *App) CheckMarkUnreadAllowed() *model.AppError {
	if a.readReceiptsEnabled() {
		return model.NewAppError("CheckMarkUnreadAllowed", "api.post.set_unread.read_receipts_enabled.app_error", nil, "", http.StatusForbidden)
	}
	return nil
}

// readReceiptsChannelInScope reports whether read receipts apply to channel:
// DMs and GMs always, other message channels only up to the configured size.
func (a *App) readReceiptsChannelInScope(rctx request.CTX, channel *model.Channel) (bool, *model.AppError) {
	if !readReceiptsNeedsMemberCount(channel) {
		return channel.IsGroupOrDirect(), nil
	}
	count, appErr := a.GetChannelMemberCount(rctx, channel.Id)
	if appErr != nil {
		return false, appErr
	}
	return a.readReceiptsMemberCountInScope(count), nil
}

// readReceiptsNeedsMemberCount reports whether the scope of channel depends on
// its member count (message channels other than DMs and GMs).
func readReceiptsNeedsMemberCount(channel *model.Channel) bool {
	return !channel.IsGroupOrDirect() && channel.IsMessageChannel()
}

func (a *App) readReceiptsMemberCountInScope(count int64) bool {
	return count <= int64(*a.Config().ServiceSettings.ReadReceiptsMaxChannelMembers)
}

// postHasNoReadReceipts reports whether post is excluded from read receipts:
// system messages and posts made by webhooks or bots.
func postHasNoReadReceipts(post *model.Post) bool {
	// Same presence test CreatePost uses for these props.
	return post.IsSystemMessage() ||
		post.GetProp(model.PostPropsFromWebhook) != nil ||
		post.GetProp(model.PostPropsFromBot) != nil
}

func isBotSession(rctx request.CTX) bool {
	session := rctx.Session()
	return session != nil && session.IsBotUser()
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

	if isBotSession(rctx) {
		return nil, notFound()
	}

	post, err := a.Srv().Store().Post().GetSingle(rctx, postID, false)
	if err != nil {
		if isNotFoundError(err) {
			return nil, notFound()
		}
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if post.UserId != callerID || postHasNoReadReceipts(post) {
		return nil, notFound()
	}

	if _, err := a.Srv().Store().Channel().GetMember(rctx, post.ChannelId, callerID); err != nil {
		if isNotFoundError(err) {
			return nil, notFound()
		}
		return nil, model.NewAppError("GetPostSeenBy", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
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
// Queryable system, webhook and bot posts are always reported as false. A bot
// session gets an empty result, as if it had authored none of the posts.
func (a *App) GetPostsSeenStatus(rctx request.CTX, postIDs []string, callerID string) (map[string]bool, *model.AppError) {
	if !a.readReceiptsEnabled() {
		return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	result := map[string]bool{}
	if isBotSession(rctx) {
		return result, nil
	}

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

	memberChannels := make([]*model.Channel, 0, len(memberships))
	countIDs := []string{}
	for _, membership := range memberships {
		channel := channelByID[membership.ChannelId]
		if channel == nil || membership.UserId != callerID {
			continue
		}
		memberChannels = append(memberChannels, channel)
		if readReceiptsNeedsMemberCount(channel) {
			countIDs = append(countIDs, channel.Id)
		}
	}

	// One query for the member counts of all channels whose scope depends on it.
	counts := map[string]int64{}
	if len(countIDs) > 0 {
		counts, err = a.Srv().Store().Channel().GetChannelsMemberCount(countIDs)
		if err != nil {
			return nil, model.NewAppError("GetPostsSeenStatus", "api.post.seen.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	allowed := make([]string, 0, len(memberChannels))
	for _, channel := range memberChannels {
		inScope := channel.IsGroupOrDirect()
		if readReceiptsNeedsMemberCount(channel) {
			count, ok := counts[channel.Id]
			inScope = ok && a.readReceiptsMemberCountInScope(count)
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
		if postHasNoReadReceipts(post) {
			result[post.Id] = false
			continue
		}
		lastViewedAt, ok := maxViewed[post.ChannelId]
		result[post.Id] = ok && lastViewedAt >= post.CreateAt
	}

	return result, nil
}

// postSeenBatch is the work for one mark-as-viewed call that may produce
// post_seen events.
type postSeenBatch struct {
	viewer     *model.User
	channelIDs []string
	// prevLastViewedAt is the viewer's LastViewedAt per channel before the
	// view; only posts created after it are newly seen.
	prevLastViewedAt map[string]int64
	// viewedUpTo is the channel LastPostAt the view moved LastViewedAt to;
	// posts created later have not been seen.
	viewedUpTo map[string]int64
}

// preparePostSeenEvents runs before LastViewedAt is updated and returns nil
// when the view must not produce post_seen events. Cheap in-memory checks
// come first: the feature flag, the viewer, the session (only the viewer
// marking their own channels as viewed counts, never an admin or plugin
// acting on their behalf) and the per viewer rate limit. Only then does it
// read the viewer's previous LastViewedAt for at most
// readReceiptsMaxChannelsPerView channels. That read uses a replica: under
// replica lag the previous value can be stale, so a post the viewer had
// already seen may be announced once more (never one they have not seen,
// since posts after the view are excluded). Marking a channel unread and
// viewing it again legitimately announces the posts after the unread point.
func (a *App) preparePostSeenEvents(rctx request.CTX, channelIDs []string, viewer *model.User) *postSeenBatch {
	if !a.readReceiptsEnabled() || len(channelIDs) == 0 || viewer == nil || viewer.IsBot || viewer.DeleteAt != 0 {
		return nil
	}
	session := rctx.Session()
	if session == nil || session.UserId != viewer.Id || session.IsBotUser() {
		return nil
	}

	limiters := a.Srv().readReceiptsLimiters()
	if limiters.emitViewer == nil || limiters.emit == nil {
		return nil
	}
	limited, _, err := limiters.emitViewer.RateLimitCtx(rctx.Context(), viewer.Id, 1)
	if err != nil || limited {
		return nil
	}

	if len(channelIDs) > readReceiptsMaxChannelsPerView {
		channelIDs = channelIDs[:readReceiptsMaxChannelsPerView]
	}
	members, err := a.Srv().Store().Channel().GetMembersByChannelIds(channelIDs, viewer.Id)
	if err != nil {
		rctx.Logger().Warn("Failed to get channel members for read receipts", mlog.String("user_id", viewer.Id), mlog.Err(err))
		return nil
	}
	prev := make(map[string]int64, len(members))
	for _, member := range members {
		if member.UserId == viewer.Id {
			prev[member.ChannelId] = member.LastViewedAt
		}
	}
	if len(prev) == 0 {
		return nil
	}

	return &postSeenBatch{
		viewer:           viewer,
		channelIDs:       slices.Clone(channelIDs),
		prevLastViewedAt: prev,
	}
}

// schedulePostSeenEvents publishes the events of batch off the request path,
// so the seen-by work never delays or throttles the channel view itself.
// viewedUpTo is the result of UpdateLastViewedAt.
func (a *App) schedulePostSeenEvents(rctx request.CTX, batch *postSeenBatch, viewedUpTo map[string]int64) {
	if batch == nil {
		return
	}
	batch.viewedUpTo = maps.Clone(viewedUpTo)
	bgCtx := rctx.WithContext(context.WithoutCancel(rctx.Context()))
	a.Srv().Go(func() {
		for _, event := range a.postSeenEvents(bgCtx, batch) {
			a.Publish(event)
		}
	})
}

// postSeenEvents builds the post_seen events for batch: one per newly seen
// post (created after the viewer's previous LastViewedAt and at or before
// the time the view moved it to), sent to the post author alone and carrying
// only {post_id, user_id}. It is bounded by readReceiptsMaxChannelsPerView,
// readReceiptsMaxPostsPerView and a per (viewer, channel) rate limit.
func (a *App) postSeenEvents(rctx request.CTX, batch *postSeenBatch) []*model.WebSocketEvent {
	if batch == nil || !a.readReceiptsEnabled() {
		return nil
	}
	viewer := batch.viewer
	limiter := a.Srv().readReceiptsLimiters().emit
	if limiter == nil {
		return nil
	}

	var events []*model.WebSocketEvent
	for _, channelID := range batch.channelIDs {
		prevViewedAt, ok := batch.prevLastViewedAt[channelID]
		if !ok {
			continue
		}
		viewedUpTo, ok := batch.viewedUpTo[channelID]
		if !ok || viewedUpTo <= prevViewedAt {
			continue
		}

		limited, _, err := limiter.RateLimitCtx(rctx.Context(), viewer.Id+":"+channelID, 1)
		if err != nil || limited {
			continue
		}

		channel, appErr := a.GetChannel(rctx, channelID)
		if appErr != nil {
			continue
		}
		inScope, appErr := a.readReceiptsChannelInScope(rctx, channel)
		if appErr != nil || !inScope {
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

		for _, postID := range postList.Order {
			post := postList.Posts[postID]
			if post == nil || post.CreateAt <= prevViewedAt || post.CreateAt > viewedUpTo {
				continue
			}
			if post.UserId == viewer.Id || post.DeleteAt != 0 || postHasNoReadReceipts(post) || !members[post.UserId] {
				continue
			}
			message := model.NewWebSocketEvent(model.WebsocketEventPostSeen, "", "", post.UserId, nil, "")
			message.Add("post_id", post.Id)
			message.Add("user_id", viewer.Id)
			events = append(events, message)
		}
	}
	return events
}
