// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useSelector} from 'react-redux';

import {Client4} from 'mattermost-redux/client';
import {getConfig, isFreeEdition as isFreeEditionSelector} from 'mattermost-redux/selectors/entities/general';

import ProductBrandingFreeEdition from './product_branding_free_edition';
import ProductBrandingLicensedEdition from './product_branding_licensed_edition';

export function ProductBranding() {
    const isFreeEdition = useSelector(isFreeEditionSelector);
    const config = useSelector(getConfig);

    const [imageError, setImageError] = useState(false);
    const [cacheBust] = useState(() => String(Date.now()));

    // Prefer the custom brand image uploaded in the System Console, falling back
    // to the built-in product branding when custom branding is off or the image
    // fails to load.
    if (config.EnableCustomBrand === 'true' && !imageError) {
        return (
            <span className='globalHeader-leftControls-productBranding-freeEdition'>
                <img
                    className='globalHeader-leftControls-productBranding-customLogo'
                    src={Client4.getBrandImageUrl(cacheBust)}
                    alt=''
                    onError={() => setImageError(true)}
                />
            </span>
        );
    }

    if (isFreeEdition) {
        return <ProductBrandingFreeEdition/>;
    }

    return <ProductBrandingLicensedEdition/>;
}
