// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export const MAX_BADGE_COUNT = 99;

const BADGE_COLOR = '#d24b4e';
const BADGE_TEXT_COLOR = '#ffffff';
const DEFAULT_ICON_SIZE = 32;
const ORIGINAL_HREF_ATTRIBUTE = 'data-original-href';

/**
 * Formats a mention count for display in a badge. Returns an empty string when there is nothing to show.
 */
export function formatBadgeCount(count: number): string {
    if (!Number.isFinite(count) || count <= 0) {
        return '';
    }

    if (count > MAX_BADGE_COUNT) {
        return `${MAX_BADGE_COUNT}+`;
    }

    return String(Math.floor(count));
}

export function getFaviconLinks(): HTMLLinkElement[] {
    return Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]'));
}

/**
 * Returns the href the link had before we started modifying it, so that badges are always drawn on top of the
 * favicon served by the server (which may be custom-branded) rather than on top of a previously badged icon.
 */
export function getOriginalFaviconHref(link: HTMLLinkElement): string {
    let href = link.getAttribute(ORIGINAL_HREF_ATTRIBUTE);
    if (href === null) {
        href = link.href;
        link.setAttribute(ORIGINAL_HREF_ATTRIBUTE, href);
    }
    return href;
}

export function restoreFavicons() {
    for (const link of getFaviconLinks()) {
        const href = link.getAttribute(ORIGINAL_HREF_ATTRIBUTE);
        if (href !== null && link.href !== href) {
            link.href = href;
        }
    }
}

const imageCache = new Map<string, Promise<HTMLImageElement>>();

function loadImage(src: string): Promise<HTMLImageElement> {
    let promise = imageCache.get(src);
    if (!promise) {
        promise = new Promise<HTMLImageElement>((resolve, reject) => {
            const image = new Image();
            image.onload = () => resolve(image);
            image.onerror = () => reject(new Error(`Failed to load favicon ${src}`));

            // Allows icons served from another origin with CORS enabled to be drawn without tainting the canvas
            image.crossOrigin = 'anonymous';
            image.src = src;
        });
        promise.catch(() => imageCache.delete(src));
        imageCache.set(src, promise);
    }
    return promise;
}

function getIconSize(link: HTMLLinkElement, image: HTMLImageElement): number {
    const match = (/^(\d+)x\d+$/i).exec(link.getAttribute('sizes') ?? '');
    if (match) {
        return parseInt(match[1], 10);
    }
    return image.naturalWidth || DEFAULT_ICON_SIZE;
}

/**
 * Draws the given icon onto a canvas with a badge in its top right corner. The badge shows the label when one is
 * given, or a dot otherwise. Returns the result as a data URL, and throws if the canvas can't be used (for example,
 * when the image is cross-origin and taints the canvas).
 */
export function drawBadgedIcon(image: HTMLImageElement, size: number, label: string): string {
    const canvas = document.createElement('canvas');
    canvas.width = size;
    canvas.height = size;

    const context = canvas.getContext('2d');
    if (!context) {
        throw new Error('Canvas is not supported');
    }

    context.drawImage(image, 0, 0, size, size);
    context.fillStyle = BADGE_COLOR;
    context.strokeStyle = BADGE_TEXT_COLOR;
    context.lineWidth = Math.max(1, size / 16);

    if (label) {
        const height = Math.round(size * 0.6);
        const radius = height / 2;
        context.font = `bold ${Math.round(height * 0.8)}px sans-serif`;
        const width = Math.min(size, Math.max(height, context.measureText(label).width + radius));
        const left = size - width;

        // Pill shape, which is a circle when the label is a single digit
        context.beginPath();
        context.arc(left + radius, radius, radius, Math.PI / 2, (3 * Math.PI) / 2);
        context.arc(size - radius, radius, radius, (3 * Math.PI) / 2, Math.PI / 2);
        context.closePath();
        context.fill();

        context.fillStyle = BADGE_TEXT_COLOR;
        context.textAlign = 'center';
        context.textBaseline = 'middle';
        context.fillText(label, left + (width / 2), radius + 1, width - (radius / 2));
    } else {
        const radius = size * 0.22;
        context.beginPath();
        context.arc(size - radius - 1, radius + 1, radius, 0, 2 * Math.PI);
        context.fill();
        context.stroke();
    }

    return canvas.toDataURL('image/png');
}

/**
 * Renders a badged version of every favicon on the page. Resolves with the new href for each link without applying
 * them so that the caller can discard results that are out of date. Rejects if any icon can't be drawn.
 */
export async function renderBadgedFavicons(mentionCount: number): Promise<Array<[HTMLLinkElement, string]>> {
    const label = formatBadgeCount(mentionCount);
    const links = getFaviconLinks();
    if (links.length === 0) {
        throw new Error('No favicon found');
    }

    return Promise.all(links.map(async (link): Promise<[HTMLLinkElement, string]> => {
        const image = await loadImage(getOriginalFaviconHref(link));
        return [link, drawBadgedIcon(image, getIconSize(link, image), label)];
    }));
}

/**
 * Sets the badge shown on the app icon when the web app is installed as a PWA. Shows the mention count when there
 * are mentions, a plain indicator when there are only unread messages, and clears the badge otherwise.
 */
export function updateAppBadge(isUnread: boolean, mentionCount: number) {
    if (typeof navigator === 'undefined' || typeof navigator.setAppBadge !== 'function' || typeof navigator.clearAppBadge !== 'function') {
        return;
    }

    try {
        let result;
        if (mentionCount > 0) {
            result = navigator.setAppBadge(mentionCount);
        } else if (isUnread) {
            result = navigator.setAppBadge();
        } else {
            result = navigator.clearAppBadge();
        }

        // The badge is only cosmetic, so ignore failures such as the app not being installed
        result?.catch(() => {});
    } catch {
        // Ignore browsers that expose the API but throw when it's called
    }
}
