// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import createDOMPurify from 'dompurify';
import React from 'react';

import {act, renderWithContext, screen, waitFor} from 'tests/react_testing_utils';

import createMermaidWorker from './create_mermaid_worker';
import MermaidDiagram, {
    MERMAID_MAX_DIAGRAMS,
    MERMAID_MAX_SOURCE_LENGTH,
    MERMAID_RENDER_TIMEOUT,
    prepareMermaidSource,
    sanitizeMermaidSvg,
    splitMermaidBlocks,
} from './markdown_preview_mermaid';

// The worker module assigns webpack's public path, which only exists in a webpack build.
Object.assign(global, {__webpack_public_path__: ''});
const {renderMermaidSvg} = require('./markdown_preview_mermaid.worker');

type Respond = (source: string) => Promise<string | null> | string | null | undefined;

class FakeWorker {
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: (() => void) | null = null;
    postMessage = jest.fn((source: string) => {
        Promise.resolve(this.respond(source)).then((data) => {
            if (data !== undefined) {
                this.onmessage?.({data} as MessageEvent);
            }
        });
    });
    terminate = jest.fn();

    constructor(private respond: Respond) {}
}

// Simulates the worker by running the real worker render function (real pele) unless overridden.
const mockWorker = (respond: Respond = renderMermaidSvg) => {
    const worker = new FakeWorker(respond);
    (createMermaidWorker as jest.Mock).mockReturnValue(worker);
    return worker;
};

const blobs: Blob[] = [];
const readBlob = (blob: Blob) => new Promise<string>((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.readAsText(blob);
});

const segmentFor = (source: string) => splitMermaidBlocks('```mermaid\n' + source + '\n```')[0] as Extract<ReturnType<typeof splitMermaidBlocks>[number], {type: 'mermaid'}>;

const renderDiagram = (source: string) => renderWithContext(<MermaidDiagram segment={segmentFor(source)}/>);

describe('components/markdown_preview_mermaid', () => {
    beforeEach(() => {
        blobs.length = 0;
        URL.createObjectURL = jest.fn((blob: Blob) => {
            blobs.push(blob);
            return `blob:http://localhost:8065/${blobs.length}`;
        });
        URL.revokeObjectURL = jest.fn();
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    test('renders a simple diagram as an <img> with a blob URL of sanitized SVG', async () => {
        const worker = mockWorker();
        const {unmount} = renderDiagram('graph TD; A-->B');

        const img = await screen.findByTestId('markdown-preview-mermaid');
        expect(img.tagName).toBe('IMG');
        expect(img.getAttribute('src')).toBe('blob:http://localhost:8065/1');
        expect(worker.postMessage).toHaveBeenCalledWith('graph TD; A-->B');
        expect(worker.terminate).toHaveBeenCalled();

        expect(blobs[0].type).toBe('image/svg+xml');
        const svg = await readBlob(blobs[0]);
        expect(svg).toMatch(/^<svg[^>]+xmlns="http:\/\/www\.w3\.org\/2000\/svg"/);

        unmount();
        expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:http://localhost:8065/1');
    });

    test('HTML and script in node labels never yield script, foreignObject or on* attributes', async () => {
        mockWorker();
        renderDiagram([
            'graph TD',
            '  A["<script>alert(1)</script>"] --> B["<img src=x onerror=alert(2)>"]',
            '  B --> C["<foreignObject><body onload=alert(3)></body></foreignObject>"]',
            '  C --> D["`<a href="javascript:alert(4)">x</a>`"]',
        ].join('\n'));

        await screen.findByTestId('markdown-preview-mermaid');
        const svg = await readBlob(blobs[0]);
        const doc = new DOMParser().parseFromString(svg, 'image/svg+xml');

        expect(doc.querySelector('script, foreignObject, foreignobject, img, body')).toBeNull();
        for (const el of doc.querySelectorAll('*')) {
            for (const attr of el.attributes) {
                expect(attr.name).not.toMatch(/^on/i);
                expect(attr.value).not.toMatch(/javascript:|data:/i);
            }
        }
        expect(document.querySelector('script')).toBeNull();
    });

    test('sanitizer strips script, foreignObject, on* and hrefs from hostile SVG', () => {
        const svg = sanitizeMermaidSvg(createDOMPurify, [
            '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" onload="alert(1)">',
            '<script>alert(2)</script>',
            '<foreignObject><div onclick="alert(3)">x</div></foreignObject>',
            '<a href="javascript:alert(4)"><text>js</text></a>',
            '<a xlink:href="https://example.com"><text>ext</text></a>',
            '<image href="data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="/>',
            '<rect onmouseover="alert(5)" width="1" height="1"/>',
            '</svg>',
        ].join(''));

        expect(svg).not.toMatch(/<script|foreignobject|onload|onclick|onmouseover|javascript:|data:|example\.com/i);
        expect(svg).toContain('<rect');
        expect(svg).not.toContain('href');
        expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"');
    });

    test('removes click, link, callback and init directives before posting to the worker', async () => {
        const worker = mockWorker(() => '<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>');
        renderDiagram([
            '%%{init: {"securityLevel": "loose"}}%%',
            'graph TD',
            '  A-->B',
            '  click A href "javascript:alert(1)"',
            '  CLICK B call alert()',
            '  click A callback "tooltip"',
            '  link A "javascript:alert(2)"',
            '  callback B "alert"',
        ].join('\n'));

        await screen.findByTestId('markdown-preview-mermaid');
        const posted = worker.postMessage.mock.calls[0][0];
        expect(posted).toContain('A-->B');
        expect(posted).not.toMatch(/click|callback|link|javascript|init/i);
        expect(prepareMermaidSource('graph TD\n  click A "https://example.com"')).toBe('graph TD\n');
    });

    test('falls back to the code block without starting a worker when the input is oversized', async () => {
        mockWorker();
        const source = 'graph TD\n' + 'A-->B\n'.repeat(MERMAID_MAX_SOURCE_LENGTH / 6);
        const {container} = renderDiagram(source);

        expect(await screen.findByText('This diagram could not be rendered.')).toBeInTheDocument();
        expect(createMermaidWorker).not.toHaveBeenCalled();
        expect(container.querySelector('img')).toBeNull();
        expect(container.querySelector('.post-code')).toBeInTheDocument();
        expect(prepareMermaidSource('graph TD\n' + 'A-->B\n'.repeat(1000))).toBeNull();
    });

    test('falls back to the code block when rendering fails', async () => {
        mockWorker();
        const {container} = renderDiagram('this is not a diagram');

        expect(await screen.findByText('This diagram could not be rendered.')).toBeInTheDocument();
        expect(container.querySelector('img')).toBeNull();
        expect(container.querySelector('.post-code')?.textContent).toContain('this is not a diagram');
        expect(URL.createObjectURL).not.toHaveBeenCalled();
    });

    test('falls back when the worker errors', async () => {
        const worker = mockWorker(() => undefined);
        renderDiagram('graph TD; A-->B');

        act(() => worker.onerror?.());

        expect(await screen.findByText('This diagram could not be rendered.')).toBeInTheDocument();
        expect(worker.terminate).toHaveBeenCalled();
    });

    test('terminates the worker and falls back to the code block on timeout', async () => {
        jest.useFakeTimers();
        const worker = mockWorker(() => undefined);
        const {container} = renderDiagram('graph TD; A-->B');

        expect(worker.postMessage).toHaveBeenCalled();
        expect(worker.terminate).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(MERMAID_RENDER_TIMEOUT);
        });

        expect(worker.terminate).toHaveBeenCalled();
        expect(screen.getByText('This diagram could not be rendered.')).toBeInTheDocument();
        expect(container.querySelector('img')).toBeNull();
        expect(container.querySelector('.post-code')?.textContent).toContain('A-->B');
    });

    test('splitMermaidBlocks only extracts top-level mermaid fences, up to the per-file limit', () => {
        expect(splitMermaidBlocks('# Title\n\n```mermaid\ngraph TD; A-->B\n```\n\ntext')).toEqual([
            {type: 'markdown', text: '# Title\n'},
            {type: 'mermaid', text: '```mermaid\ngraph TD; A-->B\n```', source: 'graph TD; A-->B'},
            {type: 'markdown', text: '\ntext'},
        ]);

        // A mermaid fence nested in another code block is just code.
        const nested = '````markdown\n```mermaid\ngraph TD; A-->B\n```\n````';
        expect(splitMermaidBlocks(nested)).toEqual([{type: 'markdown', text: nested}]);

        // Unclosed fence stays Markdown.
        expect(splitMermaidBlocks('```mermaid\ngraph TD')).toEqual([{type: 'markdown', text: '```mermaid\ngraph TD'}]);

        const many = Array.from({length: MERMAID_MAX_DIAGRAMS + 5}, () => '```mermaid\ngraph TD; A-->B\n```').join('\n');
        expect(splitMermaidBlocks(many).filter((s) => s.type === 'mermaid')).toHaveLength(MERMAID_MAX_DIAGRAMS);
    });

    test('revokes the previous object URL when the diagram changes', async () => {
        mockWorker();
        const {rerender} = renderDiagram('graph TD; A-->B');
        await screen.findByTestId('markdown-preview-mermaid');

        mockWorker();
        rerender(<MermaidDiagram segment={segmentFor('graph TD; B-->C')}/>);

        expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:http://localhost:8065/1');
        await waitFor(() => {
            expect(screen.getByTestId('markdown-preview-mermaid')).toHaveAttribute('src', 'blob:http://localhost:8065/2');
        });
    });
});
