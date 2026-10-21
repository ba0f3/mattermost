// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/v8/channels/app/oidcmobile"
)

func TestRedirectMobileLoginToOIDCPlugin(t *testing.T) {
	available := oidcmobile.Config{Enabled: true, Available: true}
	testCases := []struct {
		name        string
		siteURL     string
		callback    string
		config      oidcmobile.Config
		wantPath    string
		wantErrCode int
	}{
		{
			name: "production app from an ordinary auth browser", siteURL: "https://chat.example.com",
			callback: "mmauth://callback", config: available,
			wantPath: "/plugins/mattermost-oidc/oauth2/connect",
		},
		{
			name: "beta app and SiteURL subpath", siteURL: "https://chat.example.com/mm/",
			callback: "mmauthbeta://callback", config: available,
			wantPath: "/mm/plugins/mattermost-oidc/oauth2/connect",
		},
		{
			name: "callback query survives without HTML escaping", siteURL: "https://chat.example.com",
			callback: "mmauth://callback?state=a%2Bb&label=A%26B", config: available,
			wantPath: "/plugins/mattermost-oidc/oauth2/connect",
		},
		{
			name: "missing callback", siteURL: "https://chat.example.com", config: available,
			wantErrCode: http.StatusBadRequest,
		},
		{
			name: "callback host lookalike", siteURL: "https://chat.example.com",
			callback: "mmauth://callback.evil.example", config: available,
			wantErrCode: http.StatusBadRequest,
		},
		{
			name: "stopped plugin does not redirect", siteURL: "https://chat.example.com",
			callback: "mmauth://callback", config: oidcmobile.Config{Enabled: true},
			wantErrCode: http.StatusServiceUnavailable,
		},
		{
			name: "disabled plugin does not redirect", siteURL: "https://chat.example.com",
			callback: "mmauth://callback", config: oidcmobile.Config{},
			wantErrCode: http.StatusServiceUnavailable,
		},
		{
			name: "invalid server SiteURL", siteURL: "//chat.example.com",
			callback: "mmauth://callback", config: available,
			wantErrCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query := url.Values{"redirect_to": {tc.callback}, "ignored": {"not-forwarded"}}
			r := httptest.NewRequest(http.MethodGet, "/oauth/openid/mobile_login?"+query.Encode(), nil)
			r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15")
			w := httptest.NewRecorder()

			err := redirectMobileLoginToOIDCPlugin(w, r, tc.siteURL, tc.config)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			if tc.wantErrCode != 0 {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.StatusCode)
				require.Empty(t, w.Header().Get("Location"))
				return
			}

			require.Nil(t, err)
			require.Equal(t, http.StatusFound, w.Code)
			location, parseErr := url.Parse(w.Header().Get("Location"))
			require.NoError(t, parseErr)
			require.Equal(t, "https", location.Scheme)
			require.Equal(t, "chat.example.com", location.Host)
			require.Equal(t, tc.wantPath, location.Path)
			require.Equal(t, url.Values{"mobile_redirect": {tc.callback}}, location.Query())
			require.Empty(t, w.Header().Values("Set-Cookie"), "the plugin, not this entry point, creates the session")
		})
	}
}
