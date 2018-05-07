# go-cache

This repository is a hard-fork of [revel-cache](https://github.com/revel/revel/tree/master/cache)

Motivation to do this:
  - light-weight cache library that doesn't download the internet.
  - Doesn't use background go-routines for expiration.
  - Has both in-memory, Redis and more (to be introduced) backends.
