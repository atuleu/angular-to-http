package ath

import (
	. "gopkg.in/check.v1"
)

type CacheSuite struct{}

var _ = Suite(&CacheSuite{})

func (s *CacheSuite) TestComputeSize(c *C) {
	cache := NewCache(-1)

	cache.Store("a", make([]byte, 0, 1024))
	c.Check(int(cache.Size()), Equals, 1024)

	cache.Store("b", make([]byte, 1024*1024))
	c.Check(int(cache.Size()), Equals, 1024*1024+1024)

	cache.Store("c", make([]byte, 0, 1024))
	c.Check(int(cache.Size()), Equals, 1024*1024+2*1024)

	cache.Store("b", make([]byte, 0, 1024))
	c.Check(int(cache.Size()), Equals, 3*1024)
}

func hasKey(cache Cache, key string) bool {
	_, ok := cache.Load(key)
	return ok
}

func (s *CacheSuite) TestEviction(c *C) {
	cache := NewCache(3 * 1024)

	cache.Store("a", make([]byte, 0, 1024))
	c.Check(hasKey(cache, "a"), Equals, true)

	cache.Store("b", make([]byte, 0, 1024))
	c.Check(hasKey(cache, "a"), Equals, true)
	c.Check(hasKey(cache, "b"), Equals, true)

	cache.Store("c", make([]byte, 0, 1024))
	c.Check(hasKey(cache, "a"), Equals, true)
	c.Check(hasKey(cache, "b"), Equals, true)
	c.Check(hasKey(cache, "c"), Equals, true)

	cache.Store("d", make([]byte, 0, 1024))
	c.Check(hasKey(cache, "a"), Equals, false)
	c.Check(hasKey(cache, "b"), Equals, true)
	c.Check(hasKey(cache, "c"), Equals, true)
	c.Check(hasKey(cache, "d"), Equals, true)

	cache.Store("a", make([]byte, 0, 1024*1024))
	c.Check(hasKey(cache, "a"), Equals, false)
	c.Check(hasKey(cache, "b"), Equals, true)
	c.Check(hasKey(cache, "c"), Equals, true)
	c.Check(hasKey(cache, "d"), Equals, true)

	cache.Store("a", make([]byte, 0, 3*1024))
	c.Check(hasKey(cache, "a"), Equals, true)
	c.Check(hasKey(cache, "b"), Equals, false)
	c.Check(hasKey(cache, "c"), Equals, false)
	c.Check(hasKey(cache, "d"), Equals, false)

	cache.Store("b", make([]byte, 0, 1024))
	c.Check(hasKey(cache, "a"), Equals, false)
	c.Check(hasKey(cache, "b"), Equals, true)
	c.Check(hasKey(cache, "c"), Equals, false)
	c.Check(hasKey(cache, "d"), Equals, false)

}
