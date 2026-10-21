// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

const OIDCIdentityKeyPrefix = "oidc:v1:"

// OIDCIdentity maps a stable OIDC issuer/subject pair to a Mattermost user.
type OIDCIdentity struct {
	IdentityKey string
	UserId      string
	Issuer      string
	Subject     string
	CreatedAt   int64
	UpdatedAt   int64
}
