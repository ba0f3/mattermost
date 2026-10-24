// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

func setupReadReceipts(t *testing.T) *TestHelper {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.ServiceSettings.EnableReadReceipts = new(true)
		cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(model.ServiceSettingsDefaultReadReceiptsMaxChannelMembers)
	})
	return th
}

func readReceiptsView(t *testing.T, th *TestHelper, userID, channelID string) {
	t.Helper()
	_, appErr := th.App.MarkChannelsAsViewed(th.Context, []string{channelID}, userID, "", false, false)
	require.Nil(t, appErr)
}

func readReceiptsOptOut(t *testing.T, th *TestHelper, userID string) {
	t.Helper()
	appErr := th.App.UpdatePreferences(th.Context, userID, model.Preferences{{
		UserId:   userID,
		Category: model.PreferenceCategoryReadReceipts,
		Name:     model.PreferenceNameReadReceiptsOptOut,
		Value:    "true",
	}})
	require.Nil(t, appErr)
}

func loggedInClient(t *testing.T, th *TestHelper, user *model.User) *model.Client4 {
	t.Helper()
	client := th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return client
}

func requireAppErrorID(t *testing.T, err error, status int, id string) *model.AppError {
	t.Helper()
	require.Error(t, err)
	var appErr *model.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, status, appErr.StatusCode)
	require.Equal(t, id, appErr.Id)
	return appErr
}

func TestGetPostSeenBy(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	author := th.BasicUser
	viewer := th.BasicUser2
	channel := th.BasicChannel

	idle := th.CreateUser(t)
	th.LinkUserToTeam(t, idle, th.BasicTeam)
	th.AddUserToChannel(t, idle, channel)

	leaver := th.CreateUser(t)
	th.LinkUserToTeam(t, leaver, th.BasicTeam)
	th.AddUserToChannel(t, leaver, channel)

	deactivated := th.CreateUser(t)
	th.LinkUserToTeam(t, deactivated, th.BasicTeam)
	th.AddUserToChannel(t, deactivated, channel)

	optedOut := th.CreateUser(t)
	th.LinkUserToTeam(t, optedOut, th.BasicTeam)
	th.AddUserToChannel(t, optedOut, channel)

	th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableBotAccountCreation = new(true) })
	bot := th.CreateBotWithSystemAdminClient(t)
	botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	th.LinkUserToTeam(t, botUser, th.BasicTeam)
	th.AddUserToChannel(t, botUser, channel)

	post := th.CreatePostWithClient(t, th.Client, channel)

	t.Run("nobody has seen it yet", func(t *testing.T) {
		seen, _, err := th.Client.GetPostSeenBy(ctx, post.Id)
		require.NoError(t, err)
		require.NotNil(t, seen.UserIds)
		require.Empty(t, seen.UserIds)
	})

	for _, u := range []*model.User{viewer, leaver, deactivated, optedOut, botUser} {
		readReceiptsView(t, th, u.Id, channel.Id)
	}
	th.RemoveUserFromChannel(t, leaver, channel)
	th.UpdateActiveUser(t, deactivated, false)
	readReceiptsOptOut(t, th, optedOut.Id)

	t.Run("lists only eligible current members, never the author", func(t *testing.T) {
		seen, _, err := th.Client.GetPostSeenBy(ctx, post.Id)
		require.NoError(t, err)
		require.Equal(t, []string{viewer.Id}, seen.UserIds)
		for _, absent := range []string{author.Id, idle.Id, leaver.Id, deactivated.Id, optedOut.Id, botUser.Id} {
			assert.NotContains(t, seen.UserIds, absent)
		}
	})

	t.Run("response carries only user_ids", func(t *testing.T) {
		resp, err := th.Client.DoAPIGet(ctx, "/posts/"+post.Id+"/seen", "")
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &raw))
		require.Len(t, raw, 1)
		require.Contains(t, raw, "user_ids")
		require.NotContains(t, strings.ToLower(string(body)), "viewed")
		require.NotContains(t, strings.ToLower(string(body)), "_at")
	})

	t.Run("non-author gets the same 404 as a missing post", func(t *testing.T) {
		viewerClient := loggedInClient(t, th, viewer)
		_, resp, err := viewerClient.GetPostSeenBy(ctx, post.Id)
		CheckNotFoundStatus(t, resp)
		notAuthor := requireAppErrorID(t, err, http.StatusNotFound, "api.post.seen.not_found.app_error")

		_, resp, err = viewerClient.GetPostSeenBy(ctx, model.NewId())
		CheckNotFoundStatus(t, resp)
		missing := requireAppErrorID(t, err, http.StatusNotFound, "api.post.seen.not_found.app_error")

		require.Equal(t, missing.Message, notAuthor.Message)
		require.Equal(t, missing.DetailedError, notAuthor.DetailedError)
		require.Equal(t, missing.Where, notAuthor.Where)
	})

	t.Run("opted-out non-author still gets 404, not opted_out", func(t *testing.T) {
		optedOutClient := loggedInClient(t, th, optedOut)
		_, resp, err := optedOutClient.GetPostSeenBy(ctx, post.Id)
		CheckNotFoundStatus(t, resp)
		requireAppErrorID(t, err, http.StatusNotFound, "api.post.seen.not_found.app_error")
	})

	t.Run("invalid post id", func(t *testing.T) {
		_, resp, err := th.Client.GetPostSeenBy(ctx, "junk")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		_, resp, err := th.CreateClient().GetPostSeenBy(ctx, post.Id)
		require.Error(t, err)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestGetPostSeenByAuthorLeft(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	channel := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, th.BasicUser2, channel)
	post := th.CreatePostWithClient(t, th.Client, channel)
	readReceiptsView(t, th, th.BasicUser2.Id, channel.Id)

	seen, _, err := th.Client.GetPostSeenBy(ctx, post.Id)
	require.NoError(t, err)
	require.Equal(t, []string{th.BasicUser2.Id}, seen.UserIds)

	_, err = th.Client.RemoveUserFromChannel(ctx, channel.Id, th.BasicUser.Id)
	require.NoError(t, err)

	_, resp, err := th.Client.GetPostSeenBy(ctx, post.Id)
	CheckNotFoundStatus(t, resp)
	requireAppErrorID(t, err, http.StatusNotFound, "api.post.seen.not_found.app_error")

	seenStatus, _, err := th.Client.GetPostsSeenStatus(ctx, []string{post.Id})
	require.NoError(t, err)
	require.Empty(t, seenStatus.Seen)
}

func TestGetPostSeenByGates(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	post := th.CreatePostWithClient(t, th.Client, th.BasicChannel)

	t.Run("disabled returns 501", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableReadReceipts = new(false) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableReadReceipts = new(true) })

		_, resp, err := th.Client.GetPostSeenBy(ctx, post.Id)
		CheckNotImplementedStatus(t, resp)
		requireAppErrorID(t, err, http.StatusNotImplemented, "api.post.seen.disabled.app_error")

		_, resp, err = th.Client.GetPostsSeenStatus(ctx, []string{post.Id})
		CheckNotImplementedStatus(t, resp)
		requireAppErrorID(t, err, http.StatusNotImplemented, "api.post.seen.disabled.app_error")
	})

	t.Run("channel over the member limit returns 403 out_of_scope", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(1) })
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.ReadReceiptsMaxChannelMembers = new(model.ServiceSettingsDefaultReadReceiptsMaxChannelMembers)
		})

		_, resp, err := th.Client.GetPostSeenBy(ctx, post.Id)
		CheckForbiddenStatus(t, resp)
		requireAppErrorID(t, err, http.StatusForbidden, "api.post.seen.out_of_scope.app_error")

		// The scope check comes after the author check.
		_, resp, err = loggedInClient(t, th, th.BasicUser2).GetPostSeenBy(ctx, post.Id)
		CheckNotFoundStatus(t, resp)
		requireAppErrorID(t, err, http.StatusNotFound, "api.post.seen.not_found.app_error")

		seenStatus, _, err := th.Client.GetPostsSeenStatus(ctx, []string{post.Id})
		require.NoError(t, err)
		require.Empty(t, seenStatus.Seen)
	})

	t.Run("opted-out author gets 403 opted_out on both endpoints", func(t *testing.T) {
		readReceiptsOptOut(t, th, th.BasicUser.Id)
		defer func() {
			appErr := th.App.DeletePreferences(th.Context, th.BasicUser.Id, model.Preferences{{
				UserId:   th.BasicUser.Id,
				Category: model.PreferenceCategoryReadReceipts,
				Name:     model.PreferenceNameReadReceiptsOptOut,
			}})
			require.Nil(t, appErr)
		}()

		_, resp, err := th.Client.GetPostSeenBy(ctx, post.Id)
		CheckForbiddenStatus(t, resp)
		requireAppErrorID(t, err, http.StatusForbidden, "api.post.seen.opted_out.app_error")

		_, resp, err = th.Client.GetPostsSeenStatus(ctx, []string{post.Id})
		CheckForbiddenStatus(t, resp)
		requireAppErrorID(t, err, http.StatusForbidden, "api.post.seen.opted_out.app_error")
	})

	t.Run("works again after opting back in", func(t *testing.T) {
		_, _, err := th.Client.GetPostSeenBy(ctx, post.Id)
		require.NoError(t, err)
	})
}

func TestGetPostsSeenStatus(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	other := th.BasicUser2
	otherClient := loggedInClient(t, th, other)

	dm := th.CreateDmChannel(t, other)
	othersDMPost := th.CreatePostWithClient(t, otherClient, dm)
	othersChannelPost := th.CreatePostWithClient(t, otherClient, th.BasicChannel)

	dmSeen := th.CreatePostWithClient(t, th.Client, dm)
	readReceiptsView(t, th, other.Id, dm.Id)
	dmUnseen := th.CreatePostWithClient(t, th.Client, dm)
	channelPost := th.CreatePostWithClient(t, th.Client, th.BasicChannel)

	t.Run("reports own posts and silently omits everything else", func(t *testing.T) {
		ids := []string{dmSeen.Id, dmUnseen.Id, othersDMPost.Id, channelPost.Id, othersChannelPost.Id, model.NewId(), dmSeen.Id}
		status, _, err := th.Client.GetPostsSeenStatus(ctx, ids)
		require.NoError(t, err)
		require.Equal(t, map[string]bool{
			dmSeen.Id:      true,
			dmUnseen.Id:    false,
			channelPost.Id: false,
		}, status.Seen)
	})

	t.Run("non-author IDs are omitted for the other user too", func(t *testing.T) {
		status, _, err := otherClient.GetPostsSeenStatus(ctx, []string{dmSeen.Id, dmUnseen.Id, channelPost.Id, othersDMPost.Id})
		require.NoError(t, err)
		require.Len(t, status.Seen, 1)
		require.Contains(t, status.Seen, othersDMPost.Id)
	})

	t.Run("seen after the other member views", func(t *testing.T) {
		readReceiptsView(t, th, other.Id, th.BasicChannel.Id)
		status, _, err := th.Client.GetPostsSeenStatus(ctx, []string{channelPost.Id, dmUnseen.Id})
		require.NoError(t, err)
		require.Equal(t, map[string]bool{channelPost.Id: true, dmUnseen.Id: false}, status.Seen)

		seen, _, err := th.Client.GetPostSeenBy(ctx, dmSeen.Id)
		require.NoError(t, err)
		require.Equal(t, []string{other.Id}, seen.UserIds)
	})

	t.Run("response carries only seen", func(t *testing.T) {
		resp, err := th.Client.DoAPIPost(ctx, "/posts/seen_status", `{"post_ids":["`+dmSeen.Id+`"]}`)
		require.NoError(t, err)
		defer resp.Body.Close()
		var raw map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
		require.Len(t, raw, 1)
		require.JSONEq(t, `{"`+dmSeen.Id+`":true}`, string(raw["seen"]))
	})

	t.Run("empty list", func(t *testing.T) {
		status, _, err := th.Client.GetPostsSeenStatus(ctx, []string{})
		require.NoError(t, err)
		require.NotNil(t, status.Seen)
		require.Empty(t, status.Seen)
	})

	t.Run("too many ids", func(t *testing.T) {
		ids := make([]string, app.ReadReceiptsMaxSeenStatusPosts+1)
		for i := range ids {
			ids[i] = model.NewId()
		}
		_, resp, err := th.Client.GetPostsSeenStatus(ctx, ids)
		CheckBadRequestStatus(t, resp)
		requireAppErrorID(t, err, http.StatusBadRequest, "api.post.seen_status.too_many_ids.app_error")

		_, _, err = th.Client.GetPostsSeenStatus(ctx, ids[:app.ReadReceiptsMaxSeenStatusPosts])
		require.NoError(t, err)
	})

	t.Run("invalid id", func(t *testing.T) {
		_, resp, err := th.Client.GetPostsSeenStatus(ctx, []string{"junk"})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("bad body", func(t *testing.T) {
		resp, err := th.Client.DoAPIPost(ctx, "/posts/seen_status", "not json")
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestReadReceiptsRateLimit(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	post := th.CreatePostWithClient(t, th.Client, th.BasicChannel)

	limited := false
	for range 100 {
		_, resp, err := th.Client.GetPostSeenBy(ctx, post.Id)
		if err != nil {
			require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
			limited = true
			break
		}
	}
	require.True(t, limited, "GET /posts/{post_id}/seen must be rate limited")

	// The batch endpoint shares the per-user budget.
	_, resp, err := th.Client.GetPostsSeenStatus(ctx, []string{post.Id})
	require.Error(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Other users are not affected.
	_, _, err = loggedInClient(t, th, th.BasicUser2).GetPostsSeenStatus(ctx, []string{post.Id})
	require.NoError(t, err)

	// Viewing the channel is never throttled by read receipts.
	for range 50 {
		_, _, err := th.Client.ViewChannel(ctx, th.BasicUser.Id, &model.ChannelView{ChannelId: th.BasicChannel.Id})
		require.NoError(t, err)
	}
}

func TestPostSeenWebSocketEvent(t *testing.T) {
	mainHelper.Parallel(t)
	th := setupReadReceipts(t)
	ctx := context.Background()

	author := th.BasicUser
	viewer := th.BasicUser2
	// A fresh channel so the only user posts in it are the ones below.
	channel := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, viewer, channel)

	bystander := th.CreateUser(t)
	th.LinkUserToTeam(t, bystander, th.BasicTeam)
	th.AddUserToChannel(t, bystander, channel)

	viewerClient := loggedInClient(t, th, viewer)
	bystanderClient := loggedInClient(t, th, bystander)

	authorWS := th.CreateConnectedWebSocketClient(t)
	viewerWS := th.CreateConnectedWebSocketClientWithClient(t, viewerClient)
	bystanderWS := th.CreateConnectedWebSocketClientWithClient(t, bystanderClient)

	post := th.CreatePostWithClient(t, th.Client, channel)

	_, _, err := viewerClient.ViewChannel(ctx, viewer.Id, &model.ChannelView{ChannelId: channel.Id})
	require.NoError(t, err)

	timeout := time.After(10 * time.Second)
	var got *model.WebSocketEvent
	for got == nil {
		select {
		case event := <-authorWS.EventChannel:
			if event.EventType() == model.WebsocketEventPostSeen && event.GetData()["post_id"] == post.Id {
				got = event
			}
		case <-timeout:
			require.FailNow(t, "author did not receive post_seen")
		}
	}
	data := got.GetData()
	require.Equal(t, post.Id, data["post_id"])
	require.Equal(t, viewer.Id, data["user_id"])
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	require.Equal(t, []string{"post_id", "user_id"}, keys)
	require.Equal(t, author.Id, got.GetBroadcast().UserId)

	noPostSeen := func(name string, ws *model.WebSocketClient) {
		deadline := time.After(2 * time.Second)
		for {
			select {
			case event := <-ws.EventChannel:
				require.NotEqual(t, model.WebsocketEventPostSeen, event.EventType(), "%s must not receive post_seen", name)
			case <-deadline:
				return
			}
		}
	}
	noPostSeen("viewer", viewerWS)
	noPostSeen("bystander", bystanderWS)

	t.Run("opted-out viewer emits nothing and opted-out author receives nothing", func(t *testing.T) {
		th.CreatePostWithClient(t, th.Client, channel)
		readReceiptsOptOut(t, th, bystander.Id)
		_, _, err := bystanderClient.ViewChannel(ctx, bystander.Id, &model.ChannelView{ChannelId: channel.Id})
		require.NoError(t, err)
		noPostSeen("author (opted-out viewer)", authorWS)

		readReceiptsOptOut(t, th, author.Id)
		th.CreatePostWithClient(t, th.Client, channel)
		_, _, err = viewerClient.ViewChannel(ctx, viewer.Id, &model.ChannelView{ChannelId: channel.Id})
		require.NoError(t, err)
		noPostSeen("author (opted out)", authorWS)
	})
}
