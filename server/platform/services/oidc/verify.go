// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/mattermost/mattermost/server/public/model"
)

type idTokenClaims struct {
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	AZP               string `json:"azp"`
}

func verifyIDToken(ctx context.Context, provider *oidc.Provider, cfg Config, rawIDToken, expectedNonce string) (VerifiedIdentity, error) {
	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}

	if token.Issuer != cfg.Issuer {
		return VerifiedIdentity{}, ErrAuthenticationFailed
	}

	var claims idTokenClaims
	if err := token.Claims(&claims); err != nil {
		return VerifiedIdentity{}, fmt.Errorf("%w: claims", ErrAuthenticationFailed)
	}

	if err := validateAudience(token, cfg.ClientID, claims.AZP); err != nil {
		return VerifiedIdentity{}, err
	}

	if token.Nonce != expectedNonce {
		return VerifiedIdentity{}, ErrAuthenticationFailed
	}

	if err := validateSubject(token.Subject); err != nil {
		return VerifiedIdentity{}, ErrAuthenticationFailed
	}

	if !model.IsValidEmail(claims.Email) {
		return VerifiedIdentity{}, ErrAuthenticationFailed
	}
	if !claims.EmailVerified {
		return VerifiedIdentity{}, ErrEmailUnverified
	}

	return VerifiedIdentity{
		Issuer:            cfg.Issuer,
		Subject:           token.Subject,
		Email:             claims.Email,
		EmailVerified:     claims.EmailVerified,
		PreferredUsername: claims.PreferredUsername,
		GivenName:         claims.GivenName,
		FamilyName:        claims.FamilyName,
	}, nil
}

func validateAudience(token *oidc.IDToken, clientID, azp string) error {
	if len(token.Audience) == 0 {
		return ErrAuthenticationFailed
	}
	if len(token.Audience) != 1 || token.Audience[0] != clientID {
		return ErrAuthenticationFailed
	}
	if azp != "" && azp != clientID {
		return ErrAuthenticationFailed
	}
	return nil
}
