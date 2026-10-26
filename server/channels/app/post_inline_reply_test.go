// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestInlineReplyValidation(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	source, _, err := th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserId: th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message: "Xin chào 👋",
	}, false, false)
	require.Nil(t, err)

	makeReply := func(ref any) *model.Post {
		return &model.Post{
			UserId: th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message: "Trả lời",
			Props: model.StringInterface{model.PostPropsInlineReply: ref},
		}
	}
	valid := map[string]any{"v": float64(1), "post_id": source.Id}
	require.Nil(t, th.App.validateInlineReply(th.Context, makeReply(valid)))

	withSelection := map[string]any{
		"v": float64(1), "post_id": source.Id,
		"start": float64(9), "end": float64(11),
	}
	require.Nil(t, th.App.validateInlineReply(th.Context, makeReply(withSelection)))

	for _, ref := range []any{
		"not-an-object",
		map[string]any{"v": float64(1), "post_id": "bad"},
		map[string]any{"v": float64(2), "post_id": source.Id},
		map[string]any{"v": float64(1), "post_id": source.Id, "text": "leak"},
		map[string]any{"v": float64(1), "post_id": source.Id, "start": float64(0)},
		map[string]any{"v": float64(1), "post_id": source.Id, "start": float64(0), "end": float64(999)},
		map[string]any{"v": float64(1), "post_id": source.Id, "start": float64(-1), "end": float64(1)},
	} {
		require.NotNil(t, th.App.validateInlineReply(th.Context, makeReply(ref)))
	}

	other := th.CreateChannel(t, th.BasicTeam)
	crossChannel := makeReply(valid)
	crossChannel.ChannelId = other.Id
	require.NotNil(t, th.App.validateInlineReply(th.Context, crossChannel))

	threadReply := makeReply(valid)
	threadReply.RootId = source.Id
	require.NotNil(t, th.App.validateInlineReply(th.Context, threadReply))

	stored, _, err := th.App.CreatePostMissingChannel(th.Context, makeReply(valid), false, false)
	require.Nil(t, err)
	require.Empty(t, stored.RootId)
	require.Equal(t, source.Id, stored.GetProp(model.PostPropsInlineReply).(map[string]any)["post_id"])

	_, _, err = th.App.CreatePostMissingChannel(th.Context, crossChannel, false, false)
	require.NotNil(t, err)
	require.Equal(t, 400, err.StatusCode)
}
