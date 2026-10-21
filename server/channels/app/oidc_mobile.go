// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/oidcmobile"
)

// GetMobileOIDCPluginConfig resolves mobile login availability from local plugin
// state and effective settings. It does not contact the plugin or identity
// provider. Explicitly enabled native OpenID configuration keeps precedence.
func (a *App) GetMobileOIDCPluginConfig() oidcmobile.Config {
	cfg := a.Config()
	if cfg.OpenIdSettings.Enable != nil && *cfg.OpenIdSettings.Enable {
		return oidcmobile.Config{}
	}

	defaults := make(map[string]any)
	manifestAvailable := false
	pluginsEnabled := cfg.PluginSettings.Enable != nil && *cfg.PluginSettings.Enable
	if pluginsEnabled {
		if environment := a.GetPluginsEnvironment(); environment != nil {
			if manifest, err := environment.GetManifest(oidcmobile.PluginID); err == nil && manifest != nil {
				manifestAvailable = true
				if schema := manifest.SettingsSchema; schema != nil {
					copyMobileOIDCDefaults(defaults, schema.Settings)
					for _, section := range schema.Sections {
						if section != nil {
							copyMobileOIDCDefaults(defaults, section.Settings)
						}
					}
				}
			}
		}
	}

	config, err := oidcmobile.ResolveSettings(defaults, cfg.PluginSettings.Plugins[oidcmobile.PluginID])
	if err != nil {
		return oidcmobile.Config{}
	}
	if !config.Enabled || !pluginsEnabled || !manifestAvailable {
		return config
	}
	// A config change can disable a plugin before its process has stopped. Do not
	// continue advertising it during that transition even if it is still running.
	state := cfg.PluginSettings.PluginStates[oidcmobile.PluginID]
	if state == nil || !state.Enable {
		return config
	}
	active, err := a.IsPluginActive(oidcmobile.PluginID)
	config.Available = err == nil && active
	return config
}

func copyMobileOIDCDefaults(defaults map[string]any, settings []*model.PluginSetting) {
	for _, setting := range settings {
		if setting == nil {
			continue
		}
		switch key := strings.ToLower(setting.Key); key {
		case "enable", "buttontext", "buttoncolor":
			defaults[key] = setting.Default
		}
	}
}
