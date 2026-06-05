//go:build migrated_fynedo

package async

import "sync"

// Map is a generic wrapper around [sync.Map].
type Map[K any, V any] struct {
	// Use "comparable" as type constraint and map[K]V as the inner type
	// once Go 1.20 is our minimum version so interfaces can be used as keys.
	lock sync.RWMutex
	m    map[any]V
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.m, key)
}

// Len returns the length of the map. It is O(n) over the number of items.
func (m *Map[K, V]) Len() (count int) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.m)
}

// Load returns the value stored in the map for a key, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	val, ok := m.m[key]
	return val, ok
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, ok := m.m[key]
	delete(m.m, key)
	return val, ok
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.m == nil {
		m.m = make(map[any]V)
	}

	if val, ok := m.m[key]; ok {
		return val, true
	}
	m.m[key] = value
	return value, false
}

// Range calls f sequentially for each key and value present in the map. If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.lock.RLock()
	entries := make([]struct {
		key   K
		value V
	}, 0, len(m.m))
	for k, v := range m.m {
		entries = append(entries, struct {
			key   K
			value V
		}{k.(K), v})
	}
	m.lock.RUnlock()

	for _, entry := range entries {
		if !f(entry.key, entry.value) {
			return
		}
	}
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.m == nil {
		m.m = make(map[any]V)
	}
	m.m[key] = value
}

// Clear removes all entries from the map.
func (m *Map[K, V]) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.m = make(map[any]V) // Use range-and-delete loop once Go 1.20 is the minimum version.
}
