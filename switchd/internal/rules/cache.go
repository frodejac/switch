package rules

import (
	"sync"
)

// Cache provides thread-safe caching of compiled rules
type Cache struct {
	programs sync.Map
}

// NewCache creates a new rule cache
func NewCache() *Cache {
	return &Cache{}
}

// Get retrieves a cached program
func (c *Cache) Get(key string) (*Program, bool) {
	if value, ok := c.programs.Load(key); ok {
		return value.(*Program), true
	}
	return nil, false
}

// Set stores a program in the cache
func (c *Cache) Set(key string, program *Program) {
	c.programs.Store(key, program)
}

// Delete removes a program from the cache
func (c *Cache) Delete(key string) {
	c.programs.Delete(key)
}

// Clear removes all programs from the cache
func (c *Cache) Clear() {
	c.programs = sync.Map{}
}
