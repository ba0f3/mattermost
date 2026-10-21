// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/platform/services/oidc"
)

func (a *App) ResolveOIDCUser(rctx request.CTX, identity oidc.VerifiedIdentity, allowJIT bool) (*model.User, *model.AppError) {
	identityKey, err := oidc.IdentityKey(identity.Issuer, identity.Subject)
	if err != nil {
		return nil, model.NewAppError("ResolveOIDCUser", "api.user.authorize_oauth_user.token_failed.app_error", nil, oidc.Code(err), http.StatusUnauthorized)
	}

	mapping, mapErr := a.Srv().Store().OIDCIdentity().Get(rctx, identityKey)
	if mapErr != nil && !store.IsErrNotFound(mapErr) {
		return nil, model.NewAppError("ResolveOIDCUser", "app.user.get_by_auth.other.app_error", nil, "", http.StatusInternalServerError).Wrap(mapErr)
	}

	authData := identityKey
	user, userErr := a.Srv().Store().User().GetByAuth(&authData, model.ServiceOpenid)
	if userErr != nil && !store.IsErrNotFound(userErr) {
		return nil, model.NewAppError("ResolveOIDCUser", "app.user.get_by_auth.other.app_error", nil, "", http.StatusInternalServerError).Wrap(userErr)
	}

	if mapping != nil && user != nil && mapping.UserId != user.Id {
		return nil, oidcAppError("oidc_identity_conflict", http.StatusConflict)
	}

	if user != nil {
		if mapping == nil {
			_ = a.Srv().Store().OIDCIdentity().Ensure(rctx, &model.OIDCIdentity{
				IdentityKey: identityKey,
				UserId:      user.Id,
				Issuer:      identity.Issuer,
				Subject:     identity.Subject,
			})
		}
		if appErr := a.syncOIDCProfile(rctx, user, identity); appErr != nil {
			return nil, appErr
		}
		return user, nil
	}

	if mapping != nil {
		return nil, oidcAppError("oidc_identity_conflict", http.StatusConflict)
	}

	if !allowJIT {
		return nil, oidcAppError("oidc_account_not_provisioned", http.StatusForbidden)
	}

	return a.jitProvisionOIDCUser(rctx, identity, identityKey)
}

func (a *App) jitProvisionOIDCUser(rctx request.CTX, identity oidc.VerifiedIdentity, identityKey string) (*model.User, *model.AppError) {
	cfg := a.Config()
	if !*cfg.TeamSettings.EnableUserCreation {
		return nil, oidcAppError("oidc_account_not_provisioned", http.StatusForbidden)
	}

	empty, err := a.Srv().Store().User().IsEmpty(true)
	if err != nil {
		return nil, model.NewAppError("jitProvisionOIDCUser", "app.user.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if empty {
		return nil, oidcAppError("oidc_account_not_provisioned", http.StatusForbidden)
	}

	username := model.CleanUsername(rctx.Logger(), identity.PreferredUsername)
	if username == "" || (cfg.OpenIdSettings.UsePreferredUsername != nil && !*cfg.OpenIdSettings.UsePreferredUsername) {
		username = model.CleanUsername(rctx.Logger(), identity.Email)
	}
	if username == "" {
		username = "oidcuser"
	}
	username = a.suggestOIDCUsername(rctx, username, identityKey)

	user := &model.User{
		Username:      username,
		Email:         identity.Email,
		EmailVerified: true,
		AuthService:   model.ServiceOpenid,
		AuthData:      &identityKey,
		FirstName:     identity.GivenName,
		LastName:      identity.FamilyName,
	}

	created, createErr := a.CreateUser(rctx, user)
	if createErr != nil {
		authData := identityKey
		if existing, getErr := a.Srv().Store().User().GetByAuth(&authData, model.ServiceOpenid); getErr == nil {
			_ = a.Srv().Store().OIDCIdentity().Ensure(rctx, &model.OIDCIdentity{
				IdentityKey: identityKey,
				UserId:      existing.Id,
				Issuer:      identity.Issuer,
				Subject:     identity.Subject,
			})
			return existing, nil
		}
		return nil, oidcAppError("oidc_linking_required", http.StatusForbidden)
	}

	if err := a.Srv().Store().OIDCIdentity().Ensure(rctx, &model.OIDCIdentity{
		IdentityKey: identityKey,
		UserId:      created.Id,
		Issuer:      identity.Issuer,
		Subject:     identity.Subject,
	}); err != nil {
		return nil, model.NewAppError("jitProvisionOIDCUser", "app.user.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return created, nil
}

func (a *App) suggestOIDCUsername(rctx request.CTX, base string, identityKey string) string {
	username := base
	for i := 0; i < 5; i++ {
		if _, err := a.Srv().Store().User().GetByUsername(username); store.IsErrNotFound(err) {
			return username
		}
		suffix := identityKey[len(identityKey)-4:]
		username = model.CleanUsername(rctx.Logger(), base+suffix)
	}
	return model.CleanUsername(rctx.Logger(), base+identityKey[len(identityKey)-8:])
}

func (a *App) syncOIDCProfile(rctx request.CTX, user *model.User, identity oidc.VerifiedIdentity) *model.AppError {
	if user.Email != identity.Email {
		if existing, err := a.Srv().Store().User().GetByEmail(identity.Email); err == nil && existing.Id != user.Id {
			return oidcAppError("oidc_authentication_failed", http.StatusUnauthorized)
		}
	}
	updated := false
	if user.Email != identity.Email {
		user.Email = identity.Email
		updated = true
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		updated = true
	}
	if user.FirstName != identity.GivenName {
		user.FirstName = identity.GivenName
		updated = true
	}
	if user.LastName != identity.FamilyName {
		user.LastName = identity.FamilyName
		updated = true
	}
	if updated {
		user.UpdateAt = model.GetMillis()
		if _, err := a.Srv().Store().User().Update(rctx, user, true); err != nil {
			return model.NewAppError("syncOIDCProfile", "app.user.update.find.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		a.InvalidateCacheForUser(user.Id)
	}
	return nil
}

func (a *App) preflightOIDCUser(user *model.User) *model.AppError {
	if user.DeleteAt > 0 {
		return oidcAppError("oidc_user_disabled", http.StatusForbidden)
	}
	if user.IsBot || user.IsRemote() {
		return oidcAppError("oidc_authentication_failed", http.StatusForbidden)
	}
	return nil
}

func oidcAppError(code string, status int) *model.AppError {
	return model.NewAppError("OIDC", "api.user.authorize_oauth_user.token_failed.app_error", map[string]any{"OIDCCode": code}, code, status)
}
