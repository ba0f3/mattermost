// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {DOMPurify} from 'dompurify';
import React, {useEffect, useState} from 'react';
import {FormattedMessage} from 'react-intl';

import Markdown from 'components/markdown';

import createMermaidWorker from './create_mermaid_worker';

export const MERMAID_MAX_SOURCE_LENGTH = 20 * 1024;
export const MERMAID_MAX_LINES = 1000;
export const MERMAID_MAX_DIAGRAMS = 20;
export const MERMAID_RENDER_TIMEOUT = 3000;

export const markdownOptions = {
    atMentions: false,
    mentionHighlight: false,
    markdown: true,
};

export type MarkdownSegment = {type: 'markdown'; text: string} | {type: 'mermaid'; text: string; source: string};

const FENCE_OPEN = /^ {0,3}(`{3,}|~{3,})(.*)$/;
const FENCE_CLOSE = /^ {0,3}(`{3,}|~{3,})\s*$/;

// Click/link/callback directives can attach URLs or JS callbacks to nodes; init directives can change config.
const UNSAFE_DIRECTIVE = /^\s*(?:(?:click|link|callback)\s+\S|%%\{).*$/gim;

/** Splits top-level fenced ```mermaid blocks out of the document. Everything else stays Markdown. */
export function splitMermaidBlocks(content: string): MarkdownSegment[] {
    const lines = content.split('\n');
    const segments: MarkdownSegment[] = [];
    let start = 0;
    let fence: {marker: string; line: number; mermaid: boolean} | null = null;
    let diagrams = 0;

    for (let i = 0; i < lines.length; i++) {
        if (!fence) {
            const match = FENCE_OPEN.exec(lines[i]);
            if (match && !(match[1][0] === '`' && match[2].includes('`'))) {
                const mermaid = match[2].trim().split(/\s/)[0].toLowerCase() === 'mermaid' && diagrams < MERMAID_MAX_DIAGRAMS;
                fence = {marker: match[1], line: i, mermaid};
            }
            continue;
        }

        const match = FENCE_CLOSE.exec(lines[i]);
        if (!match || match[1][0] !== fence.marker[0] || match[1].length < fence.marker.length) {
            continue;
        }

        if (fence.mermaid) {
            if (fence.line > start) {
                segments.push({type: 'markdown', text: lines.slice(start, fence.line).join('\n')});
            }
            segments.push({
                type: 'mermaid',
                text: lines.slice(fence.line, i + 1).join('\n'),
                source: lines.slice(fence.line + 1, i).join('\n'),
            });
            start = i + 1;
            diagrams++;
        }
        fence = null;
    }

    if (start < lines.length) {
        segments.push({type: 'markdown', text: lines.slice(start).join('\n')});
    }

    return segments;
}

/** Returns the source to render, or null when it is over the size limits. */
export function prepareMermaidSource(source: string): string | null {
    if (source.length > MERMAID_MAX_SOURCE_LENGTH) {
        return null;
    }

    const stripped = source.replace(UNSAFE_DIRECTIVE, '');
    if (stripped.split('\n').length > MERMAID_MAX_LINES) {
        return null;
    }

    return stripped;
}

export function sanitizeMermaidSvg(createDOMPurify: DOMPurify, svg: string): string {
    const purify = createDOMPurify(window);
    purify.addHook('uponSanitizeAttribute', (node, data) => {
        const name = data.attrName.toLowerCase();

        // pele never emits references, so every href is dropped: javascript:, data: and plain links alike.
        if (name.startsWith('on') || name === 'href' || name === 'xlink:href') {
            data.keepAttr = false;
        }
    });

    return purify.sanitize(svg, {
        USE_PROFILES: {svg: true, svgFilters: true},
        FORBID_TAGS: ['script', 'foreignObject'],
    });
}

type Props = {
    segment: Extract<MarkdownSegment, {type: 'mermaid'}>;
};

const MermaidDiagram = ({segment}: Props) => {
    const [url, setUrl] = useState('');
    const [failed, setFailed] = useState(false);

    useEffect(() => {
        setUrl('');
        setFailed(false);

        const source = prepareMermaidSource(segment.source);
        if (source === null) {
            setFailed(true);
            return undefined;
        }

        let cancelled = false;
        let objectUrl = '';
        const worker = createMermaidWorker();

        const fail = () => {
            worker.terminate();
            if (!cancelled) {
                setFailed(true);
            }
        };

        const timeout = window.setTimeout(fail, MERMAID_RENDER_TIMEOUT);

        worker.onerror = () => {
            window.clearTimeout(timeout);
            fail();
        };

        worker.onmessage = async (event: MessageEvent) => {
            window.clearTimeout(timeout);
            worker.terminate();

            try {
                if (typeof event.data !== 'string') {
                    throw new Error('render failed');
                }

                const {default: createDOMPurify} = await import('dompurify');
                const svg = sanitizeMermaidSvg(createDOMPurify, event.data);
                if (cancelled) {
                    return;
                }
                if (!svg) {
                    throw new Error('empty svg');
                }

                objectUrl = URL.createObjectURL(new Blob([svg], {type: 'image/svg+xml'}));
                setUrl(objectUrl);
            } catch {
                fail();
            }
        };

        worker.postMessage(source);

        return () => {
            cancelled = true;
            window.clearTimeout(timeout);
            worker.terminate();
            if (objectUrl) {
                URL.revokeObjectURL(objectUrl);
            }
        };
    }, [segment.source]);

    if (url) {
        return (
            <div className='markdown-preview__mermaid'>
                <img
                    src={url}
                    alt={segment.source}
                    data-testid='markdown-preview-mermaid'
                />
            </div>
        );
    }

    return (
        <>
            <Markdown
                message={segment.text}
                options={markdownOptions}
            />
            {failed && (
                <p className='markdown-preview__mermaid-error'>
                    <FormattedMessage
                        id='markdown_preview.mermaid_error'
                        defaultMessage='This diagram could not be rendered.'
                    />
                </p>
            )}
        </>
    );
};

export default MermaidDiagram;
