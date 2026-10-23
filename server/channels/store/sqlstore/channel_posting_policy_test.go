// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func TestChannelPostingPolicyPersistence(t *testing.T) {
	StoreTestWithSqlStore(t, func(t *testing.T, rctx request.CTX, s store.Store, _ storetest.SqlStore) {
		channel, err := s.Channel().Save(rctx, &model.Channel{
			TeamId: model.NewId(), Name: model.NewId(), DisplayName: "Posting policy", Type: model.ChannelTypePrivate,
			PostingPolicy: model.ChannelPostingPolicySelected, PostingUserIDs: model.StringArray{model.NewId()}, Mandatory: true,
		}, 100)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s.Channel().PermanentDelete(rctx, channel.Id)) })

		stored, err := s.Channel().Get(channel.Id, false)
		require.NoError(t, err)
		require.Equal(t, "selected", stored.PostingPolicy)
		require.Equal(t, channel.PostingUserIDs, stored.PostingUserIDs)
		require.True(t, stored.Mandatory)

		channels, err := s.Channel().GetTeamChannels(channel.TeamId)
		require.NoError(t, err)
		require.Len(t, channels, 1)
		require.True(t, channels[0].Mandatory, "private mandatory channels must be available for team enrollment")
		mandatory, err := s.Channel().GetMandatoryChannels(channel.TeamId)
		require.NoError(t, err)
		require.Len(t, mandatory, 1)
		require.Equal(t, channel.Id, mandatory[0].Id)
		require.NoError(t, s.Channel().SetDeleteAt(channel.Id, model.GetMillis(), model.GetMillis()))
		mandatory, err = s.Channel().GetMandatoryChannels(channel.TeamId)
		require.NoError(t, err)
		require.Empty(t, mandatory, "archived mandatory channels must not enroll members")
		require.NoError(t, s.Channel().SetDeleteAt(channel.Id, 0, model.GetMillis()))

		channel.PostingPolicy = model.ChannelPostingPolicyAdmins
		channel.PostingUserIDs = nil
		channel.Mandatory = false
		_, err = s.Channel().Update(rctx, channel)
		require.NoError(t, err)
		stored, err = s.Channel().Get(channel.Id, false)
		require.NoError(t, err)
		require.Equal(t, "admins", stored.PostingPolicy)
		require.Equal(t, model.StringArray{}, stored.PostingUserIDs)
		require.False(t, stored.Mandatory)
		mandatory, err = s.Channel().GetMandatoryChannels(channel.TeamId)
		require.NoError(t, err)
		require.Empty(t, mandatory)

		legacy, err := s.Channel().Save(rctx, &model.Channel{
			TeamId: channel.TeamId, Name: model.NewId(), DisplayName: "Default posting policy", Type: model.ChannelTypeOpen,
		}, 100)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s.Channel().PermanentDelete(rctx, legacy.Id)) })
		stored, err = s.Channel().Get(legacy.Id, false)
		require.NoError(t, err)
		require.Equal(t, "all", stored.PostingPolicy)
		require.Equal(t, model.StringArray{}, stored.PostingUserIDs)
		require.False(t, stored.Mandatory)
	})
}

func TestMandatoryMembershipPaginationAfterDeparture(t *testing.T) {
	StoreTestWithSqlStore(t, func(t *testing.T, rctx request.CTX, s store.Store, _ storetest.SqlStore) {
		teamID := model.NewId()
		userIDs := []string{model.NewId(), model.NewId(), model.NewId()}
		slices.Sort(userIDs)
		for _, userID := range userIDs {
			_, err := s.Team().SaveMember(rctx, &model.TeamMember{TeamId: teamID, UserId: userID, SchemeUser: true}, 100)
			require.NoError(t, err)
		}
		t.Cleanup(func() { require.NoError(t, s.Team().RemoveAllMembersByTeam(teamID)) })
		page, err := s.Team().GetMembers(teamID, 0, 2, &model.TeamMembersGetOptions{UseMaster: true})
		require.NoError(t, err)
		require.Len(t, page, 2)
		require.Equal(t, userIDs[1], page[1].UserId)
		require.NoError(t, s.Team().RemoveMember(rctx, teamID, userIDs[0]))
		page, err = s.Team().GetMembers(teamID, 0, 2, &model.TeamMembersGetOptions{UseMaster: true, AfterUserID: page[1].UserId})
		require.NoError(t, err)
		require.Len(t, page, 1, "removing an earlier member must not skip the next user")
		require.Equal(t, userIDs[2], page[0].UserId)
	})
}
