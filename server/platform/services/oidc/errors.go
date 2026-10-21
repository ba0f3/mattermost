// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import "errors"

var (
	ErrDisabled              = errors.New("oidc: disabled")
	ErrConfigInvalid         = errors.New("oidc: config invalid")
	ErrProviderUnavailable   = errors.New("oidc: provider unavailable")
	ErrAuthenticationFailed  = errors.New("oidc: authentication failed")
	ErrEmailUnverified       = errors.New("oidc: email unverified")
	ErrTransactionInvalid    = errors.New("oidc: transaction invalid")
	ErrTransactionExpired    = errors.New("oidc: transaction expired")
	ErrConfigChanged         = errors.New("oidc: config changed")
	ErrLinkingRequired       = errors.New("oidc: linking required")
	ErrIdentityConflict      = errors.New("oidc: identity conflict")
	ErrAccountNotProvisioned = errors.New("oidc: account not provisioned")
)

// Code returns a stable machine-readable error code for logging and UI mapping.
func Code(err error) string {
	switch {
	case errors.Is(err, ErrDisabled):
		return "oidc_disabled"
	case errors.Is(err, ErrConfigInvalid):
		return "oidc_config_invalid"
	case errors.Is(err, ErrProviderUnavailable):
		return "oidc_provider_unavailable"
	case errors.Is(err, ErrAuthenticationFailed):
		return "oidc_authentication_failed"
	case errors.Is(err, ErrEmailUnverified):
		return "oidc_email_unverified"
	case errors.Is(err, ErrTransactionInvalid):
		return "oidc_transaction_invalid"
	case errors.Is(err, ErrTransactionExpired):
		return "oidc_transaction_expired"
	case errors.Is(err, ErrConfigChanged):
		return "oidc_config_changed"
	case errors.Is(err, ErrLinkingRequired):
		return "oidc_linking_required"
	case errors.Is(err, ErrIdentityConflict):
		return "oidc_identity_conflict"
	case errors.Is(err, ErrAccountNotProvisioned):
		return "oidc_account_not_provisioned"
	default:
		return "oidc_authentication_failed"
	}
}
