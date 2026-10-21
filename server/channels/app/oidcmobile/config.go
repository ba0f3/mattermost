// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package oidcmobile implements the native mobile compatibility contract for the
// mattermost-oidc plugin. Authentication and session creation stay in the plugin.
package oidcmobile

import (
	"encoding/json"
	"fmt"
	"strings"
)

const PluginID = "mattermost-oidc"

// Config contains only the public settings needed to offer the plugin's mobile
// login. Enabled distinguishes a selected but unavailable plugin from a request
// that should follow Mattermost's existing native OpenID flow.
type Config struct {
	Enabled     bool   `json:"enable"`
	Available   bool   `json:"-"`
	ButtonText  string `json:"buttontext"`
	ButtonColor string `json:"buttoncolor"`
}

// ResolveSettings applies manifest defaults and saved settings with the same
// case-insensitive keys as LoadPluginConfiguration. Private settings are neither
// copied nor serialized. Runtime availability is deliberately resolved by the
// caller rather than inferred from the configured Enable setting.
func ResolveSettings(defaults, saved map[string]any) (Config, error) {
	public := make(map[string]any, 3)
	for _, source := range []map[string]any{defaults, saved} {
		for key, value := range source {
			switch normalized := strings.ToLower(key); normalized {
			case "enable", "buttontext", "buttoncolor":
				public[normalized] = value
			}
		}
	}

	encoded, err := json.Marshal(public)
	if err != nil {
		return Config{}, fmt.Errorf("encode mobile OIDC plugin settings: %w", err)
	}
	var config Config
	if err := json.Unmarshal(encoded, &config); err != nil {
		return Config{}, fmt.Errorf("decode mobile OIDC plugin settings: %w", err)
	}
	return config, nil
}

// IsMobileUserAgent matches the native app's identifier, not mobile browsers.
// The authentication browser can have a different User-Agent, so callers must
// not use this predicate to restrict the mobile_login endpoint itself.
func IsMobileUserAgent(userAgent string) bool {
	return strings.Contains(userAgent, "Mattermost Mobile/")
}
