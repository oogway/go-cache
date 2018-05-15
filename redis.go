// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"time"

	"encoding/json"

	"github.com/gomodule/redigo/redis"
	"github.com/meson10/highbrow"
	"net"
	"log"
)

// RedisCache wraps the Redis client to meet the Cache interface.
type RedisCache struct {
	pool              *redis.Pool
	defaultExpiration time.Duration
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

func NewRedisCache(opts RedisOpts) RedisCache {
	opts = opts.padDefaults()
	var pool = &redis.Pool{
		MaxIdle:     opts.MaxIdle,
		MaxActive:   opts.MaxActive,
		IdleTimeout: time.Duration(opts.TimeoutIdle) * time.Second,
		Dial: func() (redis.Conn, error) {
			toc := time.Millisecond * time.Duration(opts.TimeoutConnect)
			tor := time.Millisecond * time.Duration(opts.TimeoutRead)
			tow := time.Millisecond * time.Duration(opts.TimeoutWrite)

			var c redis.Conn
			var err error

			highbrow.Try(defaultRetryThreshold, func() error {
				c, err = redis.Dial(opts.Protocol, opts.Host,
					redis.DialConnectTimeout(toc),
					redis.DialReadTimeout(tor),
					redis.DialWriteTimeout(tow))

				if err == nil {
					return nil
				}

				if _, ok := err.(net.Error); ok {
					log.Printf("Network Error Occured: %v, retrying...", err)
					return err
				}

				return nil
			})

			if err != nil {
				return nil, err
			}
			if len(opts.Password) > 0 {
				if _, err = c.Do("AUTH", opts.Password); err != nil {
					_ = c.Close()
					return nil, err
				}
			} else {
				// check with PING
				if _, err = c.Do("PING"); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			return c, err
		},
		// custom connection test method
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	return RedisCache{pool, opts.Expiration}
}

func (c RedisCache) Set(key string, value interface{}, expires time.Duration) error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	return c.invoke(conn.Do, key, value, expires)
}

func (c RedisCache) Add(key string, value interface{}, expires time.Duration) error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	existed, err := exists(conn, key)
	if err != nil {
		return err
	} else if existed {
		return ErrNotStored
	}
	return c.invoke(conn.Do, key, value, expires)
}

func (c RedisCache) Replace(key string, value interface{}, expires time.Duration) error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	existed, err := exists(conn, key)
	if err != nil {
		return err
	} else if !existed {
		return ErrNotStored
	}

	err = c.invoke(conn.Do, key, value, expires)
	if value == nil {
		return ErrNotStored
	}
	return err
}

func (c RedisCache) Get(key string, ptrValue interface{}) error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	raw, err := conn.Do("GET", key)
	if err != nil {
		return err
	} else if raw == nil {
		return ErrCacheMiss
	}
	item, err := redis.Bytes(raw, err)
	if err != nil {
		return err
	}
	return json.Unmarshal(item, ptrValue)
}

func generalizeStringSlice(strs []string) []interface{} {
	ret := make([]interface{}, len(strs))
	for i, str := range strs {
		ret[i] = str
	}
	return ret
}

func (c RedisCache) GetMulti(keys ...string) (Getter, error) {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	items, err := redis.Values(conn.Do("MGET", generalizeStringSlice(keys)...))
	if err != nil {
		return nil, err
	} else if items == nil {
		return nil, ErrCacheMiss
	}

	m := make(map[string][]byte)
	for i, key := range keys {
		m[key] = nil
		if i < len(items) && items[i] != nil {
			s, ok := items[i].([]byte)
			if ok {
				m[key] = s
			}
		}
	}
	return RedisItemMapGetter(m), nil
}

func exists(conn redis.Conn, key string) (bool, error) {
	return redis.Bool(conn.Do("EXISTS", key))
}

func (c RedisCache) Delete(key string) error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	existed, err := redis.Bool(conn.Do("DEL", key))
	if err == nil && !existed {
		err = ErrCacheMiss
	}
	return err
}

func (c RedisCache) Keys() ([]string, error) {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	return redis.Strings(conn.Do("KEYS", "*"))
}


func (c RedisCache) Flush() error {
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	_, err := conn.Do("FLUSHALL")
	return err
}

func (c RedisCache) invoke(f func(string, ...interface{}) (interface{}, error),
	key string, value interface{}, expires time.Duration) error {

	switch expires {
	case DefaultExpiryTime:
		expires = c.defaultExpiration
	case ForEverNeverExpiry:
		expires = time.Duration(0)
	}

	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	conn := c.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	if expires > 0 {
		_, err = f("SETEX", key, int32(expires/time.Second), b)
		return err
	}
	_, err = f("SET", key, b)
	return err
}

// RedisItemMapGetter implements a Getter on top of the returned item map.
type RedisItemMapGetter map[string][]byte

func (g RedisItemMapGetter) Get(key string, ptrValue interface{}) error {
	item, ok := g[key]
	if !ok {
		return ErrCacheMiss
	}
	return json.Unmarshal(item, ptrValue)
}
