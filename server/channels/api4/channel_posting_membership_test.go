// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestChannelPostingPolicyRejectsMemberPost(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	resp, err := th.SystemAdminClient.DoAPIPut(context.Background(), "/channels/"+th.BasicChannel.Id+"/patch", `{"posting_policy":"admins"}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	th.LoginBasic2(t)
	_, postResp, err := th.Client.CreatePost(context.Background(), &model.Post{
		ChannelId: th.BasicChannel.Id,
		Message:   "Members cannot post in an announcement channel.",
	})
	require.Error(t, err)
	CheckForbiddenStatus(t, postResp)
}

func TestChannelPostingPolicyAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	require.Nil(t, th.App.Srv().License(), "channel posting policies must work without a license")
	memberClient := th.CreateClient()
	_, _, err := memberClient.Login(ctx, th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)

	for _, channel := range []*model.Channel{th.BasicChannel, th.BasicPrivateChannel} {
		t.Run(string(channel.Type), func(t *testing.T) {
			_, err := th.SystemAdminClient.UpdateChannelRoles(ctx, channel.Id, th.BasicUser.Id, "channel_user channel_admin")
			require.NoError(t, err)
			stored, _, err := th.Client.GetChannel(ctx, channel.Id)
			require.NoError(t, err)
			require.Equal(t, model.ChannelPostingPolicyAll, stored.PostingPolicy)
			require.Empty(t, stored.PostingUserIDs)
			require.False(t, stored.Mandatory)

			root, _, err := memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Everyone can post by default."})
			require.NoError(t, err)
			_, _, err = memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: root.Id, Message: "Everyone can reply by default."})
			require.NoError(t, err)

			_, _, err = th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{PostingPolicy: new(model.ChannelPostingPolicyAdmins)})
			require.NoError(t, err, "channel admins can configure posting")
			for _, rootID := range []string{"", root.Id} {
				_, resp, err := memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: rootID, Message: "Blocked member post."})
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
			}
			for _, client := range []*model.Client4{th.Client, th.SystemAdminClient} {
				_, _, err := client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "An administrator can publish."})
				require.NoError(t, err)
			}

			selected := model.StringArray{th.BasicUser.Id}
			_, _, err = th.SystemAdminClient.PatchChannel(ctx, channel.Id, &model.ChannelPatch{
				PostingPolicy:  new(model.ChannelPostingPolicySelected),
				PostingUserIDs: &selected,
			})
			require.NoError(t, err)
			for _, rootID := range []string{"", root.Id} {
				_, resp, err := memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: rootID, Message: "An unselected member cannot publish."})
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
			}
			selected = model.StringArray{th.BasicUser2.Id}
			_, _, err = th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{PostingUserIDs: &selected})
			require.NoError(t, err)
			stored, _, err = memberClient.GetChannel(ctx, channel.Id)
			require.NoError(t, err)
			require.Equal(t, model.ChannelPostingPolicySelected, stored.PostingPolicy)
			require.Equal(t, selected, stored.PostingUserIDs)
			for _, rootID := range []string{"", root.Id} {
				_, _, err = memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: rootID, Message: "A selected member can publish."})
				require.NoError(t, err)
			}
			_, _, err = th.Client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Channel admins remain able to publish."})
			require.NoError(t, err)

			empty := model.StringArray{}
			_, _, err = th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{PostingUserIDs: &empty})
			require.NoError(t, err)
			_, resp, err := memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "A removed selection cannot publish."})
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)

			_, _, err = th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{PostingPolicy: new(model.ChannelPostingPolicyAll)})
			require.NoError(t, err)
			_, _, err = memberClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Restoring all reopens posting."})
			require.NoError(t, err)
		})
	}
}

func TestChannelPostingSettingsPermissions(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	th.LoginBasic2(t)

	_, _, err := th.Client.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{Header: new("Members may edit the header.")})
	require.NoError(t, err)

	selected := model.StringArray{th.BasicUser2.Id}
	for name, patch := range map[string]*model.ChannelPatch{
		"posting policy": {PostingPolicy: new(model.ChannelPostingPolicyAdmins)},
		"selected users": {PostingUserIDs: &selected},
		"mandatory":      {Mandatory: new(true)},
		"mixed patch":    {Header: new("Must not partially apply"), PostingPolicy: new(model.ChannelPostingPolicyAdmins)},
	} {
		t.Run(name, func(t *testing.T) {
			_, resp, err := th.Client.PatchChannel(ctx, th.BasicChannel.Id, patch)
			require.Error(t, err)
			CheckForbiddenStatus(t, resp)
		})
	}
	stored, _, err := th.Client.GetChannel(ctx, th.BasicChannel.Id)
	require.NoError(t, err)
	require.Equal(t, "Members may edit the header.", stored.Header)
	require.Equal(t, model.ChannelPostingPolicyAll, stored.PostingPolicy)
	require.Empty(t, stored.PostingUserIDs)
	require.False(t, stored.Mandatory)

	t.Run("full channel update cannot grant posting privileges", func(t *testing.T) {
		stored.PostingPolicy = model.ChannelPostingPolicySelected
		stored.PostingUserIDs = selected
		_, _, err := th.Client.UpdateChannel(ctx, stored)
		require.NoError(t, err)
		persisted, _, err := th.Client.GetChannel(ctx, stored.Id)
		require.NoError(t, err)
		require.Equal(t, model.ChannelPostingPolicyAll, persisted.PostingPolicy)
		require.Empty(t, persisted.PostingUserIDs)
	})

	t.Run("ordinary property patches preserve posting restrictions", func(t *testing.T) {
		_, _, err := th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{PostingPolicy: new(model.ChannelPostingPolicyAdmins)})
		require.NoError(t, err)
		_, _, err = th.Client.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{Header: new("An updated channel header.")})
		require.NoError(t, err)
		channel, _, err := th.Client.GetChannel(ctx, th.BasicChannel.Id)
		require.NoError(t, err)
		require.Equal(t, model.ChannelPostingPolicyAdmins, channel.PostingPolicy)
		_, resp, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Editing the header does not restore posting."})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("team admin is not automatically a channel admin", func(t *testing.T) {
		_, err := th.SystemAdminClient.UpdateTeamMemberRoles(ctx, th.BasicTeam.Id, th.BasicUser2.Id, "team_user team_admin")
		require.NoError(t, err)
		_, resp, err := th.Client.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{Mandatory: new(true)})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})
}

func TestChannelPostingPolicyPreservesPermissions(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	selected := model.StringArray{th.BasicUser2.Id}
	_, _, err := th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{
		PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: &selected,
	})
	require.NoError(t, err)
	permissions := th.SaveDefaultRolePermissions(t)
	t.Cleanup(func() { th.RestoreDefaultRolePermissions(t, permissions) })
	th.RemovePermissionFromRole(t, model.PermissionCreatePost.Id, model.ChannelUserRoleId)
	th.LoginBasic2(t)
	_, resp, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "Selection does not grant create_post."})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestChannelPostingSettingsValidation(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	outsider := th.CreateUser(t)
	teamMember := th.CreateUser(t)
	th.LinkUserToTeam(t, teamMember, th.BasicTeam)

	for name, patch := range map[string]*model.ChannelPatch{
		"invalid policy":           {PostingPolicy: new("owners")},
		"malformed selected ID":    {PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: new(model.StringArray{"invalid"})},
		"unknown selected ID":      {PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: new(model.StringArray{model.NewId()})},
		"selected nonteam user":    {PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: new(model.StringArray{outsider.Id})},
		"selected nonchannel user": {PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: new(model.StringArray{teamMember.Id})},
	} {
		t.Run(name, func(t *testing.T) {
			_, resp, err := th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, patch)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
			channel, _, err := th.Client.GetChannel(ctx, th.BasicChannel.Id)
			require.NoError(t, err)
			require.Equal(t, model.ChannelPostingPolicyAll, channel.PostingPolicy)
			require.Empty(t, channel.PostingUserIDs)
		})
	}

	direct, _, err := th.SystemAdminClient.CreateDirectChannel(ctx, th.SystemAdminUser.Id, th.BasicUser2.Id)
	require.NoError(t, err)
	group, _, err := th.SystemAdminClient.CreateGroupChannel(ctx, []string{th.SystemAdminUser.Id, th.BasicUser.Id, th.BasicUser2.Id})
	require.NoError(t, err)
	for _, channel := range []*model.Channel{direct, group} {
		for _, patch := range []*model.ChannelPatch{{PostingPolicy: new(model.ChannelPostingPolicyAdmins)}, {Mandatory: new(true)}} {
			_, resp, err := th.SystemAdminClient.PatchChannel(ctx, channel.Id, patch)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		}
	}
}

func TestChannelMandatoryMembershipAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	require.Nil(t, th.App.Srv().License(), "mandatory channels must work without a license")
	memberClient := th.CreateClient()
	_, _, err := memberClient.Login(ctx, th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)

	for _, channelType := range []model.ChannelType{model.ChannelTypeOpen, model.ChannelTypePrivate} {
		t.Run(string(channelType), func(t *testing.T) {
			channel := th.CreateChannelWithClient(t, th.Client, channelType)
			inactive := th.CreateUser(t)
			th.LinkUserToTeam(t, inactive, th.BasicTeam)
			th.UpdateActiveUser(t, inactive, false)
			formerMember := th.CreateUser(t)
			th.LinkUserToTeam(t, formerMember, th.BasicTeam)
			th.UnlinkUserFromTeam(t, formerMember, th.BasicTeam)
			guest, appErr := th.App.CreateGuest(th.Context, &model.User{
				Email: th.GenerateTestEmail(), Username: "guest_" + model.NewId(), Password: model.NewTestPassword(), EmailVerified: true,
			})
			require.Nil(t, appErr)
			th.LinkUserToTeam(t, guest, th.BasicTeam)

			_, _, err := th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{Mandatory: new(true)})
			require.NoError(t, err)
			stored, _, err := memberClient.GetChannel(ctx, channel.Id)
			require.NoError(t, err)
			require.True(t, stored.Mandatory)
			for _, user := range []*model.User{th.BasicUser, th.BasicUser2, guest} {
				_, _, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user.Id, "")
				require.NoError(t, err, "active team members must be enrolled")
			}
			for _, user := range []*model.User{inactive, formerMember} {
				_, resp, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user.Id, "")
				require.Error(t, err)
				CheckNotFoundStatus(t, resp)
			}
			th.UpdateActiveUser(t, inactive, true)
			_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, inactive.Id, "")
			require.NoError(t, err, "reactivated team members must be enrolled")

			joined := th.CreateUser(t)
			_, _, err = th.SystemAdminClient.AddTeamMember(ctx, th.BasicTeam.Id, joined.Id)
			require.NoError(t, err)
			_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, joined.Id, "")
			require.NoError(t, err, "new team members must be enrolled")
			newGuest, appErr := th.App.CreateGuest(th.Context, &model.User{
				Email: th.GenerateTestEmail(), Username: "guest_" + model.NewId(), Password: model.NewTestPassword(), EmailVerified: true,
			})
			require.Nil(t, appErr)
			_, _, err = th.SystemAdminClient.AddTeamMember(ctx, th.BasicTeam.Id, newGuest.Id)
			require.NoError(t, err)
			_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, newGuest.Id, "")
			require.NoError(t, err, "new guest members must be enrolled")
			_, _, err = th.SystemAdminClient.AddTeamMember(ctx, th.BasicTeam.Id, formerMember.Id)
			require.NoError(t, err)
			_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, formerMember.Id, "")
			require.NoError(t, err, "returning team members must be enrolled")
			_, err = th.SystemAdminClient.RemoveTeamMember(ctx, th.BasicTeam.Id, formerMember.Id)
			require.NoError(t, err, "mandatory channels must not prevent team membership cleanup")
			_, resp, err := th.SystemAdminClient.GetChannelMember(ctx, channel.Id, formerMember.Id, "")
			require.Error(t, err)
			CheckNotFoundStatus(t, resp)

			for _, client := range []*model.Client4{memberClient, th.Client, th.SystemAdminClient} {
				resp, err := client.RemoveUserFromChannel(ctx, channel.Id, th.BasicUser2.Id)
				require.Error(t, err)
				CheckForbiddenStatus(t, resp)
			}
			_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser2.Id, "")
			require.NoError(t, err)

			_, _, err = th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{Mandatory: new(false)})
			require.NoError(t, err)
			for _, user := range []*model.User{th.BasicUser2, guest, joined, newGuest} {
				_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, user.Id, "")
				require.NoError(t, err, "disabling mandatory membership preserves members")
			}
			_, err = memberClient.RemoveUserFromChannel(ctx, channel.Id, th.BasicUser2.Id)
			require.NoError(t, err)
		})
	}
}

func TestChannelMandatoryMembershipRejectsConstraints(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	t.Run("group constrained", func(t *testing.T) {
		channel, _, err := th.Client.CreateChannel(ctx, &model.Channel{
			TeamId: th.BasicTeam.Id, Type: model.ChannelTypeOpen, Name: GenerateTestChannelName(), DisplayName: "Group constrained", GroupConstrained: new(true),
		})
		require.NoError(t, err)
		_, resp, err := th.SystemAdminClient.PatchChannel(ctx, channel.Id, &model.ChannelPatch{Mandatory: new(true)})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("mandatory channel cannot become group constrained", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		_, _, err := th.Client.PatchChannel(ctx, channel.Id, &model.ChannelPatch{Mandatory: new(true)})
		require.NoError(t, err)
		_, resp, err := th.SystemAdminClient.PatchChannel(ctx, channel.Id, &model.ChannelPatch{GroupConstrained: new(true)})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("attribute policy", func(t *testing.T) {
		channel := th.CreatePrivateChannel(t)
		_, err := th.App.Srv().Store().AccessControlPolicy().Save(th.Context, &model.AccessControlPolicy{
			ID: channel.Id, Type: model.AccessControlPolicyTypeChannel, Active: true, Revision: 1,
			Version: model.AccessControlPolicyVersionV0_2, Imports: []string{},
			Rules: []model.AccessControlPolicyRule{{Actions: []string{model.AccessControlPolicyActionMembership}, Expression: "true"}},
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = th.App.Srv().Store().AccessControlPolicy().Delete(th.Context, channel.Id) })
		th.App.Srv().Store().Channel().InvalidateChannel(channel.Id)
		stored, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.True(t, stored.PolicyEnforced)
		_, resp, err := th.SystemAdminClient.PatchChannel(ctx, channel.Id, &model.ChannelPatch{Mandatory: new(true)})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestChannelPostingPolicyScheduledPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.Srv().SetLicense(model.NewTestLicenseSKU(model.LicenseShortSkuProfessional))
	ctx := context.Background()
	th.LoginBasic2(t)
	scheduled := &model.ScheduledPost{Draft: model.Draft{
		UserId: th.BasicUser2.Id, ChannelId: th.BasicChannel.Id, Message: "A post scheduled before the policy changes.",
	}, ScheduledAt: model.GetMillis() + 300000}
	created, _, err := th.Client.CreateScheduledPost(ctx, scheduled)
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{PostingPolicy: new(model.ChannelPostingPolicyAdmins)})
	require.NoError(t, err)
	_, resp, err := th.Client.CreateScheduledPost(ctx, scheduled)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	created.Message = "An update after posting permission was removed."
	_, resp, err = th.Client.UpdateScheduledPost(ctx, created)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestChannelPostingPolicyIncomingWebhook(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	ctx := context.Background()
	hook, appErr := th.App.CreateIncomingWebhookForChannel(th.BasicUser2.Id, th.BasicChannel, &model.IncomingWebhook{ChannelId: th.BasicChannel.Id})
	require.Nil(t, appErr)
	postWebhook := func() *http.Response {
		t.Helper()
		resp, err := http.Post(th.Client.URL+"/hooks/"+hook.Id, "application/json", strings.NewReader(`{"text":"An incoming integration post."}`))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp
	}
	require.Equal(t, http.StatusOK, postWebhook().StatusCode)
	_, _, err := th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{PostingPolicy: new(model.ChannelPostingPolicyAdmins)})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, postWebhook().StatusCode)
	selected := model.StringArray{th.BasicUser2.Id}
	_, _, err = th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{
		PostingPolicy: new(model.ChannelPostingPolicySelected), PostingUserIDs: &selected,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, postWebhook().StatusCode)
}

func TestChannelPostingSettingsOnCreate(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	t.Run("creator can configure selected posting", func(t *testing.T) {
		channel, _, err := th.Client.CreateChannel(ctx, &model.Channel{
			TeamId: th.BasicTeam.Id, Type: model.ChannelTypeOpen, Name: GenerateTestChannelName(), DisplayName: "Selected posting",
			PostingPolicy: model.ChannelPostingPolicySelected, PostingUserIDs: model.StringArray{th.BasicUser.Id},
		})
		require.NoError(t, err)
		stored, _, err := th.Client.GetChannel(ctx, channel.Id)
		require.NoError(t, err)
		require.Equal(t, model.ChannelPostingPolicySelected, stored.PostingPolicy)
		require.Equal(t, model.StringArray{th.BasicUser.Id}, stored.PostingUserIDs)
		_, _, err = th.Client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "The creator can publish."})
		require.NoError(t, err)
	})

	t.Run("selected users must already be channel members", func(t *testing.T) {
		_, resp, err := th.Client.CreateChannel(ctx, &model.Channel{
			TeamId: th.BasicTeam.Id, Type: model.ChannelTypeOpen, Name: GenerateTestChannelName(), DisplayName: "Invalid selection",
			PostingPolicy: model.ChannelPostingPolicySelected, PostingUserIDs: model.StringArray{th.BasicUser2.Id},
		})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("mandatory channel creation enrolls team members", func(t *testing.T) {
		channel, _, err := th.Client.CreateChannel(ctx, &model.Channel{
			TeamId: th.BasicTeam.Id, Type: model.ChannelTypePrivate, Name: GenerateTestChannelName(), DisplayName: "Required announcements",
			PostingPolicy: model.ChannelPostingPolicyAdmins, Mandatory: true,
		})
		require.NoError(t, err)
		require.True(t, channel.Mandatory)
		_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
	})
}

func TestChannelMandatoryMembershipOnRestore(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	_, _, err := th.SystemAdminClient.PatchChannel(ctx, th.BasicChannel.Id, &model.ChannelPatch{Mandatory: new(true)})
	require.NoError(t, err)
	_, err = th.SystemAdminClient.DeleteChannel(ctx, th.BasicChannel.Id)
	require.NoError(t, err)
	joined := th.CreateUser(t)
	_, _, err = th.SystemAdminClient.AddTeamMember(ctx, th.BasicTeam.Id, joined.Id)
	require.NoError(t, err)
	channel, _, err := th.SystemAdminClient.RestoreChannel(ctx, th.BasicChannel.Id)
	require.NoError(t, err)
	require.True(t, channel.Mandatory)
	_, _, err = th.SystemAdminClient.GetChannelMember(ctx, channel.Id, joined.Id, "")
	require.NoError(t, err, "restoring a mandatory channel must enroll members who joined while it was archived")
}
