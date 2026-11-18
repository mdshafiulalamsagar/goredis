// database/database.go
package database

import (
	"sync"
	"sync/atomic"
	"time"
)

type Item struct {
	Type      string      // "string", "list", "hash"
	Value     interface{} // holds string, []string, or map[string]string
	ExpiresAt time.Time
}

type Database struct {
	mu       sync.RWMutex
	data     map[string]Item
	hits     atomic.Int64
	misses   atomic.Int64
	commands atomic.Int64
	start    time.Time
}

func NewDatabase() *Database {
	db := &Database{
		data:  make(map[string]Item),
		start: time.Now(),
	}
	db.startExpirationWorker()
	return db
}

// Set stores a string value.
func (db *Database) Set(key, value string, ttl time.Duration) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	db.data[key] = Item{Type: "string", Value: value, ExpiresAt: expiresAt}
	db.commands.Add(1)
}

// Get retrieves a string value.
func (db *Database) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists || item.Type != "string" {
		db.misses.Add(1)
		return "", false
	}
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		db.misses.Add(1)
		return "", false
	}
	db.hits.Add(1)
	return item.Value.(string), true
}

// LPush adds an element to the head of a list.
func (db *Database) LPush(key string, value string) int {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		item = Item{Type: "list", Value: []string{}}
	}
	if item.Type != "list" {
		return -1 // Wrong type error
	}

	list := item.Value.([]string)
	list = append([]string{value}, list...) // Prepend
	item.Value = list
	db.data[key] = item
	return len(list)
}

// LRange gets a range of elements from a list.
func (db *Database) LRange(key string, start, stop int) ([]string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists || item.Type != "list" {
		return nil, false
	}
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	list := item.Value.([]string)
	length := len(list)

	if start < 0 { start = length + start }
	if stop < 0 { stop = length + stop }
	if start < 0 { start = 0 }
	if stop >= length { stop = length - 1 }
	if start > stop || start >= length { return []string{}, true }

	return list[start : stop+1], true
}

// HSet sets a field in a hash.
func (db *Database) HSet(key, field, value string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists {
		item = Item{Type: "hash", Value: make(map[string]string)}
	}
	if item.Type != "hash" {
		return false // Wrong type
	}

	hash := item.Value.(map[string]string)
	_, fieldExists := hash[field]
	hash[field] = value
	db.data[key] = item
	return !fieldExists // true if new field was created
}

// HGet retrieves a field from a hash.
func (db *Database) HGet(key, field string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists || item.Type != "hash" {
		return "", false
	}
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return "", false
	}

	hash := item.Value.(map[string]string)
	val, ok := hash[field]
	return val, ok
}

// HGetAll retrieves all fields and values from a hash.
func (db *Database) HGetAll(key string) (map[string]string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)

	item, exists := db.data[key]
	if !exists || item.Type != "hash" {
		return nil, false
	}
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	// Return a copy to prevent concurrent map modification outside the lock
	hash := item.Value.(map[string]string)
	cp := make(map[string]string, len(hash))
	for k, v := range hash {
		cp[k] = v
	}
	return cp, true
}

// Del removes a key.
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

// Exists checks if a key exists.
func (db *Database) Exists(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)
	item, exists := db.data[key]
	if !exists { return false }
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) { return false }
	return true
}

// TTL, Expire, Persist, Len, FlushAll remain exactly the same as before.
func (db *Database) TTL(key string) int64 {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.commands.Add(1)
	item, exists := db.data[key]
	if !exists { return -2 }
	if item.ExpiresAt.IsZero() { return -1 }
	remaining := time.Until(item.ExpiresAt)
	if remaining < 0 { return -2 }
	return int64(remaining.Seconds())
}

func (db *Database) Expire(key string, ttl time.Duration) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)
	item, exists := db.data[key]
	if !exists { return false }
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		delete(db.data, key)
		return false
	}
	item.ExpiresAt = time.Now().Add(ttl)
	db.data[key] = item
	return true
}

func (db *Database) Persist(key string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.commands.Add(1)
	item, exists := db.data[key]
	if !exists || item.ExpiresAt.IsZero() { return false }
	item.ExpiresAt = time.Time{}
	db.data[key] = item
	return true
}

func (db *Database) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	count := 0
	now := time.Now()
	for _, v := range db.data {
		if v.ExpiresAt.IsZero() || now.Before(v.ExpiresAt) { count++ }
	}
	return count
}

func (db *Database) FlushAll() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	n := len(db.data)
	db.data = make(map[string]Item)
	return n
}

func (db *Database) Snapshot() map[string]Item {
	db.mu.RLock()
	defer db.mu.RUnlock()
	cp := make(map[string]Item)
	now := time.Now()
	for k, v := range db.data {
		if v.ExpiresAt.IsZero() || now.Before(v.ExpiresAt) { cp[k] = v }
	}
	return cp
}

func (db *Database) Stats() (hits, misses, commands int64, uptime time.Duration) {
	return db.hits.Load(), db.misses.Load(), db.commands.Load(), time.Since(db.start)
}

func (db *Database) startExpirationWorker() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			db.mu.Lock()
			now := time.Now()
			for k, v := range db.data {
				if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
					delete(db.data, k)
				}
			}
			db.mu.Unlock()
		}
	}()
}