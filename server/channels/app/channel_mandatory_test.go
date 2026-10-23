// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestSyncMandatoryChannelMembers(t *testing.T) {
	th := Setup(t).InitBasic(t)
	channel := th.CreatePrivateChannel(t, th.BasicTeam)
	channel.Mandatory = true
	channel.DisableJoinLeaveMessages = true
	_, err := th.App.Srv().Store().Channel().Update(th.Context, channel)
	require.NoError(t, err)

	// More than one page, with a guest and inactive membership on the team.
	userIDs := []string{th.BasicUser.Id, th.BasicUser2.Id}
	for range 101 {
		id := model.NewId()
		user, saveErr := th.App.Srv().Store().User().Save(th.Context, &model.User{
			Username: "mandatory_" + id, Email: id + "@example.com", Roles: model.SystemUserRoleId,
		})
		require.NoError(t, saveErr)
		_, saveErr = th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
			TeamId: th.BasicTeam.Id, UserId: user.Id, SchemeUser: true,
		}, 1000)
		require.NoError(t, saveErr)
		userIDs = append(userIDs, user.Id)
	}
	guest := th.CreateGuest(t)
	_, err = th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
		TeamId: th.BasicTeam.Id, UserId: guest.Id, SchemeGuest: true,
	}, 1000)
	require.NoError(t, err)
	userIDs = append(userIDs, guest.Id)

	deleted := th.CreateUser(t)
	_, err = th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
		TeamId: th.BasicTeam.Id, UserId: deleted.Id, SchemeUser: true,
	}, 1000)
	require.NoError(t, err)
	deleted.DeleteAt = model.GetMillis()
	_, err = th.App.Srv().Store().User().Update(th.Context, deleted, true)
	require.NoError(t, err)

	left := th.CreateUser(t)
	_, err = th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
		TeamId: th.BasicTeam.Id, UserId: left.Id, SchemeUser: true,
	}, 1000)
	require.NoError(t, err)
	require.NoError(t, th.App.Srv().Store().Team().RemoveMember(th.Context, th.BasicTeam.Id, left.Id))

	require.Nil(t, th.App.SyncMandatoryChannelMembers(th.Context, channel))
	require.Nil(t, th.App.SyncMandatoryChannelMembers(th.Context, channel), "repeated enrollment must be idempotent")
	for _, userID := range userIDs {
		_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, userID)
		require.NoError(t, err)
	}
	member, err := th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, guest.Id)
	require.NoError(t, err)
	require.True(t, member.SchemeGuest)
	for _, user := range []*model.User{deleted, left} {
		_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
		var notFound *store.ErrNotFound
		require.ErrorAs(t, err, &notFound)
	}
}

func TestJoinUserToTeamMandatoryChannels(t *testing.T) {
	th := Setup(t).InitBasic(t)
	public := th.CreateChannel(t, th.BasicTeam)
	private := th.CreatePrivateChannel(t, th.BasicTeam)
	archived := th.CreateChannel(t, th.BasicTeam)
	optional := th.CreateChannel(t, th.BasicTeam)
	for _, channel := range []*model.Channel{public, private, archived} {
		channel.Mandatory = true
		_, err := th.App.Srv().Store().Channel().Update(th.Context, channel)
		require.NoError(t, err)
	}
	require.NoError(t, th.App.Srv().Store().Channel().SetDeleteAt(archived.Id, model.GetMillis(), model.GetMillis()))

	for _, user := range []*model.User{th.CreateUser(t), th.CreateGuest(t)} {
		_, appErr := th.App.JoinUserToTeam(th.Context, th.BasicTeam, user, "")
		require.Nil(t, appErr)
		for _, channel := range []*model.Channel{public, private} {
			member, err := th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
			require.NoError(t, err)
			require.Equal(t, user.IsGuest(), member.SchemeGuest)
		}
		for _, channel := range []*model.Channel{archived, optional} {
			_, err := th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
			var notFound *store.ErrNotFound
			require.ErrorAs(t, err, &notFound)
		}
		// Simulate an interrupted earlier enrollment, then retry the team join.
		require.NoError(t, th.App.Srv().Store().Channel().RemoveMember(th.Context, private.Id, user.Id))
		_, appErr = th.App.JoinUserToTeam(th.Context, th.BasicTeam, user, "")
		require.Nil(t, appErr)
		_, err := th.App.Srv().Store().Channel().GetMember(th.Context, private.Id, user.Id)
		require.NoError(t, err)
	}
}

type mandatoryFailingChannelStore struct {
	store.ChannelStore
	channelID string
}

func (s *mandatoryFailingChannelStore) SaveMember(rctx request.CTX, member *model.ChannelMember) (*model.ChannelMember, error) {
	if member.ChannelId == s.channelID {
		return nil, errors.New("mandatory enrollment unavailable")
	}
	return s.ChannelStore.SaveMember(rctx, member)
}

type mandatoryFailingStore struct {
	store.Store
	channel store.ChannelStore
}

func (s *mandatoryFailingStore) Channel() store.ChannelStore { return s.channel }

func TestMandatoryChannelEnrollmentFailureAndRetry(t *testing.T) {
	th := Setup(t).InitBasic(t)
	channel := th.CreatePrivateChannel(t, th.BasicTeam)
	channel.Mandatory = true
	_, err := th.App.Srv().Store().Channel().Update(th.Context, channel)
	require.NoError(t, err)
	user := th.CreateUser(t)
	original := th.App.Srv().Store()
	failing := &mandatoryFailingChannelStore{ChannelStore: original.Channel(), channelID: channel.Id}
	th.App.Srv().SetStore(&mandatoryFailingStore{Store: original, channel: failing})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })

	_, appErr := th.App.JoinUserToTeam(th.Context, th.BasicTeam, user, "")
	require.NotNil(t, appErr, "a successful team join must include required channel enrollment")
	require.Nil(t, th.App.JoinMandatoryChannels(th.Context, model.NewId(), user), "teams without channels have no required enrollment")
	_, appErr = th.App.JoinUserToTeam(th.Context, th.BasicTeam, user, "")
	require.NotNil(t, appErr, "existing team membership must still report enrollment failures")
	require.NotNil(t, th.App.SyncMandatoryChannelMembers(th.Context, channel), "backfill failures must be returned")

	failing.channelID = ""
	_, appErr = th.App.JoinUserToTeam(th.Context, th.BasicTeam, user, "")
	require.Nil(t, appErr)
	_, err = original.Channel().GetMember(th.Context, channel.Id, user.Id)
	require.NoError(t, err)
}
