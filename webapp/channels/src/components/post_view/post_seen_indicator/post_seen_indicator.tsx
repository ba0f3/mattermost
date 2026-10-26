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

import {openModal} from 'actions/views/modals';

import PostSeenByModal from 'components/post_seen_by_modal';

import {ModalIdentifiers} from 'utils/constants';

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

    const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
        event.stopPropagation();
        dispatch(openModal({
            modalId: ModalIdentifiers.POST_SEEN_BY,
            dialogType: PostSeenByModal,
            dialogProps: {postId: post.id},
        }));
    };

    return (
        <WithTooltip title={label}>
            <button
                type='button'
                className={seen ? 'PostSeenIndicator PostSeenIndicator--seen' : 'PostSeenIndicator'}
                aria-label={label}
                data-testid={`post_seen_indicator_${post.id}`}
                onClick={handleClick}
            >
                <Icon size={14}/>
            </button>
        </WithTooltip>
    );
}
