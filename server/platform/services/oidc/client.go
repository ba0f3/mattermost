// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// AuthorizationInput carries the outbound authorization request parameters.
type AuthorizationInput struct {
	State        string
	Nonce        string
	CodeVerifier string
}

// CallbackInput carries callback parameters for code exchange and verification.
type CallbackInput struct {
	Code          string
	ExpectedNonce string
	CodeVerifier  string
}

// Client is an immutable OIDC client for a single config snapshot.
type Client struct {
	cfg    Config
	oauth2 oauth2.Config
	cache  providerCache
	http   *http.Client
}

// NewClient constructs a client for the given config snapshot.
func NewClient(cfg Config, httpClient *http.Client) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if httpClient == nil {
		return nil, ErrConfigInvalid
	}

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     oauth2.Endpoint{AuthURL: "", TokenURL: ""},
		Scopes:       cfg.Scopes,
	}

	return &Client{
		cfg:    cfg,
		oauth2: oauthCfg,
		http:   httpClient,
	}, nil
}

func (c *Client) Fingerprint() string {
	return c.cfg.Fingerprint
}

func (c *Client) Config() Config {
	return c.cfg
}

// AuthorizationURL builds the provider authorization URL with PKCE.
func (c *Client) AuthorizationURL(ctx context.Context, in AuthorizationInput) (string, error) {
	provider, err := c.cache.get(ctx, c.http, c.cfg)
	if err != nil {
		return "", err
	}

	endpoint := provider.Endpoint()
	c.oauth2.Endpoint = oauth2.Endpoint{
		AuthURL:  endpoint.AuthURL,
		TokenURL: endpoint.TokenURL,
	}

	return c.oauth2.AuthCodeURL(
		in.State,
		oauth2.SetAuthURLParam("nonce", in.Nonce),
		oauth2.S256ChallengeOption(in.CodeVerifier),
	), nil
}

// ExchangeAndVerify exchanges the authorization code and verifies the ID token.
func (c *Client) ExchangeAndVerify(ctx context.Context, in CallbackInput) (VerifiedIdentity, error) {
	provider, err := c.cache.get(ctx, c.http, c.cfg)
	if err != nil {
		return VerifiedIdentity{}, err
	}

	endpoint := provider.Endpoint()
	c.oauth2.Endpoint = oauth2.Endpoint{
		AuthURL:  endpoint.AuthURL,
		TokenURL: endpoint.TokenURL,
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.http)
	token, err := c.oauth2.Exchange(ctx, in.Code, oauth2.VerifierOption(in.CodeVerifier))
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return VerifiedIdentity{}, ErrAuthenticationFailed
	}

	return verifyIDToken(ctx, provider, c.cfg, rawIDToken, in.ExpectedNonce)
}
