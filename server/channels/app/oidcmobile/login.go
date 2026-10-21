// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidcmobile

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrInvalidRedirect = errors.New("invalid mobile OIDC callback")
	ErrInvalidSiteURL  = errors.New("invalid Mattermost SiteURL")
)

// LoginURL converts the app's mobile login callback into the plugin's connect
// URL. The canonical SiteURL supplies the origin and optional deployment path;
// an untrusted request Host must never determine this redirect's destination.
func LoginURL(siteURL, redirectTo string) (string, error) {
	if err := validateRedirect(redirectTo); err != nil {
		return "", err
	}

	site, err := url.Parse(siteURL)
	if err != nil || site.Host == "" || site.User != nil ||
		(site.Scheme != "https" && site.Scheme != "http") ||
		site.RawQuery != "" || site.ForceQuery || site.Fragment != "" {
		return "", ErrInvalidSiteURL
	}

	site.Path = strings.TrimRight(site.Path, "/") + "/plugins/" + PluginID + "/oauth2/connect"
	site.RawPath = ""
	site.RawQuery = url.Values{"mobile_redirect": {redirectTo}}.Encode()
	return site.String(), nil
}

func validateRedirect(redirectTo string) error {
	allowed := false
	for _, callback := range []string{"mmauth://callback", "mmauthbeta://callback"} {
		if redirectTo == callback || strings.HasPrefix(redirectTo, callback+"?") {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrInvalidRedirect
	}
	callback, err := url.Parse(redirectTo)
	if err != nil || callback.Fragment != "" || strings.Contains(redirectTo, "#") {
		return ErrInvalidRedirect
	}
	// ParseQuery catches invalid escaping instead of silently dropping callback
	// parameters. Valid queries are retained byte-for-byte inside mobile_redirect.
	if _, err := url.ParseQuery(callback.RawQuery); err != nil {
		return fmt.Errorf("%w: malformed callback query", ErrInvalidRedirect)
	}
	return nil
}
