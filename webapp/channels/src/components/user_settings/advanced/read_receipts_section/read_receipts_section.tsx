// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {FormattedMessage, defineMessages} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Radio} from '@mattermost/compass-ui/components/radio';
import type {PreferencesType} from '@mattermost/types/preferences';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Preferences} from 'mattermost-redux/constants';
import {get} from 'mattermost-redux/selectors/entities/preferences';
import {isReadReceiptsFeatureEnabled} from 'mattermost-redux/selectors/entities/read_receipts';

import SettingItem from 'components/setting_item';
import SettingItemMax from 'components/setting_item_max';

import {AdvancedSections} from 'utils/constants';

import type {GlobalState} from 'types/store';

const messages = defineMessages({
    title: {
        id: 'user.settings.advance.readReceiptsTitle',
        defaultMessage: 'Send and show read receipts',
    },
    on: {
        id: 'user.settings.advance.on',
        defaultMessage: 'On',
    },
    off: {
        id: 'user.settings.advance.off',
        defaultMessage: 'Off',
    },
});

type Props = {
    active: boolean;
    areAllSectionsInactive: boolean;
    onUpdateSection: (section?: string) => void;
    userId: string;
    adminMode?: boolean;
    userPreferences?: PreferencesType;
};

export default function ReadReceiptsSection({active, areAllSectionsInactive, onUpdateSection, userId, adminMode, userPreferences}: Props) {
    const dispatch = useDispatch();
    const featureEnabled = useSelector(isReadReceiptsFeatureEnabled);
    const optOut = useSelector((state: GlobalState) => get(
        state,
        Preferences.CATEGORY_READ_RECEIPTS,
        Preferences.READ_RECEIPTS_OPT_OUT,
        'false',
        adminMode ? userPreferences : undefined,
    ));

    const [value, setValue] = useState(optOut);

    const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        setValue(e.currentTarget.value);
    }, []);

    const handleUpdateSection = useCallback((section?: string) => {
        setValue(optOut);
        onUpdateSection(section);
    }, [onUpdateSection, optOut]);

    const handleSubmit = useCallback(() => {
        if (value !== optOut) {
            dispatch(savePreferences(userId, [{
                user_id: userId,
                category: Preferences.CATEGORY_READ_RECEIPTS,
                name: Preferences.READ_RECEIPTS_OPT_OUT,
                value,
            }]));
        }
        onUpdateSection('');
    }, [dispatch, onUpdateSection, optOut, userId, value]);

    if (!featureEnabled) {
        return null;
    }

    const title = <FormattedMessage {...messages.title}/>;

    let content;
    if (active) {
        const input = (
            <fieldset key='readReceiptsSetting'>
                <legend className='form-legend hidden-label'>
                    {title}
                </legend>
                <div className='settings-radio-group'>
                    <Radio
                        id='readReceiptsOn'
                        name='readReceipts'
                        value='false'
                        checked={value !== 'true'}
                        onChange={handleChange}
                    >
                        <FormattedMessage {...messages.on}/>
                    </Radio>
                    <Radio
                        id='readReceiptsOff'
                        name='readReceipts'
                        value='true'
                        checked={value === 'true'}
                        onChange={handleChange}
                    >
                        <FormattedMessage {...messages.off}/>
                    </Radio>
                </div>
                <div className='mt-5'>
                    <FormattedMessage
                        id='user.settings.advance.readReceiptsDesc'
                        defaultMessage='When on, people can see when you have seen their messages in direct messages, group messages and small channels, and you can see when they have seen yours. When off, neither is shown.'
                    />
                </div>
            </fieldset>
        );

        content = (
            <SettingItemMax
                title={title}
                inputs={[input]}
                submit={handleSubmit}
                updateSection={handleUpdateSection}
            />
        );
    } else {
        content = (
            <SettingItem
                active={false}
                areAllSectionsInactive={areAllSectionsInactive}
                title={title}
                describe={<FormattedMessage {...(optOut === 'true' ? messages.off : messages.on)}/>}
                section={AdvancedSections.READ_RECEIPTS}
                updateSection={handleUpdateSection}
                max={null}
            />
        );
    }

    return (
        <>
            <div className='divider-light'/>
            {content}
        </>
    );
}
