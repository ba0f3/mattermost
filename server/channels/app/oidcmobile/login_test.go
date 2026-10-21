// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidcmobile

import (
	"errors"
	"net/url"
	"testing"
)

func TestLoginURL(t *testing.T) {
	for _, tt := range []struct {
		name       string
		siteURL    string
		redirectTo string
		wantPath   string
	}{
		{"production app", "https://chat.example.test", "mmauth://callback", "/plugins/mattermost-oidc/oauth2/connect"},
		{"beta app", "https://chat.example.test/", "mmauthbeta://callback", "/plugins/mattermost-oidc/oauth2/connect"},
		{"deployment subpath", "https://chat.example.test/team/chat/", "mmauth://callback", "/team/chat/plugins/mattermost-oidc/oauth2/connect"},
		{"escaped deployment subpath", "https://chat.example.test/team%20chat", "mmauth://callback", "/team chat/plugins/mattermost-oidc/oauth2/connect"},
		{"callback query", "https://chat.example.test", "mmauthbeta://callback?state=a%26b&name=a+b&next=x%3Dy", "/plugins/mattermost-oidc/oauth2/connect"},
		{"empty callback query", "http://chat.example.test:8065", "mmauth://callback?", "/plugins/mattermost-oidc/oauth2/connect"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoginURL(tt.siteURL, tt.redirectTo)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(got)
			if err != nil {
				t.Fatal(err)
			}
			site, _ := url.Parse(tt.siteURL)
			if parsed.Scheme != site.Scheme || parsed.Host != site.Host || parsed.Path != tt.wantPath {
				t.Fatalf("unexpected plugin destination: %s", got)
			}
			query := parsed.Query()
			if len(query) != 1 || len(query["mobile_redirect"]) != 1 || query.Get("mobile_redirect") != tt.redirectTo {
				t.Fatalf("callback was changed or leaked into outer query: %s", got)
			}
		})
	}
}

func TestLoginURLRejectsInvalidCallbacks(t *testing.T) {
	for _, callback := range []string{
		"",
		"https://evil.example/callback",
		"//evil.example/callback",
		"mmauth://callback.evil.example",
		"mmauth://callback@evil.example",
		"mmauth://callback/path",
		"mmauth://callback/",
		"mmauth://callback:443",
		"mmauth://callback#fragment",
		"mmauth://callback?state=ok#fragment",
		"mmauth://callback?state=ok#",
		"mmauth://callback?state=%zz",
		"mmauth://callback?state=ok\r\nLocation: https://evil.example",
		"mmauthbeta://callback.evil.example?state=ok",
	} {
		if got, err := LoginURL("https://chat.example.test", callback); got != "" || !errors.Is(err, ErrInvalidRedirect) {
			t.Errorf("LoginURL callback %q = (%q, %v), want ErrInvalidRedirect", callback, got, err)
		}
	}
}

func TestLoginURLRejectsInvalidSiteURL(t *testing.T) {
	for _, siteURL := range []string{
		"",
		"/mattermost",
		"https:///mattermost",
		"javascript://chat.example.test",
		"https://user:password@chat.example.test",
		"https://chat.example.test?query=unexpected",
		"https://chat.example.test?",
		"https://chat.example.test#fragment",
		"https://chat.example.test/%zz",
	} {
		if got, err := LoginURL(siteURL, "mmauth://callback"); got != "" || !errors.Is(err, ErrInvalidSiteURL) {
			t.Errorf("LoginURL site %q = (%q, %v), want ErrInvalidSiteURL", siteURL, got, err)
		}
	}
}
