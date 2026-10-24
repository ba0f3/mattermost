// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import MarkdownPreview, {isMarkdownFile} from 'components/markdown_preview';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import Constants from 'utils/constants';
import {TestHelper} from 'utils/test_helper';

describe('components/MarkdownPreview', () => {
    const fileUrl = 'https://example.com/api/v4/files/file_id';
    const requiredProps = {
        fileInfo: TestHelper.getFileInfoMock({
            id: 'file_id',
            name: 'readme.md',
            extension: 'md',
            size: 100,
        }),
        fileUrl,
    };

    const originalFetch = global.fetch;

    const mockFetchText = (text: string) => {
        const fetchMock = jest.fn().mockResolvedValue({
            ok: true,
            text: () => Promise.resolve(text),
        });
        global.fetch = fetchMock;
        return fetchMock;
    };

    const renderAndWaitForPreview = async (markdown: string, initialState = {}) => {
        mockFetchText(markdown);
        const result = renderWithContext(
            <MarkdownPreview {...requiredProps}/>,
            initialState,
        );
        await waitFor(() => {
            expect(result.container.querySelector('.markdown-preview')).toBeInTheDocument();
        });
        return result;
    };

    afterEach(() => {
        global.fetch = originalFetch;
    });

    test('isMarkdownFile returns true for markdown extensions, including .markdown', () => {
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'md'}))).toBe(true);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'mkd'}))).toBe(true);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'mkdown'}))).toBe(true);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'markdown'}))).toBe(true);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'MD'}))).toBe(true);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'js'}))).toBe(false);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: 'mk'}))).toBe(false);
        expect(isMarkdownFile(TestHelper.getFileInfoMock({extension: ''}))).toBe(false);
    });

    test('should show loading spinner initially', () => {
        global.fetch = jest.fn().mockImplementation(() => new Promise(() => { /* never resolves */ }));

        const {container} = renderWithContext(
            <MarkdownPreview {...requiredProps}/>,
        );

        expect(container.querySelector('.view-image__loading')).toBeInTheDocument();
    });

    test('should fetch content only from the authenticated file API route for the file id', async () => {
        const fetchMock = mockFetchText('# Hi');

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                fileUrl='https://evil.example.com/readme.md'
            />,
        );

        await waitFor(() => {
            expect(screen.getByText('Hi')).toBeInTheDocument();
        });

        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock.mock.calls[0][0]).toBe(Client4.getFileRoute('file_id'));
        expect(fetchMock.mock.calls[0][1]).toMatchObject({method: 'get'});
    });

    test('should render markdown content after fetch succeeds', async () => {
        const {container} = await renderAndWaitForPreview('# Hello\n\nThis is **bold** text\n\n- item one\n- item two');

        expect(screen.getByRole('heading', {name: 'Hello'})).toBeInTheDocument();
        expect(screen.getByText('readme.md')).toBeInTheDocument();
        expect(screen.getByText('bold').tagName).toBe('STRONG');
        expect(container.querySelectorAll('li')).toHaveLength(2);
        expect(screen.getByRole('button', {name: 'Preview'})).toHaveAttribute('aria-pressed', 'true');
    });

    test('should call getContent with raw markdown source', async () => {
        const raw = '# Title\n\nBody content';
        const getContent = jest.fn();
        mockFetchText(raw);

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                getContent={getContent}
            />,
        );

        await waitFor(() => {
            expect(getContent).toHaveBeenCalledWith(raw);
        });
    });

    test('should escape raw HTML and never render script or other HTML tags', async () => {
        const {container} = await renderAndWaitForPreview(
            'Hello <b>world</b> <script>alert(1)</script>\n\n<img src=x onerror="alert(2)">\n\n<iframe src="https://example.com"></iframe>\n\n<a href="javascript:alert(3)">raw</a>',
        );

        const rendered = container.querySelector('.markdown-preview__content')!;
        expect(rendered.querySelector('script')).toBeNull();
        expect(rendered.querySelector('b')).toBeNull();
        expect(rendered.querySelector('img')).toBeNull();
        expect(rendered.querySelector('iframe')).toBeNull();
        expect(rendered.querySelector('a')).toBeNull();
        expect(rendered.querySelector('[onerror]')).toBeNull();
        expect(rendered.textContent).toContain('<b>world</b>');
        expect(rendered.textContent).toContain('<script>alert(1)</script>');
    });

    test('should neutralize javascript:, vbscript: and data: links', async () => {
        const {container} = await renderAndWaitForPreview(
            '[js](javascript:alert(1)) [js2](JaVaScRiPt:alert(1)) [enc](java%73cript:alert(1)) [vb](vbscript:msgbox(1)) [data](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==) [ok](https://mattermost.com)',
        );

        const anchors = Array.from(container.querySelectorAll('.markdown-preview__content a'));
        expect(anchors).toHaveLength(1);
        expect(anchors[0]).toHaveAttribute('href', 'https://mattermost.com');
        for (const a of container.querySelectorAll('a[href]')) {
            expect(a.getAttribute('href')!.toLowerCase()).not.toMatch(/^(javascript|vbscript|data):/);
        }

        // The link text is kept as plain text.
        expect(screen.getByText(/js2/)).toBeInTheDocument();
    });

    test('should route external images through the image proxy like posts', async () => {
        const {container} = await renderAndWaitForPreview(
            '![logo](https://example.com/logo.png)',
            {entities: {general: {config: {HasImageProxy: 'true'}}}},
        );

        await waitFor(() => {
            expect(container.querySelector('.markdown-preview__content img')).toBeInTheDocument();
        });
        const img = container.querySelector('.markdown-preview__content img')!;
        expect(img.getAttribute('src')).toBe(`${Client4.getBaseRoute()}/image?url=${encodeURIComponent('https://example.com/logo.png')}`);
    });

    test('should toggle between rendered preview and raw source', async () => {
        const raw = '# Heading\n\n<script>alert(1)</script> **bold**';
        const {container} = await renderAndWaitForPreview(raw);

        expect(screen.getByTestId('markdown-preview-rendered')).toBeInTheDocument();
        expect(screen.queryByTestId('markdown-preview-source')).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Source'}));

        const source = screen.getByTestId('markdown-preview-source');
        expect(source.textContent).toBe(raw);
        expect(source.querySelector('*')).toBeNull();
        expect(container.querySelector('script')).toBeNull();
        expect(screen.queryByTestId('markdown-preview-rendered')).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Source'})).toHaveAttribute('aria-pressed', 'true');
        expect(screen.getByRole('button', {name: 'Preview'})).toHaveAttribute('aria-pressed', 'false');

        await userEvent.click(screen.getByRole('button', {name: 'Preview'}));

        expect(screen.getByRole('heading', {name: 'Heading'})).toBeInTheDocument();
        expect(screen.queryByTestId('markdown-preview-source')).not.toBeInTheDocument();
    });

    test('should render .markdown files', async () => {
        mockFetchText('# From .markdown');

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                fileInfo={TestHelper.getFileInfoMock({...requiredProps.fileInfo, name: 'notes.markdown', extension: 'markdown'})}
            />,
        );

        await waitFor(() => {
            expect(screen.getByRole('heading', {name: 'From .markdown'})).toBeInTheDocument();
        });
    });

    test('should fall back to FileInfoPreview without fetching when file is over the code preview size limit', async () => {
        global.fetch = jest.fn();
        const getContent = jest.fn();

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                fileInfo={TestHelper.getFileInfoMock({
                    id: 'file_id',
                    name: 'large.md',
                    extension: 'md',
                    size: Constants.CODE_PREVIEW_MAX_FILE_SIZE + 1,
                })}
                getContent={getContent}
            />,
        );

        await waitFor(() => {
            expect(document.querySelector('.file-details__name')).toBeInTheDocument();
        });

        expect(document.querySelector('.markdown-preview')).not.toBeInTheDocument();
        expect(global.fetch).not.toHaveBeenCalled();
        expect(getContent).not.toHaveBeenCalledWith(expect.stringMatching(/.+/));
    });

    test('should render a file exactly at the size limit', async () => {
        mockFetchText('# At limit');

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                fileInfo={TestHelper.getFileInfoMock({...requiredProps.fileInfo, size: Constants.CODE_PREVIEW_MAX_FILE_SIZE})}
            />,
        );

        await waitFor(() => {
            expect(screen.getByRole('heading', {name: 'At limit'})).toBeInTheDocument();
        });
    });

    test('should fall back to FileInfoPreview when fetched content exceeds the size limit', async () => {
        const getContent = jest.fn();
        mockFetchText('a'.repeat(Constants.CODE_PREVIEW_MAX_FILE_SIZE + 1));

        renderWithContext(
            <MarkdownPreview
                {...requiredProps}
                getContent={getContent}
            />,
        );

        await waitFor(() => {
            expect(document.querySelector('.file-details__name')).toBeInTheDocument();
        });
        expect(document.querySelector('.markdown-preview')).not.toBeInTheDocument();
        expect(getContent).not.toHaveBeenCalledWith(expect.stringMatching(/.+/));
    });

    test('should fall back to FileInfoPreview when fetch fails', async () => {
        global.fetch = jest.fn().mockResolvedValue({
            ok: false,
            status: 403,
            text: () => Promise.resolve(''),
        });

        renderWithContext(
            <MarkdownPreview {...requiredProps}/>,
        );

        await waitFor(() => {
            expect(document.querySelector('.file-details__name')).toBeInTheDocument();
        });

        expect(document.querySelector('.markdown-preview')).not.toBeInTheDocument();
    });

    test('should fall back to FileInfoPreview when fetch throws', async () => {
        global.fetch = jest.fn().mockRejectedValue(new Error('network'));

        renderWithContext(
            <MarkdownPreview {...requiredProps}/>,
        );

        await waitFor(() => {
            expect(document.querySelector('.file-details__name')).toBeInTheDocument();
        });
    });
});
