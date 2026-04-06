package pipeline

import (
	"container/list"
	"sync"
)

// Cache is a thread-safe LRU cache
type Cache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	lruList *list.List
	maxSize int
	onEvict func(key, value any)
}

// NewCache creates a new bounded LRU cache
func NewCache(maxSize int) *Cache {
	return &Cache{
		items:   make(map[string]*list.Element),
		lruList: list.New(),
		maxSize: maxSize,
	}
}

// SetEvictCallback sets the callback for evicted items
func (c *Cache) SetEvictCallback(fn func(key, value any)) {
	c.onEvict = fn
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(elem)
	return elem.Value.(*cacheEntry).value, true
}

// Put adds or updates a value in the cache
func (c *Cache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		elem.Value.(*cacheEntry).value = value
		c.lruList.MoveToFront(elem)
		return
	}

	// Evict if at capacity
	for c.lruList.Len() >= c.maxSize {
		elem := c.lruList.Back()
		if elem != nil {
			entry := elem.Value.(*cacheEntry)
			delete(c.items, entry.key)
			if c.onEvict != nil {
				c.onEvict(entry.key, entry.value)
			}
			c.lruList.Remove(elem)
		}
	}

	// Add new entry
	entry := &cacheEntry{key: key, value: value}
	elem := c.lruList.PushFront(entry)
	c.items[key] = elem
}

// Remove removes a key from the cache
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return
	}

	delete(c.items, key)
	c.lruList.Remove(elem)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lruList.Init()
}

// Len returns the number of items in the cache
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lruList.Len()
}

type cacheEntry struct {
	key   string
	value any
}
