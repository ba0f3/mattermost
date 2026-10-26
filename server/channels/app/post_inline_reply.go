// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"math"
	"net/http"
	"unicode/utf16"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store/sqlstore"
)

// validateInlineReply keeps Telegram-style inline references within the same
// channel and never persists text copied from another post. Unlike RootId,
// the quote reference does not create a thread.
func (a *App) validateInlineReply(rctx request.CTX, post *model.Post) *model.AppError {
	value, exists := post.GetProps()[model.PostPropsInlineReply]
	if !exists {
		return nil
	}

	invalid := func() *model.AppError {
		// The same error covers a missing, deleted and foreign-channel post to
		// avoid revealing whether the referenced post exists.
		return model.NewAppError("validateInlineReply", "api.context.invalid_param.app_error",
			map[string]any{"Name": "post.props.r2d_inline_reply"}, "", http.StatusBadRequest)
	}

	ref, ok := value.(map[string]any)
	if !ok || (len(ref) != 2 && len(ref) != 4) || post.RootId != "" || post.Type != model.PostTypeDefault {
		return invalid()
	}

	version, validVersion := ref["v"].(float64)
	id, validID := ref["post_id"].(string)
	if !validVersion || version != 1 || !validID || !model.IsValidId(id) || id == post.Id {
		return invalid()
	}

	startValue, hasStart := ref["start"]
	endValue, hasEnd := ref["end"]
	if hasStart != hasEnd || (hasStart && len(ref) != 4) || (!hasStart && len(ref) != 2) {
		return invalid()
	}

	// Read from master to support replying to a message just written. Do not
	// use a preview supplied by a client or quote a source from another channel.
	source, err := a.Srv().Store().Post().GetSingle(sqlstore.RequestContextWithMaster(rctx), id, false)
	if err != nil || source == nil || source.ChannelId != post.ChannelId ||
		source.DeleteAt != 0 || source.IsSystemMessage() ||
		(source.Type != model.PostTypeDefault && source.Type != model.PostTypeMe) {
		return invalid()
	}

	if hasStart {
		start, startOK := startValue.(float64)
		end, endOK := endValue.(float64)
		// JavaScript selections are UTF-16 code-unit offsets. Bound the range
		// and do not let malformed floats reach the integer conversion.
		if !startOK || !endOK || math.IsNaN(start) || math.IsNaN(end) ||
			math.IsInf(start, 0) || math.IsInf(end, 0) ||
			start < 0 || end <= start || end-start > 500 ||
			math.Trunc(start) != start || math.Trunc(end) != end {
			return invalid()
		}
		units := utf16.Encode([]rune(source.Message))
		if end > float64(len(units)) {
			return invalid()
		}
	}

	return nil
}
