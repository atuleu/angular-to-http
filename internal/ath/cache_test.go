package ath

import (
	"errors"
	"sync"

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

func (s *CacheSuite) TestGet(c *C) {
	cache := NewCache(-1)
	value, err := cache.Get("foo", func() ([]byte, error) {
		return make([]byte, 1), nil
	})
	c.Check(err, IsNil)
	c.Check(len(value), Equals, 1)

	value, err = cache.Get("foo", func() ([]byte, error) {
		return make([]byte, 2), nil
	})
	c.Check(err, IsNil)
	c.Check(len(value), Equals, 1)

	value, err = cache.Get("bar", func() ([]byte, error) {
		return make([]byte, 1), errors.New("oops")
	})

	c.Check(value, IsNil)
	c.Check(err, ErrorMatches, "oops")
}

func (s *CacheSuite) TestGetRaceCondition(c *C) {
	concurrent := 10
	cache := NewCache(-1)
	wg := sync.WaitGroup{}
	type Result struct {
		Value []byte
		Error error
	}
	results := make(chan Result)

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cache.Get("a", func() ([]byte, error) { return make([]byte, 1), nil })
			results <- Result{value, err}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		c.Check(r.Error, IsNil)
		c.Check(r.Value, HasLen, 1)
	}
}
