// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func TestMigration000238(t *testing.T) {
	store := newHealthFindingsMigrationStore(t)
	rctx := request.TestContext(t)

	require.True(t, healthFindingsTableExists(t, store, "channelmemberseen"))
	require.True(t, healthFindingsIndexExists(t, store, "idx_channelmemberseen_userid"))

	channel, err := store.Channel().Save(rctx, &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Migration000238",
		Name:        "m238-" + model.NewId(),
		Type:        model.ChannelTypeOpen,
		LastPostAt:  5000,
	}, -1)
	require.NoError(t, err)
	viewedLong := model.NewId()
	viewedBefore := model.NewId()
	neverViewed := model.NewId()
	for userID, lastViewedAt := range map[string]int64{viewedLong: 9000, viewedBefore: 3000, neverViewed: 0} {
		_, err = store.Channel().SaveMember(rctx, &model.ChannelMember{
			ChannelId:    channel.Id,
			UserId:       userID,
			NotifyProps:  model.GetDefaultChannelNotifyProps(),
			LastViewedAt: lastViewedAt,
		})
		require.NoError(t, err)
	}

	seen := func(t *testing.T) map[string]int64 {
		t.Helper()
		rows := []struct {
			UserId   string
			SeenUpTo int64
		}{}
		require.NoError(t, store.GetMaster().Select(&rows, "SELECT UserId, SeenUpTo FROM ChannelMemberSeen WHERE ChannelId = $1", channel.Id))
		result := map[string]int64{}
		for _, row := range rows {
			result[row.UserId] = row.SeenUpTo
		}
		return result
	}

	_, err = store.GetMaster().ExecNoTimeout(readMigrationSQL(t, "000238_create_channelmemberseen.down.sql"))
	require.NoError(t, err)
	assert.False(t, healthFindingsTableExists(t, store, "channelmemberseen"))
	assert.False(t, healthFindingsIndexExists(t, store, "idx_channelmemberseen_userid"))

	// The backfill copies LastViewedAt, capped by the channel's LastPostAt,
	// and skips members who never viewed the channel.
	_, err = store.GetMaster().ExecNoTimeout(readMigrationSQL(t, "000238_create_channelmemberseen.up.sql"))
	require.NoError(t, err)
	assert.True(t, healthFindingsTableExists(t, store, "channelmemberseen"))
	assert.True(t, healthFindingsIndexExists(t, store, "idx_channelmemberseen_userid"))
	require.Equal(t, map[string]int64{viewedLong: 5000, viewedBefore: 3000}, seen(t))

	// A second up is a no-op and leaves advanced pointers alone.
	_, seenUpTo, err := store.Channel().AdvanceReadReceiptSeenUpTo(rctx, []string{channel.Id}, viewedBefore, 6000)
	require.NoError(t, err)
	require.Equal(t, map[string]int64{channel.Id: 5000}, seenUpTo)
	_, err = store.GetMaster().ExecNoTimeout(readMigrationSQL(t, "000238_create_channelmemberseen.up.sql"))
	require.NoError(t, err)
	require.Equal(t, map[string]int64{viewedLong: 5000, viewedBefore: 5000}, seen(t))
}
