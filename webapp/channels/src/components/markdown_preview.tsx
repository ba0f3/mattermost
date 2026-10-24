// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useEffect, useLayoutEffect, useMemo, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import type {FileInfo} from '@mattermost/types/files';

import {Client4} from 'mattermost-redux/client';
import {getFileUrl} from 'mattermost-redux/utils/file_utils';

import FileInfoPreview from 'components/file_info_preview';
import Markdown from 'components/markdown';
import LoadingSpinner from 'components/widgets/loading/loading_spinner';

import Constants from 'utils/constants';
import * as SyntaxHighlighting from 'utils/syntax_highlighting';

import type {LinkInfo} from './file_preview_modal/types';
import MermaidDiagram, {markdownOptions, splitMermaidBlocks} from './markdown_preview_mermaid';

import './markdown_preview.scss';

type Props = {
    fileInfo: FileInfo;

    /** Only used for the FileInfoPreview fallback; content is always fetched from the file API by id. */
    fileUrl: string;
    getContent?: (content: string) => void;
};

type ViewMode = 'preview' | 'source';

export function isMarkdownFile(fileInfo: FileInfo | LinkInfo) {
    return SyntaxHighlighting.getLanguageFromFileExtension((fileInfo.extension || '').toLowerCase()) === 'markdown';
}

const MarkdownPreview = ({
    fileInfo,
    fileUrl,
    getContent,
}: Props) => {
    const intl = useIntl();
    const [content, setContent] = useState('');
    const [status, setStatus] = useState<'success' | 'loading' | 'fail'>('loading');
    const [viewMode, setViewMode] = useState<ViewMode>('preview');
    const segments = useMemo(() => splitMermaidBlocks(content), [content]);

    // Always read the file through the authenticated file API (GET /api/v4/files/{id}), which enforces
    // channel read permission server-side. Never fetch an arbitrary URL from here.
    const contentUrl = getFileUrl(fileInfo.id);

    // Clear parent copy state before paint when the selected file changes.
    useLayoutEffect(() => {
        getContent?.('');
    }, [contentUrl, getContent]);

    useEffect(() => {
        let cancelled = false;

        const fail = () => {
            if (!cancelled) {
                setContent('');
                setStatus('fail');
                getContent?.('');
            }
        };

        // Same limit as the code preview; larger files fall back to the file info card without fetching.
        if (fileInfo.size > Constants.CODE_PREVIEW_MAX_FILE_SIZE) {
            fail();
            return () => {
                cancelled = true;
            };
        }

        setContent('');
        setStatus('loading');

        const fetchContent = async () => {
            try {
                const response = await fetch(contentUrl, Client4.getOptions({method: 'get'}));
                if (!response.ok) {
                    fail();
                    return;
                }

                const text = await response.text();
                if (cancelled) {
                    return;
                }

                // Defensive: don't trust fileInfo.size alone.
                if (text.length > Constants.CODE_PREVIEW_MAX_FILE_SIZE) {
                    fail();
                    return;
                }

                getContent?.(text);
                setContent(text);
                setStatus('success');
            } catch {
                fail();
            }
        };

        fetchContent();

        return () => {
            cancelled = true;
        };
    }, [contentUrl, fileInfo.size, getContent]);

    if (status === 'loading') {
        return (
            <div className='view-image__loading'>
                <LoadingSpinner/>
            </div>
        );
    }

    if (status === 'fail') {
        return (
            <FileInfoPreview
                fileInfo={fileInfo}
                fileUrl={fileUrl}
            />
        );
    }

    let body;
    if (viewMode === 'source') {
        // Raw source is rendered as React text children only (auto-escaped), never as HTML.
        body = (
            <div className='post-code markdown-preview__source'>
                <div className='hljs'>
                    <div className='post-code__line-numbers'>
                        {SyntaxHighlighting.renderLineNumbers(content)}
                    </div>
                    <code data-testid='markdown-preview-source'>
                        {content}
                    </code>
                </div>
            </div>
        );
    } else {
        // Rendered through the standard post markdown pipeline: marked with sanitize (raw HTML escaped),
        // Renderer.link drops javascript:/vbscript:/data: hrefs, and images go through MarkdownImage/ExternalImage
        // which applies the image proxy exactly like posts. Fenced mermaid blocks are rendered as sanitized SVG images.
        body = (
            <div
                className='markdown-preview__content'
                data-testid='markdown-preview-rendered'
            >
                {segments.map((segment, index) => (

                    // Segments are positional slices of the file content, so the index is a stable key.
                    // eslint-disable-next-line react/no-array-index-key
                    <React.Fragment key={index}>
                        {segment.type === 'mermaid' ? (
                            <MermaidDiagram segment={segment}/>
                        ) : (
                            <Markdown
                                message={segment.text}
                                options={markdownOptions}
                            />
                        )}
                    </React.Fragment>
                ))}
            </div>
        );
    }

    return (
        <div className='markdown-preview'>
            <div className='markdown-preview__header'>
                <span className='markdown-preview__filename'>
                    {fileInfo.name}
                </span>
                <div
                    className='markdown-preview__toggle'
                    role='group'
                    aria-label={intl.formatMessage({id: 'markdown_preview.view_mode', defaultMessage: 'Markdown view mode'})}
                >
                    <button
                        type='button'
                        className={classNames('btn btn-xs', {'btn-primary': viewMode === 'preview', 'btn-tertiary': viewMode !== 'preview'})}
                        aria-pressed={viewMode === 'preview'}
                        onClick={() => setViewMode('preview')}
                    >
                        <FormattedMessage
                            id='markdown_preview.preview'
                            defaultMessage='Preview'
                        />
                    </button>
                    <button
                        type='button'
                        className={classNames('btn btn-xs', {'btn-primary': viewMode === 'source', 'btn-tertiary': viewMode !== 'source'})}
                        aria-pressed={viewMode === 'source'}
                        onClick={() => setViewMode('source')}
                    >
                        <FormattedMessage
                            id='markdown_preview.source'
                            defaultMessage='Source'
                        />
                    </button>
                </div>
            </div>
            {body}
        </div>
    );
};

export default React.memo(MarkdownPreview);
