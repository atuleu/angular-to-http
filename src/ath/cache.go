package ath

import (
	"container/list"
	"sync"
)

type Creator func() ([]byte, error)

type Cache interface {
	Get(string, Creator) ([]byte, error)
	Size() int64
}

type cacheElement struct {
	element *list.Element
	value   []byte
}

type lruCache struct {
	mx      sync.RWMutex
	data    map[string]*cacheElement
	list    list.List
	size    int64
	maxSize int64
}

func NewCache(maxSize int64) Cache {
	return &lruCache{
		data:    make(map[string]*cacheElement),
		list:    *list.New(),
		size:    0,
		maxSize: maxSize,
	}
}

func (c *lruCache) load(key string) ([]byte, bool) {
	value, ok := c.data[key]
	if ok == false {
		return nil, false
	}
	c.list.MoveToFront(value.element)
	return value.value, true
}

func (c *lruCache) store(key string, value []byte) {
	defer c.evictLeastRecent()
	if actual, ok := c.data[key]; ok == true {
		c.size += int64(cap(value) - cap(actual.value))
		actual.value = value
		c.list.MoveToFront(actual.element)
		return
	}

	c.data[key] = &cacheElement{element: c.list.PushFront(key), value: value}

}

func (c *lruCache) evictLeastRecent() {
	if c.maxSize <= 0 {
		return
	}

	for c.size > c.maxSize {
		back := c.list.Back()
		c.list.Remove(back)
		key := back.Value.(string)
		stored := c.data[key]
		c.size -= int64(cap(stored.value))
		delete(c.data, key)
	}
}

func (c *lruCache) Load(key string) ([]byte, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.load(key)
}

func (c *lruCache) Get(key string, create Creator) ([]byte, error) {
	value, ok := c.Load(key)
	if ok == true {
		return value, nil
	}
	c.mx.Lock()
	defer c.mx.Unlock()

	// reace condition, someone may have created the resource already.
	if stored, ok := c.data[key]; ok == true {
		return stored.value, nil
	}

	value, err := create()
	if err != nil {
		return nil, err
	}

	c.store(key, value)
	return value, nil
}

func (c *lruCache) Size() int64 {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.size
}
