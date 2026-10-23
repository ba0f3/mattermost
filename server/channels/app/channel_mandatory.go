// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SyncMandatoryChannelMembers enrolls all active team members, including guests.
// Existing memberships are preserved so a failed enrollment can be retried.
func (a *App) SyncMandatoryChannelMembers(rctx request.CTX, channel *model.Channel) *model.AppError {
	if !channel.Mandatory || channel.DeleteAt != 0 {
		return nil
	}
	rctx = store.RequestContextWithMaster(rctx)

	const perPage = 100
	options := &model.TeamMembersGetOptions{ExcludeDeletedUsers: true, UseMaster: true}
	for {
		members, err := a.GetTeamMembers(channel.TeamId, 0, perPage, options)
		if err != nil {
			return err
		}
		for _, member := range members {
			if err := a.enrollMandatoryChannelMember(rctx, member.UserId, channel); err != nil {
				return err
			}
		}
		if len(members) < perPage {
			return nil
		}
		options.AfterUserID = members[len(members)-1].UserId
	}
}

// JoinMandatoryChannels completes required enrollment for a new or existing team
// member. Returning an error lets callers retry a partially completed team join.
func (a *App) JoinMandatoryChannels(rctx request.CTX, teamID string, user *model.User) *model.AppError {
	rctx = store.RequestContextWithMaster(rctx)
	channels, err := a.Srv().Store().Channel().GetMandatoryChannels(teamID)
	if err != nil {
		return model.NewAppError("JoinMandatoryChannels", "app.channel.get_channels.get.app_error", nil, "team_id="+teamID, http.StatusInternalServerError).Wrap(err)
	}
	for _, channel := range channels {
		if err := a.enrollMandatoryChannelMember(rctx, user.Id, channel); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) enrollMandatoryChannelMember(rctx request.CTX, userID string, channel *model.Channel) *model.AppError {
	member, err := a.GetTeamMember(rctx, channel.TeamId, userID)
	if err != nil {
		if err.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}
	if member.DeleteAt != 0 {
		return nil
	}

	_, addErr := a.AddChannelMember(rctx, userID, channel, ChannelMemberOpts{SkipTeamMemberIntegrityCheck: true})
	// LeaveTeam can finish between the membership check and the channel insert.
	// Recheck even when AddChannelMember fails: it can fail after saving the row.
	member, err = a.GetTeamMember(rctx, channel.TeamId, userID)
	if err != nil && err.StatusCode != http.StatusNotFound {
		return err
	}
	if err != nil || member.DeleteAt != 0 {
		if _, cleanupErr := a.removeMandatoryChannelMembershipIfPresent(rctx, userID, channel.Id); cleanupErr != nil {
			return cleanupErr
		}
	}
	return addErr
}

func (a *App) removeMandatoryChannelMembershipIfPresent(rctx request.CTX, userID, channelID string) (bool, *model.AppError) {
	rctx = store.RequestContextWithMaster(rctx)
	if _, err := a.Srv().Store().Channel().GetMember(rctx, channelID, userID); err != nil {
		var notFound *store.ErrNotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, model.NewAppError("removeMandatoryChannelMembershipIfPresent", "app.channel.get_member.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	defer a.Srv().Platform().InvalidateChannelCacheForUser(userID)
	a.invalidateCacheForChannelMembers(channelID)
	if err := a.removeChannelMembership(rctx, userID, channelID, "removeMandatoryChannelMembershipIfPresent"); err != nil {
		return false, err
	}
	// Enrollment may have published user_added after the team-leave event.
	// Notify both remaining channel members and the departed user's client.
	message := model.NewWebSocketEvent(model.WebsocketEventUserRemoved, "", channelID, "", nil, "")
	message.Add("user_id", userID)
	message.Add("remover_id", "")
	a.Publish(message)
	userMessage := model.NewWebSocketEvent(model.WebsocketEventUserRemoved, "", "", userID, nil, "")
	userMessage.Add("channel_id", channelID)
	userMessage.Add("remover_id", "")
	a.Publish(userMessage)
	return true, nil
}
