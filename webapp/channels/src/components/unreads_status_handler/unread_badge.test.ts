// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {
    drawBadgedIcon,
    formatBadgeCount,
    renderBadgedFavicons,
    restoreFavicons,
    updateAppBadge,
} from './unread_badge';

// jsdom doesn't load images, so make them load (or fail to load if the URL contains "broken") as soon as src is set
function mockImageLoading() {
    const OriginalImage = window.Image;
    const imageSpy = jest.spyOn(window, 'Image').mockImplementation(() => {
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
    return imageSpy;
}

function addFavicon(href: string, sizes?: string) {
    const link = document.createElement('link');
    link.rel = 'icon';
    link.href = href;
    if (sizes) {
        link.setAttribute('sizes', sizes);
    }
    document.head.appendChild(link);
    return link;
}

describe('formatBadgeCount', () => {
    test.each([
        [0, ''],
        [-1, ''],
        [NaN, ''],
        [1, '1'],
        [9, '9'],
        [42, '42'],
        [99, '99'],
        [100, '99+'],
        [12345, '99+'],
    ])('should format %p as %p', (count, expected) => {
        expect(formatBadgeCount(count)).toBe(expected);
    });
});

describe('drawBadgedIcon', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('should draw the original icon with the count over it', () => {
        const image = new Image();
        const fillText = jest.spyOn(CanvasRenderingContext2D.prototype, 'fillText');
        const drawImage = jest.spyOn(CanvasRenderingContext2D.prototype, 'drawImage');

        const result = drawBadgedIcon(image, 32, '99+');

        expect(result).toMatch(/^data:image\/png/);
        expect(drawImage).toHaveBeenCalledWith(image, 0, 0, 32, 32);
        expect(fillText).toHaveBeenCalledWith('99+', expect.any(Number), expect.any(Number), expect.any(Number));
    });

    test('should draw a dot without text when there is no label', () => {
        const fillText = jest.spyOn(CanvasRenderingContext2D.prototype, 'fillText');
        const arc = jest.spyOn(CanvasRenderingContext2D.prototype, 'arc');

        drawBadgedIcon(new Image(), 16, '');

        expect(fillText).not.toHaveBeenCalled();
        expect(arc).toHaveBeenCalledTimes(1);
    });

    test('should throw when canvas is not supported', () => {
        jest.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValueOnce(null);

        expect(() => drawBadgedIcon(new Image(), 16, '1')).toThrow();
    });
});

describe('renderBadgedFavicons and restoreFavicons', () => {
    beforeEach(() => {
        mockImageLoading();
    });

    afterEach(() => {
        jest.restoreAllMocks();
        document.head.innerHTML = '';
    });

    test('should render every favicon from its original image and restore it afterwards', async () => {
        const link16 = addFavicon('http://localhost:8065/static/custom-16.png', '16x16');
        const link32 = addFavicon('http://localhost:8065/static/custom-32.png', '32x32');
        const drawImage = jest.spyOn(CanvasRenderingContext2D.prototype, 'drawImage');

        const favicons = await renderBadgedFavicons(5);
        expect(favicons).toEqual([
            [link16, expect.stringMatching(/^data:image\/png/)],
            [link32, expect.stringMatching(/^data:image\/png/)],
        ]);
        expect(drawImage).toHaveBeenCalledWith(expect.objectContaining({src: 'http://localhost:8065/static/custom-16.png'}), 0, 0, 16, 16);
        expect(drawImage).toHaveBeenCalledWith(expect.objectContaining({src: 'http://localhost:8065/static/custom-32.png'}), 0, 0, 32, 32);

        // Nothing is applied until the caller does so
        expect(link16.href).toBe('http://localhost:8065/static/custom-16.png');

        for (const [link, href] of favicons) {
            link.href = href;
        }

        // Re-rendering still draws over the original icon instead of the badged one
        drawImage.mockClear();
        await renderBadgedFavicons(6);
        expect(drawImage).toHaveBeenCalledWith(expect.objectContaining({src: 'http://localhost:8065/static/custom-16.png'}), 0, 0, 16, 16);

        restoreFavicons();
        expect(link16.href).toBe('http://localhost:8065/static/custom-16.png');
        expect(link32.href).toBe('http://localhost:8065/static/custom-32.png');
    });

    test('should reject when an icon fails to load', async () => {
        addFavicon('http://localhost:8065/static/broken.png', '16x16');

        await expect(renderBadgedFavicons(1)).rejects.toThrow();
    });

    test('should reject when there is no favicon', async () => {
        await expect(renderBadgedFavicons(1)).rejects.toThrow();
    });
});

describe('updateAppBadge', () => {
    const originalSetAppBadge = Object.getOwnPropertyDescriptor(navigator, 'setAppBadge');
    const originalClearAppBadge = Object.getOwnPropertyDescriptor(navigator, 'clearAppBadge');

    function mockAppBadge(setAppBadge: jest.Mock, clearAppBadge: jest.Mock) {
        Object.defineProperty(navigator, 'setAppBadge', {value: setAppBadge, configurable: true});
        Object.defineProperty(navigator, 'clearAppBadge', {value: clearAppBadge, configurable: true});
    }

    afterEach(() => {
        for (const [name, descriptor] of [['setAppBadge', originalSetAppBadge], ['clearAppBadge', originalClearAppBadge]] as const) {
            if (descriptor) {
                Object.defineProperty(navigator, name, descriptor);
            } else {
                Reflect.deleteProperty(navigator, name);
            }
        }
    });

    test('should do nothing when the API is not available', () => {
        expect('setAppBadge' in navigator).toBe(false);
        expect(() => updateAppBadge(true, 3)).not.toThrow();
    });

    test('should show the mention count, an indicator for unreads, and clear it otherwise', () => {
        const setAppBadge = jest.fn().mockResolvedValue(undefined);
        const clearAppBadge = jest.fn().mockResolvedValue(undefined);
        mockAppBadge(setAppBadge, clearAppBadge);

        updateAppBadge(true, 150);
        expect(setAppBadge).toHaveBeenLastCalledWith(150);

        updateAppBadge(true, 0);
        expect(setAppBadge).toHaveBeenCalledTimes(2);
        expect(setAppBadge).toHaveBeenLastCalledWith();

        expect(clearAppBadge).not.toHaveBeenCalled();
        updateAppBadge(false, 0);
        expect(clearAppBadge).toHaveBeenCalledTimes(1);
        expect(setAppBadge).toHaveBeenCalledTimes(2);
    });

    test('should ignore errors from the API', async () => {
        const setAppBadge = jest.fn().mockRejectedValue(new Error('Not allowed'));
        const clearAppBadge = jest.fn().mockImplementation(() => {
            throw new Error('Not allowed');
        });
        mockAppBadge(setAppBadge, clearAppBadge);

        expect(() => updateAppBadge(true, 1)).not.toThrow();
        expect(() => updateAppBadge(false, 0)).not.toThrow();

        // Let the rejected promise settle so that an unhandled rejection would fail the test
        await Promise.resolve();
        expect(setAppBadge).toHaveBeenCalledWith(1);
    });
});
