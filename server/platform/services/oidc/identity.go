// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

const maxSubjectLength = 255

// VerifiedIdentity is the sanitized identity returned after ID token verification.
type VerifiedIdentity struct {
	Issuer            string
	Subject           string
	Email             string
	EmailVerified     bool
	PreferredUsername string
	GivenName         string
	FamilyName        string
}

// identityTupleJSON serializes issuer and subject for hashing. Encoding is locked by tests.
func identityTupleJSON(issuer, subject string) ([]byte, error) {
	return json.Marshal([]string{issuer, subject})
}

// IdentityKey returns the Mattermost Users.AuthData value for a verified OIDC identity.
func IdentityKey(issuer, subject string) (string, error) {
	if err := validateSubject(subject); err != nil {
		return "", err
	}
	payload, err := identityTupleJSON(issuer, subject)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return model.OIDCIdentityKeyPrefix + hex.EncodeToString(sum[:]), nil
}

func validateSubject(subject string) error {
	if subject == "" {
		return fmt.Errorf("oidc: empty subject")
	}
	if utf8.RuneCountInString(subject) > maxSubjectLength {
		return fmt.Errorf("oidc: subject too long")
	}
	for i := 0; i < len(subject); i++ {
		if subject[i] > 127 {
			return fmt.Errorf("oidc: subject must be ASCII")
		}
	}
	return nil
}
