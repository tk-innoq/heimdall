// Copyright 2024 Dimitrij Drus <dadrus@gmx.de>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisotel"
	"github.com/rs/zerolog"

	"github.com/dadrus/heimdall/internal/cache"
	"github.com/dadrus/heimdall/internal/config"
	"github.com/dadrus/heimdall/internal/heimdall"
	"github.com/dadrus/heimdall/internal/x/errorchain"
)

var ErrConnectionCheckFailed = errors.New("cache connection failed")

// by intention. Used only during application bootstrap.
func init() { // nolint: gochecknoinits
	cache.Register("redis", &cacheFactory{})
}

type cacheFactory struct{}

func (*cacheFactory) Create(conf map[string]any) (cache.Cache, error) {
	return NewCache(conf)
}

type Cache struct {
	c rueidis.Client
}

func NewCache(conf map[string]any) (*Cache, error) {
	type Config struct {
		Addrs        []string    `mapstructure:"addrs"         validate:"gt=0,dive,required"`
		Username     string      `mapstructure:"username"`
		Password     string      `mapstructure:"password"`
		DB           int         `mapstructure:"db"`
		TLS          *config.TLS `mapstructure:"tls"`
		AdditionalCa string      `mapstructure:"additional_ca"`
		DisableCache bool        `mapstructure:"disable_cache"`
	}

	var cfg Config

	if err := decodeConfig(conf, &cfg); err != nil {
		return nil, err
	}

	opts := rueidis.ClientOption{
		ClientName:   "heimdall",
		InitAddress:  cfg.Addrs,
		ShuffleInit:  true,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DisableCache: cfg.DisableCache,
		SelectDB:     cfg.DB,
	}

	if cfg.TLS != nil && len(cfg.TLS.KeyStore.Path) != 0 {
		tlsConfig, err := configureTLS(cfg.TLS, cfg.AdditionalCa)
		if err != nil {
			return nil, errorchain.NewWithMessage(heimdall.ErrConfiguration,
				"failed configuring tls for redis cache").CausedBy(err)
		}

		opts.TLSConfig = tlsConfig
	}

	client, err := rueidisotel.NewClient(opts)
	if err != nil {
		return nil, errorchain.NewWithMessage(heimdall.ErrInternal,
			"failed creating redis cache client").CausedBy(err)
	}

	if err := client.Do(context.Background(), client.B().Ping().Build()).Error(); err != nil {
		return nil, errorchain.NewWithMessage(ErrConnectionCheckFailed, "failed connect to redis cache").
			CausedBy(err)
	}

	return &Cache{c: client}, nil
}

func (c *Cache) Start(_ context.Context) error {
	// not used for Redis.
	return nil
}

func (c *Cache) Stop(_ context.Context) error {
	c.c.Close()

	return nil
}

func (c *Cache) Get(ctx context.Context, key string) any {
	val, err := c.c.DoCache(ctx, c.c.B().Get().Key(key).Cache(), time.Minute).ToString()
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to fetch value from cache")

		return nil
	}

	return val
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	str := fmt.Sprintf("%s", value)

	if err := c.c.Do(ctx, c.c.B().Set().Key(key).Value(str).Px(ttl).Build()).Error(); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to store value in cache")
	}
}

func (c *Cache) Delete(ctx context.Context, key string) {
	// UNLINK removes the key asynchronously; so we are not blocking here.
	if err := c.c.Do(ctx, c.c.B().Unlink().Key(key).Build()).Error(); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to unlink value from cache")
	}
}
