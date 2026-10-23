// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/imports"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestChannelPostingPolicyApplicationPaths(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	_, addErr := th.App.AddChannelMember(th.Context, th.BasicUser2.Id, th.BasicChannel, ChannelMemberOpts{})
	require.Nil(t, addErr)

	staleChannel := th.BasicChannel.DeepCopy()
	channel, err := th.App.PatchChannel(th.Context, th.BasicChannel, &model.ChannelPatch{
		PostingPolicy: new(model.ChannelPostingPolicyAdmins),
	}, th.BasicUser.Id)
	require.Nil(t, err)

	t.Run("stale channel cannot bypass policy", func(t *testing.T) {
		_, _, err := th.App.CreatePost(th.Context, &model.Post{
			ChannelId: channel.Id, UserId: th.BasicUser2.Id, Message: "Blocked integration post",
		}, staleChannel, model.CreatePostFlags{})
		require.NotNil(t, err)
		require.Equal(t, http.StatusForbidden, err.StatusCode)
	})

	t.Run("system membership messages remain available", func(t *testing.T) {
		post, _, err := th.App.CreatePost(th.Context, &model.Post{
			ChannelId: channel.Id, UserId: th.BasicUser2.Id, Type: model.PostTypeJoinChannel, Message: "System membership event",
		}, channel, model.CreatePostFlags{})
		require.Nil(t, err)
		require.NotEmpty(t, post.Id)
	})

	t.Run("scheduled job rechecks changed policy", func(t *testing.T) {
		code, err := th.App.canPostScheduledPost(th.Context, &model.ScheduledPost{
			Draft: model.Draft{ChannelId: channel.Id, UserId: th.BasicUser2.Id, Message: "Previously scheduled"},
		}, channel)
		require.NoError(t, err)
		require.Equal(t, model.ScheduledPostErrorCodeNoChannelPermission, code)
	})

	t.Run("selected user must still belong to channel", func(t *testing.T) {
		selected := model.StringArray{th.BasicUser2.Id}
		channel, err = th.App.PatchChannel(th.Context, channel, &model.ChannelPatch{
			PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: &selected,
		}, th.BasicUser.Id)
		require.Nil(t, err)
		require.Nil(t, th.App.LeaveChannel(th.Context, channel.Id, th.BasicUser2.Id))
		err = th.App.CheckChannelPostingPermission(th.Context, channel.Id, th.BasicUser2.Id)
		require.NotNil(t, err)
		require.Equal(t, http.StatusForbidden, err.StatusCode)
	})
}

type stalePolicyChannelStore struct {
	store.ChannelStore
	channel *model.Channel
}

func (s *stalePolicyChannelStore) Get(id string, allowFromCache bool) (*model.Channel, error) {
	if id == s.channel.Id {
		return s.channel.DeepCopy(), nil
	}
	return s.ChannelStore.Get(id, allowFromCache)
}

func TestChannelPolicyUsesAuthoritativeState(t *testing.T) {
	th := Setup(t).InitBasic(t)
	stale := th.BasicChannel.DeepCopy()
	channel, err := th.App.PatchChannel(th.Context, th.BasicChannel, &model.ChannelPatch{
		PostingPolicy: new(model.ChannelPostingPolicyAdmins), Mandatory: new(true),
	}, th.BasicUser.Id)
	require.Nil(t, err)
	original := th.App.Srv().Store()
	th.App.Srv().SetStore(&mandatoryFailingStore{Store: original, channel: &stalePolicyChannelStore{ChannelStore: original.Channel(), channel: stale}})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })

	err = th.App.CheckChannelPostingPermission(th.Context, stale.Id, th.BasicUser2.Id)
	require.NotNil(t, err, "replica or cache lag must not bypass a newly enabled policy")
	require.Equal(t, http.StatusForbidden, err.StatusCode)
	err = th.App.LeaveChannel(th.Context, channel.Id, th.BasicUser2.Id)
	require.NotNil(t, err, "replica or cache lag must not allow leaving a mandatory channel")
	require.Equal(t, http.StatusForbidden, err.StatusCode)

	updated, err := th.App.PatchChannel(th.Context, stale, &model.ChannelPatch{Header: new("Updated header")}, th.BasicUser2.Id)
	require.Nil(t, err)
	require.Equal(t, model.ChannelPostingPolicyAdmins, updated.PostingPolicy, "ordinary edits must preserve the primary's posting policy")
	require.True(t, updated.Mandatory)
	stale.Purpose = "Updated purpose"
	updated, err = th.App.UpdateChannel(th.Context, stale)
	require.Nil(t, err)
	require.Equal(t, model.ChannelPostingPolicyAdmins, updated.PostingPolicy)
	require.True(t, updated.Mandatory)
}

type mandatoryFailingUserChannelStore struct {
	store.ChannelStore
	userID string
}

func (s *mandatoryFailingUserChannelStore) SaveMember(rctx request.CTX, member *model.ChannelMember) (*model.ChannelMember, error) {
	if member.UserId == s.userID {
		return nil, errors.New("mandatory enrollment unavailable")
	}
	return s.ChannelStore.SaveMember(rctx, member)
}

func TestMandatoryChannelLifecycleFailureAndRetry(t *testing.T) {
	t.Run("creation can be retried with the same name", func(t *testing.T) {
		th := Setup(t).InitBasic(t)
		original := th.App.Srv().Store()
		failing := &mandatoryFailingUserChannelStore{ChannelStore: original.Channel(), userID: th.BasicUser2.Id}
		th.App.Srv().SetStore(&mandatoryFailingStore{Store: original, channel: failing})
		t.Cleanup(func() { th.App.Srv().SetStore(original) })
		name := "mandatory-" + model.NewId()
		create := func() (*model.Channel, *model.AppError) {
			return th.App.CreateChannelWithUser(th.Context, &model.Channel{
				TeamId: th.BasicTeam.Id, Name: name, DisplayName: name, Type: model.ChannelTypeOpen, Mandatory: true,
			}, th.BasicUser.Id)
		}
		_, appErr := create()
		require.NotNil(t, appErr)
		failing.userID = ""
		channel, appErr := create()
		require.Nil(t, appErr, "failed creation must not leave a conflicting channel behind")
		_, err := original.Channel().GetMember(th.Context, channel.Id, th.BasicUser2.Id)
		require.NoError(t, err)
	})

	t.Run("restore remains archived until required enrollment succeeds", func(t *testing.T) {
		th := Setup(t).InitBasic(t)
		channel, appErr := th.App.PatchChannel(th.Context, th.BasicChannel, &model.ChannelPatch{Mandatory: new(true)}, th.BasicUser.Id)
		require.Nil(t, appErr)
		deletedAt := model.GetMillis()
		require.NoError(t, th.App.Srv().Store().Channel().SetDeleteAt(channel.Id, deletedAt, deletedAt))
		channel.DeleteAt = deletedAt
		th.App.Srv().Platform().InvalidateCacheForChannel(channel)
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)

		original := th.App.Srv().Store()
		failing := &mandatoryFailingChannelStore{ChannelStore: original.Channel(), channelID: channel.Id}
		th.App.Srv().SetStore(&mandatoryFailingStore{Store: original, channel: failing})
		t.Cleanup(func() { th.App.Srv().SetStore(original) })
		_, appErr = th.App.RestoreChannel(th.Context, channel, th.BasicUser.Id)
		require.NotNil(t, appErr)
		channel, err := original.Channel().GetChannelOfType(RequestContextWithMaster(th.Context), channel.Id, channel.Type)
		require.NoError(t, err)
		require.Equal(t, deletedAt, channel.DeleteAt)
		failing.channelID = ""
		_, appErr = th.App.RestoreChannel(th.Context, channel, th.BasicUser.Id)
		require.Nil(t, appErr)
		_, err = original.Channel().GetMember(th.Context, channel.Id, user.Id)
		require.NoError(t, err)
	})
}

func TestChannelMandatoryMembershipOnImport(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	channel, err := th.App.PatchChannel(th.Context, th.BasicChannel, &model.ChannelPatch{Mandatory: new(true)}, th.BasicUser.Id)
	require.Nil(t, err)
	user := th.CreateUser(t)
	data := []imports.UserTeamImportData{{Name: &th.BasicTeam.Name}}
	require.Nil(t, th.App.importUserTeams(th.Context, user, &data, false))
	_, err = th.App.GetChannelMember(th.Context, channel.Id, user.Id)
	require.Nil(t, err, "bulk-imported team members must join mandatory channels")
}

func TestChannelMandatoryMembershipOnMove(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	channel, err := th.App.PatchChannel(th.Context, th.BasicChannel, &model.ChannelPatch{Mandatory: new(true)}, th.BasicUser.Id)
	require.Nil(t, err)
	destination := th.CreateTeam(t)
	newMember := th.CreateUser(t)
	for _, user := range []*model.User{th.BasicUser, th.BasicUser2, newMember} {
		th.LinkUserToTeam(t, user, destination)
	}
	require.Nil(t, th.App.MoveChannel(th.Context, destination, channel, th.BasicUser))
	_, err = th.App.GetChannelMember(th.Context, channel.Id, newMember.Id)
	require.Nil(t, err, "moving a mandatory channel enrolls members of the destination team")
}
