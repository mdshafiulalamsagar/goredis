// database/database.go
package database

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Item represents a single value stored in the database along with its expiration time.
// If ExpiresAt is the zero value of time.Time, it means the key never expires.
type Item struct {
	Value     string
	ExpiresAt time.Time
}

// Database is a thread-safe in-memory key-value store.
type Database struct {
	mu       sync.RWMutex      // Mutex to protect concurrent access to the data map.
	data     map[string]Item   // The actual key-value data stored in RAM.
	hits     atomic.Int64      // Tracks the number of successful GET requests.
	misses   atomic.Int64      // Tracks the number of failed GET requests.
	commands atomic.Int64      // Tracks the total number of commands executed.
	start    time.Time         // The time when the database server started.
}

// NewDatabase initializes a new empty Database and starts the background expiration worker.
func NewDatabase() *Database {
	db := &Database{
		data:  make(map[string]Item),
		start: time.Now(),
	}
	
	// Start the background routine to clean up expired keys.
	db.startExpirationWorker()
	
	return db
}

// Set stores a key-value pair. If ttl is greater than zero, the key will expire after the given duration.
func (db *Database) Set(key, value string, ttl time.Duration) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	db.data[key] = Item{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	db.commands.Add(1)
}

// Get retrieves a value. It returns false if the key does not exist or has expired.
func (db *Database) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		db.misses.Add(1)
		return "", false
	}

	// Check if the item has expired. If so, treat it as non-existent.
	// (Lazy deletion: the background worker will eventually delete it from memory)
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		db.misses.Add(1)
		return "", false
	}

	db.hits.Add(1)
	return item.Value, true
}

// Del removes a key from the database. Returns true if the key existed and was deleted.
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

// Exists checks if a key is present and has not expired.
func (db *Database) Exists(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		return false
	}

	// Check for expiration.
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return false
	}

	return true
}

// TTL returns the remaining time-to-live for a key.
// Returns -2 if the key does not exist, -1 if the key exists but has no expiration.
func (db *Database) TTL(key string) int64 {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		return -2
	}

	if item.ExpiresAt.IsZero() {
		return -1
	}

	remaining := time.Until(item.ExpiresAt)
	if remaining < 0 {
		return -2
	}

	return int64(remaining.Seconds())
}

// Expire sets a timeout on an existing key. Returns true if the timeout was set.
func (db *Database) Expire(key string, ttl time.Duration) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		return false
	}

	// If the key is already expired, delete it and return false.
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		delete(db.data, key)
		return false
	}

	item.ExpiresAt = time.Now().Add(ttl)
	db.data[key] = item
	return true
}

// Persist removes the expiration from a key, making it permanent.
func (db *Database) Persist(key string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		return false
	}

	// If the key has no expiration, Persist does nothing.
	if item.ExpiresAt.IsZero() {
		return false
	}

	item.ExpiresAt = time.Time{} // Reset to zero value (no expiration)
	db.data[key] = item
	return true
}

// Keys returns all active (non-expired) keys sorted alphabetically.
func (db *Database) Keys() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	keys := make([]string, 0, len(db.data))
	now := time.Now()

	for k, v := range db.data {
		// Filter out expired keys from the result.
		if v.ExpiresAt.IsZero() || now.Before(v.ExpiresAt) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Len returns the total count of active (non-expired) keys.
func (db *Database) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, v := range db.data {
		if v.ExpiresAt.IsZero() || now.Before(v.ExpiresAt) {
			count++
		}
	}
	return count
}

// FlushAll removes everything from the database and returns the number of removed keys.
func (db *Database) FlushAll() int {
	db.mu.Lock()
	defer db.mu.Unlock()

	n := len(db.data)
	db.data = make(map[string]Item)
	return n
}

// Snapshot returns a safe copy of all active data for display purposes.
func (db *Database) Snapshot() map[string]Item {
	db.mu.RLock()
	defer db.mu.RUnlock()

	cp := make(map[string]Item)
	now := time.Now()
	
	for k, v := range db.data {
		// Only include keys that are not expired.
		if v.ExpiresAt.IsZero() || now.Before(v.ExpiresAt) {
			cp[k] = v
		}
	}
	return cp
}

// Stats returns aggregated server counters and uptime.
func (db *Database) Stats() (hits, misses, commands int64, uptime time.Duration) {
	return db.hits.Load(), db.misses.Load(), db.commands.Load(), time.Since(db.start)
}

// startExpirationWorker runs in a background goroutine and periodically deletes expired keys.
func (db *Database) startExpirationWorker() {
	go func() {
		// Ticker triggers every 1 second.
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			db.mu.Lock()
			now := time.Now()
			
			// Iterate through the map and delete keys whose expiration time has passed.
			for k, v := range db.data {
				if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
					delete(db.data, k)
				}
			}
			
			db.mu.Unlock()
		}
	}()
}