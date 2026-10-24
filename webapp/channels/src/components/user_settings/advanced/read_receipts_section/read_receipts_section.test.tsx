// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Preferences} from 'mattermost-redux/constants';
import {getPreferenceKey} from 'mattermost-redux/utils/preference_utils';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import ReadReceiptsSection from './read_receipts_section';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
}));

describe('components/user_settings/advanced/ReadReceiptsSection', () => {
    const userId = 'current_user_id';
    const defaultProps = {
        active: false,
        areAllSectionsInactive: false,
        onUpdateSection: jest.fn(),
        userId,
    };

    function makeState(enabled: string, optOut?: string): DeepPartial<GlobalState> {
        const myPreferences = optOut === undefined ? {} : {
            [getPreferenceKey(Preferences.CATEGORY_READ_RECEIPTS, Preferences.READ_RECEIPTS_OPT_OUT)]: {
                user_id: userId,
                category: Preferences.CATEGORY_READ_RECEIPTS,
                name: Preferences.READ_RECEIPTS_OPT_OUT,
                value: optOut,
            },
        };

        return {
            entities: {
                general: {config: {EnableReadReceipts: enabled}},
                preferences: {myPreferences},
            },
        };
    }

    beforeEach(() => {
        jest.mocked(savePreferences).mockClear();
        defaultProps.onUpdateSection.mockClear();
    });

    test('renders nothing when the server feature is off', () => {
        const {container} = renderWithContext(<ReadReceiptsSection {...defaultProps}/>, makeState('false'));

        expect(container).toBeEmptyDOMElement();
    });

    test('shows On by default when collapsed', () => {
        renderWithContext(<ReadReceiptsSection {...defaultProps}/>, makeState('true'));

        expect(screen.getByText('Send and show read receipts')).toBeInTheDocument();
        expect(screen.getByText('On')).toBeInTheDocument();
    });

    test('shows Off when opted out', () => {
        renderWithContext(<ReadReceiptsSection {...defaultProps}/>, makeState('true', 'true'));

        expect(screen.getByText('Off')).toBeInTheDocument();
    });

    test('saves the opt-out preference when turned off', async () => {
        renderWithContext(
            <ReadReceiptsSection
                {...defaultProps}
                active={true}
            />,
            makeState('true'),
        );

        expect(screen.getByRole('radio', {name: 'On'})).toBeChecked();

        await userEvent.click(screen.getByRole('radio', {name: 'Off'}));
        await userEvent.click(screen.getByTestId('saveSetting'));

        expect(savePreferences).toHaveBeenCalledWith(userId, [{
            user_id: userId,
            category: Preferences.CATEGORY_READ_RECEIPTS,
            name: Preferences.READ_RECEIPTS_OPT_OUT,
            value: 'true',
        }]);
        expect(defaultProps.onUpdateSection).toHaveBeenCalledWith('');
    });

    test('saves nothing when the value is unchanged', async () => {
        renderWithContext(
            <ReadReceiptsSection
                {...defaultProps}
                active={true}
            />,
            makeState('true', 'true'),
        );

        expect(screen.getByRole('radio', {name: 'Off'})).toBeChecked();

        await userEvent.click(screen.getByTestId('saveSetting'));

        expect(savePreferences).not.toHaveBeenCalled();
    });
});
