// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {JSX} from 'react';

export type StartTrialBtnProps = {
    onClick?: () => void;
    disabled?: boolean;
} & ({
    btnClass?: string;
    renderAsButton: true;
} | {
    btnClass?: never;
    renderAsButton?: false;
});

// Start-trial CTAs are intentionally disabled in this build.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const StartTrialBtn = (_props: StartTrialBtnProps): JSX.Element | null => null;

export default StartTrialBtn;
