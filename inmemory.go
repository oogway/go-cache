// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"encoding/json"
	"time"

	"sync"

	"github.com/patrickmn/go-cache"
)

type InMemoryCache struct {
	cache             cache.Cache   // Only expose the methods we want to make available
	mu                sync.RWMutex  // For increment / decrement prevent reads and writes
	defaultExpiration time.Duration // DefaultExpiration.
}

func NewInMemoryCache(defaultExpiration time.Duration) InMemoryCache {
	return InMemoryCache{
		cache:             *cache.New(defaultExpiration, time.Minute),
		mu:                sync.RWMutex{},
		defaultExpiration: defaultExpiration,
	}
}

func (c InMemoryCache) Get(key string, ptrValue interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, found := c.cache.Get(key)
	if !found {
		return ErrCacheMiss
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, ptrValue)
}

func (c InMemoryCache) GetMulti(keys ...string) (Getter, error) {
	return c, nil
}

func (c InMemoryCache) SetFields(key string, value map[string]interface{}, expires time.Duration) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	existing := map[string]interface{}{}
	v, found := c.cache.Get(key)
	if !found {
		return ErrNotStored
	}

	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytes, &existing); err != nil {
		return err
	}

	for k, v := range value {
		existing[k] = v
	}

	c.cache.Set(key, existing, expires)
	return nil
}

func (c InMemoryCache) Set(key string, value interface{}, expires time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// NOTE: go-cache understands the values of DefaultExpiryTime and ForEverNeverExpiry
	c.cache.Set(key, value, expires)
	return nil
}

func (c InMemoryCache) Add(key string, value interface{}, expires time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.cache.Add(key, value, expires)
	if err != nil {
		return ErrNotStored
	}
	return err
}

func (c InMemoryCache) Replace(key string, value interface{}, expires time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.cache.Replace(key, value, expires); err != nil {
		return ErrNotStored
	}
	return nil
}

func (c InMemoryCache) Keys() ([]string, error) {
	items := func() map[string]cache.Item {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.cache.Items()
	}()

	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}

	return keys, nil
}

func (c InMemoryCache) Delete(key string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.cache.Delete(key)
	return nil
}

func (c InMemoryCache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Flush()
	return nil
}
