// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import type {ComponentProps} from 'react';

import {isChrome, isFirefox} from '@mattermost/shared/utils/user_agent';
import type {ChannelType} from '@mattermost/types/channels';
import type {TeamType} from '@mattermost/types/teams';

import UnreadsStatusHandler, {UnreadsStatusHandlerClass} from 'components/unreads_status_handler/unreads_status_handler';

import {renderWithContext, waitFor} from 'tests/react_testing_utils';
import {Constants} from 'utils/constants';
import {TestHelper} from 'utils/test_helper';

type Props = ComponentProps<typeof UnreadsStatusHandlerClass>;

// jsdom doesn't load images, so make them load (or fail to load if the URL contains "broken") as soon as src is set
function mockImageLoading() {
    const OriginalImage = window.Image;
    return jest.spyOn(window, 'Image').mockImplementation(() => {
        const image = new OriginalImage();
        Object.defineProperty(image, 'src', {
            set(value: string) {
                image.setAttribute('src', value);
                Promise.resolve().then(() => {
                    if (value.includes('broken')) {
                        image.onerror?.(new Event('error'));
                    } else {
                        image.onload?.(new Event('load'));
                    }
                });
            },
            get() {
                return image.getAttribute('src');
            },
        });
        return image;
    });
}

jest.mock('@mattermost/shared/utils/user_agent', () => {
    const original = jest.requireActual('@mattermost/shared/utils/user_agent');
    return {
        ...original,
        isFirefox: jest.fn().mockReturnValue(true),
        isChrome: jest.fn(),
    };
});

describe('components/UnreadsStatusHandler', () => {
    const defaultProps = {
        unreadStatus: false,
        siteName: 'Test site',
        currentChannel: TestHelper.getChannelMock({
            id: 'c1',
            display_name: 'Public test 1',
            name: 'public-test-1',
            type: Constants.OPEN_CHANNEL as ChannelType,
        }),
        currentTeam: TestHelper.getTeamMock({
            id: 'team_id',
            name: 'test-team',
            display_name: 'Test team display name',
            description: 'Test team description',
            type: 'team-type' as TeamType,
        }),
        currentTeammate: null,
        inGlobalThreads: false,
        inDrafts: false,
        inScheduledPosts: false,
    };

    test('set correctly the title when needed', () => {
        // Render with slightly different prop to trigger componentDidUpdate on first rerender
        const {rerender} = renderWithContext(
            <UnreadsStatusHandler
                {...defaultProps}
                unreadStatus={true}
            />,
        );

        // Track cumulative props like Enzyme's setProps
        let currentProps: any = {...defaultProps};

        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('Public test 1 - Test team display name Test site');

        currentProps = {...currentProps, siteName: undefined};
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('Public test 1 - Test team display name');

        currentProps = {
            ...currentProps,
            currentChannel: {id: '1', type: Constants.DM_CHANNEL} as Props['currentChannel'],
            currentTeammate: {display_name: 'teammate'} as Props['currentTeammate'],
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('teammate - Test team display name');

        currentProps = {...currentProps, unreadStatus: 3};
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('(3) teammate - Test team display name');

        currentProps = {
            ...currentProps,
            currentChannel: {} as Props['currentChannel'],
            currentTeammate: {} as Props['currentTeammate'],
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('Mattermost - Join a team');

        currentProps = {
            ...currentProps,
            inDrafts: false,
            inScheduledPosts: true,
            unreadStatus: 0,
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('Scheduled - Test team display name');

        currentProps = {
            ...currentProps,
            inDrafts: false,
            inScheduledPosts: true,
            unreadStatus: 10,
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('(10) Scheduled - Test team display name');

        currentProps = {
            ...currentProps,
            inDrafts: true,
            inScheduledPosts: false,
            unreadStatus: 0,
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('Drafts - Test team display name');

        currentProps = {
            ...currentProps,
            inDrafts: true,
            inScheduledPosts: false,
            unreadStatus: 10,
        };
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('(10) Drafts - Test team display name');
    });

    test('should set correct title on mentions on safari', () => {
        // in safari browser, modification of favicon is not
        // supported, hence we need to show * in title on mentions
        (isFirefox as jest.Mock).mockImplementation(() => false);
        (isChrome as jest.Mock).mockImplementation(() => false);

        const {rerender} = renderWithContext(
            <UnreadsStatusHandler {...defaultProps}/>,
        );

        let currentProps: any = {...defaultProps, siteName: undefined};
        currentProps = {
            ...currentProps,
            currentChannel: {id: '1', type: Constants.DM_CHANNEL} as Props['currentChannel'],
            currentTeammate: {display_name: 'teammate'} as Props['currentTeammate'],
        };
        currentProps = {...currentProps, unreadStatus: 3};
        rerender(<UnreadsStatusHandler {...currentProps}/>);
        expect(document.title).toBe('(3) * teammate - Test team display name');
    });

    test('should display correct favicon', () => {
        const sizes = ['16x16', '24x24', '32x32', '64x64', '96x96'];
        sizes.forEach((size) => {
            const link = document.createElement('link');
            link.rel = 'icon';
            link.setAttribute('sizes', size);
            document.head.appendChild(link);
        });

        (isFirefox as jest.Mock).mockReturnValue(true);

        // Spy on getBadgeStatus to verify the correct badge status is computed
        // (updateFavicon is an arrow function so we can't spy on it directly)
        const getBadgeStatusSpy = jest.spyOn(UnreadsStatusHandlerClass.prototype, 'getBadgeStatus');

        const {rerender} = renderWithContext(
            <UnreadsStatusHandler {...defaultProps}/>,
        );

        rerender(
            <UnreadsStatusHandler
                {...defaultProps}
                unreadStatus={3}
            />,
        );
        expect(getBadgeStatusSpy).toHaveLastReturnedWith('Mention');

        rerender(
            <UnreadsStatusHandler
                {...defaultProps}
                unreadStatus={true}
            />,
        );
        expect(getBadgeStatusSpy).toHaveLastReturnedWith('Unread');

        rerender(
            <UnreadsStatusHandler
                {...defaultProps}
                unreadStatus={false}
            />,
        );
        expect(getBadgeStatusSpy).toHaveLastReturnedWith('None');

        getBadgeStatusSpy.mockRestore();

        // Clean up
        sizes.forEach((size) => {
            const link = document.querySelector(`link[rel="icon"][sizes="${size}"]`);
            if (link) {
                link.remove();
            }
        });
    });

    test('should display correct title when in drafts', () => {
        const {rerender} = renderWithContext(
            <UnreadsStatusHandler {...defaultProps}/>,
        );

        rerender(
            <UnreadsStatusHandler
                {...defaultProps}
                inDrafts={true}
                currentChannel={undefined}
                siteName={undefined}
            />,
        );

        expect(document.title).toBe('Drafts - Test team display name');
    });

    test('should display correct title when in scheduled posts tab', () => {
        const {rerender} = renderWithContext(
            <UnreadsStatusHandler {...defaultProps}/>,
        );

        rerender(
            <UnreadsStatusHandler
                {...defaultProps}
                inScheduledPosts={true}
                currentChannel={undefined}
                siteName={undefined}
            />,
        );

        expect(document.title).toBe('Scheduled - Test team display name');
    });

    describe('badges', () => {
        const sizes = ['16x16', '32x32'];

        function addFavicons(name: string) {
            return sizes.map((size) => {
                const link = document.createElement('link');
                link.rel = 'icon';
                link.href = `http://localhost:8065/static/${name}-${size}.png`;
                link.setAttribute('sizes', size);
                document.head.appendChild(link);
                return link;
            });
        }

        beforeEach(() => {
            mockImageLoading();
        });

        afterEach(() => {
            jest.restoreAllMocks();
            document.head.querySelectorAll('link[rel="icon"]').forEach((link) => link.remove());
            Reflect.deleteProperty(navigator, 'setAppBadge');
            Reflect.deleteProperty(navigator, 'clearAppBadge');
        });

        test('should draw a count badge over the current favicon and restore it when everything is read', async () => {
            const links = addFavicons('custom-favicon');
            const fillText = jest.spyOn(CanvasRenderingContext2D.prototype, 'fillText');

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={150}
                />,
            );

            await waitFor(() => {
                for (const link of links) {
                    expect(link.href).toMatch(/^data:image\/png/);
                }
            });
            expect(fillText).toHaveBeenCalledWith('99+', expect.any(Number), expect.any(Number), expect.any(Number));

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={false}
                />,
            );

            expect(links[0].href).toBe('http://localhost:8065/static/custom-favicon-16x16.png');
            expect(links[1].href).toBe('http://localhost:8065/static/custom-favicon-32x32.png');
        });

        test('should draw a dot when there are unread messages without mentions', async () => {
            const links = addFavicons('dot-favicon');
            const fillText = jest.spyOn(CanvasRenderingContext2D.prototype, 'fillText');

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);
            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={true}
                />,
            );

            await waitFor(() => expect(links[0].href).toMatch(/^data:image\/png/));
            expect(fillText).not.toHaveBeenCalled();
        });

        test('should draw the badge when mounted with unreads', async () => {
            const links = addFavicons('mount-favicon');

            renderWithContext(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={2}
                />,
            );

            await waitFor(() => expect(links[0].href).toMatch(/^data:image\/png/));
        });

        test('should fall back to the bundled icons when the badge cannot be drawn', async () => {
            const links = addFavicons('broken-favicon');

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);
            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={3}
                />,
            );

            // The bundled mention icons are mocked as "test-file-stub" in tests
            await waitFor(() => {
                for (const link of links) {
                    expect(link.getAttribute('href')).toBe('test-file-stub');
                }
            });

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={false}
                />,
            );
            expect(links[0].href).toBe('http://localhost:8065/static/broken-favicon-16x16.png');
        });

        test('should not redraw the badges when what they show has not changed', async () => {
            const links = addFavicons('unchanged-favicon');
            const setAppBadge = jest.fn().mockResolvedValue(undefined);
            const clearAppBadge = jest.fn().mockResolvedValue(undefined);
            Object.assign(navigator, {setAppBadge, clearAppBadge});

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);
            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={150}
                />,
            );
            await waitFor(() => expect(links[0].href).toMatch(/^data:image\/png/));

            const toDataURL = jest.spyOn(HTMLCanvasElement.prototype, 'toDataURL');
            toDataURL.mockClear();

            // Both counts are shown as 99+, and changing channel doesn't change the unread status
            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={200}
                />,
            );
            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={200}
                    inDrafts={true}
                />,
            );
            await Promise.resolve();

            expect(toDataURL).not.toHaveBeenCalled();
            expect(setAppBadge).toHaveBeenCalledTimes(1);
            expect(setAppBadge).toHaveBeenCalledWith(150);
        });

        test('should set and clear the app badge when it is supported', () => {
            const setAppBadge = jest.fn().mockResolvedValue(undefined);
            const clearAppBadge = jest.fn().mockRejectedValue(new Error('Not installed'));
            Object.assign(navigator, {setAppBadge, clearAppBadge});

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);
            expect(clearAppBadge).toHaveBeenCalledTimes(1);

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={7}
                />,
            );
            expect(setAppBadge).toHaveBeenLastCalledWith(7);

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={true}
                />,
            );
            expect(setAppBadge).toHaveBeenLastCalledWith();

            rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={false}
                />,
            );
            expect(clearAppBadge).toHaveBeenCalledTimes(2);
        });

        test('should clear badges left over from a previous mount', async () => {
            const links = addFavicons('remount-favicon');
            const setAppBadge = jest.fn().mockResolvedValue(undefined);
            const clearAppBadge = jest.fn().mockResolvedValue(undefined);
            Object.assign(navigator, {setAppBadge, clearAppBadge});

            const {unmount} = renderWithContext(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={4}
                />,
            );
            await waitFor(() => expect(links[0].href).toMatch(/^data:image\/png/));
            expect(setAppBadge).toHaveBeenLastCalledWith(4);
            unmount();

            expect(clearAppBadge).not.toHaveBeenCalled();

            renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);

            expect(links[0].href).toBe('http://localhost:8065/static/remount-favicon-16x16.png');
            expect(clearAppBadge).toHaveBeenCalledTimes(1);
        });

        test('should not set the app badge when it is not supported', () => {
            expect('setAppBadge' in navigator).toBe(false);

            const {rerender} = renderWithContext(<UnreadsStatusHandler {...defaultProps}/>);

            expect(() => rerender(
                <UnreadsStatusHandler
                    {...defaultProps}
                    unreadStatus={7}
                />,
            )).not.toThrow();
        });
    });
});
