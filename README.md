# go-cache [![Build Status](https://travis-ci.org/oogway/go-cache.svg?branch=master)](https://travis-ci.org/oogway/go-cache)

This repository is a hard-fork of [revel-cache](https://github.com/revel/revel/tree/master/cache)

Motivation to do this:
  - light-weight cache library that doesn't download the internet.
  - Doesn't use background go-routines for expiration.
  - Has both in-memory, Redis and more (to be introduced) backends.

## Quick Start

Installation

    $ go get https://github.com/oogway/go-cache


## Usage

### In-memory

Example:

```
package main

import (
	"fmt"
	"time"

	cache "github.com/oogway/go-cache"
)

func main() {
	// Use in-memory store
	store := cache.NewInMemoryCache(time.Hour)

	store.Set("key", "value", time.Hour)

	var value string
	// Set the item
	store.Get("key", &value)
	fmt.Println("Key:", value)

	store.Set("num", 1, time.Hour)
	var num int
	// Well, lets check it has set correct
	store.Get("num", &num)
	fmt.Println("Number: ", num)

	// Get incremented value
	incValue, _ := store.Increment("num", 10)
	fmt.Println("Incremented value: ", incValue)

	// Get decremented value
	decValue, _ := store.Decrement("num", 1)
	fmt.Println("Decremented value: ", decValue)

	// Remember Increment/Decrement changes value in memory
	store.Get("num", &num)
	fmt.Println("Number in memory: ", num)

	// Beware Increment/Decrement can result to err
	incValue, err := store.Increment("key", 10)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	// No longer need the item in store? DELETE IT!
	store.Delete("key")

	// Update value
	// NOTE: Replace only works iff the key exists in store
	store.Replace("num", 100, time.Hour)
	store.Get("num", &num)
	fmt.Println("Replaced Number: ", num)

	// Get rid of all keys at once
	store.Flush()
}

```

### Redis

For Redis store just initialize store as follows


```
    store := cache.NewRedisCache(cache.RedisOpts{
        Host:       "",
        Expiration: time.Hour,
    })
```

Empty host assumes redis service on local machine (`localhost:6379`)

Following are the options while initializing Redis store

```
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

```
