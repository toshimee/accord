/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inventory

import "sync"

// HashCache stores the last material SHA-256 per object key inside the inventory-controller process.
type HashCache struct {
	mu sync.RWMutex
	m  map[string]string
}

// NewHashCache returns an empty HashCache.
func NewHashCache() *HashCache {
	return &HashCache{m: make(map[string]string)}
}

// Get returns the cached hash if present.
func (c *HashCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[key]
	return v, ok
}

// Set records the latest hash for an object key.
func (c *HashCache) Set(key, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = hash
}

// Delete removes a cache entry (for example after object deletion).
func (c *HashCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

func objectKey(namespace, name string) string {
	return namespace + "/" + name
}
