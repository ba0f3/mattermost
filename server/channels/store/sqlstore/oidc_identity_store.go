// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

type SqlOIDCIdentityStore struct {
	*SqlStore
}

func newSqlOIDCIdentityStore(sqlStore *SqlStore) store.OIDCIdentityStore {
	return &SqlOIDCIdentityStore{SqlStore: sqlStore}
}

func (s SqlOIDCIdentityStore) Ensure(rctx request.CTX, identity *model.OIDCIdentity) error {
	if identity == nil {
		return errors.New("oidc identity is nil")
	}
	now := model.GetMillis()
	if identity.CreatedAt == 0 {
		identity.CreatedAt = now
	}
	identity.UpdatedAt = now

	query, args, err := s.getQueryBuilder().
		Insert("OIDCIdentities").
		Columns("IdentityKey", "UserId", "Issuer", "Subject", "CreatedAt", "UpdatedAt").
		Values(identity.IdentityKey, identity.UserId, identity.Issuer, identity.Subject, identity.CreatedAt, identity.UpdatedAt).
		Suffix(`ON CONFLICT (IdentityKey) DO UPDATE SET UserId = EXCLUDED.UserId, Issuer = EXCLUDED.Issuer, Subject = EXCLUDED.Subject, UpdatedAt = EXCLUDED.UpdatedAt`).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "oidc_identity_ensure_tosql")
	}
	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrap(err, "failed to ensure oidc identity")
	}
	return nil
}

func (s SqlOIDCIdentityStore) Get(rctx request.CTX, identityKey string) (*model.OIDCIdentity, error) {
	var identity model.OIDCIdentity
	query, args, err := s.getQueryBuilder().
		Select("IdentityKey", "UserId", "Issuer", "Subject", "CreatedAt", "UpdatedAt").
		From("OIDCIdentities").
		Where(sq.Eq{"IdentityKey": identityKey}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "oidc_identity_get_tosql")
	}
	if err := s.GetReplica().Get(&identity, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("OIDCIdentity", fmt.Sprintf("IdentityKey=%s", identityKey))
		}
		return nil, errors.Wrap(err, "failed to get oidc identity")
	}
	return &identity, nil
}

func (s SqlOIDCIdentityStore) GetForUser(rctx request.CTX, userID string) (*model.OIDCIdentity, error) {
	var identity model.OIDCIdentity
	query, args, err := s.getQueryBuilder().
		Select("IdentityKey", "UserId", "Issuer", "Subject", "CreatedAt", "UpdatedAt").
		From("OIDCIdentities").
		Where(sq.Eq{"UserId": userID}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "oidc_identity_get_for_user_tosql")
	}
	if err := s.GetReplica().Get(&identity, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("OIDCIdentity", fmt.Sprintf("UserId=%s", userID))
		}
		return nil, errors.Wrap(err, "failed to get oidc identity for user")
	}
	return &identity, nil
}

func (s SqlOIDCIdentityStore) DeleteForUser(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM OIDCIdentities WHERE UserId = ?", userID); err != nil {
		return errors.Wrap(err, "failed to delete oidc identity for user")
	}
	return nil
}
