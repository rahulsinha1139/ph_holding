package main

import (
	"sync"
	"time"
)

// CacheEntry represents a cached value with expiration
type CacheEntry struct {
	Value      interface{}
	Expiration time.Time
}

// Cache provides in-memory caching with TTL
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	c := &Cache{
		entries: make(map[string]*CacheEntry),
	}

	// Start background cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a value from cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.Expiration) {
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in cache with TTL
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
}

// Delete removes a value from cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// InvalidatePattern deletes all keys matching a pattern (simple prefix match)
func (c *Cache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(c.entries, key)
		}
	}
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// Stats returns cache statistics
func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := len(c.entries)
	expired := 0
	now := time.Now()

	for _, entry := range c.entries {
		if now.After(entry.Expiration) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_entries":   total,
		"active_entries":  total - expired,
		"expired_entries": expired,
	}
}

// cleanup removes expired entries every minute
func (c *Cache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.removeExpired()
	}
}

// removeExpired deletes all expired entries
func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.Expiration) {
			delete(c.entries, key)
		}
	}
}

// --- Cache Key Constants ---

const (
	CacheKeyCustomerList    = "customers:list"
	CacheKeySupplierList    = "suppliers:list"
	CacheKeyProductCatalog  = "products:catalog"
	CacheKeyRoleList        = "roles:list"
	CacheKeyDashboardStats  = "dashboard:stats"
	CacheKeyFinancialDash   = "financial:dashboard"
	CacheKeyCustomerPrefix  = "customer:"
	CacheKeySupplierPrefix  = "supplier:"
	CacheKeyOrderPrefix     = "order:"
	CacheKeyInvoicePrefix   = "invoice:"
)

// --- Cache TTL Constants ---

const (
	CacheTTLShort  = 1 * time.Minute  // For rapidly changing data (dashboard stats)
	CacheTTLMedium = 5 * time.Minute  // For master data (customers, suppliers)
	CacheTTLLong   = 10 * time.Minute // For static data (products, roles)
)
