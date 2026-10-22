// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {forwardRef, memo} from 'react';
import {defineMessages, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import styled from 'styled-components';

import {
    FormatBoldIcon,
    FormatItalicIcon,
    LinkVariantIcon,
    FormatStrikethroughVariantIcon,
    CodeTagsIcon,
    FormatHeaderIcon,
    FormatQuoteOpenIcon,
    FormatListBulletedIcon,
    FormatListNumberedIcon,
} from '@mattermost/compass-icons/components';
import type IconProps from '@mattermost/compass-icons/components/props';
import {WithTooltip} from '@mattermost/shared/components/tooltip';

import KeyboardShortcutSequence, {
    KEYBOARD_SHORTCUTS,
} from 'components/keyboard_shortcuts/keyboard_shortcuts_sequence';
import type {
    KeyboardShortcutDescriptor} from 'components/keyboard_shortcuts/keyboard_shortcuts_sequence';

import type {MarkdownMode} from 'utils/markdown/apply_markdown';

export const IconContainer = styled.button`
    display: flex;
    min-width: 32px;
    height: 32px;
    place-items: center;
    place-content: center;
    border: none;
    background: transparent;
    padding: 0 7px;
    border-radius: var(--radius-s);
    color: rgba(var(--center-channel-color-rgb), var(--icon-opacity));

    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.08);
        color: rgba(var(--center-channel-color-rgb), var(--icon-opacity-hover));
        fill: currentColor;
    }

    &:active,
    &.active,
    &.active:hover {
        background: rgba(var(--button-bg-rgb), 0.08);
        color: var(--button-bg);
        fill: currentColor;
    }

    @media screen and (max-width: 768px) {
        min-width: 44px;
        height: 44px;
    }

    &[disabled] {
        pointer-events: none;
        cursor: not-allowed;
        color: rgba(var(--center-channel-color-rgb), 0.32);

        &:hover,
        &:active,
        &.active,
        &.active:hover {
            background: inherit;
            color: inherit;
            fill: inherit;
        }
    }
`;

interface FormattingIconProps {
    id?: string;
    mode: MarkdownMode;
    onClick?: () => void;
    className?: string;
    disabled?: boolean;
    isActive?: boolean;
}

// Compass does not yet include an underline glyph.
const UnderlineIcon: React.FC<IconProps> = ({size = 18, color = 'currentColor'}) => (
    <svg
        width={size}
        height={size}
        viewBox='0 0 24 24'
        fill='none'
        stroke={color}
        strokeWidth='2'
        strokeLinecap='round'
        aria-hidden={true}
    >
        <path d='M7 4v7a5 5 0 0 0 10 0V4M5 21h14'/>
    </svg>
);

const MAP_MARKDOWN_MODE_TO_ICON: Record<FormattingIconProps['mode'], React.FC<IconProps>> = {
    bold: FormatBoldIcon,
    underline: UnderlineIcon,
    italic: FormatItalicIcon,
    link: LinkVariantIcon,
    strike: FormatStrikethroughVariantIcon,
    code: CodeTagsIcon,
    heading: FormatHeaderIcon,
    quote: FormatQuoteOpenIcon,
    ul: FormatListBulletedIcon,
    ol: FormatListNumberedIcon,
};

const MAP_MARKDOWN_MODE_TO_ARIA_LABEL: Record<FormattingIconProps['mode'], MessageDescriptor> = defineMessages({
    underline: {id: 'accessibility.button.underline', defaultMessage: 'underline'},
    bold: {id: 'accessibility.button.bold', defaultMessage: 'bold'},
    italic: {id: 'accessibility.button.italic', defaultMessage: 'italic'},
    link: {id: 'accessibility.button.link', defaultMessage: 'link'},
    strike: {id: 'accessibility.button.strike', defaultMessage: 'strike through'},
    code: {id: 'accessibility.button.code', defaultMessage: 'code'},
    heading: {id: 'accessibility.button.heading', defaultMessage: 'heading'},
    quote: {id: 'accessibility.button.quote', defaultMessage: 'quote'},
    ul: {id: 'accessibility.button.bulleted_list', defaultMessage: 'bulleted list'},
    ol: {id: 'accessibility.button.numbered_list', defaultMessage: 'numbered list'},
});

const MAP_MARKDOWN_MODE_TO_KEYBOARD_SHORTCUTS: Partial<Record<FormattingIconProps['mode'], KeyboardShortcutDescriptor>> = {
    bold: KEYBOARD_SHORTCUTS.msgMarkdownBold,
    italic: KEYBOARD_SHORTCUTS.msgMarkdownItalic,
    link: KEYBOARD_SHORTCUTS.msgMarkdownLink,
    strike: KEYBOARD_SHORTCUTS.msgMarkdownStrike,
    code: KEYBOARD_SHORTCUTS.msgMarkdownCode,
    heading: KEYBOARD_SHORTCUTS.msgMarkdownH3,
    quote: KEYBOARD_SHORTCUTS.msgMarkdownQuote,
    ul: KEYBOARD_SHORTCUTS.msgMarkdownUl,
    ol: KEYBOARD_SHORTCUTS.msgMarkdownOl,
};

const FormattingIcon = forwardRef<HTMLButtonElement, FormattingIconProps>((props, ref) => {
    /**
     * by passing in the otherProps spread we guarantee that accessibility
     * properties like aria-label, etc. get added to the DOM
     */
    const {mode, onClick, isActive, className, ...otherProps} = props;
    const handleMouseDown = React.useCallback((e: React.MouseEvent<HTMLButtonElement>) => {
        e.preventDefault();
    }, []);

    /* get the correct Icon from the IconMap */
    const Icon = MAP_MARKDOWN_MODE_TO_ICON[mode];
    const {formatMessage} = useIntl();
    const ariaLabelDefinition = MAP_MARKDOWN_MODE_TO_ARIA_LABEL[mode];
    const buttonAriaLabel = formatMessage(ariaLabelDefinition);

    const bodyAction = (
        <IconContainer
            ref={ref}
            type='button'
            id={props.id || `FormattingControl_${mode}`}
            onClick={onClick}
            onMouseDown={handleMouseDown}
            aria-label={buttonAriaLabel}
            aria-pressed={typeof isActive === 'boolean' ? isActive : undefined}
            className={classNames(className, {active: isActive})}
            {...otherProps}
        >
            <Icon
                color={'currentColor'}
                size={18}
            />
        </IconContainer>
    );

    /* get the correct tooltip from the ShortcutsMap */
    const shortcut = MAP_MARKDOWN_MODE_TO_KEYBOARD_SHORTCUTS[mode];

    return (
        <WithTooltip
            title={shortcut ? (
                <KeyboardShortcutSequence
                    shortcut={shortcut}
                    hoistDescription={true}
                    isInsideTooltip={true}
                />
            ) : buttonAriaLabel}
        >
            {bodyAction}
        </WithTooltip>
    );
});

FormattingIcon.displayName = 'FormattingIcon';

export default memo(FormattingIcon);
