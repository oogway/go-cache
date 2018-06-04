// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"time"

	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-redis/redis"
	"github.com/meson10/highbrow"
)

// RedisCache wraps the Redis client to meet the Cache interface.
type RedisCache struct {
	pool              *redis.Client
	defaultExpiration time.Duration
	lockRetries       int
}

const (
	defaultMaxIdle        = 5
	defaultMaxActive      = 0
	defaultTimeoutIdle    = 240
	defaultTimeoutConnect = 10000
	defaultTimeoutRead    = 5000
	defaultTimeoutWrite   = 5000
	defaultHost           = "localhost:6379"
	defaultProtocol       = "tcp"
	defaultRetryThreshold = 5
)

type RedisOpts struct {
	MaxIdle        int
	MaxActive      int
	Protocol       string
	Host           string
	Password       string
	Expiration     time.Duration
	TimeoutConnect int
	TimeoutRead    int
	TimeoutWrite   int
	TimeoutIdle    int
}

func (r RedisOpts) padDefaults() RedisOpts {
	if r.MaxIdle == 0 {
		r.MaxIdle = defaultMaxIdle
	}

	if r.MaxActive == 0 {
		r.MaxActive = defaultMaxActive
	}

	if r.TimeoutConnect == 0 {
		r.TimeoutConnect = defaultTimeoutConnect
	}

	if r.TimeoutIdle == 0 {
		r.TimeoutIdle = defaultTimeoutIdle
	}

	if r.TimeoutRead == 0 {
		r.TimeoutRead = defaultTimeoutRead
	}

	if r.TimeoutWrite == 0 {
		r.TimeoutWrite = defaultTimeoutWrite
	}

	if r.Host == "" {
		r.Host = defaultHost
	}

	if r.Protocol == "" {
		r.Protocol = defaultProtocol
	}

	return r
}

// NewRedisCache returns a new RedisCache with given parameters
// until redigo supports sharding/clustering, only one host will be in hostList
func NewRedisCache(opts RedisOpts) *RedisCache {
	opts = opts.padDefaults()
	toc := time.Millisecond * time.Duration(opts.TimeoutConnect)
	tor := time.Millisecond * time.Duration(opts.TimeoutRead)
	tow := time.Millisecond * time.Duration(opts.TimeoutWrite)
	toi := time.Duration(opts.TimeoutIdle) * time.Second
	opt := &redis.Options{
		Addr:               opts.Host,
		DB:                 0,
		DialTimeout:        toc,
		ReadTimeout:        tor,
		WriteTimeout:       tow,
		PoolSize:           opts.MaxActive,
		PoolTimeout:        30 * time.Second,
		IdleTimeout:        toi,
		Password:           opts.Password,
		IdleCheckFrequency: 500 * time.Millisecond,
	}

	c := redis.NewClient(opt)
	return &RedisCache{pool: c, lockRetries: lockRetries}
}

func (c *RedisCache) Set(key string, value interface{}, expires time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.pool.Set(key, b, expires).Err()
}

const lockRetries = 5

func (c *RedisCache) lockRetry(key string, op func() error) error {
	var breakErr error

	err := highbrow.Try(c.lockRetries, func() error {
		lockKey := fmt.Sprintf("%v-op", key)
		ret, err := c.pool.SetNX(lockKey, "1", 5*time.Second).Result()
		if err != nil {
			breakErr = err
			return nil
		}

		if !ret {
			return errors.New("Cannot get Lock. Retrying.")
		}

		defer func() {
			c.pool.Del(lockKey)
		}()

		breakErr = op()
		return nil
	})

	if breakErr == nil {
		return err
	}
	return breakErr
}

func (c *RedisCache) Add(key string, value interface{}, expires time.Duration) error {
	return c.lockRetry(key, func() error {
		exists, err := c.pool.Exists(key).Result()
		if err != nil {
			return err
		}

		if exists == 0 {
			return c.pool.Set(key, value, expires).Err()
		}

		return ErrNotStored
	})
}

func (c *RedisCache) SetFields(key string, value map[string]interface{}, expires time.Duration) error {
	return c.lockRetry(key, func() error {
		var ptrValue map[string]interface{}
		if err := c.Get(key, &ptrValue); err != nil {
			return err
		}

		for k, v := range value {
			ptrValue[k] = v
		}

		return c.Set(key, value, expires)
	})
}

func (c *RedisCache) Replace(key string, value interface{}, expires time.Duration) error {
	return c.lockRetry(key, func() error {
		exists, err := c.pool.Exists(key).Result()
		if err != nil {
			return err
		}

		if exists == 0 {
			return ErrNotStored
		}

		return c.pool.Set(key, value, expires).Err()
	})

}

func (c *RedisCache) Get(key string, ptrValue interface{}) error {
	b, err := c.pool.Get(key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}

	if err != nil {
		return err
	}

	return json.Unmarshal(b, ptrValue)
}

func (c *RedisCache) GetMulti(keys ...string) (Getter, error) {
	res, err := c.pool.MGet(keys...).Result()
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, ErrCacheMiss
	}

	m := make(map[string]string)
	for ix, key := range keys {
		m[key] = res[ix].(string)
	}
	return RedisItemMapGetter(m), nil
}

func (c *RedisCache) Delete(key string) error {
	return c.pool.Del(key).Err()
}

func (c *RedisCache) Keys() ([]string, error) {
	return c.pool.Keys("*").Result()
}

func (c *RedisCache) Flush() error {
	return c.pool.FlushAll().Err()
}

// RedisItemMapGetter implements a Getter on top of the returned item map.
type RedisItemMapGetter map[string]string

func (g RedisItemMapGetter) Get(key string, ptrValue interface{}) error {
	item, ok := g[key]
	if !ok {
		return ErrCacheMiss
	}

	return json.Unmarshal([]byte(item), ptrValue)
}
