// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/oidcmobile"
)

func mobileLoginWithOIDCPlugin(c *Context, w http.ResponseWriter, r *http.Request, pluginConfig oidcmobile.Config) {
	auditRec := c.MakeAuditRecord(model.AuditEventMobileLoginWithOAuth, model.AuditStatusFail)
	auditRec.AddMeta("service", c.Params.Service)
	auditRec.AddMeta("plugin_id", oidcmobile.PluginID)
	defer c.LogAuditRec(auditRec)

	if err := redirectMobileLoginToOIDCPlugin(w, r, c.App.GetSiteURL(), pluginConfig); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")
}

// No User-Agent check belongs here: the app's authentication browser may use
// an ordinary browser UA. Only the client-config overlay is mobile-UA scoped.
func redirectMobileLoginToOIDCPlugin(w http.ResponseWriter, r *http.Request, siteURL string, pluginConfig oidcmobile.Config) *model.AppError {
	w.Header().Set("Cache-Control", "no-store")
	if !pluginConfig.Enabled || !pluginConfig.Available {
		return model.NewAppError("mobileLoginWithOIDCPlugin", "api.user.login_by_oauth.not_available.app_error",
			map[string]any{"Service": "OpenID Connect"}, "mattermost-oidc plugin is not running or OIDC is disabled", http.StatusServiceUnavailable)
	}

	loginURL, err := oidcmobile.LoginURL(siteURL, r.URL.Query().Get("redirect_to"))
	if errors.Is(err, oidcmobile.ErrInvalidRedirect) {
		return model.NewAppError("mobileLoginWithOIDCPlugin", "api.invalid_custom_url_scheme", nil, "", http.StatusBadRequest)
	}
	if err != nil {
		return model.NewAppError("mobileLoginWithOIDCPlugin", "api.user.login_by_oauth.not_available.app_error",
			map[string]any{"Service": "OpenID Connect"}, "invalid Mattermost SiteURL", http.StatusInternalServerError)
	}

	http.Redirect(w, r, loginURL, http.StatusFound)
	return nil
}
