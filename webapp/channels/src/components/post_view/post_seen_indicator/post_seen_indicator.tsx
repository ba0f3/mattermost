// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {defineMessages, useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CheckAllIcon, CheckIcon} from '@mattermost/compass-icons/components';
import {WithTooltip} from '@mattermost/shared/components/tooltip';
import type {Post} from '@mattermost/types/posts';

import {getPostsSeenStatusBatched} from 'mattermost-redux/actions/read_receipts';
import {canShowPostReadReceipts, getPostSeenStatus} from 'mattermost-redux/selectors/entities/read_receipts';

import type {GlobalState} from 'types/store';

import './post_seen_indicator.scss';

const messages = defineMessages({
    sent: {
        id: 'post_info.read_receipts.sent',
        defaultMessage: 'Sent',
    },
    seen: {
        id: 'post_info.read_receipts.seen',
        defaultMessage: 'Seen',
    },
});

type Props = {
    post: Post;
};

export default function PostSeenIndicator({post}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const canShow = useSelector((state: GlobalState) => canShowPostReadReceipts(state, post));
    const seen = useSelector((state: GlobalState) => getPostSeenStatus(state, post.id));
    const needsStatus = canShow && seen === undefined;

    useEffect(() => {
        if (needsStatus) {
            dispatch(getPostsSeenStatusBatched([post.id]));
        }
    }, [dispatch, needsStatus, post.id]);

    if (!canShow) {
        return null;
    }

    const label = formatMessage(seen ? messages.seen : messages.sent);
    const Icon = seen ? CheckAllIcon : CheckIcon;

    return (
        <WithTooltip title={label}>
            <span
                className={seen ? 'PostSeenIndicator PostSeenIndicator--seen' : 'PostSeenIndicator'}
                role='img'
                aria-label={label}
                data-testid={`post_seen_indicator_${post.id}`}
            >
                <Icon size={14}/>
            </span>
        </WithTooltip>
    );
}
