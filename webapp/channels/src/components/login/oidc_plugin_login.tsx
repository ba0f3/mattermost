// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import type {ReactNode} from 'react';
import {useIntl} from 'react-intl';

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import {Client4} from 'mattermost-redux/client';

import ExternalLoginButton from 'components/external_login_button/external_login_button';
import ColumnLayout from 'components/header_footer_route/content_layouts/column';
import LoadingScreen from 'components/loading_screen';
import LoginOpenIDIcon from 'components/widgets/icons/login_openid_icon';

import {navigateTo} from 'utils/browser_utils';
import {isEmbedded} from 'utils/embed';

type State = {status: 'loading'} | {status: 'unavailable'} | {
    status: 'ready';
    buttonText: string;
    buttonColor: string;
};

type Props = {
    search: string;
    fallback: ReactNode;
};

// The plugin's DOM-injected button cannot mount on the native "no methods"
// screen. Resolve it here when no native authentication method is available.
const OIDCPluginLogin = ({search, fallback}: Props) => {
    const {formatMessage} = useIntl();
    const [state, setState] = useState<State>({status: 'loading'});
    const redirected = useRef(false);
    const desktop = isDesktopApp();
    const params = new URLSearchParams(search);
    const manual = params.has('oidc_manual') || params.has('extra') || params.has('error') || isEmbedded();
    const requestedReturnTo = params.get('redirect_to');
    const siteURL = Client4.getUrl().replace(/\/$/, '');
    const basePath = new URL(siteURL || '/', window.location.origin).pathname;
    let returnTo = basePath.endsWith('/') ? basePath : `${basePath}/`;
    if (requestedReturnTo?.startsWith('/') && !requestedReturnTo.startsWith('//')) {
        const target = new URL(requestedReturnTo, window.location.origin);
        if (target.origin === window.location.origin) {
            returnTo = target.pathname + target.search + target.hash;
        }
    }
    const pluginURL = `${siteURL}/plugins/mattermost-oidc`;
    const loginURL = `${pluginURL}/oauth2/connect?return_to=${encodeURIComponent(returnTo)}`;

    useEffect(() => {
        const controller = new AbortController();
        const loadConfig = async () => {
            try {
                const response = await fetch(`${pluginURL}/api/v1/config`, {signal: controller.signal});
                const config: unknown = response.ok ? await response.json() : null;
                if (controller.signal.aborted) {
                    return;
                }
                if (typeof config !== 'object' || config === null || !('enable' in config) || config.enable !== true) {
                    setState({status: 'unavailable'});
                    return;
                }
                setState({
                    status: 'ready',
                    buttonText: 'button_text' in config && typeof config.button_text === 'string' ? config.button_text : '',
                    buttonColor: 'button_color' in config && typeof config.button_color === 'string' ? config.button_color : '',
                });
            } catch {
                if (!controller.signal.aborted) {
                    setState({status: 'unavailable'});
                }
            }
        };
        loadConfig();
        return () => controller.abort();
    }, [pluginURL]);

    useEffect(() => {
        if (state.status === 'ready' && !desktop && !manual && !redirected.current) {
            redirected.current = true;
            navigateTo(loginURL);
        }
    }, [state.status, desktop, manual, loginURL]);

    useEffect(() => {
        if (!desktop || state.status !== 'ready') {
            return undefined;
        }

        // The plugin callback broadcasts completion when Electron severs the
        // popup's window.opener during the Keycloak round trip.
        const onStorage = (event: StorageEvent) => {
            if (event.key === 'mattermost_oidc_login' && event.newValue) {
                navigateTo(returnTo);
            }
        };
        window.addEventListener('storage', onStorage);
        return () => window.removeEventListener('storage', onStorage);
    }, [desktop, state.status, returnTo]);

    if (state.status === 'loading') {
        return <LoadingScreen/>;
    }
    if (state.status === 'unavailable') {
        return <>{fallback}</>;
    }

    const handleClick = (event: React.MouseEvent<HTMLAnchorElement>) => {
        if (desktop) {
            event.preventDefault();
            const popup = window.open(`${loginURL}&popup=1`, 'oidc_login', 'width=520,height=680');
            if (!popup) {
                navigateTo(loginURL);
            }
        }
    };

    return (
        <ColumnLayout
            title={formatMessage({id: 'login.title', defaultMessage: 'Log in to your account'})}
            message={formatMessage({id: 'login.cardtitle', defaultMessage: 'Log in'})}
            extraContent={
                <ExternalLoginButton
                    id='oidc-plugin'
                    url={loginURL}
                    icon={<LoginOpenIDIcon/>}
                    label={state.buttonText || formatMessage({id: 'login.openid', defaultMessage: 'Open ID'})}
                    style={state.buttonColor ? {color: state.buttonColor, borderColor: state.buttonColor} : undefined}
                    onClick={handleClick}
                />
            }
        />
    );
};

export default OIDCPluginLogin;
