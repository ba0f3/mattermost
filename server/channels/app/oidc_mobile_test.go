// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/v8/channels/app/oidcmobile"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

type mobileOIDCTestConfigService struct {
	configService
	config *model.Config
}

func (s mobileOIDCTestConfigService) Config() *model.Config {
	return s.config
}

func newMobileOIDCTestApp(config *model.Config) *App {
	return &App{ch: &Channels{
		cfgSvc: mobileOIDCTestConfigService{config: config},
		srv:    &Server{platform: &platform.PlatformService{}},
	}}
}

func TestGetMobileOIDCPluginConfigUnavailable(t *testing.T) {
	for _, tt := range []struct {
		name          string
		nativeEnabled bool
		pluginEnabled bool
		globalEnabled bool
		wantEnabled   bool
	}{
		{"native OpenID takes precedence", true, true, true, false},
		{"plugin environment uninitialized", false, true, true, true},
		{"plugins globally disabled", false, true, false, true},
		{"OIDC setting disabled", false, false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := &model.Config{}
			config.OpenIdSettings.Enable = new(tt.nativeEnabled)
			config.PluginSettings.Enable = new(tt.globalEnabled)
			config.PluginSettings.Plugins = map[string]map[string]any{
				oidcmobile.PluginID: {"Enable": tt.pluginEnabled},
			}
			got := newMobileOIDCTestApp(config).GetMobileOIDCPluginConfig()
			if got.Enabled != tt.wantEnabled || got.Available {
				t.Fatalf("unexpected availability: %#v", got)
			}
		})
	}
}

func TestGetMobileOIDCPluginConfigRuntimeState(t *testing.T) {
	pluginDir := t.TempDir()
	bundleDir := filepath.Join(pluginDir, oidcmobile.PluginID)
	if err := os.Mkdir(bundleDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"id": "mattermost-oidc",
		"name": "OIDC",
		"version": "0.1.0",
		"webapp": {"bundle_path": "webapp/main.js"},
		"settings_schema": {
			"settings": [
				{"key": "Enable", "type": "bool", "default": false},
				{"key": "ButtonText", "type": "text", "default": "Log in with OIDC"},
				{"key": "ClientSecret", "type": "text", "default": "private"}
			],
			"sections": [{
				"key": "appearance",
				"settings": [{"key": "ButtonColor", "type": "text", "default": "#0058CC"}]
			}]
		}
	}`
	if err := os.WriteFile(filepath.Join(bundleDir, "plugin.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	// A webapp-only fixture exercises real plugin runtime transitions without
	// launching a plugin process or requiring a database.
	if err := os.Mkdir(filepath.Join(bundleDir, "webapp"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "webapp", "main.js"), []byte("// test plugin"), 0600); err != nil {
		t.Fatal(err)
	}
	environment, err := plugin.NewEnvironment(nil, nil, pluginDir, t.TempDir(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	config := &model.Config{}
	config.PluginSettings.Enable = new(true)
	config.PluginSettings.Plugins = map[string]map[string]any{
		oidcmobile.PluginID: {"enable": true, "BUTTONtext": "Company SSO"},
	}
	config.PluginSettings.PluginStates = map[string]*model.PluginState{
		oidcmobile.PluginID: {Enable: true},
	}
	a := newMobileOIDCTestApp(config)
	a.ch.pluginsEnvironment = environment

	got := a.GetMobileOIDCPluginConfig()
	want := oidcmobile.Config{Enabled: true, Available: false, ButtonText: "Company SSO", ButtonColor: "#0058CC"}
	if got != want {
		t.Fatalf("installed but stopped plugin = %#v, want %#v", got, want)
	}
	if _, activated, err := environment.Activate(oidcmobile.PluginID); err != nil || !activated {
		t.Fatalf("activate test plugin: activated=%v, err=%v", activated, err)
	}
	t.Cleanup(func() { environment.Deactivate(oidcmobile.PluginID) })
	want.Available = true
	if got := a.GetMobileOIDCPluginConfig(); got != want {
		t.Fatalf("running plugin = %#v, want %#v", got, want)
	}

	// A config change must stop advertising the plugin even before its runtime
	// processes the disable, while retaining selection for a useful login error.
	config.PluginSettings.PluginStates[oidcmobile.PluginID].Enable = false
	want.Available = false
	if got := a.GetMobileOIDCPluginConfig(); got != want {
		t.Fatalf("disabled plugin = %#v, want %#v", got, want)
	}
	config.PluginSettings.PluginStates[oidcmobile.PluginID].Enable = true
	environment.Deactivate(oidcmobile.PluginID)
	if got := a.GetMobileOIDCPluginConfig(); got != want {
		t.Fatalf("stopped plugin = %#v, want %#v", got, want)
	}
}
