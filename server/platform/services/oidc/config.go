// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// Config is a normalized snapshot used by the protocol client.
type Config struct {
	Enabled              bool
	Issuer               string
	ClientID             string
	ClientSecret         string
	RedirectURI          string
	Scopes               []string
	DevAllowHTTPLoopback bool
	Fingerprint          string
}

// ConfigFromModel builds a protocol config from Mattermost settings.
func ConfigFromModel(cfg *model.Config) (Config, error) {
	settings := cfg.OpenIdSettings
	if settings.Enable == nil || !*settings.Enable {
		return Config{Enabled: false}, nil
	}

	siteURL := strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/")
	if siteURL == "" {
		return Config{}, fmt.Errorf("%w: missing site url", ErrConfigInvalid)
	}

	if appErr := settings.IsValid(siteURL); appErr != nil {
		return Config{}, fmt.Errorf("%w: %s", ErrConfigInvalid, appErr.Id)
	}

	issuer := strings.TrimSpace(*settings.Issuer)
	scopes := strings.Fields(*settings.Scope)
	if len(scopes) == 0 {
		scopes = strings.Fields(model.OpenidSettingsDefaultScope)
	}

	redirectURI, err := buildRedirectURI(siteURL)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Enabled:              true,
		Issuer:               issuer,
		ClientID:             strings.TrimSpace(*settings.Id),
		ClientSecret:         *settings.Secret,
		RedirectURI:          redirectURI,
		Scopes:               scopes,
		DevAllowHTTPLoopback: settings.DevAllowHTTPLoopback != nil && *settings.DevAllowHTTPLoopback,
	}
	c.Fingerprint = c.computeFingerprint()
	return c, nil
}

func buildRedirectURI(siteURL string) (string, error) {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return "", fmt.Errorf("%w: site url", ErrConfigInvalid)
	}
	prefix := strings.TrimRight(parsed.Path, "/")
	u := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   prefix + "/signup/openid/complete",
	}
	return u.String(), nil
}

func (c Config) computeFingerprint() string {
	h := sha256.New()
	h.Write([]byte(c.Issuer))
	h.Write([]byte{0})
	h.Write([]byte(c.ClientID))
	h.Write([]byte{0})
	h.Write([]byte(c.ClientSecret))
	h.Write([]byte{0})
	h.Write([]byte(c.RedirectURI))
	return hex.EncodeToString(h.Sum(nil))
}

func (c Config) discoveryURL() string {
	return strings.TrimRight(c.Issuer, "/") + "/.well-known/openid-configuration"
}

func (c Config) validate() error {
	if !c.Enabled {
		return ErrDisabled
	}
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" || c.RedirectURI == "" {
		return ErrConfigInvalid
	}
	return nil
}
