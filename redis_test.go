// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"os"
	"net"
	"testing"
	"time"
	"log"
	"io/ioutil"
	"regexp"
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

// Tests expects to retry redis Dial for defaultRetryThreshold times.
func TestRedisCache_RetryOnConnectionError(t *testing.T) {
	// Assuming no tcp process is running on this port on localhost
	dummyRedisTestServer := "localhost:6666"
	logFile := "/tmp/testLogFile"

	redisCache := NewRedisCache(RedisOpts{
		Host:       dummyRedisTestServer,
		Expiration: time.Duration(1),
	})

	f, err := os.OpenFile(logFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		t.Error(err)
		return
	}

	log.SetOutput(f)

	redisCache.pool.Dial()

	log.SetOutput(os.Stderr)
	f.Close()

	contents, err := ioutil.ReadFile(logFile)
	if err != nil {
		t.Error(err)
		return
	}

	defer os.Remove(logFile)

	r := regexp.MustCompile("Network Error Occured")
	results := r.FindAllStringIndex(string(contents), -1)
	assert.Equal(t, defaultRetryThreshold, len(results), string(contents))

	r = regexp.MustCompile("retrying...")
	results = r.FindAllStringIndex(string(contents), -1)
	assert.Equal(t, defaultRetryThreshold, len(results), string(contents))
}
