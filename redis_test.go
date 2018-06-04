// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmizerany/assert"
)

// These tests require redis server running on localhost:6379 (the default)
const redisTestServer = "localhost:6379"

var newRedisCache = func(t *testing.T, defaultExpiration time.Duration) Cache {
	c, err := net.Dial("tcp", redisTestServer)
	if err == nil {
		if _, err = c.Write([]byte("flush_all\r\n")); err != nil {
			t.Errorf("Write failed: %s", err)
		}
		_ = c.Close()

		redisCache := NewRedisCache(RedisOpts{
			Host:       redisTestServer,
			Expiration: defaultExpiration,
		})
		if err = redisCache.Flush(); err != nil {
			t.Errorf("Flush failed: %s", err)
		}
		return redisCache
	}
	t.Errorf("couldn't connect to redis on %s", redisTestServer)
	t.FailNow()
	panic("")
}

func TestRedisCache_TypicalGetSet(t *testing.T) {
	typicalGetSet(t, newRedisCache)
}

func TestRedisCache_Expiration(t *testing.T) {
	expiration(t, newRedisCache)
}

func TestRedisCache_EmptyCache(t *testing.T) {
	emptyCache(t, newRedisCache)
}

func TestRedisCache_SetFields(t *testing.T) {
	testSetFields(t, newRedisCache)
}

func TestRedisCache_Replace(t *testing.T) {
	testReplace(t, newRedisCache)
}

func TestRedisCache_Add(t *testing.T) {
	testAdd(t, newRedisCache)
}

func TestRedisCache_GetMulti(t *testing.T) {
	testGetMulti(t, newRedisCache)
}

func TestRedisCache_Keys(t *testing.T) {
	testKeys(t, newRedisCache)
}

func TestRedisCache_LockRetry(t *testing.T) {

	cache := newRedisCache(t, testExpiryTime)
	x, ok := cache.(*RedisCache)
	if !ok {
		t.Fatalf("Cannot convert racache")
	}

	x.lockRetries = 1

	var counter int64
	var errors int64

	var wg sync.WaitGroup
	for _, ix := range []int{1, 2} {
		wg.Add(1)

		go func(ix int) {
			defer wg.Done()

			if err := x.lockRetry("mohan", func() error {
				time.Sleep(2 * time.Second)
				return nil
			}); err != nil {
				atomic.AddInt64(&errors, 1)
			} else {
				atomic.AddInt64(&counter, 1)
			}
		}(ix)
	}

	wg.Wait()
	assert.Equal(t, int64(1), counter)
	assert.Equal(t, int64(1), errors)
}
