// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/httpservice"
	"github.com/mattermost/mattermost/server/v8/platform/services/oidc"
)

type oidcRuntime struct {
	mu         sync.RWMutex
	generation uint64
	client     *oidc.Client
	cfg        oidc.Config
}

type OIDCManager struct {
	httpService httpservice.HTTPService
	runtime     atomic.Pointer[oidcRuntime]
	generation  atomic.Uint64
}

func NewOIDCManager(httpService httpservice.HTTPService) *OIDCManager {
	return &OIDCManager{httpService: httpService}
}

func (m *OIDCManager) Reload(cfg *model.Config) {
	protoCfg, err := oidc.ConfigFromModel(cfg)
	if err != nil || !protoCfg.Enabled {
		m.runtime.Store(nil)
		m.generation.Add(1)
		return
	}

	httpClient := m.httpService.MakeClient(true)
	httpClient.Timeout = 10 * time.Second

	client, err := oidc.NewClient(protoCfg, httpClient)
	if err != nil {
		m.runtime.Store(nil)
		m.generation.Add(1)
		return
	}

	gen := m.generation.Add(1)
	rt := &oidcRuntime{
		generation: gen,
		client:     client,
		cfg:        protoCfg,
	}
	m.runtime.Store(rt)
}

func (m *OIDCManager) Current() (*oidc.Client, oidc.Config, uint64, error) {
	rt := m.runtime.Load()
	if rt == nil {
		return nil, oidc.Config{}, m.generation.Load(), oidc.ErrDisabled
	}
	return rt.client, rt.cfg, rt.generation, nil
}

func (m *OIDCManager) ValidateGeneration(generation uint64) error {
	rt := m.runtime.Load()
	if rt == nil {
		return oidc.ErrDisabled
	}
	if rt.generation != generation {
		return oidc.ErrConfigChanged
	}
	return nil
}
