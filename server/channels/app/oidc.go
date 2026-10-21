// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/platform/services/oidc"
)

type OIDCLoginInput struct {
	ReturnTo string
	AllowJIT bool
}

type OIDCLoginStart struct {
	AuthURL       string
	State         string
	BrowserSecret string
	CookieName    string
}

type OIDCCallbackInput struct {
	State         string
	Code          string
	OAuthError    string
	BrowserSecret string
}

type OIDCLoginResult struct {
	User     *model.User
	ReturnTo string
}

func (a *App) OSSOIDCEnabled() bool {
	return a.Config().OpenIdSettings.IsOSSOIDCEnabled()
}

func (a *App) BeginOIDCLogin(rctx request.CTX, in OIDCLoginInput) (OIDCLoginStart, *model.AppError) {
	if !a.OSSOIDCEnabled() {
		return OIDCLoginStart{}, oidcAppError("oidc_disabled", http.StatusNotImplemented)
	}

	client, cfg, _, err := a.Srv().oidcManager.Current()
	if err != nil {
		return OIDCLoginStart{}, oidcAppError(oidc.Code(err), http.StatusBadRequest)
	}

	siteURL := *a.Config().ServiceSettings.SiteURL
	returnTo, vErr := validateReturnTo(siteURL, in.ReturnTo)
	if vErr != nil {
		return OIDCLoginStart{}, oidcAppError(oidc.Code(vErr), http.StatusBadRequest)
	}

	state, err := randomOIDCString(32)
	if err != nil {
		return OIDCLoginStart{}, model.NewAppError("BeginOIDCLogin", "api.user.authorize_oauth_user.token_failed.app_error", nil, "", http.StatusInternalServerError)
	}
	nonce, err := randomOIDCString(32)
	if err != nil {
		return OIDCLoginStart{}, model.NewAppError("BeginOIDCLogin", "api.user.authorize_oauth_user.token_failed.app_error", nil, "", http.StatusInternalServerError)
	}
	verifier := oauth2.GenerateVerifier()
	browserSecret, err := randomOIDCString(32)
	if err != nil {
		return OIDCLoginStart{}, model.NewAppError("BeginOIDCLogin", "api.user.authorize_oauth_user.token_failed.app_error", nil, "", http.StatusInternalServerError)
	}

	extra := oidcLoginTransaction{
		Version:            oidcTransactionVersion,
		BrowserBindingHash: hashBrowserSecret(browserSecret),
		Nonce:              nonce,
		CodeVerifier:       verifier,
		ConfigFingerprint:  cfg.Fingerprint,
		ReturnTo:           returnTo,
	}
	if err := a.saveOIDCLoginTransaction(rctx, state, browserSecret, extra); err != nil {
		return OIDCLoginStart{}, oidcAppError(oidc.Code(err), http.StatusBadRequest)
	}

	authURL, err := client.AuthorizationURL(rctx.Context(), oidc.AuthorizationInput{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
	})
	if err != nil {
		return OIDCLoginStart{}, oidcAppError(oidc.Code(err), http.StatusBadGateway)
	}

	cookieName := oidcCookieName(cfg.DevAllowHTTPLoopback, hashState(state))
	return OIDCLoginStart{
		AuthURL:       authURL,
		State:         state,
		BrowserSecret: browserSecret,
		CookieName:    cookieName,
	}, nil
}

func (a *App) CompleteOIDCLogin(rctx request.CTX, in OIDCCallbackInput) (OIDCLoginResult, *model.AppError) {
	if !a.OSSOIDCEnabled() {
		return OIDCLoginResult{}, oidcAppError("oidc_disabled", http.StatusNotImplemented)
	}

	extra, err := a.consumeOIDCLoginTransaction(in.State)
	if err != nil {
		return OIDCLoginResult{}, oidcAppError(oidc.Code(err), http.StatusBadRequest)
	}
	if extra.BrowserBindingHash != hashBrowserSecret(in.BrowserSecret) {
		return OIDCLoginResult{}, oidcAppError("oidc_transaction_invalid", http.StatusBadRequest)
	}

	client, cfg, generation, cerr := a.Srv().oidcManager.Current()
	if cerr != nil || extra.ConfigFingerprint != cfg.Fingerprint {
		return OIDCLoginResult{}, oidcAppError("oidc_config_changed", http.StatusBadRequest)
	}

	if in.OAuthError != "" {
		return OIDCLoginResult{}, oidcAppError("oidc_authentication_failed", http.StatusUnauthorized)
	}
	if in.Code == "" {
		return OIDCLoginResult{}, oidcAppError("oidc_transaction_invalid", http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(rctx.Context(), 15*time.Second)
	defer cancel()

	identity, err := client.ExchangeAndVerify(ctx, oidc.CallbackInput{
		Code:          in.Code,
		ExpectedNonce: extra.Nonce,
		CodeVerifier:  extra.CodeVerifier,
	})
	if err != nil {
		return OIDCLoginResult{}, oidcAppError(oidc.Code(err), http.StatusUnauthorized)
	}

	if err := a.Srv().oidcManager.ValidateGeneration(generation); err != nil {
		return OIDCLoginResult{}, oidcAppError(oidc.Code(err), http.StatusBadRequest)
	}

	allowJIT := a.Config().OpenIdSettings.EnableJIT != nil && *a.Config().OpenIdSettings.EnableJIT
	user, appErr := a.ResolveOIDCUser(rctx, identity, allowJIT)
	if appErr != nil {
		return OIDCLoginResult{}, appErr
	}
	if appErr = a.preflightOIDCUser(user); appErr != nil {
		return OIDCLoginResult{}, appErr
	}

	if err := a.Srv().oidcManager.ValidateGeneration(generation); err != nil {
		return OIDCLoginResult{}, oidcAppError(oidc.Code(err), http.StatusBadRequest)
	}

	return OIDCLoginResult{User: user, ReturnTo: extra.ReturnTo}, nil
}

func randomOIDCString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
