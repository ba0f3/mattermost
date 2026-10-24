// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Webpack 5 emits the worker (and pele with it) as a separate chunk, so neither is part of the main bundle.
export default function createMermaidWorker() {
    return new Worker(new URL('./markdown_preview_mermaid.worker.ts', import.meta.url));
}
