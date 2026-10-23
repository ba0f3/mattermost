// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"slices"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// CanManageChannelPostingSettings is deliberately narrower than permission to
// edit channel properties, which ordinary members may have.
func (a *App) CanManageChannelPostingSettings(rctx request.CTX, channelID, userID string) (bool, *model.AppError) {
	rctx = RequestContextWithMaster(rctx)
	user, err := a.GetUser(rctx, userID)
	if err != nil {
		return false, err
	}
	if user.IsSystemAdmin() {
		return true, nil
	}
	member, err := a.GetChannelMember(rctx, channelID, userID)
	if err != nil {
		if err.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return member.SchemeAdmin, nil
}

// CheckChannelPostingPermission adds the channel's posting policy to the usual
// role permissions. It also protects posts created by integrations and jobs.
func (a *App) CheckChannelPostingPermission(rctx request.CTX, channelID, userID string) *model.AppError {
	// Authorization must not depend on a cached row or a lagging replica after
	// an administrator changes the policy.
	rctx = RequestContextWithMaster(rctx)
	channel, err := a.getChannelForPolicy(rctx, channelID)
	if err != nil {
		return err
	}
	if channel.PostingPolicy == "" || channel.PostingPolicy == model.ChannelPostingPolicyAll {
		return nil
	}
	admin, err := a.CanManageChannelPostingSettings(rctx, channel.Id, userID)
	if err != nil {
		return err
	}
	if admin {
		return nil
	}
	if channel.PostingPolicy == model.ChannelPostingPolicySelected && slices.Contains(channel.PostingUserIDs, userID) {
		if _, err := a.GetChannelMember(rctx, channel.Id, userID); err == nil {
			return nil
		}
	}
	return model.NewAppError("CheckChannelPostingPermission", "app.channel.posting_policy.denied", nil, "", http.StatusForbidden)
}

func (a *App) getChannelForPolicy(rctx request.CTX, channelID string) (*model.Channel, *model.AppError) {
	channel, err := a.Srv().Store().Channel().GetForPolicy(rctx, channelID)
	if err != nil {
		status := http.StatusInternalServerError
		var notFound *store.ErrNotFound
		if errors.As(err, &notFound) {
			status = http.StatusNotFound
		}
		return nil, model.NewAppError("getChannelForPolicy", "app.channel.get.find.app_error", map[string]any{"channel_id": channelID}, "", status).Wrap(err)
	}
	return channel, nil
}

func (a *App) validateChannelPostingMembers(rctx request.CTX, channel *model.Channel) *model.AppError {
	rctx = RequestContextWithMaster(rctx)
	for _, userID := range channel.PostingUserIDs {
		// A channel's creator becomes its first member during creation.
		if channel.Id == "" && userID == channel.CreatorId {
			continue
		}
		if _, err := a.GetChannelMember(rctx, channel.Id, userID); err != nil {
			if err.StatusCode != http.StatusNotFound {
				return err
			}
			return model.NewAppError("validateChannelPostingMembers", "app.channel.posting_policy.member_required", nil, "", http.StatusBadRequest)
		}
	}
	return nil
}

func (a *App) validateMandatoryChannel(rctx request.CTX, channel *model.Channel) *model.AppError {
	if !channel.Mandatory {
		return nil
	}
	if channel.IsGroupConstrained() || channel.PolicyEnforced {
		return model.NewAppError("validateMandatoryChannel", "app.channel.mandatory.conflicting_membership", nil, "", http.StatusBadRequest)
	}
	return nil
}
