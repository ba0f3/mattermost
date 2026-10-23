// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func TestOAuthActivationJoinsMandatoryChannels(t *testing.T) {
	for _, guest := range []bool{false, true} {
		name := "member"
		if guest {
			name = "guest"
		}
		t.Run(name, func(t *testing.T) {
			th := Setup(t).InitBasic(t)
			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Enable = true })
			id := model.NewId()
			user, oauthUser := createGitlabUser(t, th.App, th.Context, time.Now().UnixNano(), "oauth_"+id, id+"@example.com")
			if guest {
				user.Roles = model.SystemGuestRoleId
			}
			user.DeleteAt = model.GetMillis()
			_, err := th.App.Srv().Store().User().Update(th.Context, user, true)
			require.NoError(t, err)
			th.App.InvalidateCacheForUser(user.Id)

			secondTeam := th.CreateTeam(t)
			leftTeam := th.CreateTeam(t)
			for _, team := range []*model.Team{th.BasicTeam, secondTeam, leftTeam} {
				_, err = th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
					TeamId: team.Id, UserId: user.Id, SchemeUser: !guest, SchemeGuest: guest,
				}, 1000)
				require.NoError(t, err)
			}
			require.NoError(t, th.App.Srv().Store().Team().RemoveMember(th.Context, leftTeam.Id, user.Id))

			public := th.CreateChannel(t, th.BasicTeam)
			private := th.CreatePrivateChannel(t, secondTeam)
			left := th.CreateChannel(t, leftTeam)
			archived := th.CreateChannel(t, th.BasicTeam)
			for _, channel := range []*model.Channel{public, private, left, archived} {
				channel.Mandatory = true
				channel.DisableJoinLeaveMessages = true
				_, err = th.App.Srv().Store().Channel().Update(th.Context, channel)
				require.NoError(t, err)
				require.Nil(t, th.App.SyncMandatoryChannelMembers(th.Context, channel))
				_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
				var notFound *store.ErrNotFound
				require.ErrorAs(t, err, &notFound, "inactive users must be skipped during backfill")
			}
			require.NoError(t, th.App.Srv().Store().Channel().SetDeleteAt(archived.Id, model.GetMillis(), model.GetMillis()))

			payload := getGitlabUserPayload(oauthUser, t)
			require.Nil(t, th.App.UpdateOAuthUserAttrs(th.Context, bytes.NewReader(payload), user, einterfaces.GetOAuthProvider("gitlab"), "gitlab", nil))
			active, err := th.App.Srv().Store().User().Get(th.Context, user.Id)
			require.NoError(t, err)
			require.Zero(t, active.DeleteAt)
			for _, channel := range []*model.Channel{public, private} {
				member, err := th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
				require.NoError(t, err, "OAuth activation must enroll existing team members")
				require.Equal(t, guest, member.SchemeGuest)
			}
			for _, channel := range []*model.Channel{left, archived} {
				_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
				var notFound *store.ErrNotFound
				require.ErrorAs(t, err, &notFound)
			}
		})
	}
}

func TestOAuthActivationMandatoryEnrollmentRetry(t *testing.T) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Enable = true })
	id := model.NewId()
	user, oauthUser := createGitlabUser(t, th.App, th.Context, time.Now().UnixNano(), "oauth_"+id, id+"@example.com")
	_, err := th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
		TeamId: th.BasicTeam.Id, UserId: user.Id, SchemeUser: true,
	}, 1000)
	require.NoError(t, err)
	user.DeleteAt = model.GetMillis()
	_, err = th.App.Srv().Store().User().Update(th.Context, user, true)
	require.NoError(t, err)
	th.App.InvalidateCacheForUser(user.Id)

	channel := th.CreatePrivateChannel(t, th.BasicTeam)
	channel.Mandatory = true
	_, err = th.App.Srv().Store().Channel().Update(th.Context, channel)
	require.NoError(t, err)

	original := th.App.Srv().Store()
	failing := &mandatoryFailingChannelStore{ChannelStore: original.Channel(), channelID: channel.Id}
	th.App.Srv().SetStore(&mandatoryFailingStore{Store: original, channel: failing})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })

	payload := getGitlabUserPayload(oauthUser, t)
	provider := einterfaces.GetOAuthProvider("gitlab")
	require.NotNil(t, th.App.UpdateOAuthUserAttrs(th.Context, bytes.NewReader(payload), user, provider, "gitlab", nil), "enrollment failure must fail the login")
	active, err := original.User().Get(th.Context, user.Id)
	require.NoError(t, err)
	require.Zero(t, active.DeleteAt, "activation is already persisted before enrollment")

	failing.channelID = ""
	for range 2 {
		require.Nil(t, th.App.UpdateOAuthUserAttrs(th.Context, bytes.NewReader(payload), active, provider, "gitlab", nil), "a later login must repair enrollment even without profile changes")
		_, err = original.Channel().GetMember(th.Context, channel.Id, user.Id)
		require.NoError(t, err)
	}
}

func TestMandatoryEnrollmentOnExplicitActivationOnly(t *testing.T) {
	th := Setup(t).InitBasic(t)
	user := th.CreateUser(t)
	_, err := th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{
		TeamId: th.BasicTeam.Id, UserId: user.Id, SchemeUser: true,
	}, 1000)
	require.NoError(t, err)
	channel := th.CreatePrivateChannel(t, th.BasicTeam)
	channel.Mandatory = true
	_, err = th.App.Srv().Store().Channel().Update(th.Context, channel)
	require.NoError(t, err)

	user.Nickname = "Updated profile"
	_, appErr := th.App.UpdateUser(th.Context, user, false)
	require.Nil(t, appErr)
	_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
	var notFound *store.ErrNotFound
	require.ErrorAs(t, err, &notFound, "ordinary profile updates should not reconcile membership")

	_, appErr = th.App.UpdateActive(th.Context, user, true)
	require.Nil(t, appErr)
	_, err = th.App.Srv().Store().Channel().GetMember(th.Context, channel.Id, user.Id)
	require.NoError(t, err)
}
