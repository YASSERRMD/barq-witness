package explainer

import "sync"

// lruCache is a simple goroutine-safe in-memory LRU string cache.
type lruCache struct {
	mu      sync.Mutex
	cap     int
	order   []string          // insertion order, oldest first
	entries map[string]string
}

func newLRUCache(capacity int) *lruCache {
	if capacity <= 0 {
		capacity = 128
	}
	return &lruCache{
		cap:     capacity,
		order:   make([]string, 0, capacity),
		entries: make(map[string]string, capacity),
	}
}

// Get retrieves a value from the cache.
func (c *lruCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

// Set inserts or updates a value.  When at capacity the oldest entry is evicted.
func (c *lruCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		if len(c.order) >= c.cap {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}
	c.entries[key] = value
}
