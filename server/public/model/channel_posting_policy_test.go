// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelPostingPolicyDefaults(t *testing.T) {
	for _, prepare := range []func(*Channel){(*Channel).PreSave, (*Channel).PreUpdate} {
		channel := &Channel{Name: "channel", Type: ChannelTypeOpen}
		prepare(channel)
		data, err := json.Marshal(channel)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(data, &body))
		require.Equal(t, "all", body["posting_policy"])
		require.Equal(t, []any{}, body["posting_user_ids"])
		require.Equal(t, false, body["mandatory"])
	}
}

func TestChannelPostingPolicyValidation(t *testing.T) {
	userID := NewId()
	tooMany := make([]string, 1001)
	for i := range tooMany {
		tooMany[i] = NewId()
	}
	for _, tc := range []struct {
		name        string
		policy      string
		ids         []string
		channelType ChannelType
		mandatory   bool
		valid       bool
	}{
		{name: "legacy default", channelType: ChannelTypeOpen, valid: true},
		{name: "all", policy: "all", channelType: ChannelTypeOpen, valid: true},
		{name: "admins", policy: "admins", channelType: ChannelTypePrivate, valid: true},
		{name: "selected", policy: "selected", ids: []string{userID}, channelType: ChannelTypeOpen, valid: true},
		{name: "selected with no users", policy: "selected", channelType: ChannelTypePrivate, valid: true},
		{name: "mandatory private", policy: "all", channelType: ChannelTypePrivate, mandatory: true, valid: true},
		{name: "invalid policy", policy: "unknown", channelType: ChannelTypeOpen},
		{name: "invalid user", policy: "selected", ids: []string{"invalid"}, channelType: ChannelTypeOpen},
		{name: "duplicate user", policy: "selected", ids: []string{userID, userID}, channelType: ChannelTypeOpen},
		{name: "too many users", policy: "selected", ids: tooMany, channelType: ChannelTypeOpen},
		{name: "users in all", policy: "all", ids: []string{userID}, channelType: ChannelTypeOpen},
		{name: "users in admins", policy: "admins", ids: []string{userID}, channelType: ChannelTypeOpen},
		{name: "direct policy", policy: "admins", channelType: ChannelTypeDirect},
		{name: "group mandatory", policy: "all", channelType: ChannelTypeGroup, mandatory: true},
		{name: "space mandatory", policy: "all", channelType: ChannelTypeSpace, mandatory: true},
		{name: "board policy", policy: "selected", channelType: ChannelTypeOpenBoard},
		{name: "direct default", policy: "all", channelType: ChannelTypeDirect, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &Channel{Name: "channel", Type: tc.channelType}
			channel.PreSave()
			data, err := json.Marshal(map[string]any{"posting_policy": tc.policy, "posting_user_ids": tc.ids, "mandatory": tc.mandatory})
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, channel))
			if tc.valid {
				require.Nil(t, channel.IsValid())
			} else {
				require.NotNil(t, channel.IsValid())
			}
		})
	}
}

func TestChannelPostingPolicyPatchAndCopy(t *testing.T) {
	firstUser, nextUser := NewId(), NewId()
	channel := &Channel{PostingPolicy: ChannelPostingPolicySelected, PostingUserIDs: StringArray{firstUser}, Mandatory: true}
	copy := channel.DeepCopy()
	copy.PostingUserIDs[0] = nextUser
	require.Equal(t, StringArray{firstUser}, channel.PostingUserIDs, "mutating a copy must not change the original allowlist")

	channel.Patch(&ChannelPatch{})
	require.Equal(t, StringArray{firstUser}, channel.PostingUserIDs)
	require.True(t, channel.Mandatory)

	ids := StringArray{nextUser}
	patch := &ChannelPatch{PostingPolicy: new(ChannelPostingPolicySelected), PostingUserIDs: &ids, Mandatory: new(false)}
	channel.Patch(patch)
	require.Equal(t, StringArray{nextUser}, channel.PostingUserIDs)
	require.False(t, channel.Mandatory)
	ids[0] = firstUser
	require.Equal(t, StringArray{nextUser}, channel.PostingUserIDs, "patch input must not alias the saved allowlist")
	require.Equal(t, "selected", channel.Auditable()["posting_policy"])
	require.Equal(t, StringArray{nextUser}, channel.Auditable()["posting_user_ids"])
	require.Equal(t, false, channel.Auditable()["mandatory"])
	require.Equal(t, patch.PostingUserIDs, patch.Auditable()["posting_user_ids"])

	channel.Patch(&ChannelPatch{PostingPolicy: new(ChannelPostingPolicyAll), PostingUserIDs: new(StringArray{})})
	require.Empty(t, channel.PostingUserIDs)
	require.Equal(t, "all", channel.PostingPolicy)
}

func TestChannelPostingPolicyPatchClearsUnusedUsers(t *testing.T) {
	for _, policy := range []string{ChannelPostingPolicyAll, ChannelPostingPolicyAdmins} {
		channel := &Channel{PostingPolicy: ChannelPostingPolicySelected, PostingUserIDs: StringArray{NewId()}}
		channel.Patch(&ChannelPatch{PostingPolicy: &policy})
		require.Equal(t, StringArray{}, channel.PostingUserIDs)
	}
}
