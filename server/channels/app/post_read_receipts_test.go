// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func setupPostSeenEvents(t *testing.T) *TestHelper {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.ServiceSettings.EnableReadReceipts = new(true)
		cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(model.ServiceSettingsDefaultReadReceiptsMaxChannelMembers)
	})
	return th
}

// sessionFor returns a request context whose session belongs to user, as for
// a request the user makes themselves.
func sessionFor(th *TestHelper, user *model.User) request.CTX {
	session := &model.Session{Id: model.NewId(), UserId: user.Id}
	if user.IsBot {
		session.AddProp(model.SessionPropIsBot, model.SessionPropIsBotValue)
	}
	return th.Context.WithSession(session)
}

// viewForPostSeen performs the post_seen part of MarkChannelsAsViewed
// synchronously and returns the events that would be published.
func viewForPostSeen(t *testing.T, th *TestHelper, rctx request.CTX, viewer *model.User, channelIDs ...string) []*model.WebSocketEvent {
	t.Helper()
	_, err := th.App.Srv().Store().Channel().UpdateLastViewedAt(channelIDs, viewer.Id)
	require.NoError(t, err)
	return th.App.postSeenEvents(rctx, th.App.advanceReadReceiptSeenUpTo(rctx, channelIDs, viewer))
}

// newChannelPost creates a post by author with a server-assigned, strictly
// increasing CreateAt.
func newChannelPost(t *testing.T, th *TestHelper, author *model.User, channel *model.Channel, mutate ...func(*model.Post)) *model.Post {
	t.Helper()
	time.Sleep(2 * time.Millisecond)
	post := &model.Post{UserId: author.Id, ChannelId: channel.Id, Message: "message_" + model.NewId()}
	for _, m := range mutate {
		m(post)
	}
	post, _, appErr := th.App.CreatePost(th.Context, post, channel, model.CreatePostFlags{})
	require.Nil(t, appErr)
	time.Sleep(2 * time.Millisecond)
	return post
}

func seenPostIDs(events []*model.WebSocketEvent) []string {
	ids := []string{}
	for _, event := range events {
		ids = append(ids, event.GetData()["post_id"].(string))
	}
	return ids
}

func newPostSeenChannel(t *testing.T, th *TestHelper, members ...*model.User) *model.Channel {
	t.Helper()
	channel := th.CreateChannel(t, th.BasicTeam)
	for _, member := range members {
		th.LinkUserToTeam(t, member, th.BasicTeam)
		th.AddUserToChannel(t, member, channel)
	}
	return channel
}

func TestPostSeenEvents(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupPostSeenEvents(t)

	author := th.BasicUser

	t.Run("viewer's own view emits to the author", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		post := newChannelPost(t, th, author, channel)

		events := viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id)
		require.Len(t, events, 1)
		require.Equal(t, model.WebsocketEventPostSeen, events[0].EventType())
		require.Equal(t, author.Id, events[0].GetBroadcast().UserId)
		require.Equal(t, map[string]any{"post_id": post.Id, "user_id": viewer.Id}, events[0].GetData())
	})

	t.Run("already seen posts are not emitted again", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		rctx := sessionFor(th, viewer)

		first := newChannelPost(t, th, author, channel)
		require.Equal(t, []string{first.Id}, seenPostIDs(viewForPostSeen(t, th, rctx, viewer, channel.Id)))

		second := newChannelPost(t, th, author, channel)
		require.Equal(t, []string{second.Id}, seenPostIDs(viewForPostSeen(t, th, rctx, viewer, channel.Id)))

		require.Empty(t, viewForPostSeen(t, th, rctx, viewer, channel.Id))
	})

	t.Run("feature off emits nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		newChannelPost(t, th, author, channel)

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableReadReceipts = new(false) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableReadReceipts = new(true) })
		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id))
	})

	t.Run("channel out of scope emits nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		newChannelPost(t, th, author, channel)

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(1) })
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(model.ServiceSettingsDefaultReadReceiptsMaxChannelMembers)
		})
		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id))
	})

	t.Run("bot viewer emits nothing", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		channel := newPostSeenChannel(t, th, botUser)
		newChannelPost(t, th, author, channel)

		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, botUser), botUser, channel.Id))
	})

	t.Run("bot session emits nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		newChannelPost(t, th, author, channel)

		session := &model.Session{Id: model.NewId(), UserId: viewer.Id}
		session.AddProp(model.SessionPropIsBot, model.SessionPropIsBotValue)
		require.Empty(t, viewForPostSeen(t, th, th.Context.WithSession(session), viewer, channel.Id))
	})

	t.Run("deactivated viewer emits nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		newChannelPost(t, th, author, channel)

		_, appErr := th.App.UpdateActive(th.Context, viewer, false)
		require.Nil(t, appErr)
		viewer, appErr = th.App.GetUser(th.Context, viewer.Id)
		require.Nil(t, appErr)
		require.NotZero(t, viewer.DeleteAt)
		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id))
	})

	t.Run("marking viewed on behalf of the viewer emits nothing and does not count as seen", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		post := newChannelPost(t, th, author, channel)

		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, th.SystemAdminUser), viewer, channel.Id), "admin session")
		require.Empty(t, viewForPostSeen(t, th, th.Context, viewer, channel.Id), "no session")
		seenBy, appErr := th.App.GetPostSeenBy(sessionFor(th, author), post.Id, author.Id)
		require.Nil(t, appErr)
		require.Empty(t, seenBy)

		// The view on behalf did not move the seen pointer, so the viewer's
		// own view announces every post they have now seen.
		later := newChannelPost(t, th, author, channel)
		ids := seenPostIDs(viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id))
		require.ElementsMatch(t, []string{post.Id, later.Id}, ids)
	})

	t.Run("mark as unread keeps the post seen and viewing again emits nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		rctx := sessionFor(th, viewer)
		post := newChannelPost(t, th, author, channel)
		require.Equal(t, []string{post.Id}, seenPostIDs(viewForPostSeen(t, th, rctx, viewer, channel.Id)))

		_, appErr := th.App.MarkChannelAsUnreadFromPost(rctx, post.Id, viewer.Id, false)
		require.Nil(t, appErr)
		member, appErr := th.App.GetChannelMember(th.Context, channel.Id, viewer.Id)
		require.Nil(t, appErr)
		require.Less(t, member.LastViewedAt, post.CreateAt)

		seenBy, appErr := th.App.GetPostSeenBy(sessionFor(th, author), post.Id, author.Id)
		require.Nil(t, appErr)
		require.Equal(t, []string{viewer.Id}, seenBy)

		require.Empty(t, viewForPostSeen(t, th, rctx, viewer, channel.Id))
	})

	t.Run("webhook, bot and system posts emit nothing", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		// Saved through the store: CreatePost strips integration props that
		// the caller supplies.
		for _, mutate := range []func(*model.Post){
			func(p *model.Post) { p.AddProp(model.PostPropsFromWebhook, "true") },
			func(p *model.Post) { p.AddProp(model.PostPropsFromBot, "true") },
			func(p *model.Post) { p.Type = model.PostTypeHeaderChange },
		} {
			time.Sleep(2 * time.Millisecond)
			post := &model.Post{UserId: author.Id, ChannelId: channel.Id, Message: "message_" + model.NewId()}
			mutate(post)
			_, err := th.App.Srv().Store().Post().Save(th.Context, post)
			require.NoError(t, err)
		}
		time.Sleep(2 * time.Millisecond)

		require.Empty(t, viewForPostSeen(t, th, sessionFor(th, viewer), viewer, channel.Id))
	})

	t.Run("per viewer rate limit suppresses emission", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		rctx := sessionFor(th, viewer)

		prepared := 0
		for range readReceiptsEmitViewerMaxBurst + 5 {
			newChannelPost(t, th, author, channel)
			if th.App.advanceReadReceiptSeenUpTo(rctx, []string{channel.Id}, viewer) != nil {
				prepared++
			}
		}
		require.Equal(t, readReceiptsEmitViewerMaxBurst+1, prepared)

		newChannelPost(t, th, author, channel)
		require.Empty(t, viewForPostSeen(t, th, rctx, viewer, channel.Id))
	})

	t.Run("per channel rate limit suppresses emission", func(t *testing.T) {
		viewer := th.CreateUser(t)
		channel := newPostSeenChannel(t, th, viewer)
		rctx := sessionFor(th, viewer)
		newChannelPost(t, th, author, channel)

		batch := th.App.advanceReadReceiptSeenUpTo(rctx, []string{channel.Id}, viewer)
		require.NotNil(t, batch)

		emitted := 0
		for range readReceiptsEmitMaxBurst + 5 {
			if len(th.App.postSeenEvents(rctx, batch)) > 0 {
				emitted++
			}
		}
		require.Equal(t, readReceiptsEmitMaxBurst+1, emitted)
	})
}
