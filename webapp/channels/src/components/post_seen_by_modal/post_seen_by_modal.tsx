// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {FormattedMessage, defineMessages, useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {GenericModal} from '@mattermost/components';
import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';

import {getPostSeenBy} from 'mattermost-redux/actions/read_receipts';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {makeGetReactionsForPost} from 'mattermost-redux/selectors/entities/posts';
import {getPostSeenBy as getCachedSeenBy} from 'mattermost-redux/selectors/entities/read_receipts';
import {getUser, makeGetDisplayName} from 'mattermost-redux/selectors/entities/users';

import RenderEmoji from 'components/emoji/render_emoji';
import LoadingSpinner from 'components/widgets/loading/loading_spinner';
import Avatar from 'components/widgets/users/avatar';

import type {GlobalState} from 'types/store';

import './post_seen_by_modal.scss';

const messages = defineMessages({
    title: {
        id: 'post_info.read_receipts.seen_by',
        defaultMessage: 'Seen by',
    },
});

type Props = {
    postId: Post['id'];
    onExited?: () => void;
};

function UserRow({userId, children}: {userId: UserProfile['id']; children?: React.ReactNode}) {
    const getDisplayName = useMemo(() => makeGetDisplayName(), []);
    const displayName = useSelector((state: GlobalState) => getDisplayName(state, userId));
    const user = useSelector((state: GlobalState) => getUser(state, userId));

    return (
        <li className='PostSeenByModal__row'>
            <Avatar
                size='sm'
                url={Client4.getProfilePictureUrl(userId, user?.last_picture_update)}
                username={user?.username}
            />
            <span className='PostSeenByModal__name'>{displayName}</span>
            {children}
        </li>
    );
}

export default function PostSeenByModal({postId, onExited}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const getReactionsForPost = useMemo(() => makeGetReactionsForPost(), []);
    const reactions = useSelector((state: GlobalState) => getReactionsForPost(state, postId));
    const seenBy = useSelector((state: GlobalState) => getCachedSeenBy(state, postId));
    const [loading, setLoading] = useState(true);
    const [failed, setFailed] = useState(false);

    useEffect(() => {
        dispatch(getPostSeenBy(postId)).then(({error}) => {
            setFailed(Boolean(error));
            setLoading(false);
        });
    }, [dispatch, postId]);

    const reactionList = useMemo(() => Object.values(reactions ?? {}), [reactions]);

    useEffect(() => {
        if (reactionList.length > 0) {
            dispatch(getMissingProfilesByIds(reactionList.map((reaction) => reaction.user_id)));
        }
    }, [dispatch, reactionList]);

    let viewers;
    if (loading && !seenBy) {
        viewers = <LoadingSpinner/>;
    } else if (seenBy?.length) {
        viewers = (
            <ul className='PostSeenByModal__list'>
                {seenBy.map((userId) => (
                    <UserRow
                        key={userId}
                        userId={userId}
                    />
                ))}
            </ul>
        );
    } else if (failed && !seenBy) {
        viewers = (
            <p className='PostSeenByModal__empty'>
                <FormattedMessage
                    id='post_info.read_receipts.unavailable'
                    defaultMessage='Read receipts aren’t available for this message.'
                />
            </p>
        );
    } else {
        viewers = (
            <p
                className='PostSeenByModal__empty'
                data-testid='postSeenByModalEmpty'
            >
                <FormattedMessage
                    id='post_info.read_receipts.empty'
                    defaultMessage='No one has seen this message yet.'
                />
            </p>
        );
    }

    return (
        <GenericModal
            id='postSeenByModal'
            className='PostSeenByModal a11y__modal'
            onExited={onExited}
            compassDesign={true}
            modalHeaderText={formatMessage(messages.title)}
        >
            <section className='PostSeenByModal__section'>
                {viewers}
            </section>
            {reactionList.length > 0 && (
                <section className='PostSeenByModal__section'>
                    <h3 className='PostSeenByModal__heading'>
                        <FormattedMessage
                            id='post_info.read_receipts.reactions'
                            defaultMessage='Reactions'
                        />
                    </h3>
                    <ul className='PostSeenByModal__list'>
                        {reactionList.map((reaction) => (
                            <UserRow
                                key={`${reaction.user_id}-${reaction.emoji_name}`}
                                userId={reaction.user_id}
                            >
                                <span className='PostSeenByModal__emoji'>
                                    <RenderEmoji
                                        emojiName={reaction.emoji_name}
                                        size={18}
                                    />
                                </span>
                            </UserRow>
                        ))}
                    </ul>
                </section>
            )}
        </GenericModal>
    );
}
