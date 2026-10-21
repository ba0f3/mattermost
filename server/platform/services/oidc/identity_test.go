// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityKeyStable(t *testing.T) {
	issuer := "https://login.example.internal/realms/company"
	subject := "user-123"

	key1, err := IdentityKey(issuer, subject)
	require.NoError(t, err)
	key2, err := IdentityKey(issuer, subject)
	require.NoError(t, err)
	require.Equal(t, key1, key2)
	require.Len(t, key1, 72)
	require.Equal(t, "oidc:v1:", key1[:8])
}

func TestIdentityKeyDiffersByIssuerOrSubject(t *testing.T) {
	a, err := IdentityKey("https://a.example/realms/x", "sub")
	require.NoError(t, err)
	b, err := IdentityKey("https://b.example/realms/x", "sub")
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
