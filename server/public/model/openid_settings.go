// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"net/url"
	"strings"
)

// OpenIDSettings is the dedicated configuration type for OSS OIDC login.
type OpenIDSettings struct {
	Enable               *bool   `access:"authentication_openid"`
	Secret               *string `access:"authentication_openid"` // telemetry: none
	Id                   *string `access:"authentication_openid"` // telemetry: none
	Scope                *string `access:"authentication_openid"` // telemetry: none
	AuthEndpoint         *string `access:"authentication_openid"` // telemetry: none
	TokenEndpoint        *string `access:"authentication_openid"` // telemetry: none
	UserAPIEndpoint      *string `access:"authentication_openid"` // telemetry: none
	DiscoveryEndpoint    *string `access:"authentication_openid"` // telemetry: none
	ButtonText           *string `access:"authentication_openid"` // telemetry: none
	ButtonColor          *string `access:"authentication_openid"` // telemetry: none
	UsePreferredUsername *bool   `access:"authentication_openid"` // telemetry: none
	Issuer               *string `access:"authentication_openid"` // telemetry: none
	EnableJIT            *bool   `access:"authentication_openid"`
	DevAllowHTTPLoopback *bool   `access:"authentication_openid"`
}

func (s *OpenIDSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = new(false)
	}
	if s.Secret == nil {
		s.Secret = new("")
	}
	if s.Id == nil {
		s.Id = new("")
	}
	if s.Scope == nil {
		s.Scope = new(OpenidSettingsDefaultScope)
	}
	if s.DiscoveryEndpoint == nil {
		s.DiscoveryEndpoint = new("")
	}
	if s.AuthEndpoint == nil {
		s.AuthEndpoint = new("")
	}
	if s.TokenEndpoint == nil {
		s.TokenEndpoint = new("")
	}
	if s.UserAPIEndpoint == nil {
		s.UserAPIEndpoint = new("")
	}
	if s.ButtonText == nil {
		s.ButtonText = new("")
	}
	if s.ButtonColor == nil {
		s.ButtonColor = new("#145DBF")
	}
	if s.UsePreferredUsername == nil {
		s.UsePreferredUsername = new(false)
	}
	if s.Issuer == nil {
		s.Issuer = new("")
	}
	if s.EnableJIT == nil {
		s.EnableJIT = new(false)
	}
	if s.DevAllowHTTPLoopback == nil {
		s.DevAllowHTTPLoopback = new(false)
	}
}

func (s *OpenIDSettings) SSOSettings() *SSOSettings {
	return &SSOSettings{
		Enable:               s.Enable,
		Secret:               s.Secret,
		Id:                   s.Id,
		Scope:                s.Scope,
		AuthEndpoint:         s.AuthEndpoint,
		TokenEndpoint:        s.TokenEndpoint,
		UserAPIEndpoint:      s.UserAPIEndpoint,
		DiscoveryEndpoint:    s.DiscoveryEndpoint,
		ButtonText:           s.ButtonText,
		ButtonColor:          s.ButtonColor,
		UsePreferredUsername: s.UsePreferredUsername,
	}
}

// DerivedDiscoveryEndpoint returns the OIDC discovery document URL from Issuer.
func (s *OpenIDSettings) DerivedDiscoveryEndpoint() string {
	issuer := strings.TrimRight(*s.Issuer, "/")
	return issuer + "/.well-known/openid-configuration"
}

// IsOSSOIDCEnabled reports whether the dedicated OSS OIDC flow should be used.
func (s *OpenIDSettings) IsOSSOIDCEnabled() bool {
	return s.Enable != nil && *s.Enable && s.Issuer != nil && *s.Issuer != ""
}

func (s *OpenIDSettings) IsValid(siteURL string) *AppError {
	if s.Enable == nil || !*s.Enable {
		return nil
	}

	if s.Issuer == nil || strings.TrimSpace(*s.Issuer) == "" {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.issuer.app_error", nil, "", http.StatusBadRequest)
	}

	issuerURL, err := url.Parse(*s.Issuer)
	if err != nil || issuerURL.Scheme == "" || issuerURL.Host == "" {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.issuer.app_error", nil, "", http.StatusBadRequest)
	}
	if issuerURL.RawQuery != "" || issuerURL.Fragment != "" || issuerURL.User != nil {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.issuer.app_error", nil, "", http.StatusBadRequest)
	}

	devLoopback := s.DevAllowHTTPLoopback != nil && *s.DevAllowHTTPLoopback
	if !devLoopback && issuerURL.Scheme != "https" {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.issuer_https.app_error", nil, "", http.StatusBadRequest)
	}
	if devLoopback && !isLoopbackHTTPOrigin(issuerURL, siteURL) {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.dev_loopback.app_error", nil, "", http.StatusBadRequest)
	}

	if s.Id == nil || strings.TrimSpace(*s.Id) == "" {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.client_id.app_error", nil, "", http.StatusBadRequest)
	}
	if s.Secret == nil || strings.TrimSpace(*s.Secret) == "" {
		return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.secret.app_error", nil, "", http.StatusBadRequest)
	}

	if s.DiscoveryEndpoint != nil && strings.TrimSpace(*s.DiscoveryEndpoint) != "" {
		derived := s.DerivedDiscoveryEndpoint()
		if strings.TrimRight(*s.DiscoveryEndpoint, "/") != strings.TrimRight(derived, "/") {
			return NewAppError("OpenIDSettings.IsValid", "model.config.is_valid.openid.discovery_mismatch.app_error", nil, "", http.StatusBadRequest)
		}
	}

	return nil
}

func isLoopbackHTTPOrigin(issuerURL *url.URL, siteURL string) bool {
	if issuerURL.Scheme != "http" {
		return false
	}
	if !isLiteralLoopbackHost(issuerURL.Hostname()) {
		return false
	}
	site, err := url.Parse(siteURL)
	if err != nil || site.Scheme != "http" {
		return false
	}
	return isLiteralLoopbackHost(site.Hostname())
}

func isLiteralLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}
