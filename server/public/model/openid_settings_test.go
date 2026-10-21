// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenIDSettingsDerivedDiscovery(t *testing.T) {
	s := OpenIDSettings{}
	s.SetDefaults()
	*s.Issuer = "https://login.example/realms/mm"
	require.Equal(t, "https://login.example/realms/mm/.well-known/openid-configuration", s.DerivedDiscoveryEndpoint())
}

func TestOpenIDSettingsIsOSSOIDCEnabled(t *testing.T) {
	s := OpenIDSettings{}
	s.SetDefaults()
	require.False(t, s.IsOSSOIDCEnabled())
	*s.Enable = true
	require.False(t, s.IsOSSOIDCEnabled())
	*s.Issuer = "https://login.example/realms/mm"
	require.True(t, s.IsOSSOIDCEnabled())
}
