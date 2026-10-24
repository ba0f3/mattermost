// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import nock from 'nock';
import React from 'react';

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import {Client4} from 'mattermost-redux/client';

import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {navigateTo} from 'utils/browser_utils';

import OIDCPluginLogin from './oidc_plugin_login';

jest.unmock('react-intl');
jest.mock('@mattermost/shared/utils/user_agent', () => ({isDesktopApp: jest.fn()}));
jest.mock('utils/browser_utils', () => ({navigateTo: jest.fn()}));

describe('OIDC plugin as the only login method', () => {
    const pluginURL = 'https://mattermost.example.test/chat/plugins/mattermost-oidc';
    const originalClientURL = Client4.getUrl();

    beforeEach(() => {
        jest.mocked(isDesktopApp).mockReturnValue(false);
        Client4.setUrl('https://mattermost.example.test/chat');
    });

    afterEach(() => {
        nock.cleanAll();
        jest.restoreAllMocks();
        Client4.setUrl(originalClientURL);
    });

    const renderLogin = (search = '') => renderWithContext(
        <OIDCPluginLogin
            search={search}
            fallback='No sign-in methods'
        />,
        {},
        {history: createMemoryHistory()},
    );

    const enablePlugin = () => nock(pluginURL).get('/api/v1/config').reply(200, {
        enable: true,
        button_text: 'Keycloak',
        button_color: '#0058CC',
    });

    it('redirects the browser through the plugin and preserves the channel destination and server subpath', async () => {
        const config = enablePlugin();
        renderLogin('?redirect_to=%2Fchat%2F568e%2Fchannels%2Ftown-square');

        await waitFor(() => expect(navigateTo).toHaveBeenCalledWith(`${pluginURL}/oauth2/connect?return_to=%2Fchat%2F568e%2Fchannels%2Ftown-square`));
        expect(config.isDone()).toBe(true);
        expect(screen.queryByText('No sign-in methods')).not.toBeInTheDocument();
    });

    it.each(['?oidc_manual=1', '?error=access_denied', '?extra=expired'])('keeps a usable login button without redirecting for %s', async (search) => {
        enablePlugin();
        renderLogin(search);

        expect(await screen.findByRole('link', {name: /Keycloak/})).toHaveAttribute('href', `${pluginURL}/oauth2/connect?return_to=%2Fchat%2F`);
        expect(navigateTo).not.toHaveBeenCalled();
    });

    it.each([
        null,
        {enable: false},
        {enable: 'true'},
    ])('retains the no-methods message for a disabled or invalid plugin config: %j', async (config) => {
        nock(pluginURL).get('/api/v1/config').reply(200, JSON.stringify(config), {'Content-Type': 'application/json'});
        renderLogin();

        expect(await screen.findByText('No sign-in methods')).toBeVisible();
        expect(navigateTo).not.toHaveBeenCalled();
    });

    it('retains the no-methods message when the plugin is not installed', async () => {
        nock(pluginURL).get('/api/v1/config').reply(404);
        renderLogin();

        expect(await screen.findByText('No sign-in methods')).toBeVisible();
        expect(navigateTo).not.toHaveBeenCalled();
    });

    it('retains the no-methods message when the plugin request fails', async () => {
        nock(pluginURL).get('/api/v1/config').replyWithError('Connection closed');
        renderLogin();

        expect(await screen.findByText('No sign-in methods')).toBeVisible();
        expect(navigateTo).not.toHaveBeenCalled();
    });

    it.each(['https://evil.test', '//evil.test', '/\\evil.test', '/\n/evil.test'])('does not forward an external return destination: %j', async (destination) => {
        enablePlugin();
        renderLogin(`?redirect_to=${encodeURIComponent(destination)}`);

        await waitFor(() => expect(navigateTo).toHaveBeenCalledWith(`${pluginURL}/oauth2/connect?return_to=%2Fchat%2F`));
    });

    it('opens a popup on a desktop click and reloads the destination when the plugin signals completion', async () => {
        jest.mocked(isDesktopApp).mockReturnValue(true);
        const open = jest.spyOn(window, 'open').mockReturnValue(window);
        enablePlugin();
        renderLogin('?redirect_to=%2Fchat%2F568e%2Fchannels%2Ftown-square');

        const link = await screen.findByRole('link', {name: /Keycloak/});
        expect(navigateTo).not.toHaveBeenCalled();
        expect(open).not.toHaveBeenCalled();
        await userEvent.click(link);
        expect(open).toHaveBeenCalledWith(`${pluginURL}/oauth2/connect?return_to=%2Fchat%2F568e%2Fchannels%2Ftown-square&popup=1`, 'oidc_login', 'width=520,height=680');

        act(() => window.dispatchEvent(new StorageEvent('storage', {key: 'mattermost_oidc_login', newValue: 'done'})));
        expect(navigateTo).toHaveBeenCalledWith('/chat/568e/channels/town-square');
    });
});
