// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Runs pele off the main thread: render() is synchronous, so a hostile diagram can only be stopped by
// terminating this worker. Only the SVG string (or null on failure) is posted back; it is sanitized by the caller.

// The page sets webpack's public path at runtime for subpath installs (root.tsx), but a worker has its own
// webpack runtime, so load chunks from the directory this worker script was served from.
// @ts-expect-error Field used by Webpack internally
__webpack_public_path__ = self.location.href.slice(0, self.location.href.lastIndexOf('/') + 1);

const MAX_EDGES = 500;

export async function renderMermaidSvg(source: string): Promise<string | null> {
    try {
        const {render} = await import('pele');
        return render(source, {links: false, images: false, maxEdges: MAX_EDGES}).svg;
    } catch {
        return null;
    }
}

self.onmessage = async (event: MessageEvent) => {
    self.postMessage(typeof event.data === 'string' ? await renderMermaidSvg(event.data) : null);
};
