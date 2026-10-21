// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/platform/services/oidc"
)

const (
	oidcTransactionVersion = 1
	maxOIDCExtraBytes      = 2048
	maxReturnToBytes       = 512
	maxPendingOIDCCookies  = 5
)

type oidcLoginTransaction struct {
	Version             int    `json:"version"`
	BrowserBindingHash  string `json:"browser_binding_hash"`
	Nonce               string `json:"nonce"`
	CodeVerifier        string `json:"code_verifier"`
	ConfigFingerprint   string `json:"config_fingerprint"`
	ReturnTo            string `json:"return_to"`
}

func HashOIDCState(state string) string {
	return hashState(state)
}

func OIDCCookieName(devLoopback bool, state string) string {
	return oidcCookieName(devLoopback, hashState(state))
}

func CountPendingOIDCCookies(r *http.Request) int {
	return countPendingOIDCCookies(r)
}

func hashState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

func hashBrowserSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func validateReturnTo(siteURL string, returnTo string) (string, error) {
	if returnTo == "" {
		return "/", nil
	}
	if len(returnTo) > maxReturnToBytes {
		return "", oidc.ErrTransactionInvalid
	}
	if strings.Contains(returnTo, "\\") || strings.Contains(returnTo, "//") {
		return "", oidc.ErrTransactionInvalid
	}
	if !strings.HasPrefix(returnTo, "/") {
		return "", oidc.ErrTransactionInvalid
	}
	clean := path.Clean(returnTo)
	if !strings.HasPrefix(clean, "/") {
		return "", oidc.ErrTransactionInvalid
	}
	parsedSite, err := url.Parse(siteURL)
	if err != nil {
		return "", oidc.ErrTransactionInvalid
	}
	prefix := path.Clean(parsedSite.Path)
	if prefix == "." {
		prefix = ""
	}
	if prefix != "" && !strings.HasPrefix(clean, prefix) {
		return "", oidc.ErrTransactionInvalid
	}
	return clean, nil
}

func (a *App) saveOIDCLoginTransaction(rctx request.CTX, state, browserSecret string, extra oidcLoginTransaction) error {
	payload, err := json.Marshal(extra)
	if err != nil || len(payload) > maxOIDCExtraBytes {
		return oidc.ErrTransactionInvalid
	}

	token := &model.Token{
		Token:    hashState(state),
		CreateAt: model.GetMillis(),
		Type:     model.TokenTypeOIDCLogin,
		Extra:    string(payload),
	}
	if appErr := token.IsValid(); appErr != nil {
		return oidc.ErrTransactionInvalid
	}
	if err := a.Srv().Store().Token().Save(token); err != nil {
		return oidc.ErrTransactionInvalid
	}
	return nil
}

func (a *App) loadOIDCLoginTransaction(rctx request.CTX, state, browserSecret string) (*oidcLoginTransaction, error) {
	tokenKey := hashState(state)
	tok, err := a.Srv().Store().Token().GetByTokenFromMaster(tokenKey)
	if err != nil {
		return nil, oidc.ErrTransactionInvalid
	}
	if tok.Type != model.TokenTypeOIDCLogin || tok.IsExpired() {
		return nil, oidc.ErrTransactionExpired
	}

	var extra oidcLoginTransaction
	if err := json.Unmarshal([]byte(tok.Extra), &extra); err != nil {
		return nil, oidc.ErrTransactionInvalid
	}
	if extra.BrowserBindingHash != hashBrowserSecret(browserSecret) {
		return nil, oidc.ErrTransactionInvalid
	}
	return &extra, nil
}

func (a *App) consumeOIDCLoginTransaction(state string) (*oidcLoginTransaction, error) {
	tokenKey := hashState(state)
	tok, err := a.Srv().Store().Token().ConsumeOnce(model.TokenTypeOIDCLogin, tokenKey)
	if err != nil {
		return nil, oidc.ErrTransactionInvalid
	}
	var extra oidcLoginTransaction
	if err := json.Unmarshal([]byte(tok.Extra), &extra); err != nil {
		return nil, oidc.ErrTransactionInvalid
	}
	return &extra, nil
}

func oidcCookieName(devLoopback bool, stateDigest string) string {
	if devLoopback {
		return "mm_oidc_" + stateDigest[:16]
	}
	return "__Host-mm_oidc_" + stateDigest[:16]
}

func countPendingOIDCCookies(r *http.Request) int {
	count := 0
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "__Host-mm_oidc_") || strings.HasPrefix(c.Name, "mm_oidc_") {
			count++
		}
	}
	return count
}
