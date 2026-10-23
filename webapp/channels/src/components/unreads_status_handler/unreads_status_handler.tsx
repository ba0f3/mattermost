// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {injectIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';

import * as UserAgent from '@mattermost/shared/utils/user_agent';
import type {Channel} from '@mattermost/types/channels';
import type {Team} from '@mattermost/types/teams';

import {basicUnreadMeta} from 'mattermost-redux/selectors/entities/channels';
import type {BasicUnreadStatus} from 'mattermost-redux/selectors/entities/channels';
import {ensureString} from 'mattermost-redux/utils/post_utils';

import faviconMention16x16 from 'images/favicon/favicon-mentions-16x16.png';
import faviconMention24x24 from 'images/favicon/favicon-mentions-24x24.png';
import faviconMention32x32 from 'images/favicon/favicon-mentions-32x32.png';
import faviconMention64x64 from 'images/favicon/favicon-mentions-64x64.png';
import faviconMention96x96 from 'images/favicon/favicon-mentions-96x96.png';
import faviconUnread16x16 from 'images/favicon/favicon-unread-16x16.png';
import faviconUnread24x24 from 'images/favicon/favicon-unread-24x24.png';
import faviconUnread32x32 from 'images/favicon/favicon-unread-32x32.png';
import faviconUnread64x64 from 'images/favicon/favicon-unread-64x64.png';
import faviconUnread96x96 from 'images/favicon/favicon-unread-96x96.png';
import {Constants} from 'utils/constants';
import DesktopApp from 'utils/desktop_api';

import {
    formatBadgeCount,
    getFaviconLinks,
    getOriginalFaviconHref,
    renderBadgedFavicons,
    restoreFavicons,
    updateAppBadge,
} from './unread_badge';

enum BadgeStatus {
    Mention = 'Mention',
    Unread = 'Unread',
    None = 'None',
}

type Props = {
    intl: IntlShape;
    unreadStatus: BasicUnreadStatus;
    siteName?: string;
    currentChannel?: Channel;
    currentTeam?: Team;
    currentTeammate: Channel | null;
    inGlobalThreads: boolean;
    inDrafts: boolean;
    inScheduledPosts: boolean;
};

export class UnreadsStatusHandlerClass extends React.PureComponent<Props> {
    // Incremented on every favicon change so that slower, out of date renders are discarded
    private faviconRequestId = 0;

    componentDidMount() {
        // Also clears any badges left over from when this was last mounted
        this.updateBadges();
    }

    componentDidUpdate(prevProps: Props) {
        this.updateTitle();

        if (this.getBadgeKey(prevProps.unreadStatus) !== this.getBadgeKey(this.props.unreadStatus)) {
            this.updateBadges();
        }

        this.updateDesktopApp();
    }

    componentWillUnmount() {
        // Discard any pending favicon render. The badges are left as they are, like the title, since this is also
        // rendered in popouts and the app badge is shared between windows.
        this.faviconRequestId++;
    }

    get isDynamicFaviconSupported() {
        return UserAgent.isChrome() || UserAgent.isFirefox();
    }

    getBadgeStatus(unreadStatus: BasicUnreadStatus) {
        if (typeof unreadStatus === 'number') {
            return BadgeStatus.Mention;
        } else if (unreadStatus) {
            return BadgeStatus.Unread;
        }
        return BadgeStatus.None;
    }

    // Identifies what the badges currently show, so that they're only redrawn when that changes
    getBadgeKey(unreadStatus: BasicUnreadStatus) {
        const {unreadMentionCount} = basicUnreadMeta(unreadStatus);
        return `${this.getBadgeStatus(unreadStatus)}:${formatBadgeCount(unreadMentionCount)}`;
    }

    updateBadges = () => {
        const {isUnread, unreadMentionCount} = basicUnreadMeta(this.props.unreadStatus);

        this.updateFavicon(this.getBadgeStatus(this.props.unreadStatus), unreadMentionCount);

        // The desktop app sets its own badge through updateDesktopApp
        if (!UserAgent.isDesktopApp()) {
            updateAppBadge(isUnread, unreadMentionCount);
        }
    };

    updateDesktopApp = () => {
        const {unreadStatus} = this.props;
        const {isUnread, unreadMentionCount} = basicUnreadMeta(unreadStatus);

        DesktopApp.updateUnreadsAndMentions(isUnread, unreadMentionCount);
    };

    updateTitle = () => {
        const {
            siteName,
            currentChannel,
            currentTeam,
            currentTeammate,
            unreadStatus,
            inGlobalThreads,
            inDrafts,
            inScheduledPosts,
        } = this.props;
        const {formatMessage} = this.props.intl;

        const currentSiteName = siteName || '';

        const {isUnread, unreadMentionCount} = basicUnreadMeta(unreadStatus);

        const mentionTitle = unreadMentionCount > 0 ? `(${unreadMentionCount}) ` : '';
        const unreadTitle = !this.isDynamicFaviconSupported && isUnread ? '* ' : '';

        if (currentChannel && currentTeam && currentChannel.id) {
            let currentChannelName = currentChannel.display_name;
            if (currentChannel.type === Constants.DM_CHANNEL) {
                if (currentTeammate != null) {
                    currentChannelName = currentTeammate.display_name;
                }
            }
            document.title = `${mentionTitle}${unreadTitle}${currentChannelName} - ${currentTeam.display_name} ${currentSiteName}`;
        } else if (currentTeam && inGlobalThreads) {
            document.title = formatMessage({
                id: 'globalThreads.title',
                defaultMessage: '{prefix}Threads - {displayName} {siteName}',
            }, {
                prefix: `${mentionTitle}${unreadTitle}`,
                displayName: currentTeam.display_name,
                siteName: currentSiteName,
            });
        } else if (currentTeam && inDrafts) {
            document.title = formatMessage({
                id: 'drafts.title',
                defaultMessage: '{prefix}Drafts - {displayName} {siteName}',
            }, {
                prefix: `${mentionTitle}${unreadTitle}`,
                displayName: currentTeam.display_name,
                siteName: currentSiteName,
            });
        } else if (currentTeam && inScheduledPosts) {
            document.title = formatMessage({
                id: 'scheduledPosts.title',
                defaultMessage: '{prefix}Scheduled - {displayName} {siteName}',
            }, {
                prefix: `${mentionTitle}${unreadTitle}`,
                displayName: currentTeam.display_name,
                siteName: currentSiteName,
            });
        } else {
            document.title = formatMessage({id: 'sidebar.team_select', defaultMessage: '{siteName} - Join a team'}, {siteName: currentSiteName || 'Mattermost'});
        }
    };

    updateFavicon = (badgeStatus: BadgeStatus, mentionCount: number) => {
        const requestId = ++this.faviconRequestId;

        if (badgeStatus === BadgeStatus.None) {
            restoreFavicons();
            return;
        }

        renderBadgedFavicons(mentionCount).then((favicons) => {
            if (requestId === this.faviconRequestId) {
                for (const [link, href] of favicons) {
                    link.href = href;
                }
            }
        }).catch(() => {
            if (requestId === this.faviconRequestId) {
                this.swapFavicons(badgeStatus);
            }
        });
    };

    // Fallback for when the badge can't be drawn over the current favicon
    swapFavicons = (badgeStatus: BadgeStatus.Mention | BadgeStatus.Unread) => {
        const favicons: Record<string, string> = badgeStatus === BadgeStatus.Mention ? {
            '16x16': faviconMention16x16,
            '24x24': faviconMention24x24,
            '32x32': faviconMention32x32,
            '64x64': faviconMention64x64,
            '96x96': faviconMention96x96,
        } : {
            '16x16': faviconUnread16x16,
            '24x24': faviconUnread24x24,
            '32x32': faviconUnread32x32,
            '64x64': faviconUnread64x64,
            '96x96': faviconUnread96x96,
        };

        for (const link of getFaviconLinks()) {
            const favicon = favicons[link.getAttribute('sizes') ?? ''];
            if (favicon) {
                // Record the original favicon so that it can be restored later
                getOriginalFaviconHref(link);
                link.href = ensureString(favicon);
            }
        }
    };

    render() {
        return null;
    }
}

export default injectIntl(UnreadsStatusHandlerClass);
