// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

type mandatoryEnrollmentRaceStore struct {
	store.Store
	channel store.ChannelStore
}

func TestMandatoryEnrollmentCompensationNotifiesClients(t *testing.T) {
	th := Setup(t).InitBasic(t)
	channel := th.CreatePrivateChannel(t, th.BasicTeam)
	channel.Mandatory = true
	channel.DisableJoinLeaveMessages = true
	_, err := th.App.Srv().Store().Channel().Update(th.Context, channel)
	require.NoError(t, err)
	user := th.BasicUser2
	userEvents, closeUser := connectFakeWebSocket(t, th, user.Id, "", []model.WebsocketEventType{model.WebsocketEventUserRemoved})
	defer closeUser()
	memberEvents, closeMember := connectFakeWebSocket(t, th, th.BasicUser.Id, "", []model.WebsocketEventType{model.WebsocketEventUserRemoved})
	defer closeMember()

	original := th.App.Srv().Store()
	wrapped := &mandatoryEnrollmentRaceChannelStore{ChannelStore: original.Channel(), channelID: channel.Id, userID: user.Id}
	th.App.Srv().SetStore(&mandatoryEnrollmentRaceStore{Store: original, channel: wrapped})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })
	wrapped.beforeSave = func() {
		require.Nil(t, th.App.LeaveTeam(th.Context, th.BasicTeam, user, user.Id))
	}
	require.Nil(t, th.App.JoinMandatoryChannels(th.Context, th.BasicTeam.Id, user))

	select {
	case event := <-userEvents:
		require.Equal(t, channel.Id, event.GetData()["channel_id"])
	case <-time.After(10 * time.Second):
		t.Fatal("departed user's client did not receive compensated channel removal")
	}
	select {
	case event := <-memberEvents:
		require.Equal(t, user.Id, event.GetData()["user_id"])
	case <-time.After(10 * time.Second):
		t.Fatal("channel members did not receive compensated membership removal")
	}
}

func (s *mandatoryEnrollmentRaceStore) Channel() store.ChannelStore { return s.channel }

type mandatoryEnrollmentRaceChannelStore struct {
	store.ChannelStore
	channelID    string
	userID       string
	beforeSave   func()
	beforeRemove func()
	afterRemove  func()
}

func (s *mandatoryEnrollmentRaceChannelStore) SaveMember(rctx request.CTX, member *model.ChannelMember) (*model.ChannelMember, error) {
	if member.ChannelId == s.channelID && member.UserId == s.userID && s.beforeSave != nil {
		callback := s.beforeSave
		s.beforeSave = nil
		callback()
	}
	return s.ChannelStore.SaveMember(rctx, member)
}

func (s *mandatoryEnrollmentRaceChannelStore) RemoveMember(rctx request.CTX, channelID, userID string) error {
	if channelID == s.channelID && userID == s.userID && s.beforeRemove != nil {
		callback := s.beforeRemove
		s.beforeRemove = nil
		callback()
	}
	err := s.ChannelStore.RemoveMember(rctx, channelID, userID)
	if err == nil && channelID == s.channelID && userID == s.userID && s.afterRemove != nil {
		callback := s.afterRemove
		s.afterRemove = nil
		callback()
	}
	return err
}

func TestMandatoryEnrollmentDoesNotRestoreDepartedMembers(t *testing.T) {
	for _, enrollment := range []string{"join", "backfill"} {
		for _, interleaving := range []string{"after team leave", "team leave before channel insert", "channel insert before team removal", "channel archived before team removal", "channel made optional before team removal"} {
			t.Run(enrollment+"/"+interleaving, func(t *testing.T) {
				th := Setup(t).InitBasic(t)
				channel := th.CreatePrivateChannel(t, th.BasicTeam)
				channel.Mandatory = true
				channel.DisableJoinLeaveMessages = true
				_, err := th.App.Srv().Store().Channel().Update(th.Context, channel)
				require.NoError(t, err)
				user := th.BasicUser2
				original := th.App.Srv().Store()
				wrapped := &mandatoryEnrollmentRaceChannelStore{ChannelStore: original.Channel(), channelID: channel.Id, userID: user.Id}
				th.App.Srv().SetStore(&mandatoryEnrollmentRaceStore{Store: original, channel: wrapped})
				t.Cleanup(func() { th.App.Srv().SetStore(original) })

				enroll := func() {
					t.Helper()
					if enrollment == "join" {
						require.Nil(t, th.App.JoinMandatoryChannels(th.Context, th.BasicTeam.Id, user))
					} else {
						require.Nil(t, th.App.SyncMandatoryChannelMembers(th.Context, channel))
					}
				}
				leave := func() {
					t.Helper()
					require.Nil(t, th.App.LeaveTeam(th.Context, th.BasicTeam, user, user.Id))
				}

				switch interleaving {
				case "after team leave":
					leave()
					enroll()
				case "team leave before channel insert":
					wrapped.beforeSave = func() {
						leave()
						wrapped.beforeRemove = func() {
							memberships, err := original.Channel().GetAllChannelMembersForUser(th.Context, user.Id, true, true)
							require.NoError(t, err)
							require.Contains(t, memberships, channel.Id, "prime the membership cache before cleanup")
						}
					}
					enroll()
					require.Nil(t, wrapped.beforeSave, "the team leave must run before the channel membership is saved")
				case "channel insert before team removal", "channel archived before team removal", "channel made optional before team removal":
					enroll()
					wrapped.afterRemove = func() {
						enroll()
						switch interleaving {
						case "channel archived before team removal":
							require.NoError(t, original.Channel().SetDeleteAt(channel.Id, model.GetMillis(), model.GetMillis()))
						case "channel made optional before team removal":
							channel.Mandatory = false
							_, err := original.Channel().Update(th.Context, channel)
							require.NoError(t, err)
						}
					}
					leave()
					require.Nil(t, wrapped.afterRemove, "enrollment must run after LeaveTeam removes the channel membership")
				}

				member, err := original.Team().GetMember(th.Context, th.BasicTeam.Id, user.Id)
				require.NoError(t, err)
				require.NotZero(t, member.DeleteAt, "mandatory enrollment must not restore team membership")
				_, err = original.Channel().GetMember(th.Context, channel.Id, user.Id)
				var notFound *store.ErrNotFound
				require.ErrorAs(t, err, &notFound, "a departed team member must not retain mandatory channel membership")
				memberships, err := original.Channel().GetAllChannelMembersForUser(th.Context, user.Id, true, true)
				require.NoError(t, err)
				require.NotContains(t, memberships, channel.Id, "cached membership must not preserve access after cleanup")
			})
		}
	}
}
