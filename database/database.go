// database/database.go
package database

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Database — thread-safe in-memory key-value store
type Database struct {
	mu       sync.RWMutex      // concurrent access protection
	data     map[string]string // actual key-value data in RAM
	hits     atomic.Int64      // successful GET count
	misses   atomic.Int64      // failed GET count
	commands atomic.Int64      // total commands served
	start    time.Time         // server start time
}

// NewDatabase creates an empty Database
func NewDatabase() *Database {
	return &Database{
		data:  make(map[string]string),
		start: time.Now(),
	}
}

// Set stores a key-value pair
func (db *Database) Set(key, value string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
	db.commands.Add(1)
}

// Get retrieves a value and tells whether the key existed
func (db *Database) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)
	value, exists := db.data[key]
	if exists {
		db.hits.Add(1)
	} else {
		db.misses.Add(1)
	}
	return value, exists
}

// Del removes a key. Returns true if the key existed.
func (db *Database) Del(key string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)
	if _, exists := db.data[key]; exists {
		delete(db.data, key)
		return true
	}
	return false
}

// Exists checks whether a key is present
func (db *Database) Exists(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)
	_, exists := db.data[key]
	return exists
}

// Keys returns all keys sorted alphabetically
func (db *Database) Keys() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	keys := make([]string, 0, len(db.data))
	for k := range db.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Len returns total key count
func (db *Database) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.data)
}

// FlushAll removes everything, returns how many keys were removed
func (db *Database) FlushAll() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	n := len(db.data)
	db.data = make(map[string]string)
	return n
}

// Snapshot returns a safe copy of all data (for display)
func (db *Database) Snapshot() map[string]string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	cp := make(map[string]string, len(db.data))
	for k, v := range db.data {
		cp[k] = v
	}
	return cp
}

// Stats returns aggregated counters
func (db *Database) Stats() (hits, misses, commands int64, uptime time.Duration) {
	return db.hits.Load(), db.misses.Load(), db.commands.Load(), time.Since(db.start)
}
