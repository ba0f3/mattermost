// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type providerCache struct {
	mu          sync.Mutex
	cfg         Config
	provider    *oidc.Provider
	lastError   error
	lastAttempt time.Time
}

func (pc *providerCache) get(ctx context.Context, httpClient *http.Client, cfg Config) (*oidc.Provider, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.provider != nil && pc.cfg.Fingerprint == cfg.Fingerprint {
		return pc.provider, nil
	}

	if pc.lastError != nil && time.Since(pc.lastAttempt) < 5*time.Second {
		return nil, pc.lastError
	}

	if err := cfg.validate(); err != nil {
		pc.lastError = err
		pc.lastAttempt = time.Now()
		return nil, err
	}

	if err := validateDiscoveryOrigin(cfg); err != nil {
		pc.lastError = err
		pc.lastAttempt = time.Now()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.discoveryURL())
	if err != nil {
		pc.lastError = fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
		pc.lastAttempt = time.Now()
		pc.provider = nil
		return nil, pc.lastError
	}

	pc.cfg = cfg
	pc.provider = provider
	pc.lastError = nil
	pc.lastAttempt = time.Now()
	return provider, nil
}

func validateDiscoveryOrigin(cfg Config) error {
	discoveryURL, err := url.Parse(cfg.discoveryURL())
	if err != nil {
		return ErrConfigInvalid
	}
	issuerURL, err := url.Parse(cfg.Issuer)
	if err != nil {
		return ErrConfigInvalid
	}
	if !sameOrigin(issuerURL, discoveryURL) {
		return ErrConfigInvalid
	}
	if cfg.DevAllowHTTPLoopback {
		if issuerURL.Scheme != "http" || discoveryURL.Scheme != "http" {
			return ErrConfigInvalid
		}
	} else if issuerURL.Scheme != "https" {
		return ErrConfigInvalid
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}
