// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidcmobile

import (
	"reflect"
	"testing"
)

func TestResolveSettings(t *testing.T) {
	defaults := map[string]any{
		"Enable":      false,
		"ButtonText":  "Log in with OIDC",
		"ButtonColor": "#0058CC",
	}
	tests := []struct {
		name    string
		saved   map[string]any
		want    Config
		wantErr bool
	}{
		{
			name: "manifest defaults",
			want: Config{ButtonText: "Log in with OIDC", ButtonColor: "#0058CC"},
		},
		{
			name: "case insensitive overrides preserve other defaults",
			saved: map[string]any{
				"eNaBlE": true, "buttontext": "Company SSO",
			},
			want: Config{Enabled: true, ButtonText: "Company SSO", ButtonColor: "#0058CC"},
		},
		{
			name: "explicit empty button values",
			saved: map[string]any{
				"enable": true, "buttontext": "", "buttoncolor": "",
			},
			want: Config{Enabled: true},
		},
		{
			name: "private and runtime fields never enter public configuration",
			saved: map[string]any{
				"Enable": true,
				// A private value that JSON cannot marshal proves that private
				// settings are not even copied into the serialized intermediate.
				"ClientSecret": func() {},
				"IssuerURL":    "https://idp.example.test",
				"Available":    true,
			},
			want: Config{Enabled: true, ButtonText: "Log in with OIDC", ButtonColor: "#0058CC"},
		},
		{
			name:    "string enable is invalid",
			saved:   map[string]any{"Enable": "true"},
			wantErr: true,
		},
		{
			name:    "nonstring label is invalid",
			saved:   map[string]any{"Enable": true, "ButtonText": []string{"SSO"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSettings(defaults, tt.saved)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ResolveSettings() = %#v, want %#v", got, tt.want)
			}
		})
	}
	if !reflect.DeepEqual(defaults, map[string]any{
		"Enable": false, "ButtonText": "Log in with OIDC", "ButtonColor": "#0058CC",
	}) {
		t.Fatal("manifest defaults were mutated")
	}
}

func TestResolveSettingsWithoutManifest(t *testing.T) {
	config, err := ResolveSettings(nil, map[string]any{"enable": true})
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled || config.Available {
		t.Fatalf("saved Enable should identify an unavailable plugin without advertising it: %#v", config)
	}
	config, err = ResolveSettings(nil, nil)
	if err != nil || config != (Config{}) {
		t.Fatalf("absent plugin settings should be disabled, got %#v, %v", config, err)
	}
}

func TestIsMobileUserAgent(t *testing.T) {
	for _, tt := range []struct {
		userAgent string
		want      bool
	}{
		{"Mattermost Mobile/2.42.0+1 (iOS; 17; iPhone)", true},
		{"Mattermost Mobile/2.42.0 (Android; 15)", true},
		{"Mozilla/5.0 Mattermost/5.7 Electron", false},
		{"Mozilla/5.0 (iPhone) Mobile/15E148 Safari/604.1", false},
		{"Mozilla/5.0 (Linux; Android 15) Chrome Mobile", false},
		{"", false},
	} {
		if got := IsMobileUserAgent(tt.userAgent); got != tt.want {
			t.Errorf("IsMobileUserAgent(%q) = %v, want %v", tt.userAgent, got, tt.want)
		}
	}
}
