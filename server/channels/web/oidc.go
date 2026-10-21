// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

const maxPendingOIDCCookies = 5

func (w *Web) InitOIDC() {
	w.MainRouter.Handle("/oauth/openid/login", w.APIHandler(oidcLogin)).Methods(http.MethodGet)
	w.MainRouter.Handle("/oauth/openid/signup", w.APIHandler(oidcSignup)).Methods(http.MethodGet)
	w.MainRouter.Handle("/signup/openid/complete", w.APIHandler(oidcComplete)).Methods(http.MethodGet)
	w.MainRouter.Handle("/oauth/openid/mobile_login", w.APIHandler(oidcMobileUnsupported)).Methods(http.MethodGet)
}

func oidcLogin(c *Context, w http.ResponseWriter, r *http.Request) {
	startOIDC(c, w, r, false)
}

func oidcSignup(c *Context, w http.ResponseWriter, r *http.Request) {
	startOIDC(c, w, r, true)
}

func startOIDC(c *Context, w http.ResponseWriter, r *http.Request, signup bool) {
	if !c.App.OSSOIDCEnabled() {
		c.Err = model.NewAppError("startOIDC", "api.user.authorize_oauth_user.unsupported.app_error", nil, "", http.StatusNotImplemented)
		return
	}
	if signup && !*c.App.Config().TeamSettings.EnableUserCreation {
		c.Err = model.NewAppError("startOIDC", "api.oauth.signup_with_oauth.disabled.app_error", nil, "", http.StatusBadRequest)
		return
	}
	if app.CountPendingOIDCCookies(r) >= maxPendingOIDCCookies {
		c.Err = model.NewAppError("startOIDC", "api.user.authorize_oauth_user.token_failed.app_error", nil, "oidc_transaction_invalid", http.StatusTooManyRequests)
		return
	}

	returnTo := r.URL.Query().Get("redirect_to")
	start, err := c.App.BeginOIDCLogin(c.AppContext, app.OIDCLoginInput{
		ReturnTo: returnTo,
		AllowJIT: signup || (c.App.Config().OpenIdSettings.EnableJIT != nil && *c.App.Config().OpenIdSettings.EnableJIT),
	})
	if err != nil {
		c.Err = err
		return
	}

	devLoopback := c.App.Config().OpenIdSettings.DevAllowHTTPLoopback != nil && *c.App.Config().OpenIdSettings.DevAllowHTTPLoopback
	cookie := &http.Cookie{
		Name:     start.CookieName,
		Value:    start.BrowserSecret,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(5 * time.Minute / time.Second),
	}
	if devLoopback {
		cookie.Secure = false
	} else {
		cookie.Secure = true
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, start.AuthURL, http.StatusFound)
}

func oidcComplete(c *Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if !c.App.OSSOIDCEnabled() {
		c.Err = model.NewAppError("oidcComplete", "api.user.authorize_oauth_user.unsupported.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		c.Err = model.NewAppError("oidcComplete", "api.user.authorize_oauth_user.token_failed.app_error", nil, "oidc_transaction_invalid", http.StatusBadRequest)
		return
	}

	devLoopback := c.App.Config().OpenIdSettings.DevAllowHTTPLoopback != nil && *c.App.Config().OpenIdSettings.DevAllowHTTPLoopback
	cookieName := app.OIDCCookieName(devLoopback, state)
	browserCookie, err := r.Cookie(cookieName)
	if err != nil || browserCookie.Value == "" {
		c.Err = model.NewAppError("oidcComplete", "api.user.authorize_oauth_user.token_failed.app_error", nil, "oidc_transaction_invalid", http.StatusBadRequest)
		return
	}

	result, appErr := c.App.CompleteOIDCLogin(c.AppContext, app.OIDCCallbackInput{
		State:         state,
		Code:          r.URL.Query().Get("code"),
		OAuthError:    r.URL.Query().Get("error"),
		BrowserSecret: browserCookie.Value,
	})
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	if appErr != nil {
		c.Err = appErr
		return
	}

	session, appErr := c.App.DoLogin(c.AppContext, w, r, result.User, model.LoginOptions{
		IsOAuthUser: true,
	})
	if appErr != nil {
		c.Err = appErr
		return
	}
	c.AppContext = c.AppContext.WithSession(session)
	c.App.AttachSessionCookies(c.AppContext, w, r)

	redirectURL := result.ReturnTo
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func oidcMobileUnsupported(c *Context, w http.ResponseWriter, r *http.Request) {
	c.Err = model.NewAppError("oidcMobileUnsupported", "api.user.authorize_oauth_user.unsupported.app_error", nil, "oidc_client_unsupported", http.StatusNotImplemented)
}
